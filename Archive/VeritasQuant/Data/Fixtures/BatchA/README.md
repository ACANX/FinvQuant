# P1-024 批次 A 固定数据夹具（Batch A）

本目录包含阶段 1 固定数据夹具，覆盖证券（518880）、黄金期货、缺口样本与错误样本。
夹具与 checksum 固化在 CI 回归中；任何变更必须记录原因与批准（Change 记录）。

## 文件清单

| 文件 | 内容 | 用途 |
|------|------|------|
| `BatchA_Securities_518880.mvsv` | 518880 黄金 ETF 分钟行情（含半日市边界） | 证券路径固定基准 |
| `BatchA_Futures_Gold.mvsv` | 上期所黄金期货交割合约分钟行情（含夜盘） | 期货路径固定基准 |
| `BatchA_Gap.mvsv` | 含时间缺口（会话间跳空）的分钟行情 | 缺口规则与日线边界基准 |
| `BatchA_Errors.mvsv` | 含 OHLC 非法、重复主键、乱序、会话外记录的样本 | 质量规则隔离基准 |

## Checksum 契约

- 每个文件的 SHA-256 与规范化数据/事件序列哈希记录在 `BatchAChecksums.yml`
- 跨平台（Windows/Linux）必须产生相同 checksum，禁止依赖路径、locale 或换行差异
- 修改任一夹具必须更新 `BatchAChecksums.yml` 并在 Change 记录中说明原因与批准
