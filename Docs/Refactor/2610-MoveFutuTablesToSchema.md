# 改造项 D：`finv_futu_*` → `futu` schema（检查清单）

> 用途：把富途（futu）映射表迁入独立 schema 的工作拆成**可逐条勾选的检查项**，按编号顺序执行以避免漏改；执行中新发现的改造点追加到末尾（编号顺延），不插入既有编号。
> 使用方式：自上而下推进；完成一项把「状态」改为 `☑ YYYY-MM-DD`，并附提交短号。
> 编号规则：本文件条目 `D-01`～`D-xx`，**只增不改**；待决策用 `DD-01`～（双字母前缀，与改造项 C 文档内的 `D-01`～`D-03` 区分）。
> 完成判定：每项给出可执行的核验命令或明确的代码检查点，**未通过判定不得勾选**。
> 关联文档：改造项 C [`2610-MoveTablesToSchemas.md`](2610-MoveTablesToSchemas.md)（quote/quant 分 schema，本项为其补充遗漏）；需求验收条目见 [`../RequirementList.md`](../RequirementList.md)；版本变更见 [`../CHANGELOG.md`](../CHANGELOG.md)。
> 基线：仓库 `VERSION=0.1.1`，结构迁移最高版本 `V32`，种子段 `V100xxx`。

## 0. 概览

| 编号 | 标题 | 类型 | 影响面 | 风险 | 状态 |
|---|---|---|---|---|---|
| D | `finv_futu_*` → `futu` schema | 结构重组（破坏性） | 5 张表 + 10 个索引 + 5 个触发器 + 文档 9 篇 | 低（Go 代码零引用，仅需 search_path 与文档同步） | ☐ 未开始 |

**与改造项 C 的关系**：C 已把 `finv_quote_*`/`finv_quant_*` 迁入 `quote`/`quant`，本项补齐遗漏的富途映射表，规则与 C 一致（只改 schema、不改表名/列/数据）。

## 1.1 背景与要求

- 需求：`finv_futu` 开头的表迁入 `futu` schema，与 `quote` / `quant` 一起形成「业务域 = schema」的划分。
- 不做：`finv_security`、`finv_market`、`finv_exchange`、`finv_region`、`finv_currency` 等基础主数据表仍留 `public`（见 `DD-03`）。
- 不改表名：保留 `finv_futu_mapping_*` 前缀（与 C 的 `DD` 决策一致）。
- 富途映射表是「富途 OpenAPI 编码 ↔ Finv 内部编码」的对照字典，属基础主数据中的**外部映射**子域；单独 schema 便于区分「本系统自有主数据」与「外部源对接映射」。

## 1.2 现状盘点（生产库实测）

| 表 | 行数 | 索引 | 触发器 |
|---|---|---|---|
| `finv_futu_mapping_cs_market` | 29 | 3 | 1 |
| `finv_futu_mapping_exchange` | 30 | 2 | 1 |
| `finv_futu_mapping_market_code` | 54 | 2 | 1 |
| `finv_futu_mapping_region` | 25 | 2 | 1 |
| `finv_futu_mapping_security` | 503 | 3 | 1 |
| **合计** | **641** | **10** | **5** |

- 物理外键：**0**（引用关系由程序层维护）。
- 视图/函数引用：**0**（`pg_views.definition` 无匹配）。
- **Go 代码引用：0**（`grep -rn "finv_futu" internal/ --include="*.go"` 无命中）——本项**不需要改后端代码**，只需迁移 + `search_path` + 文档。
- 建表迁移：`V7`、`V8`、`V9`、`V13`、`V14`；补列/改序：`V10`、`V15`、`V16`、`V17`（均早于 `V33`，先建在 `public` 再由 `V33` 迁走，**历史文件保持原样**）。
- 种子迁移：`V100003`、`V100005`、`V100006`、`V100007`、`V100008`、`V100009`、`V100011`、`V100013`、`V100014`（**只有 INSERT/UPDATE，无 `CREATE TABLE`**）；其中 `V100009` 会 **读取** `finv_futu_mapping_security` 来补充 `finv_security`。

## 1.3 关键技术约束

1. **迁移顺序**：`V33` 排在种子段 `V100003+` **之前**。种子迁移对富途表只做 `INSERT`/`UPDATE`/`DELETE`，不带 schema 前缀，**必须靠连接串的 `search_path` 解析到 `futu`**，否则全新库首次部署会在 `V100003` 失败。
2. **与 `V100009` 的读写依赖**：该种子迁移从 `finv_futu_mapping_security` 读取数据写入 `finv_security`（`public`）。`search_path` 同时包含 `public` 与 `futu` 即可同时解析两侧表名（顺序为 `public, quote, quant, futu`，不冲突）。
3. **无 `CREATE TABLE` 风险**：与改造项 C 的 `V100023` 不同，富途表的建表全部在 `V7`～`V17`（早于 `V33`），因此**不需要**额外的收尾迁移。
4. **幂等**：`V33` 用 `pg_tables.schemaname` 判断，已迁出则跳过；末尾断言 `public` 无残留。

## 1.4 D1 迁移脚本

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| D-01 | `Deploy/Migrations/V33__move_finv_futu_tables_to_futu_schema.sql`（新建） | 单事务：`CREATE SCHEMA IF NOT EXISTS futu;` | 文件存在且含该语句 | ☐ |
| D-02 | 同上 | 5 张表 `ALTER TABLE public.<t> SET SCHEMA futu`（`cs_market` / `exchange` / `market_code` / `region` / `security`），逐表 `pg_tables` 判断保证幂等 | 脚本内 5 个表名齐全且含 `schemaname = 'public'` 判断 | ☐ |
| D-03 | 同上 | `ALTER DATABASE <当前库> SET search_path TO public, quote, quant, futu;`（`current_database()` 动态取库名） | 脚本含 `ALTER DATABASE` 与 `current_database()` | ☐ |
| D-04 | 同上 | 收敛校验：`public` 中不得残留 `finv_futu_*`，且 `futu` 中 5 张表齐全，否则 `RAISE EXCEPTION` 整体回滚 | 脚本含两条 `RAISE EXCEPTION` | ☐ |
| D-05 | 同上 | 不重建索引/触发器，不 `DROP`/`CREATE` 表 | 脚本内无 `CREATE INDEX`、`DROP TABLE`、`CREATE TABLE` | ☐ |
| D-06 | `Deploy/Migrations/V7*`～`V17*`、`V100003`～`V100015` | **历史迁移保持原样**（建表在 `V33` 之前，种子只做 DML，均无需修改） | `git diff --stat -- Deploy/Migrations/` 不含这些文件 | ☐ |
| D-07 | 临时 PG18 库 | 全链验证：`V1`→`V33`→`V100000`→`V100024` 全部成功（重点 `V100003`/`V100009`） | 迁移链无错误 | ☐ |

## 1.5 D2 连接与 search_path

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| D-08 | `internal/database/postgres.go` | DSN 的 `search_path` 追加 `futu`：`public,quote,quant,futu` | `grep -c "search_path=public,quote,quant,futu"` 输出 `1` | ☐ |
| D-09 | `internal/` 全量 | 确认无 Go 代码需要加 `futu.` 前缀（实测 0 引用），如后续新增引用须显式限定 | `grep -rn "finv_futu" internal/ --include="*.go"` 输出为空 | ☐ |
| D-10 | `Docs/DevSpec/DbSchemaSpec.md` | schema 划分表新增 `futu` 行；`search_path` 说明更新为四段 | 文档含 `futu` 与新的 `search_path` | ☐ |
| D-11 | 仓库根 | 编译与单测 | `go build ./...`、`go vet ./...`、`go test ./...` 通过 | ☐ |

## 1.6 D3 兼容与回归

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| D-12 | 生产库 | 迁移前备份（`pg_dump -Fc`） | 备份文件存在且 `pg_restore --list` 可读 | ☐ |
| D-13 | 生产库 | 迁移后行数回归：5 张表行数与迁移前一致（29/30/54/25/503） | 查询结果与基线一致 | ☐ |
| D-14 | 生产库 | 触发器仍生效：更新一行后 `gmt_update` 刷新 | `gmt_update > gmt_create` | ☐ |
| D-15 | 生产库 | 字典页数据可读：证券字典查询接口正常返回（依赖 `finv_security`，不受影响） | `Metadata/Security/List` 返回数据 | ☐ |

## 1.7 D4 文档

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| D-16 | `Docs/DataDictMapping/FinvFutuMapping{CSMarket,Exchange,MarketCode,Region,Security}.md`（5 篇） | 表名加 `futu.` 前缀（数据表说明、索引说明） | 各文件旧写法清零 | ☐ |
| D-17 | `Docs/DataDictMapping/{FinvSecurity,FinvMarket,FinvExchange}.md`（3 篇） | 引用富途映射表处加 `futu.` 前缀 | 同上 | ☐ |
| D-18 | `Docs/DevSpec/DbSchemaSpec.md` | 见 `D-10`（schema 表与 search_path） | 同上 | ☐ |
| D-19 | `Docs/DevSpec/DbSchemaSpec.md` | §4 补充说明：本项无历史迁移例外（建表早于 `V33`） | 文档含该说明 | ☐ |
| D-20 | `README.md` | 改造清单列表补本文件入口 | 链接存在 | ☐ |

## 1.8 D5 记录与发布

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| D-21 | `Docs/CHANGELOG.md` | `[未发布]` 记录：5 张富途映射表迁入 `futu` schema，`search_path` 增加 `futu` | 条目存在 | ☐ |
| D-22 | `Docs/RequirementList.md` | 自 `10108` 起追加 D 的验收条目 | 编号连续、核对命令实跑通过 | ☐ |
| D-23 | `Deploy/Win11DockerUpgrade.md` | schema 迁移段落补充 `futu` | 文档含 `futu` | ☐ |

## 1.9 D6 部署验证

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| D-24 | 运行环境 | 重建镜像并启动，自动应用 `V33` | `SELECT success FROM schema_version WHERE version='33';` 为 `t` | ☐ |
| D-25 | 运行环境 | schema 分布与 `search_path` | `futu` 中 5 张表；`SHOW search_path;` 含 `public, quote, quant, futu` | ☐ |
| D-26 | 运行环境 | 接口回归 | `/API/V1/Version` 正常、`QuoteQuery`、`Run/List`、`Metadata/Security/List` 均正常 | ☐ |
| D-27 | 运行环境 | 容器日志无 ERROR | `docker logs finvquant 2>&1 \| grep -ciE "ERROR\|失败\|panic"` 为 `0` | ☐ |

---

## 2. 待决策项

| 编号 | 待决策 | 选项 | 当前建议 | 决策 |
|---|---|---|---|---|
| DD-01 | 表名是否去掉 `finv_futu_mapping_` 前缀（`futu.mapping_security`） | ① 保留前缀（与改造项 C 一致）② 去前缀 | ①：与 C 保持一致，去前缀属破坏性改名，另立清单再议 | ☐ 待定 |
| DD-02 | `futu` 是否与其它外部映射（如后续的 IB、Yahoo 映射）合并为 `mapping` schema | ① 保持独立 `futu` ② 统一 `mapping` schema | ①：当前只有富途一个外部源，独立更直观；出现第二个源时再评估合并 | ☐ 待定 |
| DD-03 | 基础主数据表（`finv_security` / `finv_market` / `finv_exchange` / `finv_region` / `finv_currency`）是否迁 `meta` schema | ① 本次不动 ② 一并迁入 | ①：与改造项 C 的 `D-03` 一致，避免范围膨胀 | ☐ 待定 |

---

## 3. 自检

```bash
# 迁移脚本结构
grep -c "CREATE SCHEMA IF NOT EXISTS futu" Deploy/Migrations/V33__move_finv_futu_tables_to_futu_schema.sql   # 期望 1
grep -oE "'finv_futu_[a-z_]+" Deploy/Migrations/V33__move_finv_futu_tables_to_futu_schema.sql | sort -u | wc -l   # 期望 5

# 代码与文档零裸表名（计划文档/变更日志/需求清单除外）
grep -rn "finv_futu" internal/ --include="*.go"                                                # 期望为空
grep -rn "finv_futu" Docs/ Prompt.md | grep -v "docs/Refactor/\|Docs/Refactor/\|Docs/CHANGELOG.md\|Docs/RequirementList.md\|Docs/DevSpec/DbSchemaSpec.md" | grep -v "futu\.finv_futu"   # 期望为空

# 历史迁移未被修改
git diff --stat -- Deploy/Migrations/V7* Deploy/Migrations/V8* Deploy/Migrations/V9* Deploy/Migrations/V1[0-7]* Deploy/Migrations/V1000*

# 构建与测试
go build ./... && go vet ./... && go test ./...
```

## 4. 变更记录

| 日期 | 变更 | 说明 |
|---|---|---|
| 2026-10-24 | 初版 | 建立改造项 D 检查清单（`D-01`～`D-27`）；盘点 5 张富途映射表（641 行、10 索引、5 触发器、0 外键、0 视图、Go 零引用）；确认与改造项 C 的关键差异：富途表建表在 `V33` 之前、种子仅 DML，因此**不需要**收尾迁移 |
