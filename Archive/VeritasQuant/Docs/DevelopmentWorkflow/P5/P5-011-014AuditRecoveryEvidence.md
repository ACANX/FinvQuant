# P5-011~014 运行保障与供应链安全 — 证据

- **任务：** P5-011（ISSUE #207）、P5-012（#208）、P5-013（#209）、P5-014（#210）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** 本 PR（P5 第三批）

## 范围

阶段 5 受控实盘前的运行保障能力：不可变审计/日志访问/保留策略 →
WAL/对象版本化备份与恢复自动验证 → 六类 Runbook → 依赖/镜像/策略源码/沙箱安全冻结。

## 验收标准对照

| 任务 | 验收标准 | 实现 | 测试证据 |
|------|----------|------|----------|
| P5-011 | 普通用户不能删改；检索覆盖命令、审批、风险、订单、账本和人工动作 | `AuditTrailStoreV1`（追加型存储，删除/修改一律 `PermissionError`；六域检索；哈希链完整性）；`AuditRetentionPolicyV1`（按域保留期，PURGE 仅 Administrator 且归档留痕） | `tests/unit/security/test_audit_trail.py`（21 用例） |
| P5-012 | WAL 间隔 <=5 分钟；备份可读性自动验证；恢复环境与生产隔离 | `WalArchivePolicyV1`（实盘 <= 5 分钟）；`BackupReadabilityVerifierV1`（月度自动验证，摘要比对）；`RecoveryEnvironmentV1`（隔离网络/凭据/数据目录）；`BackupRecoveryServiceV1`（RTO<=1h/RPO<=5min、账本哈希/控制 100%/差异 0 才 PASS） | `tests/unit/reliability/test_backup_recovery.py`（33 用例） |
| P5-013 | 每个 Runbook 含触发、权限、步骤、验证、回退、证据和升级联系人 | `RunbookV1`（八要素完整性校验）；`RunbookRegistryV1`（六类全覆盖）；启动/停机/断连/对账/账本异常/密钥泄漏六个标准 Runbook | `tests/unit/reliability/test_runbook.py`（26 用例） |
| P5-014 | 漏洞与许可证 Gate 通过；镜像摘要、依赖锁、源码哈希和审批齐全 | `VulnerabilityGateV1`（CVSS>=7.0 阻断）；`LicenseGateV1`（白名单）；`SecurityFreezeServiceV1`（依赖锁+镜像摘要+源码哈希+≥2 审批人+不可变 freezeHash） | `tests/unit/security/test_supply_chain_freeze.py`（26 用例） |

集成安全测试：`tests/integration/test_p5_audit_recovery_safety.py`（6 用例，覆盖
审计不可变联动、WAL/可读性/隔离恢复、六类 Runbook、Gate 阻断与审批、跨模块审计贯穿）。

## 技术方案要点

- 审计哈希链：`entryHash = SHA256(canonical(entryId, ts, domain, actor, action,
  payloadHash, prevHash, traceId, details))`；合规归档断点保留在 `archivedHashes`，
  非法删除/篡改破坏链完整性；
- WAL 间隔策略：实盘最大 5 分钟，超限立即拒绝（阻止积压）；
- 备份可读性：每月自动验证（摘要比对），未验证不得用于恢复；
- 恢复验证唯一结论：RTO/RPO/账本哈希/控制 100%/差异 0/人工验证全部满足才 PASS，
  缺人工验证为 INSUFFICIENT_EVIDENCE；
- Runbook 注册表：缺失任一要素、步骤乱序或重复登记均拒绝；六类场景全覆盖才
  `coverageComplete`；
- 安全冻结：漏洞（CVSS>=7.0）与许可证 Gate 任一失败即阻断；冻结需 ≥2 审批人；
  `freezeHash` 不可变，篡改可检测。

## 验证结果

- 本批新增 **112** 个测试（21+33+26+26+6），全部通过；
- ruff / mypy / Preflight 全绿；
- 更新：TechSpec 新增 8.8「运行保障与供应链安全契约」；
- 登记表 P5-011~014 登记（IN_REVIEW）；TraceabilityMatrix 挂接。
