# 规范证券信息维护（Security）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/MetaData/Security.md`
> 对应视图：`Web/src/views/Meta/Finv/MetaData/SecurityView.vue`
> 接口契约：见 [Docs/API/Meta/MetaSecurity.md](../../API/Meta/MetaSecurity.md)

## 1. 菜单入口

- **菜单位置**：左侧导航栏 → 元数据管理 → 业务元数据维护 → 规范证券信息维护
- **菜单名称**：规范证券信息维护
- **菜单 key**：`meta-security`
- **菜单图标**：`mdi-tag-multiple`
- **URL 路由**：`/Meta/Finv/MetaData/Security`
- **对应视图组件**：`Web/src/views/Meta/Finv/MetaData/SecurityView.vue`

## 2. 业务功能概述

规范证券信息维护菜单提供 **finv_security 证券代码字典**的维护能力：

| 功能 | 说明 |
|------|------|
| 查询展示 | 分页展示证券（统一编号/usc/交易所/市场/类型/源代码/证券名称/中文名称/分类/子类/货币/上市日/启用状态），列顺序见 2.1 |
| 关键字搜索 | 按 usc / 证券代码 / 证券名称模糊过滤 |
| 新增 | 新增证券记录（usc 必填且全局唯一，交易所代码必须为正整数） |
| 修改 | 修改现有证券的非主键字段 |
| 禁用/启用 | 切换 `flag_enable`（'1'=启用，'0'=禁用）；**禁用后不再出现在历史行情查询的证券下拉字典中** |
| 收藏/取消收藏 | 行内心形按钮（`mdi-heart` 已收藏，红色 / `mdi-heart-outline` 未收藏），点击把 `usc` 写入/移出 `quote.finv_quote_secu_fav`；列表通过 `is_favorite` 回显（需求 11312） |

### 2.1 表格列（需求 11327）

| 列序 | 列 | 字段 | 说明 |
|---|---|---|---|
| 1 | 统一编号 | `usn` | 可空；按 `Docs/Design/UsnCodingSchemeDesign.md` 编制，未生成显示 `—` |
| 2 | usc | `usc` | 统一证券代码（主键） |
| 3 | 交易所 | `exchange_code` | `finv_exchange` 代码 |
| 4 | 市场 | `market_code` | `finv_market` 代码 |
| 5 | 类型 | `security_type` | 证券类型原文 |
| 6 | 源代码 | `security_code` | 交易所内代码 |
| 7 | 证券名称 | `security_name` | 英文/原文名称 |
| 8 | 中文名称 | `security_name_cn` | 中文名称 |
| 9 | 分类 | `type_secu` | `type_secu_sub` 的父类（需求 11306），空显示 `—` |
| 10 | 子类 | `type_secu_sub` | 证券子类（需求 11305/11313），空显示 `—` |
| 11 | 货币 | `currency_type` | ISO 4217 |
| 12 | 上市日 | `init_date` | `yyyyMMdd` |
| 13 | 状态 | `flag_enable` | 启用 / 禁用 chips |
| 14 | 操作 | — | 收藏、修改、禁用/启用 |

## 3. 操作流程（用户视角）

```
输入关键字 → 点击「查询」→ 查看证券列表（分页）
    → 「新增」打开对话框填写 → 「保存」
    → 行内「修改」调整字段 → 行内「禁用/启用」切换状态
    → 行内心形「收藏 / 取消收藏」（写入 quote.finv_quote_secu_fav）
```

## 4. 前端处理逻辑

### 4.1 查询参数

- **请求**：`GET /API/V1/Meta/Finv/Quant/Metadata/Security/List`，Query 参数如下：

| 参数 | 说明 |
|------|------|
| `keyword` | 关键字（匹配 usc / 证券代码 / 证券名称） |
| `flag_enable` | 按启用状态过滤（`0`/`1`，可选） |
| `page` | 页码（默认 `1`） |
| `page_size` | 每页条数（默认 `20`） |

### 4.2 新增与修改

- **请求**：`POST /API/V1/Meta/Finv/Quant/Metadata/Security/Save`，JSON body 字段见接口文档。
- 新增：`usc` 不存在时 INSERT，`flag_enable` 默认 `'1'`。
- 修改：`usc` 已存在时 UPDATE（主键不可改）。
- **对话框下拉**：
  - 「交易所代码」为下拉选择，数据源 `GET /API/V1/Meta/Finv/Quant/Metadata/Exchange/List`（仅启用记录），展示 `code abbr（name）`；
  - 「市场代码」为下拉选择，数据源 `GET /API/V1/Meta/Finv/Quant/Metadata/Market/List`（仅启用记录），展示 `code flag（name）`；两者均保留清空（清空即 0/未选）。

### 4.3 禁用/启用

- **请求**：`POST /API/V1/Meta/Finv/Quant/Metadata/Security/Toggle`，body：`{ "usc": "NVDA", "flag_enable": "0" }`。

### 4.4 响应处理

- 请求异常（网络失败）→ 提示 `网络错误：无法连接服务端`。
- 业务码非 0 → 展示 `message` 为错误提示。

## 5. 与其他菜单的联动

- 「历史行情查询」菜单的**证券代码筛选下拉字典**来源于本字典：`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options` 返回启用状态证券的 `usc`（key）与 `security_name_cn`（展示值），按 `usc:security_name_cn` 格式展示，保留手动输入。
- 在本菜单禁用某证券后，该证券将不再出现在历史行情查询的下拉选项中。

## 6. 注意事项

- `usc` 为主键且**不可修改**；同一交易所内 `security_code` 唯一（数据库约束）。
- `exchange_code` 应对齐 `finv_exchange` 字典、`currency_type` 应对齐 `finv_currency` 字典（程序层控制，不建物理外键）。
- 字典数据来源为 `finv_security` 表（V5 表结构 + V200002 15 条 + V200009 增量 503 条）。
