"""P2-001 迁移真实数据库集成测试。

需要可连接的 PostgreSQL 测试实例（CI 使用 postgres service）。
无 `VQ_TEST_DATABASE_URL` 或连接失败时自动跳过，不阻断本地无数据库环境。

验证：
- 前滚：V1 应用后 schema_version 与全部必填表存在；
- 幂等：重复执行不产生变化；
- 失败回滚：坏迁移整体回滚，不残留对象或版本记录；
- 约束：事实表不可变、唯一键、NUMERIC 精度与账户序列唯一。
"""

from __future__ import annotations

from pathlib import Path

import psycopg
import pytest

from test_db_helpers import MIGRATIONS_DIR, applyMigrations, openConnection, resetSchema

_REPO_ROOT = Path(__file__).resolve().parents[3]


@pytest.fixture(scope="module")
def database() -> bool:
    """验证连接可用并重置 schema；不可用时跳过整个模块。"""
    try:
        openConnection().close()
    except psycopg.OperationalError:
        pytest.skip("PostgreSQL 测试实例不可用，跳过迁移集成测试")
    resetSchema()
    return True


@pytest.fixture(scope="module")
def applied(database) -> object:  # noqa: ANN001
    versions = applyMigrations()
    assert versions, "首版迁移应至少应用一个版本"
    return versions


def test_migration_applies_forward(applied) -> None:  # noqa: ANN001
    # 当前迁移序列：V1 事实/投影、V2 命令资源、V3 数据字典、V4 历史行情
    assert applied == [1, 2, 3, 4]


def test_migration_idempotent(applied) -> None:  # noqa: ANN001
    assert applyMigrations() == []


def test_required_tables_created(applied) -> None:  # noqa: ANN001
    with openConnection() as connection:
        rows = connection.execute(
            "SELECT table_name FROM information_schema.tables "
            "WHERE table_schema = 'public' ORDER BY table_name"
        ).fetchall()
    tables = {row[0] for row in rows}
    required = {
        "fact_events", "inbox_records", "inbox_conflicts", "outbox_records",
        "partition_leases", "partition_checkpoints", "run_manifests",
        "ledger_journals", "ledger_entries", "order_intents", "order_events",
        "cancel_order_requests", "replace_order_requests", "execution_reports",
        "risk_decisions", "trading_controls", "account_snapshots",
        "ledger_balance_projection", "account_position_projection",
        "activity_control_projection", "schema_version",
    }
    assert required <= tables, f"缺失表: {required - tables}"


def test_failed_migration_rolls_back(applied, tmp_path: Path) -> None:  # noqa: ANN001
    """坏迁移必须整体回滚：不残留对象、不记录版本。"""
    from veritasquant.infrastructure.persistence.Migrator import Migrator

    badDir = tmp_path / "bad_migrations"
    badDir.mkdir()
    (badDir / "V1__initial.sql").write_text(
        (MIGRATIONS_DIR / "V1__initial_fact_and_projection_schema.sql").read_text(encoding="utf-8"),
        encoding="utf-8",
    )
    (badDir / "V5__broken.sql").write_text(
        "BEGIN;\nCREATE TABLE partial_table (id TEXT PRIMARY KEY);\n"
        "INSERT INTO missing_table VALUES (1);\nCOMMIT;",
        encoding="utf-8",
    )
    with openConnection() as connection:
        migrator = Migrator(badDir, connection)
        with pytest.raises(psycopg.errors.UndefinedTable):
            migrator.applyPending()
        rows = connection.execute(
            "SELECT table_name FROM information_schema.tables "
            "WHERE table_schema = 'public' AND table_name = 'partial_table'"
        ).fetchall()
        assert rows == [], "失败迁移的部分对象必须回滚"
        versionRows = connection.execute(
            "SELECT version FROM schema_version WHERE version = '5'"
        ).fetchall()
        assert versionRows == [], "失败迁移不得记录版本"


def test_fact_table_rejects_update_and_delete(applied) -> None:  # noqa: ANN001
    with openConnection() as connection:
        connection.execute(
            "INSERT INTO run_manifests (run_id, code_version, event_schema_registry_hash, "
            "strategy_version, strategy_source_hash, dependency_lock_hash, interpreter_version, "
            "sandbox_image_digest, strategy_sandbox_policy_version, strategy_dsl_schema_version, "
            "investment_plan_schema_version, config_hash, config_schema_version, data_version_id, "
            "asset_capability_version, account_group_id, account_ranks, random_seed, ts_precision, "
            "event_ordering_version, execution_model_version, fund_execution_model_version, "
            "nav_availability_policy_version, bar_path_model_version, liquidity_allocation_version, "
            "risk_policy_version, reliability_policy_version, started_at) "
            "VALUES ('run-test', 'v', '0'*64, 'v', '0'*64, '0'*64, 'v', 'd', 'v', 'v', 'v', "
            "'0'*64, 'v', 'dv', 'v', 'ag', '{}', 1, 'MILLISECOND', 'V1', 'v', 'v', 'v', 'v', 'v', "
            "'v', 'v', now())"
        )
        connection.execute(
            "INSERT INTO fact_events (event_id, event_type, schema_version, run_id, ts, ingested_at, "
            "source, producer, producer_version, correlation_id, event_ordering_version, phase, "
            "priority, source_rank, source_sequence, payload, content_hash, account_group_id, "
            "partition_rank, delivery_sequence) VALUES ('evt-1', 'TestEvent', 'V1', 'run-test', "
            "now(), now(), 'src', 'prod', '1.0', 'corr', 'V1', 10, 0, 0, 1, '{}', '0'*64, "
            "'ag-test', 0, 1)"
        )
        with pytest.raises(psycopg.errors.ObjectNotInPrerequisiteState):
            connection.execute("UPDATE fact_events SET priority = 1 WHERE event_id = 'evt-1'")
        with pytest.raises(psycopg.errors.ObjectNotInPrerequisiteState):
            connection.execute("DELETE FROM fact_events WHERE event_id = 'evt-1'")


def test_unique_keys_enforced(applied) -> None:  # noqa: ANN001
    with openConnection() as connection:
        with pytest.raises(psycopg.errors.UniqueViolation):
            connection.execute(
                "INSERT INTO fact_events (event_id, event_type, schema_version, run_id, ts, "
                "ingested_at, source, producer, producer_version, correlation_id, "
                "event_ordering_version, phase, priority, source_rank, source_sequence, payload, "
                "content_hash, account_group_id, partition_rank, delivery_sequence) "
                "VALUES ('evt-1', 'TestEvent', 'V1', 'run-test', now(), now(), 'src', 'prod', "
                "'1.0', 'corr', 'V1', 10, 0, 0, 1, '{}', '0'*64, 'ag-test', 0, 1)"
            )


def test_numeric_precision_enforced(applied) -> None:  # noqa: ANN001
    with openConnection() as connection:
        with pytest.raises(psycopg.errors.NumericValueOutOfRange):
            connection.execute(
                "INSERT INTO ledger_balance_projection (account_id, ledger_account, unit_id, "
                "book_currency, quantity, cost_amount, last_ledger_sequence) "
                "VALUES ('acct-1', 'CASH_AVAILABLE', 'CNY', 'CNY', 1e30, 1e30, 0)"
            )


def test_account_ledger_sequence_unique(applied) -> None:  # noqa: ANN001
    with openConnection() as connection:
        connection.execute(
            "INSERT INTO ledger_journals (journal_id, journal_type, account_id, ts, "
            "commit_sequence, source_event_id, instrument_metadata_version, "
            "fee_schedule_version, accounting_policy_version, run_id, journal_hash) "
            "VALUES ('j-1', 'DEPOSIT', 'acct-1', now(), 1, 'evt-1', 'v', 'v', 'v', 'run-test', '0'*64)"
        )
        with pytest.raises(psycopg.errors.UniqueViolation):
            connection.execute(
                "INSERT INTO ledger_journals (journal_id, journal_type, account_id, ts, "
                "commit_sequence, source_event_id, instrument_metadata_version, "
                "fee_schedule_version, accounting_policy_version, run_id, journal_hash) "
                "VALUES ('j-2', 'DEPOSIT', 'acct-1', now(), 1, 'evt-2', 'v', 'v', 'v', 'run-test', '0'*64)"
            )
