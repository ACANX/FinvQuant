# 上传审计（UploadLog）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.finv_dedup_export_file_upload_log`（只读，每次上传尝试一条）

## GET /UploadLog/Page（查询）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| fileId | int64 | 否 | 导出文件 ID |
| status | string | 否 | `SUCCESS` / `FAILED` |
| keyword | string | 否 | `error` 字段模糊匹配 |
| dateFrom / dateTo | string | 否 | `yyyy-MM-dd`，按 `gmt_start` 过滤 |
| page / pageSize | int | 否 | 默认 1 / 20 |

响应 `data`：`{ total, list: [{ log_id, file_id, attempt, target_path, gmt_start, gmt_finish, status, uploaded_bytes, error }] }`
