-- =====================================================================
-- FinvQuant PostgreSQL V31：open 缺失（NULL / 0）时用 last_close 兜底回填
--
-- 决策（ACANX 2026-10-24）：
--   - 数据源存在开盘价缺失的分钟 K 线（open 为 NULL 或 0，常见于集合竞价前
--     或首根异常记录），此前无处回填；last_close（前收盘价）是同一时点最接近的
--     有效价格，用它兜底可保证 open 非空，前端 K 线与指标计算不再出现空值；
--   - 回填条件严格三条：open 为 NULL 或 0、last_close 非 NULL、last_close 非 0；
--     满足条件才赋值，其余行不动；
--   - 回填后写入 finv_quote_revision_log 一条汇总留痕（batch_id=MIGRATION_V31），
--     记录规则与行数，便于事后审计；
--   - **不可逆**：回填覆盖了原始的 open 缺失状态，如需还原只能依赖升级前的
--     pg_dump 备份；因此升级前必须完成第 2 节的备份；
--   - 增量兜底由应用层同步保证（internal/quote/service.go 导入时按同一规则填充），
--     新导入数据不再产生缺 open 的行；
--   - 幂等：条件命中行数为 0 时直接返回，不写留痕。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- 前置：V30（prev_close -> last_close）已应用。
-- =====================================================================
-- ---------------------------------------------------------------------
-- 兼容修正（需求 10771，2026-10-28）：
--   历史迁移未给表名加 schema 前缀，首次建库时按 search_path 落到 public；
--   若 public 中已存在**其它服务**的同名表，V100001 会直接撞表
--   （relation "finv_quote_secu_kline_min" already exists），V100032 还会把
--   别人的表搬进 quote。此处把本程序自有表显式限定到 quote，全程不触碰 public。
--   本修正只影响全新库；已应用库按 schema_version 跳过，结构不变。
-- ---------------------------------------------------------------------

BEGIN;

DO $$
DECLARE
    cnt BIGINT := 0;
BEGIN
    SELECT count(*) INTO cnt
    FROM quote.finv_quote_secu_kline_min
    WHERE (open IS NULL OR open = 0)
      AND last_close IS NOT NULL
      AND last_close <> 0;

    RAISE NOTICE 'V31: 满足回填条件的行数=%', cnt;

    IF cnt = 0 THEN
        RAISE NOTICE 'V31: 无需回填';
        RETURN;
    END IF;

    UPDATE quote.finv_quote_secu_kline_min
       SET open = last_close
     WHERE (open IS NULL OR open = 0)
       AND last_close IS NOT NULL
       AND last_close <> 0;

    INSERT INTO quote.finv_quote_revision_log
        (ingest_batch_id, market_code, usc, affected_rows, reason, revised_by,
         previous_summary, new_summary)
    VALUES (
        'MIGRATION_V31',
        0,
        'ALL',
        cnt,
        'last_close 回填 open（open 为 NULL 或 0）',
        'system',
        jsonb_build_object(
            'rule', 'open IS NULL OR open = 0',
            'source_column', 'last_close'
        ),
        jsonb_build_object(
            'filled_rows', cnt,
            'irreversible', true
        )
    );

    RAISE NOTICE 'V31: 已回填 % 行，并写入 finv_quote_revision_log(MIGRATION_V31)', cnt;
END $$;

COMMIT;
