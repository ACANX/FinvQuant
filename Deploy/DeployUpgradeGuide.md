# FinvQuant Windows 本地部署/升级/回滚 一键脚本使用手册

> 所属：FinvQuant 部署 · 存放：`Deploy/DeployUpgradeGuide.md`
> 适用：Windows 11 本地通过 Docker 部署、日常升级、异常回滚 FinvQuant（All-in-One）。
> 配套脚本：`Deploy/deploy.cmd`（一键部署）、`Deploy/upgrade.cmd`（版本检查+按需升级）、`Deploy/rollback.cmd`（回滚）。
> 详细部署原理见 [Win11DockerDeploy.md](Win11DockerDeploy.md)；升级原理见 [Win11DockerUpgrade.md](Win11DockerUpgrade.md)。

## 0. 三个脚本分工速览

| 脚本 | 场景 | 一句话 |
|------|------|--------|
| `Deploy\deploy.cmd` | **首次部署** / 环境损坏后重装 | 生成 `.env` → 拉镜像 → 启动全部服务 → 验证 |
| `Deploy\upgrade.cmd` | **日常升级**（本地常用） | 拉远端镜像 → **对比本地版本** → 有更新才升级 → 验证 |
| `Deploy\rollback.cmd` | 升级异常需回退 | 回滚到 `.env` 指定版本镜像 → 重建 |

> ⚠️ **所有脚本都要求：在仓库根目录（`D:\Code\PyCode\FinvQuant`）运行**（脚本会自动 `pushd`，但从根目录运行最稳妥）。

---

## 1. 一键部署 deploy.cmd

### 1.1 首次部署流程

```powershell
cd D:\Code\PyCode\FinvQuant
Deploy\deploy.cmd
```

脚本自动完成：
1. **前置检查**：Docker 是否运行。
2. **生成 / 核对 `.env`（关键环节，详见 §1.2）**。
3. **拉取镜像**：`ghcr.io/acanx/finvquant:latest`。
4. **启动全部服务**：`postgres` + `redis` + `finvquant`（Compose 首次创建）。
5. **等待健康 + 验证**：版本接口 / 存活探针 / 容器状态 / 数据库迁移记录。

### 1.2 .env 环境变量文件生成（重要，勿跳过）

- **为什么要有这一步**：`Deploy/.env` 决定数据库账号、端口映射、数据目录、镜像 tag 等全部关键参数，且被 `.gitignore` 忽略（**不会随代码提交**）。新克隆/新机器上不存在，必须先生成，否则 `docker compose` 报 `env file not found`。
- **脚本行为**：
  - 若 `Deploy\.env` **不存在** → 从 `Deploy\.env.example` 自动复制生成，并在控制台打印关键配置项供核对。
  - 若 `Deploy\.env` **已存在** → 沿用现有配置，仅打印关键项供核对。
- **生成的 `.env` 需重点核对的项**：

| 变量 | 含义 | 注意 |
|------|------|------|
| `FINV_PG_USER` / `FINV_PG_PASSWORD` | 数据库账号 / 密码 | 首次创建后**改密码需同步改数据目录**，否则旧数据连不上 |
| `FINV_PG_EXPOSE_PORT` | PG 宿主映射端口 | 默认 5432，被占用可改 |
| `FINV_PG_DATA_DIR` | **PG 数据目录（宿主机）** | **升级/重装不丢数据的核心**；Windows 示例 `D:\Dev\Docker\HostFileSystem\FinvQuant\PostgreSQL` |
| `FINV_REDIS_EXPOSE_PORT` | Redis 宿主映射端口 | 默认 6380 |
| `FINV_IMAGE_TAG` | 镜像 tag | 默认 `latest`；回滚到指定版本时改为如 `v0.1.0` |
| `FINV_CONTAINER_NAME` / `FINV_PROJECT_NAME` | 容器 / 项目名 | 影响命名卷前缀，改前注意数据连续性 |

- **数据持久化关键点**（重装/升级不丢数据的保证）：
  - **PostgreSQL**：数据绑定挂载到宿主机 `FINV_PG_DATA_DIR`（含 `18/` 子目录）。**换机器/改目录 = 数据在新位置重新初始化**。
  - **Redis**：数据在命名卷 `finvquant_finvquant-redisdata`（Compose 管理，容器重建不丢）。

---

## 2. 日常升级 upgrade.cmd

### 2.1 升级前说明（重要）

- **镜像来源**：`ghcr.io/acanx/finvquant`（GHCR）。**代码改动需先提交并 push 到 `dev`/`main`/`FinvQuant`/`feat/backtest` 分支，CI 自动构建并推送最新镜像到 GHCR**；本脚本**不负责构建**。
- **版本对比逻辑**：脚本拉取远端镜像后，将远端镜像 ID 与**当前运行容器**的镜像 ID 对比：
  - 相同 → 已是最新，打印结果并退出（不执行任何破坏性操作）。
  - 不同 → 远端有更新，执行升级部署。
  - `--force` → 强制重建（即使版本相同）。

### 2.2 用法

```powershell
cd D:\Code\PyCode\FinvQuant

Deploy\upgrade.cmd                  # 标准升级：拉取→对比→按需升级
Deploy\upgrade.cmd --skip-backup    # 跳过数据库备份（应急；正常会先 pg_dump）
Deploy\upgrade.cmd --force          # 强制重建（版本相同也重建）
```

### 2.3 脚本执行的 8 个步骤（控制台会逐步打印）

| 步骤 | 动作 | 输出关键信息 |
|------|------|-------------|
| 1 | 前置检查 | Docker 可用性、`.env` 存在、finvquant 容器在运行 |
| 2 | 读取本地运行版本 | 本地容器镜像 ID、目标 tag、postgres/redis 状态 |
| 3 | 拉取远端镜像 | 远端最新镜像 ID（幂等） |
| 4 | **版本对比** | 本地 vs 远端 ID，判断"已最新 / 需升级" |
| 5 | 升级前备份 | `.env.bak` + pg_dump 到 `Deploy\backup\`（可跳过） |
| 6 | 升级部署 | 仅重建 `finvquant`，打印"存量 DB/Redis/配置"保留说明 |
| 7 | 等待健康 | 健康检查（最长 90s） |
| 8 | 验证 | 版本接口 / 存活探针 / 容器状态 / 迁移记录 |

### 2.4 升级对存量数据的影响（脚本会明示）

- **只重建 `finvquant` 服务**：`postgres` / `redis` 配置未变 → **不重建、数据不丢**。
- **PostgreSQL**：数据在 `FINV_PG_DATA_DIR` 绑定挂载，升级全程保留；新增表结构/种子由服务端启动时**自动应用迁移**（幂等，`schema_version` 表记录）。
- **Redis**：命名卷数据保留。
- **配置**：沿用 `Deploy\.env`（脚本不修改）；`.env` 有变更时脚本提示先核对。

---

## 3. 异常回滚 rollback.cmd

```powershell
cd D:\Code\PyCode\FinvQuant
# 先修改 Deploy\.env 的 FINV_IMAGE_TAG 为要回滚的版本（如 v0.1.0），然后：
Deploy\rollback.cmd
```

- 脚本：拉取目标 tag 镜像 → 重建 `finvquant` 容器（DB/Redis 保留）。
- ⚠️ **数据库迁移不可自动回滚**：升级时已执行的迁移在回滚后不会撤销。若回滚原因是迁移失败，应先修复迁移脚本发布新镜像，而非直接回滚（详见 [Win11DockerUpgrade.md](Win11DockerUpgrade.md) 第 5 节）。

---

## 4. 常见问题

| 问题 | 处理 |
|------|------|
| 提示"缺少 Deploy\.env" | 运行 `Deploy\deploy.cmd` 先生成（§1.2） |
| 提示"未检测到运行中的 finvquant 容器" | 首次部署用 `deploy.cmd`；服务停了先 `Deploy\deploy.cmd --no-env` |
| 升级后版本不变 | 确认已 push 代码并等 CI 构建完成；`docker images ghcr.io/acanx/finvquant` 看远端 digest |
| 拉取 GHCR 超时 | 检查 ghcr.io 网络；`docker login ghcr.io` 登录 |
| 镜像加速源无法访问 docker.io | GHCR 直连不受镜像源影响；本地构建依赖 docker.io 时改用 `upgrade.cmd`（拉 GHCR） |
| 前端异常但 API 正常 | 强刷浏览器（Ctrl+F5）；确认服务端确为新版本 |

---

相关文档：[Win11DockerDeploy.md](Win11DockerDeploy.md)（部署）· [Win11DockerUpgrade.md](Win11DockerUpgrade.md)（升级）· [docker-compose.yml](docker-compose.yml) · [.env.example](.env.example)
