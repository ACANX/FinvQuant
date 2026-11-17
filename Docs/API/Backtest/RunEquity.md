# GET /API/V1/Meta/Finv/Quant/Backtest/Run/Equity — 查询净值曲线数据

按报告精度（Day/Hour/Min）逐点返回净值曲线：余额（cash）、总资产（equity=现金+持仓市值）、
持仓金额（position_value）、持仓数量（position_qty）、累计收益额（profit）、
累计收益率（roi）、回撤（drawdown）。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Run/Equity?runId=xxx&page=1&pageSize=5000`

### Query 参数（URL 查询参数统一小驼峰 camelCase）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `runId` | string | ✅ | 任务 ID（缺失返回 400 / 4001） |
| `userId` | string | 可选 | 所属用户（默认 `default`，归属校验） |
| `page` | int | 可选 | 页码（默认 1） |
| `pageSize` | int | 可选 | 每页条数（默认 20；曲线查询建议 5000，单页上限 5000） |

## 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 2092,
    "list": [
      {
        "seq": 1, "ts": 1514764800, "date": 20180101, "time": 93000,
        "equity": 100000, "cash": 100000, "position_value": 0, "position_qty": 0,
        "profit": 0, "roi": 0, "drawdown": 0
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
| 资金管理 | [Docs/Menu/Backtest/FundManage.md](../../Menu/Backtest/FundManage.md) | List / Equity |
| 持仓管理 | [Docs/Menu/Backtest/PositionManage.md](../../Menu/Backtest/PositionManage.md) | List / Equity / Trades |
