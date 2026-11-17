# P1-014 标的注册表验证证据

## 实现范围

- 版本化 `InstrumentV1`、交易日历、费率表和资产能力清单。
- `518880` 黄金 ETF 的 SSE、人民币、最小手数、tick 与 T+1 结算契约。
- `AU2608` 单一上期所黄金期货合约的乘数、保证金、夜盘日历、到期日与逐日盯市结算契约。
- 注册表引用完整性与按执行模式的默认拒绝能力门禁。

## 自动化验证

执行时间：2026-07-31T13:19:45Z。

```powershell
python3 -m pytest tests\unit\instruments\test_registry.py tests\contract\test_architecture_dependencies.py -q
# 4 passed

python3 -m ruff check src\veritasquant\instruments tests\unit\instruments
# All checks passed!

python3 -m mypy src\veritasquant\instruments
# Success: no issues found in 2 source files
```

该记录仅提供作者侧实现与自动化验证证据；任务保持 `IN_REVIEW`，未标记为 `ACCEPTED`。
