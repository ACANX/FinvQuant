# FinvMarket — 交易市场字典映射

> 所属：FinvQuant 数据字典映射 · 存放：`Docs/DataDictMapping/FinvMarket.md`
> 数据表：`finv_market`（表结构：[`Deploy/Migrations/V3__finv_market.sql`](../../Deploy/Migrations/V3__finv_market.sql)；初始数据：[`Deploy/Migrations/V100015__finv_market_seed.sql`](../../Deploy/Migrations/V100015__finv_market_seed.sql)；二次定版：[`Deploy/Migrations/V100017__finv_market_seed_full.sql`](../../Deploy/Migrations/V100017__finv_market_seed_full.sql)；三次定版：[`Deploy/Migrations/V100018__finv_market_seed_full.sql`](../../Deploy/Migrations/V100018__finv_market_seed_full.sql)）
> 用途：交易所下属交易市场代码表（如 上交所股票/基金/债券等细分市场），与 [`FinvExchange.md`](FinvExchange.md) 交易所字典互补。

## 1. 表结构

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `market_code` | INTEGER | PK，1~999999 | 市场数字代码 |
| `market_flag` | TEXT | NOT NULL, UNIQUE | 市场标识（如 `SH_A` / `SZ_STAR` 等，编码待定） |
| `market_abbr` | TEXT | NOT NULL | 交易所简码 |
| `market_name` | TEXT | NOT NULL | 交易所名称 |
| `en_security_type` | TEXT | NOT NULL | 允许证券类型（如 `STOCK` / `FUND` / `BOND` 等） |
| `exchange_code` | INTEGER | NOT NULL DEFAULT 0 | 所属交易所代码（关联 [FinvExchange](FinvExchange.md) `exchange_code`；缺省 0=未维护；V20 新增） |
| `base_currency` | TEXT | NOT NULL | 基础计价货币（如 `CNY` / `USD` / `HKD`） |
| `flag_enable` | CHAR(1) | NOT NULL DEFAULT '0' | 启用标志（'0'=禁用，'1'=启用；V18 新增，供元数据维护菜单切换） |
| `gmt_create` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 首次插入时间 |
| `gmt_update` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 最后更新时间（触发器维护） |

索引：

- `idx_finv_market_security_type`：`(en_security_type, market_code)`
- `idx_finv_market_base_currency`：`(base_currency, market_code)`

## 2. 数据清单（55 条）

> V100018 三次定版（ACANX 2026-08-05）。市场编码体系重大调整（较 V100017 新增 43/删除 42/flag 变化 10）：
> 外汇 `1251`、上交所 A/B/科创板 `1110/1111/1112`、深交所 `1120/1121`、北交所 `1130`、中国商品期货 `1140`、港期所 `1150`、港股 `1201~1203`、美股 `1310~1319`、加拿大 `1401~1404`、亚太 `1501~1521`、欧洲 `1600`、债券 `2001~2004`、基金 `3001~3003`、板块 `8000`、结构化 `9145/9146`、加密 `10001~10015`；
> `en_security_type` 为富途侧类型编码（如 1110/1210/1310），`base_currency` 暂空串。完整清单见迁移文件 [`V100018__finv_market_seed_full.sql`](../../Deploy/Migrations/V100018__finv_market_seed_full.sql)。

## 3. 说明

- **对齐参考**：MySQL `finv_market` 表结构；PG 侧按项目建表规范加固（主键、NOT NULL、审计字段、触发器）。
- **与 finv_exchange 关系**：`finv_exchange` 为交易所/市场整体字典（code 10/11/12...），`finv_market` 为交易所下属细分交易市场代码表，两者通过市场编码语义关联（映射规则待定）。
- **⚠️ 与富途映射表编码不一致**：`finv_futu_mapping_market_code.finv_market_code`（暂=futu 原始值，如 1/10/30/60/200...）与 finv_market 重新编码（1100/1110/1310/2000...）**不一致**，需后续迁移按 market_abbr + market_name 语义对齐映射表。
- **审计字段**：`gmt_create` / `gmt_update` 与既有表规范一致，`gmt_update` 由 `vq_set_gmt_update()` 触发器自动维护。
