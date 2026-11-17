# P0-001 范围、阶段 Gate 与首期代表资产记录

## 记录状态

- 工作项：`P0-001`
- 状态：`IN_REVIEW`
- 结论：ProjectAuthor 已确认现有范围和未来 Gate 签署名单；本文件不是任何 M0-M5 Gate 通过记录，仍待独立评审。
- 技术依据：[VeritasQuantTechSpec.md](../../VeritasQuantTechSpec.md) 第 13、14 章。

## 待批准范围

首期仅建立严格、可重放的历史回测工程基础，验证两个代表资产路径：

1. 证券路径：`518880`，用于验证 T+1、费用和证券持仓。
2. 期货路径：由数据 manifest 明确指定的单一上期所黄金期货交割合约，用于验证合约乘数、保证金、逐日盯市和到期规则。

阶段 M0 至 M5 的目标、进入条件、验收门槛与禁止项以技术方案第 13 章和开发计划第 4 章为准。每个 Gate 仅可审查同一不可变工件，不能跳过前置 Gate。

## 非目标

- M0/M1 不接入实盘、券商仿真或生产凭据。
- M0/M1 不承诺覆盖所有目标资产、基金定投、GUI 或调度业务能力。
- 不以理想化回测收益作为任何实盘准备结论。
- 未达到各阶段技术 Gate 前，不扩大资产能力或交易模式。

## 待签署人

当前开发执行代理为 `ClaudeCode`，其只能提供实现与作者验证材料，不是下列任一 Gate 的签署人。ProjectAuthor 于 2026-07-31T04:15:00Z 在当前任务会话确认临时多角色模型和未来 Gate 签署权；人类治理角色的限制见 [SingleAgentGovernanceMaterial.md](../SingleAgentGovernanceMaterial.md)。

| Gate | PO 签署人 | TL 签署人 | QA 签署人 | SRE/安全签署人 | 状态 |
| --- | --- | --- | --- | --- | --- |
| M0 | ProjectAuthor | ProjectAuthor | ProjectAuthor | ProjectAuthor | 未来签署权已指定；非 Gate 通过 |
| M1 | ProjectAuthor | ProjectAuthor | ProjectAuthor | ProjectAuthor | 未来签署权已指定；非 Gate 通过 |
| M2 | ProjectAuthor | ProjectAuthor | ProjectAuthor | ProjectAuthor | 未来签署权已指定；非 Gate 通过 |
| M3 | ProjectAuthor | ProjectAuthor | ProjectAuthor | ProjectAuthor | 未来签署权已指定；非 Gate 通过 |
| M4 | ProjectAuthor | ProjectAuthor | ProjectAuthor | ProjectAuthor | 未来签署权已指定；非 Gate 通过 |
| M5 | ProjectAuthor | ProjectAuthor | ProjectAuthor | ProjectAuthor | 未来签署权已指定；非 Gate 通过 |

## 未决事项

范围批准与未来签署权已记录；P0-001 仍在 `IN_REVIEW`，等待对记录完整性和单人角色冲突限制的独立复核，不可标记 `ACCEPTED`。
