-- =====================================================================
-- FinvQuant PostgreSQL V300191：sys.sys_config 新增 OpenAPI 凭证缓存 TTL（19007）
--
-- 决策（ACANX 2026-11-11，需求 11214/11215）：
--   - 缓存时长属于**运行期可调参数**，放 sys_config 而不是环境变量：
--     环境变量改一次要重建容器，页面也看不到改不了。
--   - 19007「OpenAPI 凭证缓存 TTL（秒）」，data_type='1'（整数），取值存 `int_value`：
--         0        关闭缓存（每次请求都查凭证表）
--         1~600    缓存秒数（默认 10）
--     超出 [0,600] 由服务端按边界收敛（负数按 0、大于 600 按 600），读取侧不再报错。
--   - 语义：平台侧吊销/停用/轮换凭证会立即让缓存失效（`INCR finv:openapi:credver`），
--     本配置只影响「直接改库」的生效延迟与稳态查库频率（每个活跃凭证每 TTL 最多 1 次）。
--   - 【红线】发放的初始数据不得覆盖用户后来改过的值：只用
--     INSERT ... ON CONFLICT (config_no) DO NOTHING，绝不写 DO UPDATE。
--   - 幂等：重跑影响 0 行且不改值。
-- 迁移策略：单事务、失败整体回滚；末尾只断言脚本自己能保证的结构性事实
-- （配置存在、data_type 正确），数值与备注一律 RAISE NOTICE。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 迁移前快照（若已存在，记录当前值，用于验证未被覆盖）
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_sys_config_19007_pre ON COMMIT DROP AS
SELECT config_no, int_value, remark
  FROM sys.sys_config
 WHERE config_no = 19007;

-- ---------------------------------------------------------------------
-- 2. 插入配置记录（已存在则完全跳过，不修改任何列）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value,
     json_value, access_level, create_by, gmt_create, gmt_modify, modify_by, remark)
VALUES
    (19007, 'OpenAPI 凭证缓存 TTL（秒）', '1', 10, '', '', NULL, NULL, 'finv',
     CURRENT_TIMESTAMP(6), CURRENT_TIMESTAMP(6), NULL,
     '外部 OpenAPI 鉴权读取凭证的 Redis 缓存时长（默认 10 秒）：0=关闭缓存（每次请求都查凭证表），1~600=缓存秒数，超出范围按边界收敛。平台侧吊销/停用/轮换凭证会立即让缓存失效，本配置只影响直接改库的生效延迟与稳态查库频率（每个活跃凭证每 TTL 最多查表 1 次）。改后无需重启，下次配置读取（最多 10 秒）生效')
ON CONFLICT (config_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 3. 收敛校验（只断言结构性事实；数值与备注只输出提示）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_exists INT;
    v_type   TEXT;
    v_value  INT;
    v_remark TEXT;
BEGIN
    SELECT count(*) INTO v_exists FROM sys.sys_config WHERE config_no = 19007;
    IF v_exists <> 1 THEN
        RAISE EXCEPTION 'V300191 配置 19007 未插入成功';
    END IF;

    SELECT data_type, COALESCE(int_value, 0), COALESCE(remark, '')
      INTO v_type, v_value, v_remark
      FROM sys.sys_config WHERE config_no = 19007;

    IF v_type <> '1' THEN
        RAISE EXCEPTION 'V300191 配置 19007 的 data_type 应为 1（整数），实际 %', v_type;
    END IF;

    IF v_remark NOT LIKE '%19007%' AND v_remark NOT LIKE '%缓存%' THEN
        RAISE NOTICE 'V300191 配置 19007 的 remark 未提及缓存口径（已存在行可能被外部改过，请人工核对）';
    END IF;

    RAISE NOTICE 'V300191 配置 19007 就绪：当前 TTL=% 秒（0=关闭缓存）', v_value;
END $$;

COMMIT;
