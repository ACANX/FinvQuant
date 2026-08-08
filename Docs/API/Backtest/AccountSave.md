# POST /API/V1/Meta/Finv/Quant/Backtest/Account/Save — 新增/修改账户

新增或修改回测账户；`account_id` 为空则新增（自动生成 UUID），非空则 UPDATE。
服务端校验：account_code 唯一、初始资金 > 0、手续费/滑点 ≥ 0、margin_mode ∈ {FULL, FUTURES}。

## 请求

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quant/Backtest/Account/Save`

### 请求体（JSON，wire 字段 snake_case）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `account_id` | string | 可选 | 空则新增，非空则 UPDATE |
| `account_code` | string | ✅ | 账户编码（全局唯一） |
| `account_name` | string | ✅ | 账户名称 |
| `user_id` | string | 可选 | 所属用户（默认 `default`，多用户隔离） |
| `group_id` | string | 可选 | 子账户分组 / 主账户归属（单用户多子账户，NULL=主账户） |
| `env_id` | string | 可选 | 默认关联环境（缺省回测时取默认环境） |
| `initial_capital` | number | ✅ | 初始启动资金（>0） |
| `currency_type` | string | 可选 | 计价币种（默认 USD） |
| `commission_rate` | number | 可选 | 手续费率（按成交金额比例，默认 0） |
| `slippage_pct` | number | 可选 | 滑点（按成交价比例，默认 0） |
| `margin_mode` | string | 可选 | FULL 全额 / FUTURES 期货保证金（预留，默认 FULL） |
| `margin_rate` | number | 可选 | 保证金比例（默认 1） |
| `allow_backtest` | string | 可选 | 回测开关 `0`/`1` |
| `status` | string | 可选 | DRAFT/ENABLED/DISABLED |
| `remark` | string | 可选 | 备注 |

## 响应（成功 HTTP 200）

```json
{ "code": 0, "message": "保存成功", "data": { "account_id": "a0000000-0000-4000-8000-000000000001" } }
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
| 账户管理 | [Docs/Menu/Backtest/AccountManage.md](../../Menu/Backtest/AccountManage.md) | List / Get / Save / Toggle / Delete |
