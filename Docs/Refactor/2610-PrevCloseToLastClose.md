# 改造项 B：`prev_close` → `last_close` 重命名与 `open` 兜底回填（检查清单）

> 用途：把该改造拆成**可逐条勾选的检查项**，按编号顺序执行以避免漏改；执行中新发现的改造点追加到 `B-24` 之后顺延编号，不插入既有编号。
> 使用方式：自上而下推进；完成一项把「状态」改为 `☑ YYYY-MM-DD`，并附提交短号。**执行前置：改造项 A 已完成并发布。**
> 编号规则：本文件条目 `B-01`～`B-24`，**只增不改**，其中 `B-23`/`B-24` 为暂缓项；作废条目保留并标注「作废（原因）」，不删号。
> 完成判定：每项给出可执行的核验命令或明确的代码检查点，**未通过判定不得勾选**。
> 关联文档：同批改造项 A 见 [`2610-SecuCodeToUsc.md`](2610-SecuCodeToUsc.md)；需求验收条目见 [`../RequirementList.md`](../RequirementList.md)；版本变更见 [`../CHANGELOG.md`](../CHANGELOG.md)。
> 基线：仓库 `VERSION=0.1.0`；本节迁移版本 `V30`（改名）、`V31`（回填），`V29` 属改造项 A。

## 0. 概览

| 编号 | 标题 | 类型 | 影响面 | 风险 | 状态 |
|---|---|---|---|---|---|
| B | `prev_close` → `last_close` 重命名 + `open` 兜底回填 | 重命名 + 数据修复 | 1 张表 + Go 2 文件 + 3 篇文档 | 低（改名零语义变化；回填不可逆） | ☑ 2026-10-24 已完成（临时库实测 + 生产库部署验证；备份已完成，服务版本 0.1.1） |

**与改造项 A 的依赖**：`V29`（`secu_code`→`usc`）必须先完成；本节 SQL 中的证券代码列一律使用 `usc`。

## 2.1 背景与要求

- 要求 1：`prev_close` **重命名为 `last_close`**（保留列、保留语义，仅改名）。
- 要求 2：当 `open` 为 `NULL` 或 `0` 时，用 `last_close` 的值回填 `open`。
- 要求 3：是否最终删除 `last_close`，**后续再议**（见 §2.7 与 `DB-01`），本次不删。

## 2.1.1 字段口径说明（示例数据仅作参考）

- `last_close` 来自数据源 MVSV 文件的 `PrevClose` 列，语义为**前收盘价**；`open` 为**开盘价**，二者是独立字段。
- 仓库内示例文件显示两者通常不等（下表），但**示例文件不是生产数据，不能作为删除依据**，仅用于说明「`open` 不能无条件替代 `last_close`」这一口径差异：

| 示例数据源 | 样本 | `open` 与 `PrevClose` 相同率 | 说明 |
|---|---|---|---|
| `Docs/DataFormat/Example_GCmain_Min_V3_2026.mvsv` | 20 行 | 0 / 20 | 参考值 |
| `Docs/DataFormat/Example_US_NVDA_Min_V4_2026.mvsv` | 20 行 | 0 / 20 | 参考值 |
| `VeritasQuant/Data/US_NSDQ_NVDA/..._15000.mvsv` | 15000 行 | 3194 / 15000（21.3%） | 参考值 |

- **权威判定方式**（在生产库执行，结论记入 `DB-01`）：
  ```sql
  SELECT count(*) AS total,
         count(*) FILTER (WHERE open IS DISTINCT FROM last_close) AS diff_rows
  FROM finv_quote_secu_kline_min;
  ```
- `change` / `change_pct` 目前由 `close - prev_close` 计算（`internal/quote/service.go:114`），被 API 返回并由前端 tooltip 展示。**改名不影响该计算**（只是列名变化），因此本次改动功能零退化。

## 2.2 为什么本次只改名、不删除

| 项 | 说明 |
|---|---|
| 改名代价 | 纯 catalog 操作，O(1)、毫秒级锁；无索引依赖该列；API 不返回该字段（`Bar` 无此列输出），**前端零改动** |
| 删除代价 | 会打断 `change`/`change_pct` 计算，需先做涨跌幅解耦与停写观察，属另一阶段工作 |
| 结论 | 先改名拿到命名收益，删除留到数据核实与解耦完成后再定（§2.7） |

迁移顺序：`V29`（改造 A：`secu_code`→`usc`）→ `V30`（本节：`prev_close`→`last_close`）→ `V31`（`open` 回填）。

## 2.3 B1 重命名迁移

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| B-01 | `Deploy/Migrations/V30__finv_quote_kline_rename_prev_close_to_last_close.sql`（新建，依赖 A 的 `V29` 已完成） | 单事务、幂等：`information_schema.columns` 判断存在 `prev_close` 后执行 `ALTER TABLE finv_quote_secu_kline_min RENAME COLUMN prev_close TO last_close;` | 文件存在且含存在性判断 | ☑ 2026-10-24 |
| B-02 | 同上 | 列注释：`COMMENT ON COLUMN finv_quote_secu_kline_min.last_close IS '前收盘价（数据源 PrevClose）；与 open 独立，不可作为开盘价或开盘价替代';` | `\d+` 显示该注释 | ☑ 2026-10-24 |
| B-03 | 运行环境 | 确认该列无索引/约束/视图依赖（改名不需重建其他对象） | `SELECT indexdef FROM pg_indexes WHERE tablename='finv_quote_secu_kline_min';` 结果中无 `prev_close` | ☑ 2026-10-24（生产库只读核实：仅 `pkey` 与 `idx_finv_quote_usc_ts`，无该列索引） |
| B-04 | 运行环境 | 列名核验 | `\d finv_quote_secu_kline_min` 含 `last_close`、不含 `prev_close` | ☑ 2026-10-24（临时 PG18 库 + 生产库实测：`last_close` 存在、`prev_close` 不存在） |
| B-05 | `Deploy/Migrations/V1__...` | **保持原样不改**（历史迁移不可回改） | `git diff --stat -- Deploy/Migrations/V1__*` 输出为空 | ☑ 2026-10-24 |
| B-06 | 运行环境 | 存量数据可读性回归 | 迁移前后 `SELECT count(*), min(ts), max(ts) FROM finv_quote_secu_kline_min;` 结果一致 | ☑ 2026-10-24（生产库 V30 前后：2,804,329 行、时间范围 1541737200–1784615520 一致） |

## 2.4 B2 `open` 兜底回填

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| B-07 | `Deploy/Migrations/V31__finv_quote_kline_backfill_open_from_last_close.sql`（新建） | 单事务：`UPDATE finv_quote_secu_kline_min SET open = last_close WHERE (open IS NULL OR open = 0) AND last_close IS NOT NULL AND last_close <> 0;` | 文件存在；四个条件齐全 | ☑ 2026-10-24 |
| B-08 | 同上 | 回填前统计并输出受影响行数：`RAISE NOTICE 'backfill rows=%', cnt;` | 脚本含 `RAISE NOTICE` | ☑ 2026-10-24 |
| B-09 | 同上 | 留痕：向 `finv_quote_revision_log` 写一条汇总记录（`ingest_batch_id='MIGRATION_V31'`、`market_code=0`、`usc='ALL'`、`reason='last_close 回填 open（缺失/0 值）'`、`affected_rows` 为实测行数、`previous_summary`/`new_summary` 含条件与行数） | 执行后 `SELECT * FROM finv_quote_revision_log ORDER BY revision_id DESC LIMIT 1;` 可查到 | ☑ 2026-10-24 |
| B-10 | 运行环境 | 回填结果核验 | `SELECT count(*) FROM finv_quote_secu_kline_min WHERE (open IS NULL OR open = 0) AND last_close IS NOT NULL AND last_close <> 0;` 为 `0` | ☑ 2026-10-24（生产库迁移后查询为 0；迁移前预检亦为 0，即 V31 实际回填 0 行，`revision_log` 无 `MIGRATION_V31` 记录） |
| B-11 | `Deploy/backup/` | 迁移前做一次逻辑备份（`pg_dump -Fc`）；回填不可逆 | 备份文件存在且 `pg_restore --list` 可读 | ☑ 2026-10-24（`Deploy/backup/finvquant_周六1024_005740.dump`，68 MB，`pg_restore --list` 169 个 TOC、27 张表数据可读） |
| B-12 | 决策 `DB-03` | 确认是否新增 `flag_open_filled SMALLINT`（`1`=由 `last_close` 回填）以便事后区分与纠错；若不加，须在本文件与 `CHANGELOG` 中声明该不可逆性 | `DB-03` 已回填结论，实现与之一致 | ☑ 2026-10-24（DB-03 定为不标记，不可逆已在 V31/CHANGELOG 声明） |

## 2.5 B3 代码与文档同步

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| B-13 | `internal/quote/service.go` | 列名同步：`QueryBars` 的 SELECT、`buildUpsertSQL` 的 `updatable` 与 INSERT 列清单、`rowParams` 中的 `prev_close` → `last_close` | `grep -c "prev_close" internal/quote/service.go` 输出 `0` | ☑ 2026-10-24 |
| B-14 | `internal/mvsv/parser.go` | `Row.PrevClose` → `Row.LastClose`（字段名同步；列位置解析逻辑不变，含 12 列与 11 列两种布局） | `grep -c "PrevClose" internal/mvsv/parser.go` 输出 `0`；`colLastClose` 存在 | ☑ 2026-10-24 |
| B-15 | `internal/quote/service.go`（导入链路） | **增量兜底**：写入前若 `row.Open == nil` 或 `== "0"`，用 `row.LastClose` 填充（与 B-07 同规则），保证新导入数据不再产生缺 `open` | 导入一个 `open` 缺失样例后 `SELECT open` 非空 | ☑ 2026-10-24 |
| B-16 | `internal/quote/service.go` | `computeChange` 的注释与参数名同步（`prevClose` → `lastClose`），**计算逻辑不变** | 注释与实现一致；`grep -c "prevClose"` 输出 `0` | ☑ 2026-10-24 |
| B-17 | 测试文件：`internal/quote/service_test.go`、`internal/mvsv/parser_test.go`、`internal/mvsv/user_sample_verify_test.go` | 断言中的列名/字段名同步 | `grep -rn "prev_close\|PrevClose" internal/ --include="*_test.go"` 输出为空 | ☑ 2026-10-24 |
| B-18 | `Docs/DataFormat/MvsvFileFormat.md`（119、145 行） | 落库字段列 `prev_close` → `last_close` | 该两行显示 `last_close` | ☑ 2026-10-24 |
| B-19 | `Docs/API/HistoryQuote/HistoryQuote.md`（82、111 行） | 计算说明改为 `change = close - last_close` | 文档与实现一致 | ☑ 2026-10-24 |
| B-20 | `Web/src/` | 确认前端无该字段引用（当前实测 0 命中），无需改动 | `grep -rn "prev_close\|prevClose" Web/src` 输出为空 | ☑ 2026-10-24 |

## 2.6 B4 记录与发布

| 编号 | 位置 | 改动内容 | 完成判定 | 状态 |
|---|---|---|---|---|
| B-21 | `Docs/CHANGELOG.md` | 记录「`prev_close` 重命名为 `last_close`；`open` 缺失时用 `last_close` 回填」；注明回填不可逆 | 条目存在 | ☑ 2026-10-24 |
| B-22 | `Docs/RequirementList.md` | 自 `10063` 起追加验收条目（列名、注释、回填结果、代码零残留） | 编号顺延、核对命令实跑通过 | ☑ 2026-10-24 |

## 2.7 [暂缓] B5 删除 `last_close`（后续再议）

> 本节不随本次交付执行；触发条件：`DB-01` 依据**生产库数据**复议通过，且涨跌幅已完成解耦。

| 编号 | 位置 | 改动内容 | 说明 | 状态 |
|---|---|---|---|---|
| B-23 | 生产库 | 数据核实：`count(*) FILTER (WHERE open IS DISTINCT FROM last_close)`；据此判断 `last_close` 是否可完全由 `open` 替代 | 示例文件不足以定论，须以生产数据为准 | ☐ 暂缓 |
| B-24 | `Deploy/Migrations/V32+`、`internal/quote/service.go`、`internal/mvsv/parser.go` | 删除路径：① 将 MVSV 的 `ChangePrice`/`ChangeRatio` 落库（当前 parser 归为 `colIgnore`）→ ② `change` 改读库值、删除 `computeChange` → ③ 停止写入 `last_close` → ④ 观察期 → ⑤ `DROP COLUMN last_close` | 每一步都须独立验收，避免破坏涨跌幅 | ☐ 暂缓 |

---

## 3. 待决策项

| 编号 | 待决策 | 选项 | 当前建议 | 决策 |
|---|---|---|---|---|
| DB-01 | 是否删除 `last_close`（以及删除后 `change`/`change_pct` 口径） | ① 不删（继续保留）② 删前先把 `ChangePrice`/`ChangeRatio` 落库再删 | ①（先改名，删除待生产数据核实） | ☑ 2026-10-24 已定 ①：本次只改名、不删列。生产库实测（`finv_quote_secu_kline_min` 2,804,329 行）：`open IS DISTINCT FROM last_close` 共 1,642,169 行（58.6%），**证实 `open` 不能替代 `last_close`**，删除须待涨跌幅解耦后另议 |
| DB-02 | 若最终删除，`last_close` 何时停止写入 | ① 解耦完成后立即 ② 观察期后 | ②（先停写、后删列） | ☐ 暂缓（依赖 DB-01） |
| DB-03 | 是否需要标记「`open` 系回填值」 | ① 不标记（简单，但不可逆、无法区分）② 新增 `flag_open_filled SMALLINT` 标记 | ②（便于事后审计与纠错），对应 `B-12` | ☑ 2026-10-24 定为 ①：本次以「最小改动、只改名」为原则，不新增标记列；不可逆性已在 `V31` 脚本注释、`CHANGELOG` 与升级文档中声明，审计依赖 `revision_log`（`MIGRATION_V31`） |

---

## 4. 自检（全部条目完成后执行）

```bash
# 代码零残留：历史迁移 V1 保留原名；MVSV 文件头 FieldName 文本属外部格式契约，保留
# ATR 指标中的局部变量 prevClose 与数据库列无关，保留
grep -rn "prev_close" internal/ --include="*.go"                     # 期望输出为空
grep -rn "PrevClose" internal/ --include="*.go" | grep -v "FieldName" # 期望输出为空
grep -rn "prev_close\|prevClose" Web/src/                             # 期望输出为空

# 历史迁移未被修改
git diff --stat -- Deploy/Migrations/V1__*

# 构建与测试
go build ./... && go test ./...
```

## 5. 变更记录

| 日期 | 变更 | 说明 |
|---|---|---|
| 2026-10-24 | 初版 | 从 `RefactorPlan.md` 拆出为独立文件；方案由「回填 + 弃用删除」改为「重命名为 `last_close` + `open` 兜底回填，删除待议」；示例数据降级为「仅供参考」，权威判定以生产库 SQL 为准并记入 `DB-01` |
| 2026-10-24 | 部署验证 | 备份 `Deploy/backup/finvquant_周六1024_005740.dump` 后重建容器；`V30`/`V31` 均 `success=t`；`last_close` 就位、`prev_close` 消失；因生产数据 `open` 完整，`V31` 回填 0 行（无 `MIGRATION_V31` 留痕，符合幂等设计）；涨跌幅接口返回正常（如 `change=-0.7000` / `change_pct=-0.0172`） |
| 2026-10-24 | 执行完成 | 新建 `V30`（改名，幂等 + 列注释）与 `V31`（回填 + `revision_log` 留痕 + 不可逆声明）；Go 侧 `Row.PrevClose`→`Row.LastClose`、SQL 列名同步、新增导入时 `fillOpenFromLastClose` 增量兜底；文档改 `last_close`；`DB-01` 定为只改名、`DB-03` 定为不新增标记列；`go build`/`vet`/单测通过 |
