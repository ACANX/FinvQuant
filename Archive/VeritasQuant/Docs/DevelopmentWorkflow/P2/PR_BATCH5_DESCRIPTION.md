# 第五批 P2-029~035：RBAC/SSE/GUI/调度（M2A）

## 范围

按上游 ISSUE 顺序实现 7 个任务（P2-029 ~ P2-035）：

| 任务 | 内容 | 主要文件 |
| --- | --- | --- |
| P2-029 | 基础 RBAC、请求/追踪 ID、审计和限频 | `application/Security.py`、`apps/server/SecurityMiddleware.py` |
| P2-030 | 鉴权 SSE 状态流和有界 replay cursor | `application/StateStream.py`、`apps/server/StateStreamRoutes.py` |
| P2-031 | Streamlit 框架、导航和 API Client | `apps/guiclient/ApiClient.py`、`GuiApp.py`、`GuiServer.py` |
| P2-032 | 数据导入、策略、定投计划和回测操作页 | `apps/guiclient/Pages.py` |
| P2-033 | 账户、结果分析、逐笔账本和监控页 | `DomainRoutes.py` 扩展、`Pages.py` |
| P2-034 | 调度计划、JobRun 状态机和 console job 入口 | `application/Scheduling.py`、`jobs/JobEntrypoint.py`、`SchedulerService.py` |
| P2-035 | 数据导入、对账、校准和报告任务清单 | `application/JobTasks.py`、`jobs/*`、`Jobs/JobManifests.yml` |

## 非目标

- 不实现实盘双人授权/密钥服务（P5-003~005）
- 不实现 M2 Gate 发布（P2-043）
- 不实现 Prometheus 指标接入（P2-036/037）

## 技术方案影响

- 新增 streamlit 1.60.0 运行时依赖（许可证 8 项登记豁免，锁文件固定）
- 新增 `vq-gui` console script（packaging 契约同步更新）
- DomainRoutes 扩展 4 个账户域端点（ledger/cashflows/shares/analysis）
- Jobs/JobManifests.yml 4 条 PascalCase 调度清单（根级部署清单）

## 失败模式与兼容性

- 安全中间件自处理 2001/2002/2004 信封（中间件层异常不到达 FastAPI handler）
- SSE 使用独立协议版本，不套用 ResponseEnvelopeV1
- JobRun 终态不可回退；租约丢失（fencing token 不匹配）拒绝更新
- 任务执行键哈希幂等：重复触发/补跑不重复业务副作用
- GUI 只经 API（TechSpec 10.1），页面从 session_state 读账户上下文

## 测试与验证

- 全量 **918** 测试通过（本地，2026-08-03）
- ruff / mypy / Preflight / 许可证 / 依赖锁全部通过
- packaging 16 测试通过（含 vq-gui 与 4 个 vq-job-* 入口契约）
- 真实启动验证：vq-api-server 鉴权 401/2001 正确；vq-gui Streamlit 启动成功；
  vq-job-account-reconciliation 冒烟退出 0；清单解析 4 条计划

## 可观测性

- 审计记录：请求级 ALLOWED/DENIED + permission + resource
- 请求/追踪 ID 贯穿 API 响应头与错误信封
- 任务结构化日志（veritasquant.jobs）+ 退出码语义
- SSE close 事件携带 close_reason（backlog_exceeded/permission_revoked）

## 迁移/回滚

- 无数据库迁移（P2-034 调度使用进程内 JobStore，持久化在 P2-038 后接 PG）
- 依赖变更可回滚（锁文件版本化）

## 开放风险

- InMemoryJobStore/InMemoryStreamEventSource 为模拟盘默认，生产需持久化实现
  （P2-038 集成测试阶段接 PostgreSQL/Redis）
- 待 ACANX review 合并

## 登记表

- WorkItemRegister：P2-029~035 已登记（IN_REVIEW），总计 124 条
- TraceabilityMatrix：P2-029~033 → R-013、P2-034/035 → R-017
- 证据文档：P2-029~035 各 1 份（Docs/DevelopmentWorkflow/P2/）
