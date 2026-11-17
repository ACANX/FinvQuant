from __future__ import annotations

import pytest
from pydantic import ValidationError

from veritasquant.core.Models import AliasContractError, PascalAlias, StrictModel


class ExampleConfig(StrictModel):
    """用于验证 PascalCase YAML 与内部 lowerCamelCase 的最小配置模型。"""

    displayName: str = PascalAlias("DisplayName")
    retryCount: int = PascalAlias("RetryCount")


def test_strict_model_rejects_unknown_fields_and_implicit_types() -> None:
    with pytest.raises(ValidationError):
        ExampleConfig.model_validate({"DisplayName": "alpha", "RetryCount": "3"})
    with pytest.raises(ValidationError):
        ExampleConfig.model_validate({"DisplayName": "alpha", "RetryCount": 3, "Extra": True})


def test_pascal_case_alias_round_trip_only() -> None:
    model = ExampleConfig.model_validate({"DisplayName": "alpha", "RetryCount": 3})
    assert model.displayName == "alpha"
    assert model.model_dump(by_alias=True) == {"DisplayName": "alpha", "RetryCount": 3}
    with pytest.raises(ValidationError):
        ExampleConfig.model_validate({"displayName": "alpha", "RetryCount": 3})


def test_model_without_unique_explicit_alias_is_rejected() -> None:
    with pytest.raises(AliasContractError):
        class InvalidModel(StrictModel):
            value: str
