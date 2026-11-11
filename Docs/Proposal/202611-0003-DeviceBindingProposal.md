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
| 设备管理平台（批量下发/远程擦除/合规报表） | 超出当前需要 |
| Play Integrity / Key Attestation / 防 root | 成本高、依赖 Google 链路；等真有攻击面再评估 |
| 采集 IMEI / MAC / 序列号 / 手机号 | 《个人信息保护法》下的敏感个人信息，最小必要原则不需要 |
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
| 型号 / OS / OS 版本 / 架构 / App 版本 | **低**（自报，可伪造） | 仅展示、风控信号、审计；**不作认证依据**；客户端取不到时传空串，服务端不得因此拒绝 |

## 5. 流程

```
Phase 0  握手（已有）   GET  /Meta/Region/AZ/Server/Hello
Phase 1  Bind/Apply    POST …/Bind/Apply   → bind_id + challenge（Redis 600s）
Phase 2  Bind/Prove    POST …/Bind/Prove   → 验签 → state=PROVEN（**此时不生成码**）
Phase 3  人工审核      管理端「设备绑定」页 → 查看申请 → 批准 → 生成 6 位码，页面显示一次
Phase 4  Bind/Confirm  POST …/Bind/Confirm → 人在设备上输入码 → 比对哈希 → 设备 ACTIVE
（任意阶段）自动解绑    POST …/Bind/Unbind 或连续验签失败 → 吊销设备行
```

### 5.1 `Bind/Apply`

- 认证：`AKSK` 签名（静态 Key 无法签名设备请求；STS 临时凭据 2 小时到期会让流程中断）；
- 请求：

```json
{
  "install_id": "6f1c3b8e-…",
  "device_pubkey": "MFkwEwYHKoZIzj0C…",
  "key_alg": "ECDSA-P256-SHA256",
  "device": { "model": "Pixel 8", "os": "Android", "os_version": "15", "arch": "arm64-v8a", "app_version": "1.0.0" }
}
```

- 服务端：校验凭证/调用方状态 → 同 `(client_idn, install_id)` 且公钥相同且已 ACTIVE → 直接返回「已绑定」（幂等）；
  否则插 `open_device_bind(state='PENDING')`，`bind_id`（32 位大写十六进制）+ `challenge`（Redis 600s），返回两者。

### 5.2 `Bind/Prove`

- 请求：`{ "bind_id": "…", "signature": "Base64(ECDSA(privkey, challenge||bind_id||ts))", "ts": 1794427291 }`
- 服务端验签 → `state='PROVEN'`，**不生成码、不返回码**，响应只给 `{"state":"PROVEN","awaiting_approval":true}`。

### 5.3 人工审核（管理端）

- 页面「开放平台 → 接入管理 → 设备绑定」列出 `PROVEN` 待审申请：调用方、凭证号、设备型号/OS、申请 IP、时间；
- 审批人点「批准」→ 服务端生成 6 位码（`crypto/rand`）+ 存哈希 + 返回明文**给管理端一次** →
  页面显示并提示「请口述给现场人员，10 分钟内有效，输错 5 次作废」；
- 点「驳回」→ `state='REJECTED'` + 原因。

### 5.4 `Bind/Confirm`

- 请求：`{ "bind_id": "…", "code": "042917" }`
- 服务端：未过期且 `count_try < 5` → 比对 `code_hash` →
  - 成功：`open_device` 插一行（`device_id` / `pubkey` / `install_id` / 自报信息 / `dt_bind`），`bind.state='ACTIVE'`；
  - 失败：`count_try+1`，返回 `19020`；超限 → `state='EXPIRED'`（需重新申请）。

## 6. 数据模型（草案，未建表）

两张新表，命名对齐 `open_api_*`，**无物理外键**（`DbSchemaSpec` 红线）。

### 6.1 `open.open_device`

| 列 | 说明 |
|---|---|
| `idn` / `device_id`(UNIQUE) | `device_id` 由服务端生成，给客户端与业务侧引用 |
| `client_idn` / `install_id` | `UNIQUE (client_idn, install_id)`：同设备重装复用同一行 |
| `pubkey`(UNIQUE) / `key_alg` | 公钥不可重复绑定 |
| `device_model` / `os` / `os_version` / `arch` / `app_version` | **自报、不可信**，仅展示 |
| `dt_bind` / `dt_last_seen` / `count_fail` | `count_fail` 用于"连续失败自动解绑" |
| `flag_enable` / `flag_revoke` / `dt_revoke` / `reason_revoke` | 吊销留痕，原因必填（CHECK） |
| `remark` / `create_by` / `modify_by` / `gmt_create` / `gmt_update` | 审计列 |

### 6.2 `open.open_device_bind`

| 列 | 说明 |
|---|---|
| `bind_id`(UNIQUE) / `client_idn` / `credential_idn` | 申请标识与申请人 |
| `install_id` / `pubkey` / `key_alg` / 设备自报信息 | 申请时快照 |
| `state` | `PENDING` / `PROVEN` / `APPROVED` / `ACTIVE` / `REJECTED` / `EXPIRED` / `UNBOUND` |
| `code_hash` / `dt_code` / `count_try` | 码哈希（不存明文）、生成时间、尝试次数 |
| `approve_by` / `dt_approve` / `reason_reject` | 人工审核留痕 |
| `dt_expire` / `dt_create` / `ip` / `user_agent` | |

## 7. 需要你确认的（只有 3 条，其余一律暂不决策）

| # | 决策点 | 选项 | 我的建议 |
|---|---|---|---|
| **K1** | 码的存储形态 | A 只存哈希（审批时生成、只展示一次）/ B 存明文或可逆密文（后台随时可查） | **A** 更安全；若你更在意"随时能查当前码"就选 B |
| **K2** | 绑定接口的认证模式 | A 只允许 AKSK / B 也允许静态 Key | **A**（要验签设备请求，静态 Key 签不了名） |
| **K3** | 绑定粒度 | A 调用方级（`client_idn`）/ B 凭证级 | **A**（凭证轮换不影响设备记录） |

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
| 设备信息合规 | 只采集最小集、自报标注、不采硬件唯一号 |
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
