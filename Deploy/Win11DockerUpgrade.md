# FinvQuant Windows 11 Docker 增量升级文档

> 所属：FinvQuant 部署 · 存放：`Deploy/Win11DockerUpgrade.md`
> 适用：已在 Windows 11 Docker 上部署 FinvQuant（见 [部署文档](Win11DockerDeploy.md)）后的**版本增量升级**。

## 1. 升级原理

- 镜像发布：CI（`.github/workflows/CI.yml`）在 push `dev`/`main`/`FinvQuant` 分支时构建并推送 `ghcr.io/acanx/finvquant:latest` 与 `v{VERSION}-YYYYMMDDHHMM`（如 `v0.1.0-202608051901`）；push `v*` tag 时推送 `v{VERSION}`（如 `v0.1.0`）。**全局版本单一来源：项目根 `VERSION` 文件**（后端 `-ldflags`、前端 `package.json`、镜像 tag 三处共用）。
- 多平台构建：镜像支持 `linux/amd64` + `linux/arm64`，每个平台另打专属 tag（如 `v0.1.0-202608051901-linux-amd64` / `...-linux-arm64`）；`latest` 为多平台聚合（按运行架构自动选择）。
- 升级方式：拉取新镜像 → 重建 `finvquant` 容器；`postgres` / `redis` 配置未变则**不重建、数据不丢**。
- 数据库迁移：新版服务端启动时**自动应用** `Deploy/Migrations/` 中未执行的迁移（幂等，按 `V<number>__<name>.sql` 版本号升序，已应用版本记录在 `schema_version` 表），**无需手动执行 SQL**。

## 2. 升级前准备

**2.1 确认当前版本**

```powershell
# 方式一：服务端版本接口
Invoke-WebRequest http://localhost:16001/API/V1/version
# 方式二：查看容器使用的镜像
docker inspect finvquant --format '{{.Image}}'
```

**2.2 备份数据（必须）**

```powershell
# PG 逻辑备份（推荐）
docker exec fq-postgres pg_dump -U finvquant -d finvquant -F c -f /tmp/finvquant_backup.dump
docker cp fq-postgres:/tmp/finvquant_backup.dump D:/backup/finvquant_20260805.dump

# 如需物理备份：停库后复制宿主目录（含 18/ 子目录，见部署文档第 7 节）
# docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env stop postgres
# 复制 %FINV_PG_DATA_DIR%\18 到备份位置后重启
```

**2.3 备份部署配置**

```powershell
Copy-Item Deploy\.env Deploy\.env.bak
```

## 3. 升级步骤

**3.1 获取最新编排与迁移脚本（如用 git 管理）**

```powershell
git pull   # 或 git fetch + 切换到目标版本 tag
```

> 迁移脚本随镜像内嵌，服务启动时自动应用；本地 `Deploy/Migrations/` 更新仅用于查阅核对。

**3.2 拉取新镜像**

> 以下命令在**仓库根目录**执行；compose 文件位于 `Deploy/` 子目录，需用 `-f Deploy/docker-compose.yml` 显式指定（否则报 `no configuration file provided`）。

```powershell
# 升级到最新版
docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env pull finvquant

# 或升级到指定版本：先修改 Deploy\.env 中 FINV_IMAGE_TAG（dev 构建 v0.1.0-202608051901，
#   main 发布 v0.1.0，即 VERSION 文件内容）
docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env pull finvquant
```

> **镜像 tag 规则**（CI 自动打，版本来自项目根 `VERSION` 文件）：分支推送（dev/main/FinvQuant）打 `latest` + `v{VERSION}-YYYYMMDDHHMM`（Asia/Shanghai，如 `v0.1.0-202608051901`）；推送 `v*` git tag（main 发布）打 `latest` + `v{VERSION}`（git tag 名，如 `v0.1.0`）。部署指定版本时把 `FINV_IMAGE_TAG` 改为对应 tag 即可（`docker images ghcr.io/acanx/finvquant` 可查看已拉取 tag）。

**3.3 重建服务端容器**

```powershell
docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env up -d finvquant
```

- `finvquant` 镜像变化时重建；`postgres` / `redis` 无变化则保持运行，**数据不丢失**。
- 若同时更新了 `docker-compose.yml` / `.env` 中 PG/Redis 配置，可整体执行 `docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env up -d`（仍只会重建配置发生变化的服务）。

**3.4 验证升级结果**

```powershell
docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env ps     # 三个服务 healthy
docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env logs --tail=50 finvquant   # 确认迁移日志（Apply migrations ...）与启动成功
```

| 验证项 | 地址/命令 | 预期 |
|--------|-----------|------|
| 服务端版本 | http://localhost:16001/API/V1/version | 返回新版本号 |
| 存活/就绪 | http://localhost:16001/API/V1/health/live 与 /ready | 200 |
| 前端 | http://localhost:16002 | 可打开 |
| 迁移记录 | `docker exec fq-postgres psql -U finvquant -d finvquant -c "SELECT version FROM schema_version ORDER BY version;"` | 包含本次新增的版本号 |

> 升级后首次启动会自动执行数据库迁移（默认超时 30 秒），期间 `/health/ready` 短暂不通过属正常；若迁移失败，`finvquant` 会启动失败并退出，见 [6. 常见问题](#6-常见问题)。

## 4. 数据库自动迁移说明

- 迁移文件：`Deploy/Migrations/V<number>__<name>.sql`（表结构 V1~V99999，种子数据 V100000+）。
- 记录表：`schema_version`（`version` 主键 + `success` 标记）；已成功的版本不会重复执行，**支持重复启动（幂等）**。
- 迁移在**单事务**内执行（脚本含 `BEGIN/COMMIT`），失败自动回滚并导致服务启动失败。
- 新增迁移脚本只需放入 `Deploy/Migrations/`（随镜像发布），升级时服务启动即自动应用，无需手动 `psql`。

## 5. 回滚方案

```powershell
# 1. 修改 Deploy\.env 的 FINV_IMAGE_TAG 回旧版本（或 docker-compose.yml 指定旧镜像）
# 2. 拉取旧镜像并重建
docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env pull finvquant
docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env up -d finvquant
```

⚠️ **数据库迁移不可自动回滚**：升级时已执行的迁移（`schema_version` 已记录）在回滚到旧版后不会撤销。若回滚原因是迁移失败导致服务不可用，应先修复迁移脚本（发布修复版）而非直接回滚；若必须回滚且涉及破坏性变更，需从第 2 节的备份恢复数据库。

## 6. 常见问题

- **`finvquant` 启动失败，日志含 `迁移 V<n> 失败`**：迁移脚本在单事务中执行，失败已回滚；联系维护者修复脚本后发布新镜像，重新执行升级步骤即可（`schema_version` 未记录失败版本，可安全重试）。
- **`up -d` 后容器未更新**：确认 `.env` 的 `FINV_IMAGE_TAG` 与期望版本一致，且 `docker compose -f Deploy/docker-compose.yml --env-file Deploy/.env pull finvquant` 已拉到新镜像（`docker images ghcr.io/acanx/finvquant` 核对 IMAGE ID）。
- **升级后前端异常但 API 正常**：前端资源已内嵌进镜像，属版本不匹配所致，确认浏览器强刷（Ctrl+F5）且服务端版本确为新版。
- **升级中断**：`up -d` 可重复执行，操作幂等；数据持久化不受影响。

---

相关文档：[Win11 部署文档](Win11DockerDeploy.md) · [docker-compose.yml](docker-compose.yml) · [.env.example](.env.example) · [迁移目录](Migrations/)
