# P1-P1-062 增量指标窗口和完成 Bar 查询 API — 实现证据

## 元数据

- **PlanTaskId：** P1-P1-062
- **阶段 / 里程碑：** 阶段 1 / M1 严格历史回测
- **提交：** `feat/p1-041-076`（BeeAgent <BeeAgent@acanx.com>）
- **状态：** IN_REVIEW（等待 PR 合并验收）

## 实现摘要

增量指标窗口和完成 Bar 查询 API 已完成实现。核心代码：`src/veritasquant/strategy/IndicatorWindow.py`；测试：tests/unit/strategy/test_indicator_window.py（8 测试）。

## 验收标准核对

| 验收标准 | 证据 |
| --- | --- |
| 指标只用已消费数据 | 见下方不变量验证 |

## 不变量与验证

- 指标只用已消费数据；时间严格递增拒绝乱序/未来；有界容量窗口；均线/极值/累计量；queryAsOf 按 barEnd 过滤

## 确定性/checksum

- metricsHash 确定性；未来数据不影响历史窗口

## 测试结果

- 单元/契约/集成测试全部通过；全量回归 `pytest tests/` = **505 passed**（2026-08-02）。
- ruff：All checks passed；mypy：no issues；Preflight：0 issues（本次分支新增模块）。

## 关联 Issue

Closes #35
