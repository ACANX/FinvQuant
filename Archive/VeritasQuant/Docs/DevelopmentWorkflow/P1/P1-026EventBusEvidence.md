# P1-026 确定性事件总线与订阅路由验证证据

实现确定性事件总线与冻结订阅路由。相同输入产生相同投递顺序；订阅顺序在首次
投递前冻结；消费者异常、重试与失败策略显式定义，不破坏已提交状态、不绕过
inbox/outbox 边界。

## 实现与测试

- 实现：`src/veritasquant/core/EventBus.py`
  - `DeterministicEventBusV1`：订阅注册、冻结、按事件类型路由投递
  - `SubscriptionOrder`：RegistrationOrder / SourceRankOrder 两种冻结顺序
  - `ConsumerFailurePolicy`：StopRun（默认）/ IsolateConsumer / RetryFixed
  - `DeliveryResultV1`：投递结果与隔离消费者，供审计与测试断言
- 测试：`tests/unit/core/test_event_bus.py`

## 验证结果

```powershell
python3 -m pytest tests\unit\core\test_event_bus.py -q
# 10 passed
```

## 验收标准映射

| 验收标准 | 证据 |
| --- | --- |
| 订阅顺序冻结 | `test_subscription_order_is_frozen_by_registration`、`test_subscription_cannot_change_after_delivery` |
| 同输入投递顺序固定 | `test_same_input_produces_same_delivery_order`（两次构建结果完全一致） |
| 消费者异常有明确失败策略 | `test_stop_run_policy_raises_and_halts`、`test_isolate_consumer_policy_continues_others`、`test_retry_fixed_policy_retries_then_stops` |

## 关键决策

- 投递前自动冻结订阅路由；投递推进 `UtcLogicalClockV1`，与 P1-025 组合保证
  防前视与确定性。
- StopRun 为默认最安全策略：消费者异常直接抛给调用方终止运行，绝不静默吞掉。
- IsolateConsumer 只隔离失败消费者，其余按冻结顺序继续；RetryFixed 达上限后
  升级为 StopRun。
- 总线本身不持久化、不绕过 inbox/outbox；持久化边界由事件循环任务负责。

## 残余风险

- 总线为内存实现；跨进程分发（Redis Streams）按技术方案 11.3 为后续工程任务，
  需保证同一投递顺序语义在分布式边界下保持。
