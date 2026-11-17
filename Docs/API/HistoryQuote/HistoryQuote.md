# GET /API/V1/Quote/Query — 历史行情查询

按**证券代码 + 交易日**查询分钟级 K 线数据（周期目前仅支持 1 分钟 `Min`，其他周期暂不支持）。支持 **1 日 / 5 日多交易日回溯**与**分页翻页**。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Quote/Query`

### Query 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `secu_code` | string | ✅ | 证券代码（如 `NVDA` / `518880`） |
| `secu_name` | string | 可选 | 证券名称（如 `英伟达`）；前端从 finv_security 字典选中后回传，服务端原样回显，便于确认证券 |
| `date` | int | ✅ | 交易日期（`yyyymmdd`，如 `20260803`）；多日查询时作为**回溯截止日** |
| `period` | string | 可选 | 周期，目前仅支持 `Min`（1 分钟，默认） |
| `days` | int | 可选 | 回溯最近 N 个交易日（默认 `1`；`5` = 5 日分钟级 K 线；上限 `10`）。取该证券代码 `date` 当天及之前最近 N 个**有数据**的交易日 |
| `page` | int | 可选 | 页码（从 1 开始，默认 `1`） |
| `page_size` | int | 可选 | 每页条数（默认 `240`，即全天约 4 小时交易时段的分钟线数；上限 `1000`） |

## 响应

### 成功（HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "secu_code": "NVDA",
    "secu_name": "英伟达",
    "date": 20260803,
    "period": "Min",
    "days": 5,
    "page": 1,
    "page_size": 240,
    "total": 1150,
    "count": 240,
    "bars": [
      {
        "ts": 1754398800,
        "date": 20260730,
        "time": 93000,
        "open": "213.6700",
        "high": "213.8000",
        "low": "213.5000",
        "close": "213.7500",
        "volume": 275700,
        "turnover": "70010000000",
        "change": "0.0890",
        "change_pct": "0.0417",
        "remark": null
      }
    ]
  }
}
```

| data 字段 | 说明 |
|-----------|------|
| `secu_code` | 查询的证券代码（原样回显） |
| `secu_name` | 证券名称（前端回传时原样回显，可为空） |
| `days` | 实际回溯交易日数（1 或 5） |
| `page` / `page_size` | 当前页码 / 每页条数 |
| `total` | 满足条件的总条数（前端用于计算总页数） |
| `count` | 当前页返回条数 |

| bars 字段 | 说明 |
|-----------|------|
| `ts` | UTC 时间戳（秒，主键列，如 `1754398800`） |
| `date` | 交易日期（yyyymmdd，多日查询时用于区分每日分段） |
| `time` | 交易时间（hhmmss） |
| `open` / `high` / `low` / `close` | 开 / 高 / 低 / 收（数值字符串；缺失为 null） |
| `volume` | 成交量（缺失为 null，前端有值才展示） |
| `turnover` | 成交额（缺失为 null，前端有值才展示） |
| `change` | 涨跌额 = close - prev_close（缺失为 null） |
| `change_pct` | 涨跌幅 %（缺失为 null） |
| `remark` | 导入备注（可为 null） |

### 失败

| 场景 | HTTP | code |
|------|------|------|
| `secu_code` 为空 / `date` 非法 | 400 | 4001 |
| `period` 暂不支持 | 500 | 2006 |
| 服务端内部错误 | 500 | 2006 |

## 示例

```bash
# 查询单日（1 分钟 K 线）
curl "http://localhost:16001/API/V1/Quote/Query?secu_code=NVDA&date=20260803&period=Min"

# 查询 5 日分钟级 K 线（截至 20260803 的最近 5 个交易日，第 1 页）
curl "http://localhost:16001/API/V1/Quote/Query?secu_code=NVDA&date=20260803&period=Min&days=5&page=1&page_size=240"

# 翻页（第 3 页）
curl "http://localhost:16001/API/V1/Quote/Query?secu_code=NVDA&date=20260803&period=Min&days=5&page=3&page_size=240"
```

## 说明

- 多日查询时按 `date ASC, ts ASC` 排序返回，`total` 为回溯 N 个交易日内的总分钟线条数，前端据此分页。
- 涨跌额/涨跌幅由 `close` 与 `prev_close` 计算得出（`change = close - prev_close`）。
- 前端 K 线展示约定：红涨绿跌（参考富途牛牛样式，含 MA5/MA10/MA20/MA30 均线）；悬停显示时间点 OHLC 及有值的成交量/成交额/涨跌额/涨跌幅。

## 已使用位置登记

| 使用方 | 位置 | 说明 |
|--------|------|------|
| 历史行情查询菜单 | [`Docs/Menu/HistoryQuote/HistoryQuoteQuery.md`](../../Menu/HistoryQuote/HistoryQuoteQuery.md) | 前端 `QuoteQueryView.vue` 调用本接口获取分钟级 K 线（1 日 / 5 日、分页） |
