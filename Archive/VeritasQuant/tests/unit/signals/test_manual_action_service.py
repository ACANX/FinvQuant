"""P3-004 人工确认、忽略和成交登记服务与 API 测试。"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from veritasquant.signals.ManualActionService import (
    InMemoryActionStoreV1,
    ManualActionError,
    ManualActionServiceV1,
)
from veritasquant.signals.SignalReference import (
    IgnoreReasonV1,
    SignalActionType,
    SignalReferenceV1,
    SignalStatus,
)

_T0 = datetime(2026, 8, 3, 2, 0, 0, tzinfo=timezone.utc)


def _signal(**overrides: object) -> SignalReferenceV1:
    values: dict[str, object] = {
        "signalReferenceId": "sig-ref-001",
        "version": 1,
        "status": SignalStatus.Pending,
        "accountId": "acc-001",
        "strategyId": "strat-dual-ma",
        "strategyChecksum": "a" * 64,
        "sourceEventId": "evt-bar-001",
        "sourceEventType": "MarketBarEvent",
        "direction": "BUY",
        "quantity": "100.0000",
        "priceLimit": "5.0000",
        "operatorId": None,
        "generatedTs": _T0,
        "expiresAt": _T0 + timedelta(minutes=15),
        "previousSignalReferenceId": None,
    }
    values.update(overrides)
    return SignalReferenceV1.create(**values)


class TestManualActionService:
    def test_record_confirm(self) -> None:
        store = InMemoryActionStoreV1()
        service = ManualActionServiceV1(store)
        action = service.recordAction(
            signal=_signal(),
            actionType=SignalActionType.Confirm,
            operatorId="op-alice",
            reason="按信号执行",
        )
        assert action.actionType is SignalActionType.Confirm
        assert action.operatorId == "op-alice"
        assert action.version == 1
        assert len(action.auditTrail) == 1
        assert store.getAction(action.actionId) is not None

    def test_record_ignore_with_reason(self) -> None:
        store = InMemoryActionStoreV1()
        service = ManualActionServiceV1(store)
        reason = IgnoreReasonV1.create(reasonCode="MARKET_CLOSED", detail="休市")
        action = service.recordAction(
            signal=_signal(),
            actionType=SignalActionType.Ignore,
            operatorId="op-alice",
            reason="休市忽略",
            ignoreReason=reason,
        )
        assert action.ignoreReason is not None
        assert action.ignoreReason.reasonCode == "MARKET_CLOSED"

    def test_record_ignore_without_reason_rejected(self) -> None:
        store = InMemoryActionStoreV1()
        service = ManualActionServiceV1(store)
        with pytest.raises(ManualActionError):
            service.recordAction(
                signal=_signal(),
                actionType=SignalActionType.Ignore,
                operatorId="op-alice",
                reason="忽略",
                ignoreReason=None,
            )

    def test_expired_signal_rejected(self) -> None:
        store = InMemoryActionStoreV1()
        service = ManualActionServiceV1(store)
        signal = _signal(status=SignalStatus.Expired)
        with pytest.raises(ManualActionError, match="过期"):
            service.recordAction(
                signal=signal,
                actionType=SignalActionType.Confirm,
                operatorId="op-alice",
                reason="x",
            )

    def test_duplicate_action_rejected(self) -> None:
        store = InMemoryActionStoreV1()
        service = ManualActionServiceV1(store)
        service.recordAction(
            signal=_signal(),
            actionType=SignalActionType.Confirm,
            operatorId="op-alice",
            reason="x",
            actionId="act-fixed",
        )
        with pytest.raises(ManualActionError, match="已存在"):
            service.recordAction(
                signal=_signal(),
                actionType=SignalActionType.Confirm,
                operatorId="op-alice",
                reason="x",
                actionId="act-fixed",
            )

    def test_record_execution(self) -> None:
        store = InMemoryActionStoreV1()
        service = ManualActionServiceV1(store)
        signal = _signal()
        action = service.recordAction(
            signal=signal,
            actionType=SignalActionType.RegisterExecution,
            operatorId="op-alice",
            reason="执行人工成交",
        )
        execution = service.recordExecution(
            signal=signal,
            action=action,
            operatorId="op-alice",
            direction="BUY",
            quantity="100.0000",
            price="5.0100",
        )
        assert execution.signalReferenceId == signal.signalReferenceId
        assert execution.actionId == action.actionId
        assert execution.price == "5.0100"

    def test_execution_action_type_mismatch_rejected(self) -> None:
        store = InMemoryActionStoreV1()
        service = ManualActionServiceV1(store)
        signal = _signal()
        action = service.recordAction(
            signal=signal,
            actionType=SignalActionType.Confirm,
            operatorId="op-alice",
            reason="x",
        )
        with pytest.raises(ManualActionError, match="REGISTER_EXECUTION"):
            service.recordExecution(
                signal=signal,
                action=action,
                operatorId="op-alice",
                direction="BUY",
                quantity="100.0000",
                price="5.0100",
            )

    def test_execution_signal_mismatch_rejected(self) -> None:
        store = InMemoryActionStoreV1()
        service = ManualActionServiceV1(store)
        signalA = _signal()
        signalB = _signal(signalReferenceId="sig-ref-002")
        action = service.recordAction(
            signal=signalA,
            actionType=SignalActionType.RegisterExecution,
            operatorId="op-alice",
            reason="x",
        )
        with pytest.raises(ManualActionError, match="不匹配"):
            service.recordExecution(
                signal=signalB,
                action=action,
                operatorId="op-alice",
                direction="BUY",
                quantity="100.0000",
                price="5.0100",
            )


class TestSignalApi:
    def test_record_action_via_api(self) -> None:
        from fastapi import FastAPI
        from fastapi.testclient import TestClient

        from veritasquant.apps.server.SignalRoutes import SignalApi, buildSignalRouter
        from veritasquant.application.ApiErrors import ApiErrorCatalog

        store = InMemoryActionStoreV1()
        service = ManualActionServiceV1(store)
        catalog = ApiErrorCatalog.loadPackaged()
        api = SignalApi(service, catalog, signalResolver=lambda signalId: _signal())
        app = FastAPI()
        app.include_router(buildSignalRouter(api))
        client = TestClient(app)
        response = client.post(
            "/api/v1/signals/sig-ref-001/actions",
            json={
                "action_type": "CONFIRM",
                "operator_id": "op-alice",
                "reason": "按信号执行",
            },
        )
        assert response.status_code == 202
        body = response.json()
        assert body["data"]["action_type"] == "CONFIRM"
        assert body["data"]["operator_id"] == "op-alice"

    def test_record_ignore_with_reason_via_api(self) -> None:
        from fastapi import FastAPI
        from fastapi.testclient import TestClient

        from veritasquant.apps.server.SignalRoutes import SignalApi, buildSignalRouter
        from veritasquant.application.ApiErrors import ApiErrorCatalog

        store = InMemoryActionStoreV1()
        service = ManualActionServiceV1(store)
        catalog = ApiErrorCatalog.loadPackaged()
        api = SignalApi(service, catalog, signalResolver=lambda signalId: _signal())
        app = FastAPI()
        app.include_router(buildSignalRouter(api))
        client = TestClient(app)
        response = client.post(
            "/api/v1/signals/sig-ref-001/actions",
            json={
                "action_type": "IGNORE",
                "operator_id": "op-alice",
                "reason": "休市忽略",
                "ignore_reason_code": "MARKET_CLOSED",
            },
        )
        assert response.status_code == 202
        assert response.json()["data"]["ignore_reason"]["reason_code"] == "MARKET_CLOSED"

    def test_unknown_signal_404(self) -> None:
        from fastapi import FastAPI
        from fastapi.testclient import TestClient

        from veritasquant.apps.server.SignalRoutes import SignalApi, buildSignalRouter
        from veritasquant.application.ApiErrors import ApiErrorCatalog

        store = InMemoryActionStoreV1()
        service = ManualActionServiceV1(store)
        catalog = ApiErrorCatalog.loadPackaged()
        api = SignalApi(service, catalog, signalResolver=lambda signalId: None)
        app = FastAPI()
        app.include_router(buildSignalRouter(api))
        client = TestClient(app)
        response = client.post(
            "/api/v1/signals/missing/actions",
            json={"action_type": "CONFIRM", "operator_id": "op", "reason": "x"},
        )
        assert response.status_code == 400
        assert "信号不存在" in response.json()["message"]

    def test_record_execution_via_api(self) -> None:
        from fastapi import FastAPI
        from fastapi.testclient import TestClient

        from veritasquant.apps.server.SignalRoutes import SignalApi, buildSignalRouter
        from veritasquant.application.ApiErrors import ApiErrorCatalog

        store = InMemoryActionStoreV1()
        service = ManualActionServiceV1(store)
        catalog = ApiErrorCatalog.loadPackaged()
        signal = _signal()
        action = service.recordAction(
            signal=signal,
            actionType=SignalActionType.RegisterExecution,
            operatorId="op-alice",
            reason="执行人工成交",
        )
        api = SignalApi(service, catalog, signalResolver=lambda signalId: signal)
        app = FastAPI()
        app.include_router(buildSignalRouter(api))
        client = TestClient(app)
        response = client.post(
            "/api/v1/signals/sig-ref-001/executions",
            json={
                "action_id": action.actionId,
                "operator_id": "op-alice",
                "direction": "BUY",
                "quantity": "100.0000",
                "price": "5.0100",
            },
        )
        assert response.status_code == 202
        assert response.json()["data"]["price"] == "5.0100"
