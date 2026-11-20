# GET /API/V1/Meta/Finv/Quote/DeDup/Job/List — 分页查询导出任务

分页查询证券月归档导出任务（`dedup.finv_dedup_export_job`）。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quote/DeDup/Job/List`
- **Query 参数**（小驼峰）：

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `status` | string | 否 | 任务状态过滤：`WAITING`/`LOADING`/`LOADED`/`EXPORTING`/`EXPORTED`/`PAUSED`/`FAILED`/`FROZEN` |
| `usc` | string | 否 | 证券唯一编码（精确匹配） |
| `bizMonth` | int | 否 | 归档月份 `yyyyMM` |
| `page` | int | 否 | 页码，默认 1 |
| `pageSize` | int | 否 | 每页条数，默认 20，上限 200 |

### 成功（HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 1,
    "list": [
      {
        "job_id": 128,
        "usc": "159934",
        "biz_month": 202609,
        "priority_rank": 100,
        "freeze_date": "2026-10-30T00:00:00Z",
        "seal_date": "2026-10-25T00:00:00Z",
        "status": "EXPORTED",
        "load_cursor_date": 20260930,
        "load_cursor_ts": 0,
        "loaded_rows": 4820,
        "base_version": 1,
        "patch_count": 0,
        "oldest_pending_date": 20260901,
        "locked_by": null,
        "locked_until": null,
        "gmt_heartbeat": null,
        "attempt": 1,
        "gmt_next_run": null,
        "last_error": null,
        "gmt_create": "2026-10-24T16:01:03Z",
        "gmt_update": "2026-10-24T16:02:11Z"
      }
    ]
  }
}
```

> 字段与数据库列名一致（snake_case）。`load_cursor_date` 为 `yyyyMMdd`，`load_cursor_ts` 为当日已处理到的最大 `ts`。

### 错误

| `code` | HTTP | 含义 |
|---|---|---|
| `2006` | 500 | 查询导出任务失败 |

## 已使用位置登记

- 运维排查（暂无前端菜单）
