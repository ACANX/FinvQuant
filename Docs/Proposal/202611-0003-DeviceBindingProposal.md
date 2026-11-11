# 客户端设备绑定提案（Device Binding Proposal）

> 编号：**202611-0003**　文档约定见 [README.md](README.md)
>
> 状态：**待评审**（初步构想，未建表、未改代码）
>
> 依据数据：本机环境 `fq-postgres` / `finv`（`open` 域现有 5 张表 + 已上线握手端点 `/OpenAPI/V1/Meta/Region/AZ/Server/Hello`，需求 11242）
>
> 关联需求：**无**（提案批准并落地后才登记正式需求号；本文不占需求编号）

## 1. 问题陈述

现有对外 OpenAPI 的认证只回答「**这把 Key 是谁的**」（`open_api_credential`），不回答「**这台设备是谁**」。

后果（可核对的事实）：

| 现象 | 现状 | 为什么危险 |
|---|---|---|
| 凭证文件可被复制 | 静态 Key / AK / SK 是**磁盘上的字符串** | 从一台设备拷到另一台，平台看不出区别；审计里只有 IP/UA 可参考，均可伪造或变 NAT |
| 多设备共用一把凭证 | 平台不限制设备数量 | 一个调用方的 Key 泄露后，泄露点无法定位到设备 |
| STS 临时凭据泄露 | 2 小时内可被任意机器使用 | 换证后无法与设备绑定 |
| 审计只有被动信号 | `ip` / `user_agent` 可伪造 | 事后追责只能靠猜 |

目标一句话：**让凭证与一台具体设备绑定，使「拷走凭证文件」不能直接冒用**。

## 2. 现状核对

| 项 | 现状 |
|---|---|
| 认证层 | `APIKEY` / `AKSK` / `STS` 三类，均只校验「密钥/签名」，不校验设备（需求 11167/11210） |
| 握手端点 | `GET /OpenAPI/V1/Meta/Region/AZ/Server/Hello` 已上线（需求 11242），返回服务端身份与可用接口清单 |
| 防重放 | nonce 存 Redis（`finv:openapi:*`），已在用（需求 11188） |
| 设备相关表 | **无**（`open` 域共 5 张：`open_api_client` / `open_api_credential` / `open_api_credential_scope` / `open_api_endpoint` / `open_api_invoke_log`） |
| 设备信息采集 | 仅审计表 `ip` / `user_agent` 两列（HTTP 层，客户端自报） |
| 移动端 | 尚无 Android 客户端代码（本提案不依赖具体实现） |

## 3. 目标与非目标

### 3.1 目标

1. **设备身份**：每台客户端设备有一对**不可导出**的密钥，平台存公钥；
2. **绑定可审计**：谁能看到「哪台设备绑到了哪个调用方」；
3. **防复制**：把凭证文件拷到另一台机器，**无法**通过设备校验（这是唯一真正能达成的安全目标）；
4. **可撤销**：设备丢失/换机可解绑、吊销，且留下原因与时间；
5. **够轻**：不引入 CA、不要求企业 MDM。

### 3.2 非目标（明确不做，防范围蔓延）

| 不做 | 原因 |
|---|---|
| 采集 IMEI / MAC / 序列号 / 手机号 | 《个人信息保护法》下的敏感个人信息，最小必要原则下不需要；且 Android 10+ 已限制 IMEI |
| 用「设备型号/OS/架构」做认证因子 | 客户端自报、可任意伪造；只能作风控信号与展示 |
| 防「设备被 root 后就地冒充」 | 纯 HTTP 协议做不到；需要 Play Integrity / Key Attestation（见 D8） |
| 替换现有的 APIKEY/AKSK/STS | 设备绑定是**附加层**，不是新的认证模式 |
| 做通用 MDM/设备管理平台 | 超出本项目范围 |
| 绑定后强制所有接口 | 见 D5，建议按凭证开关逐步启用 |

## 4. 对原始构想的问题指正

原始设想（概括）：握手后 Android 上报设备信息 → 服务端下发设备凭据（本地不可删除）→ 客户端用凭据生成身份标识 → 服务端校验通过后生成验证码并存表 → Android 在限时内**手动输入验证码** → 相等即绑定完成。

逐条指出问题：

### 4.1 「设备凭据下发到本地、不能删除」——不可强制，且方向反了

- 客户端**总能**删除：清应用数据、卸载重装、root 后改文件。把它写成"不能删除"只是策略，不是技术保证；
- 更糟的是**下发**：凭据经过网络传输 → 可能被中间人/日志/抓包拿到；一旦拿到，攻击者**不需要设备**就能冒充；
- **建议改为**：客户端在 Android Keystore（优先 StrongBox/TEE）**本地生成** EC P-256 密钥对，私钥**永不出设备、永不上传**，只上送公钥。这样"拷走文件"也无用——私钥无法导出。
  这也顺带满足你想要的"设备凭据保存在本地"的实质：本地有的是**私钥**，服务端只有公钥。

### 4.2 「用设备凭据生成身份标识」——若是静态值，可重放

- 若客户端计算 `hash(device_credential)` 后每次都发同一个值 → 抓一次包就能永久重放；
- **必须改为挑战-应答**：服务端下发一次性 `challenge`（随机 32 字节，Redis TTL 10 分钟），
  客户端签名 `sign(challenge || bind_id || timestamp)`，服务端用公钥验签。**每次绑定/每次设备校验都是新 challenge。**

### 4.3 「服务端生成验证码 → Android 手动输入」——如果码也返回给 Android，这一步等于零

这是原始构想里**最关键的一个漏洞**：

```
若：服务端生成码 → 接口把码返回给 Android → Android 再把这个码输入回来
则：谁都能"自己给自己发码"，带外确认（out-of-band）彻底失效
```

验证码的唯一价值是**两路独立**：一路在设备上，另一路在**人**（或另一台已受信设备）上。所以必须先定义「谁看到码、谁输入码」：

| 形态 | 码显示在哪 | 谁输入/批准 | 适用 | 强度 |
|---|---|---|---|---|
| **A. 管理端 → 设备**（推荐） | 管理员在 Web「开放平台 → 设备绑定」页看到码 | 现场人员在 Android 上输入 | 新增设备、现场装机、运维上门 | **强**：证明「持有登录态的管理员」和「持有设备的现场人员」同时在场 |
| **B. 设备 → 管理端** | Android 显示码 | 管理员在自己后台核对后点「批准」 | 类似 WhatsApp/Signal 设备联机 | 强，交互更顺（无需口头传码） |
| C. 设备 → 设备自己 | 设备显示 | 同一个人在设备上输入 | 无第三方 | **弱**：只能防误绑，防不了设备已被控 |

明确建议：**默认 A，可选 B；C 明确不作为安全手段，仅用于"确认操作没点错"。**

### 4.4 「上报设备信息」——必须标注可信度

| 字段 | 可信 | 用途 |
|---|---|---|
| `install_id`（客户端生成的随机 UUID，存 Keystore） | 中（可被改，但配合公钥签名后不可抵赖） | 识别"同设备重绑" |
| 公钥 | **高**（Keystore 生成的密钥可证明持有私钥） | 认证因子 |
| 型号 / OS / OS 版本 / 架构 / App 版本 | **低**（客户端自报） | 仅风控信号、展示、审计；**不得**作为认证依据 |
| IP / UA | 低 | 审计 |

设备信息字段建议**宽松采集**：客户端拿不到时传空串，服务端不得因此拒绝绑定（否则低端机/隐私模式下无法接入）。

### 4.5 验证码本身的强度不足

6 位数字 = 10^6 空间，若无约束可被脚本爆破。必须同时满足：

- TTL ≤ 10 分钟；
- 最多 5 次尝试，超限作废该 `bind_id`（需重新 Apply）；
- 一次性（用后即废）；
- 服务端**只存哈希**（sha256/HMAC），不存明文；
- 使用 `crypto/rand`（不是 `math/rand`）；
- 验证码与 `bind_id` 强绑定，不能跨申请使用。

### 4.6 缺少「换机 / 解绑 / 设备丢失」路径

只有"绑定"没有"解除"，一次误绑就是永久僵尸行。必须定义：

| 场景 | 处理 |
|---|---|
| 同设备重装 App | `install_id` 存 Keystore（清数据才会丢）→ 命中 `(client_idn, install_id)` 唯一键 → 复用原设备行，重新走 Prove 即可（不新增设备） |
| 换新机器 | 新设备走 Apply → 管理端批准 → 旧设备行 `flag_revoke='1'` + `reason_revoke`（必填） |
| 设备丢失 | 管理端直接吊销；若该凭证开启了强制设备校验，该设备立即失效 |
| 误绑撤销 | 同上，全程留痕（吊销时间 + 原因） |

### 4.7 与现有三类凭证的关系没定义（必须先拍板）

设备绑定是**附加层**还是**替代认证**？两种设计的复杂度差一个数量级：

| 设计 | 含义 | 影响 |
|---|---|---|
| **仅登记（弱）** | 绑定后只记录"这台设备属于哪个调用方"，业务请求不校验设备签名 | 只能做审计与统计，安全收益有限；改动小（1 张表 + 2 个接口 + 1 个页面） |
| **强制 PoP（强）** | 凭证上加 `flag_device_bind`，开启后业务请求必须带设备签名 | 真正防复制；但会改变 11210 的三类凭证语义，需要单独批准；客户端所有请求都要改 |

建议：**先做弱、后加强**（D1）。

## 5. 建议的流程（草案）

```
Phase 0  握手（已有）      GET  /Meta/Region/AZ/Server/Hello                 → 确认网络/认证/平台
Phase 1  绑定申请          POST /Meta/Region/AZ/Server/Bind/Apply            → 得 bind_id + challenge
Phase 2  设备证明          POST /Meta/Region/AZ/Server/Bind/Prove            → 验签通过，生成 binding code
Phase 3  带外确认          POST /Meta/Region/AZ/Server/Bind/Confirm          → 码正确 → 设备 ACTIVE
（Phase 4 业务请求，可选）  每次请求带 X-Finv-Device + 设备签名                → 强制模式才校验
```

### 5.1 Phase 1：`Bind/Apply`

- **认证**：`AKSK` 签名（建议不用 `STS` 临时凭据——2 小时到期会让绑定中途失败；也不建议静态 Key，因为要验签设备请求）；
- **请求体**：

```json
{
  "install_id": "6f1c3b8e-…",              // 客户端生成的 UUID，存 Keystore
  "device_pubkey": "MFkwEwYHKoZIzj0C…",    // EC P-256 SPKI，Base64
  "key_alg": "ECDSA-P256-SHA256",
  "device": {                               // 全部可选、自报、可空串
    "model": "Pixel 8", "os": "Android", "os_version": "15",
    "arch": "arm64-v8a", "app_version": "1.0.0"
  }
}
```

- **服务端**：
  1. 校验凭证/调用方状态与额度（复用现有中间件）；
  2. `install_id` + `client_idn` 命中已 ACTIVE 设备且公钥相同 → 直接返回「已绑定」（幂等）；
  3. 否则在 `open_device_bind` 插一行 `state='PENDING'`，生成 `bind_id`（32 位大写十六进制）+ `challenge`（32 字节，存 Redis `finv:openapi:bind:<bind_id>` TTL 600s）；
  4. 返回 `bind_id` / `challenge` / `challenge_expire_seconds`。

### 5.2 Phase 2：`Bind/Prove`

- **请求**：`{ "bind_id": "…", "signature": "Base64(ECDSA(privkey, challenge || bind_id || ts))", "ts": 1794427291 }`
- **服务端**：取 challenge → 验签（公钥来自 Phase 1）→ `state='PROVEN'` → 生成 **binding code**（6 位数字，HMAC 存库，TTL 600s，`count_try=0`）→
  - **按 D3-A**：响应**不返回码**，码写进「设备绑定」管理页（管理员看到后口述/IM 给现场人员）；
  - **按 D3-B**：响应返回码给设备显示，管理员在后台看到同一码并点「批准」。
- 统一返回 `code_expire_seconds`。

### 5.3 Phase 3：`Bind/Confirm`

- **请求**：`{ "bind_id": "…", "code": "042917" }`
- **服务端**：校验未过期、尝试次数 < 5 → 比对哈希 →
  - 成功：`open_device` 插一行（`device_id`、`pubkey`、`install_id`、自报设备信息、`dt_bind`），`open_device_bind.state='ACTIVE'`；
  - 失败：`count_try += 1`，返回 `19020`（设备绑定验证码错误）；超限作废。

### 5.4 Phase 4（可选，强制模式）

业务请求增加：

```
X-Finv-Device: <device_id>
X-Finv-Device-Ts: 1794427291
X-Finv-Device-Sig: Base64(ECDSA(privkey, METHOD || PATH || BODY_SHA256 || device_id || ts))
```

服务端在既有鉴权之后追加一层：凭证 `flag_device_bind='1'` 时校验设备签名与 `device_id` 归属。

## 6. 数据模型（草案，未建表）

两张新表，命名对齐现有 `open_api_*`，全部**无物理外键**（`DbSchemaSpec` 红线）：

### 6.1 `open.open_device`（已绑定设备）

| 列 | 类型 | 说明 |
|---|---|---|
| `idn` | BIGINT PK | 自增 |
| `device_id` | VARCHAR(32) UNIQUE | 服务端生成的业务标识（给客户端用） |
| `client_idn` | BIGINT | 所属调用方（程序层校验，无 FK） |
| `install_id` | VARCHAR(64) | 客户端生成，`UNIQUE (client_idn, install_id)` |
| `pubkey` | TEXT | 设备公钥（SPKI Base64），`UNIQUE`（同公钥不能绑两次） |
| `key_alg` | VARCHAR(24) | `ECDSA-P256-SHA256` |
| `device_model` / `os` / `os_version` / `arch` / `app_version` | VARCHAR | **自报、不可信**，仅展示与审计 |
| `dt_bind` / `dt_last_seen` | TIMESTAMPTZ | |
| `flag_enable` / `flag_revoke` / `dt_revoke` / `reason_revoke` | | 吊销留痕（原因必填，与凭证同口径） |
| `remark` / `create_by` / `modify_by` / `gmt_create` / `gmt_update` | | 审计列 |

### 6.2 `open.open_device_bind`（绑定申请流水）

| 列 | 说明 |
|---|---|
| `bind_id` | VARCHAR(32) UNIQUE，申请标识 |
| `client_idn` / `credential_idn` | 申请人 |
| `install_id` / `pubkey` / `key_alg` / 设备自报信息 | 申请时快照 |
| `state` | `PENDING` / `PROVEN` / `ACTIVE` / `REJECTED` / `EXPIRED` |
| `code_hash` | 验证码哈希（不存明文） |
| `count_try` | 已尝试次数（上限 5） |
| `dt_expire` / `dt_create` | |
| `ip` / `user_agent` | 申请来源（复核用） |

> 若 D1 选「仅登记」，只需 6.1 + 6.2 中 PENDING/PROVEN/ACTIVE 三态；若选「强制 PoP」，`open_api_credential` 需加一列 `flag_device_bind`（属既有表加列，走独立迁移）。

## 7. 待确认决策点

| # | 决策点 | 选项 | 建议 |
|---|---|---|---|
| **D1** | 绑定后的约束强度 | A 仅登记（审计用）/ B 强制 PoP（业务请求必须设备签名） | **先 A 后 B**：A 先把表与流程跑通，B 单独立项批准 |
| **D2** | 设备密钥形态 | A 客户端生成非对称密钥对（私钥不出 Keystore）/ B 服务端下发对称密钥 | **A**（B 无法防复制，且密钥经网络传输） |
| **D3** | 验证码两路 | A 管理端显示 → 设备输入 / B 设备显示 → 管理端批准 / C 设备自产自销 | **A 默认，B 可选；C 不用于安全** |
| **D4** | 绑定粒度 | A 调用方级 / B 凭证级 / C 用户账号级 | **A**（与 `open_api_client` 对齐；凭证轮换不影响设备） |
| **D5** | 是否所有凭证强制设备校验 | A 全部 / B 按凭证 `flag_device_bind` 开关 / C 不做 | **B**，默认关闭，逐个启用 |
| **D6** | 设备信息采集范围 | A 仅型号/OS/OS 版本/架构/App 版本（可空）/ B 加安装 ID + 屏幕/语言 / C 采集硬件唯一号 | **A**；C 明确不做（合规 + 平台限制） |
| **D7** | 换机/解绑 | A 管理端手动吊销旧设备 / B 自动以新设备替换 / C 需要旧设备在线确认 | **A**（最可审计）；B 有被冒用风险 |
| **D8** | 是否接入 Play Integrity / Key Attestation | A 不做 / B 接入（可检测 root/改机） | **先 A**；等有真实攻击面再评估（成本高、需 Google 账号链路） |
| **D9** | 绑定接口的认证模式 | A 只允许 AKSK / B AKSK + 静态 Key / C 含 STS | **A**（要验签设备请求，静态 Key 无法签名；STS 2 小时太短） |
| **D10** | `device_id` 传递方式 | A 独立头 + 独立签名段 / B 并入现有 AKSK 待签串 | **A**（不破坏现有签名协议；只在强制模式下多一段） |
| **D11** | 绑定码错误码号段 | 用 `19020`（新号）还是复用 `19002` | **新增 `19020`**（`ErrorCodeSpec` 登记），便于审计区分"设备码错"与"密钥错" |
| **D12** | 未绑定设备能否先调业务接口 | A 能（灰度期，靠凭证开关）/ B 不能 | **A**（否则上线即锁死所有现有调用方） |

## 8. 风险与红线

| 风险 | 说明与对策 |
|---|---|
| **新增表属结构调整** | 建表要走迁移 + 需求登记；`open` 域现有 5 张表**不得改名/合并**（`DbSchemaSpec` §0 红线） |
| **给既有表加列** | 若 D1 选 B，需给 `open_api_credential` 加 `flag_device_bind`，走独立迁移（`ADD COLUMN IF NOT EXISTS`，不删不改既有列） |
| **改变既有认证语义** | 强制 PoP 会让"当前能调的客户端突然不能调"，必须灰度 + 默认关闭 + 明确回退开关 |
| **设备信息合规** | 只采集最小集，页面/文档写明"自报、不可信、仅展示"；不采集硬件唯一号 |
| **验证码暴力破解** | TTL 10 分钟 + 5 次尝试 + 一次性 + 服务端只存哈希 + `crypto/rand` |
| **私钥可导出风险** | 要求 Android 侧使用 `AndroidKeyStore` + `setUserAuthenticationRequired(false)` 明确评估；不得落到 `SharedPreferences` 明文 |
| **时钟偏差** | 设备签名带 `ts` 时同样受 19002 时间窗约束，客户端需先经 `/Hello` 校时 |
| **单点故障** | `challenge` 存 Redis；Redis 不可用时绑定应**拒绝**（fail-close，与现有 nonce 策略一致） |

## 9. 阶段划分（建议）

| 阶段 | 内容 | 产出 |
|---|---|---|
| P0 | 本提案评审，D1–D12 拍板 | 决策记录（本文件状态改「已批准」） |
| P1 | 建表（`open_device` / `open_device_bind`）+ `Bind/Apply` + `Bind/Prove`（不生成码，仅验签） | 迁移 + 接口 + 单测 + 文档 |
| P2 | `Bind/Confirm`（验证码）+ 管理端「设备绑定」页面（列表 / 批准 / 吊销） | 页面 + 接口 + 文档 |
| P3 | 强制 PoP（若 D1 选 B）：凭证开关 + 业务请求设备签名校验 + 审计列 `device_id` | 迁移 + 中间件 + demo |
| P4 | Android 端参考实现与联调脚本（放 `FinvQuantDesign/`） | 客户端示例 |

## 10. 验收标准（初版，P1/P2 用）

| 阶段 | 可执行核对 |
|---|---|
| P1 | `curl -X POST .../Bind/Apply`（AKSK 签名）返回 `bind_id` + `challenge`；用错误私钥签名调 `Bind/Prove` 返回 `19021`；正确私钥返回 `state=PROVEN` 且 Redis 里 `finv:openapi:bind:<bind_id>` 存在 |
| P1 | 同一 `install_id` 重复 Apply → 不新增行（`state` 仍是原行），幂等 |
| P2 | 管理页看到 6 位码；`Bind/Confirm` 输错 5 次后该 `bind_id` 作废（再试返回 `19022`）；正确码 → `open_device` 新增一行且 `flag_revoke='0'` |
| P2 | 吊销设备：`flag_revoke='1'` + `dt_revoke` + `reason_revoke` 三者齐全（CHECK 约束） |
| P3（若做） | 凭证 `flag_device_bind='1'` 时，不带设备签名调业务接口返回 `19023`；带正确签名返回 200；审计 `device_id` 有值 |

## 11. 与现有文档的关系

| 文档 | 关系 |
|---|---|
| [`Docs/OpenAPI/Region/AZ/Server/Hello.md`](../OpenAPI/Region/AZ/Server/Hello.md) | 握手是 Phase 0；本文的绑定流程在其之后 |
| [`Docs/API/OpenAPI/Auth.md`](../API/OpenAPI/Auth.md) | 设备绑定复用其签名/时间窗/nonce 口径 |
| [`Docs/DataDictMapping/OpenApiAuth.md`](../DataDictMapping/OpenApiAuth.md) | 批准后新增 `open_device*` 表的数据字典 |
| [`Docs/DevSpec/DbSchemaSpec.md`](../DevSpec/DbSchemaSpec.md) | 新增表要更新 `open` 域表清单；建表前自检 §0 红线 |
| [`Docs/DevSpec/ErrorCodeSpec.md`](../DevSpec/ErrorCodeSpec.md) | 本文建议新增 `19020`（码错）/ `19021`（设备签名不符）/ `19022`（申请作废）/ `19023`（设备未绑定） |
