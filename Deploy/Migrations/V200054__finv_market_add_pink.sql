-- =====================================================================
-- FinvQuant PostgreSQL V100054：新增美股 OTC Pink 市场 1332（需求 10750）
--
-- 背景：现有美股场外相关市场只有
--   1330 US-ADR（美股 ADR，en_security_type=1313，market_symbol=ADR）
--   1331 US-OTC（美股 OTC 场外交易，en_security_type=1313，market_symbol=USOTC）
-- PINK 指 OTC Markets Group 的 Pink 层级（Pink / Pink Limited / Pink No Information），
-- 与泛化的 US-OTC 不是同一口径，需要独立登记以便 metadata 刷新与导出目录按市场区分。
--
-- 取值依据（逐字段说明，避免凭空编造）：
--   · market_code 1332        —— 紧随 1330/1331 的 US 场外编号段（1330→1331→1332 连续）。
--   · market_flag 'PINK'      —— 用户指定；受 uq_finv_market_flag 唯一约束，不冲突。
--   · market_abbr 'US'        —— 与 1310/1315/1330/1331 等全部美股市场一致。
--   · market_name '美股 OTC Pink 场外交易' —— 参照 1331「美股 OTC 场外交易」的命名口径，
--                              指明其归属 OTC Pink 层级。
--   · en_security_type '1313' —— 与 1330/1331 同属美股场外/存托凭证类型。
--   · base_currency 'USD'     —— 美股统一美元计价。
--   · flag_enable '1'         —— 1330/1331 均为启用；新市场需可被 metadata 刷选取用。
--   · exchange_code 0         —— 与 1330/1331 一致（该市场无独立交易所编号）。
--   · market_symbol 'PINK'    —— dedup.finv_dedup_usc.market 以该列取值，
--                              与 market_flag 同一标识便于追溯。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置校验：编号与标识均未被占用
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_code bigint;
    v_flag bigint;
BEGIN
    SELECT count(*) INTO v_code FROM public.finv_market WHERE market_code = 1332;
    SELECT count(*) INTO v_flag FROM public.finv_market WHERE market_flag = 'PINK';

    IF v_code > 0 THEN
        RAISE NOTICE 'V100054 提示：market_code=1332 已存在，本迁移将跳过插入（不覆盖现有值）';
    END IF;
    IF v_flag > 0 THEN
        RAISE EXCEPTION 'V100054 前置校验失败：market_flag=PINK 已被其它 market_code 占用';
    END IF;
    RAISE NOTICE 'V100054 前置校验：1332/PINK 未被占用，准备新增';
END $$;

-- ---------------------------------------------------------------------
-- 2. 插入市场记录（幂等，不覆盖运维已调整的定义）
-- ---------------------------------------------------------------------
INSERT INTO public.finv_market
    (market_code, market_flag, market_abbr, market_name, en_security_type,
     base_currency, flag_enable, exchange_code, market_symbol)
VALUES
    (1332, 'PINK', 'US', '美股 OTC Pink 场外交易', '1313',
     'USD', '1', 0, 'PINK')
ON CONFLICT (market_code) DO NOTHING;

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_row  public.finv_market%ROWTYPE;
    v_flag bigint;
BEGIN
    SELECT * INTO v_row FROM public.finv_market WHERE market_code = 1332;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'V100054 收敛失败：1332 记录未插入';
    END IF;

    -- flag 唯一：确保 PINK 只挂在 1332 上
    SELECT count(*) INTO v_flag FROM public.finv_market WHERE market_flag = 'PINK';
    IF v_flag <> 1 THEN
        RAISE EXCEPTION 'V100054 收敛失败：market_flag=PINK 有 % 条记录，应为 1', v_flag;
    END IF;

    IF v_row.market_symbol <> 'PINK' THEN
        RAISE EXCEPTION 'V100054 收敛失败：market_symbol 应为 PINK，实际 %', v_row.market_symbol;
    END IF;
    IF v_row.flag_enable <> '1' THEN
        RAISE EXCEPTION 'V100054 收敛失败：flag_enable 应为 1（启用），实际 %', v_row.flag_enable;
    END IF;
    IF v_row.base_currency <> 'USD' THEN
        RAISE EXCEPTION 'V100054 收敛失败：base_currency 应为 USD，实际 %', v_row.base_currency;
    END IF;

    RAISE NOTICE 'V100054 完成：1332「%」已登记（en_security_type=% market_symbol=% 启用=%）',
        v_row.market_name, v_row.en_security_type, v_row.market_symbol, v_row.flag_enable;
END $$;

COMMIT;
