# 证券参数同步（SecuParamSync）

> 菜单 key：`quote-realtime-secu-param-sync`
> 路由：`/Meta/Finv/Quote/Realtime/SecuParamSync`
> 所属：导航「行情 → 实时行情 → 证券参数同步」（紧跟「缺省汇率」之后）
> 视图：`Web/src/views/Meta/Finv/Quote/Realtime/SecuParamSyncView.vue`
> 关联需求：11042（菜单与页面）、11043（同步实现）、11044（源表名可配 20014）、11045（拆两个 Tab）、
> 11046（异步运行 + 实时日志）、11047（Tab 2 口径确认）、11050（Tab 2 指标同步实现）、
> 11051（`flag_enable` 规则）、11052（Tab 1 不清空 Tab 2 补的 `url`）、
> 11054（Tab 3 上游 usc 回填）、11055（多条 `usc` 取值规则）
> 关联接口：[`API/Cdeu/SecuParamSync.md`](../../../API/Cdeu/SecuParamSync.md)
> 关联数据表：目标 `futu.finv_futu_secu_key`（V300137 建表 / V300139 补 `stock_id`+`usc` / V300141 改名）；
> 源 Supabase `public.finv_quote_secu_key`（表名可配，默认与上游同名）

## 1. 用途

把上游 Supabase 的**证券基础参数**与（后续）**证券指标数据**分批增量同步到本地库。

页面为**两个 Tab**（需求 11045）：

| Tab | 状态 | 内容 |
|-----|------|------|
| **FT证券基础参数同步** | 已实现 | Supabase 证券键表（默认 `public.finv_quote_secu_key`）→ 本地 `futu.finv_futu_secu_key` |
| **证券指标数据同步** | 已实现（需求 11050） | 上游 `public.finv_quote_secu_meta` → 指标表 + 采集属性表 + 证券键表（一条记录写三张表） |
| **上游 usc 回填** | 已实现（需求 11054） | 方向相反：本地 `futu.finv_futu_quote_collect.usc` 按 `stock_id` 关联写回上游 `public.finv_quote_secu_meta.usc` |

- 只由本页**按钮手动触发**，没有定时任务；
- 单向只读上游 → 写本地：不删除本地行，不清空本地专有列 `stock_id` / `usc`；
- 连接配置（`PROJECT_ID|APIKEY`）在系统配置 **20013** 里维护，本页只展示状态，不回显完整 key；
- 源表名存系统配置 **20014**，默认 `finv_quote_secu_key`，在本页可直接改。

## 2. 页面结构

### 2.1 头部

| 控件 | 说明 |
|------|------|
| 刷新状态 | `GET .../SecuKey/Status` 重新读配置与目标表行数（不访问 Supabase） |
| Tab 切换 | 「FT证券基础参数同步」 / 「证券指标数据同步」 |

### 2.2 Tab 1：FT证券基础参数同步

#### 2.2.1 连接与目标表（第一张卡片）

| 区块 | 展示 |
|------|------|
| Supabase 连接（20013） | `已配置`（success chip）/ `未配置`（warning chip）；已配置时显示项目 ID 与脱敏 key 前缀（`APIKEY sb_publi…（已脱敏）`）；未配置时显示 `config_error` 原文 |
| 本地目标表 | `futu.finv_futu_secu_key` + `共 N 行`（tonal chip）+ 最近变更时间（`max(gmt_update)`） |

#### 2.2.2 同步参数（第二张卡片）

| 控件 | 说明 |
|------|------|
| Supabase 源表名 | 文本输入，默认 `finv_quote_secu_key`；可写 `schema.table` 两段，只允许字母/数字/下划线；与已生效值不同时才允许保存 |
| 每批行数 | 下拉 `500 / 1000 / 2000 / 5000`，默认 1000 |
| 保存源表名 | `POST .../SecuKey/Config` 写系统配置 20014，成功后 toast 与状态刷新 |
| 立即同步 | `POST .../SecuKey/Sync/Run` **异步启动**（立刻返回 `run_id`），随后页面每 800 ms 轮询 `GET .../SecuKey/Sync/Run`；20013 未配置时禁用；运行中按钮显示「同步中…」并禁用 |

#### 2.2.3 同步进度与日志（第三张卡片，需求 11046）

同步启动后立即出现，**边跑边更新**（不必等结束）：

| 元素 | 说明 |
|------|------|
| 状态 chip | `运行中`（info）/ `已完成`（success）/ `失败`（error） |
| 累计 chip | `拉取 / 新增 / 更新 / 未变 / 跳过（有才显示）/ status 归一（有才显示）` + `N 批 · 耗时` |
| 进度条 | `v-progress-linear`，按「已拉取行数 / 源表总行数」；总行数未知时为首批前的 indeterminate |
| 进度说明 | `进度 61.5%（45000 / 73212 行）· 源表 xxx · run_id sk-…` |
| **日志区** | 独立滚动区域（`max-height: 320px`、等宽字体、深色底），每条一行 `HH:mm:ss 文本`；**自动滚到底部**；运行中末尾显示「等待下一批…」；`ERROR` 行标红 |

日志内容（每批一行，含本批差值与本批耗时）：

```
22:25:24 同步任务已启动
22:25:25 第 1 批：offset=0，拉取 5000，新增 0，更新 0，未变 5000，跳过 0（本批 807 ms）
22:25:26 第 2 批：offset=5000，拉取 5000，新增 0，更新 0，未变 5000，跳过 0（本批 428 ms）
...
22:25:32 第 15 批：offset=70000，拉取 3212，新增 0，更新 0，未变 3212，跳过 0（本批 316 ms）
22:25:32 同步完成：源表 finv_quote_secu_key 共 73212 行，15 批，新增 0，更新 0，未变 73212，跳过 0，耗时 7854 ms
```

失败时最后一行是 `ERROR 同步失败：<可读原因>`（如上游 `PGRST205`、鉴权失败、20013 未配置），
同时弹 error toast，`status` chip 变红。

### 2.3 Tab 2：证券指标数据同步（需求 11050）

#### 2.3.1 连接与目标表（第一张卡片）

| 区块 | 展示 |
|------|------|
| Supabase 连接（20013） | 同 Tab 1：`已配置` / `未配置` + 项目 ID + 脱敏 key 前缀 |
| 上游源表（固定） | `public.finv_quote_secu_meta`（不可改） |
| 本地目标表（一次同步写三张） | 三个 chip：`cdeu.finv_quote_secu_indicator` / `futu.finv_futu_quote_collect` / `futu.finv_futu_secu_key`，各带当前行数 |

#### 2.3.2 同步参数（第二张卡片）

| 控件 | 说明 |
|------|------|
| 每批行数 | 下拉 `500 / 1000 / 2000 / 5000`，默认 1000 |
| 立即同步 | `POST .../SecuKey/Indicator/Sync/Run` 异步启动；20013 未配置或运行中时禁用 |

#### 2.3.3 同步进度与日志（第三张卡片）

与 Tab 1 同款（状态 chip / 累计 chip / 进度条 / 独立滚动日志区 + 自动滚底 + `ERROR` 标红 + 「等待下一批…」），
只是累计 chip 换成三张表的口径：

`拉取 · 指标 新增/更新/未变 · 采集属性 新增/更新/跳过 · 键表补全 · N 批 · 耗时`

日志形态（每批一行，含本批差值）：

```
22:55:29 指标同步任务已启动
22:55:35 第 1 批：offset=0，拉取 5000，指标 新增 0 / 更新 1 / 未变 4999，采集属性 新增 0 / 更新 2 / 跳过 2，键表补全 0（本批 6075 ms）
22:55:49 指标同步完成：源表 public.finv_quote_secu_meta 共 10910 行，3 批；……；键表补全 10910；耗时 14275 ms
```

#### 2.3.4 字段口径（页面「说明」里同步展示）

| 目标表 | 写入内容 |
|--------|----------|
| `cdeu.finv_quote_secu_indicator` | 131 列同名同步（`dt_update` 由触发器维护，不写入不比對） |
| `futu.finv_futu_quote_collect` | 身份字段同名同步（`stock_id`/`market_type`/`market_code`/`instrument_type`/`sub_instrument_type`/`market_label`/`instrument`），`futu_symbol` 由 `ftmm_symbol` 去市场前缀派生 |
| `futu.finv_futu_secu_key` | 只补 `url`（源 `secu_url`）与 `stock_id`，按 `ftmm_symbol` 对齐 |

- 主键均为 `usc`；源 `usc` 为空时用 `ftmm_symbol` 占位（上游 3,673 行）；
- **`flag_enable`（需求 11051）**：占位 `usc` → `'0'`（禁用采集），真实 `usc` → `'1'`；插入与更新都会按此覆盖；
- 采集属性表唯一键 `(market_code, futu_symbol)` 撞车（被别的 `usc` 占用）时跳过该行并计入「跳过」。

### 2.4 Tab 3：上游 usc 回填（需求 11054）

#### 2.4.1 连接与源表（第一张卡片）

| 区块 | 展示 |
|------|------|
| Supabase 连接（20013） | 同 Tab 1/2：`已配置` / `未配置` + 项目 ID + 脱敏 key 前缀 |
| 本地源表（`usc` 来源） | `futu.finv_futu_quote_collect`、`共 N 行`、`待处理 N 个 stock_id` |
| 上游被回填的表与列 | chip `public.finv_quote_secu_meta.usc`、`关联键 stock_id`、`占位 usc N 行` |

#### 2.4.2 回填参数（第二张卡片）

| 控件 | 说明 |
|------|------|
| 每批 `stock_id` 数 | 下拉 `200 / 500 / 1000 / 2000`，默认 500（逐行 PATCH，批量大只是让单批更粗） |
| 立即回填 | `POST .../UscBackfill/Sync/Run` 异步启动；20013 未配置或运行中时禁用 |

#### 2.4.3 回填进度与日志（第三张卡片）

与 Tab 1/2 同款（状态 chip / 累计 chip / 进度条 / 独立滚动日志区 + 自动滚底 + `ERROR` 标红 + 「等待下一批…」），
累计 chip 换成回填口径：

`处理 a / b 个 stock_id · 填入 · 覆盖 · 未变 · 歧义 · 无对应 · N 批 · 耗时`

日志形态（首次全量实测，49 批）：

```
23:17:51 回填任务已启动：本地 futu.finv_futu_quote_collect → 上游 public.finv_quote_secu_meta.usc（按 stock_id 关联）
23:17:56 第 1 批：offset=0，处理 500 个 stock_id，写入 106（填入 105 / 覆盖 1），未变 240，歧义 0，无对应 154（本批 4321 ms）
23:19:49 回填完成：futu.finv_futu_quote_collect 共 24418 行（24348 个 stock_id），49 批；写入上游 3676（填入 3673 / 覆盖 3），未变 7234，歧义 0，无对应 13438，耗时 118669 ms
```

#### 2.4.4 口径（页面「说明」里同步展示）

- 关联键 **`stock_id`**（不是 `usc`，也不是「只更新上游 `usc IS NULL` 的行」）；
- 本地同一 `stock_id` 有多条 `usc`（实测 70 组）时取**等于上游 `ftmm_symbol`** 的那条，都不等则计入「歧义」跳过；
- 本地有、上游没有的 `stock_id`（13,438 个，期货/加密 FTUS 等 futu 独有）计入「无对应」；
- 上游 `usc` 已等于目标值 → 「未变」，**不发请求**（复跑幂等）；
- 本 Tab **写入线上 Supabase**：逐行 PATCH，每行校验受影响行数 = 1，RLS 拒绝直接报错。

### 2.5 说明卡片（页面最底部唯一「说明」卡片，ExplainCardSpec）

四段：**表口径**（FT证券基础参数同步）、**增量口径与进度**（缺则插入、变了才更新、未变不改写 + 轮询/实时日志）、
**配置与安全**（20013 格式与脱敏、20014 需保存才生效、`status` 非法归一为 `PENDING`）、
**证券指标数据同步**（来源固定 `public.finv_quote_secu_meta`，一次写三张表；`flag_enable` 按 `usc` 是否占位推导：
占位 `'0'`、真实 `'1'`，每次同步覆盖；Tab 1 不再清空 Tab 2 补的 `url`）。

**上游 usc 回填**（方向相反：本地 → 上游，按 `stock_id` 关联；会写入线上 Supabase；
上游 `usc` 已一致时不发请求；本地同一 `stock_id` 多条 `usc` 取等于上游 `ftmm_symbol` 的那条）。

## 3. 交互细节

| 场景 | 表现 |
|------|------|
| 页面打开 | 自动 `GET Status`；失败弹 error toast 并在页面显示 error alert |
| 20013 未配置 | 状态显示「未配置」；点「立即同步」弹 warning toast 并直接返回（不发请求） |
| 源表名未改动 | 「保存源表名」按钮禁用（避免无意义写入） |
| 同步启动成功 | 立刻出现「同步进度与日志」卡片（状态「运行中」+ 首条日志），日志逐批追加 |
| 同步完成 | toast（新增/更新/未变）+ 状态 chip 变「已完成」+ 自动刷新「连接与目标表」 |
| 同步失败 | 状态 chip 变「失败」+ error 日志行 + error toast + alert 展示原因 |
| 运行中重复点击 | 按钮禁用；若从别处再次启动，接口返回 409 并在页面提示「已有同步任务在运行」 |
| 同步进行中 | 「保存源表名」禁用，避免与进行中的同步口径不一致 |
| 离开页面 | 轮询定时器在 `onBeforeUnmount` 清理（后台任务继续跑，回来重新点同步即可；运行状态存在服务端内存） |

## 4. 与其他页面的关系

- 「缺省汇率」之后新增本菜单（需求 11042 指定位置）；
- Tab 1 只写 `futu.finv_futu_secu_key`，不改 `cdeu.*` 快照表，也不触发任何采集；
- 后续按 `ftmm_symbol` 采集行情时以本表为键来源（`status`/`priority`/`url` 供采集侧使用）；
- Tab 2 落地后同样走「异步启动 + 轮询 + 实时日志」，与 Tab 1 交互一致。
