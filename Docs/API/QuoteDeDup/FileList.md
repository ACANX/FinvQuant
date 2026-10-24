# GET /API/V1/Meta/Finv/Quote/DeDup/File/List — 分页查询导出文件

分页查询导出文件登记与上传状态（`dedup.finv_dedup_export_file`，上传任务的唯一数据源）。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quote/DeDup/File/List`
- **Query 参数**（小驼峰）：

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `uploadStatus` | string | 否 | `PENDING_UPLOAD`/`UPLOADING`/`UPLOADED`/`UPLOAD_FAILED`/`SKIPPED` |
| `fileType` | string | 否 | `BASE`/`PATCH`/`MANIFEST` |
| `usc` | string | 否 | 证券唯一编码 |
| `bizMonth` | int | 否 | 归档月份 `yyyyMM` |
| `page` | int | 否 | 页码，默认 1 |
| `pageSize` | int | 否 | 每页条数，默认 20，上限 200 |

### 成功（HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 2,
    "list": [
      {
        "file_id": 512,
        "job_id": 128,
        "usc": "159934",
        "biz_month": 202609,
        "file_type": "BASE",
        "version": 1,
        "file_name": "CN_SZ_159934_Min_FTMM_Mon_202609.mvsv",
        "file_path": "Data/ExportArchive/159934/202609/CN_SZ_159934_Min_FTMM_Mon_202609.mvsv",
        "target_file_name": "CN_SZ_159934_Min_FTMM_Mon_202609.mvsv",
        "upload_target": "cn/quotes/CN_SZ_159934_Min_FTMM_Mon_202609.mvsv",
        "format": "mvsv",
        "compression": "none",
        "row_count": 4820,
        "file_size": 524288,
        "checksum": "3f8f0c1c6f4f0d6a5d1f5f2b7a9c0e3d4b6a8c2e1f7d9a0b3c5e7f1a2b4c6d8e",
        "manifest_path": "Data/ExportArchive/159934/202609/CN_SZ_159934_Min_FTMM_Mon_202609_Manifest.json",
        "seal_date": "2026-10-25T00:00:00Z",
        "export_status": "EXPORTED",
        "upload_status": "UPLOADED",
        "upload_attempt": 1,
        "gmt_next_retry": null,
        "gmt_upload_start": "2026-10-24T16:03:00Z",
        "gmt_upload_finish": "2026-10-24T16:03:02Z",
        "remote_checksum_verified": true,
        "network_status": "NONE",
        "cleanup_status": "NONE",
        "gmt_create": "2026-10-24T16:02:11Z",
        "gmt_update": "2026-10-24T16:03:02Z"
      }
    ]
  }
}
```

字段说明：

| 字段 | 说明 |
|---|---|
| `file_name` / `target_file_name` | 本地文件名 / 上传目标文件名（当前一致，均为 `.mvsv` 或 `.json`，不做扩展名改名） |
| `upload_target` | 远端相对路径（`FINV_QUOTE_DEDUB_UPLOAD_PREFIX` + 文件名） |
| `checksum` | 文件 SHA256（小写十六进制）；gzip 时为压缩后内容 |
| `export_status` | `EXPORTED`/`SUPERSEDED`（被 L1_HOT 重写覆盖）/`SKIPPED` |
| `network_status` | `NONE`/`ARCHIVED`（网盘归档由外部流程更新） |
| `cleanup_status` | `NONE`/`LOCAL_DELETED` |

### 错误

| `code` | HTTP | 含义 |
|---|---|---|
| `2006` | 500 | 查询导出文件失败 |

## 已使用位置登记

- 运维排查（暂无前端菜单）
