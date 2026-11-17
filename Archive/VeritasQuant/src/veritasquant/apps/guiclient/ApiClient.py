"""P2-031 GUI API 客户端：只经 API 访问数据（TechSpec 10.1 GUI 边界）。

- 基于 httpx 的轻量客户端：处理 ResponseEnvelopeV1 信封、401/403/限频；
- 传播 request_id / trace_id；凭据通过 Authorization Bearer 注入；
- 所有返回均为信封解包后的 data（或抛 ApiClientError）。
"""

from __future__ import annotations

import json
import uuid
from dataclasses import dataclass
from typing import Any, Mapping

import httpx


class ApiClientError(Exception):
    """API 调用失败：信封错误、网络错误或限频。"""

    def __init__(self, code: int, message: str, httpStatus: int, retryable: bool) -> None:
        super().__init__(f"[{code}] {message} (HTTP {httpStatus})")
        self.code = code
        self.message = message
        self.httpStatus = httpStatus
        self.retryable = retryable


@dataclass(frozen=True, slots=True)
class EnvelopeV1:
    """解包后的响应信封（保留 code/message/data/error/request_id）。"""

    code: int
    message: str
    data: Mapping[str, Any] | None
    error: Mapping[str, Any] | None
    requestId: str | None
    traceId: str | None

    @classmethod
    def fromWire(cls, payload: Mapping[str, Any]) -> "EnvelopeV1":
        return cls(
            code=payload.get("code", -1),
            message=payload.get("message", ""),
            data=payload.get("data"),
            error=payload.get("error"),
            requestId=payload.get("request_id"),
            traceId=payload.get("trace_id"),
        )


class ApiClient:
    """VeritasQuant API 客户端（GUI 唯一数据通道）。"""

    def __init__(
        self,
        baseUrl: str,
        credential: str | None = None,
        timeoutSeconds: float = 30.0,
        transport: httpx.BaseTransport | None = None,
    ) -> None:
        self._baseUrl = baseUrl.rstrip("/")
        self._credential = credential
        self._timeout = httpx.Timeout(timeoutSeconds)
        self._client = httpx.Client(
            base_url=self._baseUrl,
            timeout=self._timeout,
            transport=transport,
            follow_redirects=False,
        )

    def close(self) -> None:
        self._client.close()

    def __enter__(self) -> "ApiClient":
        return self

    def __exit__(self, *exc: object) -> None:
        self.close()

    def setCredential(self, credential: str | None) -> None:
        self._credential = credential

    @property
    def baseUrl(self) -> str:
        return self._baseUrl

    def _headers(self) -> dict[str, str]:
        headers: dict[str, str] = {
            "X-Request-Id": f"gui_{uuid.uuid4().hex[:16]}",
            "Accept": "application/json",
        }
        if self._credential:
            headers["Authorization"] = f"Bearer {self._credential}"
        return headers

    def _request(
        self,
        method: str,
        path: str,
        *,
        params: Mapping[str, Any] | None = None,
        jsonBody: Mapping[str, Any] | None = None,
    ) -> EnvelopeV1:
        url = path if path.startswith("/") else f"/{path}"
        try:
            response = self._client.request(
                method,
                url,
                params=dict(params or {}),
                json=jsonBody,
                headers=self._headers(),
            )
        except httpx.HTTPError as error:
            raise ApiClientError(2006, f"网络错误: {error}", 0, True) from error

        try:
            payload = response.json()
        except json.JSONDecodeError:
            raise ApiClientError(2006, "响应非 JSON", response.status_code, False) from None

        envelope = EnvelopeV1.fromWire(payload)
        if envelope.code >= 1000:
            retryable = bool((envelope.error or {}).get("retryable", False))
            raise ApiClientError(envelope.code, envelope.message, response.status_code, retryable)
        return envelope

    # ---- 健康与版本 ----
    def healthLive(self) -> bool:
        try:
            response = self._client.get("/health/live", headers=self._headers())
            return response.status_code == 200
        except httpx.HTTPError:
            return False

    def version(self) -> Mapping[str, Any]:
        return self._request("GET", "/api/v1/version").data or {}

    # ---- 账户 ----
    def accounts(self, runId: str | None = None) -> list[Mapping[str, Any]]:
        params = {"run_id": runId} if runId else None
        envelope = self._request("GET", "/api/v1/accounts", params=params)
        return list(envelope.data.get("accounts", [])) if envelope.data else []

    def account(self, accountId: str, runId: str | None = None) -> Mapping[str, Any]:
        params = {"run_id": runId} if runId else None
        envelope = self._request("GET", f"/api/v1/accounts/{accountId}", params=params)
        return dict(envelope.data or {})

    def accountLedger(self, accountId: str, runId: str) -> list[Mapping[str, Any]]:
        envelope = self._request("GET", f"/api/v1/accounts/{accountId}/ledger", params={"run_id": runId})
        return list((envelope.data or {}).get("entries", []))

    def accountCashFlows(self, accountId: str, runId: str) -> list[Mapping[str, Any]]:
        envelope = self._request("GET", f"/api/v1/accounts/{accountId}/cashflows", params={"run_id": runId})
        return list((envelope.data or {}).get("cashflows", []))

    def accountShares(self, accountId: str, runId: str) -> list[Mapping[str, Any]]:
        envelope = self._request("GET", f"/api/v1/accounts/{accountId}/shares", params={"run_id": runId})
        return list((envelope.data or {}).get("shares", []))

    def accountAnalysis(self, accountId: str, runId: str) -> Mapping[str, Any]:
        envelope = self._request("GET", f"/api/v1/accounts/{accountId}/analysis", params={"run_id": runId})
        return dict(envelope.data or {})

    # ---- 策略 / 数据 / 基金 ----
    def strategies(self) -> list[Mapping[str, Any]]:
        envelope = self._request("GET", "/api/v1/strategies")
        return list(envelope.data.get("strategies", [])) if envelope.data else []

    def instruments(self) -> list[Mapping[str, Any]]:
        envelope = self._request("GET", "/api/v1/instruments")
        return list(envelope.data.get("instruments", [])) if envelope.data else []

    def funds(self) -> list[Mapping[str, Any]]:
        envelope = self._request("GET", "/api/v1/funds")
        return list(envelope.data.get("funds", [])) if envelope.data else []

    # ---- 回测 ----
    def createBacktest(self, payload: Mapping[str, Any]) -> Mapping[str, Any]:
        envelope = self._request("POST", "/api/v1/backtests", jsonBody=payload)
        return dict(envelope.data or {})

    def backtests(self) -> list[Mapping[str, Any]]:
        envelope = self._request("GET", "/api/v1/backtests")
        return list(envelope.data.get("backtests", [])) if envelope.data else []

    def backtest(self, runId: str) -> Mapping[str, Any]:
        envelope = self._request("GET", f"/api/v1/backtests/{runId}")
        return dict(envelope.data or {})

    def startBacktest(self, runId: str) -> Mapping[str, Any]:
        envelope = self._request("POST", f"/api/v1/backtests/{runId}/start")
        return dict(envelope.data or {})

    def cancelBacktest(self, runId: str) -> Mapping[str, Any]:
        envelope = self._request("POST", f"/api/v1/backtests/{runId}/cancel")
        return dict(envelope.data or {})

    # ---- 命令 ----
    def submitCommand(self, payload: Mapping[str, Any]) -> Mapping[str, Any]:
        envelope = self._request("POST", "/api/v1/commands", jsonBody=payload)
        return dict(envelope.data or {})

    def command(self, commandId: str) -> Mapping[str, Any]:
        envelope = self._request("GET", f"/api/v1/commands/{commandId}")
        return dict(envelope.data or {})

    def cancelCommand(self, commandId: str) -> Mapping[str, Any]:
        envelope = self._request("POST", f"/api/v1/commands/{commandId}/cancel")
        return dict(envelope.data or {})

    # ---- 报告 ----
    def reports(self) -> list[Mapping[str, Any]]:
        envelope = self._request("GET", "/api/v1/reports")
        return list(envelope.data.get("reports", [])) if envelope.data else []

    # ---- 行情导入（文件上传） ----
    def uploadImport(
        self,
        fileName: str,
        content: bytes,
        source: str,
        upsertMode: str = "FIELD",
        importedBy: str = "gui",
    ) -> Mapping[str, Any]:
        """上传 MVSV 行情文件并导入（POST /api/v1/imports/upload）。"""
        url = "/api/v1/imports/upload"
        try:
            response = self._client.post(
                url,
                headers=self._headers(),
                data={
                    "source": source,
                    "upsert_mode": upsertMode,
                    "imported_by": importedBy,
                },
                files={"file": (fileName, content, "application/octet-stream")},
            )
        except httpx.HTTPError as error:
            raise ApiClientError(2006, f"网络错误: {error}", 0, True) from error
        try:
            payload = response.json()
        except json.JSONDecodeError:
            raise ApiClientError(2006, "响应非 JSON", response.status_code, False) from None
        envelope = EnvelopeV1.fromWire(payload)
        if envelope.code >= 1000:
            retryable = bool((envelope.error or {}).get("retryable", False))
            raise ApiClientError(envelope.code, envelope.message, response.status_code, retryable)
        return dict(envelope.data or {})
