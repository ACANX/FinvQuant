# 证券行情月归档导出与上传（落地说明）

> 归属：FinvQuant · 实施：`internal/meta/finv/quote/dedup`（Go）+ `Deploy/Migrations/V100041__finv_dedup_export_archive_tables.sql`
> 方案稿：`2610-002-SecuritiesQuoteExportUploadSchemeDesign.md` v5.0（设计版），本文为**落地版**，逐条说明实现方式、配置与运维要点。
> 关联规范：[`../DevSpec/DbSchemaSpec.md`](../DevSpec/DbSchemaSpec.md)、[`../DevSpec/ApiSpec.md`](../DevSpec/ApiSpec.md)、[`../DevSpec/ErrorCodeSpec.md`](../DevSpec/ErrorCodeSpec.md)

## 1. 范围

实现「按计划分批次、分页导出月归档数据，并支持后续上传到独享存储库」的自动化数据处理链路：

| 能力 | 实现 |
|---|---|
| 每日入口统计与自适应日预算 | `CALC_DAILY_INGEST_STATS`、`CALC_DAILY_EXPORT_BUDGET` |
| 数据成熟度、自动发现证券、收口冻结 | `REFRESH_QUOTE_MATURITY`、`MARK_SEALED_JOBS_FROZEN` |
| 分批分页导出（LOAD → EXPORT → CLEANUP） | `RUN_DUE_EXPORT_JOBS`（低峰窗口内） |
| 上传独享存储库 + 远端校验 | `UPLOAD_PENDING_EXPORT_FILES` |
| 收口后延迟数据分级补丁 | `SCAN_POST_SEAL_PATCHES`（L1_HOT / L2_WARM / L3_COLD） |
| 生命周期清理 | `CLEAN_EXPIRED_EXPORT_ARTIFACTS` |
| 超时回收、健康检查与告警 | `RECOVER_TIMEOUT_EXPORT_JOBS`、`CHECK_EXPORT_JOB_HEALTH` |
| 运维接口（状态 / 任务 / 文件 / 重试 / 手动触发） | `Docs/API/QuoteDeDup/*.md` |

**不修改**既有两张业务表（`dedup.finv_quote_secu_kline_min`、`export.finv_quote_secu_kline_min`）的结构、索引、约束、触发器与 schema；新增对象全部在 `dedup` schema，统一 `finv_dedup_*` 前缀，全库无外键。

## 2. 数据表（`V41` 新增 12 张，全部 `dedup`）

| 表 | 用途 | 关键约束 |
|---|---|---|
| `finv_dedup_ingest_stats` | 每日入口流量统计 | 主键 `stat_date` |
| `finv_dedup_daily_budget` | 每日预算快照（因子 / 消耗） | 主键 `biz_date` |
| `finv_dedup_scan_watermark` | 扫描水位（成熟度、补丁） | 主键 `watermark_name` |
| `finv_dedup_quote_maturity_mon` | 证券日成熟度 | 主键 `(usc, trade_date)` |
| `finv_dedup_trade_calendar` | 交易日历 | 主键 `(usc, trade_date)` |
| `finv_dedup_job_run_log_export_file` | Job 运行日志（含阶段 / 进度 / 心跳） | 主键 `run_id` |
| `finv_dedup_usc` | 证券主数据（文件头/文件名来源） | 主键 `usc` |
| `finv_dedup_export_job` | 证券月导出任务 | 主键 `job_id`，唯一键 `(usc, biz_month)` |
| `finv_dedup_export_file` | 导出文件登记（上传任务唯一来源） | 主键 `file_id`，唯一键 `(usc, biz_month, file_type, version)` |
| `finv_dedup_export_file_upload_log` | 上传尝试审计 | 主键 `log_id` |
| `finv_dedup_export_file_patch` | 收口后补丁任务 | 主键 `patch_id` |
| `finv_dedup_runtime_setting` | 运行时设置（低峰窗口在线修改） | 主键 `setting_key` |

索引命名遵循 `idx_<表名去 finv_ 前缀并去下划线>_<用途>`（例：`idx_dedupexportfileuploadlog_file`），`V41` 共建 12 个；迁移末尾做收敛校验（表数 / 索引数 / 无外键 / schema 归属）。`V42` 另为运行日志增加 `stage` / `progress_percent` / `progress_detail` / `gmt_heartbeat` 四列与未结束记录部分索引 `idx_dedupjobrunlogexportfile_running`；`V43` 新增 `finv_dedup_runtime_setting`（键值表），保存后由服务热更新，优先于 `FINV_QUOTE_DEDUB_WINDOW_START/_END`。

> 说明：`finv_dedup_quote_maturity_mon.gmt_seale` 沿用方案稿列名（语义为 seal 时间），避免与既有设计核对时产生差异。

## 3. 文件格式与命名

- **MVsv**：16 行 `# ` 文件头 + **1 行空行** + 数据行；数据行 12 列固定为 `ts|da|ti|o|h|l|c|v|t|cp|cr|lc`（`da`=date、`ti`=time，避免与 `t`=turnover 混淆）；空值留空（出现 `||`）；UTF-8 无 BOM、LF 换行；`Count` 等于实际数据行数。
- **计数与校验**：写文件前用 `COUNT(*)` 预统计 `Count`，写完校验「实际行数 = 预统计 = 登记行数」；先写同目录临时文件，`fsync` 后原子 `rename`；返回 SHA256 与字节数写入 `finv_dedup_export_file`。
- **命名**（本地与上传目标同名，不生成、不支持 `.csv`）：
  - BASE：`{region}_{market}_{usc}_Min_{provider}_Mon_{yyyyMM}_Base.mvsv`（需求 10743 起带 `_Base` 后缀）
  - PATCH：`{region}_{market}_{usc}_Min_{provider}_Mon_{yyyyMM}_Incr_{n}.mvsv`
  - MANIFEST：`{region}_{market}_{usc}_Min_{provider}_Mon_{yyyyMM}_Manifest.json`
- **Manifest 必填字段**：`usc`、`biz_month`、`format=mvsv`、`delimiter=|`、`encoding=utf-8`、`compression`、`header_lines=16`、`blank_line_after_header=true`、`columns`、`base`（文件名/行数/大小/checksum/version）、`patches[]`、`row_count`、`checksum`、`seal_date`、`schema_version`、`generated_at`。
- **压缩**：默认 `none`；`FINV_QUOTE_DEDUB_COMPRESSION=gzip` 时单遍流式压缩，扩展名仍为 `.mvsv`，Manifest 记录 `compression=gzip`，checksum 为压缩后内容。

## 4. 执行流程

### 4.1 候选生成

- 月份必须**已结束**（`biz_month < 当前月`），且该月**所有已观测日**的成熟度均为 `COMPLETE`/`FROZEN`；
- 证券元数据 `region`/`market`/`provider`/`name`/`timezone`/`currency` 必须齐备，否则不生成候选（并在健康指标中体现）；
- 未退市，或退市日晚于该月首日；
- 同一 `(usc, biz_month)` 由唯一键 + `ON CONFLICT DO NOTHING` 保证只建一次任务；
- 排序：`FROZEN` > 临近 `freeze_date`（≤3 天）> `priority_rank` > `oldest_pending_date`。

> 与方案稿的差异（防中途导出不完整月份）：方案稿 4.1 只要求「成熟度 COMPLETE」，本实现额外要求「月份已结束」，避免月中把当月当成完整月导出。

### 4.2 领取与三阶段

- 领取：`UPDATE ... FROM (SELECT ... FOR UPDATE SKIP LOCKED)` + `locked_by`/`locked_until`/`gmt_heartbeat` 租约；心跳超时由 `RECOVER_TIMEOUT_EXPORT_JOBS` 回收为 `PAUSED`。
- **LOAD**：按 `date` 逐日、日内按 `ts > load_cursor_ts` keyset 分页，单页一条语句完成「取主表一页 → `INSERT ... ON CONFLICT (ts, usc) DO NOTHING` → 返回页游标」，每页提交并更新游标/`loaded_rows`；单日结束后校验「中转行数 ≥ 主表行数」，不足则重装该日一次；**窗口结束**时保存游标转 `PAUSED`，次日续跑。预算只在「是否启动新任务」时判断（H-05），已启动任务允许在窗口内做完，不因预算中断（避免半途暂停、重复装载）。
- **EXPORT**：按 `usc + date 范围` 流式读中转表（`ORDER BY ts`）写 MVsv；登记 BASE 与 MANIFEST（`target_file_name` = `file_name`）。
- **CLEANUP**：按 `usc + date 范围` 删除中转数据；中转表为空时 `TRUNCATE`；任务转 `EXPORTED`。

### 4.3 上传

- 领取条件：`upload_status IN ('PENDING_UPLOAD','UPLOAD_FAILED') AND gmt_next_retry <= now()`，`BASE`/`PATCH` 优先；
- **MANIFEST 必须等同一证券月其它文件全部上传完成**后才领取（`NOT EXISTS` 子查询）；
- 上传前校验本地文件存在、大小、SHA256 与登记一致；远端已存在且 checksum 一致则直接标成功（幂等重传）；
- 上传后做远端校验（大小 + SHA256）通过才标记 `UPLOADED` + `remote_checksum_verified=true`；每次尝试写 `finv_dedup_export_file_upload_log`；
- 失败指数退避（1/2/4…分钟，上限 60 分钟），尝试次数达 `FINV_QUOTE_DEDUB_MAX_ATTEMPTS` 后置 24 小时并告警 `UPLOAD_RETRY_EXHAUSTED`。

### 4.4 收口与补丁

- `seal_date` = 次月 25 日；`freeze_date` = `seal_date + sla_days`（默认 5 天）；
- 到达 `seal_date`：成熟度整月置 `FROZEN` 并输出差异报告（缺失日期 / 缺失行数 / 缺失比例）；到达 `seal_date` 仍未完成的任务置 `FROZEN` 并优先处理；
- 补丁按每个证券月独立水位（`POST_SEAL_PATCH_<usc>_<yyyyMM>`）扫描主表中 `gmt_create/gmt_update` 晚于水位的行：
  - **L1_HOT**：base 尚未上传成功 → 全量重写 base（`base_version+1`，同名原子覆盖），旧记录 `SUPERSEDED`；
  - **L2_WARM**：base 已上传、未归档/未清理 → 生成 `..._Incr_{n}.mvsv` 并更新 Manifest；
  - **L3_COLD**：base 已归档或本地已清理 → 同样生成增量补丁（离线补丁包）并记录补丁任务；
  - 补丁与正常导出共用中转表：先按日期 + `ts` keyset 分页把增量（L1 为整月）装入 `export.finv_quote_secu_kline_min`，再从转存表写 BASE/PATCH 文件，成功后清理中转数据；数据量大时不会从主表直接流式写文件；
  - 补丁原受独立小预算约束（日预算 × `FINV_QUOTE_DEDUB_PATCH_BUDGET_RATIO`，默认 10%）；为避免历史基线重新聚合等大数据量场景长期无法收敛，超出预算只记录告警，仍按完整补丁执行。

> 与方案稿的差异：K-01 规定「seal 窗口内到达的延迟数据走正常增量、不视为 Patch」。本实现对**已上传成功**的 base 一律走补丁（含 seal 之前），因为修改已上传的归档文件不可追溯；仅当 base 未上传成功时才走 L1_HOT 重写。

### 4.5 生命周期

- 本地文件清理条件：`UPLOADED` + 远端校验通过 + `network_status='ARCHIVED'` + 无待处理补丁 + 保留期（默认 30 天）到达；
- 主表旧数据清理：改由**系统配置 `20007`** 承载（`FINV_QUOTE_DEDUB_PURGE_MAIN_ENABLED` / `_PURGE_MAIN_MONTHS` 已废弃），见 §4.6；

- 中转表：`CLEAN_EXPIRED_EXPORT_ARTIFACTS` 每轮删除不属于任何活跃任务范围的残留（每批 5 万行），空表 `TRUNCATE`。

### 4.6 主表数据释放（系统配置 20007）

`20007 = 导出成功后立即删除主表行情数据标志`（`sys.sys_config`，`char_value`：`1`-删除（默认）/ `0`-保留）。

删除**只作用于 `dedup.finv_quote_secu_kline_min`**；`quote.finv_quote_secu_kline_min` 是回测/查询专用的独立副本，**不参与删除**。

删除有两条触发路径，二者互补：

| 路径 | 触发点 | 判定条件 | 覆盖场景 |
|---|---|---|---|
| **① 上传后立即删** | `UPLOAD_PENDING_EXPORT_FILES` 单个文件上传成功之后 | 本次发生了真实上传动作且远端校验通过 | 常态：新数据导出 → 上传 → 删 |
| **② 沉淀月补偿清理** | `CLEAN_EXPIRED_EXPORT_ARTIFACTS` 每轮执行（仅当 `20007=1`） | 见下方三重条件 | 漏网 + 存量：文件因内容未变而跳过上传时 |

#### 为什么需要路径 ②

`RegisterFile` 在 **checksum 未变**时会保持原有的 `UPLOADED` 状态（避免重复上传同一份内容）。因此「重新导出内容相同的月份」**不会产生上传动作**，路径 ① 永远不触发，数据永久留在主表：

```
重新导出 → checksum 未变 → 保持 UPLOADED，不进上传队列
        → 无上传动作 → 20007 路径 ① 不触发 → 数据不删
        → 水位线已推进 → delta 恒为 0 → 也不会生成 INCR
```

结果形成一个「死水区」：数据既不释放，也不产生增量。路径 ② 就是为了兜住这种沉淀状态。

#### 路径 ② 的判定条件（三重，全部满足才删）

```sql
SELECT f.file_id, f.usc, f.biz_month
  FROM dedup.finv_dedup_export_file f
 WHERE f.file_type = 'BASE'
   AND f.export_status = 'EXPORTED'                     -- ① 已导出
   AND f.upload_status = 'UPLOADED'                     -- ② 已上传成功
   AND EXISTS (主表该月有数据)                            -- ③ 本地确实还有副本
   AND NOT EXISTS (
       SELECT 1 FROM 主表 m
        WHERE m.usc = f.usc AND m.date BETWEEN 该月首末
          AND (m.gmt_create > f.gmt_create             -- ④ 文件生成后无新增
               OR m.gmt_update > f.gmt_create))        --    也无更新
```

条件 ④ 是关键保护：**只要有一行数据在 BASE 文件生成之后被写入或修改，就说明远端文件不包含它**，必须先走 INCR 补丁流程，绝不能直接删除。

#### 删除方式与观测

- 按 `(usc, biz_month)` 逐月删除，单月按 `50000` 行一批循环删空（`PurgeMainSecurityMonth`）；
- **不做行级备份**（需求方明确约定）：数据在远端 BASE 文件与 `quote` 副本中各有一份完整拷贝；
- 观测字段：`CLEAN_EXPIRED_EXPORT_ARTIFACTS` 的 `details.settled_months_purged` / `details.settled_rows_purged`，以及逐月日志 `20007 沉淀月清理 usc=… biz_month=… rows=… file_id=…`；
- 单轮最多处理 `500` 个月（`settledPurgeBatch`），避免长时间占用连接。

#### 与增量导出的关系

删除是**增量链路恢复的前提**：主表数据释放后，后续采集写入的新数据 `gmt_create` 必然晚于水位线，`CountMainDelta > 0`，`SCAN_POST_SEAL_PATCHES` 才会生成 INCR 动态任务并产出 `_Incr_N.mvsv`。若数据长期不释放，水位线会持续推进并越过所有历史数据，导致永久性 `delta = 0`。

## 5. 配置项

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `FINV_QUOTE_DEDUB_ENABLED` | `false` | 功能开关；关闭时只提供运维接口，不自动调度（M-10） |
| `FINV_QUOTE_DEDUB_TIMEZONE` | `Asia/Shanghai` | 业务时区（预算日期、调度窗口、`AT TIME ZONE` 换算） |
| `FINV_QUOTE_DEDUB_OUTPUT_DIR` | `/data/App/FinvQuant/Export` | 本地导出根目录（按 `{usc}/{yyyyMM}/` 分层） |
| `FINV_QUOTE_DEDUB_WORKER_ID` | `hostname-pid` | worker 标识（写入 `locked_by`） |
| `FINV_QUOTE_DEDUB_PAGE_SIZE` | `10000` | LOAD 单页行数（收敛到 1000–20000） |
| `FINV_QUOTE_DEDUB_WINDOW_START` / `_END` | `00:30` / `06:30` | 导出低峰窗口（支持跨零点） |
| `FINV_QUOTE_DEDUB_LEASE_SECONDS` | `300` | 任务锁租约（心跳续租） |
| `FINV_QUOTE_DEDUB_MAX_ATTEMPTS` | `8` | 上传最大尝试次数 |
| `FINV_QUOTE_DEDUB_UPLOAD_MODE` | `none` | 上传实现：`none`（不启用）/ `dir`（目录或 NAS 挂载）/ `http`（HTTP PUT 网关） |
| `FINV_QUOTE_DEDUB_UPLOAD_DIR` | `/data/App/FinvQuant/Export/Upload` | `dir` 模式的独享存储根目录 |
| `FINV_QUOTE_DEDUB_UPLOAD_HTTP_BASE` / `_TOKEN` | 空 | `http` 模式的 PUT 根地址与 Bearer 令牌 |
| `FINV_QUOTE_DEDUB_UPLOAD_PREFIX` | 空 | 远端路径前缀（如 `cn/quotes`） |
| `FINV_QUOTE_DEDUB_UPLOAD_GITHUB_API_BASE` | 空（即 `https://api.github.com`） | GitHub Enterprise 覆盖 API 端点；GitHub 上传的开关/PAT/仓库定位在 `sys_config` 20002/20003/20004（见后文补录） |
| `FINV_QUOTE_DEDUB_LOCAL_RETENTION_DAYS` | `30` | 本地文件保留天数 |
| ~~`FINV_QUOTE_DEDUB_PURGE_MAIN_ENABLED`~~ | ~~`false`~~ | **已废弃**：改由 `sys_config` 20007 承载 |
| ~~`FINV_QUOTE_DEDUB_PURGE_MAIN_MONTHS`~~ | ~~`2`~~ | **已废弃**：不再保留「按月份数延迟清理」语义 |
| `FINV_QUOTE_DEDUB_MATURITY_THRESHOLD` | `0.95` | 成熟度阈值（实际/预期） |
| `FINV_QUOTE_DEDUB_PATCH_BUDGET_RATIO` | `0.10` | Patch 独立预算比例 |
| `FINV_QUOTE_DEDUB_COMPRESSION` | `none` | `none` / `gzip`（扩展名保持 `.mvsv`） |
| `FINV_QUOTE_DEDUB_FETCH_LIMIT` | `2000` | 单轮候选生成上限 |
| `FINV_QUOTE_DEDUB_MAX_JOBS_PER_RUN` | `200` | 单轮最多处理任务数 |

上传适配器为可插拔接口（`Uploader`）：`Stat` / `Put` / `Verify`。接入 S3、MinIO、SFTP、WebDAV 等只需实现该接口并在 `NewUploader` 中注册，无需改动任务与状态机。

## 6. 调度

| Job | 触发 | 说明 |
|---|---|---|
| `CALC_DAILY_INGEST_STATS` | 每日 00:05 | 按 `gmt_create` 聚合近 30 天入口流量 |
| `CALC_DAILY_EXPORT_BUDGET` | 每日 00:05 | 计算 `budget_rows` / `hard_cap_rows` |
| `REFRESH_QUOTE_MATURITY` | 每日 00:10 | 增量刷新成熟度 + 自动发现 `usc`；首次与每月 1 日按近 3 个月全量校准 |
| `MARK_SEALED_JOBS_FROZEN` | 每日 00:20 | 收口冻结 + 差异报告 |
| `RUN_DUE_EXPORT_JOBS` | 窗口内每 60 秒 | 生成候选 → 领取 → LOAD/EXPORT/CLEANUP |
| `UPLOAD_PENDING_EXPORT_FILES` | 每 5 分钟 | 上传独享存储库 |
| `SCAN_POST_SEAL_PATCHES` | 每日 03:00 | 分级补丁 |
| `CLEAN_EXPIRED_EXPORT_ARTIFACTS` | 每日 06:00 | 残留/本地/主表清理 |
| `RECOVER_TIMEOUT_EXPORT_JOBS` | 每 5 分钟 | 任务与上传超时回收 |
| `CHECK_EXPORT_JOB_HEALTH` | 每 15 分钟 | 指标 + MVsv 结构抽检 + 告警日志 |

调度为**服务进程内**调度器（每 30 秒 tick）；多实例部署时由数据库行锁 + 租约保证同一任务不会被重复处理。`sche` schema 的持久化调度留待后续阶段。

## 7. 运维要点

1. **首次开通**：执行迁移 → 校准 `dedup.finv_dedup_usc`（`day_expect_rows`、`region/market/provider/name/timezone/currency`）→ 按需在 `dedup.finv_dedup_usc` 上设置 `priority_rank`/`sla_days`/`max_wait_days` → 设置 `FINV_QUOTE_DEDUB_ENABLED=true`。
2. **未校准的证券不会导出**：`day_expect_rows` 为空时该日成熟度保持 `PARTIAL`，证券月无法进入候选（安全默认）。
3. **网盘归档**：本实现不负责网盘上传，需外部流程完成后更新 `finv_dedup_export_file.network_status='ARCHIVED'`（否则本地文件不会被清理）。
4. **人工干预**：`POST .../Job/Retry`、`POST .../File/Retry` 重置状态；`POST .../Job/Run` 手动触发 Job；`GET .../Status` 查看指标与告警；`GET .../Job/Runtime` 查看 10 个 Job 的当前阶段 / 进度 / 心跳与最近结果（流程页每 5 秒轮询）；`PUT .../Settings/Window` 在线修改低峰窗口，保存后立即生效。
5. **核对命令**：
   ```sql
   -- 任务状态分布
   SELECT status, count(*) FROM dedup.finv_dedup_export_job GROUP BY status;
   -- 待上传（含重试耗尽）
   SELECT upload_status, count(*) FROM dedup.finv_dedup_export_file GROUP BY upload_status;
   -- 中转残留
   SELECT count(*), count(DISTINCT usc) FROM export.finv_quote_secu_kline_min;
   -- 最近一次 Job 运行
   SELECT job_kind, status, processed_rows, gmt_start, gmt_finish
     FROM dedup.finv_dedup_job_run_log_export_file ORDER BY run_id DESC LIMIT 20;
   ```

## 8. 落地阶段对照（方案稿 5.4）

| 阶段 | 状态 | 说明 |
|---|---|---|
| P0 建表 | 已完成 | `V41` 12 张表 + 索引 + 收敛校验 |
| P1 统计/预算/成熟度 | 已完成 | 三个 Job + `finv_dedup_usc` 自动发现与 `provider` 回填 |
| P2 小流量导出 | 已完成 | `RUN_DUE_EXPORT_JOBS` 全链路；建议先用少量证券验证 |
| P3 上传 | 已完成 | `UPLOAD_PENDING_EXPORT_FILES` + `dir`/`http` 适配器 + 远端校验 |
| P4 新旧并行 | 待运维 | 通过 `FINV_QUOTE_DEDUB_ENABLED` 开关灰度 |
| P5 网盘归档 / 生命周期 | 部分完成 | 生命周期清理已完成；网盘归档需外部接入后标记 `network_status` |
| P6 分级 Patch | 已完成 | L1_HOT / L2_WARM / L3_COLD |
| P7 回收与健康检查 | 已完成 | `RECOVER_TIMEOUT_EXPORT_JOBS`、`CHECK_EXPORT_JOB_HEALTH` |

## 9. 测试与验证

- 单元测试（不依赖数据库）：`go test ./internal/meta/finv/quote/dedup`，覆盖文件名/Manifest、MVsv 结构（16 行头 + 空行 + Count + gzip + 行数不一致）、预算公式、收口月份推导、退避、窗口判断、上传适配器（含目录穿越防护）、告警阈值、Job 调度表完整性。
- 结构防回归：`go test ./internal/database` 的 `TestSQLSchemaQualified` 强制所有 SQL 写 schema 全限定名。
- 实库集成测试（默认跳过，需可清空的测试库）：

  ```bash
  FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:55432/finv_migtest?sslmode=disable" \
    go test -p 1 ./internal/database ./internal/meta/finv/quote/dedup -count=1
  ```

  覆盖：全量迁移幂等性、12 表/12 索引/无外键、成熟度→候选→领取→LOAD→导出登记→上传顺序→补丁→收口→指标→生命周期、10 个 Job 端到端（实际生成并校验 `.mvsv` 与 Manifest 文件）。两个包共用同一测试库，**必须加 `-p 1` 串行**。
- 建议的端到端验证（需真实库）：
  1. `go run ./cmd/server`（`FINV_QUOTE_DEDUB_ENABLED=false`）→ 调 `POST .../Job/Run` 依次触发 `REFRESH_QUOTE_MATURITY` → `CALC_DAILY_INGEST_STATS` → `CALC_DAILY_EXPORT_BUDGET`；
  2. 校准 1 个证券的 `finv_dedup_usc`，触发 `RUN_DUE_EXPORT_JOBS`，检查生成文件的行数/文件头/空行/`# Count`/命名；
  3. `FINV_QUOTE_DEDUB_UPLOAD_MODE=dir` 并指定目录，触发 `UPLOAD_PENDING_EXPORT_FILES`，检查远端文件与 `upload_log`；
  4. 向主表补一行历史月份数据，触发 `SCAN_POST_SEAL_PATCHES`，检查 `..._Incr_1.mvsv` 与 Manifest 的 `patches[]`。
