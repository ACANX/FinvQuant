# M0 阶段 Gate 报告

## 报告控制

- 报告 ID：`M0-STAGE-GATE-001`
- 报告版本：`1.0`
- `StageGatePolicyVersion`：`V1`（依据《VeritasQuantTechSpec》第 13 章与 `StageGatePolicyVersion` 登记基线）
- 证据窗口：`2026-07-31T12:31:58Z`（P0 验收启动）至 `2026-08-01T20:42:19Z`（P0-013 归档，PR #114 合并）
- 候选提交：`16af7884563f86aee0fb11273fbc2c0e95f02ffb`（上游 `dev` HEAD）
- 报告状态：`FINAL`
- Gate 结论：`PASS`

本文件是 P0-013 的可填写 Gate 工件，签署后冻结首个迭代 Backlog。根据
[技术方案第 13 章](../../VeritasQuantTechSpec.md#13-分阶段实施路线与验收结果)，最终报告必须记录
政策版本、证据窗口、样本量、指标与阈值、测试报告哈希、审批签名以及唯一的 `PASS`、`FAIL`
或 `INSUFFICIENT_EVIDENCE` 结论。

## 审查范围与排除项

- 范围：M0/P0 工程基线、治理记录和首个迭代 Backlog 冻结条件。
- 不在范围：环境晋级、Release 创建、模拟盘、券商连接、实盘授权或凭据。
- 代表资产和阶段边界：见 [P0-001 范围记录](../P0/P0-001ScopeAndGateRecord.md)。
- 预审更新：见 [M0PreReview.md](M0PreReview.md) 的 2026-08-01 独立复核快照；P0-001 至 P0-013
  已全部 `ACCEPTED`，阻断项均已处理或由 ProjectAuthor 批准例外。

## 强制检查表

| 检查 | 所需客观证据 | 当前状态 | 独立审阅结论与引用 |
| --- | --- | --- | --- |
| P0-001 范围、非目标与未来 Gate 签署名单 | [P0-001 范围记录](../P0/P0-001ScopeAndGateRecord.md)；非作者 PO/TL 审阅记录 | 通过 | ProjectAuthor 于 2026-07-31T04:15:00Z 确认范围与 Gate 签署名单；PR #97 归档 |
| P0-002 RACI、升级和值班替补 | [P0-002 RACI](../P0/P0-002RaciAndEscalation.md)；[独立角色证据包](M0IndependentReviewEvidence.md) 中的评审者与 IC 替补确认 | 通过 | ProjectAuthor 确认临时多角色模型与 5/15 分钟升级路径；PR #98 归档；IC 替补确认见独立角色证据包 |
| P0-003 文档可定位性 | [文档索引](../DevelopmentDocumentIndex.md)；已完成的 [定位演练记录](../P0/P0-003DocumentDiscoveryDrill.md) | 通过 | ACANX 独立执行定位演练 10/10 正确，签署 [P0-003HumanDrillWorksheet.md](../P0/P0-003HumanDrillWorksheet.md)（2026-08-01T16:41:00Z） |
| P0-004 方案 A 目录边界 | [工作项证据](../Registers/WorkItemRegister.yml) 中 P0-004 的脚本、契约测试和 Linux 验证；非作者 TL 审阅 | 通过 | 契约测试 P0-004-001 通过；M0 Linux 验证 `58 passed`；PR #101 归档 |
| P0-005 Python 3.13+ 与锁定策略 | [依赖策略](../P0/P0-005DependencyPolicy.md)；双平台 CI 工件和独立 QA 记录 | 通过 | 精确锁文件、`VerifyDependencyLocks.py` 0 issues；PR #102 归档 |
| P0-006 CI 合并治理 | [CI 治理记录](../P0/P0-006CiGovernance.md)；管理员分支保护审计链接；三项必需检查配置证据 | 通过 | Ruleset `20114806`（dev）与 `20108335`（main）三项必需检查、空 bypass；截图 `BypassList_2026-07-31 194302.png`；PR #103 归档 |
| P0-007 违规阻断与定位 | [P0 独立验收包](../P0/P0IndependentAcceptancePackage.md) 中的受保护远程 CI 负例证据 | 通过 | 负例 Run [`30626945620`](https://github.com/ACANX/VeritasQuant/actions/runs/30626945620) 明确失败并定位；修复后 Run `30716079258` 全绿；PR #108 归档 |
| P0-008 Compose 演练 | [Compose 技术演练](../P0/P0-008ComposeDrillEvidence.md)；独立 SRE 复核 | 通过 | 启动/健康/清理演练退出码 0；镜像隔离与无遗留复查通过；PR #109 归档 |
| P0-009 测试证据格式 | [P0 独立验收包](../P0/P0IndependentAcceptancePackage.md) 中的 JUnit、coverage、环境和哈希复核 | 通过 | `CollectTestEvidence.py` 端到端验证（JUnit/coverage/环境/种子/工件 SHA-256）；PR #110 归档 |
| P0-010 登记表 | [登记表规范](../P0/P0-010RegisterSchema.md)；六类登记表；非作者 TL/QA 复核 | 通过 | 六类登记表唯一 ID、审计历史齐全；契约测试 P0-010-001/002；PR #111 归档 |
| P0-011 安全与许可证 | [P0 独立验收包](../P0/P0IndependentAcceptancePackage.md) 中的 Security baseline、负例和许可证复核 | 通过 | 秘密扫描负例阻断（退出码 1）；许可证 APPROVED 且 0 issues；pip_audit 持续 SUCCESS；PR #112 归档 |
| P0-012 追踪矩阵 | [P0 独立验收包](../P0/P0IndependentAcceptancePackage.md) 中的映射复核 | 通过 | R-001 至 R-017 全量映射、无孤立需求；契约测试 P0-012-001；PR #113 归档 |
| 开放风险、事故、变更和行动项 | [风险](../Registers/RiskRegister.yml)、[事故](../Registers/IncidentRegister.yml)、[变更](../Registers/ChangeRegister.yml)、[行动项](../Registers/ActionRegister.yml)；阻断项为零或有已批准且未过期的例外 | 通过 | 阻断行动项 ACT-P0-004/005/007 已关闭（见 ActionRegister.yml 与风险决议表）；无未批准开放阻断 |

## 必须附加的治理证据

1. **分支保护配置审计**：GitHub Ruleset `20108335`（main）与 `20114806`（dev）均启用
   `Quality (ubuntu-latest)`、`Quality (windows-latest)`、`Security baseline` 三项必需检查；
   要求至少一名审批者、Code Owner 审阅、解决评审会话与最近推送者审批；`bypass_actors`
   为空。修正前后截图：
   - 修正前 `Archive/Asset/Image/BypassList_2026-07-31 193936.png`（`Repository admin = Always allow`）
   - 修正后 `Archive/Asset/Image/BypassList_2026-07-31 194302.png`（bypass list 为空）
2. **P0-007 故意违规失败演练**：Run [`30626945620`](https://github.com/ACANX/VeritasQuant/actions/runs/30626945620)
   在受保护 `main` 上因故意引入的 PascalCase 违规文件失败，Quality 任务定位到
   `Configs/P0NegativeGate20260802c.yml: invalid_field`；演练分支已清理；修复后
   Run `30716079258` 全绿。
3. **人类评审者与 Incident Commander 替补**：ACANX 作为非作者评审者独立执行 P0-003
   定位演练并签署（`P0-003HumanDrillWorksheet.md`，10/10）；IC 替补确认见
   [M0IndependentReviewEvidence.md](M0IndependentReviewEvidence.md)。
4. **独立 QA/SRE 复核**：P0-003~P0-013 的独立复核快照分别归档于各 P0 证据文档
   （PR #97~#114）；本地复核命令与预期哈希见 [P0IndependentAcceptancePackage.md](../P0/P0IndependentAcceptancePackage.md)。

## 指标、样本和工件哈希

| 项目 | 阈值/要求 | 实际值 | 证据哈希或不可变链接 |
| --- | --- | --- | --- |
| 双平台 CI | Windows/Linux Python 3.13 质量任务与 Security baseline 成功 | Run `30717556159`（dev `16af788`）三 job 全部成功；P0 验收期间各 PR 均全绿 | https://github.com/ACANX/VeritasQuant/actions/runs/30717556159 |
| CI 合并阻断 | 三项必需检查已启用，普通管理员绕过已禁用 | 已启用；bypass_actors 为空 | Ruleset `20108335`/`20114806`；截图 `BypassList_2026-07-31 194302.png` |
| P0-007 负例 | 受保护 CI 明确失败并定位违规 | 失败并定位 `Configs/P0NegativeGate20260802c.yml: invalid_field` | Run `30626945620` |
| P0-003 定位演练 | 非作者在 15 分钟内完成全部必达项 | 10/10 正确，用时约 10 分钟（16:31-16:40 UTC） | `P0-003HumanDrillWorksheet.md` |
| Compose 演练 | 启动、健康、停止与无遗留清理由独立 SRE 复核 | 4 条命令退出码均为 0；停止后容器/网络/卷为 0 | `P0-008ComposeDrillEvidence.md` |
| 测试证据 | JUnit、coverage、环境、种子和工件 SHA-256 齐全 | JUnit 58 tests 0 失败；coverage/工件 SHA-256 已归档 | `P0-003-P0-012TestEvidence.json` |
| 开放阻断项 | 0 | 0（ACT-P0-004/005/007 已关闭或批准例外） | `ActionRegister.yml` |

测试证据哈希（`P0-003-P0-012TestEvidence.json`）：
- JUnit：`1beaa11fe4042d9264e89e44de6d5e8297e06e7c7bd7b3a4a5c7fd31a694dab1`
- Coverage：`07b67c62bf6c607fa8a0769f9a90777f299b66d1077b73c0cc1bb41ceece9d7d`
- wheel：`3c49c2360133806980b0f143522ed6e933c6ad051b9de76c25bf275fbf991310`
- sdist：`247017c1ef2a41061d0983e7c555ceeb2d6aee6e24c8dc3e5123da19cd4bc7cc`

## 风险、例外与决议

| 记录 ID | 当前状态 | Gate 影响 | 决议/批准人/到期时间 |
| --- | --- | --- | --- |
| `RSK-P0-001` | 已接受 | 人员独立性与值班替补 | ProjectAuthor 批准临时多角色模型；独立评审与 IC 替补确认见独立角色证据包 |
| `RSK-P0-002` | 已缓解 | 依赖、CI 与独立验收 | 双平台 CI 与独立复核快照已归档；残余治理复核延后至后续治理验收 |
| `RSK-P0-003` | 已缓解 | 分支保护与必需检查 | Ruleset 三项必需检查 + 空 bypass；管理员变更审计延后归档 |
| `RSK-P0-004` | 已关闭 | Compose 独立 SRE 复核 | P0-008 独立复核通过（PR #109）；ACT-P0-005 已关闭 |
| `RSK-P0-005` | 已缓解 | P0-006 执行顺序 | Windows 开发验证已完成，Linux CI 已实施 |
| `RSK-P0-006` | 已缓解 | 治理复核延后 | ProjectAuthor 2026-07-31T12:35:00Z 延后，不阻断 P0 收尾 |
| `ACT-P0-004` | 已关闭 | 必须关闭 | 三项必需检查启用 + 空 bypass 证据归档（2026-08-01T20:45:00Z） |
| `ACT-P0-005` | 已关闭 | 必须关闭 | Compose 启停/健康/清理演练完成（P0-008，2026-08-01T20:45:00Z） |
| `ACT-P0-007` | 已关闭 | 必须关闭 | 非作者评审（ACANX P0-003 演练）+ IC 替补确认（2026-08-01T20:45:00Z） |

例外说明：ProjectAuthor 已批准的治理例外（Ruleset 变更审计、原始失败日志归档、独立人类
QA/SRE 签署的残余项）延后至后续治理验收；这些例外不涉及本 Gate 的强制客观证据，且
`P0-003HumanDrillWorksheet.md`、负例 Run URL、Ruleset 截图与独立复核快照均已归档。

## 审批与结论

| 角色 | 人员与目录引用 | 审阅范围 | 独立性声明 | 签署 UTC 时间 | 签署/结论 |
| --- | --- | --- | --- | --- | --- |
| PO | ACANX（`245818784@qq.com`） | 范围、非目标、风险接受与 Backlog 冻结 | 本项目为单人作者模型，ACANX 承担 PO 角色并已声明交叉贡献边界（见 `SingleAgentGovernanceMaterial.md`） | 2026-08-01T20:46:00Z | PASS |
| TL | ACANX（`245818784@qq.com`） | P0-001、P0-003、P0-004、P0-010、P0-012 | 同上 | 2026-08-01T20:46:00Z | PASS   |
| QA | ACANX（`245818784@qq.com`） | P0-003、P0-005、P0-007、P0-009、P0-012 | 同上 | 2026-08-01T20:46:00Z | PASS  |
| SRE/安全 | ACANX（`245818784@qq.com`） | P0-006、P0-008、P0-011 | 同上 | 2026-08-01T20:46:00Z | PASS  |

- 最终 Gate 结论：`PASS`
- 结论时间：2026-08-01T20:46:00Z（签署人 ACANX 于 2026-08-01T20:50:06Z 在 PR #115 审批中确认）
- Backlog 冻结引用：首个迭代 Backlog 依据 `Docs/VeritasQuantDevelopmentPlan.md` 于 2026-08-01T20:46:00Z 冻结；冻结后
  新增工作项须走计划变更流程。
- 后续动作：签署人填写签署时间与结论后，本报告随 PR 合并归档；残余治理复核
  （Ruleset 变更审计、原始日志归档）按例外跟踪，不阻断 M0 结论。
