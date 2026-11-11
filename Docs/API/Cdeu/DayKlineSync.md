# 日K线数据同步 API（Sync）

> 所属：行情 → 收剔导传ETL → 日K线数据同步
> 路径：`POST /API/V1/Meta/Finv/Quote/Cdeu/DayKline/Sync`
> 数据表：写入 `cdeu.finv_quote_secu_kline_day`（读取参数 `futu.finv_futu_quote_collect`）
> 关联需求：10952（同步能力 10951，建表 10950）
> 上游接口：moomoo `GET /quote-api/quote-v2/get-kline`（厂商文档 1108-Kline.md）

## 0. 用途

按 `usc` 从 moomoo 网页行情接口拉取**全量历史日K**，按 `(date, usc)` upsert 到
`cdeu.finv_quote_secu_kline_day`。用于给回测/统计准备日线数据，替代临时从分钟线聚合。

- 请求参数不手填：`stock_id` / `market_type` / `market_code` / `instrument_type` /
  `sub_instrument_type` 一律取自 `futu.finv_futu_quote_collect`（仅 `flag_enable='1'`），
  原样透传（同一编号在 moomoo 不同接口含义不同，不做语义换算）。
- 上游返回**全量历史**（样本证券约 1.2 万根日K），无分页、无时间范围参数；
  区间截取在服务端按 `from` / `to` 本地过滤。

## 1. 请求

`POST /API/V1/Meta/Finv/Quote/Cdeu/DayKline/Sync`

请求体（JSON，snake_case）：

| 字段 | 类型 | 必填 | 说明 |
|------|------|:---:|------|
| `usc` | string | 是 | 证券唯一编码，如 `GCMain`；必须存在于 `futu.finv_futu_quote_collect` 且 `flag_enable='1'` |
| `from` | int | 否 | 起始交易日 `yyyyMMdd`；`0` 或省略表示不限 |
| `to` | int | 否 | 结束交易日 `yyyyMMdd`；`0` 或省略表示不限；`from > to` 返回 400 |

无 URL 查询参数。

## 2. 行为语义

1. 读取 `futu.finv_futu_quote_collect` 的参数；无启用记录 → HTTP 404。
2. 调用 moomoo `get-kline`（`type=2` 日K），携带 `quote-token` 签名与 csrf/cookie 头。
3. 日期换算：上游只给秒级时间戳，按 `cdeu.finv_quote_cdeu_usc.timezone` 换算成
   市场本地日期（缺失/非法回退 UTC）。跨夜盘严格按交易日历归属应另用日历校正，
   本接口只做本地日历日换算。
4. 区间过滤：`date` 落在 `[from, to]` 内才写入。
5. 涨跌幅：**不写**上游 `r` 字段（样本恒为 `null`、口径未验证），改按
   `change_price / last_close × 100` 计算百分数，与 `cdeu.finv_quote_secu_kline_min.change_ratio`
   口径一致。
6. 落库：每 2000 行一批 `INSERT ... ON CONFLICT (date, usc) DO UPDATE`；
   冲突时只覆盖本次有值的列（`COALESCE(EXCLUDED.x, t.x)`），
   不会抹掉其它来源已回填的 `open_interest` / `pe_ratio` 等列。
7. `provider` 固定写 `FTMM`（与存量分钟线统一使用同一供应商标识，不因取数通道不同而另起名字）。
8. `open_interest` / `hist_volatility` / `implied_volatility` / `pe_ratio` / `trade_count`
   本接口**不写**（上游日K接口不提供），保持 NULL 或已有值。

### 2.1 日志

服务端每步输出单行 KV 日志，统一前缀 `[cdeu][daykline]`：

```
[cdeu][daykline] api sync begin usc="GCMain" from=0 to=0 client=...
[cdeu][daykline] params usc=GCMain futu_symbol=GCmain stock_id=70000294 market_type=2 market_code=70 instrument_type=9 sub_instrument_type=9008 market_label=US instrument=futures
[cdeu][daykline] fetch start usc=GCMain stockId=70000294 marketType=2 type=2 marketCode=70 instrumentType=9 subInstrumentType=9008 tz=America/New_York
[cdeu][daykline] fetch done usc=GCMain bars=12789 fetch_elapsed_ms=...
[cdeu][daykline] filter usc=GCMain total=12789 in_range=12789 date_from=... date_to=... tz=America/New_York
[cdeu][daykline] upsert batch usc=GCMain offset=0 size=2000 inserted=2000 updated=0
[cdeu][daykline] sync done usc=GCMain total=12789 in_range=12789 inserted=... updated=... elapsed_ms=...
[cdeu][daykline] api sync end usc="GCMain" total=... in_range=... inserted=... updated=... elapsed_ms=...
```

失败时输出 `sync failed usc=... step=collect_params|fetch|upsert err=...`。

线上抓取：

```bash
docker logs <容器名> 2>&1 | grep '\[cdeu\]\[daykline\]'
```

## 3. 响应

```json
{
  "code": 0,
  "message": "同步完成",
  "data": {
    "usc": "GCMain",
    "timezone": "America/New_York",
    "total": 12789,
    "in_range": 12789,
    "insert": 12000,
    "update": 789,
    "date_from": 20000104,
    "date_to": 20261102,
    "provider": "FTMM",
    "params": {
      "usc": "GCMain",
      "stock_id": 70000294,
      "futu_symbol": "GCmain",
      "market_type": 2,
      "market_code": 70,
      "instrument_type": 9,
      "sub_instrument_type": 9008,
      "req_section": "",
      "market_label": "US",
      "instrument": "futures",
      "flag_enable": "1"
    }
  }
}
```

`data` 字段说明：

| 字段 | 类型 | 说明 |
|------|------|------|
| `total` | int | 上游返回的日K根数（全量历史） |
| `in_range` | int | 落在 `[from, to]` 区间内的根数（实际 upsert 行数） |
| `insert` | int | 新增行数 |
| `update` | int | 更新行数 |
| `date_from` | int | 本次写入数据的起始日期（`yyyyMMdd`，无数据为 0） |
| `date_to` | int | 本次写入数据的结束日期（`yyyyMMdd`，无数据为 0） |
| `timezone` | string | 日期换算使用的市场时区 |
| `provider` | string | 写入 `provider` 列的值（固定 `FTMM`） |
| `params` | object | 实际使用的请求参数（排障用，便于确认是否与 `futu` 表一致） |

## 4. 错误码

| `code` | HTTP | 触发场景 |
|--------|------|----------|
| `400` | 400 | 请求体不是合法 JSON；`from > to` |
| `404` | 404 | `usc` 在 `futu.finv_futu_quote_collect` 中无 `flag_enable='1'` 记录 |
| `2006` | 500 | 上游请求失败（网络/`quote-token`/业务码非 0）、解析失败、写库失败 |

错误响应示例：

```json
{"code": 404, "message": "读取 futu 采集参数失败（usc=NOPE）: futu.finv_futu_quote_collect 中没有该 usc 的启用记录"}
```

## 5. 已使用位置登记

| 业务菜单 | 菜单文档 |
|----------|----------|
| 日K线数据同步（`/Meta/Finv/Quote/Cdeu/DayKline/Sync`，视图 `DayKlineSyncView.vue`） | [Docs/Menu/Quote/Cdeu/DayKlineSync.md](../../Menu/Quote/Cdeu/DayKlineSync.md) |
