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
cp Deploy/.env.example Deploy/.env   # 按需修改
docker compose --env-file Deploy/.env up -d
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
├── Deploy/               # Docker Compose 编排
├── .github/workflows/    # CI/CD（构建 + 推送 GHCR）
├── Prompt.md             # 结构化需求文档（持续更新）
└── VeritasQuant/         # 既有子项目（Python 量化平台，历史保留）
```

## 文档

- [Prompt.md](Prompt.md) — 项目需求与技术基线（结构化，持续更新）
- [Deploy/Win11DockerDeploy.md](Deploy/Win11DockerDeploy.md) — Windows 11 Docker 部署文档
- [Deploy/Win11DockerUpgrade.md](Deploy/Win11DockerUpgrade.md) — Windows 11 Docker 增量升级文档
- [VeritasQuant/README.md](VeritasQuant/README.md) — 既有 Python 子项目说明

## 许可

[MIT](LICENSE) © 2026 ACANX
