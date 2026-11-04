-- =====================================================================
-- FinvQuant PostgreSQL V300073：finv_market 字典按「生产库口径」对齐（需求 10871）
--
-- 问题（2026-10-31 独立库 finv_migtest 全链验证发现）：
--   仓库种子 V300017/V300018 的 finv_market 用的是**旧编号体系**，而生产库现行为
--   另一套编号。两套体系同 flag 不同 code，导致全新库跑完全链后仍有 77 行证券的
--   market_code 找不到字典行（生产库为 0）。差异对照：
--     语义           仓库种子（旧）      生产库（现行）
--     香港期货       1150 HKFE-F       1250 HKFE-F
--     美股 OTC       1313 US-OTC       1331 US-OTC
--     道琼斯系列指数 1314 US-I2        1312 US-I2
--     美股 ARCA      1312 US-ARCA      1317 US-ARCA
--     美股 COMEX     1317 COMEX        1320 COMEX
--     美股 NYMEX     1316 NYMEX        1319 NYMEX
--     美股 CME       1318 CME          1321 CME
--     美股 CBOT      1319 CBOT         1322 CBOT
--     债券市场 BMS   1315 BMS          1340 BMS
--     纳指/纽交所    （无）            1315 US-NSDQ / 1316 US-NYSE
--     美股 BATS      （无）            1318 US-BATS
--
-- 口径：**以生产库为准**（用户 2026-10-31 明确）——V300062 的证券数据、
--       SecuSync/SecuMapping.json 的配置、后端代码都按生产库口径。
--
-- 步骤：
--   1. 旧编号行改号到生产库编号（按拓扑顺序腾空目标 code，避免主键冲突）；
--   2. 补齐生产库存在、种子缺失的市场行（按 code 与 flag 双条件幂等插入）；
--   3. 以生产库现值全量对齐 63 行的属性（名称/简码/基础货币/符号/启用标志等）；
--      其中 market_code=1110 的 market_name 在生产库为乱码 '??? A ?'，
--      本迁移按正确值「上交所 A 股」写入（属数据修正，非口径偏离）；
--   4. 证券侧兜底：把仍落在旧编号 1150/1313/1314 的证券重映射到 1250/1331/1312
--      （V300068/V300070 已处理绝大部分，此处只兜残留，生产库影响 0 行）；
--   5. 收敛校验：63 行字典逐行核验 + 证券不得存在字典外 market_code。
--
-- 幂等：改号条件含「源 code+flag 匹配且目标 code 不存在」；插入用 NOT EXISTS + ON CONFLICT；
--       属性对齐用整行元组 IS DISTINCT FROM。已在生产库（口径一致）上重跑影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 前置：V300069（symbol/base_currency）、V300070（字典补齐与市场码收口）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 旧编号 → 生产库编号（拓扑顺序；目标 code 不存在时才执行）
-- ---------------------------------------------------------------------
UPDATE public.finv_market
   SET market_code = 1322
 WHERE market_code = 1319 AND market_flag = 'CBOT'
   AND NOT EXISTS (SELECT 1 FROM public.finv_market m2 WHERE m2.market_code = 1322);
UPDATE public.finv_market
   SET market_code = 1321
 WHERE market_code = 1318 AND market_flag = 'CME'
   AND NOT EXISTS (SELECT 1 FROM public.finv_market m2 WHERE m2.market_code = 1321);
UPDATE public.finv_market
   SET market_code = 1320
 WHERE market_code = 1317 AND market_flag = 'COMEX'
   AND NOT EXISTS (SELECT 1 FROM public.finv_market m2 WHERE m2.market_code = 1320);
UPDATE public.finv_market
   SET market_code = 1319
 WHERE market_code = 1316 AND market_flag = 'NYMEX'
   AND NOT EXISTS (SELECT 1 FROM public.finv_market m2 WHERE m2.market_code = 1319);
UPDATE public.finv_market
   SET market_code = 1340
 WHERE market_code = 1315 AND market_flag = 'BMS'
   AND NOT EXISTS (SELECT 1 FROM public.finv_market m2 WHERE m2.market_code = 1340);
UPDATE public.finv_market
   SET market_code = 1317
 WHERE market_code = 1312 AND market_flag = 'US-ARCA'
   AND NOT EXISTS (SELECT 1 FROM public.finv_market m2 WHERE m2.market_code = 1317);
UPDATE public.finv_market
   SET market_code = 1312
 WHERE market_code = 1314 AND market_flag = 'US-I2'
   AND NOT EXISTS (SELECT 1 FROM public.finv_market m2 WHERE m2.market_code = 1312);
UPDATE public.finv_market
   SET market_code = 1331
 WHERE market_code = 1313 AND market_flag = 'US-OTC'
   AND NOT EXISTS (SELECT 1 FROM public.finv_market m2 WHERE m2.market_code = 1331);
UPDATE public.finv_market
   SET market_code = 1250
 WHERE market_code = 1150 AND market_flag = 'HKFE-F'
   AND NOT EXISTS (SELECT 1 FROM public.finv_market m2 WHERE m2.market_code = 1250);
-- ---------------------------------------------------------------------
-- 2. 补齐生产库存在、种子缺失的市场行（幂等；同 code 或同 flag 已存在则跳过）
-- ---------------------------------------------------------------------
INSERT INTO public.finv_market
    (market_code, market_flag, market_abbr, market_name, en_security_type,
     base_currency, exchange_code, market_symbol, flag_enable)
SELECT v.market_code, v.market_flag, v.market_abbr, v.market_name, v.en_security_type,
       v.base_currency, v.exchange_code, v.market_symbol, v.flag_enable
  FROM (VALUES
    (1110, 'SSE-A', 'SSE-A', '上交所 A 股', '1110', 'CNY', 0, 'SH', '1'),
    (1111, 'SSE-B', 'SSE-B', '上交所 B 股', '0', 'USD', 0, 'SH', '1'),
    (1112, 'SSE-STAR', 'SSE-STAR', '上交所科创板（STAR Market）', '1111', 'CNY', 0, 'SH', '1'),
    (1120, 'SZSE-A', 'SZSE-A', '深交所 A 股', '1120', 'CNY', 0, 'SZ', '1'),
    (1121, 'SZSE-NEXT', 'SZSE-NEXT', '深交所创业板（ChiNext）', '1121', 'CNY', 0, 'SZ', '1'),
    (1130, 'BSE-A', 'BSE-A', '北京证券交易所（北交所，Beijing Stock Exchange）', '1130', 'CNY', 0, 'BJ', '1'),
    (1140, 'CN-F', 'N/A', '中国商品期货', '1119', 'CNY', 0, 'CNF', '1'),
    (1201, 'HKEX-H', 'HKEX', '港股主板', '1210', 'HKD', 0, 'HK', '1'),
    (1202, 'HKEX-GEM', 'HKEX', '港股创业板（GEM）', '1211', 'HKD', 0, 'HK', '1'),
    (1203, 'HKEX_1212', 'HKEX', '港股其他品种', '1212', 'HKD', 0, 'HK', '1'),
    (1205, 'HKEX-HR', 'HKEX', '港股主板人民币柜台', '1210', 'CNY', 0, 'HK', '1'),
    (1250, 'HKFE-F', 'HKFE', '香港期货交易所', '1140', 'HKD', 0, 'HK', '1'),
    (1251, 'FX', 'FX', '外汇（Forex）', '1100', '', 0, 'CER', '1'),
    (1252, 'FX-CFD', 'FX', '外汇（Forex）-价差合约', '1100', 'USD', 0, 'CFD', '1'),
    (1253, 'FX-IDX', 'FX-IDX', '外汇（Forex）-货币指数', '1100', 'USD', 0, 'CIX', '1'),
    (1310, 'US-I', 'US', '美股指数', '1310', 'USD', 0, 'US', '1'),
    (1311, 'US-II', 'US', '美股行业指数', '1311', 'USD', 0, 'US', '1'),
    (1312, 'US-I2', 'US', '美股指数（道琼斯系列）', '1324', 'USD', 0, 'US', '1'),
    (1315, 'US-NSDQ', 'US', '美股 NSDAQ交易所', '1312', 'USD', 31, 'NSDQ', '1'),
    (1316, 'US-NYSE', 'US', '美股 NYSE交易所', '1312', 'USD', 0, 'NY', '1'),
    (1317, 'US-ARCA', 'US', '美股 ARCA 交易所', '1312', 'USD', 0, 'ARCA', '1'),
    (1318, 'US-BATS', 'US', '美股 BATS 交易所', '1312', 'USD', 0, 'BATS', '1'),
    (1319, 'NYMEX', 'NYMEX', '纽约商品交易所（能源/金属期货）', '1360', 'USD', 0, 'NYMEX', '1'),
    (1320, 'COMEX', 'COMEX', '纽约商品交易所（黄金/白银期货）', '1361', 'USD', 0, 'COMEX', '1'),
    (1321, 'CME', 'CME', '芝加哥商业交易所（外汇/利率期货）', '1370', 'USD', 0, 'CME', '1'),
    (1322, 'CBOT', 'CBOT', '芝加哥期货交易所（谷物期货）', '1380', 'USD', 0, 'CBOT', '1'),
    (1330, 'US-ADR', 'US', '美股 ADR', '1313', 'USD', 0, 'ADR', '1'),
    (1331, 'US-OTC', 'US', '美股 OTC 场外交易', '1313', 'USD', 0, 'USOTC', '1'),
    (1332, 'PINK', 'US', '美股 OTC Pink 场外交易', '1313', 'USD', 0, 'PINK', '1'),
    (1340, 'BMS', 'BMS', '债券市场', '1350', 'USD', 0, 'BMS', '1'),
    (1401, 'CA-NEO', 'CA', '加拿大 NEO 交易所', '1650', '', 0, 'NEO', '0'),
    (1402, 'CA-TSX', 'CA', '加拿大 TSX 主板', '2000', '', 0, 'TSX', '0'),
    (1403, 'CA-TSX-VENTURE', 'CA', '加拿大 TSX Venture 创业板', '2010', '', 0, 'TSXV', '0'),
    (1404, 'CA-CSE', 'CA', '加拿大 CSE 交易所', '2050', '', 0, 'CSE', '0'),
    (1501, 'JP_8300', 'JP', '日本（东京证券交易所）', '8300', 'JPY', 0, 'TK', '1'),
    (1507, 'KRX', 'KRX', '韩国证券交易所', '2100', '', 0, 'KRX', '1'),
    (1508, 'N/A_2710', 'N/A', '韩国 KOSPI 指数', '2710', 'KRW', 0, 'KR', '0'),
    (1509, 'N/A_2650', 'N/A', '马来西亚 Bursa 指数', '2650', '', 0, 'MA', '0'),
    (1511, 'SGX_1530', 'SGX', '新加坡交易所', '1530', 'SGD', 0, 'SGX', '1'),
    (1512, 'SGX-F', 'SGX-F', '新加坡期货', '1531', '', 0, '', '1'),
    (1513, 'ASX_2210', 'ASX', '澳大利亚证券交易所 - 衍生品（权证/期权）', '2210', '', 0, '', '0'),
    (1514, 'N/A_5850', 'N/A', 'ASX 权证/期权（无交易所标识）', '5850', '', 0, '', '0'),
    (1516, 'ASX_2110', 'ASX', '澳大利亚 - 其他上市品种', '2110', '', 0, '', '0'),
    (1521, 'N/A_2800', 'N/A', '印度 SENSEX 指数', '2800', 'INR', 0, 'IN', '0'),
    (1600, 'N/A_1700', 'N/A', '欧洲期货（ICE 等）', '1700', '', 0, '', '1'),
    (2001, 'BD_1540', 'BD', '债券', '1540', 'HKD', 0, 'BD', '0'),
    (2002, 'BD_1560', 'BD', '债券', '1560', '', 0, '', '0'),
    (2003, 'BD_4600', 'BD', '国债收益率', '4600', '', 0, 'BDY', '0'),
    (2004, 'BD_9000', 'BD', '债券', '9000', '', 0, '', '0'),
    (3001, 'FD_5600', 'FD', '基金（主）', '5600', 'USD', 0, 'FD', '0'),
    (3002, 'FD_5610', 'FD', '基金（子分类）', '5610', '', 0, '', '0'),
    (3003, 'FD_5630', 'FD', '基金（子分类）', '5630', '', 0, '', '0'),
    (8000, 'N/A_8000', 'N/A', '板块/行业分类', '8000', '', 0, 'SECTOR', '0'),
    (9145, 'FTSN_1450', 'FTSN', '结构性产品 - 香港（FCN=固定息票票据）', '1450', '', 0, '', '0'),
    (9146, 'FTSN_1451', 'FTSN', '结构性产品 - 新加坡', '1451', '', 0, '', '0'),
    (10001, 'CRYPTO_10001', 'CRYPTO', '加密货币 - Binance 等交易所', '10001', 'USD', 0, 'FTC', '0'),
    (10002, 'N/A_3740', 'N/A', '加密货币交易对（Binance）', '3740', '', 0, 'FBIN', '1'),
    (10003, 'N/A_3790', 'N/A', '加密货币（Coinbase）', '3790', '', 0, 'FCB', '1'),
    (10011, 'N/A_4000', 'N/A', '加密货币现货（SGD/USD 计价）', '4000', 'USD', 0, 'FTC', '1'),
    (10012, 'N/A_4140', 'N/A', '加密货币现货（USDT 计价）', '4140', '', 0, '', '1'),
    (10013, 'N/A_4120', 'N/A', '加密货币现货（通用）', '4120', 'USD', 0, 'FTC', '1'),
    (10014, 'N/A_4130', 'N/A', '加密货币现货（通用）', '4130', 'USD', 0, 'FTC', '1'),
    (10015, 'N/A_4100', 'N/A', '加密货币现货（通用）', '4100', '', 0, '', '1')
  ) AS v(market_code, market_flag, market_abbr, market_name, en_security_type,
         base_currency, exchange_code, market_symbol, flag_enable)
 WHERE NOT EXISTS (SELECT 1 FROM public.finv_market m WHERE m.market_code = v.market_code)
   AND NOT EXISTS (SELECT 1 FROM public.finv_market m WHERE m.market_flag = v.market_flag)
ON CONFLICT (market_code) DO NOTHING;

-- ---------------------------------------------------------------------
-- 3. 属性全量对齐（以生产库现值为准；整行元组比较，幂等）
-- ---------------------------------------------------------------------
UPDATE public.finv_market AS m
   SET market_flag       = v.market_flag,
       market_abbr       = v.market_abbr,
       market_name       = v.market_name,
       en_security_type  = v.en_security_type,
       base_currency     = v.base_currency,
       exchange_code     = v.exchange_code,
       market_symbol     = v.market_symbol,
       flag_enable       = v.flag_enable
  FROM (VALUES
    (1110, 'SSE-A', 'SSE-A', '上交所 A 股', '1110', 'CNY', 0, 'SH', '1'),
    (1111, 'SSE-B', 'SSE-B', '上交所 B 股', '0', 'USD', 0, 'SH', '1'),
    (1112, 'SSE-STAR', 'SSE-STAR', '上交所科创板（STAR Market）', '1111', 'CNY', 0, 'SH', '1'),
    (1120, 'SZSE-A', 'SZSE-A', '深交所 A 股', '1120', 'CNY', 0, 'SZ', '1'),
    (1121, 'SZSE-NEXT', 'SZSE-NEXT', '深交所创业板（ChiNext）', '1121', 'CNY', 0, 'SZ', '1'),
    (1130, 'BSE-A', 'BSE-A', '北京证券交易所（北交所，Beijing Stock Exchange）', '1130', 'CNY', 0, 'BJ', '1'),
    (1140, 'CN-F', 'N/A', '中国商品期货', '1119', 'CNY', 0, 'CNF', '1'),
    (1201, 'HKEX-H', 'HKEX', '港股主板', '1210', 'HKD', 0, 'HK', '1'),
    (1202, 'HKEX-GEM', 'HKEX', '港股创业板（GEM）', '1211', 'HKD', 0, 'HK', '1'),
    (1203, 'HKEX_1212', 'HKEX', '港股其他品种', '1212', 'HKD', 0, 'HK', '1'),
    (1205, 'HKEX-HR', 'HKEX', '港股主板人民币柜台', '1210', 'CNY', 0, 'HK', '1'),
    (1250, 'HKFE-F', 'HKFE', '香港期货交易所', '1140', 'HKD', 0, 'HK', '1'),
    (1251, 'FX', 'FX', '外汇（Forex）', '1100', '', 0, 'CER', '1'),
    (1252, 'FX-CFD', 'FX', '外汇（Forex）-价差合约', '1100', 'USD', 0, 'CFD', '1'),
    (1253, 'FX-IDX', 'FX-IDX', '外汇（Forex）-货币指数', '1100', 'USD', 0, 'CIX', '1'),
    (1310, 'US-I', 'US', '美股指数', '1310', 'USD', 0, 'US', '1'),
    (1311, 'US-II', 'US', '美股行业指数', '1311', 'USD', 0, 'US', '1'),
    (1312, 'US-I2', 'US', '美股指数（道琼斯系列）', '1324', 'USD', 0, 'US', '1'),
    (1315, 'US-NSDQ', 'US', '美股 NSDAQ交易所', '1312', 'USD', 31, 'NSDQ', '1'),
    (1316, 'US-NYSE', 'US', '美股 NYSE交易所', '1312', 'USD', 0, 'NY', '1'),
    (1317, 'US-ARCA', 'US', '美股 ARCA 交易所', '1312', 'USD', 0, 'ARCA', '1'),
    (1318, 'US-BATS', 'US', '美股 BATS 交易所', '1312', 'USD', 0, 'BATS', '1'),
    (1319, 'NYMEX', 'NYMEX', '纽约商品交易所（能源/金属期货）', '1360', 'USD', 0, 'NYMEX', '1'),
    (1320, 'COMEX', 'COMEX', '纽约商品交易所（黄金/白银期货）', '1361', 'USD', 0, 'COMEX', '1'),
    (1321, 'CME', 'CME', '芝加哥商业交易所（外汇/利率期货）', '1370', 'USD', 0, 'CME', '1'),
    (1322, 'CBOT', 'CBOT', '芝加哥期货交易所（谷物期货）', '1380', 'USD', 0, 'CBOT', '1'),
    (1330, 'US-ADR', 'US', '美股 ADR', '1313', 'USD', 0, 'ADR', '1'),
    (1331, 'US-OTC', 'US', '美股 OTC 场外交易', '1313', 'USD', 0, 'USOTC', '1'),
    (1332, 'PINK', 'US', '美股 OTC Pink 场外交易', '1313', 'USD', 0, 'PINK', '1'),
    (1340, 'BMS', 'BMS', '债券市场', '1350', 'USD', 0, 'BMS', '1'),
    (1401, 'CA-NEO', 'CA', '加拿大 NEO 交易所', '1650', '', 0, 'NEO', '0'),
    (1402, 'CA-TSX', 'CA', '加拿大 TSX 主板', '2000', '', 0, 'TSX', '0'),
    (1403, 'CA-TSX-VENTURE', 'CA', '加拿大 TSX Venture 创业板', '2010', '', 0, 'TSXV', '0'),
    (1404, 'CA-CSE', 'CA', '加拿大 CSE 交易所', '2050', '', 0, 'CSE', '0'),
    (1501, 'JP_8300', 'JP', '日本（东京证券交易所）', '8300', 'JPY', 0, 'TK', '1'),
    (1507, 'KRX', 'KRX', '韩国证券交易所', '2100', '', 0, 'KRX', '1'),
    (1508, 'N/A_2710', 'N/A', '韩国 KOSPI 指数', '2710', 'KRW', 0, 'KR', '0'),
    (1509, 'N/A_2650', 'N/A', '马来西亚 Bursa 指数', '2650', '', 0, 'MA', '0'),
    (1511, 'SGX_1530', 'SGX', '新加坡交易所', '1530', 'SGD', 0, 'SGX', '1'),
    (1512, 'SGX-F', 'SGX-F', '新加坡期货', '1531', '', 0, '', '1'),
    (1513, 'ASX_2210', 'ASX', '澳大利亚证券交易所 - 衍生品（权证/期权）', '2210', '', 0, '', '0'),
    (1514, 'N/A_5850', 'N/A', 'ASX 权证/期权（无交易所标识）', '5850', '', 0, '', '0'),
    (1516, 'ASX_2110', 'ASX', '澳大利亚 - 其他上市品种', '2110', '', 0, '', '0'),
    (1521, 'N/A_2800', 'N/A', '印度 SENSEX 指数', '2800', 'INR', 0, 'IN', '0'),
    (1600, 'N/A_1700', 'N/A', '欧洲期货（ICE 等）', '1700', '', 0, '', '1'),
    (2001, 'BD_1540', 'BD', '债券', '1540', 'HKD', 0, 'BD', '0'),
    (2002, 'BD_1560', 'BD', '债券', '1560', '', 0, '', '0'),
    (2003, 'BD_4600', 'BD', '国债收益率', '4600', '', 0, 'BDY', '0'),
    (2004, 'BD_9000', 'BD', '债券', '9000', '', 0, '', '0'),
    (3001, 'FD_5600', 'FD', '基金（主）', '5600', 'USD', 0, 'FD', '0'),
    (3002, 'FD_5610', 'FD', '基金（子分类）', '5610', '', 0, '', '0'),
    (3003, 'FD_5630', 'FD', '基金（子分类）', '5630', '', 0, '', '0'),
    (8000, 'N/A_8000', 'N/A', '板块/行业分类', '8000', '', 0, 'SECTOR', '0'),
    (9145, 'FTSN_1450', 'FTSN', '结构性产品 - 香港（FCN=固定息票票据）', '1450', '', 0, '', '0'),
    (9146, 'FTSN_1451', 'FTSN', '结构性产品 - 新加坡', '1451', '', 0, '', '0'),
    (10001, 'CRYPTO_10001', 'CRYPTO', '加密货币 - Binance 等交易所', '10001', 'USD', 0, 'FTC', '0'),
    (10002, 'N/A_3740', 'N/A', '加密货币交易对（Binance）', '3740', '', 0, 'FBIN', '1'),
    (10003, 'N/A_3790', 'N/A', '加密货币（Coinbase）', '3790', '', 0, 'FCB', '1'),
    (10011, 'N/A_4000', 'N/A', '加密货币现货（SGD/USD 计价）', '4000', 'USD', 0, 'FTC', '1'),
    (10012, 'N/A_4140', 'N/A', '加密货币现货（USDT 计价）', '4140', '', 0, '', '1'),
    (10013, 'N/A_4120', 'N/A', '加密货币现货（通用）', '4120', 'USD', 0, 'FTC', '1'),
    (10014, 'N/A_4130', 'N/A', '加密货币现货（通用）', '4130', 'USD', 0, 'FTC', '1'),
    (10015, 'N/A_4100', 'N/A', '加密货币现货（通用）', '4100', '', 0, '', '1')
  ) AS v(market_code, market_flag, market_abbr, market_name, en_security_type,
         base_currency, exchange_code, market_symbol, flag_enable)
 WHERE m.market_code = v.market_code
   AND (m.market_flag, m.market_abbr, m.market_name, m.en_security_type,
        m.base_currency, m.exchange_code, m.market_symbol, m.flag_enable)
       IS DISTINCT FROM
       (v.market_flag, v.market_abbr, v.market_name, v.en_security_type,
        v.base_currency, v.exchange_code, v.market_symbol, v.flag_enable);

-- ---------------------------------------------------------------------
-- 4. 证券侧兜底：旧编号残留重映射（生产库影响 0 行）
-- ---------------------------------------------------------------------
UPDATE public.finv_security SET market_code = 1250 WHERE market_code = 1150;
UPDATE public.finv_security SET market_code = 1331 WHERE market_code = 1313;
UPDATE public.finv_security SET market_code = 1312 WHERE market_code = 1314;

-- 4.2 另一批历史种子（provider/remark 为空、flag_enable='0'）的 market_code 回填：
--     它们不在 V300068 的 470 条清单内，全新库上仍为默认 0。取值以**生产库同 usc 现值**
--     为准；其中 SH000001（上证指数）在生产库已被治理改名为 `SH0001`（无同名 usc），
--     按「上交所指数 → 1110」定值，与生产库同语义证券一致。
UPDATE public.finv_security AS s
   SET market_code = v_market.market_code
  FROM (VALUES
    ('000001', 1120),
    ('000034', 1120),
    ('000050', 1120),
    ('000063', 1120),
    ('000100', 1120),
    ('00020', 1201),
    ('000333', 1120),
    ('000680', 1120),
    ('000858', 1120),
    ('000888', 1110),
    ('000985', 1120),
    ('00300', 1201),
    ('00388', 1201),
    ('00700', 1201),
    ('00762', 1201),
    ('01810', 1201),
    ('03908', 1201),
    ('09633', 1201),
    ('09868', 1201),
    ('159562', 1120),
    ('600519', 1110),
    ('600999', 1110),
    ('688027', 1112),
    ('688088', 1112),
    ('688111', 1112),
    ('688158', 1112),
    ('688318', 1112),
    ('688396', 1112),
    ('81810', 1205),
    ('920185', 1130),
    ('920418', 1130),
    ('920493', 1130),
    ('920599', 1130),
    ('920799', 1130),
    ('920808', 1130),
    ('GCMain', 1320),
    ('MGCMain', 1320),
    ('NVDA', 1315),
    ('SH000001', 1110),
    ('XAUUSD', 1252),
    ('XIACY', 1332)
  ) AS v_market(usc, market_code)
 WHERE s.usc = v_market.usc
   AND s.market_code = 0;

-- ---------------------------------------------------------------------
-- 5. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_missing int;
    v_orphan  int;
    v_total   int;
BEGIN
    -- 5.1 清单内 63 行必须存在且 flag 相符
    SELECT count(*) INTO v_missing
      FROM (VALUES
        (1110, 'SSE-A'),
        (1111, 'SSE-B'),
        (1112, 'SSE-STAR'),
        (1120, 'SZSE-A'),
        (1121, 'SZSE-NEXT'),
        (1130, 'BSE-A'),
        (1140, 'CN-F'),
        (1201, 'HKEX-H'),
        (1202, 'HKEX-GEM'),
        (1203, 'HKEX_1212'),
        (1205, 'HKEX-HR'),
        (1250, 'HKFE-F'),
        (1251, 'FX'),
        (1252, 'FX-CFD'),
        (1253, 'FX-IDX'),
        (1310, 'US-I'),
        (1311, 'US-II'),
        (1312, 'US-I2'),
        (1315, 'US-NSDQ'),
        (1316, 'US-NYSE'),
        (1317, 'US-ARCA'),
        (1318, 'US-BATS'),
        (1319, 'NYMEX'),
        (1320, 'COMEX'),
        (1321, 'CME'),
        (1322, 'CBOT'),
        (1330, 'US-ADR'),
        (1331, 'US-OTC'),
        (1332, 'PINK'),
        (1340, 'BMS'),
        (1401, 'CA-NEO'),
        (1402, 'CA-TSX'),
        (1403, 'CA-TSX-VENTURE'),
        (1404, 'CA-CSE'),
        (1501, 'JP_8300'),
        (1507, 'KRX'),
        (1508, 'N/A_2710'),
        (1509, 'N/A_2650'),
        (1511, 'SGX_1530'),
        (1512, 'SGX-F'),
        (1513, 'ASX_2210'),
        (1514, 'N/A_5850'),
        (1516, 'ASX_2110'),
        (1521, 'N/A_2800'),
        (1600, 'N/A_1700'),
        (2001, 'BD_1540'),
        (2002, 'BD_1560'),
        (2003, 'BD_4600'),
        (2004, 'BD_9000'),
        (3001, 'FD_5600'),
        (3002, 'FD_5610'),
        (3003, 'FD_5630'),
        (8000, 'N/A_8000'),
        (9145, 'FTSN_1450'),
        (9146, 'FTSN_1451'),
        (10001, 'CRYPTO_10001'),
        (10002, 'N/A_3740'),
        (10003, 'N/A_3790'),
        (10011, 'N/A_4000'),
        (10012, 'N/A_4140'),
        (10013, 'N/A_4120'),
        (10014, 'N/A_4130'),
        (10015, 'N/A_4100')
      ) AS v(market_code, market_flag)
      LEFT JOIN public.finv_market m ON m.market_code = v.market_code
     WHERE m.market_code IS NULL OR m.market_flag <> v.market_flag;
    IF v_missing <> 0 THEN
        RAISE EXCEPTION 'V300073 收敛失败：% 个市场缺失或 flag 不符', v_missing;
    END IF;

    -- 5.2 证券不得存在字典外 market_code
    SELECT count(*) INTO v_orphan
      FROM public.finv_security s
      LEFT JOIN public.finv_market m ON m.market_code = s.market_code
     WHERE m.market_code IS NULL;
    IF v_orphan <> 0 THEN
        RAISE EXCEPTION 'V300073 收敛失败：仍有 % 行证券的 market_code 不在字典内', v_orphan;
    END IF;

    SELECT count(*) INTO v_total FROM public.finv_market;
    RAISE NOTICE 'V300073 完成：finv_market 已按生产库口径对齐（共 % 行），证券字典外 = 0', v_total;
END $$;

COMMIT;
