# P2-037 建立 SLO 计算、错误预算和告警路由 — 证据

- **任务：** P2-037（ISSUE #166）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** 待第六批 PR 合并后回填

## 验收标准对照

| 验收标准 | 实现 | 证据 |
|----------|------|------|
| 指标按模式计算 | `SloPolicyV1` 按执行模式（PAPER/SIMULATION/LIVE）定义阈值 | `monitoring/SloCalculator.py` `_TARGETS`（对齐 TechSpec 12.3 表） |
| 正确性指标零预算 | 账本不平/控制丢失/跨账户路由/未授权命令零错误预算，一次违约即 EXCEEDED | `SloTargetV1.zeroBudget` + `_evaluateSli` |
| 告警含 run/account/处置链接 | `AlertRouteV1` 含 runId/accountGroup/remediationLink/dedupeKey | `route()` 生成 `run/{runId}/accounts/{group}/slo/{sli}` 链接 |
| 样本不足不自动通过 | 无观测时 INSUFFICIENT_EVIDENCE | `_evaluateSli` 空观测分支 |

## 技术方案

- 滚动 30 交易日窗口聚合（`evaluate` / `evaluateAccountGroup`），
  跨账户隔离评估：串扰不得混入同一窗口；
- 可用率类指标按违约日比例计算剩余预算；零预算指标任何一次违约即失败；
- 告警写入指标（`vq_slo_alerts_total` / `vq_slo_alerts_pending`），
  支持 `resolve()` 处置闭环；dedupe key 为 sha256 稳定摘要。

## 测试

`tests/unit/monitoring/test_slo_calculator.py`：
- 空观测证据不足；全达标预算 1.0；全违约预算 0；
- 部分违约预算比例；零预算一次违约即失败；窗口过滤旧观测；
- 账户组隔离（g1 违约不影响 g2）；
- 告警路由：正确性指标 P1、延迟类 P2、处置链接、dedupe 稳定、resolve 移除、指标写入。

## 验证结果

- ruff / mypy：通过
- 全量 pytest：971 passed（含本任务新增测试）

## 风险与开放项

- 运行期 SLI 观测接入（事件延迟/账本延迟埋点）由 TradingWorker 在第六批合并后接线，
  本任务提供计算与路由能力。
