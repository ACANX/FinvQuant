# FinvSecurity — 证券代码字典映射

> 所属：FinvQuant 数据字典映射 · 存放：`Docs/DataDictMapping/FinvSecurity.md`
> 数据表：`finv_security`（表结构：[`Deploy/Migrations/V100005__finv_security.sql`](../../Deploy/Migrations/V100005__finv_security.sql)；初始数据：[`Deploy/Migrations/V200002__finv_security_seed.sql`](../../Deploy/Migrations/V200002__finv_security_seed.sql)；增量数据：[`Deploy/Migrations/V200009__finv_security_seed_increment.sql`](../../Deploy/Migrations/V200009__finv_security_seed_increment.sql)）
> 用途：统一证券代码表，将各交易所/市场的源证券代码映射到统一证券代码（usc），关联交易所与计价货币字典。

## 1. 表结构

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `usc` | TEXT | PK | 统一证券代码（全局唯一，如 `GCMain` / `HSI` / `000985`） |
| `exchange_code` | INTEGER | NOT NULL，1~999999 | 交易所代码（关联 [FinvExchange](FinvExchange.md) `exchange_code`） |
| `market_code` | INTEGER | NOT NULL DEFAULT 0 | 交易市场代码（关联 [FinvMarket](FinvMarket.md) `market_code`；缺省 0=未维护；V100019 新增，供历史行情导入自动带出/匹配） |
| `security_type` | TEXT | NOT NULL | 证券类型（如 `Futures` / `StockIndex` / `Stock` / `ETF`） |
| `type_secu_sub` | TEXT | NOT NULL DEFAULT '' | 证券属性子类（沿用上游 `type_secu_sub`，如 `Stock.SZ` / `ETF.US`；空串=未维护；V100082 新增） |
| `security_code` | TEXT | NOT NULL | 源证券代码（交易所原始代码） |
| `security_name` | TEXT | NOT NULL | 源证券名称 |
| `security_name_cn` | TEXT | NOT NULL | 证券名称（中文） |
| `security_name_full` | TEXT | 可空 | 证券名称（全称） |
| `currency_type` | TEXT | NOT NULL | 交易计价基础货币（关联 [FinvCurrency](FinvCurrency.md) `currency_type`） |
| `init_date` | INTEGER | NOT NULL DEFAULT 20000000 | 首次上市交易日期（yyyyMMdd） |
| `timezone` | TEXT | 可空 | 时区（如 `-04:00` / `+08:00`） |
| `tz` | TEXT | 可空 | 时区标识（如 `America/New_York` / `Asia/Shanghai`） |
| `provider` | TEXT | NOT NULL DEFAULT '' | 数据提供方（如 `FTMM`；空串=未标注；V100082 新增） |
| `remark` | TEXT | NOT NULL DEFAULT '' | 备注（批次标记，如 `20261030-SupabasePG-finv_quote_collect_futu_view批量导入`；空串=无备注；V100082 新增） |
| `flag_enable` | CHAR(1) | NOT NULL DEFAULT '0' | 启用标志（'0'=禁用，'1'=启用；V100018 新增，供元数据维护菜单切换；禁用后不出现在证券下拉字典） |
| `gmt_create` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 首次插入时间 |
| `gmt_update` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 最后更新时间（触发器维护） |

约束/索引：

- `uq_finv_security_exchange_code`：`(exchange_code, security_code)` 唯一（同一交易所内源代码唯一）
- `idx_finv_security_exchange`：`(exchange_code, usc)`
- `idx_finv_security_type`：`(security_type, usc)`
- `idx_finv_security_currency`：`(currency_type, usc)`

> 说明：MySQL 字段 `currency` / `timezone` 与 JSON 键 `currency_type` / `time_zone` 不一致；`currency_type` 采用 JSON 键名，`timezone` 保持 MySQL 字段命名；`exchange_code` 对齐字典为 INTEGER。

## 2. 数据清单

> 基础初始数据 15 条（V100002，见下表）；**增量补充 500 条**（V100009，基于 `futu.finv_futu_mapping_security` 的 finv_usc，覆盖港股/美股/A股/期货/外汇/加密货币/债券/基金/板块等，清单见种子脚本）。

| usc | exchange_code | security_type | security_code | security_name | security_name_cn | security_name_full | currency_type | init_date | timezone | tz |
|-----|--------------:|---------------|---------------|------------------|--------------------|---------------|-----------|-----------|-----|-----|
| GCMain | 33 | Futures | GCMain | GCMain2512 | 黄金期货主连 | 黄金期货主连 | USD | 19710101 | -04:00 | America/New_York |
| HSTI | 21 | StockIndex | 800700 | 恒生科技指数 | 恒生科技指数 | 恒生科技指数 | HKD | 20000000 | +08:00 | Asia/Shanghai |
| SHI | 11 | StockIndex | 000001 | 上证综合指数 | 上证综指数 | 上证综指数 | CNY | 20000000 | +08:00 | Asia/Shanghai |
| HS300I | 11 | StockIndex | 000300 | 沪深300指数 | 沪深300指数 | 沪深300指数 | CNY | 20000000 | +08:00 | Asia/Shanghai |
| SZCZ | 12 | StockIndex | 深证成交指数 | 深证成交指数 | 深证成交指数 | 深证成交指数 | CNY | 20000000 | +08:00 | Asia/Shanghai |
| CYB | 12 | StockIndex | 创业板指数 | 创业板指数 | 创业板指数 | 创业板指数 | CNY | 20000000 | +08:00 | Asia/Shanghai |
| BSE50I | 13 | StockIndex | 899050 | 北证50 | 北证50指数 | 北证50指数 | CNY | 20000000 | +08:00 | Asia/Shanghai |
| KC50 | 11 | StockIndex | 000688 | 科创50 | 科创50 | 科创50指数 | CNY | 20000000 | +08:00 | Asia/Shanghai |
| A500 | 11 | StockIndex | 000510 | 中证A500 | 中证A500指数 | 中证A500指数 | CNY | 20000000 | +08:00 | Asia/Shanghai |
| HSI | 21 | StockIndex | 800000 | 恒生指数 | 恒生指数 | 恒生指数 | HKD | 20000000 | +08:00 | Asia/Shanghai |
| NDX | 31 | StockIndex | NDX | 纳斯达克综合指数 | 纳斯达克综合指数 | 纳斯达克综合指数 | USD | 20000000 | -04:00 | America/New_York |
| DAJI | 33 | StockIndex | DAJI | 道琼斯工业平均指数 | 道琼斯工业平均指数 | 道琼斯工业平均指数 | USD | 20000000 | -04:00 | America/New_York |
| SDPR500 | 33 | StockIndex | SDPR | 标准普尔500指数 | 标普500指数 | 标普500指数 | USD | 20000000 | -04:00 | America/New_York |
| HXC | 31 | StockIndex | HXC | 纳斯达克中国金龙指数 | 纳斯达克中国金龙指数 | 纳斯达克中国金龙指数 | USD | 20000000 | -04:00 | America/New_York |
| 000985 | 11 | StockIndex | 000985 | 中证全指 | 中证全指 | 中证全指 | CNY | 20000000 | +08:00 | Asia/Shanghai |

## 3. 说明

- **统一证券代码（usc）**：全局唯一标识，跨交易所/市场稳定不变；源证券代码（`security_code`）可能因交易所规则变化，usc 不随其变化。
- **增量数据（V100009）**：`usc` / `security_code` 取自 `futu.finv_futu_mapping_security.finv_usc`（富途 code 经转换规则）；`exchange_code` 按 [FinvExchange.md](FinvExchange.md) 映射（SEHK→21、US→30、SSE→11、SZSE→12、BSE→13、COMEX/NYMEX/CME/CBOT→33、CBOE→34、OSE→51、JP→52、SGX→53、FX→100）；无对应交易所（加密货币/债券/基金/板块等）暂以 `100` 兜底，待 finv_exchange 扩展后修正；`security_type` 扩展至 Stock/ETF/Fund/Futures/StockIndex/Forex/Crypto/Bond/Sector/Structured/Indicator；`init_date` 未知统一 `20000000` 占位；与既有数据冲突的行由 `ON CONFLICT DO NOTHING` 幂等跳过（usc 冲突：GCMain/HXC/NDX；(exchange_code, security_code) 冲突：000688↔KC50、899050↔BSE50I、000510↔A500、800000↔HSI、800700↔HSTI——同一证券在 finv 体系与富途 code 体系下的双记录，待 usc 归一化统一）。
- **关联字典**：`exchange_code` → [FinvExchange.md](FinvExchange.md)（交易所/市场）；`currency_type` → [FinvCurrency.md](FinvCurrency.md)（货币/汇率）。关联不建物理外键，由程序层控制（项目惯例）。
- **扩展列（V100082）**：`type_secu_sub` / `provider` / `remark` 三列均为 `TEXT NOT NULL DEFAULT ''`，空串表示「未维护 / 未标注 / 无备注」，避免存量行出现 NULL 语义分裂（与 `finv_market.market_symbol` 口径一致）。语义位置：`type_secu_sub` 紧跟 `security_type`，`provider` 在 `tz` 之后，`remark` 在 `flag_enable` 之后；物理位置按 PostgreSQL `ADD COLUMN` 限制追加在表尾。
- **Supabase 批量导入批次**：上游视图 `finv_quote_collect_futu_view`（`provider=FTMM`）按 `flag_enable='1'` 下载、转换后插入本表，约定 `flag_enable='0'`（待人工启用）、`provider='FTMM'`、`remark='20261030-SupabasePG-finv_quote_collect_futu_view批量导入'`；**表中已有记录一律不更新**（`ON CONFLICT DO NOTHING`）。上游 `market_code`（富途原码）与本表 `market_code`（`finv_market` 新编码）不是一一对应，转换在导入工具中完成，工具与映射表位于仓库外设计目录 `FinvQuantDesign/Scripts/SecuSync/`。
- **`currency_type` 口径（港股人民币柜台）**：港交所**8 开头的 5 位代码**是人民币柜台（名称带 `-R` / `-WR`，如 `80700` 腾讯控股-R、`81810` 小米集团-WR），`currency_type` 必须为 `CNY`，不能随市场默认写 `HKD`；**6 位** `800100` 一类是港元计价指数（国企指数、恒指波幅指数等），仍为 `HKD`。区分依据是代码位数 + `security_type`（指数类不适用人民币柜台规则）。
- **init_date 占位**：`20000000` 表示未上市或未知（MySQL 默认值，PG 侧保留）。
- **审计字段**：`gmt_create` / `gmt_update` 与既有表规范一致，`gmt_update` 由 `vq_set_gmt_update()` 触发器自动维护。
