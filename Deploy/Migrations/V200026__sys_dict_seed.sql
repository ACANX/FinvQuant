-- =====================================================================
-- FinvQuant PostgreSQL V100026：sys.sys_dict 初始字典定义（19 条）
--
-- 决策（ACANX 2026-10-25）：
--   - 本文件属于**种子段**（V100000+），在所有结构迁移之后执行；依赖
--     V71（sys schema）与 V73（sys.sys_dict 表结构）。
--   - 数据来源：ACANX 提供的原系统字典导出，dict_no 与各字段值、时间戳
--     原样保留（gmt_create / gmt_modify 显式赋值，插入不受触发器影响）。
--   - 本批只发放**字典主表定义**（普通标志、Meta 业务字段类型、存取级别、
--     分页模式、字段释义键序列、数据类型、字典分类、管理级别、系统类型、
--     邮件类型、民族、国籍/地区相关、证件类别、国标性别代码、币种等 19 项）；
--     sys_dict_sub 的子项数据另行提供后再补种子迁移。
--   - 幂等策略：INSERT ... ON CONFLICT (dict_no) DO NOTHING ——
--     重复执行不报错，也不覆盖生产环境已调整过的字典定义。
-- 迁移策略：单事务、失败整体回滚；末尾校验 19 个 dict_no 齐备且关键字段符合。
-- =====================================================================

BEGIN;

INSERT INTO sys.sys_dict
    (dict_no, dict_name, key_type, val_type, remark, gmt_create, gmt_modify,
     ref_spec, associate_dict, access_level, manage_level, data_level,
     en_branch, en_sub_sys, enum_class, dict_category)
VALUES
    (1001, '普通标志', '1', '0', '',
     '2024-12-19 15:39:23.208280'::timestamp(6),
     '2024-12-19 15:39:23.208280'::timestamp(6),
     NULL, NULL, 1, '0', '2', NULL, 'cic', NULL, NULL),

    (1002, 'Meta业务字段类型', '1', '0', NULL,
     '2024-12-19 15:39:22.332988'::timestamp(6),
     '2024-12-19 15:39:22.332988'::timestamp(6),
     NULL, NULL, 2, '0', '2', NULL, 'cic', NULL, NULL),

    (1003, '存取级别', '1', '0', '2223333',
     '2024-12-19 15:39:21.714433'::timestamp(6),
     '2024-12-19 15:39:21.714433'::timestamp(6),
     NULL, NULL, 2, '0', '2', NULL, 'pbs', NULL, NULL),

    (1004, '分页模式', '1', '0', NULL,
     '2024-12-19 15:39:20.839617'::timestamp(6),
     '2024-12-19 15:39:20.839617'::timestamp(6),
     NULL, NULL, 2, '0', '2', NULL, 'cic', NULL, NULL),

    (1005, '字段释义键序列', '1', '2', '字段释义键序列',
     '2024-12-19 15:39:20.125354'::timestamp(6),
     '2024-12-19 15:39:20.125354'::timestamp(6),
     NULL, NULL, 2, '0', '2', NULL, 'pbs', NULL, '5'),

    (1006, '数据类型(字典？)', '1', '0', NULL,
     '2024-12-19 15:39:19.408299'::timestamp(6),
     '2024-12-19 15:39:19.408299'::timestamp(6),
     NULL, NULL, 2, '0', '2', NULL, 'cic', NULL, NULL),

    (1007, '字典分类', '1', '0', NULL,
     '2024-12-19 15:39:18.215209'::timestamp(6),
     '2024-12-19 15:39:18.215209'::timestamp(6),
     NULL, NULL, 2, '0', '2', NULL, 'pbs', NULL, NULL),

    (1008, '管理级别', '1', '1', NULL,
     '2024-12-19 15:39:17.264117'::timestamp(6),
     '2024-12-19 15:39:17.264117'::timestamp(6),
     NULL, NULL, 2, '0', '2', NULL, 'elg', NULL, NULL),

    (1009, '数据类型', '1', '0', NULL,
     '2024-12-19 22:43:07.424602'::timestamp(6),
     '2024-12-19 22:43:07.424602'::timestamp(6),
     NULL, NULL, 1, '0', '2', NULL, 'cic', NULL, '5'),

    (1301, '系统类型', '1', '2', NULL,
     '2024-12-19 15:39:16.641587'::timestamp(6),
     '2024-12-19 15:39:16.641587'::timestamp(6),
     NULL, NULL, 2, '0', '2', NULL, 'pbs', NULL, '5'),

    (3004, '邮件类型', '1', '0', NULL,
     '2024-12-19 15:25:34.825480'::timestamp(6),
     '2024-12-19 15:25:34.825480'::timestamp(6),
     NULL, NULL, 1, '0', '2', NULL, 'mnp', NULL, 'F'),

    (3101, '民族', '1', '2', NULL,
     '2024-12-19 15:25:36.703232'::timestamp(6),
     '2024-12-19 15:25:36.703232'::timestamp(6),
     NULL, NULL, 1, '0', '2', NULL, 'cic', NULL, '2'),

    (3103, '国籍/地区字典', '1', '2', NULL,
     '2024-12-19 15:25:49.078424'::timestamp(6),
     '2024-12-19 15:25:49.078424'::timestamp(6),
     NULL, NULL, 1, '0', '2', NULL, 'cic', NULL, '2'),

    (3104, '国籍/地区字母代码对应电话区号字典', '1', '2', NULL,
     '2024-12-19 15:39:06.595235'::timestamp(6),
     '2024-12-19 15:39:06.595235'::timestamp(6),
     NULL, NULL, 1, '0', '2', NULL, 'foc', NULL, 'T'),

    (3105, '国籍', '1', '2', NULL,
     '2024-12-19 15:39:08.312214'::timestamp(6),
     '2024-12-19 15:39:08.312214'::timestamp(6),
     NULL, NULL, 1, NULL, '2', NULL, 'ses', NULL, '5'),

    (3107, '证件类别', '1', '0', NULL,
     '2024-12-19 15:26:01.040992'::timestamp(6),
     '2024-12-19 15:26:01.040992'::timestamp(6),
     NULL, NULL, 1, '0', '2', NULL, 'cic', NULL, NULL),

    (3110, '国标性别代码', '1', '0', NULL,
     '2024-12-19 23:22:00.426537'::timestamp(6),
     '2024-12-19 23:22:00.426537'::timestamp(6),
     NULL, NULL, 1, '0', '2', NULL, 'cic', NULL, '2'),

    (3121, '外汇交易币种', '1', '2', NULL,
     '2024-12-19 15:35:50.446678'::timestamp(6),
     '2024-12-19 15:35:50.446678'::timestamp(6),
     NULL, NULL, 1, '0', '2', NULL, 'act', NULL, '2'),

    (3122, '注册资本币种', '1', '2', NULL,
     '2024-12-19 15:37:43.902447'::timestamp(6),
     '2024-12-19 15:37:43.902447'::timestamp(6),
     NULL, NULL, 1, '0', '2', NULL, 'cic', NULL, '2')
ON CONFLICT (dict_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 收敛校验：19 条齐备 + 关键字段抽查
-- ---------------------------------------------------------------------
DO $$
DECLARE
    n     INT;
    bad   INT;
    nosub INT;
BEGIN
    SELECT count(*) INTO n
      FROM sys.sys_dict
     WHERE dict_no IN (1001, 1002, 1003, 1004, 1005, 1006, 1007, 1008, 1009,
                       1301, 3004, 3101, 3103, 3104, 3105, 3107, 3110, 3121, 3122);
    IF n <> 19 THEN
        RAISE EXCEPTION 'V100026: sys_dict 种子数据不完整（实际 %，预期 19）', n;
    END IF;

    -- 本批字典全部为 key_type='1'，且抽查各处易错字段
    SELECT count(*) INTO bad
      FROM sys.sys_dict
     WHERE dict_no IN (1001, 1002, 1003, 1004, 1005, 1006, 1007, 1008, 1009,
                       1301, 3004, 3101, 3103, 3104, 3105, 3107, 3110, 3121, 3122)
       AND (key_type <> '1'
           OR (dict_no = 1005 AND NOT (val_type = '2' AND dict_category = '5'))
           OR (dict_no = 1008 AND NOT (val_type = '1'))
           OR (dict_no = 3004 AND NOT (enum_class = 'mnp' AND dict_category = 'F'))
           OR (dict_no = 3104 AND NOT (en_sub_sys = 'foc' AND dict_category = 'T'))
           OR (dict_no = 3105 AND NOT (manage_level IS NULL AND enum_class = 'ses'))
           OR (dict_no = 3121 AND NOT (en_sub_sys = 'act'))
           OR gmt_create IS NULL OR gmt_modify IS NULL);
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100026: sys_dict 种子数据关键字段不符（% 行）', bad;
    END IF;

    SELECT count(*) INTO nosub
      FROM sys.sys_dict_sub
     WHERE dict_no IN (1001, 1002, 1003, 1004, 1005, 1006, 1007, 1008, 1009,
                       1301, 3004, 3101, 3103, 3104, 3105, 3107, 3110, 3121, 3122);
    RAISE NOTICE 'V100026: sys_dict 种子就绪（19 条；已有 dict_no 保持原值未覆盖；已存在子项 % 条）', nosub;
END $$;

COMMIT;
