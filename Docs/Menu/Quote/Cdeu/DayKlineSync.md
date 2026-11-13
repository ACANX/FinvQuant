# 日K线数据同步（菜单文档）

> 所属：行情 → 收剔导传ETL → 日K线数据同步
> 路由：`/Meta/Finv/Quote/Cdeu/DayKline/Sync`
> 菜单 key：`cdeu-daykline-sync`；图标：`mdi-chart-candlestick`
> 视图组件：`Web/src/views/Meta/Finv/Quote/Cdeu/DayKlineSyncView.vue`
> 关联需求：10952（同步能力 10951，建表 10950）

## 1. 菜单入口

- 位置：左侧菜单 **行情 → 收剔导传ETL → 日K线数据同步**（紧邻「过时USC映射到新USC」）。
- 路由与菜单 key 见上表；菜单登记于 `Web/src/config/menuConfig.ts`，路由登记于 `Web/src/router.ts`。

## 2. 业务功能

| 功能 | 说明 |
|------|------|
| 选择 usc | 下拉候选来自 `futu.finv_futu_quote_collect` 中 `flag_enable='1'` 的记录，也可手输 |
| 日期区间（可选） | `起始日` / `结束日` 填 `yyyyMMdd`；留空表示不限（上游总是返回全量历史） |
| 同步日K | 调用后端拉取 moomoo 日K，按 `(date, usc)` upsert 到 `cdeu.finv_quote_secu_kline_day` |
| 查询存量 | 查看该 usc 已落库的行数、日期范围、来源 |
| 原始响应 | 折叠面板展示本次同步的完整 JSON，便于整体复制给后端排障 |

## 3. 使用方法

1. 进入菜单，在 `usc` 下拉里选择目标（如 `GCMain`）。
2. 需要限定区间时填写起止日（`yyyyMMdd`），否则留空。
3. 点「同步日K」，等待返回（全量历史 + 写库可能需要数秒到数十秒，页面超时上限 180s）。
4. 看右侧「本次同步结果」：`接口返回根数 / 区间内 / 新增 / 更新 / 日期范围 / provider`；
   左侧「存量统计」自动刷新。
5. 若失败，页面顶部红色提示会显示后端返回的错误消息；同时在服务端日志中按 `[cdeu][daykline]` 过滤。

## 4. 处理逻辑

前端（`DayKlineSyncView.vue`）：

- 页面加载：`GET .../DayKline/UscList?limit=500` 拉候选，`GET .../DayKline/Stat?usc=` 拉存量；
- 点「同步日K」：`POST .../DayKline/Sync`，请求体 `{usc, from, to}`（`from`/`to` 为空则传 0）；
  该请求单独放宽超时到 180s（默认 30s 不足以覆盖全量拉取 + upsert）；
- 成功后自动刷新存量统计，并把完整响应放进「原始响应」面板。

后端链路：

1. `futu.finv_futu_quote_collect`（`flag_enable='1'`）取五个定位参数，原样透传；
2. `GET https://www.moomoo.com/quote-api/quote-v2/get-kline`（`type=2` 日K，携带 `quote-token` 签名）；
3. 按 `cdeu.finv_quote_cdeu_usc.timezone` 把秒级时间戳换算为市场本地日期；
4. 按 `from`/`to` 过滤，每 2000 行一批 `INSERT ... ON CONFLICT (date, usc) DO UPDATE`；
5. `provider` 固定 `FTMM`；涨跌幅按 `change_price / last_close × 100` 计算（不写上游 `r`）。

## 5. 使用的后端接口

| 方法 | 路径 | 文档 |
|------|------|------|
| GET | `/API/V1/Meta/Finv/Quote/Cdeu/DayKline/UscList` | [Docs/API/Cdeu/DayKlineUscList.md](../../../API/Cdeu/DayKlineUscList.md) |
| POST | `/API/V1/Meta/Finv/Quote/Cdeu/DayKline/Sync` | [Docs/API/Cdeu/DayKlineSync.md](../../../API/Cdeu/DayKlineSync.md) |
| GET | `/API/V1/Meta/Finv/Quote/Cdeu/DayKline/Stat` | [Docs/API/Cdeu/DayKlineStat.md](../../../API/Cdeu/DayKlineStat.md) |

## 6. 注意事项

- **网络可达性**：服务端必须能访问 `https://www.moomoo.com`。开发机（宿主与容器）实测 TLS 握手被重置，
  只能在部署环境验证；失败时日志 `step=fetch err=...` 会给出具体原因。
- **全量历史**：上游一次返回全部历史（约 1.2 万根/证券），无分页；`from`/`to` 只做本地过滤，
  不会减少上游流量。
- **日期归属**：按市场本地日历日换算；跨夜盘（如 COMEX 电子盘 18:00 起）严格按交易日历归属需另用日历校正。
- **不覆盖其它来源字段**：upsert 只覆盖本次有值的列，`open_interest` / `pe_ratio` 等不会被抹成 NULL。
- **本接口不写** `open_interest` / `hist_volatility` / `implied_volatility` / `pe_ratio` / `trade_count`
  （上游日K接口不提供），这些列需要另外的数据源回填。
- **排障**：`docker logs <容器名> 2>&1 | grep '\[cdeu\]\[daykline\]'`；日志按
  `sync start → params → fetch start/done → filter → upsert batch → sync done/failed` 顺序输出。
