# P1-039 期货保证金、盯市与到期证据

已实现 `FuturesMarginBookV1`，以 `Decimal` 按合约数量、结算价和合约乘数计算初始保证金及逐日盯市现金变动。保证金不足被明确标记为压力状态；到期日前禁止进入交割/平仓处理，到期未平仓合约必须显式进入交割或平仓流程。

```powershell
python3 -m pytest tests\unit\accounts\test_futures.py -q
```
