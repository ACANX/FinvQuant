# P2-015/016 基金份额 journal 与定投入金/预算 — 实现证据

- **PlanTaskId：** P2-015 / P2-016 | **里程碑：** M2A | **作者：** BeeAgent | **状态：** ACCEPTED（PR #219/#220/#221 已合并）

## 1. 实现内容
| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 份额账本 | `src/veritasquant/funds/FundShares.py` | 确认/赎回/分红再投资/现金分红 journal；ROUND_HALF_EVEN 份额精度；confirmationId 幂等；逐单位平衡 |
| 入金与预算 | `src/veritasquant/funds/InvestmentBudget.py` | DepositLedger（独立幂等记账、入金不计收益）、Reject/Cap/Skip 资金不足策略、预算裁剪 |
| 测试 | `tests/unit/funds/test_fund_shares.py`（8）、`test_investment_budget.py`（10） | |

## 2. 验收标准映射
| 标准 | 证据 |
| --- | --- |
| 份额精度和舍入正确 | `test_confirmation_adds_shares_with_precision`（ROUND_HALF_EVEN 1000.005→1000.00、1000.015→1000.02） |
| 重复确认不重复份额 | `test_duplicate_confirmation_is_idempotent` |
| 现金/份额逐单位平衡 | journal 复式语义（确认增份额+成本，赎回校验足够份额） |
| ExternalDeposit 独立幂等记账 | `test_deposit_idempotent_and_independent`、`test_deposits_excluded_from_returns` |
| Reject/Cap/Skip 行为固定 | `test_reject_policy_blocks_insufficient`、`test_cap_policy_caps_to_available`、`test_skip_policy_skips_without_consuming` |
| 入金不计收益 | DepositLedger 独立总额统计 |

## 3. 证据索引
- `src/veritasquant/funds/FundShares.py`、`InvestmentBudget.py`、对应测试
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-015-016SharesBudgetEvidence.md`
