# P1-015 MinuteBarSchemaV1 验证证据

`MinuteBarSchemaV1` 已覆盖 UTC 时间、`bar_start < bar_end <= ts`、OHLC、Decimal、来源追溯、
复权状态、标的元数据版本、tick 和最小手数约束。

```powershell
python3 -m pytest tests\unit\data\test_minute_bar.py -q
# 3 passed
```

任务保持 `IN_REVIEW`，此记录不替代独立验收。
