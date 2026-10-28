# POST /API/V1/Meta/Finv/Quote/Cdeu/Min/Upsert — 批量 Upsert

批量写入；主键冲突时按 `onConflict` 跳过或整行覆盖。

## 请求

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quote/Cdeu/Min/Upsert`（独立于 K 线新/旧前缀，需求 10777）
- **旧路径**：`/API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Upsert` 与 `/API/V1/Meta/Finv/Quote/Secu/KLine/Min/Upsert` **已不再注册**
- **Content-Type**：`application/json`

### Query 参数（小驼峰）

| 参数 | 取值 | 默认 | 说明 |
|---|---|---|---|
| `onConflict` | `skip` / `update` | `update` | 冲突处理模式（旧名 `on_conflict` 兼容） |

### 请求体

同 [`KLineBatchInsert.md`](../../../KLine/KLineBatchInsert.md)。

### 行为

| 模式 | 冲突时 | `affected` 含义 |
|---|---|---|
| `skip` | `ON CONFLICT DO NOTHING` | 实际插入行数 |
| `update` | 覆盖全部业务列 + `gmt_update = now()` | 插入 + 覆盖行数 |

- `update` 为**整行覆盖**：请求中未提供的业务字段会写成 `NULL`；只改部分字段请用 `PATCH /Batch`。
- 请求内重复键（`update` 会触发「同一行被影响两次」）提前拦截为 409（`211102`）。

### 成功（HTTP 200）

```json
{"affected":100,"mode":"update"}
```

### 错误

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

- 分钟行情采集端增量回补
