# FinvQuant 智能体协作指南

## 适用范围

本文件适用于整个仓库（FinvQuant 量化策略交易平台）。

## 仓库结构

本仓库为"外壳 + 子项目"结构：

```
.
├── Archive/             # 仓库级历史归档（含 Archive/VeritasQuant/ 历史子项目，不参与构建）
├── cmd/ internal/         # FinvQuant Go 服务端（量化平台服务端）
├── Web/                   # 前端（Vue3 + Vite8 + Vuetify4）
├── Deploy/                # 部署编排（两种方式共用 Migrations/ 与 Backup/）
│   ├── Docker/            # 容器方式：compose、Dockerfile、部署与升级脚本/文档
│   ├── Native/            # 宿主直装方式：说明文档（脚本待补充）
│   ├── Migrations/        # 数据库迁移（两种方式共用，内嵌进二进制）
│   └── Backup/            # 数据库逻辑备份（两种方式共用，不入库）
├── .github/workflows/     # CI/CD（构建 + 推送 GHCR）
├── AGENTS.md              # 本文件（仓库级协作指南）
├── LICENSE                # MIT License（Copyright 2026 ACANX）
├── README.md              # 仓库概览与快速入口
└── Prompt.md              # 结构化需求文档（持续更新）
```

## 项目目标

FinvQuant 是一个面向多资产的**严格事件驱动量化交易平台**，支持多种基金智能定投方案和用户自定义定投规则的历史回测。在进入模拟盘、券商仿真和受控实盘前，必须先产出可复现的研究和回测结果。

## 技术栈（前后端分离）

### 服务端（Go）

- **Go 1.25.3**，标准模块化结构：`cmd/server` + `internal/{api,config,database,redisclient}`。
- 依赖（均为最新）：**Gin v1.12.0**、**go-redis v9.22**、**pgx/v5**（PostgreSQL 18 驱动）。
- 默认端口 **16001**；健康检查 `/API/V1/Health/Live|Ready`、版本 `/API/V1/Version`。
- 配置走环境变量（`FINV_*`），默认值适配 Docker Compose。

### 前端（Web/）

- **Vue3 + Vite 8.2 + Vuetify 4.1**（TypeScript）。
- 默认端口 **16002**；开发环境 `/api` 代理 + 生产环境 Nginx 反代到服务端 16001。

> 当前实现说明：All-in-One 镜像（`ghcr.io/acanx/finvquant`）中，前端构建产物经 `go:embed` 内嵌进 Go 服务端二进制，单进程双端口（16001 API + 16002 前端），无需独立 Nginx 容器；详见 `Prompt.md`。

## 开发规范（Docs/DevSpec/）

仓库开发规范按类别存放于 `Docs/DevSpec/`：

| 规范文件 | 类别 |
|----------|------|
| [`Docs/DevSpec/LogSpec.md`](Docs/DevSpec/LogSpec.md) | 日志与可观测性 |
| [`Docs/DevSpec/ApiSpec.md`](Docs/DevSpec/ApiSpec.md) | API 规范（响应信封/异常映射） |
| [`Docs/DevSpec/ErrorCodeSpec.md`](Docs/DevSpec/ErrorCodeSpec.md) | 错误码规范（号段/目录） |
| [`Docs/DevSpec/FileNamingSpec.md`](Docs/DevSpec/FileNamingSpec.md) | 文件命名规范 |
| [`Docs/DevSpec/DocSpec.md`](Docs/DevSpec/DocSpec.md) | 文档规范（含变更日志 `Docs/CHANGELOG.md` 强制要求） |
| [`Docs/DevSpec/GitSpec.md`](Docs/DevSpec/GitSpec.md) | Git 提交规范（含变更日志联动） |
| [`Docs/DevSpec/FileEncodingSpec.md`](Docs/DevSpec/FileEncodingSpec.md) | 文件编码规范 |

## 工作约定

- **数据库表的存在性与归属不得擅自变更（红线）**：严禁自行执行 `DROP TABLE` / `DROP SCHEMA` / `ALTER TABLE ... SET SCHEMA` / `RENAME` / `TRUNCATE`，严禁**合并或废弃同名表**（同名表通常是刻意的隔离设计，不是冗余）。
  - 唯一授权来源：用户在会话中**明确点名**要删除/迁移/合并某张表，或 [`Docs/RequirementList.md`](Docs/RequirementList.md) 中存在对应条目。**用户沉默、代码无引用、数据看起来重复、旧改造清单写着已完成——都不算授权。**
  - 受保护清单与完整规则见 [`Docs/DevSpec/DbSchemaSpec.md`](Docs/DevSpec/DbSchemaSpec.md) **§0（红线）**：含禁止操作表、同名表保护名单（`quote.` / `dedup.` / `export.` 三份 `finv_quote_secu_kline_min` 必须并存）、文档冲突仲裁顺序、改库前自检清单、删除数据的备份要求。
  - 背景事故：`V100038` 曾以“按 API 路径归 schema”为由把 `quote.finv_quote_secu_kline_min` 迁入 `dedup`，导致回测与去重/导出链路共用一张表、隔离失效，且同一天已验收的改造清单未作废，两份文档互相矛盾（后由 `V100076` 恢复）。**动手前先查 `DbSchemaSpec` §0。**
- **进入 `Archive/VeritasQuant/`（历史归档子项目）后，以该目录内的 [`Archive/VeritasQuant/AGENTS.md`](Archive/VeritasQuant/AGENTS.md) 为项目级唯一权威协作指南**——它包含"方案优先"原则、不可违反的事件语义、风控/控制约束、Git 工作流与提交规范。
- **所有 Python 相关的开发、测试、构建、运行和部署命令统一使用 `python3` 执行**；当前默认开发、测试、运行和部署环境为 **Windows 11 上的 Python 3.13**。
- Go 服务端（`cmd/`、`internal/`）与前端（`Web/`）的开发约定见 `Prompt.md` 与各自代码注释。
- 仓库级本文件只负责结构说明；现行项目级规则以本文件与 `Docs/` 下规范为准，归档子项目的规则仅作追溯。
- 根目录文件（`.gitignore`、`AGENTS.md`、`LICENSE`、`README.md`）描述仓库外壳，不描述项目内部行为。
- **新增需求必须登记**：在 [`Docs/RequirementList.md`](Docs/RequirementList.md) 追加条目（五位数编号，自 `10001` 起只增不改，一条只描述一个要求并附可执行的核对方式），并按 [`Docs/DevSpec/DocSpec.md`](Docs/DevSpec/DocSpec.md) §4 同步更新 [`Docs/CHANGELOG.md`](Docs/CHANGELOG.md)。
