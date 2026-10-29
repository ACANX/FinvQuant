-- =====================================================================
-- FinvQuant PostgreSQL V100027：sys.sys_dict_sub 初始字典子项（89 条）
--
-- 决策（ACANX 2026-10-25）：
--   - 本文件属于**种子段**（V100000+），依赖 V71（sys schema）与
--     V73（sys.sys_dict_sub 表结构），并假定 V100026 的字典主表已就绪。
--   - 数据来源：ACANX 提供的原系统字典子项导出，sub_key / sub_val /
--     name_en / en_sys_str 与时间戳原样保留。
--   - 覆盖三组字典（共 89 条）：
--       3120 币种（数字/字母键）：14 条（sub_key 0-9、A-D）
--       3121 外汇交易币种（ISO 代码键）：11 条
--       3122 注册资本币种（ISO 代码键）：64 条（含 OTH 其它，en_sys_str 为 NULL）
--     注：3120 / 3121 / 3122 三者在 V100026 中分别对应「币种类」
--     字典主表记录（3121 外汇交易币种、3122 注册资本币种）；3120 主表记录
--     尚未提供，子项先落库，待主表补齐后一并校验。
--   - 幂等策略：INSERT ... ON CONFLICT (dict_no, sub_key) DO NOTHING ——
--     重复执行不报错，也不覆盖生产环境已调整过的子项。
-- 迁移策略：单事务、失败整体回滚；末尾校验三组条数与关键字段。
-- =====================================================================

BEGIN;

INSERT INTO sys.sys_dict_sub
    (dict_no, sub_key, sub_val, sub_sort, name_en, remark, description, flag_enable,
     associate_sub, access_level, data_level, en_sys_str, create_by, modify_by,
     gmt_modify, gmt_create)
VALUES
    -- ============ 3120 币种（数字/字母键，14 条） ============
    (3120, '0', '人民币', NULL, 'CNY', NULL, NULL, '0', NULL, 2, NULL, 'gat,gax,gcq,gaa,tms,pbs', NULL, NULL, '2024-12-19 15:28:10.900191'::timestamp(6), '2024-12-19 15:28:10.900191'::timestamp(6)),
    (3120, '1', '美元',   NULL, 'USD', NULL, NULL, '0', NULL, 2, NULL, 'gat,gax,gcq,gaa,tms,pbs', NULL, NULL, '2024-12-19 15:28:10.900191'::timestamp(6), '2024-12-19 15:28:10.900191'::timestamp(6)),
    (3120, '2', '港币',   NULL, 'HKD', NULL, NULL, '0', NULL, 2, NULL, 'gat,gax,gcq,gaa,tms,pbs', NULL, NULL, '2024-12-19 15:28:10.900191'::timestamp(6), '2024-12-19 15:28:10.900191'::timestamp(6)),
    (3120, '3', '欧元',   NULL, 'EUR', NULL, NULL, '0', NULL, 2, NULL, 'gat,gcq,gax,gaa,tms,pbs', NULL, NULL, '2024-12-19 15:28:10.900191'::timestamp(6), '2024-12-19 15:28:10.900191'::timestamp(6)),
    (3120, '4', '英镑',   NULL, 'GBP', NULL, NULL, '0', NULL, 2, NULL, 'gat,gcq,gax,gaa,tms,pbs', NULL, NULL, '2024-12-19 15:28:10.900191'::timestamp(6), '2024-12-19 15:28:10.900191'::timestamp(6)),
    (3120, '5', '澳元',   NULL, 'AUD', NULL, NULL, '0', NULL, 2, NULL, 'gat,gcq,gax,gaa,tms,pbs', NULL, NULL, '2024-12-19 15:28:10.900191'::timestamp(6), '2024-12-19 15:28:10.900191'::timestamp(6)),
    (3120, '6', '加元',   NULL, 'CAD', NULL, NULL, '0', NULL, 2, NULL, 'gat,gcq,gax,gaa,tms,pbs', NULL, NULL, '2024-12-19 15:28:10.900191'::timestamp(6), '2024-12-19 15:28:10.900191'::timestamp(6)),
    (3120, '7', '新币',   NULL, 'SGD', NULL, NULL, '0', NULL, 2, NULL, 'gat,gcq,gax,gaa,tms,pbs', NULL, NULL, '2024-12-19 15:28:10.900191'::timestamp(6), '2024-12-19 15:28:10.900191'::timestamp(6)),
    (3120, '8', '日元',   NULL, 'JPY', NULL, NULL, '0', NULL, 2, NULL, 'gat,gax,gcq,gaa,tms,pbs', NULL, NULL, '2024-12-19 15:28:10.900191'::timestamp(6), '2024-12-19 15:28:10.900191'::timestamp(6)),
    (3120, '9', '澳门元', NULL, 'MOP', NULL, NULL, '0', NULL, 2, NULL, 'gat,gax,gcq,gaa,tms,pbs', NULL, NULL, '2024-12-19 15:28:10.900191'::timestamp(6), '2024-12-19 15:28:10.900191'::timestamp(6)),
    (3120, 'A', '丹麦克朗', NULL, 'DKK', NULL, NULL, '0', NULL, 2, NULL, 'gat,gax,gcq,gaa,tms,pbs', NULL, NULL, '2024-12-19 15:28:10.900191'::timestamp(6), '2024-12-19 15:28:10.900191'::timestamp(6)),
    (3120, 'B', '便士',   NULL, 'GBX', NULL, NULL, '0', NULL, 2, NULL, 'gat,gax,gcq,gaa,tms,pbs', NULL, NULL, '2024-12-19 15:28:10.900191'::timestamp(6), '2024-12-19 15:28:10.900191'::timestamp(6)),
    (3120, 'C', '马币',   NULL, 'MYR', NULL, NULL, '0', NULL, 2, NULL, 'gat,gax,gcq,gaa,tms,pbs', NULL, NULL, '2024-12-19 15:28:10.900191'::timestamp(6), '2024-12-19 15:28:10.900191'::timestamp(6)),
    (3120, 'D', '台币',   NULL, 'TWD', NULL, NULL, '0', NULL, 2, NULL, 'gat,gax,gcq,gaa,tms,pbs', NULL, NULL, '2024-12-19 15:28:10.900191'::timestamp(6), '2024-12-19 15:28:10.900191'::timestamp(6)),

    -- ============ 3121 外汇交易币种（ISO 代码键，11 条） ============
    (3121, 'AUD', '澳元',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'act', NULL, NULL, '2024-12-19 15:33:49.907193'::timestamp(6), '2024-12-19 15:33:49.907193'::timestamp(6)),
    (3121, 'CAD', '加元',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'act', NULL, NULL, '2024-12-19 15:33:49.907193'::timestamp(6), '2024-12-19 15:33:49.907193'::timestamp(6)),
    (3121, 'CHF', '瑞士法郎', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'act', NULL, NULL, '2024-12-19 15:33:49.907193'::timestamp(6), '2024-12-19 15:33:49.907193'::timestamp(6)),
    (3121, 'CNY', '人民币',   NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'act', NULL, NULL, '2024-12-19 15:33:49.907193'::timestamp(6), '2024-12-19 15:33:49.907193'::timestamp(6)),
    (3121, 'EUR', '欧元',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'act', NULL, NULL, '2024-12-19 15:33:49.907193'::timestamp(6), '2024-12-19 15:33:49.907193'::timestamp(6)),
    (3121, 'GBP', '英镑',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'act', NULL, NULL, '2024-12-19 15:33:49.907193'::timestamp(6), '2024-12-19 15:33:49.907193'::timestamp(6)),
    (3121, 'HKD', '香港元',   NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'act', NULL, NULL, '2024-12-19 15:33:49.907193'::timestamp(6), '2024-12-19 15:33:49.907193'::timestamp(6)),
    (3121, 'JPY', '日元',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'act', NULL, NULL, '2024-12-19 15:33:49.907193'::timestamp(6), '2024-12-19 15:33:49.907193'::timestamp(6)),
    (3121, 'NZD', '新西兰元', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'act', NULL, NULL, '2024-12-19 15:33:49.907193'::timestamp(6), '2024-12-19 15:33:49.907193'::timestamp(6)),
    (3121, 'SGD', '新加坡元', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'act', NULL, NULL, '2024-12-19 15:33:49.907193'::timestamp(6), '2024-12-19 15:33:49.907193'::timestamp(6)),
    (3121, 'USD', '美元',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'act', NULL, NULL, '2024-12-19 15:33:49.907193'::timestamp(6), '2024-12-19 15:33:49.907193'::timestamp(6)),

    -- ============ 3122 注册资本币种（ISO 代码键，64 条） ============
    (3122, 'AED', '阿联酋迪拉姆', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'AUD', '澳元',         NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'BGN', '保加利亚列弗', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'BHD', '巴林第纳尔',   NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'BND', '文莱元',       NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'BRL', '巴西雷亚尔',   NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'BUK', '缅甸元',       NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'BYR', '白俄罗斯卢布', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'CAD', '加元',         NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'CHF', '瑞士法郎',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'CLP', '智利比索',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'CNY', '人民币',       NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'COP', '哥伦比亚比索', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'CRC', '哥斯达黎加科朗', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'CZK', '捷克克朗',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'DKK', '丹麦克朗',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'DZD', '阿尔及利亚第纳尔', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'EGP', '埃及镑',       NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'EUR', '欧元',         NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'GBP', '英镑',         NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'HKD', '港元',         NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'HRK', '克罗地亚库纳', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'HUF', '匈牙利福林',   NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'IDR', '印尼卢比',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'ILS', '以色列新锡克尔', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'INR', '印度卢比',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'IQD', '伊拉克第纳尔', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'ISK', '冰岛克朗',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'JOD', '约旦第纳尔',   NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'JPY', '日元',         NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'KES', '肯尼亚先令',   NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'KHR', '柬埔寨瑞尔',   NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'KRW', '韩元',         NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'KWD', '科威特第纳尔', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'LAK', '老挝基普',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'LBP', '黎巴嫩镑',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'LKR', '斯里兰卡卢比', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'MAD', '摩洛哥道拉姆', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'MOP', '澳门元',       NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'MXN', '墨西哥元',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'MYR', '马来西亚林吉特', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'NOK', '挪威克朗',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'NZD', '新西兰元',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'OMR', '阿曼里亚尔',   NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'OTH', '其它',         NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL, NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'PHP', '菲律宾比索',   NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'PLN', '波兰兹罗提',   NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'QAR', '卡塔尔里亚尔', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'RON', '罗马尼亚列伊', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'RSD', '塞尔维亚第纳尔', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'RUB', '俄罗斯卢布',   NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'SAR', '沙特里亚尔',   NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'SEK', '瑞典克朗',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'SGD', '新加坡元',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'SYP', '叙利亚镑',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'THB', '泰铢',         NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'TRY', '新土耳其里拉', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'TWD', '新台币',       NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'TZS', '坦桑尼亚先令', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'UGX', '乌干达先令',   NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'USD', '美元',         NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'VND', '越南盾',       NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'ZAR', '南非兰特',     NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6)),
    (3122, 'ZMK', '新的赞比亚克瓦查', NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic', NULL, NULL, '2024-12-19 15:36:56.002946'::timestamp(6), '2024-12-19 15:36:56.002946'::timestamp(6))
ON CONFLICT (dict_no, sub_key) DO NOTHING;

-- ---------------------------------------------------------------------
-- 收敛校验：三组条数与关键字段
-- ---------------------------------------------------------------------
DO $$
DECLARE
    n3120 INT;
    n3121 INT;
    n3122 INT;
    bad   INT;
BEGIN
    SELECT count(*) INTO n3120 FROM sys.sys_dict_sub WHERE dict_no = 3120;
    SELECT count(*) INTO n3121 FROM sys.sys_dict_sub WHERE dict_no = 3121;
    SELECT count(*) INTO n3122 FROM sys.sys_dict_sub WHERE dict_no = 3122;

    IF n3120 < 14 THEN
        RAISE EXCEPTION 'V100027: 3120 子项不足（实际 %，预期 14）', n3120;
    END IF;
    IF n3121 < 11 THEN
        RAISE EXCEPTION 'V100027: 3121 子项不足（实际 %，预期 11）', n3121;
    END IF;
    IF n3122 < 64 THEN
        RAISE EXCEPTION 'V100027: 3122 子项不足（实际 %，预期 64）', n3122;
    END IF;

    -- 关键字段抽查 + 全量约束（flag_enable='0'、access_level=2）
    SELECT count(*) INTO bad
      FROM sys.sys_dict_sub
     WHERE dict_no IN (3120, 3121, 3122)
       AND (flag_enable <> '0'
            OR flag_enable IS NULL
            OR access_level <> 2
            OR gmt_create IS NULL
            OR gmt_modify IS NULL
            OR (dict_no = 3120 AND sub_key = '1'  AND NOT (sub_val = '美元' AND name_en = 'USD'))
            OR (dict_no = 3120 AND sub_key = '3'  AND NOT (sub_val = '欧元' AND en_sys_str = 'gat,gcq,gax,gaa,tms,pbs'))
            OR (dict_no = 3120 AND sub_key = '0'  AND NOT (en_sys_str = 'gat,gax,gcq,gaa,tms,pbs'))
            OR (dict_no = 3121 AND sub_key = 'HKD' AND NOT (sub_val = '香港元' AND en_sys_str = 'act'))
            OR (dict_no = 3121 AND name_en IS NOT NULL)
            OR (dict_no = 3122 AND sub_key = 'OTH' AND en_sys_str IS NOT NULL)
            OR (dict_no = 3122 AND sub_key = 'ZMK' AND NOT (sub_val = '新的赞比亚克瓦查' AND en_sys_str = 'cic')));
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100027: sys_dict_sub 数据与预期不符（% 行）', bad;
    END IF;

    RAISE NOTICE 'V100027: sys_dict_sub 种子就绪（3120=% / 3121=% / 3122=%；已有子项保持不变）',
        n3120, n3121, n3122;
END $$;

COMMIT;
