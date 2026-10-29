-- =====================================================================
-- FinvQuant PostgreSQL V200057：把 dedup 侧 CDEU 配置（phase/process/job/job_step）
-- 同步到 cdeu，修复「全新库按版本号执行时 cdeu 配置落后于 dedup」的问题
--
-- 背景（需求 10771）：
--   V100080（把 dedup 数据复制到 cdeu）的编号小于 V200048/V200050（CDEU 流程与
--   JOB 重排），但后者的新增时间更晚；迁移按版本号顺序执行，于是全新库中
--   cdeu 拿到的是「重排前」的快照，V200048/V200050 只改了 dedup，导致运行期代码
--   读 cdeu 时缺少 PROCESS_PURGE / PROCESS_STORAGE_CLEAN / PROCESS_RETENTION
--   以及对应 JOB 定义。
--
-- 策略：
--   - 仅在 cdeu 无任何运行记录（job_run 为空）且配置行数少于 dedup 时执行，
--     升级库（已有运行数据）一律跳过；
--   - 清空后全量复制 4 张配置表（dedup 与 cdeu 同构、列顺序一致），保留 id；
--   - 复制后把序列 setval 到 max(id)，避免后续插入撞主键。
-- 幂等：已同步（行数不落后）或无运行记录的判断均会跳过。
-- 注：只同步配置表，不动 job_run / process_run / phase_run 等运行记录。
--
-- 增补（需求 10808）：部分存量库已把 dedup 冻结/迁移为其它 schema（如 dedup_frozen），
-- 此时 dedup schema 不存在，本迁移无配置来源可同步，应直接跳过；否则会在读取
-- dedup.cdeu_process 时报 42P01 导致后续迁移全部阻塞。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_run   bigint;
    v_cdeu  bigint;
    v_dedup bigint;
    pairs   text[][] := ARRAY[
        ['cdeu_phase',    'finv_quote_cdeu_phase',    'phase_id'],
        ['cdeu_process',  'finv_quote_cdeu_process',  'process_id'],
        ['cdeu_job',      'finv_quote_cdeu_job',      'job_id'],
        ['cdeu_job_step', 'finv_quote_cdeu_job_step', 'step_id']
    ];
    i    INT;
    cols TEXT;
BEGIN
    -- dedup schema 不存在（已冻结/迁移）时无配置来源，直接跳过（需求 10808）
    IF NOT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'dedup') THEN
        RAISE NOTICE 'V200057: dedup schema 不存在，跳过配置同步';
        RETURN;
    END IF;

    SELECT count(*) INTO v_run   FROM cdeu.finv_quote_cdeu_job_run;
    SELECT count(*) INTO v_cdeu  FROM cdeu.finv_quote_cdeu_process;
    SELECT count(*) INTO v_dedup FROM dedup.cdeu_process;

    IF v_run > 0 THEN
        RAISE NOTICE 'V200057: cdeu 已有运行记录（%），跳过配置同步', v_run;
        RETURN;
    END IF;
    IF v_cdeu >= v_dedup THEN
        RAISE NOTICE 'V200057: cdeu 配置不落后（cdeu=% / dedup=%），跳过', v_cdeu, v_dedup;
        RETURN;
    END IF;

    -- 清空配置表（倒序：step → job → process → phase；无物理外键）
    DELETE FROM cdeu.finv_quote_cdeu_job_step;
    DELETE FROM cdeu.finv_quote_cdeu_job;
    DELETE FROM cdeu.finv_quote_cdeu_process;
    DELETE FROM cdeu.finv_quote_cdeu_phase;

    FOR i IN 1..array_length(pairs, 1) LOOP
        SELECT string_agg(quote_ident(c.column_name), ',' ORDER BY c.ordinal_position)
          INTO cols
          FROM information_schema.columns c
          JOIN information_schema.columns s
            ON s.table_schema = 'dedup' AND s.table_name = pairs[i][1]
           AND s.column_name = c.column_name
         WHERE c.table_schema = 'cdeu' AND c.table_name = pairs[i][2];

        IF cols IS NULL THEN
            RAISE EXCEPTION 'V200057: cdeu.% 与 dedup.% 无公共列', pairs[i][2], pairs[i][1];
        END IF;

        EXECUTE format('INSERT INTO cdeu.%I (%s) SELECT %s FROM dedup.%I',
                       pairs[i][2], cols, cols, pairs[i][1]);

        EXECUTE format(
            'SELECT setval(pg_get_serial_sequence(%L, %L), coalesce((SELECT max(%I) FROM cdeu.%I), 1), (SELECT max(%I) IS NOT NULL FROM cdeu.%I))',
            'cdeu.' || pairs[i][2], pairs[i][3],
            pairs[i][3], pairs[i][2], pairs[i][3], pairs[i][2]);

        RAISE NOTICE 'V200057: 已同步 dedup.% → cdeu.%', pairs[i][1], pairs[i][2];
    END LOOP;

    RAISE NOTICE 'V200057: 完成（cdeu phase=% / process=% / job=% / job_step=%）',
        (SELECT count(*) FROM cdeu.finv_quote_cdeu_phase),
        (SELECT count(*) FROM cdeu.finv_quote_cdeu_process),
        (SELECT count(*) FROM cdeu.finv_quote_cdeu_job),
        (SELECT count(*) FROM cdeu.finv_quote_cdeu_job_step);
END $$;

COMMIT;
