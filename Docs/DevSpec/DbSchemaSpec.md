# 数据库 Schema 规范（PostgreSQL）

> 适用范围：FinvQuant 服务端全部 PostgreSQL 对象（`Deploy/Migrations/`、`internal/` 中的 SQL、运维手工操作）。
> 相关：[`LogSpec.md`](LogSpec.md)、[`ApiSpec.md`](ApiSpec.md)、[`../Refactor/2610-003-MoveTablesToSchemas.md`](../Refactor/2610-003-MoveTablesToSchemas.md)（本规范的落地改造）。

## 0. 表的存在性与归属：不可擅自变更（红线）

> **本章优先级高于本文件其它所有章节。** 与任何改造文档、迁移注释、代码现状冲突时，以本章为准。

### 0.1 绝对禁止（未经明确授权不得执行）

以下操作**一律视为破坏性变更**，无论看起来多么"冗余"、"重复"、"应该合并"，都不得自行判断执行：

| 禁止操作 | 说明 |
|---|---|
| `DROP TABLE` / `DROP SCHEMA` | 删除表或 schema |
| `ALTER TABLE ... SET SCHEMA` | **移动表到别的 schema**（本文件历史上的最大事故就是它，见 §0.5） |
| `ALTER TABLE ... RENAME` / `RENAME COLUMN` | 改表名、改列名 |
| `TRUNCATE` / 无条件 `DELETE` | 清空或全表删除数据 |
| 合并、废弃同名表 | 包括"让两张同名表改成一张"、"把某张表降级为视图"、"改成只有一个权威副本" |
| 停止写入某张表后再"整理"掉它 | 例如把 A 表的写入全部改到 B 表，然后 A 表"自然废弃" |

**同名表不是冗余，通常是刻意的隔离设计**（消费者/加工者分离、域边界分离）。判断"这张表没用"的标准不是"没有代码引用"或"数据看起来一样"，而是**是否有明文记录它被废弃**。

### 0.2 同名表保护名单（明确要求并存，禁止合并）

| 表 | 副本 | 职责 | 禁止事项 |
|---|---|---|---|
| `finv_quote_secu_kline_min` | `quote.` | 历史行情导入（`Import/Upload`）、行情查询（`History/QuoteQuery`）、**量化回测**（只读） | 不得迁走、不得改成视图、不得只保留 `dedup.` |
| | `dedup.` | K 线 API 写入（`internal/kline`）、去重 / 导出 / 上传链路（含导出前清洗） | 不得迁走、不得与 `quote.` 合并 |
| | `export.` | 导出中转快照 | 允许清空（仅此一份允许） |
| | `archive.`（如建） | 冷数据归档 | 不得与业务表合并 |

**设计依据**：回测是「消费者」，要求输入可复现；去重/导出是「加工者」，会写入、清洗、删数据。两者共用一张表时，加工侧任何数据清洗都会直接改变历史回测的输入。这正是保留双副本的原因，**不是历史遗留**。

新增同名表（如 `archive.` 副本）时，必须在本表登记职责与使用者，否则视为违规。

### 0.3 什么算"明确授权"

**只有以下两种情形算授权**：

1. 用户在会话中**明确说明**要删除/迁移/合并某张具体表（"把 `quote.xxx` 删了"、"合并这两张表"）；
2. `Docs/RequirementList.md` 中存在对应条目，且描述了该表的删除/迁移/合并。

**以下一律不算授权**：

- 用户沉默、未表态、未回应；
- 从代码引用、文档注释、迁移历史里**推断**"这是历史遗留"；
- "两张表数据一样"、"没有代码用"、"测试没覆盖"；
- 改造文档里的旧结论（见 §0.4）。

需要动这类表时，**先把疑问提出来让用户确认**，不要先改再解释。

### 0.4 文档冲突时的仲裁顺序

本仓库存在"改造清单"（`Docs/Refactor/*`）与本规范、`CHANGELOG` 三处描述同一张表的情况，历史上曾出现**同一天内自我推翻且旧清单未作废**。遇到描述不一致时，按以下顺序采信：

1. **本规范 §0 与 §1**（最新状态以此为准）；
2. `Docs/RequirementList.md` 的需求条目（含编号即视为已确认）；
3. `Docs/CHANGELOG.md` 的变更记录（看时间戳）；
4. `Docs/Refactor/*` 的改造清单（**仅代表该改造完成时的状态**，可能已被后续迁移推翻）。

**特别注意**：`Docs/Refactor/2610-003-MoveTablesToSchemas.md` 的 `C-10`～`C-20` 验收记录写的是 `quote.finv_quote_secu_kline_min`，后于 `V100038` 被推翻、再由 `V100076` 恢复。**看到这类"☑ 已完成"的清单，不能直接当作当前事实**，必须与 §1 的 schema 划分表核对。

### 0.5 事故记录（反例，勿重犯）

| 时间 | 事件 | 后果 |
|---|---|---|
| 2026-10-24 01:15 | `V100032` 按改造项 C 把 3 张 `finv_quote_*` 迁入 `quote`，清单 `C-10`/`C-11`/`C-16` 验收通过（☑） | 回测与导入正常使用 `quote.` |
| 2026-10-24 14:06 | `V100038` 以"融合 FinvQuoteDataDeDup、API 路径属 DeDup 域"为由，`ALTER TABLE quote.finv_quote_secu_kline_min SET SCHEMA dedup`；同提交把 `internal/quote`/`internal/backtest` 的 SQL 从 `quote.` 改成 `dedup.` | `quote` 副本消失，回测与去重/导出链路共用一张表，隔离设计失效；`2610-003` 的验收清单未作废，两份文档互相矛盾 |
| 2026-10-27 | `V100076` 按原设计恢复 `quote.` 副本（结构对齐 + 一次性回填 280 万行），代码回退为"`quote` 包/回测读 `quote.`，`kline`/导出写 `dedup.`" | 隔离恢复 |

**教训**：
1. `SET SCHEMA` 不是"无害的整理"，它会让依赖该 schema 的代码直接失效；
2. 按"API 路径属于哪个域"去决定表放哪个 schema 是**错误维度**，应按"谁读、谁写、是否需要隔离"决定；
3. 推翻既有改造结论时，必须同时作废该清单的验收标记，否则后来者（含 AI）会读到过期事实。

### 0.6 改库前自检清单（每次动 DDL/DML 前逐条回答）

1. 我要动的表，在 §1 的 schema 划分表里是否列出？它的职责写的是什么？
2. 是否存在同名副本？若有，§0.2 保护名单是否覆盖它？
3. 我的操作属于 §0.1 的禁止项吗？若是，**是否已获得 §0.3 定义的明确授权**？
4. 我依据的结论来自哪里？是否只是某份改造清单的旧验收记录（§0.4）？
5. 删除数据的操作是否已：① 备份到 purge 表 ② 单事务 ③ 收敛校验（`RAISE EXCEPTION`）④ 记录 `reason`/`backup_at`？
6. 变更后需要同步哪些文档（本文件 §1、`CHANGELOG`、`RequirementList`、`Docs/API/*`）？

任一条答不上来，**停手并询问**。

### 0.7 删除数据（非删表）的强制要求

删除表内数据（`DELETE`）虽不属 §0.1，但同样必须有据可查：

- 删除前把受影响行**整行备份**到 `dedup.<表>_purge*` 类表，含 `reason`（判据说明）与 `backup_at`；
- 备份表主键必须含来源标识（如 `src_schema`），避免多表同名 `(usc, ts)` 互相覆盖；
- 单事务内完成「备份 → 删除 → 校验」，校验用 `RAISE EXCEPTION` 而非 `RAISE WARNING`；
- 在 `CHANGELOG` 与 `RequirementList` 登记；清理指南类文档放 `Deploy/Backup/`（gitignored）。

## 1. Schema 划分

| Schema | 归属 | 表 |
|---|---|---|
| `quote` | 行情域 | `finv_quote_ingest_batches`、`finv_quote_revision_log`、**`finv_quote_secu_kline_min`（回测/查询/历史行情导入专用副本，`V100076` 恢复，见 §0.2 保护名单，禁止合并）** |
| `quant` | 量化域 | `finv_quant_backtest_*`（11 张）、`finv_quant_environment`、`finv_quant_template` |
| `futu` | 外部源映射域（富途 OpenAPI 编码 ↔ Finv 内部编码） | `finv_futu_mapping_cs_market`、`finv_futu_mapping_exchange`、`finv_futu_mapping_market_code`、`finv_futu_mapping_region`、`finv_futu_mapping_security` |
| `export` | 数据导出域 | `finv_quote_secu_kline_min`（行情导出转存表，与 `dedup` 同名同结构；`V100035` 创建，`V100038` 按 `dedup` 新结构重建） |
| `sche` | 定时任务调度域（调度定义、执行记录等，`V100034` 建 schema，表待业务迁移创建） | 暂无 |
| `dedup_frozen` | **[已冻结] 原 `dedup` 域**（`V100080` 迁数据、需求 10757 切代码、10759 重命名冻结；数据保留待清理淘汰）。原描述：数据去重域（批内去重暂存、跨源指纹、入库审计）+ 行情月归档导出/上传元数据 + CDEU 流程模型；`V100036` 建 schema、`V100041` 新增归档表、`V100044` 新增 CDEU 表 | `finv_quote_secu_kline_min`（分钟行情主表，`V100038` 由 `quote` 迁入，含 `change_price`/`change_ratio`/`provider`）；归档链路 11 张（`finv_dedup_runtime_setting` 已于 `V100041` 删除，窗口改由 `sys.sys_config` 20006 承载；`finv_dedup_usc_priority` 已于 `V100051` 合并进 `finv_dedup_usc`，优先级/SLA 成为证券主表上的三个可选覆盖列 `priority_rank` / `sla_days` / `max_wait_days` + `priority_gmt_update`，未配置时按 100/5/7 兜底，由 `finv_dedup_usc_priority_group_check` 保证三列同 NULL 或同有值）：`finv_dedup_ingest_stats`、`finv_dedup_daily_budget`、`finv_dedup_scan_watermark`、`finv_dedup_quote_maturity_mon`、`finv_dedup_trade_calendar`、`finv_dedup_job_run_log_export_file`、`finv_dedup_usc`、`finv_dedup_export_job`、`finv_dedup_export_file`、`finv_dedup_export_file_upload_log`、`finv_dedup_export_file_patch`；CDEU 12 张（`V100044` / `V100050` / `V100061` / `V100062`）：`cdeu_workflow`、`cdeu_phase`、`cdeu_process`、`cdeu_job`、`cdeu_job_step`、`cdeu_job_dynamic`、`cdeu_job_step_dynamic`（`V100047` 由 `cdeu_job_dynamic_step` 改名，命名与 `cdeu_job_step` / `cdeu_job_step_run` 同族）、`cdeu_workflow_run`、`cdeu_phase_run`、`cdeu_process_run`、`cdeu_job_run`、`cdeu_job_step_run`（全部无外键；`V100041` 创建归档表，`V100042` 为运行日志增加 `stage` / `progress_percent` / `progress_detail` / `gmt_heartbeat` 四列与未结束记录部分索引，`V100043` 新增的运行时设置表已于 `V100041` 删除，`V100044` 新增 CDEU 定义/运行表并 seed 13 个作业，`V100050` 将旧 `Process → Phase → Step → JOB` 重构为 `Workflow → Phase → Process → JOB → Step`，`V100061` 新增运行时单证券动态 JOB 表，`V100062` 新增动态 JOB 的串行 Step 表（`V100047` 将该表连同其序列、主键、唯一约束(2)、检查约束、非空约束与索引一并从 `cdeu_job_dynamic_step` / `cdeu_dynamic_job_step_*` 改名为 `cdeu_job_step_dynamic` / `cdeu_job_step_dynamic_*`）；`V100043`/`V100044` 给 `cdeu_process_run` / `cdeu_job_run` 补 `workflow_run_id` 冗余列并**删除自增代理键** `process_run_id` / `job_run_id`，改为业务复合主键 `cdeu_process_run(workflow_run_id, process_id)`、`cdeu_job_run(workflow_run_id, process_id, job_id)`；`cdeu_job_step_run` 保留自增 `step_run_id`，其 `job_run_id` 改为三元组引用并新增 `workflow_run_id`/`process_id`/`job_id`；`cdeu_job_run.dynamic_job_id` 记录对应动态工作项（非批次 JOB 为 NULL）；`V100045` 把 `cdeu_job_dynamic` 的唯一键从「(usc, biz_month) 全局唯一」改为**按运行唯一**（`(workflow_run_id, usc, biz_month)` / `(workflow_run_id, file_id)`，均 `NULLS NOT DISTINCT`），消除「上一轮运行的动态 JOB 被下一轮接管并覆盖 workflow_run_id」的缺陷；历史运行实例由 `CLEAN_EXPIRED_EXPORT_ARTIFACTS` 按 30 天保留期清理；`cdeu_workflow_run.run_id` 保持唯一的运行流水自增；说明见 [`../Refactor/2610-005-FinvQuoteDeDubExportUpload.md`](../Refactor/2610-005-FinvQuoteDeDubExportUpload.md) 与 [`../Refactor/2610-008-CDEUWorkflowProcessModelRefactor.md`](../Refactor/2610-008-CDEUWorkflowProcessModelRefactor.md)） |
| `cdeu` | **CDEU 工作流域**（Workflow / Phase / Process / JOB / Step 的定义表与运行实例）；`V100077` 建 schema，`V100078` 依 `dedup` 结构建表，`V100079` 统一前缀 | **26 张**（`V100080` 已从 `dedup` 迁入 133,260 行数据），**全部统一为 `finv_quote_cdeu_` 前缀**：原 `cdeu_*` 12 张（`_workflow` / `_phase` / `_process` / `_job` / `_job_step` / `_job_dynamic` / `_job_step_dynamic` / `_workflow_run` / `_phase_run` / `_process_run` / `_job_run` / `_job_step_run`）+ 原 `finv_dedup_*` 13 张（`_daily_budget` / `_export_batch` / `_export_file` / `_export_file_patch` / `_export_file_upload_log` / `_export_job` / `_ingest_stats` / `_job_run_log_export_file` / `_job_run_log_line` / `_quote_maturity_mon` / `_scan_watermark` / `_trade_calendar` / `_usc`）+ `finv_quote_secu_kline_min`（表名不变）；`prep_bak_*` 5 张**不创建**。**`V100080` 迁入数据后，需求 10757 已把代码全部切换到 `cdeu.*`**；`dedup.*` 数据保留但不再被读写，待观察后清理淘汰（约束见 §1.1） |
| `archive` | 冷数据归档域（历史行情按时间/证券下沉，`V100037` 建 schema，表待业务迁移创建） | 暂无 |
| `sys` | 系统域（用户/角色/权限、系统参数、审计留痕等平台级对象，`V100071` 建 schema） | `sys_config`（系统配置表，`V100072` 创建，按 `config_no` 的键值配置，含 `int_value`/`char_value`/`str_value`/`json_value` 四类值与 `gmt_modify` 触发器）；`sys_dict`（数据字典主表，`V100073` 创建，字典定义与分级字段）、`sys_dict_sub`（数据字典子表，`V100073` 创建，主键 `(dict_no, sub_key)`，字典项与 `flag_enable` 启用标志，无物理外键）；`sys_user`（系统用户表，`V100074` 创建，主键 `idn` 为 `bigint identity`（初值对齐 1028），`email` / `mobile_phone_no` 各自唯一，`flag_delete` 逻辑删除，无物理外键） |
| `public` | 基础主数据与系统表 | `finv_security`、`finv_market`、`finv_exchange`、`finv_region`、`finv_currency`、`schema_version` |

- **一个业务域一个 schema**，不把域对象堆在 `public`。

### 1.1 `cdeu` 域搬迁约束（需求 10753）

`V100077` 只建了空 schema 并把 `cdeu` 纳入 `search_path`，**表未搬迁**。后续把 12 张 `cdeu_*` 表从 `dedup` 迁入 `cdeu` 时：

| 禁止 | 要求 |
|---|---|
| `ALTER TABLE dedup.cdeu_xxx SET SCHEMA cdeu` 式的**同名机械搬移** | 必须重新审视并可能重构：表名去掉重复的 `cdeu_` 前缀、约束/索引命名、主键与外键策略、运行表与定义表的拆分粒度、历史数据兼容 |
| 把 `finv_dedup_export_*` 一并搬入 | 那批表属**去重/导出域**（导出产物元数据），不属于工作流域，留在 `dedup` |
| 无授权直接搬 | 属 §0 红线范围，须 `RequirementList` 有条目或用户明确点名 |

**域边界定义**：

- `cdeu` = CDEU 工作流的**定义与运行实例**（Workflow / Phase / Process / JOB / Step 的定义表与运行表）
- `dedup` = 行情批内去重、跨源指纹、入库审计、导出/上传元数据、证券主数据

**过渡期约定**：表迁入 `cdeu` 前，`schema_qualify_test.go` 的 `cdeuTables` 清单登记了 12 个表名，但不强制 `cdeu.` 前缀；迁移完成后需把对应表名从 `dedup.` 白名单切到 `{dedup., cdeu.}` 或 `{cdeu.}`。
- 新增域时同步在本文档登记其 schema 名与归属表，并在迁移脚本中 `CREATE SCHEMA IF NOT EXISTS <域名>`。
- schema 名统一**小写短名**（`quote`/`quant`/`futu`/`export`/`sche`/`dedup`/`archive`/`sys`），不加业务前缀；`sche` 为「调度」的短名沿用（如需改为 `sched`，须 `ALTER SCHEMA sche RENAME TO sched` 并同步 `search_path`、本文件与代码）。
- **同名同结构转存表**：允许在 `export` 等暂存域建与业务域**同名同结构**的表（如 `export.finv_quote_secu_kline_min` ↔ `dedup.finv_quote_secu_kline_min`），用于导出前转存；此类表必须：
  1. 建表用 `CREATE TABLE ... (LIKE <源表> INCLUDING DEFAULTS INCLUDING CONSTRAINTS INCLUDING INDEXES INCLUDING STORAGE INCLUDING COMMENTS)`，**触发器另行补齐**（`LIKE` 不复制触发器）；
  2. 索引名由 PostgreSQL 按新表名自动生成，与源表索引名不同（属正常现象）；
  3. 迁移末尾做双向列差集校验，确保与源表列名/类型/可空/默认值一致；
  4. **任何读写必须写暂存域前缀**（`export.xxx`）。`TestSQLSchemaQualified` 对 `finv_quote_secu_kline_min` 有**固定 schema 校验**（允许 `dedup.` 与 `export.`，写错成 `quote.` 会失败），其余域表按前缀规则校验。

## 2. 表命名

- 表名**保留业务前缀**（`finv_quote_*`、`finv_quant_*`），schema 与前缀不重复表述业务——前缀用于跨 schema 检索、日志与 `pg_dump` 过滤，暂不裁剪。
- 表名全小写下划线分隔，主键列名统一 `id` 语义前缀（如 `run_id`、`revision_id`）。
- **`sys` 域例外**：系统域的库表沿用原系统命名 `sys_*`（`sys_config` / `sys_dict` / `sys_dict_sub` / `sys_user`），不带 `finv_` 前缀，主键也沿用原列名（`config_no` / `dict_no` / `idn`）——目的是与既有系统的字典/用户/配置数据保持同名同列，便于直接迁移对齐；`finv_*` 前缀与 `id` 主键规则只约束行情/量化/富途等业务域。
- 跨域引用只走程序层（无物理外键），跨 schema 读取需显式写 `<schema>.<table>`。

## 3. 连接与 search_path

- 应用连接串固定带 `search_path=public,quote,quant,futu,export,sche,dedup,archive,sys`（见 `internal/database/postgres.go`），迁移与业务 SQL 均依赖此设置。
- 代码中的 SQL **必须显式写 schema 前缀**（`quote.xxx`、`quant.xxx`、`futu.xxx`、`export.xxx`、`sche.xxx`、`dedup.xxx`、`archive.xxx`、`sys.xxx`）；`search_path` 只作为历史迁移脚本与手工 `psql` 的兜底。
- **强制校验**：`internal/database/schema_qualify_test.go` 的 `TestSQLSchemaQualified` 会扫描 `internal/`、`cmd/` 下全部 Go 源码，凡含 `FROM` / `INTO` / `UPDATE` / `JOIN` / `DELETE FROM` / `TRUNCATE` 的语句，只要出现 `finv_quote_` / `finv_quant_` / `finv_futu_` 而未带对应 schema 前缀，`go test ./...` 直接失败。新增代码漏写前缀会在本地与 CI 同时被拦下。

### 3.1 融合前的同名表风险（重要）

- `search_path` 为 `public, quote, quant, futu, export, sche, dedup, archive, sys`，**`public` 位于首位**：未限定的 CRUD 语句会优先命中 `public` 中的同名表。
- 系统融合阶段若需要在其它 schema 引入与域表同名的表，必须遵守：
  1. 新表一律放入明确 schema，不得落 `public`；
  2. 代码侧引用一律写全限定名（已由 `TestSQLSchemaQualified` 强制）；
  3. 若确实需要在 `public` 放同名表，必须先把 `public` 移出 `search_path` 首位，并同步核验历史种子迁移（`V100100xxx` 不带 schema 前缀，依赖 `search_path` 定位 `quote`/`quant`/`futu` 与 `public` 的基础主数据表）。
- **已存在的实例**：`finv_quote_secu_kline_min` 有 3 份（`quote.` / `dedup.` / `export.`，见 §0.2 保护名单）。裸写表名会按 `search_path` 命中**排在最前的 `quote.`**，因此三类使用者各自必须写自己的前缀：回测/查询写 `quote.`、K 线 API 与导出链路写 `dedup.`、中转写 `export.`。**不要以「主表在哪」为理由去改其它副本**。
- 历史种子迁移是唯一仍依赖 `search_path` 的 SQL，改动其定位行为前必须跑「全新库全链迁移」验证。
- 手工运维时若未设置 `search_path`，须写全限定名，例如 `select count(*) from dedup.finv_quote_secu_kline_min;`。
- `schema_version` 与迁移机制保留在 `public`，不随业务域迁移。
- **归档域的边界（`archive` schema）**：`archive` 是**数据库冷数据归档**，与仓库根目录 `Archive/`（**文件**归档：废弃脚本/文档）无关，两者不要混淆。归档表通常沿用业务表名，会与 `dedup`、`export` 形成多处同名（如 `finv_quote_secu_kline_min`），因此库内读写必须写 `archive.` 前缀；跨域搬数据用 `INSERT INTO archive.x SELECT ... FROM dedup.x` 后按批次删除源数据，整个过程需在 `sche` 记录批次以便回溯。
- **系统域（`sys` schema）的边界**：`sys` 只承载平台级对象（用户/角色/权限、系统参数、审计留痕等），不得用来放行情/量化/映射等业务表；其表名建议沿用前缀风格（如 `sys.finv_sys_*`），读写必须写 `sys.` 前缀。`sys` 位于 `search_path` 末位，新增同名表不会改变既有解析优先级，但同样的裸写风险依然存在。
- **分钟行情表的双 schema 隔离（ACANX 2026-10-27 纠偏）**：同名表 `finv_quote_secu_kline_min` 存在三份、职责不同，**不允许合并或跨表复用**：
  | schema | 使用者 | 说明 |
  |---|---|---|
  | `quote.` | 历史行情导入、行情查询、回测 | `internal/quote`（`Import/Upload` 与 `History/QuoteQuery`）与 `internal/backtest`；回测 SQL 必须写 `quote.` 前缀 |
  | `dedup.` | K 线 API 写入、去重 / 导出 / 上传链路 | `internal/kline`（`DeDup/Secu/KLine/Min/{Add,Batch,Upsert,Patch,BatchDelete}` 等）与 `internal/meta/finv/quote/dedup`；写入与重写（含导出前清洗） |
  | `export.` | 导出中转 | 快照暂存表，允许清空 |
  隔离靠 schema 前缀强制：漏写前缀会命中 `search_path` 里的另一张表，`internal/database/schema_qualify_test.go` 的 `exactTableSchema` 已把三者列为白名单。`V100076` 恢复了被 `V100038` 迁走的 `quote.` 副本（结构与 `dedup` 对齐、数据一次性回填，之后各自演进）。
- **去重的层次选择**：`dedup.finv_quote_secu_kline_min` 已有主键 `(usc, ts)`，跨批次幂等直接用 `INSERT ... ON CONFLICT DO NOTHING` 即可；只有「批内去重暂存」「跨源内容指纹比对」「入库审计留痕」这三类才需要额外对象。`dedup` 域承载的是**去重 / 导出 / 上传链路**那份副本（`V100038` 迁入），它**不是唯一副本**——`quote.` 侧还有回测专用的同名副本（`V100076` 恢复），两者禁止合并，详见 §0.2。新增对象不得与其同名。

## 4. 迁移约定

- 结构迁移用 `V<number>__<name>.sql`（`V100001`～`V199999`），种子/数据迁移用 `V200000+`，两类均按版本号升序执行。
  - 种子段实例：`V200000`～`V100018`（交易所/货币/证券/富途映射/地区等字典）、`V100019`～`V100024`（量化与回测种子及收尾）、`V100025`（`sys.sys_config` 初始配置 5 条，`ON CONFLICT (config_no) DO NOTHING` 保证重跑不覆盖生产已调整的值）、`V100026`（`sys.sys_dict` 初始字典定义 19 条，同策略）、`V100027`（`sys.sys_dict_sub` 初始子项 89 条，同策略）、`V100028`（地区/国籍子项 387 条，同策略，`3105` 待补增量）、`V100029`（通用标志/分页/数据类型子项 28 条，同策略）、`V100030`（`3105` 国籍补齐至 235 条，增量种子）、`V100031`（`sys_config` 新增 `20000`/`20001` 运行与收集模式配置）、`V100032`（上述两项 `create_by` 修正为 `fqe`）、`V100033`（`20001` 备注文案修订）、`V100034`（`20002` 归档上传开关 + `20003` GitHub PAT，不覆盖用户配置）、`V100035`（`20004` 文件上传仓库配置 `{owner}:{repo}:{branch}:{pre_path_key}`）、`V100040`（`20005` CDEU 工作流运行模式，`str_value` 默认 `Manual`：Manual=仅手动触发、调度器不自动跑；Auto=按定时任务时间自动跑）、`V100041`（`20006` 导出低峰窗口，`str_value` 格式 **`HH:MM:SS|HH:MM:SS`**（精确到秒、半角竖线分隔、支持跨零点），默认 `00:30:00|06:30:00`；`V100042`（`20007` 导出成功后立即删除主表行情数据标志，`char_value`：`1`-上传成功后立即删除（默认，之后再次插入按 INCR 增量导出版本号 N 递增）/`0`-保留数据并覆盖同名 BASE、不生成 INCR；取代环境变量 `FINV_QUOTE_DEDUB_PURGE_MAIN_ENABLED`/`_PURGE_MAIN_MONTHS`；删除有两条触发路径——① 上传成功后立即删（`UPLOAD_PENDING_EXPORT_FILES`）；② **沉淀月补偿清理**（`CLEAN_EXPIRED_EXPORT_ARTIFACTS`，判定「BASE 已 EXPORTED + UPLOADED + 主表该月有数据 + BASE 生成后无新增/更新」），用于兜住「checksum 未变→跳过上传→① 永不触发」形成的死水区；删除只作用于 `dedup`，`quote` 副本不动）；同时**删除 `dedup.finv_dedup_runtime_setting`**——该表仅为承载两个时间而建，窗口改由 `20006` 承载，删除前硬校验「表中仅含 `EXPORT_WINDOW_START`/`EXPORT_WINDOW_END` 两键」，`V100046`（`20008` 历史运行动态 JOB 保留天数，`data_type='1'`，`int_value` 默认 `30`，合法区间 `1~3650`，越界或非法时运行期回退默认 30；配合 `V100045` 的动态实例按运行隔离，由 `CLEAN_EXPIRED_EXPORT_ARTIFACTS` 按该天数清理），授权记录见 `RequirementList` 10653）。
- **历史迁移文件不可回改**；唯一例外是「新结构已使历史迁移无法在全新库执行」的兼容修正，此类修正必须：只改被阻断语句、保持幂等、对已应用库无影响，并在 `CHANGELOG` 与对应改造清单中登记。
  - 已发生的例子：`V100029` 把 `secu_code` 改名为 `usc` 后，晚于它执行的种子迁移 `V100019`（INSERT 列清单）与 `V100023`（`CREATE TABLE` 列定义）仍引用旧列名，只修正这两处，改为 `usc`。
- 需要排在种子段之后执行的结构收尾迁移，使用 `V101000xx` 编号并在此文档与迁移文件头注明原因。
  - 已发生的例子：`V100024`（把 `V100023` 在 `public` 新建的 3 张 `finv_quant_*` 表移入 `quant`）。
  - 反例（无需收尾迁移）：`finv_futu_*` 的建表全部在 `V100007`～`V100017`，种子段只做 `INSERT`/`UPDATE`/`DELETE`，故 `V100033` 一次迁移即可完成，不需要 `V101000xx` 收尾（详见 [`../Refactor/2610-004-MoveFutuTablesToSchema.md`](../Refactor/2610-004-MoveFutuTablesToSchema.md)）。
- 新建/移动表的迁移必须幂等（`CREATE SCHEMA IF NOT EXISTS`、`pg_tables` 位置判断），并在脚本末尾做收敛校验（`RAISE EXCEPTION` 触发整体回滚）。

## 5. 变更要求

- 新增或调整 schema 归属须同步：迁移脚本、`internal/` 中的 SQL、`Docs/API`、`Docs/Menu`、`Docs/DataFormat`、`Docs/DevSpec/BacktestStrategySpec.md` 与本文件。
- 破坏性变更（schema 迁移、列改名、删表）必须在 `CHANGELOG` 标注并说明升级约束（禁止新旧镜像共存、升级前备份），且必须先满足 §0.1～§0.3 的授权与登记要求。
- **任何 `DROP TABLE` / `SET SCHEMA` / `RENAME` / `TRUNCATE` 都必须先有 `Docs/RequirementList.md` 条目**；无条目即视为越权变更，评审直接拒绝。
- 新增 schema 的迁移须同时授予应用角色 `USAGE`/`CREATE`（`V100034` 以 `current_user` 动态授权，库名/角色名变更后无需改迁移）。
- 只建 schema 的空迁移沿用「建 schema → 属主/授权 → 追加 `search_path` 末尾 → 收敛校验」四段式模板：`V100034`（`export`/`sche`）、`V100036`（`dedup`）、`V100037`（`archive`）、`V100071`（`sys`）。
