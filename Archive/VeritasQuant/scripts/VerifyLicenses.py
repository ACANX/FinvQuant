"""P0-011 依赖许可证基线校验器。"""

from __future__ import annotations

import argparse
import importlib.metadata
import re
from pathlib import Path

import yaml


_NAME = re.compile(r"^([A-Za-z0-9_.-]+)==")
_LICENSE_ALIASES = {
    "MIT License": "MIT",
    "Apache License 2.0": "Apache-2.0",
    "Apache Software License": "Apache-2.0",
    "BSD License": "BSD-3-Clause",
    "Python Software Foundation License": "PSF-2.0",
}


def lockPackageNames(path: Path, seen: set[Path] | None = None) -> set[str]:
    """递归读取精确锁文件，确保审计对象与构建依赖一致。"""
    seen = set() if seen is None else seen
    resolved = path.resolve()
    if resolved in seen:
        return set()
    seen.add(resolved)
    names: set[str] = set()
    for rawLine in path.read_text(encoding="utf-8").splitlines():
        line = rawLine.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("-r "):
            names.update(lockPackageNames(path.parent / line[3:].strip(), seen))
            continue
        match = _NAME.match(line)
        if match is None:
            raise ValueError(f"锁文件必须使用 name==version: {path}: {line}")
        names.add(normalizeName(match.group(1)))
    return names


def normalizeName(name: str) -> str:
    return re.sub(r"[-_.]+", "-", name).lower()


def normalizeLicense(value: str | None) -> str | None:
    if value is None:
        return None
    stripped = value.strip()
    return _LICENSE_ALIASES.get(stripped, stripped or None)


def metadataLicense(metadata: importlib.metadata.PackageMetadata) -> str | None:
    """优先读取 SPDX/License 字段，缺失时回退到标准 PyPI classifier。"""
    declared = metadata.get("License-Expression") or metadata.get("License")
    if declared:
        return normalizeLicense(declared)
    for classifier in metadata.get_all("Classifier", []):
        if classifier.startswith("License ::"):
            return normalizeLicense(classifier.rsplit("::", maxsplit=1)[-1].strip())
    return None


def verify(root: Path, policyPath: Path, requireApproval: bool = True) -> list[str]:
    """校验批准状态、锁定依赖的许可证和明确例外。"""
    policy = yaml.safe_load(policyPath.read_text(encoding="utf-8"))
    if not isinstance(policy, dict):
        return ["许可证策略必须是 YAML 对象"]
    errors: list[str] = []
    if requireApproval and policy.get("ApprovalStatus") != "APPROVED":
        errors.append("许可证白名单尚未获得批准")
    allowed = set(policy.get("AllowedLicenses", []))
    exceptions = {normalizeName(name): licenseName for name, licenseName in policy.get("PackageExceptions", {}).items()}
    names = lockPackageNames(root / "Requirements" / "Development.lock")
    for name in sorted(names):
        try:
            metadata = importlib.metadata.metadata(name)
        except importlib.metadata.PackageNotFoundError:
            errors.append(f"锁定依赖未安装，无法读取许可证: {name}")
            continue
        licenseName = normalizeLicense(exceptions.get(name)) or metadataLicense(metadata)
        if licenseName is None:
            errors.append(f"依赖缺少可识别许可证: {name}")
        elif licenseName not in allowed:
            errors.append(f"依赖许可证未获批准: {name}: {licenseName}")
    return errors


def main(argv: list[str] | None = None) -> int:
    """默认要求安全负责人已批准策略；仅审查命令可临时允许待批准状态。"""
    parser = argparse.ArgumentParser(description="验证锁定依赖的许可证白名单")
    parser.add_argument("--root", type=Path, default=Path.cwd(), help="仓库根目录")
    parser.add_argument("--policy", type=Path, required=True, help="许可证策略文件")
    parser.add_argument("--allow-pending", action="store_true", help="仅用于审查策略内容，不能作为 CI 门禁")
    arguments = parser.parse_args(argv)
    try:
        errors = verify(arguments.root.resolve(), arguments.policy.resolve(), not arguments.allow_pending)
    except (OSError, ValueError, yaml.YAMLError) as error:
        print(f"license validation failed: {error}")
        return 1
    for validationError in errors:
        print(validationError)
    print(f"license issues: {len(errors)}")
    return 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main())
