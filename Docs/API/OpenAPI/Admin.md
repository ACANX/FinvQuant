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
| 1 | GET | `/Secret/Status` | 读密钥配置现状（19004 / 19005 / 19006 是否已配置 + 凭证统计） |
| 1b | POST | `/Secret/Save` | 保存 19004 / 19005 / 19006（换密钥时可选重加密现有凭证） |
| 1c | GET | `/Settings` | 读鉴权配置（19001～19003 值 + 密钥是否已配置） |
| 2 | PUT | `/Settings` | 保存 19001～19003 |
| 3 | GET | `/Client/Page` | 调用方分页 |
| 4 | POST | `/Client/Save` | 新建（`idn=0`）/ 更新调用方 |
| 5 | POST | `/Client/Enable` | 启用 / 停用调用方 |
| 6 | GET | `/Credential/Page` | 凭证分页 |
| 7 | POST | `/Credential/Create` | 签发凭证（密钥只返回一次） |
| 8 | POST | `/Credential/Enable` | 启用 / 停用凭证 |
| 9 | POST | `/Credential/Revoke` | 吊销凭证（原因必填，不可恢复） |
| 10 | POST | `/Credential/Rotate` | 轮换密钥（新密钥只返回一次） |
| 10b | POST | `/Credential/Scope` | 修改授权范围（事务双写 `scope_str` + 明细表，立即生效） |
| 11 | GET | `/InvokeLog/Page` | 调用审计分页（含 join 来的 `api_code` / `api_name`） |
| 12 | GET | `/Endpoint/Page` | 接口清单分页（需求 11223） |
| 13 | GET | `/Endpoint/Options` | 授权范围下拉可选项（公开 + 启用 + 未下线） |
| 14 | POST | `/Endpoint/Flags` | 切换接口的 `flag_public` / `flag_enable` |

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

### 3.5 密钥配置（19004 / 19005 / 19006）

#### GET `/Secret/Status`

```json
{
  "code": 0, "message": "OK",
  "data": {
    "secret_kid": "k1",
    "secret_key_ready": true,
    "fallback_key_ready": false,
    "count_credential": 2,
    "count_stale_kid": 0,
    "updated_credential": 0,
    "skipped_credential": 0
  }
}
```

`count_stale_kid` = `secret_kid` 与当前 KID 不一致、建议重加密的凭证数。**响应里永远没有密钥内容**。

#### POST `/Secret/Save`

```json
{
  "secret_kid": "k2",
  "secret_key_base64": "<32 字节的 Base64>",
  "rotate_cipher": true,
  "fallback_key": "fk_fallback_xxx",
  "clear_fallback_key": false
}
```

| 字段 | 规则 |
|---|---|
| `secret_kid` | 空 = 不改；字母/数字/`._-`，1~16 位 |
| `secret_key_base64` | 空 = 不改；非空必须是 Base64 且解码后 **32 字节**（否则 400） |
| `rotate_cipher` | 换密钥时是否重加密现有凭证密文（默认建议 true） |
| `fallback_key` | 空 = 不改；≤128 字符 |
| `clear_fallback_key` | true 时清空 19006（空串无法与「不改」区分，所以用独立开关） |

行为：

- **同一个事务**里完成「重加密 + 写配置」：只换一半会造成密文与密钥不匹配的死局。
- 重加密逐条用**旧密钥解、新密钥加密**，并把 `secret_kid` 更新为新 KID；
  旧密钥解不开的行**跳过并计入 `skipped_credential`**（不中断），由调用方重新签发。
- 首次配置（此前没有密钥）时若已存在密文，这些行一律计入 `skipped_credential`（数据异常，需人工处理）。
- 响应里 `updated_credential` / `skipped_credential` 报告本次结果，密钥内容不回显。

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
| `dt_expire` | 空串 = 长期；支持 RFC3339 与 `2006-01-02`（前端把「1 天/1 个月/1 年…」预设换算成具体日期后提交） |

服务端兜底（需求 11174）：`owner_uid` 为空时用当前登录账号；`contact_email` 为空时按 `owner_uid`
查 `sys.sys_user.email` 补（查不到则保持空）。

**新建默认停用**（`flag_enable='0'`）：登记 → 核对白名单与配额 → 显式启用。

### 4.3 POST `/Client/Enable`

```json
{ "idn": 1, "enable": true }
```

停用调用方会让其**全部凭证立即失效**（鉴权链路上 client 的 `flag_enable` 是总闸）。

## 5. 凭证

### 5.1 GET `/Credential/Page?clientId=&keyword=&status=&page=&pageSize=`

- `keyword`：对 `credential_no` / `access_key_id` 模糊匹配。
- `status`（需求 11217）：`active`（缺省）= 只返回当前可用的凭证（`flag_revoke='0'` 且 `flag_enable='1'`
  且未过 `dt_expire`）；`all` = 全部，含已吊销 / 停用 / 过期，用于追溯。其它取值返回 `400`。

```json
{
  "code": 0, "message": "OK",
  "data": {
    "total": 1,
    "list": [
      {
        "idn": 1, "client_idn": 1, "client_code": "cdeu-hook",
        "credential_no": "CRED-01", "auth_mode": "AKSK",
        "access_key_id": "AK_LIVE_AB12...", "key_prefix": "AK_LIVE_AB12",
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
  "credential_no": "CRED-01",
  "auth_mode": "AKSK",
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
| `credential_no` | 字母/数字/`._-`，2~48 位，全局唯一（409）；**字母统一大写**（服务端 `ToUpper` 后入库） |
| `auth_mode` | 仅 `APIKEY` / `AKSK`；`BEARER` / `MTLS` / `BASIC` 为预留，传了即 400 |
| `sign_algorithm` | 仅 `HMAC-SHA256`；静态 Key 不参与验签，落库统一为 `HMAC-SHA256`（列上有 NOT NULL + CHECK） |
| `scope` | 每项「方法 路径」两段；方法限 `GET/POST/PUT/PATCH/DELETE/*`；路径以 `/` 开头 |
| `allow_cidr` | 同调用方口径；与调用方白名单**取交集** |

响应（**密钥明文只在此出现一次**）：

```json
{
  "code": 0,
  "message": "凭证已签发（密钥只显示一次）",
  "data": {
    "credential": { "...": "同 5.1 的凭证结构" },
    "access_key": "AK_LIVE_AB12CD34EF56...",
    "secret_key": "SK_LIVE_...",
    "note": "SK 只显示这一次：请立即复制保存（库里只存密文，丢失只能轮换）；请求用 Authorization: META1-<算法> Credential=<AK>, Signature=<签名> + X-META-TS + X-META-Nonce"
  }
}
```

- `APIKEY` 模式：`access_key` 与 `secret_key` 相同（静态 Key 本身就是公开标识，形如 `SA_LIVE_` + 32 位大写十六进制），库里只存 `sha256`；
- `AKSK` 模式：`secret_key` 是 SK，库里存 AES-256-GCM 密文（密钥取 `sys_config` 19005）。
  **19005 未配置时返回 503**，错误里明确提示先配置 19005。
- `STS` 模式（需求 11210）：签发的同样是 `AK_LIVE_` / `SK_LIVE_`，但这对 AK/SK 是**兑换凭证**——
  只能调 `POST /OpenAPI/V1/Meta/Open/Auth/Credential/STS/Issue` 换 2 小时三要素，不能直接调业务接口；
  它填写的 `scope` 是**派生出的临时凭据**的权限（留空 = 全部接口）。
  长期 AK/SK（`AKSK`）反过来：不允许调 `STS/Issue` 派生 SST，两者刻意隔离。

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

### 4.4 GET/PUT `/Settings` 的字段（含 19007）

| 字段 | 配置号 | 说明 |
|---|---|---|
| `mode` | 19001 | `0` 关闭 / `1` 强制 / `2` 宽松 |
| `sign_time_window` | 19002 | 签名时间戳允许偏差（秒，30~3600） |
| `log_retain_days` | 19003 | 审计日志保留天数（0~3650） |
| `cred_cache_ttl` | 19007 | 读凭证的 Redis 缓存秒数（0~600；`0` = 关闭缓存） |
| `secret_kid` / `secret_key_ready` / `fallback_key_ready` | 19004~19006 | 只读回显（密钥在密钥配置页维护） |

### 5.7 POST `/Credential/Scope`（修改授权范围，需求 11230）

```json
{ "idn": 31, "scope": ["GET /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni"] }
```

| 项 | 说明 |
|---|---|
| 只改什么 | **只改授权范围**；密钥、IP 白名单、有效期、备注都不动（那些各有专用入口） |
| 事务语义 | `scope_str`（可读快照）与 `open_api_credential_scope`（鉴权读取源）在**同一事务**内双写，只改一处会出现「页面显示 A、鉴权按 B」 |
| 空数组 | `scope: []` = 不限接口（全部放行）；**静态 Key 不允许**（只能 1 个具体接口、不许通配，与签发的收紧口径一致） |
| 已吊销 | 返回 `409`，只能新建凭证 |
| 生效时间 | **立即**：写完 `INCR finv:openapi:credver` 让鉴权缓存失效，不用等 19007 的 TTL |
| 响应 | 更新后的凭证对象（`scope` 为可读的 `"方法 路径"` 列表） |

```
400 入参非法: scope 元素必须是「方法 路径」两段，实际 "GET"
400 入参非法: 静态 Key 模式只能授权 1 个接口（当前 2 个）；需要多接口请改用 AK_SK 签名模式
403 …        （不涉及；鉴权失败走 190xx）
409 状态冲突: 凭证已吊销，不能修改授权范围（只能新建）
```

### 5.6 鉴权缓存（需求 11212 / 11215）

鉴权时读取凭证走 Redis 缓存，避免每个外部请求都查一次 `open.open_api_credential`：

```
finv:openapi:credver            → 全局版本号（整数）
finv:openapi:cred:<ver>:<AK>    → 凭证快照 JSON，TTL 10 秒
```

- **通过平台**（页面或管理接口）签发、轮换、吊销、停用凭证，或改动调用方额度/白名单，
  服务层都会把版本号 `INCR` 一次——旧版本的键再也不被读取，**立即生效**，不需要等 TTL。
- 单条快照 TTL 10 秒只是兜底：**绕过平台直接改库**（`psql UPDATE`）时最长 10 秒后生效；
  紧急场景可手工 `redis-cli INCR finv:openapi:credver` 让其立刻生效。
- 缓存里存的是 SK **密文**（与库里同级别）与状态字段，不含任何明文密钥。
- **负缓存**：AK 不存在的结果同样缓存，时长与下面配置项一致，避免有人拿乱码 AK 刷接口时每个请求都打一次全表查询。
- **防击穿**：TTL 刚到期时同一 AK 的并发请求走 singleflight 合并，只有一个回源查库。
- Redis 不可用时缓存层直接回源查库：缓存是性能优化，不是正确性依赖。

缓存时长由系统配置 **19007**「OpenAPI 凭证缓存 TTL（秒）」控制，在页面
【开放平台 → 接入管理 → 鉴权设置】直接改，保存后 **10 秒内生效、不需要重建容器**：

| 取值 | 效果 |
|---|---|
| `10`（默认） | 稳态下每个活跃凭证每 10 秒最多 1 次凭证表查询（并发再合并），与调用量无关 |
| `0` | 关闭缓存，每次请求都查凭证表（调试或强制实时时可临时这么配） |
| `30`/`60` | 查库更少；直接改库的生效延迟同倍放大（平台内吊销仍然立即生效） |

按上述口径，总查询量 ≈ 活跃凭据数 / TTL 秒数；配置为 `0` 时退化为「每请求一次查表」。

## 6. 调用审计

### GET `/InvokeLog/Page?clientId=&credentialId=&accessKey=&flagPass=&reasonCode=&dtFrom=&dtTo=&page=&pageSize=`

| 参数 | 说明 |
|---|---|
| `flagPass` | `1` 放行 / `0` 拒绝 / 空 = 全部 |
| `reasonCode` | `NO_KEY`/`BAD_KEY`/`REVOKE`/`EXPIRE`/`CIDR_DENY`/`SIGN_MISMATCH`/`TS_SKEW`/`NONCE_REPLAY`/`SCOPE_DENY`/`RATE_LIMIT`/`NOT_READY` |
| `accessKey` | 公开标识模糊匹配（静态 Key 存的是脱敏形态） |
| `dtFrom` / `dtTo` | `RFC3339` / `2006-01-02 15:04:05` / `2006-01-02` |

按 `dt_request` 倒序。响应字段与 `open.open_api_invoke_log` 列一一对应；`request_id` 与 `body_sha256` 均为**大写**十六进制（需求 11177），
`body_sha256` / `count_body` 只有摘要与长度，无原文。

响应里另有两个**非审计表字段**（需求 11225）：`api_code` / `api_name`，由
`LEFT JOIN open.open_api_endpoint e ON e.http_method = l.method AND e.path = l.path` 得来，
**不落库**——接口清单里改中文名或编码后，历史报表自动跟着对齐；空值表示该路径未登记
（通常是 404/405，或接口已从清单下线）。

### 6.1 审计里的密钥怎么区分（需求 11183）

静态 Key 是密钥本身，审计只落**脱敏形态**；要区分「谁在用哪把 Key」用两级信息，不依赖明文：

| 信息 | 用途 | 稳定性 |
|---|---|---|
| `credential_idn` → join `open.open_api_credential` | **主关联键**：拿到 `credential_no`（凭证号，如 `DEMO-STATIC`）、`client_idn`、模式、状态 | 稳定，凭证吊销后行仍在 |
| `access_key_id` | 人眼核对：`AKSK` 存明文 AK；`APIKEY` 存 `SA_LIVE_C48AABCD****************EADE84AF` | 冗余快照，凭证被物理删除后仍可追溯 |
| `kfp` | **严格区分密钥**（key fingerprint）：`sha256(完整 Key)` 前 16 位大写十六进制（需求 11186，字段名 11187 定稿为 `kfp`），可 `GROUP BY` / 精确 `WHERE` | 确定性、不可逆；仅 `APIKEY` 模式写入（`AKSK` 的 AK 已明文） |

脱敏形态刻意**跳过平台前缀**（`SA_LIVE_`/`AK_LIVE_`/`SK_LIVE_`），保留随机段**前 8 后 8 位共 16 位十六进制**，
中间用**等量星号**（32 位随机段对应 16 个 `*`）替代被遮蔽的位（需求 11184、11185），
因此掩码总长与原密钥一致（`SA_LIVE_` + 32 位 = 40 字符），看得出原长、可核对位数；
同一凭证**轮换前后的两把 Key 也能区分**（区分度 `16^16` 量级，旧形态「前 8 后 4」在 `SA_LIVE_` 前缀下只剩后 4 位）。
随机段 ≤16 位时退化为「前 4 + 等量星号 + 后 4」，避免「前 8 + 后 8」覆盖整串外露明文。

按调用方聚合的常用 SQL：

```sql
SELECT l.credential_idn,
       c.credential_no,
       l.access_key_id,
       count(*)                                  AS total,
       count(*) FILTER (WHERE l.flag_pass = '1') AS pass_cnt,
       count(*) FILTER (WHERE l.flag_pass = '0') AS deny_cnt
FROM open.open_api_invoke_log l
LEFT JOIN open.open_api_credential c ON c.idn = l.credential_idn
WHERE l.dt_request >= now() - interval '1 day'
GROUP BY 1, 2, 3
ORDER BY deny_cnt DESC;
```

### 6.2 要「严格区分是哪一把 Key」时用指纹

掩码只保留部分位（前缀后前 8 后 8），两把 Key 在极端情况下可能落到同一掩码，
且**无法**用「调用方报上来的完整 Key」反查历史——因为库里只有部分明文。
因此审计另存 `kfp`（key fingerprint，`sha256(完整 Key)` 前 16 位大写十六进制，需求 11186/11187）：

- 确定性：同一把 Key 恒为同一指纹，轮换前后两把 Key 必然不同；
- 不可逆：不能从指纹还原密钥，导出日志/备份不带明文；
- 可索引：`idx_open_api_invoke_log_kfp (kfp, dt_request DESC)`。

运维排障（调用方只报了完整 Key）：在服务端算一次指纹再查，**不要把完整 Key 留在查询历史里**：

```sql
-- 指纹 = upper(substr(sha256(Key), 1, 16))，用 psql 现算可免手抄
SELECT credential_idn, key_fingerprint, count(*) AS cnt,
       max(dt_request) AS last_seen
FROM open.open_api_invoke_log
WHERE kfp = upper(substr(encode(sha256('SA_LIVE_……'), 'hex'), 1, 16))
GROUP BY 1, 2;
```

按「一把 Key」聚合（同一凭证轮换前后也能分开）：

```sql
SELECT kfp,
       min(dt_request) AS first_seen,
       max(dt_request) AS last_seen,
       count(*)        AS cnt,
       count(*) FILTER (WHERE flag_pass = '0') AS deny_cnt
FROM open.open_api_invoke_log
WHERE credential_idn = 30
GROUP BY kfp
ORDER BY last_seen DESC;
```

`kfp IS NULL` 的行是「加列之前的历史记录」或「未识别到凭证 / `AKSK` 模式」——
历史行无法补算（脱敏形态反推不出完整 Key），需要区分时以新增请求为准。

`credential_idn = 0` 表示请求**没被识别到凭证**（未携带 / Key 不存在 / 凭证明文部分不合法），
这类行只能靠 `access_key_id` 的掩码 + `ip` + `dt_request` 定位，也是排查「谁在撞门」的主要依据。

## 7. 错误码

| HTTP | code | 场景 |
|---|---|---|
| 400 | `400` | 入参非法（字段格式、越界、scope 形状、scope 方法、吊销缺原因…） |
| 404 | `404` | 调用方/凭证不存在 |
| 409 | `409` | 唯一键冲突（代号/凭证号重复）、已吊销仍尝试启用或再吊销 |
| 503 | `503` | `sys_config` 19005 未配置却签发 AK/SK 凭证 |
| 500 | `2006` | 其它服务端错误（日志前缀 `[openapi][admin]`） |

对外 `/OpenAPI/V1/*` 的鉴权失败码是另一套 `190xx`（见 [`Auth.md`](Auth.md)），两者不要混用。

## 8. 接口清单（需求 11223）

对外 OpenAPI 的接口目录，表 `open.open_api_endpoint`（`V300193`）。
**这不是鉴权白名单**：允许调哪些接口由 `internal/api/router.go` 的 `openGroup` 路由注册决定，
本表只给「授权范围下拉」提供选项、给「审计报表」提供稳定的 `api_code`。

### 8.1 GET `/Endpoint/Page?keyword=&apiGroup=&flagPublic=&flagEnable=&page=1&pageSize=20`

| 参数 | 说明 |
|---|---|
| `keyword` | 模糊匹配 `api_code` / `path` / `api_name` |
| `catalog` | 精确匹配目录（`api_code` 最后一个点之前的部分，如 `finv.quote.cdeu.min`） |
| `apiGroup` | 精确匹配分组（分钟行情 / 实时行情 / 鉴权） |
| `flagPublic` / `flagEnable` | 只允许 `0` / `1`，其余值 400（避免"传了没生效"） |

排序：`catalog` 升序、`func` 升序（按目录自然聚集）。`catalog` / `func` 为生成列（需求 11238/11239）：
`catalog` 取 `api_code` 最后一个点之前、`func` 取 `path` 最后一个斜杠之后（大驼峰），均不可写入。响应：

```json
{ "code": 0, "message": "OK", "data": { "total": 4, "list": [
  { "idn": 1, "api_code": "finv.quote.cdeu.min.pageuni",
    "catalog": "finv.quote.cdeu.min", "func": "PageUni", "http_method": "GET",
    "path": "/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni",
    "api_name": "分钟行情分页查询", "api_group": "分钟行情",
    "flag_write": "0", "flag_public": "1", "flag_enable": "1",
    "dt_deprecate": "", "remark": "…", "gmt_update": "2026-11-12T…", "count_grant": 18 } ] } }
```

`count_grant` 由子查询聚合 `open.open_api_credential_scope`（按 `method + path` 命中），
该镜像表尚未接入读写，数值仅供参照。

### 8.2 GET `/Endpoint/Options`

授权范围下拉的可选项，只返回 `flag_public='1' AND flag_enable='1' AND dt_deprecate IS NULL`，
字段同上（`data.list`，不分页）。

### 8.3 POST `/Endpoint/Flags`

```json
{ "idn": 1, "flag_public": "0", "flag_enable": "1" }
```

- 两个标志都可选，但**至少给一个**，否则 400；
- 取值只允许 `"0"` / `"1"`，其余 400；
- 返回更新后的整行（含 `count_grant`）；
- **关掉标志不回收既有权限**：已授权的凭证仍可调用，收敛权限要改凭证授权范围或吊销凭证。
