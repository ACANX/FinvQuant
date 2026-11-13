-- =====================================================================
-- V300082：BRS 对象存储配置（39002）JSON 键名 access_key / secret_key → ak / sk
--
-- 需求 10891：配置里的凭证字段统一缩写成 ak / sk（与 oss 适配器的入参命名对齐）。
--   · 数据库 json_value 内键名改为 ak / sk；
--   · HTTP 接口与前端展示**保持** access_key / secret_key（wire 契约不变），
--     由服务层做映射，前端标签仍是「Access Key ID」「Secret Access Key」。
--
-- 兼容：存量配置可能仍是旧键名，本次迁移就地改名并保留原值；
--   服务层读取时也兼容旧键（见 internal/meta/finv/brs/ossconfig.go），
--   因此迁移与代码升级的先后顺序不影响已有密钥。
--
-- 幂等：仅当 jsonb 存在 'access_key' 键时才执行；重跑不产生变化。
-- 迁移策略：单事务、失败整体回滚；末尾收敛校验。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 旧键改名（json 类型无 - / || 运算符，先转 jsonb 再转回 json）
-- ---------------------------------------------------------------------
UPDATE sys.sys_config
   SET json_value = (
         (json_value::jsonb - 'access_key' - 'secret_key')
         || jsonb_build_object(
                'ak', COALESCE(json_value::jsonb ->> 'access_key', json_value::jsonb ->> 'ak', ''),
                'sk', COALESCE(json_value::jsonb ->> 'secret_key', json_value::jsonb ->> 'sk', '')
            )
       )::json,
       gmt_modify = CURRENT_TIMESTAMP(6)
 WHERE config_no = 39002
   AND json_value IS NOT NULL
   AND json_value::jsonb ? 'access_key';

-- ---------------------------------------------------------------------
-- 2. 收敛校验：不得残留旧键；若配置存在则必须带 ak / sk
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_raw jsonb;
BEGIN
    SELECT json_value::jsonb INTO v_raw
      FROM sys.sys_config
     WHERE config_no = 39002;

    IF v_raw IS NULL THEN
        RAISE NOTICE 'V300082：配置 39002 不存在，跳过校验';
        RETURN;
    END IF;

    IF v_raw ? 'access_key' OR v_raw ? 'secret_key' THEN
        RAISE EXCEPTION 'V300082 校验失败：39002 仍残留旧键 access_key / secret_key';
    END IF;

    IF NOT (v_raw ? 'ak') OR NOT (v_raw ? 'sk') THEN
        RAISE EXCEPTION 'V300082 校验失败：39002 缺少 ak / sk 键';
    END IF;
END $$;

COMMIT;
