# POST /API/V1/Meta/Finv/Quote/DeDup/File/Retry — 重试上传

把上传失败（`UPLOAD_FAILED`）的文件重置为 `PENDING_UPLOAD`，清零尝试次数并立即允许重试（`gmt_next_retry = now()`）。

## 请求

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quote/DeDup/File/Retry`
- **Query 参数**：无
- **请求体**（snake_case）：

```json
{"file_id": 512}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `file_id` | int64 | 是 | 文件登记主键，必须为正整数 |

### 成功（HTTP 200）

```json
{"code": 0, "message": "已重置为 PENDING_UPLOAD", "data": {"file_id": 512}}
```

### 错误

| `code` | HTTP | 含义 |
|---|---|---|
| `400` | 400 | 请求体格式错误或 `file_id` 非正整数 |
| `409` | 409 | 文件不存在或状态不是 `UPLOAD_FAILED` |

## 已使用位置登记

- 运维排查（暂无前端菜单）
