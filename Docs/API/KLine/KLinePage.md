# GET /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Page — 条件查询 + 分页（对象数组）

按证券/来源/日期/时间戳过滤并分页，输出紧凑字段对象数组。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Page`
- **旧前缀（过渡别名）**：`/API/V1/Meta/Finv/Quote/Secu/KLine/Min` + 相同子路径（切流期保留，P7 后可移除）
- **Content-Type**：`application/json`

### Query 参数（小驼峰；旧下划线名兼容）

| 参数 | 类型 | 说明 |
|---|---|---|
| `usc` | string | 证券编码，精确定配 |
| `provider` | string | 数据来源，精确匹配 |
| `dateFrom` / `dateTo` | int | 交易日期区间（含） |
| `tsFrom` / `tsTo` | int64 | 秒级时间戳区间（含） |
| `limit` | int | 1~1000，默认 50 |
| `offset` | int | ≥0，默认 0 |

### 成功（HTTP 200）

```json
{"code":1,"data":{"cnt":2496418,"ps":50,"pn":1,
 "list":[{"usc":"000001","ts":1792373400,"date":20261019,"time":93000,
          "o":null,"h":null,"l":null,"c":11.48,"v":392600,"t":4507048,
          "cr":"-0.17","cp":-0.02,"pa":null,"pv":"FTMM","ext":null}],
 "request_id":"B7C63349-EB36-49AB-B16A-E8E1BE475D3B","ts":1792695045},
 "message":"OK"}
```

- 紧凑字段映射（顺序固定）：`o=open`、`h=high`、`l=low`、`c=close`、`v=volume`、`t=turnover`、`cr=change_ratio`、`cp=change_price`、`pa=paocd`、`pv=provider`、`ext=ext_field`。
- `request_id` 为大写 UUID4；成功与错误响应使用同一个值（DC-07）。
- `cr` 为**两位小数文本**（`ROUND_HALF_UP`，如 `"-0.00"`），其余 numeric 为 JSON 数字。
- `pn = offset/limit + 1`；`data.ts` 为服务端**秒级**时间。
- 列表端点**不输出** `remark`。

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

- 采集端/数据核对工具分页浏览
- 去重分钟行情查询（`Web/src/views/Meta/Finv/Quote/DeDup/DeDupKLineQueryView.vue`）
