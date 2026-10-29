# 入口日统计（Stats）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.finv_dedup_ingest_stats`（只读，由 `CALC_DAILY_INGEST_STATS` 生成）

## GET /Stats/Page（查询）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| dateFrom / dateTo | string | 否 | `yyyy-MM-dd`，按 `stat_date` 过滤 |
| page / pageSize | int | 否 | 默认 1 / 20 |

响应 `data`：`{ total, list: [{ stat_date, total_rows, total_bytes, gmt_update }] }`
