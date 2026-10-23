# P4-002~006 券商仿真适配层 — 证据

- **任务：** P4-002（ISSUE #184）、P4-003（#185）、P4-004（#186）、P4-005（#187）、P4-006（#188）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** ACANX/VeritasQuant#239（已合并 2026-08-02T23:21:40Z）

## 范围

阶段 4 券商仿真适配层核心链路：统一 BrokerPort 能力协商 → 仿真券商认证/
会话/凭据 → 订单发送/受理/拒绝/撤单/查询映射 → 异步回报/缺口/迟到/更正
映射 → 开盘/盘中/收盘及重连对账。

## 验收标准对照

| 任务 | 验收标准 | 实现 | 测试证据 |
|------|----------|------|----------|
| P4-002 | 不支持能力在发单前拒绝；第三方字段只停留在适配边界 | `BrokerCapabilityV1`（能力清单）；`BrokerPort`（统一端口）；`CapabilityNegotiatorV1`（发单前协商：订单类型/有效期/标的/方向任一不支持即拒绝） | `tests/unit/broker/test_broker_port.py`（10 用例） |
| P4-003 | 凭据不入代码/配置/日志；过期、撤销和轮换测试通过 | `BrokerCredentialV1`（repr 打码）；`CredentialResolver` 注入端口；`SessionManagerV1`（令牌只存哈希；过期/撤销/轮换；最小权限集合） | `tests/unit/broker/test_broker_session.py`（11 用例） |
| P4-004 | client/broker ID 可追溯；超时进入 RECONCILIATION_REQUIRED，不盲目重发 | `BrokerOrderMappingV1`（双向映射）；`OrderStatusMapperV1`；`SimBrokerOrderGatewayV1`（能力协商+认证+发单/撤单/查询；超时/结果未知 → TIMEOUT_UNKNOWN 不重发） | `tests/unit/broker/test_broker_order_gateway.py`（9 用例） |
| P4-005 | 重复/乱序/断线重放不重复记账；未知订单隔离并查询 | `ReportDeduplicatorV1`；`ReportSequenceGuardV1`（缺口/迟到/重复判定）；`ReportCorrectionV1`（更正替换不新增记账）；`UnknownOrderIsolationV1`（隔离+查询） | `tests/unit/broker/test_report_handling.py`（10 用例） |
| P4-006 | 本地与券商订单、成交、持仓、现金逐项核对；未解释差异阻止交易 | `BrokerReconcilerV1`（开盘/盘中/收盘/重连四时点；逐项核对；blocking 门禁）；`ReconciliationReportV1` | `tests/unit/broker/test_reconciliation.py`（10 用例） |

## 技术方案要点

- 新建 `broker/` 领域包（券商适配边界），第三方字段只停留在
  `BrokerReportV1`/`OrderRequestV1` 适配模型，不扩散到执行/账本领域；
- 凭据安全：repr 打码、令牌只存 SHA-256、凭据经注入端口解析、
  绝不打日志；轮换后旧会话 T+0 有效、撤销立即失效；
- 超时/结果未知绝不盲目重发（重复副作用 0）——进入 TIMEOUT_UNKNOWN /
  RECONCILIATION_REQUIRED；
- 金额/数量路径全程 Decimal 字符串，禁止 float；
- 对账差异门禁：任一未解释差异 -> blocking=True 阻止交易。

## 验证结果

- ruff：All checks passed
- mypy：Success（broker 6 源文件）
- Preflight：0 issues
- 全量 pytest：1154 passed / 36 skipped（skipped 为 PG/Redis 集成，CI database job 覆盖）
- 新增测试：50 用例（P4-002~006）
- CI 4/4：Quality-ubuntu ✅ / Quality-windows ✅ / Security-baseline ✅ / Database-migrations ✅（PR #239 run 30771633803）

## 验收结论

- **状态：** P4-002~006 全部 ACCEPTED（2026-08-03，流转 PR 本 PR）
- **依据：** 券商仿真适配层能力（BrokerPort/认证/订单网关/回报处理/对账）已交付并通过 CI；验收标准逐项对照见上表
- **遗留：** P4-001 环境选择待 ACANX 决策；P4-007~010 为下一批开发；P4-011~014 为运行/Gate 类需仿真环境

## 风险与开放项

- P4-001（选择券商仿真环境并冻结能力/限制清单）为 PO/TL 决策类任务，
  本批以 `BrokerCapabilityV1` 结构化表达冻结清单；具体环境选择待 ACANX 决定。
- P4-007~010（诊断时间/校准/AB/测试）为下一批开发；P4-011~014 为
  运行/Gate 类任务，需仿真环境与 20 交易日窗口。
