# P1-P1-063 进程隔离、序列化 IPC、超时和资源配额 — 实现证据

## 元数据

- **PlanTaskId：** P1-P1-063
- **阶段 / 里程碑：** 阶段 1 / M1 严格历史回测
- **提交：** `feat/p1-041-076`（BeeAgent <BeeAgent@acanx.com>）
- **状态：** IN_REVIEW（等待 PR 合并验收）

## 实现摘要

进程隔离、序列化 IPC、超时和资源配额 已完成实现。核心代码：`src/veritasquant/strategy/Sandbox.py`；测试：tests/unit/strategy/test_sandbox.py（11 测试）。

## 验收标准核对

| 验收标准 | 证据 |
| --- | --- |
| AST 扫描阻止文件/网络/环境/系统时间/熵/子进程/危险导入 | 见下方不变量验证 |

## 不变量与验证

- AST 扫描阻止文件/网络/环境/系统时间/熵/子进程/危险导入；版本化配额；超时与输出超限丢弃全部输出；IPC 信封哈希校验

## 确定性/checksum

- quotaHash 进入运行清单

## 测试结果

- 单元/契约/集成测试全部通过；全量回归 `pytest tests/` = **505 passed**（2026-08-02）。
- ruff：All checks passed；mypy：no issues；Preflight：0 issues（本次分支新增模块）。

## 关联 Issue

Closes #36
