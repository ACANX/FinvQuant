# P1-035 账本投影重建证据

已实现 `LedgerProjectionStoreV1`，从只追加 journal 按提交顺序重建账户的 `LedgerProjectionSnapshotV1`。快照按科目、资产/币种计量单位和记账币种保存数量与账面金额，覆盖可用现金、冻结资金、持仓成本和已实现/未实现盈亏科目；同时记录 `last_ledger_sequence` 和稳定内容哈希。

覆盖：开户余额与订单冻结后的现金分类余额，以及丢弃投影后从同一 journal 事实集再次重建的字段和哈希一致性。

```powershell
python3 -m pytest tests\unit\accounts\test_ledger.py -q
```
