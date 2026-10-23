# P1-019 规范化 Parquet 写入验证证据

实现 `MinuteBarSchemaV1` 到不可变规范化 Parquet 的确定性写入：固定 Arrow 逻辑类型、
Decimal 精度、稳定主键排序、固定行组大小、PLAIN 编码、UNCOMPRESSED 压缩与固定
写入器版本 `vq-parquet-v1`。按 `{DatasetId}/{Market}/{Symbol}/Year/Month/{ContentHash}.parquet`
逻辑路径存储，旧版本绝不覆盖；价格与数量全程 Decimal，禁止 float。

## 实现与测试

- 实现：`src/veritasquant/data/ParquetFile.py`
  - `writeParquetBytes`：单行组确定性编码（thrift compact protocol footer）
  - `ParquetStoreV1.storeBars`：内容 SHA-256 寻址 + `O_EXCL` 拒绝覆盖
  - `sortBars`：主键 + `source_sequence` 稳定排序，乱序输入拒绝
  - `readParquetSummary`：最小读取器回读验证 schema、行序与首末 Bar
- 测试：`tests/unit/data/test_parquet_file.py`

## 验证结果

```powershell
python3 -m pytest tests\unit\data\test_parquet_file.py -q
# 9 passed
```

## 验收标准映射

| 验收标准 | 证据 |
| --- | --- |
| 固定输入在两平台生成逻辑等价内容 | 写入字节确定性测试 `test_write_is_byte_deterministic_across_calls`；纯 Python 无平台差异路径 |
| 稳定行序和预期 Schema | `test_write_and_readback_roundtrip_preserves_rows_and_order` 回读验证 21 列 schema 与首末行 |
| 旧版本不覆盖 | `test_store_never_overwrites_and_returns_logical_path`、`test_store_rejects_content_hash_conflict` |
| 输入/协议错误与边界 | `test_write_rejects_unsorted_input`、空文件、重复主键、篡改魔数拒绝 |
| 重复/重放行为 | 相同输入重复写入字节完全一致，内容哈希复用 |

## 关键决策

- **不引入 pyarrow**：`Runtime.lock` 由 P0-005 固定为最小运行依赖，新增依赖须走
  Change 流程；自研纯 Python 写入器实现字节级跨平台确定性，且完全符合
  "固定物理参数"验收要求。
- 固定物理参数：行组 65,536 行、PLAIN 编码、UNCOMPRESSED、`vq-parquet-v1`。

## 残余风险

- 读取器为本项目自研最小实现；未来若需与外部 Parquet 工具互读，应新增契约测试
  并评估引入 pyarrow（走依赖 Change 流程）。
