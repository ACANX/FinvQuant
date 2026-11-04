-- =====================================================================
-- FinvQuant PostgreSQL V60：采集阶段名称调整
--
-- PHASE_COLLECT 展示名称由「历史行情采集」调整为「历史行情收集」。
-- =====================================================================

UPDATE dedup.cdeu_phase
   SET name = '历史行情收集', gmt_update = now()
 WHERE phase_code = 'PHASE_COLLECT';
