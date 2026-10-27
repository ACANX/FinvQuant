-- =====================================================================
-- FinvQuant PostgreSQL V100053：补齐字典 3120 的主表记录（需求 10749）
--
-- 背景：V100027 已写入 sys.sys_dict_sub 中 dict_no=3120 的 14 条子项
-- （0/1/2/…/9/A/B/C/D → CNY/USD/HKD/EUR/GBP/AUD/CAD/SGD/JPY/MOP/DKK/GBX/MYR/TWD），
-- 但 V100026 的 19 条 sys.sys_dict 主表记录里没有 3120，形成「有子项、无主表」的孤儿字典。
-- 本迁移只补主表定义行，不动任何子项。
--
-- 取值依据（逐字段说明，避免凭空编造）：
--   · dict_name '币种'      —— 子项是 14 种货币的中英文对照（CNY/USD/HKD/…/TWD），
--                              且 3121「外汇交易币种」、3122「注册资本币种」均为其衍生币种字典，
--                              故 3120 是这些衍生字典所依据的**基础币种**字典。
--   · key_type '1' / val_type '2' —— 与 3103/3121/3122 等「1 键对 2 值」的码表一致。
--   · access_level 1 / manage_level '0' / data_level '2' —— 与 3121、3122 完全一致。
--   · dict_category '2'     —— 与 3101/3103/3110/3121/3122 同属「码表类」分类。
--   · gmt_create / gmt_modify '2024-12-19 15:28:10.900191' —— 取 3120 子项自身的写入时间，
--                              表示主表与子项同批导入；gmt_create 仅插入时写入，后续 UPDATE 不重写。
--   · en_sub_sys ''         —— 3121='act'、3122='cic' 都是单一子系统归属；
--                              而 3120 子项的 en_sys_str 为 'gat,gax,gcq,gaa,tms,pbs' 六子系统共用，
--                              无单一归属，故留空而不是随便挑一个。
--     ⚠ 若原库对 dict_name 或 en_sub_sys 另有取值，运维可直接 UPDATE 修正——
--       本迁移使用 ON CONFLICT DO NOTHING，重跑不会覆盖任何已调整的值。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置校验：确认子项存在且主表确实缺失
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_sub   bigint;
    v_dic   bigint;
BEGIN
    SELECT count(*) INTO v_sub FROM sys.sys_dict_sub WHERE dict_no = 3120;
    SELECT count(*) INTO v_dic FROM sys.sys_dict WHERE dict_no = 3120;

    IF v_sub = 0 THEN
        RAISE EXCEPTION 'V100053 前置校验失败：sys_dict_sub 中不存在 dict_no=3120，无需补主表';
    END IF;
    IF v_dic > 0 THEN
        RAISE NOTICE 'V100053 提示：3120 主表记录已存在，本迁移将跳过插入（不覆盖现有值）';
    ELSE
        RAISE NOTICE 'V100053 前置校验：3120 子项 % 条，主表缺失，准备补齐', v_sub;
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. 插入主表记录（幂等，不覆盖用户已调整的定义）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_dict
    (dict_no, dict_name, key_type, val_type, remark,
     gmt_create, gmt_modify,
     ref_spec, associate_dict, access_level, manage_level, data_level,
     en_branch, en_sub_sys, enum_class, dict_category)
VALUES
    (3120, '币种', '1', '2', NULL,
     '2024-12-19 15:28:10.900191'::timestamp(6),
     '2024-12-19 15:28:10.900191'::timestamp(6),
     NULL, NULL, 1, '0', '2', NULL, '', NULL, '2')
ON CONFLICT (dict_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_dic       bigint;
    v_sub       bigint;
    v_orphan    bigint;
    v_orphan_no text;
BEGIN
    -- 3.1 3120 主表已就位
    SELECT count(*) INTO v_dic FROM sys.sys_dict WHERE dict_no = 3120;
    IF v_dic <> 1 THEN
        RAISE EXCEPTION 'V100053 收敛失败：3120 主表记录数为 %，应为 1', v_dic;
    END IF;

    -- 3.2 3120 子项未被本迁移影响
    SELECT count(*) INTO v_sub FROM sys.sys_dict_sub WHERE dict_no = 3120;
    IF v_sub <> 14 THEN
        RAISE EXCEPTION 'V100053 收敛失败：3120 子项应为 14 条，实际 %', v_sub;
    END IF;

    -- 3.3 全库不再存在「有子项、无主表」的孤儿字典
    SELECT coalesce(string_agg(DISTINCT s.dict_no::text, ','), '') INTO v_orphan_no
      FROM sys.sys_dict_sub s
     WHERE NOT EXISTS (SELECT 1 FROM sys.sys_dict d WHERE d.dict_no = s.dict_no);
    SELECT count(DISTINCT s.dict_no) INTO v_orphan
      FROM sys.sys_dict_sub s
     WHERE NOT EXISTS (SELECT 1 FROM sys.sys_dict d WHERE d.dict_no = s.dict_no);
    IF v_orphan > 0 THEN
        RAISE EXCEPTION 'V100053 收敛失败：仍存在 % 个孤儿字典（有子项无主表）：%', v_orphan, v_orphan_no;
    END IF;

    RAISE NOTICE 'V100053 完成：3120「币种」主表已补齐，14 条子项就位，全库孤儿字典 0 个';
END $$;

COMMIT;
