-- =====================================================================
-- V300217：回填期货采集配置的分类中文名（需求 11273/11274 数据补全）
--
-- 背景：V300216 建 cdeu.finv_quote_cdeu_future_sync_config 时分类名未知，
--   futures_type_name 留空，前端降级显示「分类 N」。线上首轮采集（2026-11-18）
--   跑通后，按日志里各分类的合约代码反推出语义，确认 8 个 futuresType 分别是：
--
--     1 = 指数期货     YMmain/ESmain/NQmain/RTYmain/VXmain（美股指+VIX）、
--                      NK225main/TOPIXmain/NKDmain（日经/东证）、HSImain/MHImain/
--                      HTImain/HHImain（恒指系）、TWmain/SGPmain/CNmain
--     2 = 能源化工     CLmain/MCLmain/QMmain（原油族）、BZmain（布伦特）、
--                      NGmain/MNGmain/QGmain（天然气）、HOmain（取暖油）、RBmain（汽油）
--     3 = 农产品       ZSmain 大豆 / ZCmain 玉米 / ZMmain 豆粕 / ZLmain 豆油 /
--                      ZWmain 小麦 / ZOmain 燕麦 / ZRmain 稻谷 / LEmain 活牛 /
--                      HEmain 瘦肉猪 / GFmain 育肥牛
--     4 = 金属期货     GCmain 黄金 / SImain·SILmain 白银 / HGmain 铜 / ALImain 铝 /
--                      PAmain 钯 / PLmain 铂 / GLDmain·GLDMmain / QOmain·QImain·QCmain
--     5 = 利率期货     ZTmain 2Y / ZFmain 5Y / ZNmain 10Y / ZBmain 30Y /
--                      TNmain·UBmain 超长期 / 2YY·5YY·10Y·30Y 收益率 / SR3main SOFR
--     6 = 汇率期货     6Emain 欧元 / 6Bmain 英镑 / 6Jmain 日元 / 6Smain 瑞郎 /
--                      6Amain 澳元 / 6Cmain 加元 / 6Nmain 纽元 / 6Mmain 墨西哥比索 /
--                      6Lmain 巴西雷亚尔 / 6Zmain 南非兰特
--     7 = 加密货币期货 BTCmain·MBTmain / ETHmain·METHmain / SOLmain·MSLmain /
--                      XRPmain·MXPmain
--     8 = 股票期货     SNVDAmain·SAAPLmain·SMSFTmain·SGOOGmain（S 前缀）、
--                      XAMZNmain·XNVDAmain·XOMmain（X 前缀）、TCHmain 腾讯、
--                      HSBmain·HLDmain·NWDmain（港股系）
--
-- 为什么用新迁移而不是改 V300216：V300216 已在环境上执行成功（表与种子已落库），
--   历史迁移不可回改；分类名属**数据**，用 V300xxx 数据段回填，可在任何环境重放。
--
-- 幂等：UPDATE ... FROM (VALUES ...) 无条件按常量覆盖（重复执行结果一致；
--   gmt_update 由触发器 trg_finv_quote_cdeu_future_sync_config_gmt_update 维护）。
--   末尾校验「8 行齐全且名称与常量一致」，不符整事务回滚。
-- =====================================================================

BEGIN;

UPDATE cdeu.finv_quote_cdeu_future_sync_config AS c
   SET futures_type_name = v.futures_type_name,
       remark            = v.remark
  FROM (VALUES
        ('1', '指数期货',
         'futuresType=1 指数期货：道指 YM / 纳指 NQ / 标普 ES / 罗素 RTY / VIX，以及日经225、东证 TOPIX、恒生 HSI、台湾 TWN、新加坡 SGP、富时中国 A50 等股指主连'),
        ('2', '能源化工',
         'futuresType=2 能源化工：原油 CL·MCL·QM、布伦特 BZ、天然气 NG·MNG·QG、取暖油 HO、汽油 RB 等能源主连'),
        ('3', '农产品',
         'futuresType=3 农产品：大豆 ZS / 玉米 ZC / 豆粕 ZM / 豆油 ZL / 小麦 ZW / 燕麦 ZO / 稻谷 ZR，以及活牛 LE / 瘦肉猪 HE / 育肥牛 GF'),
        ('4', '金属期货',
         'futuresType=4 金属期货：黄金 GC / 白银 SI·SIL / 铜 HG / 铝 ALI / 钯 PA / 铂 PL，含 GLD·GLDM 与 QO·QI·QC 迷你合约'),
        ('5', '利率期货',
         'futuresType=5 利率期货：2 年 ZT / 5 年 ZF / 10 年 ZN / 30 年 ZB、超长期 TN·UB、收益率 2YY·5YY·10Y·30Y、三个月 SOFR SR3'),
        ('6', '汇率期货',
         'futuresType=6 汇率期货：欧元 6E / 英镑 6B / 日元 6J / 瑞郎 6S / 澳元 6A / 加元 6C / 纽元 6N / 墨西哥比索 6M / 巴西雷亚尔 6L / 南非兰特 6Z'),
        ('7', '加密货币期货',
         'futuresType=7 加密货币期货：比特币 BTC·MBT / 以太坊 ETH·METH / SOL·MSL / XRP·MXP'),
        ('8', '股票期货',
         'futuresType=8 股票期货：美股名义个股期货（S 前缀 SNVDA·SAAPL·SMSFT·SGOOG、X 前缀 XAMZN·XNVDA·XOM）与港股个股期货（TCH 腾讯、HSB·HLD·NWD 等）')
       ) AS v(futures_type, futures_type_name, remark)
 WHERE c.futures_type = v.futures_type;

DO $$
DECLARE
    v_rows  int;
    v_names text;
    v_want  CONSTANT text := '1=指数期货,2=能源化工,3=农产品,4=金属期货,'
                             '5=利率期货,6=汇率期货,7=加密货币期货,8=股票期货';
    v_blank int;
BEGIN
    SELECT count(*) INTO v_rows FROM cdeu.finv_quote_cdeu_future_sync_config;
    IF v_rows <> 8 THEN
        RAISE EXCEPTION 'V300217 配置行数不符：期望 8，实际 %', v_rows;
    END IF;

    SELECT count(*) INTO v_blank
      FROM cdeu.finv_quote_cdeu_future_sync_config
     WHERE futures_type_name IS NULL OR btrim(futures_type_name) = '';
    IF v_blank <> 0 THEN
        RAISE EXCEPTION 'V300217 仍有 % 行分类名为空', v_blank;
    END IF;

    SELECT string_agg(futures_type || '=' || btrim(futures_type_name), ',' ORDER BY futures_type)
      INTO v_names
      FROM cdeu.finv_quote_cdeu_future_sync_config;
    IF v_names IS DISTINCT FROM v_want THEN
        RAISE EXCEPTION 'V300217 分类名不符：期望 %，实际 %', v_want, v_names;
    END IF;

    RAISE NOTICE 'V300217 校验通过：% 行分类名 = %', v_rows, v_names;
END $$;

COMMIT;
