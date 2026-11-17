# P1-034 复式账本原子提交证据

已实现只追加 `LedgerStoreV1`。提交边界会由 wire 形式重新校验 `JournalV1`，避免 `model_copy` 等内存操作绕过逐单位平衡约束；校验、journal ID 唯一性和单调提交序号全部通过后才追加完整 journal。存储仅暴露不可修改的 journal 与分录快照，不提供更新或删除接口。

覆盖：完整提交、序号单调、重复 journal 拒绝，以及不平衡或序号错误提交时既有历史保持不变。

```powershell
python3 -m pytest tests\unit\accounts\test_ledger.py -q
```
