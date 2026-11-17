# GET/POST /API/V1/Meta/Finv/Quant/Metadata/Security/* — 规范证券信息维护

finv_security 证券代码字典维护接口：分页查询、新增/修改、禁用/启用、下拉选项、按代码查询详情。

> 排序约定：`List` 分页结果**启用的（flag_enable='1'）优先展示**，禁用的排后面，同状态按 usc 升序（交易所/市场 List 同理）。

## 1. 分页查询证券

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Metadata/Security/List`

### Query 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `keyword` | string | 可选 | 关键字，匹配 usc / security_code / security_name / security_name_cn（任一包含即命中） |
| `flag_enable` | string | 可选 | 按启用状态过滤：`0`（禁用）/ `1`（启用） |
| `page` | int | 可选 | 页码（从 1 开始，默认 1） |
| `page_size` | int | 可选 | 每页条数（默认 20，上限 500） |

### 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 518,
    "list": [
      {
        "usc": "GCMain",
        "exchange_code": 33,
        "market_code": 0,
        "security_type": "Futures",
        "security_code": "GCMain",
        "security_name": "GCMain2512",
        "security_name_cn": "黄金期货主连",
        "security_name_full": "黄金期货主连",
        "currency_type": "USD",
        "init_date": 19710101,
        "timezone": "-04:00",
        "tz": "America/New_York",
        "flag_enable": "1"
      }
    ]
  }
}
```

## 2. 新增/修改证券

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quant/Metadata/Security/Save`

### 请求体（JSON）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `usc` | string | ✅ | 统一证券代码（全局唯一）；**已存在则 UPDATE，不存在则 INSERT** |
| `exchange_code` | int | ✅ | 交易所代码（对齐 finv_exchange，正整数） |
| `market_code` | int | 可选 | 交易市场代码（对齐 finv_market，缺省 0；V19 新增；前端下拉数据来自 `Market/List`） |
| `security_type` | string | ✅ | 证券类型（如 Futures / StockIndex / Stock / ETF） |
| `security_code` | string | ✅ | 源证券代码（交易所原始代码） |
| `security_name` | string | ✅ | 源证券名称 |
| `security_name_cn` | string | ✅ | 证券名称（中文） |
| `security_name_full` | string | 可选 | 证券名称（全称，可为 null） |
| `currency_type` | string | ✅ | 计价基础货币（对齐 finv_currency） |
| `init_date` | int | ✅ | 首次上市交易日期 yyyymmdd（默认 20000000） |
| `timezone` | string | 可选 | 时区（如 -04:00 / +08:00，可为 null） |
| `tz` | string | 可选 | 时区标识（如 America/New_York，可为 null） |

### 响应（成功 HTTP 200）

```json
{ "code": 0, "message": "保存成功", "data": { "usc": "NVDA" } }
```

## 3. 禁用/启用证券

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quant/Metadata/Security/Toggle`

### 请求体（JSON）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `usc` | string | ✅ | 统一证券代码 |
| `flag_enable` | string | ✅ | `0`（禁用）/ `1`（启用） |

### 响应（成功 HTTP 200）

```json
{ "code": 0, "message": "切换成功", "data": { "usc": "NVDA", "flag_enable": "0" } }
```

## 4. 证券下拉选项（供历史行情查询证券代码筛选）

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Metadata/Security/Options`
- **说明**：仅返回**启用状态**（flag_enable='1'）的证券；`usc` 为下拉选项 key，`security_name_cn` 为字面展示值。

### 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 518,
    "list": [
      { "usc": "GCMain", "security_name_cn": "黄金期货主连" },
      { "usc": "NVDA", "security_name_cn": "英伟达" }
    ]
  }
}
```

## 5. 按代码查询证券详情（供历史行情导入双策略）

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Metadata/Security/Lookup?code=<代码>`
- **说明**：按统一证券代码（usc）或源证券代码（security_code）精确匹配；命中 usc 优先。供历史行情导入页两种策略使用：① 选中证券后自动带出详情核对；② 按文件头代码自动匹配补全。

### Query 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `code` | string | ✅ | 证券代码（usc 或 security_code 均可）；兼容旧参数名 `usc` |

### 响应（成功 HTTP 200，命中）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "found": true,
    "security": {
      "usc": "NVDA",
      "exchange_code": 30,
      "market_code": 0,
      "security_type": "Stock",
      "security_code": "NVDA",
      "security_name": "NVIDIA",
      "security_name_cn": "英伟达",
      "security_name_full": "英伟达",
      "currency_type": "USD",
      "init_date": 20000000,
      "timezone": "-04:00",
      "tz": "America/New_York",
      "flag_enable": "1"
    }
  }
}
```

### 响应（未命中）

```json
{ "code": 1, "message": "证券不存在", "data": { "found": false } }
```

## 失败场景

| 场景 | HTTP | code |
|------|------|------|
| 请求体格式错误 / 参数非法 | 400 | 4001 |
| 数据库执行失败（含唯一约束冲突） | 500 | 2006 |
| 切换不存在的证券 | 500 | 2006 |

## 已使用位置登记

- 规范证券信息维护菜单（`Docs/Menu/Meta/MetaSecurity.md`，视图 `MetaSecurityView.vue`）
- 历史行情查询菜单（`Docs/Menu/HistoryQuote/HistoryQuoteQuery.md`，视图 `QuoteQueryView.vue`）—— 仅使用 `GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options` 作为证券代码下拉字典来源
- 历史行情数据导入菜单（`Docs/Menu/HistoryQuote/HistoryQuoteImport.md`，视图 `QuoteImportView.vue`）—— 使用 `Options` 下拉 + `Lookup` 自动带出/匹配（导入双策略）
