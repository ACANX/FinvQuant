"""P0-007 工程命名、编码和 wire 字段边界前置检查。"""

from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterable

import yaml


_UTF8_SUFFIXES = {".py", ".md", ".yml", ".yaml", ".json", ".toml"}
_IGNORED_PARTS = {".git", ".venv", ".mypy_cache", ".pytest_cache", ".ruff_cache", "Archive", "build", "dist", "var", "__pycache__"}
_PASCAL_CASE = re.compile(r"^[A-Z][A-Za-z0-9]*$")
_SNAKE_CASE = re.compile(r"^[a-z][a-z0-9_]*$")
_ROOT_PYTHON_EXCEPTIONS = {"src", "tests", "scripts"}


def configureStandardStreams() -> None:
    """确保 Windows 非 UTF-8 控制台也能输出可定位的中文诊断。"""
    for stream in (sys.stdout, sys.stderr):
        reconfigure = getattr(stream, "reconfigure", None)
        if not callable(reconfigure):
            continue
        try:
            reconfigure(encoding="utf-8", errors="backslashreplace")
        except (OSError, ValueError):
            continue


@dataclass(frozen=True)
class Issue:
    """可定位到文件和字段的前置检查失败。"""

    path: Path
    message: str


def collectIssues(root: Path) -> list[Issue]:
    """扫描工程文件；排除归档与构建产物，防止历史资料干扰当前门禁。"""
    issues: list[Issue] = []
    for path in _sourceFiles(root):
        issues.extend(_checkUtf8(path))
        issues.extend(_checkFileName(root, path))
        if path.suffix in {".yml", ".yaml"} and _isProjectYaml(root, path):
            issues.extend(_checkYaml(path))
        if path.suffix == ".json":
            issues.extend(_checkJson(path))
    issues.extend(_checkRootEntryDirectories(root))
    issues.extend(_checkRootPythonPackages(root))
    return issues


def _sourceFiles(root: Path) -> Iterable[Path]:
    for path in root.rglob("*"):
        if not path.is_file() or any(part in _IGNORED_PARTS for part in path.relative_to(root).parts):
            continue
        if path.suffix.lower() in _UTF8_SUFFIXES:
            yield path


def _checkUtf8(path: Path) -> list[Issue]:
    try:
        path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        return [Issue(path, "文本文件必须使用 UTF-8 编码")]
    return []


def _checkFileName(root: Path, path: Path) -> list[Issue]:
    relative = path.relative_to(root)
    name = path.name
    if relative.parts[:2] == ("src", "veritasquant") and path.suffix == ".py":
        if name != "__init__.py" and not _PASCAL_CASE.fullmatch(path.stem):
            return [Issue(path, "src/veritasquant 下项目 Python 文件必须使用 PascalCase")]
    if relative.parts[:1] == ("scripts",) and path.suffix == ".py" and not _PASCAL_CASE.fullmatch(path.stem):
        return [Issue(path, "scripts 下维护工具必须使用 PascalCase")]
    if relative.parts[:1] == ("tests",) and path.suffix == ".py" and not re.fullmatch(r"test_[a-z0-9_]+", path.stem):
        return [Issue(path, "tests 下测试文件必须使用 test_snake_case.py")]
    return []


def _isProjectYaml(root: Path, path: Path) -> bool:
    relative = path.relative_to(root)
    return relative.parts[:1] in {("Apps",), ("Jobs",), ("Configs",), ("Resources",), ("Strategies",)} or relative.parts[:3] == ("src", "veritasquant", "resources")


def _checkYaml(path: Path) -> list[Issue]:
    try:
        value = yaml.safe_load(path.read_text(encoding="utf-8"))
    except yaml.YAMLError as error:
        return [Issue(path, f"YAML 解析失败: {error}")]
    issues = _checkKeys(path, value, "", _PASCAL_CASE, "项目 YAML 字段必须为 PascalCase", {"Timestamp", "timestamp"})
    # 数据子树豁免：任务参数（Jobs/*.yml Parameters）与包名例外
    # （Configs/Security/LicensePolicy.yml PackageExceptions）是数据键
    # 而非项目字段，允许 snake_case。
    issues.extend(_checkDataSubtrees(path, value))
    return issues


_DATA_SUBTREE_KEYS = {"Parameters", "PackageExceptions"}


def _checkDataSubtrees(path: Path, value: Any) -> list[Issue]:
    """检查数据子树内部键使用宽松规则（snake_case 或包名）。"""
    issues: list[Issue] = []
    if not isinstance(value, dict):
        return issues
    for key, nested in value.items():
        if key in _DATA_SUBTREE_KEYS and isinstance(nested, dict):
            for dataKey, dataValue in nested.items():
                if not isinstance(dataKey, str):
                    continue
                # 数据键只要求非空且不含空白；允许 snake_case 与包名
                if not dataKey.strip() or any(ch.isspace() for ch in dataKey):
                    issues.append(Issue(path, f"{key}.{dataKey}: 数据键无效"))
                if isinstance(dataValue, dict):
                    issues.extend(
                        _checkKeys(
                            path,
                            dataValue,
                            f"{key}.{dataKey}",
                            _SNAKE_CASE,
                            "任务参数内部字段必须为 snake_case",
                            {"Ts", "ts"},
                        )
                    )
        elif isinstance(nested, (dict, list)):
            issues.extend(_checkDataSubtrees(path, nested))
    return issues


def _checkJson(path: Path) -> list[Issue]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as error:
        return [Issue(path, f"JSON 解析失败: {error}")]
    return _checkKeys(path, value, "", _SNAKE_CASE, "JSON wire 字段必须为 snake_case", {"timestamp", "Timestamp"})


def _checkKeys(
    path: Path,
    value: Any,
    prefix: str,
    expected: re.Pattern[str],
    styleMessage: str,
    forbidden: set[str],
) -> list[Issue]:
    issues: list[Issue] = []
    if isinstance(value, dict):
        for key, nested in value.items():
            location = f"{prefix}.{key}" if prefix else str(key)
            if not isinstance(key, str):
                issues.append(Issue(path, f"{location}: 字段名必须为字符串"))
            elif key in forbidden:
                issues.append(Issue(path, f"{location}: 禁止使用 timestamp 同义 wire 字段；应使用 Ts/ts"))
            elif key in _DATA_SUBTREE_KEYS:
                # 数据子树由 _checkDataSubtrees 单独检查（宽松规则）
                continue
            elif not expected.fullmatch(key):
                issues.append(Issue(path, f"{location}: {styleMessage}"))
            if key not in _DATA_SUBTREE_KEYS:
                issues.extend(_checkKeys(path, nested, location, expected, styleMessage, forbidden))
    elif isinstance(value, list):
        for index, nested in enumerate(value):
            issues.extend(_checkKeys(path, nested, f"{prefix}[{index}]", expected, styleMessage, forbidden))
    return issues


def _checkRootEntryDirectories(root: Path) -> list[Issue]:
    issues: list[Issue] = []
    for directoryName in ("Apps", "Jobs"):
        directory = root / directoryName
        if not directory.exists():
            continue
        for path in directory.rglob("*"):
            if path.is_file() and path.suffix not in {".yml", ".yaml"}:
                issues.append(Issue(path, f"{directoryName} 只能包含部署或任务 YAML 清单"))
    return issues


def _checkRootPythonPackages(root: Path) -> list[Issue]:
    """方案 A 只允许 src/veritasquant 承载可导入项目业务包。"""
    issues: list[Issue] = []
    for path in root.iterdir():
        if path.name in _ROOT_PYTHON_EXCEPTIONS or path.name.startswith("."):
            continue
        if path.is_file() and path.suffix == ".py":
            issues.append(Issue(path, "仓库根目录不得存在平行业务 Python 文件"))
        if path.is_dir() and (path / "__init__.py").is_file():
            issues.append(Issue(path, "仓库根目录不得存在平行业务 Python 包"))
    return issues


def main(argv: list[str] | None = None) -> int:
    """输出全部前置检查失败，返回非零以阻断后续 CI 阶段。"""
    configureStandardStreams()
    parser = argparse.ArgumentParser(description="验证 VeritasQuant 工程前置规则")
    parser.add_argument("--root", type=Path, default=Path.cwd(), help="仓库根目录")
    arguments = parser.parse_args(argv)
    issues = collectIssues(arguments.root.resolve())
    for issue in issues:
        print(f"{issue.path}: {issue.message}")
    print(f"preflight issues: {len(issues)}")
    return 1 if issues else 0


if __name__ == "__main__":
    raise SystemExit(main())
