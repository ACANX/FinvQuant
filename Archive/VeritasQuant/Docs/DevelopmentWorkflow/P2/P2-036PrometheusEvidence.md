# P2-036 接入 Prometheus 指标和 Grafana 基础看板 — 证据

- **任务：** P2-036（ISSUE #165）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** 待第六批 PR 合并后回填

## 验收标准对照

| 验收标准 | 实现 | 证据 |
|----------|------|------|
| 覆盖 readiness | 三层门禁状态 + 未通过检查数 | `monitoring/MetricsCollector.py` `collectReadiness` |
| 事件延迟 | 行情 ingested_at -> 分区提交延迟直方图 | `observeEventIngest` + `WallClockLatencyRecorder` |
| 订单 | 状态机转移计数 + 拒绝计数 | `observeOrderTransition` / `observeOrderRejection` |
| 账本 | 事务提交延迟直方图 + 提交计数 | `observeLedgerCommit` |
| outbox | 最老未确认年龄 / 条数 | `collectOutbox` |
| 队列 | 利用率 / 待处理 / 连接状态 | `collectQueue` |
| 错误码 | API 业务错误码计数 | `collectErrorCodes` |
| 日志降级 | 降级状态 + 丢弃计数 | `collectLogState` |

## 技术方案

- **纯标准库实现 Prometheus 文本格式 0.0.4**（`monitoring/PrometheusMetrics.py`），
  不引入 prometheus-client 外部依赖，避免许可证审批与锁文件变更；
- `MetricsRegistry`：Counter/Gauge/Histogram + 标签，线程安全，确定性排序导出；
- `/metrics` 端点（`apps/server/MetricsRoutes.py`）：text/plain; version=0.0.4，
  天然豁免统一信封中间件（只处理 application/json），抓取只读；
- 采集器只读复制状态，绝不修改交易状态或作出交易决定（TechSpec 3.1 虚线旁路）。

## 测试

- `tests/unit/monitoring/test_prometheus_metrics.py`：注册表三类指标、
  桶累计、标签、命名空间、冲突类型拒绝、非法名称拒绝；
- `tests/unit/apps/test_metrics_routes.py`：文本格式、豁免信封、只读无副作用。

## 验证结果

- ruff / mypy：通过
- 全量 pytest：971 passed（含本任务新增测试）

## 风险与开放项

- Grafana 看板 JSON 需要运维侧导入（本任务提供指标出口，看板配置为部署工件）。
