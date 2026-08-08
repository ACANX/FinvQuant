-- =====================================================================
-- FinvQuant PostgreSQL V100012：finv_exchange 芝加哥期权交易所缩写修正（COBE → CBOE）
--
-- 决策（ACANX 2026-08-05）：
--   - 芝加哥期权交易所（Chicago Board Options Exchange）标准缩写为 CBOE，
--     富途侧（finv_futu_mapping_exchange）亦使用 CBOE，统一为 CBOE；
--   - V100010 全量文件已同步改为 CBOE，本迁移兜底修正已执行过旧数据的环境
--     （exchange_code=34 且仍为 COBE 的记录）；
--   - 幂等：仅命中 exchange_code=34 的记录，重复执行结果一致；
--   - 单事务、失败回滚。
-- =====================================================================

BEGIN;

UPDATE finv_exchange
   SET exchange_flag = 'CBOE',
       exchange_abbr = 'CBOE'
 WHERE exchange_code = 34
   AND exchange_flag = 'COBE';

COMMIT;
