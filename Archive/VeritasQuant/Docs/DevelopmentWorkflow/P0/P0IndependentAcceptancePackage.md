# P0 独立验收包

## 目的和使用边界

本包供**未编写受审代码的非作者人类**在同一候选提交上执行 P0-006、P0-007、P0-009、P0-011 与 P0-012 的独立复核。它将作者验证、远程 CI 合并门禁和人类验收严格分开；填写本文件不授权发布、环境晋级或 M0 Gate 通过。

技术依据为《VeritasQuantTechSpec》第 12.2、13 章，任务验收条件以 `WorkItemRegister.yml` 的 P0-006、P0-007、P0-009、P0-011、P0-012 为准。根据开发工作流，代理可以整理证据，不能签署验收或 Gate。

## 验收前置条件

1. 评审人确认自己不是该候选提交内受审实现的作者；若发生角色冲突，结论只能是 `CHANGES_REQUIRED`。
2. 记录候选提交 SHA、审阅者的企业目录引用和 UTC 开始时间。不得使用未提交的工作区文件作为验收对象。
3. 使用全新或已清空缓存的 Python 3.13 环境。项目的支持基线是 Python `>=3.13`；本包不接受 3.12 及更低版本替代。
4. 对 P0-006/P0-007，仓库管理员先在默认分支 `main` 启用受保护分支。必需检查名称必须与当前工作流的显示名称完全一致：`Quality (ubuntu-latest)`、`Quality (windows-latest)`、`Security baseline`；必须要求至少一名非作者评审，并禁止常规管理员绕过。管理员应保存配置页面截图或平台审计链接，包含规则范围、必需检查、审批和绕过设置。

## 本地独立复核：P0-009、P0-011、P0-012

以下命令在候选提交的仓库根目录执行。PowerShell 的 `$LASTEXITCODE` 必须为 `0`，除非表格明确要求负例返回 `1`。

| 顺序 | 命令 | 预期退出码和判定 | 归档证据 |
| --- | --- | --- | --- |
| 1 | `python3 --version` | `0`，输出 Python 3.13.x 或更高 | 完整输出 |
| 2 | `python3 scripts/VerifyDependencyLocks.py` | `0`，含 `dependency lock issues: 0` | 控制台日志 |
| 3 | `python3 scripts/Preflight.py` | `0`，含 `preflight issues: 0` | 控制台日志 |
| 4 | `python3 scripts/ScanSecrets.py` | `0`，含 `secret findings: 0` | 控制台日志 |
| 5 | `python3 -m pip_audit -r Requirements/Runtime.lock` | `0`，无已知漏洞 | 控制台日志 |
| 6 | `python3 scripts/VerifyLicenses.py --policy Configs/Security/LicensePolicy.yml` | `0`，含 `license issues: 0` | 控制台日志及 `LicensePolicy.yml` 审阅记录 |
| 7 | `python3 -m pytest tests/unit/scripts/test_engineering_scripts.py tests/contract/test_p0_engineering_baseline.py -q` | `0`，失败、错误、跳过均为 0 | JUnit 或控制台日志 |
| 8 | `python3 -m coverage run -m pytest tests/unit tests/contract tests/packaging --junitxml artifacts/IndependentQa.junit.xml` | `0`，JUnit 的 failures/errors/skipped 均为 0 | `IndependentQa.junit.xml` |
| 9 | `python3 -m coverage xml -o artifacts/IndependentQa.coverage.xml` | `0` | `IndependentQa.coverage.xml` |
| 10 | `python3 -m build` | `0`，生成 wheel 和 sdist | `dist/` 工件 |
| 11 | `python3 scripts/VerifyPackage.py --wheel dist/veritasquant-0.1.0-py3-none-any.whl` | `0`，仓库外安装、全部正式命令和包资源验证成功 | 控制台日志 |
| 12 | `python3 scripts/CollectTestEvidence.py --junit artifacts/IndependentQa.junit.xml --coverage artifacts/IndependentQa.coverage.xml --artifact dist/veritasquant-0.1.0-py3-none-any.whl --artifact dist/veritasquant-0.1.0.tar.gz --work-item P0-006 --work-item P0-009 --seed not_applicable --output artifacts/IndependentQa.evidence.json` | `0`，输出 `evidence written` | `IndependentQa.evidence.json` |

在 Windows 上，将以下命令的输出与 `IndependentQa.evidence.json` 比对。**预期哈希是同一独立运行的 JSON 中对应字段，而不是历史构建的固定字面值**；wheel/sdist 可能因构建元数据变化而改变，不能用旧哈希替代本次复核。

```powershell
Get-FileHash artifacts\IndependentQa.junit.xml, artifacts\IndependentQa.coverage.xml, dist\veritasquant-0.1.0-py3-none-any.whl, dist\veritasquant-0.1.0.tar.gz -Algorithm SHA256
Get-Content artifacts\IndependentQa.evidence.json -Raw
```

P0-009 判定条件：JSON 记录的 JUnit 计数均为 0（`failures/errors/skipped`）、`random_seed` 为 `not_applicable`、环境 Python 为 3.13+，且四个 SHA-256 与上一步逐项相等。P0-012 判定条件：第 7 步通过，`TraceabilityMatrix.yml` 包含且仅包含 R-001 至 R-017；每项都有 `PlanTaskIds`、`TestIds` 与 `LatestGate`。完整 R-001 至 R-017 的执行证据属于其矩阵规定的 M1/M2 Gate，不得作为 M0 漏项。

P0-011 判定条件：第 4 至 6 步通过；`Configs/Security/LicensePolicy.yml` 的 `ApprovalStatus` 为 `APPROVED`，许可证白名单、例外和漏洞 SLA 已被 SRE/安全审阅。负例复核只能在仓库外临时目录执行，且不得记录或提交真实凭据：

```powershell
$proofRoot = Join-Path $env:TEMP ("VeritasQuant-P0-011-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $proofRoot | Out-Null
Set-Content -LiteralPath (Join-Path $proofRoot "Source.py") -Value 'api_key = "vq_test_credential_123456"' -Encoding utf8
python3 scripts/ScanSecrets.py --root $proofRoot
if ($LASTEXITCODE -ne 1) { throw "P0-011 负例未被阻断" }
Remove-Item -LiteralPath $proofRoot -Recurse -Force
```

该负例预期返回 `1`，输出必须包含 `sensitive-assignment`，且不得回显 `vq_test_credential_123456`。清理命令只删除刚由 `$proofRoot` 创建的临时目录。

## P0-007 远程受保护分支负例：最小充分证据

本项不能由本地单元测试、临时未提交文件或成功 CI 替代。最小充分证据是：在已受保护的 `main` 上创建 PR，PR 中的故意违规使两个 Quality 必需检查失败并显示文件/字段定位，平台因必需检查失败拒绝合并；随后关闭 PR，绝不合并违规提交。

### 管理员配置核验

管理员在 GitHub `Settings -> Branches` 或 Rulesets 中核验并归档：

| 设置 | 必须值 |
| --- | --- |
| 目标 | 默认分支 `main` |
| 必需状态检查 | `Quality (ubuntu-latest)`、`Quality (windows-latest)`、`Security baseline` |
| 拉取请求 | 至少 1 名审批；审批者不是变更作者 |
| 绕过 | 普通管理员不得绕过；任何保留的紧急绕过仅限已命名角色且有审计记录 |

将配置截图或审计链接、查看者、UTC 时间和仓库 URL 放入本次证据目录。若平台规则类型不支持以上任一条件，P0-006/P0-007 结论为 `CHANGES_REQUIRED`，不得以口头确认替代。

### 负例步骤

1. 从最新 `main` 创建临时分支 `qa/p0-007-negative-gate-YYYYMMDD`。该分支只包含以下单个故意违规文件：`Configs/P0NegativeGate.yml`，内容为 `Invalid_field: 1`（UTF-8）。文件名符合项目命名规则，而字段故意违反 PascalCase，避免测试到无关错误。
2. 推送该分支并创建指向 `main` 的 PR。记录 PR URL、head SHA、创建者和 UTC 时间。
3. 等待该 PR 触发的同一工作流结束。两个 Quality job 都会执行 `python3 scripts/Preflight.py`，必须失败（退出码 `1`）。每个失败日志必须同时出现：
   - `Configs/P0NegativeGate.yml`
   - `Invalid_field: 项目 YAML 字段必须为 PascalCase`
   - `preflight issues: 1`
4. `Security baseline` 可成功；它不是本负例的失败目标。PR 的合并框必须显示必需检查未通过而不可合并。管理员在不改变规则、不重跑成成功和不使用绕过的前提下尝试合并，并归档平台拒绝的提示或审计事件。
5. 下载失败 run 的原始日志，保存 run URL、两个 Quality job URL、各 job 结论、PR SHA 和保留期。关闭 PR 并删除临时远程分支；记录关闭时间。禁止向 `main` 推送违规内容。

P0-007 的通过条件是第 3 至 5 步全部满足。只有测试 `test_preflight_rejects_invalid_project_yaml_and_timestamp_alias` 通过，或只在一个非受保护分支看到失败，均为证据不足。

## 签署记录

以下字段必须由人类填写；代理不得填写姓名、目录引用、决定或时间。

```yaml
IndependentAcceptance:
  CandidateCommit: 待填写
  RepositoryUrl: https://github.com/ACANX/VeritasQuant
  ReviewStartedTs: 待填写UTC时间
  NonAuthorQaReviewer:
    Name: 待填写
    DirectoryReference: 待填写
    ConflictCheck: PASS或FAIL
  IndependentSreSecurityReviewer:
    Name: 待填写
    DirectoryReference: 待填写
    ConflictCheck: PASS或FAIL
  P0-006:
    BranchProtectionEvidence: 待填写URL或归档路径
    RequiredChecks: [Quality (ubuntu-latest), Quality (windows-latest), Security baseline]
    Decision: ACCEPT或CHANGES_REQUIRED
  P0-007:
    PullRequestUrl: 待填写
    HeadCommit: 待填写
    FailedRunUrl: 待填写
    QualityUbuntuJobUrl: 待填写
    QualityWindowsJobUrl: 待填写
    MergeBlockedEvidence: 待填写URL或归档路径
    CleanupEvidence: 待填写URL或归档路径
    Decision: ACCEPT或CHANGES_REQUIRED
  P0-009:
    EvidenceJson: artifacts/IndependentQa.evidence.json
    HashVerification: PASS或FAIL
    Decision: ACCEPT或CHANGES_REQUIRED
  P0-011:
    SecurityRunUrl: 待填写
    SecretNegativeEvidence: 待填写路径
    LicensePolicyReview: PASS或FAIL
    Decision: ACCEPT或CHANGES_REQUIRED
  P0-012:
    TraceabilityReview: PASS或FAIL
    Decision: ACCEPT或CHANGES_REQUIRED
  Exceptions: []
  ReviewCompletedTs: 待填写UTC时间
  QaSignature: 待填写
  SreSecuritySignature: 待填写
```

所有相关项为 `ACCEPT` 且不存在例外时，QA/SRE 才可将验收结论提交给 PO/TL。该验收包仍不替代 `ACT-P0-007` 的独立评审与 Incident Commander 替补确认，也不替代 P0-013 所需的正式 M0 `StageGateReport` 和 PO/TL/QA/SRE Gate 签署。
