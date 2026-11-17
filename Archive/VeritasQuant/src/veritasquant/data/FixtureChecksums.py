"""P1-024 批次 A 固定数据夹具与跨平台 checksum。

夹具固定存放于 ``Data/Fixtures/BatchA``，其 SHA-256 与规范化数据/事件序列
checksum 固化在 ``BatchAChecksums.yml``，并纳入 CI 回归。任何夹具变更必须
更新 checksum 并记录原因与批准；跨平台必须产生相同结果。
"""

from __future__ import annotations

import hashlib
from pathlib import Path

import yaml

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.core.Time import TsPrecision

FIXTURES_DIR = Path(__file__).resolve().parents[3] / "Data" / "Fixtures" / "BatchA"
CHECKSUM_FILE = FIXTURES_DIR / "BatchAChecksums.yml"

FIXTURE_FILES = (
    "BatchA_Securities_518880.mvsv",
    "BatchA_Futures_Gold.mvsv",
    "BatchA_Gap.mvsv",
    "BatchA_Errors.mvsv",
)


class FixtureError(ValueError):
    """夹具文件或 checksum 不满足固定基准。"""


def sha256OfFile(path: Path) -> str:
    """按字节计算文件 SHA-256（不做任何换行或编码规范化）。"""
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 16), b""):
            digest.update(chunk)
    return digest.hexdigest()


def computeFixtureChecksums() -> dict[str, str]:
    """计算批次 A 全部夹具文件的字节 SHA-256。"""
    result: dict[str, str] = {}
    for name in FIXTURE_FILES:
        path = FIXTURES_DIR / name
        if not path.is_file():
            raise FixtureError(f"夹具缺失: {name}")
        result[name] = sha256OfFile(path)
    return result


def loadExpectedChecksums() -> dict[str, str]:
    """读取固化在 BatchAChecksums.yml 的期望 checksum。"""
    if not CHECKSUM_FILE.is_file():
        raise FixtureError("缺少 BatchAChecksums.yml")
    payload = yaml.safe_load(CHECKSUM_FILE.read_text(encoding="utf-8"))
    if not isinstance(payload, dict) or "Files" not in payload:
        raise FixtureError("BatchAChecksums.yml 必须包含 Files 字段")
    files = payload["Files"]
    if not isinstance(files, dict):
        raise FixtureError("Files 必须是字典")
    return {str(key): str(value) for key, value in files.items()}


def verifyFixtureChecksums() -> dict[str, str]:
    """校验夹具与固化基准一致；任何差异抛出 FixtureError。"""
    actual = computeFixtureChecksums()
    expected = loadExpectedChecksums()
    if set(actual) != set(expected):
        raise FixtureError(f"夹具清单不一致: 实际 {sorted(actual)} 期望 {sorted(expected)}")
    for name, value in actual.items():
        if value != expected[name]:
            raise FixtureError(f"夹具 {name} checksum 不匹配: 实际 {value} 期望 {expected[name]}")
    return actual


def normalizedFixtureLines(name: str) -> list[str]:
    """读取夹具并拆分为纯数据行（去除头行与注释），供事件序列哈希。"""
    lines = (FIXTURES_DIR / name).read_text(encoding="utf-8").splitlines()
    return [line for line in lines if line and not line.startswith("#")]


def fixtureDataSequenceHash(name: str) -> str:
    """按规范化数据行计算序列哈希（跨平台固定，不依赖换行符）。"""
    return canonicalHash(normalizedFixtureLines(name), TsPrecision.Millisecond)
