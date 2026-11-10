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
| `finv_quote_secu_kline_min` | `quote.` | 历史行情导入（`Import/Upload`）、行情查询（`History/QuoteQuery`）、**量化回测**（只读） | 不得迁走、不得改成视图、不得只保留加工侧那一份 |
| | `cdeu.` | K 线 API 写入（`internal/meta/finv/quote/cdeu/kline`）、去重 / 导出 / 上传链路（含导出前清洗） | 不得迁走、不得与 `quote.` 合并 |
| | ~~`export.`~~ | **已删除**：`V200088`（2026-10-31，用户明确要求）把导出中转表迁入 `cdeu.finv_quote_secu_kline_min_export` 并 `DROP` 源表；`export` schema 保留但不再承载 kline 表 | 不得重建该副本 |
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

**历史遗留副本清理**：早期建在 `public` 下的 `public.finv_quote_secu_kline_min` 不在 §0.2 保护名单（名单只有 `quote.` / `cdeu.`），由迁移 `V300085`（需求 10899）在数据搬入 `cdeu` 后清理：差集为 0 才 `DROP`，否则保留并告警。

**特别注意**：`Docs/Refactor/2610-003-MoveTablesToSchemas.md` 的 `C-10`～`C-20` 验收记录写的是 `quote.finv_quote_secu_kline_min`，后于 `V200038` 被推翻、再由 `V200076` 恢复。**看到这类"☑ 已完成"的清单，不能直接当作当前事实**，必须与 §1 的 schema 划分表核对。

### 0.5 事故记录（反例，勿重犯）

| 时间 | 事件 | 后果 |
|---|---|---|
| 2026-10-24 01:15 | `V200032` 按改造项 C 把 3 张 `finv_quote_*` 迁入 `quote`，清单 `C-10`/`C-11`/`C-16` 验收通过（☑） | 回测与导入正常使用 `quote.` |
| 2026-10-24 14:06 | `V200038` 以"融合 FinvQuoteDataDeDup、API 路径属 DeDup 域"为由，`ALTER TABLE quote.finv_quote_secu_kline_min SET SCHEMA dedup`；同提交把 `internal/meta/finv/quote`/`internal/backtest` 的 SQL 从 `quote.` 改成 `dedup.` | `quote` 副本消失，回测与去重/导出链路共用一张表，隔离设计失效；`2610-003` 的验收清单未作废，两份文档互相矛盾 |
| 2026-10-27 | `V200076` 按原设计恢复 `quote.` 副本（结构对齐 + 一次性回填 280 万行），代码回退为"`quote` 包/回测读 `quote.`，`kline`/导出写 `dedup.`" | 隔离恢复 |
| 2026-10-30 | 需求 10834：`V200086` 删除 `dedup` / `dedup_frozen`（先 `pg_dump` 备份 32 张表 / 约 13.4 万行到 `Deploy/Backup/dedup_schemas_20261030_234253.sql.gz`） | 旧域彻底淘汰；`TestSQLSchemaQualified` 新增 `forbiddenSchemaPrefixes`，代码再引用 `dedup.` / `dedup_frozen.` 直接失败 |

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

- 删除前把受影响行**整行备份**到 `archive.<表>_purge*` 类表（`dedup` 已于 `V200086` 删除，备份不再写回该域），含 `reason`（判据说明）与 `backup_at`；
- 备份表主键必须含来源标识（如 `src_schema`），避免多表同名 `(usc, ts)` 互相覆盖；
- 单事务内完成「备份 → 删除 → 校验」，校验用 `RAISE EXCEPTION` 而非 `RAISE WARNING`；
- 在 `CHANGELOG` 与 `RequirementList` 登记；清理指南类文档放 `Deploy/Backup/`（gitignored）。

## 1. Schema 划分

| Schema | 归属 | 表 |
|---|---|---|
| `quote` | 行情域 | `finv_quote_ingest_batches`、`finv_quote_revision_log`、**`finv_quote_secu_kline_min`（回测/查询/历史行情导入专用副本，`V200076` 恢复，见 §0.2 保护名单，禁止合并）** |
| `quant` | 量化域 | `finv_quant_backtest_*`（11 张）、`finv_quant_environment`、`finv_quant_template` |
| `futu` | 外部源映射域（富途 OpenAPI 编码 ↔ Finv 内部编码） | `finv_futu_mapping_cs_market`、`finv_futu_mapping_exchange`、`finv_futu_mapping_market_code`、`finv_futu_mapping_region`、`finv_futu_mapping_security`、`finv_futu_quote_collect`（富途原始属性镜像，`V200083` 新增：主键 `usc`，唯一 `(market_code, futu_symbol)`，承载 `stock_id` / `futu_symbol` / `market_type` / `market_code` / `instrument_type` / `sub_instrument_type` / `req_section` / `market_label` / `instrument` / `flag_enable` / `remark`（`V300140` 把原 `quote_market` / `type_symbol` 改名而来）；`V300147` 追加**生成列** `ftmm_symbol`（`GENERATED ALWAYS AS (market_label \|\| '.' \|\| futu_symbol) STORED`，带普通索引 `idx_finv_futu_quote_collect_ftmm_symbol`，对齐上游 `ftmm_symbol = {market}.{code}` 口径；**非唯一**——`market_label` 与 `market_code` 非 1:1，实测 75 组撞名；写入时不得显式赋值），数据由仓库外工具 `FinvQuantDesign/Scripts/SecuSync/BuildFutuQuoteCollect.py` 从上游视图初始化与增量同步）、`finv_futu_secu_key`（证券键表，`V300137` 以 `finv_quote_secu_key` 建表、`V300141` 改名为现名：主键 `ftmm_symbol`，索引 `(instrument, market)`，承载 `instrument` / `market` / `code` / `flag_enable` / `priority` / `status` / `url`，`V300139` 追加可空的 `stock_id` / `usc`（NULL=尚未解析到，共 12 列），用于 instrument+market+code ↔ 富途 `ftmm_symbol` 的键映射与采集状态跟踪；`gmt_update` 由 `public.vq_set_gmt_update()` 维护）、**USC 映射规则四表 + 引擎（`V300157`，需求 11072）**：`finv_futu_usc_rule`（规则层，`rule_no` 主键、**10 条**：P10 闸门 / P12 P14 P16 P22 前处理（`V300159` 起 P22 由市场默认提为前处理、`V300161` 新增 P16 指数去前导点）/ P20-P21 市场默认 / P40 P50 例外 / P60 兜底；`rule_type`、`priority` 唯一、`match_scope`=ALL\|NON_GATE、`default_action`=CODE\|FTMM\|LITERAL\|REWRITE\|SH4、`flag_enable`、`rule_desc`（规则说明）、`remark`）、`finv_futu_usc_rule_item`（条目层，`idn` 主键、689 条种子；匹配维度 `market`/`instrument`/`code`/`code_regex` 均可空表通配，`action`/`usc_literal`/`rewrite_tpl`、`status`=ACTIVE\|PENDING\|RETIRED、`effective_from`/`effective_to`、`priority`、`scope_env`、`item_desc`（条目说明）/`source_ref`（出处）/`remark`；唯一键 `UNIQUE NULLS NOT DISTINCT (rule_no, scope_env, market, instrument, code, code_regex)`）、`finv_futu_usc_map`（结果表，`ftmm_symbol` 主键，`usc` + 生成列 `usc_lower`，`rule_no`/`basis`/`status`/`retired_at`/`reuse_of_ftmm`/`scope_env`/`rule_version`/`computed_at`/`first_assigned_at`；**部分唯一索引 `uq_finv_futu_usc_map_active (usc_lower) WHERE status='ACTIVE'`**——按决策「退市后 `usc` 允许复用」，RETIRED 行不占位）、`finv_futu_usc_rule_log`（审计日志），以及跨域归档表 `archive.finv_usc_reuse_snapshot`（`usc` 被复用时下游各表历史行 `row_json` 整行快照，唯一键 `(usc_lower, src_table, src_key, from_ftmm)`）；引擎为函数 `futu.f_finv_futu_usc_calc(p_env)` + 视图 `futu.finv_futu_usc_calc_view`（前处理 → `canon_code` → 大小写不敏感计数 `canon_cnt`，不落库，供预演/对账） |
| `export` | 数据导出域（`V200034` 建 schema） | **暂无**：唯一的 `finv_quote_secu_kline_min` 已由 `V200088` 迁入 `cdeu.finv_quote_secu_kline_min_export` 并删除源表（schema 保留，删除 schema 需单独授权） |
| `sche` | 定时任务调度域（调度定义、执行记录等，`V200034` 建 schema，表待业务迁移创建） | 暂无 |
| ~~`dedup_frozen`~~ | **[已删除] 原 `dedup` 域的冻结副本** | `V200080` 迁数据入 `cdeu`、需求 10757 切代码、10759 重命名冻结；**`V200086`（需求 10834）已 `DROP SCHEMA dedup_frozen CASCADE`**，删除前 `pg_dump` 备份至 `Deploy/Backup/dedup_schemas_20261030_234253.sql.gz`（32 张表 / 约 13.4 万行）。此后代码不得引用 `dedup.` / `dedup_frozen.`，由 `forbiddenSchemaPrefixes` 硬拦截 |
| `cdeu` | **CDEU 工作流域**（Workflow、Phase、Process、JOB、Step 的定义表与运行实例）；`V200077` 建 schema，`V200078` 依 `dedup` 结构建表，`V200079` 统一前缀 | **28 张**（`V200080` 已从 `dedup` 迁入 133,260 行数据；`V200084` 新增 1 张模板表；`V200088` 自 `export` 域迁入 1 张导出中转表），**全部统一为 `finv_quote_cdeu_` 前缀**：原 `cdeu_*` 12 张（`_workflow` / `_phase` / `_process` / `_job` / `_job_step` / `_job_dynamic` / `_job_step_dynamic` / `_workflow_run` / `_phase_run` / `_process_run` / `_job_run` / `_job_step_run`）+ 原 `finv_dedup_*` 13 张（`_daily_budget` / `_export_batch` / `_export_file` / `_export_file_patch` / `_export_file_upload_log` / `_export_job` / `_ingest_stats` / `_job_run_log_export_file` / `_job_run_log_line` / `_quote_maturity_mon` / `_scan_watermark` / `_trade_calendar` / `_usc`）+ `finv_quote_secu_kline_min`（表名不变）+ `finv_quote_cdeu_usc_day_incr_row_tpl`（`V200084` 新增，日增量行数模板：主键 `(region, market, category)`，三列均可为空串表通配；`day_expect_rows` 供自动补全 `_usc.day_expect_rows`，初始数据见 `V300065`）+ `finv_quote_secu_kline_min_export`（行情导出中转快照表，`V200088` 自 `export` 域迁入并删除源表，与 `cdeu.finv_quote_secu_kline_min` 同构、允许清空，名字沿用 `finv_quote_` 前缀属刻意例外）；另增 `finv_cdeu_secu_realtime_stock`（`V300115`，证券实时行情快照：一行一证券、主键 `stock_id`、`V300117`–`V300120` 逐步加列、`V300126` 补 5 个人民币计价字段（需求 11023）、`V300127` 补 `remark`（需求 11028）、`V300128` 补 `gmt_create`/`gmt_update`（需求 11029）后共 **34 列**，含 `currency`（ISO 4217 三位字母货币代码）、`dt_update` 为 Unix 毫秒时间戳、`price_direct` 为 `+`/`-`/`0` 符号、`page` 为来源页码（`INTEGER`，`page_chk` 约束拒绝 ≤ 0，NULL 表示未知）；表名沿用用户给定的 `finv_cdeu_` 前缀，**属刻意例外**，不得按常规前缀改名）；另增 `finv_cdeu_secu_realtime_sync_config`（`V300121`，实时行情列表采集配置：一行=一个「市场 + 证券品种」、9 列、主键 `(market_type, instrument_symbol)`，`instrument_symbol` 区分 `stock`/`etf`/`future`，`count_plan` 为计划采集数量、`count_exists` 与 `total_available` 由采集成功后回写（接口 `pagination.total`）；结果数据仍落 `finv_cdeu_secu_realtime_stock`，不另建结果表）；另增 `finv_quote_secu_indicator`（`V300144` 以 `finv_quote_secu_meta` 建表、`V300145` 改名为现名（需求 11049）：**证券指标表**，**132 列**、主键 `usc`、索引 `(ts)` / `(dt_update)` / `(stock_id)`、触发器 `finv_quote_secu_indicator_set_dt_update` 维护 `dt_update`；列名/类型与上游 Supabase `public.finv_quote_secu_meta` 对齐，但**只含指标列**（上游的 `instrument` / `market` / `stock_code` / `instrument_type` / `sub_instrument_type` / `market_code` / `market_type` / `market_label` / `name_en` / `flag_option` / `flag_futures` / `flag_plate` / `flag_etf_theme` 共 13 列不在本表；`stock_id` 原属排除名单，已由 **`V300148`（需求 11061）补回**：`bigint`、可空（NULL = 尚未同步到）、随 Tab 2 同步写入，并与 `futu.finv_futu_quote_collect.stock_id` 同口径），身份列以 `usc` + `ftmm_symbol` 关联 `futu.finv_futu_secu_key`；`dt_collect` / `dt_create` / `dt_update` 均**可空、无默认值**（值由同步写入上游时间戳）；`V300146`（需求 11053）已为**全部列补齐中文注释**（列说明取自上游采集脚本 `schema.pg.sql`，本表专有列用本地措辞覆盖）；只保最新、不建历史表）；`prep_bak_*` 5 张**不创建**。**`V200080` 迁入数据后，需求 10757 已把代码全部切换到 `cdeu.*`**；`V200086`（需求 10834）已删除 `dedup` / `dedup_frozen` 两个 schema，旧域彻底淘汰（删除前备份见 `Deploy/Backup/`） |
| `archive` | 冷数据归档域（历史行情按时间/证券下沉，`V200037` 建 schema）。**usc 一次性对齐归档（`V300168`，需求 11085）**：`archive.finv_quote_collect_usc_align_20261110`（`futu.finv_futu_quote_collect` 改前整行快照，唯一键 `(usc_from, stock_id)`）、`archive.finv_quote_secu_indicator_usc_align_20261110`（`cdeu.finv_quote_secu_indicator` 改前整行快照，唯一键 `(usc_from)`）；两表含 `usc_from` / `usc_to` / `row_json` / `reason` / `dt_snapshot`，用于事后追溯与反向恢复，**只增不改、不参与业务查询** | usc 对齐回溯（人工排查 / 回滚），无代码读写 |
| `sys` | 系统域（用户/角色/权限、系统参数、审计留痕等平台级对象，`V200071` 建 schema） | `sys_config`（系统配置表，`V200072` 创建，按 `config_no` 的键值配置，含 `int_value`、`char_value`、`str_value`/`json_value` 四类值与 `gmt_modify` 触发器）；`sys_dict`（数据字典主表，`V200073` 创建，字典定义与分级字段）、`sys_dict_sub`（数据字典子表，`V200073` 创建，主键 `(dict_no, sub_key)`，字典项与 `flag_enable` 启用标志，无物理外键）；`sys_user`（系统用户表，`V200074` 创建，主键 `idn` 为 `bigint identity`（初值对齐 1028），`uid` / `email` / `mobile_phone_no` 各自唯一（`uid` 唯一索引 `uk_sys_user_uid` 由 `V200081` 补齐，登录以 `uid` 为账号标识），口令以 `argon2id` 哈希存 `password`、随机盐存 `password_salt`，**不存明文**，`flag_delete` 逻辑删除，无物理外键） |
| `open` | 对外 OpenAPI 接入域（调用方 / 凭证 / 调用审计，`V100013` 建 schema，`V300183` 建表，需求 11167） | **3 张**，表名一律 `open_` 前缀：`open_api_client`（调用方，一行 = 一个接入应用；`allow_cidr` 白名单、`rate_limit_rpm` / `quota_day` 配额、`flag_enable` 默认 `'0'`）；`open_api_credential`（凭证，一行 = 一种认证模式：`auth_mode`=`API_KEY`/`AK_SK`/`BEARER`/`MTLS`/`BASIC`；`access_key_id` 唯一公开标识；静态 Key 存 `secret_hash`（sha256/argon2id）、AK-SK 的 SK 存 `secret_cipher`（AES-256-GCM，密钥取 `sys_config` 19005、KID 取 19004），两者按 `auth_mode` 由 CHECK 二选一；`secret_cipher_prev` + `dt_secret_rotate` 支持无感轮换；`scope_str jsonb` 授权范围；吊销必须留 `dt_revoke` + `reason_revoke`）；`open_api_invoke_log`（调用审计，**只增不改**，只存 `body_sha256` / `count_body` 不存原文，按 `sys_config` 19003 保留天数清理）。**无物理外键**；防重放 nonce 存 Redis，**不建 nonce 表**；配置项占 `sys_config` 19xxx 号段（19001 鉴权模式开关 / 19002 签名时间窗 / 19003 日志保留天数 / 19004 密钥 KID / 19005 密钥本体 / 19006 兼容兜底密钥）；错误码占 `190xx` 子段（见 `ErrorCodeSpec` §1.2）。`scope_str` 后续可扩展为 `open.open_api_credential_scope` 表 |
| `public` | 基础主数据与系统表 | `finv_security`、`finv_market`、`finv_exchange`、`finv_region`、`finv_currency`、`schema_version`、`finv_fxcer`（`V300124` 新增，**缺省汇率表（现行唯一写入目标，需求 11002）**：主键 `usc`（上游 stockCode／`GetFxOverview` 的 map key，需求 11020 起不再有派生行），列为 `usc`／`name`／`currency_src`／`currency_dest`／`rate`／`ts_quote`／`source`／`flag_derive`／`remark`／`change_price`／`change_ratio`（需求 11016）／`gmt_create`／`gmt_update`；`rate` = 1 单位源币种折合的目标币种数量；`ts_quote` 为 Unix 毫秒（`overview` 用上游逐对时间，`rateList` 用采集时刻）；两来源（`overview`／`rateList`）写同一张表、互不删除，跨来源以最后一次采集为准；需求 11020 起不再派生 CNY（`flag_derive` 保留但只写 `'0'`，CNY 由 `get-fx-rate-list` 直采）；只保最新、不建历史表；文档见 `Docs/DataDictMapping/FinvFxcer.md`） |

- **一个业务域一个 schema**，不把域对象堆在 `public`。

### 1.0 `open` 域的边界（需求 11167）

- `open` 只承载**对外 OpenAPI 接入**的三类对象：调用方（`open_api_client`）、凭证（`open_api_credential`）、调用审计（`open_api_invoke_log`）。**不得**往里放行情/量化/映射业务表，也不得把内部用户/角色搬进来（内部身份仍在 `sys.sys_user`）。
- 表名一律 `open_` 开头（用户 2026-11-15 指定）；`open` 在 PostgreSQL 里是 unreserved keyword，不加引号即可写 `open.open_api_client`（已实测）。
- `search_path` 末尾追加 `open`（`internal/database/postgres.go` 与 `TestMigrateIntegration` 两处保持一致）；代码引用仍必须写 `open.` 全限定名。
- 凭证的 SK 只能**加密**存储（HMAC 验签需取回明文），静态 Key 只能**哈希**存储；两者由 `ck_open_api_credential_secret` 约束互斥。密钥本体不进迁移、不进 Git（`sys_config` 19005 默认空）。

### 1.1 `cdeu` 域搬迁约束（需求 10753）

`V200077` 只建空 schema 并把 `cdeu` 纳入 `search_path`；`V200078`–`V200080` 完成 26 张表建立（前缀统一 `finv_quote_cdeu_`）与 133,260 行数据迁移，需求 10757 完成代码切换、10759 把旧 `dedup` 重命名为 `dedup_frozen` 冻结。当时（`V200077` 阶段）确定的原则仍然适用：

| 禁止 | 要求 |
|---|---|
| `ALTER TABLE <旧域>.cdeu_xxx SET SCHEMA cdeu` 式的**同名机械搬移** | 必须重新审视并可能重构：表名去掉重复的 `cdeu_` 前缀、约束/索引命名、主键与外键策略、运行表与定义表的拆分粒度、历史数据兼容 |
| 把 `finv_dedup_export_*` 一并搬入 | 那批表属**去重/导出域**（导出产物元数据），不属于工作流域；`V200080` 已随域整体迁入 `cdeu` 并统一改名为 `finv_quote_cdeu_export_*` |
| 无授权直接搬 | 属 §0 红线范围，须 `RequirementList` 有条目或用户明确点名 |

**域边界定义**：

- `cdeu` = CDEU 工作流的**定义与运行实例**（Workflow / Phase / Process / JOB / Step 的定义表与运行表）
- `dedup` = 行情批内去重、跨源指纹、入库审计、导出/上传元数据、证券主数据（**该域已于 `V200086` 并入 `cdeu` 并删除，历史说明仅作追溯**）

**过渡期已结束**：全部域表已归位 `cdeu.`；`schema_qualify_test.go` 的 `forbiddenSchemaPrefixes` 会硬拦截任何 `dedup.` / `dedup_frozen.` 引用（需求 10834）。
- 新增域时同步在本文档登记其 schema 名与归属表，并在迁移脚本中 `CREATE SCHEMA IF NOT EXISTS <域名>`。
- schema 名统一**小写短名**（`quote`/`quant`/`futu`/`export`/`sche`/`cdeu`/`archive`/`sys`/`open`；`dedup` 与 `dedup_frozen` 已于 `V200086` 删除，不得再创建），不加业务前缀；`sche` 为「调度」的短名沿用（如需改为 `sched`，须 `ALTER SCHEMA sche RENAME TO sched` 并同步 `search_path`、本文件与代码）。
- **同名同结构转存表**：允许在暂存域建与业务域**同名同结构**的表，用于导出前转存；此类表必须：
  1. 建表用 `CREATE TABLE ... (LIKE <源表> INCLUDING DEFAULTS INCLUDING CONSTRAINTS INCLUDING INDEXES INCLUDING STORAGE INCLUDING COMMENTS)`，**触发器另行补齐**（`LIKE` 不复制触发器）；
  2. 索引名由 PostgreSQL 按新表名自动生成，与源表索引名不同（属正常现象）；
  3. 迁移末尾做双向列差集校验，确保与源表列名/类型/可空/默认值一致；
  4. **任何读写必须写全限定名**。`TestSQLSchemaQualified` 对 `finv_quote_secu_kline_min` 有**固定 schema 校验**（允许 `quote.` / `cdeu.`，写成已删除的 `dedup.` 或漏写前缀都会失败），对转存表 `finv_quote_secu_kline_min_export` 固定为 `cdeu.`；其余域表按前缀规则校验。
  5. **现状（`V200088` 起）**：唯一的导出中转表是 `cdeu.finv_quote_secu_kline_min_export`（用户要求从 `export` 域迁入 `cdeu` 域并加 `_export` 后缀，源表已 `DROP`）；后续若再建暂存副本，同样需在本文档 §1 与 §0.2 登记职责与使用人。

### 1.2 `usc` 编码口径（需求 10873）

`usc`（统一证券代码）是证券在各域的唯一标识，主键/关联键都用它：

- 形式：**交易所前缀 + 代码**，前缀 2 位（`SH` / `SZ` / `BJ` / `HK` / `US` 等），代码部分不含分隔符；
- **SH 指数类（`SH000001`–`SH000999` 区间）自 2026-11-01 起为 6 位**（去掉数字部分前两个 `0`）：
  `SH000001` → `SH0001`、`SH000985` → `SH0985`；代码侧与上游（`FinvQuantDesign/Scripts/SecuSync`）均已按 6 位产出；
- 迁移 `V300074` 负责历史数据对齐：8 位 usc **只在没有对应 6 位行时就地改号**（避免撞主键 `finv_security_pkey` 与唯一约束 `uq_finv_security_exchange_code`），已存在 6 位行时以 6 位为准并回填英文名、`flag_enable='1'` 优先、`gmt_create` 取较早值；
- 8 位重复行**本期保留不删**（用户决策 D7），由后续需求清理；`futu.finv_futu_mapping_security.finv_usc` 已同步为 6 位；
- 新增/修改证券时**必须**按上述口径生成 `usc`，不得再补前导 `0` 成 8 位；否则会重新产生两套 usc 并导致主键/唯一约束冲突。


### 1.3 `finv_region` 字典 idx 编码约定（需求 10877）

`public.finv_region`（区域/板块字典）主键 `idx`、业务键 `region`（唯一约束 `uq_finv_region_region`）：

| idx 段 | 语义 | 现有取值 |
|---|---|---|
| `0` | 未知/默认 | `UNK` |
| `1` | 全球（含外汇/加密货币兜底） | `GLOBAL` |
| `2`~`100` | **非国家类扩展预留段** | `2`=`FX`（外汇）、`3`=`CC`（加密货币；`V300078` 把初值 `CRYPTO` 修正为 `CC`，`V300079` 起 `finv_exchange` 10000~10007 已引用） |
| `101`+ | 富途 region 原值 + 99 的国家/地区序列 | `101`=`CN` … `123`=`ID`；`103`=`US` |

- 约束方向：`finv_exchange.region` 的每个取值都应在 `finv_region.region` 中存在（`cdeu/store.go` 的元数据回填依赖该 join）。
- 已知缺口（未处理）：`finv_exchange.region='N/A'`（120 `BD` / 121 `BMS` / 122 `FD` / 9003 `TFD`）尚未入字典，靠 `COALESCE` 退化取文本；补录需另立需求。
- 加密货币交易所（10000~10007）的 `region` 已由 `GLOBAL` 切换为 `CC`（`V300079`，需求 10879）；导出路径 `region` 段相应由 `GLOBAL_*` 变为 `CC_*`，该批证券无历史导出记录，无回溯包袱。


## 2. 表命名

- 表名**保留业务前缀**（`finv_quote_*`、`finv_quant_*`），schema 与前缀不重复表述业务——前缀用于跨 schema 检索、日志与 `pg_dump` 过滤，暂不裁剪。
- 表名全小写下划线分隔，主键列名统一 `id` 语义前缀（如 `run_id`、`revision_id`）。
- **`sys` 域例外**：系统域的库表沿用原系统命名 `sys_*`（`sys_config` / `sys_dict` / `sys_dict_sub` / `sys_user`），不带 `finv_` 前缀，主键也沿用原列名（`config_no` / `dict_no` / `idn`）——目的是与既有系统的字典/用户/配置数据保持同名同列，便于直接迁移对齐；`finv_*` 前缀与 `id` 主键规则只约束行情/量化/富途等业务域。
- 跨域引用只走程序层（无物理外键），跨 schema 读取需显式写 `<schema>.<table>`。

## 3. 连接与 search_path

- 应用连接串固定带 `search_path=public,quote,quant,futu,export,sche,cdeu,archive,sys`（见 `internal/database/postgres.go`），迁移与业务 SQL 均依赖此设置。
- 代码中的 SQL **必须显式写 schema 前缀**（`quote.xxx`、`quant.xxx`、`futu.xxx`、`export.xxx`、`sche.xxx`、`cdeu.xxx`、`archive.xxx`、`sys.xxx`）；`search_path` 只作为历史迁移脚本与手工 `psql` 的兜底。
- **强制校验**：`internal/database/schema_qualify_test.go` 的 `TestSQLSchemaQualified` 会扫描 `internal/`、`cmd/` 下全部 Go 源码，凡含 `FROM` / `INTO` / `UPDATE` / `JOIN` / `DELETE FROM` / `TRUNCATE` 的语句，只要出现 `finv_quote_` / `finv_quant_` / `finv_futu_` 而未带对应 schema 前缀，`go test ./...` 直接失败。新增代码漏写前缀会在本地与 CI 同时被拦下。

### 3.1 融合前的同名表风险（重要）

- `search_path` 为 `public, quote, quant, futu, export, sche, cdeu, archive, sys`（`cdeu` 于 `V200077` 纳入；`dedup` 已于需求 10834 由 `V200086` 删除），**`public` 位于首位**：未限定的 CRUD 语句会优先命中 `public` 中的同名表。
- 系统融合阶段若需要在其它 schema 引入与域表同名的表，必须遵守：
  1. 新表一律放入明确 schema，不得落 `public`；
  2. 代码侧引用一律写全限定名（已由 `TestSQLSchemaQualified` 强制）；
  3. 若确实需要在 `public` 放同名表，必须先把 `public` 移出 `search_path` 首位，并同步核验历史种子迁移（`V300xxx` 不带 schema 前缀，依赖 `search_path` 定位 `quote`/`quant`/`futu` 与 `public` 的基础主数据表）。
- **已存在的实例**：`finv_quote_secu_kline_min` 有 2 份（`quote.` / `cdeu.`，见 §0.2 保护名单），另有导出中转表 `cdeu.finv_quote_secu_kline_min_export`（`V200088` 自 `export.` 迁入、同构、允许清空；原 `export.` 副本已随该迁移删除，第 4 份 `dedup_frozen.` 已随 `V200086` 删除）。裸写表名会按 `search_path` 命中**排在最前的 `quote.`**，因此各类使用者各自必须写自己的前缀：回测/查询写 `quote.`、K 线 API 与导出链路写 `cdeu.`、导出中转写 `cdeu.finv_quote_secu_kline_min_export`。**不要以「主表在哪」为理由去改其它副本**。
- 历史种子迁移是唯一仍依赖 `search_path` 的 SQL，改动其定位行为前必须跑「全新库全链迁移」验证。
- 手工运维时若未设置 `search_path`，须写全限定名，例如 `select count(*) from cdeu.finv_quote_secu_kline_min;`。
- `schema_version` 与迁移机制保留在 `public`，不随业务域迁移。
- **归档域的边界（`archive` schema）**：`archive` 是**数据库冷数据归档**，与仓库根目录 `Archive/`（**文件**归档：废弃脚本/文档）无关，两者不要混淆。归档表通常沿用业务表名，会与 `dedup`、`export` 形成多处同名（如 `finv_quote_secu_kline_min`），因此库内读写必须写 `archive.` 前缀；跨域搬数据用 `INSERT INTO archive.x SELECT ... FROM dedup.x` 后按批次删除源数据，整个过程需在 `sche` 记录批次以便回溯。
- **系统域（`sys` schema）的边界**：`sys` 只承载平台级对象（用户/角色/权限、系统参数、审计留痕等），不得用来放行情/量化/映射等业务表；其表名建议沿用前缀风格（如 `sys.finv_sys_*`），读写必须写 `sys.` 前缀。`sys` 位于 `search_path` 末位，新增同名表不会改变既有解析优先级，但同样的裸写风险依然存在。
- **分钟行情表的双 schema 隔离（ACANX 2026-10-27 纠偏）**：同名表 `finv_quote_secu_kline_min` 存在两份、职责不同，**不允许合并或跨表复用**：
  | schema | 使用者 | 说明 |
  |---|---|---|
  | `quote.` | 历史行情导入、行情查询、回测 | `internal/meta/finv/quote`（`Import/Upload` 与 `History/QuoteQuery`）与 `internal/meta/finv/quant/backtest`；回测 SQL 必须写 `quote.` 前缀 |
  | `cdeu.` | K 线 API 写入、去重 / 导出 / 上传链路 | `internal/meta/finv/quote/cdeu/kline`（`Cdeu/Min/{Add,Batch,Upsert,Page,Query,Patch,BatchDelete}` 等）与 `internal/meta/finv/quote/cdeu`；写入与重写（含导出前清洗）。原 `dedup.` 副本已由 `V200086` 删除 |
  | `cdeu.finv_quote_secu_kline_min_export` | 导出中转 | 快照暂存表，允许清空；`V200088` 自 `export.` 迁入（原名 `export.finv_quote_secu_kline_min`，源表已删除） |
  隔离靠 schema 前缀强制：漏写前缀会命中 `search_path` 里的另一张表，`internal/database/schema_qualify_test.go` 的 `exactTableSchema` 已把 `finv_quote_secu_kline_min`（`quote.` / `cdeu.`）与 `finv_quote_secu_kline_min_export`（`cdeu.`）列为白名单。`V200076` 恢复了被 `V200038` 迁走的 `quote.` 副本（结构与加工侧对齐、数据一次性回填，之后各自演进）；加工侧那份已从 `dedup.` 整体迁入 `cdeu.` 并删除旧 schema。
- **去重的层次选择**：`cdeu.finv_quote_secu_kline_min` 已有主键 `(usc, ts)`，跨批次幂等直接用 `INSERT ... ON CONFLICT DO NOTHING` 即可；只有「批内去重暂存」「跨源内容指纹比对」「入库审计留痕」这三类才需要额外对象。`cdeu` 域承载的是**去重 / 导出 / 上传链路**那份副本（`V200038` 迁入 `dedup`、`V200080` 再迁入 `cdeu`），它**不是唯一副本**——`quote.` 侧还有回测专用的同名副本（`V200076` 恢复），两者禁止合并，详见 §0.2。新增对象不得与其同名。

## 4. 迁移约定

- 迁移文件统一 `V<number>__<name>.sql`，按**三段号段**划分，全部按版本号升序执行（需求 10881 号段平移后）：

  | 号段 | 用途 | 现状 |
  |---|---|---|
  | `V100000`～`V199999` | **基础准备**：创建角色、校验数据库、创建 schema | **一对象一脚本**：`V100001` 角色 `finv`、`V100002`～`V100008` 七个 schema（`quote`/`quant`/`futu`/`sys`/`sche`/`cdeu`/`archive` 各一个文件）、`V100009` `brs`（从 `V300060` 归位）、`V100010`～`V100012` 三项只读校验（库名 / 角色对 `public` 的 `CREATE` 权限 / 8 个基础 schema 的 `USAGE`+`CREATE` 权限），共 12 个文件 |
  | `V200000`～`V299999` | **表结构 DDL**：建表、加列、改约束、移表 | 原 `V1xxxxx` 段整体 `+100000`（`V200001`～`V200089`） |
  | `V300000`～`V399999` | **数据**：字典种子、数据回填与修正 | 原 `V2xxxxx` 段整体 `+100000`（`V300000`～`V300080`） |

  - 平移规则：旧 `V1xxxxx` → `V2xxxxx`、旧 `V2xxxxx` → `V3xxxxx`，编号一一对应（`+100000`），执行顺序不变；存量库的 `public.schema_version` 曾由 `Deploy/Upgrade/2611-001-MigrationSegmentsShift.sql` 一次性 `+100000`（`public.schema_version_plan_shift` marker 防重跑）；该脚本与 marker 表已随需求 10895 退役（`V300084` 删除 marker 表），未平移的旧库须手工按同一规则 +100000。
  - **基础准备段粒度：一对象一脚本**（需求 10882 ①已落地）——一个角色、一个 schema、一次库校验各占一个文件；禁止再用一个脚本批量 `CREATE SCHEMA`；新增 schema 时同时新增对应文件与编号，不得拼到已有脚本里。建库不在迁移内：`CREATE DATABASE` 不能在事务块执行，由安装脚本（容器 `POSTGRES_DB`、裸机 `Install.sh` 的 `createdb`）或初始化脚本 [`../../Deploy/Init/2610-001-CreateDatabase.sql`](../../Deploy/Init/2610-001-CreateDatabase.sql)（`psql -v db=finv -v owner=finv -f`，幂等，不建角色不设密码）完成；迁移链只做 `V100010` 库名校验。
  - 新增脚本：DDL 用 `V200090+`、数据用 `V300081+`；号段排序天然保证「DDL 段整体先于数据段」，新的 DDL 只要仍在 `V2xxxxx` 就会排到已有数据之前执行（与旧体系行为一致）。
  - 数据段实例（原 `V2xxxxx` → `V300004`～`V300080`）：地区/富途映射字典、`sys.sys_config` / `sys.sys_dict` / `sys.sys_dict_sub` 初始数据、`sys_config` 运行模式与低峰窗口等配置、`finv_security` / `futu_quote_collect` Supabase 种子、市场码与证券口径回填修正等。
  - 历史遗留（本次平移不重分类）：早期部分数据脚本与 DDL 混写在原 `V1xxxxx` 段（如 `V200018` 交易所种子、`V200025`～`V200042` 系统字典与配置、`V300060` BRS 建表），平移后分别落在 `V2xxxxx` / `V3xxxxx`；基线重做时可择机归位到 `V3xxxxx`。
- **历史迁移文件不可回改**；唯一例外是「新结构已使历史迁移无法在全新库执行」的兼容修正，此类修正必须：只改被阻断语句、保持幂等、对已应用库无影响，并在 `CHANGELOG` 与对应改造清单中登记。
  - 已发生的例子：`V200029` 把 `secu_code` 改名为 `usc` 后，晚于它执行的种子迁移 `V200019`（INSERT 列清单）与 `V200023`（`CREATE TABLE` 列定义）仍引用旧列名，只修正这两处，改为 `usc`。
- 需要排在种子段之后执行的结构收尾迁移，使用 `V201000xx` 编号并在此文档与迁移文件头注明原因。
  - 已发生的例子：`V200024`（把 `V200023` 在 `public` 新建的 3 张 `finv_quant_*` 表移入 `quant`）。
  - 反例（无需收尾迁移）：`finv_futu_*` 的建表全部在 `V200007`～`V200017`，种子段只做 `INSERT`/`UPDATE`/`DELETE`，故 `V200033` 一次迁移即可完成，不需要 `V201000xx` 收尾（详见 [`../Refactor/2610-004-MoveFutuTablesToSchema.md`](../Refactor/2610-004-MoveFutuTablesToSchema.md)）。
- 新建/移动表的迁移必须幂等（`CREATE SCHEMA IF NOT EXISTS`、`pg_tables` 位置判断），并在脚本末尾做收敛校验（`RAISE EXCEPTION` 触发整体回滚）。

## 5. 变更要求

- 新增或调整 schema 归属须同步：迁移脚本、`internal/` 中的 SQL、`Docs/API`、`Docs/Menu`、`Docs/DataFormat`、`Docs/DevSpec/BacktestStrategySpec.md` 与本文件。
- 破坏性变更（schema 迁移、列改名、删表）必须在 `CHANGELOG` 标注并说明升级约束（禁止新旧镜像共存、升级前备份），且必须先满足 §0.1～§0.3 的授权与登记要求。
- **任何 `DROP TABLE` / `SET SCHEMA` / `RENAME` / `TRUNCATE` 都必须先有 `Docs/RequirementList.md` 条目**；无条目即视为越权变更，评审直接拒绝。
- 新增 schema 的迁移须同时授予应用角色 `USAGE`/`CREATE`（`V200034` 以 `current_user` 动态授权，库名/角色名变更后无需改迁移）。
- 只建 schema 的空迁移沿用「建 schema → 属主/授权 → 追加 `search_path` 末尾 → 收敛校验」四段式模板：`V200034`（`export`/`sche`）、`V200036`（`dedup`，该 schema 已于 `V200086` 删除）、`V200037`（`archive`）、`V200071`（`sys`）。
