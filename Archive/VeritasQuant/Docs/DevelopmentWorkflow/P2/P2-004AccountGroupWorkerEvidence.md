# P2-004 账户组 worker、组内 account_rank 与故障隔离 — 实现证据

- **PlanTaskId：** P2-004
- **里程碑：** M2A
- **作者：** BeeAgent
- **状态：** ACCEPTED（PR #219/#220/#221 已合并）
- **日期：** 2026-08-02

## 1. 实现内容

| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 账户组拓扑 | `src/veritasquant/application/AccountGroupTopology.py` | `AccountGroupTopologyV1`（组内 rank 唯一、账户不重复）+ `validateGroupPartitioning`（账户跨组唯一、分区排名唯一、LIVE 不混组） |
| 组 worker | `src/veritasquant/application/AccountGroupWorker.py` | `AccountGroupWorkerV1` 组内按 rank 串行；`GroupWorkerPoolV1` 组间并行（ThreadPool），单组失败隔离 |
| 单元测试 | `tests/unit/application/test_account_group_worker.py` | 13 用例 |

## 2. 验收标准映射

| 验收标准 | 证据 |
| --- | --- |
| 组内串行、组间并行 | `test_serial_processing_by_rank`（顺序 a1→a2）；`test_parallel_fanout_processes_all_groups`（5 组并行全处理） |
| 单组失败不污染其他组 | `test_group_failure_isolates_only_its_partition`：A 组 handler 抛异常→ISOLATED，B 组正常 Active 且 `isolatedGroups == ("ag-a",)` |
| LIVE 与非 LIVE 不能混组 | `test_live_and_non_live_groups_rejected` |

## 3. 设计要点

- 拓扑创建后不可变（frozen dataclass），绑定在运行开始后冻结并写入运行清单。
- 组内串行保证可复现时序；组间由独立 worker 并行（各分区单活租约保护写入）。
- 单组失败只暂停该分区新开仓与外部发送；共享行情失效等全局场景由上层统一保护。

## 4. 测试证据

```text
$ .venv/bin/python -m pytest tests/unit/ -q   # 552 passed
$ .venv/bin/ruff check src tests              # All checks passed!
$ .venv/bin/mypy src                          # Success (111 files)
$ .venv/bin/python scripts/Preflight.py       # preflight issues: 0
```

## 5. 证据索引

- `src/veritasquant/application/AccountGroupTopology.py`、`AccountGroupWorker.py`
- `tests/unit/application/test_account_group_worker.py`
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-004AccountGroupWorkerEvidence.md`
