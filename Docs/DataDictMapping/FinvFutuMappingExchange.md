# FinvFutuMappingExchange — 富途交易所映射

> 所属：FinvQuant 数据字典映射 · 存放：`Docs/DataDictMapping/FinvFutuMappingExchange.md`
> 数据表：`finv_futu_mapping_exchange`（表结构：[`Deploy/Migrations/V14__finv_futu_mapping_exchange.sql`](../../Deploy/Migrations/V14__finv_futu_mapping_exchange.sql)；增量列：[`Deploy/Migrations/V17__finv_futu_mapping_add_flag_enable.sql`](../../Deploy/Migrations/V17__finv_futu_mapping_add_flag_enable.sql)；初始数据：[`Deploy/Migrations/V100006__finv_futu_mapping_exchange_seed.sql`](../../Deploy/Migrations/V100006__finv_futu_mapping_exchange_seed.sql)；全量替换：[`Deploy/Migrations/V100013__finv_futu_mapping_exchange_seed_full.sql`](../../Deploy/Migrations/V100013__finv_futu_mapping_exchange_seed_full.sql)）
> 用途：富途行情源 exchange 字典（30 类）的字段映射表，记录富途交易所代码、对应地区与 finv 侧交易所标识。

## 1. 表结构

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `futu_exchange` | TEXT | PK | 富途 exchange 代码（如 `SEHK` / `SSE` / `COMEX`） |
| `region` | TEXT | NOT NULL | 对应地区（如 `香港` / `中国` / `美国`；`—` 表示无地区归属） |
| `abbr` | TEXT | NOT NULL | 地区简写（对齐 [FinvRegion](FinvRegion.md)，如 `HK` / `CN` / `USA`；`—` 用 `N/A`） |
| `exchange_name` | TEXT | NOT NULL | 交易所/市场名称（如 `香港交易所`） |
| `finv_exchange` | TEXT | NOT NULL | finv 侧交易所标识（对齐 [FinvExchange](FinvExchange.md) `exchange_abbr`，如 `HKEX` / `SSE` / `TSE`） |
| `flag_enable` | CHAR(1) | NOT NULL DEFAULT '0' | 启用标志（`0`=禁用 / `1`=启用，V17 新增） |
| `gmt_create` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 首次插入时间 |
| `gmt_update` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 最后更新时间（触发器维护） |

索引：

- `idx_finv_futu_mapping_exchange_finv`：`(finv_exchange, futu_exchange)`（按 finv 侧交易所标识反查富途 exchange）

## 2. 数据清单（30 条）

| futu_exchange | region | abbr | exchange_name | finv_exchange |
|---------------|--------|------|---------------|---------------|
| ASX | 澳大利亚 | AU | 澳大利亚证券交易所 | ASX |
| BD | — | N/A | 债券（Bond） | BD |
| BMD | 马来西亚 | MY | 马来西亚衍生品交易所（Bursa Malaysia Derivatives） | BMD |
| BMS | — | N/A | 债券市场系统（Bond Market System） | BMS |
| CA | 加拿大 | CA | 加拿大证券市场（TSX/TSXV/CSE 统称） | CA |
| CBOT | 美国 | USA | 芝加哥期货交易所（谷物/国债期货） | CBOT |
| CCCOIN | 全球 | GLOBAL | 加密货币（Coinbase） | CCCOIN |
| CCBGO | 全球 | GLOBAL | 加密货币（Bitget/BitGo） | CCBGO |
| CCDDEX | 全球 | GLOBAL | 加密货币去中心化交易所（DEX） | CCDDEX |
| CCHSK | 全球 | GLOBAL | 加密货币（HashKey） | CCHSK |
| CBOE | 美国 | USA | 芝加哥期权交易所 | CBOE |
| CCPT | 全球 | GLOBAL | 加密货币（PT/Poloniex） | CCPT |
| CME | 美国 | USA | 芝加哥商业交易所（外汇/利率/股指期货） | CME |
| COMEX | 美国 | USA | 纽约商品交易所（黄金/白银期货） | COMEX |
| CRYPTO | 全球 | GLOBAL | 加密货币交易所（Binance 等） | CRYPTO |
| FD | — | N/A | 基金（Fund） | FD |
| FTSP | — | N/A | 富途结构化产品平台 | FTSP |
| FTSN | 香港/新加坡 | HK/SG | 富途结构化产品网络（Futu Structured Network） | FTSN |
| FX | 全球 | GLOBAL | 外汇市场（Forex） | FX |
| HKFE | 香港 | HK | 香港期货交易所 | HKFE |
| JP | 日本 | JP | 日本东京证券交易所 | TSE |
| KR | 韩国 | KR | 韩国证券交易所（KRX） | KRX |
| NYMEX | 美国 | USA | 纽约商品交易所（能源/金属期货） | NYMEX |
| OSE | 日本 | JP | 大阪交易所（日本商品/股指期货） | OSE |
| SEHK | 香港 | HK | 香港交易所（Hong Kong Stock Exchange） | HKEX |
| SGX | 新加坡 | SG | 新加坡交易所 | SGX |
| SSE | 中国 | CN | 上海证券交易所 | SSE |
| SZSE | 中国 | CN | 深圳证券交易所 | SZSE |
| TFD | — | N/A | 测试/仿真环境 | TFD |
| US | 美国 | USA | 美国证券市场（NYSE/NASDAQ/ARCA/BATS/OTC 统称） | US |

## 3. 说明

- **映射方向**：`futu_exchange`（富途 exchange 代码）→ `finv_exchange`（finv 侧交易所标识）。
- **finv_exchange 当前取值**：V100013 全量替换后，`finv_exchange` 对齐 [FinvExchange.md](FinvExchange.md) 的 `exchange_abbr`（如 `SEHK → HKEX`、`JP → TSE`、`KR → KRX`、`SSE → SSE`、`COMEX → COMEX`）；全部 30 个映射值均能在 finv_exchange 字典中找到对应（ACANX 2026-08-05 定版）。
- **abbr 取值**：与 [FinvRegion.md](FinvRegion.md) 简写体系一致（`CN` / `HK` / `USA` / `JP` / `SG` / `GLOBAL` 等）；无地区（`—`）用 `N/A`；`FTSN` 双地区（香港/新加坡）用 `HK/SG`。
- **关联字典**：`abbr` / `region` → [FinvRegion.md](FinvRegion.md)（区域字典）；`finv_exchange` → [FinvExchange.md](FinvExchange.md)（交易所字典，待对齐）；关联不建物理外键，由程序层控制（项目惯例）。
- **审计字段**：`gmt_create` / `gmt_update` 与既有表规范一致，`gmt_update` 由 `vq_set_gmt_update()` 触发器自动维护。
