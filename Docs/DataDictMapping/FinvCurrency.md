# FinvCurrency — 货币字典映射

> 所属：FinvQuant 数据字典映射 · 存放：`Docs/DataDictMapping/FinvCurrency.md`
> 数据表：`finv_currency`（表结构：[`Deploy/Migrations/V4__finv_currency.sql`](../../Deploy/Migrations/V4__finv_currency.sql)；初始数据：[`Deploy/Migrations/V100001__finv_currency_seed.sql`](../../Deploy/Migrations/V100001__finv_currency_seed.sql)）
> 用途：货币类型/名称/最新兑换人民币汇率字典，供市场与行情计价换算使用。

## 1. 表结构

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `currency_type` | TEXT | PK | 货币类型（如 `CNY` / `USD` / `HKD`） |
| `currency_name` | TEXT | NOT NULL | 货币名称（如 人民币 / 美元 / 港币） |
| `exchange_rate_cny` | NUMERIC(20,8) | NOT NULL, ≥0 | 最新兑换人民币汇率（1 单位本币 = N 人民币） |
| `gmt_create` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 首次插入时间 |
| `gmt_update` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 最后更新时间（触发器维护） |

> 说明：MySQL 原字段名 `exchange rate_cny`（含空格），PG 侧按 snake_case 规范命名为 `exchange_rate_cny`；汇率精度 `NUMERIC(20,8)` 对齐 MySQL `double(20,8)`。

## 2. 数据清单（7 条）

| currency_type | currency_name | exchange_rate_cny |
|---------------|---------------|-------------------|
| CNY | 人民币 | 1.0 |
| USD | 美元 | 7.269 |
| HKD | 港币 | 0.89 |
| NTD | 新台币 | 0.26 |
| JPY | 日元 | 0.03 |
| SGD | 新加坡币 | 1.1 |
| INR | 印度卢比 | 0.5 |

## 3. 说明

- **汇率口径**：`exchange_rate_cny` 表示 1 单位本币兑换的人民币数量（CNY 自身为 1.0 基准）；后续汇率变动时以 UPDATE 更新，`gmt_update` 自动记录时间。
- **对齐参考**：MySQL `finv_currency` 表结构 + FT 货币清单初始数据。
- **与 finv_exchange 关系**：`finv_exchange.base_currency`（如 CNY / USD / HKD）可关联本表 `currency_type` 获取兑换汇率。
- **审计字段**：`gmt_create` / `gmt_update` 与既有表规范一致，`gmt_update` 由 `vq_set_gmt_update()` 触发器自动维护。
