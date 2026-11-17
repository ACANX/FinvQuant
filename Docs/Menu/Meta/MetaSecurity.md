# 规范证券信息维护（MetaSecurity）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/Meta/MetaSecurity.md`
> 对应视图：`Web/src/views/MetaSecurityView.vue`
> 接口契约：见 [Docs/API/Meta/MetaSecurity.md](../../API/Meta/MetaSecurity.md)

## 1. 菜单入口

- **菜单位置**：左侧导航栏 → 元数据管理 → 业务元数据维护 → 规范证券信息维护
- **菜单名称**：规范证券信息维护
- **菜单 key**：`meta-security`
- **菜单图标**：`mdi-tag-multiple`
- **URL 路由**：`/meta/security`
- **对应视图组件**：`Web/src/views/MetaSecurityView.vue`

## 2. 业务功能概述

规范证券信息维护菜单提供 **finv_security 证券代码字典**的维护能力：

| 功能 | 说明 |
|------|------|
| 查询展示 | 分页展示证券（usc/交易所/类型/源代码/名称/中文名/货币/上市日/启用状态） |
| 关键字搜索 | 按 usc / 证券代码 / 证券名称模糊过滤 |
| 新增 | 新增证券记录（usc 必填且全局唯一，交易所代码必须为正整数） |
| 修改 | 修改现有证券的非主键字段 |
| 禁用/启用 | 切换 `flag_enable`（'1'=启用，'0'=禁用）；**禁用后不再出现在历史行情查询的证券下拉字典中** |

## 3. 操作流程（用户视角）

```
输入关键字 → 点击「查询」→ 查看证券列表（分页）
    → 「新增」打开对话框填写 → 「保存」
    → 行内「修改」调整字段 → 行内「禁用/启用」切换状态
```

## 4. 前端处理逻辑

### 4.1 查询参数

- **请求**：`GET /API/V1/Meta/FinvQuant/Metadata/Security/List`，Query 参数如下：

| 参数 | 说明 |
|------|------|
| `keyword` | 关键字（匹配 usc / 证券代码 / 证券名称） |
| `flag_enable` | 按启用状态过滤（`0`/`1`，可选） |
| `page` / `page_size` | 分页（默认 1 / 20） |

### 4.2 新增与修改

- **请求**：`POST /API/V1/Meta/FinvQuant/Metadata/Security/Save`，JSON body 字段见接口文档。
- 新增：`usc` 不存在时 INSERT，`flag_enable` 默认 `'1'`。
- 修改：`usc` 已存在时 UPDATE（主键不可改）。
- **对话框下拉**：
  - 「交易所代码」为下拉选择，数据源 `GET /API/V1/Meta/FinvQuant/Metadata/Exchange/List`（仅启用记录），展示 `code abbr（name）`；
  - 「市场代码」为下拉选择，数据源 `GET /API/V1/Meta/FinvQuant/Metadata/Market/List`（仅启用记录），展示 `code flag（name）`；两者均保留清空（清空即 0/未选）。

### 4.3 禁用/启用

- **请求**：`POST /API/V1/Meta/FinvQuant/Metadata/Security/Toggle`，body：`{ "usc": "NVDA", "flag_enable": "0" }`。

### 4.4 响应处理

- 请求异常（网络失败）→ 提示 `网络错误：无法连接服务端`。
- 业务码非 0 → 展示 `message` 为错误提示。

## 5. 与其他菜单的联动

- 「历史行情查询」菜单的**证券代码筛选下拉字典**来源于本字典：`GET /API/V1/Meta/FinvQuant/Metadata/Security/Options` 返回启用状态证券的 `usc`（key）与 `security_name_cn`（展示值），按 `usc:security_name_cn` 格式展示，保留手动输入。
- 在本菜单禁用某证券后，该证券将不再出现在历史行情查询的下拉选项中。

## 6. 注意事项

- `usc` 为主键且**不可修改**；同一交易所内 `security_code` 唯一（数据库约束）。
- `exchange_code` 应对齐 `finv_exchange` 字典、`currency_type` 应对齐 `finv_currency` 字典（程序层控制，不建物理外键）。
- 字典数据来源为 `finv_security` 表（V5 表结构 + V100002 15 条 + V100009 增量 503 条）。
