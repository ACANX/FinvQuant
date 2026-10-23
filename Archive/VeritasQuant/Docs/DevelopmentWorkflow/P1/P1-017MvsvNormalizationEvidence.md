# P1-017 MVSV 语义映射验证证据

已实现来源 `cp/cr/p` Decimal 语义校验、显式 `BarLabelMeaning`、可用时间计算、未知
`TurnoverScale` 质量标志，以及原始对象哈希、相对路径和 1 基行号追溯。

```powershell
python3 -m pytest tests\unit\data\test_mvsv_normalization.py -q
# 2 passed
```

任务保持 `IN_REVIEW`。
