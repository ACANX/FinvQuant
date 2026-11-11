# 客户端设备绑定提案（Device Binding Proposal）

> 编号：**202611-0003**　文档约定见 [README.md](README.md)
>
> 状态：**待评审**（范围已按 2026-11-11 反馈收窄：本期只做「绑定登记 + 人工审核 + 自动解绑」，
> 不做全局强制设备校验、不做设备管理平台）
>
> 依据数据：本机环境 `fq-postgres` / `finv`（`open` 域现有 5 张表 + 已上线握手端点 `/OpenAPI/V1/Meta/Region/AZ/Server/Hello`，需求 11242）
>
> 关联需求：**无**（提案批准并落地后才登记正式需求号；本文不占需求编号）

## 1. 定位：这是一层"可信标记"，不是认证体系

设备绑定的用途只有一句话：

> 让平台知道「**这次请求来自一台经过人工审核、且持有不可导出私钥的设备**」，
> 供**高安全性操作**（敏感配置变更、批量导出、权限调整等）在业务侧按需引用。

因此本期**不做**：

| 不做 | 原因 |
|---|---|
| 全局中间件强制设备签名（所有接口都校验） | 会让现有调用方"上线即锁死"；绑定只服务于少数高安全操作 |
| 设备管理平台（批量下发/远程擦除/统计报表） | 超出当前需要 |
| Play Integrity / Key Attestation / 防 root | 成本高、依赖 Google 链路；等真有攻击面再评估 |
| 用设备型号/OS/架构做认证因子 | 客户端自报、可伪造；只作风控信号与展示 |

## 2. 问题陈述（现状事实）

| 现象 | 现状 | 后果 |
|---|---|---|
| 凭证是磁盘上的字符串 | 静态 Key / AK / SK 可被复制 | 从一台机器拷到另一台，平台看不出区别 |
| 无设备维度 | 平台不记录"这把 Key 在哪台设备上用" | 泄露后无法定位泄露点 |
| 审计只有被动信号 | 仅 `ip` / `user_agent`，均可伪造 | 事后追责只能靠猜 |

目标：使「**拷走凭证文件**」不能在高安全操作上直接冒用。

## 3. 现状核对

| 项 | 现状 |
|---|---|
| 认证层 | `APIKEY` / `AKSK` / `STS` 三类，只校验密钥与签名（需求 11167/11210） |
| 握手端点 | `GET /OpenAPI/V1/Meta/Region/AZ/Server/Hello` 已上线（需求 11242） |
| 防重放 | nonce 存 Redis `finv:openapi:*`，已在用（需求 11188） |
| 设备相关表 | **无**（`open` 域 5 张表：client / credential / credential_scope / endpoint / invoke_log） |
| 设备信息采集 | 仅审计表 `ip` / `user_agent` |

## 4. 已确认的设计

### 4.1 设备密钥：客户端本地生成，私钥不出设备（已定）

- Android 侧在 **Android Keystore**（优先 StrongBox/TEE）生成 **EC P-256** 密钥对；
- **只上送公钥**（SPKI Base64），私钥永不导出、永不上传；
- 平台存公钥，用于验签。
- 结论：即使整个应用数据目录被拷走，也**无法在另一台设备上签名**。

> 原设想「服务端下发设备凭据、本地不可删除」不采纳：客户端总能删除（清数据/卸载/root），
> 「不可删除」是策略不是技术保证；且下发的凭据要过网络，一旦泄露**不需要设备**就能冒充。
> 改为本地生成后，你要的"凭据保存在本地"以"私钥保存在 Keystore"的形式实现，且不可复制。

### 4.2 设备证明：挑战应答（已定）

- Phase 1 服务端下发一次性 `challenge`（32 字节随机，Redis TTL 10 分钟）；
- 客户端签名 `sign(challenge || bind_id || ts)`；
- 服务端用公钥验签，**不复用任何静态标识**（静态标识被抓包即可重放）。

### 4.3 验证码：只存服务端表，只经受信通道展示，绝不通过设备接口返回（已确认）

这条按你的原意明确写清，作为**硬约束**：

| 规则 | 说明 |
|---|---|
| 码保存在服务端表 | `open_device_bind` 的码列；**表里只存哈希**（见 4.3.1） |
| 只有受信人员能获取 | 通过**管理端页面**（登录态 + 审批动作）展示，属于受信通道 |
| **绝不经 `/OpenAPI/V1/*` 响应返回给设备** | 设备侧永远拿不到"服务端生成的那个码"，只能由人读给它 |
| 人工审核在前 | 审批人在管理端看到申请信息（调用方 / 凭证 / 设备型号 / 申请 IP / 时间）并**确认批准**，码才生成/展示 |

流程图（两路独立，这是它唯一的安全价值）：

```
申请设备 ──Prove(验签通过)──► [待审核] ──人工审核批准──► 生成码
                                                         │
                                   管理端页面显示码（受信通道，只显示一次）
                                                         │
                                       审批人口述/IM 给现场人员
                                                         ▼
申请设备 ◄──────── 人在 Android 上手动输入码 ────────► Confirm 比对哈希 → ACTIVE
```

#### 4.3.1 表里存哈希还是明文

建议：**存哈希 + 审批时生成、只展示一次**。

- 审批人点「批准」时服务端生成 6 位码 → 存 `code_hash`（HMAC-SHA256）→ 该次响应把明文码返回给**管理端**（受信通道，仅此一次）；
- 之后任何查询都读不到明文（页面刷新需重新走审批/重发）；
- 好处：即使库被读，也拿不到有效码。

若你更希望"后台随时能查看当前码"，那就得存明文或可逆密文——安全性略降但操作更省心，作为 §7 的 K3 由你选。

### 4.4 人工审核

- 审批人是**已登录的平台账号**（沿用现有登录会话），审批动作写 `approve_by` / `dt_approve`；
- 本期**不做审批角色细分**：任何能打开「设备绑定」页的账号都可审批（与现有「开放平台」其他页面一致）；
- 审批可驳回：`state='REJECTED'` + 原因。

### 4.5 客户端文件丢失 → 自动解绑（本次新增）

客户端删除本地文件（或清除应用数据）后，私钥已不存在，任何后续证明都会失败。按你的要求做成**自动解绑**，分两条路径：

| 路径 | 触发 | 服务端动作 |
|---|---|---|
| **主动上报** | 客户端发现私钥缺失，调用 `POST .../Bind/Unbind`（用 AKSK 认证，`{ "device_id": "…", "reason": "CLIENT_KEY_LOST" }`） | 该设备 `flag_revoke='1'` + `dt_revoke` + `reason_revoke`，`state` 置 `UNBOUND` |
| **被动检测** | 同一设备在 Prove 或（后续的）设备签名校验中**连续 3 次**验签失败，且两次失败间隔 ≥ 10 分钟（避免网络抖动/时钟偏差误伤） | 同上，`reason_revoke='KEY_INVALID'`；管理页标红提示 |

- 解绑后该设备需**重新走完整绑定流程**（Apply → Prove → 审核）；
- 解绑不删除行（留审计），只置吊销标志——与凭证吊销同口径。

### 4.6 设备信息的可信度（写清边界）

| 字段 | 可信度 | 用途 |
|---|---|---|
| 公钥（Keystore 生成） | **高**（能证明持有私钥） | 认证因子 |
| `install_id`（客户端生成 UUID） | 中（配合签名后不可抵赖） | 识别"同设备重装" |
| 型号 / OS / OS 版本 / 架构 / App 版本 | **低**（自报，可伪造） | 仅展示、风控信号、审计 |
| IMEI / MAC / 序列号 / Android ID | **中**（客户端自报，root/改机可伪造；但配合公钥签名后可用于唯一性判定与人审） | 设备唯一性识别、审批时人工核对、事后追责 |
| 客户端取不到的字段 | — | 传空串 + 在 `id_missing` 里标出缺了哪几项；**不得**因缺字段让接口报错 |

### 4.7 硬件标识采集（IMEI / MAC / 序列号 / Android ID）

绑定申请**采集**以下标识，全部入库、参与设备唯一性判定：

| 字段 | 取值 | Android 上的实际情况 |
|---|---|---|
| `serial` | `Build.getSerial()` | 唯一性最好。**普通应用**在 Android 10+ 需要特权权限，取不到返回 `"unknown"`；**设备所有者 / 系统应用 / 厂商预装**可取 |
| `imei` | `TelephonyManager.getImei()` | 普通应用 Android 10+ 已不可取（需 `READ_PRIVILEGED_PHONE_STATE`）；平板/无 SIM 设备本身没有 |
| `mac` | 网卡硬件地址（`wifi0` / `eth0`） | Android 6+ 对应用隐藏，返回固定假值 `02:00:00:00:00:00`；专用设备可用 `NetworkInterface` 或系统权限取真实值 |
| `android_id` | `Settings.Secure.ANDROID_ID` | 始终可取；Android 8+ 按「应用签名 + 用户 + 设备」生成，卸载重装不变、恢复出厂会变 |
| `install_id` | 客户端生成 UUID，存 Keystore | 始终可取；清应用数据/卸载即丢 |

**设备唯一性判定优先级**（服务端比对顺序）：

```
serial > imei > mac > android_id > install_id
```

命中任一已绑定设备 → 视为同一台设备（走"重绑/复用"分支），避免一条设备因字段缺失被重复登记。

**缺失处理**：允许空值，缺失项记入 `id_missing`（如 `"IMEI,MAC"`），管理端审批页对该申请标黄显示。
若你的客户端是**设备所有者 / 系统应用 / 自有硬件**，这些字段可全部取到——那种情况下可把
「`serial` 必填」提为硬校验（见 §7 的 K4）。

> 注意：即便是真实取到的 IMEI/MAC，root 后也能改（Xposed/改机工具），因此它们只用于**唯一性识别与人工核对**，
> 真正不可伪造的仍然是「Keystore 私钥能签名」这件事。

## 5. 流程（详细）

### 5.0 状态机

```
                 Apply                 Prove              人工批准
   (无) ──────────────► PENDING ──────────► PROVEN ──────────────► APPROVED
                          │                   │                       │
                          │ 10min 未 Prove    │ 24h 未审核             │ Confirm 成功
                          ▼                   ▼                       ▼
                       EXPIRED ◄────────────────────────────────── ACTIVE
                          ▲                   │ 审批驳回              │
                          │                   ▼                      │ Unbind / 连续验签失败
                          └────────────── REJECTED                   ▼
                                                                  UNBOUND
```

| 状态 | 含义 | 谁能推进 | 停留上限（超时后） |
|---|---|---|---|
| `PENDING` | 已受理申请，等设备证明 | 设备（Prove） | 10 分钟 → `EXPIRED`（challenge TTL） |
| `PROVEN` | 设备已证明持私钥，**等人工审核** | 审批人（批准/驳回） | 24 小时 → `EXPIRED` |
| `APPROVED` | 已批准并出码，等人在设备上输入 | 人 + 设备（Confirm） | 10 分钟 → `EXPIRED`（码期） |
| `ACTIVE` | 绑定生效 | — | 长期，直到解绑 |
| `REJECTED` | 审批驳回（带原因） | 终态，可重新 Apply | — |
| `EXPIRED` | 任一环节超时/输错作废 | 终态，可重新 Apply | — |
| `UNBOUND` | 已解绑（客户端丢钥 / 人工解绑） | 终态，可重新 Apply | — |

一条铁律：**验证码只在 `APPROVED` 之后存在，且只经管理端（内部接口 + 登录态）返回一次，永不进入 `/OpenAPI/V1/*` 的响应。**

---

### 5.1 `Bind/Apply`（设备发起）

```
POST /OpenAPI/V1/Meta/Region/AZ/Server/Bind/Apply
认证：AKSK 签名（X-META-TS / X-META-Nonce 照常）
```

**请求体**

| 字段 | 必填 | 约束 | 说明 |
|---|---|---|---|
| `install_id` | ✅ | UUID，≤64，大写归一 | 客户端生成后存 Keystore（清数据才丢） |
| `device_pubkey` | ✅ | Base64 SPKI，≤512，须能解出 EC P-256 | 私钥永不上传 |
| `key_alg` | — | 默认 `ECDSA-P256-SHA256` | 白名单校验 |
| `hardware.serial` | — | ≤64 | 取不到传空串 |
| `hardware.imei` | — | 15 位数字，非法则置空并记缺失 | |
| `hardware.mac` | — | 归一为 `AA:BB:CC:DD:EE:FF`；取到假值 `02:00:00:00:00:00` 视为缺失 | |
| `hardware.android_id` | — | ≤64 | |
| `device.model` / `os` / `os_version` / `arch` / `app_version` | — | ≤64 | 自报，仅展示 |

**服务端处理顺序（8 步）**

1. 中间件认证：凭证有效、未吊销、`auth_mode=AKSK`、限流通过；
2. 入参校验（长度、字符集、公钥可解析、`key_alg` 白名单）→ 失败 `19024`；
3. 硬件标识归一：`mac` 大写、`imei` 校验 15 位数字、空值/非法值计入 `id_missing`；
4. 唯一性判定（顺序 `serial > imei > mac > android_id > install_id`）查 `open_device`：
   - 命中 **ACTIVE 且公钥相同** → **幂等返回** `{already_bound:true, device_id}`，不新建申请；
   - 命中 **ACTIVE 但公钥不同** → 按"同机换钥/重装"处理：新建申请并记 `rebind_of_device_id`（旧设备在 Confirm 成功后自动吊销，原因 `REBIND`）；
   - 命中 **已吊销/已解绑** → 允许新建申请（旧行保留审计）；
   - 未命中 → 新建申请；
5. 清理同一 `(client_idn, install_id)` 下已超时的 `PENDING` / `PROVEN` / `APPROVED` 申请 → `EXPIRED`（惰性清理，无需定时任务）；
6. 生成 `bind_id`（32 位大写十六进制）与 `challenge`（32 字节随机 → 64 位十六进制）；
7. 写 `open_device_bind`：`state=PENDING`、快照（公钥 / 硬件标识 / 自报设备信息 / `ip` / `user_agent`）、`dt_expire=now+600s`；
8. Redis `SET finv:openapi:bind:<bind_id> = <challenge> EX 600`。

**响应**

```json
{ "code": 0, "data": {
    "bind_id": "A1B2…(32 位)",
    "challenge": "9F3C…(64 位)",
    "expire_seconds": 600,
    "already_bound": false
} }
```

**失败码**：`19024` 入参非法 / `19002`~`19008` 沿用鉴权错误码 / `19022` Redis 不可用（fail-close）。

**边界**

| 场景 | 处理 |
|---|---|
| 网络超时后客户端重试 | 5 分钟内同 `install_id` 仍有 `PENDING` → 复用该 `bind_id`、**刷新** challenge 与 TTL（不新建行） |
| 并发 Apply | `(client_idn, install_id)` 的 `PENDING` 部分唯一索引兜底，冲突方拿到现有 `bind_id` |
| 调用方/凭证已停用 | 中间件直接拒（`19003`），不进入流程 |

---

### 5.2 `Bind/Prove`（设备证明，**不产生码**）

```
POST /OpenAPI/V1/Meta/Region/AZ/Server/Bind/Prove
```

**请求体**：`{ "bind_id": "…", "ts": 1794427291, "signature": "Base64(ECDSA(priv, M))" }`

**待签串 M（固定格式，逐行 `\n` 分隔）**

```
FINV-DEVICE-BIND
<bind_id>
<challenge>
<ts>
```

**服务端处理**

1. 取 `open_device_bind`：必须存在、`state=PENDING`、未过 `dt_expire`，否则 `19022`；
2. 取 Redis challenge：缺失 → 置 `EXPIRED` 并返回 `19022`；
3. 校验 `ts` 在 `sys_config` 19002 时间窗内（默认 ±300s），否则 `19006`；
4. 用申请里的 `device_pubkey` 验签（ECDSA P-256 + SHA-256）；失败 → `count_prove_fail += 1` 并返回 `19021`，累计 5 次 → `state=REJECTED`（防暴力验签）；
5. 成功：`state=PROVEN`、`dt_prove=now`、`count_prove_fail=0`，**响应里没有任何码**。

**响应**

```json
{ "code": 0, "data": { "state": "PROVEN", "awaiting_approval": true, "message": "设备已证明，等待管理员审核" } }
```

**边界**

| 场景 | 处理 |
|---|---|
| 客户端时钟偏差 | `ts` 超窗 → `19006`；客户端应先调 `/Hello` 用 `server_time.unix` 校时 |
| Prove 重试 | 已 `PROVEN` 时重复 Prove 直接返回 `state=PROVEN`（幂等），不重复计数 |
| challenge 被截获 | 无签名能力无法伪造 Prove（challenge 一次性且绑定 `bind_id`） |

---

### 5.3 人工审核（管理端，不对外）

页面：**开放平台 → 接入管理 → 设备绑定**

**待审列表显示**（`state=PROVEN`）：调用方 / 凭证号 / 设备型号 / OS+版本 / App 版本 / `serial`·`imei`·`mac`·`android_id`（缺失项标黄并显示 `id_missing`）/ 公钥指纹（`sha256(pubkey)` 前 16 位）/ 申请 IP / UA / `dt_prove`。

**动作「批准」**（内部接口 `POST /API/V1/Meta/Finv/OpenAPI/Device/Approve`，需登录）

1. 校验 `state=PROVEN` 且未过 24h；
2. 生成 6 位数字码：`crypto/rand`，范围 `000000`~`999999`（**保留前导零，全程按字符串处理**）；
3. 存 `code_hash = sha256(bind_id + ":" + code)`（用 `bind_id` 作盐，避免 10^6 空间被彩虹表枚举）；
4. `state=APPROVED`、`dt_approve=now`、`approve_by=<登录账号>`、`dt_code_expire=now+600s`、`count_try=0`；
5. **该次响应把明文码返回给管理端一次**，页面提示：*请口述给现场人员；10 分钟内有效；输错 5 次作废*。

**动作「驳回」**：`state=REJECTED` + `reason_reject`（必填，≤200 字）。

**动作「重发码」**：仅 `APPROVED` 且未 `ACTIVE` 可用 → 重新生成并覆盖 `code_hash`（旧码立即失效）。

> 页面刷新后**看不到明文码**（库里只有哈希）；要再拿一次只能点「重发码」。

---

### 5.4 `Bind/Confirm`（人在设备上输入码）

```
POST /OpenAPI/V1/Meta/Region/AZ/Server/Bind/Confirm
```

**请求体**：`{ "bind_id": "…", "code": "042917" }`

**服务端校验链**

1. 取申请：必须 `state=APPROVED`、未过 `dt_code_expire`，否则 `19022`；
2. `count_try >= 5` → `state=EXPIRED`，返回 `19022`；
3. `sha256(bind_id + ":" + code)` 与 `code_hash` 不等 → `count_try += 1`，返回 `19020`；第 5 次失败即 `state=EXPIRED`（需重新 Apply）；
4. 相等 → **单事务**：
   a. `rebind_of_device_id` 非空 → 旧设备 `flag_revoke='1'`、`dt_revoke=now`、`reason_revoke='REBIND'`；
   b. 写 `open_device`（`device_id` 生成、公钥、`install_id`、硬件标识、`id_missing`、自报信息、`dt_bind`、`flag_revoke='0'`）；
   c. 申请 `state=ACTIVE`、`dt_active=now`、`code_hash=''`（码用后即焚）；
5. 返回 `device_id` + 设备摘要。

**响应**

```json
{ "code": 0, "data": { "device_id": "8F2A…(32 位)", "state": "ACTIVE", "dt_bind": "2026-11-12T03:30:00+08:00" } }
```

**边界**

| 场景 | 处理 |
|---|---|
| Confirm 成功但响应丢了，客户端重试 | `state=ACTIVE` 时返回同一 `device_id`（幂等；`code_hash` 已清空不参与比对） |
| 码输错但仍有次数 | 返回 `19020`，提示剩余次数 |
| 审批人批错了 | 直接 `Bind/Unbind` 解绑后重新走流程 |
| 现场拿不到码 | 审批人点「重发码」 |

---

### 5.5 自动解绑

| 路径 | 触发方式 | 请求 / 条件 | 结果 |
|---|---|---|---|
| **主动上报** | 客户端发现本地私钥不存在（清数据/换机/Keystore 失效） | `POST .../Bind/Unbind`（AKSK 认证）`{ "device_id": "…", "reason": "CLIENT_KEY_LOST" }` | 设备行 `flag_revoke='1'` + `dt_revoke` + `reason_revoke`；关联申请 `state=UNBOUND` |
| **被动检测** | 设备仍持公钥但签名验不过（私钥损坏/被改机） | `Prove` 或设备签名校验失败：`count_fail += 1`，且本次与 `dt_last_fail` 间隔 **≥10 分钟**才计数（去抖）；`count_fail >= 3` | 自动吊销，`reason_revoke='KEY_INVALID'`；管理页标红 |

- **单次失败不解绑**（网络抖动、时钟偏差都会让签名失败一次）；
- 任一校验成功后 `count_fail = 0`；
- 解绑**不删行**，与凭证吊销同口径：只能新建绑定，不能复活旧行。

---

### 5.6 与业务的关系（本期只到"标记"，不做强制）

- `ACTIVE` 设备可带 `X-Finv-Device: <device_id>` 调业务接口，服务端把它记进审计；
- **本期不校验**设备签名，也不因"未绑定"拒绝任何业务请求；
- 后续若某项高敏感操作需要更高安全性，再单独批准"绑定 + 设备签名校验"作为该操作的准入条件。

## 6. 数据模型（草案，未建表）

两张新表，命名对齐 `open_api_*`，**无物理外键**（`DbSchemaSpec` 红线）。

### 6.1 `open.open_device`

| 列 | 说明 |
|---|---|
| `idn` / `device_id`(UNIQUE) | `device_id` 由服务端生成，给客户端与业务侧引用 |
| `client_idn` / `install_id` | `UNIQUE (client_idn, install_id)`：同设备重装复用同一行 |
| `pubkey`(UNIQUE) / `key_alg` | 公钥不可重复绑定 |
| `serial` / `imei` / `mac` / `android_id` | 硬件标识（可空）。`serial` / `imei` 各建**部分唯一索引**（`WHERE serial <> ''`），防同一台机器绑两次 |
| `id_missing` | VARCHAR(64)：本次缺失的标识项（如 `IMEI,MAC`），审批页标黄 |
| `device_model` / `os` / `os_version` / `arch` / `app_version` | **自报**，仅展示 |
| `dt_bind` / `dt_last_seen` | 绑定生效时间 / 最近一次设备请求时间 |
| `count_fail` / `dt_last_fail` | 被动自动解绑用：连续失败计数 + 上次失败时间（去抖 ≥10 分钟） |
| `flag_enable` / `flag_revoke` / `dt_revoke` / `reason_revoke` | 吊销留痕，原因必填（CHECK） |
| `remark` / `create_by` / `modify_by` / `gmt_create` / `gmt_update` | 审计列 |

### 6.2 `open.open_device_bind`

| 列 | 说明 |
|---|---|
| `bind_id`(UNIQUE) / `client_idn` / `credential_idn` | 申请标识与申请人 |
| `install_id` / `pubkey` / `key_alg` / 设备自报信息 | 申请时快照 |
| `serial` / `imei` / `mac` / `android_id` / `id_missing` | 硬件标识快照（审批人据此核对） |
| `state` | `PENDING` / `PROVEN` / `APPROVED` / `ACTIVE` / `REJECTED` / `EXPIRED` / `UNBOUND` |
| `code_hash` / `dt_code_expire` / `count_try` | 码哈希 `sha256(bind_id + ":" + code)`（不存明文）、码过期时间、已尝试次数（上限 5） |
| `approve_by` / `dt_approve` / `reason_reject` | 人工审核留痕（批准人 / 时间 / 驳回原因） |
| `rebind_of_device_id` | 同机换钥或重装时指向旧设备行（Confirm 成功后自动吊销旧行） |
| `count_prove_fail` | Prove 验签失败次数（上限 5，防暴力验签） |
| `dt_prove` / `dt_active` | 设备证明成功时间 / 绑定生效时间 |
| `dt_expire` / `dt_create` / `ip` / `user_agent` | |

## 7. 需要你确认的（只有 3 条，其余一律暂不决策）

| # | 决策点 | 选项 | 我的建议 |
|---|---|---|---|
| **K1** | 码的存储形态 | A 只存哈希（审批时生成、只展示一次）/ B 存明文或可逆密文（后台随时可查） | **A** 更安全；若你更在意"随时能查当前码"就选 B |
| **K2** | 绑定接口的认证模式 | A 只允许 AKSK / B 也允许静态 Key | **A**（要验签设备请求，静态 Key 签不了名） |
| **K3** | 绑定粒度 | A 调用方级（`client_idn`）/ B 凭证级 | **A**（凭证轮换不影响设备记录） |
| **K4** | 硬件标识缺失时是否允许绑定 | A 允许（`id_missing` 标记，审批人判断）/ B `serial` 必填，缺失直接拒绝 | **A**（普通 Android 机取不到 IMEI/MAC；若你的客户端是设备所有者/系统应用、能稳定取到，就选 **B**） |

以下**本期不决策**，落地时按需再定，草案里已给出默认取值，不阻塞开工：

按需再定：设备签名的请求头形态、错误码号段（草案用 `19020`~`19023`）、
高安全操作清单与引用方式、Play Integrity、换机保留策略、审批角色细分、未绑定设备的灰度策略。

## 8. 风险与红线

| 风险 | 对策 |
|---|---|
| 新增表属结构调整 | 走迁移 + 需求登记；`open` 域既有 5 张表**不得改名/合并**（`DbSchemaSpec` §0） |
| 影响既有认证语义 | 本期**不改**任何既有鉴权路径；设备绑定是独立的旁路状态 |
| 误伤解绑 | 单次失败不解绑：需「连续 3 次 + 间隔 ≥10 分钟」或客户端显式上报 |
| 码被爆破 | TTL 10 分钟 + 5 次尝试 + 一次性 + 哈希存储 + `crypto/rand` |
| 硬件标识缺失或伪造 | 可空 + `id_missing` 标记，审批页标黄提示；唯一性按 `serial > imei > mac > android_id > install_id` 顺序判定；root 可改，故只作识别不作认证因子 |
| Redis 不可用 | `challenge` 取不到则拒绝绑定（fail-close，与现有 nonce 策略一致） |

## 9. 阶段与验收

| 阶段 | 内容 | 验收核对 |
|---|---|---|
| **P1** | 建表 + `Bind/Apply` + `Bind/Prove` | AKSK 签名调 Apply 返回 `bind_id`+`challenge`；错误私钥 Prove 返回 `19021`；正确私钥返回 `state=PROVEN`；同 `install_id` 重复 Apply 幂等 |
| **P2** | 管理端「设备绑定」页（列表 / 批准出码 / 驳回）+ `Bind/Confirm` + 自动解绑 | 批准后页面显示一次 6 位码；Confirm 正确 → `open_device` 新增行；输错 5 次 → `EXPIRED`；`Bind/Unbind` 后 `flag_revoke='1'` 且原因非空 |
| **P3（以后）** | 高安全操作按需引用设备可信标记 | 届时再定，不在本期范围 |

P1/P2 之外的一切（Play Integrity、全局强制、设备管理）**明确不做**。

## 10. 相关文档

| 文档 | 关系 |
|---|---|
| [`Docs/OpenAPI/Region/AZ/Server/Hello.md`](../OpenAPI/Region/AZ/Server/Hello.md) | 握手是 Phase 0 |
| [`Docs/API/OpenAPI/Auth.md`](../API/OpenAPI/Auth.md) | 复用其签名 / 时间窗 / nonce 口径 |
| [`Docs/DataDictMapping/OpenApiAuth.md`](../DataDictMapping/OpenApiAuth.md) | 批准后新增 `open_device*` 数据字典 |
| [`Docs/DevSpec/DbSchemaSpec.md`](../DevSpec/DbSchemaSpec.md) | 新增表前自检 §0 红线 |
