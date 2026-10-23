"""P2-007 数据库 checkpoint 存储（模拟盘重启 RPO=0）。

重启恢复流程（TechSpec 6.3）：读取分区最后已提交 checkpoint，从不可变
事实序列重放至该序号；RPO=0 意味着已提交领域事实绝不丢失——checkpoint
与领域写入同事务提交，重启后从最后已提交 checkpoint 继续。
"""

from __future__ import annotations

from psycopg import Connection

from veritasquant.core.Checkpoint import EventProcessingCheckpointV1

_SAVE_SQL = """
INSERT INTO partition_checkpoints (
    run_id, partition_id, last_committed_sequence, transaction_id, checkpoint_ts
) VALUES (%s, %s, %s, %s, now())
ON CONFLICT (run_id, partition_id) DO UPDATE
SET last_committed_sequence = EXCLUDED.last_committed_sequence,
    transaction_id = EXCLUDED.transaction_id,
    checkpoint_ts = now()
"""

_LOAD_SQL = """
SELECT last_committed_sequence, transaction_id
FROM partition_checkpoints WHERE run_id = %s AND partition_id = %s
"""


class CheckpointStoreError(RuntimeError):
    """checkpoint 存储不合法。"""


class CheckpointStoreV1:
    """PostgreSQL 分区 checkpoint 存储。"""

    def __init__(self, connection: Connection) -> None:
        self._connection = connection

    def save(self, checkpoint: EventProcessingCheckpointV1) -> None:
        """保存/推进 checkpoint；重复保存相同序号是幂等的。"""
        if not checkpoint.runId or not checkpoint.partitionId or not checkpoint.transactionId:
            raise CheckpointStoreError("checkpoint 必须包含运行、分区与事务 ID")
        if checkpoint.lastCommittedSequence < 0:
            raise CheckpointStoreError("checkpoint 序号必须非负")
        with self._connection.transaction():
            self._connection.execute(
                _SAVE_SQL,
                (
                    checkpoint.runId,
                    checkpoint.partitionId,
                    checkpoint.lastCommittedSequence,
                    checkpoint.transactionId,
                ),
            )

    def load(self, runId: str, partitionId: str) -> EventProcessingCheckpointV1 | None:
        """读取分区最后已提交 checkpoint；无记录返回 None（从零重放）。"""
        row = self._connection.execute(_LOAD_SQL, (runId, partitionId)).fetchone()
        if row is None:
            return None
        sequence, transactionId = row
        return EventProcessingCheckpointV1(runId, partitionId, int(sequence), transactionId)

    def latestSequence(self, runId: str, partitionId: str) -> int:
        """分区最后已提交序号（0 表示尚无 checkpoint）。"""
        checkpoint = self.load(runId, partitionId)
        return checkpoint.lastCommittedSequence if checkpoint is not None else 0
