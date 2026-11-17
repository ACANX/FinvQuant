# P2-027 资源版本、ETag、expected_version 和长任务状态机证据

## 任务信息
- **PlanTaskId:** P2-027
- **标题:** 实现资源版本、ETag、expected_version 和长任务状态机
- **阶段/里程碑:** M2A 模拟盘与基金能力建设
- **状态:** IN_REVIEW（等待第四批 PR 合并后验收）

## 实现内容

### 命令 API（`src/veritasquant/apps/server/CommandRoutes.py`）
- `POST /api/v1/commands`：202 Accepted，`data` 返回 command_id + status 引用
- `GET /api/v1/commands/{command_id}`：查询状态，含失败快照
  （code/error_code/catalog_version/retryable/details）
- `POST /api/v1/commands/{command_id}/cancel`：状态机合法取消；非法迁移 422 + 3000
- 同键异载荷 409 + 1003；未知命令 404 + 1002

### 乐观并发控制（expected_version 语义）
- `CommandStore.update(record, expectedUpdatedTs)`：CAS 条件更新
  （WHERE updated_ts = 读取时基线）
- 陈旧写入（基于旧 updatedTs）被拒绝，不覆盖当前状态
- 保证"并发冲突不覆盖"

### 长任务状态机
- 状态迁移复用 CommandStateMachineV1（TechSpec 10.2.2 固定状态机）
- FAILED 必须携带失败快照；取消和终态迁移合法性由状态机强制
- 客户端通过 GET 查询状态，不得以 HTTP 超时推断业务失败

## 验收标准映射
| 验收标准 | 实现 | 证据 |
| --- | --- | --- |
| 并发冲突不覆盖 | CAS 乐观锁 | test_concurrent_update_conflict_does_not_overwrite |
| 202 返回 command 引用 | POST /commands 202 + data.command_id | test_submit_returns_202_with_command_reference |
| 取消和终态迁移合法 | 状态机强制 | test_cancel_command / test_invalid_transition_raises |

## 测试结果
- `tests/unit/apps/test_command_routes.py`：8 个通过
- 命令资源单元测试（含并发）：19 个通过
- ruff/mypy: 通过

## 关键决策
- 乐观锁基线使用读取时的 updatedTs（而非 createdTs），保证每次更新推进版本
- 命令路由错误映射走注册目录，未注册码安全降级
