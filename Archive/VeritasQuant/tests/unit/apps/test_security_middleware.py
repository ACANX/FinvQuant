"""P2-029 安全中间件集成测试：鉴权、RBAC、request/trace ID 与限频。"""

from __future__ import annotations

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
from fastapi.testclient import TestClient

from veritasquant.application.Security import (
    InMemoryAuditSink,
    Permission,
    Principal,
    Role,
    SecurityService,
    SimpleRequestIdGenerator,
    TokenBucketRateLimiter,
    UnauthenticatedError,
)
from veritasquant.apps.server.SecurityMiddleware import SecurityMiddleware, _matchRule, _bearerToken


class StaticPrincipalProvider:
    def __init__(self, principals: dict[str, Principal]) -> None:
        self._principals = principals

    def resolve(self, credential: str | None) -> Principal:
        if not credential:
            raise UnauthenticatedError("凭据缺失")
        principal = self._principals.get(credential)
        if principal is None:
            raise UnauthenticatedError("凭据无效")
        return principal


def _testCatalog():  # noqa: ANN201
    """最小错误目录：包含 2001/2002/2004 映射。"""
    from veritasquant.application.ApiErrors import ApiErrorCatalog

    return ApiErrorCatalog.loadPackaged()


def _buildApp() -> tuple[FastAPI, SecurityService, InMemoryAuditSink]:
    app = FastAPI()
    sink = InMemoryAuditSink()

    operator = Principal(
        principalId="u-2", roles=(Role.Operator,), accountIds=frozenset({"acc-1"})
    )
    viewer = Principal(
        principalId="u-1", roles=(Role.Viewer,), accountIds=frozenset({"acc-1"})
    )
    securityService = SecurityService(
        principalProvider=StaticPrincipalProvider({"op-token": operator, "view-token": viewer}),
        auditSink=sink,
        requestIdGenerator=SimpleRequestIdGenerator(),
        rateLimiter=TokenBucketRateLimiter(capacity=3, refillPerSecond=0.01),
    )

    @app.get("/api/v1/accounts/{account_id}")
    async def account(account_id: str) -> JSONResponse:  # noqa: ANN001
        return JSONResponse(status_code=200, content={"code": 0, "message": "ok", "data": {"account_id": account_id}})

    @app.post("/api/v1/commands")
    async def commands() -> JSONResponse:
        return JSONResponse(status_code=202, content={"code": 202, "message": "受理"})

    @app.get("/health/live")
    async def live() -> JSONResponse:
        return JSONResponse(status_code=200, content={"status": "ALIVE"})

    # 中间件 LIFO：后 add 的先执行。SecurityMiddleware 在内层，
    # 异常处理器在最外层才能捕获其抛出的安全异常。
    app.add_middleware(SecurityMiddleware, securityService=securityService, errorCatalog=_testCatalog())  # type: ignore[arg-type]

    @app.middleware("http")
    async def securityExceptionHandler(request, call_next):  # noqa: ANN001, ANN202
        try:
            return await call_next(request)
        except UnauthenticatedError:
            return JSONResponse(status_code=401, content={"code": 2001, "message": "未鉴权"})
        except Exception as error:  # noqa: BLE001
            if error.__class__.__name__ == "AccessDeniedError":
                return JSONResponse(status_code=403, content={"code": 2002, "message": "越权"})
            if error.__class__.__name__ == "RateLimitExceededError":
                response = JSONResponse(status_code=429, content={"code": 2004, "message": "限频"})
                response.headers["Retry-After"] = str(error.retryAfterSeconds)  # type: ignore[attr-defined]
                return response
            raise

    return app, securityService, sink


class TestSecurityMiddleware:
    def test_health_public(self) -> None:
        app, _, _ = _buildApp()
        client = TestClient(app)
        response = client.get("/health/live")
        assert response.status_code == 200

    def test_authenticated_account_in_scope(self) -> None:
        app, _, _ = _buildApp()
        client = TestClient(app)
        response = client.get("/api/v1/accounts/acc-1", headers={"Authorization": "Bearer op-token"})
        assert response.status_code == 200
        assert response.json()["data"]["account_id"] == "acc-1"

    def test_unauthenticated_rejected(self) -> None:
        app, _, _ = _buildApp()
        client = TestClient(app)
        response = client.get("/api/v1/accounts/acc-1")
        assert response.status_code == 401
        assert response.json()["code"] == 2001

    def test_forbidden_account_out_of_scope(self) -> None:
        app, _, _ = _buildApp()
        client = TestClient(app)
        response = client.get("/api/v1/accounts/acc-2", headers={"Authorization": "Bearer op-token"})
        assert response.status_code == 403
        assert response.json()["code"] == 2002

    def test_viewer_cannot_submit_command(self) -> None:
        app, _, _ = _buildApp()
        client = TestClient(app)
        response = client.post("/api/v1/commands", headers={"Authorization": "Bearer view-token"})
        assert response.status_code == 403
        assert response.json()["code"] == 2002

    def test_operator_can_submit_command(self) -> None:
        app, _, _ = _buildApp()
        client = TestClient(app)
        response = client.post("/api/v1/commands", headers={"Authorization": "Bearer op-token"})
        assert response.status_code == 202

    def test_rate_limit_returns_429(self) -> None:
        app, _, _ = _buildApp()
        client = TestClient(app)
        headers = {"Authorization": "Bearer op-token"}
        # 命令路由 capacity=20，refill 0.01/s 极慢 -> 第 21 次限频
        for _ in range(20):
            response = client.post("/api/v1/commands", headers=headers)
            assert response.status_code == 202
        response = client.post("/api/v1/commands", headers=headers)
        assert response.status_code == 429
        assert response.json()["code"] == 2004
        assert response.headers.get("Retry-After")

    def test_request_id_generated_when_missing(self) -> None:
        app, _, _ = _buildApp()
        client = TestClient(app)
        response = client.get("/api/v1/accounts/acc-1", headers={"Authorization": "Bearer op-token"})
        assert response.headers.get("X-Request-Id")

    def test_request_id_preserved_when_provided(self) -> None:
        app, _, _ = _buildApp()
        client = TestClient(app)
        response = client.get(
            "/api/v1/accounts/acc-1",
            headers={"Authorization": "Bearer op-token", "X-Request-Id": "my-req-123"},
        )
        assert response.headers.get("X-Request-Id") == "my-req-123"

    def test_trace_id_propagated(self) -> None:
        app, _, _ = _buildApp()
        client = TestClient(app)
        response = client.get(
            "/api/v1/accounts/acc-1",
            headers={"Authorization": "Bearer op-token", "X-Trace-Id": "my-trc-1"},
        )
        assert response.headers.get("X-Trace-Id") == "my-trc-1"


class TestRuleMatching:
    def test_rule_match_account_read(self) -> None:
        rule = _matchRule("GET", "/api/v1/accounts/acc-1")
        assert rule is not None
        assert rule.route.permission is Permission.AccountRead

    def test_rule_match_command_submit(self) -> None:
        rule = _matchRule("POST", "/api/v1/commands")
        assert rule is not None
        assert rule.route.permission is Permission.CommandSubmit

    def test_no_match_unknown_path(self) -> None:
        assert _matchRule("GET", "/api/v1/unknown") is None

    def test_bearer_token_parsing(self) -> None:
        app = FastAPI()

        @app.get("/x")
        async def x(request: Request) -> dict[str, str | None]:  # noqa: ANN001
            return {"token": _bearerToken(request)}

        client = TestClient(app)
        response = client.get("/x", headers={"Authorization": "Bearer abc-123"})
        assert response.json()["token"] == "abc-123"
        response = client.get("/x", headers={"Authorization": "Basic abc"})
        assert response.json()["token"] is None
