"""P0-009 收集 JUnit、覆盖率、环境和产物哈希的可审阅证据。"""

from __future__ import annotations

import argparse
import hashlib
import json
import platform
import sys
from datetime import datetime, timezone
from pathlib import Path
from xml.etree import ElementTree


def sha256File(path: Path) -> str:
    """以分块方式计算工件哈希，避免大报告一次性读入内存。"""
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def junitSummary(path: Path) -> dict[str, int | str]:
    """读取 JUnit 汇总字段，兼容 testsuites 或单个 testsuite 根节点。"""
    root = ElementTree.parse(path).getroot()
    suites = [root] if root.tag == "testsuite" else list(root.findall("testsuite"))
    totals: dict[str, int] = {"tests": 0, "failures": 0, "errors": 0, "skipped": 0}
    for suite in suites:
        for field in totals:
            totals[field] += int(suite.attrib.get(field, "0"))
    return {**totals, "sha256": sha256File(path)}


def main(argv: list[str] | None = None) -> int:
    """生成 snake_case JSON 证据，不包含仓库外部秘密或不可重放状态。"""
    parser = argparse.ArgumentParser(description="收集 VeritasQuant 测试和构建证据")
    parser.add_argument("--junit", type=Path, required=True)
    parser.add_argument("--coverage", type=Path, required=True)
    parser.add_argument("--artifact", type=Path, action="append", required=True)
    parser.add_argument("--work-item", action="append", required=True)
    parser.add_argument("--seed", default="not_applicable")
    parser.add_argument("--test-selection", default="tests/unit tests/contract tests/packaging")
    parser.add_argument("--output", type=Path, required=True)
    arguments = parser.parse_args(argv)
    requiredPaths = [arguments.junit, arguments.coverage, *arguments.artifact]
    missing = [str(path) for path in requiredPaths if not path.is_file()]
    if missing:
        print(f"evidence inputs missing: {', '.join(missing)}")
        return 1
    payload = {
        "collected_at": datetime.now(timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z"),
        "work_item_ids": arguments.work_item,
        "test_selection": arguments.test_selection,
        "random_seed": arguments.seed,
        "environment": {
            "os": platform.platform(),
            "python": sys.version,
            "architecture": platform.machine(),
        },
        "junit": junitSummary(arguments.junit),
        "coverage": {"path": str(arguments.coverage), "sha256": sha256File(arguments.coverage)},
        "artifacts": [{"path": str(path), "sha256": sha256File(path)} for path in arguments.artifact],
    }
    arguments.output.parent.mkdir(parents=True, exist_ok=True)
    arguments.output.write_text(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(f"evidence written: {arguments.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
