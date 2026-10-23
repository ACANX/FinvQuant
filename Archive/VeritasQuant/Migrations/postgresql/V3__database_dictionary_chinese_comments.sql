-- =====================================================================
-- VeritasQuant PostgreSQL 第三版迁移（数据字典中文注释）
--
-- 范围：为 V1/V2 全部表与字段补充中文 COMMENT（表名注释 + 字段注释）。
-- 背景：V1/V2 已发布（已应用环境按版本化迁移原则不回改），本迁移以增量
--   方式补齐数据字典，供 pgAdmin/DBeaver 等客户端直接查看中文语义。
-- 迁移策略：整个 V3 在一个事务内执行；COMMENT 语句幂等，可安全重放。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 迁移版本跟踪
-- ---------------------------------------------------------------------
COMMENT ON TABLE schema_version IS '迁移版本跟踪';
COMMENT ON COLUMN schema_version.version IS '迁移版本号（主键）';
COMMENT ON COLUMN schema_version.description IS '迁移描述';
COMMENT ON COLUMN schema_version.installed_by IS '执行迁移的数据库用户';
COMMENT ON COLUMN schema_version.installed_on IS '迁移安装时间（UTC）';
COMMENT ON COLUMN schema_version.success IS '迁移是否成功';

-- ---------------------------------------------------------------------
-- 2. 运行清单（RunManifestV1 投影）
-- ---------------------------------------------------------------------
COMMENT ON TABLE run_manifests IS '运行清单：一次运行（回测/模拟盘）的身份与版本快照';
COMMENT ON COLUMN run_manifests.run_id IS '运行唯一标识（主键）';
COMMENT ON COLUMN run_manifests.code_version IS '平台代码版本';
COMMENT ON COLUMN run_manifests.event_schema_registry_hash IS '事件模式注册表哈希';
COMMENT ON COLUMN run_manifests.strategy_version IS '策略版本';
COMMENT ON COLUMN run_manifests.strategy_source_hash IS '策略源码哈希';
COMMENT ON COLUMN run_manifests.dependency_lock_hash IS '依赖锁哈希';
COMMENT ON COLUMN run_manifests.interpreter_version IS '解释器版本';
COMMENT ON COLUMN run_manifests.sandbox_image_digest IS '沙箱镜像摘要';
COMMENT ON COLUMN run_manifests.strategy_sandbox_policy_version IS '策略沙箱策略版本';
COMMENT ON COLUMN run_manifests.strategy_dsl_schema_version IS '策略 DSL 模式版本';
COMMENT ON COLUMN run_manifests.investment_plan_schema_version IS '投资计划模式版本';
COMMENT ON COLUMN run_manifests.config_hash IS '运行配置哈希';
COMMENT ON COLUMN run_manifests.config_schema_version IS '配置模式版本';
COMMENT ON COLUMN run_manifests.data_version_id IS '数据版本标识';
COMMENT ON COLUMN run_manifests.asset_capability_version IS '资产能力版本';
COMMENT ON COLUMN run_manifests.account_group_id IS '账户组标识（账户隔离作用域）';
COMMENT ON COLUMN run_manifests.account_ranks IS '账户等级映射（JSONB）';
COMMENT ON COLUMN run_manifests.random_seed IS '随机种子（确定性重放）';
COMMENT ON COLUMN run_manifests.ts_precision IS '时间戳精度策略';
COMMENT ON COLUMN run_manifests.event_ordering_version IS '事件排序版本';
COMMENT ON COLUMN run_manifests.execution_model_version IS '执行模型版本';
COMMENT ON COLUMN run_manifests.fund_execution_model_version IS '基金执行模型版本';
COMMENT ON COLUMN run_manifests.nav_availability_policy_version IS '净值可用性策略版本';
COMMENT ON COLUMN run_manifests.bar_path_model_version IS '行情 K 线路径模型版本';
COMMENT ON COLUMN run_manifests.liquidity_allocation_version IS '流动性分配版本';
COMMENT ON COLUMN run_manifests.risk_policy_version IS '风险策略版本';
COMMENT ON COLUMN run_manifests.reliability_policy_version IS '可靠性策略版本';
COMMENT ON COLUMN run_manifests.started_at IS '运行开始时间（UTC）';
COMMENT ON COLUMN run_manifests.completed_at IS '运行完成时间（UTC，未完成时为 NULL）';
COMMENT ON COLUMN run_manifests.event_count IS '事件总数（非负）';
COMMENT ON COLUMN run_manifests.order_count IS '订单总数（非负）';
COMMENT ON COLUMN run_manifests.fund_subscription_count IS '基金申购总数（非负）';
COMMENT ON COLUMN run_manifests.fund_confirmation_count IS '基金确认总数（非负）';
COMMENT ON COLUMN run_manifests.execution_count IS '成交总数（非负）';
COMMENT ON COLUMN run_manifests.report_path IS '运行报告路径';

-- ---------------------------------------------------------------------
-- 3. 事件事实表（EventEnvelopeV1 全字段 + 分区投递元数据）
-- ---------------------------------------------------------------------
COMMENT ON TABLE fact_events IS '事件事实表（不可变）：全部领域事件的单一事实来源';
COMMENT ON COLUMN fact_events.event_id IS '事件唯一标识';
COMMENT ON COLUMN fact_events.event_type IS '事件类型';
COMMENT ON COLUMN fact_events.schema_version IS '事件模式版本';
COMMENT ON COLUMN fact_events.run_id IS '所属运行标识（引用 run_manifests）';
COMMENT ON COLUMN fact_events.ts IS '事件业务时间（UTC，全序排序键之一）';
COMMENT ON COLUMN fact_events.occurred_at IS '事件发生时间（UTC）';
COMMENT ON COLUMN fact_events.published_at IS '事件发布时间（UTC）';
COMMENT ON COLUMN fact_events.ingested_at IS '事件入账时间（UTC）';
COMMENT ON COLUMN fact_events.source IS '事件来源';
COMMENT ON COLUMN fact_events.producer IS '事件生产者';
COMMENT ON COLUMN fact_events.producer_version IS '生产者版本';
COMMENT ON COLUMN fact_events.correlation_id IS '关联标识（跨事件追踪）';
COMMENT ON COLUMN fact_events.causation_id IS '因果标识（触发本事件的事件）';
COMMENT ON COLUMN fact_events.account_id IS '账户标识（账户作用域列）';
COMMENT ON COLUMN fact_events.subaccount_id IS '分账户标识';
COMMENT ON COLUMN fact_events.event_ordering_version IS '事件排序版本';
COMMENT ON COLUMN fact_events.phase IS '事件阶段（10/20/30/40/50/60）';
COMMENT ON COLUMN fact_events.priority IS '事件优先级（非负）';
COMMENT ON COLUMN fact_events.source_rank IS '来源等级（非负）';
COMMENT ON COLUMN fact_events.source_sequence IS '来源序号（非负，分区内顺序）';
COMMENT ON COLUMN fact_events.payload IS '事件载荷（JSONB）';
COMMENT ON COLUMN fact_events.content_hash IS '载荷内容哈希（完整性校验）';
COMMENT ON COLUMN fact_events.account_group_id IS '账户组标识（分区作用域）';
COMMENT ON COLUMN fact_events.partition_rank IS '分区等级（非负）';
COMMENT ON COLUMN fact_events.delivery_sequence IS '分区投递序号（从 1 起）';

-- ---------------------------------------------------------------------
-- 4. inbox / outbox（幂等输入 + 至少一次投递）
-- ---------------------------------------------------------------------
COMMENT ON TABLE inbox_records IS 'inbox 幂等记录：可重试输入的去重与协议冲突审计';
COMMENT ON COLUMN inbox_records.idempotency_key IS '幂等键（主键）';
COMMENT ON COLUMN inbox_records.content_hash IS '输入载荷内容哈希';
COMMENT ON COLUMN inbox_records.receipt_sequence IS '回执序号（从 1 起）';
COMMENT ON COLUMN inbox_records.disposition IS '处置结果（APPLIED/DUPLICATE/CONFLICT）';
COMMENT ON COLUMN inbox_records.run_id IS '所属运行标识';
COMMENT ON COLUMN inbox_records.partition_id IS '分区标识';
COMMENT ON COLUMN inbox_records.created_at IS '创建时间（UTC）';
COMMENT ON COLUMN inbox_records.last_attempt_at IS '最近重试时间（UTC）';
COMMENT ON COLUMN inbox_records.attempt_count IS '尝试次数（非负）';

COMMENT ON TABLE inbox_conflicts IS 'inbox 协议冲突隔离（不可变）：同键异载荷审计';
COMMENT ON COLUMN inbox_conflicts.conflict_id IS '冲突记录标识（主键）';
COMMENT ON COLUMN inbox_conflicts.idempotency_key IS '发生冲突的幂等键';
COMMENT ON COLUMN inbox_conflicts.existing_content_hash IS '既有载荷哈希';
COMMENT ON COLUMN inbox_conflicts.conflicting_content_hash IS '冲突载荷哈希';
COMMENT ON COLUMN inbox_conflicts.isolated_at IS '隔离时间（UTC）';
COMMENT ON COLUMN inbox_conflicts.run_id IS '所属运行标识';
COMMENT ON COLUMN inbox_conflicts.partition_id IS '分区标识';

COMMENT ON TABLE outbox_records IS 'outbox 投递记录：领域提交后至少一次投递到消息总线';
COMMENT ON COLUMN outbox_records.outbox_id IS '投递记录标识（主键）';
COMMENT ON COLUMN outbox_records.message_id IS '消息标识';
COMMENT ON COLUMN outbox_records.sequence IS '投递序号（从 1 起）';
COMMENT ON COLUMN outbox_records.topic IS '目标主题';
COMMENT ON COLUMN outbox_records.payload_hash IS '载荷哈希';
COMMENT ON COLUMN outbox_records.status IS '投递状态（PENDING/PUBLISHED）';
COMMENT ON COLUMN outbox_records.run_id IS '所属运行标识';
COMMENT ON COLUMN outbox_records.partition_id IS '分区标识';
COMMENT ON COLUMN outbox_records.created_at IS '创建时间（UTC）';
COMMENT ON COLUMN outbox_records.published_at IS '发布时间（UTC）';
COMMENT ON COLUMN outbox_records.attempt_count IS '尝试次数（非负）';

-- ---------------------------------------------------------------------
-- 5. 单活租约与 fencing token
-- ---------------------------------------------------------------------
COMMENT ON TABLE partition_leases IS '分区单活租约：每个账户组同一时刻只有一个写入者';
COMMENT ON COLUMN partition_leases.account_group_id IS '账户组标识（主键）';
COMMENT ON COLUMN partition_leases.lease_holder IS '租约持有者标识';
COMMENT ON COLUMN partition_leases.fencing_token IS '栅栏令牌（单调递增，防陈旧写入者）';
COMMENT ON COLUMN partition_leases.lease_acquired_at IS '租约获取时间（UTC）';
COMMENT ON COLUMN partition_leases.lease_expires_at IS '租约过期时间（UTC）';
COMMENT ON COLUMN partition_leases.lease_ttl_seconds IS '租约 TTL 秒数（正数）';
COMMENT ON COLUMN partition_leases.renewed_at IS '最近续约时间（UTC）';

-- ---------------------------------------------------------------------
-- 6. 分区检查点
-- ---------------------------------------------------------------------
COMMENT ON TABLE partition_checkpoints IS '分区检查点：处理进度与事务边界';
COMMENT ON COLUMN partition_checkpoints.run_id IS '运行标识（联合主键）';
COMMENT ON COLUMN partition_checkpoints.partition_id IS '分区标识（联合主键）';
COMMENT ON COLUMN partition_checkpoints.last_committed_sequence IS '最近已提交序号（非负）';
COMMENT ON COLUMN partition_checkpoints.transaction_id IS '事务标识';
COMMENT ON COLUMN partition_checkpoints.checkpoint_ts IS '检查点时间（UTC）';

-- ---------------------------------------------------------------------
-- 7. 账本事实表（不可变复式分录）
-- ---------------------------------------------------------------------
COMMENT ON TABLE ledger_journals IS '账本 journal（不可变）：一次业务动作的复式记账载体';
COMMENT ON COLUMN ledger_journals.journal_id IS 'journal 标识（主键）';
COMMENT ON COLUMN ledger_journals.journal_type IS 'journal 类型（开户/订单预留/成交/申赎等 23 类）';
COMMENT ON COLUMN ledger_journals.account_id IS '账户标识';
COMMENT ON COLUMN ledger_journals.subaccount_id IS '分账户标识';
COMMENT ON COLUMN ledger_journals.ts IS '业务时间（UTC）';
COMMENT ON COLUMN ledger_journals.commit_sequence IS '账户内提交序号（从 1 起，重放顺序）';
COMMENT ON COLUMN ledger_journals.source_event_id IS '来源事件标识';
COMMENT ON COLUMN ledger_journals.reversal_of_journal_id IS '被冲销的 journal 标识（冲销关系）';
COMMENT ON COLUMN ledger_journals.instrument_metadata_version IS '工具元数据版本';
COMMENT ON COLUMN ledger_journals.fee_schedule_version IS '费用表版本';
COMMENT ON COLUMN ledger_journals.accounting_policy_version IS '会计政策版本';
COMMENT ON COLUMN ledger_journals.run_id IS '所属运行标识';
COMMENT ON COLUMN ledger_journals.journal_hash IS 'journal 内容哈希';

COMMENT ON TABLE ledger_entries IS '账本分录（不可变）：journal 内的借贷明细行';
COMMENT ON COLUMN ledger_entries.entry_id IS '分录标识（主键）';
COMMENT ON COLUMN ledger_entries.journal_id IS '所属 journal 标识';
COMMENT ON COLUMN ledger_entries.ledger_account IS '账本科目（现金/证券/保证金各可用/冻结/应收应付）';
COMMENT ON COLUMN ledger_entries.direction IS '方向（DEBIT 借 / CREDIT 贷）';
COMMENT ON COLUMN ledger_entries.unit_id IS '计量单位标识';
COMMENT ON COLUMN ledger_entries.asset_id IS '资产标识';
COMMENT ON COLUMN ledger_entries.currency IS '币种';
COMMENT ON COLUMN ledger_entries.quantity IS '数量（NUMERIC(38,18)，正数）';
COMMENT ON COLUMN ledger_entries.book_currency IS '记账币种';
COMMENT ON COLUMN ledger_entries.book_amount IS '记账金额（非负）';
COMMENT ON COLUMN ledger_entries.cost_amount IS '成本金额（非负）';
COMMENT ON COLUMN ledger_entries.account_id IS '账户标识';
COMMENT ON COLUMN ledger_entries.run_id IS '所属运行标识';

-- ---------------------------------------------------------------------
-- 8. 订单与执行事实表
-- ---------------------------------------------------------------------
COMMENT ON TABLE order_intents IS '订单意图（不可变）：策略发出的订单请求事实';
COMMENT ON COLUMN order_intents.intent_id IS '订单意图标识（主键）';
COMMENT ON COLUMN order_intents.run_id IS '所属运行标识';
COMMENT ON COLUMN order_intents.account_id IS '账户标识';
COMMENT ON COLUMN order_intents.subaccount_id IS '分账户标识';
COMMENT ON COLUMN order_intents.strategy_id IS '策略标识';
COMMENT ON COLUMN order_intents.strategy_version IS '策略版本';
COMMENT ON COLUMN order_intents.symbol IS '交易标的';
COMMENT ON COLUMN order_intents.instrument_metadata_version IS '工具元数据版本';
COMMENT ON COLUMN order_intents.side IS '买卖方向（BUY/SELL）';
COMMENT ON COLUMN order_intents.position_effect IS '持仓效果（OPEN/CLOSE/OPEN_CLOSE）';
COMMENT ON COLUMN order_intents.order_type IS '订单类型（MARKET/LIMIT/STOP/STOP_LIMIT）';
COMMENT ON COLUMN order_intents.quantity IS '委托数量（正数）';
COMMENT ON COLUMN order_intents.time_in_force IS '有效期（DAY/GTC/IOC/FOK）';
COMMENT ON COLUMN order_intents.ts IS '业务时间（UTC）';
COMMENT ON COLUMN order_intents.created_from_event_id IS '创建来源事件标识';
COMMENT ON COLUMN order_intents.expected_account_version IS '期望账户版本（乐观并发）';
COMMENT ON COLUMN order_intents.limit_price IS '限价（可为空）';
COMMENT ON COLUMN order_intents.stop_price IS '止损价（可为空）';
COMMENT ON COLUMN order_intents.intent_hash IS '意图内容哈希';

COMMENT ON TABLE order_events IS '订单状态迁移事实：每次迁移严格递增 order_version';
COMMENT ON COLUMN order_events.event_id IS '事件标识（主键）';
COMMENT ON COLUMN order_events.client_order_id IS '客户订单标识';
COMMENT ON COLUMN order_events.intent_id IS '关联订单意图标识';
COMMENT ON COLUMN order_events.command_id IS '产生本次迁移的命令标识';
COMMENT ON COLUMN order_events.order_version IS '订单版本（从 1 起单调递增）';
COMMENT ON COLUMN order_events.state IS '订单状态（NEW/已成交/已取消/已拒绝等）';
COMMENT ON COLUMN order_events.approved_quantity IS '风控批准数量（正数）';
COMMENT ON COLUMN order_events.order_type IS '订单类型';
COMMENT ON COLUMN order_events.side IS '买卖方向';
COMMENT ON COLUMN order_events.quantity IS '委托数量（正数）';
COMMENT ON COLUMN order_events.limit_price IS '限价（可为空）';
COMMENT ON COLUMN order_events.stop_price IS '止损价（可为空）';
COMMENT ON COLUMN order_events.effective_after_event_id IS '生效前置事件标识';
COMMENT ON COLUMN order_events.risk_decision_id IS '关联风险决定标识';
COMMENT ON COLUMN order_events.account_id IS '账户标识';
COMMENT ON COLUMN order_events.subaccount_id IS '分账户标识';
COMMENT ON COLUMN order_events.ts IS '业务时间（UTC）';
COMMENT ON COLUMN order_events.run_id IS '所属运行标识';
COMMENT ON COLUMN order_events.event_hash IS '事件内容哈希';

COMMENT ON TABLE cancel_order_requests IS '撤单请求（不可变）：撤销订单的请求事实';
COMMENT ON COLUMN cancel_order_requests.cancel_request_id IS '撤单请求标识（主键）';
COMMENT ON COLUMN cancel_order_requests.client_order_id IS '目标客户订单标识';
COMMENT ON COLUMN cancel_order_requests.broker_order_id IS '券商订单标识（可为空）';
COMMENT ON COLUMN cancel_order_requests.expected_order_version IS '期望订单版本（乐观并发）';
COMMENT ON COLUMN cancel_order_requests.reason IS '撤单原因';
COMMENT ON COLUMN cancel_order_requests.requested_by IS '请求人';
COMMENT ON COLUMN cancel_order_requests.account_id IS '账户标识';
COMMENT ON COLUMN cancel_order_requests.ts IS '业务时间（UTC）';
COMMENT ON COLUMN cancel_order_requests.run_id IS '所属运行标识';
COMMENT ON COLUMN cancel_order_requests.request_hash IS '请求内容哈希';

COMMENT ON TABLE replace_order_requests IS '改单请求（不可变）：修改订单的请求事实';
COMMENT ON COLUMN replace_order_requests.replace_request_id IS '改单请求标识（主键）';
COMMENT ON COLUMN replace_order_requests.client_order_id IS '目标客户订单标识';
COMMENT ON COLUMN replace_order_requests.expected_order_version IS '期望订单版本（乐观并发）';
COMMENT ON COLUMN replace_order_requests.new_quantity IS '新数量（可为空）';
COMMENT ON COLUMN replace_order_requests.new_limit_price IS '新限价（可为空）';
COMMENT ON COLUMN replace_order_requests.new_stop_price IS '新止损价（可为空）';
COMMENT ON COLUMN replace_order_requests.new_time_in_force IS '新有效期（可为空）';
COMMENT ON COLUMN replace_order_requests.reason IS '改单原因';
COMMENT ON COLUMN replace_order_requests.account_id IS '账户标识';
COMMENT ON COLUMN replace_order_requests.ts IS '业务时间（UTC）';
COMMENT ON COLUMN replace_order_requests.run_id IS '所属运行标识';
COMMENT ON COLUMN replace_order_requests.request_hash IS '请求内容哈希';

COMMENT ON TABLE execution_reports IS '成交回报（不可变）：券商回报的每次事件';
COMMENT ON COLUMN execution_reports.broker_report_id IS '券商回报标识（主键）';
COMMENT ON COLUMN execution_reports.client_order_id IS '客户订单标识';
COMMENT ON COLUMN execution_reports.broker_order_id IS '券商订单标识（可为空）';
COMMENT ON COLUMN execution_reports.report_sequence IS '回报序号（从 1 起）';
COMMENT ON COLUMN execution_reports.execution_type IS '回报类型（新单/部分成交/成交/取消/拒绝等）';
COMMENT ON COLUMN execution_reports.execution_id IS '成交标识（部分唯一索引，非成交回报可空）';
COMMENT ON COLUMN execution_reports.last_quantity IS '本次成交数量（非负）';
COMMENT ON COLUMN execution_reports.last_price IS '本次成交价格（可为空）';
COMMENT ON COLUMN execution_reports.cumulative_quantity IS '累计成交数量（非负）';
COMMENT ON COLUMN execution_reports.remaining_quantity IS '剩余数量（非负）';
COMMENT ON COLUMN execution_reports.broker_state IS '券商侧订单状态';
COMMENT ON COLUMN execution_reports.reason_code IS '原因码（可为空）';
COMMENT ON COLUMN execution_reports.diagnostic_ts IS '诊断时间（UTC）';
COMMENT ON COLUMN execution_reports.account_id IS '账户标识';
COMMENT ON COLUMN execution_reports.ts IS '业务时间（UTC）';
COMMENT ON COLUMN execution_reports.run_id IS '所属运行标识';
COMMENT ON COLUMN execution_reports.report_hash IS '回报内容哈希';

-- ---------------------------------------------------------------------
-- 9. 风控事实表
-- ---------------------------------------------------------------------
COMMENT ON TABLE risk_decisions IS '风险决定（不可变）：风控对订单请求的审批结果';
COMMENT ON COLUMN risk_decisions.decision_id IS '决定标识（主键）';
COMMENT ON COLUMN risk_decisions.request_event_id IS '请求事件标识';
COMMENT ON COLUMN risk_decisions.account_id IS '账户标识';
COMMENT ON COLUMN risk_decisions.decision IS '决定（APPROVED 批准/REJECTED 拒绝/REDUCED 减量）';
COMMENT ON COLUMN risk_decisions.approved_quantity IS '批准数量（非负）';
COMMENT ON COLUMN risk_decisions.rule_ids IS '命中的规则标识列表（JSONB）';
COMMENT ON COLUMN risk_decisions.risk_policy_version IS '风险策略版本';
COMMENT ON COLUMN risk_decisions.account_snapshot_version IS '账户快照版本（决策时点）';
COMMENT ON COLUMN risk_decisions.order_snapshot_version IS '订单快照版本（决策时点）';
COMMENT ON COLUMN risk_decisions.position_snapshot_version IS '持仓快照版本（决策时点）';
COMMENT ON COLUMN risk_decisions.reason_codes IS '拒绝/减量原因码列表（JSONB）';
COMMENT ON COLUMN risk_decisions.decision_hash IS '决定内容哈希';
COMMENT ON COLUMN risk_decisions.ts IS '业务时间（UTC）';
COMMENT ON COLUMN risk_decisions.run_id IS '所属运行标识';

COMMENT ON TABLE trading_controls IS '交易控制（不可变）：风控触发的交易禁令/限制';
COMMENT ON COLUMN trading_controls.control_id IS '控制标识（联合主键）';
COMMENT ON COLUMN trading_controls.control_version IS '控制版本（从 1 起，联合主键）';
COMMENT ON COLUMN trading_controls.control_request_id IS '控制请求标识';
COMMENT ON COLUMN trading_controls.idempotency_key IS '幂等键（唯一）';
COMMENT ON COLUMN trading_controls.scope IS '控制作用域';
COMMENT ON COLUMN trading_controls.action IS '动作（拒绝新单/只减仓/暂停/停止交易）';
COMMENT ON COLUMN trading_controls.strength IS '强度（非负）';
COMMENT ON COLUMN trading_controls.parameters IS '控制参数（JSONB）';
COMMENT ON COLUMN trading_controls.effective_from IS '生效起点';
COMMENT ON COLUMN trading_controls.expires_at IS '过期时间（可为空）';
COMMENT ON COLUMN trading_controls.source_decision_id IS '来源风险决定标识';
COMMENT ON COLUMN trading_controls.risk_policy_version IS '风险策略版本';
COMMENT ON COLUMN trading_controls.status IS '控制状态';
COMMENT ON COLUMN trading_controls.control_hash IS '控制内容哈希';
COMMENT ON COLUMN trading_controls.ts IS '业务时间（UTC）';
COMMENT ON COLUMN trading_controls.run_id IS '所属运行标识';

-- ---------------------------------------------------------------------
-- 10. 投影表（可删除并由已提交事实序列确定性重建）
-- ---------------------------------------------------------------------
COMMENT ON TABLE account_snapshots IS '账户快照投影：版本化余额快照（可重建，非事实源）';
COMMENT ON COLUMN account_snapshots.account_id IS '账户标识（联合主键）';
COMMENT ON COLUMN account_snapshots.snapshot_version IS '快照版本（从 1 起，联合主键）';
COMMENT ON COLUMN account_snapshots.last_ledger_sequence IS '最近账本序号（非负）';
COMMENT ON COLUMN account_snapshots.snapshot_ts IS '快照时间（UTC）';
COMMENT ON COLUMN account_snapshots.balances IS '余额集合（JSONB）';
COMMENT ON COLUMN account_snapshots.content_hash IS '快照内容哈希';
COMMENT ON COLUMN account_snapshots.run_id IS '所属运行标识';
COMMENT ON COLUMN account_snapshots.created_at IS '创建时间（UTC）';

COMMENT ON TABLE ledger_balance_projection IS '账本余额投影：科目×计量单位×币种的当前余额';
COMMENT ON COLUMN ledger_balance_projection.account_id IS '账户标识（联合主键）';
COMMENT ON COLUMN ledger_balance_projection.ledger_account IS '账本科目（联合主键）';
COMMENT ON COLUMN ledger_balance_projection.unit_id IS '计量单位标识（联合主键）';
COMMENT ON COLUMN ledger_balance_projection.book_currency IS '记账币种（联合主键）';
COMMENT ON COLUMN ledger_balance_projection.quantity IS '当前数量';
COMMENT ON COLUMN ledger_balance_projection.cost_amount IS '成本金额';
COMMENT ON COLUMN ledger_balance_projection.last_ledger_sequence IS '最近账本序号（非负）';
COMMENT ON COLUMN ledger_balance_projection.updated_at IS '更新时间（UTC）';

COMMENT ON TABLE account_position_projection IS '账户持仓投影：资产×币种的当前持仓';
COMMENT ON COLUMN account_position_projection.account_id IS '账户标识（联合主键）';
COMMENT ON COLUMN account_position_projection.asset_id IS '资产标识（联合主键）';
COMMENT ON COLUMN account_position_projection.currency IS '币种（联合主键）';
COMMENT ON COLUMN account_position_projection.quantity IS '持仓数量（非负）';
COMMENT ON COLUMN account_position_projection.cost_amount IS '成本金额';
COMMENT ON COLUMN account_position_projection.last_ledger_sequence IS '最近账本序号（非负）';
COMMENT ON COLUMN account_position_projection.updated_at IS '更新时间（UTC）';

COMMENT ON TABLE activity_control_projection IS '活动控制投影：当前生效的交易控制视图';
COMMENT ON COLUMN activity_control_projection.control_id IS '控制标识（主键）';
COMMENT ON COLUMN activity_control_projection.control_version IS '控制版本（从 1 起）';
COMMENT ON COLUMN activity_control_projection.scope IS '控制作用域';
COMMENT ON COLUMN activity_control_projection.action IS '动作（拒绝新单/只减仓/暂停/停止交易）';
COMMENT ON COLUMN activity_control_projection.strength IS '强度（非负）';
COMMENT ON COLUMN activity_control_projection.effective_from IS '生效起点';
COMMENT ON COLUMN activity_control_projection.expires_at IS '过期时间（可为空）';
COMMENT ON COLUMN activity_control_projection.source_decision_id IS '来源风险决定标识';
COMMENT ON COLUMN activity_control_projection.risk_policy_version IS '风险策略版本';
COMMENT ON COLUMN activity_control_projection.status IS '控制状态';
COMMENT ON COLUMN activity_control_projection.updated_at IS '更新时间（UTC）';

-- ---------------------------------------------------------------------
-- 11. 命令资源（V2）
-- ---------------------------------------------------------------------
COMMENT ON TABLE command_records IS '命令资源：写操作的不可变命令与生命周期状态（幂等）';
COMMENT ON COLUMN command_records.command_id IS '命令标识（主键）';
COMMENT ON COLUMN command_records.command_type IS '命令类型';
COMMENT ON COLUMN command_records.account_id IS '账户标识';
COMMENT ON COLUMN command_records.run_id IS '运行标识';
COMMENT ON COLUMN command_records.requested_by IS '请求主体（用户/系统）';
COMMENT ON COLUMN command_records.idempotency_scope IS '幂等作用域（principal+account+路由+键，唯一）';
COMMENT ON COLUMN command_records.payload_hash IS '载荷 SHA-256 哈希';
COMMENT ON COLUMN command_records.payload IS '命令载荷（JSONB）';
COMMENT ON COLUMN command_records.expected_version IS '期望版本（乐观并发，可为空）';
COMMENT ON COLUMN command_records.confirmation_token_id IS '确认令牌标识（双人审批，可为空）';
COMMENT ON COLUMN command_records.status IS '命令状态（生命周期字段）';
COMMENT ON COLUMN command_records.created_ts IS '创建时间（UTC，身份字段不可变）';
COMMENT ON COLUMN command_records.updated_ts IS '更新时间（UTC，单调递增）';
COMMENT ON COLUMN command_records.result_reference IS '结果引用（产物路径，可为空）';
COMMENT ON COLUMN command_records.failure_code IS '失败顶层码（可为空）';
COMMENT ON COLUMN command_records.failure_error_code IS '失败错误符号码（可为空）';
COMMENT ON COLUMN command_records.failure_catalog_version IS '失败错误目录版本（可为空）';
COMMENT ON COLUMN command_records.failure_retryable IS '失败是否可重试（可为空）';
COMMENT ON COLUMN command_records.failure_details IS '失败详情（JSONB，可为空）';

COMMIT;
