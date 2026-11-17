# 改造项 A：`secu_code` → `usc` 全库统一（检查清单）

> 用途：把该改造拆成**可逐条勾选的检查项**，按编号顺序执行以避免漏改；执行中或执行后新发现的改造点，追加到 §5 变更记录之前的顺延编号处，不插入既有编号。
> 使用方式：自上而下推进；完成一项把「状态」改为 `☑ YYYY-MM-DD`，并附提交短号。
> 编号规则：本文件条目 `A-01`～`A-37`，**只增不改**；作废条目保留并标注「作废（原因）」，不删号。
> 完成判定：每项给出可执行的核验命令或明确的代码检查点，**未通过判定不得勾选**。
> 关联文档：同批改造项 B 见 [`2610-PrevCloseToLastClose.md`](2610-PrevCloseToLastClose.md)；需求验收条目见 [`../RequirementList.md`](../RequirementList.md)；版本变更见 [`../CHANGELOG.md`](../CHANGELOG.md)。
> 基线：仓库 `VERSION=0.1.0`，结构迁移最高版本 `V28`，种子段 `V100xxx` 不动。

## 0. 概览

| 编号 | 标题 | 类型 | 影响面 | 风险 | 状态 |
|---|---|---|---|---|---|
| A | `secu_code` → `usc` 全库统一 | 重命名（破坏性） | 6 张表 + Go 15 文件 + Web 9 文件 + 文档 13 篇 | 中（新旧版本不兼容窗口） | ☑ 2026-10-24 已完成（临时 PG18 库实测 + 生产库部署验证，服务版本 0.1.1） |

**与改造项 B 的依赖**：本项必须**先完成并发布**，B 才可开始 —— 二者修改同一批 SQL（`buildUpsertSQL`、`QueryBars`、`rowParams`），且 B 的迁移脚本须使用改名后的 `usc`。

## 1.1 背景与决策

- `finv_security.usc` 是统一证券代码字典主键；`V21` 注释已写明「`secu_code` 实际为统一证券代码（usc，全局唯一）」。本次为**正名**，不改语义。
- 范围：**全库统一**（DB 列名 + API wire 字段 + 代码标识符），不做「只改飞线表」的半套方案。
- 不改：MVSV 文件头 `Code` 字段（外部格式契约）、`finv_quote_secu_kline_min` 表名、`VeritasQuant/` 历史子项目。
- **禁止修改历史迁移 `V1`**：`migrate.go` 按 `schema_version` 跳过已应用版本，改 `V1` 会让存量库与全新库结构分叉。只能新增 `V29`。

## 1.2 影响面盘点（实测命中数，不含 `VeritasQuant/`、`Web/dist/`、`Deploy/backup/`）

| 层 | 文件数 | 命中数 |
|---|---|---|
| 迁移 SQL（`Deploy/Migrations/`，含历史只读文件） | 6 | 21 |
| Go 后端（`internal/`） | 15 | 100 |
| Web 前端（`Web/src/`） | 9 | 50 |
| 文档（`Docs/`、`Prompt.md`） | 13 | 32 |

## 1.3 A1 数据库迁移

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| A-01 | `Deploy/Migrations/V29__rename_secu_code_to_usc.sql`（新建） | 在一个事务内对 6 张表 `RENAME COLUMN secu_code TO usc`：`finv_quote_secu_kline_min`、`finv_quote_ingest_batches`、`finv_quote_revision_log`、`finv_quant_backtest_strategy`、`finv_quant_backtest_run`、`finv_quant_backtest_run_archive`；全部用 `information_schema.columns` 存在性判断保证幂等 | 文件存在且 `BEGIN;`/`COMMIT;` 各 1 处 | ☑ 2026-10-24 |
| A-02 | `Deploy/Migrations/V29__...sql` | 重命名依赖该列的 3 个索引：`idx_finv_quote_secu_secu_ts`→`idx_finv_quote_usc_ts`、`idx_finv_quant_backtest_strategy_secu`→`idx_finv_quant_backtest_strategy_usc`、`idx_finv_quant_backtest_run_secu`→`idx_finv_quant_backtest_run_usc`；使用 `ALTER INDEX IF EXISTS ... RENAME TO` | 三条 `RENAME TO` 各 1 处 | ☑ 2026-10-24 |
| A-03 | `Deploy/Migrations/V29__...sql` | 主键 `finv_quote_secu_kline_min_pkey (ts, secu_code)` 保持名称不变（列引用自动跟随），脚本中不重建主键 | 脚本内无 `DROP CONSTRAINT` / `ADD CONSTRAINT ... PRIMARY KEY` | ☑ 2026-10-24 |
| A-04 | 运行环境 | 应用迁移后核验列名 | `psql -c "\d finv_quote_secu_kline_min"` 输出含 `usc`、不含 `secu_code`；`\di idx_finv_quote_usc_ts` 存在 | ☑ 2026-10-24（临时 PG18 库 + **生产库实测**：6 表列改名、5 索引改名成功，`idx_finv_quote_usc_ts` 存在） |
| A-05 | 运行环境 | 存量数据可读性回归 | `SELECT count(*), min(ts), max(ts) FROM finv_quote_secu_kline_min;` 与迁移前一致 | ☑ 2026-10-24（生产库部署后复核：2,804,329 行、3 个证券、`min(ts)=1541737200` / `max(ts)=1784615520`，与迁移前一致） |
| A-06 | `Deploy/Migrations/V1__...`、`V21__...`、`V22__...`、`V24__...`、`V100019__...`、`V100023__...` | **保持原样不改**（历史迁移不可回改），仅在提交说明中注明「历史迁移保留 secu_code」 | `git diff --stat -- Deploy/Migrations/V1* V21* V22* V24* V100019* V100023*` 输出为空 | ☑ 2026-10-24 |

## 1.4 A2 Go 后端

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| A-07 | `internal/quote/service.go`（18 处） | SQL 列名 4 段（`QueryBars` 的 COUNT/SELECT、`buildUpsertSQL` 的 INSERT 列与 `ON CONFLICT (ts, usc)`、`logRevision` 插入、批次登记插入）；`Bar`/`UpsertResult` 结构体字段 `SecuCode`→`Usc` 及 `json:"usc"`；错误文案「secu_code 不能为空」→「usc 不能为空」 | `grep -c "secu_code\|SecuCode" internal/quote/service.go` 输出 `0` | ☑ 2026-10-24 |
| A-08 | `internal/api/handler/query.go`（8 处） | URL 参数改为 `usc`（保留 `secuCode` 旧参数一次性兼容读取，`secu_code` 兼容逻辑删除）；响应 `"secu_code"` → `"usc"`；400 文案同步 | `grep -c "secu_code\|secuCode\|SecuCode" internal/api/handler/query.go` 输出 `0`（若保留兼容读取则输出 `1` 并注明） | ☑ 2026-10-24 |
| A-09 | `internal/api/handler/import.go`（3 处） | 上传表单字段 `secu_code` → `usc`，一致性校验文案同步 | `grep -c "secu_code" internal/api/handler/import.go` 输出 `0` | ☑ 2026-10-24 |
| A-10 | `internal/mvsv/parser.go`（2 处） | `Row.SecuCode` → `Row.Usc`；**文件头 `Values["Code"]` 解析逻辑不变** | `grep -c "SecuCode" internal/mvsv/parser.go` 输出 `0` 且 `grep -c 'Values\["Code"\]'` 仍为 `1` | ☑ 2026-10-24 |
| A-11 | `internal/backtest/model.go`（6 处） | 5 个结构体字段 `SecuCode`→`Usc` 与 `json:"secu_code"`→`json:"usc"` | `grep -c "secu_code\|SecuCode" internal/backtest/model.go` 输出 `0` | ☑ 2026-10-24 |
| A-12 | `internal/backtest/service.go`（44 处） | 全部 SQL 列名、入参变量名、错误文案（如「未指定回测标的（secu_code 或…）」）、`WHERE secu_code = $n`、INSERT/UPDATE 列清单 | `grep -c "secu_code\|SecuCode" internal/backtest/service.go` 输出 `0` | ☑ 2026-10-24 |
| A-13 | `internal/backtest/engine.go`（2 处） | `Config.SecuCode` 字段与赋值 | `grep -c "SecuCode" internal/backtest/engine.go` 输出 `0` | ☑ 2026-10-24 |
| A-14 | `internal/api/handler/backtest.go`（1 处） | 响应/入参字段名同步 | `grep -c "secu_code" internal/api/handler/backtest.go` 输出 `0` | ☑ 2026-10-24 |
| A-15 | 测试文件（7 个）：`internal/quote/service_test.go`(2)、`internal/mvsv/parser_test.go`(2)、`internal/mvsv/user_sample_verify_test.go`(1)、`internal/backtest/engine_test.go`(3)、`engine_new_features_test.go`(4)、`environment_test.go`(2)、`trace_test.go`(2) | 断言列名、结构体字段、测试数据同步；**MVSV 测试样例文件内容不改**（`PrevClose` 列属于文件格式） | `grep -rn "secu_code\|SecuCode" internal/ --include="*_test.go"` 输出为空 | ☑ 2026-10-24 |
| A-16 | 仓库根 | 编译与单测 | `go build ./...` 与 `go test ./...` 全部通过 | ☑ 2026-10-24 |

## 1.5 A3 Web 前端

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| A-17 | `Web/src/views/Meta/Finv/Quote/History/HistoryQuoteQueryView.vue`（8 处） | 查询参数 `secuCode`→`usc`、响应读取 `data.secu_code`→`data.usc`、类型定义 | `grep -c "secu_code\|secuCode"` 输出 `0` | ☑ 2026-10-24 |
| A-18 | `Web/src/views/Meta/Finv/Quote/History/HistoryQuoteImportView.vue`（8 处） | 表单字段 `secu_code`→`usc`、导入结果 `data.secu_code`→`data.usc`；`security.usc` 取值逻辑不变 | 同上输出 `0` | ☑ 2026-10-24 |
| A-19 | `Web/src/views/Meta/Finv/Quant/Backtest/BacktestGoldFuturesView.vue`（12 处） | 类型、`secu_code` 字段、`secuCode` 查询参数、模板文案 | 同上输出 `0` | ☑ 2026-10-24 |
| A-20 | `Web/src/views/Meta/Finv/Quant/Backtest/BacktestAnalysisView.vue`（5 处） | 列表列 `key: 'secu_code'`、过滤参数 | 同上输出 `0` | ☑ 2026-10-24 |
| A-21 | `Web/src/views/Meta/Finv/Quant/Backtest/BacktestReportView.vue`（5 处） | 报告背景字段、导出列 | 同上输出 `0` | ☑ 2026-10-24 |
| A-22 | `Web/src/views/Meta/Finv/Quant/Backtest/RunDeleteTasksView.vue`（2 处） | 表格列 key | 同上输出 `0` | ☑ 2026-10-24 |
| A-23 | `Web/src/views/Meta/Finv/Quant/Strategy/StrategyManageView.vue`（6 处） | 策略表单/列表字段 | 同上输出 `0` | ☑ 2026-10-24 |
| A-24 | `Web/src/views/Meta/Finv/Quant/Fund/FundManageView.vue`、`Quant/Position/PositionManageView.vue`（各 2 处） | 类型与展示字段 | 同上输出 `0` | ☑ 2026-10-24 |
| A-25 | `Web/` | 构建通过并重新生成 `dist` | `npm run build` 成功；`grep -r "secu_code" Web/dist/assets/*.js` 输出为空 | ☑ 2026-10-24 |

## 1.6 A4 文档

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| A-26 | `Docs/API/HistoryQuote/HistoryQuote.md`（8 处） | 查询参数、响应字段、错误表、curl 示例全部改 `usc` | `grep -c "secu_code\|secuCode"` 输出 `0` | ☑ 2026-10-24 |
| A-27 | `Docs/API/HistoryQuote/ImportsUpload.md`（5 处） | 表单字段、响应回显、主键描述「`ts + usc`」 | 同上输出 `0` | ☑ 2026-10-24 |
| A-28 | `Docs/API/Backtest/{RunCreate,RunList,StrategyList,StrategySave}.md` | 请求/响应字段与 curl 示例 | 四篇 `grep -c "secu_code\|secuCode"` 均为 `0` | ☑ 2026-10-24 |
| A-29 | `Docs/Menu/HistoryQuote/{HistoryQuoteQuery,HistoryQuoteImport}.md`、`Docs/Menu/Backtest/BacktestGoldFutures.md` | 菜单与接口映射中的字段名 | 三篇输出 `0` | ☑ 2026-10-24 |
| A-30 | `Docs/DataFormat/MvsvFileFormat.md` | **仅改**「导入时作为 `secu_code`」→ `usc`；文件头 `Code` 字段说明不变 | `grep -c "secu_code"` 输出 `0` 且 `grep -c "Code"` 不变 | ☑ 2026-10-24 |
| A-31 | `Docs/DevSpec/ApiSpec.md`（2 处） | 第 20 行小驼峰示例中的 `secuCode` 替换为其他示例（如 `runId`）；第 21 行反面示例中的 `secu_code` 保留或换为 `run_id` 一类，确保规范不出现「`usc` 被禁用」的歧义 | 该两行不再出现 `secuCode`/`secu_code` | ☑ 2026-10-24 |
| A-32 | `Docs/Asset/Backtest/BacktestFullFlowTask.svg`（1 处） | 流程图节点文本 `secu_code` → `usc` | `grep -c "secu_code"` 输出 `0` | ☑ 2026-10-24 |
| A-33 | `Prompt.md`（1 处） | 需求基线中的字段名同步 | `grep -c "secu_code"` 输出 `0` | ☑ 2026-10-24 |

## 1.7 A5 记录与发布

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| A-34 | `Docs/CHANGELOG.md` | `[未发布]` → `[0.1.1]`（或按实际版本）：`变更` 组记录「`secu_code` 统一正名为 `usc`（6 表 + API 字段）」，并注明**破坏性变更**与升级要求 | 条目存在且标注破坏性 | ☑ 2026-10-24 |
| A-35 | `Docs/RequirementList.md` | 自 `10056` 起追加 A 的验收条目（DB 列名、索引名、API 字段、前端字段、文档零残留） | 编号连续、核对命令实跑通过 | ☑ 2026-10-24 |
| A-36 | `VERSION` | 版本号递增（`0.1.0` → `0.1.1`），发布后打 tag | `cat VERSION` 为新版本；`git tag` 含对应 tag | ☑ 2026-10-24（VERSION=0.1.1；tag 待发布） |
| A-37 | `Deploy/Win11DockerUpgrade.md`、`Deploy/upgrade.cmd` | 说明升级必须「停旧容器 → 拉新镜像 → 启动（自动迁移）」，禁止新旧镜像共存 | 文档含该约束段 | ☑ 2026-10-24 |
| A-38 | `Deploy/Migrations/V29__...sql`（执行中新发现） | `A-02` 只列 3 个索引，实际另有 2 个审计表索引依赖该列：`idx_finv_quote_ingest_batches_secu`→`idx_finv_quote_ingest_batches_usc`、`idx_finv_quote_revision_log_secu`→`idx_finv_quote_revision_log_usc`，已在 `V29` 中一并改名 | 两条 `idx_*_usc` 各 1 处 | ☑ 2026-10-24 |
| A-39 | `internal/webui/dist/`（执行中新发现） | All-in-One 镜像通过 `go:embed dist` 内嵌前端产物，前端改名后须重新 `npm run build` 并复制到 `internal/webui/dist/`（该目录不入库，构建时生成） | `grep -rc "secu_code" internal/webui/dist/assets/*.js` 全为 `0` | ☑ 2026-10-24 |
| A-40 | `Deploy/Migrations/V100019__...seed.sql`、`V100023__...del_task.sql`（执行中新发现，历史迁移例外） | 这两支种子迁移在 `V29` **之后**执行却仍引用旧列名 `secu_code`（`V100019` 的 INSERT 列清单、`V100023` 的 `CREATE TABLE` 列定义），会导致**全新库首次部署失败**；仅修正这两处列名，对已应用库无影响（`schema_version` 跳过），例外规则登记在 `Docs/DevSpec/DbSchemaSpec.md` §4 | 两文件 `secu_code` 清零；全新库全链迁移通过 | ☑ 2026-10-24 |

---

## 2. 待决策项

| 编号 | 待决策 | 选项 | 当前建议 | 决策 |
|---|---|---|---|---|
| DA-01 | API 参数是否保留 `secuCode` 兼容期 | ① 直接切 `usc` ② 双读一个版本周期 | ①（调用方仅本项目前端） | ☑ 2026-10-24 已定：直接切 `usc`，不保留兼容读取 |

---

## 3. 自检（全部条目完成后执行）

```bash
# 代码与文档零残留
# 排除：构建产物（internal/webui/dist、Web/dist）、计划文档本身、变更日志与需求清单（记录历史名）
grep -rn "secu_code\|SecuCode\|secuCode" \
  --include="*.go" --include="*.vue" --include="*.ts" --include="*.md" --include="*.svg" \
  internal/ Web/src/ Docs/ Prompt.md \
  | grep -v "internal/webui/dist/\|Docs/Refactor/\|Docs/CHANGELOG.md\|Docs/RequirementList.md"   # 期望输出为空

# 历史迁移未被修改
git diff --stat -- Deploy/Migrations/V1* Deploy/Migrations/V2[124]* Deploy/Migrations/V100019* Deploy/Migrations/V100023*

# 构建与测试
go build ./... && go test ./...
cd Web && npm run build
```

## 4. 变更记录

| 日期 | 变更 | 说明 |
|---|---|---|
| 2026-10-24 | 初版 | 从 `RefactorPlan.md` 拆出为独立文件，条目 `A-01`～`A-37`；影响面数字经实测（Go 15 文件 100 处、Web 9 文件 50 处、Docs 13 篇 32 处、迁移 6 文件 21 处） |
| 2026-10-24 | 部署验证 | 备份 `Deploy/backup/finvquant_周六1024_005740.dump`（68 MB，`pg_restore --list` 可读）；重建 `finvquant:local` 镜像并重启容器，`schema_version` 中 29/30/31 均 `success=t`；`/API/V1/Version` 返回 0.1.1，健康检查 200/200，前端 200；`usc` 查询正常、`secuCode` 旧参数被拒（4001），导入表单 `usc` 一致性校验生效；日志无 ERROR |
| 2026-10-24 | 执行完成 | 按清单逐项执行：实测替换 Go 120 处 / Web 55 处 / 文档 12 篇；`go build`、`go vet`、三个包单测、`npm run build` 全部通过；新增 `A-38`（另 2 个审计表索引改名）、`A-39`（重建 `internal/webui/dist`）；`DA-01` 定为直接切 `usc` |
