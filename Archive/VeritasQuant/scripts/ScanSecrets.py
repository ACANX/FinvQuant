"""P0-011 高置信度秘密扫描器；用于本地和 CI 的快速阻断。"""

from __future__ import annotations

import argparse
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable


_TEXT_SUFFIXES = {".py", ".yml", ".yaml", ".json", ".toml", ".ini", ".cfg", ".env", ".ps1", ".sh"}
_IGNORED_PARTS = {".git", ".mypy_cache", ".pytest_cache", ".ruff_cache", "Archive", "build", "dist", "var", "__pycache__", "tests"}
_PATTERNS = (
    ("private-key", re.compile(r"-----BEGIN (?:[A-Z ]+ )?PRIVATE KEY-----")),
    ("github-token", re.compile(r"\bgh[pousr]_[A-Za-z0-9_]{20,}\b")),
    ("aws-access-key", re.compile(r"\bAKIA[0-9A-Z]{16}\b")),
    (
        "sensitive-assignment",
        re.compile(r"(?i)\b(?:api[_-]?key|password|secret|token|authorization)\s*[:=]\s*['\"][A-Za-z0-9_./+=-]{12,}['\"]"),
    ),
)


@dataclass(frozen=True)
class SecretFinding:
    """扫描结果只保存定位和规则名，绝不回显疑似秘密值。"""

    path: Path
    lineNumber: int
    rule: str


def collectFindings(root: Path) -> list[SecretFinding]:
    """扫描可执行配置和源码；测试与归档样本不作为生产秘密来源。"""
    findings: list[SecretFinding] = []
    for path in sourceFiles(root):
        for lineNumber, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
            for rule, pattern in _PATTERNS:
                if pattern.search(line):
                    findings.append(SecretFinding(path, lineNumber, rule))
    return findings


def sourceFiles(root: Path) -> Iterable[Path]:
    for path in root.rglob("*"):
        if not path.is_file() or path.suffix.lower() not in _TEXT_SUFFIXES:
            continue
        if any(part in _IGNORED_PARTS for part in path.relative_to(root).parts):
            continue
        yield path


def main(argv: list[str] | None = None) -> int:
    """输出定位信息，扫描命中时返回非零以阻断提交和 CI。"""
    parser = argparse.ArgumentParser(description="扫描 VeritasQuant 源码和配置中的高置信度秘密")
    parser.add_argument("--root", type=Path, default=Path.cwd(), help="仓库根目录")
    arguments = parser.parse_args(argv)
    findings = collectFindings(arguments.root.resolve())
    for finding in findings:
        print(f"{finding.path}:{finding.lineNumber}: 检测到疑似秘密 ({finding.rule})")
    print(f"secret findings: {len(findings)}")
    return 1 if findings else 0


if __name__ == "__main__":
    raise SystemExit(main())
