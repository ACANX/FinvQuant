# GET /API/V1/Meta/Finv/Quant/Backtest/Template/Get — 查询模板详情

按模板 ID 查询模板详情（含 content JSON 内容）。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Template/Get?templateId=xxx`

### Query 参数（URL 查询参数统一小驼峰 camelCase）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `templateId` | string | ✅ | 模板 ID（缺失返回 400 / 4001） |
| `userId` | string | 可选 | 所属用户（默认 `default`，归属校验：他人数据不可读） |

## 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "template_id": "t0000000-0000-4000-8000-000000000002",
    "template_code": "TPL-STRAT-DUALMA",
    "template_name": "双均线交叉策略模板",
    "template_type": "STRATEGY",
    "content": { "version": "1", "universe": { "securities": ["GCMain"] }, "indicators": [] },
    "is_builtin": "1",
    "status": "ENABLED"
  }
}
```

> 模板不存在或不属于当前用户时返回 404 / 4004。

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
