# P2-009 背压、磁盘/队列阈值与 trading-readiness — 实现证据

- **PlanTaskId：** P2-009 | **里程碑：** M2A | **作者：** BeeAgent | **状态：** ACCEPTED（PR #219/#220/#221 已合并）

## 1. 实现内容
| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 阈值与门禁 | `src/veritasquant/monitoring/TradingReadiness.py` | `QueueThresholdPolicyV1`（70% 告警/90% 硬上限）、`DiskSpacePolicyV1`（20%/10%）、`TradingReadinessGateV1`（行情/对账/账本/控制/队列/磁盘/时钟 8 项检查） |
| 测试 | `tests/unit/monitoring/test_trading_readiness.py`（12） | |

## 2. 验收标准映射
| 标准 | 证据 |
| --- | --- |
| 70%/90% 阈值行为符合方案 | `test_70_and_90_percent_thresholds`、`test_20_and_10_percent_free_ratios` |
| 硬阈值时禁止新增风险且不丢关键事件 | `mayOpenNewRisk(0.90)=False`；`allowCriticalWrites(1.00)=True`（关键 inbox/账本/控制/审计不丢弃） |

## 3. 证据索引
- `src/veritasquant/monitoring/TradingReadiness.py`、`tests/unit/monitoring/test_trading_readiness.py`
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-009BackpressureEvidence.md`
