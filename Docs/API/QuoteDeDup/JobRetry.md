# POST /API/V1/Meta/Finv/Quote/DeDup/Job/Retry — 重试导出任务

把失败 / 暂停 / 冻结的证券月导出任务重置为 `WAITING`，清除锁、退避时间与 `last_error`，由下一轮 `RUN_DUE_EXPORT_JOBS` 重新领取（中转数据保留，续跑前自动校验行数）。

## 请求

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quote/DeDup/Job/Retry`
- **Query 参数**：无
- **请求体**（snake_case）：

```json
{"job_id": 128}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `job_id` | int64 | 是 | 导出任务主键，必须为正整数 |

### 成功（HTTP 200）

```json
{"code": 0, "message": "已重置为 WAITING", "data": {"job_id": 128}}
```

### 错误

| `code` | HTTP | 含义 |
|---|---|---|
| `400` | 400 | 请求体格式错误或 `job_id` 非正整数 |
| `409` | 409 | 任务不存在或状态不允许重试（仅 `FAILED`/`PAUSED`/`FROZEN`/`WAITING` 可重置） |

## 已使用位置登记

- 运维排查（暂无前端菜单）
