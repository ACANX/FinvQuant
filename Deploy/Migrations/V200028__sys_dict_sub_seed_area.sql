-- =====================================================================
-- FinvQuant PostgreSQL V100028：sys.sys_dict_sub 地区/国籍相关初始子项（387 条）
--
-- 决策（ACANX 2026-10-25）：
--   - 本文件属于**种子段**（V100000+），依赖 V71（sys schema）与
--     V73（sys.sys_dict_sub 表结构），并假定 V100026 的字典主表已就绪。
--   - 数据来源：ACANX 提供的原系统字典子项导出，字段值与时间戳原样保留。
--   - 覆盖三组字典（共 387 条）：
--       3103 国籍/地区字典（ISO 三字母键）：249 条，en_sys_str 多为 'trade'，
--            另有部分为 NULL 或 'cic,gft,gaa,act' / 'act,cic'，access_level 1/2
--       3104 国籍/地区字母代码对应电话区号：5 条（CHN/CTN/HKG/MAC/USA）
--       3105 国籍（ISO 数字码键）：133 条（004 阿富汗 … 470 马耳他）
--   - ⚠ 3105 本批数据在 470（马耳他）处**截断**，ISO 3166-1 numeric 后续编号
--     （484 起）尚未提供；本次先落已给部分，后续以增量种子迁移补充。
--   - 幂等策略：INSERT ... ON CONFLICT (dict_no, sub_key) DO NOTHING ——
--     重复执行不报错，也不覆盖生产环境已调整过的子项；后续增量可直接叠加。
-- 迁移策略：单事务、失败整体回滚；末尾按“不少于”校验三组条数并抽查关键字段。
-- =====================================================================

BEGIN;

INSERT INTO sys.sys_dict_sub
    (dict_no, sub_key, sub_val, sub_sort, name_en, remark, description, flag_enable,
     associate_sub, access_level, data_level, en_sys_str, create_by, modify_by,
     gmt_modify, gmt_create)
VALUES
    -- ============ 3103 国籍/地区字典（249 条） ============
    ( 3103, 'ABW' , '阿鲁巴'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ADR' , '安道尔'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'AFG' , '阿富汗'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'AGO' , '安哥拉'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'AIA' , '安圭拉'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ALB' , '阿尔巴尼亚'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ALG' , '阿尔及利亚'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ANT' , '荷属安的列斯'                , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ARE' , '阿拉伯联合酋长国'              , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ARG' , '阿根廷'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ARM' , '亚美尼亚共和国'               , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ASM' , '美属萨摩亚'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ATA' , '南极洲'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ATF' , '法属南部领土'                , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ATG' , '安提瓜和巴布达'               , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'AUS' , '澳大利亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'AUT' , '奥地利'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'AZE' , '阿塞拜疆共和国'               , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BDI' , '布隆迪'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BEL' , '比利时'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BEN' , '贝宁'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BFA' , '布基纳法索'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BGD' , '孟加拉'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BGR' , '保加利亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BHR' , '巴林'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BHS' , '巴哈马'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BIH' , '波黑'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BLR' , '白俄罗斯'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BLZ' , '伯利兹'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BMU' , '百慕大'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BOL' , '玻利维亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BRA' , '巴西'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BRB' , '巴巴多斯'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BRN' , '文莱'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BTN' , '不丹'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BVT' , '布维岛'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'BWA' , '博茨瓦纳'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CAF' , '中非共和国'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CAN' , '加拿大'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CCK' , '可可（基林）群岛'              , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CHE' , '瑞士'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CHL' , '智利'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CHN' , '中国'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CIV' , '科特迪瓦'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CMR' , '喀麦隆共和国'                , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CNI' , '海峡群岛'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'COD' , '刚果（金）'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'COG' , '刚果共和国'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'COK' , '库克群岛'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'COL' , '哥伦比亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'COM' , '科摩罗'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CPV' , '佛得角'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CRI' , '哥斯达黎加'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CSR' , '圣诞岛'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic,gft,gaa,act'   , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CTN' , '中国台湾'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CUB' , '古巴'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CYM' , '开曼群岛'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CYP' , '塞浦路斯'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'CZE' , '捷克'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'DEU' , '德国'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'DJI' , '吉布提'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'DMA' , '多米尼克联邦'                , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'DNK' , '丹麦'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'DOM' , '多米尼加'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ECU' , '厄瓜多尔'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'EGY' , '埃及'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ERI' , '厄立特里亚国'                , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ESH' , '西撒哈拉'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ESP' , '西班牙'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'EST' , '爱沙尼亚共和国'               , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ETH' , '埃塞俄比亚'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'FIN' , '芬兰'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'FJI' , '斐济'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'FLK' , '马尔维纳斯群岛'               , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'FRA' , '法国'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'FRO' , '法罗群岛'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'FSM' , '密克罗尼西亚共和国'             , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GAB' , '加蓬'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GBR' , '英国'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GEO' , '格鲁吉亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GGY' , '根西岛'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'act,cic'           , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GHA' , '加纳共和国'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GIB' , '直布罗陀'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GIN' , '几内亚'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GLP' , '瓜德罗普'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GMB' , '冈比亚'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GNB' , '几内亚比绍'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GNQ' , '赤道几内亚共和国'              , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GRC' , '希腊'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GRD' , '格林纳达'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GRL' , '格陵兰'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GTM' , '危地马拉'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GUF' , '法属圭亚那'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GUM' , '关岛'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GUS' , '格恩西'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'GUY' , '圭亚那'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'HKG' , '中国香港'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'HMD' , '赫德岛和麦克唐纳岛'             , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'HND' , '洪都拉斯'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'HRV' , '克罗地亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'HTI' , '海地共和国'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'HUN' , '匈牙利'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'IDN' , '印尼'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'IND' , '印度'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'INE' , '尼维斯岛'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'IOM' , '马恩岛'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'IOT' , '英属印度洋领土'               , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'IRL' , '爱尔兰'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'IRN' , '伊朗'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'IRQ' , '伊拉克'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ISL' , '冰岛'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ISR' , '以色列'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ITA' , '意大利'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'JAM' , '牙买加'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'JOR' , '约旦'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'JPN' , '日本'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'JSY' , '泽西'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'KAZ' , '哈萨克斯坦共和国'              , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'KEN' , '肯尼亚共和国'                , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'KGZ' , '吉尔吉斯共和国'               , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'KHM' , '柬埔寨'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'KIR' , '基里巴斯共和国'               , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'KNA' , '圣基茨和尼维斯联邦'             , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'KOR' , '韩国'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'KWT' , '科威特'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'LAO' , '老挝'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'LBN' , '黎巴嫩'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'LBR' , '利比里亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'LBY' , '利比亚'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'LCA' , '圣卢西亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'LIE' , '列支敦士登'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'LKA' , '斯里兰卡'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'LSO' , '莱索托'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'LTU' , '立陶宛共和国'                , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'LUX' , '卢森堡'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'LVA' , '拉脱维亚共和国'               , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MAC' , '中国澳门'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MAN' , '曼岛'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MAR' , '摩洛哥'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MCO' , '摩纳哥'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MDA' , '摩尔多瓦共和国'               , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MDG' , '马达加斯加'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MDV' , '马尔代夫'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MEX' , '墨西哥'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MHL' , '马绍尔群岛共和国'              , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MKD' , '马其顿共和国'                , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MLI' , '马里共和国'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MLT' , '马耳他'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MMR' , '缅甸'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MNG' , '蒙古'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MNP' , '北马里亚纳'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MOZ' , '莫桑比克'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MRT' , '毛里塔尼亚'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MSR' , '蒙特塞拉特'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MTQ' , '马提尼克'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MUS' , '毛里求斯'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MWI' , '马拉维'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MYS' , '马来西亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'MYT' , '马约特'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'NAM' , '纳米比亚共和国'               , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'NCL' , '新喀里多尼亚'                , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'NER' , '尼日尔'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'NFK' , '诺福克岛'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'NGA' , '尼日利亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'NIC' , '尼加拉瓜'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'NIU' , '纽埃'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'NLA' , '荷兰属地'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'NLD' , '荷兰'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'NOR' , '挪威'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'NPL' , '尼泊尔'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'NRU' , '瑙鲁共和国'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'NZL' , '新西兰'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'OTH' , '其他'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'OWN' , '阿曼'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'PAK' , '巴基斯坦'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'PAN' , '巴拿马'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'PCN' , '皮特凯恩群岛'                , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'PER' , '秘鲁'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'PHL' , '菲律宾'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'PLW' , '帕劳'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'PNG' , '巴布亚新几内亚独立国'            , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'POL' , '波兰'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'PRI' , '波多黎各'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'PRK' , '朝鲜'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'PRT' , '葡萄牙'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'PRY' , '巴拉圭'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'PST' , '巴勒斯坦'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic,gft,gaa,act'   , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'PYF' , '法属波利尼西亚'               , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'QAT' , '卡塔尔'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'REU' , '留尼汪'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ROM' , '罗马尼亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'RUS' , '俄罗斯联邦'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'RWA' , '卢旺达'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SAU' , '沙特阿拉伯'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SDN' , '苏丹'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SEN' , '塞内加尔'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SGP' , '新加坡'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SGS' , '南乔治亚岛和南桑德韦奇岛'          , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SHN' , '圣赫勒拿'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SJM' , '斯瓦尔巴群岛'                , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SLB' , '所罗门群岛'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SLE' , '塞拉利昂'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SLV' , '萨尔瓦多'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SMR' , '圣马力诺'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SOM' , '索马里'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SPM' , '圣皮埃尔和密克隆'              , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'STP' , '圣多美和普林西比'              , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SUR' , '苏里南共和国'                , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SVK' , '斯洛伐克'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SVN' , '斯洛文尼亚'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SWE' , '瑞典'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SWZ' , '斯威士兰'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SYC' , '塞舌尔'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'SYR' , '叙利亚'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'TCA' , '特克斯和凯科斯群岛'             , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'TCD' , '乍得'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'TGO' , '多哥'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'THA' , '泰国'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'TJK' , '塔吉克斯坦共和国'              , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'TKL' , '托克劳'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'TKM' , '土库曼斯坦'                 , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'TMP' , '东帝汶'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'cic,gft,gaa,act'   , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'TON' , '汤加'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'TTO' , '特立尼达和多巴哥'              , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'TUN' , '突尼斯'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'TUR' , '土耳其'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'TUV' , '图瓦卢'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'TZA' , '坦桑尼亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'UGA' , '乌干达共和国'                , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'UKR' , '乌克兰'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'UMI' , '美属太平洋各群岛'              , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'URY' , '乌拉圭'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'USA' , '美国'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'UZB' , '乌兹别克斯坦共和国'             , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'VAT' , '梵帝冈'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'VCT' , '圣文森特和林纳丁斯'             , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'VEN' , '委内瑞拉'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'VGB' , '英属维尔京群岛'               , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'VIR' , '美属维尔京群岛'               , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'VNM' , '越南'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'VUT' , '瓦努阿图'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'WLF' , '瓦利斯和富图纳群岛'             , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'WSM' , '萨摩亚'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, NULL                , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'YEM' , '也门'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'YUG' , '南斯拉夫'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ZAF' , '南非'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ZAR' , '扎伊尔'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ZMB' , '赞比亚'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),
    ( 3103, 'ZWE' , '津巴布韦'                  , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:11:53.762252'::timestamp(6), '2024-12-19 15:11:53.762252'::timestamp(6)),

    -- ============ 3104 国籍/地区字母代码对应电话区号（5 条） ============
    ( 3104, 'CHN' , '86'                    , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, NULL                , NULL, NULL, '2024-12-19 15:14:48.105172'::timestamp(6), '2024-12-19 15:14:48.105172'::timestamp(6)),
    ( 3104, 'CTN' , '886'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, NULL                , NULL, NULL, '2024-12-19 15:14:48.105172'::timestamp(6), '2024-12-19 15:14:48.105172'::timestamp(6)),
    ( 3104, 'HKG' , '852'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, NULL                , NULL, NULL, '2024-12-19 15:14:48.105172'::timestamp(6), '2024-12-19 15:14:48.105172'::timestamp(6)),
    ( 3104, 'MAC' , '853'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, NULL                , NULL, NULL, '2024-12-19 15:14:48.105172'::timestamp(6), '2024-12-19 15:14:48.105172'::timestamp(6)),
    ( 3104, 'USA' , '001'                   , NULL, NULL, NULL, NULL, '0', NULL, 1, NULL, NULL                , NULL, NULL, '2024-12-19 15:14:48.105172'::timestamp(6), '2024-12-19 15:14:48.105172'::timestamp(6)),

    -- ============ 3105 国籍（ISO 数字码，133 条，470 处截断） ============
    ( 3105, '004' , '阿富汗'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '008' , '阿尔巴尼亚'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '010' , '南极洲'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '012' , '阿尔及利亚'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '016' , '美属萨摩亚'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '020' , '安道尔'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '024' , '安哥拉'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '028' , '安提瓜和巴布达'               , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '031' , '阿塞疆'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '032' , '阿根廷'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '036' , '澳大利亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '040' , '奥地利'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '044' , '巴哈马'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '048' , '巴林'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '050' , '孟加拉国'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '051' , '亚美尼亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '052' , '巴巴多斯'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '056' , '比利时'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '060' , '百慕大'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '064' , '不丹'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '068' , '玻利维亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '070' , '波斯尼亚和黑塞哥维那'            , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '072' , '博茨瓦纳'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '074' , '布维岛'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '076' , '巴西'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '084' , '伯利兹'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '086' , '英属印度洋领土'               , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '090' , '所罗门群岛'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '092' , '英属维尔京群岛'               , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '096' , '文莱'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '100' , '保加利亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '104' , '缅甸'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '108' , '布隆迪'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '112' , '白俄罗斯'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '116' , '柬埔寨'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '120' , '喀麦隆'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '124' , '加拿大'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '132' , '佛得角'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '136' , '开曼群岛'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '140' , '中非'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '144' , '斯里兰卡'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '148' , '乍得'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '152' , '智利'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '156' , '中国'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '158' , '中国台湾'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '162' , '圣诞岛'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '166' , '科科斯（基林）群岛'             , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '170' , '哥伦比亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '174' , '科摩罗'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '175' , '马约特'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '180' , '扎伊尔'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '184' , '库克群岛'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '188' , '哥斯达黎加'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '191' , '克罗地亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '192' , '古巴'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '196' , '塞浦路斯'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '203' , '捷克'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '204' , '贝宁'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '208' , '丹麦'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '212' , '多米尼克'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '214' , '多米尼加共和国'               , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '218' , '厄瓜多尔'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '222' , '萨尔瓦多'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '226' , '赤道几内亚'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '231' , '埃塞俄比亚'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '232' , '厄立特里亚'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '233' , '爱沙尼亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '234' , '法罗群岛'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '238' , '马尔维纳斯群岛'               , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '239' , '南乔治亚岛和南桑德'             , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '242' , '斐汶'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '246' , '芬兰'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '250' , '法国'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '254' , '法属圭亚那'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '258' , '法属波利尼西亚'               , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '260' , '法属南部领土'                , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '262' , '吉布提'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '266' , '加蓬'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '268' , '格鲁吉亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '270' , '冈比亚'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '276' , '德国'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '288' , '加纳'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '292' , '直布罗陀'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '296' , '基里巴斯'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '300' , '希腊'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '304' , '格陵兰'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '308' , '格林纳达'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '312' , '瓜德罗普'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '316' , '关岛'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '320' , '危地马拉'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '324' , '几内亚'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '328' , '圭亚那'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '332' , '海地'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '334' , '赫德岛和麦克唐纳岛'             , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '336' , '梵蒂冈'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '340' , '洪都拉斯'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '344' , '香港'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '348' , '匈牙利'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '352' , '冰岛'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '356' , '印度'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '360' , '印度尼西亚'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '364' , '伊朗'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '368' , '伊拉克'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '372' , '爱尔兰'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '374' , '巴勒斯坦'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '376' , '以色列'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '380' , '意大利'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '384' , '科特迪瓦'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '388' , '牙买加'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '392' , '日本'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '398' , '哈萨克斯坦'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '400' , '约旦'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '404' , '肯尼亚'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '408' , '朝鲜'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '410' , '韩国'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '414' , '科威特'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '417' , '吉尔吉斯斯坦'                , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '418' , '老挝'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '422' , '黎巴嫩'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '426' , '莱索托'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '428' , '拉脱维亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '430' , '利比里亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '434' , '利比亚'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '438' , '列支敦士登'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '440' , '立陶宛'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '442' , '卢森堡'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '446' , '澳门'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '450' , '马达加斯加'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '454' , '马拉维'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '458' , '马来西亚'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '462' , '马尔代夫'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '466' , '马里'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '470' , '马耳他'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade'             , NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6))

ON CONFLICT (dict_no, sub_key) DO NOTHING;

-- ---------------------------------------------------------------------
-- 收敛校验：三组条数（不少于）+ 关键字段抽查
-- ---------------------------------------------------------------------
DO $$
DECLARE
    n3103  INT;
    n3104  INT;
    n3105  INT;
    bad    INT;
    nulls  INT;
BEGIN
    SELECT count(*) INTO n3103 FROM sys.sys_dict_sub WHERE dict_no = 3103;
    SELECT count(*) INTO n3104 FROM sys.sys_dict_sub WHERE dict_no = 3104;
    SELECT count(*) INTO n3105 FROM sys.sys_dict_sub WHERE dict_no = 3105;

    IF n3103 < 249 THEN
        RAISE EXCEPTION 'V100028: 3103 子项不足（实际 %，预期不少于 249）', n3103;
    END IF;
    IF n3104 < 5 THEN
        RAISE EXCEPTION 'V100028: 3104 子项不足（实际 %，预期不少于 5）', n3104;
    END IF;
    IF n3105 < 133 THEN
        RAISE EXCEPTION 'V100028: 3105 子项不足（实际 %，预期不少于 133）', n3105;
    END IF;

    -- 全量约束：flag_enable='0'、时间戳非空、access_level 合法
    SELECT count(*) INTO bad
      FROM sys.sys_dict_sub
     WHERE dict_no IN (3103, 3104, 3105)
       AND (flag_enable IS NULL OR flag_enable <> '0'
            OR gmt_create IS NULL OR gmt_modify IS NULL
            OR access_level NOT IN (1, 2));
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100028: sys_dict_sub 基础约束不符（% 行）', bad;
    END IF;

    -- 3103 首尾与 en_sys_str 取值集合
    SELECT count(*) INTO bad
      FROM sys.sys_dict_sub
     WHERE dict_no = 3103
       AND (en_sys_str IS NOT NULL
            AND en_sys_str NOT IN ('trade', 'cic,gft,gaa,act', 'act,cic'));
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100028: 3103 en_sys_str 出现未预期取值（% 行）', bad;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM sys.sys_dict_sub WHERE dict_no = 3103 AND sub_key = 'ABW' AND sub_val = '阿鲁巴' AND en_sys_str = 'trade' AND access_level = 1) THEN
        RAISE EXCEPTION 'V100028: 3103 首条 ABW/阿鲁巴 不符';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM sys.sys_dict_sub WHERE dict_no = 3103 AND sub_key = 'ZWE' AND sub_val = '津巴布韦' AND en_sys_str = 'trade') THEN
        RAISE EXCEPTION 'V100028: 3103 末条 ZWE/津巴布韦 不符';
    END IF;

    -- 3103 例外行抽查（access_level=2 且 en_sys_str 为 NULL）
    SELECT count(*) INTO bad
      FROM sys.sys_dict_sub
     WHERE dict_no = 3103
       AND sub_key IN ('ANT','ATF','BVT','COD','COK','DMA','ERI','EST','FLK','FSM','GEO',
                       'GLP','HMD','IOT','KGZ','LVA','MDA','MKD','MNP','MTQ','MYT','PLW',
                       'REU','SGS','SHN','SJM','SPM','TCA','TJK','UMI','UZB','VAT','VCT',
                       'WLF','WSM')
       AND NOT (access_level = 2 AND en_sys_str IS NULL);
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100028: 3103 例外行（access_level=2/NULL）不符（% 行）', bad;
    END IF;

    -- 3103 其它例外：CCK(1/NULL)、CSR/PST/TMP(2/cic,gft,gaa,act)、GGY(2/act,cic)
    SELECT count(*) INTO bad
      FROM sys.sys_dict_sub
     WHERE dict_no = 3103
       AND ((sub_key = 'CCK'  AND NOT (access_level = 1 AND en_sys_str IS NULL))
         OR (sub_key IN ('CSR','PST','TMP') AND NOT (access_level = 2 AND en_sys_str = 'cic,gft,gaa,act'))
         OR (sub_key = 'GGY'  AND NOT (access_level = 2 AND en_sys_str = 'act,cic')));
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100028: 3103 个别例外行不符（% 行）', bad;
    END IF;

    -- 3104 五个区号
    SELECT count(*) INTO bad
      FROM sys.sys_dict_sub
     WHERE dict_no = 3104
       AND ((sub_key = 'CHN' AND sub_val <> '86')
         OR (sub_key = 'CTN' AND sub_val <> '886')
         OR (sub_key = 'HKG' AND sub_val <> '852')
         OR (sub_key = 'MAC' AND sub_val <> '853')
         OR (sub_key = 'USA' AND sub_val <> '001')
         OR en_sys_str IS NOT NULL
         OR access_level <> 1);
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100028: 3104 电话号码区号数据不符（% 行）', bad;
    END IF;

    -- 3105 首尾与 en_sys_str
    IF NOT EXISTS (SELECT 1 FROM sys.sys_dict_sub WHERE dict_no = 3105 AND sub_key = '004' AND sub_val = '阿富汗' AND en_sys_str = 'trade' AND access_level = 2) THEN
        RAISE EXCEPTION 'V100028: 3105 首条 004/阿富汗 不符';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM sys.sys_dict_sub WHERE dict_no = 3105 AND sub_key = '470' AND sub_val = '马耳他' AND en_sys_str = 'trade') THEN
        RAISE EXCEPTION 'V100028: 3105 末条 470/马耳他 不符';
    END IF;

    SELECT count(*) INTO bad
      FROM sys.sys_dict_sub
     WHERE dict_no = 3105 AND NOT (en_sys_str = 'trade' AND access_level = 2);
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100028: 3105 存在非 trade/access_level=2 的行（% 行）', bad;
    END IF;

    RAISE NOTICE 'V100028: 地区/国籍子项就绪（3103=% / 3104=% / 3105=%；3105 后续编号待补）',
        n3103, n3104, n3105;
END $$;

COMMIT;
