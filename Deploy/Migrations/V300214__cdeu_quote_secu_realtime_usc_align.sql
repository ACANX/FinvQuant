-- =====================================================================
-- V300214：把 cdeu.finv_quote_secu_realtime 的「富途别名 usc」归一到标准编码
--           （需求 11272，配套 V300215 的函数修订）
--
-- 背景：同一证券在新表出现两条记录——BDFS 写 finv_security.usc（`920025`），
--   旧表同步/列表采集写过富途别名（`BJ.920025` / `US.MSGE.WI`）。主键是 usc，
--   两条就都保留下来，排行页按 stock_code 搜索会看到重复。
--
-- 处理（幂等，只动 usc 与主数据不一致的行）：
--   1. 先把将被删除的整行快照到 archive.finv_quote_secu_realtime_usc_align_20261117；
--   2. 标准 usc 行不存在 → 直接改主键为标准 usc；
--   3. 标准 usc 行已存在 → 按 dt_update 谁新谁赢：别名行更新则删标准行、别名行改名；
--      否则删别名行；
--   4. 末尾校验：不再存在「usc 与主数据不一致」的行。
-- 说明：只按 (交易所, 代码) 关联主数据，关联不到的行（如加密 US.XRP）保持原样。
-- =====================================================================

BEGIN;

CREATE SCHEMA IF NOT EXISTS archive;
CREATE TABLE IF NOT EXISTS archive.finv_quote_secu_realtime_usc_align_20261117 (
    idn          bigserial PRIMARY KEY,
    alias_usc    text        NOT NULL,
    std_usc      text        NOT NULL,
    action       text        NOT NULL,
    row_json     jsonb       NOT NULL,
    dt_snapshot  timestamptz NOT NULL DEFAULT now()
);

CREATE TEMP TABLE _usc_align ON COMMIT DROP AS
SELECT t.usc AS alias_usc, btrim(s.usc) AS std_usc
  FROM cdeu.finv_quote_secu_realtime t
  JOIN public.finv_exchange e
    ON upper(btrim(e.exchange_flag)) = upper(btrim(t.market_label))
  JOIN public.finv_security s
    ON s.exchange_code = e.exchange_code
   AND btrim(s.security_code) = btrim(t.stock_code)
 WHERE t.stock_code IS NOT NULL AND btrim(t.stock_code) <> ''
   AND s.usc IS NOT NULL AND btrim(s.usc) <> ''
   AND btrim(t.usc) <> btrim(s.usc);

-- 1) 删除前快照：别名行
INSERT INTO archive.finv_quote_secu_realtime_usc_align_20261117 (alias_usc, std_usc, action, row_json)
SELECT a.alias_usc, a.std_usc, 'delete-alias', to_jsonb(t)
  FROM cdeu.finv_quote_secu_realtime t
  JOIN _usc_align a ON a.alias_usc = t.usc;

-- 1b) 删除前快照：可能被覆盖的标准行（仅当别名行更新时）
INSERT INTO archive.finv_quote_secu_realtime_usc_align_20261117 (alias_usc, std_usc, action, row_json)
SELECT a.alias_usc, a.std_usc, 'delete-std', to_jsonb(x)
  FROM cdeu.finv_quote_secu_realtime x
  JOIN _usc_align a ON a.std_usc = x.usc
 WHERE EXISTS (SELECT 1 FROM cdeu.finv_quote_secu_realtime y WHERE y.usc = a.alias_usc)
   AND (SELECT y.dt_update FROM cdeu.finv_quote_secu_realtime y WHERE y.usc = a.alias_usc)
       >= COALESCE(x.dt_update, 0);

-- 2) 别名行更新（或标准行缺失）时删除标准行，为改名腾出主键
DELETE FROM cdeu.finv_quote_secu_realtime x
 USING _usc_align a
 WHERE x.usc = a.std_usc
   AND EXISTS (SELECT 1 FROM cdeu.finv_quote_secu_realtime y WHERE y.usc = a.alias_usc);

-- 3) 别名行改主键 → 标准 usc（标准行已腾空或不曾存在）
UPDATE cdeu.finv_quote_secu_realtime t
   SET usc = a.std_usc
  FROM _usc_align a
 WHERE t.usc = a.alias_usc
   AND NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_secu_realtime x WHERE x.usc = a.std_usc);

-- 4) 剩余别名行（标准行更新，保留标准行）
DELETE FROM cdeu.finv_quote_secu_realtime t
 USING _usc_align a
 WHERE t.usc = a.alias_usc;

DO $$
DECLARE
    v_left int;
    v_archived int;
BEGIN
    SELECT count(*) INTO v_left
      FROM cdeu.finv_quote_secu_realtime t
      JOIN public.finv_exchange e
        ON upper(btrim(e.exchange_flag)) = upper(btrim(t.market_label))
      JOIN public.finv_security s
        ON s.exchange_code = e.exchange_code
       AND btrim(s.security_code) = btrim(t.stock_code)
     WHERE t.stock_code IS NOT NULL AND btrim(t.stock_code) <> ''
       AND s.usc IS NOT NULL AND btrim(s.usc) <> ''
       AND btrim(t.usc) <> btrim(s.usc);
    IF v_left > 0 THEN
        RAISE EXCEPTION 'V300214 校验失败：仍有 % 行 usc 与主数据不一致', v_left;
    END IF;
    SELECT count(*) INTO v_archived FROM archive.finv_quote_secu_realtime_usc_align_20261117;
    RAISE NOTICE 'V300214 校验通过：usc 已全部对齐主数据（本次快照 % 行，可据此回滚）', v_archived;
END $$;

COMMIT;
