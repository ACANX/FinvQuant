from __future__ import annotations

from decimal import Decimal
from pathlib import Path

import pytest

from veritasquant.application.Config import ConfigError, ConfigLayer, RuntimeConfigV1, loadAndFreezeConfig, loadYaml, mergeConfigLayers
from veritasquant.core.Models import PascalAlias, StrictModel


def configValues(riskPolicyVersion: str = "V1") -> dict[str, object]:
    return {
        "ConfigSchemaVersion": "V1",
        "ExecutionMode": "BACKTEST",
        "TsPrecision": "Second",
        "RiskPolicyVersion": riskPolicyVersion,
    }


def test_equivalent_yaml_order_produces_same_config_hash(tmp_path: Path) -> None:
    firstPath = tmp_path / "First.yml"
    secondPath = tmp_path / "Second.yml"
    firstPath.write_text("ConfigSchemaVersion: V1\nExecutionMode: BACKTEST\nTsPrecision: Second\nRiskPolicyVersion: V1\n", encoding="utf-8")
    secondPath.write_text("RiskPolicyVersion: V1\nTsPrecision: Second\nExecutionMode: BACKTEST\nConfigSchemaVersion: V1\n", encoding="utf-8")
    first = loadAndFreezeConfig(RuntimeConfigV1, [ConfigLayer("Base", loadYaml(firstPath))])
    second = loadAndFreezeConfig(RuntimeConfigV1, [ConfigLayer("Base", loadYaml(secondPath))])
    assert first.configHash == second.configHash


def test_behavior_change_changes_hash_and_unapproved_override_is_rejected() -> None:
    first = loadAndFreezeConfig(RuntimeConfigV1, [ConfigLayer("Base", configValues("V1"))])
    second = loadAndFreezeConfig(RuntimeConfigV1, [ConfigLayer("Base", configValues("V2"))])
    assert first.configHash != second.configHash
    with pytest.raises(ConfigError):
        mergeConfigLayers([ConfigLayer("Base", configValues()), ConfigLayer("Environment", configValues("V2"))])
    merged = mergeConfigLayers(
        [ConfigLayer("Base", configValues()), ConfigLayer("Environment", configValues("V2"))],
        frozenset({("RiskPolicyVersion",)}),
    )
    assert merged["RiskPolicyVersion"] == "V2"


def test_duplicate_yaml_key_is_rejected(tmp_path: Path) -> None:
    path = tmp_path / "Duplicate.yml"
    path.write_text("ConfigSchemaVersion: V1\nConfigSchemaVersion: V2\n", encoding="utf-8")
    with pytest.raises(ConfigError):
        loadYaml(path)


class DecimalConfig(StrictModel):
    """用于验证 YAML 小数拼写不影响配置身份哈希的最小 Schema。"""

    configSchemaVersion: str = PascalAlias("ConfigSchemaVersion")
    amount: Decimal = PascalAlias("Amount")


class SensitiveConfig(StrictModel):
    """用于验证密钥值和机器绝对路径均不能进入冻结快照的最小 Schema。"""

    configSchemaVersion: str = PascalAlias("ConfigSchemaVersion")
    apiToken: str = PascalAlias("ApiToken")


class PathConfig(StrictModel):
    """用于验证资源必须使用逻辑身份而不是机器路径的最小 Schema。"""

    configSchemaVersion: str = PascalAlias("ConfigSchemaVersion")
    dataPath: str = PascalAlias("DataPath")


def test_yaml_decimal_spelling_is_normalized_without_float(tmp_path: Path) -> None:
    firstPath = tmp_path / "First.yml"
    secondPath = tmp_path / "Second.yml"
    firstPath.write_text("ConfigSchemaVersion: V1\nAmount: 1.0\n", encoding="utf-8")
    secondPath.write_text("Amount: 1.00\nConfigSchemaVersion: V1\n", encoding="utf-8")
    first = loadAndFreezeConfig(DecimalConfig, [ConfigLayer("Base", loadYaml(firstPath))])
    second = loadAndFreezeConfig(DecimalConfig, [ConfigLayer("Base", loadYaml(secondPath))])
    assert first.configHash == second.configHash


def test_secret_values_and_machine_absolute_paths_are_excluded_by_rejection() -> None:
    with pytest.raises(ConfigError):
        loadAndFreezeConfig(SensitiveConfig, [ConfigLayer("Base", {"ConfigSchemaVersion": "V1", "ApiToken": "secret"})])
    with pytest.raises(ConfigError):
        loadAndFreezeConfig(PathConfig, [ConfigLayer("Base", {"ConfigSchemaVersion": "V1", "DataPath": "C:\\data\\bars"})])
