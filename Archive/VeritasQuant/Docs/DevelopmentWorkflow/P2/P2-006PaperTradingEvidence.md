# P2-006 纸上交易适配器与增量市场事件接入 — 实现证据

- **PlanTaskId：** P2-006
- **里程碑：** M2A
- **作者：** BeeAgent
- **状态：** ACCEPTED（PR #219/#220/#221 已合并）
- **日期：** 2026-08-02

## 1. 实现内容

| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 增量接入 | `src/veritasquant/execution/IncrementalFeed.py` | `IncrementalMarketFeedV1`：bar_start 严格递增、间隔超限判断流、保护状态、显式恢复 |
| 纸上交易适配器 | `src/veritasquant/execution/PaperTradingAdapter.py` | `PaperTradingAdapterV1`：增量行情驱动，复用确定性撮合，返回 `ExecutionReportEventV1` 回报契约；保护状态拒绝发单 |
| 单元测试 | `tests/unit/execution/test_paper_trading_adapter.py` | 7 用例 |

## 2. 验收标准映射

| 验收标准 | 证据 |
| --- | --- |
| 适配器遵循相同订单/回报契约 | `test_submit_returns_same_report_contract`：返回 `ExecutionReportEventV1`，mode=`PAPER_TRADING` |
| 断流进入保护状态而非继续发单 | `test_stale_feed_blocks_new_orders`：gap>上限 → protected，`submitOrder` 返回 `report=None`（无副作用）；`test_recover_allows_orders_again` 显式恢复后才可发单 |

## 3. 设计要点

- 模拟盘使用增量行情（每次只推新 Bar），与全量历史回放分离。
- 断流保护：`ingest` 检测乱序/间隔超限进入保护；恢复必须显式调用，禁止自动恢复发单。
- 撮合契约与理想/仿真/实盘一致（订单意图→状态迁移→执行回报→账本审计）。

## 4. 测试证据

```text
$ .venv/bin/python -m pytest tests/unit/ -q   # 552 passed
$ .venv/bin/ruff check src tests              # All checks passed!
```

## 5. 证据索引

- `src/veritasquant/execution/IncrementalFeed.py`、`PaperTradingAdapter.py`
- `tests/unit/execution/test_paper_trading_adapter.py`
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-006PaperTradingEvidence.md`
