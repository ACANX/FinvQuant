# FinvFutuMappingCSMarket — 富途 CS 市场映射

> 所属：FinvQuant 数据字典映射 · 存放：`Docs/DataDictMapping/FinvFutuMappingCSMarket.md`
> 数据表：`finv_futu_mapping_cs_market`（表结构：[`Deploy/Migrations/V9__finv_futu_mapping_cs_market.sql`](../../Deploy/Migrations/V9__finv_futu_mapping_cs_market.sql)；新增列/约束放宽：[`Deploy/Migrations/V16__finv_futu_mapping_cs_market_add_exchange_name.sql`](../../Deploy/Migrations/V16__finv_futu_mapping_cs_market_add_exchange_name.sql)；增量列：[`Deploy/Migrations/V17__finv_futu_mapping_add_flag_enable.sql`](../../Deploy/Migrations/V17__finv_futu_mapping_add_flag_enable.sql)；初始数据：[`Deploy/Migrations/V100008__finv_futu_mapping_cs_market_seed.sql`](../../Deploy/Migrations/V100008__finv_futu_mapping_cs_market_seed.sql)；全量替换：[`Deploy/Migrations/V100011__finv_futu_mapping_cs_market_seed_full.sql`](../../Deploy/Migrations/V100011__finv_futu_mapping_cs_market_seed_full.sql)）
> 用途：FT（富途/moomoo）行情源 CS 市场标识与交易所代码（finv_exchange.exchange_code）的字段映射表。

## 1. 表结构

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `futu_cs_market` | TEXT | PK | 富途行情源 CS 市场标识（如 `0` / `1` / `8` / `17`） |
| `finv_exchange_code` | INTEGER | NOT NULL，-1~999999 | 交易所代码（关联 [FinvExchange](FinvExchange.md) `exchange_code`；无对应用 `-1` 缺省） |
| `exchange_name` | TEXT | 可空 | 交易所/市场名称（如 `香港证券市场` / `美国证券市场`，V16 新增） |
| `flag_enable` | CHAR(1) | NOT NULL DEFAULT '0' | 启用标志（`0`=禁用 / `1`=启用，V17 新增） |
| `gmt_create` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 首次插入时间 |
| `gmt_update` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 最后更新时间（触发器维护） |

索引：

- `idx_finv_futu_mapping_cs_market_finv`：`(finv_exchange_code, futu_cs_market)`（按交易所代码反查富途 CS 市场）

## 2. 数据清单（29 条）

> V100011 全量替换 V100008（先 DELETE 存量，再 INSERT 全量 29 条）；相对 V100008 有 10 处映射值更新（0→0、6→22、8→300、17→10001、21→41、22→56、29→44、30→43、33→42、36→57）。

| futu_cs_market | exchange_name | finv_exchange_code |
|----------------|---------------|-------------------:|
| 0 | 场外交易/结构化产品/加密货币（无明确交易所） | 0 |
| 1 | 香港证券市场 | 21 |
| 2 | 美国证券市场 | 30 |
| 3 | 上海证券交易所（A 股） | 11 |
| 4 | 深圳证券交易所（A 股） | 12 |
| 6 | 香港期货交易所 | 22 |
| 8 | 美国期货交易所 | 300 |
| 10 | 上交所科创板（STAR Market） | 11 |
| 11 | 外汇市场（Forex） | 100 |
| 12 | 债券 | -1 |
| 13 | 新加坡期货 | 53 |
| 14 | 全球主要指数（FTSE/DAX/CAC 等） | -1 |
| 15 | 新加坡交易所 | 53 |
| 16 | 日本大阪交易所（期货） | 51 |
| 17 | 加密货币市场 | 10001 |
| 18 | 国债收益率 | -1 |
| 19 | 基金（Fund） | -1 |
| 21 | 加拿大 CSE 交易所 | 41 |
| 22 | 澳大利亚证券交易所 | 56 |
| 23 | 北京证券交易所（北交所，Beijing Stock Exchange） | 13 |
| 24 | 日本（板块分类） | 52 |
| 25 | 日本东京证券交易所 | 52 |
| 27 | 债券市场（Bond Market System） | -1 |
| 28 | 马来西亚 Bursa Malaysia | -1 |
| 29 | 加拿大 TSX 主板 | 44 |
| 30 | 加拿大 TSX Venture 创业板 | 43 |
| 33 | 加拿大 NEO 交易所 | 42 |
| 36 | 韩国证券交易所 | 57 |
| 37 | 其他 | -1 |

## 3. 说明

- **映射方向**：`futu_cs_market`（富途行情源 CS 市场标识）→ `finv_exchange_code`（[FinvExchange.md](FinvExchange.md) `exchange_code`）。
- **finv_exchange_code 取值**：优先取 finv_exchange 对应交易所的 exchange_code（`SEHK→21`、`US→30`、`SSE→11`、`SZSE→12`、`FX→100`、`SGX→53`、`OSE→51`、`BSE→13`、`JP→52`）；**无对应交易所（FTSN/FTSP、HKFE、美期多所聚合、BD、BMS、BMD、CA、ASX、KR、CRYPTO、FD、全球指数等）→ `-1` 缺省**（V16 将 CHECK 由 1~999999 放宽为 -1~999999）。
- **exchange_name**：V16 新增列，描述交易所/市场名称（源自富途 cs_market 推测含义）。
- **后续扩展**：`-1` 项待 finv_exchange 扩展对应交易所（如 HKFE、ASX、KR、CA、加密货币等）后，可 UPDATE 补齐真实 exchange_code。
- **审计字段**：`gmt_create` / `gmt_update` 与既有表规范一致，`gmt_update` 由 `vq_set_gmt_update()` 触发器自动维护。
