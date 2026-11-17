# 量化回测模块 · 设计图集（Docs/Asset/Backtest）

> 本目录存放量化策略通用回测框架的设计图（SVG，可用浏览器直接打开）。
> 对应实现：`internal/backtest`（引擎/服务）+ `internal/api/handler/backtest.go`（API）
> + `Web/src/views/*Backtest*`（前端）+ `Deploy/Migrations/V22~V28/V100019~V100020`（存储）。

## 图集清单

| 文件 | 内容 | 说明 |
|------|------|------|
| [BacktestFrameworkArchitecture.svg](BacktestFrameworkArchitecture.svg) | ① 程序架构设计图 | 五层架构：前端展示层 → API 接入层（29 端点）→ 服务层（CRUD/调度/持久化）→ 回测引擎层（指标/表达式/撮合风控/规则账户/报告追踪）→ 数据存储层（10 张表 + 行情/证券） |
| [BacktestFullFlowTask.svg](BacktestFullFlowTask.svg) | ② 回测全流程任务流程图 | 从任务创建 → 三层开关校验 → 三快照 → 行情加载/聚合 → 预热 → 逐 bar 回放循环（挂单撮合 → 风控 → 信号 → 限制判定 → 下单 → 账户更新 → 市值快照）→ 收尾（EXPIRED/报告）→ 持久化 → SUCCEEDED |
| [BacktestDataFlow.svg](BacktestDataFlow.svg) | ③ 全流程数据流向图 | 输入（策略定义/账户/环境/行情）→ 处理（服务层+引擎）→ 输出（equity/trade/cashflow/position_log/event_trace/report）→ 展示（回测分析/资金持仓/链路追踪页签），附数据库对象清单 |
| [BacktestGCMainSequence.svg](BacktestGCMainSequence.svg) | ④ GCMain 黄金期货时序设计图 | 以 GCMain 双均线策略为例：创建任务（快照/区间）→ 异步调度（并发≤4）→ 加载约 48 万根分钟线 → 逐 bar 回放（信号/挂单/撮合/风控）→ 批量落库 → 进度轮询 → 报告查看的完整时序 |

## 与实现的关键对应

- **无未来函数**：信号 bar 收盘确认 → 次根开盘价成交（NEXT_BAR_OPEN）；`ref()` 负偏移编译期/运行期双重拦截；
- **环境自适应**：交易时段过滤（含跨午夜）、tick_size 对齐、T+N/涨跌停/合约乘数、撮合模式、币种校验、成本覆盖链（环境 > 任务 > 策略 > 账户）；
- **⑨ 链路追踪**：资金流水（连续可校验）、持仓变化明细（OPEN/ADD/REDUCE/CLOSE）、事件追踪（FR-10 八项登记：触发原因/触发时间/结果/结束时间/委托下单时间/委托耗时/存活时间/未成交原因）；
- **多用户**：策略/账户/任务/环境/模板按 `user_id` 隔离，List/Get/Toggle/Delete/CreateRun 均做归属校验；
- **可复现**：任务保存策略/账户/环境三快照；重启时悬挂任务自动标记 FAILED。

## 变更记录

- 2026-08-06：随 PR #338 评审修复轮（第三轮）新增本图集，四张图与最终实现保持一致。
