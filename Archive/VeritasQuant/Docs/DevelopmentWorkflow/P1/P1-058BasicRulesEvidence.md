# P1-P1-058 资金、数量、集中度、保证金和陈旧数据基础规则 — 实现证据

## 元数据

- **PlanTaskId：** P1-P1-058
- **阶段 / 里程碑：** 阶段 1 / M1 严格历史回测
- **提交：** `feat/p1-041-076`（BeeAgent <BeeAgent@acanx.com>）
- **状态：** IN_REVIEW（等待 PR 合并验收）

## 实现摘要

资金、数量、集中度、保证金和陈旧数据基础规则 已完成实现。核心代码：`src/veritasquant/risk/BasicRules.py`；测试：tests/unit/risk/test_basic_rules.py（9 测试）。

## 验收标准核对

| 验收标准 | 证据 |
| --- | --- |
| 五类规则带版本/原因码/快照引用 | 见下方不变量验证 |

## 不变量与验证

- 五类规则带版本/原因码/快照引用；硬限制不能配置放宽；零权益安全失败

## 确定性/checksum

- 配置哈希稳定且版本化

## 测试结果

- 单元/契约/集成测试全部通过；全量回归 `pytest tests/` = **505 passed**（2026-08-02）。
- ruff：All checks passed；mypy：no issues；Preflight：0 issues（本次分支新增模块）。

## 关联 Issue

Closes #31
