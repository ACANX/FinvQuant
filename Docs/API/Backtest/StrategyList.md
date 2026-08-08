# GET /API/V1/Meta/Finv/Quant/Backtest/Strategy/List — 分页查询策略

分页查询结构化回测策略（支持关键字与回测开关过滤，按用户隔离）。

> 策略定义采用通用可扩展 JSON 模型（universe/data/indicators/signals/rules/risk/cost），
> 模型规范见 [Docs/DevSpec/BacktestStrategySpec.md](../../DevSpec/BacktestStrategySpec.md)。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Strategy/List`

### Query 参数（URL 查询参数统一小驼峰 camelCase）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `keyword` | string | 可选 | 关键字，匹配 strategy_code / strategy_name / description / secu_code |
| `allowBacktest` | string | 可选 | 按回测开关过滤：`0`（关闭）/ `1`（开启） |
| `userId` | string | 可选 | 所属用户（默认 `default`，多用户隔离；接入 JWT/RBAC 后由登录态决定） |
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
        "strategy_id": "b0000000-0000-4000-8000-000000000001",
        "strategy_code": "STRAT-DUALMA-GC",
        "strategy_name": "GCMain 双均线交叉策略",
        "strategy_type": "RULE_BASED",
        "description": "双均线交叉策略（示例）",
        "definition": { "version": "1", "universe": { "securities": ["GCMain"] }, "indicators": [] },
        "definition_version": 1,
        "data_period": "Min",
        "secu_code": "GCMain",
        "user_id": "default",
        "template_id": null,
        "allow_backtest": "1",
        "status": "ENABLED",
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
| 策略管理 | [Docs/Menu/Backtest/StrategyManage.md](../../Menu/Backtest/StrategyManage.md) | List / Get / Save / Toggle / Delete |
| 黄金期货合约回测验证 | [Docs/Menu/Backtest/BacktestGoldFutures.md](../../Menu/Backtest/BacktestGoldFutures.md) | List（策略下拉） |
