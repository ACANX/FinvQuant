# P1-025 UTC 逻辑时钟与阶段推进器验证证据

实现仅向前推进的 UTC 逻辑时钟与阶段推进器。时钟、事件 ts 与 phase 必须符合
完整排序键；任何回退、非法跨阶段或以系统时间替代逻辑时钟的行为都被拒绝。

## 实现与测试

- 实现：`src/veritasquant/core/LogicalClock.py`
  - `UtcLogicalClockV1`：`advance` 拒绝回退；`observe` 按事件 ts 推进；
    `checkNotBeyond` 防前视访问入口
  - `ClockPhase`：与 EventOrderingVersion V1 一致的六阶段表（10~60）
  - `PhaseAdvancerV1`：派生事件继承 ts/排序版本/因果引用，禁止更早或相同阶段
- 测试：`tests/unit/core/test_logical_clock.py`

## 验证结果

```powershell
python3 -m pytest tests\unit\core\test_logical_clock.py -q
# 10 passed
```

## 验收标准映射

| 验收标准 | 证据 |
| --- | --- |
| 时钟只前进 | `test_clock_advances_monotonically`、`test_clock_rejects_rollback` |
| 派生事件不能回到更早 phase | `test_phase_advancer_rejects_earlier_or_same_phase` |
| 非法跨阶段被拒绝 | `test_phase_advancer_rejects_unknown_phase`（未知阶段 99 拒绝） |
| 防前视（查询不超越时钟） | `test_check_not_beyond_blocks_future_queries` |
| 禁止系统时间替代 | 时钟只接受显式 `advance`/`observe`，无任何系统时间读取路径 |

## 关键决策

- 时钟状态完全由事件驱动，`now` 初始为 None，绝不读取 `datetime.now()`。
- 阶段推进规则：同一 ts 下派生事件 phase 必须严格大于父事件；更早 ts 或更早
  phase 一律拒绝，与第 15 章"派生事件必须进入更后的合法阶段"一致。
- `deriveWithClock` 组合时钟推进与阶段派生，保证事件循环主路径单一入口。

## 残余风险

- 阶段表与 `core/EventOrdering.EventPhase` 存在两份枚举定义；后续若版本化排序
  规则变更（V2），需同步两处并新增回归基准。
