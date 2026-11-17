# P2-025 统一响应中间件和异常处理器证据

## 任务信息
- **PlanTaskId:** P2-025
- **标题:** 实现统一响应中间件和异常处理器
- **阶段/里程碑:** M2A 模拟盘与基金能力建设
- **状态:** IN_REVIEW（等待第四批 PR 合并后验收）

## 实现内容

### ResponseEnvelopeMiddleware（`src/veritasquant/apps/server/ApiMiddleware.py`）
- **204 拦截**：空 204 替换为 200 + code/message 信封（TechSpec：JSON 接口不得返回无响应体的 204）
- **非信封 JSON 降级**：裸 JSON 响应安全降级为 2006 INTERNAL_SERVER_ERROR，不泄露内部载荷
- **OpenAPI 豁免**：框架自产 JSON（openapi.json/docs/redoc）不套用业务信封

### 异常处理器（`src/veritasquant/apps/server/ApiApp.py`）
- `RequestValidationError` → VALIDATION_ERROR(1001)，400
- Starlette `HTTPException` → 404 映射 RESOURCE_NOT_FOUND(1002)，其他映射 VALIDATION_ERROR(1001)
- 未预期 `Exception` → 通过 `mapException` 查错误目录安全映射；未知码降级 2006，不泄露堆栈

### 信封契约（复用 P1 ResponseEnvelopeV1）
- 所有 JSON 路由固定 `code`/`message`
- 错误 `retryable` 仅存在于 `error` 对象内，顶层禁止
- `data`/`details`/`error` 未使用默认省略，不用 null 占位

## 验收标准映射
| 验收标准 | 实现 | 证据 |
| --- | --- | --- |
| 所有 JSON 路由固定 code/message | 信封 + 中间件强制 | tests/unit/apps/test_api_middleware.py |
| 错误 retryable 仅位于 error 内 | ResponseEnvelopeV1 模型约束 | test_envelope_error_keeps_retryable_inside_error_only |
| 无空 204 | 中间件 204 拦截 | test_empty_204_is_replaced_with_envelope |

## 测试结果
- `tests/unit/apps/test_api_middleware.py`：6 个测试通过
- 全量 767 测试通过（2026-08-03，本地）
- ruff/mypy: 通过

## 关键决策
- 中间件只负责信封强制，业务错误码映射仍在应用层 mapException
- 非信封 JSON 视为内部错误而非静默放行，防止绕过契约
