# GET /API/V1/Meta/Finv/Quant/Backtest/Run/EventTraces — 查询事件追踪（⑨链路追踪）

分页查询回测任务的事件追踪明细（⑨-3）：触发原因（买入信号/卖出信号/止损/止盈）、
成交结果（FILLED/REJECTED/EXPIRED/PENDING）、委托耗时（bar 数·秒）、未成交原因（10 类）。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Run/EventTraces?runId=xxx&page=1&pageSize=1000`

### Query 参数（URL 查询参数统一小驼峰 camelCase）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `runId` | string | ✅ | 任务 ID（缺失返回 400 / 4001） |
| `userId` | string | 可选 | 所属用户（默认 `default`，归属校验） |
| `page` | int | 可选 | 页码（默认 1） |
| `pageSize` | int | 可选 | 每页条数（默认 20；明细查询建议 1000，单页上限 5000） |

## 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 320,
    "list": [
      {
        "event_id": 1, "seq": 1, "action": "BUY",
        "trigger_reason": "买入信号", "trigger_ts": 1514764800, "trigger_date": 20180102, "trigger_time": 93000,
        "exec_status": "FILLED", "exec_ts": 1514764860, "exec_date": 20180102, "exec_time": 93100,
        "latency_bars": 1, "latency_sec": 60,
        "reject_reason": "", "price": 1310.5, "qty": 10, "trade_id": 1
      }
    ]
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
