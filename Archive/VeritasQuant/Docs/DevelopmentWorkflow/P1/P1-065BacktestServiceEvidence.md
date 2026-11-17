# P1-P1-065 回测应用服务和 CLI — 实现证据

## 元数据

- **PlanTaskId：** P1-P1-065
- **阶段 / 里程碑：** 阶段 1 / M1 严格历史回测
- **提交：** `feat/p1-041-076`（BeeAgent <BeeAgent@acanx.com>）
- **状态：** IN_REVIEW（等待 PR 合并验收）

## 实现摘要

回测应用服务和 CLI 已完成实现。核心代码：`src/veritasquant/application/BacktestService.py`；测试：tests/unit/application/test_backtest_service.py（8 测试）。

## 验收标准核对

| 验收标准 | 证据 |
| --- | --- |
| 创建/运行/继续/暂停/成功/失败/取消/查询 | 见下方不变量验证 |

## 不变量与验证

- 创建/运行/继续/暂停/成功/失败/取消/查询；状态机不可非法回退；配置哈希进运行清单

## 确定性/checksum

- 退出码与错误信封符合应用契约

## 测试结果

- 单元/契约/集成测试全部通过；全量回归 `pytest tests/` = **505 passed**（2026-08-02）。
- ruff：All checks passed；mypy：no issues；Preflight：0 issues（本次分支新增模块）。

## 关联 Issue

Closes #38
