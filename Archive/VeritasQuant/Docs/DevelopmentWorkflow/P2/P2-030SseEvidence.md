# P2-030 鉴权 SSE 状态流和有界 replay cursor 证据

## 任务信息
- **PlanTaskId:** P2-030
- **标题:** 实现鉴权 SSE 状态流和有界 replay cursor
- **阶段/里程碑:** M2A 模拟盘与基金能力建设
- **状态:** IN_REVIEW（等待第五批 PR 合并后验收）

## 实现内容

### 流模型（`application/StateStream.py`）
- **StreamEventV1**：sequence（单调）/eventType/accountId/payload/occurredAt
- **ReplayCursor**：从指定 sequence 恢复；缺失/非法回退最新
- **StreamSubscription 状态机**：Active → Closed（含 closeReason）
- **InMemoryStreamEventSource**：有界保留（retentionLimit 丢弃最旧）
- **StreamService**：
  - open()：cursor 窗口校验（超出 → BacklogExceeded 明确关闭）
  - deliver()：按账户过滤投递，积压超限关闭订阅
  - revokePrincipal()：权限撤销立即断开（验收标准 2）

### SSE 路由（`apps/server/StateStreamRoutes.py`）
- `GET /api/v1/stream/events?cursor=&account_id=`（SSE 协议，不套信封）
- 握手鉴权：凭据无效 401、账户越权 403
- 协议头 `VeritasQuant-SSE-Protocol: v1` + `X-Stream-Subscription-Id`
- 事件行：`event: <type>\ndata: <json>\nid: <seq>\n\n`
- close 事件：`event: stream.close` 携带 close_reason

## 验收标准映射
| 验收标准 | 实现 | 证据 |
| --- | --- | --- |
| 重连不丢已保留事件 | ReplayCursor + 有界保留 | test_open_replays_events_after_cursor |
| 权限撤销立即断开 | revokePrincipal 关闭订阅 | test_revoke_principal_closes_subscriptions |
| 积压超限明确关闭 | BacklogExceeded + close 事件 | test_backlog_exceeded_closes_subscription |

## 测试结果
- `tests/unit/application/test_state_stream.py`：12 个测试通过
- `tests/unit/apps/test_state_stream_routes.py`：6 个测试通过
- 全量 918 测试通过（2026-08-03，本地）
- ruff/mypy: 通过

## 关键决策
- TestClient 无法处理无限 SSE 流，路由层只测握手/关闭，完整流逻辑由应用层覆盖
- SSE 用独立协议版本（不套用 ResponseEnvelopeV1，TechSpec 10.2）
