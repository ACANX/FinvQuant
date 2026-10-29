# FinvQuant

**量化策略交易平台** v0.1.0 — 前后端分离架构，支持通用量化回测全链路。

- **服务端**：Go 1.25.3 + Gin + PostgreSQL 18 + Redis 8（端口 **16001**）
- **前端**：Vue 3 + Vite 8 + Vuetify 4（端口 **16002**）
- **部署**：Docker Compose / 单容器 All-in-One 镜像（GHCR）
- **分支**：`dev`（开发分支，默认）

---

## 功能概览

| 模块 | 说明 | 状态 |
|------|------|------|
| 历史行情导入 | MVSV 分钟行情上传导入（双列布局自动识别 + 字段级覆盖 upsert） | ✅ |
| 历史行情查询 | K 线蜡烛图查询（黑金主题、三行图例显隐、dataZoom 拖拽缩放） | ✅ |
| 元数据管理 | 交易所 / 市场 / 证券字典维护（全量种子数据开箱即用） | ✅ |
| 通用量化回测 | 策略 / 账户 / 任务 / 报告 / 资金持仓 / 链路追踪 | ✅ |
| 环境管理 | 回测 / 模拟盘 / 仿真 / 实盘环境配置（交易时段、规则、成本） | ✅ |
| 模板管理 | 策略 / 账户 / 环境模板（内置 + 自定义） | ✅ |
| 链路追踪⑨ | 资金流水明细 / 持仓变化明细 / 事件追踪（触发原因·成交结果·委托耗时） | ✅ |
| 多用户隔离 | 策略 / 账户 / 任务 / 环境 / 模板按 `user_id` 隔离 | ✅ |
| 单用户多账户 | 账户支持 `group_id` 分组（主/子账户） | ✅ |
| 实盘链路 | 行情、账户、策略、订单、风控 API 规划中 | 🚧 |

---

## 快速开始（Docker）

```bash
# 1. 克隆代码（默认分支 dev）
git clone https://github.com/ACANX/FinvQuant.git
cd FinvQuant

# 2. 准备环境变量并启动
cp Deploy/.env.example Deploy/.env   # 按需修改
docker compose --env-file Deploy/.env up -d
```

- 前端控制台：http://localhost:16002
- 服务端 API：http://localhost:16001/API/V1/Health/Live

或直接使用 All-in-One 镜像（单容器 = 服务端 + 前端，无需独立 Nginx）：

```bash
docker pull ghcr.io/acanx/finvquant:latest
docker run -d --name finvquant -p 16001:16001 -p 16002:16002 \
  -e FINV_PG_HOST=host.docker.internal -e FINV_REDIS_ADDR=host.docker.internal:6379 \
  ghcr.io/acanx/finvquant:latest
```

> **镜像 tag 规则**（CI 自动打，版本单一来源 = 项目根 `VERSION` 文件，后端 -ldflags / 前端 package.json / 镜像 tag 三处共用）：
> - 分支推送（dev/main/FinvQuant）：`latest` + `v{VERSION}-YYYYMMDDHHMM`（Asia/Shanghai）
> - `v*` git tag 推送（正式发布）：`latest` + `v{VERSION}`（如 `v0.1.0`，amd64 + arm64 全架构）

---

## 本地开发

```bash
# 服务端（Go 1.25.3，需本地 PostgreSQL 18 + Redis 8）
go run ./cmd/server

# 前端（端口 16002，/api 代理到 16001）
cd Web && npm install && npm run dev
```

---

## 业务菜单（Web 控制台）

| 菜单 | 路由 | 说明 |
|------|------|------|
| 仪表盘 | /dashboard | 系统状态总览 |
| 历史行情查询 / 导入 | /Meta/Finv/Quote/History/* | K 线查询、MVSV 导入 |
| 元数据管理 | /meta/exchange 等 | 交易所 / 市场 / 证券字典维护 |
| 配置 / 环境 / 模板管理 | /Meta/Finv/Quant/{Config,Environment,Template} | 回测环境与模板 |
| 账户 / 资金 / 持仓管理 | /Meta/Finv/Quant/{Account,Fund,Position} | 回测账户与结果查看 |
| 黄金期货合约回测验证 | /Meta/Finv/Quant/Backtest/GoldFutures | 回测条件配置与启动 |
| 策略管理 | /Meta/Finv/Quant/Strategy | 结构化策略定义 |
| 回测分析 | /Meta/Finv/Quant/Backtest/Analysis | 报告指标卡 + 曲线 + ⑨链路追踪 + 导出 |
| 仿真/模拟盘/实盘（占位） | /Meta/Finv/Quant/Simulation/* 等 | 规划中，环境类型已建模 |

> 菜单/路由统一 `Meta/Finv/Quant/` 前缀 + 大驼峰（PascalCase），与后端 API 路径同前缀体系；完整菜单文档见 [Docs/Menu/Menus.md](Docs/Menu/Menus.md)。

---

## 目录结构

```
├── cmd/server/              # Go 服务端入口（端口 16001）
├── internal/                # Go 服务端内部模块
│   ├── api/                 #   Gin 路由与 HTTP 处理器
│   │   ├── router.go        #   路由注册（34+ 端点）
│   │   └── handler/         #   各领域处理器
│   ├── backtest/            #   量化回测模块（引擎/服务/指标/表达式/模型）
│   ├── config/              #   环境变量配置加载（FINV_* 前缀）
│   ├── database/            #   PostgreSQL 连接池（pgx/v5）+ 自动迁移
│   ├── meta/                #   元数据管理服务（交易所/市场/证券）
│   ├── mvsv/                #   MVSV 行情格式解析器
│   ├── quote/               #   历史行情导入/查询服务
│   ├── redisclient/         #   Redis 8 客户端
│   ├── static/              #   静态资源服务
│   └── webui/               #   go:embed 内嵌前端构建产物
├── Web/                     # 前端（Vue 3 + Vite 8 + Vuetify 4）
│   ├── src/                 #   源码
│   │   ├── views/           #     页面组件（views/Meta/Finv 按业务分层）
│   │   ├── App.vue          #     根组件
│   │   ├── router.ts        #     路由定义
│   │   └── api.ts           #     API 客户端（统一 /API/V1 前缀）
│   └── vite.config.ts       #   Vite 配置（/api 代理）
├── Deploy/                  # 部署编排
│   ├── docker-compose.yml   #   Compose 一键部署（含 PG18 + Redis 8）
│   ├── .env.example         #   环境变量模板
│   ├── Migrations/          #   数据库迁移脚本（50+ 个迁移）
│   ├── deploy.cmd           #   一键部署脚本
│   ├── upgrade.cmd          #   增量升级脚本
│   ├── rollback.cmd         #   回滚脚本
│   ├── DeployUpgradeGuide.md#   部署/升级/回滚脚本使用手册
│   ├── Win11DockerDeploy.md #   Windows 11 部署指南
│   └── Win11DockerUpgrade.md#   Windows 11 增量升级指南
├── Docs/                    # 文档
│   ├── API/                 #   服务端 API 接口文档（一接口一文档，34 个端点）
│   ├── Asset/Backtest/      #   回测架构设计图集（SVG）
│   ├── DataDictMapping/     #   数据字典映射文档
│   ├── DataFormat/          #   MVSV 行情格式说明（含 NVDA/GCMain 示例文件）
│   ├── DevSpec/             #   开发规范（ApiSpec/MenuSpec/BacktestStrategySpec/UiSpec/LogSpec/ErrorCodeSpec 等 10 篇）
│   ├── GitHubActionUpgrade.md # CI/CD 升级指南
│   └── Menu/                #   前端菜单文档（一菜单一文档）
├── Dockerfile               # 多阶段构建镜像（前端 + 后端）
├── .github/workflows/CI.yml # CI/CD：构建 + 测试 + 推送 GHCR 镜像
├── go.mod / go.sum          # Go 模块定义（Go 1.25.3）
├── VERSION                  # 版本文件（当前 0.1.0）
├── Prompt.md                # 结构化需求文档（持续更新）
├── VeritasQuant/            # 既有子项目（Python 量化平台，历史保留）
└── README.md                # 本文件
```

---

## 服务端 API

基路径：`/API/V1`（统一大写），端口 **16001**

### 健康检查

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/API/V1/Health/Live` | 存活检查 |
| GET | `/API/V1/Health/Ready` | 就绪检查 |
| GET | `/API/V1/Version` | 版本信息 |

### 历史行情

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/API/V1/Meta/Finv/Quant/Quote/Import/Upload` | MVSV 分钟行情上传导入 |
| GET | `/API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery` | 历史行情查询（K 线，ts 范围返回全部记录） |

### 元数据管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/API/V1/Meta/Finv/Quant/Metadata/Exchange/*` | 交易所信息维护 |
| GET/POST | `/API/V1/Meta/Finv/Quant/Metadata/Market/*` | 市场信息维护 |
| GET/POST | `/API/V1/Meta/Finv/Quant/Metadata/Security/*` | 证券信息维护（含 Options/Lookup） |

### 量化回测

路径前缀：`/API/V1/Meta/Finv/Quant/Backtest/`（URL 查询参数统一小驼峰 camelCase）

**策略管理**：`Strategy/List`, `Get`, `Save`, `Toggle`, `Delete`
**账户管理**：`Account/List`, `Get`, `Save`, `Toggle`, `Delete`
**任务管理**：`Run/Create`, `List`, `Get`, `Cancel`, `Delete`
**报告分析**：`Run/Report`, `Equity`, `Trades`
**链路追踪**：`Run/Cashflows`, `PositionLogs`, `EventTraces`
**删除任务**：`Run/DeleteTask/List`, `DeleteTask/Logs`, `DeleteTask/Retry`, `DeleteTask/Archives`
**环境管理**：`Environment/List`, `Get`, `Save`, `Toggle`, `Delete`
**模板管理**：`Template/List`, `Get`, `Save`, `Delete`

> 详细接口文档见：`Docs/API/README.md` 与 [Docs/API/APIs.md](Docs/API/APIs.md)（#1~45 索引）。

---

## 量化回测模块

### 能力矩阵

| 功能 | 说明 |
|------|------|
| 策略定义 | JSON 模型 v1（universe / data / indicators / signals / rules / risk / cost），保存时编译校验 |
| 指标计算 | MA / EMA / RSI / MACD / BOLL / ATR / STDDEV / HHV / LLV |
| 信号表达式 | 自研引擎（比较/逻辑/算术 + cross_up/down / ref / highest/lowest / abs），深度上限 64 |
| 无未来函数 | 信号 bar 收盘确认、次根开盘成交（NEXT_BAR_OPEN）；ref 负偏移编译期拦截 + 标识符交叉校验 |
| 回测引擎 | 逐 bar 回放（预热 → 挂单撮合 NEXT_BAR_OPEN → 止损止盈 intrabar → 信号 → 规则限制 → 账户更新 → 报告点） |
| 报告生成 | 8+2 项：余额/收益率/收益额/持仓金额曲线 + 最大投入/平均投入/到期收益率/年化/最大回撤（含区间）/夏普/波动率/胜率/盈亏比/信号归因 |
| 链路追踪⑨ | 资金流水明细（连续可校验）/ 持仓变化明细（OPEN/ADD/REDUCE/CLOSE）/ 事件追踪（8 项登记：触发原因·时间·成交结果·委托下单·成交耗时·存活时间·未成交原因） |
| 环境自适应 | 交易时段过滤（含跨午夜）、tick_size 对齐、T+N/涨跌停/合约乘数、撮合模式、币种校验、成本覆盖链（环境 > 任务 > 策略 > 账户） |
| 多用户隔离 | 策略/账户/任务/环境/模板按 `user_id` 隔离，所有 List/Get/Toggle/Delete/CreateRun 均做归属校验 |
| 异步调度 | 并发上限 4、进度/状态持久化、支持取消、重启悬挂自动标记 FAILED |
| 任务删除 | 异步删除 + 归档留痕（"曾经存在的证明"）+ 删除审计日志 |
| 内置模板 | 双均线 / RSI / 布林带 / MACD 策略模板 + 默认环境模板（GCMain 黄金期货 / 沪深 ETF） |

### 数据库迁移

- 迁移文件：`Deploy/Migrations/`（50+ 个迁移，启动时自动应用）
- 分段约定：`V1~V99999` 为表结构/变更脚本（DDL），`V100000+` 为数据种子脚本（DML）
- 表名规范：业务表统一 `finv_` 前缀，行情表 `finv_quote_xxx`，回测表 `finv_quant_xxx`

---

## CI/CD

- 触发器：push（dev/main/FinvQuant）、tag `v*`、PR、手动
- 流程：
  1. **build-server**：Go vet + build + test（Go 1.25.3）
  2. **build-web**：npm ci + build（Node 24，构建时从 VERSION 注入 package.json）
  3. **docker**（非 PR）：构建并推送 All-in-One 镜像 `ghcr.io/acanx/finvquant`（latest + 版本 tag，多平台 amd64/arm64 + 平台专属 tag）
  4. **docker-pr**（PR）：仅构建不推送，验证镜像可构建

---

## 端口约定

| 服务 | 端口 |
|------|------|
| Go 服务端 | **16001** |
| 前端 Web | **16002** |
| PostgreSQL（宿主映射） | 5433 |
| Redis（宿主映射） | 6380 |

---

## 版本历史

- **v0.1.0**（2026-08-08）：通用量化回测引擎 + 黄金期货合约回测验证（PR #338/#339）、MVSV 双布局解析、元数据字典全量种子、多平台 CI 发布、一键部署/升级/回滚脚本

---

## 文档

- [Prompt.md](Prompt.md) — 项目需求与技术基线（结构化，持续更新）
- [Docs/API/README.md](Docs/API/README.md) + [Docs/API/APIs.md](Docs/API/APIs.md) — 服务端 API 接口文档（一接口一文档）
- [Docs/Menu/Menus.md](Docs/Menu/Menus.md) — 前端菜单文档（一菜单一文档）
- [Docs/DevSpec/](Docs/DevSpec/) — 开发规范（ApiSpec / MenuSpec / BacktestStrategySpec / UiSpec / LogSpec / ErrorCodeSpec / DocSpec / FileNamingSpec / FileEncodingSpec / GitSpec）
- [Docs/DataFormat/MvsvFileFormat.md](Docs/DataFormat/MvsvFileFormat.md) — MVSV 行情文件格式规范（含 NVDA/GCMain 示例文件）
- [Docs/DataDictMapping/](Docs/DataDictMapping/) — 数据字典与映射说明
- [Docs/Asset/Backtest/README.md](Docs/Asset/Backtest/README.md) — 回测架构设计图集（SVG）
- [Deploy/DeployUpgradeGuide.md](Deploy/DeployUpgradeGuide.md) — 部署/升级/回滚脚本使用手册
- [Deploy/Win11DockerDeploy.md](Deploy/Win11DockerDeploy.md) — Windows 11 Docker 部署文档
- [Deploy/Win11DockerUpgrade.md](Deploy/Win11DockerUpgrade.md) — Windows 11 Docker 增量升级文档
- [VeritasQuant/README.md](VeritasQuant/README.md) — 既有 Python 子项目说明

---

## 许可

[MIT](LICENSE) © 2026 ACANX
