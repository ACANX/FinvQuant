# Native 部署（宿主直装，非容器）

> **状态：脚本与文档已提供，尚未在目标 VPS 实机验证** —— 首次部署请先在测试机完整走一遍
> [`Install.md`](Install.md)，确认无误后再用于生产；升级前务必阅读 [`Upgrade.md`](Upgrade.md) 的回滚约束。
>
> 上级目录说明（两种方式如何共存与共用迁移/备份）：[`../README.md`](../README.md)  
> 所属：FinvQuant 部署 · 存放：`Deploy/Native/`
> 定位：与 [`../Docker/`](../Docker/) 并列的**传统部署方式**——在 Linux 主机上直接安装 PostgreSQL、Redis
> 与 `finvquant` 二进制，不引入任何容器运行时。

## 0. 一句话结论

**这是给「有 Linux 运维能力、且明确不需要/不允许容器」的部署者准备的方式。**
如果你只是想在自己电脑或一台普通服务器上把 FinvQuant 跑起来，请直接用 [`../Docker/`](../Docker/) 的容器方式——它更省心，本项目的官方 Windows 11 文档也只覆盖容器方式。

## 1. 适用场景（以下场景才建议选 Native）

| 场景 | 说明 |
|---|---|
| 宿主**不允许或无法安装**容器运行时 | 受管主机策略禁止 Docker/Podman、内核不支持、或宿主机本身就是一个受限的 LXC/OpenVZ 容器 |
| 需要**复用已有的 PostgreSQL / Redis** | 团队已有统一维护的 PG 18 / Redis 8 实例，要求 FinvQuant 接入而不是再起一套容器 |
| 需要**精细控制**运行时 | 自定义 systemd 资源限制（`MemoryMax`/`CPUQuota`）、`ulimit`、内核参数、独立的数据目录与 IO 策略 |
| **资源受限** | 小规格 VPS（1～2 GB 内存），不想承担容器运行时的额外开销 |
| 需要与**本机其它服务深度集成** | 与本机 Nginx、监控 agent（node_exporter、filebeat）、配置管理（Ansible）统一编排 |
| 希望**二进制可控** | 自己编译/审计二进制、做本地签名校验、自定义 `VERSION` 与构建参数 |

## 2. 不适用场景（请改用 Docker 方式）

> 出现以下任一情况，请**不要**使用 Native 方式，改用 [`../Docker/`](../Docker/)：

| 场景 | 为什么不适合 | 建议 |
|---|---|---|
| Windows 10/11 桌面或 Windows Server | 官方脚本与文档只覆盖 Linux + systemd；Windows 下手工装 PG/Redis 并注册服务成本高、坑多 | 用 [`../Docker/Win11DockerDeploy.md`](../Docker/Win11DockerDeploy.md) |
| 不具备 Linux 运维能力（不会 systemd、不会配 PG 的 `pg_hba.conf`、不懂防火墙） | Native 方式把「数据库安装、账户权限、备份、升级、故障处置」全部交给你，出问题需要自己定位 | 用 Docker 方式，故障面小得多 |
| 期望「一条命令装好、以后自动升级」 | Native 升级需要**人工停服务 → 备份 → 换二进制 → 重启**，没有镜像 tag 可回退 | 用 Docker 方式（镜像 tag + 自动迁移） |
| 需要**快速回滚到任意历史版本** | Native 只能回滚到「你本地留存的旧二进制」；镜像方式可直接切 tag | 用 Docker 方式 |
| 需要**多实例并行**（多租户、灰度、A/B） | 容器隔离与端口/卷编排更简单；Native 需要自己规划多套目录、端口、systemd 实例 | 用 Docker 方式 |
| 无备份与回滚预案的生产环境 | 破坏性版本（如 `V29`～`V33` 的列改名、schema 迁移）**迁移不可自动回滚**，只能靠备份恢复 | 先建立 `pg_dump` 定时备份与演练，再考虑上线 |
| 只是本地开发/试跑 | 容器方式一条命令起全套，改代码即重建 | 用 Docker 方式 |

## 3. 采用前的自检清单

**以下 7 条必须全部为「是」，否则请改用 Docker 方式：**

1. 我能独立完成 PostgreSQL 18 的安装、建库建角色、`pg_hba.conf` 与连接数调优；
2. 我能独立完成 Redis 8 的安装与持久化（AOF/RDB）配置；
3. 我能编写与调试 systemd unit（含 `EnvironmentFile`、`Restart`、日志查看 `journalctl`）；
4. 我有可执行的数据库备份方案（至少每日 `pg_dump -Fc`），并演练过恢复；
5. 我能在升级前安排停机窗口（Native 升级不是零停机）；
6. 我清楚**数据库迁移由新版本二进制启动时自动执行、失败不会自动回滚**；
7. 我接受：主机环境（OS 版本、SELinux、防火墙、磁盘、时区）相关问题由我自己排查，项目文档只覆盖应用侧。

## 4. 你需要承担的责任（与 Docker 方式的差别）

| 事项 | Docker 方式 | Native 方式（你负责） |
|---|---|---|
| PG / Redis 安装与版本升级 | 镜像固定版本，`pull` 即升级 | 你自行规划（本项目要求 PG 18 / Redis 8） |
| 数据目录与磁盘规划 | 命名卷 + 宿主映射目录 | 你自行规划（**程序、配置、迁移、备份、导出统一在 `/data/App/FinvQuant/`**；PG 数据目录仍由你规划） |
| 备份与恢复 | 升级脚本自动 `pg_dump` 到 `Deploy/Backup/` | 你需自建定时备份（可复用同一目录约定） |
| 服务守护与开机自启 | `restart: unless-stopped` | 你需提供 systemd unit |
| 日志 | `docker logs` | `journalctl -u finvquant`（你需配日志轮转） |
| 安全加固 | 端口映射 + 容器隔离 | 你需配防火墙（仅暴露 16002/反代端口）、PG/Redis 只绑 `127.0.0.1`、TLS |
| 升级与回滚 | 切镜像 tag | 你需留存旧二进制与备份 |

## 5. 两种方式的技术差异（同源，非分叉）

| 维度 | `Deploy/Docker/` | `Deploy/Native/`（本目录） |
|---|---|---|
| 交付物 | All-in-One 镜像（前端经 `go:embed` 内嵌） | **同一份** `finvquant` 二进制（前端同样内嵌，无需外置静态资源） |
| 数据库迁移 | 镜像内 `/etc/finvquant/Migrations`（构建时拷入），启动时自动应用 | 同一批脚本随包放在 `Deploy/Migrations`，安装后即 `/data/App/FinvQuant/Deploy/Migrations`（`FINV_MIGRATIONS_DIR` 指定），启动时自动应用 |
| 配置 | `Deploy/Docker/.env` | `/data/App/FinvQuant/finvquant.env`（systemd `EnvironmentFile`），键名一致 |
| 端口 | 16001（API）+ 16002（前端） | 相同；通常再前置 Nginx 统一入口 |
| 升级动作 | 停容器 → 拉镜像 → 起容器 | 停服务 → 备份 → 换二进制 → 起服务 |
| 破坏性变更约束 | 禁止新旧镜像共存 | 同样禁止新旧二进制共存 |

> 关键点：两种方式**运行的是同一个程序、执行同一批迁移、遵循同一套 schema 规范**。Native 不是「精简版」或「另一套代码」，只是「谁来装 PG/Redis、谁来守护进程」不同。

## 6. 安装后目录布局（`/data/App/FinvQuant/`）

安装脚本把**二进制、数据库脚本、核心配置、部署脚本与部署文件**全部收拢到同一个安装根目录，
数据（备份与导出）也在此目录下，便于整目录备份、迁移与容量规划；只有 systemd unit 因框架要求放在 `/etc/systemd/system/`
（由 `<PREFIX>/Deploy/finvquant.service` 安装，两处内容一致）。

```
/data/App/FinvQuant/                  # 安装根目录（--prefix / FINVQ_PREFIX 可改）
├── bin/
│   ├── finvquant                     # 二进制（软链 /usr/local/bin/finvquant）
│   └── finvquant.prev                # 升级前的上一版本（可用于 --rollback）
├── Deploy/                           # 部署脚本、部署文件与数据库脚本（与仓库结构一致）
│   ├── Migrations/                   # 数据库迁移脚本（FINV_MIGRATIONS_DIR）
│   ├── Install.sh / Upgrade.sh / Uninstall.sh
│   ├── finvquant.service             # 渲染后的 unit（systemd 注册用）
│   ├── FinvQuant.service             # unit 模板（@PREFIX@ 占位）
│   ├── FinvQuant.env.example         # 环境变量模板
│   ├── Nginx.conf.example            # 反向代理示例
│   └── README.md / Install.md / Upgrade.md
├── finvquant.env                     # 核心配置环境变量（权限 600）
├── Backup/                           # 数据库逻辑备份（Upgrade.sh 写入 finvquant_*.dump）
└── Export/                           # 行情归档导出目录（与容器方式一致）
    └── Upload/                       # 上传暂存（UPLOAD_MODE=dir 时使用）
```

| 目录 | 权限 / 属主 | 说明 |
|---|---|---|
| `bin/` | `root:root 0755` | 二进制只读，升级时整体替换 |
| `Deploy/Migrations/` | `root:finvquant 0755` | 迁移脚本只读，服务以 `finvquant` 读取 |
| `finvquant.env` | `root:root 0600` | 含数据库密码，仅 root 可改 |
| `Deploy/` | `root:root 0755` | 脚本与模板，运维直接在此目录执行 |
| `Backup/` | `root:finvquant 0750` | 备份由 root 写入，避免 `postgres` 越权 |
| `Export/` | `finvquant:finvquant 0755` | 应用进程需写入 |

## 7. 两种方式共用的部分（不在本目录）

| 路径 | 用途 |
|---|---|
| [`../Migrations/`](../Migrations/) | 数据库迁移脚本（构建时内嵌进二进制，启动自动应用） |
| [`../Backup/`](../Backup/) | 数据库逻辑备份（`pg_dump -Fc` 产物，两种方式共用，已 `.gitignore`） |
| [`../../Docs/DevSpec/DbSchemaSpec.md`](../../Docs/DevSpec/DbSchemaSpec.md) | schema 划分与 `search_path` 规范（手工 `psql` 须写全限定名） |

## 8. 本目录文件清单

| 文件 | 用途 |
|---|---|
| [`README.md`](README.md) | 本文件：方式定位、适用/不适用场景、责任边界 |
| [`Install.md`](Install.md) | 安装手册：前置条件、二进制构建、一键/手工安装、验证、常见问题 |
| [`Upgrade.md`](Upgrade.md) | 升级与回滚手册：停机窗口、备份、迁移应用、程序/数据库回滚、定时备份 |
| [`Install.sh`](Install.sh) | 一键安装（幂等）：装依赖 → PG 18（PGDG）→ Redis → 用户目录 → 二进制/迁移/部署文件 → env（随机密码，600）→ 建库授权 → systemd → 健康检查；全部落在 `/data/App/FinvQuant/` |
| [`Upgrade.sh`](Upgrade.sh) | 一键升级：备份到 `<PREFIX>/Backup` → 停服 → 备份旧二进制 → 替换二进制/迁移/部署文件 → 启动 → 健康检查；`--rollback` 回退程序 |
| [`Uninstall.sh`](Uninstall.sh) | 卸载：停服禁用 → 删 unit 与二进制；`--purge` 删配置/迁移/部署文件，不自动删库与备份/导出 |
| [`FinvQuant.service`](FinvQuant.service) | systemd unit 模板（`@PREFIX@` 占位，安装时渲染为 `finvquant.service`；`Restart=on-failure`、`EnvironmentFile`、安全加固、journald） |
| [`FinvQuant.env.example`](FinvQuant.env.example) | 环境变量模板（`FINV_*`，与容器方式键名一致；默认路径指向 `/data/App/FinvQuant`） |
| [`Nginx.conf.example`](Nginx.conf.example) | 反向代理示例（80→443 跳转、前端 16002、API `/API/` → 16001、上传体积、TLS） |

> 文件名统一采用大驼峰（PascalCase）；品牌词写作 **FinvQuant**（与产品名一致）。
> 安装到系统时，unit 会以 `finvquant.service`（全小写，systemd 服务名惯例）注册，因此运维命令是 `systemctl status finvquant`。

## 9. 环境变量约定

沿用同一套 `FINV_*` 环境变量（键名见 [`../Docker/.env.example`](../Docker/.env.example)），差异仅在连接地址：

- `FINV_PG_HOST=127.0.0.1`、`FINV_REDIS_ADDR=127.0.0.1:6379`（Docker 方式为服务名 `postgres` / `redis`）
- `FINV_MIGRATIONS_DIR=/data/App/FinvQuant/Deploy/Migrations`、`FINV_QUOTE_DEDUB_OUTPUT_DIR=/data/App/FinvQuant/Export`（安装脚本按实际安装根目录改写）
- PG 与 Redis 建议**只绑 `127.0.0.1`**，由本机 `finvquant` 访问；对外只暴露 Web/反代端口
- 配置文件路径：`/data/App/FinvQuant/finvquant.env`（systemd `EnvironmentFile`，不要加 `export` 与引号）

## 10. 已知风险与注意事项

1. **迁移不可自动回滚**：破坏性版本（列改名、schema 迁移）升级后如需回退，只能从升级前的 `pg_dump` 恢复数据库；升级前务必完成备份。
2. **禁止新旧并用**：新旧二进制不得同时连接同一数据库（列名/schema 不兼容），升级必须「停旧 → 备份 → 起新」一次完成。
3. **不要手工改迁移**：`Deploy/Migrations/` 是历史记录，改动只允许新增；已有例外的判定标准见 `DbSchemaSpec.md` §4。
4. **`search_path` 依赖**：应用连接串固定为 `public,quote,quant,futu,export,sche,dedup,archive,sys`；手工 `psql` 时若未设置，须写全限定名（如 `quote.finv_quote_secu_kline_min`），否则可能报 `relation does not exist`。
5. **unit 名与服务名**：仓库内源文件为 `FinvQuant.service`，安装后注册为 `finvquant.service`（小写），两者是同一份配置，不要按仓库文件名去 `systemctl`。
7. **安装根目录可自定义**：`Install.sh --prefix <dir>`（或 `FINVQ_PREFIX`）可把整套程序装到其它绝对路径；`Upgrade.sh` / `Uninstall.sh` 默认从 unit 的 `ExecStart` 推断该路径，换过目录时无需手工指定。
6. **本项目不提供 Native 方式的主机环境兜底**：OS 版本、SELinux、防火墙、磁盘与内核参数等问题需要部署者自行处理；文档只覆盖应用侧配置与操作。
