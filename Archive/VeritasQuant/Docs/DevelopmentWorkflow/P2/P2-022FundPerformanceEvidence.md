# P2-022 基金业绩报告证据

## 任务信息
- **PlanTaskId:** P2-022
- **标题:** 实现 TWR、XIRR、投入本金、份额和规则贡献报告
- **阶段/里程碑:** M2A 模拟盘与基金能力建设
- **状态:** IN_REVIEW（等待第四批 PR 合并后验收）

## 实现内容

### TwrCalculatorV1（时间加权收益率）
- 现金流分段几何连乘：TWR = Π(期末值 / (期初值 + 期间净投入)) - 1
- 期初/期末价值不得为负；分段基数非零校验
- 结果按 ROUND_HALF_EVEN 量化到 8 位小数

### XirrCalculatorV1（内部收益率）
- 二分法迭代求解 NPV=0；默认最多 100 次迭代，容差 1e-10
- 至少需要两笔现金流，且同时包含投入（负）与回收（正）
- 按天数比例年化；结果量化到 6 位小数

### PrincipalReporterV1（投入本金）
- 复用 DepositLedgerV1 累计投入；现金流独立于收益计算

### RuleContributionReporterV1（规则贡献）
- 记录各金额规则/方案的投入与份额；拒绝负值
- report() 返回不可变元组；totalInvested 汇总

## 验收标准映射
| 验收标准 | 实现 | 证据 |
| --- | --- | --- |
| 与手算样本一致 | TWR/XIRR 公式与量化 | tests/unit/funds/test_fund_performance.py |
| 现金流不计收益 | PrincipalReporterV1 独立于收益 | 同上 |
| 固定定额基线与敏感性可追溯 | RuleContributionReporterV1 | 同上 |

## 测试结果
- 文件: `tests/unit/funds/test_fund_performance.py`
- 结果: 全量 767 测试通过（2026-08-03，本地）
- ruff/mypy: 通过

## 关键决策
- Decimal 全程精确计算，禁止 float 参与金额
- 现金流符号约定：负为投入，正为赎回/分红
