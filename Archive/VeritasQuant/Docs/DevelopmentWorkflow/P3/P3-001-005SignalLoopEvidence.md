# P3-001~005 信号参考闭环 — 证据

- **任务：** P3-001（ISSUE #173）、P3-002（#174）、P3-003（#175）、P3-004（#176）、P3-005（#177）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** ACANX/VeritasQuant#235（已合并 2026-08-02T22:23:48Z）

## 范围

阶段 3 信号参考闭环核心链路：信号参考契约 → 信号生成与幂等发布 → 通知路由
→ 人工审核动作登记 → 授权命令写入订单/账本。

## 验收标准对照

| 任务 | 验收标准 | 实现 | 测试证据 |
|------|----------|------|----------|
| P3-001 | 状态、版本、账户、策略、来源事件和操作者字段完整且不可变 | `SignalReferenceV1`（固定字段 + 版本链 + 不可变 transition）；`ManualReviewActionV1`（身份/理由/ts/版本/审计）；`ManualExecutionV1`；`IgnoreReasonV1` | `tests/unit/signals/test_signal_reference.py`（20 用例） |
| P3-002 | 相同输入与冻结策略 checksum 一致；重复事件不重复信号 | `SignalGeneratorV1`（内容 checksum 与生成时间无关）；`SignalPublisherV1`（账户+策略+来源事件幂等键；同键同内容=重复投递返回既有信号；同键异内容=冲突拒绝+留档） | `tests/unit/signals/test_signal_generator.py`（10 用例） |
| P3-003 | 通知失败不改变交易控制；重试不重复人工任务；投递结果可审计 | `NotificationRouterV1`（模板渲染 + 有界重试 + 失败隔离；signal+channel 幂等键；每次尝试留档） | `tests/unit/signals/test_notification_router.py`（10 用例） |
| P3-004 | 每个动作有身份、理由、ts、版本和审计；不能直接修改内核或账本 | `ManualActionServiceV1` + `SignalRoutes`（POST /api/v1/signals/{id}/actions、/executions；动作只登记意图） | `tests/unit/signals/test_manual_action_service.py`（12 用例） |
| P3-005 | 对账一致；绕过命令资源或直接改投影的请求被拒绝 | `ManualExecutionExecutorV1`（CommandResource 提交 manual_execution；仅 AUTHORIZING/ACCEPTED/RUNNING 可写入；PENDING/直接调用 writer 被拒绝） | `tests/unit/signals/test_manual_execution.py`（7 用例） |

## 技术方案要点

- **TechSpec 8.7** 新增"信号参考、人工审核与人工成交契约"章节；
- 信号生成 `generatedTs` 取事件可用时间（`availableTs`），不取服务器时间，
  保证回放确定性；
- 金额/数量路径全程字符串（Decimal 文本），禁止 float；
- 动作登记与订单/账本写入严格分离：P3-004 只登记意图，P3-005 通过
  授权命令执行，绕过命令资源直接改投影 = 契约违规被拒绝；
- 通知路由层无交易副作用，失败仅记录 FAILED 状态，不触碰交易控制。

## 验证结果

- ruff：All checks passed
- mypy：Success（signals 6 源文件 + SignalRoutes）
- Preflight：0 issues
- 全量 pytest：1069 passed / 36 skipped（skipped 为 PG/Redis 集成，CI database job 覆盖）
- 新增测试：59 用例（P3-001~005）
- CI 4/4：Quality-ubuntu ✅ / Quality-windows ✅ / Security-baseline ✅ / Database-migrations ✅（PR #235 run 30769549258）

## 验收结论

- **状态：** P3-001~005 全部 ACCEPTED（2026-08-03，流转 PR 本 PR）
- **依据：** 信号参考闭环能力（契约/生成幂等/通知路由/人工动作登记/授权命令写入）已交付并通过 CI；验收标准逐项对照见上表
- **遗留：** 实际 20 交易日运行与 50 条信号证据依赖模拟盘环境，M3 Gate 前补充

## 风险与开放项

- 信号事件类型尚未注册进 EventSchemaRegistry（P3-002 信号持久化事件化留待
  P3-009 运行前接入）；当前为内存存储 + 服务层幂等。
- GUI 端人工审核页面（P3-004 的 GUI 部分）待 Streamlit 接入阶段补充。
- 实际 20 交易日运行与 50 条信号证据依赖模拟盘环境，M3 Gate 前补充。
