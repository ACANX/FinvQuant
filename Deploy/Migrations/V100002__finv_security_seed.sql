-- =====================================================================
-- FinvQuant PostgreSQL V100002：证券代码初始数据（finv_security）
--
-- 决策（ACANX 2026-08-05）：
--   - 数据种子统一使用 V100000+ 段位，确保在所有表结构脚本之后执行；
--   - 初始 15 条：黄金期货主连 + 全球主要股指（A股/港股/美股）；
--   - exchange_code 对应 finv_exchange（11=上交所、12=深交所、13=北交所、
--     21=港交所、31=纳斯达克、33=芝商所），currency_type 对应 finv_currency；
--   - 幂等插入（ON CONFLICT (usc) DO NOTHING）。
-- 迁移策略：与既有迁移一致，整个 V100002 在一个事务内执行；失败自动回滚。
-- =====================================================================

BEGIN;

INSERT INTO finv_security
    (usc, exchange_code, security_type, security_code, security_name, security_name_cn, security_name_full, currency_type, init_date, timezone, tz)
VALUES
    ('GCMain', 33, 'Futures',    'GCMain', 'GCMain2512',       '黄金期货主连',     '黄金期货主连',     'USD', 19710101, '-04:00',      'America/New_York'),
    ('HSTI',   21, 'StockIndex', '800700', '恒生科技指数',      '恒生科技指数',      '恒生科技指数',      'HKD', 20000000, '+08:00',      'Asia/Shanghai'),
    ('SHI',    11, 'StockIndex', '000001', '上证综合指数',      '上证综指数',        '上证综指数',        'CNY', 20000000, '+08:00',      'Asia/Shanghai'),
    ('HS300I', 11, 'StockIndex', '000300', '沪深300指数',       '沪深300指数',       '沪深300指数',       'CNY', 20000000, '+08:00',      'Asia/Shanghai'),
    ('SZCZ',   12, 'StockIndex', '深证成交指数', '深证成交指数', '深证成交指数',      '深证成交指数',      'CNY', 20000000, '+08:00',      'Asia/Shanghai'),
    ('CYB',    12, 'StockIndex', '创业板指数', '创业板指数',     '创业板指数',        '创业板指数',        'CNY', 20000000, '+08:00',      'Asia/Shanghai'),
    ('BSE50I', 13, 'StockIndex', '899050', '北证50',           '北证50指数',        '北证50指数',        'CNY', 20000000, '+08:00',      'Asia/Shanghai'),
    ('KC50',   11, 'StockIndex', '000688', '科创50',           '科创50',            '科创50指数',        'CNY', 20000000, '+08:00',      'Asia/Shanghai'),
    ('A500',   11, 'StockIndex', '000510', '中证A500',         '中证A500指数',      '中证A500指数',      'CNY', 20000000, '+08:00',      'Asia/Shanghai'),
    ('HSI',    21, 'StockIndex', '800000', '恒生指数',          '恒生指数',          '恒生指数',          'HKD', 20000000, '+08:00',      'Asia/Shanghai'),
    ('NDX',    31, 'StockIndex', 'NDX',    '纳斯达克综合指数',  '纳斯达克综合指数',  '纳斯达克综合指数',  'USD', 20000000, '-04:00',      'America/New_York'),
    ('DAJI',   33, 'StockIndex', 'DAJI',   '道琼斯工业平均指数', '道琼斯工业平均指数', '道琼斯工业平均指数', 'USD', 20000000, '-04:00',      'America/New_York'),
    ('SDPR500',33, 'StockIndex', 'SDPR',   '标准普尔500指数',   '标普500指数',       '标普500指数',       'USD', 20000000, '-04:00',      'America/New_York'),
    ('HXC',    31, 'StockIndex', 'HXC',    '纳斯达克中国金龙指数', '纳斯达克中国金龙指数', '纳斯达克中国金龙指数', 'USD', 20000000, '-04:00',   'America/New_York'),
    ('000985', 11, 'StockIndex', '000985', '中证全指',          '中证全指',          '中证全指',          'CNY', 20000000, '+08:00',      'Asia/Shanghai')
ON CONFLICT (usc) DO NOTHING;

COMMIT;
