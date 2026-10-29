-- =====================================================================
-- FinvQuant PostgreSQL V100030：sys.sys_dict_sub 国籍（3105）增量子项（102 条）
--
-- 决策（ACANX 2026-10-25）：
--   - 本文件属于**种子段**（V100000+），承接 V100028 的 3105 国籍数据：
--     V100028 只到 470（马耳他），本批补齐 474（马提尼克）… 894（赞比亚），
--     使 3105 覆盖完整的 ISO 3166-1 numeric 列表（共 235 条）。
--   - 同批提供的 3103 国籍/地区字典（MAC…ZWE）与 3104 电话区号（5 条）与
--     V100028 完全一致，库中已存在，本迁移不重复插入。
--   - 字段口径与 V100028 一致：flag_enable='0'、access_level=2、
--     en_sys_str='trade'，时间戳沿用原库值。
--   - 笔误原样保留（以原库数据为准）：'242' 斐汶、'239' 南乔治亚岛和南桑德、
--     '624' 几内亚比招、'795' 土库存曼斯坦。
--   - 幂等策略：INSERT ... ON CONFLICT (dict_no, sub_key) DO NOTHING。
-- 迁移策略：单事务、失败整体回滚；末尾校验 3105 总数不少于 235 且首尾/约束正确。
-- =====================================================================

BEGIN;

INSERT INTO sys.sys_dict_sub
    (dict_no, sub_key, sub_val, sub_sort, name_en, remark, description, flag_enable,
     associate_sub, access_level, data_level, en_sys_str, create_by, modify_by,
     gmt_modify, gmt_create)
VALUES
    ( 3105, '474' , '马提尼克'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '478' , '毛里塔尼亚'                     , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '480' , '毛里求斯'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '484' , '墨西哥'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '492' , '摩纳哥'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '496' , '蒙古'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '498' , '摩尔多瓦'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '500' , '蒙特塞拉特'                     , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '504' , '摩洛哥'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '508' , '莫桑比克'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '512' , '阿曼'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '516' , '纳米比亚'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '520' , '瑙鲁'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '524' , '尼泊尔'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '528' , '荷兰'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '530' , '荷属安的列斯'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '533' , '阿鲁巴'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '540' , '新喀里多尼亚'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '548' , '瓦努阿图'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '554' , '新西兰'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '570' , '纽埃'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '574' , '诺福克岛'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '578' , '挪威'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '580' , '北马里亚纳'                     , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '581' , '美属太平洋各群岛'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '583' , '密克罗尼西亚'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '584' , '马绍尔群岛'                     , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '585' , '贝劳'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '586' , '巴基斯坦'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '591' , '巴拿马'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '598' , '巴布亚新几内亚'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '600' , '巴拉圭'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '604' , '秘鲁'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '608' , '菲律宾'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '612' , '皮特凯恩群岛'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '616' , '波兰'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '620' , '葡萄牙'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '624' , '几内亚比招'                     , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '626' , '东帝汶'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '630' , '波多黎各'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '634' , '卡塔尔'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '638' , '留尼汪'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '642' , '罗马尼亚'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '643' , '俄罗斯'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '646' , '卢旺达'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '654' , '圣赫勒拿'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '659' , '圣基茨和尼维斯'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '660' , '安圭拉'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '662' , '圣卢西亚'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '666' , '圣皮埃尔和密克隆'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '670' , '圣文森特和格林纳丁斯'                , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '674' , '圣马力诺'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '678' , '圣多美和普林西比'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '682' , '沙特阿拉伯'                     , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '686' , '塞内加尔'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '690' , '塞舌尔'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '694' , '塞拉利昂'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '702' , '新加坡'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '703' , '斯洛伐克'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '704' , '越南'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '705' , '斯洛文尼亚'                     , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '706' , '索马里'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '710' , '南非'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '716' , '津巴布韦'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '724' , '西班牙'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '732' , '西撒哈拉'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '736' , '苏丹'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '740' , '苏里南'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '744' , '斯瓦尔巴群岛'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '748' , '斯威士兰'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '752' , '瑞典'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '756' , '瑞士'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '760' , '叙利亚'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '762' , '塔吉克斯坦'                     , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '764' , '泰国'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '768' , '多哥'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '772' , '托克劳'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '776' , '汤加'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '780' , '特立尼达和多巴哥'                  , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '784' , '阿联酋'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '788' , '突尼斯'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '792' , '土耳其'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '795' , '土库存曼斯坦'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '796' , '特克斯和凯科斯群岛'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '798' , '图瓦卢'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '800' , '乌干达'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '804' , '乌克兰'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '807' , '马其顿'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '818' , '埃及'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '826' , '英国'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '834' , '坦桑尼亚'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '840' , '美国'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '850' , '美属维尔京群岛'                   , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '854' , '布基纳法索'                     , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '858' , '乌拉圭'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '860' , '乌兹别克斯坦'                    , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '862' , '委内瑞拉'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '876' , '瓦利斯和富图纳群岛'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '882' , '西萨摩亚'                      , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '887' , '也门'                        , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '891' , '南斯拉夫联盟共和国'                 , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6)),
    ( 3105, '894' , '赞比亚'                       , NULL, NULL, NULL, NULL, '0', NULL, 2, NULL, 'trade', NULL, NULL, '2024-12-19 15:16:44.652166'::timestamp(6), '2024-12-19 15:16:44.652166'::timestamp(6))

ON CONFLICT (dict_no, sub_key) DO NOTHING;

-- ---------------------------------------------------------------------
-- 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    n3105 INT;
    nnew  INT;
    bad   INT;
BEGIN
    SELECT count(*) INTO n3105 FROM sys.sys_dict_sub WHERE dict_no = 3105;
    IF n3105 < 235 THEN
        RAISE EXCEPTION 'V100030: 3105 子项不足（实际 %，预期不少于 235）', n3105;
    END IF;

    -- 本批 102 条是否齐备（按 sub_key 清单）
    SELECT count(*) INTO nnew
      FROM sys.sys_dict_sub
     WHERE dict_no = 3105
       AND sub_key IN ('474','478','480','484','492','496','498','500','504','508','512',
                       '516','520','524','528','530','533','540','548','554','570','574',
                       '578','580','581','583','584','585','586','591','598','600','604',
                       '608','612','616','620','624','626','630','634','638','642','643',
                       '646','654','659','660','662','666','670','674','678','682','686',
                       '690','694','702','703','704','705','706','710','716','724','732',
                       '736','740','744','748','752','756','760','762','764','768','772',
                       '776','780','784','788','792','795','796','798','800','804','807',
                       '818','826','834','840','850','854','858','860','862','876','882',
                       '887','891','894');
    IF nnew <> 102 THEN
        RAISE EXCEPTION 'V100030: 3105 本批增量缺失（实际 %，预期 102）', nnew;
    END IF;

    -- 首尾
    IF NOT EXISTS (SELECT 1 FROM sys.sys_dict_sub WHERE dict_no=3105 AND sub_key='474' AND sub_val='马提尼克') THEN
        RAISE EXCEPTION 'V100030: 3105/474 应为「马提尼克」';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM sys.sys_dict_sub WHERE dict_no=3105 AND sub_key='894' AND sub_val='赞比亚') THEN
        RAISE EXCEPTION 'V100030: 3105/894 应为「赞比亚」';
    END IF;

    -- 全量口径
    SELECT count(*) INTO bad
      FROM sys.sys_dict_sub
     WHERE dict_no = 3105
       AND NOT (flag_enable = '0' AND access_level = 2 AND en_sys_str = 'trade'
                AND gmt_create IS NOT NULL AND gmt_modify IS NOT NULL);
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100030: 3105 存在口径不符的行（% 行）', bad;
    END IF;

    RAISE NOTICE 'V100030: 3105 国籍子项补齐（当前 % 条，本批新增/命中 102 条）', n3105;
END $$;

COMMIT;
