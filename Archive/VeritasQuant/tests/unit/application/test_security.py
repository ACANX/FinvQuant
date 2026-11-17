"""P2-029 安全模型单元测试：RBAC、请求上下文、审计与限频。"""

from __future__ import annotations

import pytest

from veritasquant.application.Security import (
    AccessDeniedError,
    AuditRecord,
    InMemoryAuditSink,
    Permission,
    Principal,
    RateLimitExceededError,
    Role,
    RoutePermission,
    SecurityService,
    SimpleRequestIdGenerator,
    TokenBucketRateLimiter,
    UnauthenticatedError,
)


class StaticPrincipalProvider:
    """测试主体提供者：按凭据映射固定主体。"""

    def __init__(self, principals: dict[str, Principal]) -> None:
        self._principals = principals

    def resolve(self, credential: str | None) -> Principal:
        if not credential:
            raise UnauthenticatedError("凭据缺失")
        principal = self._principals.get(credential)
        if principal is None:
            raise UnauthenticatedError("凭据无效")
        return principal


def _service(principal: Principal | None = None) -> SecurityService:
    providers: dict[str, Principal] = {}
    if principal is not None:
        providers["token-1"] = principal
    return SecurityService(
        principalProvider=StaticPrincipalProvider(providers),
        auditSink=InMemoryAuditSink(),
        requestIdGenerator=SimpleRequestIdGenerator(),
        rateLimiter=TokenBucketRateLimiter(capacity=5, refillPerSecond=1.0),
    )


class TestRolePermissions:
    def test_viewer_can_read_account_but_not_write(self) -> None:
        viewer = Principal(principalId="u-1", roles=(Role.Viewer,), accountIds=frozenset({"acc-1"}))
        assert viewer.hasPermission(Permission.AccountRead)
        assert not viewer.hasPermission(Permission.AccountWrite)

    def test_operator_can_submit_command(self) -> None:
        operator = Principal(principalId="u-2", roles=(Role.Operator,))
        assert operator.hasPermission(Permission.CommandSubmit)
        assert not operator.hasPermission(Permission.RiskControl)

    def test_administrator_has_all_permissions(self) -> None:
        admin = Principal(principalId="u-3", roles=(Role.Administrator,))
        for permission in Permission:
            assert admin.hasPermission(permission)

    def test_auditor_only_reads(self) -> None:
        auditor = Principal(principalId="u-4", roles=(Role.Auditor,))
        assert auditor.hasPermission(Permission.AuditRead)
        assert not auditor.hasPermission(Permission.CommandSubmit)

    def test_default_deny_unknown_role(self) -> None:
        principal = Principal(principalId="u-5", roles=())
        assert not principal.hasPermission(Permission.AccountRead)


class TestAccountScope:
    def test_account_in_scope_allowed(self) -> None:
        principal = Principal(principalId="u-1", roles=(Role.Viewer,), accountIds=frozenset({"acc-1"}))
        assert principal.canAccessAccount("acc-1")
        assert not principal.canAccessAccount("acc-2")

    def test_empty_account_set_means_admin_only(self) -> None:
        admin = Principal(principalId="u-3", roles=(Role.Administrator,))
        assert admin.canAccessAccount("any-account")
        viewer = Principal(principalId="u-1", roles=(Role.Viewer,))
        assert not viewer.canAccessAccount("any-account")


class TestSecurityService:
    def test_unauthenticated_denied(self) -> None:
        service = _service()
        context = service.newRequestContext(None)
        route = RoutePermission(Permission.AccountRead, "account:read")
        with pytest.raises(UnauthenticatedError):
            service.authorize(context, route, "/api/v1/accounts/acc-1", "acc-1")

    def test_missing_permission_denied(self) -> None:
        viewer = Principal(principalId="u-1", roles=(Role.Viewer,), accountIds=frozenset({"acc-1"}))
        service = _service(viewer)
        context = service.newRequestContext("token-1")
        route = RoutePermission(Permission.AccountWrite, "account:write")
        with pytest.raises(AccessDeniedError) as excinfo:
            service.authorize(context, route, "/api/v1/accounts/acc-1", "acc-1")
        assert excinfo.value.permission is Permission.AccountWrite

    def test_account_out_of_scope_denied(self) -> None:
        viewer = Principal(principalId="u-1", roles=(Role.Viewer,), accountIds=frozenset({"acc-1"}))
        service = _service(viewer)
        context = service.newRequestContext("token-1")
        route = RoutePermission(Permission.AccountRead, "account:read")
        with pytest.raises(AccessDeniedError) as excinfo:
            service.authorize(context, route, "/api/v1/accounts/acc-2", "acc-2")
        assert "account_out_of_scope" in excinfo.value.detail

    def test_allowed_records_audit(self) -> None:
        operator = Principal(principalId="u-2", roles=(Role.Operator,), accountIds=frozenset({"acc-1"}))
        service = _service(operator)
        context = service.newRequestContext("token-1")
        route = RoutePermission(Permission.AccountWrite, "account:write")
        service.authorize(context, route, "/api/v1/accounts/acc-1", "acc-1")
        records = service._auditSink.records()  # type: ignore[attr-defined]
        assert any(r.outcome == "ALLOWED" and r.permission == "account:write" for r in records)

    def test_rate_limit_exceeded(self) -> None:
        operator = Principal(principalId="u-2", roles=(Role.Operator,), accountIds=frozenset({"acc-1"}))
        service = _service(operator)
        context = service.newRequestContext("token-1")
        route = RoutePermission(Permission.AccountWrite, "account:write", rateLimitCapacity=2, rateLimitRefillPerSecond=0.001)
        service.authorize(context, route, "/api/v1/accounts/acc-1", "acc-1")
        service.authorize(context, route, "/api/v1/accounts/acc-1", "acc-1")
        with pytest.raises(RateLimitExceededError) as excinfo:
            service.authorize(context, route, "/api/v1/accounts/acc-1", "acc-1")
        assert excinfo.value.retryAfterSeconds >= 1

    def test_anonymous_context_has_request_id(self) -> None:
        service = _service()
        context = service.newRequestContext(None)
        assert context.requestId.startswith("req_")
        assert context.principal is None


class TestTokenBucketRateLimiter:
    def test_consume_until_empty(self) -> None:
        limiter = TokenBucketRateLimiter(capacity=3, refillPerSecond=1.0)
        assert limiter.consume("s", now=100.0)
        assert limiter.consume("s", now=100.0)
        assert limiter.consume("s", now=100.0)
        assert not limiter.consume("s", now=100.0)

    def test_refill_over_time(self) -> None:
        limiter = TokenBucketRateLimiter(capacity=2, refillPerSecond=1.0)
        assert limiter.consume("s", now=0.0)
        assert limiter.consume("s", now=0.0)
        assert not limiter.consume("s", now=0.5)
        assert limiter.consume("s", now=1.0)

    def test_independent_scopes(self) -> None:
        limiter = TokenBucketRateLimiter(capacity=1, refillPerSecond=1.0)
        assert limiter.consume("a", now=0.0)
        assert not limiter.consume("a", now=0.0)
        assert limiter.consume("b", now=0.0)

    def test_retry_after(self) -> None:
        limiter = TokenBucketRateLimiter(capacity=1, refillPerSecond=0.5)
        assert limiter.consume("s", now=0.0)
        retry = limiter.retryAfterSeconds("s", now=0.0)
        assert retry >= 1


class TestAuditSink:
    def test_audit_record_wire_redacts_sensitive(self) -> None:
        record = AuditRecord(
            timestampIso="2026-08-03T00:00:00Z",
            requestId="req-1",
            traceId="trc-1",
            principalId="u-1",
            action="command:submit",
            resource="/api/v1/commands",
            outcome="DENIED",
            detail="token=secret-value",
            sensitive=True,
        )
        wire = record.toWire()
        assert wire["detail"] == "[已脱敏]"

    def test_sink_caps_records(self) -> None:
        sink = InMemoryAuditSink(maxRecords=3)
        for i in range(5):
            sink.append(
                AuditRecord(
                    timestampIso="t",
                    requestId=f"req-{i}",
                    traceId=None,
                    principalId=None,
                    action="a",
                    resource="r",
                    outcome="ALLOWED",
                )
            )
        assert sink.count() == 3
        assert sink.records()[0].requestId == "req-2"
