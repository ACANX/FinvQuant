-- =====================================================================
-- FinvQuant PostgreSQL V100039：清理 GCMain 误插入的月K线记录
--
-- 背景（ACANX 2026-10-27）：
--   GCMain（COMEX 黄金主连）的分钟表里混入了历史**月K线**数据：时间戳被放在
--   每月 1 号 00:00:00，字段值是整月的 OHLC/成交量，与同月正常分钟数据相比
--   相差 4~5 个数量级，会污染 MVsv 导出与后续统计。
--
-- 判定口径（ACANX 给定规则，本迁移据此落库）：
--   - 位置：time = 0（00:00:00）且 date % 100 = 1（每月 1 号）；
--   - 量级：volume >= 100000（同月中位分钟量约 50~160，相差 3~5 个数量级）；
--   - 振幅：high - low >= 20（同月中位分钟振幅 0.2~3.2）；
--   - 同时满足三者才判定为月K。
--     校验依据：73 条候选中的最小值 volume=2794498 / 振幅=43.7；
--     同位置但正常的 4 条（20260401/20260501/20260601/20260701）
--     volume 仅 14~45、振幅 <= 1.1，不会被误伤。
--
-- 处置：
--   1) 删除前把整行复制到备份表 dedup.finv_quote_secu_kline_min_kline_purge
--      （含 reason 与备份时间），可回滚、可审计；
--   2) 从主表删除这 73 行；
--   3) 收敛校验：主表该条件 0 行、备份表存在对应记录。
--
-- 本次**不处理**同表的日K线（time=0 且非 1 号，约 1326 行）与其他 usc
-- （XAUUSD 有 44 行 time=0）——待业务确认后另起迁移。
--
-- 幂等：备份 INSERT 带 NOT EXISTS 去重；DELETE 带同一条件，重跑影响 0 行。
-- 迁移策略：单事务、失败整体回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 备份表（仅建一次；重复执行不覆盖已有备份）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.finv_quote_secu_kline_min_kline_purge (
    usc             text           NOT NULL,
    ts              bigint         NOT NULL,
    date            integer        NOT NULL,
    time            integer        NOT NULL,
    open            numeric,
    high            numeric,
    low             numeric,
    close           numeric,
    paocd           numeric,
    volume          bigint,
    turnover        numeric,
    change_price    numeric,
    change_ratio    numeric,
    ext_field       text,
    last_close      numeric,
    remark          text,
    provider        varchar(32),
    gmt_create      timestamptz,
    gmt_update      timestamptz,
    reason          text           NOT NULL,
    backup_at       timestamptz    NOT NULL DEFAULT now(),
    CONSTRAINT pk_finv_quote_secu_kline_min_kline_purge PRIMARY KEY (usc, ts)
);

COMMENT ON TABLE dedup.finv_quote_secu_kline_min_kline_purge IS
    '归档主表误插入 K 线数据的清理备份（可回滚）：记录被判定为月K/日K并删除的原始行';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min_kline_purge.reason IS
    '判定原因（如 MONTH_KLINE：月初 00:00 + 成交量与振幅数量级异常）';

-- ---------------------------------------------------------------------
-- 2. 备份候选行
-- ---------------------------------------------------------------------
INSERT INTO dedup.finv_quote_secu_kline_min_kline_purge
    (usc, ts, date, time, open, high, low, close, paocd, volume, turnover,
     change_price, change_ratio, ext_field, last_close, remark, provider,
     gmt_create, gmt_update, reason)
SELECT m.usc, m.ts, m.date, m.time, m.open, m.high, m.low, m.close, m.paocd,
       m.volume, m.turnover, m.change_price, m.change_ratio, m.ext_field,
       m.last_close, m.remark, m.provider, m.gmt_create, m.gmt_update,
       'MONTH_KLINE：月初 00:00 且成交量/振幅与同月分钟数量级不符'
  FROM dedup.finv_quote_secu_kline_min m
 WHERE m.usc = 'GCMain'
   AND m.time = 0
   AND m.date % 100 = 1
   AND m.volume >= 100000
   AND (m.high - m.low) >= 20
   AND NOT EXISTS (
       SELECT 1 FROM dedup.finv_quote_secu_kline_min_kline_purge b
        WHERE b.usc = m.usc AND b.ts = m.ts
   );

-- ---------------------------------------------------------------------
-- 3. 删除主表误插入行
-- ---------------------------------------------------------------------
DELETE FROM dedup.finv_quote_secu_kline_min m
 WHERE m.usc = 'GCMain'
   AND m.time = 0
   AND m.date % 100 = 1
   AND m.volume >= 100000
   AND (m.high - m.low) >= 20;

-- ---------------------------------------------------------------------
-- 4. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_left  INT;
    v_back  INT;
    v_rows  BIGINT;
BEGIN
    -- 兼容修正（需求 10771）：全新库中没有任何 GCMain 行情，清理与校验均不适用，
    -- 原断言会误报「备份表没有 GCMain 月K记录」而中断首次部署。
    SELECT count(*) INTO v_rows FROM dedup.finv_quote_secu_kline_min WHERE usc = 'GCMain';
    IF v_rows = 0 THEN
        RAISE NOTICE 'V100039: 库内无 GCMain 行情数据（全新库），跳过清理与校验';
        RETURN;
    END IF;

    SELECT count(*) INTO v_left
      FROM dedup.finv_quote_secu_kline_min
     WHERE usc = 'GCMain' AND time = 0 AND date % 100 = 1
       AND volume >= 100000 AND (high - low) >= 20;
    IF v_left > 0 THEN
        RAISE EXCEPTION 'V100039: 主表仍有 % 行月K未清理', v_left;
    END IF;

    SELECT count(*) INTO v_back
      FROM dedup.finv_quote_secu_kline_min_kline_purge
     WHERE usc = 'GCMain' AND reason LIKE 'MONTH_KLINE%';
    IF v_back = 0 THEN
        RAISE EXCEPTION 'V100039: 备份表没有 GCMain 月K记录（备份失败？）';
    END IF;

    -- 同位置正常分钟数据必须仍存在（不能被误删）
    SELECT count(*) INTO v_left
      FROM dedup.finv_quote_secu_kline_min
     WHERE usc = 'GCMain' AND time = 0 AND date % 100 = 1;
    IF v_left = 0 THEN
        RAISE EXCEPTION 'V100039: 月初 00:00 的正常分钟数据被全部删除，请立即回滚';
    END IF;

    SELECT count(*) INTO v_rows FROM dedup.finv_quote_secu_kline_min WHERE usc = 'GCMain';
    RAISE NOTICE 'V100039: GCMain 月K已清理（备份 % 行；月初 00:00 保留正常分钟 % 行；GCMain 现 % 行）',
        v_back, v_left, v_rows;
END $$;

COMMIT;
