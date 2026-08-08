# 交易所信息维护（MetaExchange）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/Meta/MetaExchange.md`
> 对应视图：`Web/src/views/MetaExchangeView.vue`
> 接口契约：见 [Docs/API/Meta/MetaExchange.md](../../API/Meta/MetaExchange.md)

## 1. 菜单入口

- **菜单位置**：左侧导航栏 → 元数据管理 → 业务元数据维护 → 交易所信息维护
- **菜单名称**：交易所信息维护
- **菜单 key**：`meta-exchange`
- **菜单图标**：`mdi-office-building`
- **URL 路由**：`/meta/exchange`
- **对应视图组件**：`Web/src/views/MetaExchangeView.vue`

## 2. 业务功能概述

交易所信息维护菜单提供 **finv_exchange 交易所/市场字典**的维护能力：

| 功能 | 说明 |
|------|------|
| 查询展示 | 分页展示交易所字典（代码/标志/缩写/英文全称/中文名称/市场类型/地区/货币/启用状态） |
| 关键字搜索 | 按交易所代码/标志/缩写/名称模糊过滤 |
| 新增 | 新增交易所记录（代码必须为正整数，标志与缩写必填） |
| 修改 | 修改现有交易所的非主键字段 |
| 禁用/启用 | 切换 `flag_enable`（'1'=启用，'0'=禁用），禁用后不再参与业务映射 |

## 3. 操作流程（用户视角）

```
输入关键字 → 点击「查询」→ 查看交易所列表（分页）
    → 「新增」打开对话框填写 → 「保存」
    → 行内「修改」调整字段 → 行内「禁用/启用」切换状态
```

## 4. 前端处理逻辑

### 4.1 查询参数

- **请求**：`GET /API/V1/Meta/Finv/Quant/Metadata/Exchange/List`，Query 参数如下：

| 参数 | 说明 |
|------|------|
| `keyword` | 关键字（匹配代码/标志/缩写/名称） |
| `flag_enable` | 按启用状态过滤（`0`/`1`，可选） |
| `page` / `page_size` | 分页（默认 1 / 20） |

### 4.2 新增与修改

- **请求**：`POST /API/V1/Meta/Finv/Quant/Metadata/Exchange/Save`，JSON body 字段见接口文档。
- 新增：`exchange_code` 不存在时 INSERT，`flag_enable` 默认 `'1'`。
- 修改：`exchange_code` 已存在时 UPDATE（主键不可改）。

### 4.3 禁用/启用

- **请求**：`POST /API/V1/Meta/Finv/Quant/Metadata/Exchange/Toggle`，body：`{ "exchange_code": 11, "flag_enable": "0" }`。
- 成功后行内状态即时刷新并提示。

### 4.4 响应处理

- 请求异常（网络失败）→ 提示 `网络错误：无法连接服务端`。
- 业务码非 0 → 展示 `message` 为错误提示。
- 成功 → 刷新列表并展示 `保存成功` / `已启用/禁用...` 提示。

## 5. 注意事项

- `exchange_code` 为主键且**不可修改**；修改对话框只允许调整非主键字段。
- 交易所标志 `exchange_flag` 全局唯一（数据库约束），重复新增会报错。
- 字典数据来源为 `finv_exchange` 表（V2 表结构 + V100016 全量种子）。
