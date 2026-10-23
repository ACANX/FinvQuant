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
  -e FINV_PG_HOST=host.docker.internal -e FINV_REDIS_ADDR=host.docker.internal:6379 \
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
├── Deploy/               # 部署（两种方式共用 Migrations/、Backup/）
│   ├── Docker/           # 容器方式（compose + Dockerfile + 脚本/文档）
│   ├── Native/           # 宿主直装方式（说明文档，脚本待补充）
│   ├── Migrations/       # 数据库迁移（共用）
│   └── Backup/           # 数据库备份（共用，不入库）
├── .github/workflows/    # CI/CD（构建 + 推送 GHCR）
├── Prompt.md             # 结构化需求文档（持续更新）
└── VeritasQuant/         # 既有子项目（Python 量化平台，历史保留）
```

## 文档

- [Docs/CHANGELOG.md](Docs/CHANGELOG.md) — 版本变更日志（与 Git tag、`VERSION` 对应，文件名全大写）
- [Docs/RequirementList.md](Docs/RequirementList.md) — 需求清单（自 2026-10-24 起的新增需求与核对方式）
- [Docs/Refactor/2610-SecuCodeToUsc.md](Docs/Refactor/2610-SecuCodeToUsc.md) — 改造项 A 检查清单（`secu_code`→`usc`）
- [Docs/Refactor/2610-PrevCloseToLastClose.md](Docs/Refactor/2610-PrevCloseToLastClose.md) — 改造项 B 检查清单（`prev_close`→`last_close`）
- [Docs/Refactor/2610-MoveTablesToSchemas.md](Docs/Refactor/2610-MoveTablesToSchemas.md) — 改造项 C 检查清单（`finv_quote_*`→`quote`、`finv_quant_*`→`quant` schema）
- [Docs/Refactor/2610-MoveFutuTablesToSchema.md](Docs/Refactor/2610-MoveFutuTablesToSchema.md) — 改造项 D 检查清单（`finv_futu_*`→`futu` schema）
- [Prompt.md](Prompt.md) — 项目需求与技术基线（结构化，持续更新）
- [Deploy/Docker/Win11DockerDeploy.md](Deploy/Docker/Win11DockerDeploy.md) — Windows 11 Docker 部署文档
- [Deploy/Docker/Win11DockerUpgrade.md](Deploy/Docker/Win11DockerUpgrade.md) — Windows 11 Docker 增量升级文档
- [Deploy/README.md](Deploy/README.md) — 部署目录总览（Docker / Native 两种方式与共用迁移、备份）
- [Deploy/Native/README.md](Deploy/Native/README.md) — Native（宿主直装）部署方式说明与适用场景
- [VeritasQuant/README.md](VeritasQuant/README.md) — 既有 Python 子项目说明

## 许可

[MIT](LICENSE) © 2026 ACANX
