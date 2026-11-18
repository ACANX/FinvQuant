# POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Index/Fav/Snapshot/Sync — 收藏的美股/港股指数行情快照采集

> 所属：行情 → 实时行情 → 行情快照同步 →「指数」Tab（`instrument_symbol=index`，需求 11333）
> 数据表：`cdeu.finv_quote_secu_realtime`（跨 provider 实时快照表，主键 `usc`）
> 数据来源：`quote.finv_quote_secu_fav`（收藏表）→ moomoo OpenAPI `POST /api/v1.0/quote/snapshot`
> 实现：`internal/api/handler/cdeu_index_snapshot.go`、
> `internal/meta/finv/quote/cdeu/index_fav_snapshot_sync.go`（采集编排与字段转换）、
> `internal/meta/component/quote/moomoo/webapi/`（OpenAPI 客户端：签名 / 快照）

## 1. 概述

把「规范证券信息维护」页收藏的**美股 / 港股指数**（`quote.finv_quote_secu_fav` 中
`security_type ∈ {StockIndex, Index}` 且 `exchange_flag ∈ {HK, US}` 的 `usc`）批量拉取实时快照，
按 `usc` **upsert**（有值才覆盖）到 `cdeu.finv_quote_secu_realtime`。

- 同一采集器也挂在「行情快照同步 → 指数」Tab：`POST .../FTStockQuoteSnapshotSync/Run`
  传 `instrument_symbol=index` 即可触发，复用运行状态 / 逐批日志 / 轮询接口；
  若配置表 `cdeu.finv_cdeu_secu_realtime_sync_config` 里存在**启用的 `index` 行**，
  则仍走通用列表采集（配置行优先，不影响既有用法）。
- 凭据复用「加密货币」Tab 的 moomoo 配置（`sys.sys_config` 20016），无需重复填写。

## 2. 请求

| 项 | 值 |
|---|---|
| 方法 | `POST` |
| 路径 | `/API/V1/Meta/Finv/Quote/Cdeu/Realtime/Index/Fav/Snapshot/Sync` |
| 认证 | `Authorization: Bearer <token>`（本系统登录态） |

请求体可选：

| 字段 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `timeout_ms` | int | 60000 | 本次采集总超时 |

单批 code 数**固定为 400**（moomoo 快照接口上限），不提供 `batch_size` 参数。

## 3. moomoo code 规则（实测 2026-11-18）

| 交易所 | code 形式 | 示例 | 说明 |
|---|---|---|---|
| 港股 `HK` | `HK.<security_code>` | `HK.800000`（恒生指数） | 必须用官方数字代码 |
| 美股 `US` | `US..<security_code>` | `US..DJI` / `US..NDX` / `US..SPX` / `US..VIX` | **双点**前缀，指数专用 |

实测结论：

| code | 结果 |
|---|---|
| `HK.800000` | 命中 `Hang Seng Index` |
| `HK.800700` | 命中 `Hang Seng TECH Index` |
| `HK.HSI` / `HK.HSTECH` / `HK.HSIII` | 不命中（别名不支持） |
| `US..DJI` / `US..IXIC` / `US..NDX` / `US..SPX` / `US..VIX` | 命中对应美股指数 |
| `US.DJI` / `US.NDX` / `US.SPX` | 不命中（单点形态查不到） |
| `US.DJIA` | 命中，但返回的是 `GLOBAL X DOW 30® COVERED CALL ETF`（同码 ETF，不是指数） |

请求了但上游未返回的 code 只记入 `Missing`（多为上游未覆盖的长尾指数），**不报错、不写库**。

## 4. 响应

```json
{
  "code": 0,
  "message": "采集完成",
  "data": {
    "USCTotal": 5,
    "CodesRequested": 5,
    "RowsReceived": 4,
    "RowsInsert": 4,
    "RowsUpdate": 0,
    "Batches": 1,
    "Missing": ["HK.HSIII"],
    "ElapsedMs": 1774
  }
}
```

| 字段 | 说明 |
|---|---|
| `USCTotal` | 收藏表里的美股/港股指数数量 |
| `CodesRequested` | 去重后实际请求的 code 数 |
| `RowsReceived` | 上游返回的快照行数 |
| `RowsInsert` | 新增行数 |
| `RowsUpdate` | 覆盖行数 |
| `Batches` | 请求批次数 |
| `Missing` | 请求了但上游未返回的 code |
| `ElapsedMs` | 总耗时 |

## 5. 字段映射与类型转换

| 目标列 | 来源 | 说明 |
|---|---|---|
| `usc` | 收藏表 `usc` | 主键；**按请求 code 反查**，不从返回 code 反推 |
| `name` | 快照 `name` | 空时回落 `usc` |
| `stock_code` | `finv_security.security_code` | 港股 `800000`、美股 `DJI` |
| `market_type` | 交易所推导 | 港股 `1`、美股 `2`（与表内现有数据同口径） |
| `market_label` | 交易所推导 | `HK` / `US` |
| `instrument_type` | 固定 | `6`（指数，富途字典） |
| `currency` | `finv_security.currency_type` | 缺省港股 `HKD`、美股 `USD` |
| `price_nominal` | 快照 `last_price` | `last_price = 0` 时不写行情列 |
| `change_price` | `last_price − prev_close_price` | 按 6 位小数四舍五入（消除浮点尾差） |
| `change_ratio` | 上式 / `prev_close_price` | **小数比例**（`-0.009079` = `-0.91%`），8 位小数 |
| `price_amplitude` | `amplitude / 100` | 上游是百分数，表内是小数比例 |
| `volumn_ratio` | `volume_ratio` | 倍数，不除 100（与 ZXET 同口径） |
| `buysell_ratio` | `bid_ask_ratio / 100` | 表内是小数比例 |
| `trade_volumn` | `volume` | 股 |
| `trade_trunover` | `turnover` | 成交额 |
| `dt_update` | 采集时刻 | Unix 毫秒 |
| `provider` | 固定 | `MMAPI` |
| `remark` | 生成 | 来源、moomoo code、交易所与计算口径 |

其余列无来源，写 `NULL`；落库语句是 **`COALESCE(EXCLUDED.col, t.col)` 有值才覆盖**，
因此不会把库里已有的 `rmb_*`、`market_value` 等列清空（与 BDFS 采集同口径，需求 11287）。

## 6. 凭据（优先 `sys_config`，其次环境变量；**不写日志**）

| 来源 | 配置项 | 说明 |
|---|---|---|
| `sys.sys_config` 20016 | `AppKey\|私钥 PEM`（单配置项） | 页面「加密货币」/「指数」Tab 的凭据卡片维护（需求 11318 / 11319），两 Tab 共用 |
| 环境变量 `FINV_MOOMOO_APP_KEY` | AppKey | sys_config 未配置时兜底 |
| 环境变量 `FINV_MOOMOO_PRIVATE_KEY` | 私钥 PEM（可含字面 `\n`） | sys_config 未配置时兜底 |
| 环境变量 `FINV_MOOMOO_BASE_URL` | 站点地址 | 可选，默认 `https://webapi.moomoo.com` |

两处都没有配置时接口返回 `503`（`code=2006`）；只配了 AppKey 未配私钥时同样 503，并明确提示缺私钥。

## 7. 失败场景

| 场景 | HTTP | 说明 |
|---|---|---|
| 未配置凭据 / 缺私钥 | 503 | 提示配置 `sys_config` 20016 或环境变量 |
| 私钥格式错误 | 500 | PKCS#8 / Ed25519 解析失败 |
| 上游 `ret_code != 0` | 500 | 错误消息带 `ret_code` 与 `ret_msg`（整批失败，不写半批） |
| 写库失败 | 500 | `cdeu.finv_quote_secu_realtime` 写入错误 |

## 8. 实测记录（2026-11-18，本机）

| 轮次 | 收藏指数 | codes | 返回行 | 新增 | 覆盖 | 未返回 | 耗时 |
|---|---|---|---|---|---|---|---|
| 第 1 轮 | 5（HSI / HSIII / HTI / DJI / SPX） | 5 | 4 | 4 | 0 | `HK.HSIII` | 1774 ms |
| 第 2 轮（幂等复跑） | 5 | 5 | 4 | 0 | 4 | `HK.HSIII` | 2049 ms |

落库抽查（`provider='MMAPI'`）：

| usc | market_type | market_label | instrument_type | currency | price_nominal | change_price | change_ratio |
|---|---|---|---|---|---|---|---|
| `HSI` | `1` | `HK` | `6` | `HKD` | 23294.75 | -213.42 | -0.009079 |
| `HTI` | `1` | `HK` | `6` | `HKD` | 3736.33 | -21.84 | -0.005811 |
| `DJI` | `2` | `US` | `6` | `USD` | 50759.42 | 保留库内原值 | 保留库内原值 |
| `SPX` | `2` | `US` | `6` | `USD` | 7340.53 | 保留库内原值 | 保留库内原值 |

> 美股指数在非交易时段上游不返回 `prev_close_price`，此时 `change_price` / `change_ratio` 写 `NULL`，
> 因「有值才覆盖」而保留库里原值（不会写成 0），符合预期。
