"""行情导入 API 路由测试：上传 MVSV 文件 → 导入统计。"""

from __future__ import annotations

from fastapi.testclient import TestClient

from veritasquant.apps.server.ApiApp import ApiDependencies, createApp
from veritasquant.apps.server.ImportRoutes import ImportApi
from veritasquant.application.ApiApp import ApiVersionProvider
from veritasquant.application.ApiErrors import ApiErrorCatalog

_MVSV_CONTENT = (
    "# Format : \"MVSV-1\"\n"
    "# Field : \"ts|dt|o|c|l|h|v|t|cp|cr|p\"\n"
    "# Count : 1\n"
    "# EffectiveTimeZone : \"Asia/Shanghai\"\n"
    "# Code : \"518880\"\n"
    "# Market : \"SSE\"\n"
    "# MarketCode : 1\n"
    "# CurrencyCode : 1\n"
    "# PriceAccuracy : 3\n"
    "# LotSize : 100\n"
    "\n"
    "1785720600|20260803093000|7.001|7.002|6.999|7.003|100000|70010000000|0.001|0.000143|7.000\n"
).encode("utf-8")


class _StubVersionProvider(ApiVersionProvider):
    @property
    def apiVersion(self) -> str:
        return "v1"

    @property
    def catalogVersion(self) -> str:
        return "9.9.9"


class StubImportService:
    """QuoteImportService 替身：记录调用并返回固定结果。"""

    def __init__(self) -> None:
        self.calls: list[dict] = []

    def importContent(
        self,
        content: bytes,
        source: str,
        mode: str = "FIELD",
        importedBy: str = "api",
        notes: str | None = None,
        reason: str | None = None,
    ) -> dict:
        self.calls.append({
            "content": content,
            "source": source,
            "mode": mode,
            "imported_by": importedBy,
            "notes": notes,
        })
        return {
            "batch_id": "import_518880_20260804120000",
            "secu_code": "518880",
            "market_code": 1,
            "record_count": 1,
            "content_sha256": "a" * 64,
            "mode": mode,
        }


def _client(importApi: ImportApi | None) -> TestClient:
    catalog = ApiErrorCatalog.loadPackaged()
    deps = ApiDependencies(
        errorCatalog=catalog,
        versionProvider=_StubVersionProvider(),
        importApi=importApi,
    )
    return TestClient(createApp(deps))


def _upload(client: TestClient, content: bytes = _MVSV_CONTENT, **data) -> object:
    return client.post(
        "/api/v1/imports/upload",
        files={"file": ("NVDA.mvsv", content, "application/octet-stream")},
        data={"source": "cn-feed", "upsert_mode": "FIELD", **data},
    )


class TestImportUploadRoute:
    def test_upload_imports_and_returns_stats(self) -> None:
        service = StubImportService()
        catalog = ApiErrorCatalog.loadPackaged()
        client = _client(ImportApi(service, catalog))

        response = _upload(client)

        assert response.status_code == 200
        body = response.json()
        assert body["code"] == 0
        assert body["data"]["secu_code"] == "518880"
        assert body["data"]["record_count"] == 1
        assert len(service.calls) == 1
        assert service.calls[0]["source"] == "cn-feed"
        assert service.calls[0]["mode"] == "FIELD"
        assert service.calls[0]["content"] == _MVSV_CONTENT

    def test_upload_row_mode_passed_through(self) -> None:
        service = StubImportService()
        catalog = ApiErrorCatalog.loadPackaged()
        client = _client(ImportApi(service, catalog))

        response = _upload(client, upsert_mode="ROW")

        assert response.status_code == 200
        assert service.calls[0]["mode"] == "ROW"

    def test_upload_empty_file_rejected(self) -> None:
        service = StubImportService()
        catalog = ApiErrorCatalog.loadPackaged()
        client = _client(ImportApi(service, catalog))

        response = _upload(client, content=b"")

        assert response.status_code == 422
        assert response.json()["code"] == 4001
        assert service.calls == []

    def test_upload_invalid_mode_rejected(self) -> None:
        service = StubImportService()
        catalog = ApiErrorCatalog.loadPackaged()
        client = _client(ImportApi(service, catalog))

        response = _upload(client, upsert_mode="SNAPSHOT")

        assert response.status_code == 422
        assert response.json()["code"] == 4001

    def test_upload_route_absent_without_import_api(self) -> None:
        client = _client(None)
        response = _upload(client)
        assert response.status_code == 404
        assert response.json()["code"] == 1002
