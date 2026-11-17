# FinvQuant 服务端 API 文档

> 基路径：`/API/V1`（统一大写，见 [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md)）
> 服务端端口：**16001**；默认地址：`http://localhost:16001`

## 通用约定

### 响应信封

所有 REST JSON 响应顶层固定输出：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `code` | int | ✅ | 业务码；成功码集合 `{0, 1, 200, 202}` |
| `message` | string | ✅ | 文本消息 |
| `data` | object | 可选 | 业务数据 |
| `error` | object | 可选 | 错误时必填；含 `code`、`catalog_version`、`retryable` |
| `details` | object | 可选 | 补充详情 |
| `request_id` | string | 可选 | 请求追踪 ID |
| `trace_id` | string | 可选 | 链路追踪 ID |

所有 wire 字段使用 **snake_case**。

### 错误码

| 号段 | 含义 |
|------|------|
| `{0, 1, 200, 202}` | 成功 |
| `1000-2999` | 平台、安全和依赖错误 |
| `≥3000` | 项目自定义业务错误码 |

（详见 [Docs/DevSpec/ErrorCodeSpec.md](../DevSpec/ErrorCodeSpec.md)）

## 接口索引

- **接口总览（方法 / 路径 / 说明 / 详细文档 / 已使用位置）**：见 [APIs.md](APIs.md)。
- **详细接口文档**：
  - [`HistoryQuote/ImportsUpload.md`](HistoryQuote/ImportsUpload.md) — `POST /API/V1/Quote/Import/Upload`（MVSV 历史行情导入）
  - [`HistoryQuote/HistoryQuote.md`](HistoryQuote/HistoryQuote.md) — `GET /API/V1/Quote/Query`（历史行情查询）
  - [`Meta/MetaExchange.md`](Meta/MetaExchange.md) — `GET/POST /API/V1/Meta/FinvQuant/Metadata/Exchange/*`（交易所信息维护）
  - [`Meta/MetaMarket.md`](Meta/MetaMarket.md) — `GET/POST /API/V1/Meta/FinvQuant/Metadata/Market/*`（交易所下设市场信息维护）
  - [`Meta/MetaSecurity.md`](Meta/MetaSecurity.md) — `GET/POST /API/V1/Meta/FinvQuant/Metadata/Security/*`（规范证券信息维护，含 `Options` 下拉与 `Lookup` 详情查询）
