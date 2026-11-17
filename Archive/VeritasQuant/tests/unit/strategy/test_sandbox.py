from __future__ import annotations

import time
from decimal import Decimal


from veritasquant.execution.Orders import OrderSide, OrderType
from veritasquant.strategy.Sandbox import (
    IpcEnvelopeV1,
    SandboxDisposition,
    SandboxQuotaV1,
    SandboxedStrategyRunnerV1,
    StrategySourceScannerV1,
)


def _quota(**overrides: object) -> SandboxQuotaV1:
    values: dict[str, object] = {}
    values.update(overrides)
    return SandboxQuotaV1(**values)  # type: ignore[call-arg]


def test_blocks_forbidden_imports() -> None:
    scanner = StrategySourceScannerV1()
    result = scanner.scan("import os\nimport subprocess\n")
    assert result.blocked
    assert any("os" in item for item in result.violations)
    assert any("subprocess" in item for item in result.violations)


def test_blocks_forbidden_from_imports_and_calls() -> None:
    scanner = StrategySourceScannerV1()
    result = scanner.scan("from socket import socket\nopen('secret')\n")
    assert result.blocked
    assert any("socket" in item for item in result.violations)
    assert any("CALL_FORBIDDEN:open" in item for item in result.violations)


def test_blocks_nondeterministic_sources() -> None:
    scanner = StrategySourceScannerV1()
    result = scanner.scan("import random\nimport time\n")
    assert result.blocked
    assert any("NONDETERMINISTIC" in item for item in result.violations)


def test_blocks_time_and_entropy_attributes() -> None:
    scanner = StrategySourceScannerV1()
    result = scanner.scan("from datetime import datetime\nx = datetime.now()\n")
    assert result.blocked
    assert any("datetime.now" in item for item in result.violations)


def test_accepts_safe_strategy_source() -> None:
    scanner = StrategySourceScannerV1()
    safe = """
from decimal import Decimal

def onBar(context):
    close = Decimal("1.200")
    return close
"""
    result = scanner.scan(safe)
    assert not result.blocked
    assert result.violations == ()


def test_quota_hash_is_stable_and_versioned() -> None:
    assert _quota().quotaHash() == _quota().quotaHash()
    assert _quota().quotaHash() != _quota(maxOrderIntents=10).quotaHash()


def test_timeout_discards_all_output() -> None:
    quota = _quota(callbackWallSeconds=0.01)
    runner = SandboxedStrategyRunnerV1(quota=quota)

    def slowCallback() -> None:
        time.sleep(0.1)

    outcome = runner.runCallback(slowCallback)
    assert outcome.disposition is SandboxDisposition.Timeout
    assert outcome.intents == ()
    assert runner.lastDisposition is SandboxDisposition.Timeout


def test_callback_exception_is_violation() -> None:
    runner = SandboxedStrategyRunnerV1(quota=_quota())

    def brokenCallback() -> None:
        raise RuntimeError("boom")

    outcome = runner.runCallback(brokenCallback)
    assert outcome.disposition is SandboxDisposition.Violation
    assert outcome.intents == ()


def test_normal_callback_accepts_intents() -> None:
    runner = SandboxedStrategyRunnerV1(quota=_quota())
    intents = _collectIntents()

    class _Stub:
        _intents: list = []

        def run(self) -> None:
            self._intents = intents

    stub = _Stub()
    outcome = runner.runCallback(stub.run)
    assert outcome.disposition is SandboxDisposition.Accepted
    assert len(outcome.intents) == 2


def _collectIntents() -> list:
    from datetime import datetime, timezone

    from veritasquant.execution.Orders import (
        OrderIntentV1,
        PositionEffect,
        TimeInForce,
    )

    def make(intentId: str) -> OrderIntentV1:
        return OrderIntentV1.model_validate(
            {
                "IntentId": intentId,
                "RunId": "run-1",
                "AccountId": "account-1",
                "SubaccountId": "strategy-1",
                "StrategyId": "strategy-1",
                "StrategyVersion": "1.0.0",
                "Symbol": "518880",
                "InstrumentMetadataVersion": "meta-v1",
                "Side": OrderSide.Buy,
                "PositionEffect": PositionEffect.Open,
                "OrderType": OrderType.Market,
                "Quantity": Decimal("100"),
                "TimeInForce": TimeInForce.Day,
                "Ts": datetime(2026, 8, 2, tzinfo=timezone.utc),
                "CreatedFromEventId": "event-100",
                "ExpectedAccountVersion": 5,
            }
        )

    return [make("intent-1"), make("intent-2")]


def test_ipc_envelope_is_versioned_and_verified() -> None:
    runner = SandboxedStrategyRunnerV1(quota=_quota())
    envelope = runner.makeIpcEnvelope("strategy.context", {"symbol": "518880"})
    assert envelope.ipcVersion == "V1"
    assert runner.verifyIpc(envelope)
    tampered = IpcEnvelopeV1("V1", "strategy.context", {"symbol": "518880"}, "0" * 64)
    assert not runner.verifyIpc(tampered)


def test_rejects_unknown_ipc_version() -> None:
    runner = SandboxedStrategyRunnerV1(quota=_quota())
    envelope = IpcEnvelopeV1("V2", "strategy.context", {}, "0" * 64)
    assert not runner.verifyIpc(envelope)
