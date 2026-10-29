# 扫描水位（Watermark）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.finv_dedup_scan_watermark`

## 1. GET /Watermark/List（查询，不分页）

响应 `data`：`{ total, list: [{ watermark_name, last_value, gmt_update }] }`

## 2. PUT /Watermark（重置，高危）

请求体：`{ watermark_name, last_value }`，`last_value` 为 RFC3339 时间（如 `2026-10-01T00:00:00Z`）。

- 重置后增量扫描会从该时间重新开始，可能产生大量重复扫描，仅用于回补；
- 成功后写审计。
