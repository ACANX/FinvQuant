# P2-013/014 场外基金申购/赎回状态机 — 实现证据

- **PlanTaskId：** P2-013 / P2-014 | **里程碑：** M2A | **作者：** BeeAgent | **状态：** ACCEPTED（PR #219/#220/#221 已合并）

## 1. 实现内容
| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 状态机 | `src/veritasquant/funds/FundStateMachines.py` | 申购 `CREATED->ACCEPTED->WAITING_NAV->CONFIRMED`、赎回 `...->SETTLEMENT->COMPLETED`；非终态可 REJECT/CANCEL；转换历史可重放；终态不可再转换 |
| 测试 | `tests/unit/funds/test_fund_state_machines.py`（10） | |

## 2. 验收标准映射
| 标准 | 证据 |
| --- | --- |
| CREATED 至 CONFIRMED/REJECTED/CANCELLED 全边覆盖 | `test_full_confirm_path`、`test_reject_from_non_terminal_releases_funds`（拒绝释放资金）、`test_cancel_from_waiting_nav` |
| 受理冻结、失败释放正确 | 转换动作语义（ACCEPT=冻结、REJECT/CANCEL=释放/退回）在转换表强制 |
| 赎回 WAITING_NAV/SETTLEMENT、费用、到账和拒绝路径可重放 | `test_full_settlement_path`、`test_replay_paths_for_fees_and_settlement`、`test_reject_after_settlement_releases` |

## 3. 证据索引
- `src/veritasquant/funds/FundStateMachines.py`、`tests/unit/funds/test_fund_state_machines.py`
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-013-014FundStateMachinesEvidence.md`
