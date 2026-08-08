# FinvFutuMappingMarketCode — 富途市场代码映射

> 所属：FinvQuant 数据字典映射 · 存放：`Docs/DataDictMapping/FinvFutuMappingMarketCode.md`
> 数据表：`finv_futu_mapping_market_code`（表结构：[`Deploy/Migrations/V8__finv_futu_mapping_market_code.sql`](../../Deploy/Migrations/V8__finv_futu_mapping_market_code.sql)；重建列序：[`Deploy/Migrations/V15__finv_futu_mapping_market_code_reorder.sql`](../../Deploy/Migrations/V15__finv_futu_mapping_market_code_reorder.sql)；增量列：[`Deploy/Migrations/V17__finv_futu_mapping_add_flag_enable.sql`](../../Deploy/Migrations/V17__finv_futu_mapping_add_flag_enable.sql)；初始数据：[`Deploy/Migrations/V100007__finv_futu_mapping_market_code_seed.sql`](../../Deploy/Migrations/V100007__finv_futu_mapping_market_code_seed.sql)）
> 用途：FT（富途/moomoo）行情源市场代码与交易市场代码（finv_market.market_code）的字段映射表。

## 1. 表结构

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `futu_market_code` | INTEGER | PK | 富途行情源市场代码（如 `1` / `10` / `30` / `70` / `120` / `360`） |
| `market_name` | TEXT | 可空 | 市场名称（如 `港股主板` / `美股指数` / `上交所 A 股`） |
| `exchange` | TEXT | 可空 | 对应交易所（富途 exchange 代码，如 `SEHK` / `US` / `SSE`；无交易所用 `N/A`） |
| `finv_market_code` | INTEGER | NOT NULL，1~999999 | 交易市场代码（关联 [FinvMarket](FinvMarket.md) `market_code`；暂与 futu 同值，待字典对齐） |
| `flag_enable` | CHAR(1) | NOT NULL DEFAULT '0' | 启用标志（`0`=禁用 / `1`=启用，V17 新增） |
| `gmt_create` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 首次插入时间 |
| `gmt_update` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 最后更新时间（触发器维护） |

索引：

- `idx_finv_futu_mapping_market_code_finv`：`(finv_market_code, futu_market_code)`（按交易市场代码反查富途市场代码）

## 2. 数据清单（54 条）

| futu_market_code | finv_market_code | market_name | exchange |
|-----------------:|-----------------:|-------------|----------|
| 1 | 1 | 港股主板 | SEHK |
| 2 | 2 | 港股创业板（GEM） | SEHK |
| 3 | 3 | 港股其他品种 | SEHK |
| 10 | 10 | 美股指数 | US |
| 11 | 11 | 美股行业指数 | US |
| 12 | 12 | 美股 ARCA/BATS 交易所 | US |
| 13 | 13 | 美股 OTC 场外交易 | US |
| 24 | 24 | 美股指数（道琼斯系列） | US |
| 30 | 30 | 上交所 A 股 | SSE |
| 31 | 31 | 深交所 A 股 | SZSE |
| 32 | 32 | 上交所科创板（STAR Market） | SSE |
| 35 | 35 | 深交所创业板（ChiNext） | SZSE |
| 38 | 38 | 北京证券交易所（北交所，Beijing Stock Exchange） | BSE |
| 60 | 60 | 纽约商品交易所（能源/金属期货） | NYMEX |
| 70 | 70 | 纽约商品交易所（黄金/白银期货） | COMEX |
| 80 | 80 | 芝加哥期货交易所（谷物期货） | CBOT |
| 90 | 90 | 芝加哥商业交易所（外汇/利率期货） | CME |
| 113 | 113 | 香港期货交易所 | HKFE |
| 119 | 119 | 中国商品期货 | N/A |
| 120 | 120 | 外汇（Forex） | FX |
| 154 | 154 | 债券 | BD |
| 156 | 156 | 债券 | BD |
| 170 | 170 | 欧洲期货（ICE 等） | N/A |
| 180 | 180 | 新加坡交易所 | SGX |
| 181 | 181 | 新加坡期货 | SGX |
| 200 | 200 | 加拿大 TSX 主板 | CA |
| 201 | 201 | 加拿大 TSX Venture 创业板 | CA |
| 205 | 205 | 加拿大 CSE 交易所 | CA |
| 210 | 210 | 澳大利亚证券交易所 - 衍生品（权证/期权） | ASX |
| 211 | 211 | 澳大利亚 - 其他上市品种 | ASX |
| 265 | 265 | 马来西亚 Bursa 指数 | N/A |
| 271 | 271 | 韩国 KOSPI 指数 | N/A |
| 280 | 280 | 印度 SENSEX 指数 | N/A |
| 360 | 360 | 加密货币 - Binance 等交易所 | CRYPTO |
| 374 | 374 | 加密货币交易对（Binance） | N/A |
| 379 | 379 | 加密货币（Coinbase） | N/A |
| 400 | 400 | 加密货币现货（SGD/USD 计价） | N/A |
| 410 | 410 | 加密货币现货（通用） | N/A |
| 412 | 412 | 加密货币现货（通用） | N/A |
| 413 | 413 | 加密货币现货（通用） | N/A |
| 414 | 414 | 加密货币现货（USDT 计价） | N/A |
| 460 | 460 | 国债收益率 | BD |
| 560 | 560 | 基金（主） | FD |
| 561 | 561 | 基金（子分类） | FD |
| 563 | 563 | 基金（子分类） | FD |
| 585 | 585 | ASX 权证/期权（无交易所标识） | N/A |
| 800 | 800 | 板块/行业分类 | N/A |
| 830 | 830 | 日本（东京证券交易所） | JP |
| 900 | 900 | 债券 | BD |
| 1350 | 1350 | 债券市场 | BMS |
| 1450 | 1450 | 结构性产品 - 香港（FCN=固定息票票据） | FTSN |
| 1451 | 1451 | 结构性产品 - 新加坡 | FTSN |
| 1650 | 1650 | 加拿大 NEO 交易所 | CA |
| 2100 | 2100 | 韩国证券交易所 | KR |

## 3. 说明

- **映射方向**：`futu_market_code`（富途行情源市场代码）→ `finv_market_code`（交易市场代码）；富途侧代码值域由行情源决定，主键不设 CHECK 限制。
- **列顺序**：V15 重建表，列顺序为 `futu_market_code, market_name, exchange, finv_market_code`（V8 原为 futu_market_code/finv_market_code，V15 重建对齐，数据完整迁移，索引/触发器重建）。
- **finv_market_code 当前取值**：暂与 `futu_market_code` 同值（待 [FinvMarket.md](FinvMarket.md) 字典定版后对齐，届时可 UPDATE 归一化）。
- **market_name / exchange**：V15 重建表引入，描述市场名称与对应交易所（`—` 用 `N/A`）。
- **关联字典**：`exchange` → [FinvFutuMappingExchange.md](FinvFutuMappingExchange.md)（富途交易所）；`finv_market_code` → [FinvMarket.md](FinvMarket.md)（交易市场）；关联不建物理外键，由程序层控制（项目惯例）。
- **审计字段**：`gmt_create` / `gmt_update` 与既有表规范一致，`gmt_update` 由 `vq_set_gmt_update()` 触发器自动维护。
