# P0-006 CI 与合并治理记录

## 已实现配置

`.github/workflows/Ci.yml` 在 `ubuntu-latest` 与 `windows-latest` 的 Python 3.13 运行以下不可跳过步骤：依赖锁与前置检查、Ruff、Mypy、JUnit 与 coverage、wheel/sdist 构建、仓库外 wheel 安装、秘密扫描、`pip-audit` 和许可证校验。质量和构建产物以 CI artifact 保存 90 天。

## 必须由代码托管管理员完成的配置

以下状态不能由仓库文件替代，完成前 P0-006 不具备验收证据：

1. 恢复或指定有效远程 Git 仓库与默认主分支。
2. 将 `Quality (ubuntu-latest)`、`Quality (windows-latest)` 与 `Security baseline` 设为必需检查，禁止管理员普通绕过。
3. 要求至少一名非作者评审；事件、排序、账本、订单、风控、配置、API 和安全改动另加对应 CodeOwner。
4. 在两个平台从空缓存运行一次，归档 Build ID、commit、解释器版本、JUnit、coverage、哈希和产物保留链接。

当前目录已可被 Git 识别为仓库，且 `origin` 可访问 `main` 与 `dev` 分支；本地 Ubuntu WSL 的 Linux 构建、测试和仓库外 wheel 验证见 [M0LinuxValidationEvidence.md](../M0/M0LinuxValidationEvidence.md)。GitHub Actions Run `30619335295` 已完成 Windows/Linux Python 3.13 Quality 与 Security baseline，并保留质量工件 90 天。只读规则查询已确认 `main` 使用三项必需检查、非作者批准、Code Owner 审阅和最近推送者审批限制。修正前截图 `BypassList_2026-07-31 193936.png` 曾显示 `Repository admin = Always allow`；最新截图 `BypassList_2026-07-31 194302.png` 显示 Bypass list 为空。当前普通管理员绕过已移除，仍待管理员变更审计和独立验收，详见 `RSK-P0-003`。

## 当前执行顺序

`CHG-P0-001` 已获批准。GitHub Actions Run `30619335295` 已完成 Windows/Linux Python 3.13 的空缓存 Quality 和 Security baseline，`ACT-P0-003` 已完成；当前使用已归档的双平台工件支持开发。

受保护分支和必需检查配置作为 `ACT-P0-004` 排入后续。当前截图已证明 `Repository admin = Always allow` 被移除；管理员仍须归档变更审计信息。在独立验收完成前，本地测试结果或远程失败演练均不能单独构成 P0-006 的验收结论。

该调整不修改 P0-006 的双平台验收标准，也不修改技术方案。分支保护、必需检查和独立验收在 M0 正式评审前仍必须提供；在此之前 M0 只能是 `INSUFFICIENT_EVIDENCE`。

## 2026-08-01 独立复核快照

- 复核时间：`2026-08-01T18:08:00Z`。
- 复核对象：GitHub Ruleset `20114806`，名称 `dev`，状态 `active`，适用于 `refs/heads/dev`。
- `bypass_actors` 为空；规则禁止删除和非快进更新。
- Pull Request 规则要求至少一名审批者、Code Owner 审阅、最后推送者审批与审阅线程解决；三项必需检查为 `Quality (ubuntu-latest)`、`Quality (windows-latest)`、`Security baseline`。
- 最近的独立候选运行为 GitHub Actions Run `30711402480`（提交 `b25494d578214aa9c3d51689904f0bda235c4155`）；三个 job 于 `2026-08-01T17:56:10Z` 至 `17:57:27Z` 全部成功。工作流按精确锁文件在 Windows/Linux Python 3.13 环境执行质量、构建与仓库外 wheel 验证，并上传保留 90 天的质量工件。

本快照证明当前配置与 P0-006 验收标准相符。`ACT-P0-004` 仍保留为 M0 Gate 前的管理员变更审计行动；它不替代或改变本工作项的独立验收，也不构成 M0 Gate 批准。
