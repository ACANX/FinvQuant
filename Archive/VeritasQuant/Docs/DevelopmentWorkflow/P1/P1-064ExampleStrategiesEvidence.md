# P1-P1-064 均线交叉与日频动量示例策略 — 实现证据

## 元数据

- **PlanTaskId：** P1-P1-064
- **阶段 / 里程碑：** 阶段 1 / M1 严格历史回测
- **提交：** `feat/p1-041-076`（BeeAgent <BeeAgent@acanx.com>）
- **状态：** IN_REVIEW（等待 PR 合并验收）

## 实现摘要

均线交叉与日频动量示例策略 已完成实现。核心代码：`src/veritasquant/strategy/ExampleStrategies.py`；测试：tests/unit/strategy/test_example_strategies.py（4 测试）。

## 验收标准核对

| 验收标准 | 证据 |
| --- | --- |
| 固定版本/参数/预期订单清单 | 见下方不变量验证 |

## 不变量与验证

- 固定版本/参数/预期订单清单；横盘无单；负动量卖出；不绕过风控

## 确定性/checksum

- expectedIntentsForMomentumScenario 回归基准

## 测试结果

- 单元/契约/集成测试全部通过；全量回归 `pytest tests/` = **505 passed**（2026-08-02）。
- ruff：All checks passed；mypy：no issues；Preflight：0 issues（本次分支新增模块）。

## 关联 Issue

Closes #37
