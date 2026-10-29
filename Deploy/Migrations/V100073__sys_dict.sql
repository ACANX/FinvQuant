-- =====================================================================
-- FinvQuant PostgreSQL V73：sys schema 新增数据字典表 sys_dict / sys_dict_sub
--
-- 决策（ACANX 2026-10-25）：
--   - 需求：数据字典（字典主表 + 字典子表）落在 V71 新建的 sys schema，
--     与 V72 的 sys_config 同属系统域；表结构按 ACANX 提供的 DDL 落地，
--     列类型、默认值、可空性、约束名与列注释保持一致。
--   - sys_dict 为字典定义（key_type/val_type 描述键值类型、access_level 等
--     分级字段、ref_spec 参考规范），sys_dict_sub 为字典项（主键
--     (dict_no, sub_key)，flag_enable 表示启用）。
--   - 两表之间不建物理外键（与全库「禁止外键、跨表关系由应用层校验」一致）。
--   - ⚠ 触发器语义（ACANX 确认）：gmt_create 仅在插入时生成，**更新不改写**；
--     触发器只刷新 gmt_modify，等价 MySQL 的 ON UPDATE CURRENT_TIMESTAMP(6)。
--     注：sys_dict 的 gmt_create / gmt_modify 按原 DDL 允许 NULL 且无默认值，
--     插入时由应用层传值；如需库层自动生成，另行加 DEFAULT CURRENT_TIMESTAMP(6)。
--   - 时间列统一 timestamp(6)（与 MySQL DATETIME(6) 对齐，不带时区）。
--   - 幂等：CREATE SCHEMA/TABLE IF NOT EXISTS + CREATE OR REPLACE FUNCTION +
--     触发器存在性判断 + COMMENT 覆盖写。
-- 迁移策略：单事务、失败整体回滚；末尾做收敛校验。
-- 前置：V71（sys schema）、V72（sys_config）已应用。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. schema 兜底（V71 已创建；此处幂等，便于单文件手工执行）
-- ---------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS sys;

-- ---------------------------------------------------------------------
-- 2. 数据字典主表
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sys.sys_dict (
    dict_no        integer       NOT NULL,
    dict_name      varchar(255)  DEFAULT NULL,
    key_type       char(1)       DEFAULT NULL,
    val_type       char(1)       DEFAULT NULL,
    remark         varchar(2000) DEFAULT NULL,
    gmt_create     timestamp(6)  DEFAULT NULL,
    gmt_modify     timestamp(6)  DEFAULT NULL,
    ref_spec       varchar(2000) DEFAULT NULL,
    associate_dict varchar(255)  DEFAULT NULL,
    access_level   integer       DEFAULT NULL,
    manage_level   varchar(255)  DEFAULT NULL,
    data_level     varchar(255)  DEFAULT NULL,
    en_branch      varchar(2000) DEFAULT NULL,
    en_sub_sys     varchar(255)  DEFAULT NULL,
    enum_class     varchar(255)  DEFAULT NULL,
    dict_category  varchar(255)  DEFAULT NULL,
    CONSTRAINT pk_sys_dict PRIMARY KEY (dict_no)
);

COMMENT ON TABLE  sys.sys_dict                IS '数据字典主表';
COMMENT ON COLUMN sys.sys_dict.dict_no        IS '字典编号';
COMMENT ON COLUMN sys.sys_dict.dict_name      IS '字典名称';
COMMENT ON COLUMN sys.sys_dict.key_type       IS '字典键类型';
COMMENT ON COLUMN sys.sys_dict.val_type       IS '字典值类型';
COMMENT ON COLUMN sys.sys_dict.remark         IS '备注';
COMMENT ON COLUMN sys.sys_dict.gmt_create     IS '创建时间';
COMMENT ON COLUMN sys.sys_dict.gmt_modify     IS '修改时间';
COMMENT ON COLUMN sys.sys_dict.ref_spec       IS '参考规范';
COMMENT ON COLUMN sys.sys_dict.associate_dict IS '过时字段|关联字典';
COMMENT ON COLUMN sys.sys_dict.access_level   IS '访问级别';
COMMENT ON COLUMN sys.sys_dict.manage_level   IS '管理级别';
COMMENT ON COLUMN sys.sys_dict.data_level     IS '数据级别';
COMMENT ON COLUMN sys.sys_dict.en_branch      IS '允许的分支机构';
COMMENT ON COLUMN sys.sys_dict.en_sub_sys     IS '允许的子系统';
COMMENT ON COLUMN sys.sys_dict.enum_class     IS '枚举类';
COMMENT ON COLUMN sys.sys_dict.dict_category  IS '字典分类';

-- 模拟 ON UPDATE CURRENT_TIMESTAMP(6)：只刷新 gmt_modify，gmt_create 保持不变
CREATE OR REPLACE FUNCTION sys.fn_sys_dict_touch()
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
         WHERE tgname = 'trg_sys_dict_touch'
           AND tgrelid = 'sys.sys_dict'::regclass
    ) THEN
        CREATE TRIGGER trg_sys_dict_touch
            BEFORE UPDATE ON sys.sys_dict
            FOR EACH ROW
            EXECUTE FUNCTION sys.fn_sys_dict_touch();
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 3. 数据字典子表
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sys.sys_dict_sub (
    dict_no       integer       NOT NULL,
    sub_key       varchar(255)  NOT NULL,
    sub_val       varchar(255)  NOT NULL,
    sub_sort      varchar(255)  DEFAULT NULL,
    name_en       varchar(255)  DEFAULT NULL,
    remark        varchar(2000) DEFAULT NULL,
    description   varchar(2000) DEFAULT NULL,
    flag_enable   char(1)       NOT NULL DEFAULT '0',
    associate_sub varchar(255)  DEFAULT NULL,
    access_level  integer       DEFAULT NULL,
    data_level    varchar(255)  DEFAULT NULL,
    en_sys_str    varchar(255)  DEFAULT NULL,
    create_by     varchar(32)   DEFAULT NULL,
    modify_by     varchar(32)   DEFAULT NULL,
    gmt_modify    timestamp(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    gmt_create    timestamp(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT pk_sys_dict_sub PRIMARY KEY (dict_no, sub_key)
);

COMMENT ON TABLE  sys.sys_dict_sub                IS '数据字典子表';
COMMENT ON COLUMN sys.sys_dict_sub.dict_no        IS '字典编号';
COMMENT ON COLUMN sys.sys_dict_sub.sub_key        IS '子键';
COMMENT ON COLUMN sys.sys_dict_sub.sub_val        IS '子值';
COMMENT ON COLUMN sys.sys_dict_sub.sub_sort       IS '排序';
COMMENT ON COLUMN sys.sys_dict_sub.name_en        IS '英文名';
COMMENT ON COLUMN sys.sys_dict_sub.remark         IS '备注';
COMMENT ON COLUMN sys.sys_dict_sub.description    IS '描述';
COMMENT ON COLUMN sys.sys_dict_sub.flag_enable    IS '启用标志';
COMMENT ON COLUMN sys.sys_dict_sub.associate_sub  IS '关联子项';
COMMENT ON COLUMN sys.sys_dict_sub.access_level   IS '访问级别';
COMMENT ON COLUMN sys.sys_dict_sub.data_level     IS '数据级别';
COMMENT ON COLUMN sys.sys_dict_sub.en_sys_str     IS '允许的子系统';
COMMENT ON COLUMN sys.sys_dict_sub.create_by      IS '创建人';
COMMENT ON COLUMN sys.sys_dict_sub.modify_by      IS '最后修改人';
COMMENT ON COLUMN sys.sys_dict_sub.gmt_modify     IS '最后修改时间';
COMMENT ON COLUMN sys.sys_dict_sub.gmt_create     IS '创建时间';

-- 模拟 ON UPDATE CURRENT_TIMESTAMP(6)：只刷新 gmt_modify，gmt_create 保持不变
CREATE OR REPLACE FUNCTION sys.fn_sys_dict_sub_touch()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.gmt_modify := CURRENT_TIMESTAMP(6);
    RETURN NEW;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger
         WHERE tgname = 'trg_sys_dict_sub_touch'
           AND tgrelid = 'sys.sys_dict_sub'::regclass
    ) THEN
        CREATE TRIGGER trg_sys_dict_sub_touch
            BEFORE UPDATE ON sys.sys_dict_sub
            FOR EACH ROW
            EXECUTE FUNCTION sys.fn_sys_dict_sub_touch();
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 4. 收敛校验：两表存在、列数、主键、触发器、函数、注释
-- ---------------------------------------------------------------------
DO $$
DECLARE
    tbl_cnt  INT;
    col_cnt  INT;
    pk_cnt   INT;
    trg_cnt  INT;
    fn_cnt   INT;
    bad_src  INT;
    cmt_cnt  INT;
BEGIN
    SELECT count(*) INTO tbl_cnt
      FROM pg_class c
      JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'sys' AND c.relkind = 'r'
       AND c.relname IN ('sys_dict', 'sys_dict_sub');
    IF tbl_cnt <> 2 THEN
        RAISE EXCEPTION 'V73: sys.sys_dict / sys.sys_dict_sub 未全部创建成功（实际 %）', tbl_cnt;
    END IF;

    SELECT count(*) INTO col_cnt
      FROM information_schema.columns
     WHERE table_schema = 'sys' AND table_name = 'sys_dict';
    IF col_cnt <> 16 THEN
        RAISE EXCEPTION 'V73: sys.sys_dict 列数为 %，预期 16', col_cnt;
    END IF;

    SELECT count(*) INTO col_cnt
      FROM information_schema.columns
     WHERE table_schema = 'sys' AND table_name = 'sys_dict_sub';
    IF col_cnt <> 16 THEN
        RAISE EXCEPTION 'V73: sys.sys_dict_sub 列数为 %，预期 16', col_cnt;
    END IF;

    SELECT count(*) INTO pk_cnt
      FROM pg_constraint
     WHERE conname = 'pk_sys_dict'
       AND conrelid = 'sys.sys_dict'::regclass
       AND contype = 'p';
    IF pk_cnt <> 1 THEN
        RAISE EXCEPTION 'V73: sys.sys_dict 缺少主键 pk_sys_dict(dict_no)';
    END IF;

    SELECT count(*) INTO pk_cnt
      FROM pg_constraint
     WHERE conname = 'pk_sys_dict_sub'
       AND conrelid = 'sys.sys_dict_sub'::regclass
       AND contype = 'p';
    IF pk_cnt <> 1 THEN
        RAISE EXCEPTION 'V73: sys.sys_dict_sub 缺少主键 pk_sys_dict_sub(dict_no, sub_key)';
    END IF;

    SELECT count(*) INTO trg_cnt
      FROM pg_trigger
     WHERE tgname IN ('trg_sys_dict_touch', 'trg_sys_dict_sub_touch')
       AND tgrelid IN ('sys.sys_dict'::regclass, 'sys.sys_dict_sub'::regclass);
    IF trg_cnt <> 2 THEN
        RAISE EXCEPTION 'V73: 字典表触发器缺失（实际 %，预期 2）', trg_cnt;
    END IF;

    SELECT count(*) INTO fn_cnt
      FROM pg_proc p
      JOIN pg_namespace n ON n.oid = p.pronamespace
     WHERE n.nspname = 'sys'
       AND p.proname IN ('fn_sys_dict_touch', 'fn_sys_dict_sub_touch');
    IF fn_cnt <> 2 THEN
        RAISE EXCEPTION 'V73: 字典触发器函数缺失（实际 %，预期 2）', fn_cnt;
    END IF;

    -- 语义守住：创建时间只在插入时生成，更新不得改写 gmt_create
    SELECT count(*) INTO bad_src
      FROM pg_proc p
      JOIN pg_namespace n ON n.oid = p.pronamespace
     WHERE n.nspname = 'sys'
       AND p.proname IN ('fn_sys_dict_touch', 'fn_sys_dict_sub_touch')
       AND position('NEW.gmt_create' in p.prosrc) > 0;
    IF bad_src > 0 THEN
        RAISE EXCEPTION 'V73: 触发器函数不得改写 gmt_create（创建时间只在插入时生成）';
    END IF;

    SELECT count(*) INTO cmt_cnt
      FROM pg_description
     WHERE objoid = 'sys.sys_dict'::regclass
       AND objsubid BETWEEN 0 AND 16;
    IF cmt_cnt <> 17 THEN
        RAISE EXCEPTION 'V73: sys.sys_dict 注释数为 %，预期 17（表 + 16 列）', cmt_cnt;
    END IF;

    SELECT count(*) INTO cmt_cnt
      FROM pg_description
     WHERE objoid = 'sys.sys_dict_sub'::regclass
       AND objsubid BETWEEN 0 AND 16;
    IF cmt_cnt <> 17 THEN
        RAISE EXCEPTION 'V73: sys.sys_dict_sub 注释数为 %，预期 17（表 + 16 列）', cmt_cnt;
    END IF;

    RAISE NOTICE 'V73: 校验通过（sys_dict / sys_dict_sub 就绪，各 16 列，主键/触发器/函数/注释齐备）';
END $$;

COMMIT;
