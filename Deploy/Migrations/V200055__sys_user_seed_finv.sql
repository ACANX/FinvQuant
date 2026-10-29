-- =====================================================================
-- FinvQuant PostgreSQL V200055：sys.sys_user 初始化登录账号 finv
--
-- 决策（ACANX 2026-10-28）：
--   - 需求 10761：为系统登录创建初始账号 finv。
--   - 密码绝不落明文：采用 argon2id（m=64MB, t=3, p=2, keyLen=32），
--     盐 16 字节随机、hex 存放于 password_salt（32 字符，列宽 64）；
--     哈希以 PHC 编码存放于 password（97 字符，列宽 128）。
--   - 口令原文由需求方指定并在迁移中不出现；本迁移只写盐与哈希，
--     如需轮换口令，重新生成盐+哈希后 UPDATE 即可（见迁移末尾说明）。
--   - email 为 NOT NULL 且唯一，使用内部域名 finv@finvquant.local，
--     不指向真实邮箱，避免误发；mobile_phone_no 留 NULL（唯一约束允许多 NULL）。
--   - authority_str 取 '0'（与表默认一致，表示未细分权限）；
--     flag_delete='0' 表示未删除；create_by 记 'migration' 便于审计溯源。
--   - 幂等：以 uid 存在性判断，已存在则跳过插入并保留原密码，
--     避免重复执行时覆盖运维后续修改的口令。
-- 迁移策略：单事务、失败整体回滚；末尾做收敛校验。
-- 前置：V74（sys.sys_user）、V100081（uid 唯一索引）已应用。
--
-- 口令轮换方式（不走迁移，属运维操作）：
--   1) 用 internal/meta/finv/auth 的 GenerateSalt + Hash 生成新盐与 PHC 串；
--   2) UPDATE sys.sys_user SET password = '<新的 PHC 串>',
--        password_salt = '<新的 hex 盐>', modify_by = '<操作人>'
--      WHERE uid = 'finv';
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 插入初始账号（幂等：uid 已存在则跳过，不覆盖既有口令）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_user (
    uid, nick_name, password, password_salt, email,
    description, authority_str, language, time_zone,
    flag_delete, create_by
)
SELECT
    'finv',
    'finv',
    '$argon2id$v=19$m=65536,t=3,p=2$w3pSarYNvzy9sotvU5eE/w$AFxyMl0affWhCB8ASKgWVpvzcxdvUPn3VbcbocM5bDk',
    'c37a526ab60dbf3cbdb28b6f539784ff',
    'finv@finvquant.local',
    '系统初始登录账号（需求 10761）',
    '0',
    'zh-CN',
    'UTC+08:00',
    '0',
    'migration'
WHERE NOT EXISTS (
    SELECT 1 FROM sys.sys_user WHERE uid = 'finv'
);

-- ---------------------------------------------------------------------
-- 2. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    row_cnt   INT;
    bad_pwd   INT;
    bad_salt  INT;
    bad_flags INT;
BEGIN
    SELECT count(*) INTO row_cnt FROM sys.sys_user WHERE uid = 'finv';
    IF row_cnt <> 1 THEN
        RAISE EXCEPTION 'V200055: uid=finv 记录数为 %，预期 1', row_cnt;
    END IF;

    -- 密码必须是 argon2id PHC 编码串，且不得是明文口令
    SELECT count(*) INTO bad_pwd
      FROM sys.sys_user
     WHERE uid = 'finv'
       AND (password NOT LIKE '$argon2id$%' OR position('finv@2026' in password) > 0);
    IF bad_pwd > 0 THEN
        RAISE EXCEPTION 'V200055: password 不是合法 argon2id PHC 串或疑似明文';
    END IF;

    -- 盐必须是 32 位 hex
    SELECT count(*) INTO bad_salt
      FROM sys.sys_user
     WHERE uid = 'finv'
       AND (password_salt !~ '^[0-9a-f]{32}$');
    IF bad_salt > 0 THEN
        RAISE EXCEPTION 'V200055: password_salt 不是 32 位 hex 串';
    END IF;

    SELECT count(*) INTO bad_flags
      FROM sys.sys_user
     WHERE uid = 'finv'
       AND (flag_delete <> '0' OR authority_str IS NULL OR email IS NULL);
    IF bad_flags > 0 THEN
        RAISE EXCEPTION 'V200055: flag_delete/authority_str/email 取值不符合预期';
    END IF;

    RAISE NOTICE 'V200055: 校验通过（uid=finv 已就绪，密码为 argon2id 哈希，未落明文）';
END $$;

COMMIT;
