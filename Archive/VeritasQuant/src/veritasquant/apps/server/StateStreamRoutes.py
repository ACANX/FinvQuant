"""P2-030 鉴权 SSE 状态流路由：握手鉴权、replay、积压与权限断开。

- GET /api/v1/stream/events?cursor=&account_id= ：SSE 文本流；
- 握手鉴权（Bearer）+ 账户过滤；权限撤销/令牌过期/积压超限主动断开；
- 协议版本通过 `veritasquant-sse-protocol: v1` 响应头声明；
- SSE 不套用 JSON 信封（TechSpec 10.2），事件行格式：
  `event: <type>\\ndata: <json>\\nid: <sequence>\\n\\n`。
"""

from __future__ import annotations

import asyncio
import json
from dataclasses import dataclass
from typing import Any

from fastapi import APIRouter
from fastapi.responses import JSONResponse, StreamingResponse
from starlette.requests import Request
from starlette.responses import Response

from veritasquant.application.Security import (
    Principal,
    PrincipalProvider,
    UnauthenticatedError,
)
from veritasquant.application.StateStream import (
    SSE_PROTOCOL_VERSION,
    StreamCloseReason,
    StreamEventV1,
    StreamService,
)

_STREAM_PREFIX = "/api/v1/stream"


def _bearerToken(request: Request) -> str | None:
    authorization = request.headers.get("authorization")
    if not authorization:
        return None
    scheme, _, credential = authorization.partition(" ")
    if scheme.lower() != "bearer" or not credential:
        return None
    return credential.strip()


def _encodeEvent(event: StreamEventV1) -> str:
    data = {
        "type": event.eventType.value,
        "account_id": event.accountId,
        "occurred_at": event.occurredAtIso,
        "payload": event.payload,
    }
    return (
        f"event: {event.eventType.value}\n"
        f"data: {json.dumps(data, ensure_ascii=False, separators=(',', ':'))}\n"
        f"id: {event.sequence}\n\n"
    )


def _encodeClose(reason: StreamCloseReason, message: str) -> str:
    data = {"close_reason": reason.value, "message": message}
    return f"event: stream.close\ndata: {json.dumps(data, ensure_ascii=False)}\n\n"


@dataclass(frozen=True, slots=True)
class StreamDependencies:
    """SSE 路由依赖：主体提供者 + 流服务。"""

    principalProvider: PrincipalProvider
    streamService: StreamService


def buildStreamRouter(deps: StreamDependencies) -> APIRouter:
    """组装 SSE 路由；依赖注入，测试可替换替身。"""
    router = APIRouter()

    @router.get(f"{_STREAM_PREFIX}/events")
    async def streamEvents(
        request: Request,
        cursor: str | None = None,
        account_id: str | None = None,
    ) -> Response:
        # 握手鉴权：凭据无效 -> 401（SSE 协议用 HTTP 状态表达）
        credential = _bearerToken(request)
        try:
            principal: Principal = deps.principalProvider.resolve(credential)
        except UnauthenticatedError:
            return JSONResponse(
                status_code=401,
                content={"code": 2001, "message": "security.unauthenticated"},
            )

        # 账户过滤：显式 account_id 必须在主体可访问范围内
        if account_id is not None:
            if not principal.canAccessAccount(account_id):
                return JSONResponse(
                    status_code=403,
                    content={"code": 2002, "message": "security.forbidden"},
                )
            accountIds = frozenset({account_id})
        else:
            accountIds = principal.accountIds

        result = deps.streamService.open(
            principal.principalId, accountIds, cursor
        )

        # 有界 replay：cursor 超出窗口 -> 明确关闭（验收标准 3）
        if result.closeReason is not None:
            closeReason = result.closeReason

            async def closedStream() -> Any:
                yield _encodeClose(closeReason, result.message)

            return StreamingResponse(
                closedStream(),
                media_type="text/event-stream",
                headers={
                    "Cache-Control": "no-cache",
                    "X-Accel-Buffering": "no",
                    "VeritasQuant-SSE-Protocol": SSE_PROTOCOL_VERSION,
                },
            )

        # 订阅已建立：先回放已保留事件，再进入轮询循环
        subscriptionId = result.subscriptionId

        async def eventGenerator() -> Any:
            # 回放 open 时返回的事件
            for event in result.events:
                yield _encodeEvent(event)
            # 轮询待投递事件；订阅关闭时输出 close 事件后结束
            while True:
                events = deps.streamService.takePending(subscriptionId)
                if events is None:
                    closeReason = deps.streamService.subscriptionCloseReason(subscriptionId)
                    reason: StreamCloseReason = closeReason or StreamCloseReason.Shutdown
                    yield _encodeClose(reason, "订阅已关闭")
                    return
                for event in events:
                    yield _encodeEvent(event)
                await asyncio.sleep(0.2)

        return StreamingResponse(
            eventGenerator(),
            media_type="text/event-stream",
            headers={
                "Cache-Control": "no-cache",
                "X-Accel-Buffering": "no",
                "VeritasQuant-SSE-Protocol": SSE_PROTOCOL_VERSION,
                "X-Stream-Subscription-Id": subscriptionId,
            },
        )

    return router
