# GET /API/V1/Meta/Finv/Quant/Backtest/Run/List — 分页查询回测任务

分页查询回测任务（支持状态 / 标的 / 策略过滤，按用户隔离），含进度与失败原因、报告摘要与快照。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Run/List`

### Query 参数（URL 查询参数统一小驼峰 camelCase）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `status` | string | 可选 | PENDING / RUNNING / SUCCEEDED / FAILED / CANCELLED |
| `secuCode` | string | 可选 | 按标的过滤 |
| `strategyId` | string | 可选 | 按策略过滤 |
| `keyword` | string | 可选 | 匹配 run_no / strategy_name / account_name / secu_code |
| `userId` | string | 可选 | 所属用户（默认 `default`，多用户隔离） |
| `page` | int | 可选 | 页码（默认 1） |
| `pageSize` | int | 可选 | 每页条数（默认 20，上限 500） |

## 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 3,
    "list": [
      {
        "run_id": "xxxxx", "run_no": 1, "user_id": "default",
        "strategy_id": "xxx", "strategy_code": "STRAT-DUALMA-GC", "strategy_name": "GCMain 双均线交叉策略",
        "account_id": "xxx", "account_code": "ACCT-GOLD-001", "account_name": "黄金期货回测账户",
        "env_id": "xxx", "environment_snapshot": { "env_code": "ENV-BT-COMEX-GC", "config": {} },
        "secu_code": "GCMain", "market_code": 33, "period": "Min", "report_precision": "Day",
        "start_ts": 1514764800, "end_ts": 1753920000, "start_date": 20180101, "end_date": 20260731,
        "status": "SUCCEEDED", "progress": 100, "error_message": "",
        "report": { "total_return_pct": 12.34, "final_equity": 112340, "event_stats": {} },
        "started_at": "2026-08-06T04:00:00Z", "finished_at": "2026-08-06T04:01:23Z"
      }
    ]
  }
}
```

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
| 黄金期货合约回测验证 | [Docs/Menu/Backtest/BacktestGoldFutures.md](../../Menu/Backtest/BacktestGoldFutures.md) | List（任务列表与轮询） |
| 资金管理 | [Docs/Menu/Backtest/FundManage.md](../../Menu/Backtest/FundManage.md) | List / Equity |
| 持仓管理 | [Docs/Menu/Backtest/PositionManage.md](../../Menu/Backtest/PositionManage.md) | List / Equity / Trades |
