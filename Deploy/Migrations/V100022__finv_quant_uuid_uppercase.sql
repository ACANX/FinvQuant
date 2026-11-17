-- =====================================================================
-- FinvQuant PostgreSQL V100022：UUID 主键统一转大写
--
-- 背景（ACANX 需求）：
--   run_id / template_id / env_id / strategy_id / account_id 采用 UUID 的，
--   统一将英文字母转换为大写，禁止出现小写英文字母，避免大小写混用
--   导致关联/查询歧义。程序层已改为生成全大写 UUID（newUUID），
--   本迁移将存量数据同步转大写。
--
-- 覆盖范围：
--   - 5 张主表主键：strategy_id / account_id / run_id / env_id / template_id
--   - 可空外键列：strategy.template_id、account.env_id、run.strategy_id/
--     run.account_id/run.env_id
--   - 5 张任务明细表外键：equity/trade/cashflow/position_log/event_trace 的 run_id
--
-- 幂等：UPPER(值) 对大写值无影响，重复执行安全。
-- =====================================================================

-- 策略表：主键 + 来源模板
UPDATE finv_quant_backtest_strategy SET strategy_id = UPPER(strategy_id);
UPDATE finv_quant_backtest_strategy SET template_id = UPPER(template_id) WHERE template_id IS NOT NULL;

-- 账户表：主键 + 默认环境
UPDATE finv_quant_backtest_account SET account_id = UPPER(account_id);
UPDATE finv_quant_backtest_account SET env_id = UPPER(env_id) WHERE env_id IS NOT NULL;

-- 回测任务表：主键 + 引用（策略/账户/环境）
UPDATE finv_quant_backtest_run SET run_id = UPPER(run_id);
UPDATE finv_quant_backtest_run SET strategy_id = UPPER(strategy_id);
UPDATE finv_quant_backtest_run SET account_id = UPPER(account_id);
UPDATE finv_quant_backtest_run SET env_id = UPPER(env_id) WHERE env_id IS NOT NULL;

-- 回测任务表 JSON 快照内嵌 UUID（账户快照 account_id / 环境快照 env_id）同步转大写
UPDATE finv_quant_backtest_run
SET account_snapshot = jsonb_set(
        account_snapshot, '{account_id}',
        to_jsonb(UPPER(account_snapshot->>'account_id')), false)
WHERE account_snapshot->>'account_id' IS NOT NULL;
UPDATE finv_quant_backtest_run
SET env_snapshot = jsonb_set(
        env_snapshot, '{env_id}',
        to_jsonb(UPPER(env_snapshot->>'env_id')), false)
WHERE env_snapshot->>'env_id' IS NOT NULL;

-- 环境表：主键
UPDATE finv_quant_environment SET env_id = UPPER(env_id);

-- 模板表：主键
UPDATE finv_quant_template SET template_id = UPPER(template_id);

-- 任务明细表：run_id 外键
UPDATE finv_quant_backtest_equity SET run_id = UPPER(run_id);
UPDATE finv_quant_backtest_trade SET run_id = UPPER(run_id);
UPDATE finv_quant_backtest_cashflow SET run_id = UPPER(run_id);
UPDATE finv_quant_backtest_position_log SET run_id = UPPER(run_id);
UPDATE finv_quant_backtest_event_trace SET run_id = UPPER(run_id);

-- 行情导入批次：ingest_batch_id 前缀统一为全大写 IMPORT_（原为 import_）。
-- 先同步外键引用（finv_quote_revision_log.ingest_batch_id）再改主键，保证引用一致。
UPDATE finv_quote_revision_log SET ingest_batch_id = 'IMPORT_' || substr(ingest_batch_id, 8)
WHERE ingest_batch_id LIKE 'import_%';
UPDATE finv_quote_ingest_batches SET ingest_batch_id = 'IMPORT_' || substr(ingest_batch_id, 8)
WHERE ingest_batch_id LIKE 'import_%';
