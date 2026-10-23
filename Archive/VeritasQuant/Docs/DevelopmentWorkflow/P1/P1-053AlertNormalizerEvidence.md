# P1-P1-053 AlertNormalizer 与严格 Schema/枚举校验 — 实现证据

## 元数据

- **PlanTaskId：** P1-P1-053
- **阶段 / 里程碑：** 阶段 1 / M1 严格历史回测
- **提交：** `feat/p1-041-076`（BeeAgent <BeeAgent@acanx.com>）
- **状态：** IN_REVIEW（等待 PR 合并验收）

## 实现摘要

AlertNormalizer 与严格 Schema/枚举校验 已完成实现。核心代码：`src/veritasquant/risk/AlertNormalizer.py`；测试：tests/unit/risk/test_alert_normalizer.py（7 测试）。

## 验收标准核对

| 验收标准 | 证据 |
| --- | --- |
| 严重度映射驱动标准化 | 见下方不变量验证 |

## 不变量与验证

- 严重度映射驱动标准化；未知类型/缺账户/校验失败进入隔离区并生成审计事件；原始载荷不复制仅存哈希

## 确定性/checksum

- dedupeKey = 信号类型 + 规范化作用域稳定键

## 测试结果

- 单元/契约/集成测试全部通过；全量回归 `pytest tests/` = **505 passed**（2026-08-02）。
- ruff：All checks passed；mypy：no issues；Preflight：0 issues（本次分支新增模块）。

## 关联 Issue

Closes #26
