# 单 Agent 开发模式治理材料

## 当前事实

当前已知的唯一开发执行代理为 `ClaudeCode`。其可负责代码实现、作者测试、维护脚本、CI 配置和证据整理；其不能替代人类项目负责人、Gate 签署人、独立 QA、SRE/安全审批人、Incident Commander 或值班联系人。

本文件不授予权限，也不构成范围、Gate、RACI 或风险接受的签署记录。

当前使用稳定角色别名 `ProjectAuthor` 指代唯一人类项目作者。ProjectAuthor 已于 `2026-07-31T04:15:00Z` 在当前任务会话确认临时角色模型；该别名不披露姓名或凭据，联系渠道仅限当前开发会话，不能用作生产值班或外部事故响应渠道。

## 临时职责边界

| 事项 | Accountable | Responsible | 当前状态 |
| --- | --- | --- | --- |
| P0 工程实现与作者验证 | ProjectAuthor/TL | ClaudeCode | 已有本地证据，等待独立复核 |
| 范围、非目标和阶段 Gate | ProjectAuthor/PO | ProjectAuthor | P0 范围和未来签署名单已确认；未通过任何 Gate |
| 技术方案和变更批准 | ProjectAuthor/TL | ClaudeCode 提供技术材料 | 临时职责已确认；每项实际变更仍须审阅 |
| 独立 QA 验证 | ProjectAuthor/QA | ProjectAuthor | 临时职责已确认；M0 前仍需 `ACT-P0-007` 的非作者人类复核 |
| CI、分支保护和安全例外 | ProjectAuthor/SRE | ProjectAuthor | 远程 CI 已有成功证据；分支保护与必需检查仍待管理员核验 |
| 事故指挥和值班响应 | ProjectAuthor/Incident Commander | ProjectAuthor | 临时职责已确认；事故替补仍缺失 |

同一位人类负责人可以在资源受限时承担多个角色，但必须明确记录角色冲突、替补人和 Gate 回避规则；ClaudeCode 不能自我批准其实现或为其作者测试签署独立验收。

## 单人作者临时角色分配

在仅有 `ProjectAuthor` 和 `ClaudeCode` 的当前阶段，建议采用下列最小模型。它适用于工程基线和历史回测开发，不能用来授权模拟盘、券商连接或实盘。

| 角色 | 临时承担者 | 必须完成的职责 | 冲突控制 |
| --- | --- | --- | --- |
| PO | ProjectAuthor | 批准范围、非目标、优先级和变更取舍 | 记录每个范围或 Change 决策依据。 |
| TL | ProjectAuthor | 审查技术方案一致性、架构边界和失败模式 | 对 ClaudeCode 产出的代码和文档逐项审阅。 |
| QA | ProjectAuthor | 复核运行命令、JUnit、coverage、失败或跳过项 | 仅当代码由 ClaudeCode 产出时，可作为非作者验证；ProjectAuthor 自行编写的代码不能由其自行独立验收。 |
| SRE/安全 | ProjectAuthor | 管理 Git、CI、Docker、许可证和凭据边界 | 不配置生产凭据，不启用实盘；安全例外必须保留期限和依据。 |
| Incident Commander | ProjectAuthor | 决定暂停、保全证据和升级 | 无替补时，任何事故或保护失败均暂停新风险工作，不进行环境晋级。 |
| 开发执行代理 | ClaudeCode | 实现、作者测试、证据整理和风险发现 | 不修改为 ACCEPTED、APPROVED、RELEASED 或 Gate PASS。 |

### 必须保留的限制

1. `ProjectAuthor` 可以同时承担上述治理角色，但每次范围、变更、安全和 Gate 决策必须在审计历史中显示其角色和 UTC 时间。
2. ClaudeCode 产生的改动必须由 ProjectAuthor 审阅证据和差异后才可进入 `IN_REVIEW`；ProjectAuthor 自己编写的改动需要后续引入非作者评审人，不能标记独立验收完成。
3. 在没有第二位人类值班或评审人的情况下，不创建 Release、不进入任何环境晋级，尤其不启用真实券商或实盘凭据。
4. 遇到 S0/S1 风险、账本不变量失败、未授权交易或保护动作失效时，ProjectAuthor 立即停止相关工作并保全证据；ClaudeCode 只提供诊断和修复材料。

## 最小确认模板

由人类项目负责人在可审计渠道确认下列内容；联系方式应使用企业目录引用、值班组别或工单链接，不记录个人电话、令牌或凭据。

```yaml
GovernanceConfirmation:
  ProjectOwner:
    Name: 待填写
    DirectoryReference: 待填写
    ConfirmedTs: 待填写UTC时间
  TechnicalLead:
    Name: 待填写
    DirectoryReference: 待填写
    ConfirmedTs: 待填写UTC时间
  QualityApprover:
    Name: 待填写
    DirectoryReference: 待填写
    ConfirmedTs: 待填写UTC时间
  SecurityApprover:
    Name: 待填写
    DirectoryReference: 待填写
    ConfirmedTs: 待填写UTC时间
  IncidentCommander:
    Name: 待填写
    DirectoryReference: 待填写
    OnCallWindow: 待填写
    ConfirmedTs: 待填写UTC时间
  ScopeDecision:
    Decision: APPROVED或CHANGES_REQUIRED
    EvidenceReference: 待填写
    ConfirmedTs: 待填写UTC时间
  GateSignerRoster:
    M0: [待填写]
    M1: [待填写]
    M2: [待填写]
    M3: [待填写]
    M4: [待填写]
    M5: [待填写]
```

ProjectAuthor 的确认已写入 `P0-001ScopeAndGateRecord.md`、`P0-002RaciAndEscalation.md` 和相应行动项审计历史。P0-001/P0-002 仍处于 `IN_REVIEW`，并不代表独立验收、Gate 通过或环境晋级。

## 单人作者确认语句

项目作者只需在可审计渠道确认以下事实，再补充其稳定身份和目录引用：

```text
我以 ProjectAuthor 身份暂时承担 PO、TL、QA、SRE/安全和 Incident Commander 角色。
ClaudeCode 仅负责开发执行与作者测试，不拥有审批、Gate 或环境晋级权限。
我接受单人角色冲突限制：不对本人编写的代码作独立验收，不创建 Release 或环境晋级；新增第二位人类评审/值班人前，实盘保持禁用。
```
