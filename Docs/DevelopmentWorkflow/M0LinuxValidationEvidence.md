# M0 Linux 本地验证证据

## 结论

- 验证时间：`2026-07-31T08:02:15Z`
- 候选提交：`58c577339f0adb60d9e8622e5cdac0b7bafc9071`
- 环境：Windows 11 上的 `Ubuntu-24.04` WSL，大小写敏感 Linux 文件系统，Python `3.12.3`。
- 结论：本地 Linux 的目录/大小写、静态检查、测试、构建和仓库外 wheel 验证均通过。
- 限制：这不是 GitHub Actions 的空缓存 Python 3.13 运行，不能替代受保护远程 CI、分支保护和 artifact 保留证据。

## 已执行验证

| 命令 | 结果 |
| --- | --- |
| `python3 scripts/VerifyDependencyLocks.py` | 通过，`dependency lock issues: 0`。 |
| `python3 scripts/Preflight.py` | 通过，`preflight issues: 0`。 |
| `python3 -m ruff check src tests scripts` | 通过。 |
| `python3 -m mypy src scripts` | 通过，`48 source files` 无问题。 |
| `python3 -m pytest tests/unit tests/contract tests/packaging -q --junitxml Docs/DevelopmentWorkflow/M0-Linux.junit.xml` | 通过，`58 passed in 3.54s`。 |
| `python3 -m build --no-isolation --outdir /mnt/d/tmp/VeritasQuantM0LinuxDist` | 通过，生成 sdist 和 wheel。 |
| `python3 scripts/VerifyPackage.py --wheel /mnt/d/tmp/VeritasQuantM0LinuxDist/veritasquant-0.1.0-py3-none-any.whl` | 通过，仓库外虚拟环境的全部正式命令与包资源验证成功。 |
| `python3 scripts/ScanSecrets.py` | 通过，`secret findings: 0`。 |
| `python3 scripts/VerifyLicenses.py --policy Configs/Security/LicensePolicy.yml` | 通过，`license issues: 0`。 |

## 工件哈希

| 工件 | SHA-256 |
| --- | --- |
| `M0-Linux.junit.xml` | `7DD1BA327554A2642A832A6D66822387BAEB9815E75FE9D7C3AAF263E84A1F43` |
| `/mnt/d/tmp/VeritasQuantM0LinuxDist/veritasquant-0.1.0-py3-none-any.whl` | `4CEFEE8FC64E533C870F7FFC3FB5630E2FE525A12EEE81029E2441A2BCA00A0D` |
| `/mnt/d/tmp/VeritasQuantM0LinuxDist/veritasquant-0.1.0.tar.gz` | `687A8A30F37802320BA10D9A6C5B7C84EA1098EF71A81C0CE45C51ECBD346F7D` |

## 仍未满足的 M0 证据

- GitHub Actions 的 Windows/Linux Python 3.13 空缓存运行、artifact 留存和受保护分支必需检查。
- Docker Compose 的真实容器健康检查和清理；Docker Engine 已可用，但 Docker Hub 的镜像下载被网络拒绝。
- 非作者人类评审、独立 QA 验收和 Incident Commander 替补。

## 2026-08-01 P0-011 独立复核快照

- 复核时间：`2026-08-01T20:28:00Z`。
- 复核对象：`scripts/ScanSecrets.py`、`scripts/VerifyLicenses.py`、`Configs/Security/LicensePolicy.yml`、`tests/unit/scripts/test_engineering_scripts.py` 与 `.github/workflows/Ci.yml`。
- 秘密扫描：负例演练（含 `api_key` 赋值与 PRIVATE KEY 样本）返回 `secret findings: 1`、退出码 1，正确阻断；输出仅含路径/行号/规则名，不回显疑似秘密值。仓库根目录扫描 `secret findings: 0`（CI 干净环境）。
- 许可证策略：`ApprovalStatus: APPROVED`（ApprovedBy: ProjectAuthor，2026-07-31），允许 MIT/Apache-2.0/BSD-3-Clause/PSF-2.0；`VerifyLicenses.py` 实测 `license issues: 0`。
- 漏洞 SLA：Critical 24 小时 / High 72 小时 / Medium 30 天 / Low 90 天；CI Security baseline 使用 `pip_audit -r requirements/Runtime.lock`，近期运行持续 SUCCESS。
- 单元测试 `test_engineering_scripts.py`（7 项，含秘密阻断与许可证批准门禁）全部通过。

本快照证明 P0-011 验收标准（测试秘密能被阻断；高危漏洞有处置 SLA；许可证白名单经批准）的实现证据与 CI 配置相符。独立人类 SRE 对首次审计报告原始工件的复核仍作为 M0 Gate 前的治理行动保留，不替代或改变本工作项的验收。
