# 模板管理（Template）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/Backtest/TemplateManage.md`
> 对应视图：`Web/src/views/Meta/Finv/Quant/Backtest/TemplateView.vue`
> 接口契约：见 [Docs/API/Backtest/TemplateList.md](../../API/Backtest/TemplateList.md)

## 1. 菜单入口

- **菜单位置**：左侧导航栏 → 模板管理
- **菜单名称**：模板管理
- **菜单 key**：`template`
- **菜单图标**：`mdi-content-copy`
- **URL 路由**：`/Meta/Finv/Quant/Template`
- **对应视图组件**：`Web/src/views/Meta/Finv/Quant/Backtest/TemplateView.vue`

## 2. 业务功能概述

模板（策略/账户/环境三类）的独立维护：

| 功能 | 说明 |
|------|------|
| 模板查询 | 按类型过滤（策略/账户/环境模板），内置模板标记 |
| 模板新增/编辑/删除 | JSON 内容编辑；内置模板（is_builtin='1'）禁止删除 |

## 3. 使用方法

```
选择类型过滤 → 「新建模板」/ 行内「编辑」→ 填写编码/名称/类型 + 内容 JSON → 「保存」
```

## 4. 处理逻辑

- `template_type`：STRATEGY（策略定义）/ ACCOUNT（账户配置）/ ENVIRONMENT（环境配置）；
- 内置模板（双均线/RSI 策略模板、COMEX 环境模板）由 V100020 种子数据提供，禁止删除；
- 用户自定义模板可用于快速初始化策略/账户/环境（策略可关联 template_id 记录来源）。

## 5. 注意事项

- 模板内容 JSON 语法错误时前端本地拦截；
- 内置模板（is_builtin='1'）禁止修改与删除（防篡改）。

## 6. 使用的后端接口索引

| 接口 | 接口文档 |
|------|----------|
| `GET /API/V1/Meta/Finv/Quant/Backtest/Template/List` | [TemplateList.md](../../API/Backtest/TemplateList.md) |
| `GET /API/V1/Meta/Finv/Quant/Backtest/Template/Get` | [TemplateGet.md](../../API/Backtest/TemplateGet.md) |
| `POST /API/V1/Meta/Finv/Quant/Backtest/Template/Save` | [TemplateSave.md](../../API/Backtest/TemplateSave.md) |
| `POST /API/V1/Meta/Finv/Quant/Backtest/Template/Delete` | [TemplateDelete.md](../../API/Backtest/TemplateDelete.md) |
