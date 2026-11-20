-- =====================================================================
-- V300108：让「待上传文件上传」JOB 在工作流面板可见（需求 10942）
--
-- 背景：
--   ① sys_config 20005（CDEU工作流运行模式）为 Manual 时，进程内调度器整体不触发，
--      所有 JOB 都不会自动跑，只能靠手动执行推进；
--   ② 而 JOB_UPLOAD_FILES（UPLOAD_PENDING_EXPORT_FILES）的 ui_visible 为 false，
--      面板上根本不显示这个 JOB，用户既等不到自动上传、也找不到手动入口，
--      于是「月度行情上传」阶段永久卡在 RUNNING。
--
-- 本迁移只把该 JOB 的 ui_visible 置 true，不动调度与执行逻辑。
-- =====================================================================

BEGIN;

UPDATE cdeu.finv_quote_cdeu_job
   SET ui_visible = true, gmt_update = now()
 WHERE job_code = 'JOB_UPLOAD_FILES' AND NOT ui_visible;

DO $$
DECLARE v boolean;
BEGIN
    SELECT ui_visible INTO v FROM cdeu.finv_quote_cdeu_job WHERE job_code = 'JOB_UPLOAD_FILES';
    RAISE NOTICE 'JOB_UPLOAD_FILES.ui_visible = %', v;
    IF NOT v THEN
        RAISE EXCEPTION '更新失败，回滚';
    END IF;
END $$;

COMMIT;
