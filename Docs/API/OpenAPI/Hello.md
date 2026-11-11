# 握手（Hello）

> 所属：FinvQuant API 文档 · 存放：`Docs/API/OpenAPI/Hello.md`
> 需求：11242
> 实现：`internal/api/openapi/hello.go`（处理器）、`internal/api/openapi/middleware.go`（`scopeFreePaths` 豁免）
> 鉴权协议：[`Auth.md`](Auth.md)
> 接口清单：`open.open_api_endpoint` 中 `open.region.az.server.hello`

## 1. 它解决什么问题

调用方在正式收发业务数据前，需要一个**低成本、无副作用**的方式确认三件事：

1. **网络通**：能连到平台（域名解析、端口、反向代理、防火墙）；
2. **认证对**：`APIKEY` / `AKSK` / `STS` 三要素配置正确（Key 没写错、时钟没偏太多、nonce 没复用）；
3. **连对了平台**：服务端认到的「你是谁」与预期一致（调用方、凭证号、认证模式）。

这三件事合起来就是「握手」。思路照搬 TLS 的 ClientHello / ServerHello：

```
客户端 ──── ClientHello ────► 服务端
          （一次带认证信息的请求）
                              │ 鉴权中间件校验：19001 开关 / 凭证有效性 / 签名 / 时间窗 /
                              │ nonce / IP 白名单 / 限流 —— 失败按 190xx 直接拒绝
客户端 ◄─── ServerHello ───── 服务端
          （回显身份 + 服务端时间 + 能力 + 可用接口清单）
```

握手成功（HTTP 200 且 `data.handshake = "ServerHello"`）后，才开展后续业务请求。

## 2. 端点

```
GET /OpenAPI/V1/Meta/Region/AZ/Server/Hello
```

无查询参数、无请求体、无副作用（不写业务表，只写一条审计）。

## 3. 三种认证方式

| 模式 | 请求头 | 说明 |
|---|---|---|
| 静态 API Key | `X-API-Key: SA_LIVE_…`（或 `Authorization: Bearer …`） | 只读接口，静态 Key 可直接调 |
| AK/SK 签名 | `Authorization: META1-HMAC-SHA256 Credential=AK_LIVE_…, Signature=…` + `X-META-TS` + `X-META-Nonce` | 待签串 6 段，`PATH` 就是本路径 |
| STS 临时凭据 | 同上，但 `Credential=SAK_LIVE_…` 且多一个 `X-META-Token: SST_LIVE_…` | 待签串 7 段（第 7 段 = token） |

**STS 兑换凭证（长期 AK/SK，`token_fp` 为空）不能握手**：按需求 11210 的三类凭证隔离，
它只能调 `POST /OpenAPI/V1/Meta/Open/Auth/Credential/STS/Issue` 换证——用它测连通性的方式就是调换证接口。

## 4. 不需要授权范围（本次唯一的例外）

**握手接口豁免授权范围校验**（`middleware` 的 `scopeFreePaths`）。理由：

- 它只回显「服务端认到的身份」与公开能力清单，不读写任何业务数据，泄漏面为零；
- 如果还要求凭证 `scope` 里显式包含 `/Hello`，「测连通性」就得先去页面配授权，
  与「握手只验认证」的语义冲突。

注意豁免的**只是授权范围**，其余照旧：

| 校验 | 是否豁免 |
|---|---|
| 19001 鉴权模式开关 | 否 |
| 凭证存在 / 未吊销 / 未停用 / 未过期 | 否 |
| 签名、时间窗、nonce 防重放、IP 白名单、限流配额 | 否 |
| 凭证类型隔离（兑换凭证只能换证） | 否 |
| **授权范围（scope）** | **是**（仅本接口） |

## 5. 响应（ServerHello）

```json
{
  "code": 0,
  "message": "ServerHello",
  "request_id": "336123BF096A0D87D6C876C418FE4ABD",
  "data": {
    "handshake": "ServerHello",
    "protocol": "finv-openapi/1.0",
    "server": { "name": "finvquant", "version": "0.1.5", "commit": "dev", "az": "AZ" },
    "server_time": { "unix": 1794427217, "rfc3339": "2026-11-12T03:33:37+08:00", "utc_offset": "+08:00" },
    "client": {
      "client_idn": 20, "client_code": "openapi-demo",
      "credential_idn": 31, "credential_no": "DEMO-AKSK",
      "auth_mode": "AKSK",
      "access_key_id": "AK_LIVE_7A01139C5571D46EDEFF2DB63C9F0E45",
      "key_fp": "",
      "scope_count": 2,
      "expire_at": ""
    },
    "capabilities": {
      "sign_algorithms": ["HMAC-SHA256"],
      "clock_skew_seconds": 301,
      "auth_mode": "1",
      "rate_limit_rpm": 600,
      "quota_day": 100000,
      "sts": { "exchange_path": "/OpenAPI/V1/Meta/Open/Auth/Credential/STS/Issue", "ttl_seconds": 7200 },
      "request_id_header": "X-Request-Id",
      "scope_required_note": "除 /OpenAPI/V1/Meta/Region/AZ/Server/Hello 外的业务接口都需要凭证授权范围包含它"
    },
    "endpoints": [
      { "api_code": "finv.quote.cdeu.min.pageuni", "alias": "finv.quote.cdeu.min.PageUni", "http_method": "GET", "flag_write": "0" },
      { "api_code": "open.region.az.server.hello", "alias": "open.region.az.server.Hello", "http_method": "GET", "flag_write": "0" }
    ],
    "request_id": "336123BF096A0D87D6C876C418FE4ABD"
  }
}
```

### 5.1 字段说明

| 字段 | 含义 | 客户端怎么用 |
|---|---|---|
| `handshake` | 固定 `"ServerHello"` | 判断握手成功（别只看 HTTP 200） |
| `protocol` | 握手协议版本 `finv-openapi/1.0` | 版本不匹配时降级/告警 |
| `server` | 服务端标识（名称 / 版本 / commit / AZ） | 确认连到了预期实例（多节点排障） |
| `server_time.unix` | 服务端当前 Unix 秒 | **校准本地时钟**：本地与它之差的绝对值应远小于 `clock_skew_seconds` |
| `client.*` | 服务端认到的身份 | 与本地配置比对，确认 Key 没串、凭证号没搞错 |
| `client.access_key_id` | 与审计表 `access_key_id` 同形态 | 静态 Key 是脱敏形态、AKSK 是 AK 明文、STS 是临时 AK |
| `client.key_fp` | 与审计表 `kfp` 同值（`sha256(Key)` 前 16 位大写） | 只 `APIKEY` / `STS` 有值；`AKSK` 为空（AK 已明文） |
| `client.scope_count` | 该凭证被授权的接口条数，`0` = 不限接口 | 发现「我以为限制了其实没限制」 |
| `capabilities.clock_skew_seconds` | 签名时间戳允许偏差（19002） | 超过就得先校时，否则业务接口会 `19006` |
| `capabilities.auth_mode` | 19001 当前值（`0` 关闭 / `1` 强制 / `2` 宽松） | `2` 时失败放行，别误以为认证通过 |
| `capabilities.sts` | 换证路径与有效期 | 规划临时凭据刷新节奏（2 小时） |
| `endpoints[]` | 当前对外可用接口（公开 + 启用 + 未下线） | 生成客户端请求白名单，避免拼错路径 |

**响应不含任何密钥材料**：静态 Key 只有脱敏形态，AK/SK 只有 AK（公开标识），SK 与 SecurityToken 永不出现。

### 5.2 响应头

与所有 `/OpenAPI/V1/*` 一致：`X-Request-Id` 回显，32 位大写十六进制，与响应体 `request_id`、
审计表 `request_id` 三者同值——出问题直接拿它查审计。

## 6. 常见失败与处理

| 现象 | 原因 | 处理 |
|---|---|---|
| `401 / 19001` | 没带凭证 | 补 `X-API-Key` 或 `Authorization` |
| `401 / 19002` | 凭证无效（AK 未登记、SK 不符、STS 缺 `X-META-Token`） | 核对 Key/Token；`19002` 刻意合并「AK 不存在」与「SK 不匹配」防枚举 |
| `403 / 19003` | 凭证吊销/停用/过期，或调用方停用 | 页面「OpenAPI 凭证」查状态 |
| `401 / 19005` | 签名不匹配（含算法名与凭证登记不一致） | 核对待签串顺序、`BODY_SHA256` 是否大写 |
| `401 / 19006` | 时间戳超窗 | 用 `data.server_time.unix` 校准本地时钟后重试 |
| `401 / 19007` | nonce 重复 | 每次请求生成新 nonce |
| `429 / 19008` | 超速率/配额 | 放宽调用节奏或调额度 |
| `503 / 19010` | 平台未就绪（19005 未配置 / Redis fail-close） | 联系平台方 |

> 注意：握手接口**不会**返回 `19009`（授权范围不足）——它豁免了 scope 校验。
> 如果你在握手时看到 `19009`，说明请求路径写错了（打到了业务接口）。

## 7. 典型用法

### 7.1 客户端启动自检（Python 片段）

```python
status, headers, body = call("GET", "/OpenAPI/V1/Meta/Region/AZ/Server/Hello")
assert status == 200 and body["data"]["handshake"] == "ServerHello"
drift = abs(int(time.time()) - body["data"]["server_time"]["unix"])
assert drift < body["data"]["capabilities"]["clock_skew_seconds"], f"本机与平台时钟差 {drift}s，请先校时"
print("握手成功：", body["data"]["client"]["credential_no"], "可用接口", len(body["data"]["endpoints"]))
```

### 7.2 运维巡检

```bash
curl -s -H "X-API-Key: SA_LIVE_…" \
  http://127.0.0.1:16001/OpenAPI/V1/Meta/Region/AZ/Server/Hello \
  | jq '{handshake, server, client: .client.credential_no, endpoints: (.endpoints|length)}'
```

### 7.3 审计对账

```sql
SELECT dt_request, request_id, auth_mode, access_key_id, kfp, flag_pass, status_code
  FROM open.open_api_invoke_log
 WHERE path = '/OpenAPI/V1/Meta/Region/AZ/Server/Hello'
 ORDER BY idn DESC LIMIT 20;
```

## 8. 为什么不用 `/Health`

`/Health`（以及 `/api/v1/health/live|ready`）是**平台自身的存活探针**，不需要认证、也不校验调用方身份。
握手接口回答的是另一个问题：**「我这个调用方，用这套凭证，能不能跟平台说上话」**——
它必须走完整鉴权链路，因此挂在 `/OpenAPI/V1` 下，并写审计。
