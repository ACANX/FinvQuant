# VeritasQuant GUI 客户端使用指南

> 本文档介绍 `vq-gui`（Streamlit Web 操作台，默认 `http://127.0.0.1:8501/`）的
> 菜单页面功能、使用方法与注意事项。

## 1. 启动与访问

### 1.1 前置条件

- 服务端已启动（见 `Docker/Windows11Deployment.md`），`http://localhost:18000` 可访问；
- 客户端已安装：`python3 -m pip install -e .`（venv 内）。

### 1.2 启动

```powershell
vq-gui --api-url http://localhost:18000 --serve
```

> ⚠️ 必须加 `--serve` 才真正启动 GUI（缺省仅做离线参数校验，直接退出）；
> 参数是 `--api-url`（不是 `--api`）。

启动成功后浏览器自动打开（或手动访问 `http://127.0.0.1:8501/`）。

### 1.3 界面结构

- **侧边栏**：导航菜单（页面单选）+ 账户上下文（当前账户 + 运行模式）+ API 连接信息；
- **主区域**：当前页面内容；
- 所有 API 错误统一展示为 `错误 [<code>] HTTP <status>（可重试）: <message>`。

## 2. 菜单总览

| 菜单 | key | 功能 | 状态 |
| --- | --- | --- | --- |
| 仪表盘 | dashboard | 平台概览 | 占位（P2-033 后续实现） |
| 数据导入 | data_import | 提交行情/数据导入命令 | 表单可用；提交依赖命令 API 接线 |
| 策略管理 | strategies | 策略列表 + 新建草稿（PYTHON/DSL）+ DSL 校验 | 列表可用；保存为草稿 |
| 定投计划 | plans | 定投计划草稿创建 | 草稿校验；提交依赖命令 API 接线 |
| 账户管理 | accounts | 账户列表 + 账户详情 | ✅ 可用 |
| 回测中心 | backtests | 回测列表 + 启动/取消 + 新建回测 | ✅ 可用 |
| 结果分析 | analysis | TWR/XIRR/本金 + 逐笔分录/现金流/份额 | ✅ 可用（需先选账户） |
| 实时监控 | monitoring | 账户快照 + 状态摘要 | ✅ 可用（需先选账户） |
| 系统设置 | settings | 平台设置 | 占位（P2-033 后续实现） |

## 3. 页面详细介绍

### 3.1 仪表盘（占位）

显示"该页面将在后续任务中实现（P2-033）"。当前版本无功能。

### 3.2 数据导入

**功能**：上传 MVSV-1 行情文件，服务端解析后**字段级覆盖**导入 PostgreSQL（`finv_quote_secu_kline_min`）。

**表单字段**：

| 字段 | 说明 |
| --- | --- |
| 文件选择 | 本地 MVSV-1 行情文件（`POST /api/v1/imports/upload` multipart 上传，上限 50 MiB） |
| 数据源 | 如 `cn-feed` |
| 覆盖模式 | `FIELD`（只覆盖有值的字段，推荐）/ `ROW`（整行覆盖） |

**使用步骤**：选择文件 → 填写数据源/覆盖模式 → 勾选“我确认导入将覆盖同时刻同证券的对应字段值” → 上传并导入 → 显示导入统计（证券、条数、批次）。

**注意事项**：
- 导入按主键 `(ts, market_code, secu_code)` 覆盖同键数据，属危险操作，**必须勾选确认**才能提交；
- 每次导入自动登记批次（`quote_ingest_batches`）并在发生覆盖时写入修正审计（`quote_revision_log`）；
- 服务端未配置 PG（`VQ_POSTGRES_*` 环境变量）时上传返回 `[4001]`。

### 3.3 策略管理

**功能**：查看策略列表；创建策略草稿（PYTHON / DSL 二选一）；DSL 结构校验。

**DSL 校验规则**（`validateDsl`）：
- 顶层必须是 YAML 对象；
- 必需字段：`PlanType`、`FundScope`；
- `PlanType` 支持：`FixedAmountSchedule`、`ValueAveraging`、`TargetValue`、`TargetReturn`、`MaDeviation`、`ValuationPercentile`、`DrawdownMultiplier`。

**使用步骤**：展开"新建策略" → 输入名称 → 选类型（DSL/PYTHON）→ 输入源码（DSL 用 YAML）→ "校验 DSL" 看结果 → "保存策略"（保存为草稿，提交走命令流程）。

**注意事项**：
- 保存是**草稿**（提示"提交走命令流程"），不会立即生效；
- PYTHON 类型使用 `BaseStrategy` 子类；
- 策略列表来自领域 API（`/api/v1/strategies`），生产最小实现当前返回空（"暂无策略"）。

### 3.4 定投计划

**功能**：创建定投计划草稿。

**表单字段**：

| 字段 | 可选值 | 说明 |
| --- | --- | --- |
| 计划名称 | | 必填 |
| 基金代码 | 如 `FUND-A` | 必填 |
| 周期 | `Daily` / `Weekly` / `Monthly` | |
| 金额模式 | `Fixed` / `RuleBased` / `ExplicitSeries` | |
| 基础金额 | 如 `1000.00` | 必须为正数 |
| 资金来源 | `AccountCash` / `ExternalDeposit` | |

**注意事项**：创建为**草稿**（提交走命令流程）；命令 API 接线完成后可用。

### 3.5 账户管理

**功能**：账户列表展示（account_id / 执行模式 / run_id）+ 账户详情加载。

**使用步骤**：打开页面 → 若显示"无可用账户"，需在服务端 `.env.deploy` 配置 `VQ_ACCOUNTS`（逗号分隔）后重启服务端 → 选择账户 → 可选输入 `run_id` → "加载账户详情"。

**注意事项**：
- 账户列表来自服务端 `VQ_ACCOUNTS` 环境变量（生产最小实现）；
- `run_id` 可选；指定后按运行上下文隔离查询。

### 3.6 回测中心

**功能**：回测列表 + 运行控制（启动/取消）+ 新建回测。

**运行控制**（危险操作需确认）：
- 选择回测 → "▶ 启动" 需勾选"确认启动该回测"；
- "■ 取消" 需勾选"确认取消该回测"。

**新建回测表单**：

| 字段 | 说明 |
| --- | --- |
| 策略 ID | 必填（当前目录为空时可手动输入） |
| 账户 ID | 必填，必须是有权访问的账户 |
| 开始/结束日期 | 区间必填 |
| 初始资金 | 如 `1000000.00`，必须为正数 |
| 模式 | `IDEAL`（理想）/ `REALISTIC`（含真实摩擦与净值可用时间） |

**注意事项**：创建/启动/取消走领域 API（`/api/v1/backtests*`），**已接线可用**；状态非法时返回 `[1001] 400`。

### 3.7 结果分析

**功能**：指定账户的结果分析——现金流调整权益、TWR/XIRR、本金，以及逐笔分录/现金流/基金份额三个标签页。

**使用步骤**：**先在侧边栏选择账户**（未选择时提示"请先在侧边栏选择账户"）→ 可选输入 `run_id` → 查看 JSON 分析与三个 Tab。

**注意事项**：
- 结果**严格按账户隔离**（页面顶部显示当前账户）；
- 无数据时显示"无分录/无现金流/无份额"。

### 3.8 实时监控

**功能**：指定账户的快照（账户/模式/快照内容）+ 状态说明。

**使用步骤**：先在侧边栏选择账户 → 可选输入 `run_id` → 查看快照指标。

**注意事项**：订单/风险/告警的实时推送依赖 SSE 状态流（P2-030 通道已建）；当前页面显示快照 + 说明信息。

### 3.9 系统设置（占位）

显示"该页面将在后续任务中实现（P2-033）"。当前版本无功能。

## 4. 侧边栏

- **导航**：单选切换 9 个页面；
- **账户上下文**：账户选择器（显示 `account_id · 执行模式`）+ 模式徽标（TechSpec 10.1 要求账户相关界面持续显示当前账户）；无账户时显示"无可用账户"；
- **连接信息**：API 地址 + API 版本 / catalog 版本（API 不可达时显示红色错误）。

## 5. 业务处理逻辑（菜单 → 接口 → 数据落点）

### 5.1 数据导入全链路（示例）

数据导入是典型的**命令受理 + 异步执行**流程：

| 阶段 | 发生位置 | 处理逻辑 |
| --- | --- | --- |
| ① 表单校验 | GUI（本地） | `ImportRequest.validate`：数据源/标的不为空、日期区间合法、模式 FULL/INCREMENTAL |
| ② 危险操作确认 | GUI | 必须勾选“我确认导入数据将创建新数据版本” |
| ③ 提交命令 | GUI → API | `POST /api/v1/commands`，`command_type=DATA_IMPORT`，payload 含 source/instrument_id/start_date/end_date/import_mode |
| ④ 幂等查重 | 服务端 `CommandService.submit` | 幂等作用域 = 主体+账户+路由+键；**同键同载荷** → 返回原命令；**同键异载荷** → `1003 IDEMPOTENCY_CONFLICT`（409） |
| ⑤ 创建命令资源 | 服务端 → PostgreSQL | 写入 `command_records` 表（status=`PENDING`，身份字段冻结不可变） |
| ⑥ 返回受理 | API → GUI | 成功 `202 {command_id, status}`；失败：字段非法 → `400/1001`、幂等冲突 → `409/1003` |
| ⑦ 命令执行 | 任务端 `vq-job-data-ingestion` | `DataImportTask`：参数校验（缺失 → 退出码 2）、**执行键幂等**（同一执行键不重复导入）、生成 checkpoint `ckpt:data_import:<run>` |
| ⑧ 状态推进 | 执行端 → `CommandService.transition` | 状态机 `PENDING→AUTHORIZING→ACCEPTED→RUNNING→SUCCEEDED/FAILED`；FAILED 必须携带失败快照（code/error_code/catalog_version/retryable/details） |
| ⑨ 数据落点 | 数据层 | 上传导入：MVSV-1 → 解析 → 字段级覆盖 upsert 到 `finv_quote_secu_kline_min`（主键 ts+market_code+secu_code）；批次登记 `quote_ingest_batches`，覆盖写修正审计 `quote_revision_log` |

**成功场景**：提交后返回 202 受理 → 轮询 `GET /api/v1/commands/{command_id}` 看到 `SUCCEEDED` → 数据进入版本库，可用于回测/模拟盘。

**失败场景**：
- 同步失败：`400/1001`（参数非法）、`409/1003`（幂等键冲突，同键异载荷）；
- 异步失败：命令状态为 `FAILED`，`GET /api/v1/commands/{command_id}` 返回 `failure` 快照（code/error_code/catalog_version/retryable/details），失败原因可审计。

> 当前状态：命令 API 生产接线完成后此链路全通；当前 GUI 提交可能返回 `[1002] 404`（接线任务进行中）。

### 5.2 各菜单关联接口与处理要点

| 菜单 | 操作 | 关联 API | 处理要点 |
| --- | --- | --- | --- |
| 数据导入 | 上传导入 | `POST /api/v1/imports/upload`（multipart） | 上传 MVSV-1 → 服务端解析 → 字段级覆盖写 `finv_quote_secu_kline_min`；批次/修正审计自动登记 |
| 策略管理 | 列表 / 新建 | `GET /api/v1/strategies`；保存为本地草稿 | 列表当前为空（目录后续接入）；DSL 仅本地结构校验 |
| 定投计划 | 创建计划 | 本地草稿（提交走命令流程） | 字段本地校验（周期/金额模式/正数金额/资金来源） |
| 账户管理 | 列表 / 详情 | `GET /api/v1/accounts`；`GET /api/v1/accounts/{id}?run_id=` | 列表来自服务端 `VQ_ACCOUNTS`；账户不存在 → `1002` |
| 回测中心 | 列表 / 创建 / 启动 / 取消 | `GET /api/v1/backtests`；`POST /api/v1/backtests`；`POST /backtests/{run_id}/start\|cancel` | 创建走 `BacktestConfigV1` 校验（日期区间/initial_cash 正数/mode IDEAL\|REALISTIC）→ `BacktestApplicationServiceV1.createRun` → 202；非法状态操作 → `1001/400` |
| 结果分析 | 查询分析 | `GET /api/v1/accounts/{id}/analysis\|ledger\|cashflows\|shares`（`run_id` 可选） | 先选侧边栏账户；无数据返回空集 |
| 实时监控 | 账户快照 | `GET /api/v1/accounts/{id}`（`run_id` 可选） | 先选侧边栏账户；SSE 实时推送为后续能力 |

## 6. 注意事项汇总

1. **启动参数**：必须 `--api-url <地址> --serve`，缺 `--serve` 直接退出；
2. **账户来源**：服务端 `VQ_ACCOUNTS` 环境变量（未配置 → 各页面"无可用账户"）；
3. **命令 API 接线**：策略保存、定投计划创建走命令 API（`/api/v1/commands`），生产接线任务完成后可用；当前若报 404 属预期；
4. **数据导入**：上传文件直接导入 PG（`/api/v1/imports/upload`），不依赖命令 API；
4. **目录为空**：策略/标的/基金目录当前返回空列表（生产最小实现，后续阶段接入）；
5. **账户隔离**：结果分析/实时监控必须先选侧边栏账户，查询严格按账户隔离；
6. **危险操作确认**：导入/启动/取消等操作必须勾选确认，防止误操作；
7. **错误提示**：统一信封格式 `错误 [code] HTTP status: message`；`retryable` 标记可重试。

## 7. 常见问题

**Q：页面报 `错误 [1002] HTTP 404: platform.resource_not_found`？**
A：领域/命令 API 未接线或资源不存在。确认服务端镜像已更新（领域 API 接线见 PR #275），且 `VQ_ACCOUNTS` 已配置。

**Q：打开页面显示"无可用账户"？**
A：服务端 `.env.deploy` 配置 `VQ_ACCOUNTS=acc-paper-001,acc-paper-002` 后重启服务端。

**Q：点击提交没有反应？**
A：命令类操作（数据导入/保存策略/创建计划）依赖命令 API 接线，当前可能返回 404；回测创建/启动/取消可用。

## 8. 相关文档

- API 接口：`Docs/API/VeritasQuantApiReference.md`
- 数据库表结构：`Docs/PG/VeritasQuantDatabaseSchema.md`
- 部署与 FAQ：`Docker/Windows11Deployment.md`
- 实现源码：`src/veritasquant/apps/guiclient/`（GuiApp / Pages / ApiClient）
