# P2-032 数据导入、策略、定投计划和回测操作页证据

## 任务信息
- **PlanTaskId:** P2-032
- **标题:** 实现数据导入、策略、定投计划和回测操作页
- **阶段/里程碑:** M2A 模拟盘与基金能力建设
- **状态:** IN_REVIEW（等待第五批 PR 合并后验收）

## 实现内容（`apps/guiclient/Pages.py`）

### 数据导入页
- ImportRequest 表单模型 + 校验（数据源/标的/日期区间/导入模式）
- submitImport()：提交 DATA_IMPORT 命令（202 受理返回命令引用）
- 危险操作确认：勾选"确认导入将创建新数据版本"才提交

### 策略管理页
- 策略列表 + 新建草稿（DSL/PYTHON 二选一）
- validateDsl()：YAML 解析、PlanType 六类校验、FundScope 必需字段
- "校验 DSL" 即时反馈；保存走命令流程

### 定投计划页
- PlanDraft 表单模型：名称/基金/周期/金额模式/基础金额/资金来源
- 校验：周期三值、金额模式三值、正数金额、资金来源二值

### 回测中心页
- 回测列表 + 运行控制（启动/取消需二次确认勾选）
- BacktestRequest：策略/账户/日期/初始资金/IDEAL-REALISTIC 模式
- 创建回测 202 返回 run_id

## 验收标准映射
| 验收标准 | 实现 | 证据 |
| --- | --- | --- |
| 创建/校验/运行/暂停/查询主流程可用 | 表单 + API 编排 | test_create_start_cancel_flow |
| 危险操作有明确确认和状态反馈 | checkbox 确认 + success/error | test_inverted_date_range |

## 测试结果
- `tests/unit/apps/guiclient/test_pages.py`：29 个测试通过
- 全量 918 测试通过（2026-08-03，本地）
- ruff/mypy: 通过

## 关键决策
- 页面逻辑（校验/提交）与渲染解耦，可单元测试
- 所有操作走命令流程（202 受理），不直接改领域状态
