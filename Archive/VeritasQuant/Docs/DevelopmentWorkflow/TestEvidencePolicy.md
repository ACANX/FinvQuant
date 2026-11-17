# 测试与证据规范

## 稳定测试 ID

测试通过 `@pytest.mark.stable_id("<ID>")` 绑定稳定标识。ID 不得因文件移动、参数化顺序或显示名称变化而重用；追踪矩阵使用该 ID 而不是临时的 pytest node id。新行为至少提供正例、反例和边界测试。

## 每次 CI 证据

`scripts/CollectTestEvidence.py` 将 JUnit 汇总与 SHA-256、coverage SHA-256、测试选择、随机种子、UTC 收集时间、运行器 OS/架构/Python 版本以及每个构建工件的 SHA-256 写入 snake_case JSON。CI 上传 JUnit、coverage、证据 JSON 和 `dist/`，保存期为 90 天。

固定种子属性或模型测试必须显式传入 `--seed`；无随机行为的测试使用 `not_applicable`。测试跳过、JUnit 失败、缺任一输入文件或哈希不匹配均不能作为通过证据。运行、环境与证据关联的工作项 ID 必须写入 `--work-item`。

## 示例

```powershell
python3 -m coverage run -m pytest tests\unit tests\contract tests\packaging --junitxml artifacts\JUnit.xml
python3 -m coverage xml -o artifacts\Coverage.xml
python3 scripts\CollectTestEvidence.py --junit artifacts\JUnit.xml --coverage artifacts\Coverage.xml --artifact dist\veritasquant-0.1.0-py3-none-any.whl --work-item P0-009 --seed not_applicable --output artifacts\TestEvidence.json
```

该规范只定义作者和 CI 证据格式；独立 QA 验证、验收人签署与 Gate 结论仍按开发工作流执行。

## 2026-08-01 独立复核快照

- 复核时间：`2026-08-01T20:22:00Z`。
- 复核对象：`scripts/CollectTestEvidence.py`、`tests/unit/scripts/test_engineering_scripts.py`、`Docs/DevelopmentWorkflow/TestEvidencePolicy.md` 与 `Docs/DevelopmentWorkflow/P0-003-P0-012TestEvidence.json`。
- 稳定测试 ID：测试通过 `@pytest.mark.stable_id("<ID>")` 绑定，追踪矩阵引用稳定 ID 而非临时 node id；仓库内共 11 处使用。
- `CollectTestEvidence.py` 端到端验证：在 Python 3.14（Linux）下运行 `pytest`（61 passed）生成 JUnit、coverage，随后收集器输出 snake_case JSON，包含 JUnit 汇总、coverage/工件 SHA-256（64 字符）、环境（OS/架构/Python）、UTC 收集时间、种子（`not_applicable`）与工作项 ID；输入缺失时返回非零退出码。
- CI（`.github/workflows/Ci.yml`）在单元/契约/打包测试后执行 `CollectTestEvidence.py` 并上传 `artifacts/`（保留 90 天）；证据 JSON 中登记的工作项覆盖 P0-004~P0-012。

本快照证明 P0-009 验收标准（示例测试可生成 JUnit、覆盖率、种子、环境和产物 SHA-256）的实现证据与 CI 配置相符。独立人类 QA 对远程 CI 原始工件的复核仍作为 M0 Gate 前的治理行动保留，不替代或改变本工作项的验收。
