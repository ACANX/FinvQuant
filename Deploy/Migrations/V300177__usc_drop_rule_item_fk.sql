-- =====================================================================
-- FinvQuant PostgreSQL V300177：移除 USC 规则条目上的物理外键（需求 11093）
--
-- 背景：`V300157` 给 `futu.finv_futu_usc_rule_item` 建了外键
--   `finv_futu_usc_rule_item_rule_fk (rule_no) REFERENCES futu.finv_futu_usc_rule(rule_no)`。
--   这违反仓库既有约定——`Docs/DevSpec/DbSchemaSpec.md` §「跨域引用只走程序层（无物理外键）」，
--   迁移集成测试也断言「全库不应存在外键」（实测全库就这一个）。
--
-- 动作：删掉该外键约束，改由程序层保证（`uscrule` 的写入路径在插入条目前校验规则存在；
--   规则层只有 10 条固定记录，由本迁移链的种子维护）。
--
-- 幂等：约束不存在时跳过；末尾校验断言「全库外键数为 0」。
-- 迁移策略：单事务（BEGIN/COMMIT）。
-- =====================================================================

BEGIN;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
                WHERE conrelid = 'futu.finv_futu_usc_rule_item'::regclass
                  AND conname = 'finv_futu_usc_rule_item_rule_fk') THEN
        ALTER TABLE futu.finv_futu_usc_rule_item DROP CONSTRAINT finv_futu_usc_rule_item_rule_fk;
        RAISE NOTICE 'V300177：已删除 finv_futu_usc_rule_item_rule_fk';
    ELSE
        RAISE NOTICE 'V300177：外键不存在，跳过';
    END IF;
END $$;

DO $$
DECLARE
    v_fk BIGINT;
BEGIN
    SELECT count(*) INTO v_fk FROM pg_constraint WHERE contype = 'f';
    IF v_fk <> 0 THEN
        RAISE EXCEPTION 'V300177 校验失败：全库仍有 % 个物理外键（约定为 0）', v_fk;
    END IF;
    RAISE NOTICE 'V300177 完成：全库物理外键 0 个';
END $$;

COMMIT;
