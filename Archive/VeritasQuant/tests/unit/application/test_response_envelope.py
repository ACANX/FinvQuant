from __future__ import annotations

import pytest
from pydantic import ValidationError

from veritasquant.application.ApiErrors import ApiErrorCatalog, BusinessException
from veritasquant.application.ResponseEnvelope import ResponseEnvelopeV1, mapException


def test_success_response_omits_error_and_unused_fields() -> None:
    response = ResponseEnvelopeV1.success(0, "成功", data={"run_id": "run-1"})
    assert response.toWire() == {"code": 0, "message": "成功", "data": {"run_id": "run-1"}}
    with pytest.raises(ValidationError):
        ResponseEnvelopeV1.model_validate(
            {
                "code": 0,
                "message": "成功",
                "error": {"code": "INVALID", "catalog_version": "1.0", "retryable": False},
            }
        )


def test_business_exception_uses_catalog_http_error_and_filtered_details() -> None:
    catalog = ApiErrorCatalog.loadPackaged()
    result = mapException(
        BusinessException(6201, {"requiredAmount": "1000.00", "availableCash": "800.00"}),
        catalog,
        requestId="req-1",
        traceId="trace-1",
    )
    assert result.httpStatus == 422
    assert result.envelope.toWire() == {
        "code": 6201,
        "message": "account.insufficient_available_cash",
        "error": {"code": "INSUFFICIENT_AVAILABLE_CASH", "catalog_version": "1.0", "retryable": False},
        "details": {"required_amount": "1000.00", "available_cash": "800.00"},
        "request_id": "req-1",
        "trace_id": "trace-1",
    }


def test_unknown_or_sensitive_business_details_fail_closed_to_registered_500() -> None:
    catalog = ApiErrorCatalog.loadPackaged()
    result = mapException(BusinessException(6201, {"token": "do-not-leak"}), catalog)
    assert result.httpStatus == 500
    assert result.envelope.toWire()["error"]["code"] == "INTERNAL_SERVER_ERROR"
