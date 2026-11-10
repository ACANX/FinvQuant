# 提交前检查规范（CommitCheckSpec）

> 所属：FinvQuant 开发规范 · 存放：`Docs/DevSpec/`
> 适用范围：任何进入 Git 的改动（Go、`Web/`、`Deploy/Migrations/`、`Docs/`）。
> 关联：[`MergeCheckSpec.md`](MergeCheckSpec.md)（合并前复核）、[`GitSpec.md`](GitSpec.md)（提交信息与 CHANGELOG 联动）、[`DocSpec.md`](DocSpec.md) §4、[`DbSchemaSpec.md`](DbSchemaSpec.md) §0（改库红线）、[`FileNamingSpec.md`](FileNamingSpec.md) §4。
> 本清单的定位：**提交者自检**。合并前由他人复核的清单见 `MergeCheckSpec.md`。

## 1. 用法

1. 提交前逐条过；**不适用就写 `不适用：<原因>`**，不留空、不跳过。
2. §2 的一键自检先跑到全绿，再过清单。
3. 条目编号 `C1…Cn` 稳定，可在提交正文引用：`CommitCheck: C1–C7, C12`。
4. 一次提交只装一件事（用户点名的那件）。顺手改动一律拆到另一次提交。
5. 一条不勾 → 不提交。没有"后面补"。

## 2. 一键自检

```bash
# 后端：编译、静态检查、文档测试（含 OpenAPI 与 Markdown 表格校验）
go build ./... && go vet ./...
go test ./internal/docs/ ./internal/api/... -count=1

# 前端（改动 Web/ 时必跑）
cd Web && npx vue-tsc -b --force && npm run build && cd ..

# 改动范围自查：有没有把不该提交的东西带进来
git status --short && git diff --stat
```

| 命令 | 覆盖条目 |
|------|----------|
| `go build ./... && go vet ./...` | C8 |
| `go test ./internal/docs/` | C9、C18 |
| `go test ./internal/api/...` | C9 |
| `npx vue-tsc -b --force && npm run build` | C8、C21 |
| `git status --short` / `git diff --stat` | C1、C2、C3 |

## 3. 检查清单

### A. 范围与授权

| 编号 | 检查项 | 判据 |
|------|--------|------|
| C1 | 改动只覆盖本次任务点名的内容 | `git diff --stat` 的文件都能对应到任务描述；出现无关文件即拆出去 |
| C2 | 没有触碰受保护项（未经点名时） | 未改 `AGENTS.md`、`Docs/DevSpec/*`、`Docs/RequirementList.md`、`Docs/CHANGELOG.md` 以外的越界文件 |
| C3 | 没有未跟踪的调试产物混入 | `git status --short` 无 `*.log`、`tmp/`、临时脚本、`.bat` 探针 |
| C4 | 新增/删除/改名文件已获许可 | 任务里明确写过；`git status` 的 `D`/`R` 项都能指出授权来源 |
| C5 | 需求编号已登记（功能/行为/规范变更） | `Docs/RequirementList.md` 有新编号；纯勘误可豁免并写原因 |

### B. 编译与测试

| 编号 | 检查项 | 判据 |
|------|--------|------|
| C6 | 未使用 `--no-verify` 跳过钩子 | `git log -1 --stat` 与钩子输出可见；无跳过记录 |
| C7 | 改动不遗留 `TODO`/注释掉的死代码/调试打印 | `git diff` 中搜索 `TODO`、`fmt.Println`、`console.log` 为零或已说明原因 |
| C8 | 编译与类型检查零错误 | §2 命令退出码 0 |
| C9 | 受影响包的测试全绿 | §2 命令退出码 0；失败项无 `t.Skip` 掩盖 |
| C10 | 新增逻辑有对应测试或明确不测理由 | `git diff` 里有 `_test.go` / `*.spec.ts`，或提交正文写明"无测试：<原因>" |

### C. 数据库迁移（改 `Deploy/Migrations/` 时必过）

| 编号 | 检查项 | 判据 |
|------|--------|------|
| C11 | 只新增迁移，不改历史迁移 | `git diff --stat` 里 `V*` 文件只有新增，未修改既有脚本 |
| C12 | 迁移单事务、幂等、`IF EXISTS`/`ON CONFLICT` 齐备 | 重复执行不报错；种子数据一律 `ON CONFLICT DO NOTHING` |
| C13 | 收敛校验只用结构性判据 | 结尾断言不依赖行数/阈值；数据质量计数一律 `RAISE NOTICE` |
| C14 | 全新空库可跑通 | `TestMigrateIntegration` 或空库实测通过（`finv_migtest`） |
| C15 | 无新增物理外键 | 迁移后 `fk = 0`（`DbSchemaSpec` 跨域引用只走程序层） |
| C16 | 未写入生成列、未擅自删表/改名/合并表 | 迁移中无 `GENERATED` 列赋值；`DROP/RENAME/SET SCHEMA` 能指出用户授权原话 |

### D. 接口与文档

| 编号 | 检查项 | 判据 |
|------|--------|------|
| C17 | 接口行为变更同步 `.md` + `.yml` | `Docs/API/` 中两份都改；状态码、参数、字段一致 |
| C18 | 新增接口在路由与文档双向可达 | `internal/docs` 测试通过；`grep` 路由前缀能在 yml 找到 |
| C19 | 菜单文档与页面一致（改前端页面时） | `Docs/Menu/` 对应文档的列/筛选项/操作与实现一致 |
| C20 | `Docs/CHANGELOG.md` 已按 `新增/变更/修复` 记录 | 同一次提交内；条目写清"使用者能感知什么" |

### E. 前端（改 `Web/` 时必过）

| 编号 | 检查项 | 判据 |
|------|--------|------|
| C21 | 构建产物正常且页面可加载 | `npm run build` 通过；无 TS 报错；未改 `vite.config` 以外的构建参数 |
| C22 | 枚举展示符合规范 | 筛选控件 `中文[值]`（`DictFilterSpec`）、表格列仅中文（`DictColumnSpec`）、长文本 `ⓘ` 悬浮（`TableTextSpec`） |
| C23 | 空值/未登记值有兜底 | `LABELS[v] ?? v`、空值 `—`；不出现 `null`/`undefined` |
| C24 | 未把密钥/令牌写进前端或注释 | `grep -riE 'sk-\|secret\|password\|Bearer ' Web/src` 无命中 |
| C25 | 静态说明收进页面底部唯一「说明」卡片 | 无标题下说明、无表格间夹注（`ExplainCardSpec`） |

### F. 提交本身

| 编号 | 检查项 | 判据 |
|------|--------|------|
| C26 | 提交信息符合 `GitSpec`（`<type>: 简体中文摘要`） | `git log -1 --pretty=%s` 匹配前缀 |
| C27 | 提交正文写清"改了什么、为什么、影响面" | 正文可独立读懂，无须回看对话 |
| C28 | 未推送前确认没有夹带他人/无关改动 | `git show --stat HEAD` 与预期一致 |

## 4. 红线（命中任一，禁止提交）

- 未获明确授权就 `DROP/RENAME/合并` 表，或改动受保护文档；
- 明文密钥、令牌、连接串（含注释与示例）进入仓库；
- 显式写入数据库生成列（`usc_lower`、`ftmm_symbol` 等）；
- 用 `t.Skip` / 注释断言 / 删测试的方式让测试变绿；
- `git commit --no-verify`、`git push --force`（无授权）；
- 把不属于本仓库的文件（本地脚本、`.env`、备份）提交进去。

## 5. 豁免写法

| 场景 | 可豁免的组 | 必须写 |
|------|-----------|--------|
| 纯文档/注释勘误 | B、C、E | `不适用：纯文档` |
| 只改 `Docs/`（无代码） | B、C、E | `不适用：无代码变更` |
| 只改 Go 后端 | E | `不适用：未改前端` |

豁免是"跳过并写明"，不是"静默忽略"。§4 红线不可豁免。

## 6. 新增/修改条目的规则

1. **来源必须有据**：要么对应一条真实事故（写需求号或提交号），要么对应既有规范条款；不接受"感觉应该检查"。
2. **判据必须可核对**：能被命令、`grep`、观察结果证伪。禁止"代码质量良好""注意兼容性"这类无法判定的条目。
3. **一条只查一件事**，编号只增不改；作废的条目保留编号并标注 `（已废止：原因）`。
4. 新增条目同步 `Docs/CHANGELOG.md`，并在 `MergeCheckSpec` 里检查对应项是否已有覆盖，避免两处重复。
