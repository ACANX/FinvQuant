# P6-007 离线优化与模型管理 — 证据

- **任务：** P6-007（ISSUE #129）Optuna/MLflow 优化与模型管理
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **启动条件：** M1 稳定（M1 Gate 已由 ACANX 签署 PASS，2026-08-01T23:44:48Z）✅
- **PR：** 本 PR（P6 第一批）

## 范围

后续资产与优化能力待办池中的 **Optuna/MLflow 优化与模型管理**。采用纯标准库实现
（不引入 optuna/mlflow 外部依赖，与 P2-036 Prometheus 同模式），提供：
试验追踪（训练/验证/留出三段隔离）、确定性超参搜索、策略 Gate 隔离。

## 验收标准对照

| 验收标准 | 实现 | 测试证据 |
|----------|------|----------|
| 训练/验证/留出隔离 | `optimization/ExperimentTracker.py`：`DatasetSplit` 三段枚举；`TrialV1` 分段成绩；留出段默认锁定（`recordHoldout` 需显式 `unlockHoldout`，记录后重新锁定）；搜索期间写入留出成绩被拒绝 | `tests/unit/optimization/test_experiment_tracker.py`（15 用例） |
| 试验可复现 | 试验身份哈希只含确定性输入/输出（参数/数据版本/种子/实现版本/成绩），不含创建时间与自增 ID；`reproducibleWith()` 校验；固定种子网格/随机搜索产生相同结果与试验哈希 | `tests/unit/optimization/test_hyperparameter_search.py`（13 用例）+ 集成可复现测试 |
| 不能自动绕过策略 Gate | `optimization/OptimizationGate.py`：`autoAdopt()` 永远返回 `PENDING`；采用必须政策哈希匹配 + 留出达标（最小交易数/净收益下界/最大回撤）+ 至少两名互不相同批准人；`CandidateAdoptionV1` 不可变含哈希 | `tests/unit/optimization/test_optimization_gate.py`（14 用例） |

集成测试：`tests/integration/test_p6_optimization_flow.py`（4 用例，覆盖
搜索 → 留出隔离评估 → Gate 批准 → 采用完整流程、Gate 阻断、禁止自动采用、跨运行可复现）。

## 技术方案要点

- 三段隔离：搜索只用训练/验证段（`bestByValidation` 选优），留出段批准前锁定；
- 可复现：`computeHash` 排除运行时间/自增 ID，同输入必然同哈希；
- Gate 隔离：优化结果自动晋级返回 PENDING；双人批准 + 冻结政策哈希校验；
- 纯标准库实现：无新外部依赖。

## 验证结果

- 本批新增 **46** 个测试（15+13+14+4），全部通过；
- ruff / mypy / Preflight 全绿；
- 更新：TechSpec 新增 8.10「离线优化与模型管理契约」；
- 登记表 P6-007 登记（IN_REVIEW）；TraceabilityMatrix 挂接。
