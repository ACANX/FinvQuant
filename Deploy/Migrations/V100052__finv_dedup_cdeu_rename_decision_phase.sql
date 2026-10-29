-- =====================================================================
-- FinvQuant PostgreSQL V52：阶段名称调整
--
-- PHASE_DECISION 展示名称由「导出判定」调整为「导出决策」；编码与执行逻辑不变。
-- =====================================================================

UPDATE dedup.cdeu_phase
   SET name = '导出决策', gmt_update = now()
 WHERE phase_code = 'PHASE_DECISION';
