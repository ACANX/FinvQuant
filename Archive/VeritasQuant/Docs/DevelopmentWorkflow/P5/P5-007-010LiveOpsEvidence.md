# P5-007~010 实盘运行与监控 — 证据

- **任务：** P5-007（ISSUE #203）、P5-008（#204）、P5-009（#205）、P5-010（#206）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** 本 PR（P5 第二批）

## 范围

阶段 5 实盘运行与监控：实盘适配器/幂等下单/权威对账 → 独立紧急停止 →
生产 trading-readiness 门禁 → 生产监控/分页告警/24x7 联系树。

## 验收标准对照

| 任务 | 验收标准 | 实现 | 测试证据 |
|------|----------|------|----------|
| P5-007 | 发送结果未知不生成新 ID；每日现金/持仓/订单/成交差异为 0 | `LiveBrokerAdapterV1`（未知结果复用原 ID 不生成新 ID；同 clientOrderId 重试幂等）；`AuthorityReconcilerV1`（券商为权威逐项核对，差异阻止交易） | `tests/unit/broker/test_live_broker.py`（11 用例） |
| P5-008 | 紧急停止不依赖 GUI/通知；恢复前控制、账本和对账全部校验 | `EmergencyStopControllerV1`（独立通道触发 STOP_ALL/REDUCE_ONLY；状态机）；恢复审批（控制健康+账本平衡+对账干净+双人批准） | `tests/unit/broker/test_emergency_stop.py`（9 用例） |
| P5-009 | 行情、时钟、券商、账本、控制、队列、磁盘、沙箱任一不合格即禁止发单 | `TradingReadinessGateV1`（八类检查；任一 FAIL 或未执行 → 禁止发单；最新评估控制发单） | `tests/unit/broker/test_live_readiness_gate.py`（6 用例） |
| P5-010 | S0/S1 告警端到端送达；无人确认自动升级；每个告警链接 Runbook | `PagingServiceV1`（送达确认、S0/S1 到期自动升级路径 1→2→3、Runbook 链接）；`OnCallContactV1`（24x7 联系树） | `tests/unit/broker/test_paging.py`（10 用例） |

## 技术方案要点

- 幂等下单：未知结果不生成新 ID，同 clientOrderId 重试返回原结果；
- 权威对账以券商侧为权威，本地缺失/状态不一致即差异并阻止交易；
- 紧急停止控制器无 GUI/通知依赖（独立通道）；恢复必须双人批准 +
  控制/账本/对账全过；
- 生产门禁八类检查，未执行的检查视为 FAIL（不静默通过）；
- 分页告警 S0/S1 到期（15 分钟）未确认自动升级，每告警含 Runbook 链接。

## 验证结果

- ruff：All checks passed
- mypy：Success（broker 13 源文件）
- Preflight：0 issues
- 全量 pytest：1291 passed / 36 skipped（skipped 为 PG/Redis 集成，CI database job 覆盖）
- 新增测试：36 用例（P5-007: 11 + P5-008: 9 + P5-009: 6 + P5-010: 10）

## 风险与开放项

- 真实券商适配器接入需 P4-011~014 仿真环境与 M4 Gate 通过；
- P5-011~014（审计/备份/Runbook/冻结）为下一批；P5-015~022 含演练/运行/Gate 类任务。
