# P0-006/P0-007 GitHub 治理实施与取证手册

## 当前权限判定

本记录创建于 2026-07-31。执行环境中未发现 GitHub CLI（`gh`）或
`GH_*`/`GITHUB_*` 认证环境变量；仅发现通用 Git Credential Manager 配置。为避免
读取或泄露凭据，未枚举凭据存储。`git ls-remote` 已确认
`https://github.com/ACANX/VeritasQuant.git` 的 `main` 和 `dev` 可读，但这不代表写入、
分支保护或仓库管理权限。

因此，本文件是待具备仓库管理权限的人类管理员执行的操作与证据模板，不表示远程
分支保护已经配置，也不表示 P0-006 或 P0-007 已验收。

## 控制目标

以 [P0-006CiGovernance.md](P0-006CiGovernance.md)、
[WorkItemRegister.yml](../Registers/WorkItemRegister.yml) 和技术方案第 11.2、12.2 节为准，在
`main` 上建立以下不可缺少的控制：

1. 必需检查精确为 `Quality (ubuntu-latest)`、`Quality (windows-latest)` 和
   `Security baseline`。名称来自 `.github/workflows/Ci.yml` 的 job name，修改 workflow
   前后均须重新核对。
2. 要求至少一名非作者批准、解决会话，并启用陈旧批准失效和最近推送者不能自行批准。
3. 启用管理员强制执行（`enforce_admins`），禁止一般管理员绕过必需检查或审阅。
   仓库所有者仍可修改治理规则是 GitHub 平台固有的最高权限，不能通过分支保护消除；
   任何此类修改必须走变更记录和独立审阅。
4. 对受控路径要求 Code Owner 审阅。Code Owner 的真实 GitHub 用户或团队由人类负责人
   填写，自动代理不得虚构身份、团队成员关系或值班责任。

## 管理员实施清单

执行者必须是 `ACANX/VeritasQuant` 的人类仓库管理员，且不兼任此次 P0 代码作者的
最终 QA 审阅者。每一步失败时保持 `ACT-P0-004` 为 `OPEN`，不得降低任何控制后继续。

1. 在受保护的凭据渠道创建或取得最小权限、短时有效的 GitHub 凭据。所需仓库权限至少
   包含 Administration read/write；不要在 shell 历史、文档、PR、CI 日志或截图中写入令牌。
2. 先在 `main` 合入由人类填写的 `.github/CODEOWNERS`。必须至少覆盖：
   `src/veritasquant/core/`、`accounts/`、`execution/`、`risk/`、`data/`、`strategy/`、
   `application/`、`infrastructure/`、`.github/`、`Configs/`、`Docs/VeritasQuantTechSpec.md`
   和 `src/veritasquant/resources/Schemas/`。每个条目应指向实际负责的 GitHub 用户或团队。
3. 由管理员通过 GitHub Web UI 的 **Settings -> Branches -> Branch protection rules** 为
   `main` 创建规则，或使用下方 API。勾选必需检查、要求分支更新、至少一项批准、
   Code Owner 审阅、解决会话、禁止强推和删除，并启用 **Include administrators**。
4. 以只读 API 或 UI 重新读取规则，确认所有控制均已生效。将脱敏后的规则 JSON、
   页面截图、审计日志链接和操作者身份引用归档到受控证据位置。
5. 由未编写本次变更的 QA 人类复核证据和一次正常 PR 的非作者审阅记录；管理员不得
   用自己的配置操作代替该复核。

### REST API 等价操作

以下示例只供完成前述人员映射后的管理员在本地交互会话执行。它不会输出令牌。执行前
应将 `@actual-owner` 全部替换为已确认的真实 Code Owner，并确认当前工作目录不是包含
秘密的临时目录。

```powershell
$secureToken = Read-Host 'GitHub fine-grained token' -AsSecureString
$tokenPointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureToken)
try {
    $token = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($tokenPointer)
    $headers = @{
        Accept = 'application/vnd.github+json'
        Authorization = "Bearer $token"
        'X-GitHub-Api-Version' = '2022-11-28'
    }
    $rule = @{
        required_status_checks = @{
            strict = $true
            contexts = @(
                'Quality (ubuntu-latest)',
                'Quality (windows-latest)',
                'Security baseline'
            )
        }
        enforce_admins = $true
        required_pull_request_reviews = @{
            dismissal_restrictions = @{ users = @(); teams = @(); apps = @() }
            dismiss_stale_reviews = $true
            require_code_owner_reviews = $true
            required_approving_review_count = 1
            require_last_push_approval = $true
            bypass_pull_request_allowances = @{ users = @(); teams = @(); apps = @() }
        }
        restrictions = $null
        required_linear_history = $true
        allow_force_pushes = $false
        allow_deletions = $false
        block_creations = $false
        required_conversation_resolution = $true
        lock_branch = $false
        allow_fork_syncing = $false
    } | ConvertTo-Json -Depth 8

    Invoke-RestMethod -Method Put -Uri 'https://api.github.com/repos/ACANX/VeritasQuant/branches/main/protection' -Headers $headers -ContentType 'application/json' -Body $rule
    Invoke-RestMethod -Method Get -Uri 'https://api.github.com/repos/ACANX/VeritasQuant/branches/main/protection' -Headers $headers |
        ConvertTo-Json -Depth 16 | Set-Content -Encoding utf8 'BranchProtectionEvidence.json'
}
finally {
    if ($null -ne $tokenPointer) { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($tokenPointer) }
    Remove-Variable token -ErrorAction SilentlyContinue
}
```

若 GitHub 返回某字段对当前仓库计划不可用，管理员必须停止并在 Web UI 配置等价的更严格
规则，记录 API 返回的状态码和字段名；不得删除必需检查、管理员强制执行或非作者审阅
来使调用“成功”。

## P0-007 远程故意违规演练

此演练验证分支保护和远程 CI 的组合会拒绝工程约定违规。它仅创建、关闭并删除独立 PR，
不会修改 `main`。本地已有覆盖相同规则的测试：
`python3 -m pytest tests/unit/scripts/test_engineering_scripts.py -q` 于本次准备中通过
（6 passed）；远程演练仍必须独立执行，不能由该本地结果替代。

1. **前置确认**：管理员先完成上一节规则配置，并以只读界面确认 `main` 的三个检查均为
   required。QA 记录规则 URL 和检查名称。
2. **隔离分支**：从当时的 `origin/main` 创建唯一分支，例如
   `p0-007-negative-gate-20260731`。只允许该分支创建 PR，禁止直接推送 `main`。
3. **最小违规**：仅新增 `Configs/P0NegativeGate.yml`，其内容为 `Invalid_field: 1`。
   该字段故意违反 PascalCase 规则，`scripts/Preflight.py` 应只报告
   `Invalid_field: 项目 YAML 字段必须为 PascalCase` 和 `preflight issues: 1`，并以非零退出。
4. **创建 PR**：将分支推送并创建以 `main` 为 base 的 PR，标题明确为
   `P0-007 negative gate drill - do not merge`。可先创建为 Draft，但在验证常规合并门禁前必须
   转为 ready for review；不请求业务合并，不使用管理员绕过。
5. **观察而不合并**：等待三个远程检查结束。两项 Quality 应在 Preflight 步骤失败；
   `Security baseline` 可成功。记录 PR URL、每个 job URL、失败日志中违规路径和规则信息、
   分支保护页面中合并被阻止的状态。不得调用 merge API、不得点击管理员绕过、不得执行
   force push；这样即使配置错误也不会把故意违规内容带入 `main`。
6. **独立确认与清理**：非作者 QA 确认 PR 的 base 为 `main`、所有失败检查仍在 required
   列表且 UI 不提供常规合并。确认后关闭 PR 并删除该远程分支。再次读取 `main` 的 head
   commit，确认其未因演练改变。

## 最小证据包

所有链接和截图须标明 UTC 取证时间、取证人和仓库/分支；不得包含令牌、Cookie 或完整
个人信息。将以下信息提交给独立 QA/SRE，而不是由自动代理填写：

| 证据 | 必填内容 | 责任人 |
| --- | --- | --- |
| 分支保护导出或截图 | `main`、三个精确检查、管理员强制执行、PR/Code Owner/会话/强推/删除设置 | GitHub 管理员 |
| 平台审计链接 | 修改规则的审计事件、操作者目录引用、UTC 时间 | GitHub 管理员 |
| 正常 PR 审阅 | 非作者批准、Code Owner 审阅、required 检查全部成功 | 非作者 QA |
| 负例 PR | 分支、base、commit、三个 job URL、两项 Quality 的失败日志与合并阻止状态 | 非作者 QA |
| 清理确认 | PR 已关闭、演练分支已删除、`main` 的 before/after head 相同 | SRE/QA |
| 结论 | P0-006/P0-007 是否可接受、例外与未关闭风险 | 独立 QA 与 SRE |

只有上述证据齐全且独立复核通过后，才可将 `ACT-P0-004`、P0-006 和 P0-007 的对应阻断
状态更新为已关闭；本运行手册本身不是验收证据。
