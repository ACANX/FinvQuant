# P2-017/018 计划日历与日频金额模式 — 实现证据

- **PlanTaskId：** P2-017 / P2-018 | **里程碑：** M2A | **作者：** BeeAgent | **状态：** ACCEPTED（PR #219/#220/#221 已合并）

## 1. 实现内容
| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 计划日历 | `src/veritasquant/funds/PlanCalendar.py` | 自定义日历（跳过日）、每有效日只触发一次去重、节假日 Skip/Accumulate + 预算裁剪 |
| 金额规则 | `src/veritasquant/funds/AmountRules.py` | Fixed/RuleBased（净值偏离）/ExplicitSeries 三种日频模式；缺日策略；来源哈希；预算边界 |
| 测试 | `tests/unit/funds/test_plan_calendar.py`（7）、`test_amount_rules.py`（13） | |

## 2. 验收标准映射
| 标准 | 证据 |
| --- | --- |
| 每个有效日只触发一次 | `test_same_day_triggered_once`（去重） |
| 节假日 Skip/Accumulate 行为固定 | `test_skip_policy_drops_holidays`、`test_accumulate_groups_consecutive_missed_days` |
| 节假日累计预算裁剪 | `test_budget_clipping_caps_accumulated_amount`（200→150） |
| 三种日频金额模式 + 缺日策略 | `test_fixed_amount_per_fund`、`test_nav_deviation_adjusts_amount`、`test_series_lookup`、`test_missing_day_use_previous` |
| 来源哈希 | `test_same_config_same_hash`、`test_explicit_series_hash_deterministic` |
| 预算边界 | `test_budget_boundary_caps_amount` |

## 3. 证据索引
- `src/veritasquant/funds/PlanCalendar.py`、`AmountRules.py`、对应测试
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-017-018PlanCalendarAmountEvidence.md`
