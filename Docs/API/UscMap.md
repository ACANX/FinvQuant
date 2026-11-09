# USC 映射规则（UscMap）

> 接口（前缀 `/API/V1/Meta/Finv/Quote/UscMap`）：
> - `GET /Status` —— 只读快照（规则/条目数、结果表、键表匹配率、cron）
> - `GET /Rule/List` —— 规则层列表（含条目数）
> - `GET /Item/List` —— 条目分页（`ruleNo` / `market` / `status` / `keyword`）
> - `POST /Item/Save` —— 新增或编辑条目（`item_desc` 必填）
> - `POST /Item/Retire` —— 停用条目（保留行，可再改回 ACTIVE）
> - `GET /Diff/Preview` —— 预演差异（不落库，返回 `preview_token`）
> - `POST /Sync/Run` —— 异步应用（需带预演令牌，返回 `run_id`）
> - `GET /Sync/Run` —— 运行时快照（进度 + 日志，页面 800ms 轮询）
> - `GET /Log/List` —— 审计日志分页
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
     P12 指数别名/特殊指数（LITERAL）→ P14 期货后缀（REWRITE）→ P22 SH 000* 指数（SH4）→ canon_code
② P10 闸门：lower(canon_code) 在键表里计数 = 1 → usc = canon_code，定稿
③ 否则**先分层**（精确 code → 市场+品种正则 → 市场默认 → 兜底 P60）、**同层内**按 ru.priority 取第一条命中条目
     动作：CODE=canon_code / FTMM=ftmm_symbol / LITERAL=usc_literal
     （分层不能省：只按 ru.priority 排会让市场默认 P20=20 压过例外 P40=40/P50=50，实测 660 条例外里 658 条被抢走）
④ 冲突升级：候选与其它行 lower() 撞名 → 改用 ftmm_symbol（结果表 rule_no 记 ESCALATE）
⑤ 升级后仍撞名 → 本次拒绝（HTTP 500 / 运行 FAILED），人工修规则
```

## 3. 接口要点

### 3.1 `GET /Status`

| 字段 | 说明 |
|------|------|
| `rule_count` / `item_count` / `item_active` | 规则数 / 条目数 / 生效条目数 |
| `map_active` / `map_retired` / `map_reused` | 结果表在册 / 已退市 / 复用留痕行数 |
| `key_total` / `key_matched` / `key_mismatch` | 键表行数 / 与结果表 `lower(usc)` 对上的行数 / 对不上的行数 |
| `last_computed_at` / `rule_version` | 最近计算时间 / 规则集版本（规则与条目指纹的前 12 位） |
| `cron_expr` / `cron_next_run` | 系统配置 `20015` 的表达式与下次执行时间（**空 = 自动同步关闭**） |
| `archived_rows` | 复用归档表行数 |

### 3.2 `POST /Item/Save`

请求体为条目字段的 snake_case JSON（`idn=0` 表示新增）。服务端校验：

- `item_desc` 必填（Web 页要显示「为什么这么映射」）；
- `action=LITERAL` 必须有 `usc_literal`；`action=REWRITE` 必须有 `code_regex` + `rewrite_tpl`；
- `code` 与 `code_regex` 只能二选一。

保存成功写一条 `rule_item` 审计日志（INSERT/UPDATE + before/after）。

### 3.3 `GET /Diff/Preview`

不写任何表：算完差异整体回滚，返回四类分类计数、前 200 行明细、冲突清单与 `preview_token`
（= `md5(全部最终映射)`）。首次上线（结果表为空）时 `added` 等于键表行数。

### 3.4 `POST /Sync/Run`

请求体：`{"preview_token":"...","reason":"...","env":"*"}`（`operator` 取登录账号）。
服务端校验令牌：预演之后规则或键表变了 → `400/409` 拒绝（`预演结果已过期`），必须重新预演。

应用顺序（单事务）：

| 步骤 | 动作 |
|------|------|
| ① 归档 | `usc` 被新证券接管时，把下游各表历史行整行快照进 `archive.finv_usc_reuse_snapshot`（`to_regclass` 守卫，缺表跳过） |
| ② 退市 | 键表已不存在的在册行 + 本次要改值的行，先置 `RETIRED`（释放 `usc`，避免部分唯一索引瞬时冲突） |
| ③ 写结果表 | `INSERT ... ON CONFLICT (ftmm_symbol) DO UPDATE`，只写 `added`/`changed` 行；`first_assigned_at` 不覆盖 |
| ④ 回写键表 | `UPDATE futu.finv_futu_secu_key SET usc = ...` 只写变化行 |
| ⑤ 记账 | 一条汇总 + 每条复用一条审计日志 |

### 3.5 `GET /Sync/Run`

`runId` 为空时返回最近一次运行。`status` ∈ `RUNNING` / `DONE` / `FAILED`；`logs` 最多保留 500 条。
同一时刻只允许一个运行，重复启动返回 `409`。

## 4. 定时同步

`sys.sys_config 20015`（6 段 cron，默认**空串 = 关闭**）。空串或非法表达式时运行器不解析调度，只手动可用。

> 上线初期保持关闭：按当前规则首次应用会改写约 **1.68 万行** `futu.finv_futu_secu_key.usc`
> （AU / CA / JP / BMS / SG 等市场的天然唯一码从 `CA.XXX` 这类全码退回裸 `XXX`）。这是跨表主键级变更，
> 需与存量对齐方案一起确认后再由运维填回 cron（迁移 `V300158`）。

## 5. 错误码

| HTTP | code | 场景 |
|------|------|------|
| 400 | 400 | 请求体非法 / 条目校验失败 / 预演令牌过期 |
| 404 | 404 | 运行不存在（`runId` 不匹配） |
| 409 | 409 | 已有运行在执行 |
| 500 | 2006 | 查询失败 / 预演失败（含无法消解的 usc 冲突） |

## 6. 注意事项

- 结果表 `usc_lower` 是**生成列**，任何写入都不得显式赋值；
- 在册（`ACTIVE`）行 `lower(usc)` 唯一由部分唯一索引保证；退市行不占位，允许复用；
- 复用必须先归档再让旧行让位：顺序反了历史数据不可恢复；
- 手动应用会真实回写键表 `usc`，**生产库执行前必须先预演并把明细给业务确认**。
