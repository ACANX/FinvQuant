# P1-030 回测状态机验证证据

覆盖创建、运行、暂停、从 checkpoint 继续、失败、取消和终态不可回退。

```powershell
python3 -m pytest tests\unit\core\test_backtest_run.py -q
# 2 passed
```
