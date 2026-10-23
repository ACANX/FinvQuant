# P1-029 Checkpoint 与投影重建验证证据

checkpoint 只引用已提交事实序列；删除投影后从事实重建得到相同内容哈希。

```powershell
python3 -m pytest tests\unit\core\test_checkpoint.py -q
# 2 passed
```
