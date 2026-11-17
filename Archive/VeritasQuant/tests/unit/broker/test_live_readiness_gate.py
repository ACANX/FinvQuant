"""P5-009 生产 trading-readiness 门禁测试。"""

from __future__ import annotations


from veritasquant.broker.TradingReadiness import (
    CheckStatus,
    ReadinessCheckName,
    TradingReadinessGateV1,
)


def _record_all(gate: TradingReadinessGateV1, status: CheckStatus = CheckStatus.Pass) -> None:
    for check in ReadinessCheckName:
        gate.record(check, status)


class TestTradingReadinessGate:
    def test_all_pass_ready(self) -> None:
        gate = TradingReadinessGateV1()
        _record_all(gate)
        verdict = gate.evaluate()
        assert verdict.ready is True
        assert len(verdict.results) == 8
        assert gate.canSubmitOrder() is True

    def test_any_fail_blocks(self) -> None:
        """任一不合格即禁止发单。"""
        gate = TradingReadinessGateV1()
        _record_all(gate)
        gate.record(ReadinessCheckName.Broker, CheckStatus.Fail, "券商断连")
        verdict = gate.evaluate()
        assert verdict.ready is False
        assert gate.canSubmitOrder() is False
        assert len(verdict.failedChecks) == 1
        assert verdict.failedChecks[0].checkName is ReadinessCheckName.Broker

    def test_missing_check_fails(self) -> None:
        """未执行的检查视为 FAIL。"""
        gate = TradingReadinessGateV1()
        for check in list(ReadinessCheckName)[:7]:
            gate.record(check, CheckStatus.Pass)
        verdict = gate.evaluate()
        assert verdict.ready is False
        assert any(r.checkName is ReadinessCheckName.Sandbox and r.status is CheckStatus.Fail for r in verdict.results)

    def test_no_evaluation_blocks_order(self) -> None:
        gate = TradingReadinessGateV1()
        assert gate.canSubmitOrder() is False  # 未评估禁止发单

    def test_verdict_history(self) -> None:
        gate = TradingReadinessGateV1()
        _record_all(gate)
        gate.evaluate()
        _record_all(gate, CheckStatus.Fail)
        gate.evaluate()
        assert len(gate.verdicts()) == 2

    def test_latest_verdict_controls(self) -> None:
        """最新评估结果控制发单。"""
        gate = TradingReadinessGateV1()
        _record_all(gate)
        gate.evaluate()  # ready
        assert gate.canSubmitOrder() is True
        gate.record(ReadinessCheckName.Clock, CheckStatus.Fail, "时钟偏移")
        gate.evaluate()  # not ready
        assert gate.canSubmitOrder() is False
