"""P2-007 checkpoint 数据库集成测试（重启 RPO=0 语义）。"""

from __future__ import annotations

import psycopg
import pytest

from test_db_helpers import applyMigrations, openConnection, resetSchema

from veritasquant.core.Checkpoint import EventProcessingCheckpointV1
from veritasquant.infrastructure.persistence.CheckpointStore import CheckpointStoreV1

_RUN = "run-checkpoint-test"
_PARTITION = "ag-checkpoint"


@pytest.fixture(scope="module")
def database() -> bool:
    try:
        openConnection().close()
    except psycopg.OperationalError:
        pytest.skip("PostgreSQL 测试实例不可用，跳过 checkpoint 集成测试")
    resetSchema()
    assert applyMigrations()
    return True


@pytest.fixture()
def store(database):  # noqa: ANN001
    connection = openConnection()
    connection.execute("DELETE FROM partition_checkpoints WHERE run_id = %s", (_RUN,))
    connection.execute("DELETE FROM run_manifests WHERE run_id = %s", (_RUN,))
    connection.execute(
        "INSERT INTO run_manifests (run_id, code_version, event_schema_registry_hash, "
        "strategy_version, strategy_source_hash, dependency_lock_hash, interpreter_version, "
        "sandbox_image_digest, strategy_sandbox_policy_version, strategy_dsl_schema_version, "
        "investment_plan_schema_version, config_hash, config_schema_version, data_version_id, "
        "asset_capability_version, account_group_id, account_ranks, random_seed, ts_precision, "
        "event_ordering_version, execution_model_version, fund_execution_model_version, "
        "nav_availability_policy_version, bar_path_model_version, liquidity_allocation_version, "
        "risk_policy_version, reliability_policy_version, started_at) "
        "VALUES (%s, 'v', '0'*64, 'v', '0'*64, '0'*64, 'v', 'd', 'v', 'v', 'v', "
        "'0'*64, 'v', 'dv', 'v', 'ag', '{}', 1, 'MILLISECOND', 'V1', 'v', 'v', 'v', 'v', 'v', "
        "'v', 'v', now())",
        (_RUN,),
    )
    checkpointStore = CheckpointStoreV1(connection)
    yield checkpointStore
    connection.close()


def test_save_and_load_checkpoint(store) -> None:  # noqa: ANN001
    checkpoint = EventProcessingCheckpointV1(_RUN, _PARTITION, 42, "tx-42")
    store.save(checkpoint)
    loaded = store.load(_RUN, _PARTITION)
    assert loaded is not None
    assert loaded.lastCommittedSequence == 42
    assert loaded.transactionId == "tx-42"


def test_checkpoint_advance_is_idempotent_and_monotonic(store) -> None:  # noqa: ANN001
    store.save(EventProcessingCheckpointV1(_RUN, _PARTITION, 10, "tx-10"))
    store.save(EventProcessingCheckpointV1(_RUN, _PARTITION, 10, "tx-10"))  # 幂等重放
    store.save(EventProcessingCheckpointV1(_RUN, _PARTITION, 11, "tx-11"))
    assert store.latestSequence(_RUN, _PARTITION) == 11


def test_restart_replays_from_last_checkpoint_rpo_zero(store) -> None:  # noqa: ANN001
    """模拟重启：新连接读取最后已提交 checkpoint，已提交事实不丢失（RPO=0）。"""
    store.save(EventProcessingCheckpointV1(_RUN, _PARTITION, 7, "tx-7"))
    # 模拟重启：使用全新连接
    connection = openConnection()
    try:
        restartedStore = CheckpointStoreV1(connection)
        checkpoint = restartedStore.load(_RUN, _PARTITION)
        assert checkpoint is not None
        assert checkpoint.lastCommittedSequence == 7  # 重启后从序号 7 继续，无丢失
    finally:
        connection.close()


def test_unknown_partition_returns_none(store) -> None:  # noqa: ANN001
    assert store.load(_RUN, "unknown-partition") is None
    assert store.latestSequence(_RUN, "unknown-partition") == 0
