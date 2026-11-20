-- =====================================================================
-- V300109：USC 映射规则的启用状态与 GDRnext 规则发放（需求 10943）
--
-- 背景：
--   · SH000888 / SH000903 / SH000922 / SH000980 四条规则由 V300103 以
--     flag_enable='0' 入库，之后只在 LocalDocker 通过接口手工启用
--     （2026-11-02 16:51），启用状态没有落到任何迁移脚本——换库上生产后
--     这四条又回到停用，前端「过时USC映射到新USC」列表里查不到生效规则。
--   · 上游富途把黄金下月期货的 symbol 写成 GDRnext
--     （futu.finv_futu_mapping_security 与 futu.finv_futu_quote_collect 均为
--     ('71009930', 'GDRnext', 'GDRNext')）。若该 symbol 被当 usc 写进
--     cdeu.finv_quote_secu_kline_min，需要映射到大写形态 GDRNext；
--     规则表里此前没有这条记录。
--
-- 做法：
--   1) 已存在的规则：把 usc_new / match_mode / priority 校正到目标形态并置
--      flag_enable='1'；note 为空时补上依据（已有依据不覆盖）。
--      同一 usc_old 若有多行，只取「已启用优先、idn 最小」的一行更新，
--      避免撞部分唯一索引 uq_finv_quote_cdeu_usc_patch_rule_usc_old
--      （该索引只约束 flag_enable='1' 的行）。
--   2) 不存在的规则：补插（flag_enable='1'）。
--   3) 目标 usc 登记兜底（口径同 V300104）：SH 系列从旧 usc 复制元数据；
--      GDRnext 的旧形态尚未登记、无从复制，用与登记表现值一致的显式口径兜底。
--      只补不覆盖（ON CONFLICT DO NOTHING）。
--   4) 校验：5 条规则必须生效、5 个目标 usc 必须已登记，否则 RAISE EXCEPTION 回滚。
--
-- 幂等：重复执行时更新 0 行、新增 0 行（差异判断 + NOT EXISTS + DO NOTHING）。
--
-- 【全新库容忍（需求 11022）】目标 usc 登记依赖行情主数据（cdeu.finv_quote_cdeu_usc）
--   里已存在 SH000888 等源 usc，而全新空库没有任何证券数据（主数据靠导入而非迁移）。
--   因此 §4 校验改为：源 usc 一条都没有 → 只打 NOTICE（全新库，待导入后再校验）；
--   源 usc 存在但目标未登记（复制失败/被删）→ 仍然 RAISE EXCEPTION 回滚。
-- 本迁移只改规则与登记表，不迁移任何行情；行情迁移仍由 PATCH_OUTDATED_USC 执行。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_upd     bigint := 0;
    v_ins     bigint := 0;
    v_enabled bigint;
    v_missing text;
    v_sources bigint;
BEGIN
    -- -----------------------------------------------------------------
    -- 1. 已存在的规则：启用 + 校正
    -- -----------------------------------------------------------------
    UPDATE cdeu.finv_quote_cdeu_usc_patch_rule r
       SET usc_new     = p.usc_new,
           match_mode  = 'exact',
           flag_enable = '1',
           priority    = 100,
           note        = CASE WHEN btrim(r.note) = '' THEN p.note ELSE r.note END,
           gmt_update  = now()
      FROM (
          SELECT DISTINCT ON (r2.usc_old) r2.idn, r2.usc_old, w.usc_new, w.note
            FROM cdeu.finv_quote_cdeu_usc_patch_rule r2
            JOIN (VALUES
                ('SH000888', 'SH0888',  '需求 10943：SH 指数旧形态 SH000888 映射到 SH0888（依据需求 10931 附录 A.3.1）'),
                ('SH000903', 'SH0903',  '需求 10943：SH 指数旧形态 SH000903 映射到 SH0903（依据需求 10931 附录 A.3.1）'),
                ('SH000922', 'SH0922',  '需求 10943：SH 指数旧形态 SH000922 映射到 SH0922（依据需求 10931 附录 A.3.1）'),
                ('SH000980', 'SH0980',  '需求 10943：SH 指数旧形态 SH000980 映射到 SH0980（依据需求 10931 附录 A.3.1）'),
                ('GDRnext',  'GDRNext', '需求 10943：富途 symbol 小写 n 的 GDRnext 映射到大写形态 GDRNext')
            ) AS w(usc_old, usc_new, note) ON w.usc_old = r2.usc_old
           ORDER BY r2.usc_old, (r2.flag_enable = '1') DESC, r2.idn
      ) p
     WHERE r.idn = p.idn
       AND (r.usc_new IS DISTINCT FROM p.usc_new
            OR r.match_mode IS DISTINCT FROM 'exact'
            OR r.flag_enable IS DISTINCT FROM '1'
            OR r.priority IS DISTINCT FROM 100
            OR (btrim(r.note) = '' AND btrim(p.note) <> ''));
    GET DIAGNOSTICS v_upd = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 2. 不存在的规则：补插
    -- -----------------------------------------------------------------
    INSERT INTO cdeu.finv_quote_cdeu_usc_patch_rule
        (usc_old, usc_new, match_mode, flag_enable, priority, note)
    SELECT w.usc_old, w.usc_new, 'exact', '1', 100, w.note
      FROM (VALUES
                ('SH000888', 'SH0888',  '需求 10943：SH 指数旧形态 SH000888 映射到 SH0888（依据需求 10931 附录 A.3.1）'),
                ('SH000903', 'SH0903',  '需求 10943：SH 指数旧形态 SH000903 映射到 SH0903（依据需求 10931 附录 A.3.1）'),
                ('SH000922', 'SH0922',  '需求 10943：SH 指数旧形态 SH000922 映射到 SH0922（依据需求 10931 附录 A.3.1）'),
                ('SH000980', 'SH0980',  '需求 10943：SH 指数旧形态 SH000980 映射到 SH0980（依据需求 10931 附录 A.3.1）'),
                ('GDRnext',  'GDRNext', '需求 10943：富途 symbol 小写 n 的 GDRnext 映射到大写形态 GDRNext')
           ) AS w(usc_old, usc_new, note)
     WHERE NOT EXISTS (
         SELECT 1 FROM cdeu.finv_quote_cdeu_usc_patch_rule r WHERE r.usc_old = w.usc_old);
    GET DIAGNOSTICS v_ins = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 3. 目标 usc 登记兜底（只补不覆盖）
    -- -----------------------------------------------------------------
    -- 3.1 SH 系列：从旧 usc 复制元数据（口径同 V300104）
    INSERT INTO cdeu.finv_quote_cdeu_usc
        (usc, category, day_expect_rows, date_list, date_delist,
         flag_auto_discover, region, market, provider, name, timezone, currency)
    SELECT DISTINCT ON (p.usc_new)
           p.usc_new, u.category, u.day_expect_rows, u.date_list, u.date_delist,
           false, u.region, u.market, u.provider, u.name, u.timezone, u.currency
      FROM cdeu.finv_quote_cdeu_usc_patch_rule p
      JOIN cdeu.finv_quote_cdeu_usc u ON u.usc = p.usc_old
     WHERE p.usc_old IN ('SH000888', 'SH000903', 'SH000922', 'SH000980', 'GDRnext')
       AND NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_usc n WHERE n.usc = p.usc_new)
     ORDER BY p.usc_new, p.priority
    ON CONFLICT (usc) DO NOTHING;

    -- 3.2 GDRNext：旧形态 GDRnext 没有登记行，无从复制；
    --     按登记表现值口径显式兜底（已存在时不覆盖，day_expect_rows 留空由
    --     模板/日增量任务补齐，不在此处编造预期行数）。
    INSERT INTO cdeu.finv_quote_cdeu_usc
        (usc, category, date_list, flag_auto_discover, region, market, provider,
         name, timezone, currency)
    VALUES ('GDRNext', 'Futures', DATE '2000-01-01', false, 'HK', 'HK', 'FTMM',
            '人民币黄金下月期货', 'Asia/Hong_Kong', 'HKD')
    ON CONFLICT (usc) DO NOTHING;

    -- -----------------------------------------------------------------
    -- 4. 校验：规则生效 + 目标 usc 已登记
    -- -----------------------------------------------------------------
    SELECT count(*) INTO v_enabled
      FROM cdeu.finv_quote_cdeu_usc_patch_rule
     WHERE flag_enable = '1'
       AND usc_old IN ('SH000888', 'SH000903', 'SH000922', 'SH000980', 'GDRnext');
    IF v_enabled <> 5 THEN
        RAISE EXCEPTION '生效规则应为 5 条（SH000888/SH000903/SH000922/SH000980/GDRnext），实际 %', v_enabled;
    END IF;

    SELECT string_agg(w.usc_new, ', ' ORDER BY w.usc_new) INTO v_missing
      FROM (VALUES ('SH0888'), ('SH0903'), ('SH0922'), ('SH0980'), ('GDRNext')) AS w(usc_new)
     WHERE NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_usc u WHERE u.usc = w.usc_new);
    IF v_missing IS NOT NULL THEN
        -- 源 usc 是否存在决定这是「全新库缺数据」还是「复制失效」
        SELECT count(*) INTO v_sources
          FROM cdeu.finv_quote_cdeu_usc
         WHERE usc IN ('SH000888', 'SH000903', 'SH000922', 'SH000980');
        IF v_sources = 0 THEN
            RAISE NOTICE '目标 usc 未登记（%）：主数据尚未导入，全新库跳过该校验（需求 11022）', v_missing;
        ELSE
            RAISE EXCEPTION '目标 usc 未登记：%', v_missing;
        END IF;
    END IF;

    RAISE NOTICE '完成：更新规则 % 行，新增规则 % 行；5 条规则已生效且目标 usc 均已登记（重复执行应各为 0）',
        v_upd, v_ins;
END $$;

COMMIT;

-- 执行后核对
-- SELECT idn, usc_old, usc_new, match_mode, flag_enable, priority
--   FROM cdeu.finv_quote_cdeu_usc_patch_rule
--  WHERE usc_old IN ('SH000888', 'SH000903', 'SH000922', 'SH000980', 'GDRnext')
--  ORDER BY usc_old;                                                       -- 5 行，flag_enable 全为 1
-- SELECT p.usc_old, p.usc_new, (u.usc IS NOT NULL) AS registered
--   FROM cdeu.finv_quote_cdeu_usc_patch_rule p
--   LEFT JOIN cdeu.finv_quote_cdeu_usc u ON u.usc = p.usc_new
--  WHERE p.usc_old IN ('SH000888', 'SH000903', 'SH000922', 'SH000980', 'GDRnext');  -- registered 全为 t
