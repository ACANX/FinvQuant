# FinvExchange — 交易所/市场字典映射

> 所属：FinvQuant 数据字典映射 · 存放：`Docs/DataDictMapping/FinvExchange.md`
> 数据表：`finv_exchange`（表结构：[`Deploy/Migrations/V2__finv_exchange.sql`](../../Deploy/Migrations/V2__finv_exchange.sql)；初始数据：[`Deploy/Migrations/V100000__finv_exchange_seed.sql`](../../Deploy/Migrations/V100000__finv_exchange_seed.sql)；全量替换：[`Deploy/Migrations/V100010__finv_exchange_seed_full.sql`](../../Deploy/Migrations/V100010__finv_exchange_seed_full.sql) / [`Deploy/Migrations/V100016__finv_exchange_seed_full.sql`](../../Deploy/Migrations/V100016__finv_exchange_seed_full.sql)）
> 用途：交易所/市场编码字典（单一事实来源），覆盖证券 / 期货 / 黄金及贵金属 / 场外 / 期权 / 外汇市场。

## 1. 表结构

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `exchange_code` | INTEGER | PK，1~999999 | 市场数字代码 |
| `exchange_flag` | TEXT | NOT NULL, UNIQUE | 交易所标志（如 `CN` / `SH` / `NSDQ` / `FX`） |
| `exchange_abbr` | TEXT | NOT NULL | 交易所英文缩写（如 `SSE` / `SZSE` / `HKEX`） |
| `exchange_name` | TEXT | NOT NULL | 交易所英文全称 |
| `exchange_abbr_cn` | TEXT | NOT NULL | 交易所中文缩写/名称（如 `上交所` / `纳斯达克`） |
| `en_market_type` | TEXT | NOT NULL | 市场类型（证券 / 期货 / 黄金及贵金属 / 场外 / 期权 / 外汇） |
| `region` | TEXT | NOT NULL | 地区编码（如 `CN` / `HK` / `USA` / `JP`） |
| `base_currency` | TEXT | NOT NULL | 基础计价货币（如 `CNY` / `USD` / `HKD`） |
| `ft_list_exchange_code` | TEXT | 可空 | FT 行情源列表交易所编码（映射预留，暂空，后续补充） |
| `flag_enable` | CHAR(1) | NOT NULL DEFAULT '0' | 启用标志（'0'=禁用，'1'=启用；V18 新增，供元数据维护菜单切换） |
| `gmt_create` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 首次插入时间 |
| `gmt_update` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 最后更新时间（触发器维护） |

索引：

- `idx_finv_exchange_region`：`(region, exchange_code)`
- `idx_finv_exchange_market_type`：`(en_market_type, exchange_code)`

## 2. 数据清单（44 条）

> V100016 再次全量替换 V100010（先 DELETE 存量，再 INSERT 全量 44 条）；相对 V100010 仅 1 处调整：code 101 FX-CFD 的 `exchange_name`/`exchange_abbr_cn` 互换（exchange_name='FX-CFD'，exchange_abbr_cn='外汇-差价合约'），其余 43 条一致；`ft_list_exchange_code` 列暂为空。

| exchange_code | exchange_flag | exchange_abbr | exchange_name | exchange_abbr_cn | en_market_type | region | base_currency | ft_list_exchange_code |
|--------------:|---------------|---------------|---------------|------------------|----------------|--------|---------------|-----------------------|
| 10 | CN | CN | 中国证券市场 | 中国证券市场(全体) | 证券 | CN | CNY | |
| 11 | SH | SSE | 上海证券交易所 | 上交所 | 证券 | CN | CNY | |
| 12 | SZ | SZSE | 深圳证券交易所 | 深交所 | 证券 | CN | CNY | |
| 13 | BJ | BSE | 北京证券交易所 | 北交所 | 证券 | CN | CNY | |
| 14 | SHFE | SHFE | 上海期货交易所 | 上海期货交易所 | 期货 | CN | CNY | |
| 15 | SGE | SGE | 上海黄金交易所 | 上海黄金交易所 | 黄金及贵金属 | CN | CNY | |
| 19 | CNOTC | CNOTC | 中国场外交易 | 中国场外交易 | 场外 | CN | CNY | |
| 21 | HK | HKEX | 香港联合证券交易所 | 港交所 | 证券 | HK | HKD | |
| 22 | HKFE | HKFE | Hong Kong Futures Exchange | 香港期货交易所 | 期货 | HK | HKD | |
| 29 | TW | TWSE | 台湾证券交易所 | 台交所 | 证券 | TW | NTD | |
| 30 | US | US | 美国证券市场 | 美国证券市场(全体) | 证券 | USA | USD | |
| 31 | NSDQ | NSDQ | 纳斯达克证券交易所 | 纳斯达克 | 证券 | USA | USD | |
| 32 | NYSE | NYSE | 纽约证券交易所 | 纽交所 | 证券 | USA | USD | |
| 33 | COMEX | COMEX | 芝加哥商品期货交易所 | 芝商所 | 期货 | USA | USD | |
| 34 | CBOE | CBOE | 芝加哥期权交易所 | 芝加哥期权交易所 | 期权 | USA | USD | |
| 35 | PINK | PINK | 粉红单交易市场 | 粉单市场 | 场外 | USA | USD | |
| 36 | NYMEX | NYMEX | New York Mercantile Exchange | 纽约商品交易所 | 期货 | USA | USD | |
| 37 | CME | CME | Chicago Mercantile Exchange | 芝加哥商业交易所 | 期货 | USA | USD | |
| 38 | CBOT | CBOT | Chicago Board of Trade | 芝加哥期货交易所 | 期货 | USA | USD | |
| 40 | CA | CA | Canadian Securities Market | 加拿大证券市场 | 证券 | CA | CAD | |
| 51 | OSE | OSE | 大阪证券交易所 | 大阪证券交易所 | 证券 | JP | JPY | |
| 52 | TSE | TSE | 东京证券交易所 | 东京证券交易所 | 证券 | JP | JPY | |
| 53 | SGX | SGX | 新加坡证券交易所 | 新交所 | 证券 | SG | SGD | |
| 54 | INBSE | INBSE | 孟买证券交易所 | 孟买交易所 | 证券 | IN | INR | |
| 55 | NSE | NSE | 印度国家证券交易所 | 印度国家证券交易所 | 证券 | IN | INR | |
| 56 | ASX | ASX | Australian Securities Exchange | 澳大利亚证券交易所 | 证券 | AU | AUD | |
| 57 | KRX | KRX | Korea Exchange | 韩国证券交易所 | 证券 | KR | KRW | |
| 58 | BMD | BMD | Bursa Malaysia Derivatives | 马来西亚衍生品交易所 | 期货 | MY | MYR | |
| 100 | FX | FX | 外汇市场 | 外汇交易市场 | 外汇 | FX | CNY | |
| 101 | FX-CFD | FX-CFD | FX-CFD | 外汇-差价合约 | 外汇 | FX | USD | |
| 120 | BD | BD | Bond Market | 债券市场 | 债券 | N/A | USD | |
| 121 | BMS | BMS | Bond Market System | 债券市场系统 | 债券 | N/A | USD | |
| 122 | FD | FD | Fund Market | 基金市场 | 基金 | N/A | USD | |
| 9001 | FTSP | FTSP | Futu Structured Product Platform | 富途结构化产品平台 | 结构化 | HK | USD | |
| 9002 | FTSN | FTSN | Futu Structured Network | 富途结构化产品网络 | 结构化 | HK | USD | |
| 9003 | TFD | TFD | Test/Simulation Environment | 测试/仿真环境 | 测试 | N/A | USD | |
| 10000 | CC | CC | CryptoCoin Exchange | 加密货币交易所 | 加密货币 | GLOBAL | USD | |
| 10001 | CRYPTO | CRYPTO | Cryptocurrency Exchange | 加密货币交易所 | 加密货币 | GLOBAL | USD | |
| 10002 | CCBA | CCBA | Binance Exchange | 币安交易所 | 加密货币 | GLOBAL | USD | |
| 10003 | CCCOIN | CCCOIN | Coinbase | Coinbase | 加密货币 | GLOBAL | USD | |
| 10004 | CCBGO | CCBGO | Bitget/BitGo | Bitget/BitGo | 加密货币 | GLOBAL | USD | |
| 10005 | CCDDEX | CCDDEX | Decentralized Exchange (DEX) | 去中心化交易所 | 加密货币 | GLOBAL | USD | |
| 10006 | CCHSK | CCHSK | HashKey | HashKey | 加密货币 | GLOBAL | USD | |
| 10007 | CCPT | CCPT | Poloniex (PT) | Poloniex | 加密货币 | GLOBAL | USD | |

## 3. 说明

- **迁移拆分**：表结构（`V2__finv_exchange.sql`）与初始数据（`V100000__finv_exchange_seed.sql`）分文件存放；数据种子统一使用 **V100000+ 段位**，确保在所有表结构脚本（V1~V99999）之后执行，后续新增表结构/变更脚本不受影响。
- **全量替换（V100010/V100016）**：V100000 已发布不可修改，V100010 先 `DELETE FROM finv_exchange` 清空存量再 INSERT 全量 44 条；V100016（ACANX 2026-08-05 二次定版）再次全量替换，仅 code 101 名称字段互换；均单事务失败回滚、重复执行幂等。
- **exchange_code 段位**：1~999 常规市场（证券/期货/外汇等），9001+ 富途结构化产品/测试环境，10000+ 加密货币交易所；`exchange_flag` / `exchange_abbr` 均唯一。
- **数据来源**：FT 交易所清单；原始数据中 `exchange_code=19` 存在 `OTC` / `CNOTC` 两条冲突记录，经 ACANX 确认**仅保留 `19 CNOTC 中国场外交易`**。
- **映射表依赖**：`finv_futu_mapping_cs_market.finv_exchange_code`（V100008）引用的 `-1`（无对应）项，可在本表扩充后由后续迁移 UPDATE 补齐（如 HKFE/ASX/KR/CA/CRYPTO/BD/FD 等）。
- **映射预留**：`ft_list_exchange_code` 字段已建列、当前留空，待 FT 行情源列表编码映射确定后补充（`UPDATE finv_exchange SET ft_list_exchange_code = ... WHERE exchange_code = ...`）。
- **幂等插入**：初始数据使用 `INSERT ... ON CONFLICT (exchange_code) DO NOTHING`，重复执行不产生重复记录。
- **审计字段**：`gmt_create` / `gmt_update` 与既有表规范一致，`gmt_update` 由 `vq_set_gmt_update()` 触发器自动维护。
