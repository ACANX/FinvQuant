"""数据库集成测试共享 helper（不定义 fixture，仅提供连接与迁移工具）。"""

from __future__ import annotations

import os
from pathlib import Path

import psycopg

_REPO_ROOT = Path(__file__).resolve().parents[3]
MIGRATIONS_DIR = _REPO_ROOT / "Migrations" / "postgresql"

DATABASE_URL = os.environ.get(
    "VQ_TEST_DATABASE_URL",
    "postgresql://veritasquant:veritasquant@localhost:5432/veritasquant",
)


def openConnection() -> psycopg.Connection:
    """打开 autocommit 连接。"""
    return psycopg.connect(DATABASE_URL, autocommit=True)


def resetSchema() -> None:
    """重建 public schema，保证迁移可从零前滚。"""
    with openConnection() as connection:
        connection.execute("DROP SCHEMA public CASCADE")
        connection.execute("CREATE SCHEMA public")


def applyMigrations() -> list[int]:
    """应用全部待应用迁移，返回已应用版本列表。"""
    from veritasquant.infrastructure.persistence.Migrator import Migrator

    with openConnection() as connection:
        migrator = Migrator(MIGRATIONS_DIR, connection)
        versions = migrator.applyPending()
    return versions
