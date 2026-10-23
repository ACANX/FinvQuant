# P2-012 基金事件 Schema 注册与计划到期生成器 — 实现证据

- **PlanTaskId：** P2-012 | **里程碑：** M2A | **作者：** BeeAgent | **状态：** ACCEPTED（PR #219/#220/#221 已合并）

## 1. 实现内容
| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 基金事件载荷 | `src/veritasquant/funds/FundEvents.py` | 5 个基金事件（NavPublished/PlanDue/Subscription/Redemption/ShareConfirmed） |
| 注册+生成器 | `src/veritasquant/funds/FundEventRegistration.py` | `registerFundEvents` 注册进 EventSchemaRegistry；`InvestmentPlanDueGeneratorV1` 日历+计划+窗口→UTC 到期事件 |
| 测试 | `tests/unit/funds/test_fund_event_registration.py`（9） | |

## 2. 验收标准映射
| 标准 | 证据 |
| --- | --- |
| 计划时间确定性转 UTC | `scheduledUtcTs` 由本地触发时间确定性转换 |
| 历史触发不依赖服务器当前时间 | `test_historical_trigger_independent_of_server_time`：纯函数，同输入同输出 |

## 3. 证据索引
- `src/veritasquant/funds/FundEvents.py`、`FundEventRegistration.py`、`tests/unit/funds/test_fund_event_registration.py`
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-012FundEventsEvidence.md`
