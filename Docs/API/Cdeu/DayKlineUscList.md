# 日K线候选 USC 列表 API（UscList）

> 所属：行情 → 收剔导传ETL → 日K线数据同步
> 路径：`GET /API/V1/Meta/Finv/Quote/Cdeu/DayKline/UscList`
> 数据表：`futu.finv_futu_quote_collect`
> 关联需求：10952（同步能力 10951，建表 10950）

## 0. 用途

列出 `futu.finv_futu_quote_collect` 中**已启用**（`flag_enable='1'`）的证券，
供「日K线数据同步」页面的 `usc` 候选下拉使用（可手输，不强制从列表选）。

## 1. 请求

`GET /API/V1/Meta/Finv/Quote/Cdeu/DayKline/UscList?keyword=GC&limit=200`

查询参数（小驼峰）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|:---:|------|
| `keyword` | string | 否 | 匹配 `usc` 或 `futu_symbol`（ILIKE，不区分大小写） |
| `limit` | int | 否 | 返回条数上限，默认 200，服务端上限 2000；非整数返回 400 |

## 2. 响应

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 2,
    "list": [
      {"usc": "GCMain", "futu_symbol": "GCmain", "quote_market": "US", "type_symbol": "futures"},
      {"usc": "MGCMain", "futu_symbol": "MGCmain", "quote_market": "US", "type_symbol": "futures"}
    ]
  }
}
```

## 3. 错误码

| `code` | HTTP | 触发场景 |
|--------|------|----------|
| `400` | 400 | `limit` 不是整数 |
| `2006` | 500 | 查询失败（数据库错误） |

## 4. 已使用位置登记

| 业务菜单 | 菜单文档 |
|----------|----------|
| 日K线数据同步（`/Meta/Finv/Quote/Cdeu/DayKline/Sync`，视图 `DayKlineSyncView.vue`） | [Docs/Menu/Quote/Cdeu/DayKlineSync.md](../../Menu/Quote/Cdeu/DayKlineSync.md) |
