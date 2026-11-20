-- =====================================================================
-- FinvQuant PostgreSQL V300162：指数撞名例外的取值统一为 `IDX.{code}`（需求 11077 修订）
--
-- 背景：
--   V300161 为「指数去前导点后与同市场同码证券撞名」的 14 行设了 P40 例外，取值用 `{market}.{code}`
--   （如 `US.BFLY`）。实测发现其中 2 行会与 P50 批量例外的输出撞车：
--     · `US.SPAI` —— 美股股票 SPAI 在 P50 名录里（`US.SPAI`，因 CA.SPAI 同名而强制带前缀）；
--     · `US.UZF`  —— 同理（SG.UZF / US.UZF）。
--   于是指数侧 `US.SPAI` / `US.UZF` 与股票侧 P50 的 `ftmm_symbol` 同名，冲突升级也解不开（两边都落到同一个全码）。
--
-- 修订：这 14 条例外的取值统一改成 `IDX.{code}`（如 `IDX.BFLY`、`IDX.SPAI`、`IDX.UZF`）：
--   · 语义自解释（IDX = 指数），不会与任何市场前缀形态撞车；
--   · 统一形态，避免「有的用 US.xxx、有的另起写法」。
--
-- 幂等：UPDATE 带目标值判断，重跑影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- =====================================================================

BEGIN;

UPDATE futu.finv_futu_usc_rule_item
   SET usc_literal = 'IDX.' || code,
       item_desc   = replace(item_desc, '指数侧改为 US.', '指数侧改为 IDX.'),
       remark      = trim(both ' ' FROM COALESCE(remark, '') || ' | 需求 11077：取值统一为 IDX.{code}，避免与 P50 的 US.{code} 撞车')
 WHERE rule_no = 'P40'
   AND market = 'US'
   AND instrument = 'index'
   AND code IN ('BFLY','CAC','CMBO','DAX','DJT','HXC','IBEX','IBIG','JCI','OBX','SPAI','SPRO','UZF','UZX')
   AND usc_literal <> 'IDX.' || code;

DO $$
DECLARE
    v_cnt   INT;
    v_bad   INT;
BEGIN
    SELECT count(*) INTO v_cnt FROM futu.finv_futu_usc_rule_item
     WHERE rule_no = 'P40' AND market = 'US' AND instrument = 'index'
       AND code IN ('BFLY','CAC','CMBO','DAX','DJT','HXC','IBEX','IBIG','JCI','OBX','SPAI','SPRO','UZF','UZX')
       AND usc_literal = 'IDX.' || code;
    IF v_cnt <> 14 THEN
        RAISE EXCEPTION 'V300162 校验失败：应为 14 条 IDX.{code} 取值，实际 %', v_cnt;
    END IF;

    SELECT count(*) INTO v_bad FROM futu.finv_futu_usc_rule_item
     WHERE rule_no = 'P40' AND market = 'US' AND instrument = 'index'
       AND code IN ('BFLY','CAC','CMBO','DAX','DJT','HXC','IBEX','IBIG','JCI','OBX','SPAI','SPRO','UZF','UZX')
       AND usc_literal LIKE 'US.%';
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V300162 校验失败：仍有 % 条使用 US.{code} 取值', v_bad;
    END IF;

    RAISE NOTICE 'V300162 就绪：14 条指数撞名例外统一为 IDX.{code}';
END $$;

COMMIT;
