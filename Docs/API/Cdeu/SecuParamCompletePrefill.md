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
| `fields[].value` | 参考值取不到时**返回空串**（不是 `0`）：`0` 会被前端当成「已填」，点写入才被后端拒；空串会让前端立刻标「必填未填」并禁用提交 |

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
| `market_type` | 快照值优先 → 同市场已有配置行众数（来源=参考）→ 内置映射 HK=1 / US=2 / A股=4 / FX=11 / CC=17（来源=字典）（需求 11166） |
| `req_section` | 同市场同品种已有配置行的众数（来源=参考）；众数本身就是空时留空、同样标「参考」；无参考行时留空并标「字典 + 选填」——该列实测 26,529 行只有 2 行非空（需求 11166） |
| `market_code` | 同市场同品种已配置行的众数（参考值）。**匹配前先把 `instrument_type` 归一**：快照里 `12` 与 `3` 都表示股票，采集表只有 `3`，不归一会导致 `12` 的行永远匹配不到参考行（`market_code` 取不到值 → 写入时必填校验失败，需求 11164 故障现场） |
| `sub_instrument_type` | `instrument_type * 1000 + 2`（3→3002、4→4002）；`instrument_type=12` 推缺省值时按 **3** 计算（两者同为股票） |
| `instrument` | 3→stock、4→etf、6→index、7→sectors、11→crypto |
| `flag_enable` | 默认 `'0'`（停用），核对后再开采集 |
| `timezone` | **三层推断**（需求 11165）：① 同 `exchange_code` 已有证券行 `(timezone, tz)` 的众数（来源=参考）；② 同 `region` 已有行的众数（来源=参考）；③ 内置「区域 → IANA 时区」映射、偏移按该时区**当前时刻**推算（来源=字典）。三层都拿不到（`region=N/A` 等）时留空、来源=待填并给风险提示。`timezone` 存 UTC 偏移（`+08:00` / `-04:00`） |
| `tz` | 同上三层推断，存 IANA 标识（`Asia/Shanghai` / `America/New_York`） |

### 3.3 时区推断的内置区域映射（第三层，需求 11165）

| `region` | IANA 时区 | 说明 |
|----------|-----------|------|
| `CN` | `Asia/Shanghai` | A 股（SH/SZ/BJ/CN/SHFE/SGE/CNOTC） |
| `HK` | `Asia/Hong_Kong` | 港股（HK/HKFE） |
| `TW` | `Asia/Taipei` | 台股 |
| `JP` | `Asia/Tokyo` | 日本（OSE/TSE） |
| `KR` | `Asia/Seoul` | 韩国 |
| `SG` | `Asia/Singapore` | 新加坡 |
| `MY` | `Asia/Kuala_Lumpur` | 马来西亚 |
| `IN` | `Asia/Kolkata` | 印度（`+05:30` 半小时偏移） |
| `AU` | `Australia/Sydney` | 澳洲 |
| `CA` | `America/Toronto` | 加拿大 |
| `US` | `America/New_York` | 美股（冬令时 `-05:00` / 夏令时 `-04:00`，按当前时刻推算） |
| `CC` / `FX` | `Etc/UTC` | 加密与外汇 24 小时连续交易 |
| 其它（如 `N/A`） | — | 留空 + 来源=待填 + 风险提示 |

> 偏移用 Go 标准库按 IANA 时区在**当前时刻**的偏移格式化（`-07:00` 形式），
> 二进制内嵌了 `time/tzdata`，不依赖运行环境是否装了 zoneinfo。
> 注意：库里历史数据的美国偏移恒为 `-04:00`（旧录入口径），冬令时用 `-05:00` 才是对的，
> 因此以参考值为准时请核对；人工改也可以。

### 3.4 采集参数的缺省推断（需求 11166）

| 字段 | 推断链 |
|------|--------|
| `futu_symbol` | 取快照 `stock_code` |
| `market_label` | 取快照 `market_label` |
| `market_type` | 快照 `market_type` → 同市场已有配置行众数（参考）→ 内置映射 HK=1/US=2/A股=4/FX=11/CC=17（字典） |
| `market_code` | 同市场同品种归一后众数（参考）；取不到 → **空串** + 必须人工填（需求 11164） |
| `instrument_type` | 快照原值（**不擅自改写**：12 与 3 同为股票，该列被日K同步当富途品种码直接用） |
| `sub_instrument_type` | 同市场同品种众数（参考）→ `归一后 instrument_type * 1000 + 2`（字典） |
| `instrument` | 同市场同品种众数（参考）→ `12/3→stock`、`4→etf`、`6→index`、`7→sectors`、`11→crypto`（字典） |
| `req_section` | 同市场同品种众数（参考）；众数为空 → 留空并说明「选填，多数市场不需要」；无参考行 → 留空 + 选填说明（字典） |
| `flag_enable` | 固定 `'0'`（停用，人工核对后自己开） |

> `req_section` 实测：`futu.finv_futu_quote_collect` 26,529 行里只有 2 行非空
> （`US` + `instrument_type=3` 的 `NVDA` / `SPCX` 填 `1`），所以「按市场推」当下推出来是空；
> 机制留着，等存量出现真实规律会自动带出。该列是选填，空值不影响写入。

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
