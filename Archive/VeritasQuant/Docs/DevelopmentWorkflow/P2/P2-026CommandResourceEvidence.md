# P2-026 不可变命令资源和幂等键存储证据

## 任务信息
- **PlanTaskId:** P2-026
- **标题:** 实现不可变命令资源和幂等键存储
- **阶段/里程碑:** M2A 模拟盘与基金能力建设
- **状态:** IN_REVIEW（等待第四批 PR 合并后验收）

## 实现内容

### CommandService（`src/veritasquant/application/CommandResource.py`）
- **幂等作用域** = principal_id + account_id + API 路由 + Idempotency-Key（分隔符转义防碰撞）
- **同键同载荷**：返回原命令及状态，不重复副作用
- **同键异载荷**：抛 `IdempotencyConflict`（对应 1003 IDEMPOTENCY_CONFLICT）
- **状态机**：PENDING -> AUTHORIZING -> ACCEPTED -> RUNNING -> SUCCEEDED/FAILED，
  支持 CANCEL_REQUESTED -> CANCELLED；FAILED 必须携带失败快照
- **失败快照** CommandFailureV1：code/errorCode/catalogVersion/retryable/安全 details

### 持久化（`src/veritasquant/infrastructure/persistence/CommandStore.py` + V2 迁移）
- `command_records` 表：身份字段（command_id/type/account/run/scope/payload_hash/
  payload/expected_version/confirmation_token/requested_by/created_ts）一经写入冻结
- 触发器 `assert_command_identity_frozen`：禁止修改身份字段、禁止 DELETE
- 幂等作用域唯一索引 `uq_command_records_idempotency_scope`：并发双写只有一个成功
- payload_hash 约束为小写 SHA-256

## 验收标准映射
| 验收标准 | 实现 | 证据 |
| --- | --- | --- |
| 同键同载荷返回原结果 | submit 命中返回 existing | test_same_key_same_payload_returns_original |
| 同键异载荷返回 1003 | IdempotencyConflict | test_same_key_different_payload_conflicts |
| 提交后丢响应不重复副作用 | 幂等作用域冻结 | 集成测试 test_same_key_same_payload_returns_original |

## 测试结果
- 单元测试 `tests/unit/application/test_command_resource.py`：19 个通过
- 数据库集成测试 `tests/integration/database/test_postgres_command_resource.py`：7 个（CI postgres service 运行）
- 迁移契约测试 `test_v2_command_resources_declared` 通过
- ruff/mypy: 通过

## 关键决策
- 命令"不可变"指身份与 payload 冻结；status/updated_ts/result/failure 为生命周期字段
- 数据库触发器提供纵深防御，防止绕过应用层状态机直接改身份
