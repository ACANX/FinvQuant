# P2-029 基础 RBAC、请求/追踪 ID、审计和限频证据

## 任务信息
- **PlanTaskId:** P2-029
- **标题:** 实现基础 RBAC、请求/追踪 ID、审计和限频
- **阶段/里程碑:** M2A 模拟盘与基金能力建设
- **状态:** IN_REVIEW（等待第五批 PR 合并后验收）

## 实现内容

### 安全模型（`application/Security.py`）
- **角色矩阵（默认拒绝）**：Viewer/Researcher/Operator/RiskOperator/LiveApprover/
  Administrator/Auditor 七角色，权限枚举化（account:read、command:submit 等）
- **Principal 账户范围**：accountIds 集合校验，空集合仅管理员/审计可访问
- **RequestContext**：request_id + trace_id + principal 三元组
- **TokenBucketRateLimiter**：按 scope 独立桶、固定速率补充、Retry-After 计算
- **AuditRecord**：不可变审计（timestamp/request_id/principal/action/resource/
  outcome/permission/detail），敏感字段脱敏
- **SecurityService.authorize()**：未鉴权 2001 → 无权限 2002 → 越权账户 2002
  → 限频 2004，全程审计

### 安全中间件（`apps/server/SecurityMiddleware.py`）
- Bearer 凭据解析；X-Request-Id/X-Trace-Id 提取与回填（未提供时生成）
- 路由权限表（方法+前缀匹配），最长前缀优先
- 安全异常在中间件内映射为信封响应（中间件层异常不到达 FastAPI handler）

## 验收标准映射
| 验收标准 | 实现 | 证据 |
| --- | --- | --- |
| 越权账户返回统一错误 | 2002 FORBIDDEN 信封 | test_forbidden_account_out_of_scope |
| request/trace 可关联日志审计 | X-Request-Id 回填 + AuditSink | test_request_id_preserved_when_provided |
| 限频不可绕过 | TokenBucket 路由级容量 | test_rate_limit_returns_429 |

## 测试结果
- `tests/unit/application/test_security.py`：20 个测试通过
- `tests/unit/apps/test_security_middleware.py`：13 个测试通过
- 全量 918 测试通过（2026-08-03，本地）
- ruff/mypy: 通过

## 关键决策
- 默认拒绝：未配置凭据时拒绝一切业务调用
- 中间件自处理安全异常（返回信封），避免中间件栈异常丢失
