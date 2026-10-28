# Native 安装手册（宿主直装 + systemd）

> 适用：Debian 12+ / Ubuntu 22.04+（systemd），root 权限。
> 配套文件：`Install.sh`（一键安装）、`FinvQuant.service`（systemd unit 模板）、`FinvQuant.env.example`（环境变量）、`Nginx.conf.example`（反代示例）。
> **采用本方式前请先阅读 [`README.md`](README.md) 的「适用场景 / 不适用场景」与自检清单。**
> 状态：脚本已提供，但**尚未在目标 VPS 上做过完整实机验证**；首次部署建议先在测试机完整走一遍。

## 0. 安装后会得到什么

除非用 `--prefix` 另行指定，所有内容都装在**同一个安装根目录 `/data/App/FinvQuant/`** 下：

| 项目 | 路径 / 说明 |
|---|---|
| 安装根目录 | `/data/App/FinvQuant/`（`--prefix <dir>` 或 `FINVQ_PREFIX` 可改） |
| 二进制 | `/data/App/FinvQuant/bin/finvquant`（软链 `/usr/local/bin/finvquant`） |
| 配置 | `/data/App/FinvQuant/etc/finvquant.env`（权限 600，含随机生成的数据库密码） |
| 迁移脚本 | `/data/App/FinvQuant/Deploy/Migrations/`（与仓库结构一致，服务启动时自动应用未执行版本） |
| 部署脚本与文件 | `/data/App/FinvQuant/Deploy/`（`Install.sh`/`Upgrade.sh`/`Uninstall.sh`、`finvquant.service`、env 与 Nginx 模板、本手册） |
| 备份 | `/data/App/FinvQuant/Backup/`（升级脚本写入 `finvquant_YYYYMMDD_HHMMSS.dump`） |
| 导出 | `/data/App/FinvQuant/Export/`（行情归档导出，与容器方式一致） |
| systemd unit | `/etc/systemd/system/finvquant.service`（由 `Deploy/FinvQuant.service` 渲染，两处内容一致） |
| 日志 | journald：`journalctl -u finvquant` |
| 数据 | PostgreSQL 18 默认数据目录（不随本应用管理） |

目录权限（安装脚本自动设置）：

| 目录 | 属主 / 权限 |
|---|---|
| `bin/` | `root:root 0755` |
| `Deploy/Migrations/` | `root:finvquant 0755` |
| `etc/` | `root:finvquant 0750`，其中 `finvquant.env` 为 `root:root 0600` |
| `Deploy/` | `root:root 0755` |
| `Backup/` | `root:finvquant 0750` |
| `Export/` | `finvquant:finvquant 0755` |

## 1. 前置条件

| 项 | 要求 |
|---|---|
| 系统 | Debian 12+ / Ubuntu 22.04+，systemd，x86_64 或 arm64 |
| 权限 | root（`sudo`） |
| 内存 | ≥ 2 GB（PostgreSQL + Go 服务 + Redis） |
| 磁盘 | ≥ 10 GB（行情数据量取决于导入规模）；`/data` 需有足够空间（程序+备份+导出都在此） |
| 网络 | 能访问 PostgreSQL PGDG 源与发行版软件源 |
| 域名/证书 | 可选；生产建议配 Nginx + TLS |

## 2. 准备二进制与迁移脚本

### 2.1 一键下载（推荐，无需 Go/Node 环境）

直接在目标主机上按本机 OS/架构下载官方发行包并解压到安装根目录：

```bash
# 默认取最新版本（含快照），解压到 /data/App/FinvQuant
sudo bash Deploy/Script/Download.sh

# 只取正式版（main 合并后的 v0.1.3 形式）
sudo bash Deploy/Script/Download.sh -c release

# 指定版本 / 只下载不解压 / 看会下什么
sudo bash Deploy/Script/Download.sh -v v0.1.3.00357.2610281256
sudo bash Deploy/Script/Download.sh --no-extract -o ./Dist
bash Deploy/Script/Download.sh --dry-run
```

脚本会自动：识别 Linux/Windows 与 amd64/arm64 → 选对应资产 → 下载并校验（资产 `.sha256` + 包内 `SHA256SUMS`）→ 安全解压到目标目录（非空目录需 `--force`）。
包内已含 `finvquant`、`Deploy/Migrations`、`Deploy/Native` 全套脚本，可直接进入第 3 节安装。
详细参数见 [`../Script/README.md`](../Script/README.md) §13。

### 2.2 从源码构建（开发/自定义版本）

`Install.sh` 会在以下位置查找 `finvquant` 与迁移目录（按顺序）：

1. 发行包/安装后布局（迁移与仓库结构一致放在 `Deploy/` 下）：
   - 二进制：`<dir>/finvquant`
   - 迁移：`<dir>/Deploy/Migrations/`
2. 源码仓库（脚本位于 `<repo>/Deploy/Native/` 时自动识别）：
   - 二进制：`<repo>/finvquant`
   - 迁移：`<repo>/Deploy/Migrations/`
3. 其它：Native 脚本的上一级/同级 `Migrations/`
4. `--binary /path/to/finvquant` 显式指定二进制

> 若迁移目录就是安装后的 `<PREFIX>/Deploy/Migrations`（下载包解压到同一目录的情形），安装脚本会跳过复制，避免“源与目标相同”。

构建二进制（在仓库根）：

```bash
# 前端产物需先构建并复制到 embed 位置
cd Web && npm ci && npm run build
mkdir -p ../internal/webui/dist && cp -r dist/* ../internal/webui/dist/
cd ..
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=$(cat VERSION) -X main.commit=$(git rev-parse --short HEAD)" \
  -o finvquant ./cmd/server
```

## 3. 一键安装

```bash
# 上传整个仓库（或至少 Deploy/ + finvquant 二进制）到服务器，例如 /root/finvquant-src
cd /root/finvquant-src
sudo bash Deploy/Native/Install.sh

# 需要装到其它目录时（必须是绝对路径）
sudo bash Deploy/Native/Install.sh --prefix /data/App/FinvQuant
```

常用参数：

| 参数 | 说明 |
|---|---|
| `--skip-db` | 已有 PostgreSQL 18 且已建库，跳过安装与建库 |
| `--skip-redis` | 已有 Redis，跳过安装 |
| `--binary <path>` | 指定二进制路径 |
| `--prefix <dir>` | 指定安装根目录（默认 `/data/App/FinvQuant`，也可用环境变量 `FINVQ_PREFIX`） |

脚本执行顺序：前置检查 → 定位产物 → 装基础依赖 → 装 PostgreSQL 18（PGDG 源）→ 装 Redis → 建用户与目录（`/data/App/FinvQuant/{bin,etc,Backup,Export,Deploy}` + `Deploy/Migrations`）→ 装二进制、迁移与部署文件 → 生成 env（随机 DB 密码）→ 建库建角色并授权 → 渲染并注册 systemd unit → 健康检查。

成功标志：

```
[OK] 健康检查通过（Ns）：http://127.0.0.1:16001/API/V1/Health/Ready
安装完成（安装根目录：/data/App/FinvQuant）
```

## 4. 手工分步安装（不想跑脚本时）

<details>
<summary>展开查看 8 个步骤</summary>

```bash
PREFIX=/data/App/FinvQuant

# 1) 依赖
apt-get update && apt-get install -y ca-certificates curl gnupg

# 2) PostgreSQL 18（PGDG）
install -d /usr/share/postgresql-common/pgdg
curl -fsSL -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc \
  https://www.postgresql.org/media/keys/ACCC4CF8.asc
echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt $(. /etc/os-release && echo $VERSION_CODENAME)-pgdg main" \
  > /etc/apt/sources.list.d/pgdg.list
apt-get update && apt-get install -y postgresql-18
systemctl enable --now postgresql

# 3) Redis
apt-get install -y redis-server && systemctl enable --now redis-server

# 4) 用户与目录（全部在安装根目录下）
groupadd --system finvquant
useradd --system --gid finvquant --no-create-home --home-dir "$PREFIX" --shell /usr/sbin/nologin finvquant
install -d -o root -g root -m 0755 "$PREFIX"
install -d -o root -g root -m 0755 "$PREFIX/bin"
install -d -o root -g finvquant -m 0755 "$PREFIX/Deploy/Migrations"
install -d -o root -g finvquant -m 0750 "$PREFIX/etc"
install -d -o root -g finvquant -m 0750 "$PREFIX/Backup"
install -d -o finvquant -g finvquant -m 0755 "$PREFIX/Export"
install -d -o root -g root -m 0755 "$PREFIX/Deploy"

# 5) 二进制、迁移与部署文件
install -o root -g root -m 0755 ./finvquant "$PREFIX/bin/finvquant"
ln -sfn "$PREFIX/bin/finvquant" /usr/local/bin/finvquant
cp Deploy/Migrations/*.sql "$PREFIX/Deploy/Migrations/" && chmod 0644 "$PREFIX/Deploy/Migrations/"*.sql
cp Deploy/Native/*.sh "$PREFIX/Deploy/"
cp Deploy/Native/*.md Deploy/Native/*.example Deploy/Native/FinvQuant.service "$PREFIX/Deploy/"
chmod 0755 "$PREFIX/Deploy/"*.sh

# 6) 配置（路径按实际安装根目录改）
install -d -o root -g finvquant -m 0750 "$PREFIX/etc"
sed "s|/data/App/FinvQuant|$PREFIX|g" Deploy/Native/FinvQuant.env.example \
  > "$PREFIX/etc/finvquant.env"
chmod 600 "$PREFIX/etc/finvquant.env"
vi "$PREFIX/etc/finvquant.env"        # 至少修改 FINV_PG_PASSWORD

# 7) 建库建角色（密码与 env 中一致；默认用户名/库名均为 finv）
sudo -u postgres psql -c "CREATE ROLE finv LOGIN PASSWORD '<同 env 中的密码>';"
sudo -u postgres createdb -O finv finv
sudo -u postgres psql -d finv -c "GRANT ALL ON DATABASE finv TO finv;"
sudo -u postgres psql -d finv -c "GRANT ALL ON SCHEMA public TO finv;"

# 8) systemd（unit 由模板渲染，同时留存到安装目录）
sed "s|@PREFIX@|$PREFIX|g" Deploy/Native/FinvQuant.service > "$PREFIX/Deploy/finvquant.service"
chmod 0644 "$PREFIX/Deploy/finvquant.service"
install -o root -g root -m 0644 "$PREFIX/Deploy/finvquant.service" /etc/systemd/system/finvquant.service
systemctl daemon-reload && systemctl enable --now finvquant
```

</details>

## 5. 验证

```bash
systemctl status finvquant                      # active (running)
journalctl -u finvquant -n 50 --no-pager        # 启动日志、迁移执行情况
curl http://127.0.0.1:16001/API/V1/Version      # 返回 version
curl -o /dev/null -w '%{http_code}\n' http://127.0.0.1:16001/API/V1/Health/Ready   # 200
curl -o /dev/null -w '%{http_code}\n' http://127.0.0.1:16002/                      # 200

# 安装布局自检
ls -l /data/App/FinvQuant/{bin,etc,Deploy} && ls /data/App/FinvQuant/Deploy/Migrations | head -3
```

迁移确认（应包含 `29`、`30`、`31`、`32`、`33`、`100024` 等）：

```bash
sudo -u postgres psql -d finv -c \
  "select version, success from schema_version order by version::int desc limit 8;"
```

## 6. 反向代理与对外暴露

```bash
sudo cp /data/App/FinvQuant/Deploy/Nginx.conf.example /etc/nginx/sites-available/finvquant
sudo ln -sf /etc/nginx/sites-available/finvquant /etc/nginx/sites-enabled/finvquant
sudo vi /etc/nginx/sites-available/finvquant    # 改 server_name 与证书路径
sudo nginx -t && sudo systemctl reload nginx
```

防火墙（只放行 80/443，16001/16002 保持 127.0.0.1 监听）：

```bash
ufw allow 80,443/tcp
ufw enable
```

## 7. 常见问题

| 现象 | 排查 |
|---|---|
| `systemctl status` 显示 `failed` | `journalctl -u finvquant -n 100 --no-pager` 看具体错误 |
| 启动报 `Failed to load environment files` / 找不到二进制 | 确认 unit 里的 `@PREFIX@` 已渲染（`grep -n 'data/App' /etc/systemd/system/finvquant.service`），且 `/data` 已挂载 |
| 日志含 `迁移 V<n> 失败` | 迁移单事务执行、失败已回滚；多为权限不足（需 `CREATE SCHEMA` 权限，见第 3 节授权） |
| `PG 连通性检查失败` | 检查 `FINV_PG_HOST/PORT/USER/PASSWORD` 与 `pg_hba.conf`（`127.0.0.1/32` 是否 `scram-sha-256`） |
| 健康检查通过但 Web 打不开 | 确认 `FINV_SERVER_HOST=127.0.0.1` 且 Nginx 反代指向 16002；或临时改 0.0.0.0 直连验证 |
| 手工 `psql` 报 `relation does not exist` | 表在 `quote`/`quant`/`futu`/`export`/`sche`/`dedup`/`archive`/`sys` schema，需写全限定名或设置 `search_path`，见 `Docs/DevSpec/DbSchemaSpec.md` |
| 端口被占用 | `ss -ltnp \| grep -E '16001\|16002'` |
| 升级时提示「新二进制与已安装二进制为同一文件」 | 不要指向 `/usr/local/bin/finvquant` 软链；用 `--binary` 指向真正的新版本文件 |

## 8. 卸载

```bash
# 保留配置、迁移、部署文件、备份、导出与数据库
sudo bash /data/App/FinvQuant/Deploy/Uninstall.sh

# 额外删除配置/迁移/部署文件/二进制目录（仍保留 Backup 与 Export 数据，不删库）
sudo bash /data/App/FinvQuant/Deploy/Uninstall.sh --purge

# 额外打印删库与清理备份/导出目录的指引（不自动删）
sudo bash /data/App/FinvQuant/Deploy/Uninstall.sh --purge --purge-data

# 自定义安装目录时
sudo bash /data/App/FinvQuant/Deploy/Uninstall.sh --prefix /your/prefix --purge
```
