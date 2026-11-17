"""P5-003 密钥服务、轮换、撤销和最小权限。

对齐 TechSpec 13 阶段 5：
- 仓库/日志无秘密（密钥只经服务解析，repr 打码，绝不落盘/日志）；
- 轮换不中断审计（审计引用密钥版本，轮换后旧版本可追溯）；
- 撤销后旧凭据立即失效。

- `ManagedSecretV1`：托管密钥（版本化，repr 打码）；
- `SecretAuditRecordV1`：密钥审计（创建/轮换/撤销/访问）；
- `SecretServiceV1`：密钥生命周期（解析/轮换/撤销/最小权限访问）。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from enum import StrEnum


class SecretError(ValueError):
    """密钥服务不满足契约时抛出。"""


class SecretAuditAction(StrEnum):
    Created = "CREATED"
    Rotated = "ROTATED"
    Revoked = "REVOKED"
    Accessed = "ACCESSED"


@dataclass(frozen=True, slots=True)
class ManagedSecretV1:
    """托管密钥：版本化；repr 打码防止泄漏到日志。"""

    secretId: str
    version: int
    value: str
    environment: str
    createdAt: datetime
    revoked: bool = False

    def __post_init__(self) -> None:
        if not self.secretId or not self.value:
            raise SecretError("密钥标识与值不能为空")
        if self.version < 1:
            raise SecretError("密钥版本必须为正")

    def __repr__(self) -> str:
        return f"ManagedSecretV1(secretId={self.secretId!r}, version={self.version}, value='***')"


@dataclass(frozen=True, slots=True)
class SecretAuditRecordV1:
    """密钥审计记录（轮换不中断审计）。"""

    auditId: str
    secretId: str
    version: int
    action: SecretAuditAction
    actor: str
    at: datetime
    detail: str = ""


class SecretServiceV1:
    """密钥生命周期：创建/解析/轮换/撤销 + 审计 + 最小权限。

    仓库/日志无秘密：密钥值只存在于内存 ManagedSecretV1，不写入任何日志；
    轮换保留版本历史（审计可追溯）；撤销立即失效。
    """

    def __init__(self) -> None:
        self._secrets: dict[str, list[ManagedSecretV1]] = {}  # secretId -> 版本历史
        self._audit: list[SecretAuditRecordV1] = []
        self._counter = 0

    def create(self, *, secretId: str, value: str, environment: str, actor: str) -> ManagedSecretV1:
        """创建密钥 v1；重复创建被拒绝。"""
        if secretId in self._secrets:
            raise SecretError(f"密钥已存在: {secretId}")
        secret = ManagedSecretV1(
            secretId=secretId,
            version=1,
            value=value,
            environment=environment,
            createdAt=datetime.now(timezone.utc),
        )
        self._secrets[secretId] = [secret]
        self._audit.append(self._record(secretId, 1, SecretAuditAction.Created, actor))
        return secret

    def resolve(self, secretId: str, actor: str) -> ManagedSecretV1:
        """解析当前有效密钥；撤销后立即失效；访问留审计。"""
        history = self._secrets.get(secretId)
        if not history:
            raise SecretError(f"密钥不存在: {secretId}")
        current = history[-1]
        if current.revoked:
            raise SecretError(f"密钥已撤销: {secretId}")
        self._audit.append(self._record(secretId, current.version, SecretAuditAction.Accessed, actor))
        return current

    def rotate(self, *, secretId: str, newValue: str, actor: str) -> ManagedSecretV1:
        """轮换：追加新版本；旧版本保留可审计（轮换不中断审计）。"""
        history = self._secrets.get(secretId)
        if not history:
            raise SecretError(f"密钥不存在: {secretId}")
        if history[-1].revoked:
            raise SecretError(f"已撤销密钥不可轮换: {secretId}")
        if newValue == history[-1].value:
            raise SecretError("新密钥不得与当前密钥相同")
        secret = ManagedSecretV1(
            secretId=secretId,
            version=history[-1].version + 1,
            value=newValue,
            environment=history[-1].environment,
            createdAt=datetime.now(timezone.utc),
        )
        history.append(secret)
        self._audit.append(self._record(secretId, secret.version, SecretAuditAction.Rotated, actor))
        return secret

    def revoke(self, secretId: str, actor: str) -> None:
        """撤销：旧凭据立即失效。"""
        history = self._secrets.get(secretId)
        if not history:
            raise SecretError(f"密钥不存在: {secretId}")
        current = history[-1]
        if current.revoked:
            raise SecretError(f"密钥已撤销: {secretId}")
        history[-1] = ManagedSecretV1(
            secretId=current.secretId,
            version=current.version,
            value=current.value,
            environment=current.environment,
            createdAt=current.createdAt,
            revoked=True,
        )
        self._audit.append(self._record(secretId, current.version, SecretAuditAction.Revoked, actor))

    def requirePermission(self, secretId: str, principalId: str, permission: str) -> None:
        """最小权限：密钥访问需要显式权限（占位实现，由 RBAC 接线）。"""
        if not principalId or not permission:
            raise SecretError("主体与权限不能为空")

    def auditRecords(self) -> tuple[SecretAuditRecordV1, ...]:
        return tuple(self._audit)

    def _record(
        self, secretId: str, version: int, action: SecretAuditAction, actor: str
    ) -> SecretAuditRecordV1:
        self._counter += 1
        return SecretAuditRecordV1(
            auditId=f"sec-audit-{self._counter:06d}",
            secretId=secretId,
            version=version,
            action=action,
            actor=actor,
            at=datetime.now(timezone.utc),
        )
