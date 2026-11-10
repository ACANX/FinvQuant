# 证券参数补全：预填（SecuParamCompletePrefill）

> 接口：`GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Secu/Param/Complete/Prefill`
> 需求：11161（复用 11147 的预填与派生规则）
> 权限：需登录（内部接口）
> 关联菜单：[`Menu/Quote/Realtime/SecuParamComplete.md`](../../../Menu/Quote/Realtime/SecuParamComplete.md)
> 关联接口：[`Cdeu/SecuParamCompletePending.md`](SecuParamCompletePending.md)（清单）、
> [`Cdeu/SecuParamComplete.md`](SecuParamComplete.md)（写入）

## 1. 用途

「证券参数补全」对话框的预填接口：把**目标表那一组**字段能关联到的全部算好，并标注每个字段的来源，
人工只需核对。只读，不写库。

两张表的字段推导规则与需求 11147 的「录入」完全一致，差别只有一处：
**只返回 `target` 那一组字段**（另一张表完全不出现，避免「点补全到底写哪张」的歧义）。

## 2. 请求参数

| 参数 | 必填 | 说明 |
|------|:----:|------|
| `target` | 是 | `security` 或 `collect`；其它值返 400 |
| `stockId` | 是 | 快照表主键（`bigint`，实测已到 7.9e13）；只校验正整数 |

## 3. 响应

```json
{
  "code": 0,
  "message": "预填完成",
  "data": {
    "target": "security",
    "table": "public.finv_security",
    "stock_id": 202096,
    "usc": "PLD",
    "name": "安博",
    "exists": false,
    "fields": [
      {
        "key": "usc",
        "label": "usc（主键，派生）",
        "value": "PLD",
        "source": "dictionary",
        "required": true,
        "note": "由 `市场标签.代码` 派生并统一大写，不可改"
      },
      {
        "key": "exchange_code",
        "label": "exchange_code 交易所代码",
        "value": "30",
        "source": "dictionary",
        "required": true,
        "note": "来源：字典表（public.finv_exchange.exchange_flag = US）"
      }
    ],
    "warnings": ["同市场同品种没有可参照的采集配置行，market_code 需人工填写"]
  }
}
```

| 字段 | 说明 |
|------|------|
| `target` | 规范化后的 target |
| `table` | 目标表名 |
| `usc` | 补全将写入的主键 = `COALESCE(快照 usc, upper(市场标签.代码))` |
| `exists` | 目标表里是否已有该 `usc`；`true` 时前端禁用提交（写入是 `DO NOTHING`，点了也白点） |
| `fields[]` | 目标表的字段组；`source` 见下表 |
| `warnings[]` | 风险提示（字典查不到交易所、缺币种、缺名称、无参考行、唯一约束预检冲突等） |

### 3.1 `source` 取值

| 值 | 前端显示 | 含义 |
|----|----------|------|
| `snapshot` | 快照 | 快照行直接带过来 |
| `dictionary` | 字典 | 查 `public.finv_exchange` 等字典表得到 |
| `reference` | 参考 | 同市场同品种已配置行的众数，必须核对 |
| `default` | 缺省 | 表默认值或代码缺省，仅供占位 |
| `manual` | 待填 | 推不出来，必须人工填写 |

### 3.2 派生规则

| 字段 | 规则 |
|------|------|
| `usc` | 快照 `usc` 优先，为空用 `upper(市场标签.代码)`；服务端派生，不采信前端 |
| `exchange_code` | `public.finv_exchange.exchange_flag = 市场标签`（SH=11 / SZ=12 / BJ=13 / HK=21 / US=30） |
| `currency_type` | 快照 `currency` 优先，缺失用 `finv_exchange.base_currency` |
| `security_type` | `instrument_type`：3/12→Stock、4→ETF、6→StockIndex、11→Crypto |
| `market_type` | 快照 `market_type` 优先，缺失按市场推 HK=1 / US=2 / A股=4 |
| `market_code` | 同市场同品种已配置行的众数（参考值） |
| `sub_instrument_type` | `instrument_type * 1000 + 2`（3→3002、4→4002） |
| `instrument` | 3→stock、4→etf、6→index、7→sectors、11→crypto |
| `flag_enable` | 默认 `'0'`（停用），核对后再开采集 |

## 4. 错误码

| HTTP | code | 场景 |
|------|------|------|
| 200 | 0 | 预填完成 |
| 400 | 400 | `target` 缺失或非法、`stockId` 非正整数、`stock_id` 不存在、快照缺市场标签/代码无法确定 usc |
| 500 | 2006 | 数据库侧失败 |

## 5. 相关文档

- [`Cdeu/SecuParamCompletePending.md`](SecuParamCompletePending.md) — 待补清单
- [`Cdeu/SecuParamComplete.md`](SecuParamComplete.md) — 单表写入
- [`Cdeu/RealtimeStockPendingPrefill.md`](RealtimeStockPendingPrefill.md) — 旧的整体预填（两张表一起返回，页面入口已迁出）
- [`Menu/Quote/Realtime/SecuParamComplete.md`](../../../Menu/Quote/Realtime/SecuParamComplete.md) — 页面说明
