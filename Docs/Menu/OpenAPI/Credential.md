# 菜单：OpenAPI 凭证

> 所属：FinvQuant 菜单文档 · 存放：`Docs/Menu/OpenAPI/Credential.md`
> 菜单路径：金融投资 → **开放平台** → **接入管理** → OpenAPI 凭证
> 路由：`/Meta/Finv/OpenAPI/Credential`（key `openapi-credential`）
> 视图：`Web/src/views/Meta/Finv/OpenAPI/CredentialView.vue`（对话框 `CredentialDialog.vue`）
> 接口：[`../../API/OpenAPI/Admin.md`](../../API/OpenAPI/Admin.md) §5
> 需求：11173（按功能拆分）；11169（原页面）

## 1. 页面职责

签发与维护 `open.open_api_credential`（一行 = 一种认证模式的密钥）。
调用方列表通过 `GET /Client/Page` 只读加载，用于下拉选择与筛选，不在此页修改。

## 2. 列表

- 查询：调用方下拉（含「全部调用方」）+ 关键字（凭证号 / 公开标识）+ 分页。
- 列：凭证号、调用方、认证模式（静态 Key / AK-SK）、公开标识、授权范围、状态、最近使用、累计调用、操作。
- 公开标识只显示 `key_prefix…`，完整值放在 ⓘ 悬浮里；**密钥材料永不回显**（只有 `has_secret` 布尔）。
- 授权范围显示条数，明细在 ⓘ 悬浮里逐条列出；空 = 全部接口。

## 3. 签发（对话框，两步走）

1. 表单：调用方、凭证号、认证模式、授权范围（每行「方法 路径」）、IP 白名单、到期、备注；
2. 提交后同框切成「密钥展示」步骤：给出 AK（静态 Key 模式即静态 Key 本身）与 SK，
   各自带复制按钮，并提示「关闭后不再显示，丢失只能轮换」。

模式差异：

| 模式 | 返回 | 库内存储 |
|---|---|---|
| 静态 API Key | `fk_live_` + 32 位随机（`access_key` = `secret_key`） | `secret_hash`（sha256） |
| AK/SK | AK `ak_live_` + 32 位随机、SK `sk_live_` + 48 位随机 | `secret_cipher`（AES-256-GCM，密钥取 `sys_config` 19005） |

`19005` 未配置时签发 AK/SK 返回 503，页面会提示先去系统配置填写。

## 4. 状态操作

| 操作 | 语义 |
|---|---|
| 停用 / 启用 | 可逆，不换密钥；已吊销的凭证不能启用（409） |
| 轮换 | 生成新密钥并**立即**让旧密钥失效（AK/SK 的旧 SK 在签名时间窗内仍可验签） |
| 吊销 | **不可逆**，必须填原因，留 `dt_revoke` + `reason_revoke`；之后只能新建凭证 |

## 5. 说明卡片要点（页面底部）

两种模式的密钥形态与存储方式、状态语义（停用可逆 / 吊销不可逆）、轮换的旧密钥窗口、
授权范围的写法（方法与路径、`/*` 前缀匹配）。

## 6. 相关文档

- 调用方：[`Client.md`](Client.md) · 审计：[`InvokeLog.md`](InvokeLog.md) · 鉴权配置：[`Settings.md`](Settings.md)
- 数据字典（逐列）：[`../../DataDictMapping/OpenApiAuth.md`](../../DataDictMapping/OpenApiAuth.md)
