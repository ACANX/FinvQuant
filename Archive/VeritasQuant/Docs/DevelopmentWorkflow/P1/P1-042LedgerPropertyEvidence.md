# P1-P1-042 ledger property-based 随机序列 — 实现证据

## 元数据

- **PlanTaskId：** P1-P1-042
- **阶段 / 里程碑：** 阶段 1 / M1 严格历史回测
- **提交：** `feat/p1-041-076`（BeeAgent <BeeAgent@acanx.com>）
- **状态：** IN_REVIEW（等待 PR 合并验收）

## 实现摘要

ledger property-based 随机序列 已完成实现。核心代码：`src/veritasquant/accounts/PropertySequences.py`；测试：tests/unit/accounts/test_property_sequences.py（10 测试，含 10,000 组随机序列）。

## 验收标准核对

| 验收标准 | 证据 |
| --- | --- |
| 逐单位平衡 | 见下方不变量验证 |

## 不变量与验证

- 逐单位平衡；全局守恒；提交幂等；重放一致；投影隔离；最小失败样本可复现

## 确定性/checksum

- 固定种子 0~9999 可归档；10,000 组无平衡/守恒/重放失败

## 测试结果

- 单元/契约/集成测试全部通过；全量回归 `pytest tests/` = **505 passed**（2026-08-02）。
- ruff：All checks passed；mypy：no issues；Preflight：0 issues（本次分支新增模块）。

## 关联 Issue

Closes #15
