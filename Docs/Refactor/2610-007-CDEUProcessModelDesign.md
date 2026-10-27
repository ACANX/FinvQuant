# CDEU 端到端流程抽象概念模型设计

> **V100050 起旧模型已被重构**：本文的 `Process → Phase → Step → JOB` 为旧命名；新模型为 `Workflow → Phase → Process → JOB → Step`，以 [`2610-008-CDEUWorkflowProcessModelRefactor.md`](2610-008-CDEUWorkflowProcessModelRefactor.md) 为准。以下内容仅保留历史设计追溯价值。

> 所属：FinvQuant · 存放：`Docs/Refactor/`
> 状态：设计稿（v0.1，2026-10）
> 范围：为「数据采集（Collect）→ 去重处理（Dedup）→ 导出（Export）→ 上传（Upload）」建立统一的流程/阶段/步骤/作业抽象模型、运行模型、完成条件与登记表结构。
> 现有实现参考：`internal/meta/finv/quote/dedup`、[`2610-005-FinvQuoteDeDubExportUpload.md`](2610-005-FinvQuoteDeDubExportUpload.md)。

---

## 1. 目标与边界

### 1.1 目标

1. 用 **Process（流程）→ Phase（阶段）→ Step（步骤）→ Job（作业）** 四层模型描述 CDEU 全链路；
2. 每个层级都有**唯一编码、顺序、关键性、批处理能力、完成条件**；
3. 流程支持 **每日定时自动运行** 与 **手动触发** 两种模式；
4. 阶段/步骤/作业均支持**顺序编排**与**批处理实例**；
5. 完成条件支持三类语义：
   - 全部完成；
   - 关键节点全部完成；
   - 至少完成指定数量的阶段/步骤/作业；
6. 用表结构登记定义、运行实例、批处理实例与完成快照；
7. 当前 13 个作业执行单元全部结构化登记，并与已有 10 个调度 Job 保持兼容。

### 1.2 非目标

- 本设计**不改变**现有 10 个 Job 的业务实现与调度行为；
- 不在本阶段实现完整工作流引擎（BPMN/外部调度组件）；
- 不引入跨库事务；沿用「无外键、逻辑关联」的既有约定。

### 1.3 术语约定

| 术语 | 含义 |
|---|---|
| Process | 一条端到端流程，如 `CDEU_DAILY` |
| Phase | 流程中的大阶段，如采集、去重、导出、上传、保障 |
| Step | 阶段中的可编排步骤，如「单证券月导出」 |
| Job | 步骤中的最小可执行单元，对应一个 `job_kind` + 执行阶段 |
| Task | **运行时实例**：批处理步骤展开后的单个执行项；不单独建定义表，登记在 `cdeu_job_run.batch_key` |
| Definition | `process/phase/step/job` 四张定义表 |
| Run | `*_run` 四张运行实例表 |

> 说明：需求中「任务」与「作业」在不同语境下有混用；本方案统一为：**Job 是定义，Task 是 Job 的批处理运行实例**。

---

## 2. 概念模型

### 2.1 层级关系

```
Process（流程）
└── Phase（阶段，按 seq_no 顺序执行）
    └── Step（步骤，按 seq_no 顺序执行）
        └── Job（作业，按 seq_no 顺序执行）
            └── JobRun（批处理实例，按 batch_key 展开）
```

约束：

- 同一父级下 `code` 唯一、`seq_no` 唯一；
- `Phase/Step/Job` 均支持 `critical`（关键性）与 `affects_completion`（是否参与完成判定）；
- 阶段与阶段、步骤与步骤、作业与作业均**串行有序**；
- 批处理只发生在 Step/Job 层：Step 定义批处理来源与并发度，Job 为每个批次项生成一个 `JobRun`；
- 阶段本身不直接批处理，但可包含多个批处理步骤。

### 2.2 定义与运行分离

| 定义表 | 运行表 | 关系 |
|---|---|---|
| `cdeu_process` | `cdeu_process_run` | 一次流程运行快照 |
| `cdeu_phase` | `cdeu_phase_run` | 按阶段顺序展开 |
| `cdeu_step` | `cdeu_step_run` | 按步骤顺序展开；记录批处理计数 |
| `cdeu_job` | `cdeu_job_run` | 按作业顺序展开；每条批处理项一行 |

运行表保存 `*_code`、`*_name`、`seq_no`、`completion_rule_snapshot` 等快照字段，避免定义变更影响历史运行审计。

### 2.3 运行状态

```
PENDING → READY → RUNNING → SUCCESS
                          ├→ PARTIAL
                          ├→ FAILED
                          ├→ PAUSED
                          ├→ CANCELLED
                          └→ SKIPPED
```

- `SUCCESS`：完成条件满足且无失败项；
- `PARTIAL`：完成条件满足但存在失败/跳过项；
- `FAILED`：完成条件不满足且已无运行中项；
- `PAUSED`：人工或业务暂停；
- `CANCELLED`：人工取消；
- `SKIPPED`：前置条件不满足或窗口外跳过。

---

## 3. 运行模式

### 3.1 流程级运行模式

| 字段 | 取值 | 说明 |
|---|---|---|
| `run_mode` | `AUTO_DAILY` / `MANUAL` / `BOTH` | 每日定时 / 仅手动 / 两者都支持 |
| `schedule_time` | `HH:MM:SS` | 每日触发时间（业务时区） |
| `schedule_timezone` | 默认 `FINV_QUOTE_DEDUB_TIMEZONE` | 调度/业务日时区 |
| `schedule_enabled` | bool | 定时触发开关 |
| `run_key` | 文本 | 幂等键：自动运行 `process_code:biz_date:SCHEDULE`；手动运行追加时间戳/ULID，允许同日多次补跑 |

- `AUTO_DAILY`：调度器每天在 `schedule_time` 检查并创建 `process_run`；业务日 `biz_date` 按 `FINV_QUOTE_DEDUB_TIMEZONE` 计算；
- `MANUAL`：只允许 `POST /CDEU/Process/Run` 创建运行；
- `BOTH`：两种方式均可；
- 同一流程同一业务日只允许一个 `AUTO_DAILY` 运行（`run_key` 唯一）；
- 手动运行使用独立 `run_key`，允许补跑，但同一时刻只允许一个 `RUNNING` 实例（部分唯一索引 + advisory lock）。
- 当前已实现的是 **Job 级手动补跑**：流程已有当日终态时只登记只读 `cdeu_job_run`，不新建/重开流程；**Process 级手动运行接口尚未开放**。

### 3.2 阶段/步骤级运行模式

为兼容「保障类作业」（超时回收、健康检查每 5/15 分钟运行）与主链路不同，阶段/步骤增加：

| 字段 | 取值 | 说明 |
|---|---|---|
| `run_mode` | `INHERIT` / `CONTINUOUS` / `MANUAL` | 继承流程 / 独立循环 / 仅手动 |
| `affects_completion` | bool | 是否参与流程完成判定 |

- `INHERIT`：随流程运行按顺序执行；
- `CONTINUOUS`：由自身周期独立执行，不阻塞主流程完成；
- `MANUAL`：只能人工触发；
- `STEP_UPLOAD`、`STEP_RECOVER`、`STEP_HEALTH` 为 `run_mode=CONTINUOUS` 的旁路步骤；其中 `STEP_RECOVER` 挂在导出阶段、`STEP_HEALTH` 挂在上传阶段，与其正常步骤并行展示，`affects_completion=false`，不参与每日 CDEU 完成判定。

### 3.3 作业触发模式

| 字段 | 取值 | 说明 |
|---|---|---|
| `trigger_mode` | `SCHEDULED` / `EMBEDDED` / `MANUAL` | 定时 / 由父作业阶段内嵌 / 手动 |
| `schedule_kind` | `DAILY` / `INTERVAL` / `NONE` | 每日 / 固定间隔 / 无 |
| `schedule_value` | 文本 | `HH:MM` 或秒数 |
| `execution_mode` | `DIRECT` / `EMBEDDED` | 直接调用 Job / 由父 Job 的阶段驱动 |

**关于 `RUN_DUE_EXPORT_JOBS`**：现有实现是一个调度 Job，内部执行 `LOAD → EXPORT → 登记 → CLEANUP`。本模型拆成 4 个登记作业（`execution_mode=EMBEDDED`、`execution_stage` 区分），由父作业运行时上报对应阶段；其中 `REGISTER` 是逻辑阶段（当前实现归入 EXPORT）。5–8 支持单阶段手动重跑（`RunStage`），不影响正常流程执行。

---

## 4. 批处理模型

### 4.1 步骤批处理

`cdeu_phase.batch_enabled` 表示该阶段包含批处理步骤；具体批次来源、并发度和失败策略定义在步骤上。

| 字段 | 说明 |
|---|---|
| `batch_enabled` | 是否启用批处理 |
| `batch_source` | 批次来源：`EXPORT_JOB_CANDIDATES` / `EXPORT_FILES` / `PATCH_CANDIDATES` / `USC_LIST` / `CUSTOM_SQL` |
| `batch_key_field` | 批次键字段（如 `usc,biz_month`） |
| `batch_size` | 单批大小 |
| `max_parallel` | 最大并发批次数 |
| `failure_policy` | `FAIL_FAST`（失败即停）/ `CONTINUE`（继续其余批次） |

非批处理步骤按 `batch_total=1` 记录；批处理步骤按批次键展开，每个 Job 在每个批次键上生成一条 `cdeu_job_run`。

运行展开：

```
cdeu_step_run (batch_total/success/failed)
  ├── cdeu_job_run (batch_key=US.AAPL:202604)
  ├── cdeu_job_run (batch_key=US.NVDA:202604)
  └── ...
```

### 4.2 批处理完成判定

- 步骤完成条件按**批次实例**统计；
- `AT_LEAST` 支持 `min_count` 与 `min_ratio`；
- `FAIL_FAST` 时，首个关键批次失败即步骤失败；其余批次置 `SKIPPED`；
- `CONTINUE` 时，全部批次结束后按规则判定 `SUCCESS/PARTIAL/FAILED`。

---

## 5. 完成条件模型

### 5.1 规则 JSON

在每个定义的 `completion_rule` (jsonb) 中保存；运行表保存快照。

```json
{
  "mode": "ALL | CRITICAL_ALL | AT_LEAST | ANY_OF",
  "items": ["STEP_EXPORT_JOB", "STEP_UPLOAD"],
  "min_count": 2,
  "min_ratio": 0.8,
  "count_mode": "SUCCESS_ONLY | TERMINAL"
}
```

| 字段 | 必填 | 说明 |
|---|---|---|
| `mode` | 是 | 完成模式 |
| `items` | 否 | 参与判定的子级 code；省略表示全部子级 |
| `min_count` | AT_LEAST 时与 `min_ratio` 至少填一个 | 至少完成的子级数量 |
| `min_ratio` | 否 | 至少完成比例（批处理常用） |
| `count_mode` | 否 | `SUCCESS_ONLY`（默认，只计成功）/ `TERMINAL`（终态即计完成） |

### 5.2 三种语义

| 需求 | 规则 |
|---|---|
| 全部阶段/任务完成 | `{"mode":"ALL"}` |
| 关键阶段/步骤/任务完成 | `{"mode":"CRITICAL_ALL"}` |
| 至少完成某几个阶段/步骤/任务 | `{"mode":"AT_LEAST","items":["STEP_A","STEP_B"],"min_count":2}` 或 `{"mode":"ANY_OF","items":[...]}` |

### 5.3 默认规则

| 层级 | 默认 |
|---|---|
| Job | 运行结果 `SUCCESS` |
| Step | `{"mode":"ALL"}`，统计其 JobRun |
| Phase | `{"mode":"ALL"}`，统计 `affects_completion=true` 的 StepRun |
| Process | `{"mode":"ALL"}`，统计 `affects_completion=true` 的 PhaseRun |

### 5.4 判定算法（伪代码）

```
evaluate(owner, childRuns, rule):
    children = childRuns where affects_completion = true
    if any running/pending: return RUNNING
    completed = children where status in (SUCCESS) or (count_mode=TERMINAL and terminal)
    failedCritical = any(child.critical and child.status in (FAILED, CANCELLED))
    if failedCritical: return FAILED
    switch rule.mode:
      ALL:           ok = all(children in completed)
      CRITICAL_ALL:  ok = all(children where critical in completed)
      AT_LEAST:      ok = (min_count is null or count(completed in items) >= min_count)
                     and (min_ratio is null or count(completed in items) / total >= min_ratio)
      ANY_OF:        ok = any(completed in rule.items)
    if !ok:
        return FAILED if no running else RUNNING
    return SUCCESS if no failed/skipped else PARTIAL
```

> `PARTIAL` 用于「完成条件已满足，但存在非关键失败」的审计场景；关键项失败始终 `FAILED`。

---

## 6. 现有 CDEU 流程映射

### 6.1 阶段划分

| 阶段 | 编码 | 职责 | 关键 | 参与完成 |
|---|---|---|---|---|
| 采集 | `PHASE_COLLECT` | 历史行情采集（占位） | 是 | 是 |
| 去重收口 | `PHASE_DEDUP` | 收口冻结 | 是 | 是 |
| 导出决策 | `PHASE_DECISION` | 入口统计、当日预算、成熟度扫描、导出前条件检查与决策 | 是 | 是 |
| 导出 | `PHASE_EXPORT` | 单证券月导出、收口后补丁 | 是 | 是 |
| 上传 | `PHASE_UPLOAD` | 文件上传、生命周期清理、健康检查（旁路步骤） | 是 | 是 |

> 当前系统的「去重」在数据入库前已完成，`PHASE_DEDUP` 表示对去重主表的成熟度校验与收口冻结。

### 6.2 步骤与 13 个作业登记

| # | 阶段 | 步骤 | Job 编码 | 实际 `job_kind` | 执行阶段 | 触发方式 | 顺序 |
|---|---|---|---|---|---|---|---|
| — | 采集 | `STEP_HISTORY_INGEST` 历史行情数据采集（占位） | —（无 JOB） | — | - | 初始化自动成功 | 1 |
| 1 | 导出决策 | `STEP_INGEST_STATS` 入口统计 | `JOB_CALC_INGEST_STATS` | `CALC_DAILY_INGEST_STATS` | - | DIRECT/DAILY 00:05 | 1 |
| 2 | 导出决策 | `STEP_BUDGET` 预算计算 | `JOB_CALC_BUDGET` | `CALC_DAILY_EXPORT_BUDGET` | - | DIRECT/DAILY 00:05 | 2 |
| 3 | 导出决策 | `STEP_MATURITY` 成熟度扫描 | `JOB_REFRESH_MATURITY` | `REFRESH_QUOTE_MATURITY` | - | DIRECT/DAILY 00:10 | 3 |
| — | 导出决策 | `STEP_EXPORT_DECISION` 导出条件判定（占位） | —（无 JOB） | — | - | 初始化自动成功 | 4 |
| 4 | 去重收口 | `STEP_SEAL` 收口冻结 | `JOB_MARK_SEALED` | `MARK_SEALED_JOBS_FROZEN` | - | DIRECT/DAILY 00:20 | 1 |
| 5 | 导出 | `STEP_EXPORT_JOB` 单证券月导出 | `JOB_EXPORT_LOAD` | `RUN_DUE_EXPORT_JOBS` | `LOAD` | EMBEDDED/可手动重跑 | 1 |
| 6 | 导出 | 同上 | `JOB_EXPORT_EXPORT` | `RUN_DUE_EXPORT_JOBS` | `EXPORT` | EMBEDDED/可手动重跑 | 2 |
| 7 | 导出 | 同上 | `JOB_EXPORT_REGISTER` | `RUN_DUE_EXPORT_JOBS` | `REGISTER` | EMBEDDED/可手动重跑 | 3 |
| 8 | 导出 | 同上 | `JOB_EXPORT_CLEANUP` | `RUN_DUE_EXPORT_JOBS` | `CLEANUP` | EMBEDDED/可手动重跑 | 4 |
| 9 | 导出 | `STEP_PATCH` 收口后补丁 | `JOB_SCAN_PATCH` | `SCAN_POST_SEAL_PATCHES` | - | DIRECT/DAILY 03:00 | 2 |
| 10 | 导出 | `STEP_RECOVER` 超时回收（旁路） | `JOB_RECOVER_TIMEOUT` | `RECOVER_TIMEOUT_EXPORT_JOBS` | - | DIRECT/INTERVAL 300s | 3 |
| 11 | 上传 | `STEP_UPLOAD` 文件上传 | `JOB_UPLOAD_FILES` | `UPLOAD_PENDING_EXPORT_FILES` | - | DIRECT/INTERVAL 300s | 1 |
| 12 | 上传 | `STEP_LIFECYCLE_CLEAN` 生命周期清理 | `JOB_CLEAN_EXPIRED` | `CLEAN_EXPIRED_EXPORT_ARTIFACTS` | - | DIRECT/DAILY 06:00 | 2 |
| 13 | 上传 | `STEP_HEALTH` 健康检查（旁路） | `JOB_CHECK_HEALTH` | `CHECK_EXPORT_JOB_HEALTH` | - | DIRECT/INTERVAL 900s | 3 |

> 说明：表中 13 个 Job 行对应 10 个实际调度 Job；`STEP_HISTORY_INGEST` 与 `STEP_EXPORT_DECISION` 为无 JOB 的占位步骤，运行初始化时自动置为 SUCCESS；第 5–8 行是 `RUN_DUE_EXPORT_JOBS` 的 4 个阶段拆分登记。⑩ 网盘归档按自动完成处理，但当前实现尚未提供对应能力，本模型暂不登记、不参与完成判定。`STEP_RECOVER` / `STEP_HEALTH` 不是独立尾阶段，而是分别挂在导出 / 上传阶段的旁路步骤。

### 6.3 阶段完成条件示例

```json
// 流程：全部阶段完成（默认）
{"mode":"ALL"}

// 导出决策阶段：入口统计、预算、导出条件判定都完成
{"mode":"ALL"}

// 导出阶段：单证券月导出与补丁步骤都完成
{"mode":"ALL"}

// 或：至少完成单证券月导出步骤（补丁非关键）
{"mode":"AT_LEAST","items":["STEP_EXPORT_JOB"],"min_count":1}

// 上传阶段：文件上传与生命周期清理都完成
{"mode":"ALL"}

// 导出步骤：4 个内嵌作业全部成功
{"mode":"ALL"}

// 批处理步骤：至少 80% 批次成功
{"mode":"AT_LEAST","min_ratio":0.8,"count_mode":"SUCCESS_ONLY"}
```

---

## 7. 表结构设计（复用 `dedup` schema）

> 所有表建在现有 `dedup` schema，表名统一 `cdeu_*`；不加外键，通过 `*_id` 逻辑关联。  
> 每个字段均带中文 `COMMENT ON COLUMN`，便于数据字典与运维查询。  
> 业务日与调度时区统一使用 `FINV_QUOTE_DEDUB_TIMEZONE`。

### 7.1 定义表

```sql
-- 流程定义
CREATE TABLE IF NOT EXISTS dedup.cdeu_process (
    process_id          bigserial PRIMARY KEY,
    process_code        text NOT NULL,
    name                text NOT NULL,
    description         text,
    version             int  NOT NULL DEFAULT 1,
    run_mode            text NOT NULL DEFAULT 'MANUAL'
                        CHECK (run_mode IN ('AUTO_DAILY','MANUAL','BOTH')),
    schedule_time       time,
    schedule_timezone   text NOT NULL DEFAULT 'Asia/Shanghai',
    schedule_enabled    boolean NOT NULL DEFAULT false,
    completion_rule     jsonb NOT NULL DEFAULT '{"mode":"ALL"}'::jsonb,
    enabled             boolean NOT NULL DEFAULT true,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (process_code, version)
);
COMMENT ON TABLE dedup.cdeu_process IS 'CDEU 流程定义：一条端到端流程（采集→去重→导出→上传）';
COMMENT ON COLUMN dedup.cdeu_process.process_id IS '流程主键';
COMMENT ON COLUMN dedup.cdeu_process.process_code IS '流程编码，如 CDEU_DAILY';
COMMENT ON COLUMN dedup.cdeu_process.name IS '流程中文名称';
COMMENT ON COLUMN dedup.cdeu_process.description IS '流程说明';
COMMENT ON COLUMN dedup.cdeu_process.version IS '定义版本号，同编码可多版本';
COMMENT ON COLUMN dedup.cdeu_process.run_mode IS '运行模式：AUTO_DAILY 每日定时 / MANUAL 手动 / BOTH 两者';
COMMENT ON COLUMN dedup.cdeu_process.schedule_time IS '每日自动触发时间（按 schedule_timezone 解释）';
COMMENT ON COLUMN dedup.cdeu_process.schedule_timezone IS '调度时区，默认取 FINV_QUOTE_DEDUB_TIMEZONE';
COMMENT ON COLUMN dedup.cdeu_process.schedule_enabled IS '是否启用每日自动触发';
COMMENT ON COLUMN dedup.cdeu_process.completion_rule IS '完成条件 JSON：ALL / CRITICAL_ALL / AT_LEAST / ANY_OF，默认 ALL';
COMMENT ON COLUMN dedup.cdeu_process.enabled IS '流程定义是否启用';
COMMENT ON COLUMN dedup.cdeu_process.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_process.gmt_update IS '更新时间';

-- 阶段定义
CREATE TABLE IF NOT EXISTS dedup.cdeu_phase (
    phase_id            bigserial PRIMARY KEY,
    process_id          bigint NOT NULL,
    phase_code          text NOT NULL,
    name                text NOT NULL,
    seq_no              int  NOT NULL,
    critical            boolean NOT NULL DEFAULT true,
    affects_completion  boolean NOT NULL DEFAULT true,
    run_mode            text NOT NULL DEFAULT 'INHERIT'
                        CHECK (run_mode IN ('INHERIT','CONTINUOUS','MANUAL')),
    batch_enabled       boolean NOT NULL DEFAULT false,
    completion_rule     jsonb NOT NULL DEFAULT '{"mode":"ALL"}'::jsonb,
    enabled             boolean NOT NULL DEFAULT true,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (process_id, phase_code),
    UNIQUE (process_id, seq_no)
);
COMMENT ON TABLE dedup.cdeu_phase IS 'CDEU 阶段定义：流程下的大阶段';
COMMENT ON COLUMN dedup.cdeu_phase.phase_id IS '阶段主键';
COMMENT ON COLUMN dedup.cdeu_phase.process_id IS '所属流程 ID，逻辑关联 cdeu_process.process_id';
COMMENT ON COLUMN dedup.cdeu_phase.phase_code IS '阶段编码，如 PHASE_EXPORT';
COMMENT ON COLUMN dedup.cdeu_phase.name IS '阶段中文名称';
COMMENT ON COLUMN dedup.cdeu_phase.seq_no IS '阶段执行顺序，数值小的先执行';
COMMENT ON COLUMN dedup.cdeu_phase.critical IS '是否关键阶段，关键阶段失败或未完成会使流程失败';
COMMENT ON COLUMN dedup.cdeu_phase.affects_completion IS '是否参与流程完成判定';
COMMENT ON COLUMN dedup.cdeu_phase.run_mode IS '运行模式：INHERIT 随流程 / CONTINUOUS 独立循环 / MANUAL 手动';
COMMENT ON COLUMN dedup.cdeu_phase.batch_enabled IS '该阶段是否包含批处理步骤';
COMMENT ON COLUMN dedup.cdeu_phase.completion_rule IS '阶段完成条件 JSON，默认全部步骤完成';
COMMENT ON COLUMN dedup.cdeu_phase.enabled IS '阶段定义是否启用';
COMMENT ON COLUMN dedup.cdeu_phase.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_phase.gmt_update IS '更新时间';

-- 步骤定义
CREATE TABLE IF NOT EXISTS dedup.cdeu_step (
    step_id             bigserial PRIMARY KEY,
    phase_id            bigint NOT NULL,
    step_code           text NOT NULL,
    name                text NOT NULL,
    seq_no              int  NOT NULL,
    critical            boolean NOT NULL DEFAULT true,
    affects_completion  boolean NOT NULL DEFAULT true,
    run_mode            text NOT NULL DEFAULT 'INHERIT'
                        CHECK (run_mode IN ('INHERIT','CONTINUOUS','MANUAL')),
    batch_enabled       boolean NOT NULL DEFAULT false,
    batch_source        text,
    batch_key_field     text,
    batch_size          int,
    max_parallel        int NOT NULL DEFAULT 1,
    failure_policy      text NOT NULL DEFAULT 'CONTINUE'
                        CHECK (failure_policy IN ('FAIL_FAST','CONTINUE')),
    completion_rule     jsonb NOT NULL DEFAULT '{"mode":"ALL"}'::jsonb,
    enabled             boolean NOT NULL DEFAULT true,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (phase_id, step_code),
    UNIQUE (phase_id, seq_no)
);
COMMENT ON TABLE dedup.cdeu_step IS 'CDEU 步骤定义：阶段下的可编排步骤，支持批处理';
COMMENT ON COLUMN dedup.cdeu_step.step_id IS '步骤主键';
COMMENT ON COLUMN dedup.cdeu_step.phase_id IS '所属阶段 ID，逻辑关联 cdeu_phase.phase_id';
COMMENT ON COLUMN dedup.cdeu_step.step_code IS '步骤编码，如 STEP_EXPORT_JOB';
COMMENT ON COLUMN dedup.cdeu_step.name IS '步骤中文名称';
COMMENT ON COLUMN dedup.cdeu_step.seq_no IS '步骤执行顺序，数值小的先执行';
COMMENT ON COLUMN dedup.cdeu_step.critical IS '是否关键步骤，关键步骤失败或未完成会使阶段失败';
COMMENT ON COLUMN dedup.cdeu_step.affects_completion IS '是否参与阶段完成判定';
COMMENT ON COLUMN dedup.cdeu_step.run_mode IS '运行模式：INHERIT 随阶段 / CONTINUOUS 独立循环 / MANUAL 手动';
COMMENT ON COLUMN dedup.cdeu_step.batch_enabled IS '是否按批次展开执行';
COMMENT ON COLUMN dedup.cdeu_step.batch_source IS '批次来源：EXPORT_JOB_CANDIDATES / EXPORT_FILES / PATCH_CANDIDATES / USC_LIST / CUSTOM_SQL';
COMMENT ON COLUMN dedup.cdeu_step.batch_key_field IS '批次键字段，如 usc,biz_month';
COMMENT ON COLUMN dedup.cdeu_step.batch_size IS '单批处理条数';
COMMENT ON COLUMN dedup.cdeu_step.max_parallel IS '最大并发批次数';
COMMENT ON COLUMN dedup.cdeu_step.failure_policy IS '批处理失败策略：CONTINUE 继续其余批次（默认）/ FAIL_FAST 失败即停';
COMMENT ON COLUMN dedup.cdeu_step.completion_rule IS '步骤完成条件 JSON，默认全部作业完成';
COMMENT ON COLUMN dedup.cdeu_step.enabled IS '步骤定义是否启用';
COMMENT ON COLUMN dedup.cdeu_step.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_step.gmt_update IS '更新时间';

-- 作业定义
CREATE TABLE IF NOT EXISTS dedup.cdeu_job (
    job_id              bigserial PRIMARY KEY,
    step_id             bigint NOT NULL,
    job_code            text NOT NULL,
    name                text NOT NULL,
    job_kind            text NOT NULL,
    execution_stage     text,
    seq_no              int  NOT NULL,
    critical            boolean NOT NULL DEFAULT true,
    affects_completion  boolean NOT NULL DEFAULT true,
    trigger_mode        text NOT NULL DEFAULT 'SCHEDULED'
                        CHECK (trigger_mode IN ('SCHEDULED','EMBEDDED','MANUAL')),
    execution_mode      text NOT NULL DEFAULT 'DIRECT'
                        CHECK (execution_mode IN ('DIRECT','EMBEDDED')),
    manual_rerun_enabled boolean NOT NULL DEFAULT true,
    schedule_kind       text NOT NULL DEFAULT 'NONE'
                        CHECK (schedule_kind IN ('DAILY','INTERVAL','NONE')),
    schedule_value      text,
    timeout_seconds     int,
    max_attempts        int NOT NULL DEFAULT 1,
    retry_backoff_seconds int NOT NULL DEFAULT 60,
    job_params          jsonb NOT NULL DEFAULT '{}'::jsonb,
    enabled             boolean NOT NULL DEFAULT true,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (step_id, job_code),
    UNIQUE (step_id, seq_no)
);
COMMENT ON TABLE dedup.cdeu_job IS 'CDEU 作业定义：步骤下的可执行作业，按 seq_no 顺序执行';
COMMENT ON COLUMN dedup.cdeu_job.job_id IS '作业主键';
COMMENT ON COLUMN dedup.cdeu_job.step_id IS '所属步骤 ID，逻辑关联 cdeu_step.step_id';
COMMENT ON COLUMN dedup.cdeu_job.job_code IS '作业编码，如 JOB_EXPORT_LOAD';
COMMENT ON COLUMN dedup.cdeu_job.name IS '作业中文名称';
COMMENT ON COLUMN dedup.cdeu_job.job_kind IS '实际调度标识，如 RUN_DUE_EXPORT_JOBS';
COMMENT ON COLUMN dedup.cdeu_job.execution_stage IS '内嵌执行阶段：LOAD / EXPORT / REGISTER / CLEANUP；不拆分则为空';
COMMENT ON COLUMN dedup.cdeu_job.seq_no IS '作业执行顺序，数值小的先执行';
COMMENT ON COLUMN dedup.cdeu_job.critical IS '是否关键作业，关键作业失败或未完成会使步骤失败';
COMMENT ON COLUMN dedup.cdeu_job.affects_completion IS '是否参与步骤完成判定';
COMMENT ON COLUMN dedup.cdeu_job.trigger_mode IS '触发方式：SCHEDULED 定时 / EMBEDDED 父作业内嵌 / MANUAL 手动';
COMMENT ON COLUMN dedup.cdeu_job.execution_mode IS '执行方式：DIRECT 直接调用 / EMBEDDED 由父作业阶段驱动';
COMMENT ON COLUMN dedup.cdeu_job.manual_rerun_enabled IS '是否允许单阶段手动重跑（5–8 导出阶段默认开启）';
COMMENT ON COLUMN dedup.cdeu_job.schedule_kind IS '调度类型：DAILY 每日 / INTERVAL 固定间隔 / NONE 无';
COMMENT ON COLUMN dedup.cdeu_job.schedule_value IS '调度值：DAILY 为 HH:MM，INTERVAL 为秒数';
COMMENT ON COLUMN dedup.cdeu_job.timeout_seconds IS '单次执行超时秒数，空表示不限制';
COMMENT ON COLUMN dedup.cdeu_job.max_attempts IS '最大尝试次数';
COMMENT ON COLUMN dedup.cdeu_job.retry_backoff_seconds IS '重试退避基数（秒）';
COMMENT ON COLUMN dedup.cdeu_job.job_params IS '作业参数 JSON';
COMMENT ON COLUMN dedup.cdeu_job.enabled IS '作业定义是否启用';
COMMENT ON COLUMN dedup.cdeu_job.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_job.gmt_update IS '更新时间';
```

### 7.2 运行表

```sql
-- 流程运行
CREATE TABLE IF NOT EXISTS dedup.cdeu_process_run (
    run_id              bigserial PRIMARY KEY,
    process_id          bigint NOT NULL,
    process_code        text NOT NULL,
    process_version     int  NOT NULL,
    run_key             text NOT NULL,
    biz_date            date NOT NULL,
    trigger_type        text NOT NULL
                        CHECK (trigger_type IN ('SCHEDULE','MANUAL','CONTINUOUS')),
    status              text NOT NULL DEFAULT 'PENDING'
                        CHECK (status IN ('PENDING','READY','RUNNING','SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')),
    scheduled_at        timestamptz,
    started_at          timestamptz,
    finished_at         timestamptz,
    current_phase_id    bigint,
    completion_rule_snapshot jsonb,
    completion_snapshot jsonb,
    operator            text,
    error               text,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_key)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_cdeuprocessrun_running
    ON dedup.cdeu_process_run (process_id)
    WHERE status IN ('PENDING','READY','RUNNING');
COMMENT ON TABLE dedup.cdeu_process_run IS 'CDEU 流程运行实例';
COMMENT ON COLUMN dedup.cdeu_process_run.run_id IS '流程运行主键';
COMMENT ON COLUMN dedup.cdeu_process_run.process_id IS '流程定义 ID，逻辑关联 cdeu_process.process_id';
COMMENT ON COLUMN dedup.cdeu_process_run.process_code IS '流程编码快照';
COMMENT ON COLUMN dedup.cdeu_process_run.process_version IS '流程定义版本快照';
COMMENT ON COLUMN dedup.cdeu_process_run.run_key IS '幂等键：自动 process_code:biz_date:SCHEDULE；手动追加时间戳/ULID';
COMMENT ON COLUMN dedup.cdeu_process_run.biz_date IS '业务日期，按 FINV_QUOTE_DEDUB_TIMEZONE 计算';
COMMENT ON COLUMN dedup.cdeu_process_run.trigger_type IS '触发类型：SCHEDULE 定时 / MANUAL 手动 / CONTINUOUS 连续循环';
COMMENT ON COLUMN dedup.cdeu_process_run.status IS '运行状态';
COMMENT ON COLUMN dedup.cdeu_process_run.scheduled_at IS '计划触发时间';
COMMENT ON COLUMN dedup.cdeu_process_run.started_at IS '实际开始时间';
COMMENT ON COLUMN dedup.cdeu_process_run.finished_at IS '结束时间';
COMMENT ON COLUMN dedup.cdeu_process_run.current_phase_id IS '当前执行阶段 ID';
COMMENT ON COLUMN dedup.cdeu_process_run.completion_rule_snapshot IS '完成条件快照';
COMMENT ON COLUMN dedup.cdeu_process_run.completion_snapshot IS '完成判定结果快照';
COMMENT ON COLUMN dedup.cdeu_process_run.operator IS '手动触发人';
COMMENT ON COLUMN dedup.cdeu_process_run.error IS '失败原因';
COMMENT ON COLUMN dedup.cdeu_process_run.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_process_run.gmt_update IS '更新时间';

-- 阶段运行
CREATE TABLE IF NOT EXISTS dedup.cdeu_phase_run (
    phase_run_id        bigserial PRIMARY KEY,
    run_id              bigint NOT NULL,
    phase_id            bigint NOT NULL,
    phase_code          text NOT NULL,
    seq_no              int NOT NULL,
    status              text NOT NULL DEFAULT 'PENDING'
                        CHECK (status IN ('PENDING','READY','RUNNING','SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')),
    started_at          timestamptz,
    finished_at         timestamptz,
    success_count       int NOT NULL DEFAULT 0,
    failed_count        int NOT NULL DEFAULT 0,
    total_count         int NOT NULL DEFAULT 0,
    completion_rule_snapshot jsonb,
    completion_snapshot jsonb,
    error               text,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_id, phase_code)
);
COMMENT ON TABLE dedup.cdeu_phase_run IS 'CDEU 阶段运行实例';
COMMENT ON COLUMN dedup.cdeu_phase_run.phase_run_id IS '阶段运行主键';
COMMENT ON COLUMN dedup.cdeu_phase_run.run_id IS '流程运行 ID，逻辑关联 cdeu_process_run.run_id';
COMMENT ON COLUMN dedup.cdeu_phase_run.phase_id IS '阶段定义 ID';
COMMENT ON COLUMN dedup.cdeu_phase_run.phase_code IS '阶段编码快照';
COMMENT ON COLUMN dedup.cdeu_phase_run.seq_no IS '阶段顺序快照';
COMMENT ON COLUMN dedup.cdeu_phase_run.status IS '阶段运行状态';
COMMENT ON COLUMN dedup.cdeu_phase_run.started_at IS '开始时间';
COMMENT ON COLUMN dedup.cdeu_phase_run.finished_at IS '结束时间';
COMMENT ON COLUMN dedup.cdeu_phase_run.success_count IS '成功步骤数';
COMMENT ON COLUMN dedup.cdeu_phase_run.failed_count IS '失败步骤数';
COMMENT ON COLUMN dedup.cdeu_phase_run.total_count IS '参与判定步骤总数';
COMMENT ON COLUMN dedup.cdeu_phase_run.completion_rule_snapshot IS '阶段完成条件快照';
COMMENT ON COLUMN dedup.cdeu_phase_run.completion_snapshot IS '阶段完成判定快照';
COMMENT ON COLUMN dedup.cdeu_phase_run.error IS '失败原因';
COMMENT ON COLUMN dedup.cdeu_phase_run.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_phase_run.gmt_update IS '更新时间';

-- 步骤运行
CREATE TABLE IF NOT EXISTS dedup.cdeu_step_run (
    step_run_id         bigserial PRIMARY KEY,
    phase_run_id        bigint NOT NULL,
    step_id             bigint NOT NULL,
    step_code           text NOT NULL,
    seq_no              int NOT NULL,
    status              text NOT NULL DEFAULT 'PENDING'
                        CHECK (status IN ('PENDING','READY','RUNNING','SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')),
    started_at          timestamptz,
    finished_at         timestamptz,
    batch_total         int NOT NULL DEFAULT 0,
    batch_success       int NOT NULL DEFAULT 0,
    batch_failed        int NOT NULL DEFAULT 0,
    completion_rule_snapshot jsonb,
    completion_snapshot jsonb,
    error               text,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (phase_run_id, step_code)
);
COMMENT ON TABLE dedup.cdeu_step_run IS 'CDEU 步骤运行实例，记录批处理计数';
COMMENT ON COLUMN dedup.cdeu_step_run.step_run_id IS '步骤运行主键';
COMMENT ON COLUMN dedup.cdeu_step_run.phase_run_id IS '阶段运行 ID，逻辑关联 cdeu_phase_run.phase_run_id';
COMMENT ON COLUMN dedup.cdeu_step_run.step_id IS '步骤定义 ID';
COMMENT ON COLUMN dedup.cdeu_step_run.step_code IS '步骤编码快照';
COMMENT ON COLUMN dedup.cdeu_step_run.seq_no IS '步骤顺序快照';
COMMENT ON COLUMN dedup.cdeu_step_run.status IS '步骤运行状态';
COMMENT ON COLUMN dedup.cdeu_step_run.started_at IS '开始时间';
COMMENT ON COLUMN dedup.cdeu_step_run.finished_at IS '结束时间';
COMMENT ON COLUMN dedup.cdeu_step_run.batch_total IS '批次总数；非批处理步骤为 1';
COMMENT ON COLUMN dedup.cdeu_step_run.batch_success IS '成功批次数';
COMMENT ON COLUMN dedup.cdeu_step_run.batch_failed IS '失败批次数';
COMMENT ON COLUMN dedup.cdeu_step_run.completion_rule_snapshot IS '步骤完成条件快照';
COMMENT ON COLUMN dedup.cdeu_step_run.completion_snapshot IS '步骤完成判定快照';
COMMENT ON COLUMN dedup.cdeu_step_run.error IS '失败原因';
COMMENT ON COLUMN dedup.cdeu_step_run.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_step_run.gmt_update IS '更新时间';

-- 作业运行（批处理实例）
CREATE TABLE IF NOT EXISTS dedup.cdeu_job_run (
    job_run_id          bigserial PRIMARY KEY,
    step_run_id         bigint NOT NULL,
    job_id              bigint NOT NULL,
    job_code            text NOT NULL,
    job_kind            text NOT NULL,
    execution_stage     text,
    batch_key           text,
    trigger_type        text NOT NULL DEFAULT 'PROCESS'
                        CHECK (trigger_type IN ('PROCESS','MANUAL_STAGE')),
    status              text NOT NULL DEFAULT 'PENDING'
                        CHECK (status IN ('PENDING','RUNNING','SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')),
    attempt             int NOT NULL DEFAULT 0,
    started_at          timestamptz,
    finished_at         timestamptz,
    processed_rows      bigint NOT NULL DEFAULT 0,
    processed_bytes     bigint NOT NULL DEFAULT 0,
    run_log_id          bigint,
    error               text,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_cdeu_job_run_step ON dedup.cdeu_job_run (step_run_id, job_code);
CREATE INDEX IF NOT EXISTS idx_cdeu_job_run_log ON dedup.cdeu_job_run (run_log_id);
COMMENT ON TABLE dedup.cdeu_job_run IS 'CDEU 作业运行实例（批处理项）';
COMMENT ON COLUMN dedup.cdeu_job_run.job_run_id IS '作业运行主键';
COMMENT ON COLUMN dedup.cdeu_job_run.step_run_id IS '步骤运行 ID，逻辑关联 cdeu_step_run.step_run_id';
COMMENT ON COLUMN dedup.cdeu_job_run.job_id IS '作业定义 ID';
COMMENT ON COLUMN dedup.cdeu_job_run.job_code IS '作业编码快照';
COMMENT ON COLUMN dedup.cdeu_job_run.job_kind IS '实际调度标识快照';
COMMENT ON COLUMN dedup.cdeu_job_run.execution_stage IS '执行阶段快照';
COMMENT ON COLUMN dedup.cdeu_job_run.batch_key IS '批处理键，如 US.AAPL:202604';
COMMENT ON COLUMN dedup.cdeu_job_run.trigger_type IS '触发类型：PROCESS 流程内 / MANUAL_STAGE 单阶段手动重跑';
COMMENT ON COLUMN dedup.cdeu_job_run.status IS '作业运行状态';
COMMENT ON COLUMN dedup.cdeu_job_run.attempt IS '尝试次数';
COMMENT ON COLUMN dedup.cdeu_job_run.started_at IS '开始时间';
COMMENT ON COLUMN dedup.cdeu_job_run.finished_at IS '结束时间';
COMMENT ON COLUMN dedup.cdeu_job_run.processed_rows IS '处理行数';
COMMENT ON COLUMN dedup.cdeu_job_run.processed_bytes IS '处理字节数';
COMMENT ON COLUMN dedup.cdeu_job_run.run_log_id IS '逻辑关联 dedup.finv_dedup_job_run_log_export_file.run_id';
COMMENT ON COLUMN dedup.cdeu_job_run.error IS '失败原因';
COMMENT ON COLUMN dedup.cdeu_job_run.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_job_run.gmt_update IS '更新时间';
```

### 7.3 与现有运行日志的关系

- 现有 `dedup.finv_dedup_job_run_log_export_file` 继续作为**技术执行日志**（阶段、心跳、错误）；
- `cdeu_job_run.run_log_id` 逻辑关联该日志；
- 对已有 Job 的执行采用「双写」：Job 开始/结束同时写旧日志与 `cdeu_job_run`；
- 对 5–8 的 `EMBEDDED` 阶段，`cdeu_job_run` 由父 Job 在进入对应 stage 时创建/更新；单阶段手动重跑时创建 `trigger_type=MANUAL_STAGE` 的新记录。

### 7.4 索引与约束原则

- 所有 `*_code` 在父级内唯一；`seq_no` 在父级内唯一；
- 运行表保留定义快照（code/name/seq_no/rule），不依赖定义表当前值；
- 不加外键，保持与现有 `dedup` 表一致；
- 时间字段统一 `timestamptz`，调度时间按 `schedule_timezone`（默认 `FINV_QUOTE_DEDUB_TIMEZONE`）解释。

## 8. 执行引擎设计

### 8.1 自动运行

```
每日到点
  → 查找 enabled 且 run_mode ∈ (AUTO_DAILY,BOTH) 的 process
  → 检查 run_key=process_code:today:SCHEDULE 不存在
  → 创建 process_run(PENDING) + 展开 phase_run/step_run
  → 按 phase.seq_no 顺序执行（INHERIT）
  → 每个 phase 按 step.seq_no 执行
  → 每个 step 按 job.seq_no 执行
  → 批处理步骤展开 job_run（batch_key）
  → 每层完成后 evaluate(completion_rule)
  → 更新 run 状态与 completion_snapshot
```

### 8.2 手动运行

- `POST /API/V1/CDEU/Process/Run`：body `{process_code, biz_date?, phase_codes?, step_codes?, job_codes?, reason}`；
- 支持全流程、指定阶段、指定步骤、指定作业四种粒度；
- 手动运行写 `trigger_type=MANUAL`、`operator`、审计日志；
- 同一流程已有 `RUNNING` 实例时返回 409，或按 `force` 参数排队。

### 8.3 并发与幂等

- `run_key` 唯一 + `cdeu_process_run` 部分唯一索引防重；
- 阶段/步骤执行前使用 PG advisory lock（`pg_try_advisory_lock(hashtext(process_code))`）或多实例行锁；
- 现有 10 个 Job 的互斥、租约、心跳机制继续生效；
- 步骤重跑时保留原 `step_run` 审计或新建 attempt，禁止静默覆盖。

### 8.4 失败与重试

- 作业失败按 `max_attempts/retry_backoff_seconds` 重试；
- 步骤 `failure_policy=FAIL_FAST`：关键作业失败则后续步骤 `SKIPPED`；
- `CONTINUE`：继续执行其余批次/步骤，最终按完成规则判定；
- 人工可对失败步骤/作业执行 `Retry`，生成新的 `job_run.attempt`。

### 8.5 单阶段手动重跑（5–8）

- 5–8 号作业（`RUN_DUE_EXPORT_JOBS` 的 LOAD / EXPORT / REGISTER / CLEANUP）支持单独重跑；
- 接口：`POST /API/V1/CDEU/Job/RunStage`，body `{job_code, batch_key?, reason}`；
- 服务端按 `execution_stage` 调用对应阶段执行器：`LOAD` / `EXPORT` / `REGISTER`（当前归入 EXPORT）/ `CLEANUP`；
- 生成新的 `cdeu_job_run`，`trigger_type=MANUAL_STAGE`，`attempt+1`，保留历史记录；
- 重跑单阶段不重置整个流程状态；若该阶段所属步骤/流程已完成，仅记录为补充执行，由人工决定是否重新评估完成条件；
- 阶段执行器复用现有 Job 的锁、租约与心跳，避免与正常流程并发冲突。

---

## 9. 监控与可视化映射

- 现有「端到端链路」页面继续以 `GET /Status` + `GET /Job/Runtime` 展示实时阶段；
- 引入 CDEU 后，页面数据源可切换为：
  - `GET /CDEU/Process/Run/Latest`：最近一次 process_run + phase_run + step_run 树；
  - `GET /CDEU/Process/Run/Jobs`：批处理 job_run 明细；
- 旧接口保留，作为底层 Job 技术状态来源；
- 卡片状态映射：
  - `process_run.status` → 流程总体；
  - `phase_run.status` → 阶段节点；
  - `step_run.batch_*` → 批次进度；
  - `job_run.status/error` → 卡点详情。

---

## 10. 落地步骤（建议）

| 阶段 | 内容 | 兼容性 |
|---|---|---|
| P0 | 本文档评审定稿 | 不改代码 |
| P1 | ✅ 在 `dedup` schema 下新增 8 张 `cdeu_*` 表；seed 13 个 Job 定义（`V44`） | 不影响现有调度 |
| P2 | ✅ 现有 Job 执行双写 `cdeu_job_run`；`cdeu_process_run` 由每日调度创建；完成条件逐级评估。⏳ 5–8 的 `RunStage` 单阶段接口待补 | 现有 10 个 Job 行为不变 |
| P3 | ✅ 调度计划读取 CDEU 定义；Phase/Step 顺序仍由现有 Job 时刻与内嵌阶段保证 | 可灰度 |
| P4 | ✅ 前端新增「流程控制与监控」页（定义树 + 运行状态 + 历史 + 手动触发）；保留现有链路页作为技术视图 | 可回退 |

### 10.1 兼容原则

- 不修改现有 10 个 `job_kind` 的语义与调度时间；
- 13 个 Job 行中 5–8 为 `EMBEDDED`，支持单阶段手动重跑；
- 新旧运行日志并存，`run_log_id` 关联。

---

## 11. 已确认决策

1. **schema**：`cdeu_*` 表复用现有 `dedup` schema，不新增 schema。
2. **5–8 单阶段重跑**：支持，通过 `RunStage` 手动触发并生成 `MANUAL_STAGE` 运行记录。
3. **⑩ 网盘归档**：按自动完成处理；当前实现暂缺，本模型不登记、不做额外处理和完成定义。
4. **业务日时区**：沿用 `FINV_QUOTE_DEDUB_TIMEZONE`。
5. **批处理失败策略**：默认 `CONTINUE`。
6. **流程完成条件**：默认 `ALL`。

## 12. 附：13 个 Job 登记 SQL 示例

```sql
-- 示例：流程与阶段
INSERT INTO dedup.cdeu_process (process_code, name, run_mode, schedule_time, schedule_enabled, completion_rule)
VALUES ('CDEU_DAILY', '行情数据采集-去重-导出-上传', 'BOTH', '00:30:00', true, '{"mode":"ALL"}');

INSERT INTO dedup.cdeu_phase (process_id, phase_code, name, seq_no, critical, affects_completion, run_mode)
VALUES
  (1,'PHASE_COLLECT','数据采集',1,true,true,'INHERIT'),
  (1,'PHASE_DEDUP','去重收口',2,true,true,'INHERIT'),
  (1,'PHASE_EXPORT','导出',3,true,true,'INHERIT'),
  (1,'PHASE_UPLOAD','上传',4,true,true,'INHERIT');

-- 示例：作业登记（节选）
INSERT INTO dedup.cdeu_job
  (step_id, job_code, name, job_kind, execution_stage, seq_no, trigger_mode, execution_mode, schedule_kind, schedule_value)
VALUES
  (1,'JOB_CALC_INGEST_STATS','入口统计','CALC_DAILY_INGEST_STATS',NULL,1,'SCHEDULED','DIRECT','DAILY','00:05'),
  (4,'JOB_EXPORT_LOAD','导出装载','RUN_DUE_EXPORT_JOBS','LOAD',1,'EMBEDDED','EMBEDDED','NONE',NULL),
  (4,'JOB_EXPORT_EXPORT','导出文件生成','RUN_DUE_EXPORT_JOBS','EXPORT',2,'EMBEDDED','EMBEDDED','NONE',NULL),
  (4,'JOB_EXPORT_REGISTER','导出文件登记','RUN_DUE_EXPORT_JOBS','REGISTER',3,'EMBEDDED','EMBEDDED','NONE',NULL),
  (4,'JOB_EXPORT_CLEANUP','中转清理','RUN_DUE_EXPORT_JOBS','CLEANUP',4,'EMBEDDED','EMBEDDED','NONE',NULL);
```

表结构建议先按第 7 节执行 `dedup.cdeu_*` DDL。完整 seed 应覆盖第 6.2 节 13 行；`step_id` 按实际插入后的 ID 回填，或通过 `phase_id + step_code` 子查询获取。

---

## 13. 落地状态（2026-10-25）

- `Deploy/Migrations/V100044__finv_dedup_cdeu_process_model.sql` 已创建 8 张 `dedup.cdeu_*` 表并 seed；`V100045__finv_dedup_cdeu_split_ops_steps.sql` 将原 `PHASE_OPS` 的旁路步骤拆分到对应阶段：
  - 1 个流程 `CDEU_DAILY`；
  - 5 个阶段：`PHASE_COLLECT` / `PHASE_DEDUP` / `PHASE_DECISION` / `PHASE_EXPORT` / `PHASE_UPLOAD`（`V47` 新增 `PHASE_DECISION`，`V48` 将入口统计/预算移入该阶段，`V49` 将成熟度扫描移入该阶段）；
  - 12 个步骤、13 个作业（`STEP_HISTORY_INGEST` 为数据采集阶段的非关键占位步骤，`STEP_EXPORT_DECISION` 为导出决策阶段的占位步骤，均无 JOB、初始化自动成功；5–8 为 `RUN_DUE_EXPORT_JOBS` 的 LOAD / EXPORT / REGISTER / CLEANUP；`STEP_RECOVER` 在导出阶段、`STEP_HEALTH` 在上传阶段，均为旁路步骤）。
- `Service.RunWithTrigger` 执行 Job 时双写 `cdeu_job_run`，并按 `ALL` 完成条件逐级评估 Step / Phase / Process；
- 同一天多个 Job 复用同一个 `cdeu_process_run`；流程终态后的手动补跑只登记只读作业运行，不重开已完成的流程；
- `Scheduler` 启动时读取 `cdeu_job.schedule_kind/value` 覆盖默认调度时刻（`RUN_DUE_EXPORT_JOBS` 仍使用窗口内固定间隔）；
- 5–8 的 `RunStage` 单阶段重跑接口尚未实现，当前可通过 `POST .../Job/Run` 重跑完整导出 Job；
- Process 级手动运行接口尚未开放；手动 Job 在当日流程终态后只登记只读运行；
- 前端页面 `/Meta/Finv/Quote/DeDup/CDEU` 与 `GET /CDEU/Overview` 已上线：默认总体视图使用点阵画布 + 阶段悬浮节点 + 有向箭头；支持阶段概要悬浮、点击阶段切换步骤清单、阶段/步骤/作业详情、运行历史与作业手动触发；`POST /CDEU/Process/Run` 支持启动完整流程。
- 集成测试 `TestServiceIntegration` 已覆盖 13 个作业全部登记到 CDEU 运行树。

## 14. 参考

- 现有实现：[`2610-005-FinvQuoteDeDubExportUpload.md`](2610-005-FinvQuoteDeDubExportUpload.md)
- 方案稿：`D:\Code\GoCode\FinvQuantPlan\2610-002-SecuritiesQuoteDataExportUploadSchemeDesign.md` 第 2.1 节端到端链路
- 数据库规范：[`../DevSpec/DbSchemaSpec.md`](../DevSpec/DbSchemaSpec.md)
- 日志/接口规范：[`../DevSpec/LogSpec.md`](../DevSpec/LogSpec.md)、[`../DevSpec/ApiSpec.md`](../DevSpec/ApiSpec.md)
