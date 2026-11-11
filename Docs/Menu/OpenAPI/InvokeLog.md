# 菜单：OpenAPI 调用审计

> 所属：FinvQuant 菜单文档 · 存放：`Docs/Menu/OpenAPI/InvokeLog.md`
> 菜单路径：金融投资 → **开放平台** → **运行监控** → OpenAPI 调用审计
> 路由：`/Meta/Finv/OpenAPI/InvokeLog`（key `openapi-invoke-log`）
> 视图：`Web/src/views/Meta/Finv/OpenAPI/InvokeLogView.vue`
> 接口：[`../../API/OpenAPI/Admin.md`](../../API/OpenAPI/Admin.md) §6
> 需求：11173（按功能拆分）；11169（原页面）；11168（中间件写审计）

## 1. 页面职责

只读查询 `open.open_api_invoke_log`（只增不改）。

## 2. 查询条件

| 条件 | 说明 |
|---|---|
| 结果 | 全部 / 放行 / 拒绝 |
| 拒绝原因 | 11 种 `reason_code`（与错误码 `190xx` 一一对应），表格显示中文 |
| 公开标识 | 模糊匹配（AK/SK 记 AK；静态 Key 只记脱敏形态） |

按 `dt_request` 倒序分页。

## 3. 列表列

请求时间、方法、路径、结果（放行绿 / 拒绝红）、原因、HTTP 状态码、耗时（ms）、公开标识、密钥指纹 `kfp`、来源 IP、请求 ID、操作（详情）。

- 路径 / 公开标识 / 请求 ID 这类长值：单元格截断 + **ⓘ 悬浮看全文**（[`TableTextSpec`](../../DevSpec/TableTextSpec.md)）。
- 行末 **「详情」按钮**打开抽屉，按 **字段 / 取值 / 说明** 三列展示该行全部字段（[`FE18`](../../RequirementList.md)），
  取值不截断、长串自动换行、逐字段可一键复制；**点表格行不会跳转**，只看详情必须点按钮。
- 详情里额外暴露 `tfp`（STS 临时凭据的 SecurityToken 指纹），可按一次临时会话聚合排查。
- 只存 `body_sha256` 与 `count_body`（不存请求原文），排障时用它比对两端是否同一份载荷。

## 4. 空态说明

鉴权模式为 **关闭**（19001=`0`）时中间件不校验也不写审计，页面空态文案已注明；
要产生记录，请先把鉴权模式切到 `1` 或 `2`。

## 5. 说明卡片要点（页面底部）

写入时机、拒绝原因与 `190xx` 的对应关系、公开标识的脱敏口径、只存摘要不存原文、保留天数（19003）。

## 6. 相关文档

- 调用方：[`Client.md`](Client.md) · 凭证：[`Credential.md`](Credential.md) · 鉴权配置：[`Settings.md`](Settings.md)
- 错误码目录：[`../../DevSpec/ErrorCodeSpec.md`](../../DevSpec/ErrorCodeSpec.md) §1.2
