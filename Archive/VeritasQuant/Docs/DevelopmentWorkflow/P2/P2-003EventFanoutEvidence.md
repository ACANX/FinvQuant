# P2-003 共享事件持久化与确定性分区扇出 — 实现证据

- **PlanTaskId：** P2-003
- **里程碑：** M2A
- **作者：** BeeAgent（开发执行代理）
- **状态：** ACCEPTED（PR #219/#220/#221 已合并）
- **日期：** 2026-08-02

## 1. 实现内容

| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 事件存储 | `src/veritasquant/infrastructure/persistence/EventStore.py` | `fact_events` 持久化；同 `event_id` 幂等；分区投递序号独立递增 |
| 确定性扇出器 | `src/veritasquant/core/Fanout.py` | `partition_rank` 升序冻结目标；同一事件各分区内容一致；投递序号为信封外元数据 |
| 单元测试 | `tests/unit/core/test_fanout.py` | 排序、独立序号、内容一致、确定性、非法目标拒绝（7 用例） |
| 集成测试 | `tests/integration/database/test_postgres_event_fanout.py` | 真实 DB 扇出持久化、序号独立推进、幂等、旧 token 拒绝 |

## 2. 验收标准映射

| 验收标准 | 实现与证据 |
| --- | --- |
| 相同 event/hash 按固定 partition_rank 扇出 | `DeterministicFanoutV1` 按 rank 升序；集成测试验证两分区 `(rank, delivery_sequence)` 与内容哈希一致 |
| 分区快慢不改事件内容 | 快分区 A（已消费 3）与慢分区 B（0）投递同事件：序号 4 与 1，`content_hash` 相同 |

## 3. 设计要点

- 共享事件只创建一次（`event_id` 唯一），扇出只是持久化投递，不改信封/排序键/哈希。
- 单活租约串行化分区写入，`MAX(delivery_sequence)+1` 安全；`(run_id, account_group_id, delivery_sequence)` 唯一键兜底。
- 分区投递序号是信封外元数据，不参与事件内容哈希与因果时间。

## 4. 测试证据

```text
$ .venv/bin/python -m pytest tests/unit tests/contract -q
513 passed in 112.87s
$ .venv/bin/ruff check src tests          # All checks passed!
$ .venv/bin/mypy src                       # Success: no issues (104 files)
$ .venv/bin/python scripts/Preflight.py    # preflight issues: 0
```

数据库集成测试由 CI database job 执行。

## 5. 证据索引

- `src/veritasquant/infrastructure/persistence/EventStore.py`、`src/veritasquant/core/Fanout.py`
- `tests/unit/core/test_fanout.py`、`tests/integration/database/test_postgres_event_fanout.py`
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-003EventFanoutEvidence.md`
