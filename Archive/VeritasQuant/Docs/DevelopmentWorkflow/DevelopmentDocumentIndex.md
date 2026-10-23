# 开发文档索引

新成员应从本页定位当前权威设计、计划、治理记录、风险和证据。技术行为以技术方案为唯一事实来源；归档文档不作为设计依据。

| 类别 | 位置 | 用途 |
| --- | --- | --- |
| 权威技术设计 | [VeritasQuantTechSpec.md](../VeritasQuantTechSpec.md) | 事件、账户、风控、执行、API、配置和验收契约。 |
| 开发计划 | [VeritasQuantDevelopmentPlan.md](../VeritasQuantDevelopmentPlan.md) | P0-P6 任务、DoR/DoD、里程碑和风险基线。 |
| 开发工作流 | [VeritasQuantDevelopmentWorkflow.md](../VeritasQuantDevelopmentWorkflow.md) | 状态机、评审、验证、Gate 和证据规则。 |
| ADR 索引 | [AdrIndex.md](../Adr/AdrIndex.md) | 架构决策记录；当前尚无已批准 ADR。 |
| 范围/Gate 待签记录 | [P0-001ScopeAndGateRecord.md](P0/P0-001ScopeAndGateRecord.md) | 代表资产、非目标和待签署人。 |
| RACI 与升级 | [P0-002RaciAndEscalation.md](P0/P0-002RaciAndEscalation.md) | 角色责任、联系人和事故升级路径。 |
| 项目授权登记 | [ProjectAuthorizationRegister.yml](Registers/ProjectAuthorizationRegister.yml) | ProjectAuthor 对开发执行代理的持续授权范围与不可授权边界。 |
| 工作项登记 | [WorkItemRegister.yml](Registers/WorkItemRegister.yml) | P0 工作项状态、依赖、证据和阻断。 |
| 风险登记 | [RiskRegister.yml](Registers/RiskRegister.yml) | 当前风险、责任、缓解与升级。 |
| 缺陷/事故/变更/行动项 | [BugRegister.yml](Registers/BugRegister.yml)、[IncidentRegister.yml](Registers/IncidentRegister.yml)、[ChangeRegister.yml](Registers/ChangeRegister.yml)、[ActionRegister.yml](Registers/ActionRegister.yml) | 正式登记模板与审计历史。 |
| 阶段追踪矩阵 | [TraceabilityMatrix.yml](Registers/TraceabilityMatrix.yml) | 技术契约、任务、测试和 Gate 映射。 |
| P0 工程实施记录 | [P0-003-P0-012ImplementationEvidence.md](P0/P0-003-P0-012ImplementationEvidence.md) | P0 作者实施边界、工件和未决证据。 |
| 单 Agent 治理材料 | [SingleAgentGovernanceMaterial.md](SingleAgentGovernanceMaterial.md) | ClaudeCode 的执行边界、待确认人类角色与最小确认模板。 |
| P0 测试证据 | [P0-003-P0-012TestEvidence.json](P0/P0-003-P0-012TestEvidence.json) | 机器可读 JUnit、coverage、环境、种子和工件哈希。 |
| P0 独立验收包 | [P0IndependentAcceptancePackage.md](P0/P0IndependentAcceptancePackage.md) | 非作者 QA/SRE 的精确复核命令、远程负例最小证据和签署字段。 |
| P0 验收启动记录 | [P0AcceptanceKickoff.md](P0/P0AcceptanceKickoff.md) | 本轮验收候选、范围、延期治理事项与 Gate 边界。 |
| P0 自动化验收证据 | [P0AcceptanceAutomationEvidence.md](P0/P0AcceptanceAutomationEvidence.md) | P0 验收运行的检查、测试、构建、wheel 验证与工件哈希。 |
| P0-007 远程负例证据 | [P0-007RemoteNegativeDrillEvidence.md](P0/P0-007RemoteNegativeDrillEvidence.md) | PR #2 的远程 Quality 失败、`main` 未变和演练分支清理事实。 |
| P0-003 定位演练 | [P0-003DocumentDiscoveryDrill.md](P0/P0-003DocumentDiscoveryDrill.md) | 非作者新成员 15 分钟定位演练的可填写记录。 |
| M0 Linux 验证证据 | [M0LinuxValidationEvidence.md](M0/M0LinuxValidationEvidence.md) | WSL Linux 大小写、构建、测试和仓库外 wheel 验证结果。 |
| M0 独立角色与审阅 | [M0IndependentReviewEvidence.md](M0/M0IndependentReviewEvidence.md) | 非作者评审、独立 QA/SRE 与 Incident Commander 替补的确认和审阅记录。 |
| M0 Gate 报告 | [M0StageGateReport.md](M0/M0StageGateReport.md) | P0-013 的强制证据检查表、指标、风险决议和人类签署位置。 |
| M1 Gate 报告（待签署） | [M1StageGateReport.md](M1/M1StageGateReport.md)、[M1PreReview.md](M1/M1PreReview.md) | P1-076 的强制检查清单 7 项、指标样本、风险决议和人类签署位置；签署前状态为 DRAFT。 |
| CI 治理记录 | [P0-006CiGovernance.md](P0/P0-006CiGovernance.md) | 必需检查、证据保留和待管理员配置。 |
| 测试证据规范 | [TestEvidencePolicy.md](TestEvidencePolicy.md) | 稳定测试 ID、JUnit、覆盖率、种子和哈希规范。 |
| 安全许可证策略 | [../../Configs/Security/LicensePolicy.yml](../../Configs/Security/LicensePolicy.yml) | 已批准的许可证白名单及漏洞处置 SLA。 |
| Docker 开发依赖 | [../../Docker/DevelopmentEnvironment.md](../../Docker/DevelopmentEnvironment.md) | 临时 PostgreSQL/Redis 的启动和清理说明。 |
| P1 实现证据 | [P1-001-P1-013Evidence.md](P1/P1-001-P1-013Evidence.md) | P1 作者测试、工件和残余风险。 |
| P1-019~026 实现证据 | [P1-019ParquetWriteEvidence.md](P1/P1-019ParquetWriteEvidence.md)、[P1-020DataManifestEvidence.md](P1/P1-020DataManifestEvidence.md)、[P1-021QualityRulesEvidence.md](P1/P1-021QualityRulesEvidence.md)、[P1-022OrderedMergeEvidence.md](P1/P1-022OrderedMergeEvidence.md)、[P1-023BarAggregationEvidence.md](P1/P1-023BarAggregationEvidence.md)、[P1-024FixtureChecksumsEvidence.md](P1/P1-024FixtureChecksumsEvidence.md)、[P1-025LogicalClockEvidence.md](P1/P1-025LogicalClockEvidence.md)、[P1-026EventBusEvidence.md](P1/P1-026EventBusEvidence.md) | Parquet 写入、DataManifest、质量规则、归并、聚合、夹具、逻辑时钟与事件总线证据。 |
| P1-027~040 实现证据 | [P1-027InboxEvidence.md](P1/P1-027InboxEvidence.md) 至 [P1-040AccountRoutingEvidence.md](P1/P1-040AccountRoutingEvidence.md) | inbox/事务/checkpoint/回测状态机/崩溃注入/恢复/账本模型与结算/预占/证券/期货/路由证据。 |
| P1-041~076 实现证据 | [P1-041SnapshotEvidence.md](P1/P1-041SnapshotEvidence.md) 至 [P1-076StageGateReportEvidence.md](P1/P1-076StageGateReportEvidence.md)（36 份） | 账户快照、ledger 随机序列、订单/状态机/回报、执行适配器、Bar 路径、执行模型、流动性、原子边界、model-based 测试、风控模型/规范化/关联/策略引擎/审批/控制/规则/原子风险/契约、策略基类/指标窗口/沙箱/示例策略/回测服务、绩效/双轨报告/工件索引/防前视/安全套件、端到端/跨平台/性能/审计/Gate 证据。 |

所有新增开发过程文档应保存于 `Docs/DevelopmentWorkflow/`；设计决策变更仍必须同步修改技术方案。
| P2-001 数据库迁移证据 | [P2-001PostgresSchemaEvidence.md](P2/P2-001PostgresSchemaEvidence.md) | 首版 PostgreSQL 事实表/投影表/索引/迁移器/CI database job 证据。 |
| P2 数据库迁移 | [Migrations/postgresql/](../../Migrations/postgresql/V1__initial_fact_and_projection_schema.sql) | 版本化数据库迁移（禁止运行时自动改表）。 |
| P2-002 inbox/outbox/租约证据 | [P2-002InboxOutboxLeaseEvidence.md](P2/P2-002InboxOutboxLeaseEvidence.md) | 数据库 inbox/outbox/单活租约与 fencing token 证据。 |
| P2-003 事件扇出证据 | [P2-003EventFanoutEvidence.md](P2/P2-003EventFanoutEvidence.md) | 共享事件持久化与确定性分区扇出证据。 |
| P2-004~007 实现证据 | [P2-004AccountGroupWorkerEvidence.md](P2/P2-004AccountGroupWorkerEvidence.md)、[P2-005RiskSnapshotEvidence.md](P2/P2-005RiskSnapshotEvidence.md)、[P2-006PaperTradingEvidence.md](P2/P2-006PaperTradingEvidence.md)、[P2-007CheckpointReconciliationEvidence.md](P2/P2-007CheckpointReconciliationEvidence.md) | 账户组 worker、风险快照屏障、纸上交易、checkpoint/对账证据。 |
| P2-008~014 实现证据 | [P2-008StreamTransportEvidence.md](P2/P2-008StreamTransportEvidence.md)、[P2-009BackpressureEvidence.md](P2/P2-009BackpressureEvidence.md)、[P2-010-011FundNavEvidence.md](P2/P2-010-011FundNavEvidence.md)、[P2-012FundEventsEvidence.md](P2/P2-012FundEventsEvidence.md)、[P2-013-014FundStateMachinesEvidence.md](P2/P2-013-014FundStateMachinesEvidence.md) | 传输/背压/基金净值/基金事件/申赎状态机证据。 |
| P2-015~021 实现证据 | [P2-015-016SharesBudgetEvidence.md](P2/P2-015-016SharesBudgetEvidence.md)、[P2-017-018PlanCalendarAmountEvidence.md](P2/P2-017-018PlanCalendarAmountEvidence.md)、[P2-019SmartPlansEvidence.md](P2/P2-019SmartPlansEvidence.md)、[P2-020FundDslEvidence.md](P2/P2-020FundDslEvidence.md)、[P2-021PortfolioAllocationEvidence.md](P2/P2-021PortfolioAllocationEvidence.md) | 份额/入金/日历/金额/方案/DSL/分配证据。 |
