# POST /API/V1/Meta/Finv/Quote/Crypto/Fav/Snapshot/Sync — 收藏加密货币行情快照采集

> 所属：行情 → 实时行情（能力接口，暂无独立页面按钮）
> 数据表：`cdeu.finv_quote_secu_realtime_crypto`（建表 `V300225`，需求 11314）
> 数据来源：`quote.finv_quote_secu_fav`（收藏表）→ moomoo OpenAPI `POST /api/v1.0/quote/snapshot`
> 实现：`internal/api/handler/cdeu_crypto_snapshot.go`、
> `internal/meta/finv/quote/cdeu/{moomoo_openapi,store_crypto_realtime,crypto_fav_snapshot_sync}.go`

## 1. 概述

把「规范证券信息维护」页收藏的**加密货币**（`quote.finv_quote_secu_fav` 中属于
`CRYPTO` / `type_secu='Crypto'` 的 `usc`）批量拉取实时快照，upsert 到
`cdeu.finv_quote_secu_realtime_crypto`（一行一交易对，主键 `usc`）。

- 请求代码 = `CC.` + `usc`（`BTCUSD` → `CC.BTCUSD`），一次最多 400 个；
- 接口 `ret_code != 0` 或 HTTP 非 200 → 整批失败（不写半批）；
- 收藏表里没有加密货币 → 返回空结果（`usc_total=0`），不报错。

## 2. 请求

| 项 | 值 |
|---|---|
| 方法 | `POST` |
| 路径 | `/API/V1/Meta/Finv/Quote/Crypto/Fav/Snapshot/Sync` |
| 认证 | `Authorization: Bearer <token>`（本系统登录态） |

请求体可选：

| 字段 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `timeout_ms` | int | 60000 | 本次采集总超时 |
| `batch_size` | int | 400 | 单次请求 code 数（1–400） |

## 3. 响应

```json
{
  "code": 0,
  "message": "采集完成",
  "data": {
    "USCTotal": 2,
    "CodesRequested": 2,
    "RowsReceived": 2,
    "RowsInsert": 2,
    "RowsUpdate": 0,
    "Batches": 1,
    "ElapsedMs": 3260
  }
}
```

| 字段 | 说明 |
|---|---|
| `USCTotal` | 收藏表中加密货币数量 |
| `CodesRequested` | 去重后实际请求的 code 数 |
| `RowsReceived` | 上游返回的快照行数 |
| `RowsInsert` | 新增行数 |
| `RowsUpdate` | 覆盖行数 |
| `Batches` | 批次数 |
| `ElapsedMs` | 总耗时 |

## 4. 凭据（服务端环境变量，**不入库、不写日志**）

| 变量 | 说明 |
|---|---|
| `FINV_MOOMOO_APP_KEY` | 控制台创建的 AppKey ID |
| `FINV_MOOMOO_PRIVATE_KEY` | Ed25519 PKCS#8 私钥 PEM（可含字面 `\n`） |
| `FINV_MOOMOO_BASE_URL` | 可选，默认 `https://webapi.moomoo.com` |

未配置前两项时接口返回 `503`（`code=2006`），提示先配置凭据。

签名口径（传统 API Key）：`{timestamp_ms}\n{HTTP_METHOD}\n{path}\n{query}\n{sha256(body)_hex}`，
Ed25519 私钥签名后 Base64 作为 `Authorization`，另带 `X-Api-Key` / `X-Timestamp` / `X-Nonce`。

## 5. 失败场景

| 场景 | HTTP | 说明 |
|---|---|---|
| 未配置凭据 | 503 | 提示配置 `FINV_MOOMOO_APP_KEY` / `FINV_MOOMOO_PRIVATE_KEY` |
| 私钥格式错误 | 500 | PKCS#8 / Ed25519 解析失败 |
| 上游 `ret_code != 0` | 500 | 错误消息带 `ret_code` 与 `ret_msg` |
| 写库失败 | 500 | `cdeu.finv_quote_secu_realtime_crypto` 写入错误 |
