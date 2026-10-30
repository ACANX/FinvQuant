# CDEU 归档工作流设计

**文档版本**：v1.0.0
**状态**：生效
**生效日期**：2026-10-29
**最后更新**：2026-10-29
**负责人**：@ACANX
**适用版本**：FinvQuant 0.1.4

---

## 变更日志

| 版本 | 日期 | 变更内容 | 变更人 |
|---|---|---|---|
| v1.0.0 | 2026-10-29 | 初始版本：CDEU 六阶段 / 17 个 Process / 22 个 JOB 的层级模型、执行语义、数据契约、配置项与运维说明 | @ACANX |

---

## 0. 定位与读者

本文描述 FinvQuant **CDEU（Collect → Dedup → Export → Upload）分钟行情月归档工作流**的设计与实现：整体为什么这么分层、每个阶段解决什么问题、每个 JOB 拿到什么输入、做了什么、产出什么、失败会怎样。

读者对象：

- 需要判断「某个证券月为什么没导出」「文件为什么没上传」的运维人员；
- 修改归档链路（新增 JOB、调整判定条件、改文件格式）的开发人员；
- 需要对齐前端 `CdeuOverview` / `CdeuWorkflow` 页面展示含义的人员。

配套文档：

- 概念模型原始设计：[2610-007-CDEUProcessModelDesign.md](../Refactor/2610-007-CDEUProcessModelDesign.md)
- 流程模型改造：[2610-008-CDEUWorkflowProcessModelRefactor.md](../Refactor/2610-008-CDEUWorkflowProcessModelRefactor.md)
- schema 与同名表隔离红线：[DbSchemaSpec.md](../DevSpec/DbSchemaSpec.md)
- 文件格式契约：[MvsvFileFormat.md](../DataFormat/MvsvFileFormat.md)
- 接口契约：[Docs/API/Quote/Cdeu/Min/](../API/Quote/Cdeu/Min/)、[Docs/API/QuoteDeDup/CDEUOverview.md](../API/QuoteDeDup/CDEUOverview.md)
- 菜单与页面：[Docs/Menu/Menus.md](../Menu/Menus.md)、[Menus 索引第 10 行](../Menu/Menus.md)

---

## 1. 目标与边界

| 目标 | 说明 |
|---|---|
| 数据可交付 | 把分钟行情按「证券 × 业务月」切成可独立交付的文件，交付物可校验、可重传、可增量补 |
| 过程可观测 | 每个阶段、每个 JOB、每个步骤都有运行记录与日志，页面能看到「现在跑到哪」 |
| 节奏可控制 | 导出受**低峰窗口 + 日预算**双重约束，避免归档拖垮在线业务 |
| 数据可释放 | 文件交付到远端后释放本地与主表数据，库体积不会无限增长 |
| 故障可恢复 | 游标续跑、租约超时回收、上传幂等重传、历史运行记录可控清理 |

**不在范围内**：

- **采集本身**：分钟行情的抓取由外部程序负责，平台内只有只读体检（`JOB_CHECK_INGEST_READINESS`），没有写入路径。
- 回测副本（`quote.finv_quote_secu_kline_min`）的维护：那是回测与查询链路的职责，本工作流只读取、不写入。
- 对象存储之外的交付形态：当前交付目标是 Git 仓库 / 本地目录 / HTTP 三类 uploader。

---

## 2. 术语与层级模型

### 2.1 五级层级

```
Workflow（工作流）
  └─ Phase（阶段）
       └─ Process（流程）
            └─ JOB（作业）
                 └─ Step（步骤）
```

| 层级 | 语义 | 举例 |
|---|---|---|
| Workflow | 一条端到端业务链路，当前只有 `CDEU_DAILY`（每日全链路） | `CDEU_DAILY` |
| Phase | 链路上的大阶段，按业务语义划分，有先后顺序 | `PHASE_EXPORT`（月度行情导出） |
| Process | 阶段内可独立完成的一件事，是**状态评估的最小单元** | `PROCESS_UPLOAD`（文件上传） |
| JOB | 真正被调度执行的单位，一个 Process 可以有多个 JOB | `JOB_UPLOAD_FILES`（待上传文件上传） |
| Step | JOB 内部的串行原子阶段，用于展示「跑到第几步」 | `STEP_LOAD`、`STEP_EXPORT`、`STEP_REGISTER`、`STEP_CLEANUP` |

层级不是装饰：**状态评估是逐级向上的**（Step → JOB → Process → Phase → Workflow），每级都按自己的 `completion_rule` 判定，见 §2.4。

### 2.2 定义与运行分离

| 类别 | 表 | 说明 |
|---|---|---|
| 定义 | `cdeu.finv_quote_cdeu_workflow`、`finv_quote_cdeu_phase`、`finv_quote_cdeu_process`、`finv_quote_cdeu_job`、`finv_quote_cdeu_job_step` | 静态配置：层级、顺序、触发方式、完成条件、超时、重试 |
| 运行 | `cdeu.finv_quote_cdeu_workflow_run`、`finv_quote_cdeu_phase_run`、`finv_quote_cdeu_process_run`、`finv_quote_cdeu_job_run`、`finv_quote_cdeu_job_step_run` | 每次执行生成一套运行树，记录状态、起止时间、行数、错误 |
| 动态实例 | `cdeu.finv_quote_cdeu_job_dynamic`、`finv_quote_cdeu_job_step_dynamic` | 「每证券月」「每文件」这类**基数由数据决定**的 JOB 实例 |
| 运行日志 | `cdeu.finv_quote_cdeu_job_run_log_line`、`finv_quote_cdeu_job_run_log_export_file` | 结构化日志行，前端「执行日志」弹窗直接读它 |

定义表改一行就能调整工作流（加 JOB、改触发时间、改完成条件），不需要改代码；运行表只增不改，用于审计与排障。

### 2.3 运行状态

| 状态 | 含义 |
|---|---|
| `PENDING` | 尚未满足启动条件（前序阶段未完成、依赖未就绪） |
| `READY` | 条件已满足，等待调度器捡起 |
| `RUNNING` | 执行中 |
| `SUCCESS` | 正常完成 |
| `PARTIAL` | 部分子项失败，但达到完成条件（如 `min_ratio` 达标） |
| `FAILED` | 失败 |
| `PAUSED` | 主动暂停（预算/窗口耗尽、游标已保存，可续跑） |
| `CANCELLED` | 人工取消 |
| `SKIPPED` | 按约定跳过（如 MANIFEST 不上传、上传模式为 `none`） |

终态集合：`SUCCESS`、`PARTIAL`、`FAILED`、`CANCELLED`、`SKIPPED`。`PAUSED` 不是终态，续跑会重新进入 `RUNNING`。

### 2.4 完成条件（`completion_rule`）

```json
{"mode": "ALL"}
```

| mode | 判定 |
|---|---|
| `ALL` | 全部子项完成才算完成（当前所有层级都用它） |
| `MIN_COUNT` | 完成数量 ≥ `min_count` |
| `MIN_RATIO` | 完成比例 ≥ `min_ratio`（`count_mode` 决定分母是总数还是应完成数） |

判定结果三档：全部达标 → `SUCCESS`；部分达标且规则允许 → `PARTIAL`；不达标 → `FAILED`（存在活动子项时保持 `RUNNING`）。

### 2.5 触发与执行模式

| 字段 | 取值 | 含义 |
|---|---|---|
| `jobs.schedule_kind` | `DAILY` / `INTERVAL` / `NONE` | 每日定时（`schedule_value` 为 `HH:MM`）或固定间隔秒数 |
| `jobs.trigger_mode` | `SCHEDULED` / `EMBEDDED` | 调度器直接触发，或作为其它 JOB 的内部阶段被带起来 |
| `jobs.execution_mode` | `DIRECT` / `EMBEDDED` | 是否有独立的 JOB 运行记录 |
| `process.batch_enabled` | `true` / `false` | 该 Process 下的 JOB 是否按数据行展开成动态实例 |
| `process.batch_source` | `EXPORT_JOB_CANDIDATES` / `PATCH_CANDIDATES` / `EXPORT_FILES` | 动态实例的数据来源 |

**动态实例**：一个「证券 × 业务月」的导出 JOB、一个「文件」的上传 JOB，都不是定义表里的一行，而是运行时按数据生成的实例（`cdeu_job_dynamic`）。这正是「22 个 JOB 定义」能覆盖「上万个证券月」的原因。

---

## 3. 总体架构

### 3.1 数据流

```
外部采集程序
      │  写入分钟行情（原始/回测口径）
      ▼
quote.finv_quote_secu_kline_min ──► cdeu.finv_quote_secu_kline_min（主表，去重后唯一口径）
      │                                        │
      │ ① 采集就绪体检（只读）                  │ ② 收口冻结：定稿月份不可再变
      │                                        ▼
      │                          ③ 决策：流量统计 / 熟度 / 预算 / 条件评估
      │                                        │
      │                                        ▼
      │                    ④ 导出：按证券月 LOAD → 生成 MVSV 文件 → 登记
      │                            │  BASE（全量）  │  INCR（收口后补丁）
      │                            ▼               ▼
      │                   export.finv_quote_secu_kline_min（中转表，用完即清）
      │                                        │
      │                                        ▼
      │                    ⑤ 上传：校验 → 远端去重 → PUT → 远端校验 → 审计
      │                                        │
      │                                        ▼
      │                          远端仓库（Git / 目录 / HTTP）
      │                                        │
      │                                        ▼
      └──────────────── ⑥ 复核：超时回收 / 健康检查 / 数据释放 / 运行记录保留
```

关键隔离：**主表 `cdeu.` 是唯一加工口径，中转表 `export.` 只做导出期间的临时落地区，回测副本 `quote.` 本工作流只读**。仓库 `DbSchemaSpec` §0 禁止把三张同名表合并（历史事故见该文档第 70 行）。

### 3.2 代码分层

| 目录 | 职责 |
|---|---|
| `internal/meta/finv/quote/dedup/` | 领域核心：运行层（`cdeu*.go`）、存储层（`store*.go`）、导出/上传/补丁实现（`job_export.go`、`job_upload.go`、`job_patch.go`） |
| `internal/meta/finv/quote/dedup/job/` | 22 个 JOB 的执行体，每个 JOB 一个文件，`init()` 里注册到 JOB 注册表 |
| `internal/meta/finv/quote/dedup/jobmeta/` | JOB 的调度元数据（触发时间、顺序），`jobmeta.All()` 决定调度器看到的顺序 |
| `internal/meta/finv/quote/dedup/job/…` + `jobmeta/` | 拆包原则：`dedup` 包不感知具体调度清单，新增 JOB 只需在 `job/` 加文件并登记 `jobmeta` |
| `internal/meta/finv/quote/mvsv/` | MVSV-1/2 文件的解析与写出（与归档共用格式实现） |
| `internal/meta/finv/sys/` | `sys.sys_config` 配置读写（20001–20012），带 TTL 缓存 |
| `internal/api/handler/{cdeu.go,cdeu_admin.go}` | 页面接口：概览、运行历史、JOB 手动触发、暂停/取消、成熟度重算 |
| `Web/src/views/.../DeDup/Export/DeDupCDEUView.vue`、`DeDupCDEUWorkflowView.vue` | 两个页面：运行监控 + 全流程拓扑 |

### 3.3 调度与执行

- 调度器按 `jobmeta.All()` 的清单与各 JOB 的 `schedule_kind/schedule_value` 触发（`scheduler.go`）。
- 每轮触发前检查：是否在低峰窗口内、日预算是否还有余量、`cdEU` 模式（20005）是否允许自动跑。
- 一个 JOB 执行时：`StartRun` 建运行日志 → `ensureCDEUWorkflowRun` 建立/复用当日运行树 → 逐个子项执行 → `TouchCDEUHierarchy` 刷新上级心跳 → 逐级 `Evaluate` 状态。
- 手动触发（页面「运行」按钮、`POST .../Job/Run`）走同一条执行路径，但**不受低峰窗口与日预算限制**（历史全量重导不可能塞进按日增量算出的预算里），消耗仍记账。

### 3.4 表清单

| 分类 | 表 |
|---|---|
| 定义 | `cdeu.finv_quote_cdeu_workflow`、`_phase`、`_process`、`_job`、`_job_step` |
| 运行 | `cdeu.finv_quote_cdeu_workflow_run`、`_phase_run`、`_process_run`、`_job_run`、`_job_step_run` |
| 动态实例 | `cdeu.finv_quote_cdeu_job_dynamic`、`_job_step_dynamic` |
| 归档业务 | `cdeu.finv_quote_cdeu_export_job`、`_export_batch`、`_export_file`、`_export_file_patch`、`_export_file_upload_log` |
| 决策输入 | `cdeu.finv_quote_cdeu_ingest_stats`、`_daily_budget`、`_quote_maturity_mon`、`_scan_watermark`、`_usc`、`_trade_calendar` |
| 数据 | 主表 `cdeu.finv_quote_secu_kline_min`、中转表 `export.finv_quote_secu_kline_min`、回测副本 `quote.finv_quote_secu_kline_min` |
| 运行日志 | `cdeu.finv_quote_cdeu_job_run_log_line`、`_job_run_log_export_file` |

---

## 4. 阶段总览

`CDEU_DAILY` 共 6 个阶段，`run_mode=BOTH`（可手动、可按 00:05 定时），各阶段完成条件均为 `ALL`。

| # | 阶段 | 名称 | Process 数 | JOB 数 | 时间锚点（调度） | 解决的问题 |
|---|---|---|---|---|---|---|
| 1 | `PHASE_COLLECT` | 历史行情收集 | 1 | 1 | 00:03 | 上游戏据是否还在持续到达 |
| 2 | `PHASE_DEDUP` | 历史行情去重 | 1 | 1 | 00:20 | 哪个月份的数据可以定稿、缺多少 |
| 3 | `PHASE_DECISION` | 导出决策 | 4 | 4 | 00:05–00:15 | 今天导什么、能导多少、有多少可导 |
| 4 | `PHASE_EXPORT` | 月度行情导出 | 3 | 7 | 内嵌 + 03:00 | 把主表数据变成可交付文件 |
| 5 | `PHASE_UPLOAD` | 月度行情上传 | 3 | 3 | 05 min 间隔 | 把文件安全送到远端并可追溯 |
| 6 | `PHASE_REVIEW` | 归档复核 | 5 | 7 | 300 s / 900 s / 06:10–06:40 | 故障回收、数据释放、运行记录控容 |

合计：**6 阶段 / 17 Process / 22 JOB（1 个已停用）/ 15 个静态 Step**。

```
COLLECT ──► DEDUP ──► DECISION ──► EXPORT ──► UPLOAD ──► REVIEW
体检        收口定稿     预算与判定      生成文件      送达远端      清理与看护
```

---

## 5. 阶段与流程详解

每个阶段的描述统一给出：**目标 → 输入 → 输出 → 流程清单 → 关键规则 → 失败与边界**。

### 5.1 PHASE_COLLECT 历史行情收集

- **目标**：确认上游采集链路仍在供数。采集由外部程序完成，平台内没有写入路径，因此本阶段是「只读体检」而不是「抓取」。
- **输入**：`quote.finv_quote_secu_kline_min`（回测口径副本，采集程序直接写入的那张表）。
- **输出**：运行明细 JSON（最新交易日、最新写入时间、总行数、证券数、滞后天数、当月是否有数据、是否滞后），以及运行日志。
- **流程**：

| Process | JOB | Step | 触发 |
|---|---|---|---|
| `PROCESS_HISTORY_INGEST` 历史行情数据采集 | `JOB_CHECK_INGEST_READINESS` 采集就绪检查 | —（无静态 Step，单步执行） | `SCHEDULED` / `DAILY` 00:03 |

- **关键规则**：
  - 滞后天数按交易日日历计算，**> 2 天**判定为采集可能停摆，输出 `ALERT 采集滞后`；
  - 本 JOB 不修改任何业务数据，可任意次重跑。
- **失败与边界**：采集停摆时本阶段会 `FAILED`/告警，但不会阻断后续阶段——导出只依赖主表已有数据，这个设计让「采集故障」与「归档故障」可以分开定位。

### 5.2 PHASE_DEDUP 历史行情去重

- **目标**：把「数据已经不会再有变化」的月份**定稿（seal）**，并冻结该月的导出任务；同时输出近 6 个月的完整度差异报告。
- **输入**：主表 `cdeu.finv_quote_secu_kline_min`、成熟度表 `cdeu.finv_quote_cdeu_quote_maturity_mon`、导出任务表 `cdeu.finv_quote_cdeu_export_job`。
- **输出**：被定稿的成熟度行数、被冻结的导出任务数、差异报告（期望交易日 vs 实际交易日、期望行数 vs 实际行数、缺失比例）。
- **流程**：

| Process | JOB | 触发 |
|---|---|---|
| `PROCESS_SEAL` 收口冻结与差异报告生成 | `JOB_MARK_SEALED` 收口冻结与差异报告生成 | `SCHEDULED` / `DAILY` 00:20 |

- **关键规则**：
  - 业务月的**收口日 = 次月 25 日**（`SealDateOf`）；`today ≥ 收口日` 的最近月份即 `maxSealedBizMonth`，其成熟度行被置为已定稿；
  - 到 `seal_date + SLA 天数` 仍未导完的导出任务被**强制冻结**（`MarkJobsFrozen`），优先级提升；
  - 差异报告覆盖**近 6 个月**，最多 200 条，按 `missing_ratio` 逐条打日志，便于回答「这个月到底缺不缺数」。
- **失败与边界**：定稿是幂等的（重复跑只是再次置位）；已定稿月份**不会**因为后续迟到数据回退状态——迟到数据走 §5.4 的增量补丁（INCR）。

### 5.3 PHASE_DECISION 导出决策

- **目标**：在真正导出前把四个问题算清楚：今天进来多少数据、哪些证券月已成熟、今天有多少导出预算、当前符合导出条件的候选有多少。
- **输入**：主表、`cdeu.finv_quote_cdeu_ingest_stats`（历史流量）、`cdeu.finv_quote_cdeu_usc`（证券元数据）、`cdeu.finv_quote_cdeu_export_job`（已导出状态）。
- **输出**：`cdeu.finv_quote_cdeu_ingest_stats`、`cdeu.finv_quote_cdeu_daily_budget`、`cdeu.finv_quote_cdeu_quote_maturity_mon`、`cdeu.finv_quote_cdeu_scan_watermark`，以及四份运行明细。
- **流程**：

| Process | JOB | 触发 | 一句话 |
|---|---|---|---|
| `PROCESS_INGEST_STATS` 每日入口流量统计 | `JOB_CALC_INGEST_STATS` | `DAILY` 00:05 | 近 30 天每日入口行数/证券数/字节数入库 |
| `PROCESS_MATURITY` 行情成熟度扫描 | `JOB_REFRESH_MATURITY` | `DAILY` 00:05 | 增量刷新每个证券月的成熟度，发现新证券 |
| `PROCESS_BUDGET` 每日导出预算计算 | `JOB_CALC_BUDGET` | `DAILY` 00:10 | 算出今天的行/字节预算与硬顶 |
| `PROCESS_EXPORT_DECISION` 导出条件判定 | `JOB_EVALUATE_EXPORT_CONDITION` | `DAILY` 00:15 | 只读评估候选与排除原因 |

- **关键规则**：
  - **预算公式**（`ComputeBudget`）：

    ```
    backlog    = pending_rows / base_rows
    factor     = clamp(1.20 + 0.20 × sla_pressure + 0.05 × min(backlog, 5), 1.20, 2.00)
    target     = min(base_rows × factor, p75_rows)
    hard_cap   = base_rows × 2
    ```

    `sla_pressure` 由「距冻结日 3 天内待导数量」归一化得到（0–1）；`base_rows`、`p75_rows`、`base_bytes` 取近 30 天入口流量基线。基线缺失时用兜底常量，保证预算永远是一个正数。
  - **成熟度**：增量口径（按 `scan_watermark` 的 `REFRESH_QUOTE_MATURITY` 水位向后扫），**每月 1 日**额外全量校准最近 3 个月，纠正迟到数据造成的偏差。
  - **条件评估是只读的**：不写批次表，只输出候选数、排除原因分解（元数据不全 / 已退市 / 当月 / 未成熟 / 已导出）、可生成补丁的月份数与前 10 个样本。真正的批次仍由 §5.4 生成——这样「评估」与「执行」解耦，评估可以随便重跑。
- **失败与边界**：四个 JOB 互相独立，任一失败不影响其它三个；但预算缺失会让导出退化为「兜底预算」，因此 `PROCESS_BUDGET` 失败需要在页面上优先处理。

### 5.4 PHASE_EXPORT 月度行情导出

- **目标**：把主表数据按「证券 × 业务月」切成 MVSV 文件（全量 BASE + 增量 INCR），并在导出完成后立即释放中转资源。
- **输入**：主表 `cdeu.finv_quote_secu_kline_min`、导出批次候选、当日预算、证券元数据、扫描水位。
- **输出**：本地 MVSV 文件与 Manifest、`cdeu_export_file` 登记行、`cdeu_export_file_patch` 补丁记录、`cdeu_job_dynamic` 动态实例、耗用后的预算。
- **流程**：

| Process | JOB | 触发 / 基数 | 一句话 |
|---|---|---|---|
| `PROCESS_EXPORT_BATCH` 证券导出批次清单获取 | `JOB_FETCH_EXPORT_BATCH` | `EMBEDDED`（RUN_DUE_EXPORT_JOBS 第一段） | 生成当日 BASE/INCR 批次与候选任务 |
| `PROCESS_EXPORT_FILE_MON_BASE` 证券历史月度行情文件导出 | `JOB_FETCH_SECURITY_METADATA`、`JOB_EXPORT_LOAD`、`JOB_EXPORT_EXPORT`、`JOB_EXPORT_REGISTER`、`JOB_EXPORT_CLEANUP` | 动态实例：每证券月一个 | 元数据 → 装载 → 写文件 → 登记 → 清中转 |
| `PROCESS_EXPORT_FILE_MON_INCR` 证券历史月度行情增量文件导出 | `JOB_SCAN_PATCH` | `SCHEDULED` / `DAILY` 03:00 | 收口后迟到数据 → 重写 BASE 或生成 INCR |

#### 5.4.1 批次与主循环（`JOB_FETCH_EXPORT_BATCH`）

`RunDueExportJobs` 一轮做五件事：

1. 低峰窗口检查（`HH:MM:SS|HH:MM:SS`，支持跨零点；手动触发跳过）；
2. `GenerateExportBatch` 生成当日批次，写入 `cdeu_export_batch` / `cdeu_export_job`（BASE 与 INCR 候选）；
3. `RefreshSecurityMetadata` 补齐证券元数据（region / market / provider / name / timezone / currency），随后 `BackfillUscDefaults` 按日增量行数模板补 `day_expect_rows`、并把未配置的优先级 / SLA / 最大等待补为 100 / 5 / 7（两者只补空，不覆盖人工值）；
4. `EnsureDailyBudget` 取当日预算，`InsertCandidateJobs` 把候选变成可领取任务，`GenerateDynamicJobs` 展开为动态实例；
5. 循环 `ClaimJob`（租约领取）→ `ProcessExportJob`（LOAD → EXPORT → CLEANUP）→ 预算扣减（`AddBudgetConsumed`）→ 状态回写，直到预算耗尽、窗口关闭、无任务或达到单轮上限（`MaxJobsPerRun`）。

一次领取一个「证券月」是为了让失败与暂停**只影响一个证券月**；预算耗尽时该证券月的游标已落库，明天接着装。

#### 5.4.2 单证券月的五个步骤

| Step | 输入 | 处理 | 输出 | 解决的问题 |
|---|---|---|---|---|
| `STEP_LOAD`（META + LOAD） | 主表该证券月的行；元数据 | 校验元数据完整 → 按**日期游标**逐日 keyset 分页把主表行装入 `export.finv_quote_secu_kline_min`；每日装完做「中转行数 vs 主表基线行数」校验；预算/窗口耗尽即保存游标并 `PAUSED` | 中转表数据；`loaded_rows`、`load_cursor_date`、`load_cursor_ts` | 大月份被拆成可中断、可续跑的小段；装载期间新到的数据不会漏（少装会重试该日） |
| `STEP_EXPORT`（EXPORT） | 中转表数据 | 流式写 MVSV-2：16 行文件头 + 12 列数据行（`ts/da/ti/o/h/l/c/v/t/cp/cr/lc`），数值按主表原始精度输出 | `..._Mon_{yyyyMM}_Base.mvsv` + `..._Manifest.json` | 产出可独立交付、可校验的标准文件 |
| `STEP_REGISTER`（REGISTER） | 文件路径 | 计算 size、sha256、row_count，写 `cdeu_export_file`；必要时重建 Manifest | 登记行（`PENDING` 上传状态） | 让「文件是否已交付」有唯一权威记录 |
| `STEP_CLEANUP`（CLEANUP） | 中转表该证券月数据 | 删除中转表对应数据（单轮上限 5 万行） | 空出的中转空间 | 中转表是共享资源，必须用完即清，否则下一个证券月会被挤爆 |

**暂停语义**：`PAUSED` 时游标、行数与心跳都保留，`JOB_RECOVER_TIMEOUT` 不会误回收；次日继续时先做「中转行数 vs `loaded_rows`」一致性校验，不一致就重置游标全量重装（幂等）。

#### 5.4.3 收口后的增量补丁（`JOB_SCAN_PATCH`）

对已导出过的证券月，若 `scan_watermark` 之后又有新数据（迟到数据/补录），按 BASE 的交付状态分三级处理：

| 级别 | 触发条件 | 动作 | 结果 |
|---|---|---|---|
| `L1_HOT` | BASE 尚未上传成功；或系统配置 20007=0（保留主表） | **重写 BASE 同名覆盖**（原子替换），并登记一条 `L1_HOT` 补丁记录 | 远端仍是单个全量文件，最简单 |
| `L2_WARM` | BASE 已上传成功 | 生成 `..._Incr_{n}.mvsv` 并更新 Manifest，`patch_count` 递增 | 远端 BASE + 若干 INCR，按序叠加即全量 |
| `L3_COLD` | BASE 已归档 / 本地文件已清理 | 同 L2，但标记冷态，说明重导代价更高 | 明确告知运维「这份数据要从远端取回才能整月重写」 |

补丁预算 = 当日预算 × `PatchBudgetRatio`（手动触发不限）；数据量超预算时**仍完整导出**，只记录告警——截断补丁会制造静默的数据缺失，比多花预算危险得多。

- **失败与边界**：单个证券月失败不影响本轮其它证券月；失败任务由 `JOB_RECOVER_TIMEOUT`（§5.6）在租约超时后回收重试。

### 5.5 PHASE_UPLOAD 月度行情上传

- **目标**：把已登记的本地文件安全送到远端，并把「送没送到、送了几次、失败原因」留成审计记录。
- **输入**：`cdeu_export_file` 中待上传记录、本地文件、上传配置（20002/20003/20004）、uploader（Git / 目录 / HTTP）。
- **输出**：`cdeu_export_file_upload_log` 审计行、文件状态（`UPLOADED` / `FAILED` / `SKIPPED`）、远端文件。
- **流程**：

| Process | JOB | 触发 | 一句话 |
|---|---|---|---|
| `PROCESS_UPLOAD` 文件上传 | `JOB_UPLOAD_FILES` 待上传文件上传 | `INTERVAL` 300 s | 单轮最多 200 个文件，逐个校验并上传 |
| `PROCESS_LIFECYCLE_CLEAN` 生命周期清理 | `JOB_CLEAN_EXPIRED` 生命周期清理 | `enabled=false`（已停用） | 职责已拆分到 `PHASE_REVIEW` 的三个 JOB |
| `PROCESS_HEALTH` 归档健康检查 | `JOB_CHECK_HEALTH` 归档健康检查 | `INTERVAL` 900 s | 指标汇总 + MVsv 抽检 + 告警 |

#### 5.5.1 单文件上传（`UploadOneFile`）

```
标 SKIPPED（MANIFEST 不上传）
   └─ 校验本地：存在、size、sha256 与登记一致
        └─ 远端 HEAD：已存在且 size/checksum 一致 → 直接标 UPLOADED（幂等）
             └─ PUT 上传
                  └─ 远端 Verify：size + checksum
                       └─ FinishUpload(UPLOADED) + 写审计日志
```

失败路径统一：`FinishUpload(FAILED)` + 审计日志 + 退避 `backoffFor(attempt)`；达到最大次数后把下次尝试推到 24 小时后并输出 `ALERT UPLOAD_RETRY_EXHAUSTED`。

**MANIFEST 按业务约定不上传**（只上传 `.mvsv` 数据文件），统一标 `SKIPPED` 终态——否则它们会永久卡在待上传队列，阻塞「本地清理判定」与监控统计。

#### 5.5.2 上传目标路径

- `UploadLayout=by_security`（默认）：`{PREFIX}/{region}/{market}/{usc}/{文件名}`；
- 证券元数据缺 region/market 时退回扁平布局 `{PREFIX}/{文件名}` 并打日志（不阻断上传）；
- 远端仓库配置来自 `sys.sys_config` 20004 `{owner}:{repo}:{branch}:{pre_path_key}`。

### 5.6 PHASE_REVIEW 归档复核

- **目标**：给整条链路做「收尾与看护」——回收卡住的任务、检查健康度、释放已交付的数据与文件、控制运行记录的体积。
- **输入**：运行表、`cdeu_export_file`（含上传状态）、本地文件系统、主表、系统配置 20007–20011。
- **输出**：被回收的任务、健康指标与告警、被释放的主表行、被删除的本地文件、被清理的中转表与运行记录。
- **流程**：

| Process | JOB | 触发 | 一句话 |
|---|---|---|---|
| `PROCESS_RECOVER` 超时任务回收 | `JOB_RECOVER_TIMEOUT` | `INTERVAL` 300 s | 心跳/租约超时的导出任务与上传任务重新排队 |
| `PROCESS_REVIEW` 归档复核 | `JOB_CHECK_HEALTH` | `INTERVAL` 900 s | 与 5.3 同一实现，持续输出健康指标与告警 |
| `PROCESS_PURGE` 归档数据清理 | `JOB_PURGE_EXPORTED_MONTH`、`JOB_PURGE_LOCAL_FILES` | `DAILY` 06:10 / 06:15 | 释放已交付的主表数据与本地文件 |
| `PROCESS_STORAGE_CLEAN` 存储清理 | `JOB_CLEAN_STORAGE` | `DAILY` 06:20 | 清中转表无主残留，空了就 TRUNCATE |
| `PROCESS_RETENTION` 运行记录保留 | `JOB_PURGE_STALE_DYNAMIC`、`JOB_PURGE_HISTORY_RUNS` | `DAILY` 06:30 / 06:40 | 按保留策略清理动态实例与历史运行树 |

- **关键规则**：
  - **释放顺序是刻意的**：先删主表数据（06:10），再删本地文件（06:15），避免出现「本地文件没了但库里还能重导」的中间态；
  - `JOB_PURGE_EXPORTED_MONTH` 由 20007 控制，判据是「BASE 与**全部** PATCH 都已 `UPLOADED`」，扫描式判定天然覆盖「历史遗留未清理」的情况；
  - `JOB_PURGE_LOCAL_FILES` 由 20011 控制，额外要求「最后一个文件上传完成已超过 24 小时」，给人工回看留窗口；
  - `JOB_PURGE_HISTORY_RUNS` 只处理**终态**运行，且**永久保留最新一个** `workflow_run`。
- **失败与边界**：`PROCESS_PURGE`、`PROCESS_STORAGE_CLEAN`、`PROCESS_RETENTION` 的 `failure_policy=FAIL_FAST`（数据释放失败必须显式暴露）；其它 Process 为 `CONTINUE`。

---

## 6. JOB 与 Step 详解

以下按阶段列出 22 个 JOB 定义（1 个已停用）。每个 JOB 统一给出：**输入 → 处理 → 输出 → 解决的问题**，并注明幂等/失败语义。

### 6.1 采集与定稿

| JOB | 输入 | 处理 | 输出 | 解决的问题 |
|---|---|---|---|---|
| `JOB_CHECK_INGEST_READINESS` | `quote.finv_quote_secu_kline_min` | 计算最新交易日、滞后天数、总行数、证券数、当月是否有数据 | 运行明细 + 滞后告警（>2 天） | 采集是外部黑盒，需要可观测点 |
| `JOB_MARK_SEALED` | 主表、成熟度表、导出任务表 | 定稿 `maxSealedBizMonth`、冻结到期任务、生成近 6 个月差异报告 | 定稿行数、冻结任务数、差异清单 | 明确「哪个月定稿了、缺多少」，让导出有稳定基线 |

### 6.2 决策

| JOB | 输入 | 处理 | 输出 | 解决的问题 |
|---|---|---|---|---|
| `JOB_CALC_INGEST_STATS` | 主表近 30 天 | 按天聚合行数/证券数/字节 | `finv_quote_cdeu_ingest_stats` | 给预算提供「平常一天进来多少」的基线 |
| `JOB_REFRESH_MATURITY` | 主表（水位之后）、证券字典、日增量行数模板 | 增量刷新成熟度；发现新证券；按模板补全 `day_expect_rows` 与优先级/SLA 默认值；每月 1 日全量校准近 3 个月 | `quote_maturity_mon`、`scan_watermark`、`usc`、`usc_day_incr_row_tpl` | 决定「这个月能不能导」（熟度不足导出的就是残缺月） |
| `JOB_CALC_BUDGET` | 流量基线、待导行数、SLA 压力 | `ComputeBudget` 计算 factor / target / hard_cap | `finv_quote_cdeu_daily_budget` | 给导出装上「油门」，防止归档抢占在线资源 |
| `JOB_EVALUATE_EXPORT_CONDITION` | 主表、元数据、导出任务 | 只读评估 BASE 条件与排除原因、INCR 候选 | 运行明细（候选数、排除分解、样本） | 让「今天为什么只导了 3 个月」有明确答案 |

### 6.3 导出

| JOB | 输入 | 处理 | 输出 | 解决的问题 |
|---|---|---|---|---|
| `JOB_FETCH_EXPORT_BATCH` | 批次候选、预算、元数据 | 生成批次与候选任务、展开动态实例、领取并驱动 LOAD/EXPORT/CLEANUP | `export_batch` / `export_job` / 动态实例 / 预算消耗 | 把「决策」落成「实际的文件产出」，并做全局限流 |
| `JOB_FETCH_SECURITY_METADATA`（动态 Step） | 证券字典 | 补齐 region/market/provider/name/timezone/currency | 元数据完整的证券 | 文件名与文件头都依赖元数据，缺一项就无法生成合规文件 |
| `JOB_EXPORT_LOAD`（动态 Step） | 主表该证券月 | 按日游标分页装载到中转表，行数基线校验，支持暂停续跑 | 中转表数据 + 游标 | 把大月份拆成可中断的小段 |
| `JOB_EXPORT_EXPORT`（动态 Step） | 中转表 | 流式写 MVSV + Manifest | `.mvsv`、`_Manifest.json` | 产出标准交付物 |
| `JOB_EXPORT_REGISTER`（动态 Step） | 本地文件 | 计算 size/sha256/row_count 并登记 | `export_file` 记录 | 交付状态有唯一权威来源 |
| `JOB_EXPORT_CLEANUP`（动态 Step） | 中转表该证券月 | 分批删除（单批 5 万行） | 释放的中转空间 | 中转表是共享资源，必须即时回收 |
| `JOB_SCAN_PATCH` | 水位之后的迟到数据、BASE 交付状态 | L1 重写 BASE；L2/L3 生成 INCR；更新水位 | `export_file_patch`、新版本文件 | 收口后仍有数据变化时，用最小代价保持一致 |

### 6.4 上传

| JOB | 输入 | 处理 | 输出 | 解决的问题 |
|---|---|---|---|---|
| `JOB_UPLOAD_FILES` | 待上传文件、uploader 配置 | MANIFEST 标 SKIPPED → 单轮最多 200 个文件逐个校验上传 | 审计日志 + 文件状态 | 把文件可靠、可追溯地送到远端 |
| `JOB_CLEAN_EXPIRED` | — | 已停用（`enabled=false`，代码中也不在调度清单） | — | 职责拆到 `PHASE_REVIEW` 的 PURGE / STORAGE / RETENTION 三组 JOB |
| `JOB_CHECK_HEALTH` | 运行表、导出文件 | 汇总 job/upload 状态计数、按月进度、补丁进度、pending/backlog、预算因子、中转行数；抽检 20 个 MVsv 结构 | 健康指标 + 告警列表 | 让「链路是否健康」是一次可读的快照，而不是翻日志 |

### 6.5 复核

| JOB | 输入 | 处理 | 输出 | 解决的问题 |
|---|---|---|---|---|
| `JOB_RECOVER_TIMEOUT` | `export_job` 心跳/租约、上传中记录 | 回收超时的导出任务（重新排队）与超时的上传（`max(4×租约, 10 min)`） | 回收数量 | 进程崩溃/网络中断后任务不会永久卡死 |
| `JOB_PURGE_EXPORTED_MONTH` | 已交付证券月（BASE + 全部 PATCH 已上传） | 分批删主表该证券月（单批 5 万行） | 释放行数 | 库体积随交付推进而下降 |
| `JOB_PURGE_LOCAL_FILES` | 已交付且超 24 小时的本地文件 | 删除 `.mvsv` 与同月 Manifest，标记 `LOCAL_DELETED` | 释放磁盘 | 本地磁盘不成为长期仓库 |
| `JOB_CLEAN_STORAGE` | 中转表 | 清无主残留（单轮 5 万行）；空则 TRUNCATE | 干净的中转表 | 中转表膨胀会拖慢所有装载 |
| `JOB_PURGE_STALE_DYNAMIC` | 动态实例 | 按 20008 保留天数删终态实例 | 释放实例行数 | 动态实例按数据量增长，必须控容 |
| `JOB_PURGE_HISTORY_RUNS` | `cdeu_*` 运行表 | 按 20009/20010（Off / Days / Count）清理终态运行树 | 各表删除行数 | 运行记录长期累积会拖垮库 |

**Step 的作用**：JOB 内部阶段化（`STEP_LOAD` → `STEP_EXPORT` → `STEP_REGISTER` → `STEP_CLEANUP`）让页面能显示「这个证券月跑到哪一步」，也是暂停/续跑与失败定位的粒度。静态 Step 定义在 `cdeu_job_step`，动态 JOB 的 Step 状态写在 `cdeu_job_step_dynamic`。

### 6.6 静态 Step 清单（15 个）

| JOB | Step | 说明 |
|---|---|---|
| `JOB_CHECK_INGEST_READINESS` | — | 单步执行，无独立 Step |
| `JOB_MARK_SEALED` | `STEP_MARK_SEALED` | 定稿 + 冻结 + 差异报告 |
| `JOB_CALC_INGEST_STATS` | `STEP_CALC_INGEST_STATS` | 近 30 天流量聚合 |
| `JOB_REFRESH_MATURITY` | `STEP_REFRESH_MATURITY` | 增量成熟度 + 全量校准 |
| `JOB_CALC_BUDGET` | `STEP_CALC_BUDGET` | 预算计算 |
| `JOB_EVALUATE_EXPORT_CONDITION` | — | 单步只读评估 |
| `JOB_FETCH_EXPORT_BATCH` | `STEP_FETCH_EXPORT_BATCH` | 批次清单获取 + 驱动导出主循环 |
| `JOB_FETCH_SECURITY_METADATA` | `STEP_FETCH_SECURITY_METADATA` | 证券元数据补齐 |
| `JOB_EXPORT_LOAD` | `STEP_LOAD` | 主表 → 中转表 |
| `JOB_EXPORT_EXPORT` | `STEP_EXPORT` | 中转表 → MVSV 文件 |
| `JOB_EXPORT_REGISTER` | `STEP_REGISTER` | 文件登记 |
| `JOB_EXPORT_CLEANUP` | `STEP_CLEANUP` | 中转表清理 |
| `JOB_SCAN_PATCH` | `STEP_SCAN_PATCH` | 补丁扫描与生成 |
| `JOB_UPLOAD_FILES` | `STEP_UPLOAD_FILES` | 待上传文件上传 |
| `JOB_CHECK_HEALTH` | `STEP_CHECK_HEALTH` | 健康检查 |
| `JOB_RECOVER_TIMEOUT` | `STEP_RECOVER_TIMEOUT` | 超时回收 |
| `JOB_PURGE_EXPORTED_MONTH`、`JOB_PURGE_LOCAL_FILES`、`JOB_CLEAN_STORAGE`、`JOB_PURGE_STALE_DYNAMIC`、`JOB_PURGE_HISTORY_RUNS` | — | 清理类 JOB 单步执行 |

> 说明：`JOB_CLEAN_EXPIRED` 保留 `STEP_CLEAN_EXPIRED` 定义，但 JOB 本身 `enabled=false`，不参与调度。

---

## 7. 数据契约

### 7.1 关键表

| 表 | 关键字段 | 用途 |
|---|---|---|
| `finv_quote_cdeu_export_job` | `usc`、`biz_month`、`status`、`seal_date`、`base_version`、`patch_count`、`load_cursor_date`、`load_cursor_ts`、`loaded_rows`、`locked_until`、`gmt_heartbeat` | 单证券月的导出任务与游标 |
| `finv_quote_cdeu_export_file` | `file_type`（BASE/INCR/MANIFEST）、`version`、`file_path`、`file_size`、`checksum`、`row_count`、`upload_status`、`upload_attempt`、`cleanup_status`、`target_file_name` | 每个交付文件的全生命周期 |
| `finv_quote_cdeu_export_file_patch` | `patch_level`（L1_HOT/L2_WARM/L3_COLD）、`patch_version`、`source_rows`、`patch_rows`、`reason`、`arrival_start/end` | 补丁记录与迟到数据区间 |
| `finv_quote_cdeu_export_file_upload_log` | `attempt`、`target_path`、`started_at`、`result`、`uploaded_bytes`、`error_message` | 上传审计，逐次可查 |
| `finv_quote_cdeu_daily_budget` | `biz_date`、`base_rows`、`budget_rows`、`budget_bytes`、`hard_cap_rows`、`consumed_rows`、`consumed_bytes`、`factor` | 当日导出额度 |
| `finv_quote_cdeu_ingest_stats` | `stat_date`、`rows`、`usc_count`、`bytes` | 入口流量基线 |
| `finv_quote_cdeu_quote_maturity_mon` | `usc`、`biz_month`、`maturity_ratio`、`sealed` | 成熟度与定稿状态 |
| `finv_quote_cdeu_scan_watermark` | `name`、`watermark` | 增量扫描水位（成熟度、补丁） |
| `finv_quote_cdeu_usc_day_incr_row_tpl` | `region`、`market`、`category`、`day_expect_rows`、`flag_enable` | 日增量行数模板（三列均可为空串通配）：`usc.day_expect_rows` 为空时按此补全 |

### 7.2 文件产物

| 项 | 规则 |
|---|---|
| BASE 文件名 | `{region}_{market}_{usc}_Min_{provider}_Mon_{yyyyMM}_Base.mvsv` |
| INCR 文件名 | `{region}_{market}_{usc}_Min_{provider}_Mon_{yyyyMM}_Incr_{n}.mvsv` |
| Manifest 文件名 | `{region}_{market}_{usc}_Min_{provider}_Mon_{yyyyMM}_Manifest.json` |
| 文件头 | 16 行（标题、字段名、DataProvider、Field/FieldName/FieldType、TypeKLine、Count、Symbol、SecuCode、USC、Name、Region、Market、TimeZone、Currency） |
| 数据列 | 12 列固定顺序：`ts`、`da`、`ti`、`o`、`h`、`l`、`c`、`v`、`t`、`cp`、`cr`、`lc` |
| 编码 | UTF-8 无 BOM，`\|` 分隔，空值输出空串 |
| 数值精度 | 按主表原始精度输出十进制文本（裁剪无意义尾零），不使用科学计数法 |
| 压缩 | `none` 或 `gzip`（扩展名仍为 `.mvsv`），压缩方式登记在 `export_file` |
| 校验 | `file_size` + `sha256` + `row_count`；健康检查另有 MVSV 结构抽检 |

### 7.3 系统配置项

| 编号 | 名称 | 作用 |
|---|---|---|
| 20002 | 归档上传开关 | `1` 启用远端上传，`0` 关闭 |
| 20003 | 上传令牌 | 远端仓库 PAT |
| 20004 | 仓库定位 | `{owner}:{repo}:{branch}:{pre_path_key}` |
| 20005 | CDEU 运行模式 | `Manual` 仅手动 / `Auto` 允许定时自动 |
| 20006 | 导出低峰窗口 | `HH:MM:SS\|HH:MM:SS`，支持跨零点，`start == end` 视为全天 |
| 20007 | 导出后删主表 | `1` 交付后释放主表，`0` 保留（影响 BASE 覆盖/INCR 策略） |
| 20008 | 动态实例保留天数 | `PURGE_STALE_DYNAMIC` 的阈值（默认 30） |
| 20009 | 历史运行清理策略 | `Off` / `Days` / `Count` |
| 20010 | 历史运行保留阈值 | 配合 20009 使用（默认 30） |
| 20011 | 清理本地导出文件 | `1` 清理（默认）/ `0` 保留 |
| 20012 | 登录会话有效期 | 与本工作流无关，列此仅为编号连续性说明 |

### 7.4 关键阈值一览

| 项 | 值 | 位置 |
|---|---|---|
| 业务月收口日 | 次月 25 日 | `SealDateOf` |
| 采集滞后告警 | > 2 天 | `JOB_CHECK_INGEST_READINESS` |
| 预算 factor 区间 | 1.20 – 2.00 | `ComputeBudget` |
| 预算硬顶 | 基线 × 2 | `ComputeBudget` |
| 单轮导出任务上限 | `MaxJobsPerRun` | 运行配置 |
| 中转表单轮清理上限 | 50 000 行 | `job/limits.go` |
| 主表单批删除上限 | 50 000 行 | `job/limits.go` |
| 本地文件单轮清理上限 | 500 个 | `job/limits.go` |
| 单轮上传文件上限 | 200 个 | `job/upload_pending_export_files.go` |
| 健康检查 MVsv 抽检 | 20 个 | `job/check_export_job_health.go` |
| 上传超时判定 | `max(4 × 租约, 10 min)` | `JOB_RECOVER_TIMEOUT` |
| 本地文件清理延迟 | 上传完成 + 24 h | `syscfg.PurgeLocalExportFilesDelay` |
| 装载轮次保护 | 单月最多 62 轮 | `loadJob`（防游标不前进空转） |

---

## 8. 运行与运维

### 8.1 启动方式

| 方式 | 入口 | 说明 |
|---|---|---|
| 自动 | 调度器按 `jobmeta.All()` 与配置时间触发 | 受 20005/20006/预算约束 |
| 页面手动 | `POST /API/V1/Meta/Finv/Quote/DeDup/CDEU/Workflow/Run` | 启动完整工作流，不受窗口与预算限制 |
| 单 JOB 手动 | 页面 JOB 行「运行」→ `POST .../Job/Run` | 只跑一个 JOB，用于补跑与验证 |
| 单证券月手动 | 动态 JOB 行「运行」 | 常用于历史全量重导 |

### 8.2 暂停、取消与重跑

| 操作 | 语义 |
|---|---|
| 暂停（`Pause`） | 当前证券月保存游标后停在此处，`PAUSED` 可续跑 |
| 取消（`Cancel`） | 运行终止，已产出文件与登记保留（不删数据） |
| 单 JOB 重跑 | 重新建 JOB 运行记录，历史记录保留 |
| 续跑 | 从 `load_cursor_date`/`load_cursor_ts` 继续；先校验中转行数与 `loaded_rows` 一致，不一致则整月重装 |

### 8.3 告警与排查

| 告警/现象 | 含义 | 排查方向 |
|---|---|---|
| `ALERT 采集滞后` | `quote` 侧最新交易日落后 > 2 天 | 检查外部采集程序与文件投递 |
| `ALERT UPLOAD_RETRY_EXHAUSTED` | 单文件达到最大重试次数 | 看 `export_file_upload_log` 的 `error_message`；确认 20002/20003/20004 与网络 |
| `ALERT MVsv 结构异常` | 抽检文件头/列数/行数与登记不符 | 本地文件是否被外部改动；必要时重导出该证券月 |
| `FROZEN_DIFF` 日志 | 某证券月缺失交易日/行数比例偏高 | 该月数据是否真的缺失，是否需要补采后走 INCR |
| 任务长时间 `RUNNING` 不动 | 心跳未刷新、游标不前进 | 看 `gmt_heartbeat`、`locked_until`；`JOB_RECOVER_TIMEOUT` 会回收 |
| 预算一直为 0 / 极小 | 基线缺失或 SLA 压力极端 | 检查 `ingest_stats` 是否有近 30 天数据 |

---

## 9. 监控与页面映射

| 页面 | 路由 | 展示内容 |
|---|---|---|
| 工作流控制与监控 | `/Meta/Finv/Quote/Cdeu/CdeuOverview` | 阶段/Process/JOB 的当前状态与耗时、运行历史、作业进度与处理量、执行日志弹窗、单 JOB 运行、暂停/取消、成熟度重算 |
| 全流程拓扑 | `/Meta/Finv/Quote/Cdeu/CdeuWorkflow` | 每个阶段一个虚线框，框内展开全部 Process，并绘制阶段内与跨阶段的依赖连接 |

页面数据来源：

| 接口 | 用途 |
|---|---|
| `GET /API/V1/Meta/Finv/Quote/DeDup/CDEU/Overview` | 定义树 + 运行树 + 历史运行 |
| `POST /API/V1/Meta/Finv/Quote/DeDup/CDEU/Workflow/Run` | 手动启动完整工作流 |
| `POST /API/V1/Meta/Finv/Quote/DeDup/CDEU/Workflow/Terminate` | 终止运行 |
| `POST .../Job/Run` | 单 JOB / 单动态实例运行 |
| `POST .../Job/Pause`、`POST .../Job/Cancel` | 暂停 / 取消 |
| `POST .../Maturity/Refresh` | 异步重算成熟度 |
| `GET .../Status`、`GET .../Job/Runtime` | 批次进度与运行态 |

---

## 10. 已验证与已知限制

| 项 | 状态 |
|---|---|
| 6 阶段 / 17 Process / 22 JOB 定义入库并以 `ALL` 完成条件逐级评估 | ✅ |
| 动态实例（证券月导出、文件上传）按数据展开并回写状态 | ✅ |
| 预算 + 低峰窗口双重限流，手动触发可绕过 | ✅ |
| 暂停/续跑游标、租约超时回收、上传幂等重传 | ✅ |
| L1/L2/L3 补丁分级与 `scan_watermark` 增量 | ✅ |
| 收口定稿 + 近 6 个月差异报告 | ✅ |
| 数据释放（主表/本地文件）、中转表清理、运行记录保留 | ✅ |
| `JOB_CLEAN_EXPIRED` | ⛔ 已停用（定义保留、调度移除），职责拆到复核阶段三个 JOB |
| 采集写入 | ⛔ 不在平台内（外部程序负责），仅只读体检 |
| 对象存储（S3/R2）直传 | ⛔ 二期；当前为 Git / 目录 / HTTP 三类 uploader |

---

## 11. 源码索引

| 文件 | 职责 |
|---|---|
| `internal/meta/finv/quote/dedup/cdeu.go` | 运行层：状态常量、完成条件、执行上下文、阶段推进、逐级评估入口 |
| `internal/meta/finv/quote/dedup/cdeu_store.go` | 运行表读写、层级评估 SQL、历史运行树、终止与清理 |
| `internal/meta/finv/quote/dedup/cdeu_admin.go` | 页面接口：概览树、运行历史、暂停/取消、成熟度重算 |
| `internal/meta/finv/quote/dedup/job_export.go` | 单证券月 LOAD → EXPORT → CLEANUP 与动态 JOB 编排 |
| `internal/meta/finv/quote/dedup/job_patch.go` | 补丁候选、L1/L2/L3 分级、INCR 写出 |
| `internal/meta/finv/quote/dedup/job_upload.go` | 单文件上传（校验、幂等、审计、退避） |
| `internal/meta/finv/quote/dedup/job_health.go` | 健康指标汇总与告警构建 |
| `internal/meta/finv/quote/dedup/job_stats.go` | 预算计算与统计聚合 |
| `internal/meta/finv/quote/dedup/job_maturity.go` | 收口日与可定稿月份计算 |
| `internal/meta/finv/quote/dedup/naming.go` | 文件名、16 行文件头、数值文本规则 |
| `internal/meta/finv/quote/dedup/store*.go` | 主表/中转表/批次/文件/预算/成熟度/水位的 SQL |
| `internal/meta/finv/quote/dedup/job/*.go` | 22 个 JOB 的执行体（每个 JOB 一个文件，`init()` 注册） |
| `internal/meta/finv/quote/dedup/jobmeta/*.go` | JOB 调度元数据与顺序清单 |
| `internal/meta/finv/quote/dedup/scheduler.go` | 定时触发与窗口/预算前置检查 |
| `internal/meta/finv/quote/mvsv/` | MVSV-1/2 解析与写出（格式契约实现） |
| `Deploy/Migrations/V1000xx__finv_dedup_cdeu_*.sql` | 定义数据与表结构演进 |
