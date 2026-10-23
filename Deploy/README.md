# Deploy 目录说明（部署与数据库迁移）

> 所属：FinvQuant · 存放：`Deploy/`
> 本目录同时承载**两种部署方式**的编排文件，以及两种方式**共用**的数据库迁移与备份目录。

## 0. 目录速览

```
Deploy/
├── Docker/        # 部署方式一：容器（Docker + Compose）
├── Native/        # 部署方式二：宿主直装（systemd 守护，无容器）
├── Migrations/    # 共用：数据库迁移脚本（服务启动时自动应用）
├── Backup/        # 共用：数据库逻辑备份（不入库，见 .gitignore）
└── README.md      # 本文件
```

| 路径 | 是否入库 | 说明 |
|---|---|---|
| `Docker/` | 是 | `docker-compose.yml`、`Dockerfile`、`.env.example`、部署/升级/回滚脚本、Windows Docker 文档 |
| `Native/` | 是 | systemd unit、安装/升级/卸载脚本、环境变量与 Nginx 模板、安装与升级手册 |
| `Migrations/` | 是 | 结构迁移 `V1`～`V99999`、种子/数据迁移 `V100000+`，**历史文件不可回改** |
| `Backup/` | 否 | `pg_dump -Fc` 产物，两种方式共用，命名 `finvquant_YYYYMMDD_HHMMSS.dump` |

## 1. 怎么选部署方式

| 你的情况 | 选择 |
|---|---|
| Windows 10/11 桌面、只想尽快跑起来 | [`Docker/`](Docker/)（[Windows 部署文档](Docker/Win11DockerDeploy.md)） |
| Linux 服务器且已装 / 允许装 Docker | [`Docker/`](Docker/)（最省心，镜像 tag 可回滚） |
| 宿主禁止容器、需复用已有 PostgreSQL/Redis、需精细控制 systemd 资源 | [`Native/`](Native/)（先读它的[适用场景与自检清单](Native/README.md)） |
| 本地开发、试跑、需要随时重建 | [`Docker/`](Docker/) |

> 两种方式**运行同一个程序、执行同一批迁移、遵循同一套 schema 规范**，只是「谁装数据库、谁守护进程」不同。Native 不是精简版。

## 2. 共用部分

### 2.1 `Migrations/`（数据库迁移）

- 命名 `V<版本号>__<描述>.sql`；结构迁移用 `V1`～`V99999`，种子/数据迁移用 `V100000+`，全部按版本号升序执行。
- 服务（容器或二进制）**启动时自动应用未执行版本**，记录在 `public.schema_version`；单事务执行，失败整体回滚。
- **历史迁移文件不可回改**（已应用库会按 `schema_version` 跳过，改动会造成存量库与全新库结构分叉）；唯一例外是「新结构已使历史迁移无法在全新库执行」的兼容修正，规则见 [`../Docs/DevSpec/DbSchemaSpec.md`](../Docs/DevSpec/DbSchemaSpec.md) §4。
- 目录位置：容器方式构建时拷入镜像 `/etc/finvquant/Migrations`；Native 方式由安装/升级脚本同步到 `/etc/finvquant/Migrations`；统一由环境变量 `FINV_MIGRATIONS_DIR` 指向。
- 手工执行迁移**不推荐**：请用服务启动自迁移，避免 `schema_version` 与实际结构不一致。

### 2.2 `Backup/`（数据库备份）

- 由 Docker 升级脚本（`Docker/upgrade.cmd`）与 Native 升级脚本（`Native/Upgrade.sh`）生成；Native 默认写到 `/var/backups/finvquant`，可直接复制到本目录统一留存。
- 命名规范：`finvquant_YYYYMMDD_HHMMSS.dump`（纯 ASCII，按名称即按时间排序；禁止中文与本地化日期格式），见 [`../Docs/DevSpec/FileNamingSpec.md`](../Docs/DevSpec/FileNamingSpec.md) 第 5 节。
- 本目录已被 `.gitignore` 忽略，不随仓库分发。

## 3. 两种方式共同遵守的约束

| 约束 | 说明 |
|---|---|
| 禁止新旧共存 | 破坏性版本（列改名、schema 迁移）变更后，新旧程序**不得同时连接同一数据库**；升级必须「停旧 → 备份 → 起新」 |
| 升级前必须备份 | 迁移**不可自动回滚**，回退数据库只能靠升级前的 `pg_dump` |
| schema 限定 | 业务表按域位于 `quote` / `quant` / `futu` / `export`（导出）/ `sche`（调度）schema，`search_path = public,quote,quant,futu,export,sche`；手工 `psql` 须写全限定名（如 `quote.finv_quote_secu_kline_min`） |
| 端口约定 | API `16001`、前端 `16002`；生产建议只绑 `127.0.0.1` 并由 Nginx 反代 |

## 4. 各方式的快速入口

### Docker 方式

```bash
cp Deploy/Docker/.env.example Deploy/Docker/.env      # 按需修改
docker compose -f Deploy/Docker/docker-compose.yml --env-file Deploy/Docker/.env up -d
```

- 详细部署：[Docker/Win11DockerDeploy.md](Docker/Win11DockerDeploy.md)
- 增量升级：[Docker/Win11DockerUpgrade.md](Docker/Win11DockerUpgrade.md)
- 一键脚本手册：[Docker/DeployUpgradeGuide.md](Docker/DeployUpgradeGuide.md)（`deploy.cmd` / `upgrade.cmd` / `rollback.cmd`）

### Native 方式（Linux + systemd）

```bash
sudo bash Deploy/Native/Install.sh     # 幂等安装
sudo bash Deploy/Native/Upgrade.sh     # 升级（含备份）；--rollback 回退程序
```

- 方式定位与适用边界：[Native/README.md](Native/README.md)
- 安装手册：[Native/Install.md](Native/Install.md)
- 升级与回滚：[Native/Upgrade.md](Native/Upgrade.md)

## 5. 目录命名约定

- 部署方式目录用**大驼峰**：`Docker/`、`Native/`。
- 共用资源目录同样大驼峰：`Migrations/`、`Backup/`。
- 方式内部的脚本与配置文件名按内容命名并采用大驼峰（如 `Install.sh`、`FinvQuant.service`），品牌词统一写作 **FinvQuant**。
- 新增部署方式时：新建同级目录（如 `Kubernetes/`）并在本文件第 0、1 节登记，不得把不同方式的文件混放在 `Deploy/` 根下。
- **废弃文件的去处**：被替换的部署脚本/配置/文档不做就地删除，移入仓库级历史归档目录 [`../Archive/`](../Archive/)（并按需在 `Archive/` 内自建 `Deploy/` 子目录），详见 [`../Archive/README.md`](../Archive/README.md)。归档文件不参与构建、部署与 CI。
