# API 规范（ApiSpec）

> 所属：FinvQuant 开发规范 · 存放：`Docs/DevSpec/`
> 适用范围：VeritasQuant API 服务的 REST 响应契约与异常映射。
> 错误码定义见 [`ErrorCodeSpec.md`](ErrorCodeSpec.md)。

## 1. 响应信封

- 所有 REST JSON 响应顶层固定输出数值 `code` 和文本 `message`；`data`、`error`、`details`、`request_id`、`trace_id` 均为按语义可选字段。
- 所有 wire 字段（JSON 请求/响应体）使用 **snake_case**。

## 2. API 路径

- 服务端 API 路径统一使用大写 `/API/V1/` 前缀（例如 `GET /API/V1/Health/Live`），不使用 `/api/v1/`。
- **端点路径段使用 PascalCase**（例如 `POST /API/V1/Meta/Finv/Quant/Quote/Import/Upload`），不使用小写 snake 风格（例如 `/imports/upload`）；资源/模块名首字母大写，层级用 `/` 分隔。
- 前端开发代理与生产反代的路径前缀必须与之一致（`/API`）。

## 3. URL 查询参数命名

- **URL 查询参数统一使用小驼峰（lowerCamelCase）命名**：多词参数首词全小写、后续词首字母大写，例如 `pageSize`、`runId`、`secuCode`、`strategyId`、`allowBacktest`、`envType`、`templateType`、`userId`；
- **禁止使用下划线分隔（snake_case）的查询参数名**，如 `page_size`、`run_id`、`secu_code`、`allow_backtest` 等均不允许出现在 URL 查询串中；
- 单字参数保持全小写（如 `page`、`status`、`keyword`）；
- 适用范围：所有 HTTP 请求的 URL 查询参数（GET 查询串与 POST 的 URL 查询部分），无论接口属于哪个业务模块；
- **与 JSON 体字段严格区分**：JSON 请求/响应体（wire 字段）使用 snake_case，URL 查询参数使用 camelCase，两者互不套用、不得混用；
- 新增接口必须按本规范设计查询参数；修改既有接口时若发现历史查询参数不符合本规范，须同步迁移为小驼峰，并同步更新对应接口文档与前端调用方。

## 4. 响应状态形态

- 成功码集合固定为 `{0, 1, 200, 202}`；成功及非错误业务状态响应**不得携带 `error`**；所有错误**必须携带 `error`**。

## 5. 抛出与映射

- 领域代码只通过统一 `BusinessException` 抛出**已注册业务码**；应用边界统一映射顶层 `code`、HTTP、嵌套 `error`、重试属性、本地化消息和公开详情。
- 未注册业务码必须使**启动校验或 CI 失败**；敏感详情和堆栈**不得**进入 API 响应。

## 6. 契约测试

API 契约测试必须覆盖：

- 固定成功码；
- 特定业务状态；
- 异常映射；
- 命令失败快照；
- 敏感详情过滤。

## 7. 接口文档

### 7.1 文档必备性

- **新开发的 API 接口，必须补充对应的 API 接口文档**，说明请求参数、响应结构、错误码与调用示例；接口行为或契约变更时须同步更新对应文档。

### 7.2 一个接口一个文档（强制）

- **每个 API 接口必须独立成一篇文档文件**，**不允许多个接口放到同一文件中描述**；一篇文档只对应一个接口（唯一「方法 + 路径」）；
- 文档文件名与端点资源一一对应，使用 **PascalCase**：`<资源><动作>.md`，例如 `RunList.md` 对应 `GET .../Run/List`、`RunCreate.md` 对应 `POST .../Run/Create`、`StrategyGet.md` 对应 `GET .../Strategy/Get`；
- 同名资源按动作拆分文档（如 `StrategyList` / `StrategyGet` / `StrategySave` / `StrategyToggle` / `StrategyDelete`），**禁止**以 `ResourceAll.md`、`ResourceManage.md` 等合并文件替代；
- 接口新增、下线或契约变更时，只增删改对应接口的独立文档，不得将其他接口内容并入或拆分合并。

### 7.3 文档内容要求

- 文档位置通常在 `Docs/API/xxx/XXXX.md`（按业务模块建目录，文档名与端点资源对应）；
- 文档须包含：请求方法、请求路径、Query 参数（**按第 3 节小驼峰规范**列出）、请求体（JSON，snake_case）、响应示例、错误码、已使用位置登记；
- 参考示例：[`Docs/API/HistoryQuote/HistoryQuote.md`](../API/HistoryQuote/HistoryQuote.md)（对应 `GET /API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery`）。

### 7.4 索引登记

- 新增接口文档后须在 [`Docs/API/README.md`](../API/README.md) 端点索引中登记链接，并同步更新 [`Docs/API/APIs.md`](../API/APIs.md) 接口总览清单。

### 7.5 已使用位置登记

- **API 接口被哪些地方（业务菜单）使用了，须在 API 接口文档中添加对应的已使用位置登记**：列出使用该接口的业务菜单及其文档引用（如 `Docs/Menu/xxx/XXXX.md`），与菜单文档中的接口引用形成双向索引；新增使用方或接口下线时须同步更新登记。
