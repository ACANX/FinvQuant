# P2-023 基金防前视回归套件证据

## 任务信息
- **PlanTaskId:** P2-023
- **标题:** 建立基金防前视、状态机和完整回归套件
- **阶段/里程碑:** M2A 模拟盘与基金能力建设
- **状态:** IN_REVIEW（等待第四批 PR 合并后验收）

## 实现内容

### 回归套件 `tests/regression/test_fund_anti_lookahead.py`
覆盖场景（TechSpec 12.2.4）：
- 注入未来净值：当前时点决策不变
- 修改未来净值记录：当前决策不变
- 六类方案（固定金额/均线偏离/估值分位/回撤倍增/目标价值/目标收益）防前视
- 三种 Daily 金额模式（Fixed/RuleBased/ExplicitSeries）未来序列注入
- 均线方案只使用已发布历史（含污染截断验证）

### 修复的前视缺陷：MaDeviationPlanV1
**缺陷现象**：`navHistory[-maWindow:]` 直接取均线窗口，未来净值混入
序列尾部后窗口被污染，决策从 1019.80 漂移到 2146.51。

**修复**：新增 `_usableHistory(context)`，以 `availableNav`（当前时点
净值）最后一次出现位置为锚截断历史，未来数据不参与均线计算。

**契约约定**：navHistory 按日期升序，最后一项为当前时点净值
（availableNav）；调用方误将未来净值混入尾部时实现必须截断。

## 验收标准映射
| 验收标准 | 实现 | 证据 |
| --- | --- | --- |
| 注入未来数据决策不变 | test_future_nav_injection / revision | tests/regression/test_fund_anti_lookahead.py |
| 六类方案防前视 | test_six_plan_types_no_lookahead | 同上 |
| 三种 Daily 模式防前视 | test_three_daily_amount_rules_no_lookahead | 同上 |

## 测试结果
- 回归套件 + funds 模块：106 通过
- 全量 767 测试通过（2026-08-03，本地）
- ruff/mypy: 通过

## 关键决策
- `_usableHistory` 以 availableNav 为锚而非信任 navHistory 尾部
- 当前净值不在历史中时全部历史视为可用（兼容既有调用）
- 防前视是硬性验收：发现缺陷即修复，不绕过测试
