# 证券主数据（Usc）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`cdeu.finv_quote_cdeu_usc`；实现：`internal/api/handler/cdeu_admin.go`

## 0. 行序来源（需求 10920）

这张表的行**唯一来源是主表 `cdeu.finv_quote_secu_kline_min` 中真实出现过的证券**，不是证券字典 `public.finv_security`：

- 发现（`DiscoverUsc`）：首次执行全量扫描主表 `DISTINCT usc`，之后按 `gmt_update` 时间窗增量补充；
- 缺口自愈：每天最多一次检查「主表里还有没有未登记的 usc」（`uscHasUnregistered`，水位键 `DISCOVER_USC_GAP_CHECK`），发现缺口自动升级为全量扫描——补导历史数据、人工清表后都无需手工干预；
- 存量清理：10920 之前的脏行用一次性脚本 `Deploy/Upgrade/2611-001-PurgeUscWithoutQuote.sql` 删除（备份到 `archive.finv_quote_cdeu_usc_purge_20261102`，单事务 + `RAISE EXCEPTION` 校验）；
- `date_list`（上市/上线日期）来源：优先 `public.finv_security.init_date`（`RefreshSecurityMetadata`）；上游未收录时由 `DiscoverUsc` 按主表最早交易日（`min(date)`）写入，存量由 `V300097` 发放（美股 5 只用公开上市日，其余取主表最早交易日）；`date_delist` 为空表示仍在市；
- 数据发放：`V300096` 用 SQL 补齐两类缺口——① `region=CC`（加密货币）补模板行 `CC/FTC/Crypto=1440`、`timezone=UTC`、`day_expect_rows=1440`；② 上游字典（`finv_security` / futu 镜像）未收录但行情确在采的证券按显式清单补齐（美股 5 只、`XRP`、3 个 A 股指数）；`name` 仍无来源的用 `usc` 兜底（不编造名称）；
- 元数据补全（`RefreshSecurityMetadata`）：只对**已登记**的 usc 用字典刷新 `category` / `region` / `market` / `name` / `timezone` / `currency`，**不再插入新行**；
- 永不删除：某只证券当月/上月乃至连续几个月没有数据（停牌、退市、上游断供）时仍保留记录，否则会连带丢掉归档优先级、`day_expect_rows` 与人工维护的元数据。

判据：`SELECT count(*) FROM cdeu.finv_quote_cdeu_usc u WHERE NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_secu_kline_min m WHERE m.usc = u.usc);` —— 除人工通过 `PUT /Usc` 预建的证券外，该值应为 0。

`day_expect_rows` 是成熟度判定的校准值：为空时该证券的日数据会长期停留在 `PARTIAL`，永不进入导出候选。

自动补全（需求 10820 / 10823）：成熟度刷新 JOB 与导出 JOB 会按以下顺序为该列为空/元数据不全的证券自动补值，全部**只补空、不覆盖人工值**：

1. 从 `public.finv_security` 回填 `category`/`region`/`market`/`name`/`timezone`/`currency`/`provider`（**不限 `flag_enable`**，禁用只是字典维护开关；只回填已登记的 usc，不新建行）；
2. `finv_security` 未收录时，从 `futu.finv_futu_quote_collect`（富途原始属性镜像）按 `market_label`（SH/SZ/BJ/HK/US/FX/CC）与 `instrument` 兜底；
3. 按 `region` 推导 `timezone` / `currency`（富途镜像表不带这两列）：`CN`→`Asia/Shanghai`+`CNY`、`HK`→`Asia/Hong_Kong`+`HKD`、`US`→`America/New_York`+`USD`、`FX`/`GLOBAL`/`CC`→`UTC`（后三者不推币种）；
4. `SH######` / `SZ######` 前缀的 usc 按 A 股指数兜底（`region=CN`、`market=SH/SZ`、`category=StockIndex`）；
5. `day_expect_rows` 按模板表 `cdeu.finv_quote_cdeu_usc_day_incr_row_tpl`（`region` + `market` + `category`，三列可空串通配）匹配。

补全后仍缺字段的证券（通常是 `finv_security` 与富途镜像表都未收录的品种）会出现在 JOB 日志的「仍缺 …；样本=…」行里，需人工补元数据（`PUT /Usc`）或补主数据字典（如 `finv_market.market_symbol`）。

## 1. GET /Usc/Page（查询）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| usc | string | 否 | 证券代码精确匹配 |
| keyword | string | 否 | 按 `usc` 或 `name` 模糊匹配 |
| page / pageSize | int | 否 | 默认 1 / 20（最大 200） |

响应 `data`：`{ total, list: [{ usc, category, day_expect_rows, date_list, date_delist, flag_auto_discover, region, market, provider, name, timezone, currency, gmt_create, gmt_update }] }`

## 2. PUT /Usc（新增或更新）

请求体（`usc` 必填，其余为 null 表示不修改）：

`{ usc, category, day_expect_rows, date_list, date_delist, flag_auto_discover, region, market, provider, name, timezone, currency }`

- `date_list` / `date_delist` 格式 `yyyy-MM-dd`；
- `day_expect_rows` 不得为负；
- 成功后写审计：`MANUAL_ADMIN_EDIT`。

错误码：`400` 参数非法；`409` 写入失败（响应信封见 `Docs/DevSpec/ApiSpec.md`）。
