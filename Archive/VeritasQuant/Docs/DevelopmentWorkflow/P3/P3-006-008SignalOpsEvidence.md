# P3-006~008 信号运维与安全测试 — 证据

- **任务：** P3-006（ISSUE #178）、P3-007（#179）、P3-008（#180）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** ACANX/VeritasQuant#237（已合并 2026-08-02T22:49:01Z）

## 范围

阶段 3 信号参考闭环的运维与安全加固：信号端到端延迟 SLI、信号/人工偏差
分析报告、通知故障/重复确认/权限撤销测试。

## 验收标准对照

| 任务 | 验收标准 | 实现 | 测试证据 |
|------|----------|------|----------|
| P3-006 | 可计算事件可用至送达 p50/p95/p99；缺样本不判通过 | `SignalLatencyEvaluatorV1`（线性插值百分位；99.5% 10 秒内送达目标；无样本 → INSUFFICIENT_EVIDENCE）；`SignalLatencyAlertV1`（run/account/处置链接） | `tests/unit/signals/test_signal_latency.py`（13 用例） |
| P3-007 | 每条未执行或偏差有结构化原因；账户记录差异可定位 | `SignalDeviationAnalyzerV1`（NOT_EXECUTED/DIRECTION/QUANTITY/PRICE_SLIPPAGE）；未执行信号原因必须人工提供，analyzer 不伪造（计入未解释偏差）；偏差记录强制结构化原因 | `tests/unit/signals/test_deviation_report.py`（12 用例） |
| P3-008 | 重复副作用 0；权限撤销后不能操作；P0/P1 控制不受通知状态影响 | `tests/integration/test_p3_signal_ops_safety.py`：通知失败信号保持 PENDING；重复路由/动作/命令/确认不产生重复副作用；RBAC 默认拒绝 + 撤销后越权拒绝 | `tests/integration/test_p3_signal_ops_safety.py`（10 用例） |

## 技术方案要点

- **P3-006**：延迟样本 = deliveredTs - eventAvailableTs（送达不得早于可用）；
  百分位用线性插值（对齐标准 p50/p95/p99 语义）；单样本即可计算但真实
  gate 由 P3-009 的 50 条信号证据窗口保证；告警仅在 SUFFICIENT + 违约时生成。
- **P3-007**：偏差分析器不替人工编造原因——未执行信号计入未解释偏差
  （clean=False），方向/数量/滑点偏差优先采用人工登记的 deviationReason，
  缺失时用结构化默认原因（仍可追溯）；滑点容差 0.5% 可配置。
- **P3-008**：交叉契约验证（通知 × 人工动作 × 命令 × RBAC）——通知路由层
  无交易副作用；命令幂等键保证重复提交不重复执行；确认仅限 DELIVERED
  状态（重复确认被拒）；RBAC 默认拒绝 + 账户范围校验。

## 验证结果

- ruff：All checks passed
- mypy：Success（signals 8 源文件）
- Preflight：0 issues
- 全量 pytest：1104 passed / 36 skipped（skipped 为 PG/Redis 集成，CI database job 覆盖）
- 新增测试：35 用例（P3-006: 13 + P3-007: 12 + P3-008: 10）
- CI 4/4：Quality-ubuntu ✅ / Quality-windows ✅ / Security-baseline ✅ / Database-migrations ✅（PR #237 run 30770687682）

## 验收结论

- **状态：** P3-006~008 全部 ACCEPTED（2026-08-03，流转 PR 本 PR）
- **依据：** 信号延迟 SLI、偏差分析与安全测试能力已交付并通过 CI；验收标准逐项对照见上表
- **遗留：** 延迟 SLI 真实样本依赖模拟盘 20 交易日运行与 50 条信号（P3-009），M3 Gate 前补充

## 风险与开放项

- 延迟 SLI 的真实样本依赖模拟盘 20 交易日运行与 50 条信号（P3-009）；
- 偏差分析的"账户记录差异可定位"在真实账本对账中由 P2-042 工具承接，
  本任务提供信号侧偏差归因。
