# POST /API/V1/Meta/Finv/Quant/Backtest/Template/Save — 新增/修改模板

新增或修改模板；`template_id` 为空则新增（自动生成 UUID），非空则 UPDATE。
服务端强制 `is_builtin='0'`（防伪造内置标识）。

## 请求

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Template/Save`

### 请求体（JSON，wire 字段 snake_case）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `template_id` | string | 可选 | 空则新增，非空则 UPDATE |
| `template_code` | string | ✅ | 模板编码（全局唯一） |
| `template_name` | string | ✅ | 模板名称 |
| `template_type` | string | ✅ | STRATEGY / ACCOUNT / ENVIRONMENT |
| `content` | object | ✅ | 模板内容（STRATEGY=策略定义 / ACCOUNT=账户配置 / ENVIRONMENT=环境配置） |
| `user_id` | string | 可选 | 所属用户（默认 `default`） |
| `is_builtin` | string | 可选 | 是否内置 `0`/`1`（服务端强制 `0`） |
| `status` | string | 可选 | DRAFT/ENABLED/DISABLED |
| `description` | string | 可选 | 说明 |

## 响应（成功 HTTP 200）

```json
{ "code": 0, "message": "保存成功", "data": { "template_id": "t0000000-0000-4000-8000-000000000002" } }
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
