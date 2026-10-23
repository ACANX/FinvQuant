"""行情导入应用服务（MVSV 上传内容 → PostgreSQL，字段级覆盖式更新）。

应用层编排：解析（领域）→ 批量 upsert + 批次/修正审计（基础设施存储）。
供 API 路由（POST /api/v1/imports/upload）与 CLI 复用。
"""

from __future__ import annotations

from datetime import datetime, timezone
from pathlib import Path

from veritasquant.data.MvsvImport import MvsvImportError, MvsvImportResult, parseMvsvContent, parseMvsvPath
from veritasquant.data.QuoteRow import QuoteRowV1, UpsertMode
from veritasquant.infrastructure.persistence.QuoteStore import MinuteQuoteStore

_UPSERT_BATCH_ROWS = 5_000  # 单次 executemany 的行数上限


def _batchId(secuCode: str) -> str:
    stamp = datetime.now(timezone.utc).strftime("%Y%m%d%H%M%S")
    return f"import_{secuCode}_{stamp}"


class QuoteImportService:
    """将 MVSV-1 行情内容导入 PostgreSQL（字段级覆盖式更新）。"""

    def __init__(self, store: MinuteQuoteStore) -> None:
        self._store = store

    def importContent(
        self,
        content: bytes,
        source: str,
        mode: str = UpsertMode.Field,
        importedBy: str = "api",
        notes: str | None = None,
        reason: str | None = None,
    ) -> dict:
        """导入上传的 MVSV-1 字节流，返回导入统计。"""
        result = parseMvsvContent(content, sourceName=source or "upload")
        return self._persist(result, source=source, mode=mode, importedBy=importedBy, notes=notes, reason=reason)

    def importPath(
        self,
        path: Path,
        source: str,
        mode: str = UpsertMode.Field,
        importedBy: str = "cli",
        notes: str | None = None,
        reason: str | None = None,
    ) -> dict:
        """导入服务端路径下的 MVSV-1 文件（CLI 场景）。"""
        result = parseMvsvPath(path)
        return self._persist(result, source=source, mode=mode, importedBy=importedBy, notes=notes, reason=reason)

    def _persist(
        self,
        result: MvsvImportResult,
        *,
        source: str,
        mode: str,
        importedBy: str,
        notes: str | None,
        reason: str | None,
    ) -> dict:
        if not result.rows:
            raise MvsvImportError("MVSV 文件数据区为空")
        normalized = UpsertMode.Row if mode == UpsertMode.Row else UpsertMode.Field
        batchId = _batchId(result.secuCode)

        totalUpdated = 0
        for offset in range(0, len(result.rows), _UPSERT_BATCH_ROWS):
            chunk: list[QuoteRowV1] = result.rows[offset : offset + _UPSERT_BATCH_ROWS]
            stats = self._store.upsertRows(
                chunk,
                ingestBatchId=batchId,
                mode=normalized,
                reason=reason or "MVSV-1 导入（同键覆盖）",
                revisedBy=importedBy,
            )
            totalUpdated += stats["updated"]

        self._store.registerBatch(
            ingestBatchId=batchId,
            source=source,
            marketCode=result.marketCode,
            secuCode=result.secuCode,
            dataVersionId=result.contentSha256,
            fileCount=1,
            recordCount=result.recordCount,
            mode=normalized,
            tsPrecision="Second",
            configHash=result.contentSha256[:64],
            importedBy=importedBy,
            notes=notes,
        )
        return {
            "batch_id": batchId,
            "secu_code": result.secuCode,
            "market_code": result.marketCode,
            "record_count": result.recordCount,
            "content_sha256": result.contentSha256,
            "mode": normalized,
        }
