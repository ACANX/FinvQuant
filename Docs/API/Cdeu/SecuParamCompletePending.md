# 证券参数补全：待补清单（SecuParamCompletePending）

> 接口：`GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Secu/Param/Complete/Pending`
> 需求：11161
> 权限：需登录（内部接口）
> 关联菜单：[`Menu/Quote/Realtime/SecuParamComplete.md`](../../../Menu/Quote/Realtime/SecuParamComplete.md)
> 关联接口：[`Cdeu/SecuParamCompletePrefill.md`](SecuParamCompletePrefill.md)（预填）、
> [`Cdeu/SecuParamComplete.md`](SecuParamComplete.md)（写入）

## 1. 用途

「证券参数补全」菜单的列表接口。按 `target` 参数查某个目标表的缺口：

| `target` | 目标表 | 含义 |
|----------|--------|------|
| `security` | `public.finv_security` | 规范证券参数还没登记的证券 |
| `collect` | `futu.finv_futu_quote_collect` | 采集参数还没配置的证券 |

源表是快照 `cdeu.finv_cdeu_secu_realtime_stock`（采集写入的全量在售证券，一行一证券）。
只读，不写任何表。

## 2. 判定口径

```sql
NOT EXISTS (
  SELECT 1 FROM <目标表> x
   WHERE x.usc = COALESCE(NULLIF(upper(btrim(t.usc)), ''), upper(btrim(t.market_label) || '.' || btrim(t.stock_code)))
      OR x.usc = upper(btrim(t.market_label) || '.' || btrim(t.stock_code))
)
```

| 概念 | 表达式 | 为什么 |
|------|--------|--------|
| 目标键 | `COALESCE(NULLIF(upper(btrim(t.usc)), ''), 派生键)` | `usc` 是「统一证券代码」，不是「市场.代码」的机械拼接（库里还有 `AAVE.FTASB`、`SHCI` 这类取值）；快照 `usc` 与两张主数据表口径一致 |
| 派生键 | `upper(btrim(market_label) \|\| '.' \|\| btrim(stock_code))` | 仅当快照 `usc` 为空时兜底 |
| 存在性判定 | 目标键 **或** 派生键任一命中即「已存在」 | 快照 `usc` 是裸代码（`STX`）而目标表已有 `US.STX` 时，不能当成缺失，否则一补就撞 `(exchange_code, security_code)` / `(market_code, futu_symbol)` 唯一约束 |

清单、预填、写入三处用**同一个**目标键表达式，保证「补完就离开清单」。

## 3. 请求参数

| 参数 | 必填 | 说明 |
|------|:----:|------|
| `target` | 是 | `security` 或 `collect`（大小写不敏感）；其它值返 400 |
| `keyword` | 否 | 匹配 `usc` / `stock_code` / `name` / 派生键（可直接搜 `US.PLD`） |
| `marketLabel` | 否 | 市场标签，空或 `all` 表示不限 |
| `orderBy` | 否 | 白名单：`market_value`（默认）/ `trade_trunover` / `change_ratio` / `stock_id` / `dt_update` |
| `order` | 否 | `desc`（默认）/ `asc` |
| `page` | 否 | 默认 1，范围 `1..1000000` |
| `pageSize` | 否 | 默认 50，范围 `1..500` |

## 4. 响应

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "target": "security",
    "total": 635,
    "page": 1,
    "pageSize": 50,
    "list": [
      {
        "stock_id": 202096,
        "usc": "PLD",
        "ftmm_symbol": "US.PLD",
        "name": "安博",
        "stock_code": "PLD",
        "market_label": "US",
        "instrument_type": "3",
        "currency": "USD",
        "market_value": "120034567890.000000",
        "dt_update": 1735689600000
      }
    ]
  }
}
```

| 字段 | 说明 |
|------|------|
| `target` | 回填请求的 target |
| `total` | 该目标表的待补总数 |
| `list[].usc` | 快照原值（可能为空）；补全写入时为空则用 `ftmm_symbol` 兜底 |
| `list[].ftmm_symbol` | 派生键 `upper(市场标签.代码)`，大写 |
| 其余 | 与「证券行情排行」同一套快照字段口径；数值列为**文本**，保住 `NUMERIC(24,6)` 精度 |

## 5. 错误码

| HTTP | code | 场景 |
|------|------|------|
| 200 | 0 | 查询完成（可能 `total=0`） |
| 400 | 400 | `target` 缺失或非法、分页参数越界、`order` 非法、`orderBy` 不在白名单 |
| 500 | 2006 | 数据库侧失败 |

## 6. 相关文档

- [`Cdeu/SecuParamCompletePrefill.md`](SecuParamCompletePrefill.md) — 单表预填
- [`Cdeu/SecuParamComplete.md`](SecuParamComplete.md) — 单表写入
- [`Menu/Quote/Realtime/SecuParamComplete.md`](../../../Menu/Quote/Realtime/SecuParamComplete.md) — 页面说明
