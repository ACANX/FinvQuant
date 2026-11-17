"""P2-001 迁移文件的静态契约测试。

不依赖真实数据库，验证迁移文件命名、必填表/列/约束/索引、
NUMERIC 精度与不可变触发器声明，保证 CI 早期即可定位违规。
"""

from __future__ import annotations

import re
from pathlib import Path

import pytest

_REPO_ROOT = Path(__file__).resolve().parents[3]
_MIGRATIONS_DIR = _REPO_ROOT / "Migrations" / "postgresql"

_MIGRATION_NAME = re.compile(r"^V(\d+)__[A-Za-z0-9_\-]+\.sql$")

# 首版迁移必须包含的表与职责分组
_REQUIRED_TABLES = {
    "fact_events": "事件事实",
    "inbox_records": "inbox 幂等",
    "inbox_conflicts": "协议冲突隔离",
    "outbox_records": "outbox 至少一次投递",
    "partition_leases": "单活租约与 fencing token",
    "partition_checkpoints": "分区检查点",
    "run_manifests": "运行清单",
    "ledger_journals": "账本 journal",
    "ledger_entries": "账本分录",
    "order_intents": "订单意图",
    "order_events": "订单状态迁移",
    "cancel_order_requests": "撤单请求",
    "replace_order_requests": "改单请求",
    "execution_reports": "成交回报",
    "risk_decisions": "风险决定",
    "trading_controls": "交易控制",
    "account_snapshots": "账户快照投影",
    "ledger_balance_projection": "余额投影",
    "account_position_projection": "持仓投影",
    "activity_control_projection": "活动控制投影",
    "schema_version": "迁移版本跟踪",
}

# 账本/订单/风控金额字段必须使用不低于 NUMERIC(38,18) 的精确类型
_NUMERIC_18_FIELDS = {
    "ledger_entries": ("quantity", "book_amount", "cost_amount"),
    "order_intents": ("quantity",),
    "order_events": ("approved_quantity", "quantity"),
    "execution_reports": ("last_quantity", "cumulative_quantity", "remaining_quantity"),
    "risk_decisions": ("approved_quantity",),
    "ledger_balance_projection": ("quantity", "cost_amount"),
    "account_position_projection": ("quantity", "cost_amount"),
}

# 价格字段使用 NUMERIC(38,12)
_NUMERIC_12_FIELDS = {
    "order_intents": ("limit_price", "stop_price"),
    "order_events": ("limit_price", "stop_price"),
    "execution_reports": ("last_price",),
}

# 不可变事实表清单（触发器禁止 UPDATE/DELETE）
_IMMUTABLE_TABLES = {
    "fact_events",
    "inbox_records",
    "inbox_conflicts",
    "ledger_journals",
    "ledger_entries",
    "order_intents",
    "order_events",
    "cancel_order_requests",
    "replace_order_requests",
    "execution_reports",
    "risk_decisions",
    "trading_controls",
}


def _migrationFiles() -> list[Path]:
    files = sorted(_MIGRATIONS_DIR.glob("V*__*.sql"))
    assert files, f"迁移目录为空: {_MIGRATIONS_DIR}"
    return files


def test_migration_file_naming() -> None:
    for path in _migrationFiles():
        assert _MIGRATION_NAME.fullmatch(path.name), f"命名不符合 V<N>__<name>.sql: {path.name}"


def test_migration_versions_unique_and_sorted() -> None:
    versions = [int(_MIGRATION_NAME.fullmatch(path.name).group(1)) for path in _migrationFiles()]
    assert len(versions) == len(set(versions)), "迁移版本号重复"
    assert versions == sorted(versions), "迁移版本号必须递增"


def test_migration_file_utf8() -> None:
    for path in _migrationFiles():
        path.read_text(encoding="utf-8")  # 非 UTF-8 直接抛异常


def test_migration_wrapped_in_transaction() -> None:
    for path in _migrationFiles():
        text = path.read_text(encoding="utf-8")
        # 跳过文件头注释与空行后，第一条语句必须是 BEGIN;
        firstStatement = next(
            line.strip()
            for line in text.splitlines()
            if line.strip() and not line.strip().startswith("--")
        )
        assert firstStatement == "BEGIN;", f"{path.name} 第一条语句必须是 BEGIN;"
        assert "COMMIT;" in text, f"{path.name} 必须包含 COMMIT;"


def test_required_tables_declared() -> None:
    text = "\n".join(path.read_text(encoding="utf-8") for path in _migrationFiles())
    for table, purpose in _REQUIRED_TABLES.items():
        assert re.search(rf"CREATE TABLE\s+(IF NOT EXISTS\s+)?{re.escape(table)}\b", text), (
            f"缺少必填表 {table}（{purpose}）"
        )


def test_numeric_precision_declared() -> None:
    text = "\n".join(path.read_text(encoding="utf-8") for path in _migrationFiles())
    for table, fields in _NUMERIC_18_FIELDS.items():
        for field in fields:
            assert re.search(
                rf"\b{re.escape(field)}\s+NUMERIC\(\s*38\s*,\s*18\s*\)",
                text,
            ), f"{table}.{field} 必须为 NUMERIC(38,18)"
    for table, fields in _NUMERIC_12_FIELDS.items():
        for field in fields:
            assert re.search(
                rf"\b{re.escape(field)}\s+NUMERIC\(\s*38\s*,\s*12\s*\)",
                text,
            ), f"{table}.{field} 必须为 NUMERIC(38,12)"


def test_immutable_triggers_declared() -> None:
    text = "\n".join(path.read_text(encoding="utf-8") for path in _migrationFiles())
    assert "prevent_fact_mutation" in text, "缺少不可变触发器函数"
    # 触发器在 DO 块中按表清单动态创建：验证清单与 format 模板
    assert "FOREACH immutable_table IN ARRAY" in text, "缺少不可变表清单"
    assert "trg_%1$s_immutable" in text, "缺少不可变触发器 format 模板"
    for table in _IMMUTABLE_TABLES:
        assert f"'{table}'," in text or f"'{table}'\n" in text, f"不可变表清单缺少 {table}"


def test_account_scope_columns_present() -> None:
    """账户域事实表必须携带账户作用域列，保证账户分区隔离。"""
    text = "\n".join(path.read_text(encoding="utf-8") for path in _migrationFiles())
    account_scoped = {
        "fact_events": "account_id",
        "ledger_journals": "account_id",
        "ledger_entries": "account_id",
        "order_intents": "account_id",
        "order_events": "account_id",
        "cancel_order_requests": "account_id",
        "replace_order_requests": "account_id",
        "execution_reports": "account_id",
        "risk_decisions": "account_id",
    }
    for table, column in account_scoped.items():
        assert re.search(
            rf"CREATE TABLE\s+(IF NOT EXISTS\s+)?{re.escape(table)}\b[\s\S]*?"
            rf"\b{re.escape(column)}\s+TEXT",
            text,
        ), f"{table} 缺少账户作用域列 {column}"


def test_unique_keys_declared() -> None:
    text = "\n".join(path.read_text(encoding="utf-8") for path in _migrationFiles())
    expected_unique = {
        "uq_fact_events_partition_delivery": "fact_events 分区投递唯一",
        "uq_ledger_journals_account_sequence": "账本账户提交序号唯一",
        "uq_ledger_entries_journal_entry": "journal 内分录唯一",
        "uq_order_events_client_version": "订单版本唯一",
        "uq_execution_reports_account_execution": "账户内成交唯一",
        "uq_inbox_partition_receipt": "inbox 分区序号唯一",
    }
    for index, purpose in expected_unique.items():
        assert index in text, f"缺少唯一索引 {index}（{purpose}）"


def test_v2_command_resources_declared() -> None:
    """P2-026：V2 迁移必须声明命令资源表、幂等唯一与身份冻结触发器。"""
    v2 = next(
        (path for path in _migrationFiles() if path.name.startswith("V2__")),
        None,
    )
    assert v2 is not None, "缺少 V2 迁移文件"
    text = v2.read_text(encoding="utf-8")
    assert re.search(r"CREATE TABLE\s+IF NOT EXISTS\s+command_records\b", text)
    for column in (
        "command_id",
        "command_type",
        "account_id",
        "run_id",
        "requested_by",
        "idempotency_scope",
        "payload_hash",
        "payload",
        "expected_version",
        "confirmation_token_id",
        "status",
        "created_ts",
        "updated_ts",
        "result_reference",
        "failure_code",
        "failure_error_code",
        "failure_catalog_version",
        "failure_retryable",
        "failure_details",
    ):
        assert re.search(rf"\b{re.escape(column)}\s+", text), f"command_records 缺少列 {column}"
    assert "uq_command_records_idempotency_scope" in text, "缺少幂等作用域唯一索引"
    assert "assert_command_identity_frozen" in text, "缺少身份冻结触发器函数"
    assert "TRIGGER trg_command_records_frozen" in text, "缺少命令冻结触发器"
    assert "payload_hash ~ '^[0-9a-f]{64}$'" in text, "payload_hash 必须为 SHA-256 格式"


def test_migrator_discovery_logic(tmp_path: Path) -> None:
    """不连数据库，验证 Migrator 的文件发现与命名校验逻辑。"""
    from veritasquant.infrastructure.persistence.Migrator import Migrator, MigrationError

    (tmp_path / "V1__initial.sql").write_text("BEGIN;\nCOMMIT;", encoding="utf-8")
    (tmp_path / "V2__add_table.sql").write_text("BEGIN;\nCOMMIT;", encoding="utf-8")

    class _FakeCursor:
        def fetchall(self) -> list:
            return []

    class _FakeConnection:
        def __init__(self) -> None:
            self.script = None

        def execute(self, script, params=None):  # noqa: ANN001
            if params is None:
                self.script = script
            return _FakeCursor()

        def transaction(self):  # noqa: ANN003
            return _FakeTransaction()

    class _FakeTransaction:
        def __enter__(self):
            return self

        def __exit__(self, *args):  # noqa: ANN002
            return False

    migrator = Migrator(tmp_path, _FakeConnection())  # type: ignore[arg-type]
    assert migrator.pendingVersions() == [1, 2]

    badDir = tmp_path / "nested"
    badDir.mkdir()
    (badDir / "V1__ok.sql").write_text("BEGIN;\nCOMMIT;", encoding="utf-8")
    (badDir / "V2__bad!.sql").write_text("BEGIN;\nCOMMIT;", encoding="utf-8")
    with pytest.raises(MigrationError):
        Migrator(badDir, _FakeConnection()).pendingVersions()  # type: ignore[arg-type]

    duplicateDir = tmp_path / "dups"
    duplicateDir.mkdir()
    (duplicateDir / "V1__a.sql").write_text("BEGIN;\nCOMMIT;", encoding="utf-8")
    (duplicateDir / "V1__b.sql").write_text("BEGIN;\nCOMMIT;", encoding="utf-8")
    with pytest.raises(MigrationError):
        Migrator(duplicateDir, _FakeConnection()).pendingVersions()  # type: ignore[arg-type]
