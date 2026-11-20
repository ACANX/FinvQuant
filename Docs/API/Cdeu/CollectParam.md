# GET /API/V1/Meta/Finv/Quote/Secu/Collect/{Page,Update} — 采集参数查询与单行维护

> 所属：行情 → 收剔导传ETL → CDEU自动化 → 采集参数
> 数据表：`futu.finv_futu_quote_collect`（主键 `usc`；`ftmm_symbol` 为生成列）
> 左关联：`public.finv_security`（证券元数据）、`public.finv_exchange` / `public.finv_market`（交易所 / 市场显示名）
> 关联需求：11254
> 实现：`internal/api/handler/secu_collect.go`、`internal/meta/finv/quote/cdeu/secu_ftmm_collect_store.go`
> 机器可读契约：[`CollectParam.yml`](CollectParam.yml)（OpenAPI 3.1）

## 1. 概述

本组两支接口服务「采集参数」页面：采集表只有富途侧编码（`market_type` / `market_code` /
`instrument_type` / `sub_instrument_type` …），**没有**证券名称、分类、子类、计价货币、交易所、市场，
所以查询一律左关联规范证券主数据把这些信息带出来。

| 接口 | 作用 |
|---|---|
| `GET .../Secu/Collect/Page` | 分页查询（含左关联的证券元数据），支持关键字 / 市场 / 品种 / 分类 / 采集状态 / 主数据缺失筛选 |
| `POST .../Secu/Collect/Update` | 单行更新（11 个可编辑字段的白名单），只写请求里出现的字段 |

两条硬口径：

1. **不新增、不删除行**：这张表是采集链路的镜像，删行等于停采；新增行走「待采证券录入 / 证券参数补全 / 指标同步」。
2. **不可改的列**：`usc`（主键，换它等于换证券）、`ftmm_symbol`（PG 生成列，显式赋值会报错）、
   `gmt_create` / `gmt_update`（触发器维护）。

## 2. 分页查询 `GET .../Secu/Collect/Page`

### 2.1 请求参数

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|---|
| `Authorization` | header | string | 是 | — | `Bearer <token>` |
| `keyword` | query | string | 否 | 空 | 模糊匹配 `c.usc` / `c.ftmm_symbol` / `c.futu_symbol` / 证券名称（`security_name_cn`、`security_name`） |
| `marketLabel` | query | string | 否 | 空 | 行情市场（`HK`/`SH`/`SZ`/`BJ`/`US`/`FX`/`CC`）；空或 `all` 不限，大小写不敏感 |
| `instrument` | query | string | 否 | 空 | 富途证券类型（`stock`/`etfs`/`index`/`currency`/`crypto`/`futures`/`sectors`）；空或 `all` 不限 |
| `securityType` | query | string | 否 | 空 | 规范证券分类（`public.finv_security.security_type`，如 `Stock`/`ETF`/`StockIndex`/`Futures`） |
| `flagEnable` | query | string | 否 | 空 | 采集状态：`1` 启用 / `0` 禁用；空或 `all` 不限 |
| `onlyMissing` | query | string | 否 | 空 | `1`/`true` 时只列主数据里没有该证券的行（`s.usc IS NULL`） |
| `orderBy` | query | string | 否 | `usc` | 白名单：`usc`/`ftmm_symbol`/`futu_symbol`/`market_label`/`market_type`/`market_code`/`instrument`/`stock_id`/`flag_enable`/`gmt_update` |
| `order` | query | string | 否 | `asc` | `asc` / `desc`（默认升序，按证券编码看更顺） |
| `page` | query | integer | 否 | `1` | 页码 |
| `pageSize` | query | integer | 否 | `50` | 每页条数，1–500 |

### 2.2 响应

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 26428,
    "page": 1,
    "pageSize": 1,
    "list": [
      {
        "usc": "00700",
        "ftmm_symbol": "HK.00700",
        "futu_symbol": "00700",
        "market_label": "HK",
        "market_type": 1,
        "market_code": 1,
        "instrument": "stock",
        "instrument_type": 3,
        "sub_instrument_type": 3002,
        "req_section": "",
        "stock_id": 5,
        "flag_enable": "1",
        "remark": "",
        "gmt_update": "2026-11-13 21:40:12",
        "security_name": "腾讯控股",
        "security_type": "Stock",
        "security_sub_type": "Stock.HK",
        "currency_type": "HKD",
        "exchange_code": 3,
        "exchange_name": "港交所",
        "security_market_code": 12,
        "security_market_name": "香港主板",
        "security_exists": true
      }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `data.total` | integer | 满足筛选条件的总行数 |
| `data.list[].usc` | string | 证券统一编码（主键） |
| `data.list[].ftmm_symbol` | string | 派生键 = `market_label \|\| '.' \|\| futu_symbol`（PG 生成列） |
| `data.list[].futu_symbol` | string | 富途侧证券代码（原始形态） |
| `data.list[].market_label` | string | 富途行情市场标识（HK/SH/SZ/BJ/US/FX/CC） |
| `data.list[].market_type` | integer | 富途市场类型（1 港股 / 2 美股 / 4 A股 / 11 外汇 / 17 加密） |
| `data.list[].market_code` | integer | 富途市场代码（**不是** `finv_market.market_code`） |
| `data.list[].instrument` | string | 富途证券类型（stock/etfs/index/currency/crypto/futures/sectors） |
| `data.list[].instrument_type` | integer | 富途品种类型（3 股票 / 4 ETF / 6 指数 / 7 板块 / 9 期货 / 10 外汇 / 11 加密） |
| `data.list[].sub_instrument_type` | integer | 富途品种子类型（如 3002 / 4002 / 6001） |
| `data.list[].req_section` | string | 富途行情订阅段（通常为空，空串表示无） |
| `data.list[].stock_id` | integer | 富途证券内部 ID |
| `data.list[].flag_enable` | string | 采集开关：`1` 启用 / `0` 禁用 |
| `data.list[].remark` | string | 备注（空串表示无） |
| `data.list[].gmt_update` | string | 最后更新时间（`Asia/Shanghai`，`YYYY-MM-DD HH24:MI:SS`） |
| `data.list[].security_name` | string | 证券名称（主数据 `security_name_cn`，缺失回退 `security_name`） |
| `data.list[].security_type` | string | 规范证券分类（`Stock` / `ETF` / `StockIndex` / `Futures` …） |
| `data.list[].security_sub_type` | string | 证券子类（`type_secu_sub`，如 `Stock.HK`） |
| `data.list[].currency_type` | string | 计价货币（如 `HKD` / `CNY` / `USD`） |
| `data.list[].exchange_code` | integer | 交易所代码（`public.finv_exchange`） |
| `data.list[].exchange_name` | string | 交易所中文简称（`exchange_abbr_cn`） |
| `data.list[].security_market_code` | integer | 规范市场代码（`public.finv_market.market_code`） |
| `data.list[].security_market_name` | string | 规范市场名称（`market_name`） |
| `data.list[].security_exists` | boolean | 主数据里是否有该证券；`false` 时上列元数据字段全为空串 |

## 3. 单行更新 `POST .../Secu/Collect/Update`

### 3.1 请求体

```json
{
  "usc": "00700",
  "fields": {
    "futu_symbol": "00700",
    "market_label": "HK",
    "market_code": 1,
    "flag_enable": "1",
    "remark": "人工修正：补富途代码"
  }
}
```

- `usc`：定位键（必填），不参与更新。
- `fields`：只放**要改的字段**，未出现的列不改（不覆盖）。
- 可编辑 11 列：`stock_id`、`market_type`、`market_code`、`instrument_type`、`sub_instrument_type`、
  `market_label`、`instrument`、`futu_symbol`、`req_section`、`flag_enable`、`remark`。
- 出现未定义字段 → 400（`DisallowUnknownFields`）；`fields` 为空 → 400。

字段校验：

| 字段 | 规则 |
|---|---|
| `stock_id` | 正整数 |
| `market_type` | 正整数 |
| `market_code` | 正整数 |
| `instrument_type` | 正整数 |
| `sub_instrument_type` | 正整数 |
| `market_label` | 非空，服务端统一转大写 |
| `instrument` | 非空，服务端统一转小写 |
| `futu_symbol` | 非空，首尾空白裁掉 |
| `flag_enable` | 只能是 `'0'` 或 `'1'` |
| `req_section` | 可空（服务端裁首尾空白） |
| `remark` | 可空（服务端裁首尾空白） |

### 3.2 响应

```json
{ "code": 0, "message": "保存完成", "data": { "usc": "00700", "updated": true } }
```

### 3.3 错误

| 场景 | HTTP | `code` | 说明 |
|---|---|---|---|
| 请求体非法 / 未定义字段 / `fields` 为空 / 字段校验失败 | 400 | 400 | 消息点名具体字段 |
| `usc` 在采集表里不存在 | 404 | 404 | 不静默插入 |
| `(market_code, futu_symbol)` 唯一约束冲突 | 409 | 409 | 该「富途市场代码 + 代码」已被别的 `usc` 占用，不覆盖别人的行 |
| 其它数据库错误 | 500 | 2006 | — |

## 4. 已使用位置

`Web/src/views/Meta/Finv/Quote/Cdeu/CollectParamView.vue`（列表 + 筛选 + 说明卡片）与
`CollectParamDialog.vue`（单行编辑：只提交改动过的字段，409 冲突保持对话框并提示）。
