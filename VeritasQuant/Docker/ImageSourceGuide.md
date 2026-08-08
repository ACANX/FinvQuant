# Docker 镜像源使用指南（绕过 Docker Hub）

> 适用场景：国内网络下 Docker 拉取 Docker Hub（docker.io）镜像失败或极慢，
> 例如部署本项目的 PostgreSQL / Redis 时出现：
>
> ```
> Error response from daemon: Head "https://registry-1.docker.io/v2/library/postgres/manifests/18-alpine":
> Get "https://auth.docker.io/token?scope=...": EOF
> ```
>
> 该错误是访问 Docker Hub 认证服务（auth.docker.io）时连接被断开——网络不可达的典型表现。
> **仅影响 Docker Hub 上的镜像**（postgres、redis 等）；本项目 server 镜像在 ghcr.io，通常不受影响。

## 1. 手动拉取本项目 server 镜像（ghcr.io）

ghcr.io 不走 Docker Hub 加速器，环境可直接访问，无需镜像源配置：

```powershell
# 拉取最新版
docker pull ghcr.io/acanx/veritasquant:latest

# 拉取指定版本（镜像 tag 不带 V 前缀：Git tag V0.1.2 对应镜像 tag 0.1.2）
docker pull ghcr.io/acanx/veritasquant:0.1.2
```

查看已发布 tag：GitHub Packages 页面（`https://github.com/users/ACANX/packages/container/package/veritasquant`）
或 `docker manifest inspect ghcr.io/acanx/veritasquant:latest`。

> 拉取失败排查：`denied / unauthorized` → 镜像非 public 或需登录
> （`echo $env:GITHUB_TOKEN | docker login ghcr.io -u <用户名> --password-stdin`）；
> `manifest unknown` → tag 不存在（核对 Packages 页面实际 tag）。

## 2. 方案 A：命令行临时指定镜像源（单次，推荐先试这个）

Docker Hub 官方镜像的完整名称是 `library/<镜像>`（如 `library/postgres`、`library/redis`）。
默认不写源地址时 Docker 走 Docker Hub；**加上源地址前缀即改为从该源拉取**：

```powershell
# ① 从镜像源拉取（任一可达的源，见第 3 节清单）
docker pull docker.m.daocloud.io/library/postgres:18-alpine
docker pull docker.m.daocloud.io/library/redis:8-alpine

# ② 重新打 tag 为 compose 使用的官方名
docker tag docker.m.daocloud.io/library/postgres:18-alpine postgres:18-alpine
docker tag docker.m.daocloud.io/library/redis:8-alpine redis:8-alpine
```

打 tag 后本机已有 `postgres:18-alpine` / `redis:8-alpine`，`docker compose` 启动时
**发现本地已有同名镜像就不会再去远程拉取**，可直接部署：

```powershell
python3 scripts/DeployServer.py start
```

**通用写法**（适用于任何 Docker Hub 镜像）：

```powershell
docker pull <源地址>/library/<镜像名>:<tag>    # Docker Hub 官方镜像
docker tag <源地址>/library/<镜像名>:<tag> <镜像名>:<tag>
```

第三方镜像（如 `bitnami/postgresql`）用 `<源地址>/bitnami/postgresql:<tag>` 的格式。

## 3. 方案 B：Docker Desktop 永久配置镜像加速器（一劳永逸）

1. Docker Desktop → **Settings → Docker Engine**；
2. 在 JSON 中加 `registry-mirrors`（保留原有内容）：

```json
{
  "registry-mirrors": [
    "https://docker.m.daocloud.io",
    "https://docker.1ms.run",
    "https://dockerproxy.net"
  ]
}
```

3. **Apply & Restart** 重启 Docker；
4. 之后 `docker pull postgres:18-alpine` 等命令自动走镜像源，无需手动 tag。

## 4. 镜像源地址清单

| 源 | 地址 | 说明 |
| --- | --- | --- |
| DaoCloud | `docker.m.daocloud.io` | 本指南实测可用 |
| 1ms | `docker.1ms.run` | 备选 |
| dockerproxy | `dockerproxy.net` | 备选 |
| 阿里云专属 | `https://<你的ID>.mirror.aliyuncs.com` | 阿里云容器镜像服务控制台领取，相对更稳定 |

> 第三方公共源可能随时失效；失效时依次换用备选，或自行搜索当前可用源。
> 企业/办公网络建议优先使用公司内部 registry 或为 Docker 配置 HTTP 代理
> （Settings → Resources → Proxies），更稳定合规。

## 5. 验证

```powershell
# 确认本地镜像已就绪
docker images | Select-String "postgres|redis"

# 配置方案 B 后验证拉取不再走 Docker Hub
docker pull postgres:18-alpine

# 部署
python3 scripts/DeployServer.py start
curl.exe http://localhost:18000/Health/Live
```

## 6. 注意事项

- 镜像加速器只代理 Docker Hub；ghcr.io（本项目 server 镜像）不走加速器，环境能直接访问；
- 方案 A 不修改任何配置，只影响本机当前 tag；方案 B 写入 Docker Engine 全局配置；
- 若仍报 `manifest unknown`：请核对 tag 是否存在（`docker manifest inspect <镜像>:<tag>`），
  与网络无关（见 Windows11Deployment.md FAQ 6.6）；
- PostgreSQL 大版本升级（如 16 → 18）后旧数据目录不兼容，升级前先按
  [Windows11Deployment.md](Windows11Deployment.md) 第 7 章 SOP 备份迁移。
