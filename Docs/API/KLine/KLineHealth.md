# GET /Health — 探活

探测数据库连通性（替代旧服务的 `/healthz`，DC-02）。

## 请求

- **方法**：`GET`
- **路径**：`/Health`
- **旧前缀（过渡别名）**：`/API/V1/Meta/Finv/Quote/Secu/KLine/Min` + 相同子路径（切流期保留，P7 后可移除）
- **Content-Type**：`application/json`

- **主路径**：`GET /Health`（无 `/API/V1` 前缀）
- **过渡别名**：`GET /healthz`（切流期保留，P7 后移除）
- **就绪探针（全服务）**：`GET /API/V1/Health/Ready`（同时探 PG 与 Redis）

### 成功（HTTP 200）

```json
{"status":"ok"}
```

### 失败（HTTP 503）

```json
{"status":"degraded","code":211302,"message":"数据服务暂时不可用，请稍后重试",
 "detail":"数据服务暂时不可用，请稍后重试",
 "request_id":"B7C63349-EB36-49AB-B16A-E8E1BE475D3B",
 "error":{"code":211302,"catalog_version":"finvquant-1","retryable":true}}
```

## 已使用位置登记

- 监控/容器 healthcheck、采集端启动自检
