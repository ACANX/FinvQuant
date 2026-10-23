# P2-038 多账户、API、调度、基金端到端集成测试 — 证据

- **任务：** P2-038（ISSUE #167）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** 待第六批 PR 合并后回填

## 验收标准对照

| 验收标准 | 实现 | 证据 |
|----------|------|------|
| R-009 账户隔离 | 组间并行、单组故障隔离、LIVE 混组全局拒绝 | `test_multi_account_groups_parallel_no_crosstalk` / `test_group_failure_isolates_other_groups` / `test_live_group_rejects_mixed_mode_topology` |
| R-013 API 契约 | 命令幂等 202/409 映射、统一信封、显式账户 scope | `test_api_command_idempotency_conflict_mapping` |
| R-016 可靠性 | 调度幂等触发、claim/start/succeed 状态机 | `test_schedule_idempotent_trigger_and_state_machine` |
| R-017 调度任务 | 失败重试、fencing token 租约 | `test_schedule_retry_after_failure_with_fencing` / `test_schedule_fencing_rejects_stale_token` |
| 基金链路 | 份额 journal 幂等确认、赎回扣减、不可变重放、账户隔离 | `test_fund_share_journal_confirm_idempotent_and_redeem` / `test_fund_share_journal_account_isolation` |
| 跨账户串扰为 0 | 账户组/份额/命令 scope 全部隔离验证 | 上述各用例断言 |

## 技术方案

- 全部使用内存实现（InMemoryJobStore、InMemoryCommandStore、FundShareLedgerV1），
  不依赖 PostgreSQL/Redis，CI Quality 作业可直接运行（无跳过）；
- 复用真实生产组件：AccountGroupWorkerV1/GroupWorkerPoolV1、ScheduleService、
  CommandService/CommandApi、FundShareLedgerV1、EventEnvelopeV1.create（含内容哈希）；
- 命令路由测试走完整 FastAPI 应用（createApp + TestClient），验证统一信封与状态码。

## 测试文件

`tests/integration/test_e2e_multi_account_schedule_funds.py`（9 个用例）

## 验证结果

- ruff / mypy：通过
- 全量 pytest：971 passed（含本任务新增测试）

## 风险与开放项

- 券商仿真断连、崩溃恢复演练（P2-040~041 相关）依赖运行环境，后续批次推进。
