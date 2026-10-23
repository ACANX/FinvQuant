# P1-022 顺序迭代器与多源最小堆归并验证证据

实现有界内存的顺序迭代器与多源最小堆归并器。归并使用 EventOrderingVersion V1
完整排序键（ts + phase + priority + source_rank + source_sequence + event_id），
跨文件结果符合完整排序键；乱序来源在严格模式被拒绝、宽容模式被隔离。

## 实现与测试

- 实现：`src/veritasquant/data/OrderedMerge.py`
  - `SequentialIteratorV1`：逐条拉取，内存只持有当前元素；严格/宽容模式
  - `MinHeapMergerV1`：最小堆归并，堆大小恒等于源数量
  - `makeEventSortKey`：复用 `core/EventOrdering.eventOrderingKey` 的 V1 排序键
- 测试：`tests/unit/data/test_ordered_merge.py`

## 验证结果

```powershell
python3 -m pytest tests\unit\data\test_ordered_merge.py -q
# 8 passed
```

## 验收标准映射

| 验收标准 | 证据 |
| --- | --- |
| 大文件内存有界 | `test_merge_memory_is_bounded_by_source_count`（1000 条/源，堆 ≤ 1） |
| 跨文件结果符合完整排序键 | `test_merge_combines_sources_in_full_sort_order` |
| 乱序来源被拒绝或隔离 | `test_sequential_iterator_rejects_out_of_order_in_strict_mode`、`test_sequential_iterator_isolates_out_of_order_in_lenient_mode` |
| 阶段优先于来源排名 | `test_merge_respects_phase_before_source_rank`（phase 10 先于 phase 30） |
| 同键确定性 | `test_merge_is_deterministic_with_tied_keys` |

## 关键决策

- 排序键与 `core/EventOrdering.py` 完全一致，防止消费者自行省略或交换字段。
- 最小堆条目携带来源序号，同键时按来源稳定，保证确定性。
- 迭代器在 `_advance` 时校验单调性；严格模式抛错，宽容模式丢弃乱序元素并计数。

## 残余风险

- `EventFileIteratorV1` 为占位实现，事件序列文件格式由 P1-026 事件总线后续定义；
  当前归并测试直接使用内存事件列表。
