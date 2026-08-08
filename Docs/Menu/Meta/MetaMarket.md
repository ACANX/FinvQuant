# 交易所下设市场信息维护（MetaMarket）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/Meta/MetaMarket.md`
> 对应视图：`Web/src/views/MetaMarketView.vue`
> 接口契约：见 [Docs/API/Meta/MetaMarket.md](../../API/Meta/MetaMarket.md)

## 1. 菜单入口

- **菜单位置**：左侧导航栏 → 元数据管理 → 业务元数据维护 → 交易所下设市场信息维护
- **菜单名称**：交易所下设市场信息维护
- **菜单 key**：`meta-market`
- **菜单图标**：`mdi-chart-areaspline`
- **URL 路由**：`/meta/market`
- **对应视图组件**：`Web/src/views/MetaMarketView.vue`

## 2. 业务功能概述

交易所下设市场信息维护菜单提供 **finv_market 交易市场字典**的维护能力：

| 功能 | 说明 |
|------|------|
| 查询展示 | 分页展示交易市场（市场代码/标识/简码/名称/证券类型/货币/启用状态） |
| 关键字搜索 | 按市场代码/标识/名称/证券类型模糊过滤 |
| 新增 | 新增市场记录（代码必须为正整数，标识必填） |
| 修改 | 修改现有市场的非主键字段 |
| 禁用/启用 | 切换 `flag_enable`（'1'=启用，'0'=禁用） |

## 3. 操作流程（用户视角）

```
输入关键字 → 点击「查询」→ 查看市场列表（分页）
    → 「新增」打开对话框填写 → 「保存」
    → 行内「修改」调整字段 → 行内「禁用/启用」切换状态
```

## 4. 前端处理逻辑

### 4.1 查询参数

- **请求**：`GET /API/V1/Meta/Finv/Quant/Metadata/Market/List`，Query 参数如下：

| 参数 | 说明 |
|------|------|
| `keyword` | 关键字（匹配代码/标识/简码/名称/证券类型） |
| `flag_enable` | 按启用状态过滤（`0`/`1`，可选） |
| `page` / `page_size` | 分页（默认 1 / 20） |

### 4.2 新增与修改

- **请求**：`POST /API/V1/Meta/Finv/Quant/Metadata/Market/Save`，JSON body 字段见接口文档。
- 新增：`market_code` 不存在时 INSERT，`flag_enable` 默认 `'1'`。
- 修改：`market_code` 已存在时 UPDATE（主键不可改）。

### 4.3 禁用/启用

- **请求**：`POST /API/V1/Meta/Finv/Quant/Metadata/Market/Toggle`，body：`{ "market_code": 1110, "flag_enable": "0" }`。

### 4.4 响应处理

- 请求异常（网络失败）→ 提示 `网络错误：无法连接服务端`。
- 业务码非 0 → 展示 `message` 为错误提示。

## 5. 注意事项

- `market_code` 为主键且**不可修改**；`market_flag` 全局唯一（数据库约束）。
- `en_security_type` 为富途侧证券类型编码（如 1110 / 1210 / 1310），非展示名。
- 字典数据来源为 `finv_market` 表（V3 表结构 + V100018 全量种子 55 条）。
