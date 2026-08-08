# FinvQuant 智能体协作指南

## 适用范围

本文件适用于整个仓库（FinvQuant 量化策略交易平台）。

## 仓库结构

本仓库为"外壳 + 子项目"结构：

```
.
├── VeritasQuant/          # 子项目：VeritasQuant 量化交易平台（完整代码）
├── cmd/ internal/         # FinvQuant Go 服务端（量化平台服务端）
├── Web/                   # 前端（Vue3 + Vite8 + Vuetify4）
├── Deploy/                # Docker Compose 部署编排
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
| [`Docs/DevSpec/DocSpec.md`](Docs/DevSpec/DocSpec.md) | 文档规范 |
| [`Docs/DevSpec/GitSpec.md`](Docs/DevSpec/GitSpec.md) | Git 提交规范 |
| [`Docs/DevSpec/FileEncodingSpec.md`](Docs/DevSpec/FileEncodingSpec.md) | 文件编码规范 |

## 工作约定

- **进入 `VeritasQuant/` 后，以该目录内的 [`VeritasQuant/AGENTS.md`](VeritasQuant/AGENTS.md) 为项目级唯一权威协作指南**——它包含"方案优先"原则、不可违反的事件语义、风控/控制约束、Git 工作流与提交规范。
- **所有 Python 相关的开发、测试、构建、运行和部署命令统一使用 `python3` 执行**；当前默认开发、测试、运行和部署环境为 **Windows 11 上的 Python 3.13**。
- Go 服务端（`cmd/`、`internal/`）与前端（`Web/`）的开发约定见 `Prompt.md` 与各自代码注释。
- 仓库级本文件只负责结构说明；项目级规则一律以 `VeritasQuant/AGENTS.md` 为准。
- 根目录文件（`.gitignore`、`AGENTS.md`、`LICENSE`、`README.md`）描述仓库外壳，不描述项目内部行为。
