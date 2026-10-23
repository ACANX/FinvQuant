"""P2-003 确定性分区扇出器。

同一共享事件按冻结的 `partition_rank` 升序扇出到每个账户组分区：
- 扇出顺序由 partition_rank 决定，与进程调度或网络到达顺序无关；
- 分区可以暂时落后（快慢分区），但事件信封、排序键与内容哈希在所有
  分区完全一致；分区投递序号是信封外元数据，不改变事件因果时间；
- 每个分区维护独立的 delivery_sequence（从 1 递增）。

核心逻辑为纯函数，不依赖数据库，保证可本地确定性测试。
"""

from __future__ import annotations

from dataclasses import dataclass

from veritasquant.core.Events import EventEnvelopeV1


class FanoutError(ValueError):
    """扇出目标或投递契约不满足。"""


@dataclass(frozen=True, slots=True)
class FanoutTargetV1:
    """一个冻结的账户组分区目标。"""

    accountGroupId: str
    partitionRank: int


@dataclass(frozen=True, slots=True)
class PartitionDeliveryV1:
    """一次分区投递：事件内容在所有分区一致，投递序号独立。"""

    accountGroupId: str
    partitionRank: int
    event: EventEnvelopeV1
    deliverySequence: int


class DeterministicFanoutV1:
    """按 partition_rank 升序冻结目标并逐分区分配投递序号。"""

    def __init__(self, targets: tuple[FanoutTargetV1, ...]) -> None:
        if not targets:
            raise FanoutError("扇出目标不能为空")
        if len({target.accountGroupId for target in targets}) != len(targets):
            raise FanoutError("扇出目标账户组不得重复")
        self._targets = tuple(sorted(targets, key=lambda target: target.partitionRank))

    @property
    def targets(self) -> tuple[FanoutTargetV1, ...]:
        """按 partition_rank 升序的冻结目标。"""
        return self._targets

    def plan(self, event: EventEnvelopeV1, currentSequences: dict[str, int]) -> tuple[PartitionDeliveryV1, ...]:
        """为共享事件生成确定性投递计划。

        currentSequences 给出每个账户组的当前最大投递序号；
        本分区下一投递序号 = 当前序号 + 1。
        """
        deliveries: list[PartitionDeliveryV1] = []
        for target in self._targets:
            current = currentSequences.get(target.accountGroupId, 0)
            deliveries.append(
                PartitionDeliveryV1(
                    accountGroupId=target.accountGroupId,
                    partitionRank=target.partitionRank,
                    event=event,
                    deliverySequence=current + 1,
                )
            )
        return tuple(deliveries)
