-- =====================================================================
-- FinvQuant PostgreSQL V49：将 STEP_MATURITY 移入「导出判定」阶段
--
-- 调整后：
--   PHASE_DEDUP    ：STEP_SEAL（收口冻结，seq=1）
--   PHASE_DECISION ：STEP_INGEST_STATS（seq=1）
--                    STEP_BUDGET（seq=2）
--                    STEP_MATURITY（成熟度扫描，seq=3）
--                    STEP_EXPORT_DECISION（占位，seq=4）
-- JOB（REFRESH_QUOTE_MATURITY）随步骤自动迁移。
-- =====================================================================

-- 先给成熟度步骤腾出 seq=3。
UPDATE dedup.cdeu_step AS st
   SET seq_no = 4, gmt_update = now()
  FROM dedup.cdeu_phase AS ph
  JOIN dedup.cdeu_process AS p ON p.process_id = ph.process_id
 WHERE st.phase_id = ph.phase_id
   AND p.process_code = 'CDEU_DAILY'
   AND ph.phase_code = 'PHASE_DECISION'
   AND st.step_code = 'STEP_EXPORT_DECISION';

-- 成熟度扫描 → 导出判定 seq=3。
UPDATE dedup.cdeu_step AS st
   SET phase_id = (
           SELECT target.phase_id
             FROM dedup.cdeu_phase target
             JOIN dedup.cdeu_process target_process ON target_process.process_id = target.process_id
            WHERE target_process.process_code = 'CDEU_DAILY'
              AND target.phase_code = 'PHASE_DECISION'
            LIMIT 1
       ),
       seq_no = 3,
       gmt_update = now()
 WHERE st.step_code = 'STEP_MATURITY'
   AND EXISTS (
       SELECT 1
         FROM dedup.cdeu_phase current_phase
         JOIN dedup.cdeu_process current_process ON current_process.process_id = current_phase.process_id
        WHERE current_phase.phase_id = st.phase_id
          AND current_process.process_code = 'CDEU_DAILY'
   );

-- 去重收口只剩收口冻结，顺序归位为 1。
UPDATE dedup.cdeu_step AS st
   SET seq_no = 1, gmt_update = now()
  FROM dedup.cdeu_phase AS ph
  JOIN dedup.cdeu_process AS p ON p.process_id = ph.process_id
 WHERE st.phase_id = ph.phase_id
   AND p.process_code = 'CDEU_DAILY'
   AND ph.phase_code = 'PHASE_DEDUP'
   AND st.step_code = 'STEP_SEAL';
