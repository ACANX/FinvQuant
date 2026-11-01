# BRS 大表增量（分片）备份接口

> 所属：FinvQuant 服务端 · 存放：`Docs/API/BRS/`
> 适用需求：10905（大表增量备份：按分片维度切分导出，只补缺失分片）
> 关联：小表全量备份见 [`Run.md`](Run.md) / [`BackupTable.md`](BackupTable.md)；对象存储见 [`OSSConfig.md`](OSSConfig.md)
> 前端页面：`Web/src/views/Meta/BRS/Backup/Finv/BrsSliceView.vue`（菜单「备份与恢复 → 备份管理 → 大表增量备份」）

## 0. 定位

| 项 | 小表全量备份（`Run.md`） | 大表分片备份（本文） |
|---|---|---|
| 登记 | `brs.brs_bak_rdb_table` | `brs.brs_bak_rdb_table_incr`（增量，`V300088` 建表、`V300090` 改名） |
| 切分 | 整表 1 个文件 | 按分片维度切分成 N 个文件（日 / 周 / 月 / 列值 / 自定义条件） |
| 方式 | `pg_dump -Fc`（custom 格式，`pg_restore` 可选择性恢复） | 按行读取后写 **MVSV-2**（与 `qiom export` 同一 writer），gzip 压缩 |
| 增量 | 每轮整表重做（覆盖同名文件） | 分片键从库枚举，已有产物跳过，只补缺失 + 末尾 `redo_slices` 个 |
| 本地 | `<备份目录>/<执行日>/<库>.<schema>.<表>.dump` | `<备份目录>/Slice/<库>.<schema>.<表>/<分片键>.mvsv[.gz]` |
| 远端 | `<prefix>/<env>/<执行日>/<库>.<schema>.<表>.dump` | `<prefix>/<env>/<库>.<schema>.<表>/<分片键>.mvsv[.gz]` |
| 调度 | 39001 cron | 本期只支持手动触发 |

> 为什么不用 `pg_dump`：`pg_dump` **没有行级过滤选项**（不存在 `--where`），无法按分片截取数据；因此分片按行读取后写 **MVSV-2**。
> 为什么是 MVSV-2：这是系统既有的行情交换格式（`qiom export/import` 同格式），分片产物可直接 `qiom import` 恢复；CSV 只是临时格式，不作为产物。
> 约束：目标表必须与 `cdeu.finv_quote_secu_kline_min` 同构（19 列），保存登记时校验。

## 1. 接口清单

| # | 方法 | 路径 | 说明 |
|---|------|------|------|
| 1 | GET | `/API/V1/Meta/BRS/Backup/Finv/Slice/Rule/List` | 登记列表 + 每张表的分片完成度（`plan`） |
| 2 | POST | `/API/V1/Meta/BRS/Backup/Finv/Slice/Rule/Save` | 新增/编辑登记（按 `rdb_type + db + schema_name + table_name` 唯一） |
| 3 | POST | `/API/V1/Meta/BRS/Backup/Finv/Slice/Rule/Toggle` | 启用/停用登记 |
| 4 | GET | `/API/V1/Meta/BRS/Backup/Finv/Slice/Plan?ruleId=` | 单个登记的分片明细（`items` 最多 200 条） |
| 5 | POST | `/API/V1/Meta/BRS/Backup/Finv/Slice/Run/Manual` | 手动触发一轮分片备份（异步；并发时 409） |
| 6 | GET | `/API/V1/Meta/BRS/Backup/Finv/Slice/Run/Status?sinceSeq=` | 运行状态与日志（`sinceSeq` 增量拉取） |

响应信封与异常映射遵循 [`Docs/DevSpec/ApiSpec.md`](../../DevSpec/ApiSpec.md)：成功 `{"code":0,...}`，参数/校验错误 400，并发 409，服务错误 500。

## 2. `Rule/List`

无查询参数。返回 `data.list`，每项为登记字段 + `plan`：

| 字段 | 类型 | 说明 |
|------|------|------|
| `idn` | number | 主键 |
| `db` / `schema_name` / `table_name` | string | 目标表（`rdb_type` 首期固定 `postgresql`） |
| `flag_enable` | string | `1` 启用 / `0` 停用 |
| `priority` | number | 执行顺序，越大越先 |
| `slice_mode` | string | `date` / `week` / `month` / `column` / `expr` |
| `slice_key_sql` | string\|null | 分片枚举 SQL（为空时按 `slice_mode` 用内置模板） |
| `slice_where_tpl` | string | 分片条件模板，占位 `{value}` / `{start}` / `{end}` |
| `slice_file_tpl` | string | 产物文件名模板，占位 `{key}`，默认 `{key}` |
| `bak_scope` | string | 固定 `data`（分片只导数据，表结构由迁移创建） |
| `compression` | string | `gzip`（默认，产物 `.mvsv.gz`）/ `none`（`.mvsv`）；分片产物是 MVSV-2 文本，**不支持 `zstd`** |
| `min_rows` | number | 分片行数下限，低于该值跳过 |
| `max_slices` | number | 分片数上限，超过则拒绝执行 |
| `redo_slices` | number | 末尾 N 个分片每轮强制重做 |
| `flag_upload` | string | `1` 上传对象存储 / `0` 仅本地 |
| `plan` | object | 分片完成度，见下表 |

`plan` 字段：

| 字段 | 说明 |
|------|------|
| `rule_id` / `table` | 登记主键与 `db.schema.table` |
| `total` | 分片总数 |
| `done` | 已备份分片数（重做窗口内的分片算未完成） |
| `missing` | 待备份分片数 |
| `skipped` | 行数低于 `min_rows` 被跳过的分片数 |
| `redo_from` | 重做窗口起点下标 |
| `items` | 明细数组（最多前 50 条；`Plan` 接口最多 200 条） |

## 3. `Rule/Save`

请求体（JSON，字段 snake_case）：

```json
{
  "idn": 0,
  "db": "finv",
  "schema_name": "cdeu",
  "table_name": "finv_quote_secu_kline_min",
  "flag_enable": "1",
  "priority": 500,
  "slice_mode": "date",
  "slice_key_sql": null,
  "slice_where_tpl": "date = {value}",
  "slice_file_tpl": "{key}",
  "bak_scope": "data",
  "compression": "gzip",
  "min_rows": 1000,
  "max_slices": 5000,
  "redo_slices": 2,
  "flag_upload": "1",
  "remark": "大表增量备份"
}
```

校验规则：

| 字段 | 规则 |
|------|------|
| `db` / `schema_name` / `table_name` | 字母、数字、下划线（`db` 为空时默认 `finv`） |
| `slice_mode` | `date` / `week` / `month` / `column` / `expr`；`column`、`expr` 必须提供 `slice_key_sql` |
| `slice_key_sql` | 只允许 `SELECT` / `WITH` 开头，且不含分号 |
| `slice_where_tpl` | 至少包含 `{value}` 或 `{start}`/`{end}`；用 `{start}/{end}` 时要求分片 SQL 返回区间值 |
| `slice_file_tpl` | 必须包含 `{key}` |
| `bak_scope` | 只能是 `data` |
| `compression` | `gzip` / `none`（`zstd` 会被拒绝：分片产物是 MVSV-2 文本） |
| `min_rows` / `max_slices` / `redo_slices` | `min_rows>=0`、`1<=max_slices<=100000`、`0<=redo_slices<=max_slices` |

## 4. `Run/Manual` 与 `Run/Status`

`Run/Manual` 无请求体，异步执行并立即返回运行快照；已有任务在跑时返回 `409`。

`data` 字段：

| 字段 | 说明 |
|------|------|
| `run_id` | 本轮运行 ID（`yyyyMMdd-HHmmss-manual`） |
| `status` | `idle` / `running` / `succeeded` / `failed` |
| `started_at` / `finished_at` / `elapsed_ms` | 时间与耗时 |
| `total` / `done` / `failed` | 分片总数 / 已完成 / 失败 |
| `current_table` / `current_slice` | 当前处理的表与分片键 |
| `backup_dir` | 本地备份根目录 |
| `error` | 失败原因 |
| `logs` | 日志数组（`seq` / `time` / `level` / `message`） |
| `last_seq` | 最大日志序号（下次增量拉取传 `sinceSeq`） |

`Run/Status?sinceSeq=N` 只返回 `seq > N` 的日志；日志最多保留 1000 条，状态仅存服务内存（重启清空）。

## 5. 产物与恢复

产物命名：`<分片键>.mvsv[.gz]`（`slice_file_tpl` 可改，默认直接用分片键）。文件为 **MVSV-2**，
头部含 `# Field` / `# FieldType` / `# FieldName` / `# 字段名称` / `# Count` / `# Escape`，与 `qiom export` 完全一致。

> 文本列（`ext_field`/`remark`/`provider`）允许包含 `|`、换行等字符：写出时按 `# Escape : "backslash"` 转义，`qiom verify` / `qiom import` 可正确还原（详见 [`../../DataFormat/MvsvFileFormat.md`](../../DataFormat/MvsvFileFormat.md) §5.5）。

```bash
# 本地
ls /data/App/FinvQuant/Backup/BRS/Slice/finv.cdeu.finv_quote_secu_kline_min/
# 20260930.mvsv.gz  20261001.mvsv.gz ...

# 远端（R2）
# <prefix>/<env>/finv.cdeu.finv_quote_secu_kline_min/20260930.mvsv.gz
```

恢复单日数据（目标库表结构需已由迁移建好）：

```bash
# 方式一：用 qiom 导入（与 export 同格式，默认 upsert）
qiom import 20260930.mvsv.gz

# 方式二：批量按日导入
for f in $(ls 2026*.mvsv.gz | sort); do qiom import "$f"; done

# 方式三：先校验文件，再导入
qiom verify 20260930.mvsv.gz && qiom import 20260930.mvsv.gz
```

> `qiom import` 默认 upsert 到 `cdeu.finv_quote_secu_kline_min`（主键 `usc + ts`），重复导入幂等。
> 需要导入到其它库时用 `qiom import --pg-url "postgresql://user:pass@host:5432/db"`。
> 分片文件只含数据行，不含表结构；目标库必须先由迁移建表。

## 6. 分片维度扩展

| 维度 | `slice_mode` | `slice_key_sql` | `slice_where_tpl` | 产物名 |
|------|--------------|-----------------|-------------------|--------|
| 自然日 | `date` | 留空（内置） | `date = {value}` | `20261018.mvsv.gz` |
| ISO 周 | `week` | 留空（内置） | `date >= {start} AND date <= {end}` | `2026W43.mvsv.gz` |
| 自然月 | `month` | 留空（内置） | `date >= {start} AND date <= {end}` | `202610.mvsv.gz` |
| 按列值 | `column` | `SELECT DISTINCT region AS slice_key, NULL AS slice_start, NULL AS slice_end, count(*) AS slice_rows FROM cdeu.t GROUP BY 1` | `region = '{value}'` | `CN.mvsv.gz` |
| 自定义 | `expr` | 任意，须返回 4 列（`slice_key` / `slice_start` / `slice_end` / `slice_rows`） | 任意，含 `{value}` 或 `{start}`/`{end}` | `slice_file_tpl` 决定 |

> 内置模板已过滤 `date IS NULL`：分片依据字段为空的行不在分片备份范围内。
> 周/月必须用 `date >= start AND date <= end` 的区间条件才走索引；写成 `to_char(date)=...` 会全表扫描。
