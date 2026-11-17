# P1-027 Inbox 幂等验证证据

同键同哈希仅首次处理；同键不同哈希生成隔离审计并拒绝处理。

```powershell
python3 -m pytest tests\unit\core\test_inbox.py -q
# 2 passed
```
