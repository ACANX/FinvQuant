# P1-021 质量规则、隔离记录与 dry-run 摘要验证证据

实现导入提交前执行的时间顺序、重复、缺口、OHLC、会话、标的映射与来源质量规则。
失败记录进入隔离集合（`IsolationRecordV1`），生成可审阅的 `DryRunSummaryV1`
（config/input/contract 哈希与计数），严禁静默跳过或修正无效数据。

## 实现与测试

- 实现：`src/veritasquant/data/QualityRules.py`
  - `QualityRuleEngineV1`：7 类固定规则按序执行，失败自动隔离
  - `IsolationRecordV1`：RuleKind/Severity/主键/来源记录/原因
  - `DryRunSummaryV1`：提交前可审阅摘要，从全新状态执行保证幂等
  - `QualityRuleConfigV1`：规则版本化，变更必须产生新 `QualityRuleVersion`
- 测试：`tests/unit/data/test_quality_rules.py`

## 验证结果

```powershell
python3 -m pytest tests\unit\data\test_quality_rules.py -q
# 9 passed
```

## 验收标准映射

| 验收标准 | 证据 |
| --- | --- |
| 时间顺序失败不静默跳过 | `test_time_order_reversal_is_isolated_not_silently_fixed` |
| 重复失败不静默跳过 | `test_duplicate_primary_key_is_isolated` |
| 缺口失败不静默跳过 | `test_gap_over_threshold_is_isolated` |
| 会话失败不静默跳过 | `test_session_mismatch_and_unverified_session_are_isolated` |
| 映射失败不静默跳过 | `test_instrument_mapping_mismatch_is_isolated` |
| 提交前可审阅文件/配置/契约哈希 | `test_dry_run_is_reviewable_and_deterministic`（configHash/inputFileHash/contractHash 与隔离哈希） |

## 关键决策

- 规则引擎只读、绝不修正数据；失败记录进入隔离集合而非静默丢弃。
- `dryRun` 每次从全新状态执行，保证重复调用结果一致（不依赖调用顺序）。
- 会话规则通过注入的 `sessionIds` 白名单校验；未注入时不启用该规则，避免
  与标的注册表耦合。

## 残余风险

- 会话规则目前为白名单注入式；接入真实导入管线时需由标的注册表派生白名单。
