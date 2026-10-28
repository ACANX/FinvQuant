# GET /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Bounds — 交易日期边界

返回过滤范围内的最小/最大交易日期（轻量端点，供前端初始化默认日期范围使用）。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Bounds`
- **旧前缀（过渡别名）**：`/API/V1/Meta/Finv/Quote/Secu/KLine/Min` + 相同子路径
- **Query 参数**：与 `/Page` 一致（`usc`、`dateFrom/dateTo`、`tsFrom/tsTo`），通常只传 `usc`。

### 成功（HTTP 200）

```json
{"code":1,"data":{"date_from":20181108,"date_to":20260721,
  "request_id":"D1D169C3-7E8D-4FCD-93B3-CA213E069626","ts":1792827191},"message":"OK"}
```

- `date_from` / `date_to`：`min(date)` / `max(date)`，无数据时为 `null`；
- 依赖 `(usc, date, ts)` 索引，实测 `usc=GCMain` 约 17ms（对比 `/Stats` 全量聚合约 8s）。

### 错误

| 错误码 | HTTP | 说明 |
|---|---|---|
| `211003` | 422 | 字段类型不匹配 |
| `211004` | 422 | 字段取值超出范围 |
| `211301` | 500 | 数据库错误 |
| `211302` | 503 | 数据库不可用 |
| `400/404/409/413/422/503` | 同 HTTP | 兜底 |

## 已使用位置登记

- 去重分钟行情查询/统计页默认日期范围初始化（`DeDupKLineQueryView.vue`、`DeDupKLineStatsView.vue`）
