# P1-033 不可变账本模型证据

已定义版本化 `JournalV1`、`LedgerEntryV1`、资产计量单位、显式科目和会计策略。模型使用严格 Pydantic 契约与 `Decimal`，每个 journal 至少两条分录，并按资产/币种数量及账面金额币种独立验证借贷平衡。每条 journal 同时记录账户范围、UTC `ts`、来源事件、提交序号、标的/费率/会计策略版本和可选冲正引用。

覆盖：逐单位平衡、重复分录 ID、UTC、冲正链约束、浮点拒绝、计量单位格式和固定舍入策略。

```powershell
python3 -m pytest tests\unit\accounts\test_ledger.py -q
```
