# P1-016 MVSV-1 解析验证证据

实现流式 MVSV-1 读取器，保留来源头部与外部字段边界，验证 UTF-8 BOM、必填头、精确字段序列、
11 列、Count、IANA 时区、`ts`/`dt` 一致性和 Decimal 解析。

```powershell
python3 -m pytest tests\unit\data\test_mvsv.py -q
# 3 passed
```

仓库 15,000 行 NVDA 样本在固定内存迭代中完成解析。任务保持 `IN_REVIEW`。
