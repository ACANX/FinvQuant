"""P2-026 PostgreSQL 命令资源存储：身份不可变 + 幂等作用域唯一。

- 首次创建冻结身份与 payload；状态迁移仅更新生命周期字段；
- 同作用域唯一索引保证幂等键并发下只有一个命令；
- 身份字段被数据库触发器冻结，防绕过应用层状态机。
"""

from __future__ import annotations

from datetime import datetime, timezone
from typing import Any, Mapping

from psycopg import Connection
from psycopg.types.json import Jsonb

from veritasquant.application.CommandResource import (
    CommandFailureV1,
    CommandRecordV1,
    CommandStatus,
    CommandStore,
)

_INSERT_SQL = """
INSERT INTO command_records (
    command_id, command_type, account_id, run_id, requested_by,
    idempotency_scope, payload_hash, payload, expected_version,
    confirmation_token_id, status, created_ts, updated_ts,
    result_reference, failure_code, failure_error_code,
    failure_catalog_version, failure_retryable, failure_details
) VALUES (
    %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s
)
"""

_UPDATE_SQL = """
UPDATE command_records
SET status = %s,
    updated_ts = %s,
    result_reference = %s,
    failure_code = %s,
    failure_error_code = %s,
    failure_catalog_version = %s,
    failure_retryable = %s,
    failure_details = %s
WHERE command_id = %s
  AND updated_ts = %s
"""

_GET_SQL = """
SELECT command_id, command_type, account_id, run_id, requested_by,
       idempotency_scope, payload_hash, payload, expected_version,
       confirmation_token_id, status, created_ts, updated_ts,
       result_reference, failure_code, failure_error_code,
       failure_catalog_version, failure_retryable, failure_details
FROM command_records WHERE command_id = %s
"""

_GET_BY_SCOPE_SQL = """
SELECT command_id, command_type, account_id, run_id, requested_by,
       idempotency_scope, payload_hash, payload, expected_version,
       confirmation_token_id, status, created_ts, updated_ts,
       result_reference, failure_code, failure_error_code,
       failure_catalog_version, failure_retryable, failure_details
FROM command_records WHERE idempotency_scope = %s
"""


class CommandStoreError(ValueError):
    """命令存储层不满足持久化契约。"""


class PostgresCommandStore(CommandStore):
    """PostgreSQL 实现的命令资源存储。"""

    def __init__(self, connection: Connection) -> None:
        self._connection = connection

    def create(self, record: CommandRecordV1) -> CommandRecordV1:
        with self._connection.transaction():
            try:
                self._connection.execute(
                    _INSERT_SQL,
                    (
                        record.commandId,
                        record.commandType,
                        record.accountId,
                        record.runId,
                        record.requestedBy,
                        record.idempotencyScope,
                        record.payloadHash,
                        Jsonb(dict(record.payload)),
                        record.expectedVersion,
                        record.confirmationTokenId,
                        record.status.value,
                        record.createdTs,
                        record.updatedTs,
                        record.resultReference,
                        _failureField(record, "code"),
                        _failureField(record, "errorCode"),
                        _failureField(record, "catalogVersion"),
                        _failureField(record, "retryable"),
                        Jsonb(_failureField(record, "details") or {}),
                    ),
                )
            except Exception as error:
                raise CommandStoreError(f"命令创建失败: {error}") from error
        return record

    def get(self, commandId: str) -> CommandRecordV1 | None:
        row = self._connection.execute(_GET_SQL, (commandId,)).fetchone()
        return _toRecord(row) if row is not None else None

    def update(
        self, record: CommandRecordV1, expectedUpdatedTs: datetime | None = None
    ) -> CommandRecordV1:
        baseline = expectedUpdatedTs or record.createdTs
        with self._connection.transaction():
            try:
                cursor = self._connection.execute(
                    _UPDATE_SQL,
                    (
                        record.status.value,
                        record.updatedTs,
                        record.resultReference,
                        _failureField(record, "code"),
                        _failureField(record, "errorCode"),
                        _failureField(record, "catalogVersion"),
                        _failureField(record, "retryable"),
                        Jsonb(_failureField(record, "details") or {}),
                        record.commandId,
                        baseline,
                    ),
                )
            except Exception as error:
                raise CommandStoreError(f"命令状态更新失败: {error}") from error
            if cursor.rowcount != 1:
                raise CommandStoreError(
                    f"命令并发版本冲突: {record.commandId} (期望 updated_ts={baseline})"
                )
        return record

    def findByIdempotencyScope(self, scope: str) -> CommandRecordV1 | None:
        row = self._connection.execute(_GET_BY_SCOPE_SQL, (scope,)).fetchone()
        return _toRecord(row) if row is not None else None


def _failureField(record: CommandRecordV1, field: str) -> Any:
    if record.failure is None:
        return None
    return getattr(record.failure, field)


def _toRecord(row: tuple[Any, ...]) -> CommandRecordV1:
    (
        commandId,
        commandType,
        accountId,
        runId,
        requestedBy,
        idempotencyScope,
        payloadHash,
        payload,
        expectedVersion,
        confirmationTokenId,
        status,
        createdTs,
        updatedTs,
        resultReference,
        failureCode,
        failureErrorCode,
        failureCatalogVersion,
        failureRetryable,
        failureDetails,
    ) = row
    failure: CommandFailureV1 | None = None
    if failureCode is not None:
        failure = CommandFailureV1(
            code=int(failureCode),
            errorCode=failureErrorCode or "",
            catalogVersion=failureCatalogVersion or "",
            retryable=bool(failureRetryable),
            details=_asMapping(failureDetails),
        )
    return CommandRecordV1(
        commandId=commandId,
        commandType=commandType,
        accountId=accountId,
        runId=runId,
        requestedBy=requestedBy,
        idempotencyScope=idempotencyScope,
        payloadHash=payloadHash,
        payload=_asMapping(payload),
        expectedVersion=expectedVersion,
        confirmationTokenId=confirmationTokenId,
        status=CommandStatus(status),
        createdTs=_asDatetime(createdTs),
        updatedTs=_asDatetime(updatedTs),
        resultReference=resultReference,
        failure=failure,
    )


def _asMapping(value: Any) -> Mapping[str, Any]:
    return dict(value) if isinstance(value, Mapping) else {}


def _asDatetime(value: Any) -> datetime:
    if isinstance(value, datetime):
        result = value
    else:
        result = datetime.now(timezone.utc)
    if result.tzinfo is None:
        result = result.replace(tzinfo=timezone.utc)
    return result
