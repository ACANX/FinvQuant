# GET /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Stats — 去重行情统计分析

按证券（`usc`）与交易日期范围统计 `dedup.finv_quote_secu_kline_min` 的去重后数据，返回汇总指标、每日趋势、证券分布与空值完整性，供前端“去重分钟行情统计”页面做可视化。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Stats`
- **旧前缀（过渡别名）**：`/API/V1/Meta/Finv/Quote/Secu/KLine/Min` + 相同子路径（切流期保留，P7 后可移除）
- **Content-Type**：`application/json`

### Query 参数（小驼峰）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `usc` | string | 否 | 证券代码，精确匹配；缺省统计全部证券 |
| `dateFrom` / `dateTo` | int | 否 | 交易日期区间（含），`yyyymmdd` |
| `tsFrom` / `tsTo` | int64 | 否 | 秒级时间戳区间（含），与日期条件同时生效（AND） |

> 参数校验与 `/Page` 一致：`dateFrom/dateTo` 取值 `19500101`~`21001231`，`tsFrom/tsTo` 不得为负。

### 成功（HTTP 200）

```json
{"code":1,"data":{
  "summary":{
    "rows":2804329,"usc_count":3,"date_from":20260806,"date_to":20260806,"date_count":1,
    "volume_sum":"1234567890.000000","turnover_sum":"98765432100.00000000",
    "close_min":"15.120000","close_max":"4077.000000","close_avg":"4012.345678",
    "null_counts":{"date":0,"open":0,"high":0,"low":0,"close":0,"last_close":1162160,
                   "change_price":0,"change_ratio":0,"volume":0,"turnover":0,"provider":0}
  },
  "daily":[{"date":20260806,"rows":2804329,"volume_sum":"...","turnover_sum":"...",
            "close_min":"...","close_max":"...","close_avg":"..."}],
  "top_usc":[{"usc":"GCMain","rows":2729329},{"usc":"XAUUSD","rows":60000}],
  "request_id":"B7C63349-EB36-49AB-B16A-E8E1BE475D3B","ts":1792695045},
 "message":"OK"}
```

字段说明：

| 字段 | 类型 | 说明 |
|---|---|---|
| `summary.rows` | int | 过滤范围内总行数 |
| `summary.usc_count` | int | 去重证券数 |
| `summary.date_from` / `date_to` | int/null | 交易日期最小/最大值（`date` 为空时为 null） |
| `summary.date_count` | int | 去重交易日数 |
| `summary.volume_sum` / `turnover_sum` | string/null | 成交量/成交额合计（numeric 文本，保留精度） |
| `summary.close_min` / `close_max` / `close_avg` | string/null | 收盘价最小/最大/均值（6 位小数） |
| `summary.null_counts.*` | int | 对应列的空值行数（`date/open/.../provider`） |
| `daily[]` | array | 按交易日期升序聚合：记录数、成交量/额合计、收盘价最小/最大/均值 |
| `top_usc[]` | array | 按记录数倒序的证券 Top 20（`usc` + `rows`） |
| `request_id` | string | 大写 UUID4（DC-07） |
| `ts` | int | 服务端秒级时间 |

> 只读事务内完成三组查询，保证 `summary`、`daily`、`top_usc` 口径一致。
> `daily` 仅包含 `date` 非空的行；`date` 为空的行计入 `summary.null_counts.date`。

### 错误

| `211003` | 422 | 字段类型不匹配 |
| `211004` | 422 | 字段取值超出范围 |
| `211301` | 500 | 数据库错误 |
| `211302` | 503 | 数据库不可用 |
| `400/404/409/413/422/503` | 同 HTTP | 兜底（复用原 HTTP 错误码） |

## 已使用位置登记

- 去重分钟行情统计（`Web/src/views/Meta/Finv/Quote/DeDup/DeDupKLineStatsView.vue`）
