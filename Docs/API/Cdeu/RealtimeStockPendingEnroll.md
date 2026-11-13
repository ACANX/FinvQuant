# 待录入证券录入证券元数据（RealtimeStockPendingEnroll）

> 接口：`POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enroll`
> 需求：10996（落地）、11147（自动化 + `remark` 溯源）
> 权限：需登录（内部接口）
> 关联菜单：[`Menu/Quote/Realtime/PendingSecu.md`](../../../Menu/Quote/Realtime/PendingSecu.md)
> 关联接口：[`Cdeu/RealtimeStockPendingPrefill.md`](RealtimeStockPendingPrefill.md)（预填）

## 1. 用途

把一只待录入证券一次性登记到两张主数据表，并把 `usc` 回写快照，让它离开待录入清单：

| 步骤 | 表 | 语义 |
|------|----|------|
| 1 | `public.finv_security` | 证券主数据 |
| 2 | `futu.finv_futu_quote_collect` | futu 采集配置（`ftmm_symbol` 生成列不写） |
| 3 | `cdeu.finv_cdeu_secu_realtime_stock` | 只把 `usc` 写回（`NULL` / 空白 才写） |

> `cdeu.finv_quote_cdeu_usc` **不录入**（需求 11147）：该表数据由运行时采集链路动态生成。

单事务；三处写入全部按主键 `ON CONFLICT DO NOTHING` / 只填空值，**幂等且不覆盖人工数据**。

## 2. 请求

```json
{
  "stock_id": 79083232831247,
  "remark": "人工核对无误",
  "security": {
    "exchange_code": 21,
    "security_type": "Stock",
    "security_code": "09999",
    "security_name": "NTES-S",
    "security_name_cn": "网易-S",
    "security_name_full": "NTES-S",
    "currency_type": "HKD",
    "timezone": "",
    "tz": ""
  },
  "collect": {
    "futu_symbol": "09999",
    "market_type": 1,
    "market_code": 1,
    "instrument_type": 3,
    "sub_instrument_type": 3002,
    "market_label": "HK",
    "instrument": "stock",
    "req_section": "",
    "flag_enable": "0"
  }
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `stock_id` | 是 | 快照行主键；`usc` / `stock_id` / `ftmm_symbol` 一律由服务端派生，**不采信请求体里的 usc** |
| `remark` | 否 | 人工备注；服务端会拼成 `[AUTO-FILL] 待录入证券补录 <时间>[；备注：<人工备注>]` |
| `security.*` | 见下 | `security_name` / `currency_type` / `security_type` / `exchange_code` 必填（缺失返回 400） |
| `collect.*` | 见下 | `market_label` / `futu_symbol` / `market_type` / `market_code` / `instrument_type` / `instrument` 必填（缺失返回 400） |

服务端兜底：`security_code` / `futu_symbol` 取快照 `stock_code`；`security_name_cn` 缺省取 `security_name`；
`instrument` / `sub_instrument_type` 缺省按 `instrument_type` 推；`flag_enable` 缺省 `'0'`。

## 3. 响应

```json
{
  "code": 0,
  "message": "补录完成",
  "data": {
    "usc": "HK.ZZTEST1",
    "security_created": true,
    "collect_created": true,
    "snapshot_updated": true,
    "remark": "[AUTO-FILL] 待录入证券补录 2026-01-15T10:20:30+08:00；备注：人工核对无误"
  }
}
```

| 字段 | 说明 |
|------|------|
| `usc` | 服务端派生的键（`upper(市场标签.代码)`） |
| `security_created` | 本次是否新建证券主数据；已存在为 `false`（未覆盖） |
| `collect_created` | 本次是否新建采集配置；已存在为 `false`（未覆盖） |
| `snapshot_updated` | 是否回写了快照 `usc`；已有值时不改 |
| `remark` | 实际写入两张表 `remark` 的值 |

## 4. `remark` 溯源约定

- 固定前缀 **`[AUTO-FILL]`**，后接「待录入证券补录」与 RFC 3339 时间；
- 人工备注非空时追加 `；备注：<人工备注>`；
- 只在新插入时写入，`ON CONFLICT DO NOTHING` 不会覆盖既有 `remark`；
- 排查方式：`SELECT ... WHERE remark LIKE '[AUTO-FILL]%'` 即可列出全部由补录程序写入的行。

## 5. 错误码

| HTTP | code | 场景 |
|------|------|------|
| 200 | 0 | 录入完成（含三处都已存在的情形） |
| 400 | 400 | 请求体解析失败 / 必填缺失 / `stock_id` 不存在 / 唯一约束冲突 |
| 500 | 2006 | 数据库侧失败 |

唯一约束冲突（`23505`）会翻译成可读提示，例如
`public.finv_security 唯一约束冲突（Key (exchange_code, security_code)=(21, 09999) already exists）——同一 交易所/代码 或 市场/代码 已有其它 usc 的行，需人工确认后再录入`。

## 6. 相关文档

- [`Cdeu/RealtimeStockPendingPrefill.md`](RealtimeStockPendingPrefill.md) — 预填
- [`Cdeu/RealtimeStockPendingPage.md`](RealtimeStockPendingPage.md) — 待录入清单
- [`Menu/Quote/Realtime/PendingSecu.md`](../../../Menu/Quote/Realtime/PendingSecu.md) — 页面说明
