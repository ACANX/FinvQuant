# FinvQuant

量化策略交易平台，**前后端分离**架构：

- **服务端**：Go 1.25.3 + Gin + PostgreSQL 18 + Redis 8（端口 **16001**）
- **前端**：Vue3 + Vite8 + Vuetify4（端口 **16002**）
- **部署**：Docker Compose / GitHub Packages (GHCR) 镜像

## 快速开始（Docker）

```bash
# 1. 克隆代码（默认分支 dev）
git clone https://github.com/ACANX/FinvQuant.git
cd FinvQuant

# 2. 准备环境变量并启动
cp Deploy/Docker/.env.example Deploy/Docker/.env   # 按需修改
docker compose -f Deploy/Docker/docker-compose.yml --env-file Deploy/Docker/.env up -d
```

- 前端控制台：http://localhost:16002
- 服务端 API：http://localhost:16001/API/V1/Health/Live

或直接使用 All-in-One 镜像：

```bash
docker pull ghcr.io/acanx/finvquant:latest
docker run -d --name finvquant -p 16001:16001 -p 16002:16002 \
  -e FINV_PG_HOST=host.docker.internal -e FINV_PG_USER=finv -e FINV_PG_DATABASE=finv \
  -e FINV_REDIS_ADDR=host.docker.internal:6379 \
  ghcr.io/acanx/finvquant:latest
```

## 本地开发

```bash
# 服务端（Go 1.25.3）
go run ./cmd/server

# 前端（端口 16002，/api 代理到 16001）
cd Web && npm install && npm run dev
```

## 目录结构

```
├── cmd/  internal/       # Go 服务端
├── Web/                  # Vue3+Vite8+Vuetify4 前端
├── Deploy/               # 部署（Docker / Native 两种方式 + 共用迁移与备份）
│   ├── Docker/           # 容器方式（compose + Dockerfile + 脚本/文档）
│   ├── Native/           # 宿主直装方式（systemd + 安装/升级脚本与文档）
│   ├── Migrations/       # 数据库迁移（共用）
│   ├── Backup/           # 数据库备份（共用，不入库）
│   └── README.md         # 部署目录总览
├── Docs/                 # 文档（变更日志、需求清单、开发规范、改造清单、API/菜单/数据字典、FinvQuant/V1/Prompt.md 需求基线）
├── Archive/              # 仓库级历史归档（含 VeritasQuant 历史子项目，不参与构建）
├── .github/workflows/    # CI/CD（构建 + 推送 GHCR）
├── AGENTS.md             # 仓库级协作指南
└── VERSION               # 版本号（与 CHANGELOG、Git tag 对应）
```

## 文档

- [Docs/CHANGELOG.md](Docs/CHANGELOG.md) — 版本变更日志（与 Git tag、`VERSION` 对应，文件名全大写）
- [Docs/RequirementList.md](Docs/RequirementList.md) — 需求清单（自 2026-10-24 起的新增需求与核对方式）
- [Docs/FinvQuant/V1/Prompt.md](Docs/FinvQuant/V1/Prompt.md) — 项目需求与技术基线（结构化，持续更新）
- [Deploy/Docker/Win11DockerDeploy.md](Deploy/Docker/Win11DockerDeploy.md) — Windows 11 Docker 部署文档
- [Deploy/Docker/Win11DockerUpgrade.md](Deploy/Docker/Win11DockerUpgrade.md) — Windows 11 Docker 增量升级文档
- [Deploy/README.md](Deploy/README.md) — 部署目录总览（Docker / Native 两种方式与共用迁移、备份）
- [Archive/README.md](Archive/README.md) — 仓库级历史归档目录说明（废弃/临时文件存放约定）
- [Deploy/Native/README.md](Deploy/Native/README.md) — Native（宿主直装）部署方式说明与适用场景
- [Archive/VeritasQuant/README.md](Archive/VeritasQuant/README.md) — 历史归档的 Python 子项目说明（不参与现行构建）

## 许可

[MIT](LICENSE) © 2026 ACANX
