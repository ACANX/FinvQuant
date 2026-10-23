# P2-033 账户、结果分析、逐笔账本和监控页证据

## 任务信息
- **PlanTaskId:** P2-033
- **标题:** 实现账户、结果分析、逐笔账本和监控页
- **阶段/里程碑:** M2A 模拟盘与基金能力建设
- **状态:** IN_REVIEW（等待第五批 PR 合并后验收）

## 实现内容

### 后端端点（`apps/server/DomainRoutes.py` 扩展）
- `GET /api/v1/accounts/{id}/ledger?run_id=` 逐笔分录
- `GET /api/v1/accounts/{id}/cashflows?run_id=` 现金流
- `GET /api/v1/accounts/{id}/shares?run_id=` 基金份额
- `GET /api/v1/accounts/{id}/analysis?run_id=` 结果分析（TWR/XIRR/本金）
- 所有端点显式 account_id + run_id（TechSpec 10.1），未知账户 404

### API Client 扩展
- accountLedger/accountCashFlows/accountShares/accountAnalysis 四方法

### GUI 页面（`apps/guiclient/Pages.py`）
- **账户管理页**：账户列表 + 详情加载（模式/适配器）
- **结果分析页**：分析 JSON + 逐笔分录/现金流/份额三 tab
- **实时监控页**：账户快照 + 模式指标，SSE 状态流提示
- **账户上下文隔离**：`_currentAccountId()` 从 session_state 读取侧边栏
  选择（多账户切换不串数据）

## 验收标准映射
| 验收标准 | 实现 | 证据 |
| --- | --- | --- |
| 多账户切换不串数据 | session_state 账户上下文 + 显式 account_id | test_require_account_rejects_empty |
| 逐笔分录/现金流/份额可核对 | ledger/cashflows/shares 端点 | test_ledger_cashflows_shares_analysis |
| 双轨结果可核对 | analysis（TWR/XIRR/本金） | test_analysis |

## 测试结果
- `tests/unit/apps/guiclient/test_account_pages.py`：5 个测试通过
- `tests/unit/apps/test_domain_routes.py`：新增 5 个端点测试通过
- 全量 918 测试通过（2026-08-03，本地）
- ruff/mypy: 通过

## 关键决策
- 账户域端点强制 run_id（校验失败 400 + 1001）
- 页面从 session_state 读账户上下文，杜绝隐式默认账户
