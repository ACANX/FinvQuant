# P2-041 进程崩溃与恢复演练报告 — 证据

- **任务：** P2-041（ISSUE #170）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** 待第八批 PR 合并后回填

## 验收标准对照

| 验收标准 | 实现 | 证据 |
|----------|------|------|
| RTO <= 15 分钟 | `PAPER_RTO_TARGET = 15min`；`rtoWithinTarget` 检查 | `reliability/CrashDrill.py` |
| RPO = 0 | `rpoZero` 强制条件 | 同上 |
| 活动控制恢复 100% | `controlsFullyRecovered` 强制条件 | 同上 |
| 未解释差异 0 | `differencesZero` 强制条件 | 同上 |
| 记录注入点/检测时间/保护动作/哈希/outbox/审批 | `CrashDrillReportV1` 全字段 + 报告哈希 | 同上 |

## 技术方案

- 演练报告记录：注入点（CrashPoint 名称）、注入/检测/恢复时间、RTO（由检测到恢复
  计算）、RPO、保护动作、事实/投影哈希、outbox 清空时间、活动控制恢复率、
  未解释差异数、人工审批人；
- 唯一结论：任一强制条件不满足 → FAIL；条件满足但无人审批 → INSUFFICIENT_EVIDENCE；
  全部满足 → PASS（`uniqueConclusion`）；
- 证据窗口：`CrashDrillEvidenceWindowV1` 必须累计 3 次 PASS（`DRILL_COUNT_REQUIRED = 3`），
  非 PASS 演练不计入。

## 测试

`tests/unit/reliability/test_crash_drill.py`（9 用例）：PASS 条件、RTO 超限、
RPO 非零、控制恢复不足、差异非零、缺审批、assertPass 拒绝、3 次窗口完成、失败不计数。

## 验证结果

- ruff / mypy / Preflight：通过
- 全量 pytest：待第八批 PR 后确认

## 风险与开放项

- 实际 3 次演练需模拟盘运行环境与时间窗口执行，本任务提供报告与证据窗口能力；
  演练执行证据在运行阶段补充（对应 P2-040 模拟盘运行）。
