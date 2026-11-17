# P2-002 数据库端 inbox/outbox、租约与 fencing token — 实现证据

- **PlanTaskId：** P2-002
- **里程碑：** M2A
- **作者：** BeeAgent（开发执行代理）
- **状态：** ACCEPTED（PR #219/#220/#221 已合并）（PR 合并后由非作者验收）
- **日期：** 2026-08-02

## 1. 实现内容

| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 租约存储 | `src/veritasquant/infrastructure/persistence/LeaseStore.py` | 单活租约获取/续租/释放 + 单调 fencing token + 写入门禁 guard |
| inbox 存储 | `src/veritasquant/infrastructure/persistence/InboxStore.py` | 幂等接受（DUPLICATE 返回原结果）、协议冲突隔离（CONFLICT 审计）、旧 token 写入被拒 |
| outbox 存储 | `src/veritasquant/infrastructure/persistence/OutboxStore.py` | 至少一次投递、按提交序号升序发布、失败保留重试、同 message_id 幂等 |
| 单元测试 | `tests/unit/infrastructure/test_lease_inbox_outbox_stores.py` | SQL 守卫语义、输入校验、幂等 SQL 静态验证 |
| 集成测试 | `tests/integration/database/test_postgres_inbox_outbox_lease.py` | 真实 PostgreSQL 双写者 fencing、租约生命周期、幂等、重投无重复副作用 |

## 2. 验收标准映射

| 验收标准 | 实现与证据 |
| --- | --- |
| 双写者测试中旧 token 写入被拒绝 | `LeaseStoreV1.guard()` 事务内校验 `lease_holder + fencing_token + 未过期`；`test_old_token_write_rejected_by_guard` 模拟 A 租约过期、B 抢占 token=2 后 A 的 inbox 写入抛出 LeaseError |
| 重投无重复副作用 | inbox `ON CONFLICT (idempotency_key) DO NOTHING` 后读取原记录返回 DUPLICATE（receipt_sequence 不变）；outbox 同 `message_id` 幂等、`publishPending` 仅 PUBLISHED 一次 |

## 3. 设计要点

- 租约 TTL 默认 10 秒、续租间隔 3 秒（TechSpec 12.3 V1 默认值）；抢占时 `fencing_token = fencing_token + 1`。
- 租约续租/抢占 SQL 均在 WHERE 中校验 holder 与 token，从 SQL 层保证原子性。
- inbox/outbox 写入与 `guard` 在同一事务内完成，避免 check-then-act 竞态。
- 冲突记录写入 `inbox_conflicts`（不可变事实表），保存新旧哈希供审计。

## 4. 测试证据

```text
$ .venv/bin/python -m pytest tests/contract/migrations/ tests/unit/infrastructure/ -q
25 passed in 0.19s

$ .venv/bin/ruff check src tests
All checks passed!

$ .venv/bin/mypy src
Success: no issues found in 102 source files

$ .venv/bin/python scripts/Preflight.py
preflight issues: 0
```

数据库集成测试由 CI database job 在 postgres:16.4 service 上执行（`artifacts/DatabaseJUnit.xml`）。

## 5. 追踪矩阵

- R-009（多账户拓扑）：租约 + fencing token 保证账户组单活写入
- R-016（可靠性）：outbox 至少一次投递与失败保留
- R-002（事务与恢复）：inbox/outbox 与领域写入同事务

## 6. 证据索引

- `src/veritasquant/infrastructure/persistence/LeaseStore.py` / `InboxStore.py` / `OutboxStore.py`
- `tests/unit/infrastructure/test_lease_inbox_outbox_stores.py`
- `tests/integration/database/test_postgres_inbox_outbox_lease.py`
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-002InboxOutboxLeaseEvidence.md`
