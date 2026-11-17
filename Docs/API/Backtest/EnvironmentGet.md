# GET /API/V1/Meta/Finv/Quant/Backtest/Environment/Get — 查询环境详情

按环境 ID 查询回测环境详情（含 config JSON 配置）。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Environment/Get?envId=xxx`

### Query 参数（URL 查询参数统一小驼峰 camelCase）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `envId` | string | ✅ | 环境 ID（缺失返回 400 / 4001） |
| `userId` | string | 可选 | 所属用户（默认 `default`，归属校验：他人数据不可读） |

## 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "env_id": "e0000000-0000-4000-8000-000000000001",
    "env_code": "ENV-BT-COMEX-GC",
    "env_name": "COMEX 黄金期货回测环境",
    "env_type": "BACKTEST",
    "config": { "trading_sessions": [{ "start": "082000", "end": "133000" }], "currency": "USD" },
    "allow_backtest": "1",
    "status": "ENABLED"
  }
}
```

> 环境不存在或不属于当前用户时返回 404 / 4004。

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
