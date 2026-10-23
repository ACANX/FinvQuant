# VeritasQuant 数据库表结构设计（PostgreSQL）

> 本文档描述 PostgreSQL 持久层表结构（迁移 `Migrations/postgresql/`）。
> 全部表与字段已通过 V3 迁移补充**中文注释**（数据字典），可在
> pgAdmin / DBeaver 等客户端直接查看中文语义。

## 1. 设计原则

| 原则 | 说明 |
| --- | --- |
| 事件源（Event Sourcing） | 投影表**不是事实源**；任何余额/持仓必须能由不可变事实序列重建 |
| 事实不可变 | 事实表禁止 `UPDATE/DELETE`（触发器 `prevent_fact_mutation`，ERRCODE 55000） |
| 账户隔离 | 账户作用域列（`account_id`/`account_group_id`）+ 复合唯一键保证账户间数据隔离 |
| 精度规范 | 金额/数量 `NUMERIC(38,18)`；价格 `NUMERIC(38,12)`；禁 float |
| 版本化迁移 | 结构变化只进入新的版本化迁移文件（`V<N>__<name>.sql`），不修改已发布迁移 |
| 数据字典 | V3 迁移为全部表/字段补充中文 COMMENT（幂等可重放） |

## 2. 表总览（22 张）

| 表名 | 中文名 | 类型 | 职责 |
| --- | --- | --- | --- |
| `schema_version` | 迁移版本跟踪 | 基础设施 | 迁移版本记录 |
| `run_manifests` | 运行清单 | 投影 | 运行身份与版本快照 |
| `fact_events` | 事件事实表 | 事实 | 全部领域事件单一事实来源 |
| `inbox_records` | inbox 幂等记录 | 基础设施 | 输入去重与幂等 |
| `inbox_conflicts` | inbox 协议冲突隔离 | 事实 | 同键异载荷审计 |
| `outbox_records` | outbox 投递记录 | 基础设施 | 至少一次投递 |
| `partition_leases` | 分区单活租约 | 基础设施 | 单活写入者 + fencing token |
| `partition_checkpoints` | 分区检查点 | 基础设施 | 处理进度 |
| `ledger_journals` | 账本 journal | 事实 | 复式记账载体 |
| `ledger_entries` | 账本分录 | 事实 | 借贷明细行 |
| `order_intents` | 订单意图 | 事实 | 订单请求事实 |
| `order_events` | 订单状态迁移 | 事实 | 订单状态机事件 |
| `cancel_order_requests` | 撤单请求 | 事实 | 撤单请求事实 |
| `replace_order_requests` | 改单请求 | 事实 | 改单请求事实 |
| `execution_reports` | 成交回报 | 事实 | 券商回报事件 |
| `risk_decisions` | 风险决定 | 事实 | 风控审批结果 |
| `trading_controls` | 交易控制 | 事实 | 交易禁令/限制 |
| `account_snapshots` | 账户快照投影 | 投影 | 版本化余额快照 |
| `ledger_balance_projection` | 账本余额投影 | 投影 | 科目×单位×币种余额 |
| `account_position_projection` | 账户持仓投影 | 投影 | 资产持仓 |
| `activity_control_projection` | 活动控制投影 | 投影 | 生效中交易控制 |
| `command_records` | 命令资源 | 基础设施（V2） | 不可变命令 + 幂等键 |

## 3. 基础设施表

### 3.1 schema_version（迁移版本跟踪）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| version | TEXT | PK | 迁移版本号 |
| description | TEXT | NOT NULL | 迁移描述 |
| installed_by | TEXT | NOT NULL DEFAULT current_user | 执行迁移的数据库用户 |
| installed_on | TIMESTAMPTZ | NOT NULL DEFAULT now() | 迁移安装时间（UTC） |
| success | BOOLEAN | NOT NULL DEFAULT TRUE | 迁移是否成功 |

### 3.2 inbox_records（inbox 幂等记录）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| idempotency_key | TEXT | PK | 幂等键 |
| content_hash | TEXT | NOT NULL | 输入载荷内容哈希 |
| receipt_sequence | BIGINT | NOT NULL, CHECK >= 1 | 回执序号 |
| disposition | TEXT | NOT NULL, IN (APPLIED/DUPLICATE/CONFLICT) | 处置结果 |
| run_id | TEXT | NOT NULL REFERENCES run_manifests | 所属运行标识 |
| partition_id | TEXT | NOT NULL | 分区标识 |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | 创建时间（UTC） |
| last_attempt_at | TIMESTAMPTZ | | 最近重试时间（UTC） |
| attempt_count | INTEGER | NOT NULL DEFAULT 0, CHECK >= 0 | 尝试次数 |

### 3.3 inbox_conflicts（inbox 协议冲突隔离）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| conflict_id | TEXT | PK | 冲突记录标识 |
| idempotency_key | TEXT | NOT NULL | 发生冲突的幂等键 |
| existing_content_hash | TEXT | NOT NULL | 既有载荷哈希 |
| conflicting_content_hash | TEXT | NOT NULL | 冲突载荷哈希 |
| isolated_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | 隔离时间（UTC） |
| run_id | TEXT | NOT NULL REFERENCES run_manifests | 所属运行标识 |
| partition_id | TEXT | NOT NULL | 分区标识 |

### 3.4 outbox_records（outbox 投递记录）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| outbox_id | TEXT | PK | 投递记录标识 |
| message_id | TEXT | NOT NULL | 消息标识 |
| sequence | BIGINT | NOT NULL, CHECK >= 1 | 投递序号 |
| topic | TEXT | NOT NULL | 目标主题 |
| payload_hash | TEXT | NOT NULL | 载荷哈希 |
| status | TEXT | NOT NULL, IN (PENDING/PUBLISHED) | 投递状态 |
| run_id | TEXT | NOT NULL REFERENCES run_manifests | 所属运行标识 |
| partition_id | TEXT | NOT NULL | 分区标识 |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | 创建时间（UTC） |
| published_at | TIMESTAMPTZ | | 发布时间（UTC） |
| attempt_count | INTEGER | NOT NULL DEFAULT 0, CHECK >= 0 | 尝试次数 |

### 3.5 partition_leases（分区单活租约）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| account_group_id | TEXT | PK | 账户组标识 |
| lease_holder | TEXT | NOT NULL | 租约持有者标识 |
| fencing_token | BIGINT | NOT NULL, CHECK >= 0 | 栅栏令牌（防陈旧写入者） |
| lease_acquired_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | 租约获取时间（UTC） |
| lease_expires_at | TIMESTAMPTZ | NOT NULL | 租约过期时间（UTC） |
| lease_ttl_seconds | INTEGER | NOT NULL, CHECK > 0 | 租约 TTL 秒数 |
| renewed_at | TIMESTAMPTZ | | 最近续约时间（UTC） |

### 3.6 partition_checkpoints（分区检查点）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| run_id | TEXT | NOT NULL, PK(联合) | 运行标识 |
| partition_id | TEXT | NOT NULL, PK(联合) | 分区标识 |
| last_committed_sequence | BIGINT | NOT NULL, CHECK >= 0 | 最近已提交序号 |
| transaction_id | TEXT | NOT NULL | 事务标识 |
| checkpoint_ts | TIMESTAMPTZ | NOT NULL DEFAULT now() | 检查点时间（UTC） |

### 3.7 command_records（命令资源，V2）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| command_id | TEXT | PK | 命令标识 |
| command_type | TEXT | NOT NULL | 命令类型 |
| account_id | TEXT | NOT NULL | 账户标识 |
| run_id | TEXT | NOT NULL | 运行标识 |
| requested_by | TEXT | NOT NULL | 请求主体 |
| idempotency_scope | TEXT | NOT NULL, UNIQUE | 幂等作用域 |
| payload_hash | TEXT | NOT NULL, CHECK SHA-256 | 载荷 SHA-256 哈希 |
| payload | JSONB | NOT NULL | 命令载荷 |
| expected_version | TEXT | | 期望版本（乐观并发） |
| confirmation_token_id | TEXT | | 确认令牌标识（双人审批） |
| status | TEXT | NOT NULL | 命令状态（生命周期字段） |
| created_ts | TIMESTAMPTZ | NOT NULL | 创建时间（UTC，身份字段不可变） |
| updated_ts | TIMESTAMPTZ | NOT NULL | 更新时间（UTC，单调递增） |
| result_reference | TEXT | | 结果引用 |
| failure_code | INTEGER | | 失败顶层码 |
| failure_error_code | TEXT | | 失败错误符号码 |
| failure_catalog_version | TEXT | | 失败错误目录版本 |
| failure_retryable | BOOLEAN | | 失败是否可重试 |
| failure_details | JSONB | | 失败详情 |

> 身份字段一经写入冻结（触发器 `assert_command_identity_frozen` 禁止修改/删除）。

## 4. 事实表

### 4.1 fact_events（事件事实表）

事件单一事实来源（EventEnvelopeV1 全字段 + 分区投递元数据）。主键 `(event_id, account_group_id)`；
同一共享事件在每个账户组分区各持有一行。

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| event_id | TEXT | NOT NULL, PK(联合) | 事件唯一标识 |
| event_type | TEXT | NOT NULL | 事件类型 |
| schema_version | TEXT | NOT NULL | 事件模式版本 |
| run_id | TEXT | NOT NULL REFERENCES run_manifests | 所属运行标识 |
| ts | TIMESTAMPTZ | NOT NULL | 事件业务时间（UTC，全序排序键之一） |
| occurred_at | TIMESTAMPTZ | | 事件发生时间（UTC） |
| published_at | TIMESTAMPTZ | | 事件发布时间（UTC） |
| ingested_at | TIMESTAMPTZ | NOT NULL | 事件入账时间（UTC） |
| source | TEXT | NOT NULL | 事件来源 |
| producer | TEXT | NOT NULL | 事件生产者 |
| producer_version | TEXT | NOT NULL | 生产者版本 |
| correlation_id | TEXT | NOT NULL | 关联标识（跨事件追踪） |
| causation_id | TEXT | | 因果标识（触发本事件的事件） |
| account_id | TEXT | | 账户标识（账户作用域列） |
| subaccount_id | TEXT | | 分账户标识 |
| event_ordering_version | TEXT | NOT NULL | 事件排序版本 |
| phase | INTEGER | NOT NULL, CHECK IN (10,20,30,40,50,60) | 事件阶段 |
| priority | INTEGER | NOT NULL, CHECK >= 0 | 事件优先级 |
| source_rank | INTEGER | NOT NULL, CHECK >= 0 | 来源等级 |
| source_sequence | BIGINT | NOT NULL, CHECK >= 0 | 来源序号（分区内顺序） |
| payload | JSONB | NOT NULL | 事件载荷 |
| content_hash | TEXT | NOT NULL | 载荷内容哈希 |
| account_group_id | TEXT | NOT NULL | 账户组标识（分区作用域） |
| partition_rank | INTEGER | NOT NULL DEFAULT 0, CHECK >= 0 | 分区等级 |
| delivery_sequence | BIGINT | NOT NULL, CHECK >= 1 | 分区投递序号 |

关键索引：`uq_fact_events_partition_delivery (run_id, account_group_id, delivery_sequence)`（分区确定性顺序）、
`idx_fact_events_total_order (ts, phase, priority, source_rank, source_sequence, event_id)`（全序排序键）。

### 4.2 ledger_journals（账本 journal）

复式记账载体（JournalV1 + LedgerEntryV1）。不可变。

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| journal_id | TEXT | PK | journal 标识 |
| journal_type | TEXT | NOT NULL, CHECK 23 类 | journal 类型（开户/预留/成交/申赎/费用/冲销等） |
| account_id | TEXT | NOT NULL | 账户标识 |
| subaccount_id | TEXT | | 分账户标识 |
| ts | TIMESTAMPTZ | NOT NULL | 业务时间（UTC） |
| commit_sequence | BIGINT | NOT NULL, CHECK >= 1 | 账户内提交序号 |
| source_event_id | TEXT | NOT NULL | 来源事件标识 |
| reversal_of_journal_id | TEXT | FK self | 被冲销的 journal 标识 |
| instrument_metadata_version | TEXT | NOT NULL | 工具元数据版本 |
| fee_schedule_version | TEXT | NOT NULL | 费用表版本 |
| accounting_policy_version | TEXT | NOT NULL | 会计政策版本 |
| run_id | TEXT | NOT NULL REFERENCES run_manifests | 所属运行标识 |
| journal_hash | TEXT | NOT NULL | journal 内容哈希 |

关键索引：`uq_ledger_journals_account_sequence (account_id, commit_sequence)`（重放顺序确定）。

### 4.3 ledger_entries（账本分录）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| entry_id | TEXT | PK | 分录标识 |
| journal_id | TEXT | NOT NULL REFERENCES ledger_journals | 所属 journal 标识 |
| ledger_account | TEXT | NOT NULL, CHECK 10 类 | 账本科目（现金/证券/保证金×可用/冻结/应收应付） |
| direction | TEXT | NOT NULL, IN (DEBIT/CREDIT) | 方向（借/贷） |
| unit_id | TEXT | NOT NULL | 计量单位标识 |
| asset_id | TEXT | NOT NULL | 资产标识 |
| currency | TEXT | | 币种 |
| quantity | NUMERIC(38,18) | NOT NULL, CHECK > 0 | 数量 |
| book_currency | TEXT | NOT NULL | 记账币种 |
| book_amount | NUMERIC(38,18) | NOT NULL, CHECK >= 0 | 记账金额 |
| cost_amount | NUMERIC(38,18) | NOT NULL, CHECK >= 0 | 成本金额 |
| account_id | TEXT | NOT NULL | 账户标识 |
| run_id | TEXT | NOT NULL REFERENCES run_manifests | 所属运行标识 |

### 4.4 order_intents（订单意图）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| intent_id | TEXT | PK | 订单意图标识 |
| run_id | TEXT | NOT NULL REFERENCES run_manifests | 所属运行标识 |
| account_id | TEXT | NOT NULL | 账户标识 |
| subaccount_id | TEXT | | 分账户标识 |
| strategy_id | TEXT | NOT NULL | 策略标识 |
| strategy_version | TEXT | NOT NULL | 策略版本 |
| symbol | TEXT | NOT NULL | 交易标的 |
| instrument_metadata_version | TEXT | NOT NULL | 工具元数据版本 |
| side | TEXT | NOT NULL, IN (BUY/SELL) | 买卖方向 |
| position_effect | TEXT | NOT NULL, IN (OPEN/CLOSE/OPEN_CLOSE) | 持仓效果 |
| order_type | TEXT | NOT NULL, IN (MARKET/LIMIT/STOP/STOP_LIMIT) | 订单类型 |
| quantity | NUMERIC(38,18) | NOT NULL, CHECK > 0 | 委托数量 |
| time_in_force | TEXT | NOT NULL, IN (DAY/GTC/IOC/FOK) | 有效期 |
| ts | TIMESTAMPTZ | NOT NULL | 业务时间（UTC） |
| created_from_event_id | TEXT | NOT NULL | 创建来源事件标识 |
| expected_account_version | BIGINT | NOT NULL, CHECK >= 0 | 期望账户版本（乐观并发） |
| limit_price | NUMERIC(38,12) | CHECK > 0 | 限价 |
| stop_price | NUMERIC(38,12) | CHECK > 0 | 止损价 |
| intent_hash | TEXT | NOT NULL | 意图内容哈希 |

### 4.5 order_events（订单状态迁移）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| event_id | TEXT | PK | 事件标识 |
| client_order_id | TEXT | NOT NULL | 客户订单标识 |
| intent_id | TEXT | NOT NULL REFERENCES order_intents | 关联订单意图标识 |
| command_id | TEXT | NOT NULL | 产生迁移的命令标识 |
| order_version | INTEGER | NOT NULL, CHECK >= 1 | 订单版本（单调递增） |
| state | TEXT | NOT NULL, CHECK 7 态 | 订单状态 |
| approved_quantity | NUMERIC(38,18) | NOT NULL, CHECK > 0 | 风控批准数量 |
| order_type | TEXT | NOT NULL | 订单类型 |
| side | TEXT | NOT NULL | 买卖方向 |
| quantity | NUMERIC(38,18) | NOT NULL, CHECK > 0 | 委托数量 |
| limit_price | NUMERIC(38,12) | CHECK > 0 | 限价 |
| stop_price | NUMERIC(38,12) | CHECK > 0 | 止损价 |
| effective_after_event_id | TEXT | NOT NULL | 生效前置事件标识 |
| risk_decision_id | TEXT | NOT NULL | 关联风险决定标识 |
| account_id | TEXT | NOT NULL | 账户标识 |
| subaccount_id | TEXT | | 分账户标识 |
| ts | TIMESTAMPTZ | NOT NULL | 业务时间（UTC） |
| run_id | TEXT | NOT NULL REFERENCES run_manifests | 所属运行标识 |
| event_hash | TEXT | NOT NULL | 事件内容哈希 |

关键索引：`uq_order_events_client_version (client_order_id, order_version)`（版本单调唯一）。

### 4.6 cancel_order_requests（撤单请求）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| cancel_request_id | TEXT | PK | 撤单请求标识 |
| client_order_id | TEXT | NOT NULL | 目标客户订单标识 |
| broker_order_id | TEXT | | 券商订单标识 |
| expected_order_version | INTEGER | NOT NULL, CHECK >= 1 | 期望订单版本 |
| reason | TEXT | NOT NULL | 撤单原因 |
| requested_by | TEXT | NOT NULL | 请求人 |
| account_id | TEXT | NOT NULL | 账户标识 |
| ts | TIMESTAMPTZ | NOT NULL | 业务时间（UTC） |
| run_id | TEXT | NOT NULL REFERENCES run_manifests | 所属运行标识 |
| request_hash | TEXT | NOT NULL | 请求内容哈希 |

### 4.7 replace_order_requests（改单请求）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| replace_request_id | TEXT | PK | 改单请求标识 |
| client_order_id | TEXT | NOT NULL | 目标客户订单标识 |
| expected_order_version | INTEGER | NOT NULL, CHECK >= 1 | 期望订单版本 |
| new_quantity | NUMERIC(38,18) | CHECK > 0 | 新数量 |
| new_limit_price | NUMERIC(38,12) | CHECK > 0 | 新限价 |
| new_stop_price | NUMERIC(38,12) | CHECK > 0 | 新止损价 |
| new_time_in_force | TEXT | CHECK IN (DAY/GTC/IOC/FOK) | 新有效期 |
| reason | TEXT | NOT NULL | 改单原因 |
| account_id | TEXT | NOT NULL | 账户标识 |
| ts | TIMESTAMPTZ | NOT NULL | 业务时间（UTC） |
| run_id | TEXT | NOT NULL REFERENCES run_manifests | 所属运行标识 |
| request_hash | TEXT | NOT NULL | 请求内容哈希 |

### 4.8 execution_reports（成交回报）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| broker_report_id | TEXT | PK | 券商回报标识 |
| client_order_id | TEXT | NOT NULL | 客户订单标识 |
| broker_order_id | TEXT | | 券商订单标识 |
| report_sequence | BIGINT | NOT NULL, CHECK >= 1 | 回报序号 |
| execution_type | TEXT | NOT NULL, CHECK 7 类 | 回报类型 |
| execution_id | TEXT | | 成交标识（部分唯一） |
| last_quantity | NUMERIC(38,18) | NOT NULL, CHECK >= 0 | 本次成交数量 |
| last_price | NUMERIC(38,12) | CHECK > 0 | 本次成交价格 |
| cumulative_quantity | NUMERIC(38,18) | NOT NULL, CHECK >= 0 | 累计成交数量 |
| remaining_quantity | NUMERIC(38,18) | NOT NULL, CHECK >= 0 | 剩余数量 |
| broker_state | TEXT | NOT NULL, CHECK 7 态 | 券商侧订单状态 |
| reason_code | TEXT | | 原因码 |
| diagnostic_ts | TIMESTAMPTZ | NOT NULL | 诊断时间（UTC） |
| account_id | TEXT | NOT NULL | 账户标识 |
| ts | TIMESTAMPTZ | NOT NULL | 业务时间（UTC） |
| run_id | TEXT | NOT NULL REFERENCES run_manifests | 所属运行标识 |
| report_hash | TEXT | NOT NULL | 回报内容哈希 |

关键索引：`uq_execution_reports_account_execution (account_id, execution_id) WHERE execution_id IS NOT NULL`（账户内成交唯一）。

### 4.9 risk_decisions（风险决定）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| decision_id | TEXT | PK | 决定标识 |
| request_event_id | TEXT | NOT NULL | 请求事件标识 |
| account_id | TEXT | NOT NULL | 账户标识 |
| decision | TEXT | NOT NULL, IN (APPROVED/REJECTED/REDUCED) | 决定 |
| approved_quantity | NUMERIC(38,18) | NOT NULL, CHECK >= 0 | 批准数量 |
| rule_ids | JSONB | NOT NULL | 命中的规则标识列表 |
| risk_policy_version | TEXT | NOT NULL | 风险策略版本 |
| account_snapshot_version | BIGINT | NOT NULL, CHECK >= 0 | 账户快照版本 |
| order_snapshot_version | BIGINT | NOT NULL, CHECK >= 0 | 订单快照版本 |
| position_snapshot_version | BIGINT | NOT NULL, CHECK >= 0 | 持仓快照版本 |
| reason_codes | JSONB | NOT NULL | 拒绝/减量原因码列表 |
| decision_hash | TEXT | NOT NULL | 决定内容哈希 |
| ts | TIMESTAMPTZ | NOT NULL | 业务时间（UTC） |
| run_id | TEXT | NOT NULL REFERENCES run_manifests | 所属运行标识 |

### 4.10 trading_controls（交易控制）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| control_id | TEXT | NOT NULL, PK(联合) | 控制标识 |
| control_version | INTEGER | NOT NULL, CHECK >= 1, PK(联合) | 控制版本 |
| control_request_id | TEXT | NOT NULL | 控制请求标识 |
| idempotency_key | TEXT | NOT NULL, UNIQUE | 幂等键 |
| scope | TEXT | NOT NULL | 控制作用域 |
| action | TEXT | NOT NULL, IN (REJECT_NEW_ORDERS/REDUCE_ONLY/PAUSE_SCOPE/STOP_TRADING) | 动作 |
| strength | INTEGER | NOT NULL, CHECK >= 0 | 强度 |
| parameters | JSONB | NOT NULL | 控制参数 |
| effective_from | TEXT | NOT NULL | 生效起点 |
| expires_at | TEXT | | 过期时间 |
| source_decision_id | TEXT | NOT NULL | 来源风险决定标识 |
| risk_policy_version | TEXT | NOT NULL | 风险策略版本 |
| status | TEXT | NOT NULL | 控制状态 |
| control_hash | TEXT | NOT NULL | 控制内容哈希 |
| ts | TIMESTAMPTZ | NOT NULL | 业务时间（UTC） |
| run_id | TEXT | NOT NULL REFERENCES run_manifests | 所属运行标识 |

## 5. 投影表

> 投影表可删除并由已提交事实序列确定性重建；**不是事实源**。

### 5.1 run_manifests（运行清单）

36 个字段（`run_id` PK + 版本/哈希/计数/时间），详见 V1 迁移。核心字段：

| 字段 | 类型 | 中文注释 |
| --- | --- | --- |
| run_id | TEXT PK | 运行唯一标识 |
| code_version | TEXT | 平台代码版本 |
| strategy_version | TEXT | 策略版本 |
| account_group_id | TEXT | 账户组标识 |
| account_ranks | JSONB | 账户等级映射 |
| random_seed | BIGINT | 随机种子（确定性重放） |
| started_at / completed_at | TIMESTAMPTZ | 开始/完成时间 |
| event_count / order_count / execution_count | BIGINT | 各类事件计数 |

### 5.2 account_snapshots（账户快照投影）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| account_id | TEXT | NOT NULL, PK(联合) | 账户标识 |
| snapshot_version | BIGINT | NOT NULL, CHECK >= 1, PK(联合) | 快照版本 |
| last_ledger_sequence | BIGINT | NOT NULL, CHECK >= 0 | 最近账本序号 |
| snapshot_ts | TIMESTAMPTZ | NOT NULL | 快照时间（UTC） |
| balances | JSONB | NOT NULL | 余额集合 |
| content_hash | TEXT | NOT NULL | 快照内容哈希 |
| run_id | TEXT | NOT NULL REFERENCES run_manifests | 所属运行标识 |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | 创建时间（UTC） |

### 5.3 ledger_balance_projection（账本余额投影）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| account_id | TEXT | NOT NULL, PK(联合) | 账户标识 |
| ledger_account | TEXT | NOT NULL, PK(联合) | 账本科目 |
| unit_id | TEXT | NOT NULL, PK(联合) | 计量单位标识 |
| book_currency | TEXT | NOT NULL, PK(联合) | 记账币种 |
| quantity | NUMERIC(38,18) | NOT NULL | 当前数量 |
| cost_amount | NUMERIC(38,18) | NOT NULL | 成本金额 |
| last_ledger_sequence | BIGINT | NOT NULL, CHECK >= 0 | 最近账本序号 |
| updated_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | 更新时间（UTC） |

### 5.4 account_position_projection（账户持仓投影）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| account_id | TEXT | NOT NULL, PK(联合) | 账户标识 |
| asset_id | TEXT | NOT NULL, PK(联合) | 资产标识 |
| currency | TEXT | PK(联合) | 币种 |
| quantity | NUMERIC(38,18) | NOT NULL, CHECK >= 0 | 持仓数量 |
| cost_amount | NUMERIC(38,18) | NOT NULL | 成本金额 |
| last_ledger_sequence | BIGINT | NOT NULL, CHECK >= 0 | 最近账本序号 |
| updated_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | 更新时间（UTC） |

### 5.5 activity_control_projection（活动控制投影）

| 字段 | 类型 | 约束 | 中文注释 |
| --- | --- | --- | --- |
| control_id | TEXT | PK | 控制标识 |
| control_version | INTEGER | NOT NULL, CHECK >= 1 | 控制版本 |
| scope | TEXT | NOT NULL | 控制作用域 |
| action | TEXT | NOT NULL | 动作 |
| strength | INTEGER | NOT NULL, CHECK >= 0 | 强度 |
| effective_from | TEXT | NOT NULL | 生效起点 |
| expires_at | TEXT | | 过期时间 |
| source_decision_id | TEXT | NOT NULL | 来源风险决定标识 |
| risk_policy_version | TEXT | NOT NULL | 风险策略版本 |
| status | TEXT | NOT NULL | 控制状态 |
| updated_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | 更新时间（UTC） |

## 6. 迁移清单

| 版本 | 文件 | 内容 |
| --- | --- | --- |
| V1 | `V1__initial_fact_and_projection_schema.sql` | 事实表/投影表/索引/租约/检查点/不可变触发器 |
| V2 | `V2__command_resources.sql` | 命令资源表 + 幂等唯一 + 身份冻结触发器 |
| V3 | `V3__database_dictionary_chinese_comments.sql` | 全部表/字段中文注释（数据字典） |

## 7. 管理入口

- 部署方式见 `Docker/Windows11Deployment.md`（PG 数据在宿主 `<VQ_POSTGRES_DATA_DIR>/18/` 子目录）；
- 本机 pg 客户端连接：`127.0.0.1:5432`（`VQ_POSTGRES_PORT`），库 `veritasquant`，用户 `veritasquant`；
- 迁移实现：`src/veritasquant/infrastructure/persistence/Migrator.py`；迁移测试见 `tests/integration/database/`。
