-- =====================================================================
-- V300093：brs.brs_bak_rdb_datasource 增加一条 finv 示例数据源
--
-- 需求 10909 补充：表已建（V300092）但为空，先放一条 finv 示例，
-- 便于后续实现数据源维护页面/连接测试时直接有可编辑的样本。
--
-- 示例值说明：
--   · code/name     = finv / finv 示例数据源
--   · host/port/db  = 127.0.0.1 / 5432 / finv（Native 直连；容器部署改 host 为服务名）
--   · username      = finv
--   · password_cipher 留空 → 业务实现后回退环境变量 FINV_PG_PASSWORD（不在库内明文存口令）
--   · is_default    = '1'（若表中已存在其它默认数据源则本条插为 '0'，避免部分唯一索引冲突）
--
-- 幂等：code='finv' 已存在时跳过；重复执行不会新增行或改动既有数据。
-- 注意：业务功能未实现，本条仅为示例数据，备份链路**不会读取**。
-- =====================================================================

BEGIN;

INSERT INTO brs.brs_bak_rdb_datasource
    (code, name, rdb_type, host, port, database_name, username, password_cipher,
     ssl_mode, connect_timeout, is_default, flag_enable, priority, remark, creator)
SELECT 'finv',
       'finv 示例数据源',
       'postgresql',
       '127.0.0.1',
       5432,
       'finv',
       'finv',
       NULL,
       'prefer',
       10,
       CASE
           WHEN EXISTS (SELECT 1 FROM brs.brs_bak_rdb_datasource WHERE is_default = '1') THEN '0'
           ELSE '1'
       END,
       '1',
       100,
       '示例：当前 finv 实例；password_cipher 为空时回退 FINV_PG_PASSWORD；业务未实现，备份链路暂不读取',
       'migration'
 WHERE NOT EXISTS (SELECT 1 FROM brs.brs_bak_rdb_datasource WHERE code = 'finv');

DO $$
DECLARE
    v_rows integer;
BEGIN
    SELECT count(*) INTO v_rows FROM brs.brs_bak_rdb_datasource WHERE code = 'finv';
    IF v_rows = 0 THEN
        RAISE EXCEPTION 'V300093 校验失败：finv 示例数据源未写入';
    END IF;
    RAISE NOTICE 'V300093：finv 示例数据源已存在（共 % 条匹配）', v_rows;
END $$;

COMMIT;
