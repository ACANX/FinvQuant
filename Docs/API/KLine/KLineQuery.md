# POST /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Query — 批量查询

按 `(usc, ts)` / `uscs` / `provider` 组合查询（AND），返回完整字段。

## 请求

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Query`
- **旧前缀（过渡别名）**：`/API/V1/Meta/Finv/Quote/Secu/KLine/Min` + 相同子路径（切流期保留，P7 后可移除）
- **Content-Type**：`application/json`

### 请求体

```json
{"keys":[{"usc":"000001","ts":1792373400}],"uscs":["000002"],
 "provider":"FTMM","limit":50,"offset":0}
```

| 字段 | 约束 |
|---|---|
| `keys` | ≤ `FINV_KLINE_MAX_LIST_SIZE`，生成 `(usc, ts) IN (...)` |
| `uscs` | ≤ 同上，生成 `usc = ANY(...)` |
| `provider` | 精确匹配 |
| `limit` | 1~1000，默认 50 |
| `offset` | ≥0，默认 0 |

- `keys` / `uscs` / `provider` **至少一个**，否则 400（`211001`）。
- 结果按 `(usc, ts)` 升序，`count` 为满足条件总数。

### 成功（HTTP 200）

```json
{"count":1,"items":[{"usc":"000001","ts":1792373400,"date":20261019,"time":93000,
 "open":null,"high":null,"low":null,"close":11.48,"volume":392600,"turnover":4507048,
 "change_price":-0.02,"change_ratio":-0.17,"paocd":null,"ext_field":null,
 "remark":null,"provider":"FTMM"}]}
```

### 错误

| 错误码 | HTTP | 说明 |
|---|---|---|
| `211001` | 400 | 缺少必填参数 |
| `211002` | 400 | 请求体格式错误 |
| `211003` | 422 | 字段类型不匹配 |
| `211004` | 422 | 字段取值超出范围 |
| `211005` | 422 | 未知字段 |
| `211006` | 400 | 枚举值非法 |
| `211101` | 409 | 唯一键冲突 |
| `211102` | 409 | 请求内重复键 |
| `211103` | 404 | 记录不存在 |
| `211201` | 413 | 批量条数超过上限 |
| `211202` | 413 | 列表长度超过上限 |
| `211301` | 500 | 数据库错误 |
| `211302` | 503 | 数据库不可用 |
| `211303` | 422 | 数据库约束拒绝 |
| `400/404/409/413/422/503` | 同 HTTP | 兜底（复用原 HTTP 错误码） |

## 已使用位置登记

- 采集端按键回读校验
