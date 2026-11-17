# P5-002~006 实盘安全核心 — 证据

- **任务：** P5-002（ISSUE #198）、P5-003（#199）、P5-004（#200）、P5-005（#201）、P5-006（#202）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** ACANX/VeritasQuant#243（已合并 2026-08-03T00:29:09Z）

## 范围

阶段 5 实盘安全准备核心链路：环境隔离 → 密钥服务 → TLS/短期令牌 →
双人授权 → 白名单/硬上限。

## 验收标准对照

| 任务 | 验收标准 | 实现 | 测试证据 |
|------|----------|------|----------|
| P5-002 | LIVE 不能与非 LIVE 混组；测试凭据不能访问实盘；跨环境命令被拒绝 | `EnvironmentIsolationPolicyV1`（账户组环境归属 + 凭据环境绑定 + 混组/跨环境命令门禁） | `tests/unit/security/test_environment_isolation.py`（10 用例） |
| P5-003 | 仓库/日志无秘密；轮换不中断审计；撤销后旧凭据立即失效 | `SecretServiceV1`（repr 打码；版本历史保留；撤销立即失效；最小权限门禁 + 审计） | `tests/unit/security/test_secret_service.py`（10 用例） |
| P5-004 | TLS 1.2+；令牌不进 URL；过期/撤销/重放测试通过 | `TlsPolicyV1`（协议 + 弱密码套件黑名单）；`SessionSecurityServiceV1`（短期令牌哈希存储/过期/撤销/单次使用防重放） | `tests/unit/security/test_session_security.py`（13 用例） |
| P5-005 | 同人双签、过期、payload/版本变化和重放全部拒绝并审计 | `DualApprovalWorkflowV1`（payload 哈希 + 版本 + 过期 + 同人双签拒绝 + 消费防重放）；`OneTimeConfirmationServiceV1` | `tests/unit/security/test_dual_approval.py`（14 用例） |
| P5-006 | 非批准组合无法发单；账户级/单笔/单日上限不能由普通配置放宽 | `LiveWhitelistV1`（批准组合）；`HardLimitV1`（独立硬上限）；`LiveOrderGuardV1`（发单前校验 + 单日累计） | `tests/unit/security/test_live_order_guard.py`（10 用例） |

## 技术方案要点

- 新建 `security/` 领域包（实盘安全）；
- 秘密/令牌/签名全部只存哈希或打码，绝不打日志；轮换保留版本历史
  保证审计可追溯；
- 双人授权以 payload 哈希 + 版本固定请求内容，任何变化拒绝；消费
  一次性凭证防重放；
- 硬上限独立于普通配置（账户无硬上限记录直接拒绝发单）；
- 金额路径全程 Decimal 字符串，禁止 float。

## 验证结果

- ruff：All checks passed
- mypy：Success（security 6 源文件）
- Preflight：0 issues
- 全量 pytest：1255 passed / 36 skipped（skipped 为 PG/Redis 集成，CI database job 覆盖）
- 新增测试：57 用例（P5-002: 10 + P5-003: 10 + P5-004: 13 + P5-005: 14 + P5-006: 10）
- CI 4/4：Quality-ubuntu ✅ / Quality-windows ✅ / Security-baseline ✅ / Database-migrations ✅（PR #243 run 30773888386）

## 验收结论

- **状态：** P5-002~006 全部 ACCEPTED（2026-08-03，流转 PR 本 PR）
- **依据：** 实盘安全核心能力（环境隔离/密钥服务/TLS 令牌/双人授权/白名单硬上限）已交付并通过 CI；验收标准逐项对照见上表
- **遗留：** P5-001 威胁建模/安全评审待 ACANX 决策；P5-007~010 为下一批开发

## 风险与开放项

- P5-001（威胁建模/数据分类/安全评审）为评审类任务，需 ACANX 决策；
- P5-007~010 为下一批开发；P5-011~022 含文档/评审/演练/运行类任务。
