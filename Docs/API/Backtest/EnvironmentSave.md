# POST /API/V1/Meta/Finv/Quant/Backtest/Environment/Save — 新增/修改环境

新增或修改回测环境；`env_id` 为空则新增（自动生成 UUID），非空则 UPDATE。
服务端校验：trading_sessions 起止需 6 位 hhmmss、tick_size ≥ 0。

## 请求

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Environment/Save`

### 请求体（JSON，wire 字段 snake_case）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `env_id` | string | 可选 | 空则新增，非空则 UPDATE |
| `env_code` | string | ✅ | 环境编码（全局唯一） |
| `env_name` | string | ✅ | 环境名称 |
| `env_type` | string | 可选 | BACKTEST/PAPER/SIMULATION/LIVE（默认 BACKTEST） |
| `region` | string | 可选 | 地区（CN/US/HK...） |
| `market_code` | int | 可选 | 关联市场（默认 0=通用） |
| `config` | object | 可选 | 环境配置（trading_sessions / trading_rules / cost / fill_mode / currency / preferences） |
| `user_id` | string | 可选 | 所属用户（默认 `default`） |
| `is_default` | string | 可选 | 是否默认环境 `0`/`1` |
| `allow_backtest` | string | 可选 | 回测开关 `0`/`1` |
| `status` | string | 可选 | DRAFT/ENABLED/DISABLED |
| `description` | string | 可选 | 说明 |

## 响应（成功 HTTP 200）

```json
{ "code": 0, "message": "保存成功", "data": { "env_id": "e0000000-0000-4000-8000-000000000001" } }
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
