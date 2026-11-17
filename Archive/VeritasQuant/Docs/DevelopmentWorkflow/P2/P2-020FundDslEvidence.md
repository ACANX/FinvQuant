# P2-020 受限基金 DSL — 实现证据

- **PlanTaskId：** P2-020 | **里程碑：** M2A | **作者：** BeeAgent | **状态：** ACCEPTED（PR #219/#220/#221 已合并）

## 1. 实现内容
| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 受限 DSL | `src/veritasquant/funds/FundDsl.py` | 仅 ast 语法分析（禁 eval/exec）；白名单变量（nav/nav_prev/date/amount/budget/balance/nav_ma/drawdown）与函数（abs/min/max/round/percentile/avg/clamp）；类型检查；注册业务码 DSL-1001~1004 |
| 测试 | `tests/unit/funds/test_fund_dsl.py`（13） | |

## 2. 验收标准映射
| 标准 | 证据 |
| --- | --- |
| 仅白名单表达式/动作 | `test_whitelist_functions`、运算符/比较/逻辑白名单 |
| 禁止 eval、文件、网络和未来变量 | `test_unknown_function_rejected`（eval）、`test_unknown_variable_rejected`（nav_future）、`test_attribute_access_rejected`（__class__）、`test_subscript_rejected`、`test_import_rejected`、`test_lambda_rejected` |
| 错误返回注册业务码 | `test_syntax_error_maps_to_registered_code`（DSL-1001）、`test_division_by_zero_type_error`（DSL-1003） |

## 3. 证据索引
- `src/veritasquant/funds/FundDsl.py`、`tests/unit/funds/test_fund_dsl.py`
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-020FundDslEvidence.md`
