# POST /API/V1/Meta/Finv/Quant/Backtest/Environment/Delete — 删除环境

删除回测环境；环境已关联回测任务时禁止删除（返回 409 / 4009，可改为禁用）。

## 请求

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Environment/Delete`

### 请求体（JSON，wire 字段 snake_case）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `env_id` | string | ✅ | 环境 ID |
| `user_id` | string | 可选 | 所属用户（默认 `default`，归属校验） |

## 响应（成功 HTTP 200）

```json
{ "code": 0, "message": "删除成功" }
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
