-- =====================================================================
-- FinvQuant PostgreSQL V100001：货币字典初始数据（finv_currency）
--
-- 决策（ACANX 2026-08-05）：
--   - 数据种子统一使用 V100000+ 段位，确保在所有表结构脚本之后执行；
--   - 初始 7 条货币（CNY / USD / HKD / NTD / JPY / SGD / INR）；
--   - 汇率来自 FT 货币清单，1 单位本币 = N 人民币（CNY 基准为 1.0）。
-- 迁移策略：与既有迁移一致，整个 V100001 在一个事务内执行；失败自动回滚。
-- =====================================================================

BEGIN;

-- 幂等插入：重复执行不产生重复记录
INSERT INTO finv_currency (currency_type, currency_name, exchange_rate_cny)
VALUES
    ('CNY', '人民币',     1.0),
    ('USD', '美元',       7.269),
    ('HKD', '港币',       0.89),
    ('NTD', '新台币',     0.26),
    ('JPY', '日元',       0.03),
    ('SGD', '新加坡币',   1.1),
    ('INR', '印度卢比',   0.5)
ON CONFLICT (currency_type) DO NOTHING;

COMMIT;
