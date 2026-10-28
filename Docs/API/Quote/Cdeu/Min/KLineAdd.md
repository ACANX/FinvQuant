# POST /API/V1/Meta/Finv/Quote/Cdeu/Min/Add — 单条插入

向分钟行情表插入一条 K 线；主键 `(usc, ts)` 冲突返回 409。

## 请求

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quote/Cdeu/Min/Add`
- **旧路径**：`/API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Add` 与 `/API/V1/Meta/Finv/Quote/Secu/KLine/Min/Add` **已不再注册**（需求 10782）
- **Content-Type**：`application/json`

### 请求体（snake_case）

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `usc` | string | ✅ | 证券唯一编码，1~255 |
| `ts` | int64 | ✅ | **秒级** Unix 时间戳（13 位毫秒默认拒绝，见 `FINV_KLINE_TS_UNIT`） |
| `date` | int | — | 交易日期 `yyyyMMdd`（19500101~21001231） |
| `time` | int | — | 交易时间 `HHmmss`（0~235959） |
| `open` / `high` / `low` / `close` | number | — | 价格 |
| `volume` / `turnover` | number | — | ≥0 |
| `change_price` / `change_ratio` / `pacd` | number | — | 涨跌额/涨跌幅/当日累计均价（库列 `paocd`） |
| `ext_field` / `remark` | string | — | 扩展字段/备注 |
| `provider` | string | — | 数据来源，≤255 |
| `last_close` | number | — | 上一收盘价；落库，参与涨跌计算，`open` 为空时回填 `open` |

```json
{"usc":"000001","ts":1792373400,"date":20261019,"time":93000,
 "open":11.5,"high":11.6,"low":11.4,"close":11.48,
 "volume":392600,"turnover":4507048,"change_price":-0.02,
 "change_ratio":-0.17,"provider":"FTMM"}
```


> **change_price / change_ratio 自动补全**：两列都未提供时，服务端按 `close` 与参考价计算（`change_price = close - ref`、`change_ratio = (close - ref) / ref * 100`）；参考价优先 `last_close`（API 不写该列，通常为空）→ `open`（非 0）→ 都不满足则保持 NULL。**只补全缺失值，任一列已有值时不做任何修改。**

## 响应

### 成功（HTTP 201）

返回整行对象（17 个业务字段，含 `last_close`；不含 `gmt_create`/`gmt_update`）；numeric 字段为 JSON 数字。

```json
{"usc":"000001","ts":1792373400,"date":20261019,"time":93000,
 "open":11.5,"high":11.6,"low":11.4,"close":11.48,
 "volume":392600,"turnover":4507048,"change_price":-0.02,
 "change_ratio":-0.17,"pacd":null,"ext_field":null,"remark":null,"provider":"FTMM"}
```

### 错误

错误体（超集，兼容旧客户端）：
```json
{"code":211101,"message":"记录已存在（主键冲突）","detail":"记录已存在（主键冲突）",
 "request_id":"B7C63349-EB36-49AB-B16A-E8E1BE475D3B",
 "error":{"code":211101,"catalog_version":"finvquant-1","retryable":false}}
```

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

> `request_id` 为大写 UUID4；`FINV_ERROR_DETAIL_MODE=admin` 且携带有效 `X-Debug-Token`（或来自内网 CIDR）时，额外返回白名单 `debug` 字段。

## 已使用位置登记

- 分钟行情采集端（原 FinvQuoteDataDeDup 调用方，逐步切换至 16001）
