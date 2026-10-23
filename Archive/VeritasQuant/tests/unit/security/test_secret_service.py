"""P5-003 密钥服务、轮换、撤销和最小权限测试。"""

from __future__ import annotations

import pytest

from veritasquant.security.SecretService import (
    ManagedSecretV1,
    SecretAuditAction,
    SecretError,
    SecretServiceV1,
)


class TestManagedSecret:
    def test_repr_masks_value(self) -> None:
        """仓库/日志无秘密：repr 打码。"""
        from datetime import datetime, timezone

        secret = ManagedSecretV1(
            secretId="sec-001",
            version=1,
            value="super-secret-value",
            environment="LIVE",
            createdAt=datetime.now(timezone.utc),
        )
        assert "super-secret-value" not in repr(secret)
        assert "***" in repr(secret)

    def test_requires_fields(self) -> None:
        from datetime import datetime, timezone

        with pytest.raises(SecretError):
            ManagedSecretV1(
                secretId="",
                version=1,
                value="v",
                environment="LIVE",
                createdAt=datetime.now(timezone.utc),
            )
        with pytest.raises(SecretError):
            ManagedSecretV1(
                secretId="sec-001",
                version=0,
                value="v",
                environment="LIVE",
                createdAt=datetime.now(timezone.utc),
            )


class TestSecretService:
    def test_create_and_resolve(self) -> None:
        service = SecretServiceV1()
        secret = service.create(secretId="sec-001", value="v1", environment="LIVE", actor="ops")
        assert secret.version == 1
        resolved = service.resolve("sec-001", "app")
        assert resolved.value == "v1"

    def test_duplicate_create_rejected(self) -> None:
        service = SecretServiceV1()
        service.create(secretId="sec-001", value="v1", environment="LIVE", actor="ops")
        with pytest.raises(SecretError, match="已存在"):
            service.create(secretId="sec-001", value="v2", environment="LIVE", actor="ops")

    def test_rotate_keeps_version_history(self) -> None:
        """轮换不中断审计：旧版本保留可追溯。"""
        service = SecretServiceV1()
        service.create(secretId="sec-001", value="v1", environment="LIVE", actor="ops")
        rotated = service.rotate(secretId="sec-001", newValue="v2", actor="ops")
        assert rotated.version == 2
        # 当前解析到新版本
        assert service.resolve("sec-001", "app").value == "v2"
        # 审计含创建 + 轮换 + 访问
        actions = [r.action for r in service.auditRecords()]
        assert SecretAuditAction.Created in actions
        assert SecretAuditAction.Rotated in actions
        assert SecretAuditAction.Accessed in actions

    def test_rotate_same_value_rejected(self) -> None:
        service = SecretServiceV1()
        service.create(secretId="sec-001", value="v1", environment="LIVE", actor="ops")
        with pytest.raises(SecretError, match="不得与当前密钥相同"):
            service.rotate(secretId="sec-001", newValue="v1", actor="ops")

    def test_revoke_immediately_invalidates(self) -> None:
        """撤销后旧凭据立即失效。"""
        service = SecretServiceV1()
        service.create(secretId="sec-001", value="v1", environment="LIVE", actor="ops")
        service.revoke("sec-001", "ops")
        with pytest.raises(SecretError, match="已撤销"):
            service.resolve("sec-001", "app")
        with pytest.raises(SecretError, match="不可轮换"):
            service.rotate(secretId="sec-001", newValue="v2", actor="ops")

    def test_resolve_unknown_rejected(self) -> None:
        service = SecretServiceV1()
        with pytest.raises(SecretError, match="不存在"):
            service.resolve("sec-unknown", "app")

    def test_audit_traceable_after_rotation(self) -> None:
        """轮换后审计仍可追溯（版本历史）。"""
        service = SecretServiceV1()
        service.create(secretId="sec-001", value="v1", environment="LIVE", actor="ops")
        service.rotate(secretId="sec-001", newValue="v2", actor="ops")
        records = service.auditRecords()
        versions = {r.version for r in records}
        assert {1, 2}.issubset(versions)

    def test_minimal_permission_gate(self) -> None:
        service = SecretServiceV1()
        service.create(secretId="sec-001", value="v1", environment="LIVE", actor="ops")
        service.requirePermission("sec-001", "principal-001", "secret:read")  # 不抛
        with pytest.raises(SecretError):
            service.requirePermission("sec-001", "", "secret:read")
