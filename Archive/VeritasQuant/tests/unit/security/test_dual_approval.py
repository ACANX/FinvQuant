"""P5-005 双人授权和一次性确认凭证测试。"""

from __future__ import annotations

import pytest

from veritasquant.security.DualApproval import (
    ApprovalError,
    ApprovalState,
    DualApprovalWorkflowV1,
    OneTimeConfirmationServiceV1,
)

_PAYLOAD = {"account": "acc-001", "action": "increase_limit", "amount": "10000"}


class TestDualApprovalWorkflow:
    def _setup(self) -> tuple[DualApprovalWorkflowV1, str]:
        workflow = DualApprovalWorkflowV1()
        workflow.createRequest(
            requestId="req-001",
            payload=_PAYLOAD,
            payloadVersion="1.0",
            createdBy="alice",
        )
        return workflow, "req-001"

    def test_single_approval(self) -> None:
        workflow, requestId = self._setup()
        state = workflow.approve(
            requestId=requestId, approverId="alice", payload=_PAYLOAD, payloadVersion="1.0"
        )
        assert state is ApprovalState.SingleApproved

    def test_dual_approval(self) -> None:
        workflow, requestId = self._setup()
        workflow.approve(
            requestId=requestId, approverId="alice", payload=_PAYLOAD, payloadVersion="1.0"
        )
        state = workflow.approve(
            requestId=requestId, approverId="bob", payload=_PAYLOAD, payloadVersion="1.0"
        )
        assert state is ApprovalState.Approved
        assert len(workflow.approvalsFor(requestId)) == 2

    def test_same_person_double_sign_rejected(self) -> None:
        """同人双签被拒绝。"""
        workflow, requestId = self._setup()
        workflow.approve(
            requestId=requestId, approverId="alice", payload=_PAYLOAD, payloadVersion="1.0"
        )
        with pytest.raises(ApprovalError, match="同人双签"):
            workflow.approve(
                requestId=requestId, approverId="alice", payload=_PAYLOAD, payloadVersion="1.0"
            )

    def test_payload_change_rejected(self) -> None:
        """payload 变化拒绝审批。"""
        workflow, requestId = self._setup()
        with pytest.raises(ApprovalError, match="payload 已变化"):
            workflow.approve(
                requestId=requestId,
                approverId="alice",
                payload={"account": "acc-002", "action": "increase_limit", "amount": "10000"},
                payloadVersion="1.0",
            )

    def test_version_change_rejected(self) -> None:
        """版本变化拒绝审批。"""
        workflow, requestId = self._setup()
        with pytest.raises(ApprovalError, match="版本已变化"):
            workflow.approve(
                requestId=requestId, approverId="alice", payload=_PAYLOAD, payloadVersion="2.0"
            )

    def test_expired_request_rejected(self) -> None:
        """过期请求拒绝审批。"""
        workflow = DualApprovalWorkflowV1(requestTtlMinutes=1)
        workflow.createRequest(
            requestId="req-001",
            payload=_PAYLOAD,
            payloadVersion="1.0",
            createdBy="alice",
        )
        from datetime import timedelta

        request = workflow._requests["req-001"]  # type: ignore[attr-defined]
        from veritasquant.security.DualApproval import ApprovalRequestV1

        workflow._requests["req-001"] = ApprovalRequestV1(  # type: ignore[attr-defined]
            requestId=request.requestId,
            payloadHash=request.payloadHash,
            payloadVersion=request.payloadVersion,
            expiresAt=request.expiresAt - timedelta(minutes=5),
            createdBy=request.createdBy,
        )
        with pytest.raises(ApprovalError, match="已过期"):
            workflow.approve(
                requestId="req-001", approverId="alice", payload=_PAYLOAD, payloadVersion="1.0"
            )

    def test_consume_requires_dual(self) -> None:
        """未完成双人授权不得消费。"""
        workflow, requestId = self._setup()
        with pytest.raises(ApprovalError, match="未完成双人授权"):
            workflow.consume(requestId)

    def test_consume_once_no_replay(self) -> None:
        """消费后重放拒绝。"""
        workflow, requestId = self._setup()
        workflow.approve(
            requestId=requestId, approverId="alice", payload=_PAYLOAD, payloadVersion="1.0"
        )
        workflow.approve(
            requestId=requestId, approverId="bob", payload=_PAYLOAD, payloadVersion="1.0"
        )
        workflow.consume(requestId)
        with pytest.raises(ApprovalError, match="已消费"):
            workflow.consume(requestId)

    def test_unknown_request_rejected(self) -> None:
        workflow = DualApprovalWorkflowV1()
        with pytest.raises(ApprovalError, match="不存在"):
            workflow.approve(
                requestId="req-unknown", approverId="alice", payload=_PAYLOAD, payloadVersion="1.0"
            )

    def test_duplicate_request_rejected(self) -> None:
        workflow = DualApprovalWorkflowV1()
        workflow.createRequest(
            requestId="req-001",
            payload=_PAYLOAD,
            payloadVersion="1.0",
            createdBy="alice",
        )
        with pytest.raises(ApprovalError, match="已存在"):
            workflow.createRequest(
                requestId="req-001",
                payload=_PAYLOAD,
                payloadVersion="1.0",
                createdBy="alice",
            )


class TestOneTimeConfirmation:
    def test_issue_and_consume(self) -> None:
        service = OneTimeConfirmationServiceV1()
        confirmation, token = service.issue("req-001")
        consumed = service.consume(token, "req-001")
        assert consumed.requestId == "req-001"

    def test_replay_rejected(self) -> None:
        """一次性凭证重放拒绝。"""
        service = OneTimeConfirmationServiceV1()
        _, token = service.issue("req-001")
        service.consume(token, "req-001")
        with pytest.raises(ApprovalError, match="已使用"):
            service.consume(token, "req-001")

    def test_request_mismatch_rejected(self) -> None:
        service = OneTimeConfirmationServiceV1()
        _, token = service.issue("req-001")
        with pytest.raises(ApprovalError, match="不匹配"):
            service.consume(token, "req-002")

    def test_invalid_token_rejected(self) -> None:
        service = OneTimeConfirmationServiceV1()
        with pytest.raises(ApprovalError, match="无效"):
            service.consume("bogus", "req-001")
