# 证券主数据（Usc）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.finv_dedup_usc`；实现：`internal/api/handler/exportarchive_admin.go`

`day_expect_rows` 是成熟度判定的校准值：为空时该证券的日数据会长期停留在 `PARTIAL`，永不进入导出候选。

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
