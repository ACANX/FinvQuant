# Native 安装手册（宿主直装 + systemd）

> 适用：Debian 12+ / Ubuntu 22.04+（systemd），root 权限。
> 配套文件：`Install.sh`（一键安装）、`FinvQuant.service`（systemd unit）、`FinvQuant.env.example`（环境变量）、`Nginx.conf.example`（反代示例）。
> **采用本方式前请先阅读 [`README.md`](README.md) 的「适用场景 / 不适用场景」与自检清单。**
> 状态：脚本已提供，但**尚未在目标 VPS 上做过完整实机验证**；首次部署建议先在测试机完整走一遍。

## 0. 安装后会得到什么

| 项目 | 路径 / 说明 |
|---|---|
| 二进制 | `/opt/finvquant/finvquant`（软链 `/usr/local/bin/finvquant`） |
| 配置 | `/etc/finvquant/finvquant.env`（权限 600，含随机生成的数据库密码） |
| 迁移脚本 | `/etc/finvquant/Migrations/`（服务启动时自动应用未执行版本） |
| systemd unit | `/etc/systemd/system/finvquant.service` |
| 日志 | journald：`journalctl -u finvquant` |
| 数据 | PostgreSQL 18 默认数据目录（不随本应用管理） |
| 备份 | 升级脚本写入 `/var/backups/finvquant/` |

## 1. 前置条件

| 项 | 要求 |
|---|---|
| 系统 | Debian 12+ / Ubuntu 22.04+，systemd，x86_64 或 arm64 |
| 权限 | root（`sudo`） |
| 内存 | ≥ 2 GB（PostgreSQL + Go 服务 + Redis） |
| 磁盘 | ≥ 10 GB（行情数据量取决于导入规模） |
| 网络 | 能访问 PostgreSQL PGDG 源与发行版软件源 |
| 域名/证书 | 可选；生产建议配 Nginx + TLS |

## 2. 准备二进制与迁移脚本

`Install.sh` 会在以下位置查找 `finvquant` 与 `Deploy/Migrations`：

1. 仓库根（脚本位于 `<repo>/Deploy/Native/` 时自动识别）：
   - 二进制：`<repo>/finvquant`
   - 迁移：`<repo>/Deploy/Migrations/`
2. 独立发行包（脚本与产物同目录）：
   - 二进制：`<dir>/finvquant`
   - 迁移：`<dir>/Migrations/`
3. `--binary /path/to/finvquant` 显式指定二进制

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
```

常用参数：

| 参数 | 说明 |
|---|---|
| `--skip-db` | 已有 PostgreSQL 18 且已建库，跳过安装与建库 |
| `--skip-redis` | 已有 Redis，跳过安装 |
| `--binary <path>` | 指定二进制路径 |

脚本执行顺序：前置检查 → 定位产物 → 装基础依赖 → 装 PostgreSQL 18（PGDG 源）→ 装 Redis → 建用户与目录 → 装二进制与迁移 → 生成 env（随机 DB 密码）→ 建库建角色并授权 → 注册并启动 systemd → 健康检查。

成功标志：

```
[OK] 健康检查通过（Ns）：http://127.0.0.1:16001/API/V1/Health/Ready
安装完成
```

## 4. 手工分步安装（不想跑脚本时）

<details>
<summary>展开查看 8 个步骤</summary>

```bash
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

# 4) 用户与目录
useradd --system --create-home --home-dir /opt/finvquant --shell /usr/sbin/nologin finvquant
install -d -o root -g finvquant -m 0750 /etc/finvquant
install -d -o root -g finvquant -m 0755 /etc/finvquant/Migrations

# 5) 二进制与迁移
install -o root -g root -m 0755 ./finvquant /opt/finvquant/finvquant
ln -sfn /opt/finvquant/finvquant /usr/local/bin/finvquant
cp Deploy/Migrations/*.sql /etc/finvquant/Migrations/ && chmod 0644 /etc/finvquant/Migrations/*.sql

# 6) 配置
cp Deploy/Native/FinvQuant.env.example /etc/finvquant/finvquant.env
chmod 600 /etc/finvquant/finvquant.env
vi /etc/finvquant/finvquant.env        # 至少修改 FINV_PG_PASSWORD

# 7) 建库建角色（密码与 env 中一致）
sudo -u postgres psql -c "CREATE ROLE finvquant LOGIN PASSWORD '<同 env 中的密码>';"
sudo -u postgres createdb -O finvquant finvquant
sudo -u postgres psql -d finv -c "GRANT ALL ON SCHEMA public TO finv;"

# 8) systemd
cp Deploy/Native/FinvQuant.service /etc/systemd/system/finvquant.service
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
```

迁移确认（应包含 `29`、`30`、`31`、`32`、`33`、`100024` 等）：

```bash
sudo -u postgres psql -d finv -c \
  "select version, success from schema_version order by version::int desc limit 8;"
```

## 6. 反向代理与对外暴露

```bash
sudo cp Deploy/Native/Nginx.conf.example /etc/nginx/sites-available/finvquant
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
| 日志含 `迁移 V<n> 失败` | 迁移单事务执行、失败已回滚；多为权限不足（需 `CREATE SCHEMA` 权限，见第 3 节授权） |
| `PG 连通性检查失败` | 检查 `FINV_PG_HOST/PORT/USER/PASSWORD` 与 `pg_hba.conf`（`127.0.0.1/32` 是否 `scram-sha-256`） |
| 健康检查通过但 Web 打不开 | 确认 `FINV_SERVER_HOST=127.0.0.1` 且 Nginx 反代指向 16002；或临时改 0.0.0.0 直连验证 |
| 手工 `psql` 报 `relation does not exist` | 表在 `quote`/`quant`/`futu`/`export`/`sche`/`dedup`/`archive` schema，需写全限定名或设置 `search_path`，见 `Docs/DevSpec/DbSchemaSpec.md` |
| 端口被占用 | `ss -ltnp \| grep -E '16001\|16002'` |

## 8. 卸载

```bash
sudo bash Deploy/Native/Uninstall.sh              # 停服务、删 unit 与二进制，保留配置与数据
sudo bash Deploy/Native/Uninstall.sh --purge      # 额外删除 /etc/finvquant
sudo bash Deploy/Native/Uninstall.sh --purge --purge-data   # 打印删库指引（不自动删）
```
