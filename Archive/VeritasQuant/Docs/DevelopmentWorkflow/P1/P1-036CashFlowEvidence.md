# P1-036 现金流、费用税款与冲正 journal 证据

已实现受版本绑定的 `CashJournalFactoryV1`，生成开户余额、入金、出金、费用、税和全额冲正 journal。出金在构造时要求金额不超过调用方提供的可用现金；冲正不修改原始 journal，而是反转每条分录并保存 `reversal_of_journal_id`。

```powershell
python3 -m pytest tests\unit\accounts\test_ledger.py -q
```
