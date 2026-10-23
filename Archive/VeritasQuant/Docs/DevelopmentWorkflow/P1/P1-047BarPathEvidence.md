# P1-P1-047 版本化 Bar 内路径和触价/跳空矩阵 — 实现证据

## 元数据

- **PlanTaskId：** P1-P1-047
- **阶段 / 里程碑：** 阶段 1 / M1 严格历史回测
- **提交：** `feat/p1-041-076`（BeeAgent <BeeAgent@acanx.com>）
- **状态：** IN_REVIEW（等待 PR 合并验收）

## 实现摘要

版本化 Bar 内路径和触价/跳空矩阵 已完成实现。核心代码：`src/veritasquant/execution/BarPath.py`；测试：tests/unit/execution/test_bar_path.py（14 测试）。

## 验收标准核对

| 验收标准 | 证据 |
| --- | --- |
| DIRECTIONAL_OHLC_V1 路径 | 见下方不变量验证 |

## 不变量与验证

- DIRECTIONAL_OHLC_V1 路径；市价/限价/止损/止损限价触价矩阵；OCO 首次触发与 AMBIGUOUS；价格保护；tick/手数量化

## 确定性/checksum

- 路径顺序固定不选有利；派生价格买入向下卖出向上舍入

## 测试结果

- 单元/契约/集成测试全部通过；全量回归 `pytest tests/` = **505 passed**（2026-08-02）。
- ruff：All checks passed；mypy：no issues；Preflight：0 issues（本次分支新增模块）。

## 关联 Issue

Closes #20
