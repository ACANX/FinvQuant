# 数据库 Schema 规范（PostgreSQL）

> 适用范围：FinvQuant 服务端全部 PostgreSQL 对象（`Deploy/Migrations/`、`internal/` 中的 SQL、运维手工操作）。
> 相关：[`LogSpec.md`](LogSpec.md)、[`ApiSpec.md`](ApiSpec.md)、[`../Refactor/2610-MoveTablesToSchemas.md`](../Refactor/2610-MoveTablesToSchemas.md)（本规范的落地改造）。

## 1. Schema 划分

| Schema | 归属 | 表 |
|---|---|---|
| `quote` | 行情域 | `finv_quote_secu_kline_min`、`finv_quote_ingest_batches`、`finv_quote_revision_log` |
| `quant` | 量化域 | `finv_quant_backtest_*`（11 张）、`finv_quant_environment`、`finv_quant_template` |
| `futu` | 外部源映射域（富途 OpenAPI 编码 ↔ Finv 内部编码） | `finv_futu_mapping_cs_market`、`finv_futu_mapping_exchange`、`finv_futu_mapping_market_code`、`finv_futu_mapping_region`、`finv_futu_mapping_security` |
| `export` | 数据导出域 | `finv_quote_secu_kline_min`（行情导出转存表，与 `quote` 同名同结构，`V35` 创建） |
| `sche` | 定时任务调度域（调度定义、执行记录等，`V34` 建 schema，表待业务迁移创建） | 暂无 |
| `dedup` | 数据去重域（批内去重暂存、跨源指纹、入库审计，`V36` 建 schema，表待业务迁移创建） | 暂无 |
| `archive` | 冷数据归档域（历史行情按时间/证券下沉，`V37` 建 schema，表待业务迁移创建） | 暂无 |
| `public` | 基础主数据与系统表 | `finv_security`、`finv_market`、`finv_exchange`、`finv_region`、`finv_currency`、`schema_version` |

- **一个业务域一个 schema**，不把域对象堆在 `public`。
- 新增域时同步在本文档登记其 schema 名与归属表，并在迁移脚本中 `CREATE SCHEMA IF NOT EXISTS <域名>`。
- schema 名统一**小写短名**（`quote`/`quant`/`futu`/`export`/`sche`/`dedup`/`archive`），不加业务前缀；`sche` 为「调度」的短名沿用（如需改为 `sched`，须 `ALTER SCHEMA sche RENAME TO sched` 并同步 `search_path`、本文件与代码）。
- **同名同结构转存表**：允许在 `export` 等暂存域建与业务域**同名同结构**的表（如 `export.finv_quote_secu_kline_min` ↔ `quote.finv_quote_secu_kline_min`），用于导出前转存；此类表必须：
  1. 建表用 `CREATE TABLE ... (LIKE <源表> INCLUDING DEFAULTS INCLUDING CONSTRAINTS INCLUDING INDEXES INCLUDING STORAGE INCLUDING COMMENTS)`，**触发器另行补齐**（`LIKE` 不复制触发器）；
  2. 索引名由 PostgreSQL 按新表名自动生成，与源表索引名不同（属正常现象）；
  3. 迁移末尾做双向列差集校验，确保与源表列名/类型/可空/默认值一致；
  4. **任何读写必须写暂存域前缀**（`export.xxx`）——注意 `TestSQLSchemaQualified` 只校验「有没有前缀」，不校验「前缀对不对」，写错成 `quote.` 不会被测试拦下。

## 2. 表命名

- 表名**保留业务前缀**（`finv_quote_*`、`finv_quant_*`），schema 与前缀不重复表述业务——前缀用于跨 schema 检索、日志与 `pg_dump` 过滤，暂不裁剪。
- 表名全小写下划线分隔，主键列名统一 `id` 语义前缀（如 `run_id`、`revision_id`）。
- 跨域引用只走程序层（无物理外键），跨 schema 读取需显式写 `<schema>.<table>`。

## 3. 连接与 search_path

- 应用连接串固定带 `search_path=public,quote,quant,futu,export,sche,dedup,archive`（见 `internal/database/postgres.go`），迁移与业务 SQL 均依赖此设置。
- 代码中的 SQL **必须显式写 schema 前缀**（`quote.xxx`、`quant.xxx`、`futu.xxx`、`export.xxx`、`sche.xxx`、`dedup.xxx`、`archive.xxx`）；`search_path` 只作为历史迁移脚本与手工 `psql` 的兜底。
- **强制校验**：`internal/database/schema_qualify_test.go` 的 `TestSQLSchemaQualified` 会扫描 `internal/`、`cmd/` 下全部 Go 源码，凡含 `FROM` / `INTO` / `UPDATE` / `JOIN` / `DELETE FROM` / `TRUNCATE` 的语句，只要出现 `finv_quote_` / `finv_quant_` / `finv_futu_` 而未带对应 schema 前缀，`go test ./...` 直接失败。新增代码漏写前缀会在本地与 CI 同时被拦下。

### 3.1 融合前的同名表风险（重要）

- `search_path` 为 `public, quote, quant, futu, export, sche, dedup, archive`，**`public` 位于首位**：未限定的 CRUD 语句会优先命中 `public` 中的同名表。
- 系统融合阶段若需要在其它 schema 引入与域表同名的表，必须遵守：
  1. 新表一律放入明确 schema，不得落 `public`；
  2. 代码侧引用一律写全限定名（已由 `TestSQLSchemaQualified` 强制）；
  3. 若确实需要在 `public` 放同名表，必须先把 `public` 移出 `search_path` 首位，并同步核验历史种子迁移（`V100xxx` 不带 schema 前缀，依赖 `search_path` 定位 `quote`/`quant`/`futu` 与 `public` 的基础主数据表）。
- **已存在的实例**：`quote.finv_quote_secu_kline_min` 与 `export.finv_quote_secu_kline_min` 同名（`V35` 建立的导出转存表）。因此裸写 `finv_quote_secu_kline_min` 一定命中 `quote`（`search_path` 中 `quote` 在 `export` 之前），转存表的读写必须显式写 `export.`。
- 历史种子迁移是唯一仍依赖 `search_path` 的 SQL，改动其定位行为前必须跑「全新库全链迁移」验证。
- 手工运维时若未设置 `search_path`，须写全限定名，例如 `select count(*) from quote.finv_quote_secu_kline_min;`。
- `schema_version` 与迁移机制保留在 `public`，不随业务域迁移。
- **归档域的边界（`archive` schema）**：`archive` 是**数据库冷数据归档**，与仓库根目录 `Archive/`（**文件**归档：废弃脚本/文档）无关，两者不要混淆。归档表通常沿用业务表名，会与 `quote`、`export` 形成三处同名（如 `finv_quote_secu_kline_min`），因此库内读写必须写 `archive.` 前缀；跨域搬数据用 `INSERT INTO archive.x SELECT ... FROM quote.x` 后按批次删除源数据，整个过程需在 `sche` 记录批次以便回溯。
- **去重的层次选择（`dedup` schema 的适用边界）**：`quote.finv_quote_secu_kline_min` 已有主键 `(ts, usc)`，跨批次幂等直接用 `INSERT ... ON CONFLICT DO NOTHING` 即可，**不需要**额外对象；只有「批内去重暂存」「跨源内容指纹比对」「入库审计留痕」这三类才应落到 `dedup`，避免为已由主键保证的幂等再建一套去重表。

## 4. 迁移约定

- 结构迁移用 `V<number>__<name>.sql`（`V1`～`V99999`），种子/数据迁移用 `V100000+`，两类均按版本号升序执行。
- **历史迁移文件不可回改**；唯一例外是「新结构已使历史迁移无法在全新库执行」的兼容修正，此类修正必须：只改被阻断语句、保持幂等、对已应用库无影响，并在 `CHANGELOG` 与对应改造清单中登记。
  - 已发生的例子：`V29` 把 `secu_code` 改名为 `usc` 后，晚于它执行的种子迁移 `V100019`（INSERT 列清单）与 `V100023`（`CREATE TABLE` 列定义）仍引用旧列名，只修正这两处，改为 `usc`。
- 需要排在种子段之后执行的结构收尾迁移，使用 `V1000xx` 编号并在此文档与迁移文件头注明原因。
  - 已发生的例子：`V100024`（把 `V100023` 在 `public` 新建的 3 张 `finv_quant_*` 表移入 `quant`）。
  - 反例（无需收尾迁移）：`finv_futu_*` 的建表全部在 `V7`～`V17`，种子段只做 `INSERT`/`UPDATE`/`DELETE`，故 `V33` 一次迁移即可完成，不需要 `V1000xx` 收尾（详见 [`../Refactor/2610-MoveFutuTablesToSchema.md`](../Refactor/2610-MoveFutuTablesToSchema.md)）。
- 新建/移动表的迁移必须幂等（`CREATE SCHEMA IF NOT EXISTS`、`pg_tables` 位置判断），并在脚本末尾做收敛校验（`RAISE EXCEPTION` 触发整体回滚）。

## 5. 变更要求

- 新增或调整 schema 归属须同步：迁移脚本、`internal/` 中的 SQL、`Docs/API`、`Docs/Menu`、`Docs/DataFormat`、`Docs/DevSpec/BacktestStrategySpec.md` 与本文件。
- 破坏性变更（schema 迁移、列改名）必须在 `CHANGELOG` 标注并说明升级约束（禁止新旧镜像共存、升级前备份）。
- 新增 schema 的迁移须同时授予应用角色 `USAGE`/`CREATE`（`V34` 以 `current_user` 动态授权，库名/角色名变更后无需改迁移）。
