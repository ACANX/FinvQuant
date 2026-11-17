# GET /API/V1/Meta/Finv/Quant/Backtest/Environment/List — 分页查询环境

分页查询回测环境（BACKTEST/PAPER/SIMULATION/LIVE × 地区市场），支持类型过滤；
`system` 内置环境全局可见，列表返回 system + 当前用户环境。

> 环境驱动引擎自适应不同市场的交易约束与规则（交易时段、tick_size、成本基准、撮合模式、币种、偏好），
> 环境配置结构见 [Docs/DevSpec/BacktestStrategySpec.md](../../DevSpec/BacktestStrategySpec.md) 第 10 章。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Environment/List`

### Query 参数（URL 查询参数统一小驼峰 camelCase）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `envType` | string | 可选 | BACKTEST / PAPER / SIMULATION / LIVE |
| `keyword` | string | 可选 | 匹配 env_code / env_name / description / region |
| `userId` | string | 可选 | 所属用户（默认 `default`；`system` 内置环境全局可见） |
| `page` | int | 可选 | 页码（默认 1） |
| `pageSize` | int | 可选 | 每页条数（默认 20，上限 500） |

## 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 2,
    "list": [
      {
        "env_id": "e0000000-0000-4000-8000-000000000001",
        "env_code": "ENV-BT-COMEX-GC",
        "env_name": "COMEX 黄金期货回测环境",
        "env_type": "BACKTEST",
        "region": "US",
        "market_code": 0,
        "config": {
          "trading_sessions": [{ "start": "082000", "end": "133000" }],
          "trading_rules": { "t_plus": 0, "tick_size": 0.1, "contract_multiplier": 100, "limit_up_pct": 0, "limit_down_pct": 0 },
          "cost": { "commission_rate": 0.0003, "slippage_pct": 0.0001 },
          "fill_mode": "NEXT_BAR_OPEN",
          "currency": "USD",
          "preferences": { "date_format": "YYYY-MM-DD", "quote_direction": "RED_UP" }
        },
        "user_id": "system",
        "is_default": "1",
        "allow_backtest": "1",
        "status": "ENABLED",
        "description": "COMEX 黄金期货（GCMain 主连）回测默认环境",
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
| 环境管理 | [Docs/Menu/Backtest/EnvironmentManage.md](../../Menu/Backtest/EnvironmentManage.md) | List / Get / Save / Toggle / Delete |
| 黄金期货合约回测验证 | [Docs/Menu/Backtest/BacktestGoldFutures.md](../../Menu/Backtest/BacktestGoldFutures.md) | List（环境下拉） |
