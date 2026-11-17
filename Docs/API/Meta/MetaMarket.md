# GET/POST /API/V1/Meta/FinvQuant/Metadata/Market/* — 交易所下设市场信息维护

finv_market 交易市场字典维护接口：分页查询、新增/修改、禁用/启用。

> 排序约定：`List` 分页结果**启用的（flag_enable='1'）优先展示**，禁用的排后面，同状态按 market_code 升序。

## 1. 分页查询市场

- **方法**：`GET`
- **路径**：`/API/V1/Meta/FinvQuant/Metadata/Market/List`

### Query 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `keyword` | string | 可选 | 关键字，匹配 market_code / market_flag / market_abbr / market_name / en_security_type（任一包含即命中） |
| `flag_enable` | string | 可选 | 按启用状态过滤：`0`（禁用）/ `1`（启用） |
| `page` | int | 可选 | 页码（从 1 开始，默认 1） |
| `page_size` | int | 可选 | 每页条数（默认 20，上限 500） |

### 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 55,
    "list": [
      {
        "market_code": 1110,
        "market_flag": "SSE-A",
        "market_abbr": "SSE-A",
        "market_name": "上交所 A 股",
        "en_security_type": "1110",
        "exchange_code": 0,
        "base_currency": "",
        "flag_enable": "1"
      }
    ]
  }
}
```

## 2. 新增/修改市场

- **方法**：`POST`
- **路径**：`/API/V1/Meta/FinvQuant/Metadata/Market/Save`

### 请求体（JSON）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `market_code` | int | ✅ | 市场代码（正整数）；**已存在则 UPDATE，不存在则 INSERT** |
| `market_flag` | string | ✅ | 市场标识（如 SSE-A / HKEX-H / US-I），全局唯一 |
| `market_abbr` | string | ✅ | 交易所简码（如 SSE / HKEX / US） |
| `market_name` | string | ✅ | 市场名称（如 上交所 A 股） |
| `en_security_type` | string | ✅ | 允许证券类型编码（如 1110 / 1210 / 1310） |
| `exchange_code` | int | 可选 | 所属交易所代码（对齐 finv_exchange，缺省 0；V20 新增） |
| `base_currency` | string | ✅ | 基础计价货币（如 CNY / USD / HKD，可为空串） |

### 响应（成功 HTTP 200）

```json
{ "code": 0, "message": "保存成功", "data": { "market_code": 1110 } }
```

## 3. 禁用/启用市场

- **方法**：`POST`
- **路径**：`/API/V1/Meta/FinvQuant/Metadata/Market/Toggle`

### 请求体（JSON）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `market_code` | int | ✅ | 市场代码 |
| `flag_enable` | string | ✅ | `0`（禁用）/ `1`（启用） |

### 响应（成功 HTTP 200）

```json
{ "code": 0, "message": "切换成功", "data": { "market_code": 1110, "flag_enable": "0" } }
```

## 失败场景

| 场景 | HTTP | code |
|------|------|------|
| 请求体格式错误 / 参数非法 | 400 | 4001 |
| 数据库执行失败（含唯一约束冲突） | 500 | 2006 |
| 切换不存在的市场 | 500 | 2006 |

## 已使用位置登记

- 交易所下设市场信息维护菜单（`Docs/Menu/Meta/MetaMarket.md`，视图 `MetaMarketView.vue`）
