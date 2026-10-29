-- =====================================================================
-- FinvQuant PostgreSQL V72：sys schema 新增系统配置表 sys.sys_config
--
-- 决策（ACANX 2026-10-25）：
--   - 需求：系统级键值配置表落在 V71 新建的 sys schema（系统域），承载
--     数据类型可选的配置项（整型 / 字符 / 字符串 / JSON），按 config_no 定位。
--   - 表结构按 ACANX 提供的 DDL 落地；列类型、默认值、可空性、约束名
--     （pk_sys_config）与列注释保持一致。
--   - data_type 取值沿用原系统约定：'1'=整型（取 int_value）、'2'=字符（取
--     char_value）；'3'/'4' 预期对应 str_value / json_value（以原系统字典为准，
--     应用层按此映射取值）。初始配置数据见 V100025__sys_config_seed.sql。
--   - 等价 MySQL 的 ON UPDATE CURRENT_TIMESTAMP(6)：用 BEFORE UPDATE 行级
--     触发器 trg_sys_config_set_gmt_modify 维护 gmt_modify；gmt_create 只在
--     插入时取默认值，不在更新时改写。
--   - 时间列统一使用 timestamp(6)（毫秒/微秒精度与 MySQL 对齐），与 sys 域
--     以外表使用的 timestamptz 不冲突——本表为独立系统表，不参与行情链路。
--   - 幂等：CREATE SCHEMA/TABLE IF NOT EXISTS + CREATE OR REPLACE FUNCTION +
--     触发器存在性判断 + COMMENT 覆盖写（COMMENT 本身幂等）。
-- 迁移策略：单事务、失败整体回滚；末尾做收敛校验。
-- 前置：V71（sys schema）已应用。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. schema 兜底（V71 已创建；此处幂等，便于单文件手工执行）
-- ---------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS sys;

-- ---------------------------------------------------------------------
-- 2. 系统配置表
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sys.sys_config (
    config_no    integer       NOT NULL,
    config_name  varchar(128)  DEFAULT '',
    data_type    char(1)       NOT NULL,
    int_value    integer       DEFAULT 0,
    char_value   char(1)       DEFAULT '',
    str_value    varchar(6000) DEFAULT '',
    json_value   json          DEFAULT NULL,
    access_level integer       DEFAULT NULL,
    create_by    varchar(32)   DEFAULT NULL,
    gmt_create   timestamp(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    gmt_modify   timestamp(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    modify_by    varchar(32)   DEFAULT NULL,
    remark       varchar(2000) DEFAULT NULL,
    CONSTRAINT pk_sys_config PRIMARY KEY (config_no)
);

-- ---------------------------------------------------------------------
-- 3. 表与列注释
-- ---------------------------------------------------------------------
COMMENT ON TABLE  sys.sys_config              IS '系统配置表';
COMMENT ON COLUMN sys.sys_config.config_no    IS '配置编号';
COMMENT ON COLUMN sys.sys_config.config_name  IS '配置名称';
COMMENT ON COLUMN sys.sys_config.data_type    IS '数据类型';
COMMENT ON COLUMN sys.sys_config.int_value    IS '整型值';
COMMENT ON COLUMN sys.sys_config.char_value   IS '字符值';
COMMENT ON COLUMN sys.sys_config.str_value    IS '字符串值';
COMMENT ON COLUMN sys.sys_config.json_value   IS 'JSON 值';
COMMENT ON COLUMN sys.sys_config.access_level IS '访问级别';
COMMENT ON COLUMN sys.sys_config.create_by    IS '创建人';
COMMENT ON COLUMN sys.sys_config.gmt_create   IS '创建时间';
COMMENT ON COLUMN sys.sys_config.gmt_modify   IS '最后修改时间';
COMMENT ON COLUMN sys.sys_config.modify_by    IS '最后修改人';
COMMENT ON COLUMN sys.sys_config.remark       IS '备注';

-- ---------------------------------------------------------------------
-- 4. 模拟 MySQL 的 ON UPDATE CURRENT_TIMESTAMP(6)
-- ---------------------------------------------------------------------
CREATE OR REPLACE FUNCTION sys.fn_sys_config_set_gmt_modify()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.gmt_modify := CURRENT_TIMESTAMP(6);
    RETURN NEW;
END;
$$;

-- 触发器无 IF NOT EXISTS 语法，用存在性判断保证幂等
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger
         WHERE tgname = 'trg_sys_config_set_gmt_modify'
           AND tgrelid = 'sys.sys_config'::regclass
    ) THEN
        CREATE TRIGGER trg_sys_config_set_gmt_modify
            BEFORE UPDATE ON sys.sys_config
            FOR EACH ROW
            EXECUTE FUNCTION sys.fn_sys_config_set_gmt_modify();
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 5. 收敛校验：表归属、列数、主键、触发器、函数、注释
-- ---------------------------------------------------------------------
DO $$
DECLARE
    tbl_ok     INT;
    col_cnt    INT;
    pk_cnt     INT;
    trg_cnt    INT;
    fn_cnt     INT;
    bad_src    INT;
    cmt_cnt    INT;
BEGIN
    SELECT count(*) INTO tbl_ok
      FROM pg_class c
      JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'sys' AND c.relname = 'sys_config' AND c.relkind = 'r';
    IF tbl_ok = 0 THEN
        RAISE EXCEPTION 'V72: 表 sys.sys_config 未创建成功';
    END IF;

    SELECT count(*) INTO col_cnt
      FROM information_schema.columns
     WHERE table_schema = 'sys' AND table_name = 'sys_config';
    IF col_cnt <> 13 THEN
        RAISE EXCEPTION 'V72: sys.sys_config 列数为 %，预期 13', col_cnt;
    END IF;

    SELECT count(*) INTO pk_cnt
      FROM pg_constraint
     WHERE conname = 'pk_sys_config'
       AND conrelid = 'sys.sys_config'::regclass
       AND contype = 'p';
    IF pk_cnt <> 1 THEN
        RAISE EXCEPTION 'V72: sys.sys_config 缺少主键 pk_sys_config(config_no)';
    END IF;

    SELECT count(*) INTO trg_cnt
      FROM pg_trigger
     WHERE tgname = 'trg_sys_config_set_gmt_modify'
       AND tgrelid = 'sys.sys_config'::regclass;
    IF trg_cnt <> 1 THEN
        RAISE EXCEPTION 'V72: sys.sys_config 缺少触发器 trg_sys_config_set_gmt_modify';
    END IF;

    SELECT count(*) INTO fn_cnt
      FROM pg_proc p
      JOIN pg_namespace n ON n.oid = p.pronamespace
     WHERE n.nspname = 'sys' AND p.proname = 'fn_sys_config_set_gmt_modify';
    IF fn_cnt = 0 THEN
        RAISE EXCEPTION 'V72: 函数 sys.fn_sys_config_set_gmt_modify 未创建成功';
    END IF;

    -- 语义守住：创建时间只在插入时生成，更新不得改写 gmt_create
    SELECT count(*) INTO bad_src
      FROM pg_proc p
      JOIN pg_namespace n ON n.oid = p.pronamespace
     WHERE n.nspname = 'sys'
       AND p.proname = 'fn_sys_config_set_gmt_modify'
       AND position('NEW.gmt_create' in p.prosrc) > 0;
    IF bad_src > 0 THEN
        RAISE EXCEPTION 'V72: 触发器函数不得改写 gmt_create（创建时间只在插入时生成）';
    END IF;

    SELECT count(*) INTO cmt_cnt
      FROM pg_description
     WHERE objoid = 'sys.sys_config'::regclass
       AND objsubid BETWEEN 0 AND 13;
    IF cmt_cnt <> 14 THEN
        RAISE EXCEPTION 'V72: sys.sys_config 注释数为 %，预期 14（表 + 13 列）', cmt_cnt;
    END IF;

    RAISE NOTICE 'V72: 校验通过（sys.sys_config 就绪，13 列、主键、触发器与注释齐备）';
END $$;

COMMIT;
