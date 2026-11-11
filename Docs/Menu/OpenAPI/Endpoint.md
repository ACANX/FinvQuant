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

## 2.1 现有接口编码（需求 11233 定稿）

| `api_code` | 方法 | 说明 |
|---|---|---|
| `finv.quote.cdeu.min.pageuni` | GET | 分钟行情分页查询 |
| `finv.quote.cdeu.min.upsert` | POST | 分钟行情批量写入 |
| `finv.quote.cdeu.realtime.ftstockupsert` | POST | 实时行情快照写入（`FT` = 富途数据源，与路径末段一致，需求 11234） |
| `open.auth.credential.sts.issue` | POST | STS 临时凭据换证（开放平台域；路径同样带 `Meta/Open`，需求 11235–11237） |

分段含义：`finv.quote` = 行情业务域（与库表 `finv_quote_*`、菜单「行情」对齐）、
`open.auth` = 开放平台自身的鉴权接口（与 `open` schema、路径里的 `Meta/Open` 对齐），
其后按「模块.资源.动作」——如 `open.auth.credential.sts.issue`（`credential` 段与表 `open_api_credential` 对应）。

## 2.2 catalog / func 的派生规则（需求 11238 / 11239）

两列都是**生成列**（`GENERATED ALWAYS AS … STORED`），来源不同、都**不可显式写入**：

| 列 | 来源 | 规则 | 示例 |
|---|---|---|---|
| `catalog` | `api_code` | 最后一个 `.` **之前**的部分 | `finv.quote.cdeu.min` |
| `func` | `path` | 最后一个 `/` **之后**的部分，**大驼峰** | `PageUni`、`FTStockUpsert`、`Issue` |
| `alias` | `catalog` + `func` | `{catalog}.{func}` | `finv.quote.cdeu.min.PageUni` |

```
api_code                                 path（末段）         catalog                    func
finv.quote.cdeu.min.pageuni          →   …/Min/PageUni    →   finv.quote.cdeu.min        PageUni
finv.quote.cdeu.min.upsert           →   …/Min/Upsert     →   finv.quote.cdeu.min        Upsert
finv.quote.cdeu.realtime.ftstockupsert → …/Realtime/FTStockUpsert → finv.quote.cdeu.realtime FTStockUpsert
open.auth.credential.sts.issue       →   …/STS/Issue      →   open.auth.credential.sts   Issue
```

`catalog` 的恒等式 `catalog || '.' || <编码末段> = api_code` 仍成立（`func` 已改来源，不再参与该恒等式）。

> `alias` 的生成表达式不能直接写 `catalog || '.' || func`——PostgreSQL **不允许生成列引用其它生成列**
> （`cannot use generated column … in column generation expression`），所以迁移里把两个派生表达式内联展开；
> 迁移末尾仍断言 `alias = catalog || '.' || func`（`SELECT` 里可以引用生成列）。

**`func` 为什么取路径末段而不是转换编码**：项目 API 路径规范本身要求末段是 PascalCase
（`.../Min/PageUni`、`.../STS/Issue`），直接取末段就是大驼峰；SQL 无法"智能"地把 `ftstockupsert`
还原成 `FTStockUpsert`（会把内部大写压平）。因此取末段 + `CHECK (func ~ '^[A-Z][A-Za-z0-9]*$')`：
末段不合规时**写入直接失败**，让命名问题在源头暴露。

为什么用生成列而不是普通列：派生值做成普通列就得靠每条 INSERT/UPDATE 手工同步，
接口清单本来就要靠人维护，再叠一层「忘了同步」的漂移没必要；生成列让不一致在结构上不可能出现。
（与 `usc_lower`、`ftmm_symbol` 同规则：**生成列禁止显式写入**，写了 PostgreSQL 直接报错。）

用途：页面按「目录」分组/筛选；`Endpoint/Options` 也按 `catalog, func` 排序，授权范围下拉自然按目录聚集。

## 3. 列说明

| 列 | 字段 | 口径 |
|---|---|---|
| IDN | `idn` | 自增主键 |
| 接口编码 | `api_code` | **点分小写**，按「业务域.模块.动作」分段（行情类如 `finv.quote.cdeu.min.pageuni`、开放平台鉴权类如 `open.auth.credential.sts.issue`）；路径改名后编码不变，报表/套餐/限额靠它锚定 |
| 目录 | `catalog` | 接口编码**最后一个点之前**的部分（如 `finv.quote.cdeu.min`）；生成列，由 `api_code` 派生 |
| 功能 | `func` | 路径**最后一个 `/` 之后**的部分（大驼峰，如 `PageUni`、`FTStockUpsert`）；生成列，由 `path` 派生，受 `^[A-Z][A-Za-z0-9]*$` 约束 |
| 别名 | `alias` | `{catalog}.{func}`（如 `finv.quote.cdeu.min.PageUni`）；生成列，由前两列派生 |
| 方法 | `http_method` | GET / POST / …（只登记真实方法，不含 `*`） |
| 路径 | `path` | 含 `/OpenAPI/V1` 前缀，与授权范围、审计表同口径 |
| 中文名 | `api_name` | 下拉与报表展示用 |
| 分组 | `api_group` | 分钟行情 / 实时行情 / 鉴权 |
| 读写 | `flag_write` | 写接口用静态 Key 调用必须带查询参数 `_`（毫秒时间戳，±60 秒） |
| 已授权凭证 | 聚合 | `open_api_credential_scope` 里命中该 `method + path` 的规则数（需求 11228 起该表已是鉴权读取来源，数值准确） |
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

## 6. 布局与列宽（需求 11229）

表格用**固定布局 + colgroup 像素宽度**（不是浏览器自动布局）：路径可达 60+ 字符、中文名 6~8 字，
自动布局会把这两列挤在一起互相压字。因此：

| 操作 | 效果 |
|---|---|
| 拖动表头右边缘 | 调整该列宽度（最小宽度见下），松手即存 |
| 双击表头右边缘 | 按该列**当前页内容**自适应宽度（取最宽一条 + 内边距） |
| 「重置列宽」 | 全部回默认宽度 |

宽度记录在 `localStorage` 的 `finv.openapi.endpoint.widths`，刷新/重开浏览器保持；
缓存损坏或隐私模式禁用 localStorage 时自动回默认，不影响页面。

默认宽度与最小宽度：

| 列 | 默认 | 最小 | 说明 |
|---|---|---|---|
| IDN | 70 | 50 | |
| 接口编码 | 230 | 140 | 最长 38 字符（`finv.quote.cdeu.realtime.ftstockupsert`） |
| 目录 | 180 | 120 | 编码去掉最后一段，最长 24 字符 |
| 功能 | 140 | 100 | 路径末段，最长 `FTStockUpsert`（13 字符） |
| 别名 | 260 | 160 | `{catalog}.{func}`，最长约 42 字符 |
| 方法 | 80 | 60 | |
| 路径 | 360 | 160 | 等宽字体；放不下出省略号，点 ⓘ 看全文 |
| 中文名 | 160 | 110 | 放不下出省略号，悬停 `title` 看全名 |
| 分组 | 110 | 80 | |
| 读写 | 70 | 56 | |
| 已授权凭证 | 110 | 80 | |
| 公开 / 启用 | 92 | 72 | 开关居中；宽度小于 72 会被截断，故最小 72 |
| 更新时间 | 180 | 140 | |

## 7. 相关文档

- 凭证（授权范围下拉）：[`Credential.md`](Credential.md) · 审计（`api_code` 列）：[`InvokeLog.md`](InvokeLog.md)
- 鉴权配置：[`Settings.md`](Settings.md) · 开放平台总览：[`../../API/OpenAPI/Admin.md`](../../API/OpenAPI/Admin.md)
