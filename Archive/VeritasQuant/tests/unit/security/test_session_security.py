"""P5-004 TLS、短期令牌和生产会话安全测试。"""

from __future__ import annotations

import pytest

from veritasquant.security.SessionSecurity import (
    SessionSecurityError,
    SessionSecurityServiceV1,
    ShortLivedTokenV1,
    TlsPolicyV1,
)


class TestTlsPolicy:
    def test_tls12_13_ok(self) -> None:
        policy = TlsPolicyV1()
        policy.validateProtocol("TLSv1.2")
        policy.validateProtocol("TLSv1.3")

    def test_tls11_rejected(self) -> None:
        policy = TlsPolicyV1()
        with pytest.raises(SessionSecurityError, match="不支持的 TLS"):
            policy.validateProtocol("TLSv1.1")

    def test_weak_cipher_rejected(self) -> None:
        policy = TlsPolicyV1()
        with pytest.raises(SessionSecurityError, match="弱密码套件"):
            policy.validateCipherSuite("TLS_RSA_WITH_AES_128_CBC_SHA")

    def test_strong_cipher_ok(self) -> None:
        policy = TlsPolicyV1()
        policy.validateCipherSuite("TLS_AES_256_GCM_SHA384")  # 不抛


class TestSessionSecurityService:
    def test_issue_token_hashes_value(self) -> None:
        """令牌只存哈希。"""
        service = SessionSecurityServiceV1()
        token, tokenValue = service.issue("principal-001")
        assert token.tokenHash != tokenValue  # 哈希 != 明文
        assert len(token.tokenHash) == 64  # SHA-256

    def test_validate_header_ok(self) -> None:
        service = SessionSecurityServiceV1()
        _, tokenValue = service.issue("principal-001")
        token = service.validate(tokenValue, transport="header")
        assert token.principalId == "principal-001"

    def test_token_in_url_rejected(self) -> None:
        """令牌不进 URL。"""
        service = SessionSecurityServiceV1()
        _, tokenValue = service.issue("principal-001")
        with pytest.raises(SessionSecurityError, match="不得通过 URL"):
            service.validate(tokenValue, transport="url")

    def test_consume_prevents_replay(self) -> None:
        """一次性消费：使用后重放拒绝。"""
        service = SessionSecurityServiceV1()
        _, tokenValue = service.issue("principal-001")
        service.consume(tokenValue)
        with pytest.raises(SessionSecurityError, match="重放"):
            service.validate(tokenValue)

    def test_revoke_invalidates(self) -> None:
        service = SessionSecurityServiceV1()
        token, tokenValue = service.issue("principal-001")
        service.revoke(token.tokenId)
        with pytest.raises(SessionSecurityError, match="已撤销"):
            service.validate(tokenValue)

    def test_expired_token_rejected(self) -> None:
        service = SessionSecurityServiceV1(tokenTtlMinutes=1)
        token, tokenValue = service.issue("principal-001")
        from datetime import timedelta

        expired = ShortLivedTokenV1(
            tokenId=token.tokenId,
            principalId=token.principalId,
            tokenHash=token.tokenHash,
            expiresAt=token.expiresAt - timedelta(minutes=5),
            issuedAt=token.issuedAt,
        )
        service._tokens[token.tokenId] = expired  # type: ignore[attr-defined]
        with pytest.raises(SessionSecurityError, match="已过期"):
            service.validate(tokenValue)

    def test_invalid_token_rejected(self) -> None:
        service = SessionSecurityServiceV1()
        with pytest.raises(SessionSecurityError, match="无效"):
            service.validate("bogus-token")

    def test_token_limit(self) -> None:
        service = SessionSecurityServiceV1(maxTokensPerPrincipal=2)
        service.issue("principal-001")
        service.issue("principal-001")
        with pytest.raises(SessionSecurityError, match="超限"):
            service.issue("principal-001")

    def test_unknown_revoke_rejected(self) -> None:
        service = SessionSecurityServiceV1()
        with pytest.raises(SessionSecurityError, match="不存在"):
            service.revoke("token-unknown")
