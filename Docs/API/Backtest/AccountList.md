# GET /API/V1/Meta/Finv/Quant/Backtest/Account/List — 分页查询账户

分页查询回测账户（支持关键字与回测开关过滤，按用户隔离；含多用户 user_id / 多子账户 group_id）。

> 账户 = 回测运行的「初始资金 + 交易成本 + 保证金模式」基线配置。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Account/List`

### Query 参数（URL 查询参数统一小驼峰 camelCase）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `keyword` | string | 可选 | 匹配 account_code / account_name / remark |
| `allowBacktest` | string | 可选 | 回测开关过滤 `0`/`1` |
| `userId` | string | 可选 | 所属用户（默认 `default`，多用户隔离） |
| `page` | int | 可选 | 页码（默认 1） |
| `pageSize` | int | 可选 | 每页条数（默认 20，上限 500） |

## 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 1,
    "list": [
      {
        "account_id": "a0000000-0000-4000-8000-000000000001",
        "account_code": "ACCT-GOLD-001",
        "account_name": "黄金期货回测账户",
        "user_id": "default",
        "group_id": null,
        "env_id": null,
        "initial_capital": 100000,
        "currency_type": "USD",
        "commission_rate": 0.0003,
        "slippage_pct": 0.0001,
        "margin_mode": "FULL",
        "margin_rate": 1,
        "allow_backtest": "1",
        "status": "ENABLED",
        "remark": "默认回测账户（GCMain 黄金期货主连）",
        "created_by": "system"
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
| 账户管理 | [Docs/Menu/Backtest/AccountManage.md](../../Menu/Backtest/AccountManage.md) | List / Get / Save / Toggle / Delete |
| 黄金期货合约回测验证 | [Docs/Menu/Backtest/BacktestGoldFutures.md](../../Menu/Backtest/BacktestGoldFutures.md) | List（账户下拉） |
