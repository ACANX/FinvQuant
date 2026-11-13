# POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/FTSecuIndicatorUpsert — 采集端指标回传

> 外部 OpenAPI 端点。**只有对外注册**，内部 `/API/V1` 没有对应路由 ——
> 调用方只有采集端（Cloudflare Worker），人不需要从这里写数据。
>
> 正文（第 1~8 节）是人读说明，第 9 节附录链接机器可读契约
> [`SecuIndicatorUpsert.yml`](SecuIndicatorUpsert.yml)。契约变更时两者须同时更新。
>
> - 实现：`internal/api/router.go`（`openGroup` 注册）、
>   `internal/api/handler/cdeu_secu_indicator.go`、
>   `internal/meta/finv/quote/cdeu/secu_indicator_upsert.go`
> - 说明：本文件是**对外独立契约**，字段 / 错误码 / 示例全部自包含，不引用内部 API 文档。

## 1. 概述

采集端抓完一只证券后，把该证券的完整指标推到这里。服务端在**一个事务**里写三张表：

| 表 | 作用 | 不存在时 |
|---|---|---|
| `cdeu.finv_quote_secu_indicator` | 指标本体（行情 / 估值 / 股本等） | 插入 |
| `futu.finv_futu_quote_collect` | 采集参数（市场 / 品种 / 代码） | 插入（需 `stock_id` 与 `market_code`，否则跳过） |
| `futu.finv_futu_secu_key` | 把 `status` 置 `DONE` | 不改（返回 `key=NOT_FOUND`） |

三条语义（与实现一一对应）：

- **按 usc 关联**：`usc` 不是请求字段，由服务端按 `ftmm_symbol` 判定（见 §4.1）；
- **只覆盖本次给了值的列**：SQL 用 `COALESCE($n, t.col)`，源端缺字段不会把已有值刷空；
- **置 `DONE` 即"采集完成"**：采集端据此跳过该证券，所以这是**有副作用的状态推进**，
  三张表同事务提交，不存在「指标没落库但证券已被跳过」的半成品。

## 2. 认证

`/OpenAPI/V1` 全组由 `internal/api/openapi` 中间件鉴权，支持静态 API Key 与 AK/SK 签名，
由 `sys_config` 19001 控制（`0` 关闭 / `1` 启用 / `2` 宽松）。详见
[`Docs/API/OpenAPI/Auth.md`](../../../../../API/OpenAPI/Auth.md)。

| 项 | 值 |
|---|---|
| 接口编码 | `finv.quote.cdeu.realtime.ftsecuindicatorupsert` |
| 写接口 | 是 —— 静态 Key 调用**必须**带查询参数 `_`（毫秒时间戳，与服务器时间差 ±60 秒） |
| 授权范围 | 需在「授权范围」里给凭证勾上本接口；未勾选返回 `19009 SCOPE_DENY` |

签名模式下 `_` 不需要带（时间戳已在待签串里）。

## 3. 请求

### 3.1 Header

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `Content-Type` | string | 是 | 固定 `application/json` |
| `Authorization` | string | 二选一 | AK/SK 签名：`META1-HMAC-SHA256 Credential=<AK>, Signature=<base64>` |
| `X-API-Key` | string | 二选一 | 静态 Key（`SA_LIVE_` 前缀） |
| `X-META-TS` | string | 签名必填 | Unix 秒级时间戳 |
| `X-META-Nonce` | string | 签名必填 | 随机串，**同一 nonce 不可重复**（防重放） |
| `X-META-Token` | string | STS 必填 | STS 临时凭据的 `security_token` |

### 3.2 请求体

两种形态都接受（服务端自动判定，见 §4.2）：

**形态 A：直接推内层（推荐，体积小）**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `symbol` | string | 是 | 证券键，`{市场标签}.{富途代码}`，例 `SH.600006`；大小写不敏感，入库统一大写 |
| `market` | string | 否 | 市场标签，例 `SH` |
| `code` | string | 否 | 裸代码，例 `600006` |
| `page_type` | string | 否 | 页面类型，仅用于日志 |
| `collected_at` | string | 否 | 采集时刻，RFC3339，例 `2026-11-12T07:53:32+08:00` |
| `source_url` | string | 否 | 来源链接，写入指标表 `secu_url` 与键表 `url` |
| `run_id` | string | 否 | 采集批次号，写入采集参数表 `remark` 便于溯源 |
| `idempotency_key` | string | 否 | 幂等键，当前只记日志（幂等由 `usc` 主键天然保证） |
| `mode` | string | 否 | 只接受 `upsert`；其它值返回 `400` |
| `data` | object | 是 | 指标本体，字段见 §3.3 |

**形态 B：整包转发 Cloudflare Worker 外壳**

顶层为 `{timestamp, method, url, headers, body, $workers, $metadata}` 时，
服务端取 `body` 节点作为形态 A 处理。多余节点（`$workers`、`$metadata`、`headers`）一律忽略。

### 3.3 `data` 字段映射

**落 `cdeu.finv_quote_secu_indicator`**（列名与请求字段不同的在此标注）：

| 请求字段 | 目标列 | 类型 | 说明 |
|---|---|---|---|
| `symbol` | `ftmm_symbol` | string | 必填，入库统一大写 |
| `stock_id` | `stock_id` | bigint | 字符串或数字都接受，例 `"46377058462214"` |
| `name` | `name` | string | 证券名称 |
| `lot_size` | `lot_size` | integer | 每手股数 |
| `spread_code` | `spread_code` | string | 价差档位码 |
| `ex_type` | `ex_type` | smallint | 交易所类型 |
| `price_accuracy` | `price_accuracy` | smallint | 价格小数位 |
| `volume_precision` | `volume_precision` | smallint | 成交量精度 |
| `order_volume_precision` | `order_volume_precision` | smallint | 委托量精度 |
| `price` | `price` | numeric | 最新价 |
| `change` | `change_price` | numeric | **改名**：涨跌额 |
| `change_ratio` | `change_ratio` | numeric | 涨跌幅（%） |
| `price_direct` | `price_direct` | string | 涨跌方向 |
| `price_open` | `price_open` | numeric | 开盘价 |
| `price_highest` | `price_highest` | numeric | 最高价 |
| `price_lowest` | `price_lowest` | numeric | 最低价 |
| `price_last_close` | `price_last_close` | numeric | 昨收 |
| `price_nominal` | `price_nominal` | numeric | 名义价格 |
| `hight_direct` | `hight_direct` | string | 最高价方向（列名沿用库内既有拼写） |
| `open_direct` | `open_direct` | string | 开盘方向 |
| `low_direct` | `low_direct` | string | 最低价方向 |
| `volume` | `volume` | numeric | 成交量 |
| `turnover` | `turnover` | numeric | 成交额 |
| `ratio_volume` | `ratio_volume` | numeric | 量比 |
| `ratio_turnover` | `ratio_turnover` | numeric | 换手率 |
| `amplitude_price` | `amplitude_price` | numeric | 振幅 |
| `price_average` | `price_average` | numeric | 均价 |
| `change_speed_price` | `change_speed_price` | numeric | 涨速 |
| `ratio_bid_ask` | `ratio_bid_ask` | numeric | 委比 |
| `total_shares` | `total_shares` | numeric | 总股本 |
| `total_market_cap` | `total_market_cap` | numeric | 总市值 |
| `outstanding_shares` | `outstanding_shares` | numeric | 流通股本 |
| `outstanding_market_cap` | `outstanding_market_cap` | numeric | 流通市值 |
| `price_highest_52week` | `price_highest_week52` | numeric | **改名**：52 周最高 |
| `price_lowest_52week` | `price_lowest_week52` | numeric | **改名**：52 周最低 |
| `price_highest_history` | `price_highest_history` | numeric | 历史最高 |
| `price_lowest_history` | `price_lowest_history` | numeric | 历史最低 |
| `volume_sold` | `volume_sold` | numeric | 外盘 |
| `volume_bought` | `volume_bought` | numeric | 内盘 |
| `price_limit_up` | `price_limit_up` | numeric | 涨停价 |
| `price_limit_down` | `price_limit_down` | numeric | 跌停价 |
| `kcb_stock_static_volume` | `kcb_static_volume` | numeric | **改名**：科创板静态量 |
| `kcb_stock_static_turnover` | `kcb_static_turnover` | numeric | **改名**：科创板静态额 |
| `pe_ttm` | `pe_ttm` | numeric | 市盈率 TTM |
| `pe_lyr` | `pe_lyr` | numeric | 市盈率 LYR |
| `pb_ratio` | `pb_ratio` | numeric | 市净率 |
| `eps_ttm` | `eps_ttm` | numeric | 每股收益 TTM |
| `eps_lyr` | `eps_lyr` | numeric | 每股收益 LYR |
| `ps_ttm` | `ps_ttm` | numeric | 市销率 TTM |
| `ps_lyr` | `ps_lyr` | numeric | 市销率 LYR |
| `time` | `ts` | bigint | **改名**：行情时间（毫秒） |
| `market_status` | `market_status` | smallint | 市场状态码 |
| `market_status_text` | `market_status_text` | string | 市场状态文案 |
| `delay_time` | `delay_time` | smallint | 延迟秒数 |
| `exchange_data_time_ms` | `ts_exchange_dt` | bigint | **改名**：交易所数据时间 |
| `server_send_to_client_time_ms` | `ts_server_send_to_client` | bigint | **改名**：服务端下发时间 |
| `server_recv_from_exchange_time_ms` | `ts_server_recv_from_exchange` | bigint | **改名**：服务端收交易所时间 |
| `source_url` | `secu_url` | string | **改名**：来源链接 |
| `source_channel` | `source_channel` | string | 采集通道，例 `http_requests` |
| `collected_at` | `dt_collect` | timestamptz | 采集时刻 |
| `update_count` | `count_update` | integer | **改名**：更新次数 |
| `unmapped_json` | `json_unmapped` | string | 上游未映射字段的原始 JSON |

**落 `futu.finv_futu_quote_collect`**：

| 请求字段 | 目标列 | 说明 |
|---|---|---|
| `symbol` | `market_label` + `futu_symbol` | 拆成市场标签与裸代码 |
| `stock_code` | `futu_symbol` | 缺失时从 `symbol` 去掉市场前缀 |
| `stock_id` | `stock_id` | 新插入时必填 |
| `market_type` | `market_type` | 缺失按市场标签推（`HK`=1 / `US`=2 / A 股=4 / `FX`=11 / `CC`=17） |
| `market_code` | `market_code` | 缺失按同市场已有行众数补；仍取不到则**跳过本表** |
| `instrument_type` | `instrument_type` | `12` 归一到 `3`（都是股票） |
| `sub_instrument_type` | `sub_instrument_type` | 缺失按 `instrument_type * 1000 + 2` 推 |
| — | `instrument` | 由 `instrument_type` 推（`3`→`stock`、`4`→`etf`、`6`→`index`、`7`→`sectors`、`11`→`crypto`） |

**丢弃的字段**（指标表无对应列，按需求口径不处理）：
`market`、`stock_code`、`en_name`、`page_type`、`underlying_stock_raw`、`spark_info_raw`、
`kcb_stock_static_raw`、`first_seen_at`、`updated_at`、`is_option`、`is_futures`、`is_plate`、`is_etf_theme`。

## 4. 行为与语义

### 4.1 usc 判定顺序

`usc` 是「统一证券代码」，不是市场标签与代码的机械拼接（库里有 `SNTETH`、`501001` 这类裸代码）。
三张表的主键都是 `usc`，但采集端只知道 `ftmm_symbol`，因此按下表顺序判定：

| 顺序 | 来源 | 结果标记 `usc_source` |
|---|---|---|
| 1 | `futu.finv_futu_secu_key.usc`（采集目标就是从这里导出的，全部有值） | `KEY_TABLE` |
| 2 | `cdeu.finv_quote_secu_indicator` 按 `ftmm_symbol` 反查已有行 | `INDICATOR` |
| 3 | `futu.finv_futu_quote_collect` 按 `ftmm_symbol` 反查已有行 | `COLLECT` |
| 4 | 兜底：直接以归一后的 `ftmm_symbol` 当 `usc` | `FTMM_SYMBOL` |

响应会带 `usc_source`，出现 `FTMM_SYMBOL` 说明这只证券在此前从未登记过 ——
指标与采集参数会以新主键插入，可能造出与存量互不相认的行，建议在采集端告警。

### 4.2 请求形态判定

顶层有 `body` 对象且其内含 `data` 时取 `body`（形态 B），否则按顶层解析（形态 A）。
两种形态混用时以 `body` 为准，顶层字段被忽略。

### 4.3 字段级覆盖

指标表与采集参数表的更新语句都用 `COALESCE($n, 列)`：

- 请求给了值 → 覆盖；
- 请求没给（字段缺失、`null`、空串）→ **保留库中原值**；
- 数值 `0` 是有效值（会写入 `0`，不会被当成"未提供"）。

`futu.finv_futu_secu_key` 的更新口径：

| 列 | 规则 |
|---|---|
| `status` | 无条件写 `'DONE'`（已是 `DONE` 也照写，刷新 `gmt_update`，可当天最后一次采集时间用） |
| `stock_id` | 请求有值才覆盖 |
| `url` | 请求 `source_url` 非空才覆盖 |
| `usc` | **不回写**（它是本接口的 `usc` 来源，写回去等于自证） |

### 4.4 事务与幂等

- 三张表的写操作在**同一事务**里，任意一张失败整体回滚；
- 幂等：`usc` 是指标表与采集参数表的主键，重复推送只会覆盖同一行，不会产生重复记录；
- 采集参数表的两个跳过分支不视为失败：
  - `SKIPPED_NO_STOCK_ID` —— 表中无该行且请求未给 `stock_id`（`stock_id` 列 NOT NULL）；
  - `SKIPPED_NO_MARKET_CODE` —— 表中无该行、请求未给 `market_code`，且同市场也推不出众数。

## 5. 响应

### 5.1 成功（HTTP 200）

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "usc": "600006",
    "ftmm_symbol": "SH.600006",
    "usc_source": "KEY_TABLE",
    "indicator": "INSERTED",
    "collect": "UPDATED",
    "key": "DONE",
    "run_id": "20261112-075328-9uxwal",
    "stock_id": 46377058462214,
    "dt_collect": "2026-11-12T07:53:32+08:00"
  }
}
```

| 字段 | 取值 | 说明 |
|---|---|---|
| `usc` | string | 实际使用的统一证券代码 |
| `ftmm_symbol` | string | 归一后的证券键（大写） |
| `usc_source` | string | `KEY_TABLE` / `INDICATOR` / `COLLECT` / `FTMM_SYMBOL`（见 §4.1） |
| `indicator` | string | `INSERTED` / `UPDATED` |
| `collect` | string | `INSERTED` / `UPDATED` / `SKIPPED_NO_STOCK_ID` / `SKIPPED_NO_MARKET_CODE` |
| `key` | string | `DONE` / `NOT_FOUND`（键表无该证券，其余两表照写） |
| `run_id` | string | 回显请求的采集批次号 |
| `stock_id` | number | 回显解析出的 `stock_id`（请求未给则省略） |
| `dt_collect` | string | 回显解析出的采集时刻（请求未给则省略） |

### 5.2 失败（非 2xx）

```json
{
  "code": 400,
  "message": "实时行情快照入参非法: 缺少 symbol"
}
```

| HTTP | `code` | 场景 |
|---|---|---|
| `400` | `400` | JSON 解析失败 / 缺 `data` / 缺 `symbol` / `mode` 非 `upsert` / `stock_id` 非整数 |
| `401` | `19001` | 未携带凭证或签名信息 |
| `403` | `19002` / `19003` / `19009` | 凭据无效 / 已停用或过期 / 授权范围不含本接口 |
| `429` | `19007` | 超过调用额度 |
| `500` | `2006` | 写库失败（事务已回滚，三表均无变更） |

完整错误码见 [`ErrorCodeSpec.md`](../../../../../../DevSpec/ErrorCodeSpec.md) §1.2。

## 6. 错误码

| 码 | 含义 | 处理建议 |
|---|---|---|
| `400` | 入参非法 | 检查 `symbol` / `data` / `stock_id` 类型 |
| `19001` | 未携带凭证 | 补 `Authorization` 或 `X-API-Key` |
| `19002` | AK 不存在或 SK 不匹配 | 核对密钥（两者刻意合并，不区分） |
| `19003` | 凭证已停用 / 已吊销 / 已过期 | 查审计 `reason_code`（`REVOKE` / `EXPIRE`） |
| `19009` | 授权范围不含本接口 | 在「授权范围」给凭证勾上 `finv.quote.cdeu.realtime.ftsecuindicatorupsert` |
| `19007` | 超过额度 | 等窗口重置或调额度 |
| `2006` | 写库失败 | 看服务端日志 `[cdeu][secuindicator]` |

## 7. 示例

```bash
curl -X POST 'http://127.0.0.1:16001/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/FTSecuIndicatorUpsert?_=<毫秒时间戳>' \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: SA_LIVE_xxxxxxxx' \
  -d '{
    "symbol": "SH.600006",
    "market": "SH",
    "collected_at": "2026-11-12T07:53:32+08:00",
    "source_url": "https://www.moomoo.com/hans/stock/600006-SH",
    "run_id": "20261112-075328-9uxwal",
    "mode": "upsert",
    "data": {
      "symbol": "SH.600006",
      "stock_code": "600006",
      "stock_id": "46377058462214",
      "name": "东风股份",
      "instrument_type": 3,
      "sub_instrument_type": 3002,
      "market_type": 4,
      "market_code": 30,
      "price": 5.06,
      "change": -0.05,
      "time": 1794380400000,
      "source_url": "https://www.moomoo.com/hans/stock/600006-SH",
      "source_channel": "http_requests",
      "collected_at": "2026-11-12T07:53:32+08:00"
    }
  }'
```

AK/SK 签名与 STS 临时凭据的完整示例见
[`Docs/Proposal`](../../../../../../Proposal/README.md) 同目录的联调脚本说明。

## 8. 已使用位置登记

| 调用方 | 用途 | 备注 |
|---|---|---|
| Cloudflare Worker `webhook`（采集端 `moomoo-secu-fn`） | 抓取完成后回传单只证券指标 | 请求带 `run_id` / `idempotency_key` |

## 9. 附录：OpenAPI 3.1 契约（机器可读）

- 契约文件：[`SecuIndicatorUpsert.yml`](SecuIndicatorUpsert.yml)
- 接口清单登记：`open.open_api_endpoint`（`api_code = finv.quote.cdeu.realtime.ftsecuindicatorupsert`，
  迁移 `V300208`）
