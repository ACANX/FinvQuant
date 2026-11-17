# GET /API/V1/Meta/Finv/Quant/Backtest/Run/Report — 查询回测报告（汇总指标）

查询回测任务的汇总收益报告（仅 SUCCEEDED 任务可查）。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Run/Report?runId=xxx`

### Query 参数（URL 查询参数统一小驼峰 camelCase）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `runId` | string | ✅ | 任务 ID（缺失返回 400 / 4001） |
| `userId` | string | 可选 | 所属用户（默认 `default`，归属校验） |

## 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "initial_capital": 100000,
    "final_equity": 112340,
    "total_profit": 12340,
    "total_return_pct": 12.34,
    "annual_return_pct": 1.46,
    "max_drawdown_pct": 8.21,
    "max_drawdown_start": 20200301,
    "max_drawdown_end": 20200415,
    "sharpe_ratio": 0.62,
    "annual_volatility_pct": 15.2,
    "max_investment": 98000,
    "avg_investment": 72100,
    "holding_days": 620,
    "win_rate_pct": 55.3,
    "profit_factor": 1.72,
    "total_fee": 183.5,
    "event_stats": { "triggered": 320, "filled": 180, "rejected": 120, "expired": 20 }
  }
}
```

> 任务不存在 / 不属于当前用户 / 非 SUCCEEDED 时返回 404 / 4004。

## 错误码

| code | HTTP | 说明 |
|------|------|------|
| 0 | 200 | 成功 |
| 4001 | 400 | 请求体格式错误 / 参数校验错误（必填缺失、表达式错误、枚举越界等） |
| 4004 | 404 | 资源不存在 |
| 4009 | 409 | 状态冲突 / 禁止操作（已关联任务禁止删除、内置模板禁止修改、无权访问他人数据等） |
| 2006 | 500 | 其他服务端错误（message 给出具体原因） |

## 已使用位置（业务菜单）

| 业务菜单 | 菜单文档 | 使用接口 |
|----------|----------|----------|
| 回测分析 | [Docs/Menu/Backtest/BacktestAnalysis.md](../../Menu/Backtest/BacktestAnalysis.md) | List / Get / Report / Equity / Trades / Cashflows / PositionLogs / EventTraces / Cancel |
