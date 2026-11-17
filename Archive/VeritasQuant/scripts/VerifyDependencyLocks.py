"""P0-005 项目依赖声明与锁文件一致性检查。"""

from __future__ import annotations

import argparse
import re
import tomllib
from pathlib import Path


_NAME = re.compile(r"^([A-Za-z0-9_.-]+)==")


def collectLockNames(path: Path) -> set[str]:
    """读取精确锁定项；范围表达式不是可复现锁文件的有效条目。"""
    names: set[str] = set()
    for line in path.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#") or stripped.startswith("-r"):
            continue
        match = _NAME.match(stripped)
        if match is None:
            raise ValueError(f"锁文件必须使用 name==version: {path}: {stripped}")
        names.add(_normalize(match.group(1)))
    return names


def verify(root: Path) -> list[str]:
    """验证 runtime/dev 声明都能在相应锁文件中定位到精确版本。"""
    project = tomllib.loads((root / "pyproject.toml").read_text(encoding="utf-8"))["project"]
    runtimeLock = collectLockNames(root / "Requirements" / "Runtime.lock")
    developmentLock = collectLockNames(root / "Requirements" / "Development.lock") | runtimeLock
    errors: list[str] = []
    for requirement in project.get("dependencies", []):
        name = _normalize(_requirementName(requirement))
        if name not in runtimeLock:
            errors.append(f"运行依赖未被 Runtime.lock 固定: {name}")
    for requirement in project.get("optional-dependencies", {}).get("dev", []):
        name = _normalize(_requirementName(requirement))
        if name not in developmentLock:
            errors.append(f"开发依赖未被 Development.lock 固定: {name}")
    return errors


def _requirementName(requirement: str) -> str:
    return re.split(r"[<>=!~\[; ]", requirement, maxsplit=1)[0]


def _normalize(name: str) -> str:
    return re.sub(r"[-_.]+", "-", name).lower()


def main(argv: list[str] | None = None) -> int:
    """锁文件偏离项目声明时返回非零，供 CI 和审阅前执行。"""
    parser = argparse.ArgumentParser(description="验证 Python 依赖锁文件")
    parser.add_argument("--root", type=Path, default=Path.cwd(), help="仓库根目录")
    arguments = parser.parse_args(argv)
    try:
        errors = verify(arguments.root.resolve())
    except (OSError, ValueError, tomllib.TOMLDecodeError) as error:
        print(f"dependency lock validation failed: {error}")
        return 1
    for validationError in errors:
        print(validationError)
    print(f"dependency lock issues: {len(errors)}")
    return 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main())
