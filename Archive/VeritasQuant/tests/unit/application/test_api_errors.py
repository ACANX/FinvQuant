from __future__ import annotations

import copy
from importlib import resources

import pytest
import yaml

from veritasquant.application.ApiErrors import ApiErrorCatalog, ApiErrorCatalogError
from veritasquant.application.ErrorCatalogCompatibility import validateCatalogCompatibility


def test_packaged_catalog_has_fixed_success_codes_and_stable_indexes() -> None:
    catalog = ApiErrorCatalog.loadPackaged()
    assert catalog.catalogVersion == "1.0"
    assert catalog.getError(6201).errorCode == "INSUFFICIENT_AVAILABLE_CASH"
    assert catalog.errorsBySymbol["INTERNAL_SERVER_ERROR"].code == 2006


def test_duplicate_numeric_code_and_invalid_domain_range_are_rejected() -> None:
    raw = {
        "ErrorCatalogVersion": "1.0",
        "SuccessCodes": [0, 1, 200, 202],
        "Errors": [
            {
                "Code": 3000,
                "ErrorCode": "COMMAND_REJECTED",
                "Domain": "General",
                "HttpStatus": 422,
                "Retryable": False,
                "MessageKey": "command.rejected",
                "IntroducedVersion": "1.0",
                "Deprecated": False,
                "DetailSchema": {},
            },
            {
                "Code": 3000,
                "ErrorCode": "INTERNAL_SERVER_ERROR",
                "Domain": "Platform",
                "HttpStatus": 500,
                "Retryable": False,
                "MessageKey": "platform.internal_server_error",
                "IntroducedVersion": "1.0",
                "Deprecated": False,
                "DetailSchema": {},
            },
        ],
    }
    with pytest.raises(ApiErrorCatalogError):
        ApiErrorCatalog.fromMapping(raw)


def test_public_details_are_whitelisted_and_mapped_to_snake_case() -> None:
    catalog = ApiErrorCatalog.loadPackaged()
    details = catalog.filterPublicDetails(6201, {"requiredAmount": "1000.00", "availableCash": "800.00"})
    assert details == {"required_amount": "1000.00", "available_cash": "800.00"}
    with pytest.raises(ApiErrorCatalogError):
        catalog.filterPublicDetails(6201, {"token": "secret"})


def test_reusing_published_code_with_changed_semantics_is_rejected() -> None:
    previous = ApiErrorCatalog.loadPackaged()
    raw = yaml.safe_load(resources.files("veritasquant.resources").joinpath("Schemas", "ApiErrorCodes.yml").read_text(encoding="utf-8"))
    candidateRaw = copy.deepcopy(raw)
    next(item for item in candidateRaw["Errors"] if item["Code"] == 6201)["HttpStatus"] = 409
    candidate = ApiErrorCatalog.fromMapping(candidateRaw)
    with pytest.raises(ApiErrorCatalogError):
        validateCatalogCompatibility(previous, candidate)
