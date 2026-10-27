-- =====================================================================
-- FinvQuant PostgreSQL V41：证券行情月归档导出与上传 —— dedup schema 新增 12 张表
--
-- 依据：Docs/Design/2610-SecuritiesQuoteExportArchive.md
--       （方案稿 D:\Code\GoCode\FinvQuantPlan\2610-002-SecuritiesQuoteExportUploadSchemeDesign.md v5.0）
--
-- 决策（ACANX 2026-10-24）：
--   - 严格基于既有两张业务表实现，**不修改**它们的结构 / 索引 / 约束 / 触发器 / schema：
--       dedup.finv_quote_secu_kline_min  —— 分钟行情主表（主键 (ts, usc)）
--       export.finv_quote_secu_kline_min —— 导出转存表（结构与主表一致）
--   - 新增对象**全部**落在 dedup schema，统一 finv_dedup_* 前缀；
--     不新增 export / sche schema，不在 public 建表。
--   - 全库**禁止外键**：表间关系由应用层校验 + 唯一键 + 索引保证。
--   - 导出任务主键及所有关联字段统一 job_id；导出文件主键 file_id；
--     补丁任务主键 patch_id；运行日志主键 run_id；上传审计主键 log_id。
--   - 时间字段统一 gmt_*、标记字段统一 flag_*、日期字段统一 date_*；
--     条数字段 total_rows / loaded_rows / row_count / processed_rows；
--     字节字段 total_bytes / file_size / processed_bytes / uploaded_bytes。
--   - 普通索引命名规则：idx_<表名去 finv_ 前缀并去下划线>_<用途>。
--   - 幂等：CREATE TABLE IF NOT EXISTS + CREATE INDEX IF NOT EXISTS + 重复执行
--     仅刷新 COMMENT（COMMENT 本身幂等）；重复执行无副作用。
--   - 单事务、失败整体回滚；末尾做收敛校验（表数 / 外键数 / 索引数）。
-- 前置：V36（dedup schema）、V38/V39（主表与转存表结构）、V40（日期索引）已应用。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 每日入口流量统计
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.finv_dedup_ingest_stats (
  stat_date    date PRIMARY KEY,
  total_rows   bigint NOT NULL,
  total_bytes  bigint NOT NULL DEFAULT 0,
  gmt_update   timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE dedup.finv_dedup_ingest_stats IS '每日入口流量统计，用于计算日预算；基于 dedup.finv_quote_secu_kline_min 按 gmt_create 聚合';
COMMENT ON COLUMN dedup.finv_dedup_ingest_stats.stat_date IS '统计日期（业务时区换算后的自然日）';
COMMENT ON COLUMN dedup.finv_dedup_ingest_stats.total_rows IS '当日入口总条数';
COMMENT ON COLUMN dedup.finv_dedup_ingest_stats.total_bytes IS '当日入口总字节数（pg_column_size 估算）';
COMMENT ON COLUMN dedup.finv_dedup_ingest_stats.gmt_update IS '统计更新时间';

-- ---------------------------------------------------------------------
-- 2. 每日导出预算快照
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.finv_dedup_daily_budget (
  biz_date        date PRIMARY KEY,
  base_rows       bigint NOT NULL,
  base_bytes      bigint NOT NULL DEFAULT 0,
  p75_rows        bigint,
  backlog_ratio   numeric,
  sla_pressure    numeric,
  factor          numeric NOT NULL,
  budget_rows     bigint NOT NULL,
  budget_bytes    bigint NOT NULL DEFAULT 0,
  hard_cap_rows   bigint NOT NULL,
  consumed_rows   bigint NOT NULL DEFAULT 0,
  consumed_bytes  bigint NOT NULL DEFAULT 0,
  status          text NOT NULL,
  gmt_create      timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE dedup.finv_dedup_daily_budget IS '每日导出预算快照，含自适应因子与消耗量';
COMMENT ON COLUMN dedup.finv_dedup_daily_budget.biz_date IS '预算日期（业务时区自然日）';
COMMENT ON COLUMN dedup.finv_dedup_daily_budget.base_rows IS '近 30 天入口日均条数';
COMMENT ON COLUMN dedup.finv_dedup_daily_budget.base_bytes IS '近 30 天入口日均字节数';
COMMENT ON COLUMN dedup.finv_dedup_daily_budget.p75_rows IS '近 30 天入口条数 P75';
COMMENT ON COLUMN dedup.finv_dedup_daily_budget.backlog_ratio IS '积压量 / 日均条数，表示积压天数';
COMMENT ON COLUMN dedup.finv_dedup_daily_budget.sla_pressure IS '临近 SLA 任务占比';
COMMENT ON COLUMN dedup.finv_dedup_daily_budget.factor IS '自适应预算因子，clamp(1.20 ~ 2.00)';
COMMENT ON COLUMN dedup.finv_dedup_daily_budget.budget_rows IS '当日预算条数上限';
COMMENT ON COLUMN dedup.finv_dedup_daily_budget.budget_bytes IS '当日预算字节上限';
COMMENT ON COLUMN dedup.finv_dedup_daily_budget.hard_cap_rows IS '硬上限，防止压采集';
COMMENT ON COLUMN dedup.finv_dedup_daily_budget.consumed_rows IS '当日已消耗条数';
COMMENT ON COLUMN dedup.finv_dedup_daily_budget.consumed_bytes IS '当日已消耗字节数';
COMMENT ON COLUMN dedup.finv_dedup_daily_budget.status IS '预算状态：CALCULATED / RUNNING / DONE';
COMMENT ON COLUMN dedup.finv_dedup_daily_budget.gmt_create IS '预算生成时间';

-- ---------------------------------------------------------------------
-- 3. 扫描水位
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.finv_dedup_scan_watermark (
  watermark_name  text PRIMARY KEY,
  last_value      timestamptz NOT NULL,
  gmt_update      timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE dedup.finv_dedup_scan_watermark IS '增量扫描水位，如成熟度扫描、补丁扫描；watermark_name 约定见 Docs/Design/2610-SecuritiesQuoteExportArchive.md';
COMMENT ON COLUMN dedup.finv_dedup_scan_watermark.watermark_name IS '水位名称';
COMMENT ON COLUMN dedup.finv_dedup_scan_watermark.last_value IS '上次扫描到的最大时间';
COMMENT ON COLUMN dedup.finv_dedup_scan_watermark.gmt_update IS '更新时间';

-- ---------------------------------------------------------------------
-- 4. 证券归档优先级与 SLA
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.finv_dedup_usc_priority (
  usc            text PRIMARY KEY,
  priority_rank  int  NOT NULL,
  sla_days       int  NOT NULL,
  max_wait_days  int  NOT NULL DEFAULT 7,
  gmt_update     timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE dedup.finv_dedup_usc_priority IS '证券归档优先级与 SLA 配置，每日生效；缺配置时按 priority_rank=100、sla_days=5 处理';
COMMENT ON COLUMN dedup.finv_dedup_usc_priority.usc IS '证券唯一编码，逻辑关联 dedup.finv_dedup_usc.usc';
COMMENT ON COLUMN dedup.finv_dedup_usc_priority.priority_rank IS '优先级，数值越小越优先';
COMMENT ON COLUMN dedup.finv_dedup_usc_priority.sla_days IS '收口（seal_date）后多少天内必须完成导出，决定 freeze_date';
COMMENT ON COLUMN dedup.finv_dedup_usc_priority.max_wait_days IS '低优先级最长等待天数，超过后提升排序防饿死';
COMMENT ON COLUMN dedup.finv_dedup_usc_priority.gmt_update IS '配置更新时间';

-- ---------------------------------------------------------------------
-- 5. 按证券日维护的数据成熟度
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.finv_dedup_quote_maturity_mon (
  usc              text NOT NULL,
  trade_date       int4 NOT NULL,
  biz_month        int4 NOT NULL,
  expected_rows    int4 NOT NULL,
  actual_rows      int4 NOT NULL DEFAULT 0,
  maturity_status  text NOT NULL,
  gmt_first_seen   timestamptz,
  gmt_last_arrival timestamptz,
  gmt_seale        timestamptz,
  gmt_update       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (usc, trade_date)
);

CREATE INDEX IF NOT EXISTS idx_dedupquotematuritymon_month_status
    ON dedup.finv_dedup_quote_maturity_mon (biz_month, maturity_status);
CREATE INDEX IF NOT EXISTS idx_dedupquotematuritymon_status_usc
    ON dedup.finv_dedup_quote_maturity_mon (maturity_status, usc);

COMMENT ON TABLE dedup.finv_dedup_quote_maturity_mon IS '按证券日维护的数据成熟度，由主表聚合生成；只有 COMPLETE / FROZEN 的证券月进入导出候选';
COMMENT ON COLUMN dedup.finv_dedup_quote_maturity_mon.usc IS '证券唯一编码';
COMMENT ON COLUMN dedup.finv_dedup_quote_maturity_mon.trade_date IS '交易日期 yyyyMMdd';
COMMENT ON COLUMN dedup.finv_dedup_quote_maturity_mon.biz_month IS '归属月份 yyyyMM，由 trade_date/100 推导';
COMMENT ON COLUMN dedup.finv_dedup_quote_maturity_mon.expected_rows IS '预期条数，来自 dedup.finv_dedup_usc.day_expect_rows 与交易日历；非交易日为 0，未知预期为 0 且状态保持 PARTIAL';
COMMENT ON COLUMN dedup.finv_dedup_quote_maturity_mon.actual_rows IS '主表实际入库条数';
COMMENT ON COLUMN dedup.finv_dedup_quote_maturity_mon.maturity_status IS '成熟度：IMMATURE / PARTIAL / COMPLETE / FROZEN';
COMMENT ON COLUMN dedup.finv_dedup_quote_maturity_mon.gmt_first_seen IS '首次发现该日数据的时间';
COMMENT ON COLUMN dedup.finv_dedup_quote_maturity_mon.gmt_last_arrival IS '最近一次有数据到达的时间';
COMMENT ON COLUMN dedup.finv_dedup_quote_maturity_mon.gmt_seale IS '该日被收口冻结的时间（列名为方案稿原名，语义为 seal 时间）';
COMMENT ON COLUMN dedup.finv_dedup_quote_maturity_mon.gmt_update IS '更新时间';

-- ---------------------------------------------------------------------
-- 6. 证券交易日历
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.finv_dedup_trade_calendar (
  usc             text NOT NULL,
  trade_date      int4 NOT NULL,
  flag_trade_day  boolean NOT NULL,
  PRIMARY KEY (usc, trade_date)
);

COMMENT ON TABLE dedup.finv_dedup_trade_calendar IS '证券交易日历，trade_date 为 yyyyMMdd；7x24 证券全部为 true；表为空时按「交易日」保守处理';
COMMENT ON COLUMN dedup.finv_dedup_trade_calendar.usc IS '证券唯一编码';
COMMENT ON COLUMN dedup.finv_dedup_trade_calendar.trade_date IS '交易日期 yyyyMMdd';
COMMENT ON COLUMN dedup.finv_dedup_trade_calendar.flag_trade_day IS '是否交易日，true/false';

-- ---------------------------------------------------------------------
-- 7. 任务运行日志
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.finv_dedup_job_run_log_export_file (
  run_id          bigserial PRIMARY KEY,
  job_kind        text NOT NULL,
  job_id          bigint,
  usc             text,
  biz_month       int4,
  gmt_start       timestamptz NOT NULL DEFAULT now(),
  gmt_finish      timestamptz,
  status          text,
  processed_rows  bigint,
  processed_bytes bigint,
  error           text
);

CREATE INDEX IF NOT EXISTS idx_dedupjobrunlogexportfile_kind_time
    ON dedup.finv_dedup_job_run_log_export_file (job_kind, gmt_start DESC);
CREATE INDEX IF NOT EXISTS idx_dedupjobrunlogexportfile_job
    ON dedup.finv_dedup_job_run_log_export_file (job_id);

COMMENT ON TABLE dedup.finv_dedup_job_run_log_export_file IS '任务运行日志，记录每次 Job 执行的耗时、处理量与结果；job_id 逻辑关联 dedup.finv_dedup_export_job.job_id，无外键';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.run_id IS '运行记录主键';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.job_kind IS 'Job 类型：CALC_DAILY_INGEST_STATS / CALC_DAILY_EXPORT_BUDGET / REFRESH_QUOTE_MATURITY / MARK_SEALED_JOBS_FROZEN / RUN_DUE_EXPORT_JOBS / UPLOAD_PENDING_EXPORT_FILES / SCAN_POST_SEAL_PATCHES / CLEAN_EXPIRED_EXPORT_ARTIFACTS / RECOVER_TIMEOUT_EXPORT_JOBS / CHECK_EXPORT_JOB_HEALTH';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.job_id IS '关联的导出任务 ID，可为空，逻辑关联 dedup.finv_dedup_export_job.job_id';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.usc IS '证券唯一编码，可为空';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.biz_month IS '归档月份 yyyyMM，可为空';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.gmt_start IS '开始时间';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.gmt_finish IS '结束时间';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.status IS '运行结果：SUCCESS / FAILED / PAUSED';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.processed_rows IS '本次处理行数';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.processed_bytes IS '本次处理字节数';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.error IS '错误信息或差异报告摘要';

-- ---------------------------------------------------------------------
-- 8. 证券主数据（供 MVsv 文件头与文件名）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.finv_dedup_usc (
  usc                 text PRIMARY KEY,
  category            text,
  day_expect_rows     int,
  date_list           date,
  date_delist         date,
  flag_auto_discover  boolean NOT NULL DEFAULT true,
  region              text,
  market              text,
  provider            text,
  name                text,
  timezone            text,
  currency            text,
  gmt_create          timestamptz NOT NULL DEFAULT now(),
  gmt_update          timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE dedup.finv_dedup_usc IS '证券主数据，可基于主表自动发现新 usc；region/market/provider/name/timezone/currency 用于生成 MVsv 文件头与文件名，缺失时不生成导出候选';
COMMENT ON COLUMN dedup.finv_dedup_usc.usc IS '证券唯一编码';
COMMENT ON COLUMN dedup.finv_dedup_usc.category IS '证券类别：A_SHARE / CRYPTO / ...';
COMMENT ON COLUMN dedup.finv_dedup_usc.day_expect_rows IS '预期每日条数：A 股 241，加密货币 1440；为空时该日成熟度保持 PARTIAL';
COMMENT ON COLUMN dedup.finv_dedup_usc.date_list IS '上市/上线日期';
COMMENT ON COLUMN dedup.finv_dedup_usc.date_delist IS '退市/下线日期，空表示仍在市';
COMMENT ON COLUMN dedup.finv_dedup_usc.flag_auto_discover IS '是否由系统自动发现';
COMMENT ON COLUMN dedup.finv_dedup_usc.region IS '区域代码，如 CN，用于文件名与 MVsv 文件头 Region';
COMMENT ON COLUMN dedup.finv_dedup_usc.market IS '市场代码，如 SZ，用于文件名与 MVsv 文件头 Market';
COMMENT ON COLUMN dedup.finv_dedup_usc.provider IS '数据提供方，如 FTMM，用于文件名与 MVsv 文件头 DataProvider';
COMMENT ON COLUMN dedup.finv_dedup_usc.name IS '证券名称，如 黄金ETF易方达，用于 MVsv 文件头 Name';
COMMENT ON COLUMN dedup.finv_dedup_usc.timezone IS '时区，如 Asia/Shanghai，用于 MVsv 文件头 TimeZone';
COMMENT ON COLUMN dedup.finv_dedup_usc.currency IS '币种，如 CNY，用于 MVsv 文件头 Currency';
COMMENT ON COLUMN dedup.finv_dedup_usc.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.finv_dedup_usc.gmt_update IS '更新时间';

-- ---------------------------------------------------------------------
-- 9. 证券月导出任务
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.finv_dedup_export_job (
  job_id              bigserial PRIMARY KEY,
  usc                 text NOT NULL,
  biz_month           int4 NOT NULL,
  priority_rank       int4 NOT NULL,
  freeze_date         date NOT NULL,
  seal_date           date NOT NULL,
  status              text NOT NULL,
  load_cursor_date    int4,
  load_cursor_ts      int8,
  loaded_rows         bigint NOT NULL DEFAULT 0,
  base_version        int4 NOT NULL DEFAULT 0,
  patch_count         int4 NOT NULL DEFAULT 0,
  oldest_pending_date int4,
  locked_by           text,
  locked_until        timestamptz,
  gmt_heartbeat       timestamptz,
  attempt             int4 NOT NULL DEFAULT 0,
  gmt_next_run        timestamptz,
  last_error          text,
  gmt_create          timestamptz NOT NULL DEFAULT now(),
  gmt_update          timestamptz NOT NULL DEFAULT now(),
  UNIQUE (usc, biz_month)
);

CREATE INDEX IF NOT EXISTS idx_dedupexportjob_pick
    ON dedup.finv_dedup_export_job (status, priority_rank, freeze_date, oldest_pending_date)
    WHERE status IN ('WAITING', 'PAUSED');

COMMENT ON TABLE dedup.finv_dedup_export_job IS '证券月导出任务表，记录状态、游标、锁与版本；与 dedup.finv_dedup_usc 由 usc 逻辑关联，无外键；同一 (usc, biz_month) 唯一保证幂等';
COMMENT ON COLUMN dedup.finv_dedup_export_job.job_id IS '任务主键，被 finv_dedup_export_file.job_id、finv_dedup_export_file_patch.job_id、finv_dedup_job_run_log_export_file.job_id 逻辑引用';
COMMENT ON COLUMN dedup.finv_dedup_export_job.usc IS '证券唯一编码，逻辑关联 dedup.finv_dedup_usc.usc';
COMMENT ON COLUMN dedup.finv_dedup_export_job.biz_month IS '归档月份 yyyyMM';
COMMENT ON COLUMN dedup.finv_dedup_export_job.priority_rank IS '领取时的优先级快照，每日刷新';
COMMENT ON COLUMN dedup.finv_dedup_export_job.freeze_date IS '最晚完成日期 = seal_date + sla_days，到期强制收口并优先处理';
COMMENT ON COLUMN dedup.finv_dedup_export_job.seal_date IS '收口日期（次月 25 日），之后到达的数据走 Patch';
COMMENT ON COLUMN dedup.finv_dedup_export_job.status IS '状态：WAITING / LOADING / LOADED / EXPORTING / EXPORTED / PAUSED / FAILED / FROZEN';
COMMENT ON COLUMN dedup.finv_dedup_export_job.load_cursor_date IS '装载游标：已处理到的交易日期 yyyyMMdd';
COMMENT ON COLUMN dedup.finv_dedup_export_job.load_cursor_ts IS '装载游标：当日已处理到的最大 ts';
COMMENT ON COLUMN dedup.finv_dedup_export_job.loaded_rows IS '已装载到中转表的总行数';
COMMENT ON COLUMN dedup.finv_dedup_export_job.base_version IS 'base 文件当前版本号，L1_HOT 重写时递增';
COMMENT ON COLUMN dedup.finv_dedup_export_job.patch_count IS '已应用 patch 数量，Patch 文件名中的 n 由它推导';
COMMENT ON COLUMN dedup.finv_dedup_export_job.oldest_pending_date IS '最老待处理日期，用于排序防旧数据滞留';
COMMENT ON COLUMN dedup.finv_dedup_export_job.locked_by IS '持有任务的 worker 标识';
COMMENT ON COLUMN dedup.finv_dedup_export_job.locked_until IS '任务锁租约到期时间';
COMMENT ON COLUMN dedup.finv_dedup_export_job.gmt_heartbeat IS 'worker 最近心跳时间';
COMMENT ON COLUMN dedup.finv_dedup_export_job.attempt IS '已尝试次数';
COMMENT ON COLUMN dedup.finv_dedup_export_job.gmt_next_run IS '下次可运行时间，用于指数退避';
COMMENT ON COLUMN dedup.finv_dedup_export_job.last_error IS '最后一次错误信息';
COMMENT ON COLUMN dedup.finv_dedup_export_job.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.finv_dedup_export_job.gmt_update IS '更新时间';

-- ---------------------------------------------------------------------
-- 10. 导出文件登记（上传任务的唯一数据源）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.finv_dedup_export_file (
  file_id                  bigserial PRIMARY KEY,
  job_id                   bigint NOT NULL,
  usc                      text NOT NULL,
  biz_month                int4 NOT NULL,
  file_type                text NOT NULL,
  version                  int4 NOT NULL,
  file_name                text NOT NULL,
  file_path                text NOT NULL,
  target_file_name         text NOT NULL,
  upload_target            text,
  format                   text NOT NULL DEFAULT 'mvsv',
  compression              text NOT NULL DEFAULT 'none',
  row_count                bigint NOT NULL,
  file_size                bigint NOT NULL,
  checksum                 text NOT NULL,
  manifest_path            text,
  seal_date                date,
  export_status            text NOT NULL DEFAULT 'EXPORTED',
  upload_status            text NOT NULL DEFAULT 'PENDING_UPLOAD',
  upload_attempt           int4 NOT NULL DEFAULT 0,
  gmt_next_retry           timestamptz,
  gmt_upload_start         timestamptz,
  gmt_upload_finish        timestamptz,
  remote_checksum_verified boolean NOT NULL DEFAULT false,
  network_status           text NOT NULL DEFAULT 'NONE',
  cleanup_status           text NOT NULL DEFAULT 'NONE',
  gmt_create               timestamptz NOT NULL DEFAULT now(),
  gmt_update               timestamptz NOT NULL DEFAULT now(),
  UNIQUE (usc, biz_month, file_type, version)
);

CREATE INDEX IF NOT EXISTS idx_dedupexportfile_upload
    ON dedup.finv_dedup_export_file (upload_status, gmt_next_retry, gmt_create);
CREATE INDEX IF NOT EXISTS idx_dedupexportfile_network
    ON dedup.finv_dedup_export_file (network_status, cleanup_status);
CREATE INDEX IF NOT EXISTS idx_dedupexportfile_job
    ON dedup.finv_dedup_export_file (job_id);

COMMENT ON TABLE dedup.finv_dedup_export_file IS '导出文件登记表，每个物理文件一条记录，是上传独享存储任务的唯一数据源；job_id 逻辑关联 finv_dedup_export_job.job_id，无外键';
COMMENT ON COLUMN dedup.finv_dedup_export_file.file_id IS '文件记录主键，被 finv_dedup_export_file_upload_log.file_id 与 finv_dedup_export_file_patch.patch_file_id 逻辑引用';
COMMENT ON COLUMN dedup.finv_dedup_export_file.job_id IS '关联的证券月导出任务 ID，逻辑关联 dedup.finv_dedup_export_job.job_id';
COMMENT ON COLUMN dedup.finv_dedup_export_file.usc IS '证券唯一编码';
COMMENT ON COLUMN dedup.finv_dedup_export_file.biz_month IS '归档月份 yyyyMM';
COMMENT ON COLUMN dedup.finv_dedup_export_file.file_type IS '文件类型：BASE / PATCH / MANIFEST';
COMMENT ON COLUMN dedup.finv_dedup_export_file.version IS '文件版本号，base 与 patch 独立递增，MANIFEST 每次登记自增';
COMMENT ON COLUMN dedup.finv_dedup_export_file.file_name IS '本地文件名，BASE 为 region_market_usc_Min_provider_Mon_yyyyMM.mvsv，PATCH 为 ..._Incr_n.mvsv，MANIFEST 为 ..._Manifest.json';
COMMENT ON COLUMN dedup.finv_dedup_export_file.file_path IS '本地绝对路径';
COMMENT ON COLUMN dedup.finv_dedup_export_file.target_file_name IS '上传目标文件名，默认与 file_name 相同；当前不做扩展名改名';
COMMENT ON COLUMN dedup.finv_dedup_export_file.upload_target IS '独享存储目标路径，含目标文件名';
COMMENT ON COLUMN dedup.finv_dedup_export_file.format IS '文件格式，固定 mvsv';
COMMENT ON COLUMN dedup.finv_dedup_export_file.compression IS '压缩方式：none / gzip（扩展名保持 .mvsv）';
COMMENT ON COLUMN dedup.finv_dedup_export_file.row_count IS '数据行数，不含 16 行文件头和 1 行空行；MANIFEST 记录 0';
COMMENT ON COLUMN dedup.finv_dedup_export_file.file_size IS '文件字节数';
COMMENT ON COLUMN dedup.finv_dedup_export_file.checksum IS '文件 SHA256（小写十六进制），上传前后校验';
COMMENT ON COLUMN dedup.finv_dedup_export_file.manifest_path IS '该文件对应的 Manifest 本地路径；MANIFEST 自身为空';
COMMENT ON COLUMN dedup.finv_dedup_export_file.seal_date IS '导出时的收口日期，用于 Patch 级别判定';
COMMENT ON COLUMN dedup.finv_dedup_export_file.export_status IS '导出状态：EXPORTED / SUPERSEDED / SKIPPED';
COMMENT ON COLUMN dedup.finv_dedup_export_file.upload_status IS '上传状态：PENDING_UPLOAD / UPLOADING / UPLOADED / UPLOAD_FAILED / SKIPPED';
COMMENT ON COLUMN dedup.finv_dedup_export_file.upload_attempt IS '上传尝试次数';
COMMENT ON COLUMN dedup.finv_dedup_export_file.gmt_next_retry IS '下次可重试上传时间，指数退避';
COMMENT ON COLUMN dedup.finv_dedup_export_file.gmt_upload_start IS '最近一次上传开始时间';
COMMENT ON COLUMN dedup.finv_dedup_export_file.gmt_upload_finish IS '最近一次上传结束时间';
COMMENT ON COLUMN dedup.finv_dedup_export_file.remote_checksum_verified IS '远端 checksum 是否校验通过';
COMMENT ON COLUMN dedup.finv_dedup_export_file.network_status IS '网盘归档状态：NONE / ARCHIVED';
COMMENT ON COLUMN dedup.finv_dedup_export_file.cleanup_status IS '本地清理状态：NONE / LOCAL_DELETED';
COMMENT ON COLUMN dedup.finv_dedup_export_file.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.finv_dedup_export_file.gmt_update IS '更新时间';

-- ---------------------------------------------------------------------
-- 11. 上传尝试审计日志
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.finv_dedup_export_file_upload_log (
  log_id          bigserial PRIMARY KEY,
  file_id         bigint NOT NULL,
  attempt         int4 NOT NULL,
  target_path     text NOT NULL,
  gmt_start       timestamptz NOT NULL,
  gmt_finish      timestamptz,
  status          text NOT NULL,
  uploaded_bytes  bigint,
  error           text
);

CREATE INDEX IF NOT EXISTS idx_dedupexportfileuploadlog_file
    ON dedup.finv_dedup_export_file_upload_log (file_id, attempt);

COMMENT ON TABLE dedup.finv_dedup_export_file_upload_log IS '上传尝试审计日志，每次上传一条；file_id 逻辑关联 finv_dedup_export_file.file_id，无外键';
COMMENT ON COLUMN dedup.finv_dedup_export_file_upload_log.log_id IS '日志主键';
COMMENT ON COLUMN dedup.finv_dedup_export_file_upload_log.file_id IS '关联的导出文件记录 ID，逻辑关联 dedup.finv_dedup_export_file.file_id';
COMMENT ON COLUMN dedup.finv_dedup_export_file_upload_log.attempt IS '第几次上传尝试';
COMMENT ON COLUMN dedup.finv_dedup_export_file_upload_log.target_path IS '本次上传目标路径';
COMMENT ON COLUMN dedup.finv_dedup_export_file_upload_log.gmt_start IS '上传开始时间';
COMMENT ON COLUMN dedup.finv_dedup_export_file_upload_log.gmt_finish IS '上传结束时间';
COMMENT ON COLUMN dedup.finv_dedup_export_file_upload_log.status IS '上传结果：SUCCESS / FAILED';
COMMENT ON COLUMN dedup.finv_dedup_export_file_upload_log.uploaded_bytes IS '本次上传字节数';
COMMENT ON COLUMN dedup.finv_dedup_export_file_upload_log.error IS '失败原因';

-- ---------------------------------------------------------------------
-- 12. 收口后补丁任务
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.finv_dedup_export_file_patch (
  patch_id       bigserial PRIMARY KEY,
  job_id         bigint NOT NULL,
  usc            text NOT NULL,
  biz_month      int4 NOT NULL,
  patch_level    text NOT NULL,
  patch_version  int4 NOT NULL,
  source_rows    bigint NOT NULL,
  patch_rows     bigint NOT NULL,
  reason         text NOT NULL,
  arrival_window tstzrange,
  status         text NOT NULL,
  patch_file_id  bigint,
  gmt_create     timestamptz NOT NULL DEFAULT now(),
  gmt_finish     timestamptz,
  last_error     text
);

CREATE INDEX IF NOT EXISTS idx_dedupexportfilepatch_pending
    ON dedup.finv_dedup_export_file_patch (status, gmt_create);
CREATE INDEX IF NOT EXISTS idx_dedupexportfilepatch_job
    ON dedup.finv_dedup_export_file_patch (job_id);
CREATE INDEX IF NOT EXISTS idx_dedupexportfilepatch_file
    ON dedup.finv_dedup_export_file_patch (patch_file_id);

COMMENT ON TABLE dedup.finv_dedup_export_file_patch IS '收口后到达数据的补丁任务表；job_id 逻辑关联 finv_dedup_export_job，patch_file_id 逻辑关联 finv_dedup_export_file，无外键';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.patch_id IS '补丁任务主键';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.job_id IS '关联的证券月导出任务 ID，逻辑关联 dedup.finv_dedup_export_job.job_id';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.usc IS '证券唯一编码';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.biz_month IS '归档月份 yyyyMM';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.patch_level IS '补丁级别：L1_HOT / L2_WARM / L3_COLD';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.patch_version IS '补丁序号 n，对应文件名 ..._Incr_{n}.mvsv 中的 n';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.source_rows IS '补丁来源数据行数';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.patch_rows IS '实际写入补丁文件行数';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.reason IS '补丁原因：LATE_ARRIVAL / UPSTREAM_DELAY / CORRECTION';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.arrival_window IS '触发补丁的数据到达时间窗口';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.status IS '补丁状态：PENDING / RUNNING / DONE / FAILED';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.patch_file_id IS '关联的补丁文件登记记录 ID，逻辑关联 dedup.finv_dedup_export_file.file_id';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.gmt_create IS '补丁任务创建时间';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.gmt_finish IS '补丁任务完成时间';
COMMENT ON COLUMN dedup.finv_dedup_export_file_patch.last_error IS '最后一次错误信息';

-- ---------------------------------------------------------------------
-- 13. 收敛校验：表数 / 外键数 / 索引名 / 所属 schema
-- ---------------------------------------------------------------------
DO $$
DECLARE
    expected_tables CONSTANT text[] := ARRAY[
        'finv_dedup_ingest_stats',
        'finv_dedup_daily_budget',
        'finv_dedup_scan_watermark',
        'finv_dedup_usc_priority',
        'finv_dedup_quote_maturity_mon',
        'finv_dedup_trade_calendar',
        'finv_dedup_job_run_log_export_file',
        'finv_dedup_usc',
        'finv_dedup_export_job',
        'finv_dedup_export_file',
        'finv_dedup_export_file_upload_log',
        'finv_dedup_export_file_patch'
    ];
    expected_indexes CONSTANT text[] := ARRAY[
        'idx_dedupquotematuritymon_month_status',
        'idx_dedupquotematuritymon_status_usc',
        'idx_dedupjobrunlogexportfile_kind_time',
        'idx_dedupjobrunlogexportfile_job',
        'idx_dedupexportjob_pick',
        'idx_dedupexportfile_upload',
        'idx_dedupexportfile_network',
        'idx_dedupexportfile_job',
        'idx_dedupexportfileuploadlog_file',
        'idx_dedupexportfilepatch_pending',
        'idx_dedupexportfilepatch_job',
        'idx_dedupexportfilepatch_file'
    ];
    missing_tables  text;
    missing_indexes text;
    wrong_schema    text;
    fk_count        int;
    nonempty_fk     int;
BEGIN
    SELECT string_agg(u.name, ', ') INTO missing_tables
      FROM unnest(expected_tables) AS u(name)
     WHERE NOT EXISTS (
        SELECT 1 FROM pg_tables WHERE schemaname = 'dedup' AND tablename = u.name
     );
    IF missing_tables IS NOT NULL THEN
        RAISE EXCEPTION 'V41: dedup schema 缺少表 %', missing_tables;
    END IF;

    SELECT string_agg(i.name, ', ') INTO missing_indexes
      FROM unnest(expected_indexes) AS i(name)
     WHERE NOT EXISTS (
        SELECT 1 FROM pg_indexes WHERE schemaname = 'dedup' AND indexname = i.name
     );
    IF missing_indexes IS NOT NULL THEN
        RAISE EXCEPTION 'V41: dedup schema 缺少索引 %', missing_indexes;
    END IF;

    SELECT string_agg(tablename, ', ') INTO wrong_schema
      FROM pg_tables
     WHERE tablename = ANY (expected_tables)
       AND schemaname <> 'dedup';
    IF wrong_schema IS NOT NULL THEN
        RAISE EXCEPTION 'V41: 表应全部位于 dedup schema，实际有 %', wrong_schema;
    END IF;

    SELECT count(*) INTO fk_count
      FROM pg_constraint c
      JOIN pg_class r ON r.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = r.relnamespace
     WHERE c.contype = 'f' AND n.nspname = 'dedup' AND r.relname = ANY (expected_tables);
    IF fk_count <> 0 THEN
        RAISE EXCEPTION 'V41: 新增表不允许外键，实际有 % 个', fk_count;
    END IF;

    -- 主键与非空约束不属于外键；此处只做一次全量防回归断言。
    SELECT count(*) INTO nonempty_fk
      FROM pg_constraint c
      JOIN pg_class r ON r.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = r.relnamespace
     WHERE c.contype = 'f' AND n.nspname = 'dedup';
    IF nonempty_fk <> 0 THEN
        RAISE EXCEPTION 'V41: dedup schema 不允许外键，实际有 % 个', nonempty_fk;
    END IF;

    RAISE NOTICE 'V41: 校验通过（12 张表 + 12 个索引就绪，dedup 内无外键）';
END $$;

COMMIT;
