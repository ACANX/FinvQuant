# P2-001 PostgreSQL 事实表、投影表、索引与首版迁移 — 实现证据

- **PlanTaskId：** P2-001
- **里程碑：** M2A（阶段 2：模拟盘与基金能力建设）
- **作者：** BeeAgent（开发执行代理）
- **状态：** ACCEPTED（PR #219/#220/#221 已合并）（PR 合并后由非作者验收）
- **日期：** 2026-08-02

## 1. 目标与范围

完成开发计划中「设计 PostgreSQL 事实表、投影表、索引和首版迁移」：建立平台数据库事实源与投影基础设施，为 inbox/outbox、租约、事件持久化、账本、订单、风控与恢复提供一致的持久化语义。

## 2. 实现内容

| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 首版迁移 | `Migrations/postgresql/V1__initial_fact_and_projection_schema.sql` | 21 张表 + 触发器 + 索引，单事务前滚/回滚 |
| 迁移执行器 | `src/veritasquant/infrastructure/persistence/Migrator.py` | 版本化前滚、失败回滚、advisory lock 防并发、幂等 |
| 静态契约测试 | `tests/contract/migrations/test_postgres_migration_schema.py` | 命名/必填表/精度/唯一键/不可变触发器/账户作用域 |
| 数据库集成测试 | `tests/integration/database/test_postgres_migrations.py` | 真实 PostgreSQL 前滚/幂等/回滚/约束验证 |
| CI 数据库 job | `.github/workflows/Ci.yml` | postgres:16.4 service 上运行迁移集成测试 |

## 3. 表设计决策

### 3.1 事实表（不可变，只追加）

| 表 | 事实内容 | 唯一键/约束要点 |
| --- | --- | --- |
| `fact_events` | EventEnvelopeV1 全字段 + 分区投递元数据 | `(run_id, account_group_id, delivery_sequence)` 分区内唯一；全序索引 `(ts, phase, priority, source_rank, source_sequence, event_id)` |
| `inbox_records` / `inbox_conflicts` | 幂等接收与协议冲突隔离 | `idempotency_key` 唯一；冲突记录不可变 |
| `outbox_records` | 领域提交后至少一次投递 | `(run_id, partition_id, message_id)` 唯一；PENDING 按 sequence 升序扫描 |
| `ledger_journals` / `ledger_entries` | JournalV1 + LedgerEntryV1 | `(account_id, commit_sequence)` 唯一；`(journal_id, entry_id)` 唯一；冲正自引用 FK |
| `order_intents` / `order_events` / `cancel_order_requests` / `replace_order_requests` | 订单意图与状态迁移事实 | `(client_order_id, order_version)` 唯一 |
| `execution_reports` | 券商成交回报 | `(account_id, execution_id)` 部分唯一索引（非成交回报允许 NULL） |
| `risk_decisions` / `trading_controls` | 风险决定与交易控制 | `(control_id, control_version)` 复合主键；幂等键唯一 |

### 3.2 投影表（可删除重建，不是事实源）

`account_snapshots`（账户快照）、`ledger_balance_projection`（余额投影）、`account_position_projection`（持仓投影）、`activity_control_projection`（活动控制投影）、`partition_checkpoints`（分区检查点）、`run_manifests`（运行清单）。

### 3.3 租约表

`partition_leases`：`account_group_id` 主键 + `fencing_token` 单调递增 + TTL 过期，支撑 P2-002 单活租约。

### 3.4 精度与约束

- 账本/订单/风控数量金额统一 `NUMERIC(38,18)`；价格 `NUMERIC(38,12)`；禁止二进制浮点进入账本（TechSpec 11.3 / 5.4）。
- 枚举值使用 `TEXT + CHECK` 白名单（方向、状态、阶段、科目等），与领域 StrEnum 对齐。
- 不可变约束：`prevent_fact_mutation()` 触发器在 12 张事实表上禁止 `UPDATE/DELETE`（ERRCODE 55000）。
- 账户分区：全部账户域事实表携带 `account_id`（事件表另含 `account_group_id`），结合复合唯一键与索引保证账户间数据隔离与分区内确定性顺序；物理分区在 P2-004 账户组拓扑冻结后演进（见第 6 节已知限制）。

## 4. 测试与验证证据

### 4.1 本地静态契约（已通过）

```text
$ .venv/bin/python -m pytest tests/contract/migrations/ -q
10 passed in 0.14s

$ .venv/bin/ruff check src/veritasquant/infrastructure/persistence/ tests/contract/migrations/ tests/integration/database/
All checks passed!

$ .venv/bin/mypy src/veritasquant/infrastructure/persistence/
Success: no issues found in 2 source files

$ .venv/bin/python scripts/Preflight.py
preflight issues: 0

$ .venv/bin/python scripts/VerifyDependencyLocks.py
dependency lock issues: 0
```

### 4.2 数据库集成测试（CI database job）

在 `postgres:16.4-alpine` service 上执行 `tests/integration/database/`，覆盖：

| 用例 | 验证点 |
| --- | --- |
| 前滚 | V1 应用后 `schema_version` 记录版本 1，21 张必填表存在 |
| 幂等 | 重复 `applyPending()` 返回空 |
| 失败回滚 | 坏迁移整体回滚：`partial_table` 不残留、版本不记录 |
| 事实表不可变 | `UPDATE`/`DELETE` 触发 RaiseException |
| 唯一键 | 重复 `event_id`、同账户同 `commit_sequence` 被拒 |
| NUMERIC 精度 | 超出 `NUMERIC(38,18)` 范围写入被拒 |

JUnit 证据：`artifacts/DatabaseJUnit.xml`（CI 上传 `database-migration-evidence`）。

## 5. 依赖变更

- `pyproject.toml`：新增运行时依赖 `psycopg[binary]>=3.2,<4`。
- `Requirements/Runtime.lock`：固定 `psycopg==3.3.4`、`psycopg-binary==3.3.4`（支持 Python 3.13/3.14 wheel）。
- 通过 `scripts/VerifyDependencyLocks.py` 一致性校验。

## 6. 已知限制与风险

| 项 | 说明 | 处置 |
| --- | --- | --- |
| 物理分区 | V1 未做 PostgreSQL 声明式物理分区；以账户作用域列 + 复合唯一键 + 索引保证账户隔离 | P2-004 账户组拓扑冻结后按 `account_group_id` 演进；已登记风险 RSK-P2-001 |
| 迁移执行器 | 仅支持按版本号顺序全量应用，尚无 schema 版本回退 | 回滚由「备份恢复 + 前滚新版本」策略覆盖（TechSpec 12.3）；P2-007 模拟盘恢复时补充演练证据 |
| 集成测试依赖真实数据库 | 本地无 docker/postgres 时自动 skip | CI database job 固定运行，作为 PR 必需检查 |

## 7. 追踪矩阵映射

- R-009（多账户拓扑）：`fact_events` 分区投递唯一键 + 账户作用域列
- R-016（可靠性/备份恢复）：迁移前滚/回滚机制 + CI 数据库 job
- R-002（事务与恢复）：账本唯一键 + 不可变约束支撑崩溃恢复

## 8. 证据索引

- `Migrations/postgresql/V1__initial_fact_and_projection_schema.sql`
- `src/veritasquant/infrastructure/persistence/Migrator.py`
- `tests/contract/migrations/test_postgres_migration_schema.py`
- `tests/integration/database/test_postgres_migrations.py`
- `.github/workflows/Ci.yml`（database job）
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-001PostgresSchemaEvidence.md`
