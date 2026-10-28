-- =====================================================================
-- V200061：BRS 对象存储上传（二期）
--
-- 需求 10801：
--   · brs.brs_bak_rdb_table 增加 flag_upload（表级上传开关，默认 '1'）；
--   · sys.sys_config 39002 保存对象存储配置（JSON，默认 enabled=false）；
--   · 客户端通过 internal/meta/base/oss 抽象，默认适配器 s3（aws-sdk-go-v2），
--     不使用 minio-go 及 MinIO 官方库。
-- =====================================================================

BEGIN;

ALTER TABLE brs.brs_bak_rdb_table
    ADD COLUMN IF NOT EXISTS flag_upload char(1) NOT NULL DEFAULT '1';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'ck_brs_bak_rdb_table_flag_upload'
           AND conrelid = 'brs.brs_bak_rdb_table'::regclass
    ) THEN
        ALTER TABLE brs.brs_bak_rdb_table
            ADD CONSTRAINT ck_brs_bak_rdb_table_flag_upload CHECK (flag_upload IN ('0', '1'));
    END IF;
END $$;

COMMENT ON COLUMN brs.brs_bak_rdb_table.flag_upload IS '上传开关：1 上传 / 0 只留本地';

-- 对象存储配置（JSON；enabled=false 时不上传）
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value, json_value, remark)
VALUES
    (39002, 'BRS对象存储配置', '4', 0, '', '',
     '{"enabled":false,"provider":"s3","endpoint":"","bucket":"","region":"auto","access_key":"","secret_key":"","prefix":"brs/"}'::json,
     'S3 兼容对象存储；enabled=false 时仅本地备份')
ON CONFLICT (config_no) DO NOTHING;

COMMIT;
