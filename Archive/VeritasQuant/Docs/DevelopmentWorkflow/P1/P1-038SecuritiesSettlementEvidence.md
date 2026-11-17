# P1-038 证券 T+1 与公司行为证据

已实现 `SecuritiesSettlementBookV1`：证券买入进入待结算队列，当日及结算日前均不可卖；在后续交易日结算后才转为可卖。拆并股公司行为会同时按固定比例调整已结算和待结算数量，并按公司行为 ID 去重。费用、税款和现金分红分别使用 `CashJournalFactoryV1` 的 `FEE`、`TAX` 和 `DIVIDEND` 不可变 journal。

```powershell
python3 -m pytest tests\unit\accounts\test_securities.py -q
```
