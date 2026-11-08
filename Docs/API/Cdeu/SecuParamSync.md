# 证券参数同步（SecuParamSync）

> 接口：
> - `GET /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Status` —— 只读快照（连接配置状态 + 源表名 + 目标表行数）
> - `POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Config` —— 保存 Supabase 源表名（系统配置 20014）
> - `POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Sync` —— 手动触发同步（同步执行，返回逐批日志）
> 需求：11042（菜单与页面）、11043（同步实现）、11044（源表名可配 20014）；连接配置 20013（需求 11041）
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

## 6. 实现要点

- 代码：`internal/meta/finv/quote/secukey/`（`client.go` 上游读取、`store.go` 批量写入、`service.go` 编排与归一化），
  处理器 `internal/api/handler/cdeu_secu_key.go`；
- 上游 URL：`https://<PROJECT_ID>.supabase.co/rest/v1/<表名>?select=…&order=ftmm_symbol.asc&limit=&offset=`，
  头部 `apikey` + `Authorization: Bearer <key>` + `Prefer: count=exact` + `Accept-Profile: <schema>`；
- **schema 只能走 `Accept-Profile` 头**：写成 `/rest/v1/public.<表>` 会被 PostgREST 当成
  `public.public.<表>` 并返回 `PGRST205`（实测踩过）；
- 同批内 `ftmm_symbol` 重复时保留最后一条（PostgreSQL 不允许同批同键写入两次）；
- 增量判据：`ON CONFLICT (ftmm_symbol) DO UPDATE … WHERE (目标列) IS DISTINCT FROM (EXCLUDED 列)`，
  未变化的行不产生 WAL、不触发 `gmt_update`；
- 日志前缀 `[cdeu][secukey]`：`docker logs <容器> 2>&1 | grep '\[cdeu\]\[secukey\]'`。

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
