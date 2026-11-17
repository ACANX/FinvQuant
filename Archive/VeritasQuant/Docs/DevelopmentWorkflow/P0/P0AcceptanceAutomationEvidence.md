# P0 自动化验收证据

## 运行信息

| 项目 | 值 |
| --- | --- |
| 执行时间 | 2026-07-31T12:31:58Z 后的 P0 验收运行 |
| 解释器 | Python 3.13.0 |
| 工作区分支 | `dev` |
| 随机种子 | `not_applicable` |

## 通过结果

- `python3 scripts/VerifyDependencyLocks.py`：`dependency lock issues: 0`。
- `python3 scripts/Preflight.py`：`preflight issues: 0`。
- `python3 scripts/ScanSecrets.py`：`secret findings: 0`。
- `python3 scripts/VerifyLicenses.py --policy Configs/Security/LicensePolicy.yml`：
  `license issues: 0`。
- `python3 -m pip_audit -r Requirements/Runtime.lock`：无已知漏洞。
- `python3 -m pytest tests/unit tests/contract tests/packaging -q`：61 项通过。
- `python3 -m build`：wheel 与 sdist 构建成功。
- `python3 scripts/VerifyPackage.py --wheel dist/veritasquant-0.1.0-py3-none-any.whl`：
  仓库外安装和正式入口验证通过。

## 生成工件

| 工件 | SHA-256 |
| --- | --- |
| `artifacts/P0Acceptance.junit.xml` | `792496004F081A4C74D27FA5BFE61520CEDC9D1C78A9A825F4E2DBBAA4B182B8` |
| `artifacts/P0Acceptance.coverage.xml` | `32637F0CDB30918D2A5E88035F1CFB70E6EA2A16E98882540B02815365BA3B72` |
| `dist/veritasquant-0.1.0-py3-none-any.whl` | `3C0C915D138F21040EA4044A5574B68824C50E43091D125C9F51C571ABEE2B22` |
| `dist/veritasquant-0.1.0.tar.gz` | `ABC93EAFEE82DF24482F8AA840A0695FFB9B393B144BBA38808808E0830D6B92` |

机器可读证据位于 `artifacts/P0Acceptance.evidence.json`。本记录仅证明作者侧自动化验收；
独立人类审阅和 Gate 签署仍按 P0 验收启动记录执行。

