# 变更日志（CHANGELOG）

> 所属：FinvQuant · 存放：`Docs/`
> 文件名**全大写** `CHANGELOG.md`（不写作 `ChangeLog.md`）。
> 版本号唯一来源：仓库根目录 [`VERSION`](../VERSION)；每次发布须与 Git tag 一一对应。

## 记录规则

- **强制**：任何使用者或部署者可感知的修改（功能、修复、接口、数据库、配置、部署、开发规范）必须在同一次提交中更新本文件，详见 [`DevSpec/DocSpec.md`](DevSpec/DocSpec.md) 第 4 节与 [`DevSpec/GitSpec.md`](DevSpec/GitSpec.md) 第 3 节。
- 按 `新增` / `变更` / `修复` / `移除` / `安全` 五类分组记录，无内容的分组不保留。
- 尚未发布的变更记入「未发布」小节；发布时整体改为对应版本号并补上发布日期。
- 只记录**面向使用者与部署者**可见的变化；纯内部重构可合并为一条描述。
- 日期格式 `YYYY-MM-DD`；提交信息规范见 [`DevSpec/GitSpec.md`](DevSpec/GitSpec.md)，文件命名规范见 [`DevSpec/FileNamingSpec.md`](DevSpec/FileNamingSpec.md)。

---

## [未发布]

### 新增

- 新增本变更日志 `Docs/CHANGELOG.md`，汇总 0.1.0 及历史 VeritasQuant 版本的对外变更。
- 开发规范新增「变更日志」强制要求：[`DevSpec/DocSpec.md`](DevSpec/DocSpec.md) 第 4 节、[`DevSpec/GitSpec.md`](DevSpec/GitSpec.md) 第 3 节，并在 [`DevSpec/FileNamingSpec.md`](DevSpec/FileNamingSpec.md) 中明确本文件名全大写。
- 新增需求清单 [`RequirementList.md`](RequirementList.md)，采用五位数编号（自 `10001` 起），记录 2026-10-24 起的新增需求，首版 65 条；`AGENTS.md` 工作约定同步要求新增需求必须登记。
- 新增两份改造清单：改造项 A [`Refactor/2610-SecuCodeToUsc.md`](Refactor/2610-SecuCodeToUsc.md)（`secu_code` → `usc` 全库统一，`A-01`～`A-37`）、改造项 B [`Refactor/2610-PrevCloseToLastClose.md`](Refactor/2610-PrevCloseToLastClose.md)（`prev_close` → `last_close` 重命名 + `open` 兜底回填，`B-01`～`B-24`），共 61 个可勾选检查项与 4 项待决策；B 的删除动作暂缓，留待生产数据核实后另议。

### 变更

- 统一跨平台换行策略：新增根目录 [`.gitattributes`](../.gitattributes)，所有文本文件以 LF 存储并检出，`*.cmd` / `*.bat` / `*.ps1` 保留 CRLF；规范见 [`DevSpec/FileEncodingSpec.md`](DevSpec/FileEncodingSpec.md) 第 4 节。
- 按上述规则重规范化全库行尾，消除 Windows / Linux 开发环境间的纯行尾差异。
- **破坏性变更（数据库）**：证券代码列 `secu_code` 统一正名为 `usc`，覆盖 6 张表（`finv_quote_secu_kline_min`、`finv_quote_ingest_batches`、`finv_quote_revision_log`、`finv_quant_backtest_strategy`、`finv_quant_backtest_run`、`finv_quant_backtest_run_archive`），5 个依赖索引同步改名；由迁移 `V29` 幂等执行，主键 `finv_quote_secu_kline_min_pkey` 名称不变、不重建。
- **破坏性变更（接口）**：接口与表单中的证券代码字段统一为 `usc`，查询参数由 `secuCode` 改为 `usc`，不再接受旧名（`Quote/History/QuoteQuery`、`Imports/Upload`、`Backtest/Run/Create|List`、`Strategy/Save|List` 等）；`secu_name` 等其余字段不受影响。
- **升级要求**：新旧镜像不可共存（旧版本无法读写新列名），必须先停旧容器再启动新版本，让服务启动时自动执行 `V29`；升级前请按备份流程做一次 `pg_dump`。
- **破坏性变更（数据库）**：`finv_quote_secu_kline_min.prev_close` 重命名为 `last_close`（迁移 `V30`），口径为数据源的「前收盘价」，与 `open` 独立；涨跌额/涨跌幅仍按 `close - last_close` 计算，**计算逻辑与接口字段不变**，前端零改动。
- **数据修复**：`open` 为 `NULL` 或 `0` 且 `last_close` 有效时，用 `last_close` 兜底回填（迁移 `V31`），并写入 `finv_quote_revision_log`（`ingest_batch_id = MIGRATION_V31`）留痕；新导入数据由导入链路按同一规则实时兜底，不再产生缺 `open` 的行。
- **升级提示**：该回填**不可逆**（覆盖了原始缺失状态），升级前务必完成 `pg_dump` 备份；如需还原只能从备份恢复。
- **破坏性变更（数据库）**：表按业务域迁入独立 schema —— `finv_quote_*`（3 张）→ `quote`，`finv_quant_*`（13 张）→ `quant`；字典表与 `schema_version` 保留 `public`。由迁移 `V32` 幂等执行，只改对象归属、不改表名/列/数据，索引与触发器自动跟随。
- **连接要求**：应用连接串固定携带 `search_path=public,quote,quant`（`internal/database/postgres.go`）；手工 `psql` 未设置 `search_path` 时须使用全限定名（如 `quote.finv_quote_secu_kline_min`）。
- **全新库兼容修复**：历史种子迁移 `V100019`（INSERT 列名）与 `V100023`（建表列定义）已把 `secu_code` 改为 `usc`——这两支在 `V29` 之后执行，不改会导致全新库首次部署失败；已应用库不受影响（`schema_version` 跳过）。新增 `V100024` 把 `V100023` 在 `public` 新建的 3 张表移入 `quant`。
- 新增规范 [`DevSpec/DbSchemaSpec.md`](DevSpec/DbSchemaSpec.md)：schema 划分、表命名、`search_path` 约定、迁移例外规则。
- **破坏性变更（数据库）**：富途映射表 `finv_futu_mapping_*`（5 张：`cs_market`、`exchange`、`market_code`、`region`、`security`，共 641 行）迁入 `futu` schema，由迁移 `V33` 幂等执行；索引与触发器自动跟随，无表名/列/数据变化。
- **连接要求更新**：`search_path` 扩展为 `public,quote,quant,futu`（应用连接串与数据库默认值同步）；Go 代码对本组表无引用，无需改后端 SQL。
- **防回归校验**：新增 `TestSQLSchemaQualified`（`internal/database/schema_qualify_test.go`），扫描 `internal/`、`cmd/` 全部 Go 源码，要求涉及 `finv_quote_*` / `finv_quant_*` / `finv_futu_*` 的 CRUD 语句必须带 schema 全限定名（`quote.` / `quant.` / `futu.`）；漏写会让 `go test ./...` 失败，为后续系统融合的同名表场景兜底。
- 规范补充：`DbSchemaSpec.md` 新增「融合前的同名表风险」——`public` 在 `search_path` 首位，新增同名表必须先移出首位并核验历史种子迁移。
- **部署目录重组**：`Deploy/` 按部署方式拆分为 `Deploy/Docker/`（容器方式：`docker-compose.yml`、`Dockerfile`、`.env.example`、`deploy/upgrade/rollback` 脚本、三份 Windows Docker 文档）与 `Deploy/Native/`（宿主直装方式，当前仅说明文档）；两种方式共用的 `Migrations/`、`Backup/` 保留在 `Deploy/` 根下。
- **目录命名**：备份目录由 `Deploy/backup/` 调整为 `Deploy/Backup/`（与 PascalCase 目录命名一致）；升级脚本、文档与 `.gitignore` 同步更新。
- **构建路径调整**：`Dockerfile` 迁入 `Deploy/Docker/`，compose 的 `build.context` 改为 `../..`、`dockerfile` 为 `Deploy/Docker/Dockerfile`；CI workflow 的 `file` 同步更新。
- 新增 [`Deploy/Native/README.md`](Deploy/Native/README.md)：明确 Native 方式的**适用场景与不适用场景**、采用前自检清单、与容器方式的责任边界，避免误用。
- **Native 部署配套落地**：新增 `Deploy/Native/{README,Install,Upgrade}.md` 与 `Install.sh`、`Upgrade.sh`、`Uninstall.sh`、`FinvQuant.service`、`FinvQuant.env.example`、`Nginx.conf.example`；服务由 systemd 守护（`Restart=on-failure`、`EnvironmentFile`、安全加固、journald），安装脚本幂等并自动生成随机数据库密码、建库授权、健康检查；升级脚本按「备份 → 停服 → 备份旧二进制 → 替换 → 启动 → 健康检查」执行，支持 `--rollback` 回退程序。
- **文件名按品牌大小写**：`FinvQuant.service`、`FinvQuant.env.example`（安装后注册为 `finvquant.service`，运维命令仍用 `systemctl ... finvquant`）。
- 新增 [`Deploy/README.md`](Deploy/README.md)：部署目录总览——两种方式的目录边界与选择建议、共用 `Migrations/` 与 `Backup/` 的说明、两种方式共同遵守的约束（禁止新旧共存、升级前备份、schema 限定）、目录命名约定。
- **数据库名与角色名统一为 `finv`**（融合前准备，**破坏性**）：PostgreSQL 数据库 `finvquant`→`finv`、登录角色 `finvquant`→`finv`，**密码不变**；生产库已执行 `ALTER ROLE` / `ALTER DATABASE` 改名并验证（数据 2,804,329 行不变、`search_path` 随 OID 保留、授权无需重跑）。
  - 配置：`internal/config` 默认值、`docker-compose.yml`（`FINV_PG_USER`/`FINV_PG_DATABASE`/`POSTGRES_USER`/`POSTGRES_DB`/`pg_isready`）、`.env.example`、Native `FinvQuant.env.example`、`Install.sh` 兜底值、`deploy.cmd`/`upgrade.cmd` 的 `psql`/`pg_dump` 参数
  - 不改：产品/镜像/容器/项目名 `finvquant`、Linux 系统用户与 `/opt/finvquant`、二进制名、备份文件名前缀
  - 新增 [`.dockerignore`](../.dockerignore)：构建上下文排除 `Deploy/Backup/`、`Web/node_modules/`、`Docs/`、`Archive/` 等，构建耗时降至 ~38s
  - 改造清单：新增 [`Docs/Refactor/2610-RenameDatabaseAndRole.md`](Refactor/2610-RenameDatabaseAndRole.md)（`E-01`～`E-30`）
- 新增 [`Archive/README.md`](../Archive/README.md)：仓库级历史归档目录的用途与边界（放什么、不放什么、按来源建子目录、与原位置对照命名、须随替换提交一并入库），并在根 `README.md` 目录结构与本文件中登记；`Deploy/README.md` 增加「废弃文件去处」指引。
- **备份文件命名规范化**：数据库备份统一为 `finvquant_YYYYMMDD_HHMMSS.dump`（纯 ASCII，按名称即可排序）；
  Docker 升级脚本改用 PowerShell 生成时间戳（不再截取 `%date%`，避免中文系统出现「周五」等字符与区域格式错乱）；
  `Deploy/Backup/` 下 9 个历史备份已按实际生成时间重命名；规范写入 `FileNamingSpec.md` 第 5 节。

## [0.1.0] - 2026-08-08

FinvQuant 首个正式版本：Go 服务端 + Vue3 前端前后端分离骨架、历史行情、元数据字典、通用回测引擎与容器化交付链路落地。

### 新增

- **服务端**：Go 1.25.3 + Gin + pgx/v5（PostgreSQL 18）+ go-redis v9 骨架，默认端口 16001，提供 `/API/V1/Health/Live|Ready` 与 `/API/V1/Version`。
- **前端**：Vue3 + Vite 8 + Vuetify 4（TypeScript），默认端口 16002，开发态 `/api` 代理、生产态反代至 16001。
- **All-in-One 单镜像**：前端构建产物经 `go:embed` 内嵌进 Go 二进制，单进程双端口，无需独立 Nginx 容器。
- **历史行情**：MVSV 文件上传导入 PostgreSQL（field 两种列布局自动识别、字段语义与溯源头部键校验）、K 线查询（分钟级 / 5 日、`ts` 范围查询、日历点选、红涨绿跌、MA/BOLL 图例显隐、富途风格悬停详情）。
- **元数据与数据字典**：`finv_exchange`、`finv_market`、`finv_currency`、`finv_region`、`finv_security` 表及其富途映射表（security / market_code / cs_market / region / exchange），含种子数据与 `flag_enable` 启用标志。
- **元数据管理 API 与维护页面**：字典分页查询（启用项优先排序）、`Security/Lookup` 双策略匹配（`usc` 或源证券代码）、前端多级菜单与元数据维护界面。
- **回测引擎**：通用量化回测引擎（以黄金期货合约完成验证）、多用户 / 环境 / 模板模型、异步任务删除与审计留痕、分批建仓减仓与滑动窗口交易限制。
- **回测报告**：独立报告页（回测背景信息卡片、图表全宽与全屏、拒绝原因可读化、指标与报告导出）。
- **部署交付**：Docker Compose 编排、一键部署 / 版本检测升级 / 回滚脚本及 Windows 11 使用手册。
- **CI/CD**：GitHub Actions 构建并推送 `ghcr.io/acanx/finvquant`；镜像 tag 按分支区分（`dev` 时间戳 / `main` 版本 tag），多平台构建与 `linux-amd64` / `linux-arm64` 平台专属 tag，拉取时输出 manifest digest。
- **版本管理**：新增 `VERSION` 文件作为全局版本唯一来源。

### 变更

- API 路径统一为大写 PascalCase `/API/V1/`，Meta 类接口前缀迁移为 `/API/V1/Meta/Finv/Quant/`。
- 数据库迁移与表名统一 `finv_` 前缀，种子数据段位调整为 `V100000`（十万级）。
- 仓库结构重组：原项目代码整体移入 `VeritasQuant/` 子目录，根目录重建为「外壳 + 子项目」结构，`deploy/` 重命名为 `Deploy/`。
- Docker Compose 顶层项目名统一为 `finvquant`，卷名不再带 `deploy` 前缀，容器名支持自定义（默认 `finvquant`）。
- PostgreSQL 默认暴露宿主机 5432 端口，数据目录支持宿主机文件系统映射。
- 前端 View 目录按菜单分层重组（`Web/src/views/Meta/Finv/`），元数据模块独立为菜单分支。

### 修复

- 生产模式 16002 前端 `/API` 请求正确反向代理到服务端 16001。
- PostgreSQL 18 镜像数据目录挂载点修正（`/var/lib/postgresql/data` → `/var/lib/postgresql`）。
- 部署脚本中文乱码修复（GBK → UTF-8 with BOM），镜像拉取增加网络波动重试与显式 compose 配置文件。
- MVSV 解析修复：文件头引号去除导致证券匹配失败、布局 B 更正为 12 列并容忍行尾空段、`ExchangeCode` 修正。
- 回测修复：每日计数死锁、可空列 NULL 扫描崩溃、时间区间与版本断言同步。
- 迁移修复：`INSERT` 末尾缺分号导致 `COMMIT` 语法错误、种子数据冲突处理改为 `ON CONFLICT DO NOTHING`、重建表迁移列顺序调整。

---

## 历史版本（VeritasQuant 子项目）

> 以下为 `VeritasQuant/` 独立阶段的历史版本，仅为追溯保留；其事件语义与验收门禁见 [`VeritasQuant/AGENTS.md`](../VeritasQuant/AGENTS.md)。

### VeritasQuant 0.1.2 - 2026-08-03

- 新增 FastAPI 应用、统一响应中间件与异常边界。
- 新增命令 API（202 受理、幂等键、乐观并发、取消与失败快照）。
- 新增账户 / 策略 / 数据 / 回测 / 基金计划与报告 API，基础 RBAC、请求追踪 ID、审计与限频。
- 新增鉴权 SSE 状态流与有界 replay cursor。
- 新增 Streamlit 控制台框架与数据导入、策略、定投计划、回测、账户、结果分析、监控页面。
- 新增调度计划与 JobRun 状态机、可观测性指标与 SLO 计算、端到端集成测试。

### VeritasQuant 0.1.1 - 2026-08-03

- 新增 PostgreSQL 事实表 / 投影表 / 索引与首版迁移，数据库端 inbox/outbox、租约与 fencing token。
- 新增共享事件持久化与确定性分区扇出、账户组 worker、风险快照屏障、纸上交易与对账。
- 新增基金定投核心逻辑与业绩报告（TWR、XIRR、本金与规则贡献）。
- 新增 M1 / M2A 门禁检查、崩溃演练报告、每周复核工具与容量预演。
- 完成 P3 信号参考闭环、P4 券商仿真适配层、P5 实盘安全核心与运行保障、P6 离线优化试验追踪。
