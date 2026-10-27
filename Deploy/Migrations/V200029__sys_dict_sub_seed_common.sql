-- =====================================================================
-- FinvQuant PostgreSQL V100029：sys.sys_dict_sub 通用标志/分页/数据类型子项（28 条）
--
-- 决策（ACANX 2026-10-25）：
--   - 本文件属于**种子段**（V100000+），依赖 V71（sys schema）与
--     V73（sys.sys_dict_sub 表结构），并假定 V100026 的字典主表已就绪。
--   - 数据来源：ACANX 提供的原系统字典子项导出（MySQL `cms_sys_dict_sub`），
--     字段值与时间戳原样保留；原 MySQL 反引号语法转为 PostgreSQL 写法。
--   - 覆盖六组字典（共 28 条）：
--       1001 普通标志        2 条（0 否 / 1 是）
--       1002 Meta业务字段类型 2 条（0 否 / 1 否，原库两值同名，原样保留）
--       1003 存取级别        3 条（1 可修改 / 2 只读 / 3 隐藏）
--       1004 分页模式        2 条（1 普通分页 / 2 排序分页，带 sub_sort 与 description）
--       1005 字段释义键序列   14 条（0-9、A-D，sub_val 与 sub_key 相同，sub_sort 1-14）
--       1006 数据类型（字典？） 5 条（0 字符 / 1 整数 / 2 浮点数 / 3 布尔 / 4 字符串）
--   - 原库差异点按原样保留：1002 的 '1' 也是「否」；1004 的 en_sys_str 为空串
--     （不是 NULL）；1006 的 flag_enable 为 '0'（其余组为 '1'）。
--   - 幂等策略：INSERT ... ON CONFLICT (dict_no, sub_key) DO NOTHING。
-- 迁移策略：单事务、失败整体回滚；末尾按「不少于」校验六组条数并抽查关键值。
-- =====================================================================

BEGIN;

INSERT INTO sys.sys_dict_sub
    (dict_no, sub_key, sub_val, sub_sort, name_en, remark, description, flag_enable,
     associate_sub, access_level, data_level, en_sys_str, create_by, modify_by,
     gmt_modify, gmt_create)
VALUES
    -- ============ 1001 普通标志（2 条） ============
    ( 1001, '0' , '否'           , NULL, NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'mnp,sps,ses,act,gat,gaa,gax,prd,dpp,dsn,dfn,tms,bcp-prd,foc', NULL, NULL, '2024-12-19 14:36:50.929917'::timestamp(6), '2024-12-19 14:36:50.929917'::timestamp(6)),
    ( 1001, '1' , '是'           , NULL, NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'mnp,sps,ses,act,gat,gaa,gax,prd,dpp,dsn,dfn,tms,bcp-prd,foc', NULL, NULL, '2024-12-19 14:36:50.929917'::timestamp(6), '2024-12-19 14:36:50.929917'::timestamp(6)),

    -- ============ 1002 Meta业务字段类型（2 条） ============
    ( 1002, '0' , '否'           , NULL, NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'mnp,sps,ses,act,gat,gaa,gax,prd,dpp,dsn,dfn,tms,bcp-prd,foc', NULL, NULL, '2024-12-17 15:19:10.861581'::timestamp(6), '2024-12-17 15:19:10.861581'::timestamp(6)),
    ( 1002, '1' , '否'           , NULL, NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'mnp,sps,ses,act,gat,gaa,gax,prd,dpp,dsn,dfn,tms,bcp-prd,foc', NULL, NULL, '2024-12-17 15:19:10.861581'::timestamp(6), '2024-12-17 15:19:10.861581'::timestamp(6)),

    -- ============ 1003 存取级别（3 条） ============
    ( 1003, '1' , '可修改'         , NULL, NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'dpp,pbs'             , NULL, NULL, '2024-12-17 15:38:37.382419'::timestamp(6), '2024-12-17 15:38:37.382419'::timestamp(6)),
    ( 1003, '2' , '只读'          , NULL, NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'dpp,pbs'             , NULL, NULL, '2024-12-17 15:38:37.382419'::timestamp(6), '2024-12-17 15:38:37.382419'::timestamp(6)),
    ( 1003, '3' , '隐藏'          , NULL, NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'dpp,pbs'             , NULL, NULL, '2024-12-17 15:38:37.382419'::timestamp(6), '2024-12-17 15:38:37.382419'::timestamp(6)),

    -- ============ 1004 分页模式（2 条） ============
    ( 1004, '1' , '普通分页'        , '1' , NULL, NULL, '最经典的精确分页'              , '1', NULL, 2, NULL, ''                    , NULL, NULL, '2026-04-13 23:44:54.696952'::timestamp(6), '2026-04-13 23:44:54.696952'::timestamp(6)),
    ( 1004, '2' , '排序分页'        , '2' , NULL, NULL, '按固定字段排序后分页'            , '1', NULL, 2, NULL, ''                    , NULL, NULL, '2026-04-13 23:44:58.450915'::timestamp(6), '2026-04-13 23:44:58.450915'::timestamp(6)),

    -- ============ 1005 字段释义键序列（14 条） ============
    ( 1005, '0' , '0'           , '1' , NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'ftc'                 , NULL, NULL, '2024-12-18 05:41:28.686060'::timestamp(6), '2024-12-18 05:41:28.686060'::timestamp(6)),
    ( 1005, '1' , '1'           , '2' , NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'ftc'                 , NULL, NULL, '2024-12-18 05:41:28.694182'::timestamp(6), '2024-12-18 05:41:28.694182'::timestamp(6)),
    ( 1005, '2' , '2'           , '3' , NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'ftc'                 , NULL, NULL, '2024-12-18 05:41:28.701492'::timestamp(6), '2024-12-18 05:41:28.701492'::timestamp(6)),
    ( 1005, '3' , '3'           , '4' , NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'ftc'                 , NULL, NULL, '2024-12-18 05:41:28.708774'::timestamp(6), '2024-12-18 05:41:28.708774'::timestamp(6)),
    ( 1005, '4' , '4'           , '5' , NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'ftc'                 , NULL, NULL, '2024-12-18 05:41:28.717903'::timestamp(6), '2024-12-18 05:41:28.717903'::timestamp(6)),
    ( 1005, '5' , '5'           , '6' , NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'ftc'                 , NULL, NULL, '2024-12-18 05:41:28.725913'::timestamp(6), '2024-12-18 05:41:28.725913'::timestamp(6)),
    ( 1005, '6' , '6'           , '7' , NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'ftc'                 , NULL, NULL, '2024-12-18 05:41:28.734314'::timestamp(6), '2024-12-18 05:41:28.734314'::timestamp(6)),
    ( 1005, '7' , '7'           , '8' , NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'ftc'                 , NULL, NULL, '2024-12-18 05:41:28.742611'::timestamp(6), '2024-12-18 05:41:28.742611'::timestamp(6)),
    ( 1005, '8' , '8'           , '9' , NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'ftc'                 , NULL, NULL, '2024-12-18 05:41:28.749958'::timestamp(6), '2024-12-18 05:41:28.749958'::timestamp(6)),
    ( 1005, '9' , '9'           , '10', NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'ftc'                 , NULL, NULL, '2024-12-18 05:41:28.757834'::timestamp(6), '2024-12-18 05:41:28.757834'::timestamp(6)),
    ( 1005, 'A' , 'A'           , '11', NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'ftc'                 , NULL, NULL, '2024-12-18 05:41:28.764598'::timestamp(6), '2024-12-18 05:41:28.764598'::timestamp(6)),
    ( 1005, 'B' , 'B'           , '12', NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'ftc'                 , NULL, NULL, '2024-12-18 05:41:28.771488'::timestamp(6), '2024-12-18 05:41:28.771488'::timestamp(6)),
    ( 1005, 'C' , 'C'           , '13', NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'ftc'                 , NULL, NULL, '2024-12-18 05:41:28.777046'::timestamp(6), '2024-12-18 05:41:28.777046'::timestamp(6)),
    ( 1005, 'D' , 'D'           , '14', NULL, NULL, NULL                    , '1', NULL, 2, NULL, 'ftc'                 , NULL, NULL, '2024-12-18 06:08:07.841953'::timestamp(6), '2024-12-18 06:08:07.841953'::timestamp(6)),

    -- ============ 1006 数据类型（5 条） ============
    ( 1006, '0' , '字符'          , NULL, NULL, NULL, NULL                    , '0', NULL, 2, NULL, 'trade,pbs'           , NULL, NULL, '2024-12-18 14:36:58.212744'::timestamp(6), '2024-12-18 14:36:58.212744'::timestamp(6)),
    ( 1006, '1' , '整数'          , NULL, NULL, NULL, NULL                    , '0', NULL, 2, NULL, 'trade,pbs'           , NULL, NULL, '2024-12-18 14:36:58.212744'::timestamp(6), '2024-12-18 14:36:58.212744'::timestamp(6)),
    ( 1006, '2' , '浮点数'         , NULL, NULL, NULL, NULL                    , '0', NULL, 2, NULL, 'trade,pbs'           , NULL, NULL, '2026-04-13 23:48:31.995905'::timestamp(6), '2026-04-13 23:48:31.995905'::timestamp(6)),
    ( 1006, '3' , '布尔'          , NULL, NULL, NULL, NULL                    , '0', NULL, 2, NULL, 'trade,pbs'           , NULL, NULL, '2026-04-13 23:49:48.768565'::timestamp(6), '2026-04-13 23:49:48.768565'::timestamp(6)),
    ( 1006, '4' , '字符串'         , NULL, NULL, NULL, NULL                    , '0', NULL, 2, NULL, 'trade,pbs'           , NULL, NULL, '2026-04-13 23:49:46.117641'::timestamp(6), '2026-04-13 23:49:46.117641'::timestamp(6))

ON CONFLICT (dict_no, sub_key) DO NOTHING;

-- ---------------------------------------------------------------------
-- 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    n1001 INT; n1002 INT; n1003 INT; n1004 INT; n1005 INT; n1006 INT;
    bad   INT;
BEGIN
    SELECT count(*) INTO n1001 FROM sys.sys_dict_sub WHERE dict_no = 1001;
    SELECT count(*) INTO n1002 FROM sys.sys_dict_sub WHERE dict_no = 1002;
    SELECT count(*) INTO n1003 FROM sys.sys_dict_sub WHERE dict_no = 1003;
    SELECT count(*) INTO n1004 FROM sys.sys_dict_sub WHERE dict_no = 1004;
    SELECT count(*) INTO n1005 FROM sys.sys_dict_sub WHERE dict_no = 1005;
    SELECT count(*) INTO n1006 FROM sys.sys_dict_sub WHERE dict_no = 1006;

    IF n1001 < 2 OR n1002 < 2 OR n1003 < 3 OR n1004 < 2 OR n1005 < 14 OR n1006 < 5 THEN
        RAISE EXCEPTION 'V100029: 子项不足（1001=% 1002=% 1003=% 1004=% 1005=% 1006=%）',
            n1001, n1002, n1003, n1004, n1005, n1006;
    END IF;

    -- 全量：access_level=2、时间戳非空
    SELECT count(*) INTO bad
      FROM sys.sys_dict_sub
     WHERE dict_no BETWEEN 1001 AND 1006
       AND (access_level <> 2 OR gmt_create IS NULL OR gmt_modify IS NULL);
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100029: access_level/时间戳不符（% 行）', bad;
    END IF;

    -- 1001：0 否 / 1 是
    IF NOT EXISTS (SELECT 1 FROM sys.sys_dict_sub WHERE dict_no=1001 AND sub_key='0' AND sub_val='否' AND flag_enable='1') THEN
        RAISE EXCEPTION 'V100029: 1001/0 应为「否」';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM sys.sys_dict_sub WHERE dict_no=1001 AND sub_key='1' AND sub_val='是' AND flag_enable='1') THEN
        RAISE EXCEPTION 'V100029: 1001/1 应为「是」';
    END IF;

    -- 1002：两条都是「否」（原库如此）
    IF (SELECT count(*) FROM sys.sys_dict_sub WHERE dict_no=1002 AND sub_val='否') < 2 THEN
        RAISE EXCEPTION 'V100029: 1002 两值应均为「否」';
    END IF;

    -- 1003：可修改 / 只读 / 隐藏
    IF (SELECT count(*) FROM sys.sys_dict_sub
         WHERE dict_no=1003 AND ((sub_key='1' AND sub_val='可修改')
                              OR (sub_key='2' AND sub_val='只读')
                              OR (sub_key='3' AND sub_val='隐藏'))) < 3 THEN
        RAISE EXCEPTION 'V100029: 1003 取值不符';
    END IF;

    -- 1004：sub_sort 1/2、description 非空、en_sys_str 为空串
    SELECT count(*) INTO bad
      FROM sys.sys_dict_sub
     WHERE dict_no=1004
       AND NOT (sub_sort IN ('1','2') AND coalesce(description,'') <> '' AND en_sys_str = '');
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100029: 1004 sub_sort/description/en_sys_str 不符（% 行）', bad;
    END IF;

    -- 1005：sub_val=sub_key、sub_sort 1-14、en_sys_str='ftc'
    SELECT count(*) INTO bad
      FROM sys.sys_dict_sub
     WHERE dict_no=1005 AND NOT (sub_val = sub_key AND en_sys_str = 'ftc' AND sub_sort::int BETWEEN 1 AND 14);
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100029: 1005 字段不符（% 行）', bad;
    END IF;

    -- 1006：flag_enable='0'，0 字符 / 1 整数
    SELECT count(*) INTO bad
      FROM sys.sys_dict_sub
     WHERE dict_no=1006 AND NOT (flag_enable = '0' AND en_sys_str = 'trade,pbs');
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100029: 1006 flag_enable/en_sys_str 不符（% 行）', bad;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM sys.sys_dict_sub WHERE dict_no=1006 AND sub_key='0' AND sub_val='字符') THEN
        RAISE EXCEPTION 'V100029: 1006/0 应为「字符」';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM sys.sys_dict_sub WHERE dict_no=1006 AND sub_key='1' AND sub_val='整数') THEN
        RAISE EXCEPTION 'V100029: 1006/1 应为「整数」';
    END IF;

    RAISE NOTICE 'V100029: 通用子项就绪（1001=% 1002=% 1003=% 1004=% 1005=% 1006=%）',
        n1001, n1002, n1003, n1004, n1005, n1006;
END $$;

COMMIT;
