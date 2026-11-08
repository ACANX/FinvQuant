# 证券参数同步（SecuParamSync）

> 接口：
> - `GET /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Status` —— 只读快照（连接配置状态 + 源表名 + 目标表行数）
> - `POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Config` —— 保存 Supabase 源表名（系统配置 20014）
> - `POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Sync` —— 手动触发同步（同步执行，返回逐批日志；脚本用）
> - `POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run` —— 异步启动同步（返回 `run_id`，需求 11046）
> - `GET /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run` —— 运行时快照（`progress` + 逐批实时日志，页面轮询）
>
> Tab 2「证券指标数据同步」（需求 11050）：
> - `GET /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Status` —— 三张目标表行数
> - `POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Sync/Run` —— 异步启动
> - `GET /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Sync/Run` —— 运行时快照（进度 + 逐批实时日志）
> 需求：11042（菜单与页面）、11043（同步实现）、11044（源表名可配 20014）、11045（页面拆两个 Tab）、
> 11046（异步运行 + 实时日志）、11050（Tab 2 指标同步）、11051（`flag_enable` 规则）、
> 11052（Tab 1 不清空 Tab 2 补的 `url`）；连接配置 20013（需求 11041）
> 权限：需登录（内部接口，不对外暴露 OpenAPI）
> 关联表：目标 `futu.finv_futu_secu_key`（迁移 `V300137` 建表、`V300139` 补 `stock_id`/`usc`、`V300141` 改名）；
> 源 Supabase `public.finv_quote_secu_key`（表名可配，默认与上游同名）
> 关联菜单：[`Menu/Quote/Realtime/SecuParamSync.md`](../../../Menu/Quote/Realtime/SecuParamSync.md)（路由 `/Meta/Finv/Quote/Realtime/SecuParamSync`）
> 契约：[`SecuParamSync.yml`](SecuParamSync.yml)

## 1. 用途

把上游 Supabase 上的**证券键表**（`instrument + market + code → ftmm_symbol` 的键映射与采集状态）批量同步到本地
`futu.finv_futu_secu_key`，作为后续富途行情采集的键来源。

- 触发方式：只由前端「证券参数同步」页的**按钮**手动触发，无定时任务；
- 数据方向：单向只读上游 → 写本地，**不删除**本地行；
- 分批：按 `ftmm_symbol` 升序分页拉取（PostgREST `limit`/`offset`），每批一次 upsert；
- 增量：主键不存在则插入；已存在且字段有差异才更新；完全一致的行不改写；
- 凭据：Supabase `{PROJECT_ID}|{APIKEY}` 存系统配置 **20013**，本接口只回显项目 ID 与脱敏 key 前缀。

## 2. 字段映射

| 源列（Supabase） | 目标列（`futu.finv_futu_secu_key`） | 说明 |
|---|---|---|
| `ftmm_symbol` | `ftmm_symbol` | 主键 / 唯一键（`US.AAPL`）；空 → 整行跳过 |
| `instrument` | `instrument` | 证券品种（`stock`/`etf`/`index`/`crypto`/`futures`…）；空 → 跳过 |
| `market` | `market` | 市场（`US`/`HK`/`SZ`/`SH`/`BJ`/`AU`/`JP`…）；空 → 跳过 |
| `code` | `code` | 市场内代码；空 → 跳过 |
| `enable` | `flag_enable` | `NULL` 或非 0 → 1；`0` → 0 |
| `priority` | `priority` | `NULL` → 10；越界收敛到 1~1000000 |
| `status` | `status` | 空或非法 → `PENDING`（计入 `status_reset`）；合法值 `PENDING/QUEUED/DONE/FAILED/SKIP` |
| `url` | `url` | 空白按 `NULL` 落库 |
| — | `stock_id`、`usc` | **不同步**（源表无此列），同步也不清空 |

源表其余列（`note`、`count_collect`、`gmt_last_collect`、`last_status`、`last_error`、`gmt_create`、`gmt_update`）
不参与同步；目标的 `gmt_create`/`gmt_update` 由本地默认值与 `trg_finv_futu_secu_key_gmt_update` 触发器维护。

## 3. 同步状态

`GET /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Status`

只读本地，不访问 Supabase（页面打开即可用）；源表真实行数在同步结果的 `rows_total` 里给出。

### 3.1 响应 `data`

| 字段 | 类型 | 说明 |
|------|------|------|
| `configured` | bool | 系统配置 20013 是否已配置且格式合法 |
| `project_id` | string | Supabase 项目 ID（可展示的一半） |
| `api_key_masked` | string | API Key 脱敏前缀（前 8 位 + `…`；未配置为空） |
| `config_error` | string | 20013 未配置 / 格式非法的原因（合法时缺省） |
| `source_table` | string | 生效的源表名（20014 归一化结果，永不为空） |
| `source_table_raw` | string | 20014 原始取值（可能非法，便于诊断） |
| `target_table` | string | 目标表，固定 `futu.finv_futu_secu_key` |
| `target_rows` | int | 目标表当前行数 |
| `target_last_update` | string | 目标表 `max(gmt_update)`（RFC3339；空表时缺省） |
| `batch_size_default` | int | 默认批量（1000） |

### 3.2 示例

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "configured": true,
    "project_id": "abcdefghijklmnop",
    "api_key_masked": "sb_publi…",
    "source_table": "finv_quote_secu_key",
    "source_table_raw": "finv_quote_secu_key",
    "target_table": "futu.finv_futu_secu_key",
    "target_rows": 73212,
    "target_last_update": "2026-11-08T22:12:04Z",
    "batch_size_default": 1000
  }
}
```

## 4. 保存源表名

`POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Config`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `source_table` | string | 否 | 源表名；留空或非法格式按默认 `finv_quote_secu_key` 处理；允许 `schema.table` 两段 |

校验规则（`syscfg.NormalizeSupabaseTableName`）：最多两段、每段 `[A-Za-z_][A-Za-z0-9_]*`；
表名只用于拼 PostgREST URL（schema 走 `Accept-Profile` 头），校验可防 URL 注入。

- 成功：`200`，`data.source_table` 为归一化后的表名（写系统配置 20014）；
- 非法：`400`，`message` 说明具体原因。

> 只允许改源表名；**20013（含 API Key）不在本接口内暴露**，仍在系统配置里维护。

## 5. 手动同步

`POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Sync`

### 5.1 请求

两个字段都可省略，允许空请求体 `{}`。

| 字段 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|------|------|
| `source_table` | string | 否 | 取系统配置 20014 | 本次使用的源表名（不写库，仅覆盖本次） |
| `batch_size` | int | 否 | 1000 | 每批行数，收敛到 100~5000 |

> 前端超时需放宽：7 万行量级实测约 13 s（`batch_size=5000`、15 批）；
> 默认 `batch_size=1000` 时约 70+ 批、20 s 左右。页面用 15 分钟超时兜底。

### 5.2 响应 `data`

| 字段 | 类型 | 说明 |
|------|------|------|
| `source_table` | string | 实际使用的源表名（归一化后） |
| `rows_total` | int | 源表总行数（来自 `Content-Range`） |
| `batches` | int | 实际批次数 |
| `rows_receive` | int | 本次拉取的行数 |
| `insert` | int | **实际新增**的行 |
| `update` | int | **实际更新**的行（字段确有变化） |
| `unchanged` | int | 与本地一致、未改写的行 |
| `skipped` | int | 关键字段（`ftmm_symbol`/`market`/`code`/`instrument`）为空而跳过的行 |
| `status_reset` | int | `status` 非法被归一为 `PENDING` 的源行数 |
| `elapsed_ms` | int | 总耗时（毫秒） |
| `logs[]` | array | 逐批日志：`batch`/`offset`/`fetched`/`insert`/`update`/`skipped`/`elapsed_ms` |

### 5.3 错误

| 场景 | HTTP | `code` | `message` |
|------|------|--------|-----------|
| 20013 未配置 / 格式非法 | 500 | 2006 | `证券参数同步失败: Supabase 连接不可用: 系统配置 20013（…）未配置` |
| 源表名非法 | 500 | 2006 | `证券参数同步失败: 源表名 "…" 非法：…` |
| 源表不存在（HTTP 404 / PGRST205） | 500 | 2006 | `拉取 Supabase 源表失败（offset=0, limit=N）: Supabase 源表不存在或不可读（HTTP 404）…` |
| 鉴权失败（401/403） | 500 | 2006 | `Supabase 鉴权失败（HTTP 401），请检查 20013 的 APIKEY…` |
| 网络可重试错误（超时 / 429 / 5xx） | 500 | 2006 | 自动重试 3 次（退避 500ms、1s）后返回最后一次错误 |

失败**不回滚已写入的批次**：增量写入可重入，重跑只补齐剩余部分。

### 5.4 异步运行（页面实际使用，需求 11046）

页面不再用上面的同步接口（要等全部批次结束才有响应），而是「异步启动 + 轮询」：
启动拿到 `run_id` 与首条日志，之后每 800 ms 拉一次运行快照，**日志区逐批追加**，边跑边看进度。

#### 启动：`POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run`

请求体同 §5.1（`source_table` / `batch_size` 均可省略，允许空体 `{}`）。

| 场景 | HTTP | 说明 |
|------|------|------|
| 启动成功 | **202** | `data` 为启动瞬间的快照：`run_id`、`status=RUNNING`、`logs[0]="同步任务已启动"` |
| 已有运行在执行 | **409** | `message` 带正在运行的 `run_id`；同一时刻只允许一个运行（不排队、不覆盖） |
| 请求体非法 | 400 | `code=400` |

> 运行状态只存内存：进程重启即丢失，重新点按钮即可。整体超时 60 分钟。

#### 轮询：`GET /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run?runId=<id>`

`runId` 省略时返回最近一次运行；`runId` 不匹配或从未运行过返回 **404**（不返回别的运行，避免张冠李戴）。

`data` 字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `run_id` | string | 运行 ID（`sk-<yyyyMMdd-HHmmss.微秒>`） |
| `status` | string | `RUNNING` / `DONE` / `FAILED` |
| `progress` | number | 0~100，按「已拉取行数 / 源表总行数」计算（总行数未知时为 0） |
| `rows_total` / `rows_receive` | int | 源表总行数（首批后才知道）/ 累计拉取 |
| `insert` / `update` / `unchanged` / `skipped` / `status_reset` | int | 累计计数，口径同 §5.2 |
| `batches` | int | 已完成批次数 |
| `batch_size` | int | 本次使用的批量 |
| `started_at` / `finished_at` | string | RFC3339 |
| `elapsed_ms` | int | 运行中为「至今耗时」，结束后为总耗时 |
| `error` | string | `FAILED` 时的可读原因（与同步接口 500 的 `message` 同源） |
| `logs[]` | array | 实时日志：`seq`（自增）、`time`（RFC3339）、`level`（`INFO`/`ERROR`）、`text`、`batch`、`offset` |

日志形态（一次 15 批运行的实测输出）：

```
1  INFO 同步任务已启动
2  INFO 第 1 批：offset=0，拉取 5000，新增 0，更新 0，未变 5000，跳过 0（本批 807 ms）
3  INFO 第 2 批：offset=5000，拉取 5000，新增 0，更新 0，未变 5000，跳过 0（本批 428 ms）
...
16 INFO 第 15 批：offset=70000，拉取 3212，新增 0，更新 0，未变 3212，跳过 0（本批 316 ms）
17 INFO 同步完成：源表 finv_quote_secu_key 共 73212 行，15 批，新增 0，更新 0，未变 73212，跳过 0，耗时 7854 ms
```

- 每批一行，含**本批差值**（新增/更新/未变/跳过）与本批耗时；失败时最后一行是 `ERROR 同步失败：<原因>`；
- 日志上限 2000 行（按批一行，7 万行/1000 批约 74 行，余量充足），超限保留最近 2000 条。

## 6. 证券指标数据同步（Tab 2，需求 11050）

来源固定为上游 `public.finv_quote_secu_meta`（10,910 行、145 列），**一条上游记录一次写三张本地表**。

### 6.1 字段分配

| 目标表 | 写入内容 |
|--------|----------|
| `cdeu.finv_quote_secu_indicator` | **131 列同名同步**（列结构与上游同构，见 `DbSchemaSpec`）；`dt_update` 由触发器维护，**不写入、不参与比对** |
| `futu.finv_futu_quote_collect` | 身份字段同名同步：`stock_id`、`market_type`、`market_code`、`instrument_type`、`sub_instrument_type`、`market_label`、`instrument`；`futu_symbol` 由 `ftmm_symbol` 去掉市场前缀派生（`US.AAPL → AAPL`）；`flag_enable` 见 §6.2；不写 `req_section` / `remark` |
| `futu.finv_futu_secu_key` | 只补 `url`（源 `secu_url`）与 `stock_id`，按 `ftmm_symbol` 对齐；`url` / `stock_id` 无变化时不改写 |

- **主键口径**：指标表与采集属性表主键都是 `usc`；源记录 `usc` 为空（上游 3,673 行）时用 `ftmm_symbol` 占位。
- **不落库的上游列**：`name_en`、`flag_option`、`flag_futures`、`flag_plate`、`flag_etf_theme` 等本地无对应列。
- **唯一键防撞**：`futu.finv_futu_quote_collect` 有唯一约束 `(market_code, futu_symbol)`；若该组合已被**别的 `usc`** 占用，
  该行跳过并计入 `qc_skip`（不报错、不覆盖别人的 `usc`）。

### 6.2 `flag_enable` 规则（需求 11051）

`flag_enable` 由同步按「`usc` 是否占位」推导，**插入与更新都覆盖**：

| 情况 | `flag_enable` |
|------|---------------|
| `usc` 用的就是 `ftmm_symbol`（源 `usc` 为 NULL/空，占位） | `'0'`（禁用采集） |
| `usc` 为上游真实值 | `'1'`（启用采集） |

> 即 `flag_enable` 是同步派生列，不视为人工开关：人工改成其它值会在下次同步被改回。
> 若需要长期人工禁用某证券采集，请走其它机制（不要依赖该列），这一点在页面「说明」里也有写。

### 6.3 接口

#### `GET /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Status`

只读本地，`data` 字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `configured` / `project_id` / `api_key_masked` / `config_error` | — | 同 §3（20013 状态，key 脱敏） |
| `source_table` | string | 固定 `public.finv_quote_secu_meta` |
| `targets[]` | array | 三张目标表的 `{table, rows}`：指标表 / 采集属性表 / 证券键表 |
| `batch_size_default` | int | 默认批量 1000 |

#### `POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Sync/Run`

请求体：`{"batch_size": 1000}`（可省略，收敛到 100~5000）。

- `202`：`data` 为启动快照（`run_id` / `status=RUNNING` / `logs[0]`）；
- `409`：已有指标同步在运行（不排队、不覆盖）；
- 400：请求体非法。

#### `GET /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Sync/Run?runId=`

`data` 字段：`run_id`、`kind="indicator"`、`status`、`progress`（0~100）、`rows_total`、`rows_receive`、
`ind_insert` / `ind_update` / `ind_unchanged`、`qc_insert` / `qc_update` / `qc_skip`、`key_update`、
`batches`、`elapsed_ms`、`error`、`logs[]`；`runId` 省略取最近一次，不匹配返回 404。

日志形态（一次 3 批运行的实测输出）：

```
1  INFO 指标同步任务已启动
2  INFO 第 1 批：offset=0，拉取 5000，指标 新增 0 / 更新 1 / 未变 4999，采集属性 新增 0 / 更新 2 / 跳过 2，键表补全 0（本批 6075 ms）
...
6  INFO 指标同步完成：源表 public.finv_quote_secu_meta 共 10910 行，3 批；指标表 新增 0 / 更新 0 / 未变 10910；
       采集属性表 新增 0 / 更新 0 / 跳过 419；键表补全 10910；耗时 14275 ms
```

### 6.4 验收实测（2026-11-09）

| 场景 | 结果 |
|------|------|
| 首次全量同步（`batch_size=5000`） | `rows_total=10910`、3 批、指标表 `insert=10910`、采集属性表 `insert=3257`、`qc_skip=419`、键表 `key_update=10910` |
| 复跑 | `ind_insert=0 / ind_update=0 / ind_unchanged=10910`、`qc_* = 0`、`key_update=0` |
| 占位 `usc` 统计 | 指标表 `usc = ftmm_symbol` 共 3,673 行；采集属性表占位行 `flag_enable` 全为 `'0'`（反例 0 行） |
| `flag_enable` 规则验证 | 把占位行 `CC.AAVE` 改为 `'1'`、真实 usc 行 `00010` 改为 `'0'` → 同步后 `qc_update=2`，两者分别恢复 `'0'` / `'1'` |
| 增量更新验证 | 把 `00010` 的 `price` 改为 999 → 同步后 `ind_update=1`，`price` 恢复为上游值 |
| 键表补全 | `url` 与 `stock_id` 各 10,910 行写回（`futu.finv_futu_secu_key` 共 73,212 行） |
| Tab 1 与 Tab 2 共存（需求 11052） | Tab 2 补完 `url` 后再跑 Tab 1：`update=0`、`unchanged=73212`，`url` 未被清空（源键表 `url` 全为 NULL，Tab 1 改为 `COALESCE(EXCLUDED.url, t.url)`） |

## 7. 实现要点

- 代码：`internal/meta/finv/quote/secukey/`（`client.go` 上游读取、`store.go` 批量写入、`service.go` 编排与归一化、
  `runner.go` 异步运行与实时日志），处理器 `internal/api/handler/cdeu_secu_key.go`；
- 上游 URL：`https://<PROJECT_ID>.supabase.co/rest/v1/<表名>?select=…&order=ftmm_symbol.asc&limit=&offset=`，
  头部 `apikey` + `Authorization: Bearer <key>` + `Prefer: count=exact` + `Accept-Profile: <schema>`；
- **schema 只能走 `Accept-Profile` 头**：写成 `/rest/v1/public.<表>` 会被 PostgREST 当成
  `public.public.<表>` 并返回 `PGRST205`（实测踩过）；
- 同批内 `ftmm_symbol` 重复时保留最后一条（PostgreSQL 不允许同批同键写入两次）；
- 增量判据：`ON CONFLICT (ftmm_symbol) DO UPDATE … WHERE (目标列) IS DISTINCT FROM (EXCLUDED 列)`，
  未变化的行不产生 WAL、不触发 `gmt_update`；
- 日志前缀：Tab 1 `[cdeu][secukey]`、Tab 2 `[cdeu][indicator]`；
  `docker logs <容器> 2>&1 | grep -E '\[cdeu\]\[(secukey|indicator)\]'`；
- Tab 2 的 SQL 由建表列序生成（`internal/meta/finv/quote/indicator/sql.go`）：`cdeu.finv_quote_secu_indicator`
  增删列/改列名后必须重新生成，否则插入与比对会漏列。

## 7. 验收实测（2026-11-09）

| 场景 | 结果 |
|------|------|
| 首次同步（`batch_size=5000`） | `rows_total=73212`、15 批、`insert=73212`、`elapsed_ms≈13148` |
| 幂等复跑 | `insert=0`、`update=0`、`unchanged=73212`、`max(gmt_update)` 不变 |
| 本地造漂移（改 `priority`）后再同步 | `update=1`、`unchanged=73211`，`priority` 复原为上游值，本地 `stock_id`/`usc` 保留 |
| 源表名保存 `public.finv_quote_secu_key` | `code=0`，随后同步仍 `unchanged=73212`（`Accept-Profile` 生效） |
| 源表名保存 `finv;drop table x` | HTTP 400，提示只允许字母/数字/下划线 |
| 20013 清空后 | `Status.configured=false` + 可读 `config_error`；`Sync` → HTTP 500 `Supabase 连接不可用` |
| 不存在的源表 | HTTP 500 + `PGRST205` 原文与排查提示 |
| 异步启动 `POST .../Sync/Run` | HTTP 202 + `run_id` + `logs[0]`；运行中再次启动 → HTTP 409（带正在运行的 run_id） |
| 异步轮询（`batch_size=5000`） | `progress` 依次 13.66% → 27.32% → … → 100%，`batches` 与 `logs` 同步增长（2/4/5/7/9/11 → 15） |
| 异步完成 | `status=DONE`、17 行日志（启动 + 15 批 + 完成）、`unchanged=73212`、`elapsed_ms≈7854` |
| 异步失败 | 不存在的源表 → `status=FAILED` + `error` 与最后一行 `ERROR` 日志 |
| `runId` 不存在 | HTTP 404「没有找到同步运行记录」 |
