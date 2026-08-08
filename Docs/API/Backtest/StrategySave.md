# POST /API/V1/Meta/Finv/Quant/Backtest/Strategy/Save — 新增/修改策略

新增或修改结构化回测策略；`strategy_id` 为空则新增（自动生成 UUID），非空则 UPDATE。
保存时服务端对 definition 做结构校验与信号表达式编译校验（标识符交叉校验）。

## 请求

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Strategy/Save`

### 请求体（JSON，wire 字段 snake_case）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `strategy_id` | string | 可选 | 空则新增（自动生成 UUID），非空则 UPDATE |
| `strategy_code` | string | ✅ | 策略编码（全局唯一） |
| `strategy_name` | string | ✅ | 策略名称 |
| `strategy_type` | string | 可选 | 默认 `RULE_BASED`（当前版本仅实现该类型） |
| `description` | string | 可选 | 策略说明 |
| `definition` | object | ✅ | 结构化策略定义（JSON 模型 v1，保存时编译校验） |
| `definition_version` | int | 可选 | 定义版本（默认 1） |
| `data_period` | string | 可选 | 默认周期 Min/Hour/Day |
| `secu_code` | string | 可选 | 默认标的（缺省取 definition.universe.securities[0]） |
| `user_id` | string | 可选 | 所属用户（默认 `default`，多用户隔离） |
| `template_id` | string | 可选 | 来源模板（finv_quant_template） |
| `allow_backtest` | string | 可选 | 回测开关 `0`/`1`（默认 `1`） |
| `status` | string | 可选 | DRAFT/ENABLED/DISABLED（默认 ENABLED） |

## 响应（成功 HTTP 200）

```json
{ "code": 0, "message": "保存成功", "data": { "strategy_id": "b0000000-0000-4000-8000-000000000001" } }
```

> 信号表达式语法错误 / 指标 id 未声明时返回 400 / 4001。

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
