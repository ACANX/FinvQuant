# 提交前检查规范（CommitPreCheckListSpec）

> 所属：FinvQuant 开发规范 · 存放：`Docs/CheckList/`
> 适用范围：任何进入 Git 的改动（Go、`Web/`、`Deploy/Migrations/`、`Docs/`）。
> 关联：[`MergeCommitPreCheckListSpec.md`](MergeCommitPreCheckListSpec.md)（合并前复核）、[`GitSpec.md`](../DevSpec/GitSpec.md)（提交信息与 CHANGELOG 联动）、[`DocSpec.md`](../DevSpec/DocSpec.md) §4、[`DbSchemaSpec.md`](../DevSpec/DbSchemaSpec.md) §0（改库红线）、[`FileNamingSpec.md`](../DevSpec/FileNamingSpec.md) §4。
> 本清单的定位：**提交者自检**。合并前由他人复核的清单见 `MergeCommitPreCheckListSpec.md`。

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

## 3. 事故防守（先过这一节）

本项目真实踩过的坑，按「事故 → 判据」登记。编号 `R1…Rn` 只增不改；每条都对应过至少一次返工或一次线上报错。
**看到自己要碰的东西命中某条，就按判据验一遍，不看"感觉没问题"。**

| 编号 | 事故（真实发生过） | 防守判据 |
|------|--------------------|----------|
| R1 | 删除类清洗迁移在全新空库上误删种子（键表为空，`EXISTS` 恒假） | 删除/清洗语句带「源表非空」守卫；空库跑一遍确认种子仍在 |
| R2 | 数据修复迁移只改标记列就断言目标列，远端仍是旧值 → 校验失败、服务重启循环 | 修复类迁移按业务键（`ftmm_symbol` / 主键）直接写入目标列；断言与写入同口径 |
| R3 | 收敛校验用绝对行数阈值，把"刻意保留"的行判成未清理 → 迁移中止 | 只断言"计划删除集已清空"这类结构性事实；数据质量计数一律 `RAISE NOTICE` |
| R4 | 迁移残留物理外键，与"跨域引用只走程序层"冲突 | 迁移后 `fk = 0`；新增约束先确认不是 FK |
| R5 | `INSERT … ON CONFLICT (idn) DO UPDATE`，但 INSERT 列不含 `idn` → 编辑必然撞自然键 23505 | 有 `ON CONFLICT (col)` 就核对 `col` 在 INSERT 列表里；更新走显式 `UPDATE … WHERE 主键` |
| R6 | 外部 REST 过滤值未做百分号编码，`&` / `,` 被 URL 层截断 → HTTP 400（只在后面的批次遇到特殊字符键才炸） | 拼进 URL 的每个值都编码；除个数上限外按"字符预算"分片；用例覆盖 `&`、`,`、`*`、空格 |
| R7 | 显式写入数据库生成列（`usc_lower`、`ftmm_symbol`） | 迁移与代码的 INSERT/UPDATE 列表中不出现生成列 |
| R8 | 迁移种子用 `DO UPDATE`，覆盖用户后来改过的值 | 种子一律 `ON CONFLICT DO NOTHING` |
| R9 | 文档表格竖线未转义、表内插空行拆表 → `TestMarkdownTablesHaveHeader` 失败 | 表格内竖线写 `\|`；表格中间不留空行、不插引用块 |
| R10 | 枚举 `中文[值]` 从"下拉选项"溢出到"表格列"，返工两次 | 下拉 `中文[值]`（`DictFilterSpec`）、表格仅中文（`DictColumnSpec`）；单元格不读下拉的 `title` |
| R11 | 命名踩线：时间列漏 `dt_` 前缀、字段用 `-ed` 过去分词 | 新增列/字段先过 `DbSchemaSpec` 命名段；`dt_` + 动词原形/名词 |
| R12 | WSL 下 `psql -f D:\...` 找不到文件；迁移 SQL 注释含反引号破坏 Go raw string | 执行用 `docker exec -i psql < file`；Go 内嵌 SQL 注释不写反引号 |
| R13 | 迁移里写错 schema（表在 `futu` 却写 `cdeu`）、用不存在的函数，跑到一半才炸 | 迁移先在克隆库/空库跑一次；表名连 schema 一起核对 |
| R14 | 提交夹带未跟踪的调试产物（临时目录、`*.bat` 探针、临时 SQL） | `git status --short` 逐项确认来源；无关的一律不加 |
| R15 | 容器构建默认 `proxy.golang.org` 不可达，`--pull` 因镜像源 DNS 失败 | 构建加 `--build-arg GOPROXY=https://goproxy.cn,direct`；不用 `--pull` |
| R16 | 只改代码不同步迁移目录 / 接口文档，远端版本与服务端不一致 | 发布相关改动同时同步 `Deploy/Migrations`、`Docs/API`、`Docs/Menu` |
| R17 | `v-combobox` 默认 `return-object=true`（Vuetify 4.1），从对象型下拉选中后 `v-model` 拿到的是整个 `{title,value}` 对象；保存前 `String(obj)` 落库成 `[object Object]` | 对象型 items 的 `v-combobox` 一律写 `item-title` / `item-value` / `:return-object="false"`；提交前用「对象取 `value` 再取 `title`」的归一函数兜底，不用裸 `String(v)` |

## 4. 按变更类型取最小必查集

先定位自己属于哪一类，再看该类专项清单；交叉变更取并集。
**本文件只保留通用条目（C 组）与事故表（R 组）**，各类特有项各自一份清单，避免一份文件越写越厚。

| 变更类型 | 专项清单 | 另需的通用条目 |
|----------|----------|----------------|
| 前端页面/组件/样式 | [`FrontendCommitPreCheckListSpec.md`](FrontendCommitPreCheckListSpec.md) | C1–C3、C8、C20 |
| 后端逻辑/定时任务/外部客户端 | [`BackendCommitPreCheckListSpec.md`](BackendCommitPreCheckListSpec.md) | C1–C3、C6–C10 |
| 表结构/数据/迁移 | [`DbChangeCommitPreCheckListSpec.md`](DbChangeCommitPreCheckListSpec.md) | C1–C3、C9–C10 |
| 动手前的方案/选型（要拍板） | [`ProposalDocCommitPreCheckListSpec.md`](ProposalDocCommitPreCheckListSpec.md) | C1–C5、C20 |
| 已批准后的技术设计 | [`TechDesignDocCommitPreCheckListSpec.md`](TechDesignDocCommitPreCheckListSpec.md) | C17–C20 |
| 不改行为的结构调整 | [`CodeRefactorCommitPreCheckListSpec.md`](CodeRefactorCommitPreCheckListSpec.md) | C1–C3、C6–C10 |
| 部署/镜像/脚本 | 共用 C 组 + [`DbChangeCommitPreCheckListSpec.md`](DbChangeCommitPreCheckListSpec.md) 的验证段 | C8、C21、R15、R16 |
| 混合改动 | 取涉及类型并集 | 仍要求「一次提交只装一件事」 |

## 5. 通用检查清单

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

## 6. 红线（命中任一，禁止提交）

- 未获明确授权就 `DROP/RENAME/合并` 表，或改动受保护文档；
- 明文密钥、令牌、连接串（含注释与示例）进入仓库；
- 显式写入数据库生成列（`usc_lower`、`ftmm_symbol` 等）；
- 用 `t.Skip` / 注释断言 / 删测试的方式让测试变绿；
- `git commit --no-verify`、`git push --force`（无授权）；
- 把不属于本仓库的文件（本地脚本、`.env`、备份）提交进去。

## 7. 豁免写法

| 场景 | 可豁免的组 | 必须写 |
|------|-----------|--------|
| 纯文档/注释勘误 | B、C、E | `不适用：纯文档` |
| 只改 `Docs/`（无代码） | B、C、E | `不适用：无代码变更` |
| 只改 Go 后端 | E | `不适用：未改前端` |

豁免是"跳过并写明"，不是"静默忽略"。§6 红线不可豁免。

## 8. 新增/修改条目的规则

1. **来源必须有据**：要么对应一条真实事故（写需求号或提交号），要么对应既有规范条款；不接受"感觉应该检查"。
2. **判据必须可核对**：能被命令、`grep`、观察结果证伪。禁止"代码质量良好""注意兼容性"这类无法判定的条目。
3. **一条只查一件事**，编号只增不改；作废的条目保留编号并标注 `（已废止：原因）`。
4. 新增条目同步 `Docs/CHANGELOG.md`，并在 `MergeCommitPreCheckListSpec` 里检查对应项是否已有覆盖，避免两处重复。
