-- =====================================================================
-- FinvQuant PostgreSQL V200056：系统配置 20012（登录会话有效期，分钟）
--
-- 决策（ACANX 2026-10-28）：
--   - 需求 10761 引入登录会话，会话有效期需运维可调，不固化为代码常量。
--   - 会话存放在 Redis，TTL 由本配置决定；每次 Resolve 命中且剩余时间
--     不足一半时滑动续期，因此该值同时是「空闲超时」的上界。
--
-- 配置口径：
--   config_no   = 20012
--   data_type   = '1'（整数），取值存放于 int_value
--   默认值      = 480（分钟，即 8 小时）
--   合法区间    = 5 ~ 43200（约 30 天）；越界或非法时运行期回退默认 480
--
-- 注意：本迁移只做幂等 INSERT，绝不覆盖用户已配置的值。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 种子（ON CONFLICT DO NOTHING：不覆盖用户已改过的值）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value, remark, create_by)
VALUES
    (20012, '登录会话有效期(分钟)', '1', 480, '', '',
     '登录成功后 Redis 会话的 TTL，同时作为空闲超时上界（命中且剩余不足一半时滑动续期）；合法区间 5~43200，越界或非法时回退默认 480 分钟（8 小时）。',
     'finv')
ON CONFLICT (config_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 2. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_type   text;
    v_int    int;
    v_count  int;
BEGIN
    SELECT count(*) INTO v_count FROM sys.sys_config WHERE config_no = 20012;
    IF v_count <> 1 THEN
        RAISE EXCEPTION 'V200056: 20012 记录数异常 %', v_count;
    END IF;

    SELECT data_type, COALESCE(int_value, 0) INTO v_type, v_int
      FROM sys.sys_config WHERE config_no = 20012;

    IF btrim(v_type) <> '1' THEN
        RAISE EXCEPTION 'V200056: 20012 data_type 应为 1，实际 %', v_type;
    END IF;

    IF v_int < 5 OR v_int > 43200 THEN
        RAISE WARNING 'V200056: 20012 当前值 % 超出建议区间 5~43200，运行期将回退默认 480', v_int;
    END IF;

    RAISE NOTICE 'V200056 完成：20012 会话有效期 = % 分钟', v_int;
END $$;

COMMIT;
