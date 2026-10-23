"""P5-005 实盘双人授权和一次性确认凭证。

对齐 TechSpec 13 阶段 5：
- 同人双签、过期、payload/版本变化和重放全部拒绝并审计。

- `ApprovalRequestV1`：待双人授权的请求（payload 哈希 + 版本）；
- `ApprovalV1`：单次审批（审批人 + 时间 + 签名哈希）；
- `DualApprovalWorkflowV1`：双人授权流程（拒绝同人双签、过期、
  payload 变化、重放）；
- `OneTimeConfirmationV1`：一次性确认凭证（使用后失效防重放）。
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from enum import StrEnum


class ApprovalError(ValueError):
    """双人授权不满足契约时抛出。"""


class ApprovalState(StrEnum):
    Pending = "PENDING"
    SingleApproved = "SINGLE_APPROVED"
    Approved = "APPROVED"      # 双人通过
    Rejected = "REJECTED"


@dataclass(frozen=True, slots=True)
class ApprovalRequestV1:
    """待双人授权请求：payload 哈希 + 版本 + 过期时间。"""

    requestId: str
    payloadHash: str
    payloadVersion: str
    expiresAt: datetime
    createdBy: str

    def __post_init__(self) -> None:
        if not self.requestId or not self.payloadHash or not self.payloadVersion:
            raise ApprovalError("授权请求标识字段不能为空")
        if len(self.payloadHash) != 64:
            raise ApprovalError("payload 哈希必须为 SHA-256")

    @property
    def expired(self) -> bool:
        return datetime.now(timezone.utc) > self.expiresAt


@dataclass(frozen=True, slots=True)
class ApprovalV1:
    """单次审批记录。"""

    approverId: str
    approvedAt: datetime
    signatureHash: str

    def __post_init__(self) -> None:
        if not self.approverId:
            raise ApprovalError("审批人不能为空")


class DualApprovalWorkflowV1:
    """双人授权：拒绝同人双签、过期、payload/版本变化和重放。"""

    def __init__(self, *, requestTtlMinutes: int = 15) -> None:
        if requestTtlMinutes <= 0:
            raise ApprovalError("授权请求有效期必须为正")
        self._ttl = timedelta(minutes=requestTtlMinutes)
        self._requests: dict[str, ApprovalRequestV1] = {}
        self._approvals: dict[str, list[ApprovalV1]] = {}
        self._consumed: set[str] = set()  # 已消费请求（防重放）
        self._counter = 0

    def createRequest(
        self,
        *,
        requestId: str,
        payload: dict[str, object],
        payloadVersion: str,
        createdBy: str,
    ) -> ApprovalRequestV1:
        """创建授权请求；payload 哈希固定。"""
        if requestId in self._requests:
            raise ApprovalError(f"授权请求已存在: {requestId}")
        request = ApprovalRequestV1(
            requestId=requestId,
            payloadHash=_payloadHash(payload),
            payloadVersion=payloadVersion,
            expiresAt=datetime.now(timezone.utc) + self._ttl,
            createdBy=createdBy,
        )
        self._requests[requestId] = request
        self._approvals[requestId] = []
        return request

    def approve(self, *, requestId: str, approverId: str, payload: dict[str, object], payloadVersion: str) -> ApprovalState:
        """审批：拒绝同人双签、过期、payload/版本变化、重放。"""
        request = self._requests.get(requestId)
        if request is None:
            raise ApprovalError(f"授权请求不存在: {requestId}")
        if requestId in self._consumed:
            raise ApprovalError("授权请求已消费（重放拒绝）")
        if request.expired:
            raise ApprovalError("授权请求已过期")
        if _payloadHash(payload) != request.payloadHash:
            raise ApprovalError("payload 已变化，拒绝审批")
        if payloadVersion != request.payloadVersion:
            raise ApprovalError("payload 版本已变化，拒绝审批")
        approvals = self._approvals[requestId]
        if any(a.approverId == approverId for a in approvals):
            raise ApprovalError("同人双签被拒绝")
        signature = _signature(requestId, approverId, request.payloadHash)
        approvals.append(
            ApprovalV1(
                approverId=approverId,
                approvedAt=datetime.now(timezone.utc),
                signatureHash=signature,
            )
        )
        return ApprovalState.Approved if len(approvals) >= 2 else ApprovalState.SingleApproved

    def consume(self, requestId: str) -> ApprovalRequestV1:
        """消费授权（一次性）：标记防重放；必须已双人通过。"""
        request = self._requests.get(requestId)
        if request is None:
            raise ApprovalError(f"授权请求不存在: {requestId}")
        if requestId in self._consumed:
            raise ApprovalError("授权请求已消费（重放拒绝）")
        if len(self._approvals[requestId]) < 2:
            raise ApprovalError("未完成双人授权，不得消费")
        self._consumed.add(requestId)
        return request

    def stateOf(self, requestId: str) -> ApprovalState:
        request = self._requests.get(requestId)
        if request is None:
            raise ApprovalError(f"授权请求不存在: {requestId}")
        if requestId in self._consumed:
            return ApprovalState.Approved
        count = len(self._approvals[requestId])
        if count == 0:
            return ApprovalState.Pending
        if count == 1:
            return ApprovalState.SingleApproved
        return ApprovalState.Approved

    def approvalsFor(self, requestId: str) -> tuple[ApprovalV1, ...]:
        return tuple(self._approvals.get(requestId, ()))


@dataclass(frozen=True, slots=True)
class OneTimeConfirmationV1:
    """一次性确认凭证：使用后失效防重放。"""

    confirmationId: str
    requestId: str
    tokenHash: str
    createdAt: datetime
    consumed: bool = False


class OneTimeConfirmationServiceV1:
    """一次性确认凭证：签发 -> 消费（单次有效）。"""

    def __init__(self) -> None:
        self._confirmations: dict[str, OneTimeConfirmationV1] = {}
        self._counter = 0

    def issue(self, requestId: str) -> tuple[OneTimeConfirmationV1, str]:
        """签发一次性确认凭证；返回 (记录, 明文 token)。"""
        self._counter += 1
        token = f"otc-{self._counter}-{requestId}"
        confirmation = OneTimeConfirmationV1(
            confirmationId=f"otc-{self._counter:06d}",
            requestId=requestId,
            tokenHash=_hash(token),
            createdAt=datetime.now(timezone.utc),
        )
        self._confirmations[confirmation.confirmationId] = confirmation
        return confirmation, token

    def consume(self, token: str, requestId: str) -> OneTimeConfirmationV1:
        """消费确认凭证：必须匹配请求且未使用。"""
        confirmation = next(
            (c for c in self._confirmations.values() if c.tokenHash == _hash(token)),
            None,
        )
        if confirmation is None:
            raise ApprovalError("确认凭证无效")
        if confirmation.consumed:
            raise ApprovalError("确认凭证已使用（重放拒绝）")
        if confirmation.requestId != requestId:
            raise ApprovalError("确认凭证与请求不匹配")
        self._confirmations[confirmation.confirmationId] = OneTimeConfirmationV1(
            confirmationId=confirmation.confirmationId,
            requestId=confirmation.requestId,
            tokenHash=confirmation.tokenHash,
            createdAt=confirmation.createdAt,
            consumed=True,
        )
        return confirmation


def _payloadHash(payload: dict[str, object]) -> str:
    return hashlib.sha256(
        _canonical(payload).encode("utf-8")
    ).hexdigest()


def _canonical(payload: dict[str, object]) -> str:
    import json

    return json.dumps(payload, sort_keys=True, separators=(",", ":"))


def _signature(requestId: str, approverId: str, payloadHash: str) -> str:
    return hashlib.sha256(
        f"{requestId}|{approverId}|{payloadHash}".encode("utf-8")
    ).hexdigest()


def _hash(value: str) -> str:
    return hashlib.sha256(value.encode("utf-8")).hexdigest()
