-- =====================================================================
-- V300094：更新 39002（BRS 对象存储配置）的 remark 字段说明
--
-- 需求 10912：39002 的配置格式从
--   {"enabled":…,"provider":"s3",…,"access_key":…,"secret_key":…}
-- 改为
--   {"enable":…,"protocol":"s3","provider":"cloudflare-r2",…,"ak":…,"sk":…}
-- 即：enabled→enable、原 provider（协议）拆为 protocol（协议）+ provider（厂商）。
--
-- 本迁移**只改 remark 文本**，不动 json_value：
--   · 旧 json_value 由服务层兼容读取（enable 优先 enabled、protocol 兼容误拼 protocal、
--     provider="s3" 按 endpoint 推断厂商），升级后不配置也能继续用；
--   · 用户下次在页面保存时，服务层按新格式重写 json_value。
--
-- 幂等：39002 不存在则跳过；重复执行结果一致。
-- =====================================================================

BEGIN;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM sys.sys_config WHERE config_no = 39002) THEN
        RAISE NOTICE 'V300094：39002 不存在，跳过 remark 更新';
        RETURN;
    END IF;

    UPDATE sys.sys_config
       SET remark = $remark$BRS 备份上传的 S3 兼容对象存储配置（JSON）。
字段说明：
- enable：true 启用上传；false 仅本地备份（默认 false）。
- protocol：传输协议。s3 = S3 兼容协议（默认）；local = 本地目录适配器（开发/离线）。
- provider：服务厂商标签，仅作标识，S3 兼容厂商统一走 s3 适配器。
  常用值：cloudflare-r2 / aws-s3 / minio / local。
  留空或填 s3 时按 endpoint 推断：含 r2.cloudflarestorage.com → cloudflare-r2，否则 aws-s3。
- endpoint：对象存储地址。Cloudflare R2 填 https://<account_id>.r2.cloudflarestorage.com；
  AWS S3 可留空（走默认域名）；MinIO 填 http://<host>:9000。只填 scheme://host[:port]，不得带路径。
  protocol=local 时填本地目录绝对路径。
- bucket：桶名，需提前创建。
- region：R2 用 auto；AWS 填实际区域（如 ap-east-1）；MinIO 常用 us-east-1。
- ak / sk：访问凭证。R2 获取方式：Cloudflare Dashboard → R2 → Manage R2 API Tokens →
  Create API Token，权限选 Object Read & Write，保存 Access Key ID 与 Secret Access Key（Secret 只显示一次）。
- prefix：对象 key 前缀，建议 brs/；最终 key 为 <prefix>/<env>/<yyyyMMdd>/<db>.<schema>.<table>.dump。
R2 示例：
{"enable":true,"protocol":"s3","provider":"cloudflare-r2","endpoint":"https://<account_id>.r2.cloudflarestorage.com","bucket":"finvquant-backup","region":"auto","ak":"<ACCESS_KEY>","sk":"<SECRET_KEY>","prefix":"BRS/Finv/"}
兼容说明（需求 10912）：
- 旧键 enabled 仍可读（enable 优先）；误拼 protocal 仅兼容读取，不再写出；
- 旧格式 provider="s3" 视为「只有协议」，厂商按 endpoint 推断；
- 旧凭证键 access_key / secret_key 兜底读取；保存时统一写 ak / sk 与新键。
注意事项：
1) 修改后下一轮备份生效（无需重启）；
2) 上传失败会保留本地文件并记日志，不影响本地备份；
3) 凭证只存本表，禁止写入代码仓库或日志；建议用 access_level 限制读取权限；
4) 桶权限最小化（仅 Object Read & Write）；建议开启生命周期规则清理旧备份；
5) 表级开关：brs.brs_bak_rdb_table_full.flag_upload（'1' 上传 / '0' 只留本地）。$remark$,
           gmt_modify = CURRENT_TIMESTAMP(6),
           modify_by  = 'migration'
     WHERE config_no = 39002;

    RAISE NOTICE 'V300094：39002 remark 已更新为新配置格式说明';
END $$;

COMMIT;
