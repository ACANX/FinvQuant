# 加密货币行情排行（FTCryptoRank）

> 所属：导航「行情 → 实时行情 → 加密货币行情排行」
> 路由：`/Meta/Finv/Quote/Realtime/FTCryptoRank`
> 视图：`Web/src/views/Meta/Finv/Quote/Realtime/CryptoRankView.vue`
> 菜单 key：`quote-realtime-ftcrypto-rank`（父分组 `quote-realtime`，紧跟「期货行情排行」）
> 数据表：`cdeu.finv_quote_secu_realtime_crypto`（一行一币种，主键 `usc`）
> 接口：[`GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Crypto/Page`](../../../API/Cdeu/RealtimeCryptoPage.md)
> 计价币种下拉：[`GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Crypto/QuoteCurrencies`](../../../API/Cdeu/RealtimeCryptoPage.md)
> 关联需求：11323

## 1. 业务功能

展示**收藏币种**的最新一张行情快照：每行一个币种，主键 `usc`，整行覆盖更新。
数据由「行情快照同步 → 加密货币」Tab 的采集写入（moomoo OpenAPI，`provider=MMAPI`），本页只读。

## 2. 页面构成

### 2.1 筛选与排序

| 控件 | 说明 |
|---|---|
| 关键字 | 模糊匹配 `usc` / `code` / `name`（回车即查询） |
| 计价币种 | 首项「全部币种」，其余取表内 `DISTINCT quote_currency`（实测 `USD` / `USDC` / `USDT`）；选中即查 |
| 状态 | 全部状态 / 未停牌（`suspension=false`）/ 停牌（`suspension=true`）；选中即查 |
| 排序字段 | 涨跌幅（默认）、涨跌额、现价、振幅、成交量、成交额、量比、买卖比、买价、卖价、52 周最高/最低、更新时间、代码 |
| 方向 | 降序（默认）/ 升序；空值一律排在最后 |
| 每页 | 20 / 50（默认）/ 100 / 200，位于表格底部、页码控件左侧 |
| 查询 / 刷新 / 重置 | 查询回到第 1 页；刷新按当前条件重取；重置恢复默认（涨跌幅降序） |

### 2.2 表格列

`usc`、代码（`code`，形如 `CC.BTCUSD`）、名称、计价币种（`quote_currency`）、
现价、开盘、最高、最低、昨收、涨跌额、涨跌幅、振幅、成交量、成交额、量比、买卖比、
买价、卖价、52 周最高 / 最低、状态（`suspension` → 正常/停牌）、数据日期（`data_date`）、
更新时间（`dt_update`）。

### 2.3 展示规则

- 价格按量级给 2~8 位小数并去尾零（加密币价格跨度大，`0.00001234` 这类小价也能完整显示）；
- 成交量 / 成交额按 **万 / 亿 / 万亿** 折算；
- 涨跌幅 / 振幅 / 买卖比统一 `x.xx%`；空值显示 `—`；
- **红涨绿跌**：`price_direct` 为 `+` 整行红色、为 `-` 整行绿色。

## 3. 使用说明

1. 进入菜单默认按涨跌幅降序取第一页；
2. 币种下拉来自表内实际数据，收藏更多币种并采集后会自动出现新币种；
3. 表格可横向滚动（列较多）；
4. 本页不提供采集入口，采集请到「行情采集 → 行情快照同步 → 加密货币」Tab。

## 4. 注意事项

- 只包含**收藏表 `quote.finv_quote_secu_fav` 里 usc 已登记的加密货币**（采集范围由收藏决定）；
- 表内 `change_price` / `change_ratio` 由 `last - prev_close` 计算，`price_amplitude` 是振幅、不是涨跌幅；
- 数据为空时先确认收藏与采集是否成功（采集日志会写 `[crypto]` 前缀的批次日志）。
