# P2-019 六类内置智能定投方案 — 实现证据

- **PlanTaskId：** P2-019 | **里程碑：** M2A | **作者：** BeeAgent | **状态：** ACCEPTED（PR #219/#220/#221 已合并）

## 1. 实现内容
| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 六类方案 | `src/veritasquant/funds/SmartPlans.py` | 固定金额、均线偏离、估值分位、回撤倍增、目标价值、目标收益；每类固定参数 Schema + planHash + 逐期决定（纯函数，只用当时可用数据） |
| 测试 | `tests/unit/funds/test_smart_plans.py`（14） | |

## 2. 验收标准映射
| 标准 | 证据 |
| --- | --- |
| 每类固定参数 Schema 与基准数据 | dataclass 参数 + planHash 确定性 |
| 逐期预期决定 | `test_below_ma_invests_more`、`test_low_valuation_doubles`、`test_deep_drawdown_increases_amount`、`test_gap_filled`、`test_target_reached_stops` |
| 只用当时可用数据 | 决策仅读取 SmartPlanContextV1（无未来变量）；均线窗口不足跳过 |

## 3. 证据索引
- `src/veritasquant/funds/SmartPlans.py`、`tests/unit/funds/test_smart_plans.py`
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-019SmartPlansEvidence.md`
