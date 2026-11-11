# 菜单：OpenAPI 接口清单

> 所属：FinvQuant 菜单文档 · 存放：`Docs/Menu/OpenAPI/Endpoint.md`
> 菜单路径：金融投资 → **开放平台** → **安全设置** → OpenAPI 接口清单
> 路由：`/Meta/Finv/OpenAPI/Endpoint`（key `openapi-endpoint`）
> 视图：`Web/src/views/Meta/Finv/OpenAPI/EndpointView.vue`
> 接口：[`../../API/OpenAPI/Admin.md`](../../API/OpenAPI/Admin.md) §8
> 表：`open.open_api_endpoint`（`V300193`，需求 11220）
> 需求：11223（页面本身）

## 1. 页面职责

只读展示「对外 OpenAPI 提供了哪些接口」，并提供两个开关（`flag_public` / `flag_enable`）。
它服务于另外两个页面：

- 「OpenAPI 凭证」的**授权范围下拉**只列 `flag_public='1'` 且 `flag_enable='1'` 且未下线的接口；
- 「OpenAPI 调用审计」按 `path + method` 关联本表，把路径翻译成稳定的 `api_code`。

## 2. 不做什么（关键边界）

**本页不决定「能调什么」**。允许调哪些接口由后端路由注册（`internal/api/router.go` 的 `openGroup`）决定，
本表是目录 + 元数据。页面因此**不提供**「新增接口 / 改路径 / 改方法」：

- 若允许在页面新增接口，会出现「表里有、路由没有」的幽灵接口，调用方拿到 404 却查不出原因；
- 反过来「路由有、表里没有」只是报表里 `api_code` 为空，属可接受的降级。

新增接口的正确顺序：先注册路由 → 再补一条种子迁移（`ON CONFLICT DO NOTHING`）→ 页面自然出现。

## 3. 列说明

| 列 | 字段 | 口径 |
|---|---|---|
| IDN | `idn` | 自增主键 |
| 接口编码 | `api_code` | **点分小写**（如 `cdeu.min.pageuni`）；路径改名后编码不变，报表/套餐/限额靠它锚定 |
| 方法 | `http_method` | GET / POST / …（只登记真实方法，不含 `*`） |
| 路径 | `path` | 含 `/OpenAPI/V1` 前缀，与授权范围、审计表同口径 |
| 中文名 | `api_name` | 下拉与报表展示用 |
| 分组 | `api_group` | 分钟行情 / 实时行情 / 鉴权 |
| 读写 | `flag_write` | 写接口用静态 Key 调用必须带查询参数 `_`（毫秒时间戳，±60 秒） |
| 已授权凭证 | 聚合 | `open_api_credential_scope` 里命中该 `method + path` 的规则数；该表尚未接入读写，仅供参照 |
| 公开 | `flag_public` | 开关：关掉后不再出现在授权范围下拉 |
| 启用 | `flag_enable` | 开关：仅「不再推荐授权」，**已授权的凭证仍可调用** |
| 更新时间 | `gmt_update` | 触发器维护 |

## 4. 两个开关的语义差异（最容易误解）

| 开关 | 关掉后 | 想真正收回权限要做什么 |
|---|---|---|
| 公开 `flag_public` | 下拉里看不到（不能再新授权），已有授权照旧生效 | 改凭证的授权范围 |
| 启用 `flag_enable` | 下拉里看不到，已有授权照旧生效 | 改凭证的授权范围，或吊销凭证 |

也就是说**两个开关都不回收既有权限**——这是刻意的：若关一个开关就能让线上调用方全部 403，
误操作代价太大。要停用某接口对外，正确做法是逐凭证收窄授权范围或吊销凭证。

## 5. 交互约定

- 筛选：关键字（编码/路径/中文名）、分组、公开；改动任一筛选立即重查。
- 开关直接生效（写 `open.open_api_endpoint`），失败时 Toast 报错并回读最新值。
- 已下线（`dt_deprecate` 非空）的行「公开」开关置灰，避免把下线接口重新变成可授权。

## 6. 相关文档

- 凭证（授权范围下拉）：[`Credential.md`](Credential.md) · 审计（`api_code` 列）：[`InvokeLog.md`](InvokeLog.md)
- 鉴权配置：[`Settings.md`](Settings.md) · 开放平台总览：[`../../API/OpenAPI/Admin.md`](../../API/OpenAPI/Admin.md)
