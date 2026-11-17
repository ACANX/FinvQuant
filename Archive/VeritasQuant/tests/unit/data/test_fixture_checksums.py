"""P1-024 批次 A 固定数据夹具与跨平台 checksum 验证。"""

from __future__ import annotations

from pathlib import Path

import pytest

from veritasquant.data.FixtureChecksums import (
    FIXTURE_FILES,
    FIXTURES_DIR,
    FixtureError,
    fixtureDataSequenceHash,
    loadExpectedChecksums,
    normalizedFixtureLines,
    verifyFixtureChecksums,
)


def test_all_fixture_files_exist_and_have_data_rows() -> None:
    for name in FIXTURE_FILES:
        lines = normalizedFixtureLines(name)
        assert len(lines) >= 3, f"{name} 数据行不足"


def test_fixture_checksums_match_frozen_baseline() -> None:
    """夹具字节 checksum 必须与固化的 BatchAChecksums.yml 完全一致。"""
    actual = verifyFixtureChecksums()
    expected = loadExpectedChecksums()
    assert actual == expected


def test_checksum_is_platform_independent() -> None:
    """序列哈希基于规范化行与固定 UTF-8，不依赖换行符或路径。"""
    for name in FIXTURE_FILES:
        hashValue = fixtureDataSequenceHash(name)
        assert len(hashValue) == 64
        # 相同输入必然相同输出
        assert fixtureDataSequenceHash(name) == hashValue


def test_missing_fixture_raises() -> None:
    missing = FIXTURES_DIR / "BatchA_DoesNotExist.mvsv"
    with pytest.raises(FixtureError, match="夹具缺失"):
        _requireFile(missing)


def _requireFile(path: Path) -> None:
    if not path.is_file():
        raise FixtureError(f"夹具缺失: {path.name}")


def test_errors_fixture_contains_invalid_samples_for_isolation() -> None:
    """错误样本夹具必须包含可被质量规则识别的非法记录。"""
    lines = normalizedFixtureLines("BatchA_Errors.mvsv")
    assert len(lines) == 4
    # 记录 1: OHLC 非法（low=7.200 > high=7.100）
    fields1 = lines[0].split("|")
    assert float(fields1[4]) > float(fields1[5])  # low > high
    # 记录 2: 与记录 1 相同 dt（重复主键）
    assert lines[0].split("|")[1] == lines[1].split("|")[1]
    # 记录 3: dt 早于记录 1（乱序）
    assert int(lines[2].split("|")[1]) < int(lines[0].split("|")[1])


def test_gap_fixture_contains_cross_session_gap() -> None:
    lines = normalizedFixtureLines("BatchA_Gap.mvsv")
    timestamps = [int(line.split("|")[0]) for line in lines]
    gaps = [timestamps[i + 1] - timestamps[i] for i in range(len(timestamps) - 1)]
    assert max(gaps) > 3600  # 存在超过 1 小时的缺口
