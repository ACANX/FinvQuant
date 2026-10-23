# P1-023 小时/日 Bar 完成后聚合与受限窗口验证证据

实现基于已完成分钟 Bar 的小时线和日线增量聚合。未完成周期查询失败；日线仅在
会话收盘后的下一有效时点可查询；覆盖跨午夜会话与半日市边界；OHLCV 全程 Decimal。

## 实现与测试

- 实现：`src/veritasquant/data/BarAggregation.py`
  - `BarAggregatorV1`：增量窗口聚合，跨周期自动归档不可变 `AggregatedBarV1`
  - `ClosedWindowViewV1`：只读受限窗口，供策略查询
  - `_dayPeriodEnd` / `_nextValidOpen`：基于日历会话（含跨午夜）计算收盘与下一有效时点
- 测试：`tests/unit/data/test_bar_aggregation.py`

## 验证结果

```powershell
python3 -m pytest tests\unit\data\test_bar_aggregation.py -q
# 8 passed
```

## 验收标准映射

| 验收标准 | 证据 |
| --- | --- |
| 未完成 Bar 查询失败 | `test_hourly_aggregation_completes_only_after_window_end`（窗口结束前拒绝） |
| 日线只在收盘后可用 | `test_daily_bar_only_available_after_session_close_next_open`（次一交易日开盘前拒绝） |
| 边界/半日市测试通过 | `test_half_day_and_boundary_minute_bars`（09:30 开盘首分钟与 14:59 收盘前） |
| 跨周期归档 | `test_hourly_rolls_to_next_window` |
| 升序约束 | `test_append_rejects_out_of_order_minute_bars` |

## 关键决策

- 聚合窗口的 `periodEnd` 对日线取当日全部会话收盘的最大 UTC 时刻；可查询时点
  取次一交易日首会话开盘（`_nextValidOpen`），严格实现"收盘后的下一有效时点"。
- 未注入日历时日线回退为次日零点边界，保证模块可独立测试。
- 窗口归档后不可变，`CompletedAt = periodEnd`，满足 `periodStart < periodEnd <= completedAt`。

## 残余风险

- 会话时间换算使用 `zoneinfo`；阶段 1 固定 `Asia/Shanghai` 时区，其他时区（如
  America/New_York 的 DST）需在阶段 2 前补充跨时区测试。
