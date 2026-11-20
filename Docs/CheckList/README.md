# Docs/CheckList — 交付检查清单

> 用途：存放**交付环节的检查清单**（提交前自检、合并前复核）与**按工作类型的专项清单**。
> 用法核心一条：**按变更类型取最小必查集**，不是每份都从头过一遍。
> 规范本体仍在 [`Docs/DevSpec/`](../DevSpec/)，本目录只装"怎么查、查什么、判据是什么"。

## 1. 目录解决什么问题

- 同一件事（如 `ON CONFLICT` 目标列、`中文[值]` 只进下拉）在不同会话被反复踩 —— 把真实事故固化成可核对判据；
- 改前端和改迁移要看的东西完全不同 —— 拆成专项清单，改什么查什么；
- "我验证过了"无法复核 —— 每份清单都要求给命令输出、数字、产物哈希，而不是结论。

## 2. 清单一览

| 文件 | 管什么 | 什么时候用 | 条目编号 |
|------|--------|------------|----------|
| [`CommitPreCheckListSpec.md`](CommitPreCheckListSpec.md) | **入口**：通用自检项 + 事故防守表 + 按变更类型的路由 | 每次 `git commit` 前 | C1–C28、R1–R16 |
| [`MergeCommitPreCheckListSpec.md`](MergeCommitPreCheckListSpec.md) | 合并前复核与功能验收（复核人视角） | PR 合并前 | M1–M6 |
| [`FrontendCommitPreCheckListSpec.md`](FrontendCommitPreCheckListSpec.md) | 前端页面/组件/样式 | 改 `Web/` | FE1–FE15 |
| [`BackendCommitPreCheckListSpec.md`](BackendCommitPreCheckListSpec.md) | 后端逻辑/定时任务/外部客户端 | 改 `cmd/`、`internal/` | BE1–BE16 |
| [`DbChangeCommitPreCheckListSpec.md`](DbChangeCommitPreCheckListSpec.md) | 表结构、数据修复、迁移、清洗 | 改 `Deploy/Migrations/` | DB1–DB20 |
| [`ProposalDocCommitPreCheckListSpec.md`](ProposalDocCommitPreCheckListSpec.md) | 动手前的提案（要拍板） | 方案/选型/跨模块改动之前 | PP1–PP10 |
| [`TechDesignDocCommitPreCheckListSpec.md`](TechDesignDocCommitPreCheckListSpec.md) | 已批准后的技术方案设计 | 提案批准、开始实现之前 | TD1–TD13 |
| [`CodeRefactorCommitPreCheckListSpec.md`](CodeRefactorCommitPreCheckListSpec.md) | 不改行为的结构调整 | 重构 | RF1–RF13 |

## 3. 使用流程

1. **定类型**：看 `CommitPreCheckListSpec.md` §4 路由表，确定自己属于哪一类（混合改动取并集）。
2. **过专项**：打开对应专项清单，逐条看判据；不适用写 `不适用：<原因>`，不留空。
3. **过通用**：按路由表补查通用条目（范围授权 C1–C5、编译测试 C6–C10、提交纪律 C26–C28）。
4. **合并前**：提交完成后由**非提交者**按 `MergeCommitPreCheckListSpec.md` 逐条复核，结论只有"通过 / 打回（附 `M` 编号）"。

## 4. 命名规则

`<领域>[Doc]Commit` + `Pre` + `CheckListSpec.md`

| 段 | 含义 | 例子 |
|----|------|------|
| `<领域>` | 变更类型或产出物类别 | `Frontend`、`DbChange`、`CodeRefactor` |
| `Doc` | 该清单评的是**产出一份文档**（可省略） | `ProposalDoc`、`TechDesignDoc` |
| `Commit` | 该清单服务于提交/复核环节 | — |
| `Pre` | 紧贴后缀，表明这是"提交前/合并前"检查清单 | — |
| `CheckListSpec.md` | 固定后缀 | — |

新增清单沿用同一规则，领域段前置便于按文件名排序与检索。

## 5. 通用约定（所有清单共同遵守）

- **编号只增不改**：作废条目保留编号并标注 `（已废止：原因）`，不复用、不重排。
- **判据必须可证伪**：能通过命令、`grep`、观察结果判定。禁止"代码质量良好""注意兼容性"这类无法验收的条目。
- **一条只查一件事**，一条对应一个判据。
- **来源必须有据**：新增条目要么挂真实事故（需求号/提交号），要么挂既有规范条款。
- **不与其它清单重复**：提交者能自查的不进合并清单；通用项只写在 `CommitPreCheckListSpec.md`。
- **不拆前后端两份总清单**：清单按检查点组织、按变更类型路由，避免两份口径漂移。

## 6. 新增或修改清单

1. 先判断是**新条目**还是**新清单**：多数情况是给现有专项清单加一条，并同时在 `CommitPreCheckListSpec.md` §3 事故表登记 `R` 编号。
2. 改完跑校验：

   ```bash
   go test ./internal/docs/ -count=1          # 文档表格合法性
   grep -rn "旧文件名" --include=*.md .        # 改名时确认无残留
   ```

3. 同步三处：`AGENTS.md`「交付检查清单」索引、`Docs/RequirementList.md` 新编号、`Docs/CHANGELOG.md` 变更条目。
4. 改名/移动一律 `git mv`，并更新本 README 的清单表与所有交叉引用。

## 7. 与其它文档的关系

| 文档 | 关系 |
|------|------|
| [`Docs/DevSpec/`](../DevSpec/) | 规则本体（`ApiSpec`、`DbSchemaSpec` §0 红线、`UiSpec`、字典与表格展示规范等），清单只引用不复述 |
| [`Docs/Proposal/`](../Proposal/) | 提案清单评的产出物落在这里 |
| [`Docs/Design/`](../Design/) | 技术方案设计清单评的产出物落在这里 |
| [`Docs/Refactor/`](../Refactor/) | 重构步骤清单落在这里（带 `yyMM-NNN-` 编号） |
| [`Docs/CHANGELOG.md`](../CHANGELOG.md) | 清单本身的变更也必须记录 |
