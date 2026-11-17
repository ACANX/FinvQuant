# P0-010 登记表规范

`WorkItemRegister.yml`、`BugRegister.yml`、`RiskRegister.yml`、`IncidentRegister.yml`、`ChangeRegister.yml` 与 `ActionRegister.yml` 是当前版本化登记表，统一存放于 `Docs/DevelopmentWorkflow/Registers/`。每个文件均声明 `RegistryVersion`、`RecordType` 和 `RequiredFields`；记录 ID 在本表范围内不可复用。

状态机以 [VeritasQuantDevelopmentWorkflow.md](../../VeritasQuantDevelopmentWorkflow.md) 第 4 节为唯一依据：工作项、缺陷、风险、事故、变更和行动项不能发明私有终态。每次状态变化均追加 `AuditHistory`，其中记录 UTC `Ts`、操作者、动作和证据位置。`Blocked` 仅是工作项状态标志，必须填写原因、阻断方、下一动作、责任人和升级时间。

尚未有外部基线日期时，`TargetDate`、`DueDate` 与 `EscalateTs` 显式标为“待指定”，不得伪造日期或将其解释为已批准时限。首次把登记表迁移到工作项平台时，必须保留本文件的 ID、审计历史和引用关系。

## 2026-08-01 独立复核快照

- 复核时间：`2026-08-01T20:25:00Z`。
- 复核对象：`WorkItemRegister.yml`（45 条）、`BugRegister.yml`（3 条）、`RiskRegister.yml`（6 条）、`IncidentRegister.yml`（0 条）、`ChangeRegister.yml`（4 条）与 `ActionRegister.yml`（8 条），以及 `P0-010RegisterSchema.md`。
- 每份登记表均声明 `RegistryVersion`、`RecordType` 和 `RequiredFields`；记录 ID 在各自范围内唯一（脚本校验无重复）。
- 每条现存记录均带有 `AuditHistory`（UTC `Ts`、操作者、动作与证据位置）；状态机引用 `VeritasQuantDevelopmentWorkflow.md` 第 4 节，无私有终态。
- 契约测试 `P0-010-001`（工作项治理字段与审计历史）与 `P0-010-002`（登记表必填字段与唯一 ID）均通过。

本快照证明 P0-010 验收标准（每类记录有唯一 ID、状态机、责任人、时限、证据链接和审计历史）的实现证据与规范相符。登记表迁移到工作项平台前的独立复核仍作为 M0 Gate 前的治理行动保留，不替代或改变本工作项的验收。
