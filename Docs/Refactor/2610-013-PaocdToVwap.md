# 改造项：`paocd` / `pacd` → `vwap` 字段统一（检查清单）

> 用途：把本次改名拆成**可逐条勾选**的检查项，按编号顺序执行；执行中新发现的改造点追加到 `C-30` 之后顺延编号，不插入既有编号。
> 使用方式：自上而下推进；完成一项把「状态」改为 `☑ YYYY-MM-DD 提交短号`。**未通过完成判定不得勾选**。
> 关联文档：需求条目 [`../RequirementList.md`](../RequirementList.md) 10902（本次改名）/ 10903（移除兼容的待办）；版本变更 [`../CHANGELOG.md`](../CHANGELOG.md)；同类先例 [`2610-002-PrevCloseToLastClose.md`](2610-002-PrevCloseToLastClose.md)。
> 基线：迁移号 `V300087`（当前最大号 `V300086`）；改名涉及 3 张表、15 个 Go 文件、11 篇文档。

## 0. 概览

| 编号 | 标题 | 类型 | 影响面 | 风险 | 状态 |
|---|---|---|---|---|---|
| C | `paocd` / `pacd` → `vwap` | 重命名（4 个名字统一） | 3 张表 + 15 个 Go 文件 + 11 篇文档 | 低（列改名零语义变化；API 出参属破坏性，前端当前无引用） | ☑ 2026-11-02 完成（本机迁移 + API + MVSV 实测） |

## 1. 命名映射（最终态）

| 层次 | 旧 | 新 | 备注 |
|---|---|---|---|
| 数据库列 | `paocd` | `vwap` | 3 张表 `RENAME COLUMN` |
| API 出参 JSON | `pacd` | `vwap` | 不再输出 `pacd` |
| API 入参 JSON | `pacd` | `vwap` 优先 + `pacd` 兼容 | 兼容项见需求 10903 |
| `/Page`、`/PageUni` 短名 | `pa` | `vwa` | `column` 固定 15 项、顺序不变 |
| MVSV-2 导出列名 | `paocd` / `Paocd` | `vwap` / `Vwap` | `Field` / `FieldName` / 中文名 |
| MVSV 导入 | `paocd`、`pacd` | `paocd`、`pacd`、`vwap` | 旧文件可继续导入 |

## 2. 数据库（DB）

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| C-01 | `Deploy/Migrations/V300087__finv_quote_kline_rename_paocd_to_vwap.sql`（新建） | 对 `quote.finv_quote_secu_kline_min`、`cdeu.finv_quote_secu_kline_min`、`cdeu.finv_quote_secu_kline_min_export` 执行 `RENAME COLUMN paocd TO vwap`；幂等（已是 `vwap` 或表不存在则跳过）+ 收敛校验 | 迁移在 `schema_version` 为 `success` | ☑ 2026-11-02 `5f9d5e5` |
| C-02 | 同上 | 列注释改为「成交量加权平均价（VWAP；原名 paocd / 当日实时累计均价）」 | `col_description` 显示新注释 | ☑ 2026-11-02 `5f9d5e5` |
| C-03 | 运行环境 | 确认无索引 / 视图 / 物化视图 / 触发器 / 函数依赖该列 | `pg_indexes`、`pg_views`、`pg_proc` 查询 `paocd` 为空 | ☑ 2026-11-02 `5f9d5e5`（三类依赖均 0 条） |
| C-04 | 运行环境 | 列名核验 | `information_schema.columns` 中 3 张表均为 `vwap`、无 `paocd` | ☑ 2026-11-02 `5f9d5e5` |
| C-05 | 历史迁移 | `V200001` / `V200039` / `V200076` / `V200078` / `V200088` / `V300039` 里的 `paocd` 字样**保持不动** | `git diff` 中这些文件无改动 | ☑ 2026-11-02 `5f9d5e5` |

## 3. 后端代码

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| C-10 | `internal/meta/finv/quote/cdeu/kline/model.go` | `ColPaocd` → `ColVwap="vwap"`；`Row.Pacd` → `Row.Vwap`；`ListItem.Pa` → `Vwa`（`json:"vwa"`）；`CompactFieldMap` 短名 `vwa`；`MarshalJSON` 输出 `vwap` | `go build ./...` 通过 | ☑ 2026-11-02 `5f9d5e5`/`c96b928` |
| C-11 | `kline/sql.go`、`kline/request.go`、`kline/service.go`、`cdeu/store.go` | DDL 类型键、插入列映射、扫描顺序、6 段 SQL 列名 | 同上 | ☑ 2026-11-02 `5f9d5e5` |
| C-12 | `kline/request.go` | 入参 `Vwap json:"vwap"` 优先；`Pacd json:"pacd"` 仅在 `vwap` 未传或 `null` 时兜底；`ToPatchItem` 同步判定 | 兼容测试通过 | ☑ 2026-11-02 `c96b928` |
| C-13 | `internal/meta/finv/quote/service.go`、`cmd/qiom/export.go` | upsert SQL、导出 SQL 与扫描列 | `go build` 通过 | ☑ 2026-11-02 `5f9d5e5`/`2256a3d` |
| C-14 | `mvsv/parser.go`、`parser_v2.go`、`writer_v2.go`、`cmd/qiom/import.go` | `Row.Paocd` → `Row.Vwap`；导入 `case "paocd","pacd","vwap"`；导出列名/FieldName/中文名/类型改 `vwap` | 兼容测试通过 | ☑ 2026-11-02 `2256a3d` |
| C-15 | 测试 | 更新既有断言；新增 `kline/vwap_compat_test.go`（7 例）与 `mvsv/vwap_compat_test.go`（3 例） | `go test ./internal/meta/finv/quote/...` 通过 | ☑ 2026-11-02 `c96b928`/`2256a3d` |

## 4. 接口与文件格式

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| C-20 | `/Add`、`/Query` | 响应字段 `pacd` → `vwap`，不再输出 `pacd` | 响应体含 `vwap`、不含 `pacd` | ☑ 2026-11-02 `c96b928`（本机实测） |
| C-21 | `/Page`、`/PageUni` | 第 13 项短名 `pa` → `vwa` | `column[12] == "vwa"` 且值为 `vwap` 数值 | ☑ 2026-11-02 `c96b928`（实测 `['…','cr','cp','vwa','pv','ext']`） |
| C-22 | `/Upsert`、`/Batch` | 入参 `vwap` 优先、`pacd` 兼容 | `pacd=3.56` 落库 `vwap=3.56`；`vwap=9.99` 与 `pacd=3.56` 同传取 `9.99` | ☑ 2026-11-02 `c96b928`（实测） |
| C-23 | MVSV-2 导出 | `# Field`/`# FieldName`/`# 字段名称`/`# FieldType` 第 14 列改 `vwap` | 导出文件头含 `vwap`、不含 `paocd` | ☑ 2026-11-02 `2256a3d` |
| C-24 | MVSV / qiom 导入 | 列名兼容 `paocd`、`pacd`、`vwap` | 三名各构造文件均可导入为 `vwap` | ☑ 2026-11-02 `2256a3d` |

## 5. 文档

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| C-30 | `Docs/API/Quote/Cdeu/Min/{KLineAdd,KLinePage,KLinePageUni,KLineQuery,KLineUpsert}.md` + `KLineUpsert.yml` | 字段表、示例 JSON、紧凑字段映射 | 无残留旧字段（除兼容说明） | ☑ 2026-11-02 |
| C-31 | `Docs/OpenAPI/Quote/Cdeu/Min/{PageUni,Upsert}.md` + `Upsert.yml` | 同上（OpenAPI 目录双份） | 同上 | ☑ 2026-11-02 |
| C-32 | `Docs/DataFormat/MvsvFileFormat.md`、`Docs/Design/QiomDesign.md` | MVSV-2 文件头示例与列清单 | 示例第 14 列为 `vwap` / `Vwap` | ☑ 2026-11-02 |
| C-33 | `Docs/FinvQuant/Brs.md` | 恢复章节补「旧备份恢复后需手工改名」提示 | 文档含 `RENAME COLUMN paocd TO vwap` 说明 | ☑ 2026-11-02 |
| C-34 | `Deploy/Upgrade/2610-004-MigratePublicKlineToCdeu.sql` | 头部补列名提示（改名后须把 `paocd` 换成 `vwap`） | 头部含「列名提示」段 | ☑ 2026-11-02 |

## 6. 不改动清单（历史记录，保持原样）

| 位置 | 原因 |
|---|---|
| `Docs/RequirementList.md` 10902 之前的历史条目、`Docs/CHANGELOG.md` 历史段落 | 历史事实，不回改 |
| `Docs/Refactor/2610-010-CdeuKlineDumpImport.md`、`Docs/FinvQuant/MigratePublicKlineToCdeu.md` | 描述当时（改名之前）的搬迁过程 |
| `Deploy/Migrations/V200001`/`V200039`/`V200076`/`V200078`/`V200088`/`V300039` | 历史迁移不可回改（`DbSchemaSpec` §4） |
| `Archive/VeritasQuant/**` | 归档子项目 |
| `Web/**` | 前端无该字段引用（grep 0 处） |

## 7. 后续待办

| 编号 | 事项 | 触发条件 |
|---|---|---|
| C-40 | 移除 `pacd` 入参兼容（`kline/request.go` 的 `Pacd` 字段、`mvsv` 的 `pacd` 分支、`qiom import` 的 `pacd` 分支） | 前端与外部调用方全部切到 `vwap` 后，见需求 10903 |
| C-41 | 旧 `.dump` 恢复后的列名校正若反复发生，可在恢复链内置改名步骤 | `Docs/FinvQuant/Brs.md` 的手工提示不再够用时 |
