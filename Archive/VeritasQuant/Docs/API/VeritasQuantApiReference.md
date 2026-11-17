# VeritasQuant 服务端 API 接口文档

> 本文档描述服务端（`vq-api-server`）对外提供的 HTTP API。
> 服务端默认监听 `0.0.0.0:18000`（容器内固定 18000，宿主映射见 `.env.deploy` 的 `VQ_API_PORT`）。

## 1. 通用约定

### 1.1 Base URL

```
http://<host>:18000
```

### 1.2 响应信封（ResponseEnvelopeV1）

除 `/metrics`（Prometheus 文本格式）与 SSE 流外，所有响应均为统一信封：

```json
{
  "code": 0,
  "message": "成功",
  "data": { },
  "error": null,
  "request_id": "req_xxx"
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `code` | int | 业务状态码：`0/1/200/202` 为成功或非错误业务状态；`>=1000` 为错误 |
| `message` | string | 人类可读消息（中文） |
| `data` | object \| null | 业务数据（成功时存在） |
| `error` | object \| null | 错误信息（仅错误响应携带） |
| `request_id` | string \| null | 请求追踪标识 |

`error` 结构：

```json
{
  "error": {
    "code": "RESOURCE_NOT_FOUND",
    "catalog_version": "1.0",
    "retryable": false
  }
}
```

### 1.3 错误码（ApiErrorCodes.yml）

| Code | ErrorCode | HTTP | 说明 |
| ---: | --- | ---: | --- |
| 1001 | `VALIDATION_ERROR` | 400 | 输入校验失败 |
| 1002 | `RESOURCE_NOT_FOUND` | 404 | 资源不存在（含未命中路由） |
| 1003 | `IDEMPOTENCY_CONFLICT` | 409 | 幂等键冲突（同键异载荷） |
| 1004 | `VERSION_CONFLICT` | 409 | 版本冲突（乐观并发） |
| 2001 | `UNAUTHENTICATED` | 401 | 未认证 |
| 2002 | `FORBIDDEN` | 403 | 无权限（隐藏资源存在性） |
| 2003 | `CONFIRMATION_EXPIRED` | 410 | 确认凭证过期 |
| 2004 | `RATE_LIMITED` | 429 | 限频（携带 `Retry-After`） |
| 2005 | `NOT_TRADING_READY` | 503 | 交易就绪门禁未通过 |
| 2006 | `INTERNAL_SERVER_ERROR` | 500 | 内部错误 |
| 3000 | `COMMAND_REJECTED` | 422 | 命令状态机拒绝 |

### 1.4 认证与鉴权

- 生产服务端默认使用内置默认主体（模拟盘/仿真本地实验），`GET /health/*`、`/api/v1/version`、`/metrics` 公开；
- 安全中间件按路由定义权限规则（`SecurityMiddleware`），令牌/双人审批等能力在实盘安全阶段启用；
- 任何实盘（LIVE）操作默认禁用。

## 2. 端点一览

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/Health/Live` | 存活探针 |
| GET | `/Health/Ready` | 就绪探针（依赖 catalog 等就绪条件） |
| GET | `/api/v1/version` | 版本信息 |
| GET | `/metrics` | Prometheus 指标（text/plain，豁免信封） |
| GET | `/api/v1/stream/events` | SSE 状态流 |
| GET | `/api/v1/accounts` | 账户列表 |
| GET | `/api/v1/accounts/{account_id}` | 账户详情（`run_id` 可选） |
| GET | `/api/v1/accounts/{account_id}/ledger` | 账本分录 |
| GET | `/api/v1/accounts/{account_id}/cashflows` | 现金流 |
| GET | `/api/v1/accounts/{account_id}/shares` | 基金份额 |
| GET | `/api/v1/accounts/{account_id}/analysis` | 结果分析（TWR/XIRR/本金） |
| GET | `/api/v1/strategies` | 策略列表 |
| GET | `/api/v1/instruments` | 标的信息 |
| GET | `/api/v1/funds` | 基金列表 |
| POST | `/api/v1/backtests` | 创建回测 |
| GET | `/api/v1/backtests` | 回测列表 |
| GET | `/api/v1/backtests/{run_id}` | 回测状态 |
| POST | `/api/v1/backtests/{run_id}/start` | 启动回测 |
| POST | `/api/v1/backtests/{run_id}/cancel` | 取消回测 |
| POST | `/api/v1/commands` | 提交命令（幂等） |
| GET | `/api/v1/commands/{command_id}` | 查询命令状态 |
| POST | `/api/v1/commands/{command_id}/cancel` | 取消命令 |

## 3. 健康检查与版本

### 3.1 GET /Health/Live

存活探针：进程能响应即存活。

```json
{"code": 0, "message": "存活", "data": {"status": "ALIVE", "service": "veritasquant-api"}}
```

### 3.2 GET /Health/Ready

就绪探针：依赖（错误目录等）就绪才返回 READY，否则 503。

```json
{"code": 0, "message": "就绪", "data": {"status": "READY"}}
```

### 3.3 GET /api/v1/version

版本信息。

```json
{"code": 0, "message": "版本信息", "data": {"api_version": "v1", "catalog_version": "0.1.2", "service": "veritasquant-api"}}
```

## 4. 指标与状态流

### 4.1 GET /metrics

Prometheus 文本格式（`text/plain; version=0.0.4`），**不套业务信封**。包含平台核心指标（事件/账本/风控等计数，见 `MetricsRoutes`）。

### 4.2 GET /api/v1/stream/events（SSE）

Server-Sent Events 状态流：`text/event-stream`，推送账户/订单/风险等状态变更（P2-030 通道）。参数见 `StateStreamRoutes`。

## 5. 账户领域

### 5.1 GET /api/v1/accounts

账户列表。

**业务处理逻辑**：列表来自服务端 `VQ_ACCOUNTS` 环境变量（逗号分隔账户 ID）；每个账户附带 `execution_mode`（取自 `VQ_ENVIRONMENT`，仅 PAPER/SIMULATION，拒绝 LIVE）与 `run_id`（默认 null）。未配置 `VQ_ACCOUNTS` 时返回空列表。

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| （无必填） | | 返回全部可见账户 |

```json
{"code": 0, "message": "账户列表", "data": {"accounts": [{"account_id": "acc-paper-001", "execution_mode": "PAPER", "run_id": null}]}}
```

### 5.2 GET /api/v1/accounts/{account_id}

账户详情。`run_id` 查询参数可选（指定运行上下文）。

**业务处理逻辑**：账户必须存在于 `VQ_ACCOUNTS` 配置，否则返回 `1002 RESOURCE_NOT_FOUND`（404，不泄露资源存在性之外的细节）；存在时返回账户元信息（`execution_mode`、`snapshot`）。账本/现金流/份额/分析在无投影数据时返回空集合。

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `run_id` | string | 可选；运行标识 |

```json
{"code": 0, "message": "账户视图", "data": {"account_id": "acc-paper-001", "execution_mode": "PAPER", "run_id": null, "snapshot": {}}}
```

### 5.3 GET /api/v1/accounts/{account_id}/ledger

账本分录列表（按账户隔离；`run_id` 可选）。

```json
{"code": 0, "message": "账本分录", "data": {"entries": [{"journal_id": "j-1", "amount": "100.00"}]}}
```

### 5.4 GET /api/v1/accounts/{account_id}/cashflows

现金流列表（`run_id` 可选）。

### 5.5 GET /api/v1/accounts/{account_id}/shares

基金份额列表（`run_id` 可选）。

### 5.6 GET /api/v1/accounts/{account_id}/analysis

结果分析：现金流调整权益、TWR/XIRR、本金、份额、逐笔分录（`run_id` 可选）。

```json
{"code": 0, "message": "结果分析", "data": {"twr": null, "xirr": null, "principal": "0.00"}}
```

> 账户不存在返回 1002 `RESOURCE_NOT_FOUND`。

## 6. 目录（策略/标的/基金）

### 6.1 GET /api/v1/strategies

```json
{"code": 0, "message": "策略列表", "data": {"strategies": [{"strategy_id": "s-1", "version": "1.0.0"}]}}
```

### 6.2 GET /api/v1/instruments

```json
{"code": 0, "message": "标的列表", "data": {"instruments": [{"symbol": "FUND-A", "kind": "OTC_FUND"}]}}
```

### 6.3 GET /api/v1/funds

```json
{"code": 0, "message": "基金列表", "data": {"funds": [{"fund_symbol": "FUND-A", "status": "OPEN"}]}}
```

> 生产最小实现当前返回空列表（目录注册后续阶段接入）。

## 7. 回测

### 7.1 POST /api/v1/backtests

创建回测（返回 202）。

**业务处理逻辑**：请求体经 `BacktestConfigV1` 严格校验（run_id/account_id/strategy_id/strategy_version/数据区间必填、`initial_cash` 为 Decimal 字符串、`execution_mode` 为 PAPER/SIMULATION、`random_seed` 整数）→ `BacktestApplicationServiceV1.createRun` 创建回测运行 → 返回 202 与 `{run_id, status}`。

**失败场景**：校验失败 → `1001 VALIDATION_ERROR`（400）。

请求体：

```json
{
  "run_id": "bt-001",
  "account_id": "acc-paper-001",
  "strategy_id": "strat-1",
  "strategy_version": "1.0.0",
  "data_range_start": "2024-01-01",
  "data_range_end": "2024-12-31",
  "initial_cash": "1000000.00",
  "execution_mode": "PAPER",
  "execution_model_version": "1.0.0",
  "random_seed": 42
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `run_id` | string | ✅ | 运行标识 |
| `account_id` | string | ✅ | 账户标识 |
| `strategy_id` | string | ✅ | 策略标识 |
| `strategy_version` | string | ✅ | 策略版本 |
| `data_range_start` / `data_range_end` | string | ✅ | 数据区间（YYYY-MM-DD） |
| `initial_cash` | string | ✅ | 初始资金（Decimal 字符串，禁 float） |
| `execution_mode` | string | ✅ | `PAPER` / `SIMULATION` |
| `execution_model_version` | string | ✅ | 执行模型版本 |
| `random_seed` | int | ✅ | 随机种子 |

响应：

```json
{"code": 202, "message": "回测已创建", "data": {"run_id": "bt-001", "status": "CREATED"}}
```

### 7.2 GET /api/v1/backtests

回测列表。

```json
{"code": 0, "message": "回测列表", "data": {"backtests": [{"run_id": "bt-001", "status": "CREATED"}]}}
```

### 7.3 GET /api/v1/backtests/{run_id}

回测状态；不存在返回 1002。

### 7.4 POST /api/v1/backtests/{run_id}/start

启动回测（非法状态返回 1001）。

### 7.5 POST /api/v1/backtests/{run_id}/cancel

取消回测（非法状态返回 1001）。

## 8. 命令 API

### 8.1 POST /api/v1/commands

幂等提交命令（返回 202；同键同载荷返回原命令；同键异载荷返回 1003/409）。

**业务处理逻辑**（命令受理 + 异步执行）：

1. 服务端计算**幂等作用域**（主体 + 账户 + 路由 + `idempotency_key`）；
2. 查重：同键同载荷 → 返回原命令记录（重复提交安全）；同键异载荷 → `1003 IDEMPOTENCY_CONFLICT`（409）；
3. 首次提交 → 创建命令资源写入 `command_records` 表（status=`PENDING`，身份字段冻结不可变）；
4. 返回 `202 {command_id, status}`（受理）；
5. 执行端推进状态机 `PENDING→AUTHORIZING→ACCEPTED→RUNNING→SUCCEEDED/FAILED`（支持取消）；
6. **失败场景**：异步失败 → 状态 `FAILED` + 失败快照（`failure.code/error_code/catalog_version/retryable/details`）持久化在 `command_records.failure_*` 列，可审计；
7. **数据落点**：命令资源在 `command_records` 表；具体业务数据由命令执行端（如 `vq-job-data-ingestion`）落盘（行情/净值文件 + DataManifest 数据版本）。

请求体：

```json
{
  "command_id": "cmd-001",
  "command_type": "create_account",
  "account_id": "acc-paper-001",
  "run_id": "bt-001",
  "requested_by": "system",
  "idempotency_key": "unique-key-1",
  "payload": { },
  "expected_version": null
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `command_id` | string | ✅ | 命令标识 |
| `command_type` | string | ✅ | 命令类型 |
| `account_id` | string | ✅ | 账户标识 |
| `run_id` | string | ✅ | 运行标识 |
| `requested_by` | string | ✅ | 请求主体 |
| `idempotency_key` | string | ✅ | 幂等键 |
| `payload` | object | ✅ | 命令载荷 |
| `expected_version` | string | 可选 | 期望版本（乐观并发） |

### 8.2 GET /api/v1/commands/{command_id}

查询命令状态（含失败快照）；不存在返回 1002。

```json
{"code": 0, "message": "命令状态", "data": {"command_id": "cmd-001", "command_type": "create_account", "status": "SUCCEEDED", "created_ts": "...", "updated_ts": "..."}}
```

### 8.3 POST /api/v1/commands/{command_id}/cancel

取消命令（非法状态迁移返回 3000/422；不存在返回 1002）。

## 9. 使用示例（PowerShell）

```powershell
# 健康检查
curl.exe http://localhost:18000/Health/Live

# 版本
curl.exe http://localhost:18000/api/v1/version

# 账户列表
curl.exe http://localhost:18000/api/v1/accounts

# 创建回测
curl.exe -X POST http://localhost:18000/api/v1/backtests `
  -H "Content-Type: application/json" `
  -d '{"run_id":"bt-001","account_id":"acc-paper-001","strategy_id":"strat-1","strategy_version":"1.0.0","data_range_start":"2024-01-01","data_range_end":"2024-12-31","initial_cash":"1000000.00","execution_mode":"PAPER","execution_model_version":"1.0.0","random_seed":42}'
```

## 10. 相关文档

- 错误目录定义：`src/veritasquant/resources/Schemas/ApiErrorCodes.yml`
- 路由实现：`src/veritasquant/apps/server/`（ApiApp / DomainRoutes / CommandRoutes / StateStreamRoutes / MetricsRoutes）
- 部署：`Docker/Windows11Deployment.md`
