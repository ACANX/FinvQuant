# GET /API/V1/Meta/Finv/Quant/Backtest/Template/List — 分页查询模板

分页查询模板（策略/账户/环境三类，STRATEGY/ACCOUNT/ENVIRONMENT），支持类型过滤；
内置模板（is_builtin=1，user_id='system'）全局可见且禁止删除。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Template/List`

### Query 参数（URL 查询参数统一小驼峰 camelCase）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `templateType` | string | 可选 | STRATEGY / ACCOUNT / ENVIRONMENT |
| `keyword` | string | 可选 | 匹配 template_code / template_name / description |
| `userId` | string | 可选 | 所属用户（默认 `default`；`system` 内置模板全局可见） |
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
        "template_id": "t0000000-0000-4000-8000-000000000002",
        "template_code": "TPL-STRAT-DUALMA",
        "template_name": "双均线交叉策略模板",
        "template_type": "STRATEGY",
        "content": { "version": "1", "universe": { "securities": ["GCMain"] }, "indicators": [] },
        "user_id": "system",
        "is_builtin": "1",
        "status": "ENABLED",
        "description": "内置：双均线交叉策略模板（GCMain 示例）",
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
| 模板管理 | [Docs/Menu/Backtest/TemplateManage.md](../../Menu/Backtest/TemplateManage.md) | List / Get / Save / Delete |
| 策略管理 | [Docs/Menu/Backtest/StrategyManage.md](../../Menu/Backtest/StrategyManage.md) | List（templateType=STRATEGY 载入策略模板） |
