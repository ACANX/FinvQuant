# P2-010/011 FundNavSchemaV1、基金状态/费率/日历与净值导入 — 实现证据

- **PlanTaskId：** P2-010 / P2-011 | **里程碑：** M2A | **作者：** BeeAgent | **状态：** ACCEPTED（PR #219/#220/#221 已合并）

## 1. 实现内容
| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 净值 Schema | `src/veritasquant/data/FundNav.py` | `FundNavSchemaV1`（ts 与 nav_date 分离、ts>=ingested/published）、`FundStatusV1`、`FundRateScheduleV1`、`FundCalendarV1`、`NavAvailabilityPolicyV1`（保守策略） |
| 净值导入 | `src/veritasquant/data/FundNavImporter.py` | 原始记录→不可变净值；缺发布时间用保守策略+质量标志；重复主键隔离；修订走 SupersedesDataVersionId 新版本 |
| 测试 | `tests/unit/data/test_fund_nav.py`（12） | |

## 2. 验收标准映射
| 标准 | 证据 |
| --- | --- |
| nav_date 与可用 ts 分离 | `test_nav_date_separated_from_available_ts`、ts 早于 ingested/published 拒绝 |
| 未知发布时间按保守策略 | `test_missing_published_at_uses_conservative_policy`：NextTradingDayOpen + MissingPublishedAt 质量标志 |
| 修订生成新版本 | `test_revision_creates_new_version`：SupersedesDataVersionId 关联、禁止覆盖已用净值 |

## 3. 证据索引
- `src/veritasquant/data/FundNav.py`、`FundNavImporter.py`、`tests/unit/data/test_fund_nav.py`
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-010-011FundNavEvidence.md`
