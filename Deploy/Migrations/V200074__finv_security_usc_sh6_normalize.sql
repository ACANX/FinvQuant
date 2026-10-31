-- =====================================================================
-- FinvQuant PostgreSQL V200074：SH 指数类 usc 由 8 位规范为 6 位（需求 10873）
--
-- 背景：
--   上游证券代码口径已调整：`SH000001`~`SH000999` 区间由 8 位改为 6 位（去掉数字
--   部分前两个 0），即 `SH000001` → `SH0001`、`SH000985` → `SH0985`。
--   但库里同时存在两套 usc：
--     · 8 位：`V200009` 增量种子（`provider=''`、英文名、`gmt_create=2026-08-05`）
--     · 6 位：`V200062` Supabase 批量导入（`provider='FTMM'`、中文名）与历史手工改名
--   生产库 22 组并存、全新库 23 组并存（多一条 `SH000001`，尚无 6 位对应）。
--
-- 冲突风险与规避（本期只做「对齐 + 回填」，**不删除** 8 位残留行）：
--   1. 直接把 8 位行 `UPDATE usc='SH0002'` 会撞主键 `finv_security_pkey`；
--   2. 同时改 `security_code` 会撞唯一约束 `uq_finv_security_exchange_code`
--      （`(exchange_code, security_code)`）；
--   3. 脚本重跑、或「代码改后已插入 6 位行」再执行，会重复触发上述两类冲突；
--   4. 其他表若已写入 8 位 usc（如 kline 主键 `(ts, usc)`），改号同样会撞主键。
--   本迁移的策略：**只在目标不存在时改号**（无对应 6 位 → 就地改号；已有 6 位 →
--   6 位为准并回填字段），删除动作留待后续需求，因此任何前置状态下都不会冲突。
--
-- 决策依据（用户 2026-11-01 逐条批准）：
--   D1 8 位行英文名回填 6 位行；D2 `flag_enable` 取 `'1'` 优先；D3 `gmt_create`
--   保留较早值；D4 `SH0985` 的 `exchange_code`/`security_code` 对齐为 `11`/`'SH0985'`；
--   D5 `security_type` 保留 6 位现网值；D6 其余 17 张含 `usc` 的表本期只巡检不处理；
--   D7 8 位残留行**暂不删除**；D8 `SH0001` 的 `security_code` 归一为 `'SH0001'`。
--
-- 幂等：每一步都带「目标不存在 / 值不相等」条件，重复执行影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 前置：V200009（8 位种子）、V200062（6 位种子）、V200073（market_code 口径）。
-- 执行前备份：`Deploy/Backup/usc_sh6_pre_*.dump`（`finv_security` +
--   `futu.finv_futu_mapping_security` 的 pg_dump 自定义格式）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 状态 S1：8 位 usc 没有对应 6 位行 → 就地改号（usc 与 security_code 同步）
--
--    双 NOT EXISTS 保护：
--      · 目标 usc 不存在        → 避主键冲突；
--      · 目标 (exchange, code) 不存在 → 避 uq_finv_security_exchange_code 冲突。
--    映射 `SH` + substr(usc,5) 是单射（8 位 → 6 位一一对应），语句内不会互相撞。
-- ---------------------------------------------------------------------
UPDATE public.finv_security AS s
   SET usc           = 'SH' || substr(s.usc, 5),
       security_code = CASE WHEN s.security_code = s.usc
                            THEN 'SH' || substr(s.usc, 5)
                            ELSE s.security_code END,
       gmt_update    = now()
 WHERE s.usc ~ '^SH[0-9]{6}$'
   AND NOT EXISTS (SELECT 1 FROM public.finv_security x
                    WHERE x.usc = 'SH' || substr(s.usc, 5))
   AND NOT EXISTS (SELECT 1 FROM public.finv_security y
                    WHERE y.exchange_code = s.exchange_code
                      AND y.security_code = CASE WHEN s.security_code = s.usc
                                                 THEN 'SH' || substr(s.usc, 5)
                                                 ELSE s.security_code END);

-- ---------------------------------------------------------------------
-- 2. D4：`SH0985`（中证全指）的 6 位行对齐到 `exchange_code=11` / `security_code='SH0985'`
--    历史导入把它记成了 `exchange_code=10`（CN 泛化市场）/ `security_code='000985'`，
--    与其余 21 组（上交所具体市场 + 与 usc 同形的代码）不一致。
--    已符合的库（全新库）影响 0 行；冲突保护同步骤 1。
-- ---------------------------------------------------------------------
UPDATE public.finv_security
   SET exchange_code = 11,
       security_code = 'SH0985',
       gmt_update    = now()
 WHERE usc = 'SH0985'
   AND (exchange_code <> 11 OR security_code <> 'SH0985')
   AND NOT EXISTS (SELECT 1 FROM public.finv_security x
                    WHERE x.exchange_code = 11
                      AND x.security_code = 'SH0985'
                      AND x.usc <> 'SH0985');

-- ---------------------------------------------------------------------
-- 3. 状态 S2：8 位与 6 位并存 → 以 6 位行为主体，回填 8 位行里更完整的信息
--    · D1：6 位行 `security_name` 与中文名相同时（导入时中文名占位），用 8 位行的英文名覆盖；
--    · D2：`flag_enable` 取 `'1'` 优先（任一为启用即启用，避免合并后被停用）；
--    · D3：`gmt_create` 保留较早值（同一证券的历史起点）。
--    回填后 6 位行 `security_name <> security_name_cn`，重跑不再命中（幂等）。
-- ---------------------------------------------------------------------
UPDATE public.finv_security AS t
   SET security_name = CASE WHEN t.security_name = t.security_name_cn
                            THEN s.security_name
                            ELSE t.security_name END,
       flag_enable   = CASE WHEN s.flag_enable = '1' THEN '1' ELSE t.flag_enable END,
       gmt_create    = LEAST(t.gmt_create, s.gmt_create),
       gmt_update    = now()
  FROM public.finv_security AS s
 WHERE s.usc ~ '^SH[0-9]{6}$'
   AND t.usc = 'SH' || substr(s.usc, 5)
   AND (   (t.security_name = t.security_name_cn AND s.security_name <> s.security_name_cn)
        OR (s.flag_enable = '1' AND t.flag_enable <> '1')
        OR s.gmt_create < t.gmt_create);

-- ---------------------------------------------------------------------
-- 4. D8：`SH0001`（上证指数）的 security_code 归一
--    生产库为历史手工改名，usc 已是 `SH0001` 但 security_code 仍留着 8 位 `SH000001`；
--    全新库由步骤 1 改号时已同步为 `SH0001`，此处影响 0 行。
-- ---------------------------------------------------------------------
UPDATE public.finv_security
   SET security_code = 'SH0001',
       gmt_update    = now()
 WHERE usc = 'SH0001'
   AND security_code = 'SH000001';

-- ---------------------------------------------------------------------
-- 5. `futu.finv_futu_mapping_security` 的 `finv_usc` 同步为 6 位
--    8 位 usc 的源头（`V200003` 种子 23 行，`flag_enable='0'`、Go/Python 均未引用）。
--    主键是 `futu_stock_id`、`finv_usc` 无唯一约束 → 改值不会冲突；条件化保证幂等。
-- ---------------------------------------------------------------------
UPDATE futu.finv_futu_mapping_security
   SET finv_usc  = 'SH' || substr(finv_usc, 5),
       gmt_update = now()
 WHERE finv_usc ~ '^SH[0-9]{6}$';

-- ---------------------------------------------------------------------
-- 6. D6：其余含 `usc` 列的表本期只巡检（不迁移、不阻断）
--    当前实测这些表 8 位 SH usc 均为 0 行；若将来出现（说明有代码/脚本仍在写 8 位），
--    这里输出 NOTICE 便于定位，是否迁移由后续需求决定。
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r      record;
    n      bigint;
    cn     bigint := 0;
BEGIN
    FOR r IN
        SELECT c.table_schema, c.table_name
          FROM information_schema.columns c
          JOIN information_schema.tables t
            ON t.table_schema = c.table_schema AND t.table_name = c.table_name
         WHERE c.column_name = 'usc'
           AND t.table_type = 'BASE TABLE'
           AND NOT (c.table_schema = 'public' AND c.table_name = 'finv_security')
         ORDER BY 1, 2
    LOOP
        EXECUTE format('SELECT count(*) FROM %I.%I WHERE usc ~ %L',
                       r.table_schema, r.table_name, '^SH[0-9]{6}$') INTO n;
        IF n > 0 THEN
            cn := cn + 1;
            RAISE NOTICE 'V200074 巡检：%.% 仍有 % 行 8 位 SH usc（决策 D6 暂不自动迁移，需人工评估）',
                         r.table_schema, r.table_name, n;
        END IF;
    END LOOP;
    IF cn = 0 THEN
        RAISE NOTICE 'V200074 巡检：其余含 usc 列的表均无 8 位 SH usc';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 7. 收敛校验（不满足即 EXCEPTION，整体回滚）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    n_bad_code  bigint;   -- 6 位 usc 行仍留着 8 位 security_code
    n_futu8     bigint;   -- futu 映射表仍留 8 位
    n_orphan8   bigint;   -- 8 位行没有 6 位对应（步骤 1 未生效）
    n_left8     bigint;   -- 本期保留的 8 位重复行（D7）
    v_sh6       bigint;
BEGIN
    SELECT count(*) INTO n_bad_code
      FROM public.finv_security
     WHERE usc ~ '^SH[0-9]{4}$'
       AND security_code ~ '^SH[0-9]{6}$';
    IF n_bad_code <> 0 THEN
        RAISE EXCEPTION 'V200074 校验失败：% 行 6 位 usc 的 security_code 仍为 8 位', n_bad_code;
    END IF;

    SELECT count(*) INTO n_futu8
      FROM futu.finv_futu_mapping_security
     WHERE finv_usc ~ '^SH[0-9]{6}$';
    IF n_futu8 <> 0 THEN
        RAISE EXCEPTION 'V200074 校验失败：futu 映射表仍有 % 行 8 位 finv_usc', n_futu8;
    END IF;

    SELECT count(*) INTO n_orphan8
      FROM public.finv_security s
     WHERE s.usc ~ '^SH[0-9]{6}$'
       AND NOT EXISTS (SELECT 1 FROM public.finv_security t
                        WHERE t.usc = 'SH' || substr(s.usc, 5));
    IF n_orphan8 <> 0 THEN
        RAISE EXCEPTION 'V200074 校验失败：% 行 8 位 usc 没有 6 位对应（改号未生效）', n_orphan8;
    END IF;

    SELECT count(*) INTO v_sh6 FROM public.finv_security WHERE usc ~ '^SH[0-9]{4}$';
    SELECT count(*) INTO n_left8 FROM public.finv_security WHERE usc ~ '^SH[0-9]{6}$';
    RAISE NOTICE 'V200074 完成：6 位 SH usc 共 % 行；保留 8 位重复行 % 行（决策 D7 暂不删除，后续需求清理）',
                 v_sh6, n_left8;
END $$;

COMMIT;
