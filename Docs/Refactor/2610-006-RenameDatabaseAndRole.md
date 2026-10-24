# 改造项 E：数据库名与角色名 `finvquant` → `finv`（检查清单）

> 用途：把 PostgreSQL 的**数据库名**与**登录角色名**从 `finvquant` 改为 `finv`（密码保持不变）的工作，拆成**可逐条勾选的检查项**，按编号顺序执行以避免漏改；执行中新发现的改造点追加到末尾（编号顺延），不插入既有编号。
> 使用方式：自上而下推进；完成一项把「状态」改为 `☑ YYYY-MM-DD`，并附提交短号。
> 编号规则：本文件条目 `E-01`～`E-xx`，**只增不改**；待决策用 `EE-01`～（双字母前缀）。
> 完成判定：每项给出可执行的核验命令或明确的代码检查点，**未通过判定不得勾选**。
> 关联文档：改造项 A～D（[`2610-001-SecuCodeToUsc.md`](2610-001-SecuCodeToUsc.md) / [`2610-002-PrevCloseToLastClose.md`](2610-002-PrevCloseToLastClose.md) / [`2610-003-MoveTablesToSchemas.md`](2610-003-MoveTablesToSchemas.md) / [`2610-004-MoveFutuTablesToSchema.md`](2610-004-MoveFutuTablesToSchema.md)）；schema 规范见 [`../DevSpec/DbSchemaSpec.md`](../DevSpec/DbSchemaSpec.md)；需求验收见 [`../RequirementList.md`](../RequirementList.md)。
> 基线：仓库 `VERSION=0.1.1`，结构迁移最高版本 `V33`，种子段 `V100024`。

## 0. 概览

| 编号 | 标题 | 类型 | 影响面 | 风险 | 状态 |
|---|---|---|---|---|---|
| E | 数据库名 / 角色名 `finvquant` → `finv` | 配置与实例标识变更（破坏性） | 配置文件 4 处、脚本 4 处、源码默认值 1 处、文档 8 篇、生产库 1 次 `ALTER` | 中（需停机、需处理活动连接；**不做数据搬迁**） | ☑ 2026-10-24 已完成（30 项全部完成，生产库已改名并验证） |

## 1.1 背景与要求

- 需求：融合（合并多套系统）前，把数据库名与角色名统一为 `finv`，避免与产品名 `FinvQuant` 所在的命名空间混淆。
- **密码不变**：沿用原有密码，只改库名与角色名。
- **不改的东西**（`EE-01`）：
  - 产品/镜像/容器/项目名：`ghcr.io/acanx/finvquant`、容器 `finvquant` / `fq-postgres` / `fq-redis`、compose 项目名 `finvquant`；
  - Linux 系统用户与安装目录：`finvquant`、`/opt/finvquant`；
  - 二进制名 `finvquant`、PG 数据目录路径（含 `FinvQuant`）；
  - 备份文件名前缀 `finvquant_YYYYMMDD_HHMMSS.dump`。
- 只改**数据库对象标识**：`FINV_PG_USER`、`FINV_PG_DATABASE`、`POSTGRES_USER`、`POSTGRES_DB`、`pg_isready`、`psql -U/-d`、`pg_dump -d`。

## 1.2 为什么"改名"不需要重授权（记录判断依据）

PostgreSQL 的权限与 `search_path` 都绑定 **OID**（对象内部 ID），而非名字：

- `ALTER ROLE finvquant RENAME TO finv` → 角色 OID 不变 → 该角色在 `pg_database.datacl`、各 schema/表的 `ACL` 中的授权**自动生效**，无需 `GRANT` 重跑；
- `ALTER DATABASE finvquant RENAME TO finv` → 库 OID 不变 → `pg_db_role_setting`（`ALTER DATABASE ... SET search_path`）**随 OID 保留**；
- 库 owner = 角色 OID，改名后仍是同一角色。
- **唯一需要人工确认**：`pg_hba.conf` / `.pgpass` / 外部连接串等**按名字匹配**的地方（`E-18`）。

## 2. 改动清单

| 编号 | 项目 | 位置 | 判定方式 | 状态 |
|---|---|---|---|---|
| E-01 | 确定"不改"范围：产品名、镜像、容器、项目名、系统用户、路径、备份前缀 | 本文件 §1.1 | 人工确认，写入本节 | ☑ 2026-10-24 |
| E-02 | 源码默认值：`PgUser` / `PgDatabase` → `finv`（`PgPassword` 默认值不动） | `internal/config/config.go` | `grep -n '"finv"' internal/config/config.go` 命中 2 行 | ☑ 2026-10-24 |
| E-03 | compose：`FINV_PG_USER` / `FINV_PG_DATABASE` 默认值 → `finv` | `Deploy/Docker/docker-compose.yml` | `grep -c 'FINV_PG_USER:-finv}' ` 命中；无 `FINV_PG_USER:-finvquant}` | ☑ 2026-10-24 |
| E-04 | compose：`POSTGRES_USER` / `POSTGRES_DB` 默认值 → `finv` | 同上 | 同上 | ☑ 2026-10-24 |
| E-05 | compose：健康检查 `pg_isready -U finv -d finv` | 同上 | `grep -n 'pg_isready' docker-compose.yml` 输出含 `-U finv -d finv` | ☑ 2026-10-24 |
| E-06 | compose：**不改** `name:`、服务名、`container_name`、镜像、卷名 | 同上 | 人工核对 diff 未触及这些行 | ☑ 2026-10-24 |
| E-07 | `.env.example`：`FINV_PG_USER` / `FINV_PG_DATABASE` → `finv`，密码保持原值 | `Deploy/Docker/.env.example` | `grep -E '^FINV_PG_(USER\|DATABASE)=' Deploy/Docker/.env.example` 输出 `finv` | ☑ 2026-10-24 |
| E-08 | 本机 `.env`（不入库）同步改为 `finv` | `Deploy/Docker/.env` | `grep -E '^FINV_PG_(USER\|DATABASE)=' Deploy/Docker/.env` 输出 `finv` | ☑ 2026-10-24 |
| E-09 | `deploy.cmd` 的迁移版本查询改用新库名 | `Deploy/Docker/deploy.cmd` | `grep -c 'psql -U finv -d finv' ` = 1 | ☑ 2026-10-24 |
| E-10 | `upgrade.cmd` 的 `pg_dump` 与版本查询改用新库名 | `Deploy/Docker/upgrade.cmd` | `grep -c 'pg_dump -U finv -d finv' ` = 1 且版本查询同样 | ☑ 2026-10-24 |
| E-11 | `rollback.cmd` 若有库名引用则同步（无则记录） | `Deploy/Docker/rollback.cmd` | `grep -c 'finvquant' rollback.cmd` 全为产品名（服务名/容器名） | ☑ 2026-10-24 |
| E-12 | Native 安装脚本建库/建角色兜底值 → `finv`；**系统用户 `APP_USER` 保持 `finvquant`** | `Deploy/Native/Install.sh` | `grep -E 'DB_(NAME\|USER)=.*finv"' ` 命中；`grep 'APP_USER=' ` 仍为 `finvquant` | ☑ 2026-10-24 |
| E-13 | Native env 模板 `FINV_PG_USER` / `FINV_PG_DATABASE` → `finv` | `Deploy/Native/FinvQuant.env.example` | 同 E-07 | ☑ 2026-10-24 |
| E-14 | Native 升级/卸载脚本：库名全部来自 env 变量，无硬编码（核对即可） | `Upgrade.sh`、`Uninstall.sh` | `grep -c 'finvquant' Upgrade.sh` 仅剩产品名 | ☑ 2026-10-24 |
| E-15 | 生产库改名前置检查：无长事务、已做备份 | Docker `fq-postgres` | `pg_stat_activity` 无长事务；`Deploy/Backup/` 有当日 dump | ☑ 2026-10-24 |
| E-16 | 停应用容器释放连接 | `docker stop finvquant` | `pg_stat_activity` 中 `datname='finvquant'` 计数 = 0 | ☑ 2026-10-24 |
| E-17 | 执行 `ALTER ROLE finvquant RENAME TO finv;` | `psql -d postgres` | `select count(*) from pg_roles where rolname='finv'` = 1 且 `finvquant` = 0 | ☑ 2026-10-24 |
| E-18 | 执行 `ALTER DATABASE finvquant RENAME TO finv;` | `psql -d postgres` | `select count(*) from pg_database where datname='finv'` = 1 且 `finvquant` = 0 | ☑ 2026-10-24 |
| E-19 | 确认授权随 OID 保留：库 owner、`datacl`、`public`/`quote`/`quant`/`futu` 的 `USAGE`/`CREATE` | `psql -d finv` | 查询 `pg_database.datacl` 与 `information_schema.role_usage_grants`，`finv` 均具备 | ☑ 2026-10-24 |
| E-20 | 确认库级 `search_path` 保留 | `psql -d finv` | `select setconfig from pg_db_role_setting s join pg_database d on d.oid=s.setdatabase where d.datname='finv'` 输出 `search_path=public, quote, quant, futu` | ☑ 2026-10-24 |
| E-21 | 确认 `pg_hba.conf` 无按用户名白名单（容器默认 `all`） | `docker exec fq-postgres cat /var/lib/postgresql/data/pg_hba.conf` | 无 `finvquant` 字面量 | ☑ 2026-10-24 |
| E-22 | 重启服务（compose 会因 env 变化重建 postgres 容器，**数据卷保留**） | `docker compose up -d` | `docker ps` 三容器 healthy | ☑ 2026-10-24 |
| E-23 | 数据完整性验证 | `psql -d finv` | 行数 2,804,329 不变；`schema_version` 最高 `V100024`；`success` 全 `t` | ☑ 2026-10-24 |
| E-24 | 接口与前端回归 | `/API/V1/Health/Ready`、`/API/V1/Version`、行情页 | 200/版本正确/图表有数据 | ☑ 2026-10-24 |
| E-25 | 文档同步（8 篇）：Docker 2 篇、Native 3 篇、`Deploy/README.md`、`Prompt.md`、`DbSchemaSpec.md` | 各文档 | `grep -rn 'FINV_PG_USER' ` 默认值均为 `finv` | ☑ 2026-10-24 |
| E-26 | 迁移脚本核对：库名未硬编码（`V32`/`V33` 用 `current_database()`），**无需修改历史迁移** | `Deploy/Migrations/V32*.sql`、`V33*.sql` | `grep -c 'current_database()' ` 各命中 1；`grep -c 'ALTER DATABASE finvquant'` = 0 | ☑ 2026-10-24 |
| E-27 | `ALTER ROLE` 不能改当前会话用户 → 临时超级用户 `fq_rename_admin` 执行改名，完成后 `DROP ROLE` | 见 §3.1 | `select count(*) from pg_roles where rolname like 'fq_rename%'` = 0 | ☑ 2026-10-24 |
| E-28 | 断开外部 idle 连接（Navicat 等）：`ALLOW_CONNECTIONS false` + `pg_terminate_backend` | 见 §3.1 | 改名前 `pg_stat_activity` 中 `datname='finvquant'` = 0 | ☑ 2026-10-24 |
| E-29 | 新增 `.dockerignore`：构建上下文排除 `Deploy/Backup/`、`Web/node_modules/`、`Docs/`、`Archive/` 等 | 仓库根 `.dockerignore` | `test -f .dockerignore`；构建耗时从分钟级降到 ~38s | ☑ 2026-10-24 |
| E-30 | 重建镜像使 `internal/config` 新默认值进入镜像（运行时仍由 `.env` 决定） | `docker compose build finvquant` | 镜像重建成功，容器 healthy | ☑ 2026-10-24 |

## 3. 执行记录

### 3.1 生产库改名（2026-10-24）

实际执行中遇到两个必须处理的问题（追加为 E-27、E-28）：

1. **停应用容器后仍有 3 个 idle 连接**（来源 `application_name=Navicat`，客户端地址 `172.19.0.1`＝宿主经端口映射连入）。
   `ALTER DATABASE ... RENAME` 要求独占，故先切断新连接再强制断开：
   ```sql
   ALTER DATABASE finvquant ALLOW_CONNECTIONS false;
   SELECT count(pg_terminate_backend(pid)) FROM pg_stat_activity WHERE datname='finvquant';
   ```
2. **`ALTER ROLE` 不能重命名当前会话用户**（`ERROR: session user cannot be renamed`）。
   而本实例的唯一超级用户正是待改名的 `finvquant`，故需临时超级用户：
   ```sql
   -- 以 finvquant 连接
   CREATE ROLE fq_rename_admin SUPERUSER LOGIN PASSWORD '<随机>';
   ```
   ```sql
   -- 以 fq_rename_admin 连接（psql 多个 -c 各自独立事务；ALTER DATABASE 不能在事务块内）
   ALTER ROLE finvquant RENAME TO finv;
   ALTER DATABASE finvquant RENAME TO finv;
   ALTER DATABASE finv ALLOW_CONNECTIONS true;
   ```
   ```sql
   -- 以 finv 连接，清理临时角色
   DROP ROLE fq_rename_admin;
   ```
   注：`pg_` 前缀是保留前缀（`role name "pg_temp_admin" is reserved`），临时角色不可用 `pg_` 开头。

改名后核对：

| 项 | 结果 |
|---|---|
| 角色 `finv` 存在、`finvquant` 不存在 | ✓ |
| 数据库 `finv` 存在、`finvquant` 不存在 | ✓ |
| 库 owner | `finv` |
| `pg_db_role_setting` | `search_path=public, quote, quant, futu`（随 OID 保留） |
| 表数量（4 个 schema） | quote 3 / quant 13 / futu 5 / public 6 = 27 张 |
| `quote.finv_quote_secu_kline_min` 行数 | 2,804,329（不变） |
| `schema_version` 最高版本 | `V100024`，`success` 全 `t` |
| 备份文件名前缀 | 仍为 `finvquant_`（产品前缀，不变） |
| 接口 `/API/V1/Health/Ready` | `{"postgres":"ok","redis":"ok"}` |
| 行情接口（`NVDA` / `20260630`） | `code:0`，返回真实分钟线 |
| 前端 `16002` | HTTP 200 |

### 3.2 无需重授权（验证结论）

改名前后对比 `pg_database.datacl`、`pg_namespace.nspacl` 与 `information_schema.role_usage_grants`：授权对象一律为 OID，改名后自动指向 `finv`，**未执行任何 `GRANT` 语句**。

## 4. 风险与回滚

| 风险 | 处置 |
|---|---|
| `ALTER DATABASE` 因活动连接失败 | 先 `docker stop finvquant`；必要时 `ALTER DATABASE finvquant ALLOW_CONNECTIONS false` 后再试（改回 `true`） |
| compose 改了 `POSTGRES_USER`/`POSTGRES_DB` 后 postgres 容器被重建 | 数据卷是绑定挂载（`FINV_PG_DATA_DIR`），entrypoint 检测到已有数据会**跳过初始化**；重建前确认备份 |
| 改名后应用连不上 | 检查 `.env` 的 `FINV_PG_USER`/`FINV_PG_DATABASE` 是否同为 `finv`（大小写敏感） |
| 需要回滚 | `ALTER DATABASE finv RENAME TO finvquant;` + `ALTER ROLE finv RENAME TO finvquant;`（同样要求无连接），再回滚 `.env` 与镜像 |
| 全新部署是否受影响 | 不受影响：`V32`/`V33` 的 `search_path` 用 `current_database()`，任意库名下均可执行 |

## 5. 后续

- 融合（多系统合并）时若再有实例重名，按本文件流程复用：改名 → 核对 OID 级授权 → 同步配置 → 回归。
- 备份文件名前缀 `finvquant_` 属产品标识，如需与库名统一另立改造项，不在本项范围。
