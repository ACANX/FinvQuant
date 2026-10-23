# P2-005 AccountRiskSnapshot 屏障与组合只读评估 — 实现证据

- **PlanTaskId：** P2-005
- **里程碑：** M2A
- **作者：** BeeAgent
- **状态：** ACCEPTED（PR #219/#220/#221 已合并）
- **日期：** 2026-08-02

## 1. 实现内容

| 工件 | 路径 | 说明 |
| --- | --- | --- |
| 组合快照 | `src/veritasquant/risk/PortfolioSnapshot.py` | `AccountRiskSnapshotV1`（barrier_event_id、逻辑 ts、账本/订单/控制版本、内容哈希）；`PortfolioSnapshotSetV1`（同屏障收齐、账户唯一）；`PortfolioSnapshotRegistryV1`（登记 + `tryAssemble` 屏障组装） |
| 单元测试 | `tests/unit/risk/test_portfolio_snapshot.py` | 12 用例 |

## 2. 验收标准映射

| 验收标准 | 证据 |
| --- | --- |
| 不齐或不同 barrier 的快照不能拼接 | `test_missing_account_returns_none_keeps_tight_control`、`test_stale_barrier_returns_none` |
| 缺失时保持更严格控制 | `tryAssemble` 缺任一账户或屏障不一致时返回 None（调用方维持上一条更严格控制并禁止新增风险） |
| 组合评估器只读 | Registry 仅登记/组装，无任何账户写接口 |

## 3. 设计要点

- 快照内容哈希覆盖屏障与全部版本字段；同账户同屏障内容冲突拒绝登记。
- 组装必须是"全部目标账户同一 barrier"的完整集合，绝不用新旧快照拼接。
- 评估器只读；最终控制仍由各分区 RiskEngine 幂等发布（TechSpec 3.3）。

## 4. 测试证据

```text
$ .venv/bin/python -m pytest tests/unit/ -q   # 552 passed
$ .venv/bin/ruff check src tests              # All checks passed!
```

## 5. 证据索引

- `src/veritasquant/risk/PortfolioSnapshot.py`
- `tests/unit/risk/test_portfolio_snapshot.py`
- 本文件 `Docs/DevelopmentWorkflow/P2/P2-005RiskSnapshotEvidence.md`
