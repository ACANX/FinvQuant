"""P4-003 仿真券商认证、会话与凭据测试。"""

from __future__ import annotations

import pytest

from veritasquant.broker.BrokerSession import (
    BrokerAuthError,
    BrokerCredentialV1,
    BrokerSessionV1,
    InMemoryCredentialResolverV1,
    SessionManagerV1,
)


def _resolver() -> InMemoryCredentialResolverV1:
    return InMemoryCredentialResolverV1(
        {
            "cred-sim-001": BrokerCredentialV1(
                credentialId="cred-sim-001",
                secret="super-secret-value",
                environment="SIMULATION",
            )
        }
    )


class TestBrokerCredential:
    def test_repr_masks_secret(self) -> None:
        """凭据绝不进入日志：repr 打码。"""
        credential = BrokerCredentialV1("cred-001", "plaintext-secret")
        assert "plaintext-secret" not in repr(credential)
        assert "***" in repr(credential)

    def test_requires_fields(self) -> None:
        with pytest.raises(BrokerAuthError):
            BrokerCredentialV1("", "secret")
        with pytest.raises(BrokerAuthError):
            BrokerCredentialV1("cred-001", "")


class TestSessionManager:
    def test_authenticate_creates_session(self) -> None:
        manager = SessionManagerV1(_resolver())
        session = manager.authenticate("cred-sim-001")
        assert session.sessionId.startswith("session-")
        assert session.credentialId == "cred-sim-001"
        # 令牌只存哈希
        assert len(session.tokenHash) == 64
        assert "token-" not in session.tokenHash

    def test_authenticate_invalid_credential_rejected(self) -> None:
        manager = SessionManagerV1(_resolver())
        with pytest.raises(BrokerAuthError, match="凭据无效"):
            manager.authenticate("cred-unknown")

    def test_validate_registered_session(self) -> None:
        manager = SessionManagerV1(_resolver())
        session = manager.authenticate("cred-sim-001")
        validated = manager.validate(session)
        assert validated.sessionId == session.sessionId

    def test_validate_unregistered_session_rejected(self) -> None:
        manager = SessionManagerV1(_resolver())
        session = manager.authenticate("cred-sim-001")
        forged = BrokerSessionV1(
            sessionId="session-forged",
            credentialId=session.credentialId,
            principalId=session.principalId,
            tokenHash=session.tokenHash,
            expiresAt=session.expiresAt,
        )
        with pytest.raises(BrokerAuthError, match="未登记"):
            manager.validate(forged)

    def test_revoke_session(self) -> None:
        """权限撤销后不能操作。"""
        manager = SessionManagerV1(_resolver())
        session = manager.authenticate("cred-sim-001")
        manager.revoke(session.sessionId)
        with pytest.raises(BrokerAuthError, match="已撤销"):
            manager.validate(session)

    def test_revoke_credential_revokes_sessions(self) -> None:
        manager = SessionManagerV1(_resolver())
        session = manager.authenticate("cred-sim-001")
        manager.revokeCredential("cred-sim-001")
        with pytest.raises(BrokerAuthError, match="凭据无效"):
            manager.authenticate("cred-sim-001")
        with pytest.raises(BrokerAuthError, match="已撤销"):
            manager.validate(session)

    def test_rotate_credential(self) -> None:
        """轮换后新认证用新凭据；旧会话到过期前仍有效。"""
        resolver = _resolver()
        manager = SessionManagerV1(resolver)
        old_session = manager.authenticate("cred-sim-001")
        manager.rotateCredential("cred-sim-001", "new-secret")
        new_session = manager.authenticate("cred-sim-001")
        assert new_session.sessionId != old_session.sessionId
        # 旧会话未被撤销（T+0 轮换语义），新会话可用
        manager.validate(old_session)
        manager.validate(new_session)

    def test_minimal_permissions(self) -> None:
        """最小权限集合：未授予的权限不可用。"""
        manager = SessionManagerV1(
            _resolver(), permissions=frozenset({"order:submit"})
        )
        session = manager.authenticate("cred-sim-001")
        assert session.hasPermission("order:submit") is True
        assert session.hasPermission("admin:*") is False

    def test_expired_session_rejected(self) -> None:
        manager = SessionManagerV1(_resolver(), sessionTtlMinutes=1)
        session = manager.authenticate("cred-sim-001")
        # 人为构造过期会话
        from datetime import timedelta

        expired = BrokerSessionV1(
            sessionId=session.sessionId,
            credentialId=session.credentialId,
            principalId=session.principalId,
            tokenHash=session.tokenHash,
            expiresAt=session.expiresAt - timedelta(minutes=5),
        )
        manager._sessions[session.sessionId] = expired  # type: ignore[attr-defined]
        with pytest.raises(BrokerAuthError, match="已过期"):
            manager.validate(session)
