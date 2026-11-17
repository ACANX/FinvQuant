"""P5-004 TLS、短期令牌和生产会话安全。

对齐 TechSpec 13 阶段 5：
- TLS 1.2+（TLS 配置校验：协议版本 + 密码套件黑名单）；
- 令牌不进 URL（令牌只经 Header/Body 传递，拒绝 URL 参数来源）；
- 过期/撤销/重放测试通过。

- `TlsPolicyV1`：TLS 1.2+ 协议与密码套件策略；
- `ShortLivedTokenV1`：短期令牌（哈希存储、过期、撤销、单次使用防重放）；
- `SessionSecurityServiceV1`：令牌签发/校验/撤销/重放防护。
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone


class SessionSecurityError(ValueError):
    """会话安全不满足契约时抛出。"""


@dataclass(frozen=True, slots=True)
class TlsPolicyV1:
    """TLS 1.2+ 协议与密码套件策略。"""

    minProtocolVersion: str = "TLSv1.2"
    forbiddenCipherSuites: frozenset[str] = frozenset(
        {
            "TLS_RSA_WITH_AES_128_CBC_SHA",
            "TLS_RSA_WITH_AES_256_CBC_SHA",
            "TLS_RSA_WITH_3DES_EDE_CBC_SHA",
            "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
        }
    )

    def validateProtocol(self, protocolVersion: str) -> None:
        """拒绝低于 TLS 1.2 的协议。"""
        allowed = {"TLSv1.2", "TLSv1.3"}
        if protocolVersion not in allowed:
            raise SessionSecurityError(f"不支持的 TLS 协议: {protocolVersion}")

    def validateCipherSuite(self, cipherSuite: str) -> None:
        """拒绝已知弱密码套件。"""
        if cipherSuite in self.forbiddenCipherSuites:
            raise SessionSecurityError(f"弱密码套件被拒绝: {cipherSuite}")


@dataclass(frozen=True, slots=True)
class ShortLivedTokenV1:
    """短期令牌：哈希存储、过期、撤销、单次使用防重放。"""

    tokenId: str
    principalId: str
    tokenHash: str  # 令牌只存哈希
    expiresAt: datetime
    issuedAt: datetime
    revoked: bool = False
    used: bool = False  # 一次性令牌：使用后失效（防重放）

    def __post_init__(self) -> None:
        if not self.tokenId or not self.principalId:
            raise SessionSecurityError("令牌标识字段不能为空")


class SessionSecurityServiceV1:
    """生产会话安全：签发短期令牌 + 校验（过期/撤销/重放防护）。

    令牌不进 URL：签发时记录令牌值，仅通过 Header/Body 传递；
    校验时先验证哈希，再检查过期/撤销/已使用（防重放）。
    """

    def __init__(self, *, tokenTtlMinutes: int = 5, maxTokensPerPrincipal: int = 10) -> None:
        if tokenTtlMinutes <= 0 or maxTokensPerPrincipal <= 0:
            raise SessionSecurityError("令牌有效期与上限必须为正")
        self._ttl = timedelta(minutes=tokenTtlMinutes)
        self._maxTokens = maxTokensPerPrincipal
        self._tokens: dict[str, ShortLivedTokenV1] = {}
        self._counter = 0

    def issue(self, principalId: str) -> tuple[ShortLivedTokenV1, str]:
        """签发短期令牌；返回 (记录, 明文令牌)。明文只此一次返回。"""
        active = [
            t
            for t in self._tokens.values()
            if t.principalId == principalId
            and not t.revoked
            and datetime.now(timezone.utc) <= t.expiresAt
        ]
        if len(active) >= self._maxTokens:
            raise SessionSecurityError(f"主体 {principalId} 活跃令牌超限")
        self._counter += 1
        tokenValue = f"st-{self._counter}-{principalId}"
        now = datetime.now(timezone.utc)
        token = ShortLivedTokenV1(
            tokenId=f"token-{self._counter:06d}",
            principalId=principalId,
            tokenHash=_hash(tokenValue),
            expiresAt=now + self._ttl,
            issuedAt=now,
        )
        self._tokens[token.tokenId] = token
        return token, tokenValue

    def validate(self, tokenValue: str, transport: str = "header") -> ShortLivedTokenV1:
        """校验令牌：transport 必须是 header/body（令牌不进 URL）；防重放。"""
        if transport == "url":
            raise SessionSecurityError("令牌不得通过 URL 传递")
        tokenHash = _hash(tokenValue)
        token = next((t for t in self._tokens.values() if t.tokenHash == tokenHash), None)
        if token is None:
            raise SessionSecurityError("令牌无效")
        if token.revoked:
            raise SessionSecurityError("令牌已撤销")
        if datetime.now(timezone.utc) > token.expiresAt:
            raise SessionSecurityError("令牌已过期")
        if token.used:
            raise SessionSecurityError("令牌已使用（重放拒绝）")
        return token

    def consume(self, tokenValue: str) -> ShortLivedTokenV1:
        """一次性消费：使用后标记 used（防重放）。"""
        token = self.validate(tokenValue)
        self._tokens[token.tokenId] = ShortLivedTokenV1(
            tokenId=token.tokenId,
            principalId=token.principalId,
            tokenHash=token.tokenHash,
            expiresAt=token.expiresAt,
            issuedAt=token.issuedAt,
            revoked=token.revoked,
            used=True,
        )
        return token

    def revoke(self, tokenId: str) -> None:
        token = self._tokens.get(tokenId)
        if token is None:
            raise SessionSecurityError(f"令牌不存在: {tokenId}")
        self._tokens[tokenId] = ShortLivedTokenV1(
            tokenId=token.tokenId,
            principalId=token.principalId,
            tokenHash=token.tokenHash,
            expiresAt=token.expiresAt,
            issuedAt=token.issuedAt,
            revoked=True,
            used=token.used,
        )

    def tokenCount(self) -> int:
        return len(self._tokens)


def _hash(value: str) -> str:
    return hashlib.sha256(value.encode("utf-8")).hexdigest()
