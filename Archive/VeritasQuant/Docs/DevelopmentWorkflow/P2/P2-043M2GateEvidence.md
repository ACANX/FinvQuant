# P2-043 M2 Gate 检查项与报告 — 证据

- **任务：** P2-043（ISSUE #172）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** 待第八批 PR 合并后回填

## 验收标准对照

| 验收标准 | 实现 | 证据 |
|----------|------|------|
| 平台与策略 Gate 分别给出结论 | `m2PlatformChecks`（6 项）与 `m2StrategyChecks`（4 项）分开构建 | `application/StageGateReport.py` |
| 仅 PASS 的冻结版本进入阶段 3 | `assertUniqueConclusion()`：非 PASS 抛出 | 同上 |

## 检查项内容

**平台正确性 gate（M2-P01~P06）：**
- 连续至少 60 个有效交易日运行
- trading-readiness 达到第 12.3 节 SLO
- 每日账本/订单/持仓对账差异为 0
- 重复副作用计数为 0
- 至少 3 次进程崩溃恢复演练且 RTO 达标
- 数据缺口全部隔离或在交易前补齐

**单策略晋级 gate（M2-S01~S04）：**
- 至少 50 个可执行信号
- 未成交/部分成交率、滑点、延迟落入预注册容许区间
- 风险硬限制违反 0
- 净收益与最大回撤满足冻结的 StrategyAcceptancePolicy

## 测试

`tests/unit/application/test_m2_gate_checks.py`（6 用例）：平台/策略检查项存在、
全 PASS 报告、单项 FAIL、开放 S1 阻断、平台与策略分别结论。

## 验证结果

- ruff / mypy / Preflight：通过
- 全量 pytest：待第八批 PR 后确认

## 风险与开放项

- 实际 M2 Gate 执行需 P2-040 模拟盘 60 日运行数据与演练/对账证据，
  本任务提供检查项与报告能力。
