# P2-035 数据导入、对账、校准和报告任务清单证据

## 任务信息
- **PlanTaskId:** P2-035
- **标题:** 实现数据导入、对账、校准和报告任务清单
- **阶段/里程碑:** M2A 模拟盘与基金能力建设
- **状态:** IN_REVIEW（等待第五批 PR 合并后验收）

## 实现内容

### 任务执行器（`application/JobTasks.py`）
- 四类任务统一用例：
  - **DataImportTask**：source/instrument_id 校验、checkpoint 生成
  - **ReconciliationTask**：account_group 校验、账本对账差异统计
  - **ExecutionCalibrationTask**：model_version 校验、校准 checkpoint
  - **ReportGenerationTask**：report_type 校验（复用 P2-022 报告体系）
- **幂等**：执行键 SHA-256 哈希，recorder 去重（重复 → 4 幂等跳过）
- **退出码**：0 成功 / 2 参数无效 / 3 业务失败 / 4 幂等跳过
- **结构化日志**：veritasquant.jobs logger

### 任务入口（`src/veritasquant/jobs/`）
- 4 个 vq-job-* 全部接入 JobEntrypoint（job_run_id + job_execution_key +
  参数 Schema 版本校验），复用 runTask 分派

### 部署清单（`Jobs/JobManifests.yml`）
- 4 条 PascalCase 完整清单（TechSpec 11.5 必填字段全含）：
  每日数据导入、每日对账、每周校准、每周报告

## 验收标准映射
| 验收标准 | 实现 | 证据 |
| --- | --- | --- |
| 根级 Jobs 仅含 PascalCase Yml 清单 | JobManifests.yml 契约测试 | test_manifest_is_pascal_case_yaml |
| 每个命令幂等 | 执行键哈希去重 | test_idempotent_skip |
| 每个命令有日志和退出码 | logger + 退出码语义 | test_data_ingestion_job_flow |

## 测试结果
- `tests/unit/jobs/test_job_tasks.py`：22 个测试通过
- 真实冒烟：vq-job-account-reconciliation 退出 0
- 清单解析验证：4 条计划正确加载
- 全量 918 测试通过（2026-08-03，本地）
- ruff/mypy: 通过

## 关键决策
- 任务不依赖常驻 API 进程内存（TechSpec 11.5）
- 业务幂等由执行键 + command_id/inbox/outbox/checkpoint 多层保证
