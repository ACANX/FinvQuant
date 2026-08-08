# GET /API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery — 历史行情查询

按**证券代码 + 时间范围**查询分钟级 K 线数据（周期目前仅支持 1 分钟 `Min`，其他周期暂不支持）。返回时间范围内**全部记录**（不分页），适配 A 股 / 港股 / 美股 / 24h 电子盘等单日数据量不同的市场（如 GCMain 全天约 2181 根分钟线）。

支持两种时间范围指定方式：
- **推荐**：`startTs` / `endTs`（UTC 秒，前端拖动窗口时直接调整）；
- **兼容**：`date` + `days`（服务端内部换算为 ts 范围）。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery`

### Query 参数（URL 查询参数遵循小驼峰规范，见 ApiSpec §3）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `secuCode` | string | ✅ | 证券代码（如 `GCMain` / `518880` / `NVDA`） |
| `secuName` | string | 可选 | 证券名称（如 `英伟达`）；前端从 finv_security 字典选中后回传，服务端原样回显，便于确认证券 |
| `startTs` | int | 二选一 | 查询起始时间（UTC 秒）；与 `endTs` 成对使用（闭区间） |
| `endTs` | int | 二选一 | 查询结束时间（UTC 秒）；与 `startTs` 成对使用（闭区间） |
| `date` | int | 二选一 | 交易日期（`yyyymmdd`）；回溯截止日，与 `days` 配合转 ts 范围 |
| `days` | int | 可选 | 回溯自然日数（配合 `date`，默认 `1`；如 `5` = 截止日往前 5 天连续范围；上限 `30`） |
| `period` | string | 可选 | 周期，目前仅支持 `Min`（1 分钟，默认） |

> `startTs`/`endTs` 与 `date`/`days` 二选一；同时提供时优先 `startTs`/`endTs`。
> 兼容旧 snake_case 参数（`secu_code`/`secu_name`/`start_ts`/`end_ts`），新调用方请使用小驼峰。

## 响应

### 成功（HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "secu_code": "GCMain",
    "secu_name": "",
    "start_ts": 1784160000,
    "end_ts": 1784246399,
    "period": "Min",
    "total": 2181,
    "count": 2181,
    "bars": [
      {
        "ts": 1784160000,
        "date": 20260716,
        "time": 0,
        "open": "4041.3000",
        "high": "4041.8000",
        "low": "4040.9000",
        "close": "4041.3000",
        "volume": 100,
        "turnover": "404130000",
        "change": null,
        "change_pct": null,
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
| `start_ts` / `end_ts` | 实际查询的 ts 范围（UTC 秒，闭区间） |
| `period` | 周期（Min） |
| `total` | 时间范围内总条数（= `count`，不分页） |
| `count` | 返回条数 |

| bars 字段 | 说明 |
|-----------|------|
| `ts` | UTC 时间戳（秒，主键列，如 `1784160000`） |
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
| `secu_code` 为空 / `date` 非法 / `start_ts`/`end_ts` 非法 | 400 | 4001 |
| `period` 暂不支持 | 500 | 2006 |
| 服务端内部错误 | 500 | 2006 |

## 示例

```bash
# 方式一：按 ts 范围（UTC 秒，单日 2026-07-16 全天）
curl "http://localhost:16001/API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery?secuCode=GCMain&period=Min&startTs=1784160000&endTs=1784246399"

# 方式二：按日期 + 回溯（单日，截止 2026-07-16）
curl "http://localhost:16001/API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery?secuCode=GCMain&date=20260716&period=Min"

# 方式二：按日期 + 回溯 5 日（截止 2026-07-16，往前 5 天连续范围）
curl "http://localhost:16001/API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery?secuCode=GCMain&date=20260716&period=Min&days=5"
```

## 说明

- 按 `ts ASC` 排序返回；`total` 为时间范围内总条数（不分页，避免固定 page_size 截断单日数据）。
- `date`+`days` 换算：结束 = `date` 当日 23:59:59（UTC），开始 = 结束日往前 `days-1` 天 00:00:00。
- 涨跌额/涨跌幅由 `close` 与 `prev_close` 计算得出（`change = close - prev_close`）。
- 前端 K 线展示约定：红涨绿跌（参考富途牛牛样式，含 MA5/MA10/MA20/MA30 均线与布林带）；滚轮 / 拖拽平移查看（dataZoom inside），不做分页翻页。

## 已使用位置登记

| 使用方 | 位置 | 说明 |
|--------|------|------|
| 历史行情查询菜单 | [`Docs/Menu/HistoryQuote/HistoryQuoteQuery.md`](../../Menu/HistoryQuote/HistoryQuoteQuery.md) | 前端 `HistoryQuoteQueryView.vue` 调用本接口获取分钟级 K 线（1 日 / 5 日、ts 范围、不分页、拖拽平移） |
