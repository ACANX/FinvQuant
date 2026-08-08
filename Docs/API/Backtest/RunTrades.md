# GET /API/V1/Meta/Finv/Quant/Backtest/Run/Trades — 查询成交记录

分页查询回测任务的成交记录（时间/方向/价格/数量/金额/手续费/平仓盈亏/成交后持仓与现金/触发信号/引擎内序号 seq）。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Run/Trades?runId=xxx&page=1&pageSize=500`

### Query 参数（URL 查询参数统一小驼峰 camelCase）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `runId` | string | ✅ | 任务 ID（缺失返回 400 / 4001） |
| `userId` | string | 可选 | 所属用户（默认 `default`，归属校验） |
| `page` | int | 可选 | 页码（默认 1） |
| `pageSize` | int | 可选 | 每页条数（默认 20；明细查询建议 500，单页上限 5000） |

## 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 180,
    "list": [
      {
        "trade_id": 1, "seq": 1, "ts": 1514764800, "date": 20180102, "time": 93000,
        "action": "BUY", "price": 1310.5, "qty": 10, "amount": 13105,
        "fee": 3.93, "profit": 0, "position_after": 10, "cash_after": 86891,
        "signal": "买入信号"
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
| 持仓管理 | [Docs/Menu/Backtest/PositionManage.md](../../Menu/Backtest/PositionManage.md) | List / Equity / Trades |
