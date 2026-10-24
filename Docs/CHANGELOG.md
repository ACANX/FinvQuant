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

> 0.1.3 开发中。变更按 `新增` / `变更` / `修复` / `移除` / `安全` 分类追加，发布时整体改为版本号并补日期。

### 新增

- **证券行情月归档导出与上传自动化链路**：新增 `internal/meta/finv/quote/dedup`，按「成熟度 → 候选 → LOAD（主表→`export` 中转表）→ EXPORT（→ MVsv 文件）→ CLEANUP（清理中转）→ 上传独享存储 → 收口补丁 → 生命周期清理」实现 10 个 Job（`CALC_DAILY_INGEST_STATS`、`CALC_DAILY_EXPORT_BUDGET`、`REFRESH_QUOTE_MATURITY`、`MARK_SEALED_JOBS_FROZEN`、`RUN_DUE_EXPORT_JOBS`、`UPLOAD_PENDING_EXPORT_FILES`、`SCAN_POST_SEAL_PATCHES`、`CLEAN_EXPIRED_EXPORT_ARTIFACTS`、`RECOVER_TIMEOUT_EXPORT_JOBS`、`CHECK_EXPORT_JOB_HEALTH`）；每日预算自适应（`factor = clamp(1.20 + 0.20*sla + 0.05*min(backlog,5), 1.20, 2.00)`）、keyset 分页、行锁 + 租约 + 心跳、暂停续跑、指数退避。
- 新增迁移 **`V41__finv_dedup_export_archive_tables.sql`**：在 `dedup` schema 新建 12 张 `finv_dedup_*` 表（入口统计、日预算、扫描水位、优先级、成熟度、交易日历、运行日志、证券主数据、导出任务、导出文件、上传审计、补丁任务）与 12 个规范命名索引，全库无外键，末尾做收敛校验；**不修改** `dedup.finv_quote_secu_kline_min` 与 `export.finv_quote_secu_kline_min`。
- 新增导出格式实现：MVsv「16 行文件头 + 1 行空行 + `ts|da|ti|o|h|l|c|v|t|cp|cr|lc` 数据行」、临时文件 + SHA256 + 原子 `rename`、结构与 `Count` 校验、可选 gzip；BASE / PATCH / MANIFEST 命名与 Manifest 字段按方案稿落地。
- 新增可插拔上传适配器（`Uploader`：`Stat` / `Put` / `Verify`）：内置 `none`（不启用）、`dir`（目录 / NAS 挂载）、`http`（HTTP PUT 网关）三种实现；上传前本地校验、远端查重、上传后远端 checksum 校验，每次尝试写 `finv_dedup_export_file_upload_log`。
- 新增 6 个运维接口（`/API/V1/Meta/Finv/Quote/DeDup/*`）：`Status`、`Job/List`、`File/List`、`Job/Retry`、`File/Retry`、`Job/Run`，均为一接口一文档（`Docs/API/QuoteDeDup/*.md`），并在 `Docs/API/README.md` 与 `Docs/API/APIs.md` 登记。
- 新增配置项（均在 `config.Config` 中集中解析）：`FINV_QUOTE_DEDUB_ENABLED`、`_TIMEZONE`、`_OUTPUT_DIR`、`_WORKER_ID`、`_PAGE_SIZE`、`_WINDOW_START`、`_WINDOW_END`、`_LEASE_SECONDS`、`_MAX_ATTEMPTS`、`_UPLOAD_MODE`、`_UPLOAD_DIR`、`_UPLOAD_HTTP_BASE`、`_UPLOAD_HTTP_TOKEN`、`_UPLOAD_PREFIX`、`_LOCAL_RETENTION_DAYS`、`_PURGE_MAIN_ENABLED`、`_PURGE_MAIN_MONTHS`、`_MATURITY_THRESHOLD`、`_PATCH_BUDGET_RATIO`、`_COMPRESSION`、`_FETCH_LIMIT`、`_MAX_JOBS_PER_RUN`。
- 新增落地文档 [`Docs/Refactor/2610-005-FinvQuoteDeDubExportUpload.md`](Refactor/2610-005-FinvQuoteDeDubExportUpload.md)（流程、配置、调度、运维要点、与方案稿的差异说明、P0–P7 阶段对照）。
- **前端月归档导出控制台**（`Web/src/views/Meta/Finv/Quote/DeDup/Export/`）：新增 3 个页面，挂到「数据采集 → 行情数据去重 → 月归档导出」菜单：
  - **归档监控总览**（`/Meta/Finv/Quote/DeDup/Export/Overview`）：指标卡（待装载行数、积压天数、预算因子与消耗、FROZEN 缺失、近 7 天补丁、心跳超时、上传重试耗尽、待清理文件、MVsv 异常）、预算使用率与完整性进度条、任务/上传状态分布、告警列表（`alerts`）、运行参数摘要、10 个 Job 调度表与「立即运行」，支持 30 秒自动刷新；
  - **月归档导出任务**（`.../Export/Job`）：`finv_dedup_export_job` 分页表（状态、已装载行数、BASE 版本、补丁数、收口/冻结日、执行者、心跳、下次运行、最近错误），支持状态 / 证券 / 业务月过滤与人工重试；
  - **月归档导出文件与上传**（`.../Export/File`）：`finv_dedup_export_file` 分页表（类型、版本、行数、大小、上传状态、尝试次数、上传起止、远端校验、网盘归档、清理状态、下次重试），支持上传状态 / 文件类型过滤、「仅看已上传」快捷筛选与重试上传。
  - 复用 `Web/src/api.ts`（`apiGet` / `apiPost`）与 `Web/src/utils.ts`（新增 `fmtISO` / `fmtBytes` / `fmtMonth`），仅调用已有 6 个后端接口，无后端改动。
- **归档运维台（第二批：全表查询 + 管理员编辑）**：补齐 12 张 `finv_dedup_*` 表的运维能力：
  - 新增 10 个查询接口：`Stats/Page`、`Budget/Page`、`Maturity/Page`、`Watermark/List`、`Priority/List`、`Calendar/Page`、`Usc/Page`、`RunLog/Page`、`UploadLog/Page`、`Patch/Page`；
  - 新增 11 个管理员写接口：`PUT Usc`、`PUT|DELETE Priority`、`PUT|DELETE Calendar`、`PUT Budget/Override`、`PUT Watermark`（重置）、`POST Patch/Retry`、`POST Job/Pause`、`POST Job/Cancel`、`POST Maturity/Refresh`；
  - 可编辑白名单 5 张表：`finv_dedup_usc`（含决定能否导出的 `day_expect_rows` 校准值）、`finv_dedup_usc_priority`、`finv_dedup_trade_calendar`、`finv_dedup_daily_budget`、`finv_dedup_scan_watermark`；其余 7 张表只读；
  - 所有写操作追加审计（`finv_dedup_job_run_log_export_file`，`job_kind=MANUAL_ADMIN_EDIT`）；
  - 前端新增 3 个页面：**归档统计与预算**（`.../Export/Stats`）、**归档主数据与规则**（`.../Export/Config`）、**归档日志与补丁**（`.../Export/Log`）；`Web/src/api.ts` 新增 `apiPut` / `apiDelete`；
  - `Docs/API/QuoteDeDup/` 新增 11 篇文档（Usc / Priority / Calendar / Maturity / Stats / Budget / Watermark / RunLog / UploadLog / Patch / JobControl），`Docs/API/APIs.md` 登记 58–84 号接口并回填前端页面。

### 变更

- 服务启动流程（`cmd/server/main.go`）：始终创建归档服务（运维接口可用），仅当 `FINV_QUOTE_DEDUB_ENABLED=true` 时启动进程内调度器（每 30 秒 tick；多实例由数据库行锁 + 租约防重）。
- `internal/api/router.go`：新增 `Deps.ExportArchive` 与 `/Meta/Finv/Quote/DeDup/**` 路由组（服务未注入时不注册）。
- `Docs/DevSpec/DbSchemaSpec.md`：`dedup` schema 归属表登记新增归档链路 12 张表。
- `Docs/RequirementList.md`：新增归档链路需求条目（自 `10291` 起）。
- 默认不修改既有两张业务表，不新增 `export` / `sche` schema，导出/上传文件统一 `.mvsv`（不生成、不支持 `.csv`）。
- **CI/CD 拆分为三种发布包**（`.github/workflows/CI.yml`）：新增 `pr-binaries` / `pr-publish` 与 `release-binaries` / `release-publish`，与既有 `snapshot-binaries` / `snapshot-publish` 同级平行：
  - **快照包（snapshot）**：push 代码到分支（非 `main`，且提交未关联已合并 PR）→ `v{VERSION}.{PPPPP}.{yymmddHHmm}`（prerelease，时间戳保证同一 PR 多次 push 唯一）；
  - **构建包（pr）**：PR 合并 → `v{VERSION}.{PPPPP}`（prerelease；**B 策略**：同名 release/tag 已存在时删除后重建）；
  - **正式版包（release）**：代码合并到 `main` → `v{VERSION}`（正式 release、`--latest`；tag 已存在则跳过，需先 bump `VERSION`）。
  - 三种场景互斥（提交已关联已合并 PR 时只出构建包），`push` 分支范围扩展为 `dev` / `main` / `FinvQuant` / `feat/**`，镜像构建仍仅限 `dev` / `main` / `FinvQuant` 与 `v*` tag。
- `Deploy/Script/PackSnapshot.sh` 支持三种包版本格式（`v{BASE}`、`v{BASE}.{PPPPP}`、`v{BASE}.{PPPPP}.{yymmddHHmm}`）与 `--kind snapshot|pr|release`，包内 `BUILD-INFO.txt` 增加包类型；`workflow_dispatch` 增加 `kind` 输入，便于手动构建指定类型包。
- `Docs/DevSpec/GitSpec.md` §4 补充三种包的版本/tag 与发布规则。
- **包目录调整**：`internal/exportarchive` 迁移到 `internal/meta/finv/quote/dedup`（包名 `dedup`），`cmd/server/main.go`、`internal/api`（handler / router / 测试）与 `Docs`（CHANGELOG、RequirementList、Refactor 落地文档）中的引用同步更新；接口路径、配置项与功能行为不变。
- `Deploy/Docker/Dockerfile` 移除 Go 构建阶段冗余的 `--platform=$TARGETPLATFORM`（BuildKit 默认即目标平台），消除 `RedundantTargetPlatform` 构建检查告警；前端构建阶段的 `$BUILDPLATFORM` 保留（跨平台复用产物所需）。
- **依赖安全升级（Dependabot 24 个告警清零）**：
  - Go：`golang.org/x/crypto` 0.48.0 → **0.52.0**、`golang.org/x/net` 0.51.0 → **0.55.0**、`github.com/quic-go/quic-go` 0.59.0 → **0.59.1**（连带 `x/sys` 0.45.0、`x/text` 0.37.0、`x/sync` 0.20.0）；
  - 前端：`vite` 8.2.0 → **8.3.5**、`vue` 3.5.40 → **3.5.44**（含 `@vue/server-renderer` 3.5.44）、`source-map-js` → **1.2.2**、`nanoid` → **3.3.20**；`package.json` 的 `vite` / `vue` 最低版本同步提升，避免回退；
  - 验证：`go build/vet/test` 全绿、`npm audit` 输出 `found 0 vulnerabilities`、`npm run build` 通过、Docker 镜像重建并实跑健康。

## [0.1.2] - 2026-10-24

### 新增

- **分钟行情 K 线 API 融合（FinvQuoteDataDeDup → FinvQuant）**：新增 `internal/kline`（领域服务/校验/格式化/错误目录）与 `internal/api/handler/kline*.go`，在 16001 进程内实现原 Python 服务的 8 类业务接口（`Add`、`Batch`、`Upsert`、`Page`、`PageUni`、`Query`、`PATCH Batch`、`DELETE Batch`、`Count`），统一前缀 **`/API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min`**；旧前缀 `/API/V1/Meta/Finv/Quote/Secu/KLine/Min` 作为切流期过渡别名同时注册。
- 新增运维端点 **`GET /Health`**（探数据库，替代旧 `/healthz`，`/healthz` 保留为过渡别名）与 `GET /` 服务标识（返回 `service/prefix/docs/version`）。
- 新增迁移 **`V38__finv_quote_kline_move_to_dedup.sql`**：分钟行情主表 `quote.finv_quote_secu_kline_min` 迁入 `dedup` schema（`SET SCHEMA`，不重写数据），补齐 `change_price`/`change_ratio`/`provider` 三列，并按 `dedup` 新结构重建 `export.finv_quote_secu_kline_min` 转存表（含触发器与收敛校验）。
- 新增配置项：`FINV_KLINE_MAX_BATCH_ROWS`（默认 5000）、`FINV_KLINE_MAX_LIST_SIZE`（默认 5000）、`FINV_KLINE_TS_UNIT`（`s`/`ms`）、`FINV_ERROR_DETAIL_MODE`（`safe`/`admin`）、`FINV_ERROR_DEBUG_TOKEN`、`FINV_ERROR_DETAIL_ALLOWED_CIDRS`。
- 新增 10 篇接口文档（`Docs/API/KLine/*.md`）并在 `Docs/API/README.md`、`Docs/API/APIs.md` 登记；`Docs/DevSpec/ErrorCodeSpec.md` 增补 `21xxxx` 号段与复用规则；`Docs/DevSpec/ApiSpec.md` 登记兼容例外。
- **数据采集前端菜单**：新增一级菜单「数据采集」→ 二级菜单「行情数据去重」→ 两个三级子菜单：「去重分钟行情查询」（按 `usc` + 交易日期范围分页查询 `dedup.finv_quote_secu_kline_min`）与「去重分钟行情统计」（汇总指标、字段完整性、每日趋势与证券分布可视化）。
- 新增迁移 **`V40__finv_quote_kline_date_indexes.sql`**：为 `dedup.finv_quote_secu_kline_min` 增加 `(usc, date, ts)` 与 `(date, usc, ts)` 复合索引，支撑「证券 + 交易日期范围」分页查询与按日聚合（此前仅 `(usc, ts)`，日期过滤需顺扫历史 2.8M 行，单次分页可达 8~17 秒）。
- 新增接口 **`GET /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Stats`**：返回汇总统计（行数/证券数/日期范围/成交量与成交额合计/收盘价区间与均值/各列空值行数）、按日期聚合趋势、证券记录数 Top 20；文档见 `Docs/API/KLine/KLineStats.md`。

### 变更

- **错误码口径（DC-05）**：分钟行情 API 使用 `211xxx` 具体错误码（`211001`~`211303`，15 条，编号/HTTP/安全文案见 `internal/kline/apierr.go` 与接口文档）；其余/兜底错误复用原 HTTP 错误码（`400/404/409/413/422/503`，`500` 沿用 `2006`）。
- **破坏性变更（接口）**：存量接口旧业务号 `4001/4004/4009` 停用，分别收敛为 `400`/`404`/`211104`（前端仅判断 `code !== 0`，不受影响；调用方如依赖具体码需同步调整）。
- **错误详情分级（DC-06）**：错误响应默认只返回安全文案；`FINV_ERROR_DETAIL_MODE=admin` 且携带有效 `X-Debug-Token` 或来源 IP 命中内网 CIDR 时，才附加白名单 `debug` 字段；DSN、完整 SQL、堆栈、令牌、内部主机名永不进入响应；服务端日志始终记录全量并可用 `request_id` 关联。
- `request_id` 统一为**大写 UUID4**（DC-07），与 `internal/backtest` 现有写法一致。
- 分钟行情表归属调整为 `dedup.finv_quote_secu_kline_min`；`internal/quote`、`internal/backtest`、`Docs/API/HistoryQuote/*`、`Docs/DevSpec/DbSchemaSpec.md` 等引用同步改为 `dedup.`；`TestSQLSchemaQualified` 对该表强制 `dedup.`/`export.` 前缀。
- **去重行情页面数字展示**：默认取消千分位分隔符，查询表格数值与总数、统计 KPI/图表提示均输出纯数字文本；大额统计按 `万/亿/万亿` 缩写展示（不再叠加千分位）。
- **去重行情默认查询条件**：证券代码默认 `GCMain`；开始/结束日期默认最近一个月（以库内该证券最新交易日期为结束日回推 1 个月，若取不到则回退本机当天），不再默认缺省。
- **侧边导航缩进优化**：一级→二级、二级→三级每级缩进在上一版 28px 基础上再减半至 **14px**（计算样式：二级 22px、三级 36px），三级菜单标题完整展示且留白充足；菜单图标与标题的水平间距由 32px 调整为 **约 1/3（10.67px）**；样式在 `App.vue` 内全局生效。
- **侧边导航顺序**：「数据采集」菜单移动到「历史行情」之前，紧跟仪表盘。
- **前端入口缓存**：`internal/static` 对 `index.html` 与 SPA 回退统一返回 `Cache-Control: no-cache, no-store, must-revalidate`（附 `Pragma/Expires`），避免前端重建后浏览器继续使用旧 `index.html` 引用旧 JS；同步刷新 `internal/webui/dist` 内嵌产物，保证非 Docker 构建也使用最新前端。
- **去重行情统计页成交额图表**：在「每日成交量」之后新增「每日成交额」独立图表，与「每日记录数」「每日成交量」「每日收盘价均值」共用时间轴与绘图区网格（四图纵向排列、时间轴对齐）。
- 新增轻量接口 **`GET /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Bounds`**：返回过滤范围 `date_from/date_to`，供前端初始化默认日期范围；实测 `usc=GCMain` 约 17ms（原用 `/Stats` 全量聚合约 8s）。
- **去重行情统计页布局**：「每日记录数与成交量」拆分为「每日记录数」「每日成交量」两个独立图表，与「每日收盘价均值」共三个图表；三者使用同一绘图区网格与时间轴刻度配置，横向完全对齐；「字段完整性（空值行数）」独立一行；图表坐标轴显式关闭千分位（ECharts 默认格式同步覆盖）。
- **时间展示修正**：`time=0` 按合法值展示为 `00:00:00`，仅 `null/undefined` 展示为 `-`（全局 `fmtTime` 同步修正）。
- 分钟行情分页/查询排序调整为 `usc, date, ts`（与新增日期索引一致，`date` 与 `ts` 单调对应，结果顺序对调用方等价）。
- 前端菜单层级调整（未发布版内）：原「去重行情」一级菜单更名为「数据采集」，其下新增二级菜单「行情数据去重」，去重查询/统计调整为该二级菜单下的三级子菜单（`Docs/RequirementList.md` 10276）。
- 接口错误码变更、表结构变更与文档索引更新同批提交（参见 `Docs/RequirementList.md` 10241 起）。

### 变更（涨跌字段补全规则）

- **MVSV 导入**：`cp`/`cr` 列由“解析后忽略”改为**有值时落库**；两列均缺失时由服务端自动补全——参考价 `last_close`（非 0）优先，缺失/为 0 时回退 `open`（非 0），两者都不可用则保持 `NULL`；计算式 `change_price = close - ref`、`change_ratio = (close - ref) / ref * 100`（百分比，6 位小数，`big.Rat` 精确运算）。
- **分钟行情 KLine API**：`/Add`、`/Batch`、`/Upsert` 在请求未提供 `change_price`/`change_ratio`（两列都为空）时按同一规则补全；API 不写 `last_close`，因此参考价回退 `open`。**任一列已有值时不做修改。**
- 导入/写入链路补全不改变 `FIELD`/`ROW` 覆盖语义：`FIELD` 模式下补全值为 NULL 时保留库内旧值，`ROW` 模式仍整行覆盖。

### 修复

- **分钟行情按日期范围分页性能**：新增日期索引并调整排序后，`GCMain + 最近一个月` 分页查询由约 7 秒降至约 0.03 秒，消除 503/超时。
- **修复去重行情页面报错 `OK` 且无数据**：前端 `apiRequest` 原先只接受 `code=0`，而分钟行情 KLine API 成功码为 `1`，导致 `/Page`、`/Stats` 被误判为失败；现按 `ApiSpec` 统一接受成功码 `{0,1,200,202}`。
- 修正分钟行情 `ts` 精度口径：库内与接口契约均为**秒级**（原 Python 代码/文档中的「毫秒」为文档缺陷）；13 位毫秒默认拒绝（`211004`），仅 `FINV_KLINE_TS_UNIT=ms` 时换算。
- `/Page`、`/PageUni` 的 `change_ratio` 统一按 `ROUND_HALF_UP` 输出两位小数文本；`/PageUni` 数值去尾零、`-0` 归一为 `0`。

## [0.1.1] - 2026-10-24

### 新增

- 新增本变更日志 `Docs/CHANGELOG.md`，汇总 0.1.0 及历史 VeritasQuant 版本的对外变更。
- 开发规范新增「变更日志」强制要求：[`DevSpec/DocSpec.md`](DevSpec/DocSpec.md) 第 4 节、[`DevSpec/GitSpec.md`](DevSpec/GitSpec.md) 第 3 节，并在 [`DevSpec/FileNamingSpec.md`](DevSpec/FileNamingSpec.md) 中明确本文件名全大写。
- 新增需求清单 [`RequirementList.md`](RequirementList.md)，采用五位数编号（自 `10001` 起），记录 2026-10-24 起的新增需求，首版 65 条；`AGENTS.md` 工作约定同步要求新增需求必须登记。
- 新增两份改造清单：改造项 A [`Refactor/2610-001-SecuCodeToUsc.md`](Refactor/2610-001-SecuCodeToUsc.md)（`secu_code` → `usc` 全库统一，`A-01`～`A-37`）、改造项 B [`Refactor/2610-002-PrevCloseToLastClose.md`](Refactor/2610-002-PrevCloseToLastClose.md)（`prev_close` → `last_close` 重命名 + `open` 兜底回填，`B-01`～`B-24`），共 61 个可勾选检查项与 4 项待决策；B 的删除动作暂缓，留待生产数据核实后另议。

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
- **`VeritasQuant/` 子项目整体移入 `Archive/VeritasQuant/`**（`git mv`，保留提交历史，590 个文件 / 6.4 MB）：该 Python 量化平台为 FinvQuant 前身，现已不参与构建、测试与部署；其 `.github/workflows/` 从未生效（GitHub 只识别仓库根），保留仅为追溯。同步更新根 `README.md`、`AGENTS.md`、`Prompt.md` 的目录结构与路径、`.dockerignore`（改为整体排除 `Archive/`）、`Archive/README.md`（新增归档登记表）、`Docs/API/HistoryQuote/ImportsUpload.md` 示例命令、`internal/mvsv/parser.go` 注释与 `Docs/CHANGELOG.md` 历史章节的链接。
- **快照产物命名改用连字符**：`FinvQuant-v0.1.1.00000.2610240325-ubuntu-amd64.tar.gz`（原 `FinvQuant_v0.1.1.00000.2610240325_ubuntu_amd64.tar.gz`），包内顶层目录名同步；快照 release 标题改为 `{快照版本}-FinvQuant`（如 `v0.1.1.00000.2610240325-FinvQuant`），git tag 仍为 `{快照版本}`；release 说明删除「产物」列表整节与「BUILD-INFO 时间戳一致」「SHA256SUMS 校验方式」两句冗余描述（附件已由 release assets 列表呈现，正文无需重复）。
- **`BUILD-INFO.txt` 补全 CI 溯源信息**：CI 构建时写入运行号（`CI run <id>`）、仓库地址与流水线链接（经 `CI_RUN_ID`/`BUILD_REPO`/`BUILD_RUN_URL` 环境变量传入），本地构建时这几行自动省略。
- **快照产物命名规范调整**（`PackSnapshot.sh` / CI 同步）：产物目录 `dist/` → **`Dist/`**；包名前缀 `finvquant_` → **`FinvQuant_`**；平台段 `ubuntu2604` → **`ubuntu`**、`win11` → **`windows`**。新命名示例：`FinvQuant_v0.1.1.00123.2610241530_ubuntu_arm64.tar.gz`。本次仅改命名与输出目录，包内容与打包流程不变。
- **新增 [`Deploy/Script/README.md`](Deploy/Script/README.md)（打包脚本文档）**：文件职责与「改打包只改 .sh」的维护约定、快速开始、全部参数表（含默认值）、版本三段来源与一致性保证、两种包的目录树与 `SHA256SUMS` 用法、环境要求、native/WSL/Git Bash 三环境差异与 zip 后端回退链、6 个常用配方、产物校验命令（架构字节/时间戳一致性/zip 正斜杠）、9 条常见问题、与 CI 三个 job 的调用关系；`Deploy/README.md` §2.3 相应精简为概览并链接该文档。
- **新增 `Deploy/Script/PackSnapshot.cmd`（Windows 封装）**：仅负责定位 Git for Windows 的 `bash.exe` 并转调 `PackSnapshot.sh`，打包逻辑仍只有一份。
  - 定位顺序：`git.exe` 位置推导 `<GitRoot>\bin\bash.exe` → Git 默认安装目录 → PATH；**主动跳过 `C:\Windows\System32\bash.exe`**（WSL 转发器，无法处理 Windows 路径）
  - 找不到 bash 时给出明确指引（装 Git for Windows / MSYS2，或改用 WSL）
  - `PackSnapshot.sh` 同步增强：识别 native / WSL / Git Bash 三种环境（WSL 用 `WSLENV` + `wslpath`，Git Bash 用 `cygpath`）；zip 打包按 `zip` → Windows 自带 `tar.exe` → `powershell.exe` → `python3` 依次回退（PowerShell 的 `Compress-Archive` 会写成反斜杠路径，故排在 bsdtar 之后）
- **新增 `Deploy/Script/PackSnapshot.sh`（快照打包的唯一实现）**：CI 的 `snapshot-binaries` 与本地手工出包调用同一脚本，避免两套逻辑漂移。
  - 用法：`bash Deploy/Script/PackSnapshot.sh [--version vX.Y.Z.PPPPP.yymmddHHmm] [--target linux/amd64|linux/arm64|windows/amd64|all] [--pr N] [--timestamp yymmddHHmm] [--build-web] [--keep-tmp]`
  - 默认 `--target all`，一次产出 Ubuntu 26.04（amd64/arm64）与 Windows 11（amd64）三个包到 `dist/`，附 `.sha256`
  - 版本三段由脚本统一解析/补零；同一快照多产物共用同一时间戳（可直接传 `--version` 复现 CI 产物）
  - WSL 兼容：自动探测 `go`/`go.exe`，Windows 版 Go 经 `WSLENV` 传递 `GOOS`/`GOARCH`，输出路径经 `wslpath` 转换
  - CI 侧改为单步 `BuildAndPack` 调用该脚本，删除原先内联的 `go build` 与打包脚本（减少约 90 行重复逻辑）
- **CI 增量支持 `feat/integration` 快照构建**（不改变既有镜像规则）：
  - 触发：`push` 到 `feat/integration`（并加入 PR 目标分支以便验证）
  - 版本标识：`v{VERSION}.{PPPPP}.{yymmddHHmm}` —— `PPPPP` 为关联 PR 号（5 位左补零，无 PR 为 `00000`），`yymmddHHmm` 为 Asia/Shanghai 构建时间戳；**由 `snapshot-meta` job 只算一次**，经 job outputs 传给所有产物，保证同一快照不同产物时间戳一致
  - 产物：Ubuntu 26.04 二进制包 `tar.gz`（`linux/amd64` + `linux/arm64`）、Windows 11 二进制包 `zip`（`windows/amd64`）；包内含二进制、`Deploy/Migrations`、部署脚本与文档、`VERSION`、`BUILD-INFO.txt`、`SHA256SUMS`（Linux 包带 Native 全套，Windows 包带 Docker 编排且不含 Dockerfile）
  - 发布：以快照版本为 git tag 建 **prerelease**，附三个包与 `.sha256`
  - 隔离：`docker` job 显式排除 `feat/integration`，集成分支不构建镜像、不污染 `latest`
  - 复用：`build-web` 上传 `web-dist` artifact 供二进制打包复用，避免二次 `npm ci`
  - 规范：`GitSpec.md` 新增第 4 节「版本号与构建标识」
- **新增 `archive` schema（冷数据归档域）**（`V37`）：幂等建 schema、以 `current_user` 设定属主并授予 `USAGE`/`CREATE`、追加到 `search_path` 末尾（`public,quote,quant,futu,export,sche,dedup,archive`）、末尾收敛校验；生产库已应用并验证（`V37 success=t`、属主 `finv`、权限齐备）。
  - 注意区分：`archive` **schema** 是数据库冷数据归档（历史行情下沉），仓库根 `Archive/` 是**文件**归档（废弃脚本/文档），两者无关。
  - 风险提示：归档表沿用业务表名会形成 `quote` / `export` / `archive` 三处同名，任何读写必须写 `archive.` 前缀。
- **新增 `dedup` schema（去重域）**（`V36`）：幂等建 schema、以 `current_user` 设定属主并授予 `USAGE`/`CREATE`、追加到 `search_path` 末尾（`public,quote,quant,futu,export,sche,dedup`）、末尾收敛校验；生产库已应用并验证（`V36 success=t`、属主 `finv`、权限齐备）。
  - 适用边界已在 `DbSchemaSpec.md` 写明：跨批次幂等由 `quote.finv_quote_secu_kline_min` 主键 `(ts, usc)` + `INSERT ... ON CONFLICT DO NOTHING` 保证，**无需**额外去重表；`dedup` 只承载「批内去重暂存」「跨源内容指纹比对」「入库审计留痕」三类对象。
- **`export` schema 新增行情导出转存表**（`V35`）：`export.finv_quote_secu_kline_min`，与 `quote.finv_quote_secu_kline_min` **同名同结构**（`LIKE ... INCLUDING DEFAULTS/CONSTRAINTS/INDEXES/STORAGE/COMMENTS` 建表 + 单独补 `gmt_update` 触发器），用于某证券月度/日历史行情导出前转存；迁移含双向列差集与主键/索引/触发器校验，幂等可重复执行。
  - ⚠ 同名风险：`quote` 在 `search_path` 中先于 `export`，裸写表名会命中业务表，转存表读写**必须**写 `export.` 前缀（`DbSchemaSpec.md` §2/§3.1 已补规则与实例）。
  - 验证：源/目标同期行数一致（`NVDA` 2026-06 转存 7,220 行）、`gmt_update` 触发器生效、`TRUNCATE` 后可复用，转存表现为空。
- **新增 `export` / `sche` 两个 schema**（融合前准备）：`export` 用于数据导出域、`sche` 用于定时任务调度域；`V34` 幂等建 schema、以 `current_user` 动态设定属主并授予 `USAGE`/`CREATE`、把两者追加到数据库与连接串的 `search_path` 末尾（`public,quote,quant,futu,export,sche`，不改变既有解析优先级）、末尾做收敛校验；生产库已应用并验证（`V34 success=t`、两 schema 为空、属主 `finv`、权限齐备）。
- **数据库名与角色名统一为 `finv`**（融合前准备，**破坏性**）：PostgreSQL 数据库 `finvquant`→`finv`、登录角色 `finvquant`→`finv`，**密码不变**；生产库已执行 `ALTER ROLE` / `ALTER DATABASE` 改名并验证（数据 2,804,329 行不变、`search_path` 随 OID 保留、授权无需重跑）。
  - 配置：`internal/config` 默认值、`docker-compose.yml`（`FINV_PG_USER`/`FINV_PG_DATABASE`/`POSTGRES_USER`/`POSTGRES_DB`/`pg_isready`）、`.env.example`、Native `FinvQuant.env.example`、`Install.sh` 兜底值、`deploy.cmd`/`upgrade.cmd` 的 `psql`/`pg_dump` 参数
  - 不改：产品/镜像/容器/项目名 `finvquant`、Linux 系统用户与 `/opt/finvquant`、二进制名、备份文件名前缀
  - 新增 [`.dockerignore`](../.dockerignore)：构建上下文排除 `Deploy/Backup/`、`Web/node_modules/`、`Docs/`、`Archive/` 等，构建耗时降至 ~38s
  - 改造清单：新增 [`Docs/Refactor/2610-006-RenameDatabaseAndRole.md`](Refactor/2610-006-RenameDatabaseAndRole.md)（`E-01`～`E-30`）
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

> 以下为 `VeritasQuant/` 独立阶段的历史版本，仅为追溯保留；其事件语义与验收门禁见 [`Archive/VeritasQuant/AGENTS.md`](../Archive/VeritasQuant/AGENTS.md)（该子项目已于 2026-10-24 移入归档目录）。

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
