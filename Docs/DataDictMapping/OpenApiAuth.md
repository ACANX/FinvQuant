# OpenApiAuth — 外部 OpenAPI 鉴权（调用方 / 凭证 / 调用审计）

> 所属：FinvQuant 数据字典映射 · 存放：`Docs/DataDictMapping/OpenApiAuth.md`
> 数据表：`open.open_api_client` / `open.open_api_credential` / `open.open_api_invoke_log`
> 建表迁移：[`Deploy/Migrations/V300183__open_create_openapi_auth_tables.sql`](../../Deploy/Migrations/V300183__open_create_openapi_auth_tables.sql)（schema 见 `V100013`）
> 需求：11167（建表）；11168（鉴权中间件，待实现）；11169（「OpenAPI 调用方管理」菜单，待实现）
> 用途：给 `/OpenAPI/V1/*` 加鉴权。支持**静态 API Key**、**AK/SK 签名**两种主用模式，并预留 **Bearer JWT** / **双向 TLS** / **Basic** 三种。

## 1. 设计要点（先看这 8 条）

| 要点 | 结论 |
|---|---|
| 一个调用方能有几条凭证 | 多条，可同时启用**多种认证模式**（凭证表一行 = 一种模式） |
| 静态 Key 怎么存 | 只存**哈希**（`secret_hash`，sha256 或 argon2id），永不落明文；`key_prefix` 存前 8 位供列表展示与检索 |
| AK/SK 的 SK 怎么存 | **必须可逆**（HMAC 验签要用密钥本身），故存 **AES-256-GCM 密文**（`secret_cipher`），密钥取 `sys_config` 19005、KID 取 19004 |
| 会不会存错 | `ck_open_api_credential_secret` 约束：`AKSK` 只能有 `secret_cipher`、其他模式只能有 `secret_hash`，写错直接报错 |
| 密钥怎么轮换 | `secret_cipher_prev` + `dt_secret_rotate`：换钥后新旧并行一段窗口，再清旧密文，调用方无感 |
| 防重放存在哪 | **Redis**（`SET key NX PX`，保留 2 倍签名时间窗），**不建 nonce 表**（用户决策 2026-11-15） |
| 临时关闭鉴权 | `sys_config` 19001：`0` 关闭（全放行）/ `1` 启用（强制）/ `2` 宽松（校验并记日志但失败也放行，用于灰度） |
| 兼容旧调用方 | 环境变量 `FINV_OPENAPI_KEY` 与 `sys_config` 19006 任一命中即按旧口径放行（过渡期用） |

## 2. `open.open_api_client`（16 列，主键 `idn`）

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `idn` | BIGINT | PK, identity | 主键 |
| `client_code` | VARCHAR(32) | NOT NULL, UNIQUE | 调用方代号，人工识别用（`cdeu-hook`） |
| `client_name` | VARCHAR(128) | NOT NULL | 名称（「CDEU 采集钩子」） |
| `owner_uid` | VARCHAR(32) | 可空 | 负责人（对 `sys.sys_user.uid`，程序层校验，**无物理外键**） |
| `contact_email` | VARCHAR(128) | 可空 | 联系邮箱 |
| `contract_no` | VARCHAR(64) | 可空 | 合同/授权编号，审计追溯用 |
| `allow_cidr` | CIDR[] | NOT NULL DEFAULT `'{}'` | 调用方级 IP 白名单，空数组=不限；判定用 `<<=`；与凭证级取**交集** |
| `rate_limit_rpm` | INTEGER | NOT NULL DEFAULT 0, `>= 0` | 每分钟调用上限，0=不限（计数在 Redis） |
| `quota_day` | INTEGER | NOT NULL DEFAULT 0, `>= 0` | 每日配额，0=不限（计数在 Redis） |
| `dt_expire` | TIMESTAMPTZ | 可空 | 授权到期（NULL=长期） |
| `flag_enable` | CHAR(1) | NOT NULL DEFAULT `'0'`, `IN ('0','1')` | 启用标志；**新建默认停用**，核对后手工开 |
| `remark` | VARCHAR(2000) | 可空 | 备注 |
| `create_by` / `modify_by` | VARCHAR(32) | 可空 | 审计列 |
| `gmt_create` | TIMESTAMPTZ | NOT NULL DEFAULT `now()` | 创建时间 |
| `gmt_update` | TIMESTAMPTZ | NOT NULL DEFAULT `now()` | 更新时间（触发器 `trg_open_api_client_set_gmt_update` 维护） |

索引：`idx_open_api_client_enable (flag_enable) WHERE flag_enable='1'`。

## 3. `open.open_api_credential`（26 列，主键 `idn`）

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `idn` | BIGINT | PK, identity | 主键 |
| `client_idn` | BIGINT | NOT NULL | 所属调用方（程序层关联，无外键） |
| `credential_no` | VARCHAR(48) | NOT NULL, UNIQUE | 凭证号（`CRED-CDEU-01`，字母统一大写） |
| `auth_mode` | VARCHAR(16) | NOT NULL, `IN ('APIKEY','AKSK','BEARER','MTLS','BASIC')` | 认证模式 |
| `access_key_id` | VARCHAR(64) | NOT NULL, UNIQUE | **公开标识**：`APIKEY` 模式即 key 本身（`SAK_LIVE_...`）、`AKSK` 模式为 AK（`AK_LIVE_...`）、`MTLS` 模式为证书指纹；**英文字母一律大写**（需求 11176） |
| `key_prefix` | VARCHAR(12) | 可空 | 密钥前 8 位明文（展示/检索用，绝不回显全量） |
| `secret_hash` | VARCHAR(128) | 可空 | 密钥哈希：`APIKEY` 用 sha256 十六进制、`BASIC` 用 argon2id PHC 串；`AKSK` 必须为 NULL |
| `secret_cipher` | TEXT | 可空 | SK 密文（AES-256-GCM，Base64）；`AKSK` 必须有值，其他模式必须为 NULL |
| `secret_cipher_prev` | TEXT | 可空 | 轮换期旧 SK 密文（双密钥并行），窗口结束置空 |
| `dt_secret_rotate` | TIMESTAMPTZ | 可空 | 最近一次轮换时间 |
| `secret_kid` | VARCHAR(16) | 可空 | 加密密钥版本（对应 `sys_config` 19004） |
| `sign_algorithm` | VARCHAR(16) | NOT NULL DEFAULT `'HMAC-SHA256'`, `IN ('HMAC-SHA256','HMAC-SHA512','ED25519')` | 签名算法 |
| `scope_str` | JSONB | NOT NULL DEFAULT `'[]'`, `jsonb_typeof = 'array'` | 授权范围，元素必须是 `{"method":"POST","path":"/OpenAPI/V1/..."}` 对象；`[]`/`null`/空串 = 全部接口。**其它非法格式（如字符串数组）按数据损坏处理：拒绝请求（19009）**，不得当成全接口放行（需求 11190） |
| `allow_cidr` | CIDR[] | NOT NULL DEFAULT `'{}'` | 凭证级白名单，与调用方级取交集 |
| `dt_expire` | TIMESTAMPTZ | 可空 | 凭证过期（NULL=随调用方） |
| `dt_last_use` | TIMESTAMPTZ | 可空 | 最近一次认证成功（异步节流回写，可能略滞后） |
| `count_use` | BIGINT | NOT NULL DEFAULT 0, `>= 0` | 累计认证成功次数（异步累加） |
| `flag_enable` | CHAR(1) | NOT NULL DEFAULT `'1'` | 启用标志 |
| `flag_revoke` | CHAR(1) | NOT NULL DEFAULT `'0'` | 吊销标志；吊销后**不可再启用**，只能新建凭证 |
| `dt_revoke` | TIMESTAMPTZ | 可空 | 吊销时间（`flag_revoke='1'` 时必填） |
| `reason_revoke` | VARCHAR(500) | 可空 | 吊销原因（`flag_revoke='1'` 时必填） |
| `remark` | VARCHAR(2000) | 可空 | 备注 |
| `create_by` / `modify_by` | VARCHAR(32) | 可空 | 审计列 |
| `gmt_create` | TIMESTAMPTZ | NOT NULL DEFAULT `now()` | 创建时间 |
| `gmt_update` | TIMESTAMPTZ | NOT NULL DEFAULT `now()` | 更新时间（触发器 `trg_open_api_credential_set_gmt_update`） |

索引：`index (client_idn)`；部分索引 `idx_open_api_credential_active (access_key_id) WHERE flag_enable='1' AND flag_revoke='0'`（鉴权热路径只扫有效行）。

约束要点：

```sql
-- 吊销必须留痕
flag_revoke = '0' OR (dt_revoke IS NOT NULL AND NULLIF(btrim(reason_revoke),'') IS NOT NULL)
-- 密钥存储方式与认证模式互斥
(auth_mode =  'AKSK' AND secret_cipher IS NOT NULL AND secret_hash IS NULL)
OR (auth_mode <> 'AKSK' AND secret_hash   IS NOT NULL)
```

## 4. `open.open_api_invoke_log`（20 列，主键 `idn`，只增不改）

> 表名历史：`V300183` 初版叫 `open_api_call_log`，定稿改为 `open_api_invoke_log`（需求 11170，
> 既有库由 `V300184` 就地改名，索引与约束名同步由 `*_call_log*` 改为 `*_invoke_log*`）。

| 字段 | 类型 | 说明 |
|------|------|------|
| `idn` | BIGINT | 主键（identity） |
| `client_idn` / `credential_idn` | BIGINT | 调用方/凭证（`0` = 未识别，匿名请求也留痕） |
| `access_key_id` / `auth_mode` | VARCHAR | **冗余存**：凭证被删后仍可追溯。`AKSK` 模式存**明文 AK**（AK 是公开标识，用于区分调用方）；`APIKEY` 模式存**脱敏形态** `<前缀><随机段前 8><等量星号><后 8>`（如 `SA_LIVE_C48AABCD****************EADE84AF`，需求 11183/11184/11185；星号个数 = 被遮蔽的位数，掩码总长 = 原密钥长度）——静态 Key 本身就是密钥，绝不落明文 |
| `request_id` | VARCHAR(64) | 链路 ID（响应头 `X-Request-Id`），32 位**大写**十六进制（需求 11177） |
| `kfp` | VARCHAR(16) | 密钥指纹（key fingerprint）：`sha256(完整 Key)` 前 16 位**大写**十六进制，仅 `APIKEY` 模式写入；不可逆，用于严格区分密钥与按 Key 反查（需求 11186，字段名 11187 定稿 `kfp`） |
| `tfp` | VARCHAR(16) | STS SecurityToken 指纹（`sha256` 前 16 位大写十六进制）。`auth_mode='STS'` 且非空 = **临时凭据**（2 小时）；为空 = **兑换凭证**（长期 AK/SK，只能换证）。不可逆，用于按某次临时凭据聚合排查（需求 11191/11210） |
| `dt_request` | TIMESTAMPTZ | 请求时间 |
| `method` / `path` / `query_str` | VARCHAR(8)/VARCHAR(512)/VARCHAR(2000) | 请求行（查询串截断到 2000） |
| `status_code` / `biz_code` | INTEGER | HTTP 码 + 信封 `code` |
| `flag_pass` | CHAR(1) | `1` 放行 / `0` 拒绝 |
| `reason_code` | VARCHAR(32) | `NO_KEY`/`BAD_KEY`/`REVOKE`/`EXPIRE`/`CIDR_DENY`/`SIGN_MISMATCH`/`TS_SKEW`/`NONCE_REPLAY`/`SCOPE_DENY`/`RATE_LIMIT` |
| `ip` / `user_agent` | INET / VARCHAR(512) | 客户端（UA 截断到 512） |
| `body_sha256` / `count_body` | CHAR(64) / INTEGER | **只存摘要与长度**，不存原文；摘要为**大写**十六进制（需求 11177），与待签串里的 `BODY_SHA256` 同一口径 |
| `latency_ms` | INTEGER | 处理耗时（毫秒） |
| `gmt_create` | TIMESTAMPTZ | 入库时间 |

索引：`(dt_request DESC)`、`(credential_idn, dt_request DESC)`、`(flag_pass, dt_request DESC)`。
**无 `gmt_update`、无更新触发器**——日志只增不改。保留天数见 `sys_config` 19003。

## 5. 系统配置（`sys.sys_config`，19xxx 号段）

| `config_no` | 名称 | `data_type` | 默认 | 说明 |
|---|---|---|---|---|
| 19001 | OpenAPI 鉴权模式 | `2`（char） | `'0'` | `0` 关闭（全放行）/ `1` 启用（强制）/ `2` 宽松（校验 + 记日志但仍放行） |
| 19002 | 签名时间窗（秒） | `1`（int） | `300` | 超出即 `19006`；nonce 去重保留时长 = 2 倍 |
| 19003 | 调用日志保留天数 | `1`（int） | `90` | `0` = 不清理 |
| 19004 | 凭证加密密钥 KID | `3`（str） | `k1` | 与 `secret_kid` 对应，支持历史密文按各自 KID 解密 |
| 19005 | 凭证加密密钥 | `3`（str） | 空 | AES-256-GCM 密钥（32 字节 Base64）；空 = AK/SK 模式一律拒绝（`19010`）；**敏感，不预填** |
| 19006 | 兼容静态密钥（兜底） | `3`（str） | 空 | 与环境变量 `FINV_OPENAPI_KEY` 任一命中即放行；**敏感，不预填** |

## 6. 认证与签名协议

### 6.1 模式一：静态 API Key

```
X-API-Key: SA_LIVE_XXXXXXXXXXXXXXXXXXXXXXXXXXXX
或 Authorization: Bearer SA_LIVE_XXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

服务端：按 `access_key_id` 定位凭证 → `sha256(提供值)` 与 `secret_hash` 恒时比对。

### 6.2 模式二：AK/SK 签名

```
Authorization: META1-HMAC-SHA256 Credential=<AK>, Signature=<signature_base64>   （AK 形如 AK_LIVE_…，全大写）
X-META-TS: 1737000000
X-META-Nonce: 8e2f1c9a4b7d4c0e

canonical = METHOD \n
            PATH \n                                   （不含域名/端口，不含查询串）
            SORTED_QUERY \n                           （键升序、值 URL 编码后以 & 连接）
            sha256(body) 十六进制 \n                   （GET/空体 = sha256("")）
            X-META-TS \n
            X-META-Nonce

signature = base64(HMAC-SHA256(SK, canonical))
```

服务端校验顺序（任一失败即按 §1.2 对应码拒绝，且**都写调用日志**）：

1. 取 `access_key_id` 定位凭证（不存在 → `19002`）；
2. 解密 `secret_cipher`（`19005` 未配置 → `19010`）；轮换窗口内 `secret_cipher_prev` 也参与验签；
3. 状态与时效：`flag_enable` / `flag_revoke` / `dt_revoke` / client 与 credential 的 `dt_expire`（→ `19003`）；
4. IP 白名单（client ∩ credential，→ `19004`）；
5. 时间窗（`19002`，→ `19006`）；
6. nonce（Redis `SET NX PX`，→ `19007`）；
7. 签名（→ `19005`，注意与「AK 不存在」的 `19002` 区分：AK 已知才可能走到这里）；
8. 授权范围（`scope_str`，→ `19009`）；
9. 限流/配额（Redis 计数，→ `19008`）。

### 6.3 实现位置与状态（需求 11168）

| 环节 | 实现 |
|---|---|
| 中间件 | `internal/api/openapi/middleware.go`（`Middleware.Handler`，挂在 `/OpenAPI/V1` 路由组，先于 `WrapOpenAPIResponse`） |
| 凭证读取 | `internal/api/openapi/credential.go`（`PGSource.Lookup`，一条 JOIN 查询，走 `uk_open_api_credential_access_key` 唯一索引） |
| 配置 | `internal/api/openapi/settings.go`（19001～19006，10 秒 TTL 缓存，无需重启） |
| 签名 | `internal/api/openapi/sign.go`（`CanonicalString` / `Sign` / `VerifySignature`） |
| 加解密 | `internal/api/openapi/credential.go`（`EncryptSecret` / `DecryptSecret`，AES-256-GCM） |
| nonce 与限流 | `internal/api/openapi/redis_state.go`（Redis `SET NX PX` / `INCR` + TTL） |
| 审计 | `internal/api/openapi/audit.go`（同步写 `open.open_api_invoke_log`，1.5 秒超时，失败只记日志） |
| 错误码 | `internal/api/openapi/errcode.go`（`19001`～`19010`，含号段/HTTP 映射/唯一性契约测试） |

预留模式（`BEARER` / `MTLS` / `BASIC`）当前**一律按 `19010` 明确拒绝**，不做「看起来支持其实没验」。

### 6.4 预留模式

- `BEARER`：`Authorization: Bearer <JWT>`，`access_key_id` 对应 JWT `kid`，公钥放运行环境（不入表）。
- `MTLS`：由反向代理校验客户端证书并把指纹放进请求头，服务端与 `access_key_id` 比对。
- `BASIC`：`Authorization: Basic ...`，`secret_hash` 用 argon2id（复用 `internal/meta/finv/auth/password.go`）。

## 7. 生命周期

| 动作 | 做法 |
|---|---|
| 新建调用方 | 插入 `open_api_client`，`flag_enable='0'`；核对白名单与配额后再置 `'1'` |
| 新建凭证 | 生成密钥 → 明文**只在创建响应里返回一次** → 落库只存 `secret_hash`（静态 Key）或 `secret_cipher`（SK） |
| 轮换 SK | 新 SK 写入 `secret_cipher`、旧值搬到 `secret_cipher_prev`、写 `dt_secret_rotate`；窗口结束后清 `secret_cipher_prev` |
| 停用 | `flag_enable='0'`（可恢复） |
| 吊销 | `flag_revoke='1'` + 必填 `dt_revoke` / `reason_revoke`（不可恢复，只能新建凭证） |
| 排查 | 查 `open_api_invoke_log`（按 `credential_idn` / `flag_pass` / `reason_code` / `dt_request`） |

## 8. 说明

- 全库 **0 物理外键**：`client_idn`、`owner_uid`、`credential_idn` 都只在程序层校验（`DbSchemaSpec` §2）。
- 时间列口径：创建/修改统一 `gmt_create`/`gmt_update`；业务时间（请求时间、最近使用、过期、吊销、轮换）用 `dt_` 前缀。
- 日志表刻意不建 `gmt_update` 与更新触发器：审计记录不允许改。
- `scope_str` 先单列 jsonb，条目多到需要索引/分组再演进成 `open.open_api_credential_scope`（用户决议：先单列）。
- 上游 Supabase 与本域无关，`open` 域数据**不进** `brs` 备份登记之前需先评估（凭证密文含密钥材料，备份策略另行确认）。
