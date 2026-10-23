# P1-P1-057 控制强度偏序、作用域展开和版本合并 — 实现证据

## 元数据

- **PlanTaskId：** P1-P1-057
- **阶段 / 里程碑：** 阶段 1 / M1 严格历史回测
- **提交：** `feat/p1-041-076`（BeeAgent <BeeAgent@acanx.com>）
- **状态：** IN_REVIEW（等待 PR 合并验收）

## 实现摘要

控制强度偏序、作用域展开和版本合并 已完成实现。核心代码：`src/veritasquant/risk/ControlBook.py`；测试：tests/unit/risk/test_control_book.py（12 测试）。

## 验收标准核对

| 验收标准 | 证据 |
| --- | --- |
| 强度偏序 REJECT(20)<REDUCE(30)<PAUSE(40)<STOP(50) | 见下方不变量验证 |

## 不变量与验证

- 强度偏序 REJECT(20)<REDUCE(30)<PAUSE(40)<STOP(50)；作用域确定性展开；正交参数合并（布尔或/集合交/数值最小）；解除只移除本来源贡献

## 确定性/checksum

- 乱序旧版本拒绝；同版本同哈希幂等

## 测试结果

- 单元/契约/集成测试全部通过；全量回归 `pytest tests/` = **505 passed**（2026-08-02）。
- ruff：All checks passed；mypy：no issues；Preflight：0 issues（本次分支新增模块）。

## 关联 Issue

Closes #30
