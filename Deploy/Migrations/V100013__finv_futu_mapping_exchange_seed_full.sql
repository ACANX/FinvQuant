-- =====================================================================
-- FinvQuant PostgreSQL V100013：finv_futu_mapping_exchange 全量初始数据（30 条，替换 V100006 存量）
--
-- 决策（ACANX 2026-08-05）：
--   - 用户提供全量 30 条数据（与 V100006 键集合一致，3 处 finv_exchange 映射更新），
--     作为 finv_futu_mapping_exchange 的最终初始数据；
--   - V100006 已发布不可修改，新增 V100013 迁移：先 DELETE 清空存量，
--     再 INSERT 全量 30 条（满足"存量先删除、全量入库"要求）；
--   - finv_exchange 映射更新（相对 V100006，对齐 finv_exchange 字典 exchange_flag）：
--       JP   : JP   -> TSE  （日本东京证券交易所，finv_exchange=52 TSE）
--       KR   : KR   -> KRX  （韩国证券交易所，finv_exchange=57 KRX）
--       SEHK : SEHK -> HKEX （香港交易所，finv_exchange=21 HKEX）
--     其余 27 条与 V100006 一致；
--   - gmt_create / gmt_update 由 DEFAULT now() 生成；
--   - 单事务、失败回滚；先清后插幂等。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 清空存量初始数据（V100006 及手动补充数据）
-- ---------------------------------------------------------------------
DELETE FROM finv_futu_mapping_exchange;

-- ---------------------------------------------------------------------
-- 2. 全量初始数据（30 条）
-- ---------------------------------------------------------------------
INSERT INTO finv_futu_mapping_exchange
    (futu_exchange, region, abbr, exchange_name, finv_exchange)
VALUES
    ('ASX',    '澳大利亚',     'AU',     '澳大利亚证券交易所',                               'ASX'),
    ('BD',     '—',            'N/A',    '债券（Bond）',                                    'BD'),
    ('BMD',    '马来西亚',     'MY',     '马来西亚衍生品交易所（Bursa Malaysia Derivatives）', 'BMD'),
    ('BMS',    '—',            'N/A',    '债券市场系统（Bond Market System）',              'BMS'),
    ('CA',     '加拿大',       'CA',     '加拿大证券市场（TSX/TSXV/CSE 统称）',             'CA'),
    ('CBOT',   '美国',         'USA',    '芝加哥期货交易所（谷物/国债期货）',                'CBOT'),
    ('CCCOIN', '全球',         'GLOBAL', '加密货币（Coinbase）',                            'CCCOIN'),
    ('CCBGO',  '全球',         'GLOBAL', '加密货币（Bitget/BitGo）',                        'CCBGO'),
    ('CCDDEX', '全球',         'GLOBAL', '加密货币去中心化交易所（DEX）',                    'CCDDEX'),
    ('CCHSK',  '全球',         'GLOBAL', '加密货币（HashKey）',                             'CCHSK'),
    ('CBOE',   '美国',         'USA',    '芝加哥期权交易所',                                'CBOE'),
    ('CCPT',   '全球',         'GLOBAL', '加密货币（PT/Poloniex）',                         'CCPT'),
    ('CME',    '美国',         'USA',    '芝加哥商业交易所（外汇/利率/股指期货）',           'CME'),
    ('COMEX',  '美国',         'USA',    '纽约商品交易所（黄金/白银期货）',                  'COMEX'),
    ('CRYPTO', '全球',         'GLOBAL', '加密货币交易所（Binance 等）',                    'CRYPTO'),
    ('FD',     '—',            'N/A',    '基金（Fund）',                                    'FD'),
    ('FTSP',   '—',            'N/A',    '富途结构化产品平台',                              'FTSP'),
    ('FTSN',   '香港/新加坡',  'HK/SG',  '富途结构化产品网络（Futu Structured Network）',    'FTSN'),
    ('FX',     '全球',         'GLOBAL', '外汇市场（Forex）',                               'FX'),
    ('HKFE',   '香港',         'HK',     '香港期货交易所',                                  'HKFE'),
    ('JP',     '日本',         'JP',     '日本东京证券交易所',                              'TSE'),
    ('KR',     '韩国',         'KR',     '韩国证券交易所（KRX）',                           'KRX'),
    ('NYMEX',  '美国',         'USA',    '纽约商品交易所（能源/金属期货）',                  'NYMEX'),
    ('OSE',    '日本',         'JP',     '大阪交易所（日本商品/股指期货）',                  'OSE'),
    ('SEHK',   '香港',         'HK',     '香港交易所（Hong Kong Stock Exchange）',          'HKEX'),
    ('SGX',    '新加坡',       'SG',     '新加坡交易所',                                    'SGX'),
    ('SSE',    '中国',         'CN',     '上海证券交易所',                                  'SSE'),
    ('SZSE',   '中国',         'CN',     '深圳证券交易所',                                  'SZSE'),
    ('TFD',    '—',            'N/A',    '测试/仿真环境',                                   'TFD'),
    ('US',     '美国',         'USA',    '美国证券市场（NYSE/NASDAQ/ARCA/BATS/OTC 统称）',  'US');

COMMIT;
