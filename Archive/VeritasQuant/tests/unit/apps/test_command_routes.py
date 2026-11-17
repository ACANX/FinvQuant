"""P2-026/027 命令 API 路由测试：幂等提交、查询、取消。"""

from __future__ import annotations

from fastapi.testclient import TestClient

from veritasquant.apps.server.ApiApp import ApiDependencies, createApp
from veritasquant.apps.server.CommandRoutes import CommandApi
from veritasquant.application.ApiApp import ApiVersionProvider
from veritasquant.application.ApiErrors import ApiErrorCatalog
from veritasquant.application.CommandResource import (
    CommandFailureV1,
    CommandService,
    CommandStatus,
    CommandStore,
    CommandError,
)


class InMemoryCommandStore(CommandStore):
    def __init__(self) -> None:
        self._records: dict[str, object] = {}
        self._byScope: dict[str, str] = {}

    def create(self, record) -> object:
        if record.commandId in self._records:
            raise CommandError("命令已存在")
        if record.idempotencyScope in self._byScope:
            raise CommandError("幂等作用域重复")
        self._records[record.commandId] = record
        self._byScope[record.idempotencyScope] = record.commandId
        return record

    def get(self, commandId: str):
        return self._records.get(commandId)

    def update(self, record, expectedUpdatedTs=None):
        existing = self._records.get(record.commandId)
        if existing is None:
            raise CommandError("命令不存在")
        if expectedUpdatedTs is not None and existing.updatedTs != expectedUpdatedTs:
            raise CommandError("命令并发版本冲突")
        self._records[record.commandId] = record
        return record

    def findByIdempotencyScope(self, scope: str):
        commandId = self._byScope.get(scope)
        return self._records.get(commandId) if commandId else None


class _StubVersionProvider(ApiVersionProvider):
    @property
    def apiVersion(self) -> str:
        return "v1"

    @property
    def catalogVersion(self) -> str:
        return "9.9.9"


def _client() -> TestClient:
    catalog = ApiErrorCatalog.loadPackaged()
    store = InMemoryCommandStore()
    service = CommandService(store)
    api = CommandApi(service, catalog)
    deps = ApiDependencies(
        errorCatalog=catalog,
        versionProvider=_StubVersionProvider(),
        commandApi=api,
    )
    return TestClient(createApp(deps))


def _submitBody(**overrides) -> dict:
    body = {
        "command_id": "cmd-001",
        "command_type": "FUND_SUBSCRIBE",
        "account_id": "acc-1",
        "run_id": "run-1",
        "requested_by": "user-1",
        "idempotency_key": "idem-1",
        "payload": {"fundSymbol": "FUND-A", "amount": "100.00"},
    }
    body.update(overrides)
    return body


def test_submit_returns_202_with_command_reference() -> None:
    client = _client()
    response = client.post("/api/v1/commands", json=_submitBody())
    assert response.status_code == 202
    payload = response.json()
    assert payload["code"] == 202
    assert payload["data"]["command_id"] == "cmd-001"
    assert payload["data"]["status"] == "PENDING"


def test_same_key_same_payload_returns_original() -> None:
    client = _client()
    first = client.post("/api/v1/commands", json=_submitBody())
    assert first.status_code == 202
    second = client.post("/api/v1/commands", json=_submitBody())
    assert second.status_code == 202  # 重放仍返回已受理
    assert second.json()["data"]["command_id"] == first.json()["data"]["command_id"]


def test_same_key_different_payload_conflicts_1003() -> None:
    client = _client()
    client.post("/api/v1/commands", json=_submitBody())
    response = client.post(
        "/api/v1/commands",
        json=_submitBody(payload={"fundSymbol": "FUND-B", "amount": "999.00"}),
    )
    assert response.status_code == 409
    payload = response.json()
    assert payload["code"] == 1003
    assert payload["error"]["code"] == "IDEMPOTENCY_CONFLICT"


def test_get_command_status() -> None:
    client = _client()
    client.post("/api/v1/commands", json=_submitBody())
    response = client.get("/api/v1/commands/cmd-001")
    assert response.status_code == 200
    payload = response.json()
    assert payload["data"]["command_id"] == "cmd-001"
    assert payload["data"]["status"] == "PENDING"
    assert payload["data"]["created_ts"]


def test_get_unknown_command_404() -> None:
    client = _client()
    response = client.get("/api/v1/commands/missing")
    assert response.status_code == 404
    assert response.json()["code"] == 1002


def test_cancel_command() -> None:
    client = _client()
    client.post("/api/v1/commands", json=_submitBody())
    # 推进到 ACCEPTED 后取消（PENDING 不能直接取消 -> 走状态机）
    catalog = ApiErrorCatalog.loadPackaged()
    store = InMemoryCommandStore()
    service = CommandService(store)
    record, _ = service.submit(
        commandId="cmd-001",
        commandType="FUND_SUBSCRIBE",
        accountId="acc-1",
        runId="run-1",
        requestedBy="user-1",
        idempotencyKey="idem-1",
        route="/api/v1/commands",
        payload={"fundSymbol": "FUND-A", "amount": "100.00"},
    )
    service.transition(record.commandId, CommandStatus.Authorizing)
    service.transition(record.commandId, CommandStatus.Accepted)
    api = CommandApi(service, catalog)
    deps = ApiDependencies(
        errorCatalog=catalog,
        versionProvider=_StubVersionProvider(),
        commandApi=api,
    )
    client2 = TestClient(createApp(deps))
    response = client2.post("/api/v1/commands/cmd-001/cancel")
    assert response.status_code == 200
    payload = response.json()
    assert payload["data"]["status"] == "CANCEL_REQUESTED"


def test_invalid_cancel_returns_422() -> None:
    """PENDING 直接取消非法：状态机拒绝。"""
    client = _client()
    client.post("/api/v1/commands", json=_submitBody())
    response = client.post("/api/v1/commands/cmd-001/cancel")
    assert response.status_code == 422
    assert response.json()["code"] == 3000


def test_failed_command_includes_failure_snapshot() -> None:
    catalog = ApiErrorCatalog.loadPackaged()
    store = InMemoryCommandStore()
    service = CommandService(store)
    record, _ = service.submit(
        commandId="cmd-001",
        commandType="FUND_SUBSCRIBE",
        accountId="acc-1",
        runId="run-1",
        requestedBy="user-1",
        idempotencyKey="idem-1",
        route="/api/v1/commands",
        payload={"fundSymbol": "FUND-A", "amount": "100.00"},
    )
    service.transition(record.commandId, CommandStatus.Authorizing)
    service.transition(record.commandId, CommandStatus.Accepted)
    service.transition(record.commandId, CommandStatus.Running)
    service.transition(
        record.commandId,
        CommandStatus.Failed,
        failure=CommandFailureV1(
            code=9201,
            errorCode="INVESTMENT_PLAN_BUDGET_EXCEEDED",
            catalogVersion="1.0",
            retryable=False,
            details={"required": "100.00", "available": "50.00"},
        ),
    )
    api = CommandApi(service, catalog)
    deps = ApiDependencies(
        errorCatalog=catalog,
        versionProvider=_StubVersionProvider(),
        commandApi=api,
    )
    client = TestClient(createApp(deps))
    response = client.get("/api/v1/commands/cmd-001")
    assert response.status_code == 200
    payload = response.json()
    assert payload["data"]["status"] == "FAILED"
    failure = payload["data"]["failure"]
    assert failure["code"] == 9201
    assert failure["error_code"] == "INVESTMENT_PLAN_BUDGET_EXCEEDED"
    assert failure["retryable"] is False
    assert failure["details"] == {"required": "100.00", "available": "50.00"}
