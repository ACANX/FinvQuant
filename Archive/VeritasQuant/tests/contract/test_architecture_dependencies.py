from __future__ import annotations

import ast
from pathlib import Path


# 领域目录只能被 application 编排，不能反向依赖入口、Web UI 或具体基础设施客户端。
DOMAIN_PACKAGES = ("core", "data", "instruments", "accounts", "strategy", "execution", "risk", "monitoring")
FORBIDDEN_IMPORT_PREFIXES = (
    "veritasquant.apps",
    "veritasquant.jobs",
    "veritasquant.cli",
    "fastapi",
    "streamlit",
    "sqlalchemy",
    "psycopg",
    "redis",
)


def test_domain_packages_do_not_depend_on_entrypoints_or_concrete_infrastructure() -> None:
    sourceRoot = Path("src/veritasquant")
    violations: list[str] = []
    for package in DOMAIN_PACKAGES:
        for sourceFile in (sourceRoot / package).rglob("*.py"):
            tree = ast.parse(sourceFile.read_text(encoding="utf-8"), filename=str(sourceFile))
            for node in ast.walk(tree):
                if isinstance(node, ast.Import):
                    for alias in node.names:
                        _collectViolation(sourceFile, alias.name, violations)
                elif isinstance(node, ast.ImportFrom) and node.module:
                    _collectViolation(sourceFile, node.module, violations)
    assert not violations, "领域模块存在禁止依赖:\n" + "\n".join(violations)


def _collectViolation(sourceFile: Path, moduleName: str, violations: list[str]) -> None:
    if moduleName.startswith(FORBIDDEN_IMPORT_PREFIXES):
        violations.append(f"{sourceFile}: {moduleName}")
