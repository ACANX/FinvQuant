# 改造项 C：`finv_quote_*` → `quote` schema、`finv_quant_*` → `quant` schema（检查清单）

> 用途：把按业务域分表的工作拆成**可逐条勾选的检查项**，按编号顺序执行以避免漏改；执行中或执行后新发现的改造点，追加到末尾（编号顺延），不插入既有编号。
> 使用方式：自上而下推进；完成一项把「状态」改为 `☑ YYYY-MM-DD`，并附提交短号。
> 编号规则：本文件条目 `C-01`～`C-xx`，**只增不改**；作废条目保留并标注「作废（原因）」，不删号。
> 完成判定：每项给出可执行的核验命令或明确的代码检查点，**未通过判定不得勾选**。
> 关联文档：改造项 A [`2610-SecuCodeToUsc.md`](2610-SecuCodeToUsc.md)、改造项 B [`2610-PrevCloseToLastClose.md`](2610-PrevCloseToLastClose.md)；需求验收条目见 [`../RequirementList.md`](../RequirementList.md)；版本变更见 [`../CHANGELOG.md`](../CHANGELOG.md)。
> 基线：仓库 `VERSION=0.1.1`，结构迁移最高版本 `V31`，种子段 `V100xxx` 不动。

## 0. 概览

| 编号 | 标题 | 类型 | 影响面 | 风险 | 状态 |
|---|---|---|---|---|---|
| C | 按业务域把表迁入独立 schema | 结构重组（破坏性） | 16 张表 + Go 104 处引用 + 文档 16 篇 63 处 | 中（search_path 与迁移顺序耦合） | ☐ 未开始 |

**执行顺序**：在 A、B 之后执行（A/B 已发布 `0.1.1`）。C 只调整对象所属 schema，**不改列、不改数据**。

## 1.1 背景与要求

- 需求：`finv_quote` 开头的表迁入 `quote` schema，`finv_quant` 开头的表迁入 `quant` schema，实现「业务域 = schema」的分包，避免所有对象堆在 `public`。
- 不做：字典/基础数据表（`finv_security`、`finv_market`、`finv_exchange`、`finv_region`、`finv_currency`、`finv_futu_mapping_*`）与 `schema_version` 保持 `public`，本次不动（见 `D-03`）。
- 不改表名：本次只改 schema，表名保留 `finv_quote_*` / `finv_quant_*` 前缀（见 `D-01`）。
- 不改列、不改数据、不改索引名：`ALTER TABLE ... SET SCHEMA` 为 catalog 级操作，索引/约束/identity 序列/触发器自动跟随，不扫描数据、不重写表。

## 1.2 现状盘点（生产库实测）

| 类别 | 数量 | 明细 |
|---|---|---|
| 迁入 `quote` | 3 张 | `finv_quote_secu_kline_min`、`finv_quote_ingest_batches`、`finv_quote_revision_log` |
| 迁入 `quant` | 13 张 | `finv_quant_backtest_{account,cashflow,del_log,del_task,equity,event_trace,position_log,run,run_archive,strategy,trade}`（11）、`finv_quant_environment`、`finv_quant_template` |
| 保留 `public` | 10 张 | `finv_currency`、`finv_exchange`、`finv_futu_mapping_{cs_market,exchange,market_code,region,security}`、`finv_market`、`finv_region`、`finv_security` |
| 系统表 | 1 张 | `schema_version`（迁移版本记录，留在 `public`） |
| 触发器 | 16 个 | 其中 6 个位于将移动的表（`finv_quote_secu_kline_min`、`finv_quant_backtest_{account,run,strategy}`、`finv_quant_environment`、`finv_quant_template`）；触发函数 `vq_set_gmt_update()` 在 `public`，按 OID 绑定，随表移动无影响 |
| identity 序列 | 随表 | 全部为 `GENERATED ALWAYS AS IDENTITY`，`SET SCHEMA` 自动跟随，无需单独处理 |
| 物理外键 | 0 | 引用关系全部由程序层维护，无跨 schema 外键需要重建 |

**代码与文档引用面（实测）**

| 层 | 文件数 | 命中数 |
|---|---|---|
| Go 后端（`internal/`，表名出现在 SQL 与注释） | 6 | 104（`finv_quote_*` 11、`finv_quant_*` 93） |
| 文档（`Docs/`、`Prompt.md`） | 16 | 63 |
| 历史迁移（`Deploy/Migrations/`，**只读不改**） | 18 | — |

## 1.3 关键技术约束（本改造真正的风险点）

1. **迁移顺序与种子段耦合**：迁移按版本号排序执行，`V32`（移动表）排在 `V100001`～`V100023`（种子数据）之前。种子迁移是历史文件、**不可回改**，其 SQL 写的是不带 schema 的表名（如 `INSERT INTO finv_quote_revision_log ...`）。因此 **`V32` 执行后，连接必须能通过 `search_path` 解析到 `quote` / `quant`**，否则全新库首次部署会在种子阶段失败。
2. **`ALTER DATABASE ... SET search_path` 只对新连接生效**：迁移用的连接池（`migrate.go`）在 `V32` 之前就已建立连接，因此**单靠 `V32` 无法让种子迁移生效**。必须同时让**连接串本身**携带 `search_path`（见 `C-09`），这样 pool 中每条连接从建立起就带正确的搜索路径；`quote` / `quant` 尚未创建时，PostgreSQL 会静默忽略不存在的 schema，不影响 `V1`～`V31`。
3. **应用 SQL 显式 schema 前缀**：`Go` 侧 SQL 统一写 `quote.xxx` / `quant.xxx`（见 `C-10`～`C-12`），不依赖 `search_path`；`search_path` 仅作为历史种子迁移与手工 `psql` 的兜底。
4. **幂等**：`V32` 用 `pg_tables.schemaname` 判断表当前所在 schema，已迁移则跳过；重复执行安全。

## 1.4 C1 迁移脚本

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| C-01 | `Deploy/Migrations/V32__move_finv_tables_to_quote_quant_schemas.sql`（新建） | 单事务：`CREATE SCHEMA IF NOT EXISTS quote;`、`CREATE SCHEMA IF NOT EXISTS quant;` | 文件存在，含两条 `CREATE SCHEMA IF NOT EXISTS` | ☐ |
| C-02 | 同上 | 3 张 `finv_quote_*` 表 `ALTER TABLE public.<t> SET SCHEMA quote;`，逐表用 `pg_tables` 判断当前 schema 保证幂等 | 脚本内 3 个表名齐全且含 `schemaname = 'public'` 判断 | ☐ |
| C-03 | 同上 | 13 张 `finv_quant_*` 表 `SET SCHEMA quant`，同样幂等判断 | 脚本内 13 个表名齐全 | ☐ |
| C-04 | 同上 | `ALTER DATABASE <当前库> SET search_path TO public, quote, quant;`（用 `current_database()` 动态取库名，避免写死） | 脚本含 `ALTER DATABASE` 与 `current_database()` | ☐ |
| C-05 | 同上 | 收敛校验：迁移末尾用 `DO` 块断言「应迁的表已不在 `public`」，不满足则 `RAISE EXCEPTION` 使整事务回滚 | 脚本含 `RAISE EXCEPTION` | ☐ |
| C-06 | 同上 | 不重建索引/触发器/序列，不 `DROP`/`CREATE` 任何表 | 脚本内无 `CREATE INDEX`、`DROP TABLE`、`CREATE TABLE` | ☐ |
| C-07 | `Deploy/Migrations/V1*`～`V31`、`V100xxx` | **历史迁移保持原样**（不可回改） | `git diff --stat -- Deploy/Migrations/` 中不包含既有文件 | ☐ |
| C-08 | 临时 PG18 库 | 全链验证：`V1`～`V31` → `V32` → `V100001`～`V100023` 全部成功（重点：种子迁移在 `V32` 之后仍能写入） | 迁移链无错误，`schema_version` 含 `32` | ☐ |

## 1.5 C2 应用连接与 SQL

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| C-09 | `internal/database/postgres.go` | DSN 追加 `search_path=public,quote,quant`（pgx runtime param），保证连接池每条连接自带搜索路径，兼容历史种子迁移与未加前缀的 SQL | `grep -c "search_path=public,quote,quant" internal/database/postgres.go` 输出 `1` | ☐ |
| C-10 | `internal/quote/service.go`（9 处） | 表名加 schema 前缀：`quote.finv_quote_secu_kline_min`、`quote.finv_quote_revision_log`、`quote.finv_quote_ingest_batches`（含 `ON CONFLICT`/`COALESCE` 中的表名引用） | 该文件内 `finv_quote_` 前均带 `quote.` 或位于注释 | ☐ |
| C-11 | `internal/backtest/service.go`（93 处） | `quant.finv_quant_*` 全量前缀；其中 `FROM finv_quote_secu_kline_min` 改为 `FROM quote.finv_quote_secu_kline_min` | `grep -c "quant\.finv_quant_" internal/backtest/service.go` 与表名出现次数一致 | ☐ |
| C-12 | `internal/mvsv/parser.go`（1 处注释） | 注释中的表名同步为 `quote.finv_quote_secu_kline_min` | 注释已更新 | ☐ |
| C-13 | `internal/quote/service_test.go`、`internal/backtest/run_delete_test.go` | 断言中的表名同步（SQL 字符串断言、表名列表） | `go test ./...` 通过 | ☐ |
| C-14 | 仓库根 | 编译与单测 | `go build ./...`、`go vet ./...`、`go test ./...` 全部通过 | ☐ |

## 1.6 C3 兼容与回归

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| C-15 | 生产库 | 迁移前备份（`pg_dump -Fc`），回滚唯一依据 | 备份文件存在且 `pg_restore --list` 可读 | ☐ |
| C-16 | 生产库 | 迁移后数据完整性回归：行数与时间范围与迁移前一致 | `SELECT count(*), min(ts), max(ts) FROM quote.finv_quote_secu_kline_min;` 与迁移前一致 | ☐ |
| C-17 | 生产库 | 触发器仍生效：更新一行后 `gmt_update` 自动刷新 | `UPDATE quote.finv_quote_secu_kline_min SET remark = remark WHERE ts = (SELECT min(ts) ...)` 后 `gmt_update > gmt_create` | ☐ |

## 1.7 C4 文档

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| C-18 | `Docs/API/HistoryQuote/{HistoryQuote,ImportsUpload}.md` | 表名加 `quote.` 前缀 | `grep -c "quote.finv_quote_"` 不小于 1 | ☐ |
| C-19 | `Docs/API/Backtest/{RunCreate,StrategySave}.md`、`Docs/Menu/Backtest/*.md`（5 篇） | 表名加 `quant.` 前缀 | 各文件旧写法清零（无未加前缀的 `finv_quant_`） | ☐ |
| C-20 | `Docs/DataFormat/MvsvFileFormat.md`、`Docs/Menu/HistoryQuote/HistoryQuoteImport.md` | 落库表名加 `quote.` 前缀 | 同上 | ☐ |
| C-21 | `Docs/DevSpec/BacktestStrategySpec.md` | 第 13 章表结构说明中的表名加 `quant.` 前缀 | 同上 | ☐ |
| C-22 | `Prompt.md` | 技术基线段中的表名同步 | 同上 | ☐ |
| C-23 | `Docs/DevSpec/DocSpec.md` 或 `FileNamingSpec.md` | 补充「数据库对象命名：按域分 schema，表名保留业务前缀」约定 | 规范含该条款 | ☐ |

## 1.8 C5 记录与发布

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| C-24 | `Docs/CHANGELOG.md` | `[未发布]` 记录本改造：16 张表迁入 `quote`/`quant` schema，标注**破坏性变更**（旧镜像 SQL 不带 schema 前缀，依赖 `search_path` 才可用；手工 `psql` 需带 schema 或设置 `search_path`） | 条目存在且标注破坏性 | ☐ |
| C-25 | `Docs/RequirementList.md` | 自 `10092` 起追加 C 的验收条目（schema 存在、表位置、search_path、文档零残留） | 编号连续、核对命令实跑通过 | ☐ |
| C-26 | `Deploy/Win11DockerUpgrade.md` | 破坏性变更约束段补充：升级到含 `V32` 的版本时同样禁止新旧镜像共存；回滚需恢复备份 | 文档含该说明 | ☐ |

## 1.9 C6 部署验证

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| C-27 | 运行环境 | 重建镜像并启动，迁移自动应用 `V32` | `SELECT version, success FROM schema_version WHERE version='32';` 为 `t` | ☐ |
| C-28 | 运行环境 | 服务与接口回归 | `/API/V1/Version` 正常、`/Health/Ready` 200、`QuoteQuery?usc=GCMain` 返回 bars、回测 `Run/List` 正常 | ☐ |
| C-29 | 运行环境 | `search_path` 生效确认 | `SHOW search_path;` 含 `public, quote, quant` | ☐ |
| C-30 | 运行环境 | 容器日志无 ERROR | `docker logs finvquant 2>&1 \| grep -ciE "ERROR\|失败\|panic"` 为 `0` | ☐ |

---

## 2. 待决策项

| 编号 | 待决策 | 选项 | 当前建议 | 决策 |
|---|---|---|---|---|
| D-01 | 表名是否去掉冗余前缀（`quote.finv_quote_secu_kline_min` → `quote.secu_kline_min`） | ① 保留前缀（本次只改 schema）② 同时去前缀（表名更简洁，但需全量改 SQL 与文档，且属破坏性改名） | ①：本次只改 schema；去前缀另立清单，待 schema 稳定后再议 | ☐ 待定 |
| D-02 | 应用 SQL 是否显式加 schema 前缀 | ① 显式前缀 + `search_path` 兜底 ② 仅依赖 `search_path`（SQL 不动，改动面最小） | ①：显式前缀最稳，`search_path` 只兜底历史种子迁移与手工操作 | ☐ 待定 |
| D-03 | 其余 `finv_*` 字典表是否也迁入 `meta` schema | ① 本次不动 ② 一并迁入 | ①：避免范围膨胀；字典表属基础主数据，后续可单独评估 | ☐ 待定 |

---

## 3. 自检（全部条目完成后执行）

```bash
# 迁移脚本结构
grep -c "CREATE SCHEMA IF NOT EXISTS" Deploy/Migrations/V32__move_finv_tables_to_quote_quant_schemas.sql   # 期望 2
grep -c "SET SCHEMA" Deploy/Migrations/V32__move_finv_tables_to_quote_quant_schemas.sql                    # 期望 16

# 代码中的表名均带 schema 前缀（排除纯注释行较难，人工复核注释即可）
grep -rn "FROM finv_quote_\|INTO finv_quote_\|FROM finv_quant_\|INTO finv_quant_\|UPDATE finv_quant_\|UPDATE finv_quote_" internal/ --include="*.go"   # 期望为空

# 文档零残留（计划文档与变更日志除外）
grep -rn "FROM finv_quote_\|FROM finv_quant_" Docs/ Prompt.md | grep -v "Docs/Refactor/\|Docs/CHANGELOG.md"   # 期望为空

# 历史迁移未被修改
git diff --stat -- Deploy/Migrations/

# 构建与测试
go build ./... && go vet ./... && go test ./...
```

## 4. 变更记录

| 日期 | 变更 | 说明 |
|---|---|---|
| 2026-10-24 | 初版 | 建立改造项 C 检查清单（`C-01`～`C-30`）；盘点生产库 27 张表的归属（quote 3 / quant 13 / public 10 + `schema_version`）；识别关键约束：`V32` 早于种子段执行，必须靠连接串 `search_path` 才能让历史种子迁移写入新 schema |
