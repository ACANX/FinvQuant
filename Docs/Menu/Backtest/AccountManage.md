# 账户管理（AccountManage）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/Backtest/AccountManage.md`
> 对应视图：`Web/src/views/AccountManageView.vue`
> 接口契约：见 [Docs/API/Backtest/AccountList.md](../../API/Backtest/AccountList.md)

## 1. 菜单入口

- **菜单位置**：左侧导航栏 → 账户管理
- **菜单名称**：账户管理（回测账户）
- **菜单 key**：`account`
- **菜单图标**：`mdi-account-cog-outline`
- **URL 路由**：`/meta/finvquant/account`
- **对应视图组件**：`Web/src/views/AccountManageView.vue`

## 2. 业务功能概述

回测账户 CRUD（账户 = 初始资金 + 交易成本 + 保证金模式基线）：

| 功能 | 说明 |
|------|------|
| 查询展示 | 分页展示账户（编码/名称/初始资金/币种/手续费率/滑点/保证金模式/回测开关） |
| 新增账户 | 填写编码/名称/初始资金（>0）/币种/手续费率/滑点/保证金模式（FULL 全额 / FUTURES 期货预留） |
| 编辑账户 | 修改字段（多用户 user_id、多子账户 group_id、默认环境 env_id 可维护） |
| 回测开关 | 行内切换 allow_backtest |
| 删除 | 已关联回测任务的账户禁止删除 |

## 3. 使用方法

```
输入关键字 → 「查询」→ 列表浏览
→ 「新增账户」→ 填写表单 → 「保存」
→ 行内「编辑」→ 行内开关 → 「删除」（无任务关联时）
```

## 4. 处理逻辑

- **保存**：`POST /API/V1/Meta/Finv/Quant/Backtest/Account/Save`；
  服务端校验：account_code 唯一、初始资金 > 0、手续费/滑点 ≥ 0、margin_mode ∈ {FULL, FUTURES}；
- **多用户/多子账户**：`user_id`（默认 default）隔离，`group_id` 分组（主账户 = 分组根，子账户通过 group_id 关联）；
- **回测开关**：账户 allow_backtest='0' 时，回测任务创建被拒绝；
- 保证金模式 FUTURES 为期货杠杆回测预留（当前引擎按 FULL 全额撮合，margin_rate 字段已支持）。

## 5. 注意事项

- 初始资金覆盖可在黄金期货回测验证页按任务级覆盖（不修改账户本身）；
- 账户删除前请确认无历史回测任务关联（否则提示改禁用）。

## 6. 使用的后端接口索引

| 接口 | 接口文档 |
|------|----------|
| `GET /API/V1/Meta/Finv/Quant/Backtest/Account/List` | [AccountList.md](../../API/Backtest/AccountList.md) |
| `POST /API/V1/Meta/Finv/Quant/Backtest/Account/Save` | [AccountSave.md](../../API/Backtest/AccountSave.md) |
| `POST /API/V1/Meta/Finv/Quant/Backtest/Account/Toggle` | [AccountToggle.md](../../API/Backtest/AccountToggle.md) |
| `POST /API/V1/Meta/Finv/Quant/Backtest/Account/Delete` | [AccountDelete.md](../../API/Backtest/AccountDelete.md) |
