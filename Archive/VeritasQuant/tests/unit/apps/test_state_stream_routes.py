"""P2-030 SSE 路由测试：鉴权握手、replay、账户过滤与断开语义。"""

from __future__ import annotations


from fastapi import FastAPI
from fastapi.testclient import TestClient

from veritasquant.application.Security import Principal, Role, UnauthenticatedError
from veritasquant.application.StateStream import (
    InMemoryStreamEventSource,
    StreamCloseReason,
    StreamEventType,
    StreamEventV1,
    StreamService,
)
from veritasquant.apps.server.StateStreamRoutes import (
    StreamDependencies,
    buildStreamRouter,
    _encodeClose,
    _encodeEvent,
)


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


def _buildApp() -> tuple[FastAPI, StreamService, StaticPrincipalProvider]:
    app = FastAPI()
    source = InMemoryStreamEventSource()
    for i in range(1, 4):
        source.append(
            StreamEventV1(
                sequence=i,
                eventType=StreamEventType.CommandStatus,
                accountId="acc-1",
                payload={"command_id": f"cmd-{i}"},
                occurredAtIso="2026-08-03T00:00:00Z",
            )
        )
    service = StreamService(source, maxBacklog=10, replayWindow=10)
    provider = StaticPrincipalProvider(
        {
            "op-token": Principal(
                principalId="u-op",
                roles=(Role.Operator,),
                accountIds=frozenset({"acc-1"}),
            ),
            "admin-token": Principal(principalId="u-admin", roles=(Role.Administrator,)),
        }
    )
    app.include_router(buildStreamRouter(StreamDependencies(provider, service)))
    return app, service, provider


class TestSseRouteAuth:
    def test_unauthenticated_returns_401(self) -> None:
        app, _, _ = _buildApp()
        client = TestClient(app)
        response = client.get("/api/v1/stream/events")
        assert response.status_code == 401
        assert response.json()["code"] == 2001

    def test_invalid_credential_returns_401(self) -> None:
        app, _, _ = _buildApp()
        client = TestClient(app)
        response = client.get(
            "/api/v1/stream/events", headers={"Authorization": "Bearer wrong"}
        )
        assert response.status_code == 401

    def test_account_out_of_scope_returns_403(self) -> None:
        app, _, _ = _buildApp()
        client = TestClient(app)
        response = client.get(
            "/api/v1/stream/events?account_id=acc-2",
            headers={"Authorization": "Bearer op-token"},
        )
        assert response.status_code == 403
        assert response.json()["code"] == 2002

    def test_cursor_beyond_window_closes(self) -> None:
        """积压超限：cursor 超出窗口时路由立即返回关闭事件（有限流）。"""
        app, service, _ = _buildApp()
        for i in range(4, 16):
            service._source.append(  # noqa: SLF001
                StreamEventV1(
                    sequence=i,
                    eventType=StreamEventType.CommandStatus,
                    accountId="acc-1",
                    payload={"command_id": f"cmd-{i}"},
                    occurredAtIso="2026-08-03T00:00:00Z",
                )
            )
        client = TestClient(app)
        # 关闭流是有限生成器，TestClient 可正常读取
        response = client.get(
            "/api/v1/stream/events?cursor=2",
            headers={"Authorization": "Bearer op-token"},
        )
        assert response.status_code == 200
        assert "backlog_exceeded" in response.text
        assert "超出" in response.text


class TestSseEncode:
    def test_encode_event_format(self) -> None:
        event = StreamEventV1(
            sequence=7,
            eventType=StreamEventType.CommandStatus,
            accountId="acc-1",
            payload={"command_id": "cmd-7"},
            occurredAtIso="2026-08-03T00:00:00Z",
        )
        text = _encodeEvent(event)
        assert "event: command.status" in text
        assert "id: 7" in text
        assert "cmd-7" in text
        assert text.endswith("\n\n")

    def test_encode_close_format(self) -> None:
        text = _encodeClose(StreamCloseReason.PermissionRevoked, "权限已撤销")
        assert "event: stream.close" in text
        assert "permission_revoked" in text
