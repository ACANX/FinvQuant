-- =====================================================================
-- FinvQuant PostgreSQL V100037：重置归档导出状态，准备按新 region/market 口径重跑
--
-- 背景（ACANX 2026-10-26）：
--   V100036 把 dedup.finv_dedup_usc 的 region/market 改为新口径
--   （market 取 finv_market.market_symbol，region 按 finv_region 规范化），
--   导致 MVsv 文件名与归档目录变化（如 CN_SSE-A_… → CN_SH_…）。
--   已有 101 个 EXPORTED 任务与 226 条文件登记是旧口径产物，必须清除后
--   重跑完整工作流，否则：
--     - RegisterFile 在「checksum 未变 + 已 UPLOADED」时保持 UPLOADED，
--       新文件不会按新路径重新上传；
--     - EXPORTED 任务不会被 BASE 候选生成重新选中。
--
-- 本迁移只处理**数据表**（不动本地文件与远端仓库对象，二者由运维另行清理）：
--   1) 删除文件登记（dedup.finv_dedup_export_file / _patch）与批次（_batch）；
--      上传审计日志 finv_dedup_export_file_upload_log 保留，作为历史凭据。
--   2) 导出任务回退为 FROZEN（可被 ClaimJob 重新领取），并清空游标、装载计数、
--      版本、补丁计数、锁与错误。
--   3) 当日预算清零消耗并抬高上限，保证一轮能覆盖全量重导
--      （主表约 280 万行；CALC_DAILY_EXPORT_BUDGET 若随后重算，按入口统计
--      得出的预算同样满足，不会卡住）。
--   4) 成熟度 FROZEN、扫描水位、usc 主数据、CDEU 流程记录均不动：
--      前三者是重导的前置输入，后者是历史审计；动态作业由
--      EnsureDynamicExportJob / GenerateUploadDynamicJobs 在新一轮自动重置。
--
-- 幂等 / 重跑保护：仅当仍存在 status='EXPORTED' 的任务时才执行重置；
-- 重置完成后重跑会直接跳过（避免导出进行中被二次清空）。
-- 迁移策略：单事务、失败整体回滚。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_exported  INT;
    v_jobs      INT;
    v_files     INT;
    v_patches   INT;
    v_batches   INT;
    v_left      INT;
BEGIN
    SELECT count(*) INTO v_exported
      FROM dedup.finv_dedup_export_job
     WHERE status = 'EXPORTED';

    IF v_exported = 0 THEN
        RAISE NOTICE 'V100037: 无 EXPORTED 任务（已重置过或尚未导出），跳过';
        RETURN;
    END IF;

    -- -----------------------------------------------------------------
    -- 1. 清理旧口径的文件登记与批次
    -- -----------------------------------------------------------------
    DELETE FROM dedup.finv_dedup_export_file_patch;
    GET DIAGNOSTICS v_patches = ROW_COUNT;

    DELETE FROM dedup.finv_dedup_export_file;
    GET DIAGNOSTICS v_files = ROW_COUNT;

    DELETE FROM dedup.finv_dedup_export_batch;
    GET DIAGNOSTICS v_batches = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 2. 导出任务回退为 FROZEN（保留 job_id，避免破坏 CDEU 引用与历史）
    -- -----------------------------------------------------------------
    UPDATE dedup.finv_dedup_export_job
       SET status             = 'FROZEN',
           attempt            = 0,
           locked_by          = NULL,
           locked_until       = NULL,
           gmt_heartbeat      = NULL,
           gmt_next_run       = NULL,
           load_cursor_date   = NULL,
           load_cursor_ts     = NULL,
           loaded_rows        = 0,
           base_version       = 0,
           patch_count        = 0,
           oldest_pending_date = NULL,
           last_error         = NULL,
           gmt_update         = now()
     WHERE status <> 'FROZEN'
        OR attempt <> 0
        OR locked_by IS NOT NULL
        OR locked_until IS NOT NULL
        OR gmt_next_run IS NOT NULL
        OR load_cursor_date IS NOT NULL
        OR load_cursor_ts IS NOT NULL
        OR loaded_rows <> 0
        OR base_version <> 0
        OR patch_count <> 0
        OR last_error IS NOT NULL;
    GET DIAGNOSTICS v_jobs = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 3. 当日预算：清零消耗 + 抬高上限（一轮覆盖全量重导）
    --    主表 finv_quote_secu_kline_min 约 280 万行，留足余量取 500 万。
    -- -----------------------------------------------------------------
    UPDATE dedup.finv_dedup_daily_budget
       SET budget_rows     = 5000000,
           budget_bytes    = 640000000,
           hard_cap_rows   = 6000000,
           consumed_rows   = 0,
           consumed_bytes  = 0,
           status          = 'CALCULATED'
     WHERE biz_date = CURRENT_DATE;

    IF NOT FOUND THEN
        INSERT INTO dedup.finv_dedup_daily_budget
            (biz_date, base_rows, base_bytes, p75_rows, backlog_ratio, sla_pressure,
             factor, budget_rows, budget_bytes, hard_cap_rows,
             consumed_rows, consumed_bytes, status, gmt_create)
        VALUES (CURRENT_DATE, 0, 0, 0, 0, 0, 1.20,
                5000000, 640000000, 6000000, 0, 0, 'CALCULATED', now());
    END IF;

    -- -----------------------------------------------------------------
    -- 4. 收敛校验
    -- -----------------------------------------------------------------
    SELECT count(*) INTO v_left FROM dedup.finv_dedup_export_file;
    IF v_left <> 0 THEN
        RAISE EXCEPTION 'V100037: 仍有 % 条文件登记未清除', v_left;
    END IF;

    SELECT count(*) INTO v_left FROM dedup.finv_dedup_export_batch;
    IF v_left <> 0 THEN
        RAISE EXCEPTION 'V100037: 仍有 % 条导出批次未清除', v_left;
    END IF;

    SELECT count(*) INTO v_left
      FROM dedup.finv_dedup_export_job
     WHERE status <> 'FROZEN';
    IF v_left <> 0 THEN
        RAISE EXCEPTION 'V100037: 仍有 % 个导出任务不是 FROZEN', v_left;
    END IF;

    SELECT count(*) INTO v_left
      FROM dedup.finv_dedup_daily_budget
     WHERE biz_date = CURRENT_DATE AND (budget_rows < 2804326 OR consumed_rows <> 0);
    IF v_left <> 0 THEN
        RAISE EXCEPTION 'V100037: 当日预算未就绪（需 budget_rows >= 2804326 且 consumed_rows = 0）';
    END IF;

    RAISE NOTICE 'V100037: 导出状态已重置（文件 % 条、补丁 % 条、批次 % 条已清；任务 % 个回退 FROZEN；当日预算 % - % 行）',
        v_files, v_patches, v_batches, v_jobs,
        (SELECT budget_rows FROM dedup.finv_dedup_daily_budget WHERE biz_date = CURRENT_DATE),
        (SELECT hard_cap_rows FROM dedup.finv_dedup_daily_budget WHERE biz_date = CURRENT_DATE);
END $$;

COMMIT;
