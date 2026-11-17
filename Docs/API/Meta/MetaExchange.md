# GET/POST /API/V1/Meta/FinvQuant/Metadata/Exchange/* — 交易所信息维护

finv_exchange 交易所/市场字典维护接口：分页查询、新增/修改、禁用/启用。

> 排序约定：`List` 分页结果**启用的（flag_enable='1'）优先展示**，禁用的排后面，同状态按 exchange_code 升序。

## 1. 分页查询交易所

- **方法**：`GET`
- **路径**：`/API/V1/Meta/FinvQuant/Metadata/Exchange/List`

### Query 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `keyword` | string | 可选 | 关键字，匹配 exchange_code / exchange_flag / exchange_abbr / exchange_name / exchange_abbr_cn（任一包含即命中） |
| `flag_enable` | string | 可选 | 按启用状态过滤：`0`（禁用）/ `1`（启用） |
| `page` | int | 可选 | 页码（从 1 开始，默认 1） |
| `page_size` | int | 可选 | 每页条数（默认 20，上限 500） |

### 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 44,
    "list": [
      {
        "exchange_code": 11,
        "exchange_flag": "SH",
        "exchange_abbr": "SSE",
        "exchange_name": "上海证券交易所",
        "exchange_abbr_cn": "上交所",
        "en_market_type": "证券",
        "region": "CN",
        "base_currency": "CNY",
        "ft_list_exchange_code": null,
        "flag_enable": "1"
      }
    ]
  }
}
```

## 2. 新增/修改交易所

- **方法**：`POST`
- **路径**：`/API/V1/Meta/FinvQuant/Metadata/Exchange/Save`

### 请求体（JSON）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `exchange_code` | int | ✅ | 交易所代码（正整数）；**已存在则 UPDATE，不存在则 INSERT** |
| `exchange_flag` | string | ✅ | 交易所标志（如 CN / SH / NSDQ），全局唯一 |
| `exchange_abbr` | string | ✅ | 英文缩写（如 SSE / SZSE / HKEX） |
| `exchange_name` | string | ✅ | 英文全称 / 名称 |
| `exchange_abbr_cn` | string | ✅ | 中文名称（如 上交所 / 纳斯达克） |
| `en_market_type` | string | ✅ | 市场类型（证券/期货/外汇等） |
| `region` | string | ✅ | 地区编码（如 CN / HK / USA） |
| `base_currency` | string | ✅ | 基础计价货币（如 CNY / USD） |
| `ft_list_exchange_code` | string | 可选 | FT 行情源列表编码（映射预留，可为 null） |

### 响应（成功 HTTP 200）

```json
{ "code": 0, "message": "保存成功", "data": { "exchange_code": 11 } }
```

## 3. 禁用/启用交易所

- **方法**：`POST`
- **路径**：`/API/V1/Meta/FinvQuant/Metadata/Exchange/Toggle`

### 请求体（JSON）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `exchange_code` | int | ✅ | 交易所代码 |
| `flag_enable` | string | ✅ | `0`（禁用）/ `1`（启用） |

### 响应（成功 HTTP 200）

```json
{ "code": 0, "message": "切换成功", "data": { "exchange_code": 11, "flag_enable": "0" } }
```

## 失败场景

| 场景 | HTTP | code |
|------|------|------|
| 请求体格式错误 / 参数非法 | 400 | 4001 |
| 数据库执行失败（含唯一约束冲突） | 500 | 2006 |
| 切换不存在的交易所 | 500 | 2006 |

## 已使用位置登记

- 交易所信息维护菜单（`Docs/Menu/Meta/MetaExchange.md`，视图 `MetaExchangeView.vue`）
