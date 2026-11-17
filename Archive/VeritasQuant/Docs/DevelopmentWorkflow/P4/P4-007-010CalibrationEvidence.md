# P4-007~010 执行校准与安全测试 — 证据

- **任务：** P4-007（ISSUE #189）、P4-008（#190）、P4-009（#191）、P4-010（#192）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** ACANX/VeritasQuant#241（已合并 2026-08-02T23:55:39Z）

## 范围

阶段 4 执行校准与安全测试：高精度诊断时间采集 → 执行校准数据集与候选
参数生成 → 候选模型 A/B 回测与批准流程 → 券商契约/限频/断连/结果未知测试。

## 验收标准对照

| 任务 | 验收标准 | 实现 | 测试证据 |
|------|----------|------|----------|
| P4-007 | submitted/accepted/filled 等保留来源精度且不参与 ts 因果排序 | `DiagnosticCollectorV1`（按 clientOrderId 采集阶段时间，重复阶段取最晚；时间线排序与诊断分离）；`ExecutionReasonV1`（受控原因代码）；`DiagnosticReportV1`（阶段延迟） | `tests/unit/broker/test_diagnostics.py`（11 用例） |
| P4-008 | 延迟、滑点、成交率、部分成交按标的/时段可重算；数据和代码版本齐全 | `CalibrationDatasetBuilderV1`（从诊断报告构建样本；按标的+时段聚合 p50/p95；builderVersion+dataVersion）；`CandidateParameterGeneratorV1`（确定性候选生成） | `tests/unit/broker/test_calibration.py`（12 用例） |
| P4-009 | 候选先跑固定历史样本；未审批不成为默认；避免同样本训练验收 | `AbTestEvaluatorV1`（训练/验证/留出三段划分，训练集不参与验收）；`ModelApprovalWorkflowV1`（批准必须有评估证据且留出集跑赢基准；未审批不得 setDefault） | `tests/unit/broker/test_model_approval.py`（12 用例） |
| P4-010 | 两次断连、重放、超时、限频和未知结果均无重复订单/账本 | `tests/integration/test_p4_broker_contract_safety.py`：断连重放去重（两轮）；超时 TIMEOUT_UNKNOWN 不盲目重发；限频滑动窗口拒绝超额；未知结果隔离查询；重复提交幂等映射 | `tests/integration/test_p4_broker_contract_safety.py`（9 用例） |

## 技术方案要点

- 诊断时间只做分析不参与事件 `ts` 因果排序（与 ExecutionReportEventV1
  diagnosticTs 语义一致）；来源精度保留毫秒；
- 校准样本从诊断时间线提取（成交价经 detail 传递，滑点按基准价计算转基点）；
- A/B 采用固定历史样本三段划分：训练集成绩不作为验收依据，批准以留出集
  跑赢基准为准（避免同样本训练验收）；
- 限频在能力协商层之后、发单前强制（滑动窗口 1 秒），超额拒绝且不产生
  任何订单副作用；
- 重复提交同一 clientOrderId 复用既有映射（幂等，不重复记账）。

## 验证结果

- ruff：All checks passed
- mypy：Success（broker 9 源文件）
- Preflight：0 issues
- 全量 pytest：1198 passed / 36 skipped（skipped 为 PG/Redis 集成，CI database job 覆盖）
- 新增测试：44 用例（P4-007: 11 + P4-008: 12 + P4-009: 12 + P4-010: 9）
- CI 4/4：Quality-ubuntu ✅ / Quality-windows ✅ / Security-baseline ✅ / Database-migrations ✅（PR #241 run 30772682151）

## 验收结论

- **状态：** P4-007~010 全部 ACCEPTED（2026-08-03，流转 PR 本 PR）
- **依据：** 执行校准与安全测试能力（诊断/校准/AB 批准/契约安全）已交付并通过 CI；验收标准逐项对照见上表
- **遗留：** P4-013 校准精度评估需 P4-011 真实运行数据；P4-011/012/014 为运行/Gate 类需仿真环境

## 风险与开放项

- P4-013（执行模型校准精度评估）需真实仿真运行数据（P4-011 窗口）；
- P4-011/012/014 为运行/Gate 类任务，需仿真环境与 20 交易日窗口。
