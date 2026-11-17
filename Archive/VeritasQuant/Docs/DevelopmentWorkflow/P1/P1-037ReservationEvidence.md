# P1-037 订单资源预占与释放证据

已实现按账户隔离的 `ReservationBookV1`，支持现金、证券和保证金资源的批准预占、按 `execution_id` 幂等的部分成交消耗，以及拒单/撤单终态时仅释放剩余资源。资源不足、成交超额、同回报不同数量和跨账户访问均明确拒绝。

完整订单迁移及其与账本/outbox 的同事务提交将在 P1-043 至 P1-050 的订单事实模型完成后接入；本工作项只提供预占状态与剩余额度契约。

```powershell
python3 -m pytest tests\unit\accounts\test_reservation.py -q
```
