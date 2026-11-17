"""行情导入 API 路由：用户上传 MVSV 文件 → PostgreSQL。

- POST /api/v1/imports/upload：multipart 上传 MVSV-1 行情文件，
  服务端解析并字段级覆盖导入 `finv_quote_secu_kline_min`。
依赖通过 buildImportRouter(api) 注入，测试可替换替身。
"""

from __future__ import annotations

import logging

from fastapi import APIRouter, File, Form, UploadFile
from fastapi.responses import JSONResponse

from veritasquant.application.ApiErrors import ApiErrorCatalog
from veritasquant.application.QuoteImportService import QuoteImportService
from veritasquant.application.ResponseEnvelope import ResponseEnvelopeV1
from veritasquant.data.MvsvImport import MvsvImportError
from veritasquant.data.QuoteRow import UpsertMode

logger = logging.getLogger("veritasquant.apps.server.import_routes")

_IMPORTS_PREFIX = "/api/v1/imports"

# 上传文件大小上限（50 MiB）；超限返回 1001 VALIDATION_ERROR
_MAX_UPLOAD_BYTES = 50 * 1024 * 1024


class ImportApi:
    """导入路由依赖：封装 QuoteImportService 与统一错误映射。"""

    def __init__(self, service: QuoteImportService, catalog: ApiErrorCatalog) -> None:
        self._service = service
        self._catalog = catalog

    def upload(
        self,
        *,
        content: bytes,
        fileName: str,
        source: str,
        mode: str,
        importedBy: str,
        notes: str | None = None,
    ) -> tuple[ResponseEnvelopeV1, int]:
        """解析并导入上传内容；失败返回 DATA_IMPORT_CONTRACT_INVALID(4001)。"""
        if not content:
            return self._error(4001, "上传文件为空"), 422
        if len(content) > _MAX_UPLOAD_BYTES:
            limitMiB = _MAX_UPLOAD_BYTES // (1024 * 1024)
            return self._error(4001, f"文件超过大小上限 {limitMiB} MiB"), 422
        try:
            if not source.strip():
                raise MvsvImportError("数据源不能为空")
            if mode not in (UpsertMode.Field, UpsertMode.Row):
                raise MvsvImportError("upsert_mode 必须为 FIELD 或 ROW")
            result = self._service.importContent(
                content=content,
                source=source.strip(),
                mode=mode,
                importedBy=importedBy or "gui",
                notes=notes or f"上传文件: {fileName}",
            )
        except MvsvImportError as error:
            logger.warning("行情导入失败: %s", error)
            return self._error(4001, str(error)), 422
        envelope = ResponseEnvelopeV1.success(
            code=0,
            message="行情导入完成",
            data=result,
        )
        return envelope, 200

    def _error(self, code: int, message: str) -> ResponseEnvelopeV1:
        definition = self._catalog.getError(code)
        return ResponseEnvelopeV1.model_validate(
            {
                "code": definition.code,
                "message": message,
                "error": {
                    "code": definition.errorCode,
                    "catalog_version": self._catalog.catalogVersion,
                    "retryable": definition.retryable,
                },
            }
        )


def buildImportRouter(api: ImportApi | None) -> APIRouter:
    """构建导入路由；api 为 None 时不挂载业务端点（测试/未接线状态）。"""
    router = APIRouter(prefix=_IMPORTS_PREFIX)
    if api is None:
        return router

    @router.post("/upload")
    async def uploadQuoteFile(
        file: UploadFile = File(...),
        source: str = Form(...),
        upsert_mode: str = Form(UpsertMode.Field),
        imported_by: str = Form("gui"),
    ) -> JSONResponse:
        content = await file.read()
        envelope, status = api.upload(
            content=content,
            fileName=file.filename or "upload.mvsv",
            source=source,
            mode=upsert_mode,
            importedBy=imported_by,
        )
        return JSONResponse(status_code=status, content=envelope.toWire())

    return router
