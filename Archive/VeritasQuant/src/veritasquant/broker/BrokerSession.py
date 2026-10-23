"""P4-003 仿真券商认证、会话与最小权限凭据接入。

对齐 TechSpec 7.1 与 13 阶段 4：
- 凭据不入代码/配置/日志（只通过注入的密钥解析器提供，且绝不打日志）；
- 过期、撤销和轮换测试通过。

- `BrokerCredentialV1`：运行时凭据载体（不入日志，repr 打码）；
- `CredentialResolver`：凭据解析端口（由部署环境注入，如密钥服务）；
- `BrokerSessionV1`：仿真券商会话（认证状态、令牌生命周期、最小权限）；
- `SessionManagerV1`：会话建立/校验/轮换/撤销。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Protocol


class BrokerAuthError(ValueError):
    """券商认证或会话不满足契约时抛出。"""


@dataclass(frozen=True, slots=True)
class BrokerCredentialV1:
    """运行时凭据；repr 打码防止泄漏到日志。"""

    credentialId: str
    secret: str
    environment: str = "SIMULATION"

    def __post_init__(self) -> None:
        if not self.credentialId or not self.secret:
            raise BrokerAuthError("凭据标识与密钥不能为空")

    def __repr__(self) -> str:
        return f"BrokerCredentialV1(credentialId={self.credentialId!r}, secret='***')"


class CredentialResolver(Protocol):
    """凭据解析端口：由密钥服务实现，禁止把凭据写入代码/配置/日志。"""

    def resolve(self, credentialId: str) -> BrokerCredentialV1 | None: ...


class InMemoryCredentialResolverV1:
    """测试/演示用内存凭据解析器（仅模拟盘环境）。"""

    def __init__(self, credentials: dict[str, BrokerCredentialV1] | None = None) -> None:
        self._credentials = dict(credentials or {})

    def resolve(self, credentialId: str) -> BrokerCredentialV1 | None:
        return self._credentials.get(credentialId)

    def rotate(self, credentialId: str, newSecret: str) -> None:
        current = self._credentials.get(credentialId)
        if current is None:
            raise BrokerAuthError(f"凭据不存在: {credentialId}")
        self._credentials[credentialId] = BrokerCredentialV1(
            credentialId=credentialId,
            secret=newSecret,
            environment=current.environment,
        )

    def revoke(self, credentialId: str) -> None:
        if credentialId not in self._credentials:
            raise BrokerAuthError(f"凭据不存在: {credentialId}")
        del self._credentials[credentialId]


@dataclass(frozen=True, slots=True)
class BrokerSessionV1:
    """券商会话：认证令牌 + 生命周期。"""

    sessionId: str
    credentialId: str
    principalId: str
    tokenHash: str  # 令牌只存哈希，原始令牌绝不持久化
    expiresAt: datetime
    revoked: bool = False
    permissions: frozenset[str] = frozenset()  # 最小权限集合

    def __post_init__(self) -> None:
        if not self.sessionId or not self.credentialId or not self.principalId:
            raise BrokerAuthError("会话标识字段不能为空")

    @property
    def expired(self) -> bool:
        return datetime.now(timezone.utc) > self.expiresAt

    def hasPermission(self, permission: str) -> bool:
        return permission in self.permissions


class SessionManagerV1:
    """会话生命周期管理：认证、校验、轮换、撤销。

    凭据只通过 CredentialResolver 解析；会话只保存 tokenHash；任何日志
    路径不得包含原始凭据或令牌。
    """

    def __init__(
        self,
        resolver: CredentialResolver,
        *,
        sessionTtlMinutes: int = 30,
        permissions: frozenset[str] | None = None,
    ) -> None:
        if resolver is None:
            raise BrokerAuthError("凭据解析器不能为空")
        if sessionTtlMinutes <= 0:
            raise BrokerAuthError("会话有效期必须为正")
        self._resolver = resolver
        self._ttl = timedelta(minutes=sessionTtlMinutes)
        self._permissions = permissions or frozenset({"order:submit", "order:cancel", "order:query"})
        self._sessions: dict[str, BrokerSessionV1] = {}
        self._counter = 0

    def authenticate(self, credentialId: str) -> BrokerSessionV1:
        """用凭据建立会话；凭据不存在/被撤销 -> 拒绝。"""
        credential = self._resolver.resolve(credentialId)
        if credential is None:
            raise BrokerAuthError(f"凭据无效或已撤销: {credentialId}")
        self._counter += 1
        token = f"token-{self._counter}-{credentialId}"
        session = BrokerSessionV1(
            sessionId=f"session-{self._counter:06d}",
            credentialId=credentialId,
            principalId=f"broker-principal-{credentialId}",
            tokenHash=_hashToken(token),
            expiresAt=datetime.now(timezone.utc) + self._ttl,
            revoked=False,
            permissions=self._permissions,
        )
        self._sessions[session.sessionId] = session
        return session

    def validate(self, session: BrokerSessionV1) -> BrokerSessionV1:
        """校验会话：必须已登记、未撤销、未过期。"""
        registered = self._sessions.get(session.sessionId)
        if registered is None:
            raise BrokerAuthError(f"会话未登记: {session.sessionId}")
        if registered.revoked:
            raise BrokerAuthError(f"会话已撤销: {session.sessionId}")
        if registered.expired:
            raise BrokerAuthError(f"会话已过期: {session.sessionId}")
        return registered

    def revoke(self, sessionId: str) -> None:
        """撤销会话（权限撤销后不能操作）。"""
        if sessionId not in self._sessions:
            raise BrokerAuthError(f"会话不存在: {sessionId}")
        current = self._sessions[sessionId]
        self._sessions[sessionId] = BrokerSessionV1(
            sessionId=current.sessionId,
            credentialId=current.credentialId,
            principalId=current.principalId,
            tokenHash=current.tokenHash,
            expiresAt=current.expiresAt,
            revoked=True,
            permissions=current.permissions,
        )

    def rotateCredential(self, credentialId: str, newSecret: str) -> None:
        """轮换凭据：旧会话保持有效到过期，新认证使用新凭据。"""
        if not isinstance(self._resolver, InMemoryCredentialResolverV1):
            raise BrokerAuthError("当前凭据解析器不支持轮换")
        self._resolver.rotate(credentialId, newSecret)

    def revokeCredential(self, credentialId: str) -> None:
        """撤销凭据：同时撤销其全部会话。"""
        if not isinstance(self._resolver, InMemoryCredentialResolverV1):
            raise BrokerAuthError("当前凭据解析器不支持撤销")
        self._resolver.revoke(credentialId)
        for session in list(self._sessions.values()):
            if session.credentialId == credentialId and not session.revoked:
                self.revoke(session.sessionId)


def _hashToken(token: str) -> str:
    import hashlib

    return hashlib.sha256(token.encode("utf-8")).hexdigest()
