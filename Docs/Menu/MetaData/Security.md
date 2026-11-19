# 规范证券信息维护（Security）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/MetaData/Security.md`
> 对应视图：`Web/src/views/Meta/Finv/MetaData/SecurityView.vue`
> 接口契约：见 [Docs/API/Meta/MetaSecurity.md](../../API/Meta/MetaSecurity.md)
> 关联需求：11347（新增对话框支持手填 + 缺省值）、11348（上市日期下限放宽）、11349（时区偏移与时区标识分离）

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
| 组合筛选 | 交易所、市场、类型（`security_type`）、子类（`type_secu_sub`）、货币、状态、提供方，均可单选、与关键字叠加；选项来自 `Security/FilterOptions`（实测类型 18 / 子类 31 / 货币 43 / 提供方 1 项）；「重置」清空全部筛选（不影响列配置与列宽）（需求 11330） |
| 新增 | 新增证券记录（usc 必填且全局唯一，交易所代码必须为 1–999999 的整数，可手填下拉之外的值） |
| 修改 | 修改现有证券的非主键字段 |
| 禁用/启用 | 切换 `flag_enable`（'1'=启用，'0'=禁用）；**禁用后不再出现在历史行情查询的证券下拉字典中** |
| 选择列 | 右上角「选择列（n / 20）」打开勾选对话框：20 个可选列任意勾选，保存到 `localStorage`（`finv:meta:security:columns`），可「恢复默认」（同时清空列宽）；**「类型」默认隐藏**（需求 11328） |
| 分页 | 表格底部**唯一**分页控件（表格自带 footer 已关闭、`:items-per-page="-1"` 显示当前页全部行，需求 11331）：左侧「共 N 条，第 x / y 页」，右侧「每页 15（默认）/20/50/100/200 条」+ 页码（7 个可见）+「跳至 _ 页」+ 跳转按钮；每页条数变更回到第 1 页 |
| 列宽调节 | 表头右边界可**拖动**调整该列宽度（存 `localStorage` 的 `finv:meta:security:widths`）；双击边界恢复该列默认宽度（需求 11329） |
| 收藏/取消收藏 | 行内心形按钮（`mdi-heart` 已收藏，红色 / `mdi-heart-outline` 未收藏），点击把 `usc` 写入/移出 `quote.finv_quote_secu_fav`；列表通过 `is_favorite` 回显（需求 11312） |

### 2.1 表格列（需求 11327）

| 列序 | 列 | 字段 | 说明 |
|---|---|---|---|
| 1 | 统一编号 | `usn` | 可空；按 `Docs/Design/UsnCodingSchemeDesign.md` 编制，未生成显示 `—` |
| 2 | usc | `usc` | 统一证券代码（主键） |
| 3 | 交易所 | `exchange_code` | `finv_exchange` 代码 |
| 4 | 市场 | `market_code` | `finv_market` 代码 |
| 5 | 类型 | `security_type` | 证券类型原文（**默认隐藏**，可在「选择列」里勾选显示，需求 11328） |
| 6 | 源代码 | `security_code` | 交易所内代码 |
| 7 | 证券名称 | `security_name` | 英文/原文名称 |
| 8 | 中文名称 | `security_name_cn` | 中文名称 |
| 9 | 分类 | `type_secu` | `type_secu_sub` 的父类（需求 11306），空显示 `—` |
| 10 | 子类 | `type_secu_sub` | 证券子类（需求 11305/11313），空显示 `—` |
| 11 | 货币 | `currency_type` | ISO 4217 |
| 12 | 上市日 | `init_date` | `yyyyMMdd` |
| 13 | 状态 | `flag_enable` | 启用 / 禁用 chips |
| 14 | 操作 | — | 收藏、修改、禁用/启用（固定在最后一列） |

> 列宽与换行（需求 11329）：表格 `table-layout: auto` + 单元格 `white-space: nowrap`，
> 20 列全开时列内容**不会**被压缩成多行，超宽时整体横向滚动；每列宽度默认取 2.1 的设定值，
> 可拖拽微调，配置持久化。
>
> 可选列共 20 个：在上表 14 列之外，还有「全称」（`security_name_full`）、时区（`timezone`）、
> 时区标识（`tz`）、提供方（`provider`）、备注（`remark`）、创建时间（`gmt_create`）、
> 更新时间（`gmt_update`）；默认全部隐藏，勾选后追加到表格（`全称` / `备注` 以 ⓘ 悬浮提示展示，
> 符合 `TableTextSpec`）。

### 2.2 默认排序（需求 11330）

列表按 **交易所 → 市场 → 源代码 升序**（`exchange_code, market_code, security_code`），
同键再按 `usc` 兜底，保证翻页顺序稳定；表格列头不提供本地排序，避免覆盖服务端顺序。

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
- **对话框字段行为（需求 11347）**：

| 字段 | 控件 | 缺省值 | 可手动输入 | 说明 |
|---|---|---|---|---|
| 交易所代码 | 可输入下拉（combobox） | 空 | 是 | 候选来自 `GET /API/V1/Meta/Finv/Quant/Metadata/Exchange/List`（仅启用记录），展示 `code abbr（name）`；**不限于候选**，可直接输入任意整数 |
| 市场代码 | 可输入下拉（combobox） | 空（保存按 `0`） | 是 | 候选来自 `GET /API/V1/Meta/Finv/Quant/Metadata/Market/List`（仅启用记录），展示 `code flag（name）`；可直接输入任意整数 |
| 证券类型 | 可输入下拉（combobox） | `Stock` | 是 | 候选 = 现网 `security_type` distinct 值 + 常见值；缺省取现网最高频值（13485 行） |
| 计价货币 | 可输入下拉（combobox） | 按交易所预填 | 是 | 候选 = 现网 `currency_type` distinct 值 + 常见值；交易所变化时若用户未手改则自动预填 |
| 上市日期 | 数字框 + 日历选择 | `20000000` | 是 | 手输 `yyyymmdd` 8 位数字，或点右侧日历图标选择（`yyyy-mm-dd` 自动转 `yyyymmdd`）；`20000000` 表示未知；**支持 1950 年前上市的老公司**（如 Bank of New York 1784 年、纽交所 1792 年），下限 `10000101` |
| 时区偏移 | 可输入下拉（combobox） | 按交易所预填 | 是 | 候选**只含偏移量**（`±HH:MM`，如 `+08:00`、`-04:00`），不含 IANA 名；交易所变化时若用户未手改则自动预填 |
| 时区标识 | 可输入下拉（combobox） | 按交易所预填 | 是 | 候选为 **IANA 标识**（如 `America/New_York`、`Asia/Shanghai`、`Etc/UTC`）；交易所变化时若用户未手改则自动预填 |

- **按交易所预填「计价货币 + 时区偏移」**（仅新增模式；编辑模式不覆盖已有数据）：

| 交易所代码 | 市场 | 计价货币 | 时区偏移 | 时区标识 |
|---|---|---|---|---|
| 10 | 中国证券市场 | CNY | +08:00 | Asia/Shanghai |
| 11 | 上海证券交易所 | CNY | +08:00 | Asia/Shanghai |
| 12 | 深圳证券交易所 | CNY | +08:00 | Asia/Shanghai |
| 13 | 北京证券交易所 | CNY | +08:00 | Asia/Shanghai |
| 14 | 上海期货交易所 | CNY | +08:00 | Asia/Shanghai |
| 15 | 上海黄金交易所 | CNY | +08:00 | Asia/Shanghai |
| 19 | 中国场外交易 | CNY | +08:00 | Asia/Shanghai |
| 21 | 香港联合证券交易所 | HKD | +08:00 | Asia/Hong_Kong |
| 22 | Hong Kong Futures Exchange | HKD | +08:00 | Asia/Hong_Kong |
| 29 | 台湾证券交易所 | TWD | +08:00 | Asia/Taipei |
| 30 | 美国证券市场 | USD | -04:00 | America/New_York |
| 31 | 纳斯达克证券交易所 | USD | -04:00 | America/New_York |
| 32 | 纽约证券交易所 | USD | -04:00 | America/New_York |
| 33 | 芝加哥商品期货交易所 | USD | -05:00 | America/Chicago |
| 34 | 芝加哥期权交易所 | USD | -05:00 | America/Chicago |
| 35 | 粉红单交易市场 | USD | -04:00 | America/New_York |
| 36 | New York Mercantile Exchange | USD | -05:00 | America/Chicago |
| 37 | Chicago Mercantile Exchange | USD | -05:00 | America/Chicago |
| 38 | Chicago Board of Trade | USD | -05:00 | America/Chicago |
| 40 | Canadian Securities Market | CAD | -05:00 | America/Toronto |
| 51 | 大阪证券交易所 | JPY | +09:00 | Asia/Tokyo |
| 52 | 东京证券交易所 | JPY | +09:00 | Asia/Tokyo |
| 53 | 新加坡证券交易所 | SGD | +08:00 | Asia/Singapore |
| 10001 | 加密货币市场 | USD | +00:00 | Etc/UTC |

> 「时区标识」依据现网 `finv_security.tz` 实测分布：
> `Asia/Shanghai` 12498 / `America/New_York` 10176 / `Asia/Hong_Kong` 257 / `Etc/UTC` 172
> / `Asia/Singapore` 3 / `Asia/Tokyo` 1 / `Europe/London` 1 / `Australia/Sydney` 1。
> 注意现网 HKEX（21）历史数据以 `Asia/Shanghai` 为多（3311 行）、`Asia/Hong_Kong` 仅 256 行，
> 预填按地理正确的 `Asia/Hong_Kong` 取值，历史口径未统一。
>
> 前 4 段（11/12/13、21、30/31/32/35、10001）取自 `public.finv_security` 现网分布实测
> （2026-11-19：`11→CNY+08:00` 4122 行、`12→CNY+08:00` 4706 行、`13→CNY+08:00` 358 行、
> `21→HKD+08:00` 3497 行、`30→USD-04:00` 9197 行、`31→USD-04:00` 885 行、`32→USD-04:00` 61 行、
> `10001→USD+00:00` 172 行）；其余交易所现网无数据或不足 5 行，按各市场常规交易时区取值。

- **保存前校验**（对齐 DB CHECK 约束，前端先拦，避免落成 500）：

| 字段 | 校验 | DB 约束 |
|---|---|---|
| `exchange_code` | 整数 `1–999999` | `finv_security_exchange_code_check` |
| `market_code` | 整数 `≥ 0`，留空按 `0` | 列默认 `0` |
| `init_date` | 整数 `10000101–21001231`（`20000000` 表示未知） | `finv_security_init_date_check`（V300233 已放宽下限） |

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
