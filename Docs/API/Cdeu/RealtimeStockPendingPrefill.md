# 待录入证券补录预填（RealtimeStockPendingPrefill）

> 接口：`GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Prefill`
> 需求：11147
> 权限：需登录（内部接口）
> 关联菜单：[`Menu/Quote/Realtime/PendingSecu.md`](../../../Menu/Quote/Realtime/PendingSecu.md)（对话框「录入证券元数据」）
> 关联接口：[`Cdeu/RealtimeStockPendingEnroll.md`](RealtimeStockPendingEnroll.md)（提交录入）

## 1. 用途

给「录入证券元数据」对话框一次算齐所有可自动填的字段：**能关联到的一律自动填，并标注来源**，
人工只做核对。原先这套字段全靠手填，一条证券要填十几个值。

两张目标表：

| 表 | 说明 |
|----|------|
| `public.finv_security` | 证券主数据 |
| `futu.finv_futu_quote_collect` | futu 采集配置（`ftmm_symbol` 是生成列，不填） |

> `cdeu.finv_quote_cdeu_usc` **不在补录范围**（需求 11147）：该表数据由运行时采集链路动态生成，人工录入没有意义。

## 2. 请求

| 参数 | 位置 | 类型 | 必填 | 说明 |
|------|------|------|------|------|
| `stockId` | query | integer | 是 | 快照表 `cdeu.finv_cdeu_secu_realtime_stock.stock_id` |

`stockId` 不存在返回 400（`stock_id=<n> 不存在`）。

## 3. 响应

```json
{
  "code": 0,
  "message": "预填完成",
  "data": {
    "stock_id": 79083232831247,
    "usc": "HK.09999",
    "name": "NTES-S",
    "security": [
      { "key": "usc", "label": "usc（主键，派生）", "value": "HK.09999", "source": "dictionary", "required": true, "note": "..." },
      { "key": "exchange_code", "label": "exchange_code 交易所代码", "value": "21", "source": "dictionary", "required": true, "note": "..." }
    ],
    "collect": [
      { "key": "market_code", "label": "market_code 市场细码", "value": "1", "source": "reference", "required": true, "note": "..." }
    ],
    "warnings": [
      "public.finv_security 里 交易所 21 + 代码 09999 已存在（usc=09999），直接录入会撞唯一约束，需先人工处理旧行"
    ]
  }
}
```

字段说明：`security[]` / `collect[]` 每项为 `{key, label, value, source, required, note}`。

### 3.1 `source` 取值

| source | 含义 | 来源 |
|--------|------|------|
| `snapshot` | 快照行直接带过来 | 代码 / 名称 / 市场 / 品种类型 / 币种 |
| `dictionary` | 查字典表得到 | `public.finv_exchange`（`exchange_code`、`base_currency`）、品种类型映射 |
| `reference` | 同市场同品种已配置行的众数 | `futu.finv_futu_quote_collect`（`market_code` / `sub_instrument_type` / `instrument`） |
| `default` | 表默认值或代码缺省，仅供占位 | `security_name_cn` 缺省取英文名、`flag_enable='0'` |
| `manual` | 推不出来，必须人工确认 | 交易所标签未登记、无参考行等 |

### 3.2 派生规则

| 目标表 | 字段 | 规则 |
|--------|------|------|
| 两表 | `usc` | `upper(市场标签.代码)`，服务端派生（请求体里的值一律不采信） |
| `finv_security` | `exchange_code` | `public.finv_exchange.exchange_flag = 市场标签`（SH=11 / SZ=12 / BJ=13 / HK=21 / US=30） |
| `finv_security` | `currency_type` | 快照 `currency` 优先，缺失用 `finv_exchange.base_currency` |
| `finv_security` | `security_type` | `instrument_type` 映射：3/12→Stock、4→ETF、6→StockIndex、11→Crypto |
| `finv_security` | `security_name_cn` | 快照无中文名，先用英文名占位（标记 `default`，需人工核对） |
| `quote_collect` | `market_type` | 快照 `market_type` 优先，缺失按市场推：HK=1 / US=2 / A股=4 |
| `quote_collect` | `market_code` | 同市场同品种已配置行的众数（标记 `reference`，必须核对） |
| `quote_collect` | `sub_instrument_type` | `instrument_type * 1000 + 2`（3→3002、4→4002） |
| `quote_collect` | `instrument` | 3→stock、4→etf、6→index、7→sectors、11→crypto |
| `quote_collect` | `flag_enable` | 默认 `'0'`（停用），核对后再开采集 |

### 3.3 `warnings`

预填命中以下情况时给出（界面直接展示，避免提交后才发现）：

- 市场标签在 `public.finv_exchange` 里没有对应交易所（`exchange_code` 需人工填）；
- 快照无币种且字典无基准币种；
- 同市场同品种没有可参照的采集配置行（`market_code` 需人工填）；
- `public.finv_security` 的 `(exchange_code, security_code)` 已被**另一个 usc** 占用；
- `futu.finv_futu_quote_collect` 的 `(market_code, futu_symbol)` 已被另一个 usc 占用。

> 第 4、5 条是老库历史口径造成的真实障碍：旧数据的 `usc` 是裸代码（如 `09999`），
> 新口径是 `HK.09999`，撞唯一约束。**不得自动改旧行主键**，必须人工裁决。

## 4. 错误码

| HTTP | code | 场景 |
|------|------|------|
| 200 | 0 | 预填成功 |
| 400 | 400 | `stockId` 非法 / 快照行不存在 |
| 500 | 2006 | 数据库侧失败 |

## 5. 相关文档

- [`Cdeu/RealtimeStockPendingEnroll.md`](RealtimeStockPendingEnroll.md) — 提交录入
- [`Cdeu/RealtimeStockPendingPage.md`](RealtimeStockPendingPage.md) — 待录入清单
- [`Menu/Quote/Realtime/PendingSecu.md`](../../../Menu/Quote/Realtime/PendingSecu.md) — 页面说明
