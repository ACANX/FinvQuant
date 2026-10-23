# 第四批 P2-022~028：报告/回归/API 基础（M2A）

## 范围

按上游 ISSUE 顺序实现 7 个任务（P2-022 ~ P2-028）：

| 任务 | 内容 | 主要文件 |
| --- | --- | --- |
| P2-022 | 基金业绩报告：TWR、XIRR、投入本金、规则贡献 | `funds/FundPerformance.py` |
| P2-023 | 基金防前视回归套件 + MaDeviation 前视修复 | `tests/regression/test_fund_anti_lookahead.py`、`funds/SmartPlans.py` |
| P2-024 | FastAPI 应用、依赖注入、版本路由、健康接口 | `apps/server/ApiApp.py`、`ApiRuntime.py`、`ApiServer.py` |
| P2-025 | 统一响应中间件和异常处理器 | `apps/server/ApiMiddleware.py` |
| P2-026 | 不可变命令资源和幂等键存储 | `application/CommandResource.py`、`infrastructure/persistence/CommandStore.py`、`Migrations/postgresql/V2__command_resources.sql` |
| P2-027 | 命令 API：202 受理、取消、失败快照、乐观并发 | `apps/server/CommandRoutes.py` |
| P2-028 | 账户、策略、数据、回测、基金计划和报告 API | `apps/server/DomainRoutes.py` |

## 非目标

- 不实现 RBAC/鉴权/SSE（P2-029/030）
- 不实现 Streamlit GUI（P2-031~033）
- 不实现调度 JobRun（P2-034/035）
- 不接入 Prometheus 指标（P2-036/037）

## 技术方案影响

- 新增 FastAPI/uvicorn 运行依赖（已批准，Runtime.lock 固定）；dev 新增 httpx（TestClient）
- 新增 V2 迁移：`command_records` 表 + 幂等唯一索引 + 身份冻结触发器
- 依赖方向：`apps -> application -> 领域`，领域模块不反向依赖 FastAPI

## 失败模式与兼容性

- 命令身份字段被数据库触发器冻结，绕过应用层的 UPDATE 被拒
- 非信封 JSON 响应安全降级 2006，不泄露内部载荷
- OpenAPI/docs 框架 JSON 豁免业务信封
- 幂等作用域唯一索引保证并发双写只有一个成功

## 测试与验证

- 全量 **767** 测试通过（本地，2026-08-03）
- ruff / mypy / Preflight 通过
- packaging 15 测试通过（入口离线校验契约保持）
- 新增数据库集成测试（V2 迁移 + 命令存储）由 CI postgres service 运行
- 真实启动验证：`vq-api-server --serve` 后 liveness/readiness/version 均 200

## 可观测性

- liveness/readiness 分层健康接口（TechSpec 12.3）
- 统一响应信封含 code/message/request_id/trace_id 契约
- 命令失败快照保存 code/error_code/catalog_version/retryable

## 迁移/回滚

- V2 迁移单事务执行，失败自动回滚
- 回滚需新迁移，禁止运行时改表

## 开放风险

- 无阻断风险；等待 ACANX review 与合并

## 登记表

- WorkItemRegister：P2-022~028 已登记（IN_REVIEW）
- TraceabilityMatrix：新增 R-019~R-022 行
- 证据文档：P2-022~028 各 1 份（Docs/DevelopmentWorkflow/P2/）
