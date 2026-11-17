# P0-003 至 P0-012 作者实施记录

## Definition of Ready

计划项的 ID、阶段、估算、依赖、验收标准和技术方案引用已在 `WorkItemRegister.yml` 登记。ProjectAuthor 已确认 P0-001 的范围、未来 Gate 签署名单及 P0-002 的临时 RACI；两项仍处于 `IN_REVIEW`，且独立评审、远程 CI、Docker 运行与事故替补等证据缺口仍阻止正式 `READY`/验收。本记录仅说明按用户要求完成的工程实现与作者验证准备，不能替代独立评审或验收。

本次未改变事件、账户、风险、执行、API、配置或持久化技术契约，因此不需要创建 Change 或修改权威技术方案。

## 已落盘工件

| 任务 | 实现 | 验收映射 | 外部或独立证据缺口 |
| --- | --- | --- | --- |
| P0-003 | `DevelopmentDocumentIndex.md` | 定位技术方案、计划、工作流、ADR、登记表与证据 | 新成员 15 分钟定位演练与 TL 确认。 |
| P0-004 | `src/veritasquant/`、`Apps/`、`Jobs/`、`Migrations/`、`Docker/`、`Configs/`、`Resources/`、`scripts/Preflight.py` | 方案 A、根级平行包和目录边界回归测试 | Linux 证据的非作者审阅。 |
| P0-005 | `pyproject.toml`、两份精确锁、`VerifyDependencyLocks.py`、依赖政策 | Python 3.13+ 运行/开发分组和可审阅精确锁 | Windows/Linux Python 3.13 空缓存安装的独立审阅。 |
| P0-006 | `.github/workflows/Ci.yml`、`P0-006CiGovernance.md` | 双平台质量、构建、wheel 与 90 天 artifact | 受保护分支、必需检查和独立验收。 |
| P0-007 | `Preflight.py` 和正反例测试 | UTF-8、命名、YAML、JSON、`timestamp`、根级包定位；PR #2 远程负例结果与清理见 `P0-007RemoteNegativeDrillEvidence.md` | 两个失败 job 的原始日志、Ruleset 绕过设置和 PR 非 Draft 差异的独立 QA/SRE 审阅。 |
| P0-008 | 无状态 `docker-compose.yml`、自检脚本、运行手册和 Compose 演练证据 | PostgreSQL/Redis healthcheck，`tmpfs` 与 `down --volumes` 清理 | 自动化演练的独立人类 SRE 复核。 |
| P0-009 | stable ID marker、`CollectTestEvidence.py`、证据规范 | JUnit、coverage、种子、环境和 SHA-256 示例测试 | 独立 QA 使用真实 CI 产物审阅。 |
| P0-010 | 六类登记表、字段规范和 P0 完整工作项 | ID、状态机引用、责任、时限、证据和审计历史 | ProjectAuthor 已确认临时治理映射；仍缺独立复核与登记系统迁移计划。 |
| P0-011 | `ScanSecrets.py`、`VerifyLicenses.py`、许可证策略和 CI audit | 测试秘密阻断、高危漏洞 SLA、许可证门禁 | Security baseline 的独立人类 SRE 审阅。 |
| P0-012 | `TraceabilityMatrix.yml` 与矩阵回归测试 | R-001 至 R-017 均有计划任务、测试 ID、CI 套件和 Gate | 映射的独立 QA/TL 审阅；执行结果按 P1-P5 回写。 |

## 未执行动作

- 未将 P0-001 至 P0-013 的任何工作项标记为 `ACCEPTED`。
- 未冻结首个迭代 Backlog，未创建 Release，也未执行任何环境晋级。
- 未以 ProjectAuthor 对许可证策略的批准替代 Linux 或受保护远程 CI 证据。
- 秘密扫描器首次运行的误报已登记为 `BUG-P0-001`，当前仅处于 `VERIFYING`。

## 本地作者验证

执行时间：2026-07-30T19:23:41Z（证据收集 UTC）。运行环境：Windows 11、Python 3.13.0、AMD64。完整机器可读记录见 [P0-003-P0-012TestEvidence.json](P0-003-P0-012TestEvidence.json)。

- `Preflight.py`、`VerifyDependencyLocks.py`、Ruff 与 Mypy 均通过。
- 单元、契约和打包测试共 58 项通过，失败、错误和跳过均为 0；JUnit SHA-256 为 `1beaa11fe4042d9264e89e44de6d5e8297e06e7c7bd7b3a4a5c7fd31a694dab1`，coverage SHA-256 为 `07b67c62bf6c607fa8a0769f9a90777f299b66d1077b73c0cc1bb41ceece9d7d`。
- `python3 -m build` 成功；wheel SHA-256 为 `3c49c2360133806980b0f143522ed6e933c6ad051b9de76c25bf275fbf991310`，sdist SHA-256 为 `247017c1ef2a41061d0983e7c555ceeb2d6aee6e24c8dc3e5123da19cd4bc7cc`。仓库外新虚拟环境的 12 个正式命令 `--help` 和包内资源检查均通过。
- 秘密扫描结果为 0；运行依赖 `pip-audit` 结果为 `No known vulnerabilities found`。许可证内容审查通过，但强制模式按设计因 `PENDING_APPROVAL` 返回失败。
- `docker compose ... config` 通过；启动时 Docker Desktop Linux Engine 不可用，未创建容器、数据或秘密，详见 `RSK-P0-004`。
