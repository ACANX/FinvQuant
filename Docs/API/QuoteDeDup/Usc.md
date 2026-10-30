# 证券主数据（Usc）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.finv_dedup_usc`；实现：`internal/api/handler/cdeu_admin.go`

`day_expect_rows` 是成熟度判定的校准值：为空时该证券的日数据会长期停留在 `PARTIAL`，永不进入导出候选。

自动补全（需求 10820 / 10823）：成熟度刷新 JOB 与导出 JOB 会按以下顺序为该列为空/元数据不全的证券自动补值，全部**只补空、不覆盖人工值**：

1. 从 `public.finv_security` 回填 `category`/`region`/`market`/`name`/`timezone`/`currency`/`provider`（**不限 `flag_enable`**，禁用只是字典维护开关）；
2. `finv_security` 未收录时，从 `futu.finv_futu_quote_collect`（富途原始属性镜像）按 `quote_market`（SH/SZ/BJ/HK/US/FX/CC）与 `type_symbol` 兜底；
3. 按 `region` 推导 `timezone` / `currency`（富途镜像表不带这两列）；
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
