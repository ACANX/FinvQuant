# P1-028 事务与 Outbox 验证证据

领域事实与 outbox 先在事务中暂存，再原子提交；发布失败保留待发布条目并以同一消息 ID 重试。

```powershell
python3 -m pytest tests\unit\core\test_transaction.py -q
# 2 passed
```
