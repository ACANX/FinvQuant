# FinvQuant 项目需求文档（Prompt）

> 本文档是 FinvQuant 仓库（量化策略交易平台）的结构化需求说明，**支持后续持续更新与补充完善**。
> 更新方式：直接编辑本文件，保持分节结构，在对应章节追加或修订内容；重大变更请在文末"变更记录"登记。

## 1. 项目定位

FinvQuant 是一个**量化策略交易平台**，采用**前后端分离**架构：
- 服务端：Go 语言（Gin），提供量化策略与交易相关 API；
- 前端：Web 控制台（Vue3），提供策略管理、行情与交易界面。

## 2. 仓库结构

```
.
├── cmd/server/            # Go 服务端入口（端口 16001）
├── internal/              # Go 服务端内部模块
│   ├── api/               #   Gin 路由与处理器
│   ├── config/            #   配置加载（环境变量）
│   ├── database/          #   PostgreSQL 18 连接（pgx/v5）
│   └── redisclient/       #   Redis 8 连接（go-redis/v9）
├── Web/                   # 前端（Vue3 + Vite8 + Vuetify4，端口 16002）
├── Deploy/                # Docker Compose 部署编排
├── .github/workflows/     # GitHub Actions：构建 + 推送 GHCR 镜像
├── Dockerfile             # 服务端镜像（多阶段构建）
├── VeritasQuant/          # 既有子项目（Python 量化平台，历史保留）
├── go.mod / go.sum        # Go 模块定义
├── .gitignore             # Go 版本忽略规则
└── Prompt.md              # 本需求文档
```

## 3. 技术选型（当前基线）

| 层 | 技术 | 版本 | 说明 |
|----|------|------|------|
| 服务端语言 | Go | **1.25.3** | 平台服务端唯一语言 |
| Web 框架 | Gin | **最新**（v1.12.x） | HTTP API 框架 |
| 数据库 | PostgreSQL | **18** | 业务主库（pgx/v5 驱动） |
| 缓存/消息 | Redis | **8** | go-redis/v9 客户端 |
| 前端框架 | Vue | 3.x | Composition API |
| 构建工具 | Vite | **8.x** | 前端构建/开发服务器 |
| UI 组件库 | Vuetify | **4.x** | Material Design 组件 |
| 包管理器 | npm | 最新 | 前端依赖管理 |
| Go 模块代理 | goproxy.cn | — | 国内构建加速（可选） |

> 版本策略：依赖采用"最新稳定版"；升级时同步更新本文档与 `go.mod`/`package.json`。

## 4. 服务端（Go）设计

### 4.1 端口与启动
- 默认监听端口：**16001**
- 配置方式：环境变量（`FINV_*` 前缀），见 `internal/config/config.go`
- 健康检查：`GET /API/V1/Health/Live`、`GET /API/V1/Health/Ready`
- 版本信息：`GET /API/V1/Version`

### 4.2 模块划分（internal）
- `api`：Gin 路由注册、HTTP 处理器
- `config`：环境变量配置加载
- `database`：PostgreSQL 18 连接池（pgxpool + Ping 探活）
- `redisclient`：Redis 8 客户端（Ping 探活）

### 4.3 依赖基线
- `github.com/gin-gonic/gin` v1.12.x
- `github.com/redis/go-redis/v9` v9.22.x
- `github.com/jackc/pgx/v5` v5.x

## 5. 前端（Web）设计

- 默认端口：**16002**
- 技术栈：Vue 3.5 + Vite 8.2 + Vuetify 4.1（TypeScript）
- 目录：`Web/`（vite 标准结构）
- 开发代理：`/api` → `http://localhost:16001`（见 `Web/vite.config.ts`）
- 生产部署：Nginx 静态托管 + `/api` 反向代理到服务端 16001（见 `Web/nginx.conf`）

## 6. 部署与镜像

### 6.1 All-in-One 镜像（GitHub Packages / GHCR）

**单镜像 `ghcr.io/acanx/finvquant`**：一个容器同时提供服务端与前端（Go 内嵌前端构建产物，单进程双端口）。

| 端口 | 服务 |
|------|------|
| 16001 | Go 服务端 API（Gin） |
| 16002 | 前端 Web（内嵌静态资源，SPA fallback） |

> 设计说明：前端静态文件通过 `go:embed` 内嵌进 Go 二进制，无需独立 Nginx 镜像，拉取**一个镜像**即可完整部署。

### 6.2 本地 Docker 部署

```bash
# 方式一：Compose 一键（含 PG18 / Redis8）
cp Deploy/.env.example Deploy/.env   # 按需修改
docker compose --env-file Deploy/.env up -d

# 方式二：直接拉取 All-in-One 镜像运行
docker pull ghcr.io/acanx/finvquant:latest
docker run -d --name finvquant -p 16001:16001 -p 16002:16002 \
  -e FINV_PG_HOST=host.docker.internal -e FINV_REDIS_ADDR=host.docker.internal:6379 \
  ghcr.io/acanx/finvquant:latest
```

### 6.3 依赖服务（Compose 内置，独立镜像）
- `postgres`：`postgres:18-alpine`（宿主映射 5433）
- `redis`：`redis:8-alpine`（宿主映射 6380）

### 6.4 数据持久化（宿主机映射）

**PostgreSQL 数据目录映射到 Docker 宿主机文件系统**（容器重建/删除不丢数据），通过 `FINV_PG_DATA_DIR` 配置：

| 平台 | 示例 |
|------|------|
| Windows 11 | `D:\Dev\Docker\HostFileSystem\FinvQuant\PostgreSQL` |
| Linux/macOS | `/data/finvquant/postgresql` |

```dotenv
# Deploy/.env
FINV_PG_DATA_DIR=D:/Dev/Docker/HostFileSystem/FinvQuant/PostgreSQL
```

未设置时默认使用 `Deploy/pgdata`（Compose 项目相对目录）。

> ⚠️ PG18+ 镜像数据目录变更：挂载点为 `/var/lib/postgresql`（单一挂载），实际数据在宿主目录下的 `18/` 子目录；挂载 `/var/lib/postgresql/data` 会被镜像判定为 unused mount 并拒绝启动。

## 7. CI/CD（GitHub Actions）

- 文件：`.github/workflows/build-publish.yml`
- 触发：push（dev/main/FinvQuant）、tag `v*`、PR、手动
- 流程：
  1. `build-server`：Go vet + build + test（Go 1.25.3）
  2. `build-web`：npm ci + build（Node 24）
  3. `docker`（非 PR）：构建并推送 **All-in-One 镜像** `ghcr.io/acanx/finvquant`（latest + 版本 tag）
  4. `docker-pr`（PR）：仅构建不推送，验证镜像可构建

## 7.5 数据库建表规范

- 所有业务表名必须以 **`finv_` 作为前缀**。
- 行情模块表名统一为 **`finv_quote_xxx`**（例如 `finv_quote_secu_kline_min`、`finv_quote_ingest_batches`、`finv_quote_revision_log`）。
- 迁移文件存放于 `Deploy/Migrations/`，命名 `V<number>__<name>.sql`，服务端启动时自动应用。
- **迁移分段约定**：`V1~V99999` 为表结构/变更脚本（DDL），`V100000+` 为数据种子脚本（DML）；初始数据必须放在数据种子段，确保在所有表结构脚本执行完后再执行。

## 8. 端口约定（汇总）

| 服务 | 端口 |
|------|------|
| Go 服务端 | **16001** |
| 前端 Web | **16002** |
| PostgreSQL（宿主映射） | 5433 |
| Redis（宿主映射） | 6380 |

## 9. 待办 / 规划（Roadmap）

- [x] 历史行情导入：PG 建表（V1 迁移启动自动执行）、`POST /API/V1/Quote/Import/Upload` 上传导入（支持备注字段）、前端「历史行情数据导入」菜单页
- [x] 历史行情查询：`GET /API/V1/Quote/Query`（secu_code/date/period=Min）、前端「历史行情查询」菜单页（K 线蜡烛图、红涨绿跌、悬停详情）
- [x] 通用量化回测：策略/账户/任务/报告全链路（见下）
- [ ] 服务端业务模块：行情、账户、策略、订单、风控 API（实盘链路）
- [ ] 前端业务页面：行情看板、交易面板（模拟盘/实盘）
- [ ] 数据库迁移与 Schema 管理（golang-migrate / goose）
- [ ] Redis 缓存策略与消息通道接入
- [ ] 认证授权（JWT / RBAC）
- [ ] 与 `VeritasQuant/` 子项目的数据/能力集成（行情导入 PG 等）

## 9.5 通用量化回测模块（2026-08-06 已落地）

### 能力矩阵

| 模块 | 说明 | 状态 |
|------|------|------|
| 量化策略验证 | 顶级菜单；含「黄金期货合约回测验证」（GCMain 等标的配置+启动+报告） | ✅ 已落地 |
| 账户管理 | 回测账户（初始资金/手续费/滑点/保证金模式），CRUD + 回测开关 | ✅ 已落地 |
| 策略管理 | 结构化策略定义（JSON 模型 v1），内置模板（双均线/RSI/布林带/MACD） | ✅ 已落地 |
| 回测分析 | 回测任务列表 + 收益分析报告（指标卡 + 四类曲线 + 成交记录） | ✅ 已落地 |
| 资金管理 | 回测任务资金曲线（现金/总资产）查看 | ✅ 已落地 |
| 持仓管理 | 回测任务持仓曲线 + 开平仓记录查看 | ✅ 已落地 |
| 仿真数据验证 | 数据校验/比对（规划中） | 🚧 规划 |
| 模拟盘验证 | 虚拟资金模拟盘（规划中） | 🚧 规划 |
| 实盘仿真验证 | 实盘行情流+仿真撮合（规划中） | 🚧 规划 |
| 实盘交易 | 真实经纪商接入（规划中，仿真验证通过后开放） | 🚧 规划 |

### 服务端（internal/backtest）

- 策略定义模型 `StrategyDefinition`（universe/data/indicators/signals/rules/risk/cost），JSONB 持久化 + 保存时编译校验；
- 指标计算（MA/EMA/RSI/MACD/BOLL/ATR/STDDEV/HHV/LLV）；
- 信号表达式引擎（自研，支持比较/逻辑/算术 + cross_up/cross_down/ref/highest/lowest/abs）；
- 回测引擎：逐 bar 回放（预热 → 挂单撮合（NEXT_BAR_OPEN）→ 止损止盈 → 信号 → 规则限制 → 账户更新 → 报告点）；
- 报告：余额/收益率/收益额/持仓金额曲线（按报告精度）+ 最大投入/平均投入/到期收益率/最大回撤/夏普/胜率/盈亏比等技术指标；
- **链路追踪（需求⑨）**：资金流水明细（初始注入/买入付款/卖出收款/手续费/保证金占用释放）、持仓变化明细（开仓/加仓/减仓/平仓 + 成本变化）、交易事件追踪（触发原因/成交结果 FILLED·REJECTED·EXPIRED/委托耗时 bar·秒/未成交原因分类统计）；
- **环境/模板（自适应）**：环境模型（回测/模拟盘/仿真/实盘类型 × 地区/市场），配置含交易时段、交易规则（T+N/涨跌停/合约乘数/tick_size）、成本基准、撮合模式；模板模型（策略/账户/环境三类，内置+自定义）；回测任务保存环境快照；引擎自适应环境交易时段与成本规则；
- **多用户/多子账户**：策略/账户/任务均带 `user_id`（多用户隔离，默认 default），账户支持 `group_id` 分组（单用户多子账户），组合回测在 universe 多证券上预留（后续版本实现组合净值）；
- 异步任务调度：并发上限 4、进度/状态持久化、可取消；
- 数据库：V22~V28（策略/账户/任务/净值曲线/成交记录/资金流水/持仓变化/事件追踪/环境/模板）+ V100019/V100020 种子数据（默认账户 + GCMain 双均线示例策略 + 默认环境与内置模板）。
- **表名规范**：量化回测模块统一前缀 `finv_quant_`（如 `finv_quant_backtest_strategy` / `finv_quant_backtest_run` / `finv_quant_environment` / `finv_quant_template`）。

### API（/API/V1/Meta/FinvQuant/Backtest/**）

- 策略：`Strategy/List|Get|Save|Toggle|Delete`
- 账户：`Account/List|Get|Save|Toggle|Delete`
- 任务：`Run/Create|List|Get|Cancel`、报告 `Run/Report`、曲线 `Run/Equity`、成交 `Run/Trades`、链路追踪 `Run/Cashflows|PositionLogs|EventTraces`
- 环境：`Environment/List|Get|Save|Toggle|Delete`
- 模板：`Template/List|Get|Save|Delete`

### 规范文档

- 策略定义模型与表达式语法：`Docs/DevSpec/BacktestStrategySpec.md`

## 9.6 评审意见与新需求（2026-08-06 第二轮）

### 9.6.1 评审意见落实（ACANX 评论 issuecomment-5197476192）

1. **⑨ 持仓变动详细情况链路追踪分析**：资金流水明细 / 持仓变化明细 / 交易事件结果追踪（事件触发原因、成交结果与否、委托耗时、未能成交的原因）—— 已落地（见 9.5 链路追踪 + 迁移 V27）；
2. **表名命名规则**：量化回测模块统一 `finv_quant_` 前缀（示例 `finv_quant_backtest_strategy`）—— 已落地（V22~V27 迁移与代码同步改名）。

### 9.6.2 新增需求（2026-08-06 第三轮）

1. **多用户回测运行**：系统设计支持多用户并发回测（策略/账户/任务均按 `user_id` 隔离与过滤；当前无认证系统，`user_id` 先以字符串字段承载，接入 JWT/RBAC 后与登录态绑定）；
2. **单用户多子账户**：一个用户可拥有多个账户（主/子账户），账户支持 `group_id` 分组归属（主账户 = group 根，子账户通过 group_id 关联），回测任务绑定具体账户；
3. **投资组合回测预留**：当前阶段不实现组合回测；策略定义 `universe.securities` 已支持多标的声明，组合净值/多标的撮合在后续版本实现（模型与字段预留，不加约束）；
4. **环境/模板模型**：
   - **环境（Environment）**：回测 / 模拟盘 / 仿真 / 实盘交易环境的配置差异（撮合模式、成本、交易时段、交易规则）；不同市场（如 COMEX 黄金 vs 沪深 ETF）的交易约束、交易规则、地区习惯偏好差异；
   - **模板（Template）**：策略模板 / 账户模板 / 环境模板，相同部分（环境、约束、规则、限制、策略）复用，差异部分支持自定义配置；
   - **自适应与动态切换**：回测任务创建时指定环境（`env_id`）并保存环境快照；引擎自适应环境配置（交易时段过滤、tick_size/价格数量精度、成本覆盖：环境 > 任务 > 策略 > 账户）；前端环境管理页 + 回测配置页支持环境选择与切换；
5. **API 路径规范**：后端 `/API/V1/Backtest/**` → `/API/V1/Meta/FinvQuant/Backtest/**`；前端菜单/路由统一加 `Meta/FinvQuant/` 前缀；受影响代码与文档同步替换。

## 10. 变更记录

| 日期 | 变更 | 说明 |
|------|------|------|
| 2026-08-04 | 初始版本 | 初始化 Go 服务端（Gin 最新 / PG18 / Redis8 / Go 1.25.3）+ Vue3+Vite8+Vuetify4 前端；端口 16001/16002；GHCR 镜像构建与 Docker Compose 部署；.gitignore 改 Go 版 |
| 2026-08-04 | All-in-One 镜像 | 合并 server/web 双镜像为单镜像 `ghcr.io/acanx/finvquant`：前端经 `go:embed` 内嵌进 Go 二进制，单进程双端口（16001 API + 16002 前端），拉取一个镜像即可完整部署 |
| 2026-08-04 | 目录与持久化 | `deploy/` 重命名为 `Deploy/`；PG 数据目录支持映射到 Docker 宿主机文件系统（`FINV_PG_DATA_DIR`，Windows 示例 `D:\Dev\Docker\HostFileSystem\FinvQuant\PostgreSQL`） |
| 2026-08-04 | API 路径大写 | API 路径统一 `/API/V1/` 前缀（写入 ApiSpec 规范） |
| 2026-08-04 | 历史行情导入 | PG 建表（`Deploy/Migrations/V1__finv_quote_secu_kline_min.sql` 启动自动迁移）；Go MVSV-1 解析器 + 字段级覆盖 upsert 导入服务；`POST /API/V1/Quote/Import/Upload`；前端新增「历史行情数据导入」菜单页（批量上传 MVSV 分钟行情） |
| 2026-08-04 | 规范与文档 | 建表规范：`finv_` 前缀、行情表 `finv_quote_xxx`；迁移重命名 `Deploy/Migrations/V1__finv_quote_secu_kline_min.sql`（表名修正 `finv_quote_ingest_batches`/`finv_quote_revision_log`）；新增 `Docs/API/` 服务端接口文档（含 /Quote/Import/Upload 端点） |
| 2026-08-06 | 通用量化回测 | 前端新增顶级菜单「量化策略验证/账户管理/资金管理/持仓管理/策略管理/回测分析/仿真数据验证/模拟盘验证/实盘仿真验证/实盘交易」；「量化策略验证 → 黄金期货合约回测验证」支持配置初始资金/策略/限制/回测开关并启动回测、查看报告。服务端新增 `internal/backtest`（结构化策略定义模型+指标+信号表达式引擎+回测引擎+报告生成+异步任务调度），迁移 V22~V26 + V100019 种子（默认账户 + GCMain 双均线示例策略）；API `/API/V1/Backtest/**`（策略/账户/任务/报告/曲线/成交）；规范文档 `Docs/DevSpec/BacktestStrategySpec.md` |
| 2026-08-06 | 回测链路追踪+命名规范 | 按评审意见：① 表名统一 `finv_quant_` 前缀（V22~V26 迁移与代码同步改名）；② 新增需求⑨链路追踪——迁移 V27（资金流水 `finv_quant_backtest_cashflow`/持仓变化 `finv_quant_backtest_position_log`/事件追踪 `finv_quant_backtest_event_trace`），引擎登记触发原因/成交结果/委托耗时/未成交原因，报告新增 `event_stats` 统计，前端回测分析页新增 ⑨ 链路追踪统计与三张明细表；API 新增 `Run/Cashflows|PositionLogs|EventTraces` |
| 2026-08-06 | 多用户/环境模板/API路径 | 按新增需求：① 多用户回测（策略/账户/任务加 `user_id`）+ 单用户多子账户（账户 `group_id` 分组）+ 组合回测预留（universe 多标的）；② 环境/模板模型——迁移 V28（`finv_quant_environment` 环境表 + `finv_quant_template` 模板表）+ V100020 种子，环境配置含交易时段/交易规则/成本基准/撮合模式，引擎自适应环境（时段过滤、tick_size 精度、成本覆盖链 环境>任务>策略>账户），任务保存环境快照，前端新增环境与模板管理页；③ API 路径 `/API/V1/Backtest/**` → `/API/V1/Meta/FinvQuant/Backtest/**`，前端菜单/路由统一加 `Meta/FinvQuant/` 前缀，代码与文档同步替换 |
| 2026-08-06 | 回测评审修复（第三轮） | 按 PR #338 审查报告逐项修复：🔴 ① `ref()` 负偏移前视漏洞（编译期常量负偏移报错 + 运行期动态负偏移返回 NaN + 表达式深度上限 64）；② 信号表达式标识符交叉校验（未声明指标保存即报错）+ buy/sell 至少一个非空。🟠 ③ 成本覆盖链统一为 环境>任务>策略>账户（抽取 `resolveCosts` + 单测）；④ 环境交易规则全部生效（T+N 交收、涨跌停、合约乘数、环境 fill_mode CURRENT_CLOSE、币种一致性校验）；⑤ 卖出数量模式 FIXED/PERCENT/AMOUNT/ALL + 方向语义校验；⑥ 多用户隔离补全（策略/账户/任务 List/Get/Toggle/Delete/CreateRun 归属校验，环境/模板同）；⑦ 事件追踪补 FR-10 ⑤委托下单时间(order_ts)与⑦存活时间(alive_sec)，V27 迁移同步加列；⑧ V26 trade remark 落值；⑨ 种子补齐布林带/MACD 策略模板 + 沪深环境模板；⑩ 文档口径修正（environment_snapshot/菜单层级/死链/关键字过滤等）。🟡 盈亏比 +Inf 哨兵 999999、平均委托耗时剔除风控事件、聚合 bar 时段判定取周期起点、Min 精度曲线降采样 2 万点、RSI 首值对齐 Wilder、重启悬挂恢复、内置模板禁改、is_default 部分唯一索引、错误码 400/404/409 映射、created_by 可传。前端：取消按钮/5s 轮询/Run-Get 深链/关键字过滤输入框/Hour 标签/成交表去双触发/模板 API 化/共享 utils/请求超时。测试新增 16 项全过；文档同步（DevSpec/API/菜单/Prompt） |
