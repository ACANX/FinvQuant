# FinvQuant 项目需求文档（Prompt）

> 本文档是 FinvQuant 仓库（量化策略交易平台）的结构化需求说明，**支持后续持续更新与补充完善**。
> 更新方式：直接编辑本文件，保持分节结构，在对应章节追加或修订内容；重大变更请在文末"变更记录"登记。

## 1. 项目定位

FinvQuant 是一个**量化策略交易平台**，采用**前后端分离**架构：
- 服务端：Go 语言（Gin），提供量化策略与交易相关 API；
- 前端：Web 控制台（Vue3），提供策略管理、行情与交易界面。

## 2. 仓库结构

```
.
├── cmd/server/            # Go 服务端入口（端口 16001）
├── internal/              # Go 服务端内部模块
│   ├── api/               #   Gin 路由与处理器
│   ├── config/            #   配置加载（环境变量）
│   ├── database/          #   PostgreSQL 18 连接（pgx/v5）
│   └── redisclient/       #   Redis 8 连接（go-redis/v9）
├── Web/                   # 前端（Vue3 + Vite8 + Vuetify4，端口 16002）
├── Deploy/                # Docker Compose 部署编排
├── .github/workflows/     # GitHub Actions：构建 + 推送 GHCR 镜像
├── Dockerfile             # 服务端镜像（多阶段构建）
├── VeritasQuant/          # 既有子项目（Python 量化平台，历史保留）
├── go.mod / go.sum        # Go 模块定义
├── .gitignore             # Go 版本忽略规则
└── Prompt.md              # 本需求文档
```

## 3. 技术选型（当前基线）

| 层 | 技术 | 版本 | 说明 |
|----|------|------|------|
| 服务端语言 | Go | **1.25.3** | 平台服务端唯一语言 |
| Web 框架 | Gin | **最新**（v1.12.x） | HTTP API 框架 |
| 数据库 | PostgreSQL | **18** | 业务主库（pgx/v5 驱动） |
| 缓存/消息 | Redis | **8** | go-redis/v9 客户端 |
| 前端框架 | Vue | 3.x | Composition API |
| 构建工具 | Vite | **8.x** | 前端构建/开发服务器 |
| UI 组件库 | Vuetify | **4.x** | Material Design 组件 |
| 包管理器 | npm | 最新 | 前端依赖管理 |
| Go 模块代理 | goproxy.cn | — | 国内构建加速（可选） |

> 版本策略：依赖采用"最新稳定版"；升级时同步更新本文档与 `go.mod`/`package.json`。

## 4. 服务端（Go）设计

### 4.1 端口与启动
- 默认监听端口：**16001**
- 配置方式：环境变量（`FINV_*` 前缀），见 `internal/config/config.go`
- 健康检查：`GET /API/V1/health/live`、`GET /API/V1/health/ready`
- 版本信息：`GET /API/V1/version`

### 4.2 模块划分（internal）
- `api`：Gin 路由注册、HTTP 处理器
- `config`：环境变量配置加载
- `database`：PostgreSQL 18 连接池（pgxpool + Ping 探活）
- `redisclient`：Redis 8 客户端（Ping 探活）

### 4.3 依赖基线
- `github.com/gin-gonic/gin` v1.12.x
- `github.com/redis/go-redis/v9` v9.22.x
- `github.com/jackc/pgx/v5` v5.x

## 5. 前端（Web）设计

- 默认端口：**16002**
- 技术栈：Vue 3.5 + Vite 8.2 + Vuetify 4.1（TypeScript）
- 目录：`Web/`（vite 标准结构）
- 开发代理：`/api` → `http://localhost:16001`（见 `Web/vite.config.ts`）
- 生产部署：Nginx 静态托管 + `/api` 反向代理到服务端 16001（见 `Web/nginx.conf`）

## 6. 部署与镜像

### 6.1 All-in-One 镜像（GitHub Packages / GHCR）

**单镜像 `ghcr.io/acanx/finvquant`**：一个容器同时提供服务端与前端（Go 内嵌前端构建产物，单进程双端口）。

| 端口 | 服务 |
|------|------|
| 16001 | Go 服务端 API（Gin） |
| 16002 | 前端 Web（内嵌静态资源，SPA fallback） |

> 设计说明：前端静态文件通过 `go:embed` 内嵌进 Go 二进制，无需独立 Nginx 镜像，拉取**一个镜像**即可完整部署。

### 6.2 本地 Docker 部署

```bash
# 方式一：Compose 一键（含 PG18 / Redis8）
cp Deploy/.env.example Deploy/.env   # 按需修改
docker compose --env-file Deploy/.env up -d

# 方式二：直接拉取 All-in-One 镜像运行
docker pull ghcr.io/acanx/finvquant:latest
docker run -d --name finvquant -p 16001:16001 -p 16002:16002 \
  -e FINV_PG_HOST=host.docker.internal -e FINV_REDIS_ADDR=host.docker.internal:6379 \
  ghcr.io/acanx/finvquant:latest
```

### 6.3 依赖服务（Compose 内置，独立镜像）
- `postgres`：`postgres:18-alpine`（宿主映射 5433）
- `redis`：`redis:8-alpine`（宿主映射 6380）

### 6.4 数据持久化（宿主机映射）

**PostgreSQL 数据目录映射到 Docker 宿主机文件系统**（容器重建/删除不丢数据），通过 `FINV_PG_DATA_DIR` 配置：

| 平台 | 示例 |
|------|------|
| Windows 11 | `D:\Dev\Docker\HostFileSystem\FinvQuant\PostgreSQL` |
| Linux/macOS | `/data/finvquant/postgresql` |

```dotenv
# Deploy/.env
FINV_PG_DATA_DIR=D:/Dev/Docker/HostFileSystem/FinvQuant/PostgreSQL
```

未设置时默认使用 `Deploy/pgdata`（Compose 项目相对目录）。

> ⚠️ PG18+ 镜像数据目录变更：挂载点为 `/var/lib/postgresql`（单一挂载），实际数据在宿主目录下的 `18/` 子目录；挂载 `/var/lib/postgresql/data` 会被镜像判定为 unused mount 并拒绝启动。

## 7. CI/CD（GitHub Actions）

- 文件：`.github/workflows/build-publish.yml`
- 触发：push（dev/main/FinvQuant）、tag `v*`、PR、手动
- 流程：
  1. `build-server`：Go vet + build + test（Go 1.25.3）
  2. `build-web`：npm ci + build（Node 24）
  3. `docker`（非 PR）：构建并推送 **All-in-One 镜像** `ghcr.io/acanx/finvquant`（latest + 版本 tag）
  4. `docker-pr`（PR）：仅构建不推送，验证镜像可构建

## 7.5 数据库建表规范

- 所有业务表名必须以 **`finv_` 作为前缀**。
- 行情模块表名统一为 **`finv_quote_xxx`**（例如 `finv_quote_secu_kline_min`、`finv_quote_ingest_batches`、`finv_quote_revision_log`）。
- 迁移文件存放于 `Deploy/Migrations/`，命名 `V<number>__<name>.sql`，服务端启动时自动应用。
- **迁移分段约定**：`V1~V99999` 为表结构/变更脚本（DDL），`V100000+` 为数据种子脚本（DML）；初始数据必须放在数据种子段，确保在所有表结构脚本执行完后再执行。

## 8. 端口约定（汇总）

| 服务 | 端口 |
|------|------|
| Go 服务端 | **16001** |
| 前端 Web | **16002** |
| PostgreSQL（宿主映射） | 5433 |
| Redis（宿主映射） | 6380 |

## 9. 待办 / 规划（Roadmap）

- [x] 历史行情导入：PG 建表（V1 迁移启动自动执行）、`POST /API/V1/Quote/Import/Upload` 上传导入（支持备注字段）、前端「历史行情数据导入」菜单页
- [x] 历史行情查询：`GET /API/V1/Quote/Query`（secu_code/date/period=Min）、前端「历史行情查询」菜单页（K 线蜡烛图、红涨绿跌、悬停详情）
- [ ] 服务端业务模块：行情、账户、策略、订单、风控 API
- [ ] 前端业务页面：策略管理、行情看板、交易面板
- [ ] 数据库迁移与 Schema 管理（golang-migrate / goose）
- [ ] Redis 缓存策略与消息通道接入
- [ ] 认证授权（JWT / RBAC）
- [ ] 与 `VeritasQuant/` 子项目的数据/能力集成（行情导入 PG 等）

## 10. 变更记录

| 日期 | 变更 | 说明 |
|------|------|------|
| 2026-08-04 | 初始版本 | 初始化 Go 服务端（Gin 最新 / PG18 / Redis8 / Go 1.25.3）+ Vue3+Vite8+Vuetify4 前端；端口 16001/16002；GHCR 镜像构建与 Docker Compose 部署；.gitignore 改 Go 版 |
| 2026-08-04 | All-in-One 镜像 | 合并 server/web 双镜像为单镜像 `ghcr.io/acanx/finvquant`：前端经 `go:embed` 内嵌进 Go 二进制，单进程双端口（16001 API + 16002 前端），拉取一个镜像即可完整部署 |
| 2026-08-04 | 目录与持久化 | `deploy/` 重命名为 `Deploy/`；PG 数据目录支持映射到 Docker 宿主机文件系统（`FINV_PG_DATA_DIR`，Windows 示例 `D:\Dev\Docker\HostFileSystem\FinvQuant\PostgreSQL`） |
| 2026-08-04 | API 路径大写 | API 路径统一 `/API/V1/` 前缀（写入 ApiSpec 规范） |
| 2026-08-04 | 历史行情导入 | PG 建表（`Deploy/Migrations/V1__finv_quote_secu_kline_min.sql` 启动自动迁移）；Go MVSV-1 解析器 + 字段级覆盖 upsert 导入服务；`POST /API/V1/Quote/Import/Upload`；前端新增「历史行情数据导入」菜单页（批量上传 MVSV 分钟行情） |
| 2026-08-04 | 规范与文档 | 建表规范：`finv_` 前缀、行情表 `finv_quote_xxx`；迁移重命名 `Deploy/Migrations/V1__finv_quote_secu_kline_min.sql`（表名修正 `finv_quote_ingest_batches`/`finv_quote_revision_log`）；新增 `Docs/API/` 服务端接口文档（含 /Quote/Import/Upload 端点） |
