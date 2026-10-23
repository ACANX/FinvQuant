# P0-007 远程受保护分支负例演练证据

## 结论边界

本记录归档自动化执行期间可复核的 GitHub 平台事实，不构成 P0-006、P0-007 或 M0 的
`ACCEPT` 结论，也不代替独立人类 QA/SRE 的签署。技术依据为
《VeritasQuantTechSpec》第 12.2、13 章；完整的独立验收要求见
[P0IndependentAcceptancePackage.md](P0IndependentAcceptancePackage.md)。

## 主分支规则快照

取证时间：2026-07-31T11:25:00Z。只读 GitHub REST API 显示 `main` Ruleset
`20108335` 为 `active`，且仅匹配 `refs/heads/main`。其可读取控制如下：

- Pull request 要求 1 个批准、Code Owner 审阅、解决评审会话。
- `dismiss_stale_reviews_on_push` 为 `true`；`require_last_push_approval` 为 `true`。
- 严格必需检查为 `Quality (ubuntu-latest)`、`Quality (windows-latest)` 和
  `Security baseline`。
- 禁止删除和非快进更新。

匿名只读响应未返回 `bypass_actors` 的可审计详情。因此，管理员仍须按验收包归档绕过
名单、管理员强制执行和审计日志；本记录不声称已验证该项。

后续提供的截图
`Archive/Asset/Image/BypassList_2026-07-31 193936.png` 的 SHA-256 为
`FB4FC98ED6760536D8D0A2D6D571B356FFE958F49DE591B6ED154D2EE43808F0`。截图显示
Ruleset `main` 为 Active、仅匹配 `main`，但 bypass list 包含 `Repository admin`，权限为
`Always allow`。该截图仅保留为修正前的历史事实。

管理员后续提供的截图 `Archive/Asset/Image/BypassList_2026-07-31 194302.png` 的 SHA-256
为 `B8740D9C1D378F07BF6138A74225C69D1386F8369B9113F57E9A918600CCE629`。该截图显示同一
Ruleset 为 Active、仅匹配 `main`，且明确显示 `Bypass list is empty`。以该当前快照为准，
普通管理员的全量 bypass 已移除。两个截图的文件名本地时间均不能反向证明 PR #2 创建时的
配置；管理员仍须归档规则变更审计记录和配置操作者引用。

## 演练范围和生命周期

| 项目 | 事实 |
| --- | --- |
| 演练分支 | `p0-007-negative-drill` |
| head 提交 | `d96f8f11c87f6583138ce1f8f4d8969eacb0c776` |
| 与 `main` 的最终差异 | 仅 `Configs/P0NegativeGate.yml`，内容为 `Invalid_field: 1` |
| 本地负例 | `python3 scripts/Preflight.py` 返回 `1`，输出 `Invalid_field: 项目 YAML 字段必须为 PascalCase` 和 `preflight issues: 1` |
| PR | [#2](https://github.com/ACANX/VeritasQuant/pull/2)，base 为 `main`，创建于 2026-07-31T11:24:51Z |
| PR 关闭 | 2026-07-31T11:27:21Z，`merged=false` |
| `main` 关闭前后提交 | 均为 `3988ee4404caee56f2fee06bb98c045b0f8a7283` |
| 远程分支清理 | 已执行 `git push origin --delete p0-007-negative-drill`；关闭后远程不再列出该引用 |

PR #2 被创建为普通 PR 而非 Draft；未请求合并、未使用管理员绕过，且 Quality 必需检查
失败。该流程差异与治理手册中的 Draft 建议不一致，必须由独立 QA/SRE 决定是否接受，
不得由本记录或自动代理消除。

## PR 触发的 CI 结果

PR 的 `pull_request` 工作流为
[30626945620](https://github.com/ACANX/VeritasQuant/actions/runs/30626945620)，head 与上述
提交一致，整体结论为 `failure`。三个作业的最终结论如下：

| 作业 | 结论 | 链接 |
| --- | --- | --- |
| `Quality (ubuntu-latest)` | `failure` | [job 91144301687](https://github.com/ACANX/VeritasQuant/actions/runs/30626945620/job/91144301687) |
| `Quality (windows-latest)` | `failure` | [job 91144301678](https://github.com/ACANX/VeritasQuant/actions/runs/30626945620/job/91144301678) |
| `Security baseline` | `success` | [job 91144301613](https://github.com/ACANX/VeritasQuant/actions/runs/30626945620/job/91144301613) |

PR API 在检查结束后返回 `mergeable_state=unstable`。结合 `main` Ruleset 中的同名严格
必需检查和两个 Quality 的 `failure`，常规合并门禁处于阻止状态。自动化执行未调用合并
API，也未触发任何绕过操作。

## 独立验收待办

ProjectAuthor 于本任务会话将以下事项延后至后续治理验收。它们不再阻断 P0 开发或收尾，
但本记录不会把 P0-007 标记为独立验收通过。

1. 非作者 QA 下载两个 Quality job 的原始日志，核验其中均含
   `Configs/P0NegativeGate.yml`、`Invalid_field: 项目 YAML 字段必须为 PascalCase` 和
   `preflight issues: 1`，并归档保留期。
2. GitHub 管理员归档 Ruleset 变更审计日志，覆盖旧 bypass 的移除操作、操作者和 UTC 时间。
3. 独立 QA/SRE 在验收包中记录 PR 非 Draft 的处理结论、清理复核及签署决定。
