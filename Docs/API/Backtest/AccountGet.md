# GET /API/V1/Meta/Finv/Quant/Backtest/Account/Get — 查询账户详情

按账户 ID 查询回测账户详情。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Account/Get?accountId=xxx`

### Query 参数（URL 查询参数统一小驼峰 camelCase）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `accountId` | string | ✅ | 账户 ID（缺失返回 400 / 4001） |
| `userId` | string | 可选 | 所属用户（默认 `default`，归属校验：他人数据不可读） |

## 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "account_id": "a0000000-0000-4000-8000-000000000001",
    "account_code": "ACCT-GOLD-001",
    "account_name": "黄金期货回测账户",
    "initial_capital": 100000,
    "allow_backtest": "1",
    "status": "ENABLED"
  }
}
```

> 账户不存在或不属于当前用户时返回 404 / 4004。

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
