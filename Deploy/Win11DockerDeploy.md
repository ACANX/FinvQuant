# FinvQuant 部署到 Windows 11 Docker（部署文档）

> 所属：FinvQuant 部署 · 存放：`Deploy/Win11DockerDeploy.md`
> 编排文件：[`Deploy/docker-compose.yml`](docker-compose.yml) · 环境变量示例：[`Deploy/.env.example`](.env.example)
> 适用：首次在 **Windows 11** 上通过 Docker Compose 部署 FinvQuant（All-in-One）。

## 1. 部署架构

`docker-compose.yml` 编排 3 个容器：

| 服务 | 镜像 | 端口（宿主） | 说明 |
|------|------|--------------|------|
| `finvquant` | `ghcr.io/acanx/finvquant:${FINV_IMAGE_TAG:-latest}` | 16001 / 16002 | All-in-One：Go 服务端（16001 API）+ 内嵌前端（16002 Web） |
| `postgres` | `postgres:18-alpine` | 127.0.0.1:5432（默认，可改 `FINV_PG_EXPOSE_PORT`） | PostgreSQL 18，数据绑定挂载到宿主机目录 |
| `redis` | `redis:8-alpine` | 127.0.0.1:6380（默认，可改 `FINV_REDIS_EXPOSE_PORT`） | Redis 8，AOF 持久化，数据在命名卷 `finvquant_finvquant-redisdata` |

- `finvquant` 通过 Compose 内网（服务名 `postgres` / `redis`）访问数据库，**不需要** `host.docker.internal`。
- 服务端启动时**自动应用数据库迁移**（`Deploy/Migrations/`，幂等，见 [升级文档](Win11DockerUpgrade.md) 第 4 节）。

## 2. 前置条件

- **Windows 11**（64 位，建议保持系统更新）。
- **Docker Desktop for Windows**（含 Docker Compose v2 插件），后端推荐 **WSL2**。
- 空闲端口：`16001`、`16002`、`5432`（PG 宿主映射）、`6380`（Redis 宿主映射）；如被占用见 [7. 常见问题](#7-常见问题)。
- 网络可达 `ghcr.io`（拉取镜像）与 `github.com`（拉取部署文件）。

## 3. 安装 Docker Desktop（WSL2）

1. 下载安装 [Docker Desktop for Windows](https://www.docker.com/products/docker-desktop/)。
2. 安装完成后打开 **Settings → General**，确认勾选 **Use the WSL 2 based engine**。
3. **Settings → Resources → WSL Integration**，勾选要集成的发行版（或使用默认的 Docker 专用 WSL 环境）。
4. PowerShell 验证：

```powershell
docker version
docker compose version
```

> 提示：Docker Desktop 的 WSL2 磁盘默认位于 `C:\Users\<用户名>\AppData\Local\Docker\wsl`；大数据量读写场景建议把 Docker Desktop 数据目录迁移到空间充足的盘符（Settings → Resources → Advanced → Disk image location）。

## 4. 获取部署文件

**方式 A：克隆仓库（推荐，含完整迁移脚本与文档）**

```powershell
git clone https://github.com/ACANX/FinvQuant.git
cd FinvQuant
```

**方式 B：仅取部署所需文件**（使用 GHCR 镜像，无需源码构建）

```powershell
# 需要 Deploy/ 目录（docker-compose.yml + .env.example + Migrations/）
# 迁移脚本随镜像内嵌，服务启动时自动应用；本地 Migrations/ 仅用于查阅与手动核对
```

> 部署默认使用 GHCR 镜像 `ghcr.io/acanx/finvquant`，**不需要本地构建**（`docker-compose.yml` 中的 `build:` 段仅用于 CI 或源码构建场景）。

## 5. 配置环境变量

```powershell
Copy-Item Deploy\.env.example Deploy\.env
# 用编辑器打开 Deploy\.env 按需修改
```

各变量说明：

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `FINV_PG_USER` | `finvquant` | PostgreSQL 用户 |
| `FINV_PG_PASSWORD` | `finvquant` | PostgreSQL 密码（生产环境务必修改） |
| `FINV_PG_DATABASE` | `finvquant` | PostgreSQL 数据库名 |
| `FINV_PG_EXPOSE_PORT` | `5432` | PG 暴露到宿主机的端口（本机 5432 被占时改为 5433 等） |
| `FINV_PG_DATA_DIR` | `./pgdata` | **PG 数据目录**（Windows 推荐 `D:/Dev/Docker/HostFileSystem/FinvQuant/PostgreSQL`，见下方注意事项） |
| `FINV_REDIS_PASSWORD` | 空 | Redis 密码（生产环境建议设置） |
| `FINV_REDIS_EXPOSE_PORT` | `6380` | Redis 暴露到宿主机的端口 |
| `FINV_IMAGE_TAG` | `latest` | 镜像 tag（`latest` 或版本号 `v*`，升级时使用） |
| `FINV_CONTAINER_NAME` | `finvquant` | **主服务容器名**（默认 `finvquant`；与既有容器冲突时可自定义，如 `finvquant-prod`） |
| `FINV_PG_CONTAINER_NAME` | `fq-postgres` | PostgreSQL 容器名 |
| `FINV_REDIS_CONTAINER_NAME` | `fq-redis` | Redis 容器名 |
| `TZ` | `Asia/Shanghai` | 时区 |

**Windows 路径注意事项：**
- `.env` 中的路径使用**正斜杠**或双反斜杠：`D:/Dev/Docker/HostFileSystem/FinvQuant/PostgreSQL`（不要写 `D:\...`）。
- 目标目录可不存在（Docker 会自动创建），但**父目录必须存在**。

## 6. 启动与验证

> 以下命令均在**仓库根目录**（含 `Deploy/` 的目录）执行；compose 文件位于 `Deploy/` 子目录，需用 `-f Deploy/docker-compose.yml` 显式指定（否则报 `no configuration file provided`）。
>
> 便捷方式：先 `cd Deploy` 再执行 `docker compose --env-file .env <命令>` 可省略 `-f`（此时 compose 项目目录为 `Deploy/`，未显式设置的相对路径卷 `./pgdata` 将落在 `Deploy/pgdata`）。
>
> **⚠️ 项目名/容器名说明**：compose 顶层 `name` 已固定项目名为 `finvquant`（默认，可用 `.env` 的 `FINV_PROJECT_NAME` 自定义），命名卷前缀为 `finvquant_`（如 `finvquant_finvquant-redisdata`），**不再带 `deploy` 前缀**；容器名由 `container_name` 固定（主服务默认 `finvquant`，见 `.env` 的 `FINV_CONTAINER_NAME`）。切换 `FINV_PROJECT_NAME` 会导致 Redis 命名卷不连续（旧数据在旧项目名前缀的卷中），需手动迁移，建议保持默认。

```powershell
docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env up -d
```

查看状态：

```powershell
docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env ps
```

**验证清单：**

| 验证项 | 地址/命令 | 预期 |
|--------|-----------|------|
| 前端控制台 | http://localhost:16002 | 页面可打开 |
| 服务端存活 | http://localhost:16001/API/V1/Health/Live | 返回 200 |
| 服务端就绪 | http://localhost:16001/API/V1/Health/Ready | 返回 200（含 PG/Redis 连通检查） |
| 服务端版本 | http://localhost:16001/API/V1/Version | 返回版本号 |
| 容器健康 | `docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env ps` | 三个服务 `healthy` |

> 首次启动 `finvquant` 会执行数据库迁移（创建 `schema_version` 表及业务表），约数秒；迁移完成前 `/Health/Ready` 可能不通过，稍等重试即可。

## 7. 数据持久化与备份

- **PostgreSQL**：数据绑定挂载到宿主目录 `${FINV_PG_DATA_DIR}`。⚠️ **PG 18 镜像挂载点为 `/var/lib/postgresql`（单一挂载）**，实际数据位于宿主目录下的 **`18/` 子目录**（`D:/Dev/Docker/HostFileSystem/FinvQuant/PostgreSQL/18/`）。备份/迁移该目录时请包含 `18/` 层级。
- **Redis**：数据在命名卷 `finvquant_finvquant-redisdata`（项目名 `finvquant` + 卷名 `finvquant-redisdata`，AOF），容器重建不丢；卷整体备份方式：`docker run --rm -v finvquant_finvquant-redisdata:/data -v D:/backup:/backup alpine tar czf /backup/redisdata.tar.gz -C /data .`

**PG 逻辑备份（推荐，最可靠）：**

```powershell
docker exec fq-postgres pg_dump -U finvquant -d finvquant -F c -f /tmp/finvquant_backup.dump
docker cp fq-postgres:/tmp/finvquant_backup.dump D:/backup/finvquant_20260805.dump
```

## 8. 常用运维命令

```powershell
docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env ps        # 查看状态
docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env logs -f finvquant   # 跟踪服务端日志
docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env restart finvquant    # 重启服务端
docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env down      # 停止并删除容器（数据卷/宿主数据目录保留）
docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env down -v   # 停止并删除容器+命名卷（⚠️ 慎用，Redis 数据会丢失）
```

## 9. 常见问题

- **端口被占用**：改 `.env` 中 `FINV_PG_EXPOSE_PORT` / `FINV_REDIS_EXPOSE_PORT`；`16001`/`16002` 被占需改 `docker-compose.yml` 中 `finvquant.ports` 左侧映射（如 `"26001:16001"`）。
- **PG 18 报 unused mount / 拒绝启动**：确认挂载的是 `/var/lib/postgresql` 而非 `/var/lib/postgresql/data`（当前编排已按 PG18 修正，勿回退）。
- **`finvquant` 容器反复重启**：先看日志 `docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env logs finvquant`；常见原因：PG/Redis 未就绪（`depends_on` 已含健康检查）、数据库迁移失败、`FINV_PG_PASSWORD` 与 postgres 服务不一致。
- **防火墙拦截访问**：Windows Defender 防火墙需放行 16001/16002 入站规则（仅本机访问可忽略）。
- **路径不识别**：确认 `.env` 使用正斜杠 `D:/...` 且父目录存在。

---

相关文档：[Win11 增量升级文档](Win11DockerUpgrade.md) · [docker-compose.yml](docker-compose.yml) · [.env.example](.env.example)
