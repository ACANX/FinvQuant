# 外部 OpenAPI 鉴权（Auth）

> 所属：FinvQuant API 文档 · 存放：`Docs/API/OpenAPI/Auth.md`
> 需求：11168（鉴权中间件）；11167（表结构）
> 实现：`internal/api/openapi`（`Middleware` / `PGSource` / `SettingsStore` / `Recorder`）
> 数据字典：[`../../DataDictMapping/OpenApiAuth.md`](../../DataDictMapping/OpenApiAuth.md)
> 错误码目录：[`../../DevSpec/ErrorCodeSpec.md`](../../DevSpec/ErrorCodeSpec.md) §1.2（`190xx`）

## 1. 适用范围

`/OpenAPI/V1/*` 全部接口。与内部接口（`/API/V1/*` 的登录会话 `Authorization: Bearer <token>`）**完全独立**：
同一个 `Authorization` 头在两套体系里含义不同，互不通用。

鉴权行为由 `sys.sys_config` **19001** 控制：

| 19001 | 含义 | 行为 |
|---|---|---|
| `0` | 关闭（**默认**） | 不做任何校验，全部放行（等价早期的过渡实现），不写审计日志 |
| `1` | 启用 | 必须通过鉴权，失败按 `190xx` 拒绝 |
| `2` | 宽松（灰度） | 照常校验、照常写审计日志，但**校验失败仍放行**（只打告警日志） |

配置改动最多 10 秒生效（进程内短 TTL 缓存），**不需要重启**。

## 2. 模式一：静态 API Key

```http
GET /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni?column=usc&row=AAPL&limit=1
X-API-Key: SA_LIVE_XXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

也接受 `Authorization: Bearer SA_LIVE_XXXXXXXXXXXXXXXXXXXXXXXXXXXX`（两者都带时以 `X-API-Key` 为准）。

> **大小写（需求 11176）**：凭证号、静态 Key、AK、SK 里的英文字母**一律大写**，便于人工抄写与电话核对。
> 调用方手写小写也能通过：服务端会按大写归一后查库与比对哈希。

- 凭证表中的 `access_key_id` 与 `X-API-Key` 的值**相同**（静态 Key 本身就是公开标识）；
- 服务端比对 `sha256(提供的 Key)` 与库内 `secret_hash`（恒定时间比较），库内**不存明文**；
- 密钥只在创建凭证时返回一次，之后无法从系统取回（丢了就轮换/新建）。

## 3. 模式二：AK/SK 签名

```http
POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert?batch=1
Authorization: META1-HMAC-SHA256 Credential=AK_LIVE_XXXXXXXXXXXXXXXX, Signature=oQ7Fk3v...=
X-META-TS: 1737000000
X-META-Nonce: 8e2f1c9a4b7d4c0e
Content-Type: application/json
```

### 3.1 待签串（canonical string）

```
METHOD \n
PATH \n
SORTED_QUERY \n
BODY_SHA256 \n
TIMESTAMP \n
NONCE
```

| 段 | 取值 |
|---|---|
| `METHOD` | 大写的 HTTP 方法（`GET` / `POST` …） |
| `PATH` | 请求路径，**不含**域名、端口、查询串（如 `/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert`） |
| `SORTED_QUERY` | 查询参数按**参数名升序**、同名参数的**值也升序**，键与值都做 RFC 3986 百分号编码（空格 → `%20`），再用 `&` 连接；无查询串时为空行 |
| `BODY_SHA256` | 请求体的 SHA-256 十六进制**大写**（需求 11177）；GET / 空体用 `sha256("")` 的大写形式 `E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855` |
| `TIMESTAMP` | 与 `X-META-TS` **完全一致**的字符串（Unix 秒） |
| `NONCE` | 与 `X-META-Nonce` 完全一致的字符串（建议 8~64 位随机串，每次请求不同） |

### 3.2 签名

```
signature = Base64( HMAC-SHA256( SK, canonical ) )
```

`Authorization: META1-<签名算法> Credential=<AK>, Signature=<base64 签名>`

- 方案前缀 `META1-` 后紧跟**签名算法名**（与凭证登记的 `sign_algorithm` 一致，如 `HMAC-SHA256`）；
- 参数按 `,` 分隔、每段 `k=v`，参数名大小写不敏感，值两端空白与可选双引号会被去掉；
- `Credential` 与 `Signature` 必须同时存在，缺一即 `19002`；
- 大小写不敏感的前缀（`meta1-hmac-sha256` 也能识别），但算法名会统一转大写比对。

> 旧写法（`Authorization: FinvAK <AK>:<签名>` + `X-Finv-Timestamp` / `X-Finv-Nonce`）**已废止**（需求 11179），
> 中间件不再接受，请按上面的新格式改造。

### 3.3 服务端校验顺序

| # | 检查 | 失败码 |
|---|---|---|
| 1 | 按 `access_key_id` 定位凭证（不存在也返回 19002，**不区分**「AK 不存在」与「SK 不匹配」，防 AK 枚举） | `19002` |
| 2 | 用 `sys_config` 19005 的密钥解密 SK（`secret_cipher`；轮换窗口内 `secret_cipher_prev` 也参与验签） | `19010`（密钥未配置/解不开） |
| 3 | 凭证与调用方状态：`flag_enable` / `flag_revoke` / `dt_revoke` / 双方 `dt_expire` | `19003` |
| 4 | IP 白名单（调用方级 ∩ 凭证级，都为空表示不限） | `19004` |
| 5 | 时间戳偏差 ≤ `sys_config` 19002（默认 300 秒） | `19006` |
| 6 | nonce 在窗口内未被使用过（Redis `SET NX PX`，窗口 = 2 × 时间窗） | `19007` |
| 7 | 校验请求头里的算法与凭证登记的 `sign_algorithm` 一致，再重算签名并恒定时间比较 | `19005` |
| 8 | 授权范围 `scope_str`（空数组 = 全部接口；支持精确路径与尾部 `/*` 前缀） | `19009` |
| 9 | 每分钟速率 `rate_limit_rpm` / 每日配额 `quota_day`（Redis 计数） | `19008` |

Redis 不可用时，nonce 与限流检查**放行**并打告警日志（宁可放行，也不因缓存故障把外部调用方全挡住）；但签名、状态、白名单、时间窗这些不依赖外部状态的检查照旧执行。

### 3.4 最小可运行示例（Go）

```go
canonical := strings.Join([]string{
    "POST",
    "/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert",
    "",                                  // 无查询串
    sha256Hex(body),                     // 空体用 sha256("")
    timestamp, nonce,
}, "\n")
mac := hmac.New(sha256.New, []byte(sk))
mac.Write([]byte(canonical))
signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
```

## 4. 兼容期兜底密钥（过渡）

鉴权启用后，为了不打断尚未改造的旧调用方，仍接受一个**兜底静态密钥**：

- 环境变量 `FINV_OPENAPI_KEY`；
- `sys_config` **19006**（敏感项，默认空）。

两者任一命中即放行（审计日志里 `auth_mode='FALLBACK'`）。兜底密钥**不受**凭证表的停用/吊销控制，也不参与限流；
正式调用方应改为登记凭证，兜底通道在过渡结束后清空这两个值即可关闭。

## 4.5 方法与删除能力（需求 11180）

对外 OpenAPI 只提供**查询**与**写入（upsert）**两类接口，**不提供删除能力**：

- `GET` / `POST` 之外的请求方法（如 `DELETE`）会先经过鉴权中间件，鉴权通过后返回 `405`：
  ```json
  { "code": 405, "message": "OpenAPI 不提供删除能力；如需清理数据请走内部管理接口" }
  ```
- 之所以注册成真实路由而不是留给 404：删除类请求**同样要过鉴权**，无凭证照样 `19001`，
  不会因为「方法不支持」而绕过鉴权被用来探测接口是否开放。
- 需要清理数据请走内部管理接口（`/API/V1/*`，需登录），或联系平台方。

### 4.6 静态 Key 的额外约束（需求 11188）

静态 Key 没有签名，靠**收紧使用条件**降低风险，以下规则在签发与调用两侧都强制：

| 约束 | 规则 | 违反时 |
|---|---|---|
| 授权接口 | 只能授权 **1 个具体接口**（留空 = 全部、通配 `*` 都不允许） | 签发 400；调用 19009 |
| 调用额度 | 调用方必须先配置 `rate_limit_rpm` 或 `quota_day`（至少一个 > 0）才允许签发 | 签发 400 |
| 有效期 | 必须显式设置到期日，**最长 90 天**（留空视为随调用方，不允许） | 签发 400 |
| 写接口时间戳 | `POST/PUT/PATCH/DELETE` 必须带查询参数 `_`（毫秒时间戳），与服务器时间差 ≤ **60 秒** | 19006 |
| 认证模式取值 | `auth_mode` 统一为 `APIKEY` / `AKSK`（需求 11189，不再使用 `API_KEY` / `AK_SK`） | — |

> AK/SK 签名模式不受「接口个数」限制（可授权多个接口），也不需要 `_` 参数（签名里的时间戳已覆盖）。

### 4.7 防重放与 Redis 故障策略（需求 11188）

| 项 | 规则 |
|---|---|
| 签名时间窗 | `X-META-TS` 与服务器时间差 ≤ `SignTimeWindow`（`sys_config 19002`，默认 300 秒）→ 否则 19006 |
| nonce 去重 | `X-META-Nonce` 必须是 **8~64 位**字母/数字/`_`/`-`，Redis `SET NX PX` 去重（TTL = 2× 时间窗）→ 重复 19007 |
| Redis 故障 | `FINV_OPENAPI_REDIS_FAIL_MODE=close`（默认）→ 拒绝请求（19010，503）；`=open` → 放行并告警（旧行为） |

> 静态 Key 模式**不做 nonce 去重**（没有签名，nonce 可伪造），写接口靠 `_` 时间戳把重放窗口压到 60 秒内。

### 4.8 STS 临时凭据（需求 11191）

调用方可把长期 AK/SK 换成**短期三要素**，用于短时任务（批处理、临时联调），泄露窗口从「长期」压到 2 小时。

```
POST /OpenAPI/V1/Meta/Auth/Credential/STS/Issue
```

- 必须用**长期 AK/SK** 签名调用（父凭证的授权范围要包含该路径）；返回：

  ```json
  { "code": 0, "message": "OK", "data": {
      "access_key_id":  "SA_LIVE_...",
      "secret_key":     "SS_LIVE_...",
      "security_token": "ST_LIVE_...",
      "expire_at":      "2026-11-11T10:30:00Z",
      "expire_seconds": 7200 } }
  ```

- **有效期固定 2 小时**，不可续期；**不能用 STS 再换 STS**（防凭据链无限延长）。
- 临时凭据不落库，存 Redis（`finv:openapi:sts:<临时 AK>`，TTL 2h），权限继承父凭证（scope / IP 白名单 / 调用方限流）。
- **临时 AK 的前缀 `SA_LIVE_` 与静态 Key 相同**（需求 11192）：两者靠传输位置区分——
  静态 Key 走 `X-API-Key` / `Authorization: Bearer`，STS 临时 AK 走签名头 `Credential=SA_LIVE_…`；
  把静态 Key 塞进 `Credential=` 会查不到临时凭据，直接 `19002` 拒绝。

调用时三要素都要带，签名串比长期 AK/SK **多一段**（SecurityToken）：

```
Authorization: META1-HMAC-SHA256 Credential=SA_LIVE_…, Signature=<base64>
X-META-TS:    1737000000
X-META-Nonce: 8~64 位
X-META-Token: ST_LIVE_…

待签串 = METHOD \n PATH \n SORTED_QUERY \n BODY_SHA256 \n TIMESTAMP \n NONCE \n TOKEN
```

> 长期 AK/SK 不带 `X-META-Token`，待签串仍是 6 段，**协议完全不变**，现有客户端无需改造。

审计：`auth_mode='STS'`、`credential_idn` = 父凭证、`kfp` = 临时 AK 指纹、`token_fp` = SecurityToken 指纹
（可按某一次临时凭据把整段会话的调用捞出来）。

## 5. 错误响应

失败时 HTTP 状态码与业务码成对出现（`code` 为 `190xx`），并回显 `X-Request-Id`：

> `X-Request-Id` 的值**一律为大写十六进制**（服务端生成 32 位）。客户端可透传该头用于链路串联，
> 但只接受 8~64 位十六进制并统一转大写，其它形态会被忽略并重新生成；审计表 `request_id` 与它同值。

```json
{
  "code": 19001,
  "message": "未携带 API Key 或签名信息",
  "request_id": "C620F2756DAC8F303D982603A7739D1F"
}
```

完整码表见 [`ErrorCodeSpec.md`](../../DevSpec/ErrorCodeSpec.md) §1.2。成功响应仍走统一信封
`{"code":0,"data":<原始数据>,"message":"OK"}`（`WrapOpenAPIResponse`）。

## 6. 审计

每次经过鉴权的调用（放行与拒绝都算）写一条 `open.open_api_invoke_log`：

- 谁：`client_idn` / `credential_idn` / `access_key_id`（冗余存，凭证删了也能追）/ `auth_mode`；
- 什么：`request_id` / `dt_request` / `method` / `path` / `query_str` / `ip` / `user_agent`；
- 结果：`flag_pass` / `status_code` / `biz_code` / `reason_code`（`NO_KEY`/`BAD_KEY`/`REVOKE`/`EXPIRE`/`CIDR_DENY`/`SIGN_MISMATCH`/`TS_SKEW`/`NONCE_REPLAY`/`SCOPE_DENY`/`RATE_LIMIT`/`NOT_READY`）；
- 体积：`body_sha256`（**大写**十六进制）+ `count_body`（**不存请求原文**）。
- 链路：响应头 `X-Request-Id` 与审计表 `request_id` 同为 32 位**大写**十六进制，可直接按它回查。

审计写入超时 1.5 秒，失败只打应用日志，不影响响应；表按 `sys_config` 19003（默认 90 天）清理。

## 7. 上线步骤（建议顺序）

1. 迁移 `V300183`/`V300184` 已建表建配置，此时 19001 默认 `'0'`，**行为与之前完全一致**；
2. 在 `open.open_api_client` / `open.open_api_credential` 登记调用方与凭证（管理菜单见需求 11169，暂可手工 SQL）；
3. 把 19001 改为 `'2'`（宽松），观察 `open.open_api_invoke_log` 里的失败记录，逐个通知调用方改造；
4. 失败清零后把 19001 改为 `'1'`（强制）；
5. 过渡结束后清空 `FINV_OPENAPI_KEY` 与 19006。

## 8. 说明

- 预留模式：`BEARER`（JWT）/ `MTLS`（证书指纹）/ `BASIC`（用户名口令）在表结构里已有位置，
  中间件当前**明确按 `19010` 拒绝**，避免「看起来支持其实没验」。
- 请求体上限 64 MiB，超限按 `19002` 拒绝（不再继续读取，防内存放大）。
- 幂等与重复请求：签名里含 `nonce`，业务接口本身的幂等性由各自 `ON CONFLICT` 语义保证，两者互不替代。
