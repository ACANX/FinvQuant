# P2-028 账户、策略、数据、回测、基金计划和报告 API 证据

## 任务信息
- **PlanTaskId:** P2-028
- **标题:** 实现账户、策略、数据、回测、基金计划和报告 API
- **阶段/里程碑:** M2A 模拟盘与基金能力建设
- **状态:** IN_REVIEW（等待第四批 PR 合并后验收）

## 实现内容

### 路由（`src/veritasquant/apps/server/DomainRoutes.py`）

| 领域 | 路由 | 语义 |
| --- | --- | --- |
| 账户 | `GET /api/v1/accounts/{account_id}?run_id=` | 显式 account_id/run_id；缺失 run_id -> 400；未知账户 404 |
| 策略 | `GET /api/v1/strategies` | 策略元数据列表（不执行策略代码） |
| 数据 | `GET /api/v1/instruments` | 标的列表（只读） |
| 基金 | `GET /api/v1/funds` | 基金列表（只读） |
| 回测 | `POST /api/v1/backtests` | 创建回测，202 + 状态引用 |
| 回测 | `GET /api/v1/backtests` | 回测列表 |
| 回测 | `GET /api/v1/backtests/{run_id}` | 查询状态；未知 404 |
| 回测 | `POST /api/v1/backtests/{run_id}/start` | 开始 |
| 回测 | `POST /api/v1/backtests/{run_id}/cancel` | 取消 |

### 设计要点
- 领域端口（AccountViewProvider 等 Protocol）注入 DomainApis，测试可替换替身
- 所有响应统一 ResponseEnvelopeV1；错误映射走注册错误目录
- 回测复用 BacktestApplicationServiceV1（复用 P1 状态机）
- OpenAPI schema 在 /api/v1/openapi.json 暴露，供契约测试与 GUI 生成客户端

## 验收标准映射
| 验收标准 | 实现 | 证据 |
| --- | --- | --- |
| 每个账户操作显式 account_id/run_id | 路径 + 查询参数强制 | test_account_requires_explicit_ids |
| OpenAPI 与契约测试覆盖所有返回码 | openapi.json + 信封契约测试 | test_openapi_schema_available |

## 测试结果
- `tests/unit/apps/test_domain_routes.py`：15 个测试通过
- 全量 767 测试通过（2026-08-03，本地）
- ruff/mypy: 通过

## 关键决策
- 只读查询路由不注入写服务，GUI 数据只经 API（TechSpec 10.1）
- 回测运行创建返回 202（长任务语义），状态查询走 GET
