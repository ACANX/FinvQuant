-- =====================================================================
-- FinvQuant PostgreSQL V45：将原「保障（旁路）」阶段拆分为具体步骤
--
-- 背景：CDEU 定义最初把 STEP_RECOVER / STEP_HEALTH 放在最后的 PHASE_OPS，
--       导致流程图画成一个独立尾部阶段。旁路步骤应与业务阶段并行展示：
--         - STEP_RECOVER  → PHASE_EXPORT（导出阶段，seq=3，旁路）
--         - STEP_HEALTH   → PHASE_UPLOAD（上传阶段，seq=3，旁路）
--       两者均保持 run_mode=CONTINUOUS、critical=false、affects_completion=false，
--       不参与流程完成判定，但会出现在对应阶段的步骤清单中。
-- 说明：只改定义表；历史运行表（cdeu_phase_run/cdeu_step_run）保留原样，
--       不删除历史审计数据；全库无外键。
-- =====================================================================

UPDATE dedup.cdeu_step AS s
   SET phase_id = ph.phase_id,
       seq_no = 3,
       run_mode = 'CONTINUOUS',
       critical = false,
       affects_completion = false,
       gmt_update = now()
  FROM dedup.cdeu_phase AS ph
  JOIN dedup.cdeu_process AS p ON p.process_id = ph.process_id
 WHERE s.step_code = 'STEP_RECOVER'
   AND p.process_code = 'CDEU_DAILY'
   AND ph.phase_code = 'PHASE_EXPORT';

UPDATE dedup.cdeu_step AS s
   SET phase_id = ph.phase_id,
       seq_no = 3,
       run_mode = 'CONTINUOUS',
       critical = false,
       affects_completion = false,
       gmt_update = now()
  FROM dedup.cdeu_phase AS ph
  JOIN dedup.cdeu_process AS p ON p.process_id = ph.process_id
 WHERE s.step_code = 'STEP_HEALTH'
   AND p.process_code = 'CDEU_DAILY'
   AND ph.phase_code = 'PHASE_UPLOAD';

DELETE FROM dedup.cdeu_phase AS ph
 USING dedup.cdeu_process AS p
 WHERE ph.process_id = p.process_id
   AND p.process_code = 'CDEU_DAILY'
   AND ph.phase_code = 'PHASE_OPS';
