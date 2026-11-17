# P2-008 Redis Streams 可替换跨进程传输 — 实现证据

- **PlanTaskId：** P2-008 | **里程碑：** M2A | **作者：** BeeAgent | **状态：** ACCEPTED（PR #219/#220/#221 已合并）
- **日期：** 2026-08-02

## 1. 实现内容
| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 传输协议+内存实现 | `src/veritasquant/infrastructure/messaging/StreamTransport.py` | `TransportMessageV1`（contentHash 与传输元数据无关）、`InMemoryStreamTransportV1`（至少一次语义、积压可见、重连不丢事件） |
| Redis Streams 实现 | `src/veritasquant/infrastructure/messaging/RedisStreamTransport.py` | XADD/XREADGROUP/XACK，幂等消费组创建 |
| 背压/重连策略 | `src/veritasquant/infrastructure/messaging/StreamPolicy.py` | 70%/90% 背压等级、指数退避重连状态机 |
| 测试 | `tests/unit/infrastructure/test_stream_transport.py`（11）、`tests/integration/database/test_redis_stream_transport.py`（CI） | |

## 2. 验收标准映射
| 标准 | 证据 |
| --- | --- |
| 传输元数据不进入事件哈希 | `test_content_hash_does_not_include_stream_metadata`：不同 stream/messageId 下 contentHash 不变 |
| 重复、积压和重连测试 | 至少一次重复消费内容一致、pendingCount 积压、reconnect 保持事件；Redis 集成测试覆盖发布/消费/确认 |

## 3. 证据索引
- `src/veritasquant/infrastructure/messaging/*.py`、`tests/unit/infrastructure/test_stream_transport.py`、`tests/integration/database/test_redis_stream_transport.py`
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-008StreamTransportEvidence.md`
