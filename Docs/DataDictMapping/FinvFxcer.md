# FinvFxcer — 缺省汇率表 v2

> 所属：FinvQuant 数据字典映射 · 存放：`Docs/DataDictMapping/FinvFxcer.md`
> 数据表：`public.finv_fxcer`（表结构：[`Deploy/Migrations/V300124__public_create_finv_fxcer.sql`](../../Deploy/Migrations/V300124__public_create_finv_fxcer.sql)）
> 需求：11002（建表与字段口径）；10999／11000／11001（接口、双模式、双来源）；11020（取消 CNY 派生）；11021（清理 v1 遗留）
> 用途：多货币环境下把证券市值、成交额等金额统一到同一计价货币时的**缺省汇率**来源；只保最新、手动触发更新。

## 1. 表结构（13 列，主键 `usc`）

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `usc` | TEXT | PK, `^[A-Z]{6}$` | 上游标识：`get-fx-rate-list` 的 `stockCode`、`get-fx-overview` 的 map key（如 `USDCNH`） |
| `name` | TEXT | NOT NULL DEFAULT `''` | 展示名称（简体中文）：列表接口原样用其 `name`（`美元/日元`），概览按币种中文名合成（`美元/离岸人民币`） |
| `currency_src` | TEXT | `^[A-Z]{3}$` | 源币种（ISO 4217 alpha-3；人民币 `CNY`、离岸人民币 `CNH`） |
| `currency_dest` | TEXT | `^[A-Z]{3}$` | 目标币种 |
| `rate` | NUMERIC(24,10) | `> 0` | **缺省汇率：1 单位 `currency_src` = N `currency_dest`** |
| `ts_quote` | BIGINT | `> 0` | 报价时间（Unix 毫秒）：`overview` 用上游逐对 `time`；`rateList` 接口不返回时间，用采集时刻 |
| `source` | TEXT | NOT NULL | `moomoo.get-fx-overview` / `moomoo.get-fx-rate-list` / `derive:CNH` |
| `flag_derive` | CHAR(1) | NOT NULL DEFAULT `'0'`, `IN ('0','1')` | `0`=直采、`1`=派生；**需求 11020 起采集只写 `'0'`**（不再派生 CNY），字段保留以兼容历史数据 |
| `change_price` | NUMERIC(24,10) | 可空 | 涨跌额（带符号）：`get-fx-rate-list` 的 `changeAmount`；概览来源不提供，写入时保持原值（`COALESCE`） |
| `change_ratio` | NUMERIC(12,6) | 可空 | 涨跌幅（百分数数值，`-0.35` 表示 `-0.35%`）：`changePercent` 去掉 `%` 后落库；概览来源不提供 |
| `remark` | TEXT | — | 口径说明（可空） |
| `gmt_create` | TIMESTAMPTZ | NOT NULL DEFAULT `now()` | 创建时间 |
| `gmt_update` | TIMESTAMPTZ | NOT NULL DEFAULT `now()` | 更新时间（`public.vq_set_gmt_update()` 触发器维护，可作批次时间） |

其他约束：`CHECK currency_src <> currency_dest`；无物理外键（与仓库既有约定一致）。

涨跌两列（需求 11016，`V300125` 追加）：只有表格模式来源 `get-fx-rate-list` 提供，矩阵模式来源
`get-fx-overview` 只给价格与时间，因此两列可空；需求 11020 起不再派生 `CNY` 行，两列只来自直采行。

## 2. 数据来源与行数

| 来源（`source`） | 接口 | 行数（实测） | `ts_quote` |
|---|---|---|---|
| `moomoo.get-fx-overview` | `get-fx-overview`（9 币种两两组合 72 对） | 72（全部直采；需求 11020 起不再派生 CNY） | 上游逐对 `time` |
| `moomoo.get-fx-rate-list` | `get-fx-rate-list`（`listId=1`，币种 36 个、280 个货币对） | 280（跳过 `USDindex`/`EURindex` 等 2 条非货币对） | 采集时刻 |
| ~~`derive:CNH`~~ | 由 `CNH` 对同价换标 | **0（已停用）** | 需求 11020 起不再产生；库里若残留旧行属历史数据 |

两个来源互补：概览覆盖 9 个主要币种的完整矩阵，列表补齐长尾币种。`XAU`/`XAG`（黄金/白银）虽非货币但出现在 `listId=1` 中，**按需求保留**、不过滤。

矩阵展示顺序（前端，需求 11003/11004）：`USD`、`CNY`、`CNH`、`HKD`、`EUR`、`JPY`、`SGD`、`GBP`、`AUD`、`CAD`、`NZD`、`CHF`、`PHP`、`IDR`、`ZAR`、`RUB`、`THB`、`TWD`、`XAU`、`XAG`，其余按字母序。

## 3. 写入语义

```sql
INSERT INTO public.finv_fxcer (usc, name, currency_src, currency_dest, rate, change_price,
                               change_ratio, ts_quote, source, flag_derive, remark)
VALUES (...)
ON CONFLICT (usc) DO UPDATE
   SET name = EXCLUDED.name, currency_src = EXCLUDED.currency_src,
       currency_dest = EXCLUDED.currency_dest, rate = EXCLUDED.rate,
       change_price = COALESCE(EXCLUDED.change_price, public.finv_fxcer.change_price),
       change_ratio = COALESCE(EXCLUDED.change_ratio, public.finv_fxcer.change_ratio),
       ts_quote = EXCLUDED.ts_quote, source = EXCLUDED.source,
       flag_derive = EXCLUDED.flag_derive, remark = EXCLUDED.remark
 WHERE (EXCLUDED.ts_quote >= public.finv_fxcer.ts_quote
        OR EXCLUDED.source <> public.finv_fxcer.source)
   AND (EXCLUDED.flag_derive = '0' OR public.finv_fxcer.flag_derive = '1');
```

- 同一来源内：旧 `ts_quote` 不覆盖新报价；
- 跨来源：以最后一次采集为准（`overview` 用上游时间、`rateList` 用采集时刻，两时钟不可比；只比时间会让先跑的 `rateList` 永久压住 `overview`）；
- 不再派生 `CNY` 行（需求 11020）：`CNY` 相关货币对只由 `rateList` 直采，没采到就是没数据，不伪造；
- 涨跌两列用 `COALESCE`：`overview` 轮次传空 → 保留此前的涨跌值，不会被矩阵模式采集清空；
- 不做 `DELETE`：未刷新到的行保留旧值，用 `ts_quote` / `gmt_update` 判断过期。

## 4. 换算口径（消费端）

查询 `A → B`：① 直采行 `(A,B)`；② 反向行 `(B,A)` 取倒数；③ 经 `USD` 交叉；④ 都不命中时按 `public.finv_currency.exchange_rate_cny` 兜底并告警（该字典为静态种子）。

`CNY` 无需特判：`CNY` 行与其它币种同为直采行，按 ISO 码直接查；若库里还没有 `CNY` 行，说明尚未用表格模式采集，需先补采（不再有派生兜底）。

## 5. 说明

- 迁移不写种子数据：未触发采集前表为空；
- `usc` 只能由上游标识或「源+目标」生成，**不允许人工编造**；
- 币种中文名优先取 `name`，`GET .../Fx/Cer` 会按行内 `name`（`美元/日元`）反推补齐 `currency_names`；仍未命中的由前端回落为代码。
