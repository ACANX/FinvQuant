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
     $remark$BRS 备份上传的 S3 兼容对象存储配置（JSON）。
字段说明：
- enabled：true 启用上传；false 仅本地备份（默认 false）。
- provider：固定 s3（通过 internal/meta/base/oss 抽象，可扩展 local 等适配器）。
- endpoint：对象存储地址。Cloudflare R2 填 https://<account_id>.r2.cloudflarestorage.com；AWS S3 留空（走默认域名）；MinIO 填 http://<host>:9000。
- bucket：桶名，需提前创建。
- region：R2 用 auto；AWS 填实际区域（如 ap-east-1）；MinIO 常用 us-east-1。
- access_key / secret_key：访问凭证。R2 获取方式：Cloudflare Dashboard → R2 → Manage R2 API Tokens → Create API Token，权限选 Object Read & Write，保存 Access Key ID 与 Secret Access Key（Secret 只显示一次）。
- prefix：对象 key 前缀，建议 brs/；最终 key 为 <prefix>/<yyyyMMdd>/<schema>.<table>_<HHmmss>.dump。
R2 示例：
{"enabled":true,"provider":"s3","endpoint":"https://<account_id>.r2.cloudflarestorage.com","bucket":"finvquant-backup","region":"auto","access_key":"<ACCESS_KEY>","secret_key":"<SECRET_KEY>","prefix":"brs/"}
注意事项：
1) 修改后下一轮备份生效（无需重启）；
2) 上传失败会保留本地文件并记日志，不影响本地备份；
3) 凭证只存本表，禁止写入代码仓库或日志；建议用 access_level 限制读取权限；
4) 桶权限最小化（仅 Object Read & Write）；建议开启生命周期规则清理旧备份；
5) 表级开关：brs.brs_bak_rdb_table.flag_upload（'1' 上传 / '0' 只留本地）。$remark$)
ON CONFLICT (config_no) DO NOTHING;

COMMIT;
