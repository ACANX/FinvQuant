"""P2-001 版本化数据库迁移执行器。

迁移文件位于显式指定的目录（默认仓库 `Migrations/postgresql/`），按
`V<number>__<name>.sql` 排序逐个应用。每个迁移文件在单独事务中执行：
- 前滚：成功则写入 `schema_version` 并提交；
- 失败：整个迁移回滚，应用保持 not-ready，绝不部分应用；
- 并发：使用 advisory lock 保证同一时刻只有一个迁移器写入。

迁移器不包含任何业务规则；结构变化只进入新的版本化迁移文件。
"""

from __future__ import annotations

import re
from pathlib import Path

from psycopg import Connection

_MIGRATION_NAME = re.compile(r"^V(\d+)__[A-Za-z0-9_\-]+\.sql$")

_VERSION_TABLE_SQL = """
CREATE TABLE IF NOT EXISTS schema_version (
    version          TEXT        NOT NULL,
    description      TEXT        NOT NULL,
    installed_by     TEXT        NOT NULL DEFAULT current_user,
    installed_on     TIMESTAMPTZ NOT NULL DEFAULT now(),
    success          BOOLEAN     NOT NULL DEFAULT TRUE,
    PRIMARY KEY (version)
)
"""


class MigrationError(RuntimeError):
    """迁移文件命名、读取或执行失败。"""


class Migrator:
    """按版本顺序应用 `Migrations/postgresql` 下的 SQL 迁移。"""

    _LOCK_ID = 748_201_001  # 固定 advisory lock id，避免不同实例并发迁移

    def __init__(self, migrationsDir: Path, connection: Connection) -> None:
        if not migrationsDir.is_dir():
            raise MigrationError(f"迁移目录不存在: {migrationsDir}")
        self._migrationsDir = migrationsDir
        self._connection = connection

    def appliedVersions(self) -> list[int]:
        """返回已成功应用的迁移版本（升序）。"""
        self._ensureVersionTable()
        rows = self._connection.execute(
            "SELECT version FROM schema_version WHERE success = TRUE ORDER BY version"
        ).fetchall()
        versions: list[int] = []
        for (version,) in rows:
            try:
                versions.append(int(version))
            except ValueError as error:
                raise MigrationError(f"schema_version 中存在非整数版本: {version}") from error
        return versions

    def pendingVersions(self) -> list[int]:
        """返回尚未应用的迁移版本（升序）。"""
        applied = set(self.appliedVersions())
        return [version for version in self._discoverVersions() if version not in applied]

    def applyPending(self) -> list[int]:
        """应用全部待应用迁移；任一失败立即回滚并抛出，已应用版本保持不变。"""
        with self._connection.transaction():
            self._connection.execute("SELECT pg_advisory_lock(%s)", (self._LOCK_ID,))
        try:
            applied: list[int] = []
            for version in self.pendingVersions():
                self._applyOne(version)
                applied.append(version)
            return applied
        finally:
            with self._connection.transaction():
                self._connection.execute("SELECT pg_advisory_unlock(%s)", (self._LOCK_ID,))

    def _ensureVersionTable(self) -> None:
        """确保版本表存在（首版迁移本身包含该表；此处幂等创建避免先有鸡先有蛋）。"""
        self._connection.execute(_VERSION_TABLE_SQL)

    def _applyOne(self, version: int) -> None:
        path = self._pathFor(version)
        script = path.read_text(encoding="utf-8")
        description = path.stem
        # 每个迁移在独立事务中执行：失败回滚，绝不部分应用
        with self._connection.transaction():
            self._connection.execute(script)
            self._connection.execute(
                "INSERT INTO schema_version (version, description) VALUES (%s, %s)",
                (str(version), description),
            )

    def _discoverVersions(self) -> list[int]:
        versions: list[int] = []
        for path in self._migrationsDir.glob("V*__*.sql"):
            match = _MIGRATION_NAME.fullmatch(path.name)
            if match is None:
                raise MigrationError(f"迁移文件名不符合 V<N>__<name>.sql: {path.name}")
            versions.append(int(match.group(1)))
        if len(versions) != len(set(versions)):
            raise MigrationError("迁移版本号重复")
        return sorted(versions)

    def _pathFor(self, version: int) -> Path:
        for path in self._migrationsDir.glob(f"V{version}__*.sql"):
            return path
        raise MigrationError(f"找不到迁移 V{version}")
