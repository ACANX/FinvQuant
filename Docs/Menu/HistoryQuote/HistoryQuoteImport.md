# 历史行情数据导入（HistoryQuoteImport）

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/HistoryQuote/`
> 菜单层级：元数据管理 → 业务元数据维护 → **历史行情数据导入**
> 路由：`/meta/import`（name `meta-import`）· 视图：`Web/src/views/QuoteImportView.vue`
> 关联 API：`POST /API/V1/Meta/Finv/Quant/Quote/Import/Upload`（[ImportsUpload.md](../../API/HistoryQuote/ImportsUpload.md)）、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Lookup`（[MetaSecurity.md](../../API/Meta/MetaSecurity.md)）

## 功能概述

上传 MVSV-1 分钟级历史行情文件，服务端解析后字段级覆盖导入 PostgreSQL（`finv_quote_secu_kline_min`）。

**证券选择双策略**（页面顶部 tabs 切换）：

### 策略 1：先选证券，再选文件核对

1. 在「证券代码」下拉（来源 `Security/Options`，格式 `usc:security_name_cn`，可搜索/手动输入）选择证券；
2. 选中后自动调用 `Security/Lookup` 带出证券详情卡片（usc / 名称 / 源代码 / 类型 / 交易所 / 市场 / 币种 / 启用状态），并自动回填「市场代码」（**优先取字典 `market_code`**，未维护（0）时回退交易所 `exchange_code`）；
3. 选择 MVSV 文件后，前端解析文件头（`# Code` / `# MarketCode`），与所选证券**双向核对**：
   - 文件 Code 必须等于证券的 usc 或 security_code；
   - 文件 MarketCode 与字典 `market_code` 比对（字典未维护时跳过市场代码强校验，仅提示）；
4. 核对全部一致（绿色提示）后，「上传并导入」按钮才可用；不一致则提示具体差异项，不允许上传。

### 策略 2：先选文件，自动匹配证券

1. 先选择 MVSV 文件，前端立即解析文件头并展示证券代码 / 市场代码；
2. 按文件 Code 自动调用 `Security/Lookup` 匹配证券字典（usc 或 security_code），匹配成功后：
   - 展示匹配到的证券信息卡片；
   - 自动补全「证券代码」（usc）与「市场代码」（以文件头 MarketCode 为准），两字段只读；
3. 补全成功后才允许上传导入；未匹配到时提示先在「规范证券信息维护」登记。

### 通用规则

- 两种策略都需勾选「我确认导入将覆盖同时刻同证券的对应字段值」后上传按钮才可用；
- 上传仍提交 `market_code` / `secu_code` 表单字段，服务端 `import.go` 对文件头做一致性校验兜底（不一致返回 4001）；
- 数据源 `source` 必填；覆盖模式 FIELD（推荐）/ ROW 可选；备注可选。

## 关键实现

- 前端文件头解析：读取文件前 4096 字节文本，按 `# Key : Value` 提取 `Code` / `MarketCode`（大小写不敏感），缺失时报错。
- 上传按钮可用性（`canUpload`）：
  - 策略 1：已选证券 + 核对状态 `ok` + 已确认；
  - 策略 2：已匹配证券 + 文件头存在 + 已确认。
- 视图：`Web/src/views/QuoteImportView.vue`（tsc + vite build 通过）。

## 已使用 API 登记

| API | 用途 |
|-----|------|
| `POST /API/V1/Meta/Finv/Quant/Quote/Import/Upload` | 上传 MVSV 文件并导入 |
| `GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options` | 证券代码下拉字典 |
| `GET /API/V1/Meta/Finv/Quant/Metadata/Security/Lookup` | 策略 1 自动带出 / 策略 2 自动匹配 |
