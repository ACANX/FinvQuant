# POST /API/V1/Meta/Finv/Quant/Backtest/Run/Create — 创建并启动回测任务

创建回测任务（策略快照 + 账户快照 + 环境快照 + 标的区间 + 回测配置），校验通过后**异步执行**，
返回任务初始状态（PENDING/RUNNING），前端轮询 `Run/Get` / `Run/List` 查看进度与结果。

## 请求

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Run/Create`

### 请求体（JSON，wire 字段 snake_case）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `strategy_id` | string | ✅ | 策略 ID（策略 allow_backtest 必须为 `1`） |
| `account_id` | string | ✅ | 账户 ID（账户 allow_backtest 必须为 `1`） |
| `env_id` | string | 可选 | 环境 ID（缺省取账户 env_id → 系统默认回测环境；环境 allow_backtest 必须为 `1`） |
| `created_by` | string | 可选 | 创建人（默认 `console`，接入认证后为登录账号） |
| `user_id` | string | 可选 | 所属用户（默认 `default`，多用户隔离） |
| `secu_code` | string | 可选 | 回测标的（缺省取策略 universe.securities[0]，如 GCMain） |
| `start_date` | int | 可选 | 起始交易日 yyyymmdd（缺省用行情最早日期） |
| `end_date` | int | 可选 | 结束交易日 yyyymmdd（缺省用行情最晚日期） |
| `period` | string | 可选 | 数据周期 Min/Hour/Day（缺省取策略 data.period） |
| `report_precision` | string | 可选 | 报告时间精度 Min/Hour/Day（缺省 Day） |
| `options` | object | 可选 | 回测配置，见下表 |

### options 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `enable_backtest` | bool | **回测开关**（false 直接拒绝创建） |
| `initial_capital` | number | 初始资金覆盖（覆盖账户初始资金，可选） |
| `commission_rate` | number | 手续费率覆盖（覆盖链：环境 > 任务 > 策略 > 账户，可选） |
| `slippage_pct` | number | 滑点覆盖（可选） |
| `max_trades_per_day` | int | 每日最大成交笔数覆盖（可选） |
| `allowed_times` | string[] | 限定交易时间点覆盖（hhmmss 数组，可选） |

## 请求示例

```json
{
  "strategy_id": "b0000000-0000-4000-8000-000000000001",
  "account_id": "a0000000-0000-4000-8000-000000000001",
  "env_id": "e0000000-0000-4000-8000-000000000001",
  "secu_code": "GCMain",
  "start_date": 20180101,
  "end_date": 20260731,
  "period": "Min",
  "report_precision": "Day",
  "options": { "enable_backtest": true, "max_trades_per_day": 10 }
}
```

## 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "回测任务已创建并启动",
  "data": {
    "run_id": "xxxxx",
    "run_no": 1,
    "status": "PENDING",
    "progress": 0,
    "secu_code": "GCMain",
    "period": "Min",
    "report_precision": "Day",
    "start_date": 20180101,
    "end_date": 20260731
  }
}
```

## 错误码 / 失败原因（message）

| 场景 | HTTP / code | 说明 |
|------|------|------|
| `回测开关未启用...` | 400 / 4001 | 策略 / 账户 / 环境 allow_backtest 为 `0`，或 options.enable_backtest=false |
| `策略不存在 / 账户不存在 / 环境不存在` | 404 / 4004 | 引用对象缺失（或不属于当前用户） |
| `环境...币种...不一致` | 409 / 4009 | 环境计价币种与账户币种不一致 |
| `标的 xxx 无任何行情数据` | 400 / 4001 | finv_quote_secu_kline_min 无该标的行情 |
| `开始日期不能晚于结束日期` | 400 / 4001 | 日期区间非法 |
| `不支持的周期 / 报告精度` | 400 / 4001 | 枚举越界 |

## 执行链路

创建任务 → 插入 `finv_quant_backtest_run`（PENDING，含策略/账户/环境快照）→ 异步 goroutine：
RUNNING → 加载行情（`finv_quote_secu_kline_min` 按 date 区间）→ 按 period 聚合 →
回测引擎逐 bar 回放（环境交易时段过滤 / tick_size 对齐 / 成本覆盖链）→ 落库
曲线（`finv_quant_backtest_equity`）、成交（`finv_quant_backtest_trade`）、
链路追踪（`finv_quant_backtest_cashflow` / `position_log` / `event_trace`）→ SUCCEEDED（report 写入）。

## 已使用位置（业务菜单）

| 业务菜单 | 菜单文档 | 使用接口 |
|----------|----------|----------|
| 黄金期货合约回测验证 | [Docs/Menu/Backtest/BacktestGoldFutures.md](../../Menu/Backtest/BacktestGoldFutures.md) | Run/Create（启动回测） |
