# P1-018 原始对象存储验证证据

原始数据按 SHA-256 内容寻址；相同字节复用对象，不同来源路径独立追溯；路径穿越、篡改对象和覆盖均拒绝。

```powershell
python3 -m pytest tests\unit\data\test_raw_object_store.py -q
# 2 passed
```
