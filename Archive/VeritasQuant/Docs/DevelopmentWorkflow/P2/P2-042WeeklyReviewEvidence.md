# P2-042 每周复核工具 — 证据

- **任务：** P2-042（ISSUE #171）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** 待第八批 PR 合并后回填

## 验收标准对照

| 验收标准 | 实现 | 证据 |
|----------|------|------|
| 60 日每日差异为 0 | `WeeklyReviewerV1(windowDays=60)` 窗口完整性 + 每日差异 0 | `reliability/WeeklyReview.py` |
| 所有缺口有隔离/修复证据 | `DataGapV1.resolved`（True=修复，False=隔离 + isolationNote） | 同上 |
| 重复副作用计数为 0 | `recordDuplicateSideEffects` + `duplicateSideEffects` 检查 | 同上 |

## 技术方案

- 每日对账结果聚合（账本/订单/持仓三类差异），周复核报告唯一状态：
  CLEAN / HAS_FINDINGS / INSUFFICIENT_EVIDENCE；
- `assertClean()` 在证据窗口任何非 CLEAN 状态下抛出，防止不合格数据继续；
- 报告哈希为不可变证据（canonicalHash）。

## 测试

`tests/unit/reliability/test_weekly_review.py`（7 用例）：CLEAN、差异发现、
未解决缺口、已解决缺口不算、重复副作用、无数据证据不足、60 日窗口完整。

## 验证结果

- ruff / mypy / Preflight：通过
- 全量 pytest：待第八批 PR 后确认

## 风险与开放项

- 实际周复核需模拟盘每日对账数据输入，本任务提供复核与结论能力；
  运行期数据由 P2-040 模拟盘运行产生。
