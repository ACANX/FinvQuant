# ISSUE #253 Windows 11 Docker 部署验证 — 证据

- **任务：** ISSUE #253（部署验证：Win11 Docker 服务端 + 本地客户端）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **请求人：** ACANX（朋友个人需求）
- **PR：** 本 PR（部署批次）

## 范围

为 Windows 11 本地验证环境交付服务端 Docker 部署物与客户端连接方案：
镜像容器部署服务端（API + PostgreSQL + Redis），本地启动客户端完成模拟盘/券商仿真实验。

## 验收标准对照

| 验收标准 | 交付物 | 验证 |
|----------|--------|------|
| Docker 服务端可部署（容器全部健康） | `Docker/Dockerfile`（多阶段构建、非 root、只读根、HEALTHCHECK）+ `Docker/docker-compose.deploy.yml`（server/postgresql/redis 三服务、健康检查、持久卷、`depends_on` 条件） | `tests/contract/test_deploy_win11_docker.py` 编排结构断言 |
| 本地客户端可连接服务端完成模拟/仿真实验 | 客户端入口（`vq-run-paper-trading`/`vq-gui`）连接 `http://localhost:8000`；教程第 7 步给出实验路径 | 教程文档 + console script 清单 |
| 部署说明覆盖环境要求/依赖说明/详细步骤/常见问题 | `Docker/Windows11Deployment.md`（8 节：环境要求/获取代码/部署物/依赖/步骤/FAQ/安全） | 教程内容断言（11 用例中 3 项） |
| 部署脚本可复现 | `scripts/DeployServer.py`（check/build/start/status/logs/stop）+ `Docker/.env.deploy.example` + `.gitignore` 忽略明文密码 | 脚本命令构造测试 + 无 Docker 时明确失败验证 |

## 技术方案要点

- 多阶段镜像：builder 构建 wheel（不携带构建工具链），runtime 仅运行时依赖；
- 非 root 用户（uid/gid 10001）+ `read_only: true` + `no-new-privileges` 最小权限；
- PostgreSQL `scram-sha-256` + 密码必填环境变量（无弱默认）；持久卷保留数据；
- 部署脚本 Docker 不可用时明确失败，绝不回落本机服务（沿用 P0-008 模式）；
- 实盘 LIVE 默认禁用，教程与编排均声明边界。

## 验证结果

- 本批新增 **11** 个契约测试，全部通过；
- `scripts/DeployServer.py check` 在本机（无 Docker）明确报错退出码 1（预期行为）；
- ruff / mypy / Preflight 全绿；
- 更新：TechSpec 新增 8.11「容器化部署与本地运行契约」；
- 登记表 DEPLOY-001 登记（IN_REVIEW）；TraceabilityMatrix 挂接。
