# P2-007 模拟盘 checkpoint、重启与每日对账 — 实现证据

- **PlanTaskId：** P2-007
- **里程碑：** M2A
- **作者：** BeeAgent
- **状态：** ACCEPTED（PR #219/#220/#221 已合并）
- **日期：** 2026-08-02

## 1. 实现内容

| 工件 | 路径 | 说明 |
| --- | --- | --- |
| checkpoint 存储 | `src/veritasquant/infrastructure/persistence/CheckpointStore.py` | `CheckpointStoreV1`：`partition_checkpoints` upsert/读取；与领域写入同事务提交 |
| 每日对账 | `src/veritasquant/application/Reconciliation.py` | `DailyReconciliationV1`：账本/订单/持仓差异检测、分类（缺失/多余/金额不符/状态不符）、恢复门禁 |
| 单元测试 | `tests/unit/application/test_reconciliation_checkpoint.py` | 7 用例 |
| 集成测试 | `tests/integration/database/test_postgres_checkpoint.py` | 保存/加载、单调推进、重启 RPO=0（CI 运行） |

## 2. 验收标准映射

| 验收标准 | 证据 |
| --- | --- |
| 重启后 RPO=0 | checkpoint 与领域事实同事务提交（TechSpec 6.3）；`test_restart_replays_from_last_checkpoint_rpo_zero` 用全新连接读取最后已提交 checkpoint 无丢失 |
| 账户/订单/持仓差异可检测、分类并阻止恢复 | `test_missing_extra_and_state_differences_classified`（三类差异分类正确）；`recoveryBlocked` 在存在差异时置位，`test_differences_must_be_resolved_before_recovery` |

## 3. 设计要点

- checkpoint 保存使用 `ON CONFLICT DO UPDATE` 幂等推进；重复重放不重复副作用。
- 对账器为纯函数：权威事实（账本/订单/持仓）与运行状态逐一比对，任何未解释差异阻止恢复交易。
- 差异清零后恢复门禁解除，符合"恢复交易前未解释对账差异 = 0"（TechSpec 12.3）。

## 4. 测试证据

```text
$ .venv/bin/python -m pytest tests/unit/ -q   # 552 passed
$ .venv/bin/ruff check src tests              # All checks passed!
```

数据库 checkpoint 集成测试由 CI database job 执行。

## 5. 证据索引

- `src/veritasquant/infrastructure/persistence/CheckpointStore.py`
- `src/veritasquant/application/Reconciliation.py`
- `tests/unit/application/test_reconciliation_checkpoint.py`
- `tests/integration/database/test_postgres_checkpoint.py`
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-007CheckpointReconciliationEvidence.md`
