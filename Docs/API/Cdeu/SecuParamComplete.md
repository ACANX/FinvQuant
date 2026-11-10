# 证券参数补全：单表写入（SecuParamComplete）

> 接口：`POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Secu/Param/Complete`
> 需求：11161
> 权限：需登录（内部接口）
> 关联菜单：[`Menu/Quote/Realtime/SecuParamComplete.md`](../../../Menu/Quote/Realtime/SecuParamComplete.md)
> 关联接口：[`Cdeu/SecuParamCompletePending.md`](SecuParamCompletePending.md)（清单）、
> [`Cdeu/SecuParamCompletePrefill.md`](SecuParamCompletePrefill.md)（预填）

## 1. 用途

把一只证券写进 **`target` 指定的那一张** 参数表，单事务、只写一张：

| `target` | 写入表 | 主键冲突处理 |
|----------|--------|--------------|
| `security` | `public.finv_security` | `ON CONFLICT (usc) DO NOTHING`（已存在不覆盖） |
| `collect` | `futu.finv_futu_quote_collect` | 同上；`ftmm_symbol` 是生成列，不写 |

**不写**另一张表，**不回写**快照 `usc`（那是「[待采证券录入](../../../Menu/Quote/Realtime/PendingSecu.md)」
页「手动补录USC」/「入队」的职责）。

## 2. 请求

```json
{
  "target": "security",
  "stock_id": 202096,
  "remark": "人工核对无误",
  "security": {
    "exchange_code": 30,
    "security_type": "Stock",
    "security_code": "PLD",
    "security_name": "安博",
    "security_name_cn": "安博",
    "security_name_full": "安博",
    "currency_type": "USD",
    "timezone": "",
    "tz": ""
  },
  "collect": {}
}
```

| 字段 | 必填 | 说明 |
|------|:----:|------|
| `target` | 是 | `security` 或 `collect`；其它值返 400 |
| `stock_id` | 是 | 快照行主键；`usc` / `ftmm_symbol` 由服务端派生，**不采信请求体里的 usc** |
| `remark` | 否 | 人工备注；服务端拼成 `[AUTO-FILL] 待采证券补录 <时间>[；备注：<人工备注>]` |
| `security` | 视 target | `target=security` 时必填：`exchange_code`（**必须 >0**）/ `security_type` / `security_code` / `security_name` / `currency_type` |
| `collect` | 视 target | `target=collect` 时必填：`market_label` / `futu_symbol` / `market_type` / `market_code` / `instrument_type` / `instrument`（数值项**必须 >0**） |

**非目标表的那一组完全不看**（可传空对象 `{}`）。请求体不允许未知字段（`DisallowUnknownFields`）。

服务端兜底：`security_code` / `futu_symbol` 取快照 `stock_code`；`security_name_cn` 缺省取 `security_name`；
`market_label` 缺省取快照市场标签；`instrument` / `sub_instrument_type` 缺省按 `instrument_type` 推；
`flag_enable` 缺省 `'0'`。

## 3. 响应

```json
{
  "code": 0,
  "message": "补全完成",
  "data": {
    "target": "security",
    "usc": "PLD",
    "created": true,
    "remark": "[AUTO-FILL] 待采证券补录 2026-01-15T10:20:30+08:00；备注：人工核对无误"
  }
}
```

| 字段 | 说明 |
|------|------|
| `target` | 本次补全的表（规范化后） |
| `usc` | 实际写入的主键 = `COALESCE(快照 usc, upper(市场标签.代码))` |
| `created` | `true` 新插入；`false` 该 `usc` 已存在（未覆盖人工数据），HTTP 仍是 200 |
| `remark` | 实际写入的备注 |

## 4. 幂等与并发

- 重复提交同一只有两个结果：首次 `created=true`，之后 `created=false`（不再写）；
- 主键 `ON CONFLICT DO NOTHING` 保证并发安全，不会出现「后写覆盖前写」；
- 唯一约束冲突（`23505`，老库同一 交易所+代码 挂着旧口径 usc）会翻译成可读提示，
  **不自动改旧行主键**，需人工裁决。

## 5. 错误码

| HTTP | code | 场景 |
|------|------|------|
| 200 | 0 | 补全完成（含「已存在未覆盖」） |
| 400 | 400 | `target` 缺失/非法、`stock_id` 非法或不存在、必填缺失、唯一约束冲突 |
| 500 | 2006 | 数据库侧失败 |

必填校验的错误消息会**逐项列出到底哪一项不合格**（带实际值），例如：

```
实时行情快照入参非法: 采集配置缺少必填 —— market_code=0（必须 >0，同市场同品种没有参考行时需人工填）
实时行情快照入参非法: 证券主数据缺少必填 —— exchange_code=0（必须 >0）
```

## 6. 相关文档

- [`Cdeu/SecuParamCompletePending.md`](SecuParamCompletePending.md) — 待补清单
- [`Cdeu/SecuParamCompletePrefill.md`](SecuParamCompletePrefill.md) — 单表预填
- [`Cdeu/RealtimeStockPendingEnroll.md`](RealtimeStockPendingEnroll.md) — 旧的整体录入（一次写两张表，页面入口已迁出）
- [`Menu/Quote/Realtime/SecuParamComplete.md`](../../../Menu/Quote/Realtime/SecuParamComplete.md) — 页面说明
