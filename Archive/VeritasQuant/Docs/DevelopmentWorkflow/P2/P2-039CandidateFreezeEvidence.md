# P2-039 冻结 M2A 候选版本并完成容量/故障预演 — 证据

- **任务：** P2-039（ISSUE #168）
- **日期：** 2026-08-03
- **实现：** BeeAgent
- **PR：** 待第七批 PR 合并后回填

## 验收标准对照

| 验收标准 | 实现 | 证据 |
|----------|------|------|
| 无 S0/S1 | `CapacityForecasterV1.runPreflight()` 输出 S0/S1 判定；S0/S1 阻断冻结 | `src/veritasquant/reliability/CandidateFreeze.py` |
| 队列/磁盘/数据库容量 ≥ 证据窗口 2 倍预测峰值 | `MIN_SAFETY_FACTOR = 2.0`；required = forecastPeak × 2 | `CapacityForecasterV1.forecast()` |
| 冻结 M2A 候选版本 | `CandidateFreezeV1` 冻结清单 + 身份哈希；`CandidateFreezeStoreV1` 只追加、禁止修改历史 | `CandidateFreezeV1.freezeHash()` / `Store.freeze()` |
| 容量不足拒绝冻结 | 预演未通过时 `Store.freeze()` 抛 ValueError | `CandidateFreezeStoreV1.freeze()` |

## 技术方案

- **冻结清单**（对齐 RunManifestV1 身份字段）：代码版本、依赖锁哈希、
  事件 Schema 注册表哈希、沙箱/DSL/计划/配置/风控/可靠性政策版本、冻结人/时间；
  任何字段变化都改变冻结哈希；
- **容量预演**：按资源（队列/磁盘/数据库）取观测样本峰值，
  以 2%/日保守增长率外推证据窗口（60 交易日）峰值，再乘 2.0 安全系数；
  当前容量 < 需求 50% → S0；< 100% → S1；< 200% → S2 观察；≥ 200% → 通过；
- **故障预演语义**：S0/S1 即阻断（容量耗尽会在证据窗口内必然发生），
  预演未通过禁止冻结候选版本。

## 测试

`tests/unit/reliability/test_candidate_freeze.py`（10 用例）：
- 充足容量通过；S1 阻断；S0 极端超卖；无观测跳过；
- 冻结记录/哈希/查询；预演失败拒绝冻结；字段变化哈希变化；
- 无 S0/S1 检查；追加历史保留。

## 验证结果

- ruff / mypy / Preflight：通过
- 全量 pytest：待第七批 PR 后确认（本任务 10 个用例已通过）

## 风险与开放项

- 实际容量观测接入（磁盘/队列/数据库用量采集）需运行环境接线，
  本任务提供预演计算与冻结门禁能力。
