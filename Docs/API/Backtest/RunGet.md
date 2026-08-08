# GET /API/V1/Meta/Finv/Quant/Backtest/Run/Get — 查询任务详情

按任务 ID 查询回测任务详情（含策略/账户/环境快照与报告摘要），供深链加载与进度轮询。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Run/Get?runId=xxx`

### Query 参数（URL 查询参数统一小驼峰 camelCase）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `runId` | string | ✅ | 任务 ID（缺失返回 400 / 4001） |
| `userId` | string | 可选 | 所属用户（默认 `default`，归属校验：他人数据不可读） |

## 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "run_id": "xxxxx",
    "run_no": 1,
    "status": "RUNNING",
    "progress": 42,
    "strategy_snapshot": {},
    "account_snapshot": {},
    "env_snapshot": { "env_code": "ENV-BT-COMEX-GC", "config": {} }
  }
}
```

> 任务不存在或不属于当前用户时返回 404 / 4004。

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
| 回测分析 | [Docs/Menu/Backtest/BacktestAnalysis.md](../../Menu/Backtest/BacktestAnalysis.md) | List / Get（深链加载）/ Report / Equity / Trades / Cashflows / PositionLogs / EventTraces / Cancel |
| 黄金期货合约回测验证 | [Docs/Menu/Backtest/BacktestGoldFutures.md](../../Menu/Backtest/BacktestGoldFutures.md) | Get（轮询进度） |
