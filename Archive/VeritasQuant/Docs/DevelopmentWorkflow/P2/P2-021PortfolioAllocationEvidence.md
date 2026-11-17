# P2-021 多基金分配、权重竞争与组合预算 — 实现证据

- **PlanTaskId：** P2-021 | **里程碑：** M2A | **作者：** BeeAgent | **状态：** ACCEPTED（PR #219/#220/#221 已合并）

## 1. 实现内容
| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 组合分配 | `src/veritasquant/funds/PortfolioAllocation.py` | 权重降序确定性分配；总额不超预算/现金/单基金风险上限；AccountIsolatedAllocator 多账户隔离 |
| 测试 | `tests/unit/funds/test_portfolio_allocation.py`（8） | |

## 2. 验收标准映射
| 标准 | 证据 |
| --- | --- |
| 同日分配总额不超预算/现金/风险上限 | `test_total_never_exceeds_budget_or_cash`、`test_limited_cash_caps_total`、`test_single_fund_risk_cap_enforced` |
| 顺序确定 | `test_deterministic_order_weight_desc`、`test_tie_broken_by_symbol` |
| 多账户隔离 | `test_accounts_allocated_independently`、`test_missing_account_rejected` |

## 3. 证据索引
- `src/veritasquant/funds/PortfolioAllocation.py`、`tests/unit/funds/test_portfolio_allocation.py`
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-021PortfolioAllocationEvidence.md`
