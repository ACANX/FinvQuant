# OpenAPI 调用方管理接口（Admin）

> 所属：FinvQuant API 文档 · 存放：`Docs/API/OpenAPI/Admin.md`
> 需求：11169（管理页与接口）；11167（表结构）；11168（鉴权中间件）
> 鉴权协议：[`Auth.md`](Auth.md) · 数据字典：[`../../DataDictMapping/OpenApiAuth.md`](../../DataDictMapping/OpenApiAuth.md)
> 菜单文档：[`../../Menu/OpenAPI/Client.md`](../../Menu/OpenAPI/Client.md)

## 1. 概述

- 全部为**内部接口**（`/API/V1/Meta/Finv/OpenAPI/*`，需登录），与对外 `/OpenAPI/V1/*` 无关。
- 只操作 `open` schema 三张表与 `sys.sys_config` 19xxx，不碰行情业务表。
- URL 查询参数小驼峰、JSON 体字段 snake_case（ApiSpec §3）。
- **敏感列永不返回**：`secret_hash` / `secret_cipher` / `secret_cipher_prev` 不出现在任何响应里；
  凭证只回 `has_secret` 布尔。密钥明文仅在签发/轮换的响应里出现一次。

## 2. 接口清单

| # | 方法 | 路径 | 说明 |
|---|------|------|------|
| 1 | GET | `/Settings` | 读鉴权配置（19001～19003 值 + 密钥是否已配置） |
| 2 | PUT | `/Settings` | 保存 19001～19003 |
| 3 | GET | `/Client/Page` | 调用方分页 |
| 4 | POST | `/Client/Save` | 新建（`idn=0`）/ 更新调用方 |
| 5 | POST | `/Client/Enable` | 启用 / 停用调用方 |
| 6 | GET | `/Credential/Page` | 凭证分页 |
| 7 | POST | `/Credential/Create` | 签发凭证（密钥只返回一次） |
| 8 | POST | `/Credential/Enable` | 启用 / 停用凭证 |
| 9 | POST | `/Credential/Revoke` | 吊销凭证（原因必填，不可恢复） |
| 10 | POST | `/Credential/Rotate` | 轮换密钥（新密钥只返回一次） |
| 11 | GET | `/InvokeLog/Page` | 调用审计分页 |

## 3. 鉴权配置

### 3.1 GET `/Settings`

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "mode": "0",
    "sign_time_window": 300,
    "log_retain_days": 90,
    "secret_kid": "k1",
    "secret_key_ready": false,
    "fallback_key_ready": false
  }
}
```

`secret_key_ready` / `fallback_key_ready` 只表示「19005 / 19006 是否已填值」，不回显内容。

### 3.2 PUT `/Settings`

```json
{ "mode": "1", "sign_time_window": 300, "log_retain_days": 90 }
```

| 字段 | 约束 |
|---|---|
| `mode` | `0` / `1` / `2`，其它值 400 |
| `sign_time_window` | 30 ~ 3600（秒），越界 400 |
| `log_retain_days` | 0 ~ 3650（天，0 = 不清理），越界 400 |

保存后 10 秒内生效（进程内短 TTL 缓存），无需重启。

## 4. 调用方

### 4.1 GET `/Client/Page?keyword=&page=1&pageSize=20`

```json
{
  "code": 0, "message": "OK",
  "data": {
    "total": 1,
    "list": [
      {
        "idn": 1, "client_code": "cdeu-hook", "client_name": "CDEU 采集钩子",
        "owner_uid": "", "contact_email": "", "contract_no": "",
        "allow_cidr": ["10.0.0.1/32"], "rate_limit_rpm": 600, "quota_day": 0,
        "dt_expire": "", "flag_enable": "1", "remark": "",
        "gmt_update": "2026-11-11T08:00:00+08:00",
        "count_credential": 2, "count_credential_active": 1
      }
    ]
  }
}
```

`keyword` 对 `client_code` / `client_name` 做 `ILIKE` 模糊匹配；`pageSize` 上限 200。

### 4.2 POST `/Client/Save`

```json
{
  "idn": 0,
  "client_code": "cdeu-hook",
  "client_name": "CDEU 采集钩子",
  "owner_uid": "acanx",
  "contact_email": "ops@example.com",
  "contract_no": "CT-2026-001",
  "allow_cidr": ["10.0.0.1", "192.168.0.0/16"],
  "rate_limit_rpm": 600,
  "quota_day": 0,
  "dt_expire": "2027-12-31T00:00:00+08:00",
  "remark": ""
}
```

| 字段 | 约束 |
|---|---|
| `idn` | `0` = 新建；`>0` = 更新（不存在返回 404） |
| `client_code` | 小写字母/数字/`._-`，2~32 位，全局唯一（撞车 409） |
| `client_name` | 非空，≤128 字符 |
| `allow_cidr` | 裸 IP 自动补 `/32`（IPv6 `/128`）；非法网段 400 |
| `rate_limit_rpm` / `quota_day` | ≥0，`0` 表示不限 |
| `dt_expire` | 空串 = 长期；支持 RFC3339 与 `2006-01-02` |

**新建默认停用**（`flag_enable='0'`）：登记 → 核对白名单与配额 → 显式启用。

### 4.3 POST `/Client/Enable`

```json
{ "idn": 1, "enable": true }
```

停用调用方会让其**全部凭证立即失效**（鉴权链路上 client 的 `flag_enable` 是总闸）。

## 5. 凭证

### 5.1 GET `/Credential/Page?clientId=&keyword=&page=&pageSize=`

`keyword` 对 `credential_no` / `access_key_id` 模糊匹配。

```json
{
  "code": 0, "message": "OK",
  "data": {
    "total": 1,
    "list": [
      {
        "idn": 1, "client_idn": 1, "client_code": "cdeu-hook",
        "credential_no": "cred-01", "auth_mode": "AK_SK",
        "access_key_id": "ak_live_ab12...", "key_prefix": "ak_live_ab12",
        "has_secret": true, "sign_algorithm": "HMAC-SHA256",
        "scope": ["POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert"],
        "allow_cidr": [], "dt_expire": "", "dt_last_use": "2026-11-11T08:10:00+08:00",
        "count_use": 42, "flag_enable": "1", "flag_revoke": "0",
        "dt_revoke": "", "reason_revoke": "", "remark": "",
        "gmt_update": "2026-11-11T08:10:00+08:00"
      }
    ]
  }
}
```

`scope` 为「方法 路径」可读列表；空数组表示该路由组全部接口。

### 5.2 POST `/Credential/Create`

```json
{
  "client_idn": 1,
  "credential_no": "cred-01",
  "auth_mode": "AK_SK",
  "sign_algorithm": "HMAC-SHA256",
  "scope": ["POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert"],
  "allow_cidr": [],
  "dt_expire": "",
  "remark": ""
}
```

| 字段 | 约束 |
|---|---|
| `client_idn` | 必须存在（404） |
| `credential_no` | 字母/数字/`._-`，2~48 位，全局唯一（409） |
| `auth_mode` | 仅 `API_KEY` / `AK_SK`；`BEARER` / `MTLS` / `BASIC` 为预留，传了即 400 |
| `sign_algorithm` | 仅 `HMAC-SHA256`（AK/SK 模式；静态 Key 忽略该字段） |
| `scope` | 每项「方法 路径」两段；方法限 `GET/POST/PUT/PATCH/DELETE/*`；路径以 `/` 开头 |
| `allow_cidr` | 同调用方口径；与调用方白名单**取交集** |

响应（**密钥明文只在此出现一次**）：

```json
{
  "code": 0,
  "message": "凭证已签发（密钥只显示一次）",
  "data": {
    "credential": { "...": "同 5.1 的凭证结构" },
    "access_key": "ak_live_ab12cd34ef56...",
    "secret_key": "sk_live_...",
    "note": "SK 只显示这一次：请立即复制保存（库里只存密文，丢失只能轮换）；请求用 Authorization: FinvAK <AK>:<签名>"
  }
}
```

- `API_KEY` 模式：`access_key` 与 `secret_key` 相同（静态 Key 本身就是公开标识），库里只存 `sha256`；
- `AK_SK` 模式：`secret_key` 是 SK，库里存 AES-256-GCM 密文（密钥取 `sys_config` 19005）。
  **19005 未配置时返回 503**，错误里明确提示先配置 19005。

### 5.3 POST `/Credential/Enable`

```json
{ "idn": 1, "enable": false }
```

已吊销的凭证不能启用/停用（409）。

### 5.4 POST `/Credential/Revoke`

```json
{ "idn": 1, "reason": "调用方已下线" }
```

- `reason` 必填（去空格后为空即 400），≤500 字符；
- 同时写 `flag_revoke='1'`、`flag_enable='0'`、`dt_revoke=now()`；
- 已吊销再吊销返回 409，并回显原吊销原因。

### 5.5 POST `/Credential/Rotate`

```json
{ "idn": 1 }
```

- 静态 Key：换 `access_key_id` + `secret_hash`，旧 Key 立即失效；
- AK/SK：换 `secret_cipher`，旧密文挪到 `secret_cipher_prev`（签名时间窗内新旧都能验签）；
- 同时重置 `dt_last_use` / `count_use`，写 `dt_secret_rotate`；
- 响应结构同签发（`access_key` / `secret_key` 为新密钥，只显示一次）。

## 6. 调用审计

### GET `/InvokeLog/Page?clientId=&credentialId=&accessKey=&flagPass=&reasonCode=&dtFrom=&dtTo=&page=&pageSize=`

| 参数 | 说明 |
|---|---|
| `flagPass` | `1` 放行 / `0` 拒绝 / 空 = 全部 |
| `reasonCode` | `NO_KEY`/`BAD_KEY`/`REVOKE`/`EXPIRE`/`CIDR_DENY`/`SIGN_MISMATCH`/`TS_SKEW`/`NONCE_REPLAY`/`SCOPE_DENY`/`RATE_LIMIT`/`NOT_READY` |
| `accessKey` | 公开标识模糊匹配（静态 Key 存的是脱敏形态） |
| `dtFrom` / `dtTo` | `RFC3339` / `2006-01-02 15:04:05` / `2006-01-02` |

按 `dt_request` 倒序。响应字段与 `open.open_api_invoke_log` 列一一对应（`body_sha256` / `count_body` 只有摘要与长度，无原文）。

## 7. 错误码

| HTTP | code | 场景 |
|---|---|---|
| 400 | `400` | 入参非法（字段格式、越界、scope 形状、scope 方法、吊销缺原因…） |
| 404 | `404` | 调用方/凭证不存在 |
| 409 | `409` | 唯一键冲突（代号/凭证号重复）、已吊销仍尝试启用或再吊销 |
| 503 | `503` | `sys_config` 19005 未配置却签发 AK/SK 凭证 |
| 500 | `2006` | 其它服务端错误（日志前缀 `[openapi][admin]`） |

对外 `/OpenAPI/V1/*` 的鉴权失败码是另一套 `190xx`（见 [`Auth.md`](Auth.md)），两者不要混用。
