# 待录入证券入队（RealtimeStockPendingEnqueue）

> 接口：`GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enqueue?stockId=`（预检，只读）
> 　　　`POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enqueue`（确认入队，写库）
> 需求：11153（入队）、11154（两步走 + 人工确认）
> 权限：需登录（内部接口）
> 关联菜单：[`Menu/Quote/Realtime/PendingSecu.md`](../../../Menu/Quote/Realtime/PendingSecu.md)（操作列「入队」按钮 + 入队确认对话框）
> 关联接口：[`Cdeu/RealtimeStockPendingPage.md`](RealtimeStockPendingPage.md)、[`Cdeu/RealtimeStockUSCBackfill.md`](RealtimeStockUSCBackfill.md)

## 1. 用途

「入队」= 把一只待录入证券的**证券键**登记进本地键表 `futu.finv_futu_secu_key`，让采集与
USC 映射链路有据可依。它**不写**任何主数据表（那是「录入」按钮的事），也不解析 `usc`。

### 1.1 两步走（人工确认，需求 11154）

| 步 | 动作 | 接口 | 说明 |
|:--:|------|------|------|
| 1 | 点列表行「入队」 | `GET .../Pending/Enqueue?stockId=` | 只打开确认对话框并预检，**不写库** |
| 2 | 对话框内点「确认入队」 | `POST .../Pending/Enqueue` | 人工核对（可改正 `instrument`）后才落库 |

> 刻意不做「点一下直接入队」：入队会新增键表记录，键表是下游采集/映射的输入，
> 自动化误写代价高（例如 `instrument` 推错会让 USC 规则匹配不到），因此保留人工确认环节。

## 2. 字段派生

| 键表字段 | 来源 |
|----------|------|
| `ftmm_symbol` | `upper(市场标签 \|\| '.' \|\| 代码)`（快照派生，主键，**不可覆盖**） |
| `market` | 快照 `market_label` |
| `code` | 快照 `stock_code` |
| `stock_id` | 快照 `stock_id` |
| `instrument` | 由 `instrument_type`（主）与 `market_type`（兜底）推导，**允许在对话框里改正** |
| `status` | 固定 `PENDING` |
| `flag_enable` | 固定 `1` |
| `priority` | 固定 `10` |
| `usc` | 留空（等「USC 映射规则」解析） |

### 2.1 `instrument` 推导（词表与键表一致）

| `instrument_type` | `instrument` |
|------------------:|--------------|
| 3 / 12 | `stock` |
| 4 | `etf` |
| 6 | `index` |
| 7 | `sector` |
| 9 | `futures` |
| 11 | `crypto` |
| 缺失（0）时按 `market_type` 兜底 | 11→`forex`、17→`crypto`、其余→`stock` |

> **注意**：键表用 `sector`（单数），采集表 `futu.finv_futu_quote_collect.instrument` 用 `sectors`（复数）。
> 写混了会让 USC 映射规则的 `instrument` 匹配失败。

## 3. GET 预检

### 3.1 请求

| 参数 | 位置 | 类型 | 必填 | 说明 |
|------|------|------|------|------|
| `stockId` | query | integer(int64) | 是 | 快照 `stock_id`，正整数 |

### 3.2 响应

```json
{
  "code": 0,
  "message": "预检完成",
  "data": {
    "stock_id": 990002001,
    "name": "QUEUE TEST",
    "ftmm_symbol": "HK.ZZQUEUE1",
    "market": "HK",
    "code": "ZZQUEUE1",
    "instrument_type": "3",
    "market_type": "1",
    "instrument": "stock",
    "existed": false,
    "status": "",
    "usc": "",
    "priority": 0,
    "writable": true,
    "hint": "键表尚无 HK.ZZQUEUE1，确认后写入 status=PENDING / flag_enable=1 / priority=10；入队后到「USC 映射规则」跑一次计算即可解析出 usc"
  }
}
```

| 字段 | 说明 |
|------|------|
| `existed` | 键表里已存在该 `ftmm_symbol` |
| `writable` | 是否允许提交（= `!existed`）；false 时对话框改为显示「去『USC 映射规则』」 |
| `status` / `usc` / `priority` | 已存在时的现值 |
| `hint` | 下一步说明（原文直接展示） |

## 4. POST 确认入队

### 4.1 请求

```json
{ "stock_id": 990002001, "instrument": "stock" }
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `stock_id` | 是 | 快照主键 |
| `instrument` | 否 | 覆盖派生值；空=用派生值；非词表取值直接 400 |

### 4.2 响应

```json
{
  "code": 0,
  "message": "入队完成",
  "data": {
    "ftmm_symbol": "HK.ZZQUEUE1",
    "market": "HK",
    "code": "ZZQUEUE1",
    "instrument": "stock",
    "stock_id": 990002001,
    "enqueued": true,
    "existed": false,
    "status": "PENDING",
    "usc": "",
    "priority": 10,
    "hint": "已入队 HK.ZZQUEUE1（instrument=stock）；到「USC 映射规则」跑一次计算后，本页可用「手动补录USC」完成同步"
  }
}
```

已存在时不插入，返回 `message="键表已存在该记录，未重复入队"`、`enqueued=false`、`existed=true`。

## 5. 幂等与并发

- 先按主键查一次，存在即返回（不写）；
- 插入用 `ON CONFLICT (ftmm_symbol) DO NOTHING RETURNING`，并发抢先插入时归为「已存在」而非报错；
- 重复点击「确认入队」不会产生第二条键记录。

## 6. 错误码

| HTTP | code | 场景 |
|------|------|------|
| 200 | 0 | 预检成功 / 入队成功 / 已存在未重复入队 |
| 400 | 400 | `stockId` 非法或快照行不存在 / `instrument` 不在词表内 / 请求体解析失败 |
| 500 | 2006 | 数据库侧失败 |

## 7. 入队之后

键表出现该 `ftmm_symbol` 后，`usc` 仍是空：

1. 去「USC 映射规则」跑「预演 → 应用」，规则引擎会按 P10/P20/P21/P40 等规则解析出 `usc`；
2. 回到本页点「手动补录USC」，`usc` 落到快照表，该行离开待录入清单；
3. 若规则也解不出（`ESCALATE`），需要补规则条目后再来一遍。
