-- =====================================================================
-- FinvQuant PostgreSQL V100008：富途 CS 市场映射初始数据（finv_futu_mapping_cs_market）
--
-- 决策（ACANX 2026-08-05）：
--   - 数据来源：富途自选股 cs_market 字段统计（29 类）；
--   - 字段转换：futu_cs_market <- cs_market；exchange_name <- 推测含义；
--   - finv_exchange_code：取 finv_exchange 表对应交易所的 exchange_code
--     （11=SH 上交所、12=SZ 深交所、13=BJ 北交所、21=HK 港交所、30=US 美国证券市场、
--      51=OSE 大阪、52=TSE 东京、53=SGX 新加坡、100=FX 外汇）；
--     无对应交易所（FTSN/FTSP/HKFE/美期聚合/BD/BMS/BMD/CA/ASX/KR/CRYPTO/FD/指数等）→ -1 缺省；
--   - 依赖 V16（exchange_name 列 + CHECK 放宽允许 -1）先执行；
--   - 幂等插入（ON CONFLICT (futu_cs_market) DO NOTHING）。
-- 迁移策略：与既有迁移一致，整个 V100008 在一个事务内执行；失败自动回滚。
-- =====================================================================

BEGIN;

INSERT INTO finv_futu_mapping_cs_market
    (futu_cs_market, exchange_name, finv_exchange_code)
VALUES
('0', '场外交易/结构化产品/加密货币（无明确交易所）', -1),
('1', '香港证券市场', 21),
('2', '美国证券市场', 30),
('3', '上海证券交易所（A 股）', 11),
('4', '深圳证券交易所（A 股）', 12),
('6', '香港期货交易所', -1),
('8', '美国期货交易所', -1),
('10', '上交所科创板（STAR Market）', 11),
('11', '外汇市场（Forex）', 100),
('12', '债券', -1),
('13', '新加坡期货', 53),
('14', '全球主要指数（FTSE/DAX/CAC 等）', -1),
('15', '新加坡交易所', 53),
('16', '日本大阪交易所（期货）', 51),
('17', '加密货币市场', -1),
('18', '国债收益率', -1),
('19', '基金（Fund）', -1),
('21', '加拿大 CSE 交易所', -1),
('22', '澳大利亚证券交易所', -1),
('23', '北京证券交易所（北交所，Beijing Stock Exchange）', 13),
('24', '日本（板块分类）', 52),
('25', '日本东京证券交易所', 52),
('27', '债券市场（Bond Market System）', -1),
('28', '马来西亚 Bursa Malaysia', -1),
('29', '加拿大 TSX 主板', -1),
('30', '加拿大 TSX Venture 创业板', -1),
('33', '加拿大 NEO 交易所', -1),
('36', '韩国证券交易所', -1),
('37', '其他', -1)

ON CONFLICT (futu_cs_market) DO NOTHING;

COMMIT;
