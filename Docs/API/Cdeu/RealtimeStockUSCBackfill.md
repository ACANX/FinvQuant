# 待录入证券手动补录 usc（RealtimeStockUSCBackfill）

> 接口：`POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/USCBackfill`
> 需求：11146
> 权限：需登录（内部接口）
> 关联菜单：[`Menu/Quote/Realtime/PendingSecu.md`](../../../Menu/Quote/Realtime/PendingSecu.md)（按钮「手动补录USC」）
> 关联接口：[`Cdeu/RealtimeStockPendingPage.md`](RealtimeStockPendingPage.md)（待录入清单，只读判定）

## 1. 用途

把 `cdeu.finv_cdeu_secu_realtime_stock` 里 **`usc` 为空**（`NULL` 或空白串）的快照行**就地补上**：

1. 派生键 `ftmm_symbol = upper(btrim(market_label) || '.' || btrim(stock_code))`（英文字母统一大写，需求 11143）；
2. `JOIN futu.finv_futu_secu_key` 按 `ftmm_symbol` 取 `usc`；
3. 只写「快照 usc 为空 **且** 键表 usc 非空」的行 —— 键表里查不到、或键表 usc 也是空的，**保持不动**（仍是待录入）。

与待录入清单的关系：清单接口是**查询时只读判定**（JOIN 现算，不落库），所以键表命中但快照仍为空的行会「从清单消失但库里还是 NULL」；
本接口负责把它们真正落库。两者口径完全一致（同一个派生键表达式），补录后清单不会反而变长。

## 2. 幂等性

- 只更新 `NULLIF(btrim(usc), '') IS NULL` 的行，已补过的行不会被改写；
- 重复点击 `updated` 为 `0`，`null_before` 也只统计当前仍为空的行；
- `gmt_update` 由表上触发器 `vq_set_gmt_update` 自动刷新，脚本不显式赋值。

## 3. 请求

无请求体、无查询参数（`{}` 即可）。前端按 `POST` + 空对象发送，超时给到 120 s。

## 4. 响应

```json
{
  "code": 0,
  "message": "usc 补录完成",
  "data": {
    "null_before": 1151,
    "updated": 940,
    "unresolved": 211,
    "elapsed_ms": 268
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `null_before` | integer | 补录前 `usc` 为空（NULL / 空白）的行数 |
| `updated` | integer | 本次真正写入 `usc` 的行数 |
| `unresolved` | integer | 补录后仍为空的行数（= `null_before - updated`，键表里没有对应证券） |
| `elapsed_ms` | integer | 耗时（毫秒） |

## 5. 行为与实现

单条 SQL（`src` 取可补录集、`upd` 实际写入并计数），不自带事务控制以外的锁；键表以 `ftmm_symbol` 为主键，匹配必为一行，不会产生重复更新。
前端在成功后刷新清单，并提示「本次补录 N 行 / 补录前为空 M 行 / 仍无法补全 K 行」。

## 6. 错误码

| HTTP | code | 场景 |
|------|------|------|
| 200 | 0 | 补录完成（含 0 行的情形） |
| 500 | 2006 | 数据库侧失败（消息带原始错误） |

## 7. 相关文档

- [`Cdeu/RealtimeStockPendingPage.md`](RealtimeStockPendingPage.md) — 待录入清单（只读判定）
- [`Menu/Quote/Realtime/PendingSecu.md`](../../../Menu/Quote/Realtime/PendingSecu.md) — 页面说明
- [`DevSpec/DbSchemaSpec.md`](../../DevSpec/DbSchemaSpec.md) — 表归属与 `futu` schema 约定
