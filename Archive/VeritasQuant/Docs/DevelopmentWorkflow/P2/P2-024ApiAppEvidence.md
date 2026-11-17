# P2-024 FastAPI 应用、依赖注入、版本路由和健康接口证据

## 任务信息
- **PlanTaskId:** P2-024
- **标题:** 建立 FastAPI 应用、依赖注入、版本路由和健康接口
- **阶段/里程碑:** M2A 模拟盘与基金能力建设
- **状态:** IN_REVIEW（等待第四批 PR 合并后验收）

## 实现内容

### 应用组装（`src/veritasquant/apps/server/ApiApp.py`）
- `createApp(deps)` 纯函数：导入无副作用，测试可注入替身依赖
- `ApiDependencies` 显式依赖注入容器（错误目录、版本提供者、健康探针、命令/领域 API）
- 基路径固定 `/api/v1`，OpenAPI 文档挂载于 `/api/v1/docs`

### 版本路由与健康接口
- `GET /api/v1/version`：返回 api_version、catalog_version（包元数据）、service
- `GET /Health/Live`：liveness，进程存活即 200 code=0
- `GET /Health/Ready`：readiness，全部探针通过才 READY；失败返回 503 + 2005
- `ErrorCatalogProbe`：错误目录已加载且含必需错误码

### 分层健康检查（TechSpec 12.3）
- liveness：进程主循环能否响应（决定是否重启）
- readiness：依赖自检通过（决定是否接收流量）
- trading-readiness 由 P2-009 TradingReadinessGateV1 覆盖

### 入口（`src/veritasquant/apps/server/ApiServer.py`）
- 默认离线参数校验（packaging 契约：--help 返回 0）
- `--serve` 时组装依赖并启动 uvicorn

## 验收标准映射
| 验收标准 | 实现 | 证据 |
| --- | --- | --- |
| /api/v1 可用 | version 路由 + OpenAPI | tests/unit/apps/test_api_app.py |
| liveness/readiness 可用 | /Health/Live、/Health/Ready | 同上 |
| 入口导入无副作用 | createApp 纯函数 | test_import_has_no_side_effects |
| GUI 不直连数据库 | 依赖注入 + 端口隔离 | 架构约束（TechSpec 11.1） |

## 测试结果
- `tests/unit/apps/test_api_app.py`：11 个测试通过
- 真实启动验证：uvicorn --serve 后 liveness/readiness/version 均 200
- packaging 15 测试通过；ruff/mypy 通过

## 关键决策
- 领域层（application.ApiApp）只定义端口与结果模型，不依赖 FastAPI
- 版本号从已安装 wheel 元数据读取，回退 0.0.0
