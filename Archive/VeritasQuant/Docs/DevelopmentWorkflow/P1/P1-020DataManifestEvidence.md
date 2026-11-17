# P1-020 DataManifestV1 数据版本哈希与修订链验证证据

实现不可变 `DataManifestV1` 与 `SignedDataManifestV1`：`data_version_id` 只由规范化
身份字段的规范 JSON SHA-256 计算。存储 URI、下载时间、本机绝对路径、签名和备注
不参与身份；内容、Schema、日历、映射或质量规则变化必然产生新 ID；篡改被拒绝。

## 实现与测试

- 实现：`src/veritasquant/data/DataManifest.py`
  - `DataManifestV1`：Schema/精度/文件指纹/原始对象/版本引用/隔离/修订链身份字段
  - `computeDataVersionId`：`canonicalHash(identityFields)`（复用 `core/CanonicalJson`）
  - `SignedDataManifestV1`：模型校验器强制 `data_version_id` 与身份一致，篡改拒绝
  - `signManifest`：签名并返回持久化 manifest
- 测试：`tests/unit/data/test_data_manifest.py`

## 验证结果

```powershell
python3 -m pytest tests\unit\data\test_data_manifest.py -q
# 9 passed
```

## 验收标准映射

| 验收标准 | 证据 |
| --- | --- |
| 路径变化不改身份 | `test_storage_uri_and_download_time_do_not_change_identity`（URI/时间/本机路径/备注不改 ID） |
| 内容/Schema/日历变化必改 ID | `test_content_change_must_change_id`、`test_schema_and_calendar_change_must_change_id` |
| manifest 篡改被拒绝 | `test_tampered_manifest_is_rejected_on_validation`、`test_sign_manifest_explicit_verify_raises_domain_error` |
| 修订链合法 | `test_revision_chain_requires_reason_and_valid_hash`（Supersedes 必须带修订原因） |
| 隔离记录一致性 | `test_isolation_hash_consistency_rule`（有/无隔离记录的零哈希约束） |

## 关键决策

- 身份哈希严格限定身份字段集合；数组按键排序、Decimal/UTC/null 按第 4.1 节规范化，
  与 `core/CanonicalJson.canonicalHash` 完全一致。
- 模型校验器在构造时强制执行身份校验，直接 `model_validate` 构造也无法绕过。
- 注意：pydantic 将模型校验器异常包装为 `ValidationError`，测试按此捕获。

## 残余风险

- 依赖版本字段 `dependencyVersions` 为自由版本字符串；若需参与身份哈希，应在
  后续任务中明确其受控格式。
