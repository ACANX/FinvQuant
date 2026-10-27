-- =====================================================================
-- FinvQuant PostgreSQL V100016：finv_exchange 全量初始数据（44 条，替换 V100010 存量）
--
-- 决策（ACANX 2026-08-05）：
--   - 用户再次提供 finv_exchange 全量 44 条（较 V100010 仅 1 处字段调整：
--     code 101 FX-CFD 的 exchange_name/exchange_abbr_cn 互换，即
--     exchange_name='FX-CFD'、exchange_abbr_cn='外汇-差价合约'）；
--   - V100010 已发布不可修改，新增 V100016：先 DELETE 清空存量，
--     再 INSERT 全量 44 条（满足"存量先删除、全量入库"要求）；
--   - gmt_create / gmt_update 由 DEFAULT now() 生成，不随数据插入；
--   - ft_list_exchange_code 留空（映射预留）；
--   - 单事务、失败回滚；先清后插幂等。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 清空存量初始数据（V100010 及手动补充数据）
-- ---------------------------------------------------------------------
DELETE FROM finv_exchange;

-- ---------------------------------------------------------------------
-- 2. 全量初始数据（44 条）
-- ---------------------------------------------------------------------
INSERT INTO finv_exchange
    (exchange_code, exchange_flag, exchange_abbr, exchange_name, exchange_abbr_cn, en_market_type, region, base_currency)
VALUES
(10, 'CN', 'CN', '中国证券市场', '中国证券市场(全体)', '证券', 'CN', 'CNY'),
(11, 'SH', 'SSE', '上海证券交易所', '上交所', '证券', 'CN', 'CNY'),
(12, 'SZ', 'SZSE', '深圳证券交易所', '深交所', '证券', 'CN', 'CNY'),
(13, 'BJ', 'BSE', '北京证券交易所', '北交所', '证券', 'CN', 'CNY'),
(14, 'SHFE', 'SHFE', '上海期货交易所', '上海期货交易所', '期货', 'CN', 'CNY'),
(15, 'SGE', 'SGE', '上海黄金交易所', '上海黄金交易所', '黄金及贵金属', 'CN', 'CNY'),
(19, 'CNOTC', 'CNOTC', '中国场外交易', '中国场外交易', '场外', 'CN', 'CNY'),
(21, 'HK', 'HKEX', '香港联合证券交易所', '港交所', '证券', 'HK', 'HKD'),
(22, 'HKFE', 'HKFE', 'Hong Kong Futures Exchange', '香港期货交易所', '期货', 'HK', 'HKD'),
(29, 'TW', 'TWSE', '台湾证券交易所', '台交所', '证券', 'TW', 'NTD'),
(30, 'US', 'US', '美国证券市场', '美国证券市场(全体)', '证券', 'USA', 'USD'),
(31, 'NSDQ', 'NSDQ', '纳斯达克证券交易所', '纳斯达克', '证券', 'USA', 'USD'),
(32, 'NYSE', 'NYSE', '纽约证券交易所', '纽交所', '证券', 'USA', 'USD'),
(33, 'COMEX', 'COMEX', '芝加哥商品期货交易所', '芝商所', '期货', 'USA', 'USD'),
(34, 'CBOE', 'CBOE', '芝加哥期权交易所', '芝加哥期权交易所', '期权', 'USA', 'USD'),
(35, 'PINK', 'PINK', '粉红单交易市场', '粉单市场', '场外', 'USA', 'USD'),
(36, 'NYMEX', 'NYMEX', 'New York Mercantile Exchange', '纽约商品交易所', '期货', 'USA', 'USD'),
(37, 'CME', 'CME', 'Chicago Mercantile Exchange', '芝加哥商业交易所', '期货', 'USA', 'USD'),
(38, 'CBOT', 'CBOT', 'Chicago Board of Trade', '芝加哥期货交易所', '期货', 'USA', 'USD'),
(40, 'CA', 'CA', 'Canadian Securities Market', '加拿大证券市场', '证券', 'CA', 'CAD'),
(51, 'OSE', 'OSE', '大阪证券交易所', '大阪证券交易所', '证券', 'JP', 'JPY'),
(52, 'TSE', 'TSE', '东京证券交易所', '东京证券交易所', '证券', 'JP', 'JPY'),
(53, 'SGX', 'SGX', '新加坡证券交易所', '新交所', '证券', 'SG', 'SGD'),
(54, 'INBSE', 'INBSE', '孟买证券交易所', '孟买交易所', '证券', 'IN', 'INR'),
(55, 'NSE', 'NSE', '印度国家证券交易所', '印度国家证券交易所', '证券', 'IN', 'INR'),
(56, 'ASX', 'ASX', 'Australian Securities Exchange', '澳大利亚证券交易所', '证券', 'AU', 'AUD'),
(57, 'KRX', 'KRX', 'Korea Exchange', '韩国证券交易所', '证券', 'KR', 'KRW'),
(58, 'BMD', 'BMD', 'Bursa Malaysia Derivatives', '马来西亚衍生品交易所', '期货', 'MY', 'MYR'),
(100, 'FX', 'FX', '外汇市场', '外汇交易市场', '外汇', 'FX', 'CNY'),
(101, 'FX-CFD', 'FX-CFD', 'FX-CFD', '外汇-差价合约', '外汇', 'FX', 'USD'),
(120, 'BD', 'BD', 'Bond Market', '债券市场', '债券', 'N/A', 'USD'),
(121, 'BMS', 'BMS', 'Bond Market System', '债券市场系统', '债券', 'N/A', 'USD'),
(122, 'FD', 'FD', 'Fund Market', '基金市场', '基金', 'N/A', 'USD'),
(9001, 'FTSP', 'FTSP', 'Futu Structured Product Platform', '富途结构化产品平台', '结构化', 'HK', 'USD'),
(9002, 'FTSN', 'FTSN', 'Futu Structured Network', '富途结构化产品网络', '结构化', 'HK', 'USD'),
(9003, 'TFD', 'TFD', 'Test/Simulation Environment', '测试/仿真环境', '测试', 'N/A', 'USD'),
(10000, 'CC', 'CC', 'CryptoCoin Exchange', '加密货币交易所', '加密货币', 'GLOBAL', 'USD'),
(10001, 'CRYPTO', 'CRYPTO', 'Cryptocurrency Exchange', '加密货币交易所', '加密货币', 'GLOBAL', 'USD'),
(10002, 'CCBA', 'CCBA', 'Binance Exchange', '币安交易所', '加密货币', 'GLOBAL', 'USD'),
(10003, 'CCCOIN', 'CCCOIN', 'Coinbase', 'Coinbase', '加密货币', 'GLOBAL', 'USD'),
(10004, 'CCBGO', 'CCBGO', 'Bitget/BitGo', 'Bitget/BitGo', '加密货币', 'GLOBAL', 'USD'),
(10005, 'CCDDEX', 'CCDDEX', 'Decentralized Exchange (DEX)', '去中心化交易所', '加密货币', 'GLOBAL', 'USD'),
(10006, 'CCHSK', 'CCHSK', 'HashKey', 'HashKey', '加密货币', 'GLOBAL', 'USD'),
(10007, 'CCPT', 'CCPT', 'Poloniex (PT)', 'Poloniex', '加密货币', 'GLOBAL', 'USD');

COMMIT;
