# Hello

> 所属：FinvQuant 对外 OpenAPI 文档 · 存放：`Docs/OpenAPI/Region/AZ/Server/Hello.md`
> 对应路径：`/OpenAPI/V1/Meta/Region/AZ/Server/Hello`
> 需求：11242（端点）、11243（`kfp` 字段名）、11245（本文按接口规格重写）
> 实现：`internal/api/openapi/hello.go`；`middleware.go` 的 `scopeFreePaths`（授权范围豁免）
> 鉴权协议：[`Auth.md`](../../../API/OpenAPI/Auth.md) · 调用示例与自检：[`Usage/Handshake.md`](../../Usage/Handshake.md)

本文只描述**接口规格**（是什么、要什么、返回什么、失败是什么码）。
「客户端怎么接、运维怎么巡检」属使用说明，见 [`Usage/Handshake.md`](../../Usage/Handshake.md)。

## 1. 接口定位

握手接口。一次调用同时回答三件事：**网络通、认证对、连对平台**。

设计参照 TLS 握手的前两步，只做连通性与身份确认，不承载业务：

| TLS | 本接口 |
|---|---|
| ClientHello | 一次带认证信息的 `GET` 请求（认证由鉴权中间件完成） |
| ServerHello | 响应 `data`（服务端标识 / 时间 / 认到的身份 / 能力 / 可用接口清单） |
| version | `data.protocol`（`finv-openapi/1.0`） |
| random | `data.server_time`（供客户端校准签名时间窗） |
| session id | `data.request_id`（与审计表同值） |
| cipher suite | `data.capabilities.sign_algorithms` |
| extensions | `data.capabilities` + `data.endpoints[]` |

## 2. 请求规格

| 项 | 规格 |
|---|---|
| 方法 | `GET` |
| 路径 | `/OpenAPI/V1/Meta/Region/AZ/Server/Hello` |
| 查询参数 | 无 |
| 请求体 | 无 |
| 幂等 / 副作用 | 幂等，无业务副作用（只写一条审计） |
| 响应信封 | `{code, data, message, request_id}`（与所有 `/OpenAPI/V1/*` 一致，`code=0`、`message="ServerHello"`） |

## 3. 认证要求

三种认证模式均支持，认证信息放在请求头：

| 模式 | 必需请求头 | 待签串 |
|---|---|---|
| 静态 API Key（`APIKEY`） | `X-API-Key: SA_LIVE_…`（或 `Authorization: Bearer SA_LIVE_…`） | 不签名 |
| AK/SK 签名（`AKSK`） | `Authorization: META1-HMAC-SHA256 Credential=AK_LIVE_…, Signature=…` + `X-META-TS` + `X-META-Nonce` | 6 段（`PATH` 为本路径） |
| STS 临时凭据（`STS`） | 同上，但 `Credential=SAK_LIVE_…`，并额外带 `X-META-Token: SST_LIVE_…` | 7 段（第 7 段 = token） |

签名的待签串构造、时间窗、nonce 规则见 [`Auth.md`](../../../API/OpenAPI/Auth.md) §3。

**不支持**：`STS` **兑换凭证**（长期 AK/SK、`token_fp` 为空）不能调用本接口——
按需求 11210 的三类凭证隔离，它只能调 `POST /OpenAPI/V1/Meta/Open/Auth/Credential/STS/Issue` 换证。

## 4. 授权范围要求

**本接口豁免授权范围校验**（`middleware` 的 `scopeFreePaths`，需求 11242）：

- 凭证 `scope` 不需要包含本路径，任何通过认证的凭证都能调用；
- 理由：本接口只回显「服务端认到的身份」与公开能力清单，不读写业务数据，泄漏面为零；
  若还要求显式授权，「先测连通性」就得先去配权限，与握手语义冲突。

豁免的**只是授权范围**，其余校验照旧：

| 校验项 | 是否豁免 |
|---|---|
| 鉴权模式开关（`sys_config` 19001） | 否 |
| 凭证存在 / 未吊销 / 未停用 / 未过期 / 调用方启用 | 否 |
| 签名、时间戳窗（19002）、nonce 防重放 | 否 |
| IP 白名单（调用方级 ∩ 凭证级） | 否 |
| 速率与每日配额（超限 `19008`） | 否 |
| 凭证类型隔离（兑换凭证只能换证） | 否 |
| **授权范围（`scope`）** | **是（仅本接口）** |

## 5. 响应规格

```json
{
  "code": 0,
  "message": "ServerHello",
  "request_id": "336123BF096A0D87D6C876C418FE4ABD",
  "data": {
    "handshake": "ServerHello",
    "protocol": "finv-openapi/1.0",
    "server": { "name": "finvquant", "version": "0.1.5", "commit": "dev", "az": "AZ" },
    "server_time": { "unix": 1794427291, "rfc3339": "2026-11-11T20:01:31Z", "utc_offset": "+00:00" },
    "client": {
      "client_idn": 47,
      "client_code": "openapi-demo",
      "credential_idn": 30,
      "credential_no": "DEMO-STATIC",
      "auth_mode": "APIKEY",
      "access_key_id": "SA_LIVE_D767FC48****************AE1D3130",
      "kfp": "6BA17C277CC6633B",
      "scope_count": 1,
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
      { "api_code": "finv.quote.cdeu.min.pageuni", "alias": "finv.quote.cdeu.min.PageUni", "http_method": "GET", "flag_write": "0" }
    ],
    "request_id": "336123BF096A0D87D6C876C418FE4ABD"
  }
}
```

### 5.1 `data` 字段

| 字段 | 类型 | 必有 | 说明 |
|---|---|---|---|
| `handshake` | string | ✅ | 固定 `"ServerHello"`；握手成功的判定依据（不要只看 HTTP 200） |
| `protocol` | string | ✅ | 握手协议版本，当前固定 `finv-openapi/1.0` |
| `server.name` | string | ✅ | 服务端名称，固定 `finvquant` |
| `server.version` | string | ✅ | 服务端版本号 |
| `server.commit` | string | ✅ | 服务端构建 commit（`dev` 表示本地构建） |
| `server.az` | string | ✅ | 可用区标识，当前固定 `AZ` |
| `server_time.unix` | int64 | ✅ | 服务端当前 Unix 秒；客户端据此校准签名时间窗 |
| `server_time.rfc3339` | string | ✅ | 同一时刻的 RFC3339 表示 |
| `server_time.utc_offset` | string | ✅ | 服务端时区偏移（如 `+08:00`） |
| `client.client_idn` | int64 | ✅ | 调用方主键 |
| `client.client_code` | string | ✅ | 调用方编码 |
| `client.credential_idn` | int64 | ✅ | 凭证主键（与审计 `credential_idn` 同值） |
| `client.credential_no` | string | ✅ | 凭证号 |
| `client.auth_mode` | string | ✅ | 实际命中的认证模式：`APIKEY` / `AKSK` / `STS` |
| `client.access_key_id` | string | ✅ | 公开标识，形态与审计表 `access_key_id` 一致：`APIKEY` 为脱敏形态、`AKSK` 为 AK 明文、`STS` 为临时 AK |
| `client.kfp` | string | ✅ | 密钥指纹，**字段名与审计表列名一致**（需求 11243）：`sha256(完整 Key)` 前 16 位大写；`AKSK` 为空串（AK 已明文，无需指纹） |
| `client.scope_count` | int | ✅ | 该凭证被授权的接口条数；`0` = 不限接口 |
| `client.expire_at` | string | ✅ | 凭证到期时间（RFC3339）；空串表示随调用方 / 长期 |
| `capabilities.sign_algorithms` | string[] | ✅ | 服务端接受的签名算法（当前仅 `HMAC-SHA256`） |
| `capabilities.clock_skew_seconds` | int | ✅ | 签名时间戳允许偏差（`sys_config` 19002） |
| `capabilities.auth_mode` | string | ✅ | 鉴权模式开关当前值：`0` 关闭 / `1` 强制 / `2` 宽松 |
| `capabilities.rate_limit_rpm` | int | ✅ | 该凭证的有效每分钟速率上限（`0` = 不限） |
| `capabilities.quota_day` | int | ✅ | 该凭证的有效每日配额（`0` = 不限） |
| `capabilities.sts.exchange_path` | string | ✅ | STS 换证路径 |
| `capabilities.sts.ttl_seconds` | int | ✅ | STS 临时凭据有效期（固定 7200 秒） |
| `capabilities.request_id_header` | string | ✅ | 链路 ID 响应头名，固定 `X-Request-Id` |
| `capabilities.scope_required_note` | string | ✅ | 提示「哪些接口需要授权范围」（即本接口以外的业务接口） |
| `endpoints[]` | array | ✅ | 当前对外可用接口清单（`flag_public='1'` 且启用中且未下线），按 `catalog, func` 排序，最多 200 条 |
| `endpoints[].api_code` | string | ✅ | 接口编码（稳定锚点） |
| `endpoints[].alias` | string | ✅ | 别名 `{catalog}.{func}` |
| `endpoints[].http_method` | string | ✅ | HTTP 方法 |
| `endpoints[].flag_write` | string | ✅ | `1` = 写接口（静态 Key 调用需带 `_` 毫秒时间戳）/ `0` = 读 |
| `request_id` | string | ✅ | 链路 ID，与响应头 `X-Request-Id`、审计表 `request_id` 三者同值 |

### 5.2 敏感信息边界

响应**不含任何密钥材料**：静态 Key 只有脱敏形态（前缀 + 前 8 + 等量星号 + 后 8），
`AKSK` 只回 AK（公开标识），SK 与 SecurityToken 永不出现。

### 5.3 响应头

| 头 | 说明 |
|---|---|
| `X-Request-Id` | 32 位大写十六进制；与响应体 `request_id`、审计表 `request_id` 同值。客户端可透传（只接受 8~64 位十六进制并转大写） |

## 6. 错误码

失败时不返回 `ServerHello`，按鉴权错误码拒绝（详见 [`ErrorCodeSpec`](../../../DevSpec/ErrorCodeSpec.md) §1.2 与 [`Auth.md`](../../../API/OpenAPI/Auth.md) §5）：

| 错误码 | HTTP | 含义 |
|---|---|---|
| `19001` | 401 | 未携带任何凭证 |
| `19002` | 401 | 凭证无效（AK 未登记 / SK 不匹配 / 缺 `X-META-Token`） |
| `19003` | 403 | 凭证吊销、停用、过期，或调用方停用 |
| `19004` | 401 | 来源 IP 不在白名单 |
| `19005` | 401 | 签名不匹配（含算法名与凭证登记不一致） |
| `19006` | 401 | 时间戳超窗（可先用 `server_time.unix` 校准） |
| `19007` | 401 | nonce 重复（重放） |
| `19008` | 429 | 超出速率或每日配额 |
| `19010` | 503 | 平台未就绪（19005 密钥未配置 / Redis fail-close） |

**本接口不会返回 `19009`**（授权范围不足，因为它豁免了 scope 校验）。
若调用本接口看到 `19009`，说明请求路径写错了（打到了业务接口）。

## 7. 相关文档

- 调用示例、客户端自检、运维巡检、审计对账、常见失败处置：[`Usage/Handshake.md`](../../Usage/Handshake.md)
- 鉴权协议（待签串 / 时间窗 / nonce）：[`Auth.md`](../../../API/OpenAPI/Auth.md)
- 接口清单（本表数据来源）：[`Menu/OpenAPI/Endpoint.md`](../../../Menu/OpenAPI/Endpoint.md)
