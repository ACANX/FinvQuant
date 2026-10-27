-- =====================================================================
-- FinvQuant PostgreSQL V100078：在 cdeu schema 建立与 dedup 对应的表结构（需求 10754）
--
-- 命名规则（用户 2026-10-28 指定）：
--   1. `dedup_` 开头的表       → 前缀替换为 `finv_quote_cdeu_`
--   2. `finv_dedup_` 开头的表  → 前缀替换为 `finv_quote_cdeu_`
--   3. 其他表名保持不变（`cdeu_*` 12 张、`finv_quote_secu_kline_min` 1 张）
--   4. `prep_bak` 开头的表不在 cdeu 中创建（本库 5 张，全部排除）
--
-- 本次只建表结构，**不发放任何初始数据**（无 seed、无数据搬迁）。
--
-- 派生标识符同样按词根替换，保持与表名一致：
--   `finv_dedup_*` → `finv_quote_cdeu_*`（约束 / 主键 / 唯一键 / 序列）
--   `idx_dedup*`   → `idx_quotecdeu*`   （按 V100041 的 `idx_<表名去finv_与下划线>_<用途>` 规范）
--   `uq_dedup*`    → `uq_quotecdeu*`
--   `cdeu_*` 系列标识符保持不变（表名未变）
--
-- 唯一例外：`finv_quote_cdeu_export_batch_batch_no_usc_biz_month_export_type_key`
-- 加前缀后为 67 字符，超过 PostgreSQL 的 63 字符标识符上限（会被静默截断），
-- 改用短名 `uq_finv_quote_cdeu_export_batch`（33 字符），语义不变。
--
-- 结构来源：`pg_dump --schema-only -n dedup`（PostgreSQL 18.4），保证列 / 类型 / 默认值 /
-- 行内 CHECK / NOT NULL 与 dedup 完全一致，不做任何人工增减。
-- 触发器 `trg_finv_quote_secu_kline_min_gmt_update` 引用 `public.vq_set_gmt_update()`，
-- 该函数与 schema 无关，跨 schema 复用无需重建。
--
-- 本脚本全部使用 `CREATE ...`（无 IF NOT EXISTS）：迁移框架按 schema_version 保证只执行一次，
-- 重复执行应报错而非静默通过，便于暴露「cdeu 下已存在同名对象」的异常状态。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 0. 前置校验
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'cdeu') THEN
        RAISE EXCEPTION 'V100078 前置校验失败：cdeu schema 不存在（需先执行 V100077）';
    END IF;
    RAISE NOTICE 'V100078 前置校验：cdeu schema 就绪，开始建表';
END $$;

CREATE TABLE cdeu.cdeu_job_dynamic (
    dynamic_job_id bigint CONSTRAINT cdeu_dynamic_job_dynamic_job_id_not_null NOT NULL,
    process_id bigint CONSTRAINT cdeu_dynamic_job_process_id_not_null NOT NULL,
    job_code text CONSTRAINT cdeu_dynamic_job_job_code_not_null NOT NULL,
    name text CONSTRAINT cdeu_dynamic_job_name_not_null NOT NULL,
    usc text CONSTRAINT cdeu_dynamic_job_usc_not_null NOT NULL,
    biz_month integer CONSTRAINT cdeu_dynamic_job_biz_month_not_null NOT NULL,
    export_type text,
    export_job_id bigint,
    seq_no integer DEFAULT 1 CONSTRAINT cdeu_dynamic_job_seq_no_not_null NOT NULL,
    status text DEFAULT 'PENDING'::text CONSTRAINT cdeu_dynamic_job_status_not_null NOT NULL,
    error text,
    gmt_start timestamp with time zone,
    gmt_finish timestamp with time zone,
    gmt_create timestamp with time zone DEFAULT now() CONSTRAINT cdeu_dynamic_job_gmt_create_not_null NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() CONSTRAINT cdeu_dynamic_job_gmt_update_not_null NOT NULL,
    job_category text DEFAULT 'EXPORT'::text NOT NULL,
    file_id bigint,
    file_type text,
    file_version integer,
    run_log_id bigint,
    workflow_run_id bigint,
    CONSTRAINT cdeu_dynamic_job_export_type_check CHECK ((export_type = ANY (ARRAY['BASE'::text, 'INCR'::text]))),
    CONSTRAINT cdeu_dynamic_job_status_check CHECK ((status = ANY (ARRAY['PENDING'::text, 'RUNNING'::text, 'SUCCESS'::text, 'PARTIAL'::text, 'FAILED'::text, 'PAUSED'::text, 'CANCELLED'::text, 'SKIPPED'::text]))),
    CONSTRAINT cdeu_job_dynamic_job_category_check CHECK ((job_category = ANY (ARRAY['EXPORT'::text, 'UPLOAD'::text])))
);

CREATE TABLE cdeu.cdeu_job (
    job_id bigint NOT NULL,
    process_id bigint CONSTRAINT cdeu_job_step_id_not_null NOT NULL,
    job_code text NOT NULL,
    name text NOT NULL,
    job_kind text NOT NULL,
    execution_stage text,
    seq_no integer NOT NULL,
    critical boolean DEFAULT true NOT NULL,
    affects_completion boolean DEFAULT true NOT NULL,
    trigger_mode text DEFAULT 'SCHEDULED'::text NOT NULL,
    execution_mode text DEFAULT 'DIRECT'::text NOT NULL,
    manual_rerun_enabled boolean DEFAULT true NOT NULL,
    schedule_kind text DEFAULT 'NONE'::text NOT NULL,
    schedule_value text,
    timeout_seconds integer,
    max_attempts integer DEFAULT 1 NOT NULL,
    retry_backoff_seconds integer DEFAULT 60 NOT NULL,
    job_params jsonb DEFAULT '{}'::jsonb NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    gmt_create timestamp with time zone DEFAULT now() NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL,
    ui_visible boolean DEFAULT true NOT NULL,
    CONSTRAINT cdeu_job_execution_mode_check CHECK ((execution_mode = ANY (ARRAY['DIRECT'::text, 'EMBEDDED'::text]))),
    CONSTRAINT cdeu_job_schedule_kind_check CHECK ((schedule_kind = ANY (ARRAY['DAILY'::text, 'INTERVAL'::text, 'NONE'::text]))),
    CONSTRAINT cdeu_job_trigger_mode_check CHECK ((trigger_mode = ANY (ARRAY['SCHEDULED'::text, 'EMBEDDED'::text, 'MANUAL'::text])))
);

CREATE TABLE cdeu.cdeu_job_run (
    job_id bigint NOT NULL,
    job_code text NOT NULL,
    job_kind text NOT NULL,
    execution_stage text,
    batch_key text,
    trigger_type text DEFAULT 'PROCESS'::text NOT NULL,
    status text DEFAULT 'PENDING'::text NOT NULL,
    attempt integer DEFAULT 0 NOT NULL,
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    processed_rows bigint DEFAULT 0 NOT NULL,
    processed_bytes bigint DEFAULT 0 NOT NULL,
    run_log_id bigint,
    error text,
    gmt_create timestamp with time zone DEFAULT now() NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL,
    workflow_run_id bigint NOT NULL,
    process_id bigint NOT NULL,
    dynamic_job_id bigint,
    CONSTRAINT cdeu_job_run_status_check CHECK ((status = ANY (ARRAY['PENDING'::text, 'RUNNING'::text, 'SUCCESS'::text, 'PARTIAL'::text, 'FAILED'::text, 'PAUSED'::text, 'CANCELLED'::text, 'SKIPPED'::text]))),
    CONSTRAINT cdeu_job_run_trigger_type_check CHECK ((trigger_type = ANY (ARRAY['PROCESS'::text, 'MANUAL_STAGE'::text])))
);

CREATE TABLE cdeu.cdeu_job_step (
    step_id bigint NOT NULL,
    job_id bigint NOT NULL,
    step_code text NOT NULL,
    name text NOT NULL,
    seq_no integer NOT NULL,
    critical boolean DEFAULT true NOT NULL,
    affects_completion boolean DEFAULT true NOT NULL,
    timeout_seconds integer,
    enabled boolean DEFAULT true NOT NULL,
    gmt_create timestamp with time zone DEFAULT now() NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE cdeu.cdeu_job_step_dynamic (
    dynamic_step_id bigint NOT NULL,
    dynamic_job_id bigint NOT NULL,
    step_code text NOT NULL,
    name text NOT NULL,
    seq_no integer NOT NULL,
    status text DEFAULT 'PENDING'::text NOT NULL,
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    error text,
    gmt_create timestamp with time zone DEFAULT now() NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cdeu_job_step_dynamic_status_check CHECK ((status = ANY (ARRAY['PENDING'::text, 'RUNNING'::text, 'SUCCESS'::text, 'PARTIAL'::text, 'FAILED'::text, 'PAUSED'::text, 'CANCELLED'::text, 'SKIPPED'::text])))
);

CREATE TABLE cdeu.cdeu_job_step_run (
    step_run_id bigint NOT NULL,
    step_id bigint NOT NULL,
    step_code text NOT NULL,
    seq_no integer NOT NULL,
    status text DEFAULT 'PENDING'::text NOT NULL,
    attempt integer DEFAULT 0 NOT NULL,
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    processed_rows bigint DEFAULT 0 NOT NULL,
    processed_bytes bigint DEFAULT 0 NOT NULL,
    error text,
    gmt_create timestamp with time zone DEFAULT now() NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL,
    workflow_run_id bigint NOT NULL,
    process_id bigint NOT NULL,
    job_id bigint NOT NULL,
    CONSTRAINT cdeu_job_step_run_status_check CHECK ((status = ANY (ARRAY['PENDING'::text, 'RUNNING'::text, 'SUCCESS'::text, 'PARTIAL'::text, 'FAILED'::text, 'PAUSED'::text, 'CANCELLED'::text, 'SKIPPED'::text])))
);

CREATE TABLE cdeu.cdeu_phase (
    phase_id bigint NOT NULL,
    workflow_id bigint CONSTRAINT cdeu_phase_process_id_not_null NOT NULL,
    phase_code text NOT NULL,
    name text NOT NULL,
    seq_no integer NOT NULL,
    critical boolean DEFAULT true NOT NULL,
    affects_completion boolean DEFAULT true NOT NULL,
    run_mode text DEFAULT 'INHERIT'::text NOT NULL,
    batch_enabled boolean DEFAULT false NOT NULL,
    completion_rule jsonb DEFAULT '{"mode": "ALL"}'::jsonb NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    gmt_create timestamp with time zone DEFAULT now() NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cdeu_phase_run_mode_check CHECK ((run_mode = ANY (ARRAY['INHERIT'::text, 'CONTINUOUS'::text, 'MANUAL'::text])))
);

CREATE TABLE cdeu.cdeu_phase_run (
    phase_run_id bigint NOT NULL,
    workflow_run_id bigint CONSTRAINT cdeu_phase_run_run_id_not_null NOT NULL,
    phase_id bigint NOT NULL,
    phase_code text NOT NULL,
    seq_no integer NOT NULL,
    status text DEFAULT 'PENDING'::text NOT NULL,
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    success_count integer DEFAULT 0 NOT NULL,
    failed_count integer DEFAULT 0 NOT NULL,
    total_count integer DEFAULT 0 NOT NULL,
    completion_rule_snapshot jsonb,
    completion_snapshot jsonb,
    error text,
    gmt_create timestamp with time zone DEFAULT now() NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cdeu_phase_run_status_check CHECK ((status = ANY (ARRAY['PENDING'::text, 'READY'::text, 'RUNNING'::text, 'SUCCESS'::text, 'PARTIAL'::text, 'FAILED'::text, 'PAUSED'::text, 'CANCELLED'::text, 'SKIPPED'::text])))
);

CREATE TABLE cdeu.cdeu_process (
    process_id bigint CONSTRAINT cdeu_step_step_id_not_null NOT NULL,
    phase_id bigint CONSTRAINT cdeu_step_phase_id_not_null NOT NULL,
    process_code text CONSTRAINT cdeu_step_step_code_not_null NOT NULL,
    name text CONSTRAINT cdeu_step_name_not_null NOT NULL,
    seq_no integer CONSTRAINT cdeu_step_seq_no_not_null NOT NULL,
    critical boolean DEFAULT true CONSTRAINT cdeu_step_critical_not_null NOT NULL,
    affects_completion boolean DEFAULT true CONSTRAINT cdeu_step_affects_completion_not_null NOT NULL,
    run_mode text DEFAULT 'INHERIT'::text CONSTRAINT cdeu_step_run_mode_not_null NOT NULL,
    batch_enabled boolean DEFAULT false CONSTRAINT cdeu_step_batch_enabled_not_null NOT NULL,
    batch_source text,
    batch_key_field text,
    batch_size integer,
    max_parallel integer DEFAULT 1 CONSTRAINT cdeu_step_max_parallel_not_null NOT NULL,
    failure_policy text DEFAULT 'CONTINUE'::text CONSTRAINT cdeu_step_failure_policy_not_null NOT NULL,
    completion_rule jsonb DEFAULT '{"mode": "ALL"}'::jsonb CONSTRAINT cdeu_step_completion_rule_not_null NOT NULL,
    enabled boolean DEFAULT true CONSTRAINT cdeu_step_enabled_not_null NOT NULL,
    gmt_create timestamp with time zone DEFAULT now() CONSTRAINT cdeu_step_gmt_create_not_null NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() CONSTRAINT cdeu_step_gmt_update_not_null NOT NULL,
    CONSTRAINT cdeu_step_failure_policy_check CHECK ((failure_policy = ANY (ARRAY['FAIL_FAST'::text, 'CONTINUE'::text]))),
    CONSTRAINT cdeu_step_run_mode_check CHECK ((run_mode = ANY (ARRAY['INHERIT'::text, 'CONTINUOUS'::text, 'MANUAL'::text])))
);

CREATE TABLE cdeu.cdeu_workflow (
    workflow_id bigint CONSTRAINT cdeu_process_process_id_not_null NOT NULL,
    workflow_code text CONSTRAINT cdeu_process_process_code_not_null NOT NULL,
    name text CONSTRAINT cdeu_process_name_not_null NOT NULL,
    description text,
    version integer DEFAULT 1 CONSTRAINT cdeu_process_version_not_null NOT NULL,
    run_mode text DEFAULT 'MANUAL'::text CONSTRAINT cdeu_process_run_mode_not_null NOT NULL,
    schedule_time time without time zone,
    schedule_timezone text DEFAULT 'Asia/Shanghai'::text CONSTRAINT cdeu_process_schedule_timezone_not_null NOT NULL,
    schedule_enabled boolean DEFAULT false CONSTRAINT cdeu_process_schedule_enabled_not_null NOT NULL,
    completion_rule jsonb DEFAULT '{"mode": "ALL"}'::jsonb CONSTRAINT cdeu_process_completion_rule_not_null NOT NULL,
    enabled boolean DEFAULT true CONSTRAINT cdeu_process_enabled_not_null NOT NULL,
    gmt_create timestamp with time zone DEFAULT now() CONSTRAINT cdeu_process_gmt_create_not_null NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() CONSTRAINT cdeu_process_gmt_update_not_null NOT NULL,
    CONSTRAINT cdeu_process_run_mode_check CHECK ((run_mode = ANY (ARRAY['AUTO_DAILY'::text, 'MANUAL'::text, 'BOTH'::text])))
);

CREATE TABLE cdeu.cdeu_process_run (
    phase_run_id bigint CONSTRAINT cdeu_step_run_phase_run_id_not_null NOT NULL,
    process_id bigint CONSTRAINT cdeu_step_run_step_id_not_null NOT NULL,
    process_code text CONSTRAINT cdeu_step_run_step_code_not_null NOT NULL,
    seq_no integer CONSTRAINT cdeu_step_run_seq_no_not_null NOT NULL,
    status text DEFAULT 'PENDING'::text CONSTRAINT cdeu_step_run_status_not_null NOT NULL,
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    batch_total integer DEFAULT 0 CONSTRAINT cdeu_step_run_batch_total_not_null NOT NULL,
    batch_success integer DEFAULT 0 CONSTRAINT cdeu_step_run_batch_success_not_null NOT NULL,
    batch_failed integer DEFAULT 0 CONSTRAINT cdeu_step_run_batch_failed_not_null NOT NULL,
    completion_rule_snapshot jsonb,
    completion_snapshot jsonb,
    error text,
    gmt_create timestamp with time zone DEFAULT now() CONSTRAINT cdeu_step_run_gmt_create_not_null NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() CONSTRAINT cdeu_step_run_gmt_update_not_null NOT NULL,
    workflow_run_id bigint NOT NULL,
    CONSTRAINT cdeu_step_run_status_check CHECK ((status = ANY (ARRAY['PENDING'::text, 'READY'::text, 'RUNNING'::text, 'SUCCESS'::text, 'PARTIAL'::text, 'FAILED'::text, 'PAUSED'::text, 'CANCELLED'::text, 'SKIPPED'::text])))
);

CREATE TABLE cdeu.cdeu_workflow_run (
    run_id bigint CONSTRAINT cdeu_process_run_run_id_not_null NOT NULL,
    workflow_id bigint CONSTRAINT cdeu_process_run_process_id_not_null NOT NULL,
    workflow_code text CONSTRAINT cdeu_process_run_process_code_not_null NOT NULL,
    workflow_version integer CONSTRAINT cdeu_process_run_process_version_not_null NOT NULL,
    run_key text CONSTRAINT cdeu_process_run_run_key_not_null NOT NULL,
    biz_date date CONSTRAINT cdeu_process_run_biz_date_not_null NOT NULL,
    trigger_type text CONSTRAINT cdeu_process_run_trigger_type_not_null NOT NULL,
    status text DEFAULT 'PENDING'::text CONSTRAINT cdeu_process_run_status_not_null NOT NULL,
    scheduled_at timestamp with time zone,
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    current_phase_id bigint,
    completion_rule_snapshot jsonb,
    completion_snapshot jsonb,
    operator text,
    error text,
    gmt_create timestamp with time zone DEFAULT now() CONSTRAINT cdeu_process_run_gmt_create_not_null NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() CONSTRAINT cdeu_process_run_gmt_update_not_null NOT NULL,
    CONSTRAINT cdeu_process_run_status_check CHECK ((status = ANY (ARRAY['PENDING'::text, 'READY'::text, 'RUNNING'::text, 'SUCCESS'::text, 'PARTIAL'::text, 'FAILED'::text, 'PAUSED'::text, 'CANCELLED'::text, 'SKIPPED'::text]))),
    CONSTRAINT cdeu_process_run_trigger_type_check CHECK ((trigger_type = ANY (ARRAY['SCHEDULE'::text, 'MANUAL'::text, 'CONTINUOUS'::text])))
);

CREATE TABLE cdeu.finv_quote_cdeu_daily_budget (
    biz_date date NOT NULL,
    base_rows bigint NOT NULL,
    base_bytes bigint DEFAULT 0 NOT NULL,
    p75_rows bigint,
    backlog_ratio numeric,
    sla_pressure numeric,
    factor numeric NOT NULL,
    budget_rows bigint NOT NULL,
    budget_bytes bigint DEFAULT 0 NOT NULL,
    hard_cap_rows bigint NOT NULL,
    consumed_rows bigint DEFAULT 0 NOT NULL,
    consumed_bytes bigint DEFAULT 0 NOT NULL,
    status text NOT NULL,
    gmt_create timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE cdeu.finv_quote_cdeu_export_batch (
    batch_id bigint NOT NULL,
    batch_no text NOT NULL,
    biz_date date NOT NULL,
    usc text NOT NULL,
    biz_month integer NOT NULL,
    export_type text NOT NULL,
    source_job_id bigint,
    status text DEFAULT 'READY'::text NOT NULL,
    gmt_create timestamp with time zone DEFAULT now() NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT finv_quote_cdeu_export_batch_export_type_check CHECK ((export_type = ANY (ARRAY['BASE'::text, 'INCR'::text]))),
    CONSTRAINT finv_quote_cdeu_export_batch_status_check CHECK ((status = ANY (ARRAY['READY'::text, 'CONSUMED'::text])))
);

CREATE TABLE cdeu.finv_quote_cdeu_export_file (
    file_id bigint NOT NULL,
    job_id bigint NOT NULL,
    usc text NOT NULL,
    biz_month integer NOT NULL,
    file_type text NOT NULL,
    version integer NOT NULL,
    file_name text NOT NULL,
    file_path text NOT NULL,
    target_file_name text NOT NULL,
    upload_target text,
    format text DEFAULT 'mvsv'::text NOT NULL,
    compression text DEFAULT 'none'::text NOT NULL,
    row_count bigint NOT NULL,
    file_size bigint NOT NULL,
    checksum text NOT NULL,
    manifest_path text,
    seal_date date,
    export_status text DEFAULT 'EXPORTED'::text NOT NULL,
    upload_status text DEFAULT 'PENDING_UPLOAD'::text NOT NULL,
    upload_attempt integer DEFAULT 0 NOT NULL,
    gmt_next_retry timestamp with time zone,
    gmt_upload_start timestamp with time zone,
    gmt_upload_finish timestamp with time zone,
    remote_checksum_verified boolean DEFAULT false NOT NULL,
    network_status text DEFAULT 'NONE'::text NOT NULL,
    cleanup_status text DEFAULT 'NONE'::text NOT NULL,
    gmt_create timestamp with time zone DEFAULT now() NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE cdeu.finv_quote_cdeu_export_file_patch (
    patch_id bigint NOT NULL,
    job_id bigint NOT NULL,
    usc text NOT NULL,
    biz_month integer NOT NULL,
    patch_level text NOT NULL,
    patch_version integer NOT NULL,
    source_rows bigint NOT NULL,
    patch_rows bigint NOT NULL,
    reason text NOT NULL,
    arrival_window tstzrange,
    status text NOT NULL,
    patch_file_id bigint,
    gmt_create timestamp with time zone DEFAULT now() NOT NULL,
    gmt_finish timestamp with time zone,
    last_error text
);

CREATE TABLE cdeu.finv_quote_cdeu_export_file_upload_log (
    log_id bigint NOT NULL,
    file_id bigint NOT NULL,
    attempt integer NOT NULL,
    target_path text NOT NULL,
    gmt_start timestamp with time zone NOT NULL,
    gmt_finish timestamp with time zone,
    status text NOT NULL,
    uploaded_bytes bigint,
    error text
);

CREATE TABLE cdeu.finv_quote_cdeu_export_job (
    job_id bigint NOT NULL,
    usc text NOT NULL,
    biz_month integer NOT NULL,
    priority_rank integer NOT NULL,
    freeze_date date NOT NULL,
    seal_date date NOT NULL,
    status text NOT NULL,
    load_cursor_date integer,
    load_cursor_ts bigint,
    loaded_rows bigint DEFAULT 0 NOT NULL,
    base_version integer DEFAULT 0 NOT NULL,
    patch_count integer DEFAULT 0 NOT NULL,
    oldest_pending_date integer,
    locked_by text,
    locked_until timestamp with time zone,
    gmt_heartbeat timestamp with time zone,
    attempt integer DEFAULT 0 NOT NULL,
    gmt_next_run timestamp with time zone,
    last_error text,
    gmt_create timestamp with time zone DEFAULT now() NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE cdeu.finv_quote_cdeu_ingest_stats (
    stat_date date NOT NULL,
    total_rows bigint NOT NULL,
    total_bytes bigint DEFAULT 0 NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE cdeu.finv_quote_cdeu_job_run_log_export_file (
    run_id bigint NOT NULL,
    job_kind text NOT NULL,
    job_id bigint,
    usc text,
    biz_month integer,
    gmt_start timestamp with time zone DEFAULT now() NOT NULL,
    gmt_finish timestamp with time zone,
    status text,
    processed_rows bigint,
    processed_bytes bigint,
    error text,
    stage text,
    progress_percent double precision,
    progress_detail text,
    gmt_heartbeat timestamp with time zone
);

CREATE TABLE cdeu.finv_quote_cdeu_job_run_log_line (
    line_id bigint NOT NULL,
    run_id bigint NOT NULL,
    level text DEFAULT 'INFO'::text NOT NULL,
    stage text,
    message text NOT NULL,
    progress_percent double precision,
    gmt_create timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE cdeu.finv_quote_cdeu_quote_maturity_mon (
    usc text NOT NULL,
    trade_date integer NOT NULL,
    biz_month integer NOT NULL,
    expected_rows integer NOT NULL,
    actual_rows integer DEFAULT 0 NOT NULL,
    maturity_status text NOT NULL,
    gmt_first_seen timestamp with time zone,
    gmt_last_arrival timestamp with time zone,
    gmt_seale timestamp with time zone,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE cdeu.finv_quote_cdeu_scan_watermark (
    watermark_name text NOT NULL,
    last_value timestamp with time zone NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE cdeu.finv_quote_cdeu_trade_calendar (
    usc text NOT NULL,
    trade_date integer NOT NULL,
    flag_trade_day boolean NOT NULL
);

CREATE TABLE cdeu.finv_quote_cdeu_usc (
    usc text NOT NULL,
    category text,
    day_expect_rows integer,
    date_list date,
    date_delist date,
    flag_auto_discover boolean DEFAULT true NOT NULL,
    region text,
    market text,
    provider text,
    name text,
    timezone text,
    currency text,
    gmt_create timestamp with time zone DEFAULT now() NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL,
    priority_rank integer,
    sla_days integer,
    max_wait_days integer,
    priority_gmt_update timestamp with time zone,
    CONSTRAINT finv_quote_cdeu_usc_priority_group_check CHECK ((((priority_rank IS NULL) AND (sla_days IS NULL) AND (max_wait_days IS NULL)) OR ((priority_rank IS NOT NULL) AND (sla_days IS NOT NULL) AND (max_wait_days IS NOT NULL))))
);

CREATE TABLE cdeu.finv_quote_secu_kline_min (
    usc text CONSTRAINT finv_quote_secu_kline_min_secu_code_not_null NOT NULL,
    ts bigint NOT NULL,
    date integer,
    "time" integer,
    open numeric(20,6),
    high numeric(20,6),
    low numeric(20,6),
    close numeric(20,6),
    paocd numeric(20,6),
    volume bigint,
    turnover numeric(30,8),
    change_price numeric(20,6),
    change_ratio numeric(20,6),
    ext_field text,
    last_close numeric(20,6),
    remark text,
    provider character varying(255),
    gmt_create timestamp with time zone DEFAULT now() NOT NULL,
    gmt_update timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT finv_quote_secu_kline_min_date_check CHECK (((date IS NULL) OR ((date >= 19500101) AND (date <= 21001231)))),
    CONSTRAINT finv_quote_secu_kline_min_time_check CHECK ((("time" IS NULL) OR (("time" >= 0) AND ("time" <= 235959)))),
    CONSTRAINT finv_quote_secu_kline_min_ts_check CHECK ((ts >= 0)),
    CONSTRAINT finv_quote_secu_kline_min_turnover_check CHECK (((turnover IS NULL) OR (turnover >= (0)::numeric))),
    CONSTRAINT finv_quote_secu_kline_min_volume_check CHECK (((volume IS NULL) OR (volume >= 0)))
);

CREATE SEQUENCE cdeu.cdeu_dynamic_job_dynamic_job_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.cdeu_job_job_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.cdeu_job_step_dynamic_dynamic_step_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.cdeu_job_step_run_step_run_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.cdeu_job_step_step_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.cdeu_phase_phase_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.cdeu_phase_run_phase_run_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.cdeu_process_process_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.cdeu_process_run_run_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.cdeu_step_step_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.finv_quote_cdeu_export_batch_batch_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.finv_quote_cdeu_export_file_file_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.finv_quote_cdeu_export_file_patch_patch_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.finv_quote_cdeu_export_file_upload_log_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.finv_quote_cdeu_export_job_job_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.finv_quote_cdeu_job_run_log_export_file_run_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE cdeu.finv_quote_cdeu_job_run_log_line_line_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE cdeu.cdeu_dynamic_job_dynamic_job_id_seq OWNED BY cdeu.cdeu_job_dynamic.dynamic_job_id;

ALTER SEQUENCE cdeu.cdeu_job_job_id_seq OWNED BY cdeu.cdeu_job.job_id;

ALTER SEQUENCE cdeu.cdeu_job_step_dynamic_dynamic_step_id_seq OWNED BY cdeu.cdeu_job_step_dynamic.dynamic_step_id;

ALTER SEQUENCE cdeu.cdeu_job_step_run_step_run_id_seq OWNED BY cdeu.cdeu_job_step_run.step_run_id;

ALTER SEQUENCE cdeu.cdeu_job_step_step_id_seq OWNED BY cdeu.cdeu_job_step.step_id;

ALTER SEQUENCE cdeu.cdeu_phase_phase_id_seq OWNED BY cdeu.cdeu_phase.phase_id;

ALTER SEQUENCE cdeu.cdeu_phase_run_phase_run_id_seq OWNED BY cdeu.cdeu_phase_run.phase_run_id;

ALTER SEQUENCE cdeu.cdeu_process_process_id_seq OWNED BY cdeu.cdeu_workflow.workflow_id;

ALTER SEQUENCE cdeu.cdeu_process_run_run_id_seq OWNED BY cdeu.cdeu_workflow_run.run_id;

ALTER SEQUENCE cdeu.cdeu_step_step_id_seq OWNED BY cdeu.cdeu_process.process_id;

ALTER SEQUENCE cdeu.finv_quote_cdeu_export_batch_batch_id_seq OWNED BY cdeu.finv_quote_cdeu_export_batch.batch_id;

ALTER SEQUENCE cdeu.finv_quote_cdeu_export_file_file_id_seq OWNED BY cdeu.finv_quote_cdeu_export_file.file_id;

ALTER SEQUENCE cdeu.finv_quote_cdeu_export_file_patch_patch_id_seq OWNED BY cdeu.finv_quote_cdeu_export_file_patch.patch_id;

ALTER SEQUENCE cdeu.finv_quote_cdeu_export_file_upload_log_log_id_seq OWNED BY cdeu.finv_quote_cdeu_export_file_upload_log.log_id;

ALTER SEQUENCE cdeu.finv_quote_cdeu_export_job_job_id_seq OWNED BY cdeu.finv_quote_cdeu_export_job.job_id;

ALTER SEQUENCE cdeu.finv_quote_cdeu_job_run_log_export_file_run_id_seq OWNED BY cdeu.finv_quote_cdeu_job_run_log_export_file.run_id;

ALTER SEQUENCE cdeu.finv_quote_cdeu_job_run_log_line_line_id_seq OWNED BY cdeu.finv_quote_cdeu_job_run_log_line.line_id;

ALTER TABLE ONLY cdeu.cdeu_job ALTER COLUMN job_id SET DEFAULT nextval('cdeu.cdeu_job_job_id_seq'::regclass);

ALTER TABLE ONLY cdeu.cdeu_job_dynamic ALTER COLUMN dynamic_job_id SET DEFAULT nextval('cdeu.cdeu_dynamic_job_dynamic_job_id_seq'::regclass);

ALTER TABLE ONLY cdeu.cdeu_job_step ALTER COLUMN step_id SET DEFAULT nextval('cdeu.cdeu_job_step_step_id_seq'::regclass);

ALTER TABLE ONLY cdeu.cdeu_job_step_dynamic ALTER COLUMN dynamic_step_id SET DEFAULT nextval('cdeu.cdeu_job_step_dynamic_dynamic_step_id_seq'::regclass);

ALTER TABLE ONLY cdeu.cdeu_job_step_run ALTER COLUMN step_run_id SET DEFAULT nextval('cdeu.cdeu_job_step_run_step_run_id_seq'::regclass);

ALTER TABLE ONLY cdeu.cdeu_phase ALTER COLUMN phase_id SET DEFAULT nextval('cdeu.cdeu_phase_phase_id_seq'::regclass);

ALTER TABLE ONLY cdeu.cdeu_phase_run ALTER COLUMN phase_run_id SET DEFAULT nextval('cdeu.cdeu_phase_run_phase_run_id_seq'::regclass);

ALTER TABLE ONLY cdeu.cdeu_process ALTER COLUMN process_id SET DEFAULT nextval('cdeu.cdeu_step_step_id_seq'::regclass);

ALTER TABLE ONLY cdeu.cdeu_workflow ALTER COLUMN workflow_id SET DEFAULT nextval('cdeu.cdeu_process_process_id_seq'::regclass);

ALTER TABLE ONLY cdeu.cdeu_workflow_run ALTER COLUMN run_id SET DEFAULT nextval('cdeu.cdeu_process_run_run_id_seq'::regclass);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_export_batch ALTER COLUMN batch_id SET DEFAULT nextval('cdeu.finv_quote_cdeu_export_batch_batch_id_seq'::regclass);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_export_file ALTER COLUMN file_id SET DEFAULT nextval('cdeu.finv_quote_cdeu_export_file_file_id_seq'::regclass);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_export_file_patch ALTER COLUMN patch_id SET DEFAULT nextval('cdeu.finv_quote_cdeu_export_file_patch_patch_id_seq'::regclass);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_export_file_upload_log ALTER COLUMN log_id SET DEFAULT nextval('cdeu.finv_quote_cdeu_export_file_upload_log_log_id_seq'::regclass);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_export_job ALTER COLUMN job_id SET DEFAULT nextval('cdeu.finv_quote_cdeu_export_job_job_id_seq'::regclass);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_job_run_log_export_file ALTER COLUMN run_id SET DEFAULT nextval('cdeu.finv_quote_cdeu_job_run_log_export_file_run_id_seq'::regclass);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_job_run_log_line ALTER COLUMN line_id SET DEFAULT nextval('cdeu.finv_quote_cdeu_job_run_log_line_line_id_seq'::regclass);

ALTER TABLE ONLY cdeu.cdeu_job_dynamic
    ADD CONSTRAINT cdeu_dynamic_job_pkey PRIMARY KEY (dynamic_job_id);

ALTER TABLE ONLY cdeu.cdeu_job
    ADD CONSTRAINT cdeu_job_pkey PRIMARY KEY (job_id);

ALTER TABLE ONLY cdeu.cdeu_job_run
    ADD CONSTRAINT cdeu_job_run_pkey PRIMARY KEY (workflow_run_id, process_id, job_id);

ALTER TABLE ONLY cdeu.cdeu_job_step_dynamic
    ADD CONSTRAINT cdeu_job_step_dynamic_dynamic_job_id_seq_no_key UNIQUE (dynamic_job_id, seq_no);

ALTER TABLE ONLY cdeu.cdeu_job_step_dynamic
    ADD CONSTRAINT cdeu_job_step_dynamic_dynamic_job_id_step_code_key UNIQUE (dynamic_job_id, step_code);

ALTER TABLE ONLY cdeu.cdeu_job_step_dynamic
    ADD CONSTRAINT cdeu_job_step_dynamic_pkey PRIMARY KEY (dynamic_step_id);

ALTER TABLE ONLY cdeu.cdeu_job
    ADD CONSTRAINT cdeu_job_step_id_job_code_key UNIQUE (process_id, job_code);

ALTER TABLE ONLY cdeu.cdeu_job
    ADD CONSTRAINT cdeu_job_step_id_seq_no_key UNIQUE (process_id, seq_no);

ALTER TABLE ONLY cdeu.cdeu_job_step
    ADD CONSTRAINT cdeu_job_step_job_id_seq_no_key UNIQUE (job_id, seq_no);

ALTER TABLE ONLY cdeu.cdeu_job_step
    ADD CONSTRAINT cdeu_job_step_job_id_step_code_key UNIQUE (job_id, step_code);

ALTER TABLE ONLY cdeu.cdeu_job_step
    ADD CONSTRAINT cdeu_job_step_pkey PRIMARY KEY (step_id);

ALTER TABLE ONLY cdeu.cdeu_job_step_run
    ADD CONSTRAINT cdeu_job_step_run_key_job_step UNIQUE (workflow_run_id, process_id, job_id, step_code);

ALTER TABLE ONLY cdeu.cdeu_job_step_run
    ADD CONSTRAINT cdeu_job_step_run_pkey PRIMARY KEY (step_run_id);

ALTER TABLE ONLY cdeu.cdeu_phase
    ADD CONSTRAINT cdeu_phase_pkey PRIMARY KEY (phase_id);

ALTER TABLE ONLY cdeu.cdeu_phase
    ADD CONSTRAINT cdeu_phase_process_id_phase_code_key UNIQUE (workflow_id, phase_code);

ALTER TABLE ONLY cdeu.cdeu_phase
    ADD CONSTRAINT cdeu_phase_process_id_seq_no_key UNIQUE (workflow_id, seq_no);

ALTER TABLE ONLY cdeu.cdeu_phase_run
    ADD CONSTRAINT cdeu_phase_run_pkey PRIMARY KEY (phase_run_id);

ALTER TABLE ONLY cdeu.cdeu_phase_run
    ADD CONSTRAINT cdeu_phase_run_run_id_phase_code_key UNIQUE (workflow_run_id, phase_code);

ALTER TABLE ONLY cdeu.cdeu_workflow
    ADD CONSTRAINT cdeu_process_pkey PRIMARY KEY (workflow_id);

ALTER TABLE ONLY cdeu.cdeu_workflow
    ADD CONSTRAINT cdeu_process_process_code_version_key UNIQUE (workflow_code, version);

ALTER TABLE ONLY cdeu.cdeu_process_run
    ADD CONSTRAINT cdeu_process_run_pkey PRIMARY KEY (workflow_run_id, process_id);

ALTER TABLE ONLY cdeu.cdeu_workflow_run
    ADD CONSTRAINT cdeu_process_run_run_key_key UNIQUE (run_key);

ALTER TABLE ONLY cdeu.cdeu_process
    ADD CONSTRAINT cdeu_step_phase_id_seq_no_key UNIQUE (phase_id, seq_no);

ALTER TABLE ONLY cdeu.cdeu_process
    ADD CONSTRAINT cdeu_step_phase_id_step_code_key UNIQUE (phase_id, process_code);

ALTER TABLE ONLY cdeu.cdeu_process
    ADD CONSTRAINT cdeu_step_pkey PRIMARY KEY (process_id);

ALTER TABLE ONLY cdeu.cdeu_process_run
    ADD CONSTRAINT cdeu_step_run_phase_run_id_step_code_key UNIQUE (phase_run_id, process_code);

ALTER TABLE ONLY cdeu.cdeu_workflow_run
    ADD CONSTRAINT cdeu_workflow_run_pkey PRIMARY KEY (run_id);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_daily_budget
    ADD CONSTRAINT finv_quote_cdeu_daily_budget_pkey PRIMARY KEY (biz_date);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_export_batch
    ADD CONSTRAINT uq_finv_quote_cdeu_export_batch UNIQUE (batch_no, usc, biz_month, export_type);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_export_batch
    ADD CONSTRAINT finv_quote_cdeu_export_batch_pkey PRIMARY KEY (batch_id);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_export_file_patch
    ADD CONSTRAINT finv_quote_cdeu_export_file_patch_pkey PRIMARY KEY (patch_id);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_export_file
    ADD CONSTRAINT finv_quote_cdeu_export_file_pkey PRIMARY KEY (file_id);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_export_file_upload_log
    ADD CONSTRAINT finv_quote_cdeu_export_file_upload_log_pkey PRIMARY KEY (log_id);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_export_file
    ADD CONSTRAINT finv_quote_cdeu_export_file_usc_biz_month_file_type_version_key UNIQUE (usc, biz_month, file_type, version);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_export_job
    ADD CONSTRAINT finv_quote_cdeu_export_job_pkey PRIMARY KEY (job_id);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_export_job
    ADD CONSTRAINT finv_quote_cdeu_export_job_usc_biz_month_key UNIQUE (usc, biz_month);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_ingest_stats
    ADD CONSTRAINT finv_quote_cdeu_ingest_stats_pkey PRIMARY KEY (stat_date);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_job_run_log_export_file
    ADD CONSTRAINT finv_quote_cdeu_job_run_log_export_file_pkey PRIMARY KEY (run_id);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_job_run_log_line
    ADD CONSTRAINT finv_quote_cdeu_job_run_log_line_pkey PRIMARY KEY (line_id);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_quote_maturity_mon
    ADD CONSTRAINT finv_quote_cdeu_quote_maturity_mon_pkey PRIMARY KEY (usc, trade_date);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_scan_watermark
    ADD CONSTRAINT finv_quote_cdeu_scan_watermark_pkey PRIMARY KEY (watermark_name);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_trade_calendar
    ADD CONSTRAINT finv_quote_cdeu_trade_calendar_pkey PRIMARY KEY (usc, trade_date);

ALTER TABLE ONLY cdeu.finv_quote_cdeu_usc
    ADD CONSTRAINT finv_quote_cdeu_usc_pkey PRIMARY KEY (usc);

ALTER TABLE ONLY cdeu.finv_quote_secu_kline_min
    ADD CONSTRAINT finv_quote_secu_kline_min_pkey PRIMARY KEY (ts, usc);

CREATE INDEX idx_cdeu_job_dynamic_file ON cdeu.cdeu_job_dynamic USING btree (file_id, job_category);

CREATE INDEX idx_cdeu_job_dynamic_process ON cdeu.cdeu_job_dynamic USING btree (process_id, seq_no);

CREATE INDEX idx_cdeu_job_dynamic_run_log ON cdeu.cdeu_job_dynamic USING btree (run_log_id);

CREATE INDEX idx_cdeu_job_dynamic_status ON cdeu.cdeu_job_dynamic USING btree (status, gmt_update);

CREATE INDEX idx_cdeu_job_dynamic_workflow_run ON cdeu.cdeu_job_dynamic USING btree (workflow_run_id, job_category);

CREATE INDEX idx_cdeu_job_run_dynamic ON cdeu.cdeu_job_run USING btree (dynamic_job_id) WHERE (dynamic_job_id IS NOT NULL);

CREATE INDEX idx_cdeu_job_run_log ON cdeu.cdeu_job_run USING btree (run_log_id);

CREATE INDEX idx_cdeu_job_run_process_code ON cdeu.cdeu_job_run USING btree (workflow_run_id, process_id, job_code);

CREATE INDEX idx_cdeu_job_run_workflow ON cdeu.cdeu_job_run USING btree (workflow_run_id, job_code);

CREATE INDEX idx_cdeu_job_step_dynamic_job ON cdeu.cdeu_job_step_dynamic USING btree (dynamic_job_id, seq_no);

CREATE INDEX idx_cdeu_job_step_job ON cdeu.cdeu_job_step USING btree (job_id, seq_no);

CREATE INDEX idx_cdeu_job_step_run_seq ON cdeu.cdeu_job_step_run USING btree (workflow_run_id, process_id, job_id, seq_no);

CREATE INDEX idx_cdeu_process_run_workflow ON cdeu.cdeu_process_run USING btree (workflow_run_id, process_id);

CREATE INDEX idx_cdeuworkflowrun_biz ON cdeu.cdeu_workflow_run USING btree (biz_date, status);

CREATE INDEX idx_quotecdeuexportbatch_biz ON cdeu.finv_quote_cdeu_export_batch USING btree (biz_date, status);

CREATE INDEX idx_quotecdeuexportbatch_lookup ON cdeu.finv_quote_cdeu_export_batch USING btree (batch_no, export_type, usc, biz_month);

CREATE INDEX idx_quotecdeuexportfile_job ON cdeu.finv_quote_cdeu_export_file USING btree (job_id);

CREATE INDEX idx_quotecdeuexportfile_network ON cdeu.finv_quote_cdeu_export_file USING btree (network_status, cleanup_status);

CREATE INDEX idx_quotecdeuexportfile_upload ON cdeu.finv_quote_cdeu_export_file USING btree (upload_status, gmt_next_retry, gmt_create);

CREATE INDEX idx_quotecdeuexportfilepatch_file ON cdeu.finv_quote_cdeu_export_file_patch USING btree (patch_file_id);

CREATE INDEX idx_quotecdeuexportfilepatch_job ON cdeu.finv_quote_cdeu_export_file_patch USING btree (job_id);

CREATE INDEX idx_quotecdeuexportfilepatch_pending ON cdeu.finv_quote_cdeu_export_file_patch USING btree (status, gmt_create);

CREATE INDEX idx_quotecdeuexportfileuploadlog_file ON cdeu.finv_quote_cdeu_export_file_upload_log USING btree (file_id, attempt);

CREATE INDEX idx_quotecdeuexportjob_pick ON cdeu.finv_quote_cdeu_export_job USING btree (status, priority_rank, freeze_date, oldest_pending_date) WHERE (status = ANY (ARRAY['WAITING'::text, 'PAUSED'::text]));

CREATE INDEX idx_quotecdeujobrunlogexportfile_job ON cdeu.finv_quote_cdeu_job_run_log_export_file USING btree (job_id);

CREATE INDEX idx_quotecdeujobrunlogexportfile_kind_time ON cdeu.finv_quote_cdeu_job_run_log_export_file USING btree (job_kind, gmt_start DESC);

CREATE INDEX idx_quotecdeujobrunlogexportfile_running ON cdeu.finv_quote_cdeu_job_run_log_export_file USING btree (job_kind, gmt_start DESC) WHERE (gmt_finish IS NULL);

CREATE INDEX idx_quotecdeuquotematuritymon_month_status ON cdeu.finv_quote_cdeu_quote_maturity_mon USING btree (biz_month, maturity_status);

CREATE INDEX idx_quotecdeuquotematuritymon_status_usc ON cdeu.finv_quote_cdeu_quote_maturity_mon USING btree (maturity_status, usc);

CREATE INDEX idx_finv_quote_cdeu_job_run_log_line_run ON cdeu.finv_quote_cdeu_job_run_log_line USING btree (run_id, line_id);

CREATE INDEX idx_finv_quote_date_usc_ts ON cdeu.finv_quote_secu_kline_min USING btree (date, usc, ts);

CREATE INDEX idx_finv_quote_usc_date_ts ON cdeu.finv_quote_secu_kline_min USING btree (usc, date, ts);

CREATE INDEX idx_finv_quote_usc_ts ON cdeu.finv_quote_secu_kline_min USING btree (usc, ts);

CREATE UNIQUE INDEX uq_cdeu_job_dynamic_export_run ON cdeu.cdeu_job_dynamic USING btree (workflow_run_id, usc, biz_month) NULLS NOT DISTINCT WHERE (job_category = 'EXPORT'::text);

CREATE UNIQUE INDEX uq_cdeu_job_dynamic_upload_run ON cdeu.cdeu_job_dynamic USING btree (workflow_run_id, file_id) NULLS NOT DISTINCT WHERE (job_category = 'UPLOAD'::text);

CREATE UNIQUE INDEX uq_cdeuworkflowrun_running ON cdeu.cdeu_workflow_run USING btree (workflow_id) WHERE (status = ANY (ARRAY['PENDING'::text, 'READY'::text, 'RUNNING'::text]));

CREATE TRIGGER trg_finv_quote_secu_kline_min_gmt_update BEFORE UPDATE ON cdeu.finv_quote_secu_kline_min FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();

COMMENT ON TABLE cdeu.cdeu_job_dynamic IS '按批次动态生成的单证券导出 JOB；(usc, biz_month) 全局唯一';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.dynamic_job_id IS '动态 JOB 主键';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.process_id IS '所属 Process，逻辑关联 cdeu_process.process_id';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.job_code IS '动态 JOB 编码，如 JOB_EXPORT_BASE_GCMAIN_202501';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.name IS '动态 JOB 名称，包含证券/年月/类型';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.usc IS '证券唯一编码';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.biz_month IS '导出业务年月 yyyyMM';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.export_type IS '导出类型：BASE / INCR';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.export_job_id IS '关联 finv_quote_cdeu_export_job.job_id';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.seq_no IS '同 Process 内展示顺序';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.status IS '动态 JOB 最近一次运行状态';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.error IS '最近一次失败原因';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.gmt_start IS '最近一次开始时间';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.gmt_finish IS '最近一次结束时间';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.gmt_update IS '更新时间';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.run_log_id IS '最近一次执行对应的 Job 运行日志 ID，逻辑关联 finv_quote_cdeu_job_run_log_export_file.run_id，无外键';

COMMENT ON COLUMN cdeu.cdeu_job_dynamic.workflow_run_id IS '生成本动态 JOB 的 CDEU 工作流运行 ID，逻辑关联 cdeu_workflow_run.run_id，无外键';

COMMENT ON TABLE cdeu.cdeu_job IS 'CDEU 作业定义：步骤下的可执行作业，按 seq_no 顺序执行';

COMMENT ON COLUMN cdeu.cdeu_job.job_id IS '作业主键';

COMMENT ON COLUMN cdeu.cdeu_job.process_id IS '所属步骤 ID，逻辑关联 cdeu_step.step_id';

COMMENT ON COLUMN cdeu.cdeu_job.job_code IS '作业编码，如 JOB_EXPORT_LOAD';

COMMENT ON COLUMN cdeu.cdeu_job.name IS '作业中文名称';

COMMENT ON COLUMN cdeu.cdeu_job.job_kind IS '实际调度标识，如 RUN_DUE_EXPORT_JOBS';

COMMENT ON COLUMN cdeu.cdeu_job.execution_stage IS '内嵌执行阶段：LOAD / EXPORT / REGISTER / CLEANUP；不拆分则为空';

COMMENT ON COLUMN cdeu.cdeu_job.seq_no IS '作业执行顺序，数值小的先执行';

COMMENT ON COLUMN cdeu.cdeu_job.critical IS '是否关键作业，关键作业失败或未完成会使步骤失败';

COMMENT ON COLUMN cdeu.cdeu_job.affects_completion IS '是否参与步骤完成判定';

COMMENT ON COLUMN cdeu.cdeu_job.trigger_mode IS '触发方式：SCHEDULED 定时 / EMBEDDED 父作业内嵌 / MANUAL 手动';

COMMENT ON COLUMN cdeu.cdeu_job.execution_mode IS '执行方式：DIRECT 直接调用 / EMBEDDED 由父作业阶段驱动';

COMMENT ON COLUMN cdeu.cdeu_job.manual_rerun_enabled IS '是否允许单阶段手动重跑（5–8 导出阶段默认开启）';

COMMENT ON COLUMN cdeu.cdeu_job.schedule_kind IS '调度类型：DAILY 每日 / INTERVAL 固定间隔 / NONE 无';

COMMENT ON COLUMN cdeu.cdeu_job.schedule_value IS '调度值：DAILY 为 HH:MM，INTERVAL 为秒数';

COMMENT ON COLUMN cdeu.cdeu_job.timeout_seconds IS '单次执行超时秒数，空表示不限制';

COMMENT ON COLUMN cdeu.cdeu_job.max_attempts IS '最大尝试次数';

COMMENT ON COLUMN cdeu.cdeu_job.retry_backoff_seconds IS '重试退避基数（秒）';

COMMENT ON COLUMN cdeu.cdeu_job.job_params IS '作业参数 JSON';

COMMENT ON COLUMN cdeu.cdeu_job.enabled IS '作业定义是否启用';

COMMENT ON COLUMN cdeu.cdeu_job.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.cdeu_job.gmt_update IS '更新时间';

COMMENT ON TABLE cdeu.cdeu_job_run IS 'CDEU 作业运行实例（批处理项）';

COMMENT ON COLUMN cdeu.cdeu_job_run.job_id IS '作业定义 ID';

COMMENT ON COLUMN cdeu.cdeu_job_run.job_code IS '作业编码快照';

COMMENT ON COLUMN cdeu.cdeu_job_run.job_kind IS '实际调度标识快照';

COMMENT ON COLUMN cdeu.cdeu_job_run.execution_stage IS '执行阶段快照';

COMMENT ON COLUMN cdeu.cdeu_job_run.batch_key IS '已废弃：批次维度改由 dynamic_job_id 承载；该列历史值为空，不再写入';

COMMENT ON COLUMN cdeu.cdeu_job_run.trigger_type IS '触发类型：PROCESS 流程内 / MANUAL_STAGE 单阶段手动重跑';

COMMENT ON COLUMN cdeu.cdeu_job_run.status IS '作业运行状态';

COMMENT ON COLUMN cdeu.cdeu_job_run.attempt IS '尝试次数';

COMMENT ON COLUMN cdeu.cdeu_job_run.started_at IS '开始时间';

COMMENT ON COLUMN cdeu.cdeu_job_run.finished_at IS '结束时间';

COMMENT ON COLUMN cdeu.cdeu_job_run.processed_rows IS '处理行数';

COMMENT ON COLUMN cdeu.cdeu_job_run.processed_bytes IS '处理字节数';

COMMENT ON COLUMN cdeu.cdeu_job_run.run_log_id IS '逻辑关联 cdeu.finv_quote_cdeu_job_run_log_export_file.run_id';

COMMENT ON COLUMN cdeu.cdeu_job_run.error IS '失败原因';

COMMENT ON COLUMN cdeu.cdeu_job_run.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.cdeu_job_run.gmt_update IS '更新时间';

COMMENT ON COLUMN cdeu.cdeu_job_run.workflow_run_id IS '工作流运行 ID（主键组成列）';

COMMENT ON COLUMN cdeu.cdeu_job_run.process_id IS 'Process 定义 ID（主键组成列，替代已删除的 process_run_id）';

COMMENT ON COLUMN cdeu.cdeu_job_run.dynamic_job_id IS '动态工作项 ID，逻辑关联 cdeu.cdeu_job_dynamic.dynamic_job_id；非批次 JOB 为 NULL';

COMMENT ON TABLE cdeu.cdeu_job_step IS 'CDEU JOB 下的串行原子步骤定义';

COMMENT ON COLUMN cdeu.cdeu_job_step.step_id IS '步骤主键';

COMMENT ON COLUMN cdeu.cdeu_job_step.job_id IS '所属 JOB ID，逻辑关联 cdeu_job.job_id';

COMMENT ON COLUMN cdeu.cdeu_job_step.step_code IS '步骤编码，如 STEP_LOAD';

COMMENT ON COLUMN cdeu.cdeu_job_step.name IS '步骤中文名称';

COMMENT ON COLUMN cdeu.cdeu_job_step.seq_no IS 'JOB 内执行顺序，数值小的先执行';

COMMENT ON COLUMN cdeu.cdeu_job_step.critical IS '是否关键步骤';

COMMENT ON COLUMN cdeu.cdeu_job_step.affects_completion IS '是否参与 JOB 完成判定';

COMMENT ON COLUMN cdeu.cdeu_job_step.timeout_seconds IS '单步超时秒数，空表示沿用 JOB';

COMMENT ON COLUMN cdeu.cdeu_job_step.enabled IS '步骤定义是否启用';

COMMENT ON COLUMN cdeu.cdeu_job_step.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.cdeu_job_step.gmt_update IS '更新时间';

COMMENT ON TABLE cdeu.cdeu_job_step_dynamic IS '动态 JOB（cdeu_job_dynamic）的串行 Step 运行记录；V100047 由 cdeu_job_dynamic_step 改名，命名与 cdeu_job_step / cdeu_job_step_run 同族。';

COMMENT ON COLUMN cdeu.cdeu_job_step_dynamic.dynamic_step_id IS '动态 Step 主键';

COMMENT ON COLUMN cdeu.cdeu_job_step_dynamic.dynamic_job_id IS '所属动态 JOB ID';

COMMENT ON COLUMN cdeu.cdeu_job_step_dynamic.step_code IS 'Step 编码：STEP_LOAD / STEP_EXPORT / STEP_REGISTER / STEP_CLEANUP';

COMMENT ON COLUMN cdeu.cdeu_job_step_dynamic.name IS 'Step 中文名称';

COMMENT ON COLUMN cdeu.cdeu_job_step_dynamic.seq_no IS 'JOB 内串行顺序';

COMMENT ON COLUMN cdeu.cdeu_job_step_dynamic.status IS 'Step 运行状态';

COMMENT ON COLUMN cdeu.cdeu_job_step_dynamic.started_at IS '开始时间';

COMMENT ON COLUMN cdeu.cdeu_job_step_dynamic.finished_at IS '结束时间';

COMMENT ON COLUMN cdeu.cdeu_job_step_dynamic.error IS '失败原因';

COMMENT ON COLUMN cdeu.cdeu_job_step_dynamic.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.cdeu_job_step_dynamic.gmt_update IS '更新时间';

COMMENT ON TABLE cdeu.cdeu_job_step_run IS 'CDEU JOB 内 Step 运行实例';

COMMENT ON COLUMN cdeu.cdeu_job_step_run.step_run_id IS 'JOB Step 运行主键';

COMMENT ON COLUMN cdeu.cdeu_job_step_run.step_id IS 'JOB Step 定义 ID';

COMMENT ON COLUMN cdeu.cdeu_job_step_run.step_code IS '步骤编码快照';

COMMENT ON COLUMN cdeu.cdeu_job_step_run.seq_no IS 'JOB 内顺序快照';

COMMENT ON COLUMN cdeu.cdeu_job_step_run.status IS '步骤运行状态';

COMMENT ON COLUMN cdeu.cdeu_job_step_run.attempt IS '尝试次数';

COMMENT ON COLUMN cdeu.cdeu_job_step_run.started_at IS '开始时间';

COMMENT ON COLUMN cdeu.cdeu_job_step_run.finished_at IS '结束时间';

COMMENT ON COLUMN cdeu.cdeu_job_step_run.processed_rows IS '处理行数';

COMMENT ON COLUMN cdeu.cdeu_job_step_run.processed_bytes IS '处理字节数';

COMMENT ON COLUMN cdeu.cdeu_job_step_run.error IS '失败原因';

COMMENT ON COLUMN cdeu.cdeu_job_step_run.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.cdeu_job_step_run.gmt_update IS '更新时间';

COMMENT ON TABLE cdeu.cdeu_phase IS 'CDEU 阶段定义：流程下的大阶段';

COMMENT ON COLUMN cdeu.cdeu_phase.phase_id IS '阶段主键';

COMMENT ON COLUMN cdeu.cdeu_phase.workflow_id IS '所属流程 ID，逻辑关联 cdeu_process.process_id';

COMMENT ON COLUMN cdeu.cdeu_phase.phase_code IS '阶段编码，如 PHASE_EXPORT';

COMMENT ON COLUMN cdeu.cdeu_phase.name IS '阶段中文名称';

COMMENT ON COLUMN cdeu.cdeu_phase.seq_no IS '阶段执行顺序，数值小的先执行';

COMMENT ON COLUMN cdeu.cdeu_phase.critical IS '是否关键阶段，关键阶段失败或未完成会使流程失败';

COMMENT ON COLUMN cdeu.cdeu_phase.affects_completion IS '是否参与流程完成判定';

COMMENT ON COLUMN cdeu.cdeu_phase.run_mode IS '运行模式：INHERIT 随流程 / CONTINUOUS 独立循环 / MANUAL 手动';

COMMENT ON COLUMN cdeu.cdeu_phase.batch_enabled IS '该阶段是否包含批处理步骤';

COMMENT ON COLUMN cdeu.cdeu_phase.completion_rule IS '阶段完成条件 JSON，默认全部步骤完成';

COMMENT ON COLUMN cdeu.cdeu_phase.enabled IS '阶段定义是否启用';

COMMENT ON COLUMN cdeu.cdeu_phase.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.cdeu_phase.gmt_update IS '更新时间';

COMMENT ON TABLE cdeu.cdeu_phase_run IS 'CDEU 阶段运行实例';

COMMENT ON COLUMN cdeu.cdeu_phase_run.phase_run_id IS '阶段运行主键';

COMMENT ON COLUMN cdeu.cdeu_phase_run.workflow_run_id IS '流程运行 ID，逻辑关联 cdeu_process_run.run_id';

COMMENT ON COLUMN cdeu.cdeu_phase_run.phase_id IS '阶段定义 ID';

COMMENT ON COLUMN cdeu.cdeu_phase_run.phase_code IS '阶段编码快照';

COMMENT ON COLUMN cdeu.cdeu_phase_run.seq_no IS '阶段顺序快照';

COMMENT ON COLUMN cdeu.cdeu_phase_run.status IS '阶段运行状态';

COMMENT ON COLUMN cdeu.cdeu_phase_run.started_at IS '开始时间';

COMMENT ON COLUMN cdeu.cdeu_phase_run.finished_at IS '结束时间';

COMMENT ON COLUMN cdeu.cdeu_phase_run.success_count IS '成功步骤数';

COMMENT ON COLUMN cdeu.cdeu_phase_run.failed_count IS '失败步骤数';

COMMENT ON COLUMN cdeu.cdeu_phase_run.total_count IS '参与判定步骤总数';

COMMENT ON COLUMN cdeu.cdeu_phase_run.completion_rule_snapshot IS '阶段完成条件快照';

COMMENT ON COLUMN cdeu.cdeu_phase_run.completion_snapshot IS '阶段完成判定快照';

COMMENT ON COLUMN cdeu.cdeu_phase_run.error IS '失败原因';

COMMENT ON COLUMN cdeu.cdeu_phase_run.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.cdeu_phase_run.gmt_update IS '更新时间';

COMMENT ON TABLE cdeu.cdeu_process IS 'CDEU 步骤定义：阶段下的可编排步骤，支持批处理';

COMMENT ON COLUMN cdeu.cdeu_process.process_id IS '步骤主键';

COMMENT ON COLUMN cdeu.cdeu_process.phase_id IS '所属阶段 ID，逻辑关联 cdeu_phase.phase_id';

COMMENT ON COLUMN cdeu.cdeu_process.process_code IS '步骤编码，如 STEP_EXPORT_JOB';

COMMENT ON COLUMN cdeu.cdeu_process.name IS '步骤中文名称';

COMMENT ON COLUMN cdeu.cdeu_process.seq_no IS '步骤执行顺序，数值小的先执行';

COMMENT ON COLUMN cdeu.cdeu_process.critical IS '是否关键步骤，关键步骤失败或未完成会使阶段失败';

COMMENT ON COLUMN cdeu.cdeu_process.affects_completion IS '是否参与阶段完成判定';

COMMENT ON COLUMN cdeu.cdeu_process.run_mode IS '运行模式：INHERIT 随阶段 / CONTINUOUS 独立循环 / MANUAL 手动';

COMMENT ON COLUMN cdeu.cdeu_process.batch_enabled IS '是否按批次展开执行';

COMMENT ON COLUMN cdeu.cdeu_process.batch_source IS '批次来源：EXPORT_JOB_CANDIDATES / EXPORT_FILES / PATCH_CANDIDATES / USC_LIST / CUSTOM_SQL';

COMMENT ON COLUMN cdeu.cdeu_process.batch_key_field IS '批次键字段，如 usc,biz_month';

COMMENT ON COLUMN cdeu.cdeu_process.batch_size IS '单批处理条数';

COMMENT ON COLUMN cdeu.cdeu_process.max_parallel IS '最大并发批次数';

COMMENT ON COLUMN cdeu.cdeu_process.failure_policy IS '批处理失败策略：CONTINUE 继续其余批次（默认）/ FAIL_FAST 失败即停';

COMMENT ON COLUMN cdeu.cdeu_process.completion_rule IS '步骤完成条件 JSON，默认全部作业完成';

COMMENT ON COLUMN cdeu.cdeu_process.enabled IS '步骤定义是否启用';

COMMENT ON COLUMN cdeu.cdeu_process.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.cdeu_process.gmt_update IS '更新时间';

COMMENT ON TABLE cdeu.cdeu_workflow IS 'CDEU 流程定义：一条端到端流程（采集→去重→导出→上传）';

COMMENT ON COLUMN cdeu.cdeu_workflow.workflow_id IS '流程主键';

COMMENT ON COLUMN cdeu.cdeu_workflow.workflow_code IS '流程编码，如 CDEU_DAILY';

COMMENT ON COLUMN cdeu.cdeu_workflow.name IS '流程中文名称';

COMMENT ON COLUMN cdeu.cdeu_workflow.description IS '流程说明';

COMMENT ON COLUMN cdeu.cdeu_workflow.version IS '定义版本号，同编码可多版本';

COMMENT ON COLUMN cdeu.cdeu_workflow.run_mode IS '运行模式：AUTO_DAILY 每日定时 / MANUAL 手动 / BOTH 两者';

COMMENT ON COLUMN cdeu.cdeu_workflow.schedule_time IS '每日自动触发时间（按 schedule_timezone 解释）';

COMMENT ON COLUMN cdeu.cdeu_workflow.schedule_timezone IS '调度时区，默认取 FINV_QUOTE_DEDUB_TIMEZONE';

COMMENT ON COLUMN cdeu.cdeu_workflow.schedule_enabled IS '是否启用每日自动触发';

COMMENT ON COLUMN cdeu.cdeu_workflow.completion_rule IS '完成条件 JSON：ALL / CRITICAL_ALL / AT_LEAST / ANY_OF，默认 ALL';

COMMENT ON COLUMN cdeu.cdeu_workflow.enabled IS '流程定义是否启用';

COMMENT ON COLUMN cdeu.cdeu_workflow.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.cdeu_workflow.gmt_update IS '更新时间';

COMMENT ON TABLE cdeu.cdeu_process_run IS 'CDEU 步骤运行实例，记录批处理计数';

COMMENT ON COLUMN cdeu.cdeu_process_run.phase_run_id IS '阶段运行 ID，逻辑关联 cdeu_phase_run.phase_run_id';

COMMENT ON COLUMN cdeu.cdeu_process_run.process_id IS '步骤定义 ID';

COMMENT ON COLUMN cdeu.cdeu_process_run.process_code IS '步骤编码快照';

COMMENT ON COLUMN cdeu.cdeu_process_run.seq_no IS '步骤顺序快照';

COMMENT ON COLUMN cdeu.cdeu_process_run.status IS '步骤运行状态';

COMMENT ON COLUMN cdeu.cdeu_process_run.started_at IS '开始时间';

COMMENT ON COLUMN cdeu.cdeu_process_run.finished_at IS '结束时间';

COMMENT ON COLUMN cdeu.cdeu_process_run.batch_total IS '批次总数；非批处理步骤为 1';

COMMENT ON COLUMN cdeu.cdeu_process_run.batch_success IS '成功批次数';

COMMENT ON COLUMN cdeu.cdeu_process_run.batch_failed IS '失败批次数';

COMMENT ON COLUMN cdeu.cdeu_process_run.completion_rule_snapshot IS '步骤完成条件快照';

COMMENT ON COLUMN cdeu.cdeu_process_run.completion_snapshot IS '步骤完成判定快照';

COMMENT ON COLUMN cdeu.cdeu_process_run.error IS '失败原因';

COMMENT ON COLUMN cdeu.cdeu_process_run.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.cdeu_process_run.gmt_update IS '更新时间';

COMMENT ON COLUMN cdeu.cdeu_process_run.workflow_run_id IS '工作流运行 ID（冗余列，来源于 cdeu_phase_run.workflow_run_id，用于免 JOIN 按运行筛选）';

COMMENT ON TABLE cdeu.cdeu_workflow_run IS 'CDEU 流程运行实例';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.run_id IS '流程运行主键';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.workflow_id IS '流程定义 ID，逻辑关联 cdeu_process.process_id';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.workflow_code IS '流程编码快照';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.workflow_version IS '流程定义版本快照';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.run_key IS '幂等键：自动 process_code:biz_date:SCHEDULE；手动追加时间戳/ULID';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.biz_date IS '业务日期，按 FINV_QUOTE_DEDUB_TIMEZONE 计算';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.trigger_type IS '触发类型：SCHEDULE 定时 / MANUAL 手动 / CONTINUOUS 连续循环';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.status IS '运行状态';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.scheduled_at IS '计划触发时间';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.started_at IS '实际开始时间';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.finished_at IS '结束时间';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.current_phase_id IS '当前执行阶段 ID';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.completion_rule_snapshot IS '完成条件快照';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.completion_snapshot IS '完成判定结果快照';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.operator IS '手动触发人';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.error IS '失败原因';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.cdeu_workflow_run.gmt_update IS '更新时间';

COMMENT ON TABLE cdeu.finv_quote_cdeu_daily_budget IS '每日导出预算快照，含自适应因子与消耗量';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_daily_budget.biz_date IS '预算日期（业务时区自然日）';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_daily_budget.base_rows IS '近 30 天入口日均条数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_daily_budget.base_bytes IS '近 30 天入口日均字节数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_daily_budget.p75_rows IS '近 30 天入口条数 P75';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_daily_budget.backlog_ratio IS '积压量 / 日均条数，表示积压天数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_daily_budget.sla_pressure IS '临近 SLA 任务占比';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_daily_budget.factor IS '自适应预算因子，clamp(1.20 ~ 2.00)';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_daily_budget.budget_rows IS '当日预算条数上限';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_daily_budget.budget_bytes IS '当日预算字节上限';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_daily_budget.hard_cap_rows IS '硬上限，防止压采集';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_daily_budget.consumed_rows IS '当日已消耗条数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_daily_budget.consumed_bytes IS '当日已消耗字节数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_daily_budget.status IS '预算状态：CALCULATED / RUNNING / DONE';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_daily_budget.gmt_create IS '预算生成时间';

COMMENT ON TABLE cdeu.finv_quote_cdeu_export_batch IS '导出批次清单：当前批次需要导出的证券、业务年月与 BASE/INCR 类型，作为后续导出 Process 的输入';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_batch.batch_id IS '批次项主键';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_batch.batch_no IS '批次号，当前按业务日期生成 yyyyMMdd';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_batch.biz_date IS '批次业务日期';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_batch.usc IS '证券唯一编码';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_batch.biz_month IS '导出业务年月 yyyyMM';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_batch.export_type IS '导出类型：BASE 基础文件 / INCR 增量文件';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_batch.source_job_id IS 'INCR 条目关联的 finv_quote_cdeu_export_job.job_id；BASE 可为空';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_batch.status IS '条目状态：READY 待消费 / CONSUMED 已消费';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_batch.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_batch.gmt_update IS '更新时间';

COMMENT ON TABLE cdeu.finv_quote_cdeu_export_file IS '导出文件登记表，每个物理文件一条记录，是上传独享存储任务的唯一数据源；job_id 逻辑关联 finv_quote_cdeu_export_job.job_id，无外键';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.file_id IS '文件记录主键，被 finv_quote_cdeu_export_file_upload_log.file_id 与 finv_quote_cdeu_export_file_patch.patch_file_id 逻辑引用';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.job_id IS '关联的证券月导出任务 ID，逻辑关联 cdeu.finv_quote_cdeu_export_job.job_id';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.usc IS '证券唯一编码';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.biz_month IS '归档月份 yyyyMM';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.file_type IS '文件类型：BASE / PATCH / MANIFEST';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.version IS '文件版本号，base 与 patch 独立递增，MANIFEST 每次登记自增';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.file_name IS '本地文件名，BASE 为 region_market_usc_Min_provider_Mon_yyyyMM.mvsv，PATCH 为 ..._Incr_n.mvsv，MANIFEST 为 ..._Manifest.json';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.file_path IS '本地绝对路径';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.target_file_name IS '上传目标文件名，默认与 file_name 相同；当前不做扩展名改名';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.upload_target IS '独享存储目标路径，含目标文件名';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.format IS '文件格式，固定 mvsv';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.compression IS '压缩方式：none / gzip（扩展名保持 .mvsv）';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.row_count IS '数据行数，不含 16 行文件头和 1 行空行；MANIFEST 记录 0';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.file_size IS '文件字节数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.checksum IS '文件 SHA256（小写十六进制），上传前后校验';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.manifest_path IS '该文件对应的 Manifest 本地路径；MANIFEST 自身为空';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.seal_date IS '导出时的收口日期，用于 Patch 级别判定';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.export_status IS '导出状态：EXPORTED / SUPERSEDED / SKIPPED';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.upload_status IS '上传状态：PENDING_UPLOAD / UPLOADING / UPLOADED / UPLOAD_FAILED / SKIPPED';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.upload_attempt IS '上传尝试次数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.gmt_next_retry IS '下次可重试上传时间，指数退避';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.gmt_upload_start IS '最近一次上传开始时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.gmt_upload_finish IS '最近一次上传结束时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.remote_checksum_verified IS '远端 checksum 是否校验通过';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.network_status IS '网盘归档状态：NONE / ARCHIVED';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.cleanup_status IS '本地清理状态：NONE / LOCAL_DELETED';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file.gmt_update IS '更新时间';

COMMENT ON TABLE cdeu.finv_quote_cdeu_export_file_patch IS '收口后到达数据的补丁任务表；job_id 逻辑关联 finv_quote_cdeu_export_job，patch_file_id 逻辑关联 finv_quote_cdeu_export_file，无外键';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.patch_id IS '补丁任务主键';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.job_id IS '关联的证券月导出任务 ID，逻辑关联 cdeu.finv_quote_cdeu_export_job.job_id';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.usc IS '证券唯一编码';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.biz_month IS '归档月份 yyyyMM';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.patch_level IS '补丁级别：L1_HOT / L2_WARM / L3_COLD';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.patch_version IS '补丁序号 n，对应文件名 ..._Incr_{n}.mvsv 中的 n';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.source_rows IS '补丁来源数据行数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.patch_rows IS '实际写入补丁文件行数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.reason IS '补丁原因：LATE_ARRIVAL / UPSTREAM_DELAY / CORRECTION';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.arrival_window IS '触发补丁的数据到达时间窗口';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.status IS '补丁状态：PENDING / RUNNING / DONE / FAILED';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.patch_file_id IS '关联的补丁文件登记记录 ID，逻辑关联 cdeu.finv_quote_cdeu_export_file.file_id';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.gmt_create IS '补丁任务创建时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.gmt_finish IS '补丁任务完成时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_patch.last_error IS '最后一次错误信息';

COMMENT ON TABLE cdeu.finv_quote_cdeu_export_file_upload_log IS '上传尝试审计日志，每次上传一条；file_id 逻辑关联 finv_quote_cdeu_export_file.file_id，无外键';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_upload_log.log_id IS '日志主键';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_upload_log.file_id IS '关联的导出文件记录 ID，逻辑关联 cdeu.finv_quote_cdeu_export_file.file_id';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_upload_log.attempt IS '第几次上传尝试';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_upload_log.target_path IS '本次上传目标路径';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_upload_log.gmt_start IS '上传开始时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_upload_log.gmt_finish IS '上传结束时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_upload_log.status IS '上传结果：SUCCESS / FAILED';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_upload_log.uploaded_bytes IS '本次上传字节数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_file_upload_log.error IS '失败原因';

COMMENT ON TABLE cdeu.finv_quote_cdeu_export_job IS '证券月导出任务表，记录状态、游标、锁与版本；与 cdeu.finv_quote_cdeu_usc 由 usc 逻辑关联，无外键；同一 (usc, biz_month) 唯一保证幂等';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.job_id IS '任务主键，被 finv_quote_cdeu_export_file.job_id、finv_quote_cdeu_export_file_patch.job_id、finv_quote_cdeu_job_run_log_export_file.job_id 逻辑引用';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.usc IS '证券唯一编码，逻辑关联 cdeu.finv_quote_cdeu_usc.usc';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.biz_month IS '归档月份 yyyyMM';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.priority_rank IS '领取时的优先级快照，每日刷新';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.freeze_date IS '最晚完成日期 = seal_date + sla_days，到期强制收口并优先处理';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.seal_date IS '收口日期（次月 25 日），之后到达的数据走 Patch';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.status IS '状态：WAITING / LOADING / LOADED / EXPORTING / EXPORTED / PAUSED / FAILED / FROZEN';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.load_cursor_date IS '装载游标：已处理到的交易日期 yyyyMMdd';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.load_cursor_ts IS '装载游标：当日已处理到的最大 ts';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.loaded_rows IS '已装载到中转表的总行数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.base_version IS 'base 文件当前版本号，L1_HOT 重写时递增';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.patch_count IS '已应用 patch 数量，Patch 文件名中的 n 由它推导';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.oldest_pending_date IS '最老待处理日期，用于排序防旧数据滞留';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.locked_by IS '持有任务的 worker 标识';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.locked_until IS '任务锁租约到期时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.gmt_heartbeat IS 'worker 最近心跳时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.attempt IS '已尝试次数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.gmt_next_run IS '下次可运行时间，用于指数退避';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.last_error IS '最后一次错误信息';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_export_job.gmt_update IS '更新时间';

COMMENT ON TABLE cdeu.finv_quote_cdeu_ingest_stats IS '每日入口流量统计，用于计算日预算；基于 cdeu.finv_quote_secu_kline_min 按 gmt_create 聚合';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_ingest_stats.stat_date IS '统计日期（业务时区换算后的自然日）';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_ingest_stats.total_rows IS '当日入口总条数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_ingest_stats.total_bytes IS '当日入口总字节数（pg_column_size 估算）';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_ingest_stats.gmt_update IS '统计更新时间';

COMMENT ON TABLE cdeu.finv_quote_cdeu_job_run_log_export_file IS '任务运行日志，记录每次 Job 执行的耗时、处理量与结果；job_id 逻辑关联 cdeu.finv_quote_cdeu_export_job.job_id，无外键';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.run_id IS '运行记录主键';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.job_kind IS 'Job 类型：CALC_DAILY_INGEST_STATS / CALC_DAILY_EXPORT_BUDGET / REFRESH_QUOTE_MATURITY / MARK_SEALED_JOBS_FROZEN / RUN_DUE_EXPORT_JOBS / UPLOAD_PENDING_EXPORT_FILES / SCAN_POST_SEAL_PATCHES / CLEAN_EXPIRED_EXPORT_ARTIFACTS / RECOVER_TIMEOUT_EXPORT_JOBS / CHECK_EXPORT_JOB_HEALTH';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.job_id IS '关联的导出任务 ID，可为空，逻辑关联 cdeu.finv_quote_cdeu_export_job.job_id';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.usc IS '证券唯一编码，可为空';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.biz_month IS '归档月份 yyyyMM，可为空';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.gmt_start IS '开始时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.gmt_finish IS '结束时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.status IS '运行结果：SUCCESS / FAILED / PAUSED';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.processed_rows IS '本次处理行数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.processed_bytes IS '本次处理字节数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.error IS '错误信息或差异报告摘要';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.stage IS '当前/最近执行阶段：RUNNING / STATS / BUDGET / MATURITY / SEAL / LOAD / EXPORT / CLEANUP / UPLOAD / PATCH / RECOVER / HEALTH / DONE / FAILED / PAUSED';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.progress_percent IS '进度百分比 0~100；NULL 表示不适用，负值表示无法估算总量的不确定进度';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.progress_detail IS '进度明细（如当前证券月、已装载行数、已上传文件数）';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_export_file.gmt_heartbeat IS '最近心跳时间，用于判断运行中的 Job 是否卡住';

COMMENT ON TABLE cdeu.finv_quote_cdeu_job_run_log_line IS 'Job 运行详细日志行，run_id 逻辑关联 finv_quote_cdeu_job_run_log_export_file.run_id，无外键';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_line.level IS '日志级别：INFO / WARN / ERROR';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_line.stage IS '发生日志时的执行阶段';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_line.message IS '日志正文';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_job_run_log_line.progress_percent IS '写入日志时的进度百分比，NULL 表示不适用';

COMMENT ON TABLE cdeu.finv_quote_cdeu_quote_maturity_mon IS '按证券日维护的数据成熟度，由主表聚合生成；只有 COMPLETE / FROZEN 的证券月进入导出候选';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_quote_maturity_mon.usc IS '证券唯一编码';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_quote_maturity_mon.trade_date IS '交易日期 yyyyMMdd';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_quote_maturity_mon.biz_month IS '归属月份 yyyyMM，由 trade_date/100 推导';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_quote_maturity_mon.expected_rows IS '预期条数，来自 cdeu.finv_quote_cdeu_usc.day_expect_rows 与交易日历；非交易日为 0，未知预期为 0 且状态保持 PARTIAL';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_quote_maturity_mon.actual_rows IS '主表实际入库条数';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_quote_maturity_mon.maturity_status IS '成熟度：IMMATURE / PARTIAL / COMPLETE / FROZEN';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_quote_maturity_mon.gmt_first_seen IS '首次发现该日数据的时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_quote_maturity_mon.gmt_last_arrival IS '最近一次有数据到达的时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_quote_maturity_mon.gmt_seale IS '该日被收口冻结的时间（列名为方案稿原名，语义为 seal 时间）';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_quote_maturity_mon.gmt_update IS '更新时间';

COMMENT ON TABLE cdeu.finv_quote_cdeu_scan_watermark IS '增量扫描水位，如成熟度扫描、补丁扫描；watermark_name 约定见 Docs/Design/2610-SecuritiesQuoteExportArchive.md';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_scan_watermark.watermark_name IS '水位名称';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_scan_watermark.last_value IS '上次扫描到的最大时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_scan_watermark.gmt_update IS '更新时间';

COMMENT ON TABLE cdeu.finv_quote_cdeu_trade_calendar IS '证券交易日历，trade_date 为 yyyyMMdd；7x24 证券全部为 true；表为空时按「交易日」保守处理';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_trade_calendar.usc IS '证券唯一编码';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_trade_calendar.trade_date IS '交易日期 yyyyMMdd';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_trade_calendar.flag_trade_day IS '是否交易日，true/false';

COMMENT ON TABLE cdeu.finv_quote_cdeu_usc IS '证券主数据，可基于主表自动发现新 usc；region/market/provider/name/timezone/currency 用于生成 MVsv 文件头与文件名，缺失时不生成导出候选';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.usc IS '证券唯一编码';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.category IS '证券类别：A_SHARE / CRYPTO / ...';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.day_expect_rows IS '预期每日条数：A 股 241，加密货币 1440；为空时该日成熟度保持 PARTIAL';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.date_list IS '上市/上线日期';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.date_delist IS '退市/下线日期，空表示仍在市';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.flag_auto_discover IS '是否由系统自动发现';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.region IS '区域代码，如 CN，用于文件名与 MVsv 文件头 Region';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.market IS '市场代码，如 SZ，用于文件名与 MVsv 文件头 Market';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.provider IS '数据提供方，如 FTMM，用于文件名与 MVsv 文件头 DataProvider';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.name IS '证券名称，如 黄金ETF易方达，用于 MVsv 文件头 Name';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.timezone IS '时区，如 Asia/Shanghai，用于 MVsv 文件头 TimeZone';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.currency IS '币种，如 CNY，用于 MVsv 文件头 Currency';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.gmt_update IS '更新时间';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.priority_rank IS '归档优先级，数值越小越优先；NULL 表示未配置，按 100 兜底';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.sla_days IS '收口（seal_date）后多少天内必须完成导出，决定 freeze_date；NULL 表示未配置，按 5 兜底';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.max_wait_days IS '低优先级最长等待天数，超过后提升排序防饿死；NULL 表示未配置，按 7 兜底';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc.priority_gmt_update IS '优先级配置最后修改时间（与主数据 gmt_update 分离，便于审计）';

COMMENT ON TABLE cdeu.finv_quote_secu_kline_min IS '分钟行情 K 线数据（融合 FinvQuoteDataDeDup；ts 为秒级 Unix 时间戳；last_close 位于 ext_field 之后）';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.usc IS '证券唯一编码';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.ts IS '时间戳（秒级 Unix 时间戳）';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.date IS '交易日期（yyyyMMdd）';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min."time" IS '交易时间（HHmmss）';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.open IS '开盘价';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.high IS '最高价';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.low IS '最低价';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.close IS '收盘价（当前分钟收盘价）';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.paocd IS '当日实时累计均价';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.volume IS '成交量';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.turnover IS '成交额';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.change_price IS '涨跌额';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.change_ratio IS '涨跌幅';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.ext_field IS '扩展字段（预留，存附加信息）';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.last_close IS '前收盘价（数据源 MVSV 的 PrevClose / p 列）；与 open 独立，不可作为开盘价或开盘价的替代';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.remark IS '备注';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.provider IS '数据来源（行情供应商）';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.gmt_create IS '创建时间';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_min.gmt_update IS '更新时间';


-- ---------------------------------------------------------------------
-- 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_tables      bigint;
    v_indexes     bigint;
    v_constraints bigint;
    v_seq         bigint;
    v_missing     text;
BEGIN
    SELECT count(*) INTO v_tables FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'cdeu' AND c.relkind = 'r';
    IF v_tables <> 26 THEN
        RAISE EXCEPTION 'V100078 收敛失败：cdeu 下应有 26 张表，实际 %', v_tables;
    END IF;

    SELECT string_agg(t, ',') INTO v_missing
      FROM unnest(ARRAY[
        'cdeu_workflow','cdeu_phase','cdeu_process','cdeu_job','cdeu_job_step',
        'cdeu_workflow_run','cdeu_phase_run','cdeu_process_run','cdeu_job_run','cdeu_job_step_run',
        'cdeu_job_dynamic','cdeu_job_step_dynamic',
        'finv_quote_cdeu_daily_budget','finv_quote_cdeu_export_batch','finv_quote_cdeu_export_file',
        'finv_quote_cdeu_export_file_patch','finv_quote_cdeu_export_file_upload_log',
        'finv_quote_cdeu_export_job','finv_quote_cdeu_ingest_stats',
        'finv_quote_cdeu_job_run_log_export_file','finv_quote_cdeu_job_run_log_line',
        'finv_quote_cdeu_quote_maturity_mon','finv_quote_cdeu_scan_watermark',
        'finv_quote_cdeu_trade_calendar','finv_quote_cdeu_usc',
        'finv_quote_secu_kline_min'
      ]) t
     WHERE NOT EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
                        WHERE n.nspname = 'cdeu' AND c.relname = t AND c.relkind = 'r');
    IF v_missing IS NOT NULL THEN
        RAISE EXCEPTION 'V100078 收敛失败：缺少表 %', v_missing;
    END IF;

    SELECT count(*) INTO v_indexes FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'cdeu' AND c.relkind = 'i';
    SELECT count(*) INTO v_constraints FROM pg_constraint co JOIN pg_namespace n ON n.oid = co.connamespace
     WHERE n.nspname = 'cdeu';
    SELECT count(*) INTO v_seq FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'cdeu' AND c.relkind = 'S';

    IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
                WHERE n.nspname = 'cdeu' AND c.relname LIKE 'prep_bak%') THEN
        RAISE EXCEPTION 'V100078 收敛失败：cdeu 中出现 prep_bak 表';
    END IF;

    RAISE NOTICE 'V100078 完成：cdeu 下 % 张表 / % 个索引 / % 个约束 / % 个序列（无 prep_bak）',
        v_tables, v_indexes, v_constraints, v_seq;
END $$;

COMMIT;
