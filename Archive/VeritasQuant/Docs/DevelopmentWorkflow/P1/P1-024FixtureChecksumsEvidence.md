# P1-024 批次 A 固定数据夹具与跨平台 checksum 验证证据

建立批次 A 固定数据夹具，覆盖证券（518880）、黄金期货、缺口样本与错误样本；
数据字节 SHA-256 与规范化数据序列哈希固化在 `BatchAChecksums.yml`，纳入测试回归。
任何夹具变更必须更新 checksum 并记录原因与批准。

## 夹具与实现

- 夹具目录：`Data/Fixtures/BatchA/`
  - `BatchA_Securities_518880.mvsv`：黄金 ETF 分钟行情（含 09:30 开盘、11:30 午盘边界）
  - `BatchA_Futures_Gold.mvsv`：上期所黄金期货（含 21:00 夜盘、02:30 收盘跨午夜）
  - `BatchA_Gap.mvsv`：跨午休缺口样本（10:00 → 13:00）
  - `BatchA_Errors.mvsv`：OHLC 非法、重复主键、乱序、会话外四种错误样本
  - `BatchAChecksums.yml`：字节 SHA-256 + 数据序列哈希固化基准
- 实现：`src/veritasquant/data/FixtureChecksums.py`
  - `verifyFixtureChecksums`：校验夹具与固化基准一致，差异抛错
  - `fixtureDataSequenceHash`：规范化数据行序列哈希（跨平台固定）
- 测试：`tests/unit/data/test_fixture_checksums.py`

## 验证结果

```powershell
python3 -m pytest tests\unit\data\test_fixture_checksums.py -q
# 6 passed
```

## 验收标准映射

| 验收标准 | 证据 |
| --- | --- |
| 夹具含证券 | `BatchA_Securities_518880.mvsv`（SSE/518880，LotSize 100） |
| 夹具含期货 | `BatchA_Futures_Gold.mvsv`（SHFE/AU2612，夜盘跨午夜） |
| 夹具含缺口 | `BatchA_Gap.mvsv` + `test_gap_fixture_contains_cross_session_gap` |
| 夹具含错误样本 | `BatchA_Errors.mvsv` + `test_errors_fixture_contains_invalid_samples_for_isolation` |
| 数据与事件 checksum 在 CI 固定 | `test_fixture_checksums_match_frozen_baseline`（与 yml 基准完全一致） |

## 关键决策

- 字节 SHA-256 不做换行/编码规范化，直接按原始字节计算，跨平台一致。
- 数据序列哈希基于去除头行与注释的规范行，使用 `canonicalHash` 保证 UTF-8 排序确定。
- 错误样本与质量规则（P1-021）衔接：OHLC 非法、重复主键、乱序、会话外均为
  隔离规则可识别的固定输入。

## 残余风险

- 夹具目前为静态文本；接入 MVSV 解析器（P1-016/P1-017）的规范化事件序列哈希
  需在导入管线集成任务中补充端到端契约测试。
