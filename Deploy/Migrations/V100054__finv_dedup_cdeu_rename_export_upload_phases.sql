-- =====================================================================
-- FinvQuant PostgreSQL V54：导出 / 上传阶段命名调整
--
--   PHASE_EXPORT → 月度行情导出
--   PHASE_UPLOAD → 月度行情上传
-- 编码与执行逻辑不变。
-- =====================================================================

UPDATE dedup.cdeu_phase
   SET name = '月度行情导出', gmt_update = now()
 WHERE phase_code = 'PHASE_EXPORT';

UPDATE dedup.cdeu_phase
   SET name = '月度行情上传', gmt_update = now()
 WHERE phase_code = 'PHASE_UPLOAD';
