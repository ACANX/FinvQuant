"""P2-002 数据库单活租约与 fencing token。

每个账户组写进程必须持有有效租约才能写入；租约携带单调递增
fencing token。租约丢失后旧进程的所有新写入由持久层拒绝
（guard 校验当前租约持有者与 token），防止双写者脑裂。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta, timezone

from psycopg import Connection
from psycopg.rows import dict_row

_ACQUIRE_SQL = """
INSERT INTO partition_leases (
    account_group_id, lease_holder, fencing_token, lease_acquired_at,
    lease_expires_at, lease_ttl_seconds
) VALUES (%s, %s, 1, now(), now() + make_interval(secs => %s), %s)
ON CONFLICT (account_group_id) DO NOTHING
"""

# 仅当租约过期或持有者相同时才能更新（新 token = 旧 token + 1）
_TAKE_OVER_SQL = """
UPDATE partition_leases
SET lease_holder = %s,
    fencing_token = fencing_token + 1,
    lease_acquired_at = now(),
    lease_expires_at = now() + make_interval(secs => %s),
    lease_ttl_seconds = %s,
    renewed_at = now()
WHERE account_group_id = %s
  AND (lease_expires_at <= now() OR lease_holder = %s)
RETURNING fencing_token, lease_expires_at
"""

_RENEW_SQL = """
UPDATE partition_leases
SET lease_expires_at = now() + make_interval(secs => %s),
    renewed_at = now()
WHERE account_group_id = %s
  AND lease_holder = %s
  AND fencing_token = %s
  AND lease_expires_at > now()
"""

_RELEASE_SQL = """
DELETE FROM partition_leases
WHERE account_group_id = %s AND lease_holder = %s AND fencing_token = %s
"""

# 事务内写入门禁：校验调用方仍是当前有效租约持有者（token 必须精确匹配）
_GUARD_SQL = """
SELECT 1 FROM partition_leases
WHERE account_group_id = %s
  AND lease_holder = %s
  AND fencing_token = %s
  AND lease_expires_at > now()
"""


class LeaseError(RuntimeError):
    """租约不可用、过期或 fencing token 不匹配。"""


@dataclass(frozen=True, slots=True)
class LeaseV1:
    """一次成功获取的租约身份。"""

    accountGroupId: str
    holder: str
    fencingToken: int
    expiresAt: datetime


class LeaseStoreV1:
    """PostgreSQL 单活租约；fencing token 单调递增。"""

    def __init__(self, connection: Connection) -> None:
        self._connection = connection

    def acquire(self, accountGroupId: str, holder: str, ttlSeconds: int = 10) -> LeaseV1:
        """获取租约；已被他人持有且未过期时抛出 LeaseError。"""
        if not accountGroupId or not holder:
            raise LeaseError("账户组与持有者不能为空")
        if ttlSeconds <= 0:
            raise LeaseError("租约 TTL 必须为正")
        with self._connection.transaction():
            inserted = self._connection.execute(
                _ACQUIRE_SQL, (accountGroupId, holder, ttlSeconds, ttlSeconds)
            )
            if inserted.rowcount == 1:
                # 全新租约：token 从 1 开始
                return LeaseV1(accountGroupId, holder, 1, self._expiryFor(ttlSeconds))
            # 已存在：仅当过期或持有者相同时抢占（token 单调 +1）
            row = self._connection.execute(
                _TAKE_OVER_SQL, (holder, ttlSeconds, ttlSeconds, accountGroupId, holder)
            ).fetchone()
            if row is None:
                current = self._connection.execute(
                    "SELECT lease_holder, fencing_token, lease_expires_at "
                    "FROM partition_leases WHERE account_group_id = %s",
                    (accountGroupId,),
                ).fetchone()
                holderName = current[0] if current else "unknown"
                raise LeaseError(
                    f"账户组 {accountGroupId} 租约被 {holderName} 持有且未过期"
                )
            token, expiresAt = row
        return LeaseV1(accountGroupId, holder, token, expiresAt)

    @staticmethod
    def _expiryFor(ttlSeconds: int) -> datetime:
        return datetime.now(timezone.utc) + timedelta(seconds=ttlSeconds)

    def renew(self, accountGroupId: str, holder: str, fencingToken: int, ttlSeconds: int = 10) -> bool:
        """续租；非当前持有者、token 不匹配或已过期返回 False。"""
        with self._connection.transaction():
            cursor = self._connection.execute(
                _RENEW_SQL, (ttlSeconds, accountGroupId, holder, fencingToken)
            )
            return cursor.rowcount == 1

    def release(self, accountGroupId: str, holder: str, fencingToken: int) -> bool:
        """主动释放租约；仅当前持有者有效。"""
        with self._connection.transaction():
            cursor = self._connection.execute(
                _RELEASE_SQL, (accountGroupId, holder, fencingToken)
            )
            return cursor.rowcount == 1

    def guard(self, accountGroupId: str, holder: str, fencingToken: int) -> None:
        """写入门禁：调用方不是当前有效持有者时抛出 LeaseError。"""
        row = self._connection.execute(
            _GUARD_SQL, (accountGroupId, holder, fencingToken)
        ).fetchone()
        if row is None:
            raise LeaseError(
                f"账户组 {accountGroupId} 租约丢失或 token 过期："
                f"holder={holder} token={fencingToken}"
            )

    def currentHolder(self, accountGroupId: str) -> dict[str, object] | None:
        """返回当前租约状态（诊断用）。"""
        with self._connection.cursor(row_factory=dict_row) as cursor:
            return cursor.execute(
                "SELECT account_group_id, lease_holder, fencing_token, lease_expires_at "
                "FROM partition_leases WHERE account_group_id = %s",
                (accountGroupId,),
            ).fetchone()

    @staticmethod
    def defaultTtl() -> int:
        """技术方案 V1 默认 TTL 10 秒。"""
        return 10

    @staticmethod
    def renewIntervalSeconds() -> int:
        """技术方案 V1 默认每 3 秒续租。"""
        return 3

    @staticmethod
    def _expiresAtFromTtl(ttlSeconds: int) -> datetime:
        return datetime.now(timezone.utc) + timedelta(seconds=ttlSeconds)
