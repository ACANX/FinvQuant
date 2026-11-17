# P1-031 崩溃注入验证证据

提交前后 inbox、账本、订单、控制、checkpoint 和 outbox 共 12 个边界均可按固定命中次数确定性注入受控异常。

```powershell
python3 -m pytest tests\unit\core\test_crash_injection.py -q
# 13 passed
```
