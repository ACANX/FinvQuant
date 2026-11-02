# USC 补丁规则 API

> 所属：行情 → 收剔导传ETL → USC 补丁规则
> 路径前缀：`/API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch`
> 数据表：`cdeu.finv_quote_cdeu_usc_patch_rule`
> 关联需求：10931
> 设计方案：[Docs/Proposal/202611-0002-UscPatchSchemeProposal.md](../../Proposal/202611-0002-UscPatchSchemeProposal.md)

## 0. 用途

`cdeu.finv_quote_secu_kline_min` 会持续写入过时 `usc`（如 `SH000688`、`KC50`）的行情。本组接口维护「旧 usc → 新 usc」的补丁规则，并按规则把旧 usc 的行情迁移到新 usc。

- 规则可动态增删改、可停用，不写死在代码或迁移里。
- 主键 `(ts, usc)` 冲突用 upsert 规避：目标 usc 已有同一 `ts` 的行时，按 `gmt_update` 较新者胜覆盖。
- 迁移与删除在同一语句内完成，先写入目标 usc、再删旧 usc 行。

## 1. 查询规则列表

`GET /API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/List`

查询参数（小驼峰）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | 否 | 页码，默认 1 |
| `pageSize` | int | 否 | 每页条数，默认 20 |
| `keyword` | string | 否 | 匹配 `usc_old`、`usc_new`、`note` |
| `status` | string | 否 | `1` 仅生效、`0` 仅停用、空为全部 |

响应 `data`：`{ total, list: [{ idn, usc_old, usc_new, match_mode, flag_enable, priority, note, gmt_create, gmt_update }] }`

## 2. 保存规则

`POST /API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/Save`

请求体：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `idn` | int | 否 | 0 或省略表示新增，大于 0 表示更新 |
| `usc_old` | string | 是 | 过时 usc |
| `usc_new` | string | 是 | 目标 usc，不能与 `usc_old` 相同 |
| `match_mode` | string | 否 | `exact`（默认）或 `prefix` |
| `flag_enable` | string | 否 | `1` 生效、`0` 停用，默认 `0` |
| `priority` | int | 否 | 默认 100，数值小的先匹配 |
| `note` | string | 否 | 规则依据 |

响应 `data`：`{ idn }`

约束：同一个 `usc_old` 同时只允许一条生效规则（部分唯一索引），冲突时返回错误。

## 3. 启用或停用

`POST /API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/Enable`

请求体：`{ "idn": 5, "enable": true }`

停用不删除，便于追溯。

## 4. 删除规则

`POST /API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/Delete`

请求体：`{ "idn": 5 }`

建议优先停用；删除仅用于录错的规则。

## 5. 试算

`GET /API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/Preview`

只读，不写库。响应 `data`：

| 字段 | 说明 |
|------|------|
| `list` | 每条生效规则一行：`usc_old`、`usc_new`、`rows`、`conflict_rows`、`min_ts`、`max_ts` |
| `total_rows` | 全部生效规则命中行数之和 |
| `total_conflict` | 其中会与目标 usc 冲突的行数之和 |

启用规则前必须先看试算结果，确认 `conflict_rows` 符合预期。

启用规则时（`Enable` 且 `enable=true`）后端会自动把目标 usc 登记到 `cdeu.finv_quote_cdeu_usc`（从旧 usc 复制元数据，只补不覆盖），执行时再兜底同步一次，避免「主表有行情、登记表没有」的空档。

## 6. 手动执行

`POST /API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/Run`

请求体：`{ "batch_size": 5000 }`（可选，默认 5000）

**异步执行**：立即返回运行快照（`status=running`），前端按 `sinceSeq` 轮询 §6.1 的接口获取增量日志，交互与 BRS 手动备份（`/Meta/BRS/Backup/Finv/Run`）一致。已有实例在运行时返回 HTTP 409、`code=2009`。

### 6.1 查询执行状态与日志

`GET /API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/Status?sinceSeq=`

响应 `data`：

| 字段 | 说明 |
|------|------|
| `rule_count` | 本次生效规则数，为 0 时直接返回 |
| `batches` | 执行批次数 |
| `scanned_rows` | 扫描到的旧 usc 行数 |
| `moved_rows` | 写入目标 usc 的行数 |
| `conflict_rows` | 其中与目标 usc 冲突、按较新者胜覆盖的行数 |
| `deleted_rows` | 删除的旧 usc 行数 |
| `registered_rows` | 本轮为补丁目标 usc 补齐的登记行数（主表出现新 usc 但登记表没有时自动复制，需求 10935） |
| `from_ts` | 处理区间下界 |
| `to_ts` | 处理区间上界 |
| `current_batch` | 当前批次的时间区间（可读格式） |
| `logs` | 增量日志：`seq`、`time`、`level`、`message` |
| `last_seq` | 日志序号，下次轮询作为 `sinceSeq` 传入 |

日志样例（每批一行）：

```
[info] 开始执行过时 USC 补丁（run_id=20261102-083949，触发=manual）
[info] 第 1 批 2026-10-09 09:30 ~ 2026-10-09 15:00：本批扫描 300 行；累计迁移 300 行、冲突 0 行、删除 300 行；本批耗时 58 ms
[info] 运行结束（成功）：3 批，扫描 700 行，迁移 700 行，冲突 0 行，删除 700 行，耗时 252 ms
```

定时任务 `PATCH_OUTDATED_USC` 每日 05:00 执行同一逻辑，并与手动触发共用单实例约束（`status=running` 时定时触发跳过）。

## 7. 注意事项

- 上游 `futu.finv_futu_mapping_security` 未同步时旧 usc 会反复回流，`moved_rows` 长期偏高说明上游待修。
- 已导出的旧 usc 归档文件不自动重刷，靠保留策略自然淘汰。
- `cdeu.finv_quote_cdeu_usc` 的旧 usc 登记行保留不删（需求 10920 的「永不删除」原则）。
