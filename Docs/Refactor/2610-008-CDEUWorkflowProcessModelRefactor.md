# CDEU 工作流模型重构（Workflow → Phase → Process → JOB → Step）

> 状态：已落地（V50）
> 生效迁移：[`V100050__finv_dedup_cdeu_workflow_process_jobstep.sql`](../../Deploy/Migrations/V100050__finv_dedup_cdeu_workflow_process_jobstep.sql)
> 关联设计：[`2610-007-CDEUProcessModelDesign.md`](2610-007-CDEUProcessModelDesign.md)（旧四层模型；本文为 V50 后的权威模型）

## 1. 背景

旧模型为 `Process → Phase → Step → JOB` 四层。JOB 需要继续拆成多个**串行原子 Step** 时，新的 `Step` 与旧 `Step` 名称冲突，因此按以下顺序整体改名：

| 旧概念 | 新概念 | 说明 |
|---|---|---|
| Process（流程） | **Workflow** | 端到端工作流 |
| Phase（阶段） | **Phase** | 保持不变 |
| Step（步骤） | **Process** | 阶段内的可编排处理过程，原字段/编码随之迁移 |
| JOB（作业） | **JOB** | 阶段内的执行单元 |
| — | **Step** | 新增：JOB 内按 `seq_no` 串行执行的原子步骤 |

最终层级：

```text
Workflow（工作流）
  └─ Phase（阶段）
       └─ Process（处理过程）
            └─ JOB（作业）
                 └─ Step（作业内串行原子步骤）
```

## 1.1 当前阶段（V53）

| seq | Phase | 名称 | 说明 |
|---|---|---|---|
| 1 | `PHASE_COLLECT` | 历史行情收集 | 历史行情数据采集（占位 Process） |
| 2 | `PHASE_DEDUP` | 历史行情去重 | 收口冻结与差异报告生成 |
| 3 | `PHASE_DECISION` | 导出决策 | 入口统计与行情成熟度并行，完成后计算预算并做导出条件判定 |
| 4 | `PHASE_EXPORT` | 月度行情导出 | 证券历史月度行情文件导出（`PROCESS_EXPORT_FILE_MON_BASE`）、证券历史月度行情增量文件导出（`PROCESS_EXPORT_FILE_MON_INCR`）、超时回收 |
| 5 | `PHASE_UPLOAD` | 月度行情上传 | 文件上传、生命周期清理、健康检查 |
| 6 | `PHASE_REVIEW` | 归档复核 | 上传完成后的归档复核（当前占位 Process，自动成功） |

`PHASE_EXPORT` 下的 Process 顺序：

| seq | Process | 名称 | 输出 / 输入 |
|---|---|---|---|
| 1 | `PROCESS_EXPORT_BATCH` | 证券导出批次清单获取 | 输出 `dedup.finv_dedup_export_batch` |
| 2 | `PROCESS_EXPORT_FILE_MON_BASE` | 证券历史月度行情文件导出 | 读取批次清单 BASE 条目 |
| 3 | `PROCESS_EXPORT_FILE_MON_INCR` | 证券历史月度行情增量文件导出 | 读取批次清单 INCR 条目 |
| 4 | `PROCESS_RECOVER` | 超时任务回收 | 旁路 |

`PROCESS_EXPORT_BATCH` 的 JOB：

```text
JOB_FETCH_EXPORT_BATCH（证券导出批次清单获取，BATCH）
```

`PROCESS_EXPORT_FILE_MON_BASE` 当前 JOB 顺序：

```text
JOB_FETCH_SECURITY_METADATA（证券元数据获取，META）
→ JOB_EXPORT_LOAD（LOAD）
→ JOB_EXPORT_EXPORT（EXPORT）
→ JOB_EXPORT_REGISTER（REGISTER）
→ JOB_EXPORT_CLEANUP（CLEANUP）
```

批次清单生成后，会按 `(usc, biz_month)` 动态生成单证券 JOB，写入
`dedup.cdeu_job_dynamic`：

- `BASE` / `INCR` 导出动态 JOB → 对应导出 Process，`(usc, biz_month)` 全局唯一；
- 上传动态 JOB → `PROCESS_UPLOAD`，每个已导出且未完成上传的月归档文件一条，按 `file_id` 唯一；
- 导出动态 JOB 内部固定拆为 `STEP_LOAD → STEP_EXPORT → STEP_REGISTER → STEP_CLEANUP` 四个串行 Step；上传动态 JOB 为 `STEP_UPLOAD`；
- 固定 LOAD / EXPORT / REGISTER / CLEANUP / 补丁扫描 / 上传执行壳 JOB 前端隐藏。

批次清单表 `dedup.finv_dedup_export_batch` 字段：

| 字段 | 说明 |
|---|---|
| `batch_no` | 批次号，当前为业务日期 `yyyyMMdd` |
| `biz_date` | 批次业务日期 |
| `usc` | 证券唯一编码 |
| `biz_month` | 导出年月 `yyyyMM` |
| `export_type` | `BASE` / `INCR` |
| `source_job_id` | INCR 对应的导出任务 ID |
| `status` | `READY` / `CONSUMED` |

- BASE 流程通过 `InsertCandidateJobs` 读取 `batch_no + BASE` 条目；
- INCR 流程通过 `PatchCandidates` 读取 `batch_no + INCR` 条目；无当日批次时回退到全量补丁候选。

## 2. 数据库映射（V50）

### 2.1 定义表

| 旧表 | 新表 | 关键字段变化 |
|---|---|---|
| `cdeu_process` | `cdeu_workflow` | `process_id`→`workflow_id`，`process_code`→`workflow_code` |
| `cdeu_phase` | `cdeu_phase` | `process_id`→`workflow_id` |
| `cdeu_step` | `cdeu_process` | `step_id`→`process_id`，`step_code`→`process_code` |
| `cdeu_job` | `cdeu_job` | `step_id`→`process_id` |
| 新增 | `cdeu_job_step` | `step_id`、`job_id`、`step_code`、`seq_no`、`critical`、`affects_completion`、`timeout_seconds`、`enabled` |

### 2.2 运行表

| 旧表 | 新表 | 关键字段变化 |
|---|---|---|
| `cdeu_process_run` | `cdeu_workflow_run` | `process_id`→`workflow_id`，`process_code`→`workflow_code`，`process_version`→`workflow_version` |
| `cdeu_phase_run` | `cdeu_phase_run` | `run_id`→`workflow_run_id` |
| `cdeu_step_run` | `cdeu_process_run` | `step_run_id`→`process_run_id`，`step_id`→`process_id`，`step_code`→`process_code` |
| `cdeu_job_run` | `cdeu_job_run` | `step_run_id`→`process_run_id` |
| 新增 | `cdeu_job_step_run` | `step_run_id`、`job_run_id`、`step_id`、`step_code`、`seq_no`、状态、尝试次数、处理量、异常 |

`cdeu_process` 的原 `STEP_*` 编码统一改为 `PROCESS_*`（例如 `STEP_SEAL`→`PROCESS_SEAL`），避免与新 JOB Step 混淆。

### 2.3 改名顺序

因为 `process` / `process_run` 两个名称会被旧 `step` / `step_run` 接管，V50 严格按以下顺序执行：

1. `cdeu_process_run` → `cdeu_workflow_run`
2. `cdeu_process` → `cdeu_workflow`
3. `cdeu_step_run` → `cdeu_process_run`
4. `cdeu_step` → `cdeu_process`
5. 重命名相关列与索引
6. 新建 `cdeu_job_step` / `cdeu_job_step_run`
7. 为现有 JOB seed 默认 Step

## 3. 当前数据与 JOB Step seed

V50 为现有 13 个 JOB 各建立 1 个默认 Step：

- 内嵌导出阶段 JOB 使用 `STEP_LOAD`、`STEP_EXPORT`、`STEP_REGISTER`、`STEP_CLEANUP`；
- 其他 JOB 使用 `STEP_<JOB_CODE 去掉 JOB_ 前缀>`，例如 `STEP_CALC_INGEST_STATS`、`STEP_MARK_SEALED`。

这只是“每个 JOB 至少一个原子 Step”的基线。后续需要把某个 JOB 拆成多个串行 Step 时，直接向 `cdeu_job_step` 增加行并设置递增 `seq_no` 即可；JOB 执行时会为每个 Step 创建对应的 `cdeu_job_step_run`。

当前运行行为保持不变：

- 原有 10 个调度 Job 仍由调度器触发；
- `RUN_DUE_EXPORT_JOBS` 的 4 个内嵌阶段仍是 4 个 JOB，但每个 JOB 下已登记对应 Step；
- 阶段完成判定仍由 Phase → Process → JOB 逐级汇总，JOB Step 用于记录 JOB 内部原子执行状态，暂不参与上级完成条件计算。

## 4. API 变更

### 4.1 `GET /CDEU/Overview`

响应字段由旧层级改为新层级：

| 旧字段 | 新字段 |
|---|---|
| `process` | `workflow` |
| `phases[].steps` | `phases[].processes` |
| `steps[].step_id/step_code/step_run_id` | `processes[].process_id/process_code/process_run_id` |
| `jobs[]` | 保持 `jobs[]` |
| — | `jobs[].steps[]`：JOB 内 Step 定义与最近一次运行状态 |

运行摘要字段：

- `run.process_code` → `run.workflow_code`
- `run.process_version` → `run.workflow_version`

### 4.2 启动工作流

主路径改为：

```text
POST /API/V1/Meta/Finv/Quote/DeDup/CDEU/Workflow/Run
```

为兼容旧调用，仍保留：

```text
POST /API/V1/Meta/Finv/Quote/DeDup/CDEU/Process/Run
```

两者行为一致。

## 5. 前端表述

`DeDupCDEUView.vue` 已同步调整：

- 页面标题：**CDEU 工作流控制与监控**
- 顶部按钮：**启动工作流**
- KPI：**工作流状态**
- 阶段概览表：**Process 名称 / Process KEY**
- JOB 表：**JOB 名称 / JOB 编码 / Step**
- 阶段详情：按 `Phase → Process → JOB → Step` 层级展示；每个 JOB 下方显示其 Step 状态。
- JOB 进度卡片同时展示 JOB 总数与 Step 总数。

## 6. 兼容与注意事项

- V50 之前的历史运行没有 `cdeu_job_step_run` 记录，API 会按对应 JOB 状态回填 Step 展示状态，不会把历史页面显示成待执行。
- 旧启动路径继续可用，新前端统一使用 `/CDEU/Workflow/Run`。
- 本次未把 4 个导出阶段 JOB 合并为一个 JOB；如后续需要合并，应在新的迁移中单独处理运行数据迁移。
