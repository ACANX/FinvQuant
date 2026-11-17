# Windows 11 本地部署教程（Docker 服务端 + 本地客户端）

> 对应 ISSUE #253：在 Windows 11 上通过 Docker（镜像容器）部署 VeritasQuant 服务端，
> 并在本地启动客户端连接服务端完成**模拟盘（PAPER）**与**券商仿真（SIMULATION）**实验。
>
> ⚠️ 实盘交易默认禁用；本教程不涉及实盘（LIVE）启用。

---

## 1. 环境要求

| 项目 | 要求 |
|------|------|
| 操作系统 | Windows 11（64 位，22H2 或更新；家庭版需支持 WSL2） |
| Docker Desktop | 4.x 或更新，启用 **WSL2 后端**（Settings → General → Use the WSL 2 based engine） |
| WSL2 | 内置 Linux 内核；`wsl --version` 可查（无则 `wsl --update`） |
| 内存 | 建议 ≥ 8 GB（Docker 引擎 + PostgreSQL + Redis + API 服务） |
| 磁盘 | 建议 ≥ 10 GB 可用空间（镜像 + 数据卷） |
| 网络 | 可拉取 Docker Hub 镜像（国内可配置镜像加速器） |
| Python | 仅客户端需要 ≥ 3.13（服务端在容器内运行，宿主无需 Python） |

验证 Docker 环境：

```powershell
docker --version
docker compose version
docker info
```

若 `docker info` 报错，先启动 Docker Desktop 并等待引擎就绪（托盘鲸鱼图标变绿）。

---

## 2. 获取代码

```powershell
git clone https://github.com/ACANX/VeritasQuant.git
cd VeritasQuant
```

（如已克隆，先拉取最新：`git pull`）

---

## 3. 部署物清单

| 文件 | 用途 |
|------|------|
| `Docker/Dockerfile` | 服务端应用镜像（多阶段构建，非 root 运行） |
| `Docker/docker-compose.deploy.yml` | 服务端编排：API + PostgreSQL + Redis（持久卷、健康检查） |
| `Docker/.env.deploy.example` | 环境变量模板（密码、端口、数据目录） |
| `Docker/ImageSourceGuide.md` | 镜像源使用指南（国内网络绕过 Docker Hub 拉取 PG/Redis 镜像） |
| `scripts/DeployServer.py` | 部署脚本（check/build/start/status/logs/stop） |

---

## 4. 依赖说明

### 4.1 服务端（容器内）

| 组件 | 镜像/来源 | 说明 |
|------|-----------|------|
| API 服务 | `ghcr.io/acanx/veritasquant:<tag>`（GitHub Packages 构建，默认 `latest`） | FastAPI + Uvicorn，默认 `0.0.0.0:18000`；也可本地构建（`veritasquant/server:local`） |
| PostgreSQL | `postgres:18-alpine` | 事实/投影持久化（订单、成交、账户、审计）；数据挂载点 `/var/lib/postgresql`（18+ 镜像结构，实际在宿主 `18\` 子目录） |
| Redis | `redis:8-alpine` | 跨进程事件分发（Redis Streams） |

> 服务端镜像由 GitHub Actions（CI `build-image` job）构建并发布到
> GitHub Container Registry（ghcr.io）。镜像 tag 规则：
> - **dev 推送**：`0.1.2-<UTC时间戳>`（如 `0.1.2-202608031852`，每次 dev 构建唯一可追溯）+ `latest`；
> - **main 推送**：`0.1.2`（版本 tag，来自 `VQ_VERSION`）+ `latest`；
> - **Git tag `V0.1.2`**：`V0.1.2` + `0.1.2` + `latest`；
> - **手动触发（workflow_dispatch）**：`0.1.2-<UTC时间戳>` + `latest`。
> Windows 部署直接拉取即可，无需本地构建。

### 4.2 客户端（Windows 本地）

客户端通过已安装的 Python 包运行（连接服务端 API）：

| 入口 | 用途 |
|------|------|
| `vq-api-server` | （通常只在容器内）API 服务 |
| `vq-run-backtest` | 回测 |
| `vq-run-paper-trading` | 模拟盘交易 |
| `vq-gui` / `vq-gui-client` | GUI 客户端 |
| `vq-import-market-data` / `vq-validate-market-data` | 数据导入与校验 |

> GUI 的账户/策略/回测页面依赖领域 API（服务端已接线）。账户列表来自
> `.env.deploy` 的 `VQ_ACCOUNTS`（逗号分隔，如 `VQ_ACCOUNTS=acc-paper-001,acc-paper-002`）；
> 策略/标的/基金目录当前为空，后续阶段接入。

### 4.3 管理数据库（本机 pg 客户端连接容器）

PostgreSQL 已映射到宿主回环地址，可用 pgAdmin / DBeaver / psql 等客户端连接管理：

| 参数 | 值 |
|------|-----|
| Host | `127.0.0.1`（localhost） |
| Port | `5432`（`VQ_POSTGRES_PORT` 可改） |
| Database | `veritasquant` |
| User | `veritasquant` |
| Password | `Docker\.env.deploy` 中的 `VQ_POSTGRES_PASSWORD` |

示例（psql）：

```powershell
psql -h 127.0.0.1 -p 5432 -U veritasquant -d veritasquant
```

> 默认仅绑定本机回环，不暴露到局域网；如需局域网访问，修改 compose 中
> `127.0.0.1:${VQ_POSTGRES_PORT:-5432}:5432` 为 `0.0.0.0` 并评估安全风险。

---

## 5. 详细部署步骤

### 步骤 1：配置环境变量

```powershell
Copy-Item Docker\.env.deploy.example Docker\.env.deploy
# 用编辑器打开 Docker\.env.deploy，设置：
#   VQ_POSTGRES_PASSWORD=你的强密码（至少 12 位）
#   VQ_ENVIRONMENT=PAPER        （或 SIMULATION）
#   VQ_API_PORT=18000         （宿主映射端口，12000 以后避开常用端口）
```

### 步骤 2：环境自检（无需 Docker 也可运行）

```powershell
python3 scripts/DeployServer.py check
```

该命令校验编排文件语法（`docker compose config --quiet`）。

### 步骤 3：拉取服务端镜像（默认方式，无需本地构建）

服务端镜像由 GitHub Actions 构建并发布到 ghcr.io。默认编排直接拉取 `latest`：

```powershell
# 拉取 ghcr.io/acanx/veritasquant:latest
python3 scripts/DeployServer.py start   # 首次启动会自动拉取镜像
```

如需固定版本，在 `Docker/.env.deploy` 中设置（tag 必须与 GitHub Packages 页面实际发布的 tag 完全一致，见 6.6）：

```
VQ_IMAGE_TAG=latest
```

手动拉取验证：

```powershell
docker pull ghcr.io/acanx/veritasquant:latest
```

> **可选：本地构建**（不使用 GitHub 镜像时）——取消 `Docker/docker-compose.deploy.yml`
> 中 `server.build` 块注释，并把 `image` 改回 `veritasquant/server:local`，然后：
>
> ```powershell
> python3 scripts/DeployServer.py build
> ```

### 步骤 4：启动服务端

```powershell
python3 scripts/DeployServer.py start
```

- `--wait` 会等待三个容器健康检查全部通过；
- 查看状态：`python3 scripts/DeployServer.py status`；
- 查看日志：`python3 scripts/DeployServer.py logs --service server`。

### 步骤 5：验证服务端

```powershell
# liveness：进程存活
curl http://localhost:18000/Health/Live

# readiness：就绪门禁
curl http://localhost:18000/Health/Ready

# 版本
curl http://localhost:18000/api/v1/version
```

预期：`/Health/Live` 返回 `{"code":0,"data":{"status":"ALIVE",...}}`。

### 步骤 6：安装本地客户端

```powershell
# 使用项目 venv（建议）或全局环境
python3 -m venv .venv-client
.\.venv-client\Scripts\Activate.ps1
python3 -m pip install -e .
```

客户端入口即可用（`vq-run-backtest --help` 验证）。

### 步骤 7：执行模拟/仿真实验

**回测（离线，不依赖服务端）：**

```powershell
vq-run-backtest --config .\你的回测配置.yml
```

**模拟盘实验（连接服务端）：**

```powershell
vq-run-paper-trading --config .\你的模拟盘配置.yml --api http://localhost:18000
```

**GUI 客户端（连接服务端）：**

```powershell
# 注意：--api-url 指定后端地址，--serve 才真正启动 GUI（缺省仅离线校验）
vq-gui --api-url http://localhost:18000 --serve
```

> 说明：具体实验参数以各 console script 的 `--help` 与
> [Docs/VeritasQuantDevelopmentPlan.md](Docs/VeritasQuantDevelopmentPlan.md) 为准；
> 模拟盘/仿真配置中 `execution_mode` 使用 `PAPER` / `SIMULATION`。

### 步骤 8：停止服务端

```powershell
python3 scripts/DeployServer.py stop
```

- 停止并删除容器与网络；
- **数据保留**：PostgreSQL 数据在宿主目录 `D:\Dev\Docker\HostFileSystem\VeritasQuant\PostgreSQL\`（PG18+ 实际在其 `18\` 子目录），Redis 数据在 `vq-redis` 命名卷，均不丢失；
- 如需彻底清理：`docker compose -f Docker\docker-compose.deploy.yml down --volumes`（仅删除命名卷；PostgreSQL 宿主目录需手动删除）。

---

## 6. 常见问题（FAQ）

### 6.1 Docker Desktop 启动失败 / WSL2 报错

- 确认 BIOS 开启虚拟化（任务管理器 → 性能 → CPU → 虚拟化：已启用）；
- `wsl --update` 更新内核；`wsl --status` 查看状态；
- Docker Desktop → Troubleshoot → Restart。

### 6.2 `VQ_POSTGRES_PASSWORD` 未设置

`docker compose` 会以 `VQ_POSTGRES_PASSWORD` 为必填变量，未设置时启动失败。
在 `Docker/.env.deploy` 中填入密码后重试。

### 6.3 端口 18000 被占用

- 修改 `Docker/.env.deploy` 中 `VQ_API_PORT=18001` 后重启；
- 或释放占用：`netstat -ano | findstr :18000` → `taskkill /PID <pid> /F`。

### 6.4 镜像拉取慢 / 超时

- 国内网络访问 Docker Hub 不稳定（常见报错 `Get "https://auth.docker.io/token?...": EOF`），
  详见独立指南 [Docker/ImageSourceGuide.md](ImageSourceGuide.md)：
  - 命令行临时指定镜像源（`docker pull <源>/library/<镜像>:<tag>` + `docker tag`）；
  - 或 Docker Desktop → Settings → Docker Engine 配置 `registry-mirrors` 后 `Apply & Restart`。

### 6.5 拉取 ghcr.io 镜像失败（denied / unauthorized）

- 确认 `Docker/.env.deploy` 中 `VQ_IMAGE_OWNER` 与镜像实际命名空间一致（默认 `acanx`）；
- 私有镜像需先登录：`echo $env:GITHUB_TOKEN | docker login ghcr.io -u <用户名> --password-stdin`（公开镜像无需登录）；
- 确认镜像已发布：在 GitHub 仓库 Packages 页面查看 `ghcr.io/acanx/veritasquant` 是否存在对应 tag。

### 6.6 拉取到不存在的版本 tag

- 检查 `VQ_IMAGE_TAG` 拼写：必须与 GitHub Packages 页面实际发布的 tag 完全一致（Git tag `V0.1.2` 触发构建时发布的镜像 tag 为 `0.1.2` 与 `latest`，镜像 tag 不带 `V` 前缀）；
- 查看已发布 tag：`docker manifest inspect ghcr.io/acanx/veritasquant:latest` 或 GitHub Packages 页面（`https://github.com/users/ACANX/packages/container/package/veritasquant`）。

### 6.7 容器启动后立即退出

```powershell
python3 scripts/DeployServer.py logs --service server
```

常见原因：端口冲突、`read_only: true` 下写入只读路径。确认 `.env.deploy` 中
`VQ_DATA_DIR` 指向宿主可写目录。

### 6.8 客户端连不上服务端

- 确认容器健康：`python3 scripts/DeployServer.py status`；
- 确认宿主端口：`curl http://localhost:18000/Health/Live`；
- 客户端配置中 API 地址使用 `http://localhost:18000`（勿用 `127.0.0.1` 时混用 IPv6）。

### 6.9 数据持久化

- PostgreSQL 数据映射宿主目录 `D:\Dev\Docker\HostFileSystem\VeritasQuant\PostgreSQL\`（`VQ_POSTGRES_DATA_DIR` 可覆盖），PG18+ 实际存放于其 `18\` 子目录，容器删除不丢数据；
- Redis 使用命名卷 `vq-redis`，`stop` 不删数据；
- 运行产物在 `vq-runtime` 卷与宿主 `VQ_DATA_DIR` 目录；
- ⚠️ PostgreSQL 大版本升级（如 16 → 18）数据目录格式不兼容：先 `pg_dump` 备份再迁移，或确认无重要数据后删除旧卷/旧目录。

### 6.10 客户端 pip 安装失败（No matching distribution found for setuptools）

症状：`python3 -m pip install -e .` 报 `Could not find a version that satisfies the requirement setuptools>=69 (from versions: none)`。

常见原因：**系统代理拦截了 PyPI 请求**（构建隔离阶段 pip 需要从镜像源拉取 setuptools，请求被代理吃掉后返回空列表）。

解决（按顺序尝试）：

```powershell
# ① 关掉系统代理 / VPN 后重试
python3 -m pip install -e .

# ② 或换源安装（阿里云镜像国内更快）
python3 -m pip install -e . -i https://mirrors.aliyun.com/pypi/simple/

# ③ 永久换源
python3 -m pip config set global.index-url https://mirrors.aliyun.com/pypi/simple/

# ④ 若代理必须开启，检查环境变量代理是否指向失效地址并修正
Remove-Item Env:HTTP_PROXY, Env:HTTPS_PROXY   # 临时清除后再试
```

### 6.11 PowerShell 中 curl 显示中文乱码 / curl 命令报错

PowerShell 5.1 中 `curl` 是 `Invoke-WebRequest` 的别名（不是真正的 curl）：

- 中文乱码：`Invoke-WebRequest` 按系统代码页（GBK）解码 UTF-8 JSON 导致；
- `curl --version` / `curl -v` 报错：别名把参数当 PowerShell 参数解析。

解决：使用真正的 curl：

```powershell
curl.exe -s http://localhost:18000/api/v1/version
```

或先切换控制台代码页为 UTF-8：`chcp 65001`。

### 6.12 PostgreSQL 18 数据目录结构变化（unused mount 拒绝启动）

PG18+ 官方镜像改变了数据目录结构，与 16/17 不兼容：

- 数据不再存放于 `/var/lib/postgresql/data`，而是 `/var/lib/postgresql/<大版本>/`（如 `18/`）；
- compose 挂载点必须为 `/var/lib/postgresql`（单一挂载）；挂到 `/var/lib/postgresql/data` 会被镜像判定为 unused mount 并**拒绝启动**（docker-library/postgres#1259），表现为容器启动约 1s 即退出、`vq-postgresql is unhealthy`；
- 宿主数据实际落在 `<VQ_POSTGRES_DATA_DIR>/18/` 子目录（如 `D:\...\PostgreSQL\18\`）；
- 该结构便于 `pg_upgrade --link` 跨版本升级（如 18 → 19）；
- 从 PG16/17 迁移：旧数据格式不兼容，先 `pg_dump` 备份再 `pg_restore` 恢复（见 7.2 场景 A）。

---

## 7. 版本升级与数据运维（SOP）

> 总原则：**先备份、再升级；升级可回滚；变更留痕**（发布、备份、回滚操作建议登记 Change，见 [开发工作流](Docs/VeritasQuantDevelopmentWorkflow.md)）。

### 7.1 数据备份（Backup）

**PostgreSQL（推荐 `pg_dump` 一致性逻辑备份，发布前必做）：**

```powershell
# ① 全量逻辑备份（custom 格式，含压缩）
docker exec vq-postgresql pg_dump -U veritasquant -d veritasquant -Fc -f /tmp/vq_backup.dump
# ② 拷贝到宿主备份目录（先创建目录）
New-Item -ItemType Directory -Force -Path D:\Dev\Docker\HostFileSystem\VeritasQuant\Backup | Out-Null
docker cp vq-postgresql:/tmp/vq_backup.dump D:\Dev\Docker\HostFileSystem\VeritasQuant\Backup\vq_$(Get-Date -Format yyyyMMdd_HHmmss).dump
```

> ⚠️ 必须通过 `pg_dump` 做一致性逻辑备份；**不要直接复制数据目录文件**（PG 运行中文件级复制不一致）。

**Redis（AOF 已开启，发布前导出快照）：**

```powershell
docker exec vq-redis redis-cli --rdb /tmp/vq_redis.rdb
docker cp vq-redis:/tmp/vq_redis.rdb D:\Dev\Docker\HostFileSystem\VeritasQuant\Backup\vq_redis_$(Get-Date -Format yyyyMMdd_HHmmss).rdb
```

**运行产物与导入数据：**

```powershell
# vq-runtime 卷（回测输出/报告/日志）
docker run --rm -v vq-runtime:/data -v D:/Dev/Docker/HostFileSystem/VeritasQuant/Backup:/backup alpine sh -c "cp -a /data/. /backup/runtime_$(Get-Date -Format yyyyMMdd)/"
# VQ_DATA_DIR 宿主目录直接复制即可
Copy-Item -Recurse .\data D:\Dev\Docker\HostFileSystem\VeritasQuant\Backup\data_$(Get-Date -Format yyyyMMdd)
```

**建议频率**：每日定时备份 + 每次版本发布前必做。

### 7.2 数据迁移（Migration）

**场景 A：PostgreSQL 大版本升级（如 16 → 18，数据目录格式不兼容）**

```powershell
# ① 先按 7.1 备份
# ② 停止服务
python3 scripts/DeployServer.py stop
# ③ 升级编排中的镜像版本（如 postgres:18-alpine），并确认宿主数据目录为空
#    （旧版本数据文件与新版本不兼容，PG 会拒绝启动，需先清空/移走旧目录；
#     PG18+ 挂载点为 /var/lib/postgresql，数据实际在 <目录>/18/ 子目录）
# ④ 启动（新数据目录自动初始化）
python3 scripts/DeployServer.py start
# ⑤ 恢复数据
Copy-Item D:\Dev\Docker\HostFileSystem\VeritasQuant\Backup\vq_20260804.dump .\restore.dump
docker cp .\restore.dump vq-postgresql:/tmp/restore.dump
docker exec vq-postgresql pg_restore -U veritasquant -d veritasquant --clean --if-exists /tmp/restore.dump
Remove-Item .\restore.dump
```

**场景 B：应用 Schema 迁移**

服务端启动时自动执行版本化数据库迁移（V1/V2/V3），升级应用镜像**无需手动迁移**；迁移前仍建议先备份。

**场景 C：PostgreSQL 数据目录更换位置**

```powershell
python3 scripts/DeployServer.py stop   # 停止后文件级复制才安全
Copy-Item -Recurse D:\Dev\Docker\HostFileSystem\VeritasQuant\PostgreSQL D:\Dev\Docker\HostFileSystem\VeritasQuant\PostgreSQLNew   # 含 18/ 子目录一并复制
# 修改 .env.deploy 的 VQ_POSTGRES_DATA_DIR 指向新目录
python3 scripts/DeployServer.py start
```

### 7.3 关停（Shutdown）

- 正常关停（保留全部数据）：`python3 scripts/DeployServer.py stop`；
- 彻底清理（删除命名卷 `vq-runtime` / `vq-redis`；PostgreSQL 宿主目录需手动删除）：
  `docker compose -f Docker\docker-compose.deploy.yml down --volumes`；
- 升级前关停：直接 `stop` 即可，数据均在宿主目录/命名卷中，不会丢失。

### 7.4 升级到新版本（Upgrade）

**标准流程（7 步）：**

```powershell
# ① 记录当前版本
curl.exe http://localhost:18000/api/v1/version

# ② 按 7.1 备份（PG + Redis + 运行产物）

# ③ 确认新版本镜像已发布（GitHub Packages 页面或 manifest 检查）
docker manifest inspect ghcr.io/acanx/veritasquant:0.1.3

# ④ 编辑 Docker\.env.deploy，将 VQ_IMAGE_TAG 固定为新版本（不建议长期用 latest）
#    VQ_IMAGE_TAG=0.1.3

# ⑤ 重新部署（compose 检测镜像变化自动重建 server 容器）
python3 scripts/DeployServer.py start

# ⑥ 验证
curl.exe http://localhost:18000/Health/Live
curl.exe http://localhost:18000/Health/Ready
curl.exe http://localhost:18000/api/v1/version   # 应显示新版本号

# ⑦ 验收通过后可选清理旧版本镜像
docker image rm ghcr.io/acanx/veritasquant:0.1.2
```

**回滚：**

```powershell
# ① .env.deploy 的 VQ_IMAGE_TAG 改回旧版本（如 0.1.2）
# ② python3 scripts/DeployServer.py start
# ③ 若 Schema 已变更导致应用不兼容，按 7.2 场景 A 恢复数据库备份
```

**镜像 tag 语义**：dev 推送生成 `0.1.2-<UTC时间戳>`（每次构建唯一）+ `latest`；
main 推送生成 `0.1.2`（版本 tag）+ `latest`；Git tag `V0.1.2` 额外发布 `V0.1.2` 与去 V 版本 tag；
手动触发（workflow_dispatch）生成 `0.1.2-<UTC时间戳>` + `latest`。不用 `sha-*` 作为镜像 tag。

### 7.5 升级检查清单

| 步骤 | 操作 | 命令/位置 |
| --- | --- | --- |
| 1 | 备份 PostgreSQL | `pg_dump` + `docker cp`（7.1） |
| 2 | 备份 Redis | `redis-cli --rdb` + `docker cp`（7.1） |
| 3 | 记录当前版本 | `/api/v1/version` |
| 4 | 确认新版本镜像存在 | Packages 页面 / `docker manifest inspect` |
| 5 | 固定版本 tag | `.env.deploy` 的 `VQ_IMAGE_TAG` |
| 6 | 重新部署 | `python3 scripts/DeployServer.py start` |
| 7 | 验证 live/ready/version | `/Health/Live`、`/Health/Ready`、`/api/v1/version` |
| 8 | 回滚预案确认 | 旧版本镜像仍可拉取 / 备份文件完好 |

---

## 8. 安全与边界

- 本编排仅用于本地模拟盘/仿真；
- PostgreSQL 默认仅绑定宿主回环 `127.0.0.1:5432`（供本机 pg 客户端管理），不暴露到局域网；如需局域网访问，将映射改为 `0.0.0.0` 并评估风险；
- `.env.deploy` 含明文密码，已被 `.gitignore` 忽略，不要提交；
- 实盘（LIVE）默认禁用；任何实盘启用必须先走 Change 流程并满足
  [TechSpec 13 阶段 5 gate](Docs/VeritasQuantTechSpec.md)。
