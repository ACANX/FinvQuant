# VeritasQuant 业务开发工作流

## 1. 文档目的

本文档定义 VeritasQuant 从需求提出到生产运行、从风险发现到复盘关闭的标准工作流，供人员协作和后续自动化管理使用。它与 [VeritasQuantDevelopmentPlan.md](VeritasQuantDevelopmentPlan.md) 共同约束项目执行；技术行为仍以 [VeritasQuantTechSpec.md](VeritasQuantTechSpec.md) 为唯一权威设计。

工作流目标是：

1. 每项功能都有可追溯的业务目标、技术契约、实现、测试、发布和运行证据。
2. 每次升级使用同一不可变工件逐环境晋级，不能跳过 Gate 或在环境中现场修改。
3. 风险、突发问题和事故在发现时立即登记，完整保存时间线和证据，为复盘提供数据。
4. 自动化系统可以依据明确状态、字段和规则推进工作，不依赖口头判断完成度。

## 1.1 开发过程文档存放规则

`Docs/DevelopmentWorkflow/` 目录下的开发过程文档按阶段与里程碑划分子目录存放，规则如下：

| 文档前缀 | 存放位置 | 示例 |
| --- | --- | --- |
| `P0`、`P1`、`Pn`（阶段/计划任务） | `Docs/DevelopmentWorkflow/P0/`、`P1/`、`Pn/` | `P0-006CiGovernance.md` → `Docs/DevelopmentWorkflow/P0/P0-006CiGovernance.md`；`P1-001-P1-013Evidence.md` → `Docs/DevelopmentWorkflow/P1/` |
| `M0`、`M1`、`Mn`（里程碑/Gate） | `Docs/DevelopmentWorkflow/M0/`、`M1/`、`Mn/` | `M0StageGateReport.md` → `Docs/DevelopmentWorkflow/M0/M0StageGateReport.md`；`M0-Linux.junit.xml` → `Docs/DevelopmentWorkflow/M0/` |
| 登记表（`*Register.yml`、`TraceabilityMatrix.yml`） | `Docs/DevelopmentWorkflow/Registers/` | `WorkItemRegister.yml`、`RiskRegister.yml`、`ActionRegister.yml`、`TraceabilityMatrix.yml` 等全部登记表 |
| 其他（索引、政策、跨阶段规范） | `Docs/DevelopmentWorkflow/` 根目录 | `TestEvidencePolicy.md`、`DevelopmentDocumentIndex.md`、`SingleAgentGovernanceMaterial.md` |

规则约束：

1. **阶段前缀（`P`）与里程碑前缀（`M`）不区分大小写**，`p0-` 与 `P0-` 均按阶段归类；但新文档统一使用大写前缀命名。
2. **复合前缀文档**（如 `P0-006P0-007GitHubGovernanceRunbook.md`）按首个前缀（`P0`）归入对应阶段子目录。
3. **跨阶段证据文件**（如 `P0-003-P0-012TestEvidence.json`）按首个前缀归入该阶段子目录，并在正文或索引中注明覆盖范围。
4. **登记表**（以 `Register.yml` 结尾或 `TraceabilityMatrix.yml`）统一存放于 `Registers/`，无论文件名是否带阶段前缀；登记表之间及被文档/测试引用时使用 `Docs/DevelopmentWorkflow/Registers/<file>` 路径。
5. **子目录内文档引用上级目录文件**时使用 `../`；引用其他子目录文件时使用 `../<子目录>/`；根目录文件引用子目录文件时直接使用 `<子目录>/` 前缀。
6. **新增文档**必须先按上述规则确定存放位置，再在 `DevelopmentDocumentIndex.md` 中登记；索引中的链接必须指向实际存放路径。
7. **移动既有文档**必须同步更新所有引用该文档的 md/yml/json 路径与链接，并通过 `git mv` 保留历史。
8. 设计决策变更仍必须同步修改技术方案，不因文档归类而改变事实来源。

## 2. 工作项模型

### 2.1 类型与层级

| 类型 | ID 前缀 | 用途 | 允许父项 |
| --- | --- | --- | --- |
| 阶段 | `STG` | 对应 M0-M5 Gate 的工作集合 | 无 |
| Epic | `EPC` | 一个可独立验收的业务/技术能力 | 阶段 |
| Feature | `FTR` | 用户可感知或跨模块能力 | Epic |
| Story | `STY` | 单一业务结果 | Feature/Epic |
| Task | `TSK` | 不超过 10 人日的开发、测试、文档或运维任务 | Story/Feature |
| Bug | `BUG` | 实际行为偏离已批准契约 | 任意交付项 |
| Spike | `SPK` | 有时限、有结论的技术探索 | Story/Feature |
| Risk | `RSK` | 尚未发生但可能影响目标的不确定性 | 阶段/任意工作项 |
| Incident | `INC` | 已发生并影响安全、正确性、可用性或进度的事件 | 阶段/发布 |
| Change | `CHG` | 改变已批准范围、契约、基线或环境的请求 | 阶段/发布 |
| Action | `ACT` | 复盘、审计或 Gate 产生的后续行动 | Risk/Incident/Gate |
| Release | `REL` | 一组不可变工件和版本的晋级单位 | 阶段 |

计划文档中的 `P0-001` 等 ID 保持稳定，并在工作项系统中作为 `PlanTaskId`；系统生成的 `TSK-*` 不能替代计划 ID。一个任务可以拆成多个技术 Task，但所有子项验收后父计划任务才可完成。

### 2.2 所有工作项的必填字段

| 字段 | 说明 |
| --- | --- |
| `WorkItemId` | 系统唯一 ID |
| `PlanTaskId` | 对应开发计划 ID；临时事故/风险可为空 |
| `Title` / `Description` | 明确结果和事实，不使用“优化一下”等模糊表达 |
| `Stage` / `Milestone` | 所属阶段与目标 Gate |
| `TechSpecRefs` | 技术方案章节或明确“不改变技术契约” |
| `Owner` / `Approver` | 唯一执行责任人和验收人 |
| `Priority` / `Severity` | 计划优先级或缺陷/事故严重度 |
| `EstimatePersonDays` | 人日估算；观察窗口另填 |
| `Dependencies` | 前置工作项、环境、数据、外部审批 |
| `AcceptanceCriteria` | 可验证的完成条件 |
| `TestEvidence` | 测试 ID、报告、种子、日志和产物哈希 |
| `RiskLinks` | 关联风险、事故或变更 |
| `TargetDate` | 基线目标日期 |
| `Status` / `StatusChangedTs` | 当前状态与 UTC 变更时间 |

字段名用于未来项目自有 Yml 自动化配置时必须采用 PascalCase；通过 REST API 传输时由 Schema 映射为 snake_case。

## 3. 端到端业务开发流程

### 3.1 流程总览

```text
需求/问题进入
    -> 受理与分类
    -> 业务价值和范围确认
    -> 技术方案/兼容性影响分析
    -> 拆分、估算、依赖和风险评审
    -> Ready 准入
    -> 实现与持续验证
    -> 代码/设计评审
    -> 独立验收
    -> 合并和不可变构建
    -> 环境晋级与观察
    -> Gate/发布
    -> 运行监控与反馈
    -> 复盘和下一迭代
```

### 3.2 步骤、输入、输出和自动化规则

| 步骤 | 主责 | 必需输入 | 关键动作 | 必需输出 | 自动化控制 |
| --- | --- | --- | --- | --- | --- |
| 1. 受理 | PO | 需求、缺陷、风险或事故事实 | 去重、分类、分配 ID、确认来源和紧急度 | 已登记工作项 | 无 ID 的工作不得进入开发队列 |
| 2. 业务分析 | PO/领域负责人 | 已登记工作项 | 明确用户、价值、范围、非目标、业务规则和验收人 | 业务验收标准 | 缺业务验收标准不能进入技术分析 |
| 3. 技术影响 | TL | 业务验收标准、技术方案 | 分析事件、账户、风控、执行、API、配置、持久化、安全和兼容性 | 设计结论、TechSpec/ADR 变更 | 触及权威契约但未更新 TechSpec 时阻止 Ready |
| 4. 拆分估算 | TL/团队 | 已批准设计 | 拆成 <=10 人日任务，标依赖、测试、风险和回滚 | 可排期 Backlog | 超限任务、无 Owner/依赖/验收项自动退回 |
| 5. Ready 评审 | PO/TL/QA | 完整工作项 | 检查 Definition of Ready、容量和外部条件 | Ready 任务 | 只有 Ready 状态可进入迭代 |
| 6. 实现 | 开发者 | Ready 任务、冻结基线 | 小批提交、同步测试、日志/指标/错误/审计 | 代码、迁移、配置、测试、文档 | 分支/PR 必须关联工作项；强制本地/CI preflight |
| 7. 评审 | 非作者/TL | PR 和证据 | 代码、契约、失败模式、安全、性能与可维护性评审 | 批准或修改请求 | 关键模块要求 CodeOwner；作者不能自批 |
| 8. 验证 | QA/验收人 | 候选构建 | 独立执行契约、集成、回归、故障和业务验收 | 验收结果与证据包 | 测试失败、跳过或证据缺失禁止 Accepted |
| 9. 合并构建 | CI | 已批准 PR | 合并主干，生成 wheel/镜像/SBOM/哈希 | 不可变候选 Release | 同一 commit 只生成一个签名工件集合 |
| 10. 环境晋级 | SRE/PO | Release、环境准入证据 | 部署、迁移、自检、冒烟、观察、回滚验证 | 环境部署记录 | 只允许前一环境通过的同一 digest 晋级 |
| 11. Gate/发布 | Gate 委员 | 阶段证据、风险、事故、SLO | 独立审查平台和策略 Gate | PASS/FAIL/INSUFFICIENT_EVIDENCE | 缺强制证据只能证据不足，不能人工改绿 |
| 12. 运行反馈 | SRE/PO | 运行指标、日志、反馈 | 监控、对账、风险管理、复盘 | 风险/缺陷/改进 Backlog | 异常自动创建或关联 Incident/Risk |

## 4. 状态机

### 4.1 需求、Story 与 Task

```text
DRAFT -> ANALYZING -> READY -> IN_PROGRESS -> IN_REVIEW -> VERIFYING
      -> ACCEPTED -> RELEASED -> CLOSED
```

| 状态 | 进入条件 | 退出条件 |
| --- | --- | --- |
| `DRAFT` | 已登记最小事实 | 完成分类、责任人和初始价值判断 |
| `ANALYZING` | 已受理 | 业务/技术影响、依赖、风险、估算和验收明确 |
| `READY` | Definition of Ready 全满足 | 被迭代承诺并开始实施 |
| `IN_PROGRESS` | 有负责人且未超 WIP | 代码/配置/文档和作者测试完成 |
| `IN_REVIEW` | PR/变更包及证据齐全 | 所有阻断意见解决、必要批准完成 |
| `VERIFYING` | 候选构建可用 | 独立测试和业务验收通过 |
| `ACCEPTED` | Definition of Done 满足 | 包含在不可变 Release 中 |
| `RELEASED` | 已部署目标环境 | 完成观察且无回滚/阻断问题 |
| `CLOSED` | 运行结果稳定、证据归档 | 终态；重开必须记录原因 |

`BLOCKED` 是状态标志而不是替代状态。设置后必须填写 `BlockedReason`、`BlockedBy`、`NextAction`、`Owner` 和 `EscalateTs`；解除时记录阻断时长和结果。

### 4.2 Bug

```text
NEW -> TRIAGED -> READY -> FIXING -> REVIEWING -> VERIFYING
    -> FIXED -> RELEASED -> CLOSED
             -> REOPENED -> FIXING
```

- `TRIAGED` 必须确认期望行为来源、影响版本、环境、复现率、严重度和是否回归。
- S0/S1 Bug 自动关联 Incident，禁止只按普通缺陷处理。
- `FIXED` 必须有修复前失败、修复后通过的同一测试或等价证据。
- `WONT_FIX`、`DUPLICATE`、`BY_DESIGN` 需要批准人和明确依据，不允许作为清理积压的批量结论。

### 4.3 Risk

```text
IDENTIFIED -> ANALYZING -> MITIGATING -> MONITORING -> CLOSED
                         -> ACCEPTED(expiring) -> REVIEW_DUE
```

- 风险暴露建议使用 `Probability(1-5) * Impact(1-5)`；资金正确性、未授权交易和风控失效的 Impact 固定为 5。
- 暴露 >=15 或影响 M1-M5 的风险每周复核；>=20 每日复核并进入里程碑报告。
- 风险接受必须填写残余风险、批准人、到期日和触发条件；到期自动回到 `REVIEW_DUE`。

### 4.4 Incident

```text
DETECTED -> TRIAGED -> CONTAINED -> RESOLVING -> VERIFYING
         -> RECOVERED -> REVIEWING -> ACTION_TRACKING -> CLOSED
```

恢复服务不等于关闭事故。只有证据包归档、复盘完成、根因/促成因素明确、行动项进入跟踪且关键行动完成后才能关闭。S0/S1 必须由非直接责任开发者参与复核。

### 4.5 Change

```text
PROPOSED -> IMPACT_ANALYSIS -> APPROVAL_PENDING -> APPROVED
         -> SCHEDULED -> IMPLEMENTED -> VERIFIED -> CLOSED
         -> REJECTED
```

改变技术契约、阶段范围、里程碑基线、生产配置、数据库或实盘额度必须建立 Change。紧急变更可先由授权人执行遏制，但 1 个工作日内必须补齐记录、影响和回滚证据。

## 5. Backlog 与迭代工作流

### 5.1 Backlog 维护

- 按 `Stage -> Epic -> Feature -> Story -> Task` 保持层级，禁止只有零散 Task 没有业务结果。
- 优先级依次考虑正确性/安全阻断、关键路径、Gate 证据、业务价值、技术债。
- `Ready` 队列至少覆盖未来 2 个迭代，不超过未来 4 个迭代，避免过早细化远期任务。
- 每周清理重复、失效和无责任人条目；删除只能标记关闭并保留历史。
- 技术债必须说明如果不处理的可量化影响和最晚处理 Gate。

### 5.2 迭代规划

1. 读取上次实际吞吐、可用人日、请假、值班和阶段缓冲。
2. 先选择关键路径和 S0/S1 修复，再选择 Gate 证据和普通功能。
3. 单人同时 `IN_PROGRESS` 工作项上限为 2，团队总 WIP 不超过实际开发人数的 1.5 倍。
4. 每个迭代只有一个可验证目标，任务总和必须能说明如何达成目标。
5. 承诺后新增范围必须走 Change；普通新需求进入下一迭代。

### 5.3 每日与迭代结束

- 每日自动生成：已验收/进行中/阻断、剩余人日、关键路径、风险变化和失败流水线。
- 阻断超过 2 日变黄，超过 5 日变红并升级 PO/TL。
- 迭代评审只演示可运行的不可变构建，不用本地未提交代码代替。
- 未完成任务返回 `READY` 并重新估算，记录未完成原因；不能机械复制原剩余量。
- 回顾行动项转成 `ACT-*`，有责任人、截止日和验证指标。

## 6. 设计与兼容性工作流

### 6.1 必须先改技术方案的变更

以下变化在代码实施前必须更新 `VeritasQuantTechSpec.md`：

- 事件字段、Schema 版本、`ts` 精度、完整排序键或 phase。
- 订单/基金申请/预警/控制状态机及账户账本语义。
- 风控发布权、策略权限、实盘授权或执行适配行为。
- API 固定字段、成功/错误码、错误目录、命令幂等和资源版本。
- 配置合并/哈希、DSL 语义、数据库事实源或恢复边界。
- 阶段 Gate、资产能力或实盘安全边界。

变更流程为：提出 CHG -> 影响分析 -> 更新技术方案 -> ADR（需要时）-> 兼容/迁移计划 -> 测试计划 -> 批准 -> 实现。代码 PR 不得偷偷定义与方案不同的新事实。

### 6.2 版本升级规则

| 对象 | 兼容升级 | 破坏性升级 | 强制证据 |
| --- | --- | --- | --- |
| 事件 Schema | 同主版本增加可选字段/元数据 | 删除、改名、单位/语义/精度变化升主版本 | 旧样本、升级器确定性、未知版本隔离 |
| REST API | v1 增加已声明可选响应字段 | 固定字段或语义变化发布 `/api/v2` | OpenAPI diff、旧客户端、信封契约 |
| 错误目录 | 新增唯一代码、弃用标记 | 改既有语义需目录/API 主版本 | 唯一性、号段、不复用、异常映射 |
| 配置 | 增加有默认值字段 | 删除/改名/默认行为变化升 Schema 版本 | 等价/不等价哈希、旧配置迁移 |
| DSL | 增加兼容节点/白名单函数 | 语义、类型或默认动作变化升主版本 | 编译快照、沙箱、防前视回归 |
| 数据 manifest | 增加非身份元数据 | 身份字段或规范算法变化升主版本 | 跨平台 ID、旧 manifest 可读性 |
| 数据库 | 向后兼容 expand | contract 删除必须跨版本分阶段 | 所有支持前序版本迁移、失败回滚 |

REST 错误响应必须保持如下结构，不得把 `retryable` 移回顶层：

```json
{
  "code": 6201,
  "message": "账户可用资金不足",
  "error": {
    "code": "INSUFFICIENT_AVAILABLE_CASH",
    "catalog_version": "1.0",
    "retryable": false
  },
  "request_id": "req_01J...",
  "trace_id": "trc_01J...",
  "details": {
    "required_amount": "1000.00",
    "available_cash": "800.00"
  }
}
```

## 7. 代码评审与合并工作流

### 7.1 分支与提交

- 采用短生命周期分支，命名建议为 `feature/TSK-123-short-name`、`fix/BUG-123-short-name`。
- 分支目标在 3 个工作日内形成可评审增量；超过 5 日必须说明拆分困难和合并风险。
- 提交保持单一目的并关联工作项；生成文件与源文件同提交，禁止无法复现的手工产物。
- 主分支始终可构建；禁止直接推送和绕过必需检查。

### 7.2 PR 必填内容

1. 业务结果、范围和非目标。
2. 技术方案/ADR/工作项引用。
3. 关键实现与失败模式说明。
4. API/事件/配置/数据库/错误码兼容性说明。
5. 测试列表、命令、结果、种子和证据链接。
6. 日志、指标、告警、审计和敏感信息影响。
7. 部署、迁移、回滚和运行观察方法。
8. 已知风险和未解决项；不得用空白表示没有风险。

### 7.3 审批规则

| 变更范围 | 最低审批 |
| --- | --- |
| 普通模块内变更 | 1 名非作者工程师 |
| 事件、排序、恢复、账本、订单、风控 | 领域负责人 + QA |
| API/配置/DSL/数据库兼容性 | 对应 Owner + QA |
| 安全、权限、密钥、实盘 | TL + SRE/安全 + QA |
| Gate Policy、额度、风险例外 | PO + TL + 风险/安全负责人 |

未解决阻断意见、CI 红灯、测试跳过、覆盖证据缺失和未批准迁移均阻止合并。管理员绕过只用于恢复代码托管平台本身，必须自动创建 Incident/Change 并在 1 个工作日内复核。

## 8. CI 自动化流水线

### 8.1 分层流水线

| 顺序 | 阶段 | 主要检查 | 失败处理 |
| ---: | --- | --- | --- |
| 1 | `Preflight` | UTF-8、文件/目录命名、Yml 字段 PascalCase、JSON snake_case、禁用 `timestamp` | 阻止后续浪费资源并定位违规 |
| 2 | `Build` | Python 版本、依赖锁、wheel/sdist、包数据 | 阻止合并 |
| 3 | `Static` | 格式、lint、类型、依赖边界、死代码 | 阻止合并 |
| 4 | `Security` | 秘密、SAST、依赖漏洞、许可证、SBOM | 高危阻止；例外必须有到期 CHG |
| 5 | `UnitContract` | 单元、事件/API/配置/错误码/状态机契约 | 阻止合并 |
| 6 | `PropertyModel` | 固定种子快速属性/模型集 | 保存最小失败样本并阻止合并 |
| 7 | `Integration` | 数据->事件->风险->执行->账本->报告、API/数据库 | 阻止合并 |
| 8 | `CrashRecovery` | 关键事务边界抽样注入 | 主分支/夜间失败创建 S1/S2 Bug |
| 9 | `Regression` | 固定 checksum、防前视、Windows/Linux 一致性 | 未批准变化阻止发布 |
| 10 | `Packaging` | 仓库外安装、console script、无副作用导入 | 阻止发布 |
| 11 | `Artifact` | wheel、镜像、SBOM、迁移、报告、签名、SHA-256 | 产物进入只读仓库 |

PR 使用快速且有代表性的集合，主分支每日运行全量，夜间/每周运行 10,000 组属性模型、全崩溃矩阵、迁移和安全沙箱。任何“不稳定测试”必须登记 BUG、指定 Owner 和修复期限；重跑只用于证明稳定性，不能选择一次绿色结果覆盖失败。

### 8.2 证据清单

每次 CI 运行至少保存：

- commit、分支、构建 ID、UTC 开始/结束时间、运行器 OS/架构和工具版本。
- 依赖锁、测试选择规则、随机种子、执行/跳过/失败数量。
- JUnit、覆盖率、属性最小失败样本、性能与 checksum 报告。
- wheel/镜像/SBOM/迁移/Schema/错误目录及其 SHA-256。
- 关联工作项、风险、变更和 Release ID。

### 8.3 CI 触发注意事项（Workflow 编写与 PR 来源）

**8.3.1 fork PR 修改 workflow 文件不会自动运行 checks**

- 现象：来自 fork 的 PR 若修改 `.github/workflows/` 下任何文件，GitHub 安全策略拒绝自动运行该 PR 的 checks；required checks 永久显示 `Expected — Waiting for status to be reported`，无法满足分支规则（ruleset）的合并条件。
- 解法：将分支推送到上游仓库（需 collaborator 写权限），改为**同仓库 PR**（head 分支位于上游仓库，workflow 修改受信任，CI 自动运行）。
- 备注：GitHub API 不支持把 fork PR 的 head 切换为同仓库分支（`head.repo` 不可变），需关闭原 PR 后重新创建。

**8.3.2 GitHub Actions 表达式不支持字符串切片**

- 现象：`${{ github.ref_name[1:] }}` 等 `[start:end]` 切片语法非法，导致 workflow 文件解析失败；push 触发的 run 名称显示为 workflow 文件路径（如 `.github/workflows/Ci.yml`）且立即 failure，PR 的 checks 永不产生。
- 解法：用 shell 参数展开处理，例如去 V 前缀：`echo "stripped=${GITHUB_REF_NAME#V}" >> "$GITHUB_OUTPUT"`，后续 step 引用 `${{ steps.xxx.outputs.stripped }}`；非目标分支时输出为空，配合 `enable` 条件 `!= ''` 避免生成多余 tag。
- 排查：workflow 文件无效时，Actions 页面的 run 名称不是 workflow 的 `name:` 而是文件路径（如 `.github/workflows/Ci.yml`）。

## 9. 发布与环境晋级工作流

### 9.1 环境链

```text
Local -> CI -> Development -> Integration -> HistoricalBacktest
      -> PaperTrading -> SignalReference -> BrokerSimulation
      -> LiveShadow -> ControlledLive
```

- 环境晋级使用同一 commit 和 artifact digest，禁止重新构建。
- 每个环境有独立配置、账户、凭据和数据权限；配置版本进入运行清单。
- `ControlledLive` 默认关闭，只有已批准 Release、账户、策略、标的和限额组合可启用。
- 降级/回滚不能删除已经发生的订单、成交或 journal；通过兼容版本继续处理和对账。

### 9.2 发布步骤

1. 创建 `REL-*`，冻结 commit、工件、配置 Schema、迁移和发行说明。
2. 校验前一环境 Gate、开放风险、事故、漏洞和回滚准备。
3. 备份并验证恢复点；对数据库执行兼容性预检查。
4. 部署到目标环境，先迁移 expand，再部署应用。
5. 执行 liveness、readiness、trading-readiness 和业务冒烟。
6. 观察预定义窗口，核对错误率、延迟、队列、账本、订单和对账。
7. 满足阈值后标记 Released；不满足立即执行回滚/保护并建立 Incident。
8. contract 清理只在所有旧版本退出且兼容窗口结束后另行发布。

## 10. 风险与突发问题工作流

### 10.1 发现与登记

以下来源应自动或人工创建/关联 Risk、Bug 或 Incident：CI 失败、SLO/错误预算、告警、对账差异、账本不变量、未来探针、漏洞扫描、代码评审、用户反馈、外部数据/券商通知、进度偏差和复盘行动。

记录必须包含事实与推测分栏，禁止在证据不足时把假设写成根因。S0/S1 发生后优先保护账户和证据：停止新增风险、保留事实、冻结相关版本、限制写入，再进行诊断。

### 10.2 事故指挥

| 角色 | 职责 |
| --- | --- |
| Incident Commander | 唯一协调、严重度、优先级、升级和恢复决策 |
| Operations Lead | 遏制、恢复、环境和外部依赖操作 |
| Investigation Lead | 时间线、假设验证、日志/数据分析和复现 |
| Communications Lead | 对内/对外状态更新，不未经核实承诺根因 |
| Scribe | 实时记录所有动作、命令、时间和结果 |

重要操作使用双人复核，命令及输出进入证据包。事故频道不是唯一事实源，Scribe 必须同步正式时间线。

### 10.3 复盘与行动项

复盘结构固定为：摘要、影响、检测方式、完整时间线、技术根因、促成因素、有效/无效处置、量化 SLI、证据索引、经验和行动项。不得把“人为失误”作为最终根因；必须分析为何系统允许、未检测或未阻止该操作。

行动项按优先顺序覆盖：消除风险、自动阻止、自动检测、缩短恢复、完善文档/培训。只有可验证的系统性改进才能关闭行动项；“提醒注意”不能作为 S0/S1 的唯一措施。

## 11. 自动化管理规则

### 11.1 自动状态与校验

后续接入工作项平台时应实现：

- PR 创建时由分支/标题关联工作项并把任务推进到 `IN_REVIEW`。
- 必填字段或 Definition of Ready 不完整时禁止加入迭代。
- CI 全绿且审批满足时允许合并，但不自动标记业务 `ACCEPTED`。
- 独立验收报告上传后由验收人签署，才推进 `ACCEPTED`。
- 发布系统按 artifact digest 回写环境、时间、配置版本和结果。
- SLO、账本不变量、未来探针、对账和安全告警自动创建或关联 Incident。
- 风险复核、例外、行动项和证书/密钥到期前自动提醒并升级。
- 任务、风险、事故、变更的每次状态变化都保留操作者和 UTC `ts`。

### 11.2 自动看板

| 看板 | 核心数据 |
| --- | --- |
| 里程碑 | 基线/预测日期、SPI、关键路径、缓冲消耗、Gate 证据完整率 |
| 迭代 | 承诺/验收人日、WIP、周期时间、阻断老化、范围变动 |
| 质量 | 测试通过/跳过、缺陷趋势、逃逸率、checksum 变化、覆盖率 |
| 风险事故 | 暴露、严重度、响应时长、恢复时长、超期行动项、重复事故 |
| 运行 | readiness、事件/订单/账本延迟、outbox、队列、磁盘、对账、错误码 |
| 发布 | 环境、artifact digest、配置/数据/策略版本、审批、回滚状态 |

看板数据必须可下钻到原始工作项和不可变证据，不能只保留聚合百分比。

## 12. 周报与 Gate 报告模板要求

### 12.1 周报

每周报告至少包含：

1. 本周已验收结果和对应证据，不列仅“忙于开发”的活动。
2. 下周关键路径目标、负责人和依赖。
3. 基线日期、P50/P85 预测、SPI、范围变动和缓冲消耗。
4. 新增/升级/关闭风险，开放 S0-S2 事故和超期行动项。
5. 测试、缺陷、性能、SLO、对账和证据完整率趋势。
6. 需要 PO/TL 决策的事项、最迟决策时间和不决策影响。

### 12.2 StageGateReport

Gate 报告必须包含 `StageGatePolicyVersion`、候选 Release、代码/配置/数据/策略/模型/风险版本、证据窗口、样本数、每项指标的阈值和实测值、所有强制测试哈希、开放风险/事故、审批签名及唯一结论。`PASS`、`FAIL`、`INSUFFICIENT_EVIDENCE` 之外不允许使用“有条件通过”等模糊状态。

## 13. 工作流启用检查清单

本清单是**启用状态检查**，不是待办任务清单；每项在对应阶段 Gate 确认，确认时须附客观证据与确认记录（确认人、UTC 时间、证据位置）。仅当该项要求的阶段到达且证据齐全时勾选。

| 启用项 | 确认阶段 | 客观证据要求 | 状态 | 确认记录 |
| --- | --- | --- | --- | --- |
| 工作项系统已创建所有类型、状态和必填字段 | M0 | `WorkItemRegister.yml` 含全部类型/状态/必填字段；契约测试 P0-010-001/002 通过 | ☑ | 已确认：M0 Gate 检查表 P0-010 行（PR #111） |
| 开发计划 ID 已批量导入且与技术方案/测试/Gate 建立关系 | M0 | `TraceabilityMatrix.yml` 覆盖 R-001 至 R-017，无孤立需求；契约测试 P0-012-001 通过 | ☑ | 已确认：M0 Gate 检查表 P0-012 行（PR #113） |
| 分支保护、CodeOwner、必需 CI 和禁止管理员普通绕过已启用 | M0 | Ruleset 三项必需检查（Quality ubuntu/windows、Security baseline）、Code Owner 审阅、空 bypass；截图归档 | ☑ | 已确认：M0 Gate 检查表 P0-006 行（PR #103） |
| 风险、事故、变更、复盘和行动项模板已启用 | M0 | `RiskRegister.yml`/`IncidentRegister.yml`/`ChangeRegister.yml`/`ActionRegister.yml` 含必填字段与审计历史；契约测试 P0-010-002 通过 | ☑ | 已确认：M0 Gate 检查表 P0-010 行（PR #111） |
| 自动看板可追溯到原始证据，指标口径与计划一致 | M0 起持续 | 看板数据可下钻到工作项与不可变证据（见第 11.2 节）；指标口径与计划一致 | 部分 | 基础已建（追踪矩阵 + 登记表）；随阶段持续校验 |
| Release 可记录并校验同一 artifact digest 的逐环境晋级 | M1 起（首个 Release） | 阶段 0 无环境晋级；M1 首个交付物起逐环境校验 artifact digest | ☐ | 待 M1 Gate |
| 告警可自动关联 run/account/request/trace 和工作项 | M2 起（运行监控） | 模拟盘/运行期告警与运行上下文、工作项关联；SLO 与对账指标接入 | ☐ | 待 M2 Gate |
| 团队已演练一次普通 Feature、紧急 Bug 和 S1 Incident 全流程 | M1 前（团队到位后） | 演练记录（Feature/Bug/Incident 各一次）；单人开发阶段由治理例外跟踪（ACT-P0-007） | ☐ | 待团队组建后确认 |
| 每季度复核工作流有效性，并通过版本化 Change 改进 | 持续（每季度） | 季度复核记录 + 版本化 Change 审计 | ☐ | 季度例行，不绑定 Gate |
