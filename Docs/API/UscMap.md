# USC 映射规则（UscMap）

> 接口（前缀 `/API/V1/Meta/Finv/Quote/UscMap`）：
> - `GET /Status` —— 只读快照（规则/条目数、结果表、键表匹配率、cron）
> - `GET /Rule/List` —— 规则层列表（含条目数）
> - `GET /Item/List` —— 条目分页（`ruleNo` / `market` / `status` / `keyword`）
> - `POST /Item/Save` —— 新增或编辑条目（`item_desc` 必填）
> - `POST /Item/Retire` —— 停用条目（保留行，可再改回 ACTIVE）
> - `GET /Diff/Preview` —— 预演差异（不落库，返回 `preview_token`）
> - `GET /Diff/Page` —— 分页浏览最近一次预演的**完整明细**（`token` / `diff` / `page` / `pageSize`）
> - `POST /Sync/Run` —— 异步应用（需带预演令牌，返回 `run_id`）
> - `GET /Sync/Run` —— 运行时快照（进度 + 日志，页面 800ms 轮询）
> - `GET /Log/List` —— 审计日志分页
> - `GET /Schedule` / `POST /Schedule` —— 读取/保存每日同步 cron（`sys_config 20015`，空串=关闭）
>
> 需求：11072（USC 映射规则结构化建表 + 维护/预演/应用）、11073（页面与菜单）
> 权限：需登录（内部接口）
> 关联表：`futu.finv_futu_usc_rule` / `futu.finv_futu_usc_rule_item` / `futu.finv_futu_usc_map` /
> `futu.finv_futu_usc_rule_log` / `archive.finv_usc_reuse_snapshot`（迁移 `V300157`）
> 关联菜单：[`Menu/Quote/Realtime/UscMapping.md`](../../Menu/Quote/Realtime/UscMapping.md)
> 契约：[`UscMap.yml`](UscMap.yml)

## 1. 用途

把证券代码映射（USC）规则从文档/手写 SQL 固化为**可维护的数据**，并支撑：

- 新增上市证券：往条目层插一行即可（不用改代码、不用重发脚本）；
- 退市/过期：条目 `status='RETIRED'` + `effective_to`，结果表行保留但释放 `usc`；
- 周期品种（期货主力）：`code_regex` + `effective_from/to` 描述滚动规则；
- 环境差异：`scope_env`（`*` / `prod` / `dev`）；
- 变更可追溯：`finv_futu_usc_rule_log` 记 before/after，结果表留 `rule_no` + `basis`。

## 2. 计算口径

```
① 前处理 futu.f_finv_futu_usc_calc(env)
     P12 指数别名/特殊指数（LITERAL）→ P14 期货后缀（REWRITE）→ P16 指数代码去前导点（REWRITE）
     → P22 SH 000* 指数（SH4）→ canon_code
     另：富途全码里的双点会被折叠成单点（`AU..XFL` → `AU.XFL`），最终 usc 不带前导点。
② P10 闸门：lower(canon_code) 在键表里计数 = 1 → usc = canon_code，定稿
③ 否则**先分层**（精确 code → 市场+品种正则 → 市场默认 → 兜底 P60）、**同层内**按 ru.priority 取第一条命中条目
     动作：CODE=canon_code / FTMM=ftmm_symbol / LITERAL=usc_literal
     （分层不能省：只按 ru.priority 排会让市场默认 P20=20 压过例外 P40=40/P50=50，实测 660 条例外里 658 条被抢走）
④ 冲突升级：候选与其它行 lower() 撞名 → 改用 ftmm_symbol（结果表 rule_no 记 ESCALATE）
⑤ 升级后仍撞名 → 本次拒绝（HTTP 500 / 运行 FAILED），人工修规则
```

**口径示例（XRP 家族）**：同一个 `code=XRP` 在三个市场各有归属，靠分层与例外区分 ——
加密现货 `CC.XRP` 占住主读法 `XRP`（P40 精确例外，`LITERAL XRP`）、美股 ETF `US.XRP` 用富途全码
`US.XRP`（P40 精确例外，`FTMM`；需求 11034 的 `ETFXRP` 自造读法已于需求 11097 作废）、
加拿大 ETF `CA.XRP` 走市场默认全码 `CA.XRP`（P21）。
`CC.XRP` 早期按「同名时加密侧用全码」写成了 `FTMM` 条目并已停用，需求 11096 起改由
`P40 / CC / XRP → LITERAL XRP` 固化，不再依赖 P20 市场默认顺带得出。

## 3. 接口要点

### 3.1 `GET /Status`

| 字段 | 说明 |
|------|------|
| `rule_count` / `item_count` / `item_active` | 规则数 / 条目数 / 生效条目数 |
| `map_active` / `map_retire` / `map_reuse` | 结果表在册 / 已退市 / 复用留痕行数 |
| `key_total` / `key_match` / `key_miss` | 键表行数 / 与结果表 `lower(usc)` 对上的行数 / 对不上的行数 |
| `last_compute_at` / `rule_version` | 最近计算时间 / 规则集版本（规则与条目指纹的前 12 位） |
| `cron_expr` / `cron_next_run` | 系统配置 `20015` 的表达式与下次执行时间（**空 = 自动同步关闭**） |
| `archive_rows` | 复用归档表行数 |

### 3.2 `POST /Item/Save`

请求体为条目字段的 snake_case JSON（`idn=0` 表示新增）。服务端校验：

- `item_desc` 必填（Web 页要显示「为什么这么映射」）；
- `action=LITERAL` 必须有 `usc_literal`；`action=REWRITE` 必须有 `code_regex` + `rewrite_tpl`；
- `code` 与 `code_regex` 只能二选一。

保存成功写一条 `rule_item` 审计日志（INSERT/UPDATE + before/after）。

`idn>0` 与 `idn=0` 走**两条不同语句**：前者 `UPDATE ... WHERE idn`（改键也算编辑，条目不存在时报错），
后者 `INSERT`。条目的自然键 `(rule_no, scope_env, market, instrument, code, code_regex)`
（`NULLS NOT DISTINCT`）唯一：新增时若该键已存在，返回 **409** 并提示已有条目的 `idn`/状态，
让用户改去编辑那条（不再把数据库的 23505 原始报错抛给前端）。

### 3.3 `GET /Diff/Preview`

不写任何表：算完差异整体回滚，返回分类计数、前 200 行明细、冲突清单与 `preview_token`
（= `md5(全部最终映射)`）。首次上线（结果表为空）时 `insert` 等于键表行数。

分类计数（`中文字段`）：

| 字段 | 含义 |
|------|------|
| `insert` | 结果表缺该证券 |
| `update` | 结果表有但 `usc` 与本次计算不同 |
| `retire` | 键表已无该证券（结果表行标记退市、释放 `usc`） |
| `reuse` | 本次改用的 `usc` 曾被别的退市证券占用（需归档下游） |
| `keep` | 结果表与计算一致 |
| `escalate` | 候选 `usc` 撞名后改用 `ftmm_symbol` |
| `key_update` | **键表漂移**：`futu.finv_futu_secu_key.usc` 与规则结果不一致（手工改值 / 外部同步覆盖），应用时回写；这类行结果表本身没变，所以单独计数，不计入 `update` |

### 3.3.1 `GET /Diff/Page`

预演不落库，明细也没法从库里再查：`/Diff/Preview` 只回前 **200** 行，完整明细（通常 1.6 万～7 万行）
由服务端缓存在内存，用本接口分页浏览。

| 参数 | 说明 |
|------|------|
| `token` | 预演令牌；与缓存不一致（重新预演过 / 服务重启）返回 400，提示重新预演 |
| `diff` | `insert` / `update` / `retire` / `key_update`；空 = 全部（`keep` 不返回） |
| `keyword` | 关键字，大小写不敏感地匹配 `ftmm_symbol` / `code` / 当前 usc / 新 usc（几万条里直接定位一只证券，不用翻页） |
| `page` / `pageSize` | 分页，页大小上限 200 |

缓存有效期 **30 分钟**，重新预演会整体替换；服务重启后失效。返回体含 `total`（该类别总数）与 `preview_token`。

### 3.3.2 `GET /Map/Page`

**结果表随时查询**，与预演无关（不需要先预演、也不受预演缓存的 30 分钟有效期限制）：
直接分页读 `futu.finv_futu_usc_map`（预演与应用写入的在册映射）。

| 参数 | 说明 |
|------|------|
| `keyword` | 关键字，大小写不敏感地匹配 `ftmm_symbol` / `code` / `usc`（例如 `US.XRP`、`XRP`、`ETFXRP`） |
| `market` | 市场标签精确匹配（`US` / `SH` / `…`）；空 = 全部 |
| `status` | `ACTIVE` / `RETIRED`；空 = 全部 |
| `page` / `pageSize` | 分页，页大小上限 200 |

返回 `total` + `list`（`ftmm_symbol` / `market` / `code` / `instrument` / `usc` / `rule_no` / `basis` /
`status` / `scope_env` / `rule_version` / `dt_compute` / `dt_retire` / `reuse_of_ftmm`）。

### 3.4 `POST /Sync/Run`

请求体：`{"preview_token":"...","reason":"...","env":"*"}`（`operator` 取登录账号）。
服务端校验令牌：预演之后规则或键表变了 → `400/409` 拒绝（`预演结果已过期`），必须重新预演。

应用顺序（单事务）：

| 步骤 | 动作 |
|------|------|
| ① 归档 | `usc` 被新证券接管时，把下游各表历史行整行快照进 `archive.finv_usc_reuse_snapshot`（`to_regclass` 守卫，缺表跳过） |
| ② 退市 | 键表已不存在的在册行 + 本次要改值的行，先置 `RETIRED`（释放 `usc`，避免部分唯一索引瞬时冲突） |
| ③ 写结果表 | `INSERT ... ON CONFLICT (ftmm_symbol) DO UPDATE`，只写 `insert`/`update` 行；`dt_first_assign` 不覆盖 |
| ④ 回写键表 | `UPDATE futu.finv_futu_secu_key SET usc = ...` 只写变化行 |
| ⑤ 记账 | 一条汇总 + 每条复用一条审计日志 |

> 归档表时间列名为 `dt_snapshot`（`V300167` 起），与结果表的 `dt_retire` / `dt_compute` / `dt_first_assign` 同为 `dt_` 口径。

### 3.5 `GET /Sync/Run`

`runId` 为空时返回最近一次运行。`status` ∈ `RUNNING` / `DONE` / `FAILED`；`logs` 最多保留 500 条。
同一时刻只允许一个运行，重复启动返回 `409`。

## 4. 定时同步与手动执行

`sys.sys_config 20015`（6 段 cron，默认**空串 = 关闭**）。空串或非法表达式时运行器不解析调度。

页面「预演与应用」Tab 顶部的「定时同步」卡片可直接改 cron（或一键关闭），并且有 **「立即执行一次」**：

- 与定时任务**走同一条链路**（`Preview` 取令牌 → `Apply`），不需要等到设定时间；
- 前端点了之后先预演、把差异摆出来，再弹确认框，确认后异步运行。

接口上「立即执行」就是 `POST /Sync/Run`（可带 `preview_token`，也可不带）；`trigger` 字段区分 `manual` / `cron`。

> 上线初期保持关闭：按当前规则首次应用会改写约 **1.68 万行** `futu.finv_futu_secu_key.usc`
> （AU / CA / JP / BMS / SG 等市场的天然唯一码从 `CA.XXX` 这类全码退回裸 `XXX`）。这是跨表主键级变更，
> 需与存量对齐方案一起确认后再由运维填回 cron（迁移 `V300158`）。

## 5. 错误码

| HTTP | code | 场景 |
|------|------|------|
| 400 | 400 | 请求体非法 / 条目校验失败 / 预演令牌过期 |
| 404 | 404 | 运行不存在（`runId` 不匹配） |
| 409 | 409 | 已有运行在执行；条目自然键重复（新增撞上已存在条目，`message` 给出已有 `idn`/状态/说明） |
| 500 | 2006 | 查询失败 / 预演失败（含无法消解的 usc 冲突） |

## 5.5 字段命名约定

统计字段一律用**动词原形**，禁止 `-ed` 形式（需求 11080）：

| 旧名 | 现名 | 含义 |
|------|------|------|
| `added` | `insert` | 新增映射 |
| `changed` | `update` | usc 变化 |
| `removed` | `retire` | 键表已无该证券 → 退市 |
| `reused` | `reuse` | usc 被新证券接管 |
| `unchanged` | `keep` | 无变化 |
| `escalated` | `escalate` | 冲突升级为 ftmm |
| `archived` / `archived_rows` | `archive` / `archive_rows` | 复用归档行数 |
| `key_updated` / `key_matched` / `key_mismatch` | `key_update` / `key_match` / `key_miss` | 键表回写/匹配/未匹配 |
| `last_computed_at` | `last_compute_at` | 最近计算时间 |
| `retired_at` / `computed_at` / `first_assigned_at` | `dt_retire` / `dt_compute` / `dt_first_assign` | 结果表列（`V300165` 去 `-ed`，`V300166` 统一 `dt_` 前缀） |
| `started_at` / `finished_at` / `elapsed_ms` | `start_at` / `finish_at` / `duration_ms` | 运行时间 |
| diff 取值 `added`/`changed`/`removed`/`unchanged` | `insert`/`update`/`retire`/`keep` | 明细分类 |

## 6. 注意事项

- 结果表 `usc_lower` 是**生成列**，任何写入都不得显式赋值；
- 在册（`ACTIVE`）行 `lower(usc)` 唯一由部分唯一索引保证；退市行不占位，允许复用；
- 复用必须先归档再让旧行让位：顺序反了历史数据不可恢复；
- 手动应用会真实回写键表 `usc`，**生产库执行前必须先预演并把明细给业务确认**。
