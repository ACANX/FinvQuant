# P0-002 RACI、值班联系人与升级路径

## 记录状态

- 工作项：`P0-002`
- 状态：`IN_REVIEW`
- 说明：ProjectAuthor 已于 `2026-07-31T04:15:00Z` 在当前任务会话确认临时多角色模型、开发阶段联系渠道和值班限制。该确认不是 M0 Gate 通过，也不能替代 `ACT-P0-007` 要求的非作者人类复核与事故替补。

## RACI

| 交付或决策类别 | Accountable | Responsible | Consulted | Informed |
| --- | --- | --- | --- | --- |
| 范围、阶段 Gate、非目标 | PO | PO/TL | QA、SRE | 团队 |
| 技术方案、ADR、架构边界 | TL | TL/CE | QA、SRE | PO |
| 工程包、依赖和构建 | CE | CE | QA | TL |
| 测试策略、验收和证据 | QA | QA | TL、CE | PO |
| CI、环境、秘密和依赖安全 | SRE | SRE | TL、QA | PO |
| 风险登记和例外 | PO | 风险记录责任角色 | TL、QA、SRE | 团队 |
| S0/S1 事故指挥 | Incident Commander | Operations Lead | TL、SRE、QA | PO |
| Gate 证据审查 | PO | TL/QA | SRE、CE | 团队 |

每行只有一个 Accountable 角色。当前由 ProjectAuthor 临时承担这些角色；其联系渠道仅适用于开发阶段，不能视为生产值班或环境晋级授权。

## 单 Agent 开发执行边界

| 开发执行代理 | 可负责事项 | 不可负责事项 | 当前确认来源 |
| --- | --- | --- | --- |
| Codex | 开发范围内的代码、测试、CI、仓库治理和证据维护 | Gate 签署、独立 QA 验收、实盘启用、风险例外、生产凭据和破坏性操作 | [ProjectAuthorizationRegister.yml](../Registers/ProjectAuthorizationRegister.yml) `AUTH-DEV-001` |

具体人类角色与确认材料使用 [SingleAgentGovernanceMaterial.md](../SingleAgentGovernanceMaterial.md)。`ACT-P0-006` 已记录临时治理确认；M0 前仍须完成 `ACT-P0-007`。

在仅有一位人类项目作者时，ProjectAuthor 暂时承担 PO、TL、QA、SRE/安全和 Incident Commander 角色，ClaudeCode 只作开发执行代理。该分配已由 ProjectAuthor 明确确认，仍受材料中的角色冲突、独立验收和实盘禁止限制约束。

## 升级路径

| 触发 | 首责角色 | 升级对象 | 时限 | 记录位置 |
| --- | --- | --- | --- | --- |
| S0 安全/资金正确性风险 | Incident Commander | PO、TL、SRE | 5 分钟确认 | IncidentRegister.yml |
| S1 重复副作用、串账户、前视 | TL | PO、QA、SRE | 15 分钟确认 | IncidentRegister.yml |
| 关键路径阻断超过 5 工作日 | PO | PO/TL | 当日升级 | RiskRegister.yml |
| CI、构建或安全门禁失败 | QA/SRE | TL | 当前工作日 | WorkItemRegister.yml |

## 已确认的开发阶段联系人

| 角色 | 姓名 | 联系方式 | 值班时段 | 本人确认时间 |
| --- | --- | --- | --- | --- |
| PO | ProjectAuthor | 当前任务会话（仅开发阶段） | 按需响应；无生产值班 | 2026-07-31T04:15:00Z |
| TL | ProjectAuthor | 当前任务会话（仅开发阶段） | 按需响应；无生产值班 | 2026-07-31T04:15:00Z |
| QA | ProjectAuthor | 当前任务会话（仅开发阶段） | 按需响应；无生产值班 | 2026-07-31T04:15:00Z |
| SRE/安全 | ProjectAuthor | 当前任务会话（仅开发阶段） | 按需响应；无生产值班 | 2026-07-31T04:15:00Z |
| Incident Commander | ProjectAuthor | 当前任务会话（仅开发阶段） | 发生 S0/S1 时停止新风险工作并保全证据 | 2026-07-31T04:15:00Z |

ProjectAuthor 的联系渠道不用于生产值班或外部事故响应；在 M0 前必须通过 `ACT-P0-007` 指定独立人类评审者和事故替补。
