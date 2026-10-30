-- =====================================================================
-- FinvQuant PostgreSQL V100082：finv_security 增加 type_secu_sub / provider / remark
--
-- 决策（ACANX 2026-10-30）：
--   - 需求 10814～10816：Supabase 上游视图 finv_quote_collect_futu_view 的启用数据
--     需要批量导入 finv_security，导入前必须有承载「上游属性子类」「数据提供方」
--     「批次备注」三类信息的列，故先扩列再灌数；
--   - 三列统一 TEXT NOT NULL DEFAULT ''（空字符串表示「未维护 / 未标注 / 无备注」），
--     与 V100075 给 finv_market.market_symbol 的口径一致，避免存量 518 行出现
--     NULL 语义分裂，也避免 gmt_* 之外再出现可空列；
--   - 物理位置只能追加到表尾（PostgreSQL ADD COLUMN 限制）；语义位置：
--     type_secu_sub 紧跟 security_type（同属证券类型信息），provider 在 tz 之后、
--     flag_enable 之前（同属来源/生命周期信息），remark 在 flag_enable 之后；
--     结构体、文档与查询按语义顺序展示（与 V100019 / V100075 同一约定）；
--   - 本批次导入约定（由导入脚本写入，不在本迁移内落数据）：
--     flag_enable='0'、provider='FTMM'、
--     remark='20261030-SupabasePG-finv_quote_collect_futu_view批量导入'；
--     导入工具位于仓库外设计目录 FinvQuantDesign/Scripts/SecuSync；
--   - 暂不建索引：当前无按 provider / remark 的点查，避免无谓写放大；
--     后续若出现按批次清理的需求，另起迁移补部分索引（WHERE remark <> ''）。
-- 迁移策略：单事务、失败整体回滚；末尾做收敛校验（列存在性/类型/可空/默认值）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 增加三列（幂等：ADD COLUMN IF NOT EXISTS）
-- ---------------------------------------------------------------------
ALTER TABLE public.finv_security
    ADD COLUMN IF NOT EXISTS type_secu_sub TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS provider      TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS remark        TEXT NOT NULL DEFAULT '';

-- ---------------------------------------------------------------------
-- 2. 列注释（语义与取值约定，COMMENT 覆盖写天然幂等）
-- ---------------------------------------------------------------------
COMMENT ON COLUMN public.finv_security.type_secu_sub IS
    '证券属性子类（沿用上游 type_secu_sub，如 Stock.SZ / ETF.US；空字符串表示未维护）';

COMMENT ON COLUMN public.finv_security.provider IS
    '数据提供方（如 FTMM；空字符串表示未标注）';

COMMENT ON COLUMN public.finv_security.remark IS
    '备注（批次标记，如 20261030-SupabasePG-finv_quote_collect_futu_view批量导入；空字符串表示无备注）';

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_col  TEXT;
    v_type TEXT;
    v_null TEXT;
    v_def  TEXT;
    v_cols TEXT[] := ARRAY['type_secu_sub', 'provider', 'remark'];
BEGIN
    FOREACH v_col IN ARRAY v_cols LOOP
        SELECT c.data_type, c.is_nullable, c.column_default
          INTO v_type, v_null, v_def
          FROM information_schema.columns AS c
         WHERE c.table_schema = 'public'
           AND c.table_name   = 'finv_security'
           AND c.column_name  = v_col;

        IF v_type IS NULL THEN
            RAISE EXCEPTION 'V100082 收敛失败：finv_security.% 未创建', v_col;
        END IF;
        IF v_type <> 'text' THEN
            RAISE EXCEPTION 'V100082 收敛失败：finv_security.% 类型为 %，预期 text', v_col, v_type;
        END IF;
        IF v_null <> 'NO' THEN
            RAISE EXCEPTION 'V100082 收敛失败：finv_security.% 允许为空，预期 NOT NULL', v_col;
        END IF;
        -- 去掉默认值表达式里的引号后应正好是 ::text（即 DEFAULT ''）
        IF v_def IS NULL OR replace(v_def, chr(39), '') <> '::text' THEN
            RAISE EXCEPTION 'V100082 收敛失败：finv_security.% 默认值为 %，预期空字符串',
                v_col, COALESCE(v_def, '(NULL)');
        END IF;
    END LOOP;

    RAISE NOTICE 'V100082 完成：finv_security.type_secu_sub / provider / remark 就绪（TEXT NOT NULL DEFAULT ''''）';
END $$;

COMMIT;
