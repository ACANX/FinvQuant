-- =====================================================================
-- FinvQuant PostgreSQL V100006：富途交易所映射初始数据（finv_futu_mapping_exchange）
--
-- 决策（ACANX 2026-08-05）：
--   - 数据来源：富途自选股 exchange 字段统计（30 类）；
--   - 字段转换：futu_exchange <- exchange；region <- 对应地区；abbr <- 地区简写
--     （与 finv_region 简写体系一致，— 用 N/A）；exchange_name <- 推测含义；
--     finv_exchange <- exchange（与 futu_exchange 同值，后续可对齐 finv_exchange 调整）；
--   - 依赖 V14（表结构）先执行；幂等插入（ON CONFLICT (futu_exchange) DO NOTHING）。
-- 迁移策略：与既有迁移一致，整个 V100006 在一个事务内执行；失败自动回滚。
-- =====================================================================

BEGIN;

INSERT INTO finv_futu_mapping_exchange
    (futu_exchange, region, abbr, exchange_name, finv_exchange)
VALUES
('ASX', '澳大利亚', 'AU', '澳大利亚证券交易所', 'ASX'),
('BD', '—', 'N/A', '债券（Bond）', 'BD'),
('BMD', '马来西亚', 'MY', '马来西亚衍生品交易所（Bursa Malaysia Derivatives）', 'BMD'),
('BMS', '—', 'N/A', '债券市场系统（Bond Market System）', 'BMS'),
('CA', '加拿大', 'CA', '加拿大证券市场（TSX/TSXV/CSE 统称）', 'CA'),
('CBOT', '美国', 'USA', '芝加哥期货交易所（谷物/国债期货）', 'CBOT'),
('CCCOIN', '全球', 'GLOBAL', '加密货币（Coinbase）', 'CCCOIN'),
('CCBGO', '全球', 'GLOBAL', '加密货币（Bitget/BitGo）', 'CCBGO'),
('CCDDEX', '全球', 'GLOBAL', '加密货币去中心化交易所（DEX）', 'CCDDEX'),
('CCHSK', '全球', 'GLOBAL', '加密货币（HashKey）', 'CCHSK'),
('CBOE', '美国', 'USA', '芝加哥期权交易所', 'CBOE'),
('CCPT', '全球', 'GLOBAL', '加密货币（PT/Poloniex）', 'CCPT'),
('CME', '美国', 'USA', '芝加哥商业交易所（外汇/利率/股指期货）', 'CME'),
('COMEX', '美国', 'USA', '纽约商品交易所（黄金/白银期货）', 'COMEX'),
('CRYPTO', '全球', 'GLOBAL', '加密货币交易所（Binance 等）', 'CRYPTO'),
('FD', '—', 'N/A', '基金（Fund）', 'FD'),
('FTSP', '—', 'N/A', '富途结构化产品平台', 'FTSP'),
('FTSN', '香港/新加坡', 'HK/SG', '富途结构化产品网络（Futu Structured Network）', 'FTSN'),
('FX', '全球', 'GLOBAL', '外汇市场（Forex）', 'FX'),
('HKFE', '香港', 'HK', '香港期货交易所', 'HKFE'),
('JP', '日本', 'JP', '日本东京证券交易所', 'JP'),
('KR', '韩国', 'KR', '韩国证券交易所（KRX）', 'KR'),
('NYMEX', '美国', 'USA', '纽约商品交易所（能源/金属期货）', 'NYMEX'),
('OSE', '日本', 'JP', '大阪交易所（日本商品/股指期货）', 'OSE'),
('SEHK', '香港', 'HK', '香港交易所（Hong Kong Stock Exchange）', 'SEHK'),
('SGX', '新加坡', 'SG', '新加坡交易所', 'SGX'),
('SSE', '中国', 'CN', '上海证券交易所', 'SSE'),
('SZSE', '中国', 'CN', '深圳证券交易所', 'SZSE'),
('TFD', '—', 'N/A', '测试/仿真环境', 'TFD'),
('US', '美国', 'USA', '美国证券市场（NYSE/NASDAQ/ARCA/BATS/OTC 统称）', 'US')

ON CONFLICT (futu_exchange) DO NOTHING;

COMMIT;
