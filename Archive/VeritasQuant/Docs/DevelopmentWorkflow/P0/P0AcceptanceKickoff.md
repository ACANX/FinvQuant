# P0 验收启动记录

## 启动决定

ProjectAuthor 于 2026-07-31T12:31:58Z 正式启动 P0 验收。本记录启动证据收集与独立复核，
不构成 P0 或 M0 通过、Release、环境晋级或 Backlog 冻结。

## 验收候选

| 项目 | 值 |
| --- | --- |
| 仓库 | `https://github.com/ACANX/VeritasQuant` |
| 候选分支 | `main` |
| 候选提交 | `1e24643794b1ac8befb7bd1e901e3f8a7098516c` |
| 验收范围 | P0-001 至 P0-013 |
| 技术依据 | 《VeritasQuantTechSpec》第 12.2、13 章 |

## 当前处理方式

- P0-007 的 PR #2 远程负例、`main` 未变和演练分支清理已留存。原始 job 日志、Ruleset
  变更审计和独立人类签署转入后续治理验收，不阻断 P0 开发或收尾。
- P0-006 的 Ruleset 已启用三项严格必需检查、Code Owner、非作者批准、陈旧批准失效、
  最近推送者审批限制和空 bypass list；独立验收仍待安排。
- 本轮作者侧自动化验收结果、工件路径和 SHA-256 见
  [P0AcceptanceAutomationEvidence.md](P0AcceptanceAutomationEvidence.md)。
- 其余 P0 证据按照工作项登记和独立验收包收集。未完成的人类复核保持为验收待办，
  不得由自动代理代签。

## 验收边界

P0-013 已进入 `IN_REVIEW`。正式 M0 Gate 仍须满足《VeritasQuantTechSpec》第 13 章的
证据和签署要求；在此之前，[M0PreReview.md](../M0/M0PreReview.md) 的结论继续为
`INSUFFICIENT_EVIDENCE`。
