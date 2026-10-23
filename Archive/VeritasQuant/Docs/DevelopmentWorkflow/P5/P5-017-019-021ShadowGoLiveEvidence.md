# P5-017/019/021 影子运行与上线决策 — 证据

- **任务：** P5-017（ISSUE #213）、P5-019（#215）、P5-021（#217）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** 本 PR（P5 第四批）

## 范围

阶段 5 受控实盘前的影子运行与上线决策支撑工具：
影子运行冻结 → 上线前评审 → 每日 Go/No-Go。
验收口径为「能力就绪」（与 P2-041~043 同模式），实际影子运行与演练需模拟盘环境与时间窗口。

## 验收标准对照

| 任务 | 验收标准 | 实现 | 测试证据 |
|------|----------|------|----------|
| P5-017 | 版本与上限经双人签署；观察前不得修改阈值解释结果 | `security/ShadowFreeze.py`：`ShadowFreezeEntryV1`（双人签署、互不相同、负值拒绝）+ `ShadowFreezeServiceV1`（四类对象全覆盖、不可变 recordHash、SUPERSEDED 历史保留、篡改检测） | `tests/unit/security/test_shadow_freeze.py`（17 用例） |
| P5-019 | 开放 S0/S1、未解释对账、超期高风险和阻断行动项均为 0 | `security/GoLiveReview.py`：`ReviewCheckV1`（FAIL/NOT_EXECUTED 阻断）+ `GoLiveReviewServiceV1`（三类检查全覆盖、S0/S1/对账/超期行动项为 0 + 人工签署才 PASS） | `tests/unit/security/test_go_live_review.py`（20 用例） |
| P5-021 | 每日有指标快照、风险状态、审批人和唯一决策；失败自动退回仿真 | `security/DailyGoNoGo.py`：`DailyMetricSnapshotV1`（Decimal 语义利用率）+ `RiskStateV1` + `DailyGoNoGoServiceV1`（硬限制失败自动 NO_GO 退回仿真、决策哈希） | `tests/unit/security/test_daily_go_no_go.py`（15 用例） |

集成安全测试：`tests/integration/test_p5_shadow_go_live_safety.py`（12 用例，覆盖
双人签署/四类覆盖/篡改检测、评审门禁、硬限制自动退回，以及
「冻结 → 评审 → 每日 Go/No-Go」完整联动流程）。

## 技术方案要点

- 影子冻结：每项条目双人签署（禁止同人双签），清单必须覆盖账户/策略/额度/验收政策；
  `recordHash` 不可变，篡改即校验失败；额度变更走 supersede + 重新冻结，历史保留；
- 上线评审：三类检查（安全/可靠性/操作准备）缺一不可；FAIL/NOT_EXECUTED 均阻断；
  S0/S1、未解释对账、超期高风险行动项任一 > 0 即 FAIL；人工签署才 PASS；
- 每日 Go/No-Go：指标快照自动算利用率；硬限制失败（>100% 或违反数 > 0）
  自动 NO_GO 并退回仿真；开放告警/对账差异/保护控制激活同样 NO_GO；
  每日记录含决策哈希可审计。

## 验证结果

- 本批新增 **64** 个测试（17+20+15+12），全部通过；
- ruff / mypy / Preflight 全绿；
- 更新：TechSpec 新增 8.9「影子运行冻结、上线评审与每日 Go/No-Go 契约」；
- 登记表 P5-017/019/021 登记（IN_REVIEW）；TraceabilityMatrix 挂接。
