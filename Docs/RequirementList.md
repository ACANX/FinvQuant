# 需求清单（RequirementList）

> 用途：记录系统中**自 `2026-10-24` 起**新增的需求，拆成不可再分、可单独核对的需求点，供后续逐条验收。
> 编号：从 `10001` 起升序（五位数），**只增不改**；新增需求一律从当前最大编号往后追加，不插入既有区间。
> 写法：每条只描述一个要求，不使用「并且」式并列；「核对方式」列给出可执行命令或明确的人工判定点。
> 结果：验收时填 `√`（满足）或 `×`（不满足，并在括注中简述差异）；未验收留空。
> 执行前提：核对命令均在仓库根目录（`FinvQuant/`）下执行；表格内的 `|` 以 `\|` 转义。
> 关联规范：[`DevSpec/DocSpec.md`](DevSpec/DocSpec.md) §4（变更日志）、[`DevSpec/GitSpec.md`](DevSpec/GitSpec.md) §3（提交联动）、[`DevSpec/FileEncodingSpec.md`](DevSpec/FileEncodingSpec.md) §4（换行符）、[`DevSpec/FileNamingSpec.md`](DevSpec/FileNamingSpec.md) §1（全大写文件名例外）。

## 一、变更日志文档（10001–10015）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10001 | 变更日志文件位于 `Docs/CHANGELOG.md`。 | `test -f Docs/CHANGELOG.md` 退出码为 0 | |
| 10002 | 变更日志文件名为全大写 `CHANGELOG.md`。 | `git ls-files Docs` 中匹配 `^Docs/CHANGELOG\.md$` 的条目数为 1 | |
| 10003 | `Docs/` 下不存在大小写不同的变更日志变体文件。 | `git ls-files Docs` 中匹配 `^Docs/ChangeLog\.md$` 的条目数为 0 | |
| 10004 | 变更日志首行为 `# 变更日志（CHANGELOG）`。 | `head -1 Docs/CHANGELOG.md` 输出与预期一致 | |
| 10005 | 变更日志含「记录规则」章节。 | `grep -x '## 记录规则' Docs/CHANGELOG.md` 命中 1 行 | |
| 10006 | 记录规则首条声明必须在同一次提交中更新本文件。 | `grep -c '必须在同一次提交中更新本文件' Docs/CHANGELOG.md` 输出 `1` | |
| 10007 | 变更日志含 `[未发布]` 小节。 | `grep -Fx '## [未发布]' Docs/CHANGELOG.md` 命中 | |
| 10008 | `[未发布]` 小节下含 `新增` 与 `变更` 两个分组。 | 该小节内 `### 新增`、`### 变更` 各命中 1 次 | |
| 10009 | 变更日志含版本小节 `[0.1.0] - 2026-08-08`。 | `grep -Fx '## [0.1.0] - 2026-08-08' Docs/CHANGELOG.md` 命中 | |
| 10010 | `0.1.0` 小节含 `新增`、`变更`、`修复` 三个分组。 | 该小节内三个 `###` 标题各命中 1 次 | |
| 10011 | 变更日志含「历史版本（VeritasQuant 子项目）」章节。 | `grep -F '## 历史版本（VeritasQuant 子项目）'` 命中 | |
| 10012 | 历史版本含 `0.1.2` 与 `0.1.1` 两个小节，日期均为 `2026-08-03`。 | `grep -c '2026-08-03' Docs/CHANGELOG.md` 输出 `2` | |
| 10013 | 变更日志声明版本号唯一来源为根目录 `VERSION`。 | `grep -F '../VERSION' Docs/CHANGELOG.md` 命中 | |
| 10014 | 变更日志声明五类记录分组（新增/变更/修复/移除/安全）。 | `grep -F '新增` / `变更` / `修复` / `移除` / `安全`' Docs/CHANGELOG.md` 命中 | |
| 10015 | 变更日志为 UTF-8 无 BOM、LF 换行。 | `file Docs/CHANGELOG.md` 显示 `UTF-8 text` 且不含 `CRLF`、`BOM` | |

## 二、开发规范条款（10016–10034）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10016 | `Docs/DevSpec/DocSpec.md` 含章节 `## 4. 变更日志（CHANGELOG，强制）`。 | `grep -F '## 4. 变更日志（CHANGELOG，强制）' Docs/DevSpec/DocSpec.md` 命中 | |
| 10017 | `DocSpec` §4 声明变更日志固定路径为 `Docs/CHANGELOG.md`。 | `grep -F '固定存放于 `Docs/CHANGELOG.md`' Docs/DevSpec/DocSpec.md` 命中 | |
| 10018 | `DocSpec` §4 声明变更日志文件名必须全大写。 | `grep -F '文件名全大写' Docs/DevSpec/DocSpec.md` 命中 | |
| 10019 | `DocSpec` §4 将「功能新增、删除或行为变更」列为必须记录项。 | `grep -F '功能新增、删除或行为变更' Docs/DevSpec/DocSpec.md` 命中 | |
| 10020 | `DocSpec` §4 将「缺陷修复」列为必须记录项。 | `grep -F '缺陷修复（使用者可感知的问题）' Docs/DevSpec/DocSpec.md` 命中 | |
| 10021 | `DocSpec` §4 将「接口路径、字段、错误码变更」列为必须记录项。 | `grep -F '接口路径、请求/响应字段、错误码变更' Docs/DevSpec/DocSpec.md` 命中 | |
| 10022 | `DocSpec` §4 将「数据库表结构、迁移、种子数据变更」列为必须记录项。 | `grep -F '数据库表结构、迁移脚本、种子数据变更' Docs/DevSpec/DocSpec.md` 命中 | |
| 10023 | `DocSpec` §4 将「配置、环境变量、部署升级回滚流程变更」列为必须记录项。 | `grep -F '配置项、环境变量、镜像与部署/升级/回滚流程变更' Docs/DevSpec/DocSpec.md` 命中 | |
| 10024 | `DocSpec` §4 将「开发规范本身变更」列为必须记录项。 | `grep -F '开发规范（本目录 `Docs/DevSpec/`）本身变更' Docs/DevSpec/DocSpec.md` 命中 | |
| 10025 | `DocSpec` §4 给出「使用者或部署者是否可感知」的豁免判定标准。 | `grep -F '使用者或部署者是否可感知' Docs/DevSpec/DocSpec.md` 命中 | |
| 10026 | `DocSpec` §4 规定未同步更新时评审应拒绝合并。 | `grep -F '代码评审应要求补充后再合并' Docs/DevSpec/DocSpec.md` 命中 | |
| 10027 | `Docs/DevSpec/GitSpec.md` 含章节 `## 3. 与变更日志的联动（强制）`。 | `grep -F '## 3. 与变更日志的联动（强制）' Docs/DevSpec/GitSpec.md` 命中 | |
| 10028 | `GitSpec` §3 要求变更日志与功能变更进入同一次提交。 | `grep -F '必须同时包含 `Docs/CHANGELOG.md`' Docs/DevSpec/GitSpec.md` 命中 | |
| 10029 | `GitSpec` §3 提供指向 `DocSpec` §4 的链接。 | `grep -F 'DocSpec.md`](DocSpec.md) 第 4 节' Docs/DevSpec/GitSpec.md` 命中 | |
| 10030 | `GitSpec` §3 声明纯文档勘误无需更新变更日志。 | `grep -F '纯文档勘误、格式整理等无可感知影响的提交无需更新变更日志' Docs/DevSpec/GitSpec.md` 命中 | |
| 10031 | `Docs/DevSpec/FileNamingSpec.md` §1 含全大写固定名例外条款。 | `grep -F '例外（全大写固定名）' Docs/DevSpec/FileNamingSpec.md` 命中 | |
| 10032 | `FileNamingSpec` 明令禁止 `ChangeLog.md` 变体。 | `grep -F '不得写作 `ChangeLog.md`' Docs/DevSpec/FileNamingSpec.md` 命中 | |
| 10033 | `AGENTS.md` 规范表中 `DocSpec` 行标注变更日志强制要求。 | `grep -F '文档规范（含变更日志 `Docs/CHANGELOG.md` 强制要求）' AGENTS.md` 命中 | |
| 10034 | `AGENTS.md` 规范表中 `GitSpec` 行标注变更日志联动。 | `grep -F 'Git 提交规范（含变更日志联动）' AGENTS.md` 命中 | |

## 三、跨平台换行与仓库卫生（10035–10050）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10035 | 仓库根目录存在 `.gitattributes`。 | `test -f .gitattributes` 退出码为 0 | |
| 10036 | `.gitattributes` 声明所有文本以 LF 存储并检出。 | `grep -Fx '* text=auto eol=lf' .gitattributes` 命中 | |
| 10037 | `.gitattributes` 强制 `*.cmd` 使用 CRLF。 | `grep -Fx '*.cmd text eol=crlf' .gitattributes` 命中 | |
| 10038 | `.gitattributes` 强制 `*.bat`、`*.ps1` 使用 CRLF。 | 两个精确匹配各命中 1 行 | |
| 10039 | `.gitattributes` 强制 `*.sh` 使用 LF。 | `grep -Fx '*.sh text eol=lf' .gitattributes` 命中 | |
| 10040 | `.gitattributes` 将图片等二进制资源标记为 `binary`。 | `grep -c ' binary$' .gitattributes` 输出大于 0 | |
| 10041 | 已跟踪文本文件在工作区的行尾为 LF（`*.cmd`/`*.bat`/`*.ps1` 除外）。 | `git ls-files --eol` 筛出 `w/crlf` 的文件名，除 `.cmd`/`.bat`/`.ps1` 外结果为空 | |
| 10042 | `Deploy/` 下三个批处理脚本在工作区保持 CRLF。 | `git ls-files --eol Deploy` 中 `w/crlf` 且以 `.cmd` 结尾的条目数为 3 | |
| 10043 | 三个批处理脚本为 UTF-8 带 BOM 编码。 | `file Deploy/*.cmd` 均显示 `UTF-8 (with BOM)` | |
| 10044 | 未暂存改动中不存在仅行尾不同的文件。 | `git diff --stat` 与 `git diff --ignore-cr-at-eol --stat` 输出行数相同 | |
| 10045 | 仓库本地 Git 配置关闭自动 CRLF 转换。 | `git config --local core.autocrlf` 输出 `false` | |
| 10046 | 仓库本地 Git 配置指定检出换行为 LF。 | `git config --local core.eol` 输出 `lf` | |
| 10047 | `Docs/DevSpec/FileEncodingSpec.md` 含章节 `## 4. 换行符（跨平台一致，强制）`。 | `grep -F '## 4. 换行符（跨平台一致，强制）' Docs/DevSpec/FileEncodingSpec.md` 命中 | |
| 10048 | `FileEncodingSpec` §4 列明 `* text=auto eol=lf`、`eol=crlf`、`eol=lf`、`binary` 四类条款。 | 该节内 `text=auto eol=lf`、`eol=crlf`、`eol=lf`、`binary` 四个关键词各命中至少 1 次 | |
| 10049 | `FileEncodingSpec` §4 给出三步假修改修正流程。 | 该节内 `--renormalize`、`git checkout -- .`、`ls-files --eol` 三条命令各出现 1 次 | |
| 10050 | `FileEncodingSpec` §4 说明 `.gitattributes` 属性优先于 `core.autocrlf`。 | `grep -F '优先于' Docs/DevSpec/FileEncodingSpec.md` 命中 | |

## 四、需求清单自身（10051–10055）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10051 | 本清单位于 `Docs/RequirementList.md`。 | `test -f Docs/RequirementList.md` 退出码为 0 | |
| 10052 | 本清单编号自 `10001` 起连续升序且无断号。 | 按行提取首个五位数字序列，相邻两号差值恒为 1 | |
| 10053 | 本清单每条需求均含编号、需求描述、核对方式、结果四列。 | 在 Markdown 预览中每个数据行显示为 4 列 | |
| 10054 | 本清单「结果」列未验收条目为空。 | 数据行最后一列去除空白后为空或为 `√`/`×` | |
| 10055 | 本清单汇总表的分类条目数与正文实际条目数一致。 | 汇总表「条目数」列合计值与 10052 条核对得到的编号总数相同 | |

## 五、改造计划文档（10056–10065）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10056 | 两个改造清单文件分别位于 `Docs/Refactor/2610-001-SecuCodeToUsc.md` 与 `Docs/Refactor/2610-002-PrevCloseToLastClose.md`。 | 两个路径 `test -f` 退出码均为 0 | |
| 10057 | 改造项 A 清单编号自 `A-01` 起连续无缺号。 | `2610-001-SecuCodeToUsc.md` 中提取 `A-\d{2}` 编号后相邻差值恒为 1 | |
| 10058 | 改造项 B 清单编号自 `B-01` 起连续无缺号。 | `2610-002-PrevCloseToLastClose.md` 中提取 `B-\d{2}` 编号后相邻差值恒为 1 | |
| 10059 | 两个文件合计包含 4 项待决策（`DA-01`、`DB-01`～`DB-03`）。 | 两文件中 `DA-0[0-9]` 与 `DB-0[0-9]` 表格行合计为 4 | |
| 10060 | `README.md` 文档列表包含指向两个改造清单的链接。 | `grep -c 'Docs/Refactor/' README.md` 输出不小于 2 | |
| 10061 | 改造项 B 清单已声明示例数据仅供参考、权威判定以生产库 SQL 为准。 | `grep -c "示例文件不是生产数据，不能作为删除依据" Docs/Refactor/2610-002-PrevCloseToLastClose.md` 输出 `1` | |
| 10062 | 改造项 B 的 `DB-01` 记录了「本次只改名、不删列」的结论。 | `grep -c "本次只改名，不删列" Docs/Refactor/2610-002-PrevCloseToLastClose.md` 输出 `1` | |
| 10063 | 改造项 B 的重命名迁移与回填迁移各有一条清单条目。 | 命中 `V200030__finv_quote_kline_rename_prev_close_to_last_close` 与 `V200031__finv_quote_kline_backfill_open_from_last_close` 各 1 处 | |
| 10064 | `Docs/Refactor/` 下的过程文档文件名均带 `yyMM-` 四位前缀。 | `ls Docs/Refactor/*.md` 结果全部匹配 `^[0-9]{4}-` | |
| 10065 | `FileNamingSpec.md` 记录了 `yyMM-` 前缀规则及其例外清单。 | `grep -c "yyMM-" Docs/DevSpec/FileNamingSpec.md` 输出不小于 2 | |

## 六、改造项 A 验收（10066–10079）

> 对应清单 [`Refactor/2610-001-SecuCodeToUsc.md`](Refactor/2610-001-SecuCodeToUsc.md)：`secu_code` → `usc` 全库统一。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10066 | 存在 `V29` 改名迁移且为单事务脚本。 | `grep -c "^BEGIN;" Deploy/Migrations/V200029__rename_secu_code_to_usc.sql` 与 `grep -c "^COMMIT;"` 均为 `1` | |
| 10067 | `V29` 覆盖 6 张表的 `secu_code` → `usc` 改名。 | 脚本内表名数组包含 `finv_quote_secu_kline_min`、`finv_quote_ingest_batches`、`finv_quote_revision_log`、`finv_quant_backtest_strategy`、`finv_quant_backtest_run`、`finv_quant_backtest_run_archive` 共 6 项 | |
| 10068 | `V29` 对 5 个依赖索引执行改名。 | `grep -c "idx_finv_quote_usc_ts\|idx_finv_quote_ingest_batches_usc\|idx_finv_quote_revision_log_usc\|idx_finv_quant_backtest_strategy_usc\|idx_finv_quant_backtest_run_usc" Deploy/Migrations/V200029__rename_secu_code_to_usc.sql` 输出不小于 `5` | |
| 10069 | `V29` 不重建主键。 | 脚本内无 `DROP CONSTRAINT` 与 `ADD CONSTRAINT ... PRIMARY KEY` | |
| 10070 | Go 代码中不再出现旧字段名。 | `grep -rn "secu_code\|SecuCode\|secuCode" internal/ --include="*.go"` 输出为空 | |
| 10071 | 前端源码中不再出现旧字段名。 | `grep -rn "secu_code\|SecuCode\|secuCode" Web/src/` 输出为空 | |
| 10072 | 行情查询接口的查询参数与响应字段均为 `usc`。 | `grep -c "usc" internal/api/handler/query.go` 不小于 `5`，且该文件无旧名 | |
| 10073 | 文档中除计划文档、变更日志与本清单外无旧字段名。 | `grep -rn "secu_code\|secuCode" Docs/ \| grep -v "Docs/Refactor/\|Docs/CHANGELOG.md\|Docs/RequirementList.md"` 输出为空 | |
| 10074 | `CHANGELOG` 已记录该破坏性变更与升级要求。 | `grep -c "破坏性变更" Docs/CHANGELOG.md` 不小于 `2` | |
| 10075 | 版本号已递增至 `0.1.1`。 | `cat VERSION` 输出 `0.1.1` | |
| 10076 | 升级文档声明新旧镜像不可共存。 | `grep -c "禁止新旧镜像共存" Deploy/Docker/Win11DockerUpgrade.md` 输出 `1` | |
| 10077 | 前端构建产物中不含旧字段名。 | `grep -rc "secu_code" Web/dist/assets/*.js` 全为 `0` | |
| 10078 | `V29` 同时覆盖两个审计表的索引改名（执行中新发现，原清单 `A-02` 只列 3 个）。 | 脚本内包含 `idx_finv_quote_ingest_batches_usc` 与 `idx_finv_quote_revision_log_usc` | |
| 10079 | All-in-One 内嵌产物 `internal/webui/dist` 已按新前端重新生成（执行中新发现，清单原未列）。 | `grep -rc "secu_code" internal/webui/dist/assets/*.js` 全为 `0` | |

## 七、改造项 B 验收（10080–10091）

> 对应清单 [`Refactor/2610-002-PrevCloseToLastClose.md`](Refactor/2610-002-PrevCloseToLastClose.md)：`prev_close` → `last_close` 重命名 + `open` 兜底回填。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10080 | 存在 `V30` 改名迁移且为单事务脚本。 | `grep -c "^BEGIN;" Deploy/Migrations/V200030__finv_quote_kline_rename_prev_close_to_last_close.sql` 与 `grep -c "^COMMIT;"` 均为 `1` | |
| 10081 | `V30` 以幂等方式把 `prev_close` 改名为 `last_close`。 | 脚本内同时含 `information_schema.columns`、`column_name = 'prev_close'` 与 `RENAME COLUMN prev_close TO last_close` | |
| 10082 | `V30` 为 `last_close` 写入口径注释。 | `grep -c "与 open 独立" Deploy/Migrations/V200030__finv_quote_kline_rename_prev_close_to_last_close.sql` 输出 `1` | |
| 10083 | `V31` 回填条件完整（缺失/0 值 + `last_close` 非空非 0）。 | 脚本内出现 `open IS NULL OR open = 0`、`last_close IS NOT NULL`、`last_close <> 0` 三处条件 | |
| 10084 | `V31` 输出回填行数。 | `grep -c "RAISE NOTICE" Deploy/Migrations/V200031__finv_quote_kline_backfill_open_from_last_close.sql` 不小于 `2` | |
| 10085 | `V31` 向 `finv_quote_revision_log` 写入留痕记录。 | 脚本内含 `MIGRATION_V31` 与 `INSERT INTO finv_quote_revision_log` | |
| 10086 | `V31` 明确声明回填不可逆。 | `grep -c "不可逆" Deploy/Migrations/V200031__finv_quote_kline_backfill_open_from_last_close.sql` 不小于 `1` | |
| 10087 | Go 代码中不再用旧列名（MVSV 文件头文本与 ATR 局部变量除外）。 | `grep -rn "prev_close" internal/ --include="*.go"` 输出为空 | |
| 10088 | 导入链路按同一规则增量兜底 `open`。 | `grep -c "fillOpenFromLastClose" internal/meta/finv/quote/service.go` 不小于 `2` | |
| 10089 | 数据格式与接口文档已改用 `last_close`。 | `grep -c "last_close" Docs/DataFormat/MvsvFileFormat.md Docs/API/HistoryQuote/HistoryQuote.md` 各不小于 `1` | |
| 10090 | 前端源码不含该字段引用。 | `grep -rn "prev_close\|prevClose" Web/src/` 输出为空 | |
| 10091 | `CHANGELOG` 记录改名、回填与不可逆提示。 | `grep -c "last_close" Docs/CHANGELOG.md` 不小于 `2` | |

## 八、改造项 C 验收（10092–10107）

> 对应清单 [`Refactor/2610-003-MoveTablesToSchemas.md`](Refactor/2610-003-MoveTablesToSchemas.md)：`finv_quote_*` → `quote`、`finv_quant_*` → `quant` schema。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10092 | 存在 `V32` 迁移且创建两个业务 schema。 | `grep -c "CREATE SCHEMA IF NOT EXISTS" Deploy/Migrations/V200032__move_finv_tables_to_quote_quant_schemas.sql` 输出 `2` | |
| 10093 | `V32` 覆盖 16 张表的迁移（3 张 quote + 13 张 quant，通过数组循环执行 `SET SCHEMA`）。 | `grep -oE "'finv_(quote\|quant)_[a-z_]+" Deploy/Migrations/V200032__move_finv_tables_to_quote_quant_schemas.sql \| sort -u \| wc -l` 输出 `16` | |
| 10094 | `V32` 设置数据库默认 `search_path`。 | 脚本内含 `ALTER DATABASE` 与 `current_database()` | |
| 10095 | `V32` 具备收敛校验，失败整体回滚。 | `grep -c "RAISE EXCEPTION" Deploy/Migrations/V200032__move_finv_tables_to_quote_quant_schemas.sql` 不小于 `2` | |
| 10096 | 存在 `V200024` 收尾迁移，处理种子段新建的 3 张 quant 表。 | `test -f Deploy/Migrations/V300024__finv_quant_seed_tables_move_to_quant_schema.sql` | |
| 10097 | 应用连接串携带 `search_path`。 | `grep -c "search_path=public,quote,quant" internal/database/postgres.go` 输出 `1` | |
| 10098 | Go 代码中表名均带 schema 前缀（无裸表名）。 | `grep -rn "FROM finv_quote_\|FROM finv_quant_\|INTO finv_quote_\|INTO finv_quant_\|UPDATE finv_quant_\|UPDATE finv_quote_" internal/ --include="*.go"` 输出为空 | |
| 10099 | 文档中表名均带 schema 前缀。 | `grep -rn "FROM finv_quote_\|FROM finv_quant_" Docs/ \| grep -v "Docs/Refactor/\|Docs/CHANGELOG.md\|Docs/RequirementList.md"` 输出为空 | |
| 10100 | 生产库 `quote` schema 含 3 张行情表。 | `SELECT count(*) FROM pg_tables WHERE schemaname='quote'` 为 `3` | |
| 10101 | 生产库 `quant` schema 含 13 张量化表。 | `SELECT count(*) FROM pg_tables WHERE schemaname='quant'` 为 `13` | |
| 10102 | 生产库 `public` 不再有业务域表。 | `SELECT count(*) FROM pg_tables WHERE schemaname='public' AND tablename ~ '^finv_(quote\|quant)_'` 为 `0` | |
| 10103 | 数据库与连接 `search_path` 为 `public, quote, quant`。 | `SHOW search_path;` 输出含 `public, quote, quant` | |
| 10104 | 表迁移后 `gmt_update` 触发器仍生效。 | 更新一行后 `gmt_update` 大于 `gmt_create` | |
| 10105 | 存在数据库 schema 规范文档。 | `test -f Docs/DevSpec/DbSchemaSpec.md` | |
| 10106 | `CHANGELOG` 记录本次 schema 迁移与连接要求。 | `grep -c "quote" Docs/CHANGELOG.md` 不小于 `2` | |
| 10107 | 历史种子迁移的列名例外已登记。 | `grep -c "V200019" Docs/DevSpec/DbSchemaSpec.md Docs/CHANGELOG.md` 各不小于 `1` | |

## 九、改造项 D 验收（10108–10120）

> 对应清单 [`Refactor/2610-004-MoveFutuTablesToSchema.md`](Refactor/2610-004-MoveFutuTablesToSchema.md)：`finv_futu_*` → `futu` schema。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10108 | 存在 `V33` 迁移且创建 `futu` schema。 | `grep -c "CREATE SCHEMA IF NOT EXISTS futu" Deploy/Migrations/V200033__move_finv_futu_tables_to_futu_schema.sql` 输出 `1` | |
| 10109 | `V33` 覆盖 5 张富途映射表。 | `grep -oE "'finv_futu_[a-z_]+" Deploy/Migrations/V200033__move_finv_futu_tables_to_futu_schema.sql \| sort -u \| wc -l` 输出 `5` | |
| 10110 | `V33` 更新数据库默认 `search_path` 含 `futu`。 | 脚本含 `public, quote, quant, futu` | |
| 10111 | `V33` 具备收敛校验与回滚。 | `grep -c "RAISE EXCEPTION" Deploy/Migrations/V200033__move_finv_futu_tables_to_futu_schema.sql` 不小于 `2` | |
| 10112 | 应用连接串 `search_path` 含 `futu`。 | `grep -c "search_path=public,quote,quant,futu" internal/database/postgres.go` 输出 `1` | |
| 10113 | Go 代码不引用富途映射表（零改动，注释除外）。 | `grep -rn "finv_futu" internal/ --include="*.go" \| grep -v "// "` 输出为空 | |
| 10114 | 生产库 `futu` schema 含 5 张表。 | `SELECT count(*) FROM pg_tables WHERE schemaname='futu'` 为 `5` | |
| 10115 | 生产库 `public` 无富途映射表残留。 | `SELECT count(*) FROM pg_tables WHERE schemaname='public' AND tablename LIKE 'finv_futu%'` 为 `0` | |
| 10116 | 迁移后行数不变（503/30/54/25/29）。 | 五张表分别计数与基线一致 | |
| 10117 | 触发器随表迁移仍生效。 | 更新一行后 `gmt_update` 大于 `gmt_create` | |
| 10118 | 数据字典文档已加 `futu.` 前缀。 | `grep -rn "finv_futu" Docs/DataDictMapping/ \| grep -v "futu.finv_futu\|idx_finv_futu\|Migrations/"` 输出为空 | |
| 10119 | `DbSchemaSpec` 已登记 `futu` schema 与四段 `search_path`。 | `grep -c "futu" Docs/DevSpec/DbSchemaSpec.md` 不小于 `3` | |
| 10120 | `CHANGELOG` 记录本项变更。 | `grep -c "finv_futu_mapping" Docs/CHANGELOG.md` 不小于 `1` | |

## 十、SQL schema 限定防回归（10121–10124）

> 背景：系统融合后可能出现同名表，CRUD SQL 必须显式带 schema 前缀；本组为自动化校验要求。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10121 | 存在 SQL schema 限定的自动化校验测试。 | `test -f internal/database/schema_qualify_test.go` 且 `grep -c "func TestSQLSchemaQualified" internal/database/schema_qualify_test.go` 输出 `1` | |
| 10122 | 该校验覆盖 `finv_quote_` / `finv_quant_` / `finv_futu_` 三类域表。 | `grep -c "finv_quote_\|finv_quant_\|finv_futu_" internal/database/schema_qualify_test.go` 不小于 `3` | |
| 10123 | 代码中存在未限定表名时校验会失败（非空跑）。 | 临时新增一行 `SELECT 1 FROM finv_quote_secu_kline_min` 后 `go test ./internal/database/ -run TestSQLSchemaQualified` 退出码非 0，删除后恢复 0 | |
| 10124 | 规范文档记录强制规则与融合前同名表风险。 | `grep -c "融合前的同名表风险" Docs/DevSpec/DbSchemaSpec.md` 输出 `1` | |

## 十一、部署目录结构与 Native 说明（10125–10131）

> 背景：为支持「容器」与「宿主直装」两种部署方式，`Deploy/` 按方式分目录；Native 方式须明确适用边界。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10125 | Docker 方式专属文件位于 `Deploy/Docker/`。 | `Deploy/Docker/` 下存在 `docker-compose.yml`、`Dockerfile`、`.env.example`、`deploy.cmd`、`upgrade.cmd`、`rollback.cmd` | |
| 10126 | 共用文件保留在 `Deploy/` 根下。 | `Deploy/Migrations/` 与 `Deploy/Backup/` 均存在 | |
| 10127 | 备份目录名为 `Backup`（首字母大写，无小写同名目录）。 | `ls Deploy/ \| grep -x Backup` 输出 `Backup`；`ls Deploy/ \| grep -cx backup` 输出 `0`（Windows 大小写不敏感，以 `ls` 实际输出为准） | |
| 10128 | compose 的构建路径指向新位置。 | `grep -n "dockerfile: Deploy/Docker/Dockerfile" Deploy/Docker/docker-compose.yml` 输出 `1` | |
| 10129 | CI 使用新的 Dockerfile 路径。 | `grep -c "file: Deploy/Docker/Dockerfile" .github/workflows/CI.yml` 输出 `2` | |
| 10130 | Native 目录含说明文档，且写明适用与不适用场景。 | `grep -c "适用场景（以下场景才建议选 Native）" Deploy/Native/README.md` 与 `grep -c "不适用场景" Deploy/Native/README.md` 均不小于 `1` | |
| 10131 | 面向使用者的文档不再出现旧的 Docker 部署路径。 | `grep -rn "Deploy/\.env\|Deploy/docker-compose.yml\|Deploy/Win11Docker" README.md Docs/FinvQuant/V1/Prompt.md` 输出为空 | |

## 十二、Native 部署配套与生成物命名（10132–10140）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10132 | Native 目录下脚本与配置文件采用大驼峰文件名。 | `ls Deploy/Native/` 中 `.sh` 与品牌名文件均以大写字母开头（`Install.sh`、`Upgrade.sh`、`Uninstall.sh`、`FinvQuant.service`、`FinvQuant.env.example`） | |
| 10133 | systemd unit 源文件名按品牌写作 `FinvQuant.service`（无小写 s 变体）。 | `ls Deploy/Native/ \| grep -x FinvQuant.service` 输出 1 行；`ls Deploy/Native/ \| grep -cx Finvquant.service` 输出 `0`（Windows 大小写不敏感，以 `ls` 实际输出为准） | |
| 10134 | unit 含运行所需关键指令。 | `grep -cE "^(ExecStart\|EnvironmentFile\|Restart\|User\|WantedBy)=" Deploy/Native/FinvQuant.service` 输出 `5` | |
| 10135 | 三个 Shell 脚本通过语法检查。 | `bash -n Deploy/Native/{Install,Upgrade,Uninstall}.sh` 退出码均为 0 | |
| 10136 | Native 升级脚本的备份命名为 `finvquant_YYYYMMDD_HHMMSS.dump`。 | `grep -c "finvquant_\$(date +%Y%m%d_%H%M%S).dump" Deploy/Native/Upgrade.sh` 输出 `1` | |
| 10137 | 现有备份文件名均为纯 ASCII 且符合时间格式。 | `ls Deploy/Backup/*.dump` 的文件名全部匹配 `finvquant_[0-9]{8}_[0-9]{6}\.dump` | |
| 10138 | Docker 升级脚本不依赖 `%date%` 截取时间。 | `grep -c "powershell -NoProfile -Command \"Get-Date -Format yyyyMMdd_HHmmss\"" Deploy/Docker/upgrade.cmd` 输出 `1`，且该文件无 `%date:~` | |
| 10139 | 命名规范记录生成物（备份/导出）命名规则。 | `grep -c "finvquant_YYYYMMDD_HHMMSS.dump" Docs/DevSpec/FileNamingSpec.md` 输出 `1` | |
| 10140 | Native 目录 README 的文件清单与实际文件一致。 | README 第 7 节列出的文件逐个 `test -f Deploy/Native/<name>` 均成功 | |

## 十三、Deploy 目录总览文档（10141–10144）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10141 | `Deploy/` 根下存在总览文档。 | `test -f Deploy/README.md` | |
| 10142 | 总览文档说明两种部署方式目录与共用目录。 | `grep -c "Docker/\|Native/\|Migrations/\|Backup/" Deploy/README.md` 不小于 `4` | |
| 10143 | 总览文档列出两种方式共同遵守的约束。 | `grep -c "禁止新旧共存\|升级前必须备份\|schema 限定" Deploy/README.md` 不小于 `3` | |
| 10144 | 根 README 与 Native 文档均链接到该总览。 | `grep -c "Deploy/README.md" README.md` 不小于 `1`；`grep -c "../README.md" Deploy/Native/README.md` 不小于 `1` | |

## 十四、Archive 归档目录（10145–10148）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10145 | 仓库根存在归档目录且含说明文档。 | `test -f Archive/README.md` | |
| 10146 | 归档说明界定「适合放入」与「不适合放入」的范围。 | `grep -c "什么该放进这里\|什么不该放进这里" Archive/README.md` 输出 `2` | |
| 10147 | 根 README 目录结构登记 `Archive/`。 | `grep -c "^├── Archive/" README.md` 输出 `1` | |
| 10148 | 部署总览给出废弃文件去处指引。 | `grep -c "Archive/README.md" Deploy/README.md` 不小于 `1` | |

## 十五、数据库名与角色名统一为 finv（10149–10158）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10149 | 源码默认的 PG 用户与库名为 `finv`，密码默认值不变。 | `grep -c 'envOr("FINV_PG_USER", "finv")\|envOr("FINV_PG_DATABASE", "finv")' internal/config/config.go` 输出 `2`；`grep -c 'envOr("FINV_PG_PASSWORD", "finvquant")'` 输出 `1` | |
| 10150 | compose 中应用与数据库的用户/库名默认值均为 `finv`。 | `grep -c 'FINV_PG_USER:-finv}\|FINV_PG_DATABASE:-finv}' Deploy/Docker/docker-compose.yml` 不小于 `2` | |
| 10151 | compose 健康检查使用新库名。 | `grep -c 'pg_isready -U ${FINV_PG_USER:-finv} -d ${FINV_PG_DATABASE:-finv}' Deploy/Docker/docker-compose.yml` 输出 `1` | |
| 10152 | `.env.example` 与 Native env 模板的用户/库名为 `finv`。 | 两个文件中 `grep -cE '^FINV_PG_(USER\|DATABASE)=finv$'` 各输出 `2` | |
| 10153 | Docker 脚本的 `psql`/`pg_dump` 使用新库名，无残留旧名。 | `grep -rc 'finvquant' Deploy/Docker/*.cmd` 中无 `-U finvquant`/`-d finvquant` | |
| 10154 | Native 安装脚本建库/建角色兜底值为 `finv`，系统用户仍为 `finvquant`。 | `grep -c 'echo finv)' Deploy/Native/Install.sh` 输出 `2`；`grep -c 'APP_USER="finvquant"' ` 输出 `1` | |
| 10155 | 生产库的库名、角色名已改为 `finv`，旧名不存在。 | `psql -U finv -d finv -c "select count(*) from pg_database where datname='finvquant'"` 输出 `0`；`pg_roles` 同理 | |
| 10156 | 改名后数据与迁移记录不变。 | `select count(*) from quote.finv_quote_secu_kline_min` = `2804329`；`select max(version::int) from schema_version` = `100024` | |
| 10157 | 改名后库级 `search_path` 与授权随 OID 保留。 | `select array_to_string(setconfig,',') from pg_db_role_setting ...` 含 `search_path=public, quote, quant, futu`；三个业务 schema owner 为 `finv` | |
| 10158 | 仓库根存在 `.dockerignore` 且排除备份与前端依赖。 | `grep -c 'Deploy/Backup/\|Web/node_modules/' .dockerignore` 输出 `2` | |

## 十六、新增 export / sche 两个 schema（10159–10166）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10159 | 存在创建两个 schema 的迁移且可重复执行。 | `test -f Deploy/Migrations/V200034__create_export_and_sche_schemas.sql`；文件内含 `CREATE SCHEMA IF NOT EXISTS export` 与 `... sche` | |
| 10160 | 生产库存在 `export`、`sche` 两个 schema。 | `select count(*) from pg_namespace where nspname in ('export','sche')` 输出 `2` | |
| 10161 | 两个 schema 的属主为应用角色（`finv`）。 | `select count(*) from pg_namespace where nspname in ('export','sche') and pg_get_userbyid(nspowner)='finv'` 输出 `2` | |
| 10162 | 应用角色对两个 schema 具备 `USAGE` 与 `CREATE`。 | `select has_schema_privilege('finv','export','USAGE') and has_schema_privilege('finv','export','CREATE')` 为 `t`，`sche` 同理 | |
| 10163 | 数据库默认 `search_path` 含两个新 schema 且位于末尾。 | `select array_to_string(setconfig,',') from pg_db_role_setting ...` 输出 `search_path=public, quote, quant, futu, export, sche` | |
| 10164 | 应用连接串 `search_path` 同步为六段。 | `grep -c 'search_path=public,quote,quant,futu,export,sche' internal/database/postgres.go` 输出 `1` | |
| 10165 | 迁移记录已登记 `V34` 且成功。 | `select success from schema_version where version::int=34` 输出 `t` | |
| 10166 | schema 规范文件已登记两个新 schema。 | `grep -o '\`export\`\|\`sche\`' Docs/DevSpec/DbSchemaSpec.md \| wc -l` 不小于 `4` | |

## 十七、export 行情导出转存表（10167–10175）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10167 | 存在创建转存表的迁移。 | `test -f Deploy/Migrations/V200035__create_export_staging_kline_table.sql` | |
| 10168 | 转存表与业务表同名。 | `select count(*) from pg_tables where schemaname='export' and tablename='finv_quote_secu_kline_min'` 输出 `1` | |
| 10169 | 转存表列定义与业务表完全一致。 | 对 `information_schema.columns` 的 `(column_name,data_type,is_nullable,column_default)` 做双向 `EXCEPT` 后计数为 `0` | |
| 10170 | 转存表具备主键与 `(usc,ts)` 普通索引。 | `select count(*) from pg_index ... where indisprimary` 输出 `1`；存在 `pg_get_indexdef` 含 `(usc, ts)` 的非主键索引 | |
| 10171 | 转存表具备 `gmt_update` 触发器。 | `select count(*) from pg_trigger where tgname='trg_finv_quote_secu_kline_min_gmt_update' and tgrelid='export.finv_quote_secu_kline_min'::regclass` 输出 `1` | |
| 10172 | 迁移可重复执行（幂等）。 | 连续执行两次迁移文件，第二次输出「已存在，跳过建表/触发器已存在」且校验通过 | |
| 10173 | 转存可用：同日同证券行数与源表一致。 | `INSERT INTO export.... SELECT * FROM quote.... WHERE usc='NVDA' AND ts∈[2026-06,2026-07)` 后源/目标均为 `7220` | |
| 10174 | 转存表可清空复用。 | `TRUNCATE export.finv_quote_secu_kline_min` 后 `count(*)` 为 `0` | |
| 10175 | 迁移记录已登记 `V35` 且成功。 | `select success from schema_version where version::int=35` 输出 `t` | |

## 十八、新增 dedup 去重 schema（10176–10183）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10176 | 存在创建 dedup schema 的迁移且幂等。 | `test -f Deploy/Migrations/V200036__create_dedup_schema.sql`；含 `CREATE SCHEMA IF NOT EXISTS dedup` | |
| 10177 | 生产库存在 `dedup` schema。 | `select count(*) from pg_namespace where nspname='dedup'` 输出 `1` | |
| 10178 | 属主为应用角色（`finv`）。 | `select pg_get_userbyid(nspowner) from pg_namespace where nspname='dedup'` 输出 `finv` | |
| 10179 | 应用角色具备 `USAGE` 与 `CREATE`。 | `select has_schema_privilege('finv','dedup','USAGE') and has_schema_privilege('finv','dedup','CREATE')` 为 `t` | |
| 10180 | 数据库默认 `search_path` 含 `dedup` 且位于末尾。 | `select array_to_string(setconfig,',') from pg_db_role_setting ...` 输出 `search_path=public, quote, quant, futu, export, sche, dedup` | |
| 10181 | 应用连接串 `search_path` 同步为七段。 | `grep -c 'search_path=public,quote,quant,futu,export,sche,dedup' internal/database/postgres.go` 输出 `1` | |
| 10182 | 迁移记录已登记 `V36` 且成功。 | `select success from schema_version where version::int=36` 输出 `t` | |
| 10183 | 规范文件登记 `dedup` 并说明去重层次与适用边界。 | `grep -c 'dedup' Docs/DevSpec/DbSchemaSpec.md` 不小于 `4`；含「ON CONFLICT DO NOTHING」说明 | |

## 十九、新增 archive 归档 schema（10184–10191）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10184 | 存在创建 archive schema 的迁移且幂等。 | `test -f Deploy/Migrations/V200037__create_archive_schema.sql`；含 `CREATE SCHEMA IF NOT EXISTS archive` | |
| 10185 | 生产库存在 `archive` schema。 | `select count(*) from pg_namespace where nspname='archive'` 输出 `1` | |
| 10186 | 属主为应用角色（`finv`）。 | `select pg_get_userbyid(nspowner) from pg_namespace where nspname='archive'` 输出 `finv` | |
| 10187 | 应用角色具备 `USAGE` 与 `CREATE`。 | `select has_schema_privilege('finv','archive','USAGE') and has_schema_privilege('finv','archive','CREATE')` 为 `t` | |
| 10188 | 数据库默认 `search_path` 含 `archive` 且位于末尾。 | `select array_to_string(setconfig,',') from pg_db_role_setting ...` 输出 `search_path=public, quote, quant, futu, export, sche, dedup, archive` | |
| 10189 | 应用连接串 `search_path` 同步为八段。 | `grep -c 'search_path=public,quote,quant,futu,export,sche,dedup,archive' internal/database/postgres.go` 输出 `1` | |
| 10190 | 迁移记录已登记 `V37` 且成功。 | `select success from schema_version where version::int=37` 输出 `t` | |
| 10191 | 规范文件登记 `archive` 并说明与文件目录 `Archive/` 的区别及同名风险。 | `grep -c 'archive' Docs/DevSpec/DbSchemaSpec.md` 不小于 `4`；含「Archive/」与「三处同名」说明 | |

## 二十、feat/integration 快照构建（10192–10202）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10192 | `feat/integration` 的 push 会触发流水线。 | `grep -c 'feat/integration' .github/workflows/CI.yml` 不小于 `4`；`push.branches` 含该分支 | |
| 10193 | 存在快照版本计算 job，输出统一版本号。 | `snapshot-meta` job 的 `outputs` 含 `version`/`base_version`/`pr`/`timestamp`/`commit` | |
| 10194 | 版本格式为 `v{VERSION}.{PPPPP}.{yymmddHHmm}`。 | job 内以 `printf '%05d'` 左补零、`date +%y%m%d%H%M` 取时间戳、拼成 `v${BASE}.${PR5}.${TS}` | |
| 10195 | 无 PR 时 PR 段为 `00000`。 | 非纯数字时 `PR=0` 并经 `printf '%05d'` 输出 `00000` | |
| 10196 | 同一快照不同产物时间戳一致。 | 所有产物 job 引用 `needs.snapshot-meta.outputs.version`，无 job 自行 `date` | |
| 10197 | Linux 产两种架构。 | `matrix.include` 中含 `linux/amd64` 与 `linux/arm64` | |
| 10198 | Windows 产出包。 | `matrix.include` 中含 `windows/amd64`，归档格式 `zip` | |
| 10199 | 包内含二进制、迁移、部署脚本与构建信息。 | 解包后存在 `finvquant*`、`Deploy/Migrations/`、`Deploy/Native/`（Linux）或 `Deploy/Docker/`（Windows）、`VERSION`、`BUILD-INFO.txt`、`SHA256SUMS` | |
| 10200 | 快照发布为 prerelease 且 tag 为快照版本。 | `gh release create "${VER}" --prerelease --target <sha>` | |
| 10201 | 既有镜像流程不被集成分支影响。 | `docker` job 的 `if` 含 `github.ref != 'refs/heads/feat/integration'`；`dev/main/FinvQuant/tag` 规则未改动 | |
| 10202 | 版本标识规则写入规范。 | `grep -c 'PPPPP' Docs/DevSpec/GitSpec.md` 不小于 `1`；含「同一快照的所有产物必须共用同一个时间戳」 | |

## 二十一、本地快照打包脚本（10203–10211）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10203 | 存在唯一的快照打包脚本（Windows 侧为薄封装）。 | `test -f Deploy/Script/PackSnapshot.sh` 且 `test -f Deploy/Script/PackSnapshot.cmd`；`.cmd` 含 `grep -q 'norc "%SH%"'` 命中（转调 .sh）、且不含 `go build` / `tar -` / `zip -` | |
| 10204 | CI 调用同一脚本而非内联逻辑。 | `grep -c 'Deploy/Script/PackSnapshot.sh' .github/workflows/CI.yml` 不小于 `1`（含注释共 2 处）；`snapshot-binaries` 的 `run` 内无 `go build` | |
| 10205 | 脚本可一次产出三个平台包。 | `bash Deploy/Script/PackSnapshot.sh --version v0.1.1.00000.2610241530` 后 `Dist/` 含 `_ubuntu_amd64`、`_ubuntu_arm64`、`_windows_amd64` 三个包与 `.sha256` | |
| 10206 | 版本三段解析正确（含左补零）。 | `--version v0.1.1.00123.2610241530` 时输出 `PR 编号 : 00123`；不给 `--version` 时 PR 段为 `00000` | |
| 10207 | 同一快照多产物时间戳一致。 | 三包文件名中的 `yymmddHHmm` 与包内 `BUILD-INFO.txt` 的「构建时间戳」完全相同 | |
| 10208 | 包内结构符合约定。 | Linux 包含 `Deploy/Native/`、Windows 包含 `Deploy/Docker/` 且**不含** Dockerfile；两包均含 `Migrations/`、`VERSION`、`BUILD-INFO.txt`、`SHA256SUMS` | |
| 10209 | 非法参数被拒绝。 | `bash Deploy/Script/PackSnapshot.sh --version v0.1.1.123.2610241530; echo $?`（不加管道）退出码 `1`，并提示「版本格式不合法」 | |
| 10210 | 缺少前端产物时给出明确提示。 | 删除 `internal/webui/dist/index.html` 后运行，报错提示「请先构建前端或加 --build-web」 | |
| 10211 | 脚本语法与帮助可用。 | `bash -n Deploy/Script/PackSnapshot.sh` 退出码 0；`--help` 输出用法 | |
| 10212 | `.cmd` 能定位 Git Bash 并跳过 WSL 转发器。 | 在无 Git Bash 的 PATH 下运行 `.cmd`，输出的「使用 bash」路径**不含** `\System32\`；正常环境为 `<GitRoot>\bin\bash.exe` | |
| 10213 | `.cmd` 与 `.sh` 参数完全一致（薄封装）。 | `PackSnapshot.cmd --version v0.1.1.00042.2610241600 --target windows/amd64` 产出与 `.sh` 同名的 zip，且 `BUILD-INFO.txt` 中「构建方式」同为 `Deploy/Script/PackSnapshot.sh` | |
| 10214 | Windows 侧产出的 zip 使用标准正斜杠路径。 | `unzip -l dist/*.zip` 的 Name 列为 `finvquant_.../Deploy/Migrations/...`（无 `\`）；文件数 80 | |
| 10215 | 缺少 zip/powershell 时有回退或明确报错。 | 打 zip 时若无 `zip` 命令，依次尝试 Windows `tar.exe`、`powershell.exe`、`python3`；全无时报错列出四种可选方式 | |

## 二十二、打包脚本文档（10216–10222）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10216 | `Script` 目录有配套说明文档。 | `test -f Deploy/Script/README.md` | |
| 10217 | 文档覆盖全部参数并标出默认值。 | `grep -c '^\| `--' Deploy/Script/README.md` 不小于 `9` | |
| 10218 | 文档说明版本三段与时间戳一致性。 | `grep -c 'v{BASE_VERSION}.{PPPPP}.{yymmddHHmm}' Deploy/Script/README.md` 不小于 `1`；含「同一快照的所有产物共用同一个时间戳」 | |
| 10219 | 文档给出两种包的目录树与 SHA256SUMS 用法。 | 含 `FinvQuant_v0.1.1.00123.2610241530_ubuntu_arm64/` 与 `sha256sum -c SHA256SUMS` | |
| 10220 | 文档说明三种运行环境与 zip 回退链。 | 含「native」「WSL」「Git Bash」三词；含 `tar.exe（bsdtar` 与 `powershell.exe` → `python3` 顺序 | |
| 10221 | 文档给出产物校验命令。 | 含 `EM_X86_64`、`EM_AARCH64` 与「正斜杠」判定 | |
| 10222 | `Deploy/README` 精简并链接详细文档。 | `grep -c 'Script/README.md' Deploy/README.md` 不小于 `1`；该节行数不超过 20 行 | |

## 二十三、产物命名规范调整（10223–10229）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10223 | 产物目录为 `Dist`（大写首字母）。 | `grep -c 'OUT_DIR="${ROOT}/Dist"' Deploy/Script/PackSnapshot.sh` 输出 `1`；CI 中 `--out Dist` | |
| 10224 | 包名前缀首字母大写 `FinvQuant_`。 | `ls Dist/` 中包名以 `FinvQuant_v` 开头，无小写 `finvquant_v` 开头的包 | |
| 10225 | Linux 平台段为 `ubuntu`（不含版本号）。 | 包名匹配 `FinvQuant_.*_ubuntu_(amd64\|arm64)\.tar\.gz` | |
| 10226 | Windows 平台段为 `windows`（不用 win11）。 | 包名匹配 `FinvQuant_.*_windows_amd64\.zip`；全仓 `--include=*.sh --include=*.cmd --include=*.yml` 中无 `win11_` | |
| 10227 | 包内顶层目录名与包名一致。 | `tar -tzf Dist/*_ubuntu_arm64.tar.gz \| head -1` 为 `FinvQuant_<版本>_ubuntu_arm64/` | |
| 10228 | `.gitignore` / `.dockerignore` 同时忽略 `dist/` 与 `Dist/`。 | 两文件中均有 `dist/` 与 `Dist/` 两行 | |
| 10231 | 产物文件名用连字符连接（无下划线）。 | `ls Dist/` 中包名匹配 `^FinvQuant-v[^-]+-ubuntu-(amd64\|arm64)\.tar\.gz$` 与 `^FinvQuant-v[^-]+-windows-amd64\.zip$`，不含 `_` | |
| 10232 | 快照 release 标题为 `{版本}-FinvQuant`。 | GitHub releases 最新一条的 `name` 形如 `v0.1.1.00000.2610240325-FinvQuant`，`tag_name` 为该版本号（不含 `-FinvQuant`） | |
| 10233 | release 说明不重复列产物、不含冗余校验描述。 | release body 无「## 产物」小节、无三个包文件名、无「BUILD-INFO 的时间戳与版本号一致」「SHA256SUMS 为包内文件校验和」两句；附件由 GitHub release 的 assets 列表呈现 | |
| 10230 | `BUILD-INFO.txt` 在 CI 中包含运行号、仓库与流水线链接。 | CI 产物内 `BUILD-INFO.txt` 含 `构建方式    : Deploy/Script/PackSnapshot.sh（CI run <数字>）`、`仓库        : https://github.com/ACANX/FinvQuant`、`构建流水线  : https://github.com/.../actions/runs/<数字>` | |
| 10229 | 规范与文档同步新命名。 | `GitSpec.md` 含 `FinvQuant_{快照版本}_{ubuntu\|windows}_{amd64\|arm64}`；`Deploy/Script/README.md` 与 `Deploy/README.md` 无 `ubuntu2604`/`win11_` | |

## 二十四、VeritasQuant 归档（10234–10240）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10234 | `VeritasQuant/` 已移入 `Archive/`，根目录不再有该目录。 | `test -d Archive/VeritasQuant` 且 `test ! -d VeritasQuant` | |
| 10235 | 移动保留 Git 历史（识别为重命名而非删除+新增）。 | `git log --follow Archive/VeritasQuant/README.md` 能追溯到移动前提交；`git status` 中该批文件显示为 `R` | |
| 10236 | 归档不影响 Go 构建与测试。 | `go build ./...` 与 `go test ./internal/database/ -run TestSQLSchemaQualified` 均通过 | |
| 10237 | 文档路径引用已更新。 | `grep -c 'Archive/VeritasQuant' README.md AGENTS.md Docs/FinvQuant/V1/Prompt.md Archive/README.md` 均不小于 `1`；根 README 目录树无裸 `VeritasQuant/` | |
| 10238 | `Archive/README.md` 登记该归档项。 | 含 `\| \`VeritasQuant/\` \|` 行，说明归档日期与「不参与现行构建」 | |
| 10239 | 构建上下文排除归档目录。 | `.dockerignore` 含 `Archive/`，且无独立 `VeritasQuant/` 行 | |
| 10240 | 可执行示例与代码注释路径同步。 | `grep -rn 'Archive/VeritasQuant' Docs/API/HistoryQuote/ImportsUpload.md internal/meta/finv/quote/mvsv/parser.go` 各不小于 `1` | |

## 二十五、分钟行情 K 线 API 融合（10241–10263）

> 来源：`FinvQuoteDataDeDup`（Python/FastAPI）退役，接口融合进 FinvQuant Go 服务端。
> 关联文档：`Docs/API/KLine/*.md`、`Docs/API/APIs.md`、`Docs/DevSpec/ErrorCodeSpec.md`、`Docs/CHANGELOG.md`。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10241 | 分钟行情 API 注册新前缀 `/API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min`。 | `grep -c "Meta/Finv/Quote/DeDup/Secu/KLine/Min" internal/api/router.go` 输出不小于 `1` | |
| 10242 | 旧前缀 `/API/V1/Meta/Finv/Quote/Secu/KLine/Min` 作为过渡别名同时注册。 | `grep -c "LegacyAPIPrefix" internal/api/router.go` 输出不小于 `1` | |
| 10243 | 提供 `GET /Health` 探活端点（含 `/healthz` 过渡别名）。 | `grep -c 'GET("/Health"' internal/api/router.go` 与 `grep -c 'GET("/healthz"' internal/api/router.go` 均为 `1` | |
| 10244 | 分钟行情主表位于 `dedup.finv_quote_secu_kline_min`。 | `grep -c "const Table = \"dedup.finv_quote_secu_kline_min\"" internal/meta/finv/quote/cdeu/kline/sql.go` 输出 `1` | |
| 10245 | 迁移 V38 将主表从 `quote` 迁入 `dedup` 并可重复执行。 | `ls Deploy/Migrations/V200038__finv_quote_kline_move_to_dedup.sql` 不存在时退出码非 0；脚本含 `SET SCHEMA dedup` 与 `IF EXISTS` 幂等判断 | |
| 10246 | 迁移 V38 补齐 `change_price`/`change_ratio`/`provider` 三列。 | `grep -c "ADD COLUMN IF NOT EXISTS change_price" Deploy/Migrations/V200038__finv_quote_kline_move_to_dedup.sql` 输出 `1` | |
| 10247 | `export.finv_quote_secu_kline_min` 按 dedup 新结构重建。 | `grep -c "LIKE dedup.finv_quote_secu_kline_min" Deploy/Migrations/V200038__finv_quote_kline_move_to_dedup.sql` 输出 `1` | |
| 10248 | `ts` 契约与库内均为秒级；13 位毫秒默认拒绝。 | `go test ./internal/meta/finv/quote/cdeu/kline/ -run TestTSUnit` 通过；用例覆盖秒级通过与 13 位拒绝 | |
| 10249 | `FINV_KLINE_TS_UNIT=ms` 时毫秒入参换算为秒。 | `go test ./internal/meta/finv/quote/cdeu/kline/ -run TestTSUnit` 通过（含 ms 分支） | |
| 10250 | 具体错误使用 `211xxx` 编号且全局唯一。 | `go test ./internal/meta/finv/quote/cdeu/kline/ -run TestCatalogIntegrity` 通过 | |
| 10251 | 错误目录共 15 条具体码，覆盖校验/资源/容量/存储四类。 | 同上；`internal/meta/finv/quote/cdeu/kline/apierr.go` 中 `Code` 常量计数为 15 | |
| 10252 | 具体码的 HTTP 映射与接口文档一致。 | `grep -c "211101" Docs/API/KLine/KLineAdd.md` 与 `grep -c "409" Docs/API/KLine/KLineAdd.md` 均不小于 `1` | |
| 10253 | 未归类的错误兜底复用 HTTP 错误码（500 用 `2006`）。 | `go test ./internal/api/handler/ -run TestErrorResponderUnknownErrorMasked` 通过 | |
| 10254 | 存量接口旧号 `4001/4004/4009` 已停用。 | `grep -rn '"code": *\(4001\|4004\|4009\)' internal/ cmd/` 输出为空 | |
| 10255 | 错误响应默认（safe）不含 `debug` 与内部细节。 | `go test ./internal/api/handler/ -run TestErrorResponderSafeByDefault` 通过 | |
| 10256 | `admin` 模式 + 正确 `X-Debug-Token` 才输出白名单 `debug`。 | `go test ./internal/api/handler/ -run TestErrorResponderAdminWithToken` 通过 | |
| 10257 | 令牌缺失/错误时静默回退 safe 且不提示令牌错误。 | 同上用例覆盖 | |
| 10258 | 来源 IP 命中 `FINV_ERROR_DETAIL_ALLOWED_CIDRS` 视为高权限。 | `grep -c "AllowedCIDRs" internal/config/config.go` 不小于 `1`；`grep -c "Contains(ip)" internal/api/handler/errresp.go` 输出 `1` | |
| 10259 | `request_id` 为大写 UUID4。 | `go test ./internal/api/handler/ -run TestErrorResponderSafeByDefault` 通过 | |
| 10260 | `/Page` 返回 `{code:1,data:{cnt,ps,pn,list,request_id,ts},message:"OK"}`。 | `grep -c '"pn":         kline.PageNumber' internal/api/handler/kline.go` 输出 `1`；手工调用 `/Page` 校验字段 | |
| 10261 | `/Page` 的 `cr` 为两位小数文本（HALF_UP），NULL 为 `null`。 | `go test ./internal/meta/finv/quote/cdeu/kline/ -run TestRatioText` 通过 | |
| 10262 | `/PageUni` 的 `column` 固定 15 项且 `row` 值全部字符串化/去尾零。 | `go test ./internal/meta/finv/quote/cdeu/kline/ -run TestRowUniRow` 通过 | |
| 10263 | `PATCH /Batch` 区分「未传」与「显式 null」。 | `go test ./internal/meta/finv/quote/cdeu/kline/ -run TestPatchExplicitNull` 通过 | |

## 二十六、涨跌字段补全规则（10264–10269）

> 来源：导入/写入时 `change_price`、`change_ratio` 缺失的自动补全规则（ACANX 2026-10-24）。
> 关联：`internal/meta/finv/quant/calc/change.go`、`internal/meta/finv/quote/mvsv/parser.go`、`internal/meta/finv/quote/service.go`、`internal/meta/finv/quote/cdeu/kline/request.go`。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10264 | MVSV 文件的 `cp`/`cr` 列有值时按文件值落库，不做重算。 | `go test ./internal/meta/finv/quote/mvsv/ -run TestParseChangeFields` 通过；导入含 `cp/cr` 的文件后直查两列等于文件值 | |
| 10265 | `cp`/`cr` 均缺失且 `last_close` 非空非 0 时，用 `last_close` 作为参考价计算。 | `go test ./internal/meta/finv/quote/ -run TestComputeChangeFields` 通过（用例 2） | |
| 10266 | `last_close` 缺失或为 0 且 `open` 非空非 0 时，用 `open` 作为参考价计算。 | 同上（用例 3、4）通过 | |
| 10267 | `last_close` 与 `open` 都不可用时保持 NULL，不报错。 | 同上（用例 5）通过 | |
| 10268 | KLine API 写入未提供 `change_price`/`change_ratio` 时按 `open` 补全；任一列有值时不修改。 | `grep -c "calc.Change" internal/meta/finv/quote/cdeu/kline/request.go` 输出不小于 `1`；`go test ./internal/meta/finv/quote/cdeu/kline/` 通过 | |
| 10269 | 涨跌计算使用十进制精确运算（big.Rat）并保留 6 位小数。 | `go test ./internal/meta/finv/quant/calc/ -run TestChangeRule` 通过（含 6 位小数用例） | |

## 二十七、去重行情前端菜单与统计（10270–10275）

> 来源：去重行情（`dedup` schema）前端查询与统计可视化需求（ACANX 2026-10-24）。
> 关联：`Web/src/App.vue`、`Web/src/router.ts`、`Web/src/views/Meta/Finv/Quote/DeDup/*`、`internal/meta/finv/quote/cdeu/kline/stats.go`、`internal/api/handler/kline.go`。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10270 | 前端新增一级菜单「去重行情」，下设「去重分钟行情查询」「去重分钟行情统计」两个子菜单。 | `grep -c "dedup-kline-query\|dedup-kline-stats" Web/src/App.vue` 不小于 `2`；`grep -c "DeDupKLine" Web/src/router.ts` 不小于 `4` | |
| 10271 | 查询页支持按 `usc` 与交易日期范围（含）分页查询去重后的分钟行情。 | 打开 `/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Query`，输入 `GCMain` + 日期 `2026-08-06`，结果总数与 `select count(*) from dedup.finv_quote_secu_kline_min where usc='GCMain' and date=20260806;` 一致 | |
| 10272 | 查询页支持页码/每页条数切换、重置，且开始日期晚于结束日期时前端拦截。 | 切换每页条数后请求 `offset` 随之变化；输入倒序日期点击查询不发起请求并提示 | |
| 10273 | 新增 `/Stats` 接口，返回 `summary`（含 `null_counts`）、`daily`、`top_usc`，三组口径在同一只读事务内一致。 | `go test ./internal/api/ -run TestKLineRoutesRegistered` 通过；请求 `GET .../Stats` 返回 200 且 `summary.rows` 与 `Count` 接口一致 | |
| 10274 | 统计页展示 KPI、字段完整性、每日记录数/成交量、每日收盘价均值、证券 Top 20 图表。 | 打开 `/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Stats`，三类图表均渲染且切换证券/日期后刷新 | |
| 10275 | 新增菜单文档与接口文档，并在 `Menus.md`、`APIs.md`、`README.md`、`CHANGELOG.md` 同步索引。 | `Docs/Menu/HistoryQuote/DeDupKLineQuery.md`、`DeDupKLineStats.md`、`Docs/API/KLine/KLineStats.md` 存在且 `Menus.md`/`APIs.md` 含对应条目 | |
| 10276 | 「去重行情」一级菜单更名为「数据采集」，并在其下新增二级菜单「行情数据去重」，原「去重分钟行情查询」「去重分钟行情统计」作为该二级菜单的三级子菜单。 | `grep -A16 "key: 'data-collection'" Web/src/App.vue` 可见 `title: '数据采集'` 与 `key: 'quote-dedup'`；`Docs/Menu/Menus.md` 层级结构为 数据采集 → 行情数据去重 → 两个叶子 | |
| 10277 | 去重行情查询/统计页面的数字展示默认不使用千分位分隔符（大额统计保留 `万/亿/万亿` 缩写）。 | 查询页总数显示 `2729329` 而非 `2,729,329`；表格数值为 `1224.3`；统计页记录总数显示 `2804329`；`grep -c "toLocaleString" Web/src/views/Meta/Finv/Quote/DeDup/*.vue` 为 `0` | |
| 10278 | 前端 `time` 字段值为 0 时展示 `00:00:00`，仅 `null/undefined` 展示 `-`。 | `grep -n "t === undefined \|\| t === null" Web/src/utils.ts` 命中；查询 `2026-07-20` 首行时间显示 `00:00:00` | |
| 10279 | 查询页/统计页开始与结束日期默认最近一个月，不再默认缺省；结束日取库内最新交易日期，取不到时回退本机当天。 | 打开两个页面，日期框默认 `2026-06-21` ~ `2026-07-21`（当前库最新交易日 2026-07-21），且首屏有数据 | |
| 10280 | 查询页/统计页证券代码默认 `GCMain`。 | 打开两个页面，证券输入框默认为 `GCMain` | |
| 10281 | 新增 `V40` 复合索引支撑证券+日期范围分页；`GCMain + 最近一个月` 分页查询耗时小于 1 秒。 | `select indexname from pg_indexes where tablename='finv_quote_secu_kline_min';` 含 `idx_finv_quote_usc_date_ts`、`idx_finv_quote_date_usc_ts`；`curl` 实测耗时 < 1s | |
| 10282 | 去重行情统计页的「每日记录数与成交量」「每日收盘价均值」「字段完整性（空值行数）」各自独占一行；图表坐标轴不使用千分位。 | 打开统计页，「字段完整性」位于日期汇总与每日图表之间的独立行；两个每日图表上下排列且宽度相同；坐标轴显示 `2500`、`4250` 而非 `2,500`、`4,250` | |
| 10283 | 侧边导航一级→二级、二级→三级菜单缩进各减少为原来的 50%，三级菜单标题完整展示不被遮盖。 | 浏览器计算样式：二级行 `padding-left=36px`、三级行 `64px`（原 `64px`/`120px`）；三级标题 `去重分钟行情查询` 宽度约 115px 且无省略号 | |
| 10284 | 统计页「每日记录数与成交量」拆分为「每日记录数」「每日成交量」两个图表；与「每日收盘价均值」三个图表的时间轴（绘图区左右边界与刻度）对齐。 | 统计页可见 3 个每日图表卡片且宽度相同；三个 canvas 的 `x` 与 `width` 一致（实测 `x=309,w=1042`）；x 轴日期标签位置一致 | |
| 10285 | 侧边导航一级→二级、二级→三级缩进在上一版基础上再减半（每级增量 28px → 14px）。 | 计算样式：二级行 `padding-left=22px`、三级行 `36px`（上一版 36/64）；三级标题完整显示 | |
| 10286 | 侧边菜单图标与标题的水平间距调整为原来的 1/3（32px → 约 10.67px）。 | 浏览器实测图标右边界到标题左边界 `gap≈11px`（原 32px）；`--v-list-prepend-gap` 计算值为 `10.6562px` | |
| 10287 | 前端入口 `index.html`（含 SPA 回退）禁止缓存，保证前端重建后立即加载新资源；内嵌前端产物与 `Web/dist` 保持一致。 | `curl -I http://<前端端口>/` 返回 `Cache-Control: no-cache, no-store, must-revalidate`；`grep -o "index-[^\"]*\.js" internal/webui/dist/index.html` 与 `Web/dist/index.html` 结果一致 | |
| 10288 | 统计页在「每日成交量」之后新增「每日成交额」图表，与「每日记录数」「每日成交量」「每日收盘价均值」时间轴对齐。 | 统计页存在 4 个每日图表卡片，宽度一致，canvas `x/w` 一致（实测 `x=309,w=1042`），y 依次递增 | |
| 10289 | 新增 `/Bounds` 轻量接口用于默认日期范围初始化，`GCMain` 查询耗时小于 100ms。 | `curl` 请求 `/Bounds?usc=GCMain` 返回 `date_from/date_to` 且耗时 < 100ms（实测约 17ms）；两个页面首屏默认日期为 2026-06-21 ~ 2026-07-21 | |
| 10290 | 「数据采集」一级菜单移动到「历史行情」之前（紧跟仪表盘）。 | `grep -n "key: 'data-collection'\|key: 'history-quote'" Web/src/App.vue` 中 data-collection 行号小于 history-quote；浏览器侧边栏顺序为 仪表盘 → 数据采集 → 历史行情 | |

## 二十八、行情月归档导出与上传（10291–10310）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10291 | 迁移 `V41` 在 `dedup` schema 新增 12 张归档链路表。 | `grep -c 'CREATE TABLE IF NOT EXISTS dedup.finv_dedup_' Deploy/Migrations/V200041__finv_dedup_export_archive_tables.sql` 输出 `12` | |
| 10292 | 新增表全部位于 `dedup` schema 且全库无外键，不修改既有两张业务表。 | `grep -c 'REFERENCES' Deploy/Migrations/V200041__finv_dedup_export_archive_tables.sql` 输出 `0`；`grep -c 'ALTER TABLE dedup.finv_quote_secu_kline_min' Deploy/Migrations/V200041__finv_dedup_export_archive_tables.sql` 输出 `0` | |
| 10293 | 12 个普通索引按 `idx_<表名去 finv_ 前缀并去下划线>_<用途>` 命名。 | `grep -c 'CREATE INDEX IF NOT EXISTS idx_dedup' Deploy/Migrations/V200041__finv_dedup_export_archive_tables.sql` 输出 `12` | |
| 10294 | 导出文件为 MVsv：16 行 `# ` 文件头 + 1 行空行 + 数据行。 | `go test ./internal/meta/finv/quote/dedup -run TestWriteMVsvStructure` 通过 | |
| 10295 | `# Field` 行固定为 `ts\|da\|ti\|o\|h\|l\|c\|v\|t\|cp\|cr\|lc`。 | `go test ./internal/meta/finv/quote/dedup -run TestWriteMVsvStructure` 通过（断言 Field 行） | |
| 10296 | `# Count` 等于实际数据行数，不一致时写出失败且不留半成品。 | `go test ./internal/meta/finv/quote/dedup -run 'TestWriteMVsvRowCountMismatch\|TestVerifyMVsvRejectsBrokenFiles'` 通过 | |
| 10297 | BASE / PATCH / MANIFEST 文件名符合方案稿约定。 | `go test ./internal/meta/finv/quote/dedup -run TestFileNames` 通过 | |
| 10298 | 上传与本地同名（`.mvsv` / `.json`），不生成、不支持 `.csv`。 | `grep -rn '\.csv' internal/meta/finv/quote/dedup` 无输出；`go test ./internal/meta/finv/quote/dedup -run TestFileNames` 通过 | |
| 10299 | 导出任务以 `(usc, biz_month)` 唯一，保证幂等。 | `grep -F 'UNIQUE (usc, biz_month)' Deploy/Migrations/V200041__finv_dedup_export_archive_tables.sql` 命中 | |
| 10300 | LOAD 按日 keyset 分页、`ON CONFLICT (ts, usc) DO NOTHING` 幂等、按 `usc` + 日期范围清理。 | `grep -F 'ON CONFLICT (ts, usc) DO NOTHING' internal/meta/finv/quote/dedup/store.go` 命中；`grep -F 'DELETE FROM ' internal/meta/finv/quote/dedup/store.go` 命中 | |
| 10301 | 窗口耗尽时保存游标转 `PAUSED`（预算只限制启动新任务，不中断已启动任务），续跑前校验 `loaded_rows` 与实际行数不一致则重置游标。 | `grep -F 'ResetLoadCursor' internal/meta/finv/quote/dedup/job_export.go` 命中；`grep -F 'JobStatusPaused' internal/meta/finv/quote/dedup/job_export.go` 命中 | |
| 10302 | 多 worker 用行锁 + 租约 + 心跳安全领取（`FOR UPDATE SKIP LOCKED`）。 | `grep -F 'FOR UPDATE SKIP LOCKED' internal/meta/finv/quote/dedup/store.go` 命中；`grep -F 'make_interval(secs' internal/meta/finv/quote/dedup/store.go` 命中 | |
| 10303 | 日预算按公式计算并落库（`factor = clamp(1.20 + 0.20*sla + 0.05*min(backlog,5), 1.20, 2.00)`）。 | `go test ./internal/meta/finv/quote/dedup -run TestComputeBudget` 通过 | |
| 10304 | 导出候选要求该月所有已观测日均为 `COMPLETE`/`FROZEN` 且月份已结束。 | `grep -F "HAVING bool_and(m.maturity_status IN ('COMPLETE', 'FROZEN'))" internal/meta/finv/quote/dedup/store.go` 命中；`grep -F 'm.biz_month < $1' internal/meta/finv/quote/dedup/store.go` 命中 | |
| 10305 | 每个物理文件登记到 `finv_dedup_export_file`，唯一键 `(usc, biz_month, file_type, version)`。 | `grep -F 'UNIQUE (usc, biz_month, file_type, version)' Deploy/Migrations/V200041__finv_dedup_export_archive_tables.sql` 命中 | |
| 10306 | MANIFEST 在同一证券月 BASE / PATCH 全部上传完成后才领取。 | `grep -F "f.file_type <> 'MANIFEST' OR NOT EXISTS" internal/meta/finv/quote/dedup/store_files.go` 命中 | |
| 10307 | 上传前后校验大小与 SHA256，每次尝试写上传审计日志。 | `grep -F 's.uploader.Verify' internal/meta/finv/quote/dedup/job_upload.go` 命中；`grep -F 'InsertUploadLog' internal/meta/finv/quote/dedup/job_upload.go` 命中 | |
| 10308 | 收口后数据按 L1_HOT / L2_WARM / L3_COLD 分级补丁，并登记 `finv_dedup_export_file_patch`。 | `grep -F 'PatchLevelL1Hot' internal/meta/finv/quote/dedup/job_patch.go` 命中；`grep -F 'InsertPatchTask' internal/meta/finv/quote/dedup/job_patch.go` 命中 | |
| 10309 | 生命周期清理默认不删主表数据，需显式开关；本地文件需「上传成功 + 远端校验 + 网盘归档」才清理。 | `grep -F 'FINV_QUOTE_DEDUB_PURGE_MAIN_ENABLED' internal/config/config.go` 命中；`grep -F "network_status = 'ARCHIVED'" internal/meta/finv/quote/dedup/store_files.go` 命中 | |
| 10310 | 健康检查输出指标与告警，并登记 10 个 Job 的调度表。 | `go test ./internal/meta/finv/quote/dedup -run 'TestBuildAlerts\|TestJobRuntimes'` 通过 | |

## 二十九、三种发布包的 CI/CD（10311–10320）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10311 | 快照包在推送代码到分支时触发，版本为 `v{VERSION}.{PPPPP}.{yymmddHHmm}`。 | `grep -F 'v${BASE}.${PR5}.${TS}' .github/workflows/CI.yml` 命中 | |
| 10312 | 构建包在 PR 合并时触发，版本为 `v{VERSION}.{PPPPP}`（不含时间戳）。 | `grep -F 'VERSION="v${BASE}.${PR5}"' .github/workflows/CI.yml` 命中 | |
| 10313 | 正式版包在代码合并到 `main` 时触发，版本为 `v{VERSION}`。 | `grep -F 'VERSION="v${BASE}"' .github/workflows/CI.yml` 命中 | |
| 10314 | 三个 binaries job（`snapshot-binaries` / `pr-binaries` / `release-binaries`）同级平行存在。 | `grep -c '^  [a-z-]*-binaries:$' .github/workflows/CI.yml` 输出 `3` | |
| 10315 | 构建包采用 B 策略：同名 release/tag 已存在时删除后重建。 | `grep -F 'gh release delete "${VER}" --yes --cleanup-tag' .github/workflows/CI.yml` 命中 | |
| 10316 | 三种场景互斥：提交已关联已合并 PR 时不出快照包。 | `grep -F '改由 pr 场景构建' .github/workflows/CI.yml` 命中 | |
| 10317 | 正式版 tag 已存在时跳过并告警（需先 bump `VERSION`）。 | `grep -F '请先 bump VERSION' .github/workflows/CI.yml` 命中 | |
| 10318 | `PackSnapshot.sh` 支持三种版本格式与 `--kind` 参数。 | `grep -F -- '--kind)         KIND=' Deploy/Script/PackSnapshot.sh` 命中 | |
| 10319 | 包内 `BUILD-INFO.txt` 标注包类型（snapshot / pr / release）。 | `grep -F '包类型      : ${KIND}' Deploy/Script/PackSnapshot.sh` 命中 | |
| 10320 | 手动触发（`workflow_dispatch`）可选择包类型。 | `grep -F 'options: [snapshot, pr, release]' .github/workflows/CI.yml` 命中 | |

## 三十、前端月归档导出控制台（10321–10330）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10321 | 新增 3 个归档页面：总览、导出任务、文件与上传。 | `ls Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExport{Overview,Job,File}View.vue` 列出 3 个文件 | |
| 10322 | 3 个页面在 `router.ts` 注册独立路由。 | `grep -c "path: '/Meta/Finv/Quote/DeDup/Export/" Web/src/router.ts` 输出 `3` | |
| 10323 | 菜单挂到「数据采集 → 行情数据归档」（二级分组 + 3 个三级叶子）。 | `grep -F '行情数据归档' Web/src/App.vue` 命中 | |
| 10324 | 总览页调用 `GET .../DeDup/Status` 展示指标、告警与运行参数。 | `grep -F 'Meta/Finv/Quote/DeDup/Status' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportOverviewView.vue` 命中 | |
| 10325 | 总览页支持手动触发 Job（`POST .../Job/Run`）。 | `grep -F 'Meta/Finv/Quote/DeDup/Job/Run' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportOverviewView.vue` 命中 | |
| 10326 | 任务页调用 `Job/List` 分页查询并支持 `Job/Retry` 人工重试。 | `grep -F 'Meta/Finv/Quote/DeDup/Job/List' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportJobView.vue` 与 `grep -F 'Job/Retry'` 均命中 | |
| 10327 | 文件页调用 `File/List` 分页查询并支持 `File/Retry` 重试上传。 | `grep -F 'Meta/Finv/Quote/DeDup/File/List' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportFileView.vue` 与 `grep -F 'File/Retry'` 均命中 | |
| 10328 | 文件页提供「仅看已上传」快捷筛选（`uploadStatus=UPLOADED`）。 | `grep -F "uploadStatus.value = 'UPLOADED'" Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportFileView.vue` 命中 | |
| 10329 | 新增通用格式化工具 `fmtISO` / `fmtBytes` / `fmtMonth`。 | `grep -F 'export function fmtISO' Web/src/utils.ts` 命中 | |
| 10330 | 前端类型检查与生产构建通过。 | `cd Web && npm run build` 成功（`vue-tsc -b && vite build`） | |

## 三十一、归档运维台与后续工程化修复（10331–10356）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10331 | `GET .../Usc/Page` 分页查询证券主数据。 | `grep -F 'ListUscs' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10332 | `PUT .../Usc` 新增或更新证券主数据（含 `day_expect_rows`）。 | `grep -F 'func (s *Store) UpsertUsc' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10333 | `GET .../Priority/List` 查询优先级 / SLA。 | `grep -F 'func (s *Store) ListPriorities' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10334 | `PUT .../Priority` 新增或更新优先级。 | `grep -F 'func (s *Store) UpsertPriority' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10335 | `DELETE .../Priority` 删除优先级。 | `grep -F 'func (s *Store) DeletePriority' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10336 | `GET .../Calendar/Page` 分页查询交易日历。 | `grep -F 'func (s *Store) ListCalendar' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10337 | `PUT .../Calendar` 新增或更新交易日历标记。 | `grep -F 'func (s *Store) UpsertCalendar' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10338 | `DELETE .../Calendar` 删除交易日历记录。 | `grep -F 'func (s *Store) DeleteCalendar' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10339 | `GET .../Maturity/Page` 分页查询成熟度。 | `grep -F 'func (s *Store) ListMaturity' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10340 | `POST .../Maturity/Refresh` 异步触发成熟度重算。 | `grep -F 'MaturityRefresh' internal/api/handler/cdeu_admin.go` 命中 | |
| 10341 | `GET .../Stats/Page` 分页查询入口日统计。 | `grep -F 'func (s *Store) ListIngestStats' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10342 | `GET .../Budget/Page` 分页查询每日预算。 | `grep -F 'func (s *Store) ListBudgets' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10343 | `PUT .../Budget/Override` 覆盖当日预算（校验因子区间与硬上限）。 | `grep -F 'func (s *Store) OverrideBudget' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10344 | `GET .../Watermark/List` 查询扫描水位。 | `grep -F 'func (s *Store) ListWatermarks' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10345 | `PUT .../Watermark` 重置水位（RFC3339 校验）。 | `grep -F 'func (s *Store) ResetWatermark' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10346 | `GET .../RunLog/Page` 分页查询运行日志。 | `grep -F 'func (s *Store) ListRunLogs' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10347 | `GET .../UploadLog/Page` 分页查询上传审计。 | `grep -F 'func (s *Store) ListUploadLogs' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10348 | `GET .../Patch/Page` 分页查询补丁任务。 | `grep -F 'func (s *Store) ListPatches' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10349 | `POST .../Patch/Retry` 重试补丁任务（重置为 PENDING）。 | `grep -F 'func (s *Store) ResetPatchForRetry' internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10350 | `POST .../Job/Pause` 与 `POST .../Job/Cancel` 暂停 / 取消任务。 | `grep -F 'JobPause' internal/api/handler/cdeu_admin.go` 与 `grep -F 'JobCancel'` 均命中 | |
| 10351 | 所有写操作追加审计（`job_kind=MANUAL_ADMIN_EDIT`）。 | `grep -F "MANUAL_ADMIN_EDIT" internal/meta/finv/quote/dedup/store_admin.go` 命中 | |
| 10352 | 前端新增 3 个运维台页面（Stats / Config / Log）。 | `ls Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchive{Stats,Config,Log}View.vue` 列出 3 个文件 | |
| 10353 | `api.ts` 新增 `apiPut` / `apiDelete`。 | `grep -F 'export async function apiPut' Web/src/api.ts` 与 `grep -F 'export async function apiDelete'` 均命中 | |
| 10354 | `Docs/API/QuoteDeDup/` 新增 11 篇运维台接口文档。 | `ls Docs/API/QuoteDeDup/{Usc,Priority,Calendar,Maturity,Stats,Budget,Watermark,RunLog,UploadLog,Patch,JobControl}.md` 列出 11 个文件 | |
| 10355 | Dockerfile 不再出现 `RedundantTargetPlatform` 告警（移除 Go 阶段冗余的 `$TARGETPLATFORM`）。 | `grep -c '^FROM --platform=\$TARGETPLATFORM' Deploy/Docker/Dockerfile` 输出 `0`；`docker build --check -f Deploy/Docker/Dockerfile .` 输出 `no warnings found` | |
| 10356 | 依赖漏洞清零：Go 升到修复版，前端 `npm audit` 无漏洞。 | `grep -F 'golang.org/x/crypto v0.52.0' go.mod` 命中；`grep -F '"vite": "^8.3.5"' Web/package.json` 命中；`cd Web && npm audit` 输出 `found 0 vulnerabilities` | |

## 三十二、归档 Job 执行可视化与全链路流程图（10357–10386）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10357 | 后端新增 `GET /API/V1/Meta/Finv/Quote/DeDup/Job/Runtime`，返回 10 个 Job 的运行快照（阶段 / 进度 / 心跳 / 最近结果 / 24 小时统计）。 | `grep -F 'Job/Runtime' internal/api/router.go` 命中；`test -f Docs/API/QuoteDeDup/JobRuntime.md` 退出码为 0 | |
| 10358 | 运行日志表新增 `stage` / `progress_percent` / `progress_detail` / `gmt_heartbeat` 四列并建未结束记录部分索引。 | `grep -F 'progress_percent' Deploy/Migrations/V200042__finv_dedup_job_run_log_progress.sql` 命中；`grep -F 'idx_dedupjobrunlogexportfile_running' Deploy/Migrations/V200042__finv_dedup_job_run_log_progress.sql` 命中 | |
| 10359 | 导出 Job 在 LOAD / EXPORT / CLEANUP 阶段上报进度与心跳（写文件时按行数周期性刷新）。 | `grep -F '"LOAD"' internal/meta/finv/quote/dedup/job_export.go` 与 `grep -F 'written%20000' internal/meta/finv/quote/dedup/job_export.go` 均命中 | |
| 10360 | 上传 / 补丁 / 维护 / 统计类 Job 上报阶段进度（UPLOAD / PATCH / CLEANUP / RECOVER / HEALTH / STATS / BUDGET / MATURITY / SEAL）。 | `grep -rlF 'reportProgress(ctx' internal/meta/finv/quote/dedup/job_*.go \| wc -l` 输出 `7` | |
| 10361 | 侧边菜单新增「端到端链路」独立入口与路由。 | `grep -F 'dedup-export-chain' Web/src/App.vue Web/src/router.ts` 命中 2 处；`test -f Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 退出码为 0 | |
| 10362 | 流程页以流程图展示去重 → 成熟度/预算 → 收口 → 装载 → 导出 → 清理 → 上传 → 归档 → 补丁 → 保障的全链路节点状态与进度。 | `grep -F '总体视角 · 数据流向' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 命中；`grep -c "kind: 'main'," Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 输出 `11` | |
| 10363 | 流程页自动刷新运行快照（5 秒）与业务指标（20 秒），并汇总当前执行环节与告警。 | `grep -F 'setInterval(loadRuntime, 5000)' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 与 `grep -F 'statusTimer = setInterval(() =>'` 均命中 | |
| 10364 | 归档监控总览页的 Job 调度表保持「调度 + 手动触发」，不内联运行状态与历史记录。 | `grep -c 'Job/Runtime' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportOverviewView.vue` 输出 `0` | |
| 10365 | 低峰窗口支持运行时修改并立即生效（新增运行时设置表 + 服务端热更新）。 | `test -f Deploy/Migrations/V200043__finv_dedup_runtime_setting.sql` 退出码为 0；`grep -F 'SetWindow' internal/meta/finv/quote/dedup/service.go` 命中 | |
| 10366 | 新增 `GET /Settings` 与 `PUT /Settings/Window`（窗口来源 / 更新时间 / 校验 / 审计）。 | `grep -F 'Settings/Window' internal/api/router.go` 命中；`test -f Docs/API/QuoteDeDup/Settings.md` 退出码为 0 | |
| 10367 | 服务启动加载数据库运行时设置，未配置时回退环境变量。 | `grep -F 'LoadRuntimeSettings' cmd/server/main.go` 与 `grep -F 'LoadRuntimeSettings' internal/meta/finv/quote/dedup/service.go` 均命中 | |
| 10368 | 每个 Job 提供中文名称与功能 / 流程 / 工作细节 / 注意事项，前端在 Job 标识处悬浮展示（不新增列）。 | `grep -F 'Cautions' internal/meta/finv/quote/dedup/service.go` 命中；`grep -F 'jobDoc(' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 命中 | |
| 10369 | 「端到端链路」页提供低峰窗口编辑对话框（开始 / 结束时间、来源与更新时间），保存后刷新。 | `grep -F 'openWindowDialog' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 与 `grep -F 'saveWindow'` 均命中 | |
| 10370 | 总体视角以 ①–⑪ 数据流向图展示每一步的数据表、状态、阶段进度与业务数字。 | `grep -F '总体视角 · 数据流向' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 命中；`grep -F 'dataFlow:' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 命中 | |
| 10371 | 任务视角展示 10 个 Job 的说明、当前状态、最近运行与在总体链路中的步骤位置，并可跳转总体视角。 | `grep -F '在总体链路中的位置' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 命中；`grep -F 'stepsOfJob'` 命中 | |
| 10372 | 总体视角可定位卡点 / 错误步骤：卡点列表 + 步骤红 / 黄边框 + 错误信息。 | `grep -F '链路卡点' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 命中；`grep -F 'chain-error'` 命中 | |
| 10373 | 总体视角使用 SVG 绘制数据流向图（节点 + 箭头 + 节点内数据流文案），支持点击节点联动详情。 | `grep -F '<svg' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 与 `grep -F 'marker-end'` 均命中 | |
| 10374 | 旁路任务连接到下游步骤：补丁 → ⑤ 中转表（再经 ⑥→⑦→⑧→⑨）、超时回收 → 重新排队、健康检查 → 调度反馈。 | `grep -F 'patch-load' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue`、`grep -F 'recover-dispatch'`、`grep -F 'health-dispatch'` 均命中 | |
| 10375 | 定时循环任务（导出调度 / 上传 / 超时回收 / 健康检查）以紫色自环标识，自环从节点底部中点（出口）绕外侧回到顶部中点（入口），并显示调度周期。 | `grep -F 'LOOP_STEPS' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 与 `grep -F '自环规范'` 均命中 | |
| 10376 | 节点第三行展示关键指标，表名改为悬浮 / 详情面板展示。 | `grep -F 'chain-node-metric' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 命中；`grep -F 'currentStep.dataFlow'` 命中（详情面板） | |
| 10377 | ⑤–⑧ 包裹为「导出任务批次」大节点，展示成功 / 失败 / 总数与分段进度条，随 5 秒运行快照更新。 | `grep -F '导出任务批次' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 命中；`grep -F 'exportGroup'` 命中 | |
| 10378 | ⑤–⑧ 子步骤展示各自完成度（装载 / 导出 / 登记的进度条与关键指标）。 | `grep -F 'loadPercent' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 与 `grep -F 'exportPercent'` 均命中 | |
| 10379 | `GET /Status` 与 `GET /Job/Runtime` 返回当前月批次进度 `job_progress`。 | `grep -F 'job_progress' internal/api/handler/cdeu.go` 命中；`grep -F 'JobStatusCountsByMonth' internal/meta/finv/quote/dedup/store_files.go` 命中 | |
| 10380 | 流程图连接线使用正交折线，并通过实线（主流程）/虚线（旁路）/紫色（定时循环）区分，减少线与线重叠。 | `grep -F 'stroke-linejoin="round"' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 命中；`grep -c ' C ' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 输出 `0` | |
| 10381 | 旁路任务使用 ⑫ / ⑬ / ⑭ 独立序号；⑫ 作为本月导出批次的一部分，补丁数据先进入 ⑤ 中转表，再走 ⑥→⑦→⑧→⑨ 上传。 | `grep -F "no: '⑫'" Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue`、`no: '⑬'`、`no: '⑭'` 均命中；`grep -F "'补丁数据入中转表'"` 命中 | |
| 10382 | 补丁（L1/L2/L3）先分页装入中转表，再从转存表写 BASE/PATCH 文件，完成后清理中转数据。 | `grep -F 'LoadPatchPage' internal/meta/finv/quote/dedup/store.go` 与 `grep -F 'stagePatchRows' internal/meta/finv/quote/dedup/job_patch.go` 均命中 | |
| 10383 | 大数据量补丁不再按补丁小预算截断，仅记录告警并完整执行，避免历史重聚合长期无法收敛。 | `grep -F '仍按完整补丁执行' internal/meta/finv/quote/dedup/job_patch.go` 命中 | |
| 10384 | 流程图将 ⑫ 纳入「导出任务批次（本月）」范围，并标注补丁数据先进入 ⑤ 中转表，而非跳过中转表直接上传。 | `grep -F 'exportGroup' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 命中；`grep -F '补丁数据入中转表'` 命中 | |
| 10385 | 批次进度统计包含 ⑫ 补丁任务（成功 / 失败 / 运行中 / 待处理），与导出任务合并展示。 | `grep -F 'patch_total' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 命中；`grep -F 'PatchStatusCountsByMonth' internal/meta/finv/quote/dedup/store_files.go` 命中 | |
| 10386 | 新增 CDEU 抽象概念模型设计文档：Process / Phase / Step / Job 四层、运行模式、批处理、完成条件、表结构与 13 个 Job 登记映射。 | `test -f Docs/Refactor/2610-007-CDEUProcessModelDesign.md` 退出码为 0；`grep -c 'cdeu_process' Docs/Refactor/2610-007-CDEUProcessModelDesign.md` 输出大于 `0` | |
| 10387 | 规范要求过程文档 `yyMM-NNN-` 编号在 `Docs/` 下全局唯一，不得重号或复用，并提供自动校验。 | `grep -F '全局唯一' Docs/DevSpec/FileNamingSpec.md` 命中；`go test ./internal/docs` 通过（重号时失败） | |
| 10388 | 按 CDEU 模型新增 8 张 `dedup.cdeu_*` 表并 seed 1 个流程 / 4 个阶段 / 10 个步骤 / 13 个作业（V45 将旁路步骤并入导出/上传阶段）；现有 Job 执行时双写 CDEU 运行树并按完成条件逐级评估。 | `test -f Deploy/Migrations/V200044__finv_dedup_cdeu_process_model.sql` 退出码为 0；`grep -F 'CDEU_DAILY' Deploy/Migrations/V200044__finv_dedup_cdeu_process_model.sql` 命中；`go test ./internal/meta/finv/quote/dedup` 通过 | |
| 10389 | 前端新增「流程控制与监控」菜单，按 Process / Phase / Step / Job 展示 CDEU 运行状态、历史运行与手动触发；后端新增 `GET /CDEU/Overview`。 | `grep -F '/Meta/Finv/Quote/Cdeu/CdeuOverview' Web/src/router.ts Web/src/config/menuConfig.ts` 命中；`grep -F 'CDEU/Overview' internal/api/router.go` 命中；`cd Web && npm run build` 通过 | |
| 10390 | 流程控制页默认「总体视图」，阶段悬浮展示概要；支持启动完整 CDEU 流程（`POST /CDEU/Process/Run`）。 | `grep -F 'viewMode' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'v-tooltip' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'CDEU/Process/Run' internal/api/router.go` 命中；`cd Web && npm run build` 通过 | |
| 10391 | 总体视图点击阶段后，在「阶段概览」位置展示该阶段的步骤清单表格（步骤状态、作业/批次进度、开始时间、耗时、异常），不再重复展示阶段汇总表。 | `grep -F 'selectedPhase' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'selectedPhase.steps' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10392 | 「阶段流转」改为画布式工作流：悬浮节点 + 点阵画布 + 阶段间有向箭头，箭头由前序阶段指向后续阶段，点击节点切换步骤清单。 | `grep -F 'workflow-canvas' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'marker-end' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10393 | 删除尾部「保障（旁路）」阶段，将超时回收步骤并入导出阶段、健康检查步骤并入上传阶段，与正常步骤并行展示且不参与完成判定。 | `test -f Deploy/Migrations/V200045__finv_dedup_cdeu_split_ops_steps.sql` 退出码为 0；`grep -F 'PHASE_OPS' Deploy/Migrations/V200045__finv_dedup_cdeu_split_ops_steps.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10394 | 在「阶段概览」步骤表格中点击某一步骤，下方展示该步骤定义的 JOB 清单（作业、调度标识、执行阶段、状态、开始时间、耗时、处理量、操作）。 | `grep -F 'selectedStep' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'selectedStep.jobs' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10395 | 运行中的阶段在画布节点上显示脉冲高亮/流光连线，运行中的步骤在步骤行与详情头部显示高亮与呼吸动效，便于一眼定位当前执行位置。 | `grep -F 'workflow-node-running' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'step-row-running' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10396 | 将「运行历史」改为「最新运行」位置的下拉列表，支持选择历史运行切换；默认展示最近一次运行，启动流程后展示当前运行。 | `grep -F 'run-select-btn' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'previousRuns' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10397 | 所有执行中的状态图标改为旋转动效（阶段画布、步骤表格、详情、JOB 清单、历史列表），并遵循 reduced-motion 设置。 | `grep -F 'spin-icon' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F "mdi-loading" Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10398 | 状态图例移入「阶段流转」画布卡片，支持显示/隐藏，并移除右侧独立图例卡片。 | `grep -F 'legendVisible' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'workflow-legend' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10399 | 移除独立「流程规则」卡片，将完成条件/失败策略并入页面头部摘要，时区/查询时间与内嵌阶段说明收进信息悬浮提示；步骤/ JOB 区域改为全宽展示。 | `grep -F '流程规则' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 应无输出；`grep -F '完成条件' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10400 | 阶段流转画布增加「开始」「结束」首尾节点，并用有向箭头与首个/最后一个阶段连接；开始节点随运行存在显示已开始，结束节点随流程终态显示完成/失败/等待。 | `grep -F 'workflowStart' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'workflowEnd' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10401 | 数据采集阶段增加非关键「历史行情数据采集」占位步骤，步骤下无 JOB，流程初始化时自动成功，并保留后续接入历史采集 Job 的位置。 | `test -f Deploy/Migrations/V200046__finv_dedup_cdeu_history_ingest_step.sql` 退出码为 0；`grep -F 'STEP_HISTORY_INGEST' Deploy/Migrations/V200046__finv_dedup_cdeu_history_ingest_step.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10402 | 阶段步骤表将「步骤名称」「步骤 KEY」拆为两列，JOB 表将「作业名称」「调度标识」拆为两列；四列均支持拖拽调整列宽。 | `grep -F 'stepNameWidth' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'col-resizer' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10403 | 在去重收口与导出之间增加 `PHASE_DECISION`「导出判定」阶段及 `STEP_EXPORT_DECISION` 占位步骤，完成导出前检查判定；当前无 JOB，初始化自动成功。 | `test -f Deploy/Migrations/V200047__finv_dedup_cdeu_decision_phase.sql` 退出码为 0；`grep -F 'PHASE_DECISION' Deploy/Migrations/V200047__finv_dedup_cdeu_decision_phase.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10404 | JOB 清单不再把名称/触发方式/执行次数/异常堆叠在「作业名称」单元格：拆为作业名称、作业编码、调度标识、触发方式、执行次数、异常等独立列；作业名称/编码/调度标识支持列宽拖拽。 | `grep -F 'jobCodeWidth' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F '执行次数' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10405 | JOB 清单「执行阶段」列文字颜色统一为暖色 `rgb(251, 140, 0)`，阶段详情中的执行阶段标签同步。 | `grep -F 'rgb(251, 140, 0)' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10406 | 将数据采集阶段的 `STEP_INGEST_STATS`、`STEP_BUDGET` 调整到 `PHASE_DECISION` 导出判定阶段执行，JOB 随步骤迁移。 | `test -f Deploy/Migrations/V200048__finv_dedup_cdeu_move_stats_to_decision.sql` 退出码为 0；`grep -F 'STEP_INGEST_STATS' Deploy/Migrations/V200048__finv_dedup_cdeu_move_stats_to_decision.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10407 | 将 `STEP_MATURITY` 从去重收口阶段调整到 `PHASE_DECISION` 导出判定阶段执行，JOB 随步骤迁移；去重收口保留收口冻结。 | `test -f Deploy/Migrations/V200049__finv_dedup_cdeu_move_maturity_to_decision.sql` 退出码为 0；`grep -F 'STEP_MATURITY' Deploy/Migrations/V200049__finv_dedup_cdeu_move_maturity_to_decision.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10408 | CDEU 工作流模型重构为 `Workflow → Phase → Process → JOB → Step`：旧 Process 改为 Workflow、旧 Step 改为 Process、JOB 改为 JOB，并在 JOB 下新增串行 Step；数据库表/字段、API JSON、前端表述与启动路径同步调整。 | `test -f Deploy/Migrations/V200050__finv_dedup_cdeu_workflow_process_jobstep.sql` 退出码为 0；`grep -F 'cdeu_job_step' Deploy/Migrations/V200050__finv_dedup_cdeu_workflow_process_jobstep.sql` 命中；`grep -F 'workflow' internal/meta/finv/quote/dedup/cdeu_admin.go` 命中；`grep -F 'Process KEY' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup ./internal/api` 通过 | |
| 10409 | 将「收口冻结与差异报告」统一改名为「收口冻结与差异报告生成」，覆盖 Process、JOB、JOB Step 与运行快照展示名称。 | `test -f Deploy/Migrations/V200051__finv_dedup_cdeu_rename_seal_process.sql` 退出码为 0；`grep -F '收口冻结与差异报告生成' Deploy/Migrations/V200051__finv_dedup_cdeu_rename_seal_process.sql` 命中；`grep -F '收口冻结与差异报告生成' internal/meta/finv/quote/dedup/service.go` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10410 | 将 `PHASE_DECISION` 阶段名称由「导出判定」调整为「导出决策」。 | `test -f Deploy/Migrations/V200052__finv_dedup_cdeu_rename_decision_phase.sql` 退出码为 0；`grep -F "name = '导出决策'" Deploy/Migrations/V200052__finv_dedup_cdeu_rename_decision_phase.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10411 | 将 `PHASE_COLLECT` 命名为「历史行情采集」、`PHASE_DEDUP` 命名为「历史行情去重」，并在上传后新增 `PHASE_REVIEW`「归档复核」阶段（占位 Process，初始化自动成功）。 | `test -f Deploy/Migrations/V200053__finv_dedup_cdeu_phase_rename_review.sql` 退出码为 0；`grep -F "PHASE_REVIEW" Deploy/Migrations/V200053__finv_dedup_cdeu_phase_rename_review.sql` 命中；`grep -F "历史行情采集" Deploy/Migrations/V200053__finv_dedup_cdeu_phase_rename_review.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10412 | 「阶段流转」标题改为「CDEU工作流」；阶段间距扩大一倍，同时缩小阶段间箭头，使箭头之间的线段有足够展示宽度。 | `grep -F 'CDEU工作流' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'const WORKFLOW_GAP = 48' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'markerWidth="5"' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10413 | 画布「开始」「结束」节点的 icon 放在文字左侧，文字改为上下两行（标题 / 状态）；终端节点宽度同步扩大以容纳横向布局。 | `grep -F 'workflow-terminal-text' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'const WORKFLOW_TERMINAL_WIDTH = 96' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10414 | 将 `PHASE_EXPORT` 名称改为「月度行情导出」、`PHASE_UPLOAD` 名称改为「月度行情上传」。 | `test -f Deploy/Migrations/V200054__finv_dedup_cdeu_rename_export_upload_phases.sql` 退出码为 0；`grep -F "PHASE_EXPORT" Deploy/Migrations/V200054__finv_dedup_cdeu_rename_export_upload_phases.sql` 命中；`grep -F "月度行情导出" Deploy/Migrations/V200054__finv_dedup_cdeu_rename_export_upload_phases.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10415 | 前端「阶段概览」改为「流程概览」；Process 下的 JOB 标题改为「JOB 清单 · `<Process 名称>`」顺序。 | `grep -F '流程概览' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'JOB 清单 · {{ selectedProcess.name }}' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10416 | Process 表格将「关键」/「旁路」标识从 Process 名称单元格拆出，单独增加「标识」列展示。 | `grep -F '>标识<' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F '>关键<' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10417 | 移除流程概览标题右侧的「点击上方阶段切换 Process · 点击 Process 查看 JOB 清单 · 悬浮查看阶段概要」描述文字。 | `grep -F '点击上方阶段切换 Process' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 无输出；`cd Web && npm run build` 通过 | |
| 10418 | 工作流页头元信息由单行文字改为带图标的彩色 chip 可视化：工作流名称、运行模式、每日调度、完成条件、失败策略。 | `grep -F 'workflow-meta-row' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F '完成条件' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10419 | 将 `PROCESS_EXPORT_JOB` 名称由「单证券月导出」改为「证券历史月度行情文件导出」。 | `test -f Deploy/Migrations/V200055__finv_dedup_cdeu_rename_export_process.sql` 退出码为 0；`grep -F "证券历史月度行情文件导出" Deploy/Migrations/V200055__finv_dedup_cdeu_rename_export_process.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10420 | 将 `PROCESS_PATCH` 编码改为 `PROCESS_EXPORT_FILE_MON_INCR`、名称改为「证券历史月度行情增量文件导出」；将 `PROCESS_EXPORT_JOB` 编码改为 `PROCESS_EXPORT_FILE_MON`。 | `test -f Deploy/Migrations/V200056__finv_dedup_cdeu_rename_export_file_processes.sql` 退出码为 0；`grep -F "PROCESS_EXPORT_FILE_MON_INCR" Deploy/Migrations/V200056__finv_dedup_cdeu_rename_export_file_processes.sql` 命中；`grep -F "PROCESS_EXPORT_FILE_MON" Deploy/Migrations/V200056__finv_dedup_cdeu_rename_export_file_processes.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10421 | 在 `PROCESS_EXPORT_FILE_MON` 下新增 JOB `JOB_FETCH_SECURITY_METADATA`「证券元数据获取」，作为导出 JOB 的 META 阶段，在 LOAD 前刷新证券元数据。 | `test -f Deploy/Migrations/V200057__finv_dedup_cdeu_add_security_metadata_job.sql` 退出码为 0；`grep -F "JOB_FETCH_SECURITY_METADATA" Deploy/Migrations/V200057__finv_dedup_cdeu_add_security_metadata_job.sql` 命中；`grep -F "RefreshSecurityMetadata" internal/meta/finv/quote/dedup/store.go` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10422 | 在导出阶段新增 `PROCESS_EXPORT_BATCH`「证券导出批次清单获取」Process 及 `JOB_FETCH_EXPORT_BATCH` JOB，生成当批次 `usc + biz_month + BASE/INCR` 清单并写入 `dedup.finv_dedup_export_batch`；BASE/INCR 导出流程分别读取对应条目。 | `test -f Deploy/Migrations/V200058__finv_dedup_cdeu_export_batch_process.sql` 退出码为 0；`grep -F "finv_dedup_export_batch" Deploy/Migrations/V200058__finv_dedup_cdeu_export_batch_process.sql` 命中；`grep -F "GenerateExportBatch" internal/meta/finv/quote/dedup/store.go` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10423 | 将 BASE 导出 Process 编码由 `PROCESS_EXPORT_FILE_MON` 改为 `PROCESS_EXPORT_FILE_MON_BASE`；同时修复历史已完成运行新增 Process 后 CDEU 只读登记找不到运行树的问题。 | `test -f Deploy/Migrations/V200059__finv_dedup_cdeu_rename_base_export_process.sql` 退出码为 0；`grep -F "PROCESS_EXPORT_FILE_MON_BASE" Deploy/Migrations/V200059__finv_dedup_cdeu_rename_base_export_process.sql` 命中；`grep -F "SeedCDEUProcessRuns" internal/meta/finv/quote/dedup/cdeu.go` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10424 | 将 `PHASE_COLLECT` 名称由「历史行情采集」改为「历史行情收集」。 | `test -f Deploy/Migrations/V200060__finv_dedup_cdeu_rename_collect_phase.sql` 退出码为 0；`grep -F "历史行情收集" Deploy/Migrations/V200060__finv_dedup_cdeu_rename_collect_phase.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10425 | BASE/INCR 导出 Process 按批次清单动态生成单证券 JOB：以 `(usc, biz_month)` 全局唯一，写入 `dedup.cdeu_dynamic_job`；前端隐藏固定 LOAD/EXPORT/REGISTER/CLEANUP 执行壳 JOB，并展示证券、年月、BASE/INCR 类型及动态状态。 | `test -f Deploy/Migrations/V200061__finv_dedup_cdeu_dynamic_export_jobs.sql` 退出码为 0；`grep -F "cdeu_dynamic_job" Deploy/Migrations/V200061__finv_dedup_cdeu_dynamic_export_jobs.sql` 命中；`grep -F "GenerateDynamicJobs" internal/meta/finv/quote/dedup/store.go` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10426 | 每个动态单证券导出 JOB 内部拆为 `STEP_LOAD → STEP_EXPORT → STEP_REGISTER → STEP_CLEANUP` 四个串行 Step，运行时更新 Step 状态，前端展示对应 Step 信息。 | `test -f Deploy/Migrations/V200062__finv_dedup_cdeu_dynamic_job_steps.sql` 退出码为 0；`grep -F "cdeu_dynamic_job_step" Deploy/Migrations/V200062__finv_dedup_cdeu_dynamic_job_steps.sql` 命中；`grep -F "MarkDynamicJobStepStatus" internal/meta/finv/quote/dedup/store.go` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10427 | `cdeu_dynamic_job` 表改名为 `cdeu_job_dynamic`（Step 表同步为 `cdeu_job_dynamic_step`）；`PROCESS_UPLOAD` 基于已导出且未完成上传的月归档文件动态生成上传 JOB，每个文件一条并按 `file_id` 唯一，运行时同步 `STEP_UPLOAD` 状态。 | `test -f Deploy/Migrations/V200063__finv_dedup_cdeu_rename_job_dynamic_and_upload.sql` 退出码为 0；`grep -F "cdeu_job_dynamic" Deploy/Migrations/V200063__finv_dedup_cdeu_rename_job_dynamic_and_upload.sql` 命中；`grep -F "GenerateUploadDynamicJobs" internal/meta/finv/quote/dedup/store.go` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10428 | 点击阶段后，在工作流画布下方展示该阶段内 Process 的拓扑顺序图；顺序按 `seq_no` 生成，起点连接上一阶段输出、终点连接下一阶段输入，Process 节点可点击选中。 | `grep -F 'process-topology' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F '上一阶段输出' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10429 | 阶段内 Process 拓扑支持「流程拓扑」开关显示/隐藏，默认显示；导出阶段将 `PROCESS_RECOVER` 作为与 `PROCESS_EXPORT_FILE_MON_BASE` 并行的支路展示。 | `grep -F 'processTopologyVisible' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'processTopologyHasParallel' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10430 | 导出决策阶段按依赖调整：入口流量统计与行情成熟度扫描并行，每日导出预算计算在其后；调度改为 00:05 启动统计/成熟度、00:10 启动预算，拓扑图展示并行汇合关系。 | `test -f Deploy/Migrations/V200064__finv_dedup_cdeu_decision_parallel_schedule.sql` 退出码为 0；`grep -F "JOB_CALC_BUDGET" Deploy/Migrations/V200064__finv_dedup_cdeu_decision_parallel_schedule.sql` 命中；`grep -F "processTopologyMode" Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10431 | 画布上阶段与 Process 视觉样式区分：阶段为实线大卡片，Process 为虚线紧凑卡片；开始/结束节点由圆形改为矩形。 | `grep -F 'border-radius: 8px' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'border: 1px dashed' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10432 | 新增 `/Meta/Finv/Quote/DeDup/CDEUWorkflow` 菜单与「全流程拓扑」页面：每个阶段以虚线框圈定范围，框内展开该阶段全部 Process，并绘制阶段内及跨阶段的完整依赖连接。 | `grep -F "path: '/Meta/Finv/Quote/Cdeu/CdeuWorkflow'" Web/src/router.ts` 命中；`grep -F "全流程拓扑" Web/src/App.vue` 命中；`test -f Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 退出码为 0；`cd Web && npm run build` 通过 | |
| 10433 | 「全流程拓扑」图例绘制在拓扑画布内部，随画布一起滚动，不再放在画布外部的页面区域。 | `grep -F 'fulltopo-legend-in-canvas' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'legendTop' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10434 | 「全流程拓扑」中阶段内部并行流程的分叉点绘制在阶段虚线框内部；所有阶段虚线框高度统一，宽度按该阶段流程列数自适应。 | `grep -F 'FORK_SPACE' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'phaseHeight' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'fulltopo-fork' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10435 | 「全流程拓扑」去掉阶段虚线框左侧的绿色实边，阶段框仅保留虚线边界；增大阶段内分叉点到 Process 入口的间距，分叉后的折线、箭头与节点入口保持清晰距离。 | `grep -F '.fulltopo-phase::before' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 无命中；`grep -F 'const FORK_SPACE = 72' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'to.x - 16' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10436 | 修正阶段 2 → 阶段 3 的入口连线：跨阶段箭头进入虚线框后先竖向换行，再横向进入分叉点，形成完整 Z 字折线，箭头不再与竖线重叠。 | `grep -F '形成完整 Z 字折线' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'PHASE_PAD + 24' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10437 | 「全流程拓扑」按运行状态实时同步：运行中每 2 秒刷新、空闲每 10 秒刷新，页面重新可见时立即刷新；显示运行状态，运行中的阶段/流程节点带动态高亮，避免页面停留在静态快照。 | `grep -F 'refreshInterval' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'fulltopo-node-running' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'visibilitychange' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10438 | 「全流程拓扑」不再作为侧边栏菜单项；在「工作流控制与监控」页面头部提供「全流程拓扑」链接，点击后在新页面打开 `/Meta/Finv/Quote/DeDup/CDEUWorkflow`。 | `grep -F 'dedup-export-cdeu-workflow' Web/src/App.vue` 无命中；`grep -F 'href="/Meta/Finv/Quote/DeDup/CDEUWorkflow"' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'target="_blank"' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10439 | 「全流程拓扑」在横版流程图卡片下方增加独立的竖版全流程拓扑卡片，阶段自上而下堆叠、阶段内 Process 纵向排列，用于横竖版效果对比；竖版与横版共用同一实时状态数据。 | `grep -F '竖版全流程拓扑' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'verticalPhaseLayouts' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'verticalEdges' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10440 | 竖版全流程拓扑的阶段标题改为贴合阶段虚线框左侧的纵向标题栏，流程节点从标题栏右侧自上而下排列，跨阶段箭头改为从上一阶段出口进入下一阶段顶部。 | `grep -F 'fulltopo-vphase-title' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'V_PHASE_TITLE_WIDTH' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10441 | 竖版全流程拓扑所有阶段虚线框使用一致宽度；流程列数较少的阶段在框内水平居中排列。 | `grep -F 'width: maxWidth' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'contentOffset' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10442 | 竖版全流程拓扑的阶段 3（导出决策）和阶段 4（月度行情导出）增加框内垂直间距：加大入口分叉空间与并行分支行距，保证金分叉折线和箭头完整显示、不互相挤压。 | `grep -F 'V_BRANCH_FORK_SPACE' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'V_BRANCH_ROW_GAP' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10443 | 全流程拓扑的 Process 节点左上角编号使用 `process_id`，不再使用阶段内排序序号 `seq_no`；横版与竖版同步调整。 | `grep -F '{{ node.process.process_id }}' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中 2 次；`grep -F '{{ node.process.seq_no }}' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 无命中；`cd Web && npm run build` 通过 | |
| 10444 | 竖版全流程拓扑阶段 3「导出决策」入口分叉后的两条水平折线段在现有位置基础上再上移 10px。 | `grep -F 'verticalForkPath' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'midY - 10' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10445 | 全流程拓扑 Process 矩形节点宽度按标题、Process 编码和 JOB 文案自适应，不再使用固定宽度，也不再用省略号截断标题；横版和竖版同步。 | `grep -F 'processNodeWidth' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'text-overflow: clip' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10446 | 竖版全流程拓扑中，阶段进度由左侧标题区下方的水平线改为标题区与流程区之间的竖直进度线，按阶段进度纵向填充。 | `grep -F 'fulltopo-vphase-divider' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`grep -F 'V_PHASE_TITLE_WIDTH - 3' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10447 | 导出动态 JOB 的 `name` 字段统一为 `EXPORT_MON_{BASE/INCR}_{usc}_{biz_month}` 格式（如 `EXPORT_MON_BASE_GCMain_201912`），并回填已有 `cdeu_job_dynamic` 记录。 | `grep -F "EXPORT_MON_" internal/meta/finv/quote/dedup/store.go` 命中；`test -f Deploy/Migrations/V200065__finv_dedup_cdeu_dynamic_job_name.sql` 退出码为 0；`grep -F "EXPORT_MON_" Deploy/Migrations/V200065__finv_dedup_cdeu_dynamic_job_name.sql` 命中；`go test ./internal/meta/finv/quote/dedup ./internal/database` 通过 | |
| 10448 | 每个 JOB 运行提供详细执行日志：新增运行日志行表、`GET .../RunLog/Detail?runId=` 接口，CDEU JOB 行提供「执行日志」入口，弹窗展示阶段/进度/错误等逐行日志；运行中自动刷新。 | `test -f Deploy/Migrations/V200066__finv_dedup_job_run_log_line.sql` 退出码为 0；`grep -F 'RunLog/Detail' internal/api/router.go` 命中；`grep -F '执行日志' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`go test ./...` 通过；`cd Web && npm run build` 通过 | |
| 10449 | 低峰窗口支持 `00:00:00` ~ `23:59:59` 全天候测试；窗口按分钟粒度判定，`23:59` 分钟包含在窗口内，`HH:MM:SS` 输入兼容并归一为分钟保存。 | `grep -F '00:00:00 ~ 23:59:59' internal/meta/finv/quote/dedup/options.go` 命中；`grep -F 'fullDay' internal/meta/finv/quote/dedup/options_test.go` 命中；`go test ./internal/meta/finv/quote/dedup` 通过 | |
| 10450 | CDEU 控制台的 JOB 清单支持水平滚动：列总宽超出容器时出现横向滚动条，可查看右侧的执行阶段、状态、异常、操作等列。 | `grep -F 'job-table-scroll' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'overflow-x: auto' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10451 | 修复人工清理 CDEU 定义数据后 `StartCDEUWorkflow` 报「未登记 CDEU_DAILY 工作流定义」：新增幂等迁移重建缺失的 15 个 JOB 与 Job Step 定义。 | `test -f Deploy/Migrations/V200067__finv_dedup_cdeu_reseed_jobs.sql` 退出码为 0；`grep -F 'JOB_FETCH_EXPORT_BATCH' Deploy/Migrations/V200067__finv_dedup_cdeu_reseed_jobs.sql` 命中；`go test ./internal/database` 通过 | |
| 10452 | 上传动态 JOB 的编码/名称统一为 `JOB_UPLOAD_{FILE_TYPE}_V{VERSION}_{USC大写}_{YYYYMM}`（如 `JOB_UPLOAD_BASE_V1_GCMAIN_201910`）；调度标识由 `DYNAMIC_EXPORT` 改为 `DYNAMIC_UPLOAD`，并回填已有上传动态 JOB。 | `grep -F "JOB_UPLOAD_' \|\| f.file_type" internal/meta/finv/quote/dedup/store.go` 命中；`grep -F 'DYNAMIC_UPLOAD' internal/meta/finv/quote/dedup/cdeu_admin.go` 命中；`test -f Deploy/Migrations/V200068__finv_dedup_cdeu_upload_dynamic_name.sql` 退出码为 0；`go test ./internal/meta/finv/quote/dedup ./internal/database` 通过 | |
| 10453 | 上传动态 JOB 的 `name` 与 `job_code` 分离：`job_code` 为 `JOB_UPLOAD_BASE_V1_GCMAIN_201910`，`name` 为 `UPLOAD_BASE_V1_GCMain_201910`；回填已有上传动态 JOB。 | `grep -F "'UPLOAD_' \|\| f.file_type" internal/meta/finv/quote/dedup/store.go` 命中；`test -f Deploy/Migrations/V200069__finv_dedup_cdeu_upload_dynamic_display_name.sql` 退出码为 0；`grep -F "UPLOAD_' \|\| file_type" Deploy/Migrations/V200069__finv_dedup_cdeu_upload_dynamic_display_name.sql` 命中；`go test ./internal/meta/finv/quote/dedup ./internal/database` 通过 | |
| 10454 | 动态 JOB 关联执行日志与工作流运行：新增 `run_log_id` / `workflow_run_id` 字段；动态 JOB 完成后可在 JOB 清单查看执行日志；当前工作流 JOB 清单只展示本轮生成的动态 JOB，避免历史任务混入。 | `grep -F 'run_log_id = COALESCE' internal/meta/finv/quote/dedup/store.go` 命中；`grep -F 'workflow_run_id' internal/meta/finv/quote/dedup/cdeu_admin.go` 命中；`test -f Deploy/Migrations/V200070__finv_dedup_cdeu_dynamic_job_run_link.sql` 退出码为 0；`go test ./internal/meta/finv/quote/dedup ./internal/database` 通过 | |
| 10455 | 补丁扫描中的每个证券月单独登记 Job 运行日志，动态补丁 JOB 的执行日志只展示该证券月的处理过程，不再全部共用父级补丁任务日志。 | `grep -F 'candidateRunID' internal/meta/finv/quote/dedup/job_patch.go` 命中；`grep -F 'MarkDynamicJobStatus(ctx, candidate.Usc' internal/meta/finv/quote/dedup/job_patch.go` 命中；`go test ./internal/meta/finv/quote/dedup` 通过 | |
| 10456 | CDEU JOB 清单的 Step 列只展示当前执行到的 Step（优先 RUNNING，否则最后一个已完成 Step），并显示 `当前序号/总 Step 数`，不再把所有 Step 挤在单元格里。 | `grep -F 'currentJobStep' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F '当前执行到的 Step' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10457 | 修复导出/上传目录与持久化挂载不一致：默认导出目录统一为 `Data/ExportArchive`、上传目录为 `Data/ExportArchive/Upload`，与 Docker `/app/Data/ExportArchive` 挂载对齐，容器重建后文件不丢失。 | `grep -F 'Data/ExportArchive' internal/config/config.go` 命中；`grep -F 'FINV_QUOTE_DEDUB_OUTPUT_DIR' Deploy/Docker/docker-compose.yml` 命中；`go test ./internal/config ./internal/meta/finv/quote/dedup` 通过 | |
| 10458 | 无增量数据的 INCR 补丁任务标记为 `SKIPPED`，不再显示为成功；执行日志写入「无增量数据，跳过补丁文件生成」，避免用户误以为已生成补丁文件。 | `grep -F 'CDEUStatusSkipped' internal/meta/finv/quote/dedup/job_patch.go` 命中；`grep -F '无增量数据' internal/meta/finv/quote/dedup/job_patch.go` 命中；`go test ./internal/meta/finv/quote/dedup` 通过 | |
| 10459 | INCR 动态 JOB 延迟到补丁扫描确认 `delta > 0` 后才创建：批次清单只作为扫描候选，零增量不再生成 `JOB_EXPORT_INCR_*` 动态任务。 | `grep -F 'b.export_type' internal/meta/finv/quote/dedup/store.go` 命中；`grep -F 'EnsureDynamicExportJob' internal/meta/finv/quote/dedup/job_patch.go` 命中；`go test ./internal/meta/finv/quote/dedup` 通过 | |
| 10460 | 导出目录统一改为容器内 `/data/App/FinvQuant/Export`、上传目录 `/data/App/FinvQuant/Export/Upload`；Docker 挂载点同步调整，宿主机默认映射到 `D:/Data/App/FinvQuant/Export`。 | `grep -F '/data/App/FinvQuant/Export' internal/config/config.go` 命中；`grep -F '/data/App/FinvQuant/Export' Deploy/Docker/docker-compose.yml` 命中；`grep -F 'D:/Data/App/FinvQuant/Export' Deploy/Docker/.env` 命中 | |
| 10461 | 重置 GCMain 201811–201910 的 12 条导出任务状态并重新导出 BASE，使文件重新生成到新的持久化导出目录。 | `grep -F 'GCMain' Docs/CHANGELOG.md` 命中；`find /mnt/d/Data/App/FinvQuant/Export/GCMain -name '*.mvsv'` 至少 12 个 | |
| 10462 | Docker 导出宿主机目录由 `D:/Data/App/FinvQuant/Export` 调整为 `D:/Dev/Docker/HostFileSystem/FinvQuant/Export`，容器内路径保持 `/data/App/FinvQuant/Export`，已有文件迁移到新目录。 | `grep -F 'D:/Dev/Docker/HostFileSystem/FinvQuant/Export' Deploy/Docker/.env` 命中；`docker inspect finvquant --format '{{range .Mounts}}{{println .Source " -> " .Destination}}{{end}}'` 命中新宿主机路径；至少 12 个 `.mvsv` 文件存在于新目录 | |
| 10463 | CDEU 的 Process/Phase/Workflow 完成判定纳入动态 JOB：存在 PENDING/READY/RUNNING 动态 JOB 时不得标记完成；动态 JOB 状态变化后重新评估所属工作流运行。 | `grep -F 'cdeuProcessHasPendingDynamic' internal/meta/finv/quote/dedup/cdeu_store.go` 命中；`grep -F 'cdeuProcessChildren' internal/meta/finv/quote/dedup/cdeu_store.go` 命中；`grep -F 'EvaluateCDEURunProcesses' internal/meta/finv/quote/dedup/store.go` 命中；`go test ./internal/meta/finv/quote/dedup` 通过 | |
| 10464 | 修复动态 JOB 的 Step 未返回：按 `dynamic_job_id` 回填 Step，避免切片扩容导致指针失效；动态 JOB 在 JOB 清单可显示当前 Step。 | `grep -F 'dynamicSteps' internal/meta/finv/quote/dedup/cdeu_admin.go` 命中；`grep -F 'dynamic_job_id' internal/meta/finv/quote/dedup/cdeu_admin.go` 命中；`go test ./internal/meta/finv/quote/dedup` 通过 | |
| 10465 | PENDING/FAILED/PAUSED/SKIPPED 的导出动态 JOB 在 JOB 清单和详情视图显示「手动执行」按钮；新增 `POST /Job/Dynamic/Run` 按 `dynamic_job_id` 执行单任务。 | `grep -F 'Job/Dynamic/Run' internal/api/router.go` 命中；`grep -F '手动执行' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`go test ./internal/api/... ./internal/meta/finv/quote/dedup` 通过 | |
| 10466 | JOB 执行日志增强：LOAD/EXPORT/REGISTER/CLEANUP 每个 Step 单独记录开始/结束日志，日志包含 `job_code`、`job_name`、`step_code`、`step_name`；MVsv 写入日志记录绝对路径；清理日志记录中转表名。 | `grep -F 'StepStart' internal/meta/finv/quote/dedup/job_export.go` 命中；`grep -F 'absoluteBasePath' internal/meta/finv/quote/dedup/job_export.go` 命中；`grep -F '中转表=' internal/meta/finv/quote/dedup/job/logging.go` 命中；`go test ./internal/meta/finv/quote/dedup` 通过 | |
| 10467 | 归档 JOB 定义按单个 JOB 拆分到 `internal/meta/finv/quote/dedup/job/`，每个静态 JOB 一个文件；BASE/INCR/上传等仅入参不同的动态 JOB 复用对应父 JOB 文件。 | `find internal/meta/finv/quote/dedup/job -maxdepth 1 -name '*.go'` 命中 `export.go`、`scan_post_seal_patches.go` 等；`grep -F 'jobdef.All()' internal/meta/finv/quote/dedup/service.go` 命中；`go test ./internal/meta/finv/quote/dedup` 通过 | |
| 10468 | 归档 JOB 业务执行逻辑迁移到 `dedup/job` 子包：每个 JOB 一个执行文件，通过 `RegisterJobExecutor` 注入 `Service.dispatch`；BASE/INCR 动态导出复用 `export.go`，动态上传复用 `upload_pending_export_files.go`。 | `grep -F 'RegisterJobExecutor' internal/meta/finv/quote/dedup/job/*.go` 命中；`grep -F 'lookupJobExecutor' internal/meta/finv/quote/dedup/job_registry.go` 命中；`grep -F 'dedup/job' cmd/server/main.go` 命中；`go test ./...` 通过 | |
| 10469 | CDEU 手动执行 JOB 前弹出确认框，默认只执行当前 JOB；勾选「自动续跑同流程下的后续 JOB」后，当前 JOB 完成时按工作流顺序继续执行后续 JOB（同一 `job_kind` 只执行一次）。 | `grep -F '自动续跑同流程下的后续 JOB' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'runSubsequentJobs' internal/meta/finv/quote/dedup/service.go` 命中；`go test ./internal/meta/finv/quote/dedup -run TestSubsequentJobDefinitions` 通过 | |
| 10470 | 手动触发静态/动态 JOB 时接口立即返回 `run_log_id`，前端无需等待刷新即可打开执行日志弹窗；动态 JOB 改为异步执行。 | `grep -F 'run_log_id' internal/api/handler/cdeu.go` 命中；`grep -F 'openJobLogByID' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`go test ./internal/api/...` 通过 | |
| 10471 | JOB 执行日志弹窗支持全屏/退出全屏切换，全屏时日志区域占满可用高度。 | `grep -F 'logFullscreen' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'job-log-body-fullscreen' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`npm run build` 通过 | |
| 10472 | 修复动态导出 JOB 勾选自动续跑后只执行静态后续 JOB 的问题：先按 `dynamic_job_id` 顺序续跑同 Process 的后续动态导出 JOB，再执行静态后续 JOB；日志弹窗展示自动续跑状态，JOB 清单在续跑期间改为 1 秒刷新。 | `grep -F 'runSubsequentDynamicExportJobs' internal/meta/finv/quote/dedup/job_export.go` 命中；`grep -F '手动续跑动态 JOB' internal/meta/finv/quote/dedup/job_export.go` 命中；`docker logs finvquant 2>&1 \| grep '手动续跑动态 JOB'` 观察到连续动态 JOB；`go test ./...` 通过 | |
| 10473 | 修复 CDEU JOB 清单「处理量」列为空：动态导出 JOB 关联运行日志读取 `processed_rows`/`processed_bytes`，动态上传 JOB 关联导出文件读取 `row_count`/`file_size`；静态 JOB 在 CDEU 运行记录为 0 时回退读取运行日志。 | `curl 'http://localhost:16001/API/V1/Meta/Finv/Quote/DeDup/CDEU/Overview'` 返回的 `JOB_EXPORT_BASE_*` 含非零 `processed_rows`/`processed_bytes`，`JOB_UPLOAD_*` 含非零行数/字节数；`go test ./...` 通过 | |
| 10474 | CDEU Process / JOB 清单「耗时」列在耗时不超过 10 秒时展示毫秒精度（不足 1 秒为 `xxxms`，1–10 秒为 `x.xxxs`），超过 10 秒继续按秒 / 分 / 小时展示。 | `grep -F 'millisecondPrecision' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'durationMs <= 10_000' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`npm run build` 通过 | |

## 三十三、新增 sys 系统域 schema（10475–10482）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10475 | 存在创建 `sys` schema 的迁移且幂等。 | `test -f Deploy/Migrations/V200071__create_sys_schema.sql`；含 `CREATE SCHEMA IF NOT EXISTS sys` | |
| 10476 | 数据库存在 `sys` schema。 | `select count(*) from pg_namespace where nspname='sys'` 输出 `1` | |
| 10477 | `sys` schema 属主为应用角色（`finv`）。 | `select pg_get_userbyid(nspowner) from pg_namespace where nspname='sys'` 输出 `finv` | |
| 10478 | 应用角色对 `sys` 具备 `USAGE` 与 `CREATE`。 | `select has_schema_privilege('finv','sys','USAGE') and has_schema_privilege('finv','sys','CREATE')` 为 `t` | |
| 10479 | 数据库默认 `search_path` 含 `sys` 且位于末尾。 | `select array_to_string(setconfig,',') from pg_db_role_setting where setdatabase=(select oid from pg_database where datname=current_database())` 中 `search_path` 为 `public, quote, quant, futu, export, sche, dedup, archive, sys` | |
| 10480 | 应用连接串 `search_path` 同步为九段并含末尾 `sys`。 | `grep -c 'search_path=public,quote,quant,futu,export,sche,dedup,archive,sys' internal/database/postgres.go` 输出 `1` | |
| 10481 | 迁移记录已登记 `V71` 且成功。 | `select success from schema_version where version='71'` 输出 `t` | |
| 10482 | 规范文件登记 `sys` 并说明其边界（只放平台级对象，读写须写 `sys.` 前缀）。 | `grep -c 'sys' Docs/DevSpec/DbSchemaSpec.md` 不小于 `4`；含「系统域（`sys` schema）的边界」说明 | |

## 三十四、sys 系统配置表（10483–10490）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10483 | 存在创建 `sys.sys_config` 的迁移且幂等。 | `test -f Deploy/Migrations/V200072__sys_config.sql`；含 `CREATE TABLE IF NOT EXISTS sys.sys_config` 与 `CREATE OR REPLACE FUNCTION` | |
| 10484 | 数据库存在 `sys.sys_config` 表。 | `select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='sys' and c.relname='sys_config' and c.relkind='r'` 输出 `1` | |
| 10485 | 表包含 13 列且类型与定义一致（`config_no integer`、`data_type char(1)`、`str_value varchar(6000)`、`json_value json`、`gmt_create`/`gmt_modify timestamp(6)`）。 | `select count(*) from information_schema.columns where table_schema='sys' and table_name='sys_config'` 输出 `13`；`select data_type from information_schema.columns where table_schema='sys' and table_name='sys_config' and column_name='json_value'` 输出 `json` | |
| 10486 | 主键为 `pk_sys_config (config_no)`。 | `select conname, pg_get_constraintdef(oid) from pg_constraint where conname='pk_sys_config'` 输出含 `PRIMARY KEY (config_no)` | |
| 10487 | 表与 13 列均带中文注释。 | `select count(*) from pg_description where objoid='sys.sys_config'::regclass and objsubid between 0 and 13` 输出 `14`；`select obj_description('sys.sys_config'::regclass)` 输出 `系统配置表` | |
| 10488 | 存在 `BEFORE UPDATE` 行级触发器 `trg_sys_config_set_gmt_modify`，调用 `sys.fn_sys_config_set_gmt_modify()`。 | `select tgname, tgtype from pg_trigger where tgrelid='sys.sys_config'::regclass and not tgisinternal` 输出 `trg_sys_config_set_gmt_modify`；`select proname from pg_proc p join pg_namespace n on n.oid=p.pronamespace where n.nspname='sys'` 含 `fn_sys_config_set_gmt_modify` | |
| 10489 | 更新行时 `gmt_modify` 自动刷新为当前时间，`gmt_create` 保持不变。 | `insert into sys.sys_config(config_no,data_type,int_value) values(999999,'I',1); update sys.sys_config set int_value=2 where config_no=999999; select gmt_modify > gmt_create from sys.sys_config where config_no=999999` 输出 `t`（校验后删除该行） | |
| 10490 | 迁移末尾收敛校验通过并登记 `V72`。 | `select success from schema_version where version='72'` 输出 `t` | |

## 三十五、sys 数据字典表（10491–10501）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10491 | 存在创建 `sys.sys_dict` / `sys.sys_dict_sub` 的迁移且幂等。 | `test -f Deploy/Migrations/V200073__sys_dict.sql`；含两个 `CREATE TABLE IF NOT EXISTS sys.sys_` 与两个 `CREATE OR REPLACE FUNCTION` | |
| 10492 | 数据库存在 `sys.sys_dict` 表。 | `select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='sys' and c.relname='sys_dict' and c.relkind='r'` 输出 `1` | |
| 10493 | `sys_dict` 包含 16 列且关键列类型符合定义（`dict_no integer`、`key_type char(1)`、`ref_spec varchar(2000)`、`gmt_create timestamp(6)`）。 | `select count(*) from information_schema.columns where table_schema='sys' and table_name='sys_dict'` 输出 `16`；`select character_maximum_length from information_schema.columns where table_schema='sys' and table_name='sys_dict' and column_name='ref_spec'` 输出 `2000` | |
| 10494 | `sys_dict` 主键为 `pk_sys_dict (dict_no)`。 | `select pg_get_constraintdef(oid) from pg_constraint where conname='pk_sys_dict'` 输出含 `PRIMARY KEY (dict_no)` | |
| 10495 | `sys_dict` 表与 16 列均带中文注释。 | `select count(*) from pg_description where objoid='sys.sys_dict'::regclass and objsubid between 0 and 16` 输出 `17`；`select obj_description('sys.sys_dict'::regclass)` 输出 `数据字典主表` | |
| 10496 | 数据库存在 `sys.sys_dict_sub` 表。 | `select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='sys' and c.relname='sys_dict_sub' and c.relkind='r'` 输出 `1` | |
| 10497 | `sys_dict_sub` 包含 16 列、主键为 `(dict_no, sub_key)`，且 `flag_enable` 非空默认 `'0'`。 | `select count(*) from information_schema.columns where table_schema='sys' and table_name='sys_dict_sub'` 输出 `16`；`select pg_get_constraintdef(oid) from pg_constraint where conname='pk_sys_dict_sub'` 输出含 `PRIMARY KEY (dict_no, sub_key)`；`select column_default from information_schema.columns where table_schema='sys' and table_name='sys_dict_sub' and column_name='flag_enable'` 含 `0` | |
| 10498 | `sys_dict_sub` 表与 16 列均带中文注释。 | `select count(*) from pg_description where objoid='sys.sys_dict_sub'::regclass and objsubid between 0 and 16` 输出 `17`；`select obj_description('sys.sys_dict_sub'::regclass)` 输出 `数据字典子表` | |
| 10499 | 两表各有一个 `BEFORE UPDATE` 触发器并调用对应函数（`fn_sys_dict_touch` / `fn_sys_dict_sub_touch`）。 | `select tgname from pg_trigger where tgrelid in ('sys.sys_dict'::regclass,'sys.sys_dict_sub'::regclass) and not tgisinternal order by tgname` 输出 `trg_sys_dict_sub_touch`、`trg_sys_dict_touch` | |
| 10500 | 迁移末尾收敛校验通过并登记 `V73`。 | `select success from schema_version where version='73'` 输出 `t` | |
| 10501 | 两表触发器只刷新 `gmt_modify`：更新行时 `gmt_create` 保持不变（创建时间只在插入时生成），且触发器函数体不得含 `gmt_create` 赋值。 | `select prosrc from pg_proc p join pg_namespace n on n.oid=p.pronamespace where n.nspname='sys' and p.proname in ('fn_sys_dict_touch','fn_sys_dict_sub_touch')` 均不含 `NEW.gmt_create`；`insert into sys.sys_dict(dict_no,dict_name,gmt_create) values(999999,'t','2020-01-01 00:00:00'); update sys.sys_dict set dict_name='t2' where dict_no=999999; select to_char(gmt_create,'YYYY-MM-DD') from sys.sys_dict where dict_no=999999` 输出 `2020-01-01`（校验后删除该行） | |

## 三十六、sys 系统用户表（10502–10512）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10502 | 存在创建 `sys.sys_user` 的迁移且幂等（建表、序列初值、触发器均不因重跑而失效）。 | `test -f Deploy/Migrations/V200074__sys_user.sql`；含 `CREATE TABLE IF NOT EXISTS sys.sys_user`、`pg_get_serial_sequence` 与 `CREATE OR REPLACE FUNCTION` | |
| 10503 | 数据库存在 `sys.sys_user` 表。 | `select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='sys' and c.relname='sys_user' and c.relkind='r'` 输出 `1` | |
| 10504 | `sys_user` 包含 24 列且关键列类型符合定义（`idn bigint`、`authority_str varchar(6000)`、`description text`、`gmt_create timestamp(6)`）。 | `select count(*) from information_schema.columns where table_schema='sys' and table_name='sys_user'` 输出 `24`；`select data_type from information_schema.columns where table_schema='sys' and table_name='sys_user' and column_name='description'` 输出 `text` | |
| 10505 | 主键为 `pk_sys_user (idn)`。 | `select pg_get_constraintdef(oid) from pg_constraint where conname='pk_sys_user'` 输出含 `PRIMARY KEY (idn)` | |
| 10506 | `email` 与 `mobile_phone_no` 各自有唯一约束（`uk_sys_user_email` / `uk_sys_user_mobile`）。 | `select conname from pg_constraint where conrelid='sys.sys_user'::regclass and contype='u' order by conname` 输出 `uk_sys_user_email`、`uk_sys_user_mobile` | |
| 10507 | `idn` 为自增 identity 列，空表时下一个值为 `1028`（对齐原库 `AUTO_INCREMENT=1028`）。 | `select is_identity from information_schema.columns where table_schema='sys' and table_name='sys_user' and column_name='idn'` 输出 `YES`；空表执行 `select nextval(pg_get_serial_sequence('sys.sys_user','idn'))` 输出 `1028` | |
| 10508 | 表与 24 列均带中文注释。 | `select count(*) from pg_description where objoid='sys.sys_user'::regclass and objsubid between 0 and 24` 输出 `25`；`select obj_description('sys.sys_user'::regclass)` 输出 `系统用户表` | |
| 10509 | 存在 `BEFORE UPDATE` 触发器 `trg_sys_user_set_gmt_modify` 并调用 `sys.fn_sys_user_set_gmt_modify()`。 | `select tgname from pg_trigger where tgrelid='sys.sys_user'::regclass and not tgisinternal` 输出 `trg_sys_user_set_gmt_modify`；`select proname from pg_proc p join pg_namespace n on n.oid=p.pronamespace where n.nspname='sys'` 含 `fn_sys_user_set_gmt_modify` | |
| 10510 | 更新行时 `gmt_modify` 自动刷新、`gmt_create` 保持不变（创建时间只在插入时生成）。 | `select prosrc from pg_proc p join pg_namespace n on n.oid=p.pronamespace where n.nspname='sys' and p.proname='fn_sys_user_set_gmt_modify'` 不含 `NEW.gmt_create`；`insert into sys.sys_user(uid,nick_name,password,password_salt,email,gmt_create) values('t','t','t','t','t@t','2020-01-01 00:00:00'); update sys.sys_user set nick_name='t2' where email='t@t'; select to_char(gmt_create,'YYYY-MM-DD') from sys.sys_user where email='t@t'` 输出 `2020-01-01`（校验后删除该行） | |
| 10511 | 默认值符合定义：`authority_str` 默认 `'0'`、`flag_delete` 默认 `'0'`、`language` 默认 `'zh-CN'`、`time_zone` 默认 `'UTC+08:00'`。 | `select column_name, column_default from information_schema.columns where table_schema='sys' and table_name='sys_user' and column_name in ('authority_str','flag_delete','language','time_zone') order by column_name` 输出含 `'0'`、`'0'`、`'zh-CN'`、`'UTC+08:00'` | |
| 10512 | 迁移末尾收敛校验通过并登记 `V74`。 | `select success from schema_version where version='74'` 输出 `t` | |

## 三十七、sys_config 初始配置数据（10513–10518）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10513 | 初始配置数据以种子段迁移形式提供（编号 `V300000+`）且幂等。 | `test -f Deploy/Migrations/V300025__sys_config_seed.sql`；含 `INSERT INTO sys.sys_config` 与 `ON CONFLICT (config_no) DO NOTHING` | |
| 10514 | 数据库存在 `1010` / `1020` / `1031` / `5001` / `5002` 五个配置项。 | `select count(*) from sys.sys_config where config_no in (1010,1020,1031,5001,5002)` 输出 `5` | |
| 10515 | `1020`（默认最大单页展示记录数）为整型配置：`data_type='1'`、`int_value=2000`。 | `select data_type, int_value from sys.sys_config where config_no=1020` 输出 `1`、`2000` | |
| 10516 | `1010` / `1031` / `5001` / `5002` 为字符配置：`data_type='2'`、`char_value='1'`。 | `select count(*) from sys.sys_config where config_no in (1010,1031,5001,5002) and data_type='2' and char_value='1'` 输出 `4` | |
| 10517 | 重跑不影响生产已调整的配置值（不覆盖）。 | 先 `update sys.sys_config set str_value='x' where config_no=1031`，再重新执行 `Deploy/Migrations/V300025__sys_config_seed.sql`，`select str_value from sys.sys_config where config_no=1031` 仍输出 `x` | |
| 10518 | 迁移末尾收敛校验通过并登记 `V200025`。 | `select success from schema_version where version='100025'` 输出 `t` | |

## 三十八、sys_dict 初始字典定义（10519–10524）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10519 | 字典定义数据以种子段迁移形式提供（`V300000+`）且幂等。 | `test -f Deploy/Migrations/V300026__sys_dict_seed.sql`；含 `INSERT INTO sys.sys_dict` 与 `ON CONFLICT (dict_no) DO NOTHING` | |
| 10520 | 数据库存在 `1001`–`1009`、`1301`、`3004`、`3101`、`3103`、`3104`、`3105`、`3107`、`3110`、`3121`、`3122` 共 19 个字典定义。 | `select count(*) from sys.sys_dict where dict_no in (1001,1002,1003,1004,1005,1006,1007,1008,1009,1301,3004,3101,3103,3104,3105,3107,3110,3121,3122)` 输出 `19` | |
| 10521 | 19 条字典的 `key_type` 全为 `'1'` 且 `gmt_create` / `gmt_modify` 均非空。 | `select count(*) from sys.sys_dict where key_type='1' and gmt_create is not null and gmt_modify is not null` 不小于 `19` | |
| 10522 | 关键字段抽查正确：`1005`（`val_type='2'`、`dict_category='5'`）、`3004`（`enum_class='mnp'`、`dict_category='F'`）、`3104`（`en_sub_sys='foc'`、`dict_category='T'`）、`3105`（`manage_level IS NULL`、`enum_class='ses'`）、`3121`（`en_sub_sys='act'`）。 | `select dict_no from sys.sys_dict where (dict_no=1005 and not (val_type='2' and dict_category='5')) or (dict_no=3004 and not (enum_class='mnp' and dict_category='F')) or (dict_no=3104 and not (en_sub_sys='foc' and dict_category='T')) or (dict_no=3105 and not (manage_level is null and enum_class='ses')) or (dict_no=3121 and en_sub_sys<>'act')` 输出 `0 行` | |
| 10523 | 重跑不影响生产已调整的字典定义（不覆盖）。 | 先 `update sys.sys_dict set remark='x' where dict_no=1001`，再重新执行 `Deploy/Migrations/V300026__sys_dict_seed.sql`，`select remark from sys.sys_dict where dict_no=1001` 仍输出 `x` | |
| 10524 | 迁移末尾收敛校验通过并登记 `V200026`。 | `select success from schema_version where version='100026'` 输出 `t` | |

## 三十九、sys_dict_sub 初始字典子项（10525–10531）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10525 | 字典子项数据以种子段迁移形式提供（`V300000+`）且幂等。 | `test -f Deploy/Migrations/V300027__sys_dict_sub_seed.sql`；含 `INSERT INTO sys.sys_dict_sub` 与 `ON CONFLICT (dict_no, sub_key) DO NOTHING` | |
| 10526 | 三组字典子项条数正确：`3120` = 14、`3121` = 11、`3122` = 64，共 89 条。 | `select dict_no, count(*) from sys.sys_dict_sub where dict_no in (3120,3121,3122) group by dict_no order by dict_no` 输出 `3120\|14`、`3121\|11`、`3122\|64` | |
| 10527 | `3120` 子键完整（`0`–`9`、`A`–`D`）且 `en_sys_str` 两种顺序正确：`0/1/2/8/9/A/B/C/D` 为 `gat,gax,gcq,gaa,tms,pbs`，`3/4/5/6/7` 为 `gat,gcq,gax,gaa,tms,pbs`。 | `select count(*) from sys.sys_dict_sub where dict_no=3120 and sub_key in ('0','1','2','8','9','A','B','C','D') and en_sys_str='gat,gax,gcq,gaa,tms,pbs'` 输出 `9`；`select count(*) from sys.sys_dict_sub where dict_no=3120 and sub_key in ('3','4','5','6','7') and en_sys_str='gat,gcq,gax,gaa,tms,pbs'` 输出 `5` | |
| 10528 | `3121` 共 11 条，`en_sys_str` 均为 `act`，`name_en` 均为 `NULL`，且包含 `HKD`→`香港元`。 | `select count(*) from sys.sys_dict_sub where dict_no=3121 and en_sys_str='act' and name_en is null` 输出 `11`；`select sub_val from sys.sys_dict_sub where dict_no=3121 and sub_key='HKD'` 输出 `香港元` | |
| 10529 | `3122` 共 64 条，含 `OTH`→`其它` 且其 `en_sys_str` 为 `NULL`，其余 `en_sys_str` 均为 `cic`。 | `select count(*) from sys.sys_dict_sub where dict_no=3122` 输出 `64`；`select en_sys_str from sys.sys_dict_sub where dict_no=3122 and sub_key='OTH'` 输出空值；`select count(*) from sys.sys_dict_sub where dict_no=3122 and sub_key<>'OTH' and en_sys_str='cic'` 输出 `63` | |
| 10530 | 89 条均满足 `flag_enable='0'`、`access_level=2`、`gmt_create` / `gmt_modify` 非空。 | `select count(*) from sys.sys_dict_sub where dict_no in (3120,3121,3122) and flag_enable='0' and access_level=2 and gmt_create is not null and gmt_modify is not null` 输出 `89` | |
| 10531 | 迁移末尾收敛校验通过并登记 `V200027`。 | `select success from schema_version where version='100027'` 输出 `t` | |

## 四十、sys_dict_sub 地区/国籍子项（10532–10538）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10532 | 地区/国籍子项数据以种子段迁移形式提供（`V300000+`）且幂等。 | `test -f Deploy/Migrations/V300028__sys_dict_sub_seed_area.sql`；含 `ON CONFLICT (dict_no, sub_key) DO NOTHING` | |
| 10533 | 三组条数不少于预期：`3103` = 249、`3104` = 5、`3105` = 133（共 387）。 | `select dict_no, count(*) from sys.sys_dict_sub where dict_no in (3103,3104,3105) group by dict_no order by dict_no` 输出 `3103\|249`、`3104\|5`、`3105\|133` | |
| 10534 | `3103` 首尾为 `ABW` 阿鲁巴 / `ZWE` 津巴布韦，且 `en_sys_str` 只出现 `trade`、`cic,gft,gaa,act`、`act,cic` 或 `NULL`。 | `select count(*) from sys.sys_dict_sub where dict_no=3103 and en_sys_str is not null and en_sys_str not in ('trade','cic,gft,gaa,act','act,cic')` 输出 `0` | |
| 10535 | `3103` 例外行正确：`ANT/ATF/BVT/COD/COK/DMA/ERI/EST/FLK/FSM/GEO/GLP/HMD/IOT/KGZ/LVA/MDA/MKD/MNP/MTQ/MYT/PLW/REU/SGS/SHN/SJM/SPM/TCA/TJK/UMI/UZB/VAT/VCT/WLF/WSM` 为 `access_level=2` 且 `en_sys_str IS NULL`；`CCK` 为 `1/NULL`；`CSR/PST/TMP` 为 `2/cic,gft,gaa,act`；`GGY` 为 `2/act,cic`。 | `select count(*) from sys.sys_dict_sub where dict_no=3103 and sub_key in ('ANT','ATF','BVT','COD','COK','DMA','ERI','EST','FLK','FSM','GEO','GLP','HMD','IOT','KGZ','LVA','MDA','MKD','MNP','MTQ','MYT','PLW','REU','SGS','SHN','SJM','SPM','TCA','TJK','UMI','UZB','VAT','VCT','WLF','WSM') and not (access_level=2 and en_sys_str is null)` 输出 `0` | |
| 10536 | `3104` 五条电话区号正确（`CHN`=86、`CTN`=886、`HKG`=852、`MAC`=853、`USA`=001），且 `en_sys_str` 为 `NULL`、`access_level=1`。 | `select sub_key, sub_val from sys.sys_dict_sub where dict_no=3104 order by sub_key` 输出五行；`select count(*) from sys.sys_dict_sub where dict_no=3104 and en_sys_str is null and access_level=1` 输出 `5` | |
| 10537 | `3105` 为 `004` 阿富汗 至 `470` 马耳他，且全部 `en_sys_str='trade'`、`access_level=2`。 | `select count(*) from sys.sys_dict_sub where dict_no=3105 and not (en_sys_str='trade' and access_level=2)` 输出 `0`；`select sub_val from sys.sys_dict_sub where dict_no=3105 and sub_key='470'` 输出 `马耳他` | |
| 10538 | 迁移末尾收敛校验通过并登记 `V200028`。 | `select success from schema_version where version='100028'` 输出 `t` | |

## 四十一、sys_dict_sub 通用标志/分页/数据类型子项（10539–10544）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10539 | 数据以种子段迁移形式提供（`V300000+`）且幂等。 | `test -f Deploy/Migrations/V300029__sys_dict_sub_seed_common.sql`；含 `ON CONFLICT (dict_no, sub_key) DO NOTHING` | |
| 10540 | 六组条数正确：`1001`=2、`1002`=2、`1003`=3、`1004`=2、`1005`=14、`1006`=5（共 28）。 | `select dict_no, count(*) from sys.sys_dict_sub where dict_no between 1001 and 1006 group by dict_no order by dict_no` 输出六行对应条数 | |
| 10541 | `1001` 为 `0`→否 / `1`→是；`1002` 的 `0` 与 `1` 均为「否」（原库如此）。 | `select count(*) from sys.sys_dict_sub where dict_no=1001 and ((sub_key='0' and sub_val='否') or (sub_key='1' and sub_val='是'))` 输出 `2`；`select count(*) from sys.sys_dict_sub where dict_no=1002 and sub_val='否'` 输出 `2` | |
| 10542 | `1003` 为可修改/只读/隐藏；`1004` 两条带 `sub_sort`（1/2）与非空 `description`，且 `en_sys_str` 为空串（非 `NULL`）。 | `select count(*) from sys.sys_dict_sub where dict_no=1003` 输出 `3`；`select count(*) from sys.sys_dict_sub where dict_no=1004 and sub_sort in ('1','2') and coalesce(description,'')<>'' and en_sys_str=''` 输出 `2` | |
| 10543 | `1005` 共 14 条，`sub_val = sub_key`、`sub_sort` 为 1–14（按数值比较）、`en_sys_str='ftc'`。 | `select count(*) from sys.sys_dict_sub where dict_no=1005 and sub_val=sub_key and sub_sort::int between 1 and 14 and en_sys_str='ftc'` 输出 `14` | |
| 10544 | `1006` 共 5 条（字符/整数/浮点数/布尔/字符串），`flag_enable='0'`；迁移登记 `V200029` 成功。 | `select count(*) from sys.sys_dict_sub where dict_no=1006 and flag_enable='0' and en_sys_str='trade,pbs'` 输出 `5`；`select success from schema_version where version='100029'` 输出 `t` | |

## 四十二、sys_dict_sub 国籍（3105）增量补齐（10545–10550）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10545 | 3105 增量以种子段迁移形式提供（`V300000+`）且幂等。 | `test -f Deploy/Migrations/V300030__sys_dict_sub_seed_nationality_increment.sql`；含 `ON CONFLICT (dict_no, sub_key) DO NOTHING` | |
| 10546 | `3105` 国籍已补齐至 235 条，区间为 `004`–`894`。 | `select min(sub_key), max(sub_key), count(*) from sys.sys_dict_sub where dict_no=3105` 输出 `004` / `894` / `235` | |
| 10547 | 本批新增的 102 个 `sub_key`（`474`–`894`）全部存在。 | `select count(*) from sys.sys_dict_sub where dict_no=3105 and sub_key >= '474'` 输出 `102` | |
| 10548 | 新增首尾正确：`474`→马提尼克、`894`→赞比亚。 | `select sub_val from sys.sys_dict_sub where dict_no=3105 and sub_key='474'` 输出 `马提尼克`；`... sub_key='894'` 输出 `赞比亚` | |
| 10549 | `3105` 全量 235 条均为 `flag_enable='0'`、`access_level=2`、`en_sys_str='trade'`。 | `select count(*) from sys.sys_dict_sub where dict_no=3105 and not (flag_enable='0' and access_level=2 and en_sys_str='trade')` 输出 `0` | |
| 10550 | `3103`（249）与 `3104`（5）不受本次增量影响；迁移登记 `V200030` 成功。 | `select dict_no, count(*) from sys.sys_dict_sub where dict_no in (3103,3104) group by dict_no order by dict_no` 输出 `3103\|249`、`3104\|5`；`select success from schema_version where version='100030'` 输出 `t` | |

## 四十三、sys_config 新增运行/收集模式配置（10551–10556）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10551 | 配置项以种子段迁移形式提供（`V300000+`）且幂等。 | `test -f Deploy/Migrations/V300031__sys_config_seed_quote_mode.sql`；含 `ON CONFLICT (config_no) DO NOTHING` | |
| 10552 | 存在 `config_no=20000`，`config_name='FinvQuote服务运行模式'`、`data_type='2'`、`char_value='0'`。 | `select config_name, data_type, char_value from sys.sys_config where config_no=20000` 输出对应三值 | |
| 10553 | `20000` 的 `remark` 为 `0-云端模式（默认） 1-本地模式`。 | `select remark from sys.sys_config where config_no=20000` 输出与预期一致 | |
| 10554 | 存在 `config_no=20001`，`config_name='历史行情收集模式'`、`data_type='2'`、`char_value='0'`。 | `select config_name, data_type, char_value from sys.sys_config where config_no=20001` 输出对应三值 | |
| 10555 | `20001` 的 `remark` 说明被动/主动两种收集模式。 | `select remark from sys.sys_config where config_no=20001` 包含 `被动收集` 与 `主动采集模式` | |
| 10556 | 两条配置时间戳非空；迁移登记 `V200031` 成功。 | `select success from schema_version where version='100031'` 输出 `t`；`select count(*) from sys.sys_config where config_no in (20000,20001) and gmt_create is not null and gmt_modify is not null` 输出 `2` | |

## 四十四、sys_config 创建人修正为 fqe（10557–10560）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10557 | 修正以独立种子段迁移形式提供（`V300000+`）且幂等（不回改已应用的 `V200031`）。 | `test -f Deploy/Migrations/V300032__sys_config_set_create_by_fqe.sql`；含 `SET create_by = 'fqe'` 与 `AND (create_by IS NULL OR create_by <> 'fqe')` | |
| 10558 | `20000` 与 `20001` 的 `create_by` 均为 `fqe`。 | `select count(*) from sys.sys_config where config_no in (20000,20001) and create_by='fqe'` 输出 `2` | |
| 10559 | 修正不改写 `gmt_create`（仍为插入时间），`gmt_modify` 由触发器刷新。 | `select config_no, gmt_create, gmt_modify from sys.sys_config where config_no in (20000,20001)` 中 `gmt_modify >= gmt_create` 且 `gmt_create` 仍为初次插入时间 | |
| 10560 | 其余配置的 `create_by` 不受影响；迁移登记 `V200032` 成功。 | `select count(*) from sys.sys_config where config_no in (1010,1020,1031,5001,5002) and create_by is null` 输出 `5`；`select success from schema_version where version='100032'` 输出 `t` | |

## 四十五、sys_config 20001 备注文案修订（10561–10564）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10561 | 文案修订以独立种子段迁移形式提供（`V300000+`）且幂等（`IS DISTINCT FROM`）。 | `test -f Deploy/Migrations/V300033__sys_config_update_20001_remark.sql`；含 `remark IS DISTINCT FROM` | |
| 10562 | `20001` 的 `remark` 为 `0-被动收集（默认），外部程序通过暴露的API接口添加；1-主动采集模式，服务主动从外部源采集入库保存`。 | `select remark from sys.sys_config where config_no=20001` 输出与预期一致 | |
| 10563 | `20001` 的 `config_name` / `data_type` / `char_value` / `create_by` 未被改动，`gmt_create` 不变。 | `select config_name, data_type, char_value, create_by from sys.sys_config where config_no=20001` 输出 `历史行情收集模式` / `2` / `0` / `fqe` | |
| 10564 | `20000` 的备注不受影响；迁移登记 `V200033` 成功。 | `select remark from sys.sys_config where config_no=20000` 输出 `0-云端模式（默认） 1-本地模式`；`select success from schema_version where version='100033'` 输出 `t` | |

## 四十六、sys_config 归档自动上传 Git 开关与 PAT（10565–10570）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10565 | 两项配置以种子段迁移形式提供（`V300000+`），仅 `ON CONFLICT DO NOTHING`，无针对 20002/20003 的 `UPDATE` 语句。 | `test -f Deploy/Migrations/V300034__sys_config_seed_archive_upload.sql`；`grep -cE '^[[:space:]]*UPDATE[[:space:]]+sys\.sys_config' 文件` 输出 `0`；`grep -cE '^ON CONFLICT \(config_no\) DO NOTHING;' 文件` 输出 `1`（排除注释中的提及） | |
| 10566 | `20002`「启用月归档数据自动上传Git仓库标志」存在，`data_type='2'`、`char_value='0'`、`remark='0-关闭(默认) 1-启用'`。 | `select config_name, data_type, char_value, remark from sys.sys_config where config_no=20002` 输出对应四值 | |
| 10567 | `20003`「归档数据上传GitHub仓库的PAT值」存在，`data_type='3'`、`char_value=''`、**`str_value IS NULL`（无默认值）**。 | `select config_name, data_type, char_value, str_value from sys.sys_config where config_no=20003` 输出 `... / 3 / (空) / (NULL)` | |
| 10568 | PAT 存储容量足够（`str_value` 为 `varchar(6000)`），未误用 `char(1)` 的 `char_value`。 | `select character_maximum_length from information_schema.columns where table_schema='sys' and table_name='sys_config' and column_name='str_value'` 输出 `6000` | |
| 10569 | 交付的初始种子**不覆盖**用户后配值：已存在的行在重跑时值保持不变。 | 实测：先用 `update ... set char_value='1' where config_no=20002` 与模拟 PAT 写入 `20003.str_value`，重跑 `V200034` 输出 `INSERT 0 0`，两值仍为 `1` / 模拟 PAT（验证后已回置为 `0` / `NULL`） | |
| 10570 | 两条配置 `create_by='finv'`；迁移登记 `V200034` 成功。 | `select count(*) from sys.sys_config where config_no in (20002,20003) and create_by='finv'` 输出 `2`；`select success from schema_version where version='100034'` 输出 `t` | |

## 四十七、sys_config 文件上传仓库配置 20004（10571–10576）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10571 | 配置以种子段迁移形式提供（`V300000+`），仅 `ON CONFLICT DO NOTHING`，无针对 20004 的 `UPDATE` 语句。 | `test -f Deploy/Migrations/V300035__sys_config_seed_archive_repo.sql`；`grep -cE '^[[:space:]]*UPDATE[[:space:]]+sys\.sys_config' 文件` 输出 `0` | |
| 10572 | `20004`「文件上传仓库配置」存在，`data_type='3'`、`char_value=''`、`create_by='finv'`。 | `select config_name, data_type, char_value, create_by from sys.sys_config where config_no=20004` 输出对应四值 | |
| 10573 | **不发放默认值**：`str_value IS NULL`（待用户自行配置）。 | `select str_value is null from sys.sys_config where config_no=20004` 输出 `t` | |
| 10574 | `remark` 明确给出 `{owner}:{repo}:{branch}:{pre_path_key}` 格式及各段含义。 | `select remark from sys.sys_config where config_no=20004` 包含 `{owner}:{repo}:{branch}:{pre_path_key}` 与 `pre_path_key=文件保存的目录前缀` | |
| 10575 | 交付的初始种子**不覆盖**用户已配置值。 | 实测：写入 `ACANX:FinvQuant-Archive:main:Archive/FinvQuant/Monthly` 后重跑 `V200035` 输出 `INSERT 0 0`，值原样保留（验证后已回置为 `NULL`） | |
| 10576 | 用户配置格式不合约定时只告警、不阻断迁移；迁移登记 `V200035` 成功。 | 实测：置为 3 段非法值 `ACANX:x:main` 后重跑，输出 `WARNING ... 当前 3 段` 且 `COMMIT` 成功；`select success from schema_version where version='100035'` 输出 `t` | |

## 四十八、月归档上传 GitHub 仓库（Contents API，10577–10586）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10577 | 新增 `internal/meta/finv/sys`（需求 10762 归位前为 `internal/syscfg`）读取 `sys.sys_config` 的 `20002`/`20003`/`20004`，带 30s TTL 缓存且 SQL 显式限定 `sys.`。 | `test -f internal/meta/finv/sys/syscfg.go`；`grep -n 'FROM sys.sys_config' internal/meta/finv/sys/syscfg.go` 有输出；`go test ./internal/meta/finv/sys/` 通过 | |
| 10578 | `20004` 按 `{owner}:{repo}:{branch}:{pre_path_key}` 解析；段数不符/空段/`..` 越界等返回可读错误。 | `go test ./internal/meta/finv/sys/ -run TestParseRepoSpec -v` 全部通过（含 11 个正反用例） | |
| 10579 | 用 GitHub Contents API（纯 `net/http`）实现上传，未引入第三方 SDK。 | `grep -rn 'github.com/google/go-github\|github.com/go-github' go.mod internal/` 无输出；`grep -n 'net/http' internal/meta/finv/quote/dedup/uploader_github.go` 有输出 | |
| 10580 | `20002='1'` 时上传自动路由到 GitHub；`'0'` 时保持原 `none/dir/http`，在线切换无需重启。 | 生产验证：`20002=0` → `Status.options.upload_mode=dir`；临时置 `1`（等缓存过期）→ `github`；回置 `0` → `dir` | |
| 10581 | 上传幂等：远端内容（大小 + git blob sha）一致时不重复提交；内容不同时携带远端 sha 覆盖。 | `go test ./internal/meta/finv/quote/dedup/ -run 'TestGitHubPut' -v` 通过（含 `SkipsIdenticalContent` 断言 PUT 0 次、`UpdatesWithRemoteSHA` 断言携带 sha） | |
| 10582 | 上传后校验大小与内容 sha，不一致时报错并计入上传审计。 | `go test ... -run TestGitHubVerify -v` 通过（含大小不一致、内容被替换两个失败分支） | |
| 10583 | 配置缺失/非法时返回可读错误（含 `20002`/`20003` 编号），不静默回退到其它目标。 | `go test ... -run TestGitHubPutRequiresCredential -v` 通过 | |
| 10584 | 大文件拦截：超过 Contents API 单文件上限时明确报错且不发请求。 | `go test ... -run TestGitHubPutRejectsOversizeFile -v` 通过 | |
| 10585 | 5xx/429/网络错误做有限重试；请求体为合法 JSON 且 base64 无换行、`Content-Length` 精确。 | `go test ... -run 'TestGitHubPutRetriesServerError\|TestBuildContentsBody\|TestBase64Length' -v` 通过 | |
| 10586 | 全量回归通过，且文档同步（CHANGELOG / Status.md / JobRun.md / `.env.example` / compose）。 | 容器内 `go test ./...` 全部 `ok`；`grep -n 'upload_mode' Docs/API/QuoteDeDup/Status.md` 含 `github` 说明；`grep -n 'UPLOAD_GITHUB_API_BASE' Deploy/Docker/docker-compose.yml Deploy/Docker/.env.example` 有输出 | |

## 四十九、上传范围收窄为仅 .mvsv（10587–10592）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10587 | 上传领取只取 `.mvsv` 数据文件（`BASE`/`PATCH`），不领取 `MANIFEST`。 | `grep -n "f.file_type <> 'MANIFEST'" internal/meta/finv/quote/dedup/store_files.go` 有输出（位于 `PickUploadFile`） | |
| 10588 | 待上传的 `MANIFEST` 统一标记为 `SKIPPED` 并写审计日志（含原因）。 | 实测：置 MANIFEST(223) 为待上传后跑 Job，`upload_status=SKIPPED`；`select status, error from dedup.finv_dedup_export_file_upload_log where file_id=223 order by log_id desc limit 1` 输出 `SKIPPED` + `MANIFEST 按业务约定不上传...` | |
| 10589 | 重导出相同内容时 `MANIFEST` 保持 `SKIPPED`，不反复入队。 | `grep -n "EXCLUDED.file_type = 'MANIFEST'" internal/meta/finv/quote/dedup/store_files.go` 有输出（`RegisterFile` upsert 分支） | |
| 10590 | 本地清理条件放宽：`MANIFEST` 的 `UPLOADED`/`SKIPPED` 在网盘归档 + 保留期到达后可清理。 | `grep -n "file_type = 'MANIFEST' AND" internal/meta/finv/quote/dedup/store_files.go` 在 `LocalCleanupCandidates`、`LocalFilesPendingCleanup` 各命中 1 处 | |
| 10591 | 上传 Job 结果包含 `manifests_skipped` 计数。 | 实测 Job 日志：`完成 mode=github uploaded=1 failed=0 manifests_skipped=1` | |
| 10592 | 实测：`MANIFEST` 不推送远端；`.mvsv` 正常推送且内容一致。 | GitHub 侧 `GET /contents` 目标目录仅含 `USA_US-NSDQ_NVDA_Min_FTMM_Mon_202607.mvsv`（13151 字节），下载后 SHA256 与本地一致；此前遥留的 Manifest 已删除 | |

## 五十、远端归档目录按证券分层（10593–10598）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10593 | 上传相对路径改为 `{region}_{market}/{usc}/{文件名}`（最终 = 20004 的 `pre_path_key` + 分层），行为默认开启。 | `go test ./internal/meta/finv/quote/dedup/ -run TestArchiveTargetPath -v` 通过（含默认分层、外层前缀、flat 回退、空 layout 四个子用例） | |
| 10594 | 分层规则写在代码（与命名契约同源），而**非**塞进 `sys_config` 20004；20004 仍只描述仓库定位。 | `grep -n 'ArchiveTargetPath' internal/meta/finv/quote/dedup/options.go` 有实现与设计理由注释；`grep -n 'region_market\|{region}' internal/meta/finv/sys/syscfg.go` 无输出（配置层不感知分层） | |
| 10595 | 提供 `FINV_QUOTE_DEDUB_UPLOAD_LAYOUT`（`by_security` 默认 / `flat` 旧行为），非法值回退默认。 | `go test ./internal/meta/finv/quote/dedup/ -run TestNormalizedLayout -v` 通过；`grep -n 'UPLOAD_LAYOUT' Deploy/Docker/docker-compose.yml Deploy/Docker/.env.example` 有输出 | |
| 10596 | `region/market` 从 `dedup.finv_dedup_usc` 读取，缺失时退回扁平并告警（不阻塞上传）。 | `grep -n '证券元数据缺 region/market' internal/meta/finv/quote/dedup/job_upload.go` 有输出 | |
| 10597 | 登记阶段（`upload_target`）与上传阶段用同一函数计算路径，不出现两套规则。 | `grep -rn 'ArchiveTargetPath' internal/meta/finv/quote/dedup/job_export.go internal/meta/finv/quote/dedup/job_patch.go` 共 4 处；`grep -n 's.options.TargetPath(' internal/meta/finv/quote/dedup/job_upload.go` 仅作为 flat/回退分支 | |
| 10598 | 实测：原有 2 个 `.mvsv` 已迁到新分层目录且内容一致，旧扁平路径副本已删。 | GitHub 目录树：`USA_US-NSDQ/NVDA/…`（13151）、`FX_FX-CFD/XAUUSD/…`（52923），两侧 SHA256 均与本地一致；旧路径 DELETE 返回 200；库内 `upload_target` 已为新路径 | |

## 五十一、finv_market 新增 market_symbol（10599–10604）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10599 | `public.finv_market` 新增字符串列 `market_symbol`，`NOT NULL DEFAULT ''`。 | `select data_type, is_nullable, column_default from information_schema.columns where table_schema='public' and table_name='finv_market' and column_name='market_symbol'` 输出 `text` / `NO` / `''::text` | |
| 10600 | 列带语义注释（说明取值与空串含义）。 | `select col_description('public.finv_market'::regclass, attnum) from pg_attribute where attrelid='public.finv_market'::regclass and attname='market_symbol'` 含「市场符号」 | |
| 10601 | 迁移幂等（`ADD COLUMN IF NOT EXISTS`）且失败整体回滚，首尾登记 `V75`。 | 重跑输出 `column "market_symbol" ... skipping` 且 `COMMIT`；`select success from schema_version where version='75'` 输出 `t` | |
| 10602 | `Market/List` 返回 `market_symbol`，`keyword` 检索覆盖该列。 | `GET .../Metadata/Market/List?page=1&page_size=2` 的 `list[*].market_symbol` 字段存在；`GET ...&keyword=NSDQ` 能命中 | |
| 10603 | `Market/Save` 可写入 `market_symbol`（UPDATE 与 INSERT 分支均包含该列）。 | round-trip 实测：置 `SSE-A-TEST` 后回读为 `SSE-A-TEST`，再置回空串后库内为 `''`（验证后已恢复原值） | |
| 10604 | 前端交易市场页可展示与编辑该列；文档同步（API、CHANGELOG）。 | `grep -c market_symbol Web/src/views/Meta/Finv/MetaData/MetaMarketView.vue` 输出 `6`；`Docs/API/Meta/MetaMarket.md` 含字段说明 | |

## 五十二、证券主数据 region/market 口径修正（10605–10610）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10605 | `finv_exchange.region` 的 `USA` 统一为字典值 `US`。 | `select count(*) from public.finv_exchange where region='USA'` 输出 `0`；`select count(*) from public.finv_exchange where region='US'` 输出 `9` | |
| 10606 | `dedup.finv_dedup_usc.market` 改为关联 `finv_market.market_symbol` 取值（为空时回退 `market_flag`、`exchange_flag`）。 | `grep -n 'NULLIF(m.market_symbol' internal/meta/finv/quote/dedup/store.go` 有输出；`select count(*) from dedup.finv_dedup_usc u join public.finv_security s on s.usc=u.usc join public.finv_market m on m.market_code=s.market_code where nullif(m.market_symbol,'') is not null and u.market<>m.market_symbol` 输出 `0` | |
| 10607 | `region` 以 `finv_region` 字典规范化，字典无项时回退 `exchange.region` 原值。 | `grep -n 'LEFT JOIN public.finv_region r' internal/meta/finv/quote/dedup/store.go` 有输出；`select distinct region from dedup.finv_dedup_usc` 输出 `CN/FX/HK/US`（无 `USA`） | |
| 10608 | `RefreshSecurityMetadata` 对 region/market 改为以字典为准覆写（新值为 NULL 时保留旧值），其余列仍只补缺。 | `grep -n 'COALESCE(EXCLUDED.region' internal/meta/finv/quote/dedup/store.go` 有输出；`go test ./...` 通过 | |
| 10609 | 存量数据已重绑（`CN→SH/SZ/BJ`、`HK→HK/HKR`、`FX→CFD`、`US→BMS/COMEX/NSDQ`），迁移幂等。 | 实测：首跑 `UPDATE 9` + `UPDATE 39`；重跑均为 `UPDATE 0` | |
| 10610 | 校验：无 `USA` 残留、`market` 与 `market_symbol` 对齐、`region/market` 均非空；迁移 `V200036` 登记成功。 | 迁移内置 `DO` 块四条断言均通过；`select success from schema_version where version='100036'` 输出 `t` | |

## 五十三、归档导出状态重置（10611–10615）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10611 | 清除旧口径的文件登记与批次（否则 `RegisterFile` 会因已 `UPLOADED` 而不重新上传）。 | `select count(*) from dedup.finv_dedup_export_file` 输出 `0`；`select count(*) from dedup.finv_dedup_export_batch` 输出 `0`（迁移日志：文件 226、批次 202 已清） | |
| 10612 | 101 个导出任务回退为 `FROZEN` 并清空游标/版本/补丁计数/锁/错误。 | `select status, count(*) from dedup.finv_dedup_export_job group by status` 仅输出 `FROZEN \| 101` | |
| 10613 | 当日预算清零消耗并抬高上限（一轮覆盖全量重导）。 | `select budget_rows, hard_cap_rows, consumed_rows from dedup.finv_dedup_daily_budget where biz_date=current_date` 输出 `5000000 / 6000000 / 0` | |
| 10614 | 重跑保护：已重置后再次执行迁移不会二次清空（避免导出进行中被破坏）。 | 重跑输出 `无 EXPORTED 任务（已重置过或尚未导出），跳过` 且 `COMMIT` | |
| 10615 | 前置输入保留：成熟度 `FROZEN`（2491）、扫描水位、`usc` 主数据、CDEU 历史与上传审计日志均不改；迁移 `V200037` 登记成功。 | `select count(*) from dedup.finv_dedup_quote_maturity_mon where maturity_status='FROZEN'` 输出 `2491`；`select count(*) from dedup.finv_dedup_usc` 输出 `39`；`select success from schema_version where version='100037'` 输出 `t` | |

## 五十四、证券主数据 currency 刷新修正（10616–10620）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10616 | `RefreshSecurityMetadata` 对 `currency` 改为「以主数据为准覆盖」（新值为 NULL 时保留旧值），其余列仍只补缺。 | `grep -n 'COALESCE(EXCLUDED.currency' internal/meta/finv/quote/dedup/store.go` 有输出；`go test ./...` 通过 | |
| 10617 | 存量数据已重刷：`81810`（小米集团-WR 人民币柜台）`HKD → CNY`。 | `select currency from dedup.finv_dedup_usc where usc='81810'` 输出 `CNY`；迁移首次执行 `UPDATE 1` | |
| 10618 | 全库 `currency` 与 `finv_security.currency_type` 零偏差且无空值。 | `select count(*) from dedup.finv_dedup_usc u join public.finv_security s on s.usc=u.usc where nullif(s.currency_type,'') is not null and coalesce(u.currency,'')<>s.currency_type` 输出 `0`；空值数 `0` | |
| 10619 | 幂等：重跑迁移影响 0 行。 | 重跑输出 `UPDATE 0` 且 `COMMIT` | |
| 10620 | 迁移 `V200038` 登记成功，服务重启后数据完好（经历一次 PG 崩溃恢复）。 | `select success from schema_version where version='100038'` 输出 `t`；`select currency from dedup.finv_dedup_usc where usc='81810'` 仍为 `CNY` | |

## 五十五、清理 GCMain 误插入月K线（10621–10625）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10621 | 判定口径可复现：`time=0` + `date%100=1` + `volume>=100000` + `high-low>=20` 三条同时成立。 | `select count(*) from dedup.finv_quote_secu_kline_min where usc='GCMain' and time=0 and date%100=1 and volume>=100000 and (high-low)>=20` 执行前输出 `73`、执行后 `0` | |
| 10622 | 删除前整行备份到 `dedup.finv_quote_secu_kline_min_kline_purge`（含 `reason` / `backup_at`，主键 `(usc,ts)`），可回滚。 | `select count(*) from dedup.finv_quote_secu_kline_min_kline_purge` 输出 `73`；`select distinct reason from ...` 含 `MONTH_KLINE` | |
| 10623 | 仅删误插行，同位置正常分钟数据不得误伤。 | `select count(*) from dedup.finv_quote_secu_kline_min where usc='GCMain' and time=0 and date%100=1` 输出 `4`（`20260401/20260501/20260601/20260701`，volume 14~45） | |
| 10624 | GCMain 行数按预期减少 73。 | 执行前 `2729329` → 执行后 `2729256` | |
| 10625 | 幂等：重跑影响 0 行；迁移 `V200039` 登记成功。 | 重跑输出 `INSERT 0 0` / `DELETE 0`；`select success from schema_version where version='100039'` 输出 `t` | |

## 五十六、恢复 quote 回测隔离副本（10626–10631）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10626 | `quote` schema 下存在 `finv_quote_secu_kline_min`，与 `dedup` 同名且结构对齐（列/类型/精度）。 | `select count(*) from information_schema.columns where table_schema='quote' and table_name='finv_quote_secu_kline_min'` 输出 `19`（同 dedup） | |
| 10627 | 主键 `(ts, usc)`、5 个 CHECK、3 个业务索引、`gmt_update` 触发器齐备。 | `select contype, count(*) from pg_constraint where conrelid='quote.finv_quote_secu_kline_min'::regclass group by 1` 含 `p=1`、`c=5`；`select count(*) from pg_indexes where schemaname='quote' and tablename='finv_quote_secu_kline_min'` 输出 `4`；`pg_trigger` 有 `trg_finv_quote_secu_kline_min_gmt_update` | |
| 10628 | 数据已回填且与 `dedup` 一致（回填时点）。 | `quote` 与 `dedup` 各 `2804256` 行；GCMain 子集 `rows/vol/turnover` 三项完全相等 | |
| 10629 | 表注释明确「回测专用、与 dedup 隔离、必须写 quote. 前缀」。 | `select obj_description('quote.finv_quote_secu_kline_min'::regclass)` 含「量化回测专用」与「quote. 前缀」 | |
| 10630 | 测试闸门允许 `quote.` 前缀，不再把它排除在外。 | `grep -n 'finv_quote_secu_kline_min' internal/database/schema_qualify_test.go` 含 `{"dedup.", "quote.", "export."}`；`go test ./...` 通过 | |
| 10631 | 启动迁移超时放宽到 10 分钟（本次回填实测 240 秒，原 30 秒必超时）；迁移 `V76` 登记成功。 | `grep -n '10\*time.Minute' cmd/server/main.go` 有输出；`select success from schema_version where version='76'` 输出 `t` | |

## 五十七、分钟行情双 schema 读写分工（10632–10637）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10632 | `internal/meta/finv/quote`（`Import/Upload` 文件导入、`History/QuoteQuery` 查询）一律读写 `quote.finv_quote_secu_kline_min`。 | `grep -c 'quote.finv_quote_secu_kline_min' internal/meta/finv/quote/service.go` ≥ 4；`grep 'dedup.finv_quote_secu_kline_min' internal/meta/finv/quote/*.go` 无输出 | |
| 10633 | `internal/meta/finv/quant/backtest` 行情加载与日期区间解析读取 `quote.`（回测只读 quote）。 | `grep -n 'quote.finv_quote_secu_kline_min' internal/meta/finv/quant/backtest/service.go` 输出 2 行；包内无 `dedup.finv_quote_secu_kline_min` | |
| 10634 | `internal/meta/finv/quote/cdeu/kline`（K 线 API 写入/查询）保持 `dedup.`，不得改回 `quote.`。 | `grep -n 'const Table' internal/meta/finv/quote/cdeu/kline/sql.go` 输出 `"dedup.finv_quote_secu_kline_min"`；`go test ./internal/meta/finv/quote/cdeu/kline` 通过 | |
| 10635 | 端到端隔离：K 线 API `/Add` 写入后数据仅落 `dedup`，`History/QuoteQuery` 查同一 `usc` 查不到。 | 实测：`/Add` 返回 201 后 `dedup=1 / quote=0`；`QuoteQuery` 返回 `total: 0` | |
| 10636 | `schema_qualify_test.go` 白名单含 `quote.`、`dedup.`、`export.` 三者，`go test ./...` 全绿。 | `grep -n 'finv_quote_secu_kline_min' internal/database/schema_qualify_test.go` 含三者；`go test ./...` 通过 | |
| 10637 | 文档同步：导入/回测/查询 API 文档与 `DbSchemaSpec` 均按新归属描述。 | `Docs/API/HistoryQuote/ImportsUpload.md`、`Docs/API/Backtest/RunCreate.md` 含 `quote.finv_quote_secu_kline_min`；`DbSchemaSpec.md` 隔离表列出 `internal/meta/finv/quote` / `internal/meta/finv/quote/cdeu/kline` 归属 | |

## 五十八、数据库表保护红线（10638–10645）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10638 | `DbSchemaSpec.md` 新增 **§0 红线章节**，优先级高于全文其它章节，且位于 §1 之前。 | `grep -n '^## 0\.' Docs/DevSpec/DbSchemaSpec.md` 输出行号小于 `^## 1\.` 的行号 | |
| 10639 | 红线章列出**绝对禁止操作表**：`DROP TABLE`/`DROP SCHEMA`/`SET SCHEMA`/`RENAME`/`TRUNCATE`/无条件 `DELETE`/合并废弃同名表。 | `sed -n '/## 0\./,/## 1\./p' Docs/DevSpec/DbSchemaSpec.md` 含全部七类操作关键词 | |
| 10640 | 红线章含**同名表保护名单**，明确 `quote.`/`dedup.`/`export.` 三份 `finv_quote_secu_kline_min` 必须并存、禁止合并，并说明隔离理由（消费者/加工者分离）。 | 同上区间内 grep `finv_quote_secu_kline_min` 至少 3 次，且含「禁止合并」与「消费者」「加工者」 | |
| 10641 | 红线章明确**授权判定标准**：仅「用户明确点名」或「`RequirementList` 有条目」算授权，沉默/推断/旧清单均不算。 | 同上区间内含「明确授权」专节与四条「不算授权」情形 | |
| 10642 | 红线章给出**文档冲突仲裁顺序**，指出 `2610-003` 的旧验收标记不可当事实。 | 同上区间内含「仲裁顺序」与 `2610-003` | |
| 10643 | 红线章含**改库前自检清单**（6 问）与**删除数据的强制要求**（整行备份 + `reason`/`backup_at` + 单事务 + `RAISE EXCEPTION`）。 | 同上区间内 grep `0\.6` 与 `0\.7` 小节均存在 | |
| 10644 | `Docs/Refactor/2610-003-MoveTablesToSchemas.md` 加**时效声明横幅**，并把 `C-10`/`C-11`/`C-16`/`C-18`/`C-20` 状态改为「⚠ 作废」。 | `grep -c '⚠ \*\*作废\*\*' Docs/Refactor/2610-003-MoveTablesToSchemas.md` 输出 `5`；文件头含 `时效声明` | |
| 10645 | 仓库级 `AGENTS.md`「工作约定」首条为数据库红线，指向 `DbSchemaSpec.md` §0，并说明 V38 事故。 | `sed -n '/## 工作约定/,/^$/p' AGENTS.md` 首条含「红线」与 `DbSchemaSpec.md` | |

## 五十九、系统配置 20005：CDEU 工作流运行模式（10646–10652）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10646 | `sys.sys_config` 新增 `20005`「CDEU工作流运行模式」，字符串类型（`data_type='3'`），默认值 `Manual`。 | `select config_no, data_type, str_value from sys.sys_config where config_no=20005` 输出 `20005 \| 3 \| Manual` | |
| 10647 | 迁移 `V200040` 幂等且**不覆盖用户已配值**（`ON CONFLICT DO NOTHING` + 快照硬校验）；重跑影响 0 行。 | 重跑后 `schema_version` 仍 `success=t`；已有值被改写时抛 `RAISE EXCEPTION` | |
| 10648 | `Manual`（默认）时进程内调度器**不自动触发任何 CDEU Job**；手动 `POST .../Job/Run` 不受限。 | 部署后日志含「CDEU 运行模式 = Manual（默认）：调度器不自动触发任何 Job」；间隔类 Job 到期后 6 分钟零新增 `finv_dedup_job_run_log_export_file` 记录 | |
| 10649 | `Auto` 时按定时任务时间自动跑（行为同改造前）。 | 将 20005 改为 `Auto` 并重启后，日志含「CDEU 运行模式 = Auto」，间隔类 Job 到期后出现新运行日志 | |
| 10650 | 取值大小写不敏感；空值/未知值/未配置一律按 `Manual` 兜底（安全侧不自动跑）。 | `go test ./internal/meta/finv/sys -run 'TestNormalizeCDEUMode\|TestValuesCDEUScheduleEnabled'` 通过 | |
| 10651 | `GET .../DeDup/Status` 的 `options` 暴露 `cdeu_mode` / `cdeu_schedule_enabled` / `cdeu_mode_error`。 | 调用接口返回 `"cdeu_mode":"Manual"`、`"cdeu_schedule_enabled":false` | |
| 10652 | 文档同步：`DbSchemaSpec` 种子清单、`CHANGELOG`、`Docs/API/QuoteDeDup/Status.md` 均描述 20005。 | 三处 `grep -c '20005'` 均 ≥ 1；`Status.md` 含 `cdeu_mode` 字段说明 | |

## 六十、导出低峰窗口改为系统配置 20006（10653–10660）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10653 | **【删除表授权记录】** 用户于 2026-10-27 明确授权删除 `dedup.finv_dedup_runtime_setting`（原话：“表删除，留着过年？”）。删除前硬校验表内仅含 `EXPORT_WINDOW_START`/`EXPORT_WINDOW_END` 两个键，发现其他键即中止。 | `select count(*) from pg_tables where schemaname='dedup' and tablename='finv_dedup_runtime_setting'` 输出 `0`；迁移 `V200041` 日志含“已删除” | |
| 10654 | `sys.sys_config` 新增 `20006`「导出低峰窗口」，字符串类型，**精确到秒**，格式 `HH:MM:SS\|HH:MM:SS`（半角竖线分隔）。 | `select str_value from sys.sys_config where config_no=20006` 匹配 `^\d{2}:\d{2}:\d{2}\|\d{2}:\d{2}:\d{2}$` | |
| 10655 | 配置初始发放值固定为 `00:30:00\|06:30:00`（**不从旧表搬运现值**，初始值与实际运行值不一致属正常，实际值由运维配置）；`ON CONFLICT DO NOTHING` 不覆盖用户已配值。 | `select str_value from sys.sys_config where config_no=20006` 为 `00:30:00\|06:30:00`；迁移重跑影响 0 行 | |
| 10656 | 环境变量 `FINV_QUOTE_DEDUB_WINDOW_START`/`_END` 废弃，窗口一律取 20006；缺失/非法时按内置默认 `00:30:00\|06:30:00` 降级。 | `grep -c 'FINV_QUOTE_DEDUB_WINDOW' Deploy/Docker/docker-compose.yml` 输出 `0`；`grep -n 'WINDOW' Deploy/Docker/.env.example` 仅剩废弃说明注释 | |
| 10657 | `PUT /Settings/Window` 接口保留，内部改写 20006；**严格接受 `HH:MM:SS`**，拒绝 `HH:MM`。 | 传 `{"window_start":"01:15","window_end":"06:30"}` 返回 400；传 `HH:MM:SS` 返回 200 且落库格式一致 | |
| 10658 | `GET /Settings` / `GET /Status` 返回秒级窗口，`source` 取值改为 `sysconfig` / `default`。 | `GET .../Settings` 返回 `"window_start":"HH:MM:SS"`、`"source":"sysconfig"` | |
| 10659 | 代码分层：窗口读取/写入统一走 `syscfg`（`KeyExportWindow`/`ParseExportWindow`/`FormatExportWindow`/`FormatClockSeconds`/`Store.WriteString`），`dedup` 包不再直连配置表。 | `grep -rn 'finv_dedup_runtime_setting' internal/` 无输出；`go vet ./... && go test ./...` 通过 | |
| 10660 | 前端时间控件精确到秒（`step="1"`），来源文案改为「系统配置 20006 / 内置默认值」。 | `grep -n 'step="1"' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 输出 2 行 | |

## 六十一、待办：sys_config 通用管理接口（10661）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10661 | **【待办，本次不实现】** 提供 `sys.sys_config` 通用管理接口（列表 / 详情 / 修改），供运维在界面维护 20000–20006 等配置；`20003`（GitHub PAT）等敏感项在读响应中必须掩码。**接口路径与端点待后续确定**，不在本次范围内。 | 待接口设计定稿后补充核对方式 | 待办 |

## 六十二、预算与窗口仅约束自动调度（10662–10666）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10662 | **日预算与低峰窗口只约束自动调度（`CDEUTriggerSchedule`）；手动触发（`Job/Run`）两者均不受限。** | 把当日预算置为 `consumed_rows = budget_rows`（`remaining=0`）后手动 `POST .../Job/Run {"job_kind":"RUN_DUE_EXPORT_JOBS"}`，观察运行日志仍在推进、`EXPORTED` 数递增 | |
| 10663 | 触发来源通过 `Result.Trigger` 与 context（`withTrigger`/`TriggerFromContext`/`IsManualTriggerFromContext`）传递，下沉到 LOAD 分页循环，不只入口一处。 | `grep -rn 'IsManualTriggerFromContext' internal/meta/finv/quote/dedup/` 命中 `job_export.go` 的 `loadDate` | |
| 10664 | 手动触发仍写入 `finv_dedup_daily_budget.consumed_rows`（保留审计与 Auto 模式基线），进度详情标注「手动触发：不限制」。 | `GET .../RunLog/Detail?runId=<手动运行>` 的 `progress_detail` 含「手动触发：不限制」 | |
| 10665 | 调度触发行为不变：`20005=Manual` 不触发；`Auto` 时窗口外不触发、预算耗尽即停。 | 部署后日志含「CDEU 运行模式 = Manual」且无新增运行日志；预算耗尽时调度触发输出「预算或窗口耗尽」 | |
| 10666 | 文档同步：`Docs/API/QuoteDeDup/JobRun.md` 新增「手动触发与调度触发的约束差异」对照表。 | `grep -n '手动触发与调度触发的约束差异' Docs/API/QuoteDeDup/JobRun.md` 命中 | |

## 六十三、CDEU 概览展示动态 JOB 实例（10667–10670）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10667 | 概览接口对每个 process 返回**全部动态实例**，不因 `workflow_run_id != 当前 run` 而被过滤掉。 | `GET .../CDEU/Overview?runId=0` 中 `PROCESS_EXPORT_FILE_MON_BASE` 的 `jobs` 里 `is_dynamic=true` 的条目数 = 89 | |
| 10668 | 动态实例带 `from_current_run` 标记，前端可区分「本次运行的实例」与「在办存量实例」。 | 同上响应中存在 `from_current_run` 字段；切 `runId=49` 时 BASE 的 `from_current_run=true` 为 89 | |
| 10669 | 前端 process 节点文本显示 `x/y JOB · N 实例（成功 k）`，悬浮提示列出前 8 条实例明细。 | 打开 CDEU 工作流页，`PROCESS_EXPORT_FILE_MON_BASE` 节点显示「1/1 JOB · 89 实例（成功 89）」 | |
| 10670 | 静态 JOB 计数与动态实例计数分开统计，互不污染。 | 前端 `processJobText` 分别用 `is_dynamic` 分组；`PROCESS_EXPORT_FILE_MON_BASE` 静态显示 1、实例显示 89 | |

## 六十四、系统配置 20007：导出成功后删除主表数据（10671–10678）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10671 | `sys.sys_config` 新增 `20007`，`data_type='2'`，`char_value` 默认 `1`；迁移 `V200042` 幂等且不覆盖用户已配值。 | `select data_type, char_value from sys.sys_config where config_no=20007` 输出 `2 \| 1`；重跑影响 0 行 | |
| 10672 | **`20007=1`**：文件**远端上传成功后**删除 `dedup.finv_quote_secu_kline_min` 中该 `(usc, biz_month)` 的数据；上传失败不删。 | 触发 `UPLOAD_PENDING_EXPORT_FILES` 后查该月主表行数归零；upload job 的 `details.main_rows_purged` > 0 | |
| 10673 | **`20007=0`**：不删主表；重复导出重写同名 BASE 覆盖，**不生成 INCR 文件**。 | 置 20007=0 后制造增量并触发 `SCAN_POST_SEAL_PATCHES`，`finv_dedup_export_file` 中不新增 `file_type='PATCH'` 记录，BASE 记录 `version` 递增且文件名不变 | |
| 10674 | 删除范围仅 `dedup` schema；`quote.finv_quote_secu_kline_min` 行数不受影响。 | 20007=1 清理前后对比 `quote` 主表行数不变 | |
| 10675 | INCR 文件名序号 `_Incr_N` 随 `patch_count` 递增；BASE 文件名不含版本号、不存在递增。 | 连续两次增量导出后文件名分别为 `..._Incr_1.mvsv`、`..._Incr_2.mvsv`；BASE 文件名不变 | |
| 10676 | 旧延迟清理下线：`CLEAN_EXPIRED_EXPORT_ARTIFACTS` 不再按月删主表；`FINV_QUOTE_DEDUB_PURGE_MAIN_ENABLED`/`_PURGE_MAIN_MONTHS` 废弃。 | `grep -rn 'PURGE_MAIN' internal/ Deploy/Docker/docker-compose.yml` 仅剩 `.env.example` 的废弃说明注释；clean job 的 `details` 含 `purge_after_export` 而非 `main_deleted` | |
| 10677 | 两种模式分支互斥：`20007=0` 强制 `rewriteBase`，`20007=1` 保持分级逻辑（未上传成功则重写、已上传则 INCR）。 | `grep -n 'forceRewriteBase' internal/meta/finv/quote/dedup/job_patch.go` 命中；开启 20007=1 时补齐逻辑不报「主表无数据」 | |
| 10678 | 删除不做行级备份，但记录删除行数到运行进度与 upload job `details.main_rows_purged`。 | 运行日志 `progress_detail` 含「20007 主表清理：… 删除 N 行」 | |

## 六十五、CDEU 运行实例表去除自增代理键（10679–10686）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10679 | `cdeu_process_run` / `cdeu_job_run` 必须有 `workflow_run_id` 冗余列，且按运行筛选 JOB 无需 JOIN 三层。 | `select * from dedup.cdeu_job_run where workflow_run_id=54` 可直接执行并返回 15 行 | |
| 10680 | `cdeu_process_run` 主键为 `(workflow_run_id, process_id)`，不存在 `process_run_id` 列。 | `select pg_get_constraintdef(oid) from pg_constraint where conname='cdeu_process_run_pkey'` 输出 `PRIMARY KEY (workflow_run_id, process_id)`；`information_schema.columns` 无 `process_run_id` | |
| 10681 | `cdeu_job_run` 主键为 `(workflow_run_id, process_id, job_id)`，不存在 `job_run_id` / `process_run_id` 列。 | 同上方式核对 | |
| 10682 | `cdeu_job_step_run` 保留自增 `step_run_id`，`job_run_id` 改为 `(workflow_run_id, process_id, job_id)` 三元组引用。 | `\d dedup.cdeu_job_step_run` 显示三列且唯一键为 `(workflow_run_id, process_id, job_id, step_code)` | |
| 10683 | `cdeu_job_run.dynamic_job_id` 可空，批次 JOB 指向 `cdeu_job_dynamic`，非批次 JOB 为 NULL。 | `select count(*) from dedup.cdeu_job_run where dynamic_job_id is not null` 与动态实例执行次数一致 | |
| 10684 | 历史脏数据删除前必须整行备份到 purge 表。 | `select count(*) from dedup.cdeu_job_run_purge` = 589；`cdeu_job_step_run_purge` = 433 | |
| 10685 | `CDEU/Overview` 不再返回 `job_run_id` / `process_run_id`，前端显示 `第 N 次`。 | 接口响应正文不含这两个字段；界面「运行编号」显示 `第 N 次` 或「未运行」 | |
| 10686 | 迁移必须做 DDL 前整表快照，且收敛校验三表行数不变。 | 存在 `dedup.cdeu_process_run_bak_v100044` 等三张备份表；`schema_version` 中 `100043`/`100044` 均 `success=t` | |

## 六十六、动态 JOB 按运行隔离（10687–10693）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10687 | `cdeu_job_dynamic` 的唯一键必须包含 `workflow_run_id`，同一证券月在不同运行下各自独立。 | `pg_indexes` 中存在 `uq_cdeu_job_dynamic_export_run (workflow_run_id, usc, biz_month) NULLS NOT DISTINCT`；两次不同 run 插入同一 `(usc,biz_month)` 均成功 | |
| 10688 | 同一运行内重复插入同一 `(usc, biz_month)` 必须被唯一约束拒绝。 | 同 run 重复 INSERT 报 `unique_violation` | |
| 10689 | `workflow_run_id IS NULL`（手动单实例执行）时也按唯一处理。 | 两条 `workflow_run_id = NULL` 的同 `(usc,biz_month)` 记录，第二条报 `unique_violation` | |
| 10690 | 重新运行流程时不得覆盖上一轮动态实例的 `workflow_run_id` 与状态。 | 连续两次运行后，`select workflow_run_id, status from dedup.cdeu_job_dynamic where usc=? and biz_month=?` 返回两行且 `workflow_run_id` 不同 | |
| 10691 | `CDEU/Overview?runId=N` 只返回属于第 N 次运行的动态实例。 | run 54 无动态实例（只显示 15 个静态 JOB）；run 49 显示 BASE 90 / INCR 12 / UPLOAD 202 | |
| 10692 | 历史运行动态实例必须有保留策略，默认保留 30 天。 | `CLEAN_EXPIRED_EXPORT_ARTIFACTS` 的 `details.stale_dynamic_deleted` 字段存在；`PurgeStaleDynamicJobRuns` 只删「所属运行已终态 且 实例已终态」的记录 | |
| 10693 | 迁移不得删除任何存量动态实例。 | 迁移前后 `select count(*) from dedup.cdeu_job_dynamic` 均为 507 | |

## 六十七、系统配置 20008：动态 JOB 保留天数（10694–10698）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10694 | `sys.sys_config` 新增 `20008`「历史运行动态JOB保留天数」，`data_type='1'`，`int_value` 默认 `30`；迁移 `V200046` 幂等且不覆盖用户值。 | `select data_type, int_value from sys.sys_config where config_no=20008` 输出 `1 \| 30`；重跑影响 0 行 | |
| 10695 | 合法区间 `1~3650`，越界或读取失败时回退默认 `30`。 | 置 `20008=0` 后 `GET .../Settings` 的 `dynamic_job_retention_days` 返回 `30` | |
| 10696 | `CLEAN_EXPIRED_EXPORT_ARTIFACTS` 按 20008 的天数清理历史动态实例。 | 运行日志 `progress_detail` 含「保留 N 天」；完成日志含 `retention_days=N` | |
| 10697 | 清理只针对「所属运行已终态 且 实例自身已终态」的记录，当前运行不受影响。 | `PurgeStaleDynamicJobRuns` 的 SQL 含 `wr.status IN (...)`、`dj.status IN (...)`、`wr.gmt_create < now() - make_interval(days => $1)` 三重条件 | |
| 10698 | `Settings` / `Status` 接口暴露 `dynamic_job_retention_days`。 | 两个接口的响应中均能取到该字段且值等于 20008 的 `int_value` | |

## 六十八、20007 沉淀月补偿清理（10699–10704）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10699 | 20007=1 时，`CLEAN_EXPIRED_EXPORT_ARTIFACTS` 必须清理「已沉淀」证券月的主表数据（BASE 已导出+已上传，且 BASE 生成后数据未再变化）。 | 运行后 `details.settled_months_purged` > 0 且 `dedup.finv_quote_secu_kline_min` 对应月份行数归零 | |
| 10700 | 补偿清理不得误删：BASE 生成之后有任何 `gmt_create`/`gmt_update` 晚于 `BASE.gmt_create` 的行时，该月不得被清理。 | 事务内插入 1 行 `gmt_create=now()` 后，`SettledSecurityMonths` 不再返回该月 | |
| 10701 | 补偿清理只作用于 `dedup` schema，`quote` 副本行数不变。 | 清理前后 `select count(*) from quote.finv_quote_secu_kline_min` 相同 | |
| 10702 | 数据释放后增量链路自动恢复：新写入数据的 `delta > 0`，`SCAN_POST_SEAL_PATCHES` 会生成 INCR 任务。 | 清空后插入 1 行新数据，`CountMainDelta` 返回 > 0 | |
| 10703 | 20007=0 时补偿清理不执行。 | 置 20007=0 后运行 clean job，`details.settled_months_purged` 为 0 | |
| 10704 | 机制必须在文档中明确（两条触发路径 + 判定条件 + 与增量链路的关系）。 | `Docs/Refactor/2610-005-FinvQuoteDeDubExportUpload.md` 存在 §4.6 章节 | |

## 六十九、CDEU 页面展示运行模式（10705–10706）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10705 | `GET /CDEU/Overview` 返回 `cdeu_mode`（20005 取值 `Manual`/`Auto`）与 `cdeu_schedule_enabled`，读取失败按 `Manual` 兜底。 | 接口响应含两字段；把 20005 改为 `Auto` 后 `cdeu_schedule_enabled=true` | |
| 10706 | `/Meta/Finv/Quote/DeDup/CDEU` 页头「运行模式」展示 20005 的「自动/手动」，替换原先的 `workflow.run_mode`。 | 页面页头 chip 显示「运行模式 手动」或「运行模式 自动」，悬浮提示含「系统配置 20005」 | |

## 七十、CDEU 父级状态修复与运行中动效（10707–10712）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10707 | Process 子级必须从 `cdeu_job` 定义表取（LEFT JOIN `cdeu_job_run`），运行记录缺失时状态按 `PENDING`，不得因空子集判父级 `SUCCESS`。 | 用不存在的 runId 模拟：返回 5 条 `PENDING` 子级而非 0 条 | √ |
| 10708 | Phase / Workflow 子级同理以 `cdeu_process` / `cdeu_phase` 定义为基准。 | 同上方式验证 phase_id=3 返回 4 条 PENDING、工作流返回 6 个 Phase | √ |
| 10709 | 已终态父级在仍有活跃子级时能回退：`TouchCDEUHierarchy` 允许 `SUCCESS`/`SKIPPED` → `RUNNING` 并清空 `finished_at`；`EvaluateCDEUProcess/Phase` 检测到活跃子级时重算。 | 父级 `finished_at` 不得早于其子级 `started_at` | √ |
| 10710 | 全流程拓扑中 `RUNNING` 的阶段框与 Process 框必须有青色跑马灯动效。 | 页面中阶段框与 Process 框出现斜向流光动画，颜色为青色 `rgb(0,229,255)` | √ |
| 10711 | 动效必须尊重系统「减少动画」偏好。 | 开启 `prefers-reduced-motion: reduce` 后动画停止，仅保留静态高亮 | √ |
| 10712 | 跑马灯只作用于 Phase 与 Process 两级，JOB 级不加（JOB 不在拓扑图展示）。 | 拓扑图 JOB 区域无动效 | √ |
| 10713 | CDEU 全流程拓扑与 CDEU 主页面中，阶段（Phase）与流程（Process）的进度条右侧需展示完成百分比文本（如 `62%`），数值与进度条宽度同源。**（由 10748 落地）** | 阶段框与 Process 框的进度条右侧出现百分比；未开始时显示 `0%`，完成时显示 `100%` | √（见 10748） |

## 七十一、清理历史备份表（10714）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10714 | 经用户明确授权，删除全部历史备份/快照表（迁移快照 5 张、CDEU 脏数据备份 2 张、导出测试快照 8 张、行情清理备份 2 张），删除前须校验无依赖且不涉及主表。 | `select relname from pg_class where relname like '%_bak%' or relname like '%_purge%'` 返回 0 行；`dedup`/`quote`/`export` 三张主表行数不变 | √ |

## 七十二、CDEU 动态 Step 表改名（10715）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10715 | `dedup.cdeu_job_dynamic_step` 重命名为 `dedup.cdeu_job_step_dynamic`，其序列、主键、2 个唯一约束、检查约束、8 个非空约束、索引同步改名（旧前缀 `cdeu_dynamic_job_step_` 全部消除），数据与列定义不变。 | `select to_regclass('dedup.cdeu_job_step_dynamic')` 非 NULL 且 `to_regclass('dedup.cdeu_job_dynamic_step')` 为 NULL；`select count(*) from pg_constraint where conname like 'cdeu\_dynamic\_job\_step\_%'` 为 0；行数仍为 1,345；`go test ./...` 通过 | √ |

## 七十三、20007 主表清理改为归档复核阶段独立 JOB（10716）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10716 | 将「导出成功后删除 dedup 主表行情数据」（20007）从上传 JOB 内联逻辑中拆出，改为独立 JOB `JOB_PURGE_EXPORTED_MONTH`（中文名「已归档行情数据清理」），并定义在 **`PHASE_REVIEW`（归档复核）阶段的 `PROCESS_REVIEW` 流程**下；上传 JOB 只负责上传，不再删数据。 | `grep -F "JOB_PURGE_EXPORTED_MONTH" internal/meta/finv/quote/dedup/cdeu_store.go` 命中；`select count(*) from dedup.cdeu_job j join dedup.cdeu_process p on p.process_id=j.process_id join dedup.cdeu_phase ph on ph.phase_id=p.phase_id where ph.phase_code='PHASE_REVIEW' and j.job_code='JOB_PURGE_EXPORTED_MONTH'` = 1；`UPLOAD_PENDING_EXPORT_FILES` 的 `details` 不再含 `main_rows_purged` | √ |

### 10716 背景与现状

当前实现（2026-10-27 时点）：

| 项 | 内容 |
|---|---|
| 承载 JOB | `JOB_UPLOAD_FILES` / `UPLOAD_PENDING_EXPORT_FILES` |
| 代码位置 | `internal/meta/finv/quote/dedup/job/upload_pending_export_files.go:80-92` |
| 触发时机 | 每个 BASE / PATCH 文件**远端上传成功后立即**执行 |
| 删除范围 | `dedup.finv_quote_secu_kline_min` 该 `(usc, biz_month)` 全部行（分批 50,000） |
| 开关 | `sys_config` 20007（`'1'` 删 / `'0'` 保留） |
| 兜底 | `CLEAN_EXPIRED_EXPORT_ARTIFACTS` 的 `purgeSettledMainData`，只处理漏删月份 |

### 10716 要解决的问题

1. **删除早于复核**：文件刚上传成功、`PHASE_REVIEW`（归档复核）尚未执行，源数据已被删除。若复核阶段发现文件异常（行数不符、checksum 不一致、远端校验失败），`dedup` 侧数据已不可恢复，只能依赖 `quote` 副本，而 `quote` 与 `dedup` 不保证逐行一致。
2. **职责混杂**：上传 JOB 同时承担「上传」与「数据生命周期终结」两件事，语义上不属于同一职责边界。
3. **不可独立重跑与审计**：删除失败仅打日志后放弃，没有独立 JOB 记录、没有 `attempt`、没有前端实例，只能靠定时任务兜底。
4. **前端不可见**：`PROCESS_UPLOAD` 的动态实例只体现「上传」，看不到「清理」这个业务动作及其进度。
5. **`PHASE_REVIEW` 是空阶段**：该阶段已有 `PROCESS_REVIEW` 定义（`process_id=23`）但没有任何 JOB，正好承载该职责。

### 10716 目标设计

| 设计项 | 方案 |
|---|---|
| JOB code | `JOB_PURGE_EXPORTED_MONTH` |
| 中文名 | 已归档行情数据清理 |
| 归属 | `PHASE_REVIEW` → `PROCESS_REVIEW`（`seq_no` 置为该阶段首位） |
| 实例粒度 | 动态实例，一个 `(usc, biz_month)` 一条，前端可见进度与结果 |
| 数据源 | `dedup.finv_dedup_export_file` 按 `(usc, biz_month)` 聚合 |
| 触发前提 | 该证券月的**全部**文件（BASE + 所有 PATCH）`upload_status='UPLOADED'`，MANIFEST 已处理完毕 |
| 开关 | 沿用 20007：`'0'` 时该 JOB 整体 `SKIPPED`，不生成动态实例 |
| 幂等 | 已删空的 `(usc, biz_month)` 返回 0 行即完成，不报错 |
| 兜底 | 保留 `CLEAN_EXPIRED_EXPORT_ARTIFACTS` 的 `purgeSettledMainData` |

### 10716 待定设计点（实现前需确认）

1. **复核门禁**：是否必须等 `JOB_CHECK_HEALTH` 无 FAILED 才允许清理？还是只校验「该证券月全部文件已上传」？
2. **20007 语义**：是否把「上传后删」的语义改写为「复核后删」（配置项名称与 `remark` 需同步），还是保持 20007 仅作开关、由 JOB 位置体现时机？
3. **切换方式**：`UPLOAD_PENDING_EXPORT_FILES` 的内联清理是直接移除，还是先加过渡开关并存一个版本？
4. **失败重试**：删除失败是否引入 `attempt` + `gmt_next_run` 重试，还是维持「日志 + 兜底」？
5. **历史补删**：切换时已 `UPLOADED` 但未清理的月份如何一次性补做（是否复用该 JOB 的历史扫描模式）。

## 七十四、收尾阶段 JOB 重分配与解耦（10717–10722）

> 本节为 10716 的详细设计，**已随 `V300048__cdeu_review_phase_job_reorg.sql` 全部落地**（2026-10-27）。
> 后续演进：`10744` 起本地文件清理由 `PROCESS_PURGE` 的 `JOB_PURGE_LOCAL_FILES` 承担（20011 控制），`JOB_CLEAN_STORAGE` 只保留中转表残留清理。
> 用户已批准：① 拆分四个新流程；② `JOB_RECOVER_TIMEOUT` 移入；③ 执行顺序认可；④ 收尾 5 个 JOB 全部为关键 JOB（`affects_completion = true`）。

### 目标结构

```
PHASE_REVIEW 归档复核
├─ seq 1  PROCESS_RECOVER        超时任务回收   → JOB_RECOVER_TIMEOUT        （由 PHASE_EXPORT 迁入）
├─ seq 2  PROCESS_REVIEW         归档复核      → JOB_CHECK_HEALTH           （由 PHASE_UPLOAD 迁入）
├─ seq 3  PROCESS_PURGE          归档数据清理   → JOB_PURGE_EXPORTED_MONTH  （新建）
├─ seq 4  PROCESS_STORAGE_CLEAN  存储清理      → JOB_CLEAN_STORAGE         （由 JOB_CLEAN_EXPIRED 拆出）
└─ seq 5  PROCESS_RETENTION      运行记录保留   → JOB_PURGE_STALE_DYNAMIC   （由 JOB_CLEAN_EXPIRED 拆出）

PHASE_UPLOAD 月度行情上传
└─ PROCESS_UPLOAD → JOB_UPLOAD_FILES   （只上传，不再删主表）
```

### 前后对照

| JOB | 调整前 | 调整后 | 职责 |
|---|---|---|---|
| `JOB_UPLOAD_FILES` | `PHASE_UPLOAD`：上传 + 内联删主表 | `PHASE_UPLOAD`：**只上传** | 上传 |
| `JOB_CHECK_HEALTH` | `PHASE_UPLOAD` / `PROCESS_HEALTH` | **`PHASE_REVIEW` / `PROCESS_REVIEW`** | 指标汇总 + MVsv 抽检（只读） |
| `JOB_RECOVER_TIMEOUT` | `PHASE_EXPORT` / `PROCESS_RECOVER` | **`PHASE_REVIEW` / `PROCESS_RECOVER`** | 超时锁回收 |
| `JOB_PURGE_EXPORTED_MONTH` | — | **`PHASE_REVIEW` / `PROCESS_PURGE`** | 删 `dedup` 主表（扫描式） |
| `JOB_CLEAN_STORAGE` | 属 `JOB_CLEAN_EXPIRED` | **`PHASE_REVIEW` / `PROCESS_STORAGE_CLEAN`** | 中转表残留 + 本地过期文件 |
| `JOB_PURGE_STALE_DYNAMIC` | 属 `JOB_CLEAN_EXPIRED` | **`PHASE_REVIEW` / `PROCESS_RETENTION`** | 历史动态实例（20008） |
| `JOB_CLEAN_EXPIRED` | `PHASE_UPLOAD` / `PROCESS_LIFECYCLE_CLEAN` | **撤销** | — |

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10717 | 撤销 `JOB_CLEAN_EXPIRED`（`PROCESS_LIFECYCLE_CLEAN`），其四类职责拆为三个独立 JOB：`JOB_PURGE_EXPORTED_MONTH`（删 `dedup` 主表）、`JOB_CLEAN_STORAGE`（中转表残留 + 本地过期文件）、`JOB_PURGE_STALE_DYNAMIC`（历史动态实例，20008）。 | `grep -F "JOB_CLEAN_EXPIRED" internal/` 无命中；`select count(*) from dedup.cdeu_job where job_code in ('JOB_PURGE_EXPORTED_MONTH','JOB_CLEAN_STORAGE','JOB_PURGE_STALE_DYNAMIC')` = 3 | √ |
| 10718 | `PHASE_REVIEW` 下新建四个流程并按序执行：`PROCESS_RECOVER`(1) → `PROCESS_REVIEW`(2) → `PROCESS_PURGE`(3) → `PROCESS_STORAGE_CLEAN`(4) → `PROCESS_RETENTION`(5)；`PROCESS_RECOVER`、`PROCESS_HEALTH` 从原阶段迁入/改挂。 | `select process_code, seq_no from dedup.cdeu_process p join dedup.cdeu_phase ph on ph.phase_id=p.phase_id where ph.phase_code='PHASE_REVIEW' order by seq_no` 返回上述 5 行 | √ |
| 10719 | `JOB_CHECK_HEALTH` 归属由 `PHASE_UPLOAD`/`PROCESS_HEALTH` 改为 `PHASE_REVIEW`/`PROCESS_REVIEW`；`JOB_RECOVER_TIMEOUT` 归属由 `PHASE_EXPORT`/`PROCESS_RECOVER` 改为 `PHASE_REVIEW`/`PROCESS_RECOVER`。 | 上述两个 JOB 的 `process_id` 均指向 `PHASE_REVIEW` 下的流程 | √ |
| 10720 | `JOB_PURGE_EXPORTED_MONTH` 采用**扫描式判据**，合并原「上传后内联删除」与「沉淀月补偿清理」两个入口：按 `(usc, biz_month)` 聚合 `finv_dedup_export_file`，凡「全部文件 `upload_status='UPLOADED'` 且该月主表仍有数据」即删除，不再区分本次上传与历史遗留；20007=`'0'` 时整体跳过；幂等（删空返回 0 即完成）。 | 删除判据 SQL 中无「本次上传」条件；对同一月份重复执行第二次删除行数为 0；`UPLOAD_PENDING_EXPORT_FILES` 的 `details` 不再含 `main_rows_purged` | √ |
| 10721 | `PHASE_REVIEW` 下五个 JOB 的 `affects_completion` 全部设为 `true`（关键 JOB），任一失败即阻断整轮归档完成判定。 | `select count(*) from dedup.cdeu_job j join dedup.cdeu_process p on p.process_id=j.process_id join dedup.cdeu_phase ph on ph.phase_id=p.phase_id where ph.phase_code='PHASE_REVIEW' and j.affects_completion = false` = 0 | √ |
| 10722 | `PHASE_UPLOAD` 精简为只含 `PROCESS_UPLOAD` → `JOB_UPLOAD_FILES` 一个流程一条 JOB，内联主表清理代码移除。 | `select count(*) from dedup.cdeu_process p join dedup.cdeu_phase ph on ph.phase_id=p.phase_id where ph.phase_code='PHASE_UPLOAD'` = 1；`grep -F "purgeMainSecurityMonth" internal/meta/finv/quote/dedup/job/upload_pending_export_files.go` 无命中 | √ |

## 七十五、按 workflow_run_id 清理历史流程运行数据（10723，已落地）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10723 | 支持按 `workflow_run_id` 整体清理 CDEU 历史运行数据以释放数据库空间，提供两种可选删除策略：① `Days`——删除超过 N 天（默认 30）的 `workflow_run`；② `Count`——只保留最近 N 个（默认 30 个）`workflow_run_id`。策略与阈值走 `sys_config`，允许 `Off` 关闭。 | `select config_no from sys.sys_config where config_no in (20009,20010)` 返回 2 行；构造 >30 天或 >30 个的历史 run 后执行清理 JOB，对应 run 及其子表行数归零，`cdeu_workflow_run` 剩余数量符合策略 | √ |

### 10723 清理范围与依赖顺序

清理以 `workflow_run_id` 为根，按子表→父表顺序删除，全部在单事务内完成：

| 顺序 | 表 | 关联条件 |
|---|---|---|
| 1 | `dedup.cdeu_job_step_dynamic` | `dynamic_job_id IN (SELECT dynamic_job_id FROM cdeu_job_dynamic WHERE workflow_run_id = $1)` |
| 2 | `dedup.cdeu_job_dynamic` | `workflow_run_id = $1` |
| 3 | `dedup.cdeu_job_step_run` | `workflow_run_id = $1` |
| 4 | `dedup.cdeu_job_run` | `workflow_run_id = $1` |
| 5 | `dedup.cdeu_process_run` | `workflow_run_id = $1` |
| 6 | `dedup.cdeu_phase_run` | `workflow_run_id = $1` |
| 7 | `dedup.finv_dedup_job_run_log_line` | `run_id IN (该 run 下所有 cdeu_job_run.run_log_id ∪ cdeu_job_dynamic.run_log_id)` |
| 8 | `dedup.finv_dedup_job_run_log_export_file` | 同上（`run_id` 为日志批次 ID，非 `workflow_run_id`） |
| 9 | `dedup.cdeu_workflow_run` | `run_id = $1` |

> 注意：`finv_dedup_job_run_log_line.run_id` / `finv_dedup_job_run_log_export_file.run_id` 是**JOB 运行日志批次 ID**（对应 `cdeu_job_run.run_log_id`），不是 `workflow_run_id`，必须经子查询间接关联。

### 10723 安全约束

1. **只删终态 run**：`cdeu_workflow_run.status IN ('SUCCESS','FAILED','PARTIAL','CANCELLED','SKIPPED')`；`PENDING`/`READY`/`RUNNING` 一律跳过。
2. **永不删当前 run**：排序后最近 1 个 `workflow_run_id` 无条件保留（防止 `Count=1` 或时钟异常误删正在跑的流程）。
3. **孤儿 run**：`cdeu_job_run` 中存在 `workflow_run_id` 在 `cdeu_workflow_run` 已无对应行的情况（当前有 13 个此类历史 run，如 40–48、50），清理时应按 `cdeu_job_run.workflow_run_id` 汇总扫描，不依赖 `cdeu_workflow_run` 是否存在。
4. **删除前统计**：逐 run 记录各表删除行数，写入 JOB `details` 与日志，便于审计。
5. **不备份**（用户口径：运行日志无长期价值），但需记录删除摘要。

### 10723 与 20008 的关系（需在实现前明确）

| 配置 | 现状 | 建议 |
|---|---|---|
| `20008` 历史运行动态JOB保留天数（默认 30） | 由 `PurgeStaleDynamicJobRuns` 单独清理 `cdeu_job_dynamic` | 按 run 级联清理覆盖后，20008 降级为**兜底**（处理无对应 run 的孤立动态实例）；或直接由 20009/20010 取代 |

### 10723 建议配置

| 编号 | 名称 | 类型 | 取值 |
|---|---|---|---|
| `20009` | 历史流程运行数据清理策略 | `'3'`（字符串） | `Off` / `Days` / `Count`（默认 `Days`） |
| `20010` | 历史流程运行数据保留阈值 | `'1'`（整数） | 天数或个数，默认 `30`；越界回退默认 |

### 10723 当前数据量（2026-10-27 基线，供实现后对比）

| 表 | 行数 | 占用 |
|---|---|---|
| `finv_dedup_job_run_log_line` | 39,094 | 8.0 MB |
| `finv_dedup_job_run_log_export_file` | 7,100 | 2.8 MB |
| `cdeu_job_step_dynamic` | 1,345 | 520 kB |
| `cdeu_job_dynamic` | 202 | 512 kB |
| `cdeu_job_run` | 258 | 504 kB |
| `cdeu_job_step_run` | 258 | 272 kB |
| `cdeu_process_run` | 266 | 240 kB |
| `cdeu_workflow_run` | 5 | 112 kB |
| `cdeu_phase_run` | 96 | 112 kB |

### 10723 执行位置（建议）

挂在 10717 新建的 `JOB_PURGE_STALE_DYNAMIC`（`PHASE_REVIEW` / `PROCESS_RETENTION`）上扩展，或在该流程下新增 `JOB_PURGE_HISTORY_RUNS` 排在它之前——因两者职责相邻（都属"保留期清理"），实现时二选一。

## 七十六、run 59 暴露的 5 个缺陷（10724–10728，本轮修复）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10724 | **导出 LOAD 游标越界导致 int4 溢出、JOB 卡死**：`load_cursor_date` 为 `0`（非 NULL）时被当作有效游标，`nextDate(0)` 计算 `time.Date(0,0,0)` 得到负数日期并逐次放大（`0 → -8799 → -79475 → … → -2147469187`），最终 `CountMainDate` 绑定 int4 参数溢出报 `-2147539797 is less than minimum value for int4`。修复：游标必须落在 `[firstDate, lastDate]` 区间内才采纳，否则回退 `firstDate`；`nextDate` 对非法输入返回原值避免无限下探。 | 手动把某 job 的 `load_cursor_date` 置 0 后执行，LOAD 从当月 1 日开始装载而非负数日期；日志中不出现 `date=-…` | √ |
| 10725 | **空流程被误判 SUCCESS**：`PROCESS_REVIEW`（归档复核）有流程定义但 `cdeu_job` 中 0 条 JOB，`cdeuProcessChildren` 返回空集 → `decideCDEUStatus(total=0)` 判 `SUCCESS`，并向上传染使 `PHASE_REVIEW` 也 `SUCCESS`，而 `PHASE_EXPORT`/`PHASE_UPLOAD` 仍在 `RUNNING`。修复：流程/阶段在「有定义但子级集合为空」时不得判完成，应保持 `PENDING`。 | run 59 中 `PHASE_REVIEW` 不得早于 `PHASE_EXPORT`/`PHASE_UPLOAD` 完成；`select status from dedup.cdeu_phase_run where workflow_run_id=59 and phase_id=12` 在导出未完成时为 `PENDING`/`RUNNING` | √ |
| 10726 | **流程概览 JOB 列混展示两个指标**：JOB 列显示为 `1/102 · 执行中 1`（`DeDupCDEUView.vue:1645-1647` 把 `success/total` 与 `running` 拼在同一单元格）。修复：JOB 列只显示 `成功/总数`，运行中/失败数量改为独立列。 | 流程概览表格 JOB 列只出现 `1/102` 形式；「执行中」「失败」为独立列 | √ |
| 10727 | **运行中的动态 JOB 无法查看执行日志**：`cdeu_job_dynamic.run_log_id` 在 JOB 运行期间为空（run 59 中 `dynamic_job_id=6542` 状态 `RUNNING` 但 `run_log_id IS NULL`），前端 `v-if="job.run_log_id"` 因此不渲染「执行日志」按钮；日志本身已写入（`run_id=7470` 有 13,254 行）。修复：动态 JOB 启动/运行中即回写 `run_log_id`。 | 运行中的动态 JOB 行出现「执行日志」按钮且能打开实时日志 | √ |
| 10728 | **跑马灯加在了错误页面**：动效只加在「全流程拓扑」（`DeDupCDEUWorkflowView.vue`），而「流程概览」（`DeDupCDEUView.vue`）的 Phase 卡片与 Process 区块没有跑马灯，用户侧不可见。修复：在流程概览的 Phase / Process 运行中状态补齐青色跑马灯。 | 流程概览中运行中的 Phase 卡片与 Process 区块出现青色流光动效 | √ |

## 七十七、阶段内流程串行与统计列拆分（10729–10731，本轮修复）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10729 | **同一阶段内 `affects_completion=true` 的流程必须按 `seq_no` 串行**：前序流程未终态时，后序流程不得进入 `RUNNING`/`SUCCESS`，应保持 `PENDING`；同时补写 `started_at`（当前 `EvaluateCDEUProcess`/`EvaluateCDEUPhase` 的 UPDATE 只写 `status` 与 `finished_at`，导致「执行中」的流程开始时间列为空）。 | 运行中检查 `PROCESS_EXPORT_FILE_MON_BASE` 与 `PROCESS_EXPORT_FILE_MON_INCR` 不得同时为 `RUNNING`；执行中的流程 `started_at` 非空 | √ |
| 10730 | **流程表格 JOB 统计拆列**：JOB 总数、成功数、失败数、执行中数各占独立列，不再以 `1/102 · 执行中 1` 形式混在一个单元格。 | 表格中「JOB总数」「成功」「失败」「执行中」为四列独立展示 | √ |
| 10731 | **批次统计同样拆列**：批次总数与批次失败数分开，不在同一列内以 `· 失败 N` 拼接。 | 批次列只显示总数；批次失败为独立列 | √ |
| 10733 | **阶段（Phase）级缺串行与失败传播门禁**：`PHASE_EXPORT` 为 `FAILED` 时，后续 `PHASE_UPLOAD` / `PHASE_REVIEW` 仍因自身子级全 `SUCCESS` 而被判 `SUCCESS`。上一轮只补了 Process 层门禁，Phase 层未覆盖。修复：`EvaluateCDEUPhase` 增加前序阶段检查——前序 `PENDING/READY/RUNNING` 时本阶段保持 `PENDING`，前序 `FAILED/PARTIAL/CANCELLED` 时本阶段标 `SKIPPED`。 | `PHASE_EXPORT` 失败后，`PHASE_UPLOAD`/`PHASE_REVIEW` 不得显示为成功；应分别为 `SKIPPED`/`PENDING` | √ |
| 10734 | **异常态 JOB 不得隐藏「手动执行」按钮**：`showRunButton` 对 `EMBEDDED` 步骤只在 `JOB_EXPORT_LOAD` 上放行，且 `manual_rerun_enabled=false` 的 JOB 一律无按钮，导致 JOB 失败后用户没有任何重试入口。修复：`FAILED`/`PARTIAL`/`PAUSED`/`CANCELLED` 一律显示手动执行按钮。 | 任意失败状态的 JOB 行均出现「手动执行」按钮 | √ |
| 10736 | **空转循环靠刷心跳免疫超时回收**：`loadJob` 外层 `for date <= lastDate` 在游标异常时无限空转，而每轮 `UpdateLoadProgress` 都刷新 `gmt_heartbeat` 与 `locked_until`，导致 `RECOVER_TIMEOUT_EXPORT_JOBS` 的 `locked_until < now()` 永不成立——「循环活着但什么也没干，却永远不超时」（run 59 的 `JOB_EXPORT_BASE_GCMain_201901` 卡 47 分钟即此现象）。修复：① `loadJob` 增加 `nextDate` 必须严格递增校验 + 总轮数上限 62；② `loadDate` 的 keyset 分页增加 `MaxTS` 前进校验。任一项不满足即报错退出，交给重试/超时机制。 | 构造游标异常后，JOB 在数十轮内报错结束而非无限空转；日志出现「装载游标不前进」或「装载游标异常：已循环 N 轮」 | √ |
| 10735 | **修复脚本不得把未终态记录直接改成 `CANCELLED` 而不经用户确认**：`fix_run59.sql` 将 run 59 的 phase/process/job 未终态行批量标为 `CANCELLED`，导致界面「状态」列出现「已取消」而用户不知情。修复：此类批量状态改写必须先征得用户同意；确需取消时应记录到变更日志并说明影响范围。 | 未经确认的执行脚本不出现 `UPDATE ... SET status='CANCELLED'` 批量改写 | √ |
| 10732 | **【规范项·非功能需求】** **数据修复与运行中循环的竞态**：把 job 状态直接 UPDATE 回 `WAITING` 时，容器内仍在循环的 `RUN_DUE_EXPORT_JOBS`（每 60 秒一轮）会立刻 claim 到这些 job 并继续执行，导致「修复数据的瞬间又开始跑批」。修复方向：修复脚本必须先把 job 置为 `FROZEN`/`PAUSED` 等不可 claim 状态，确认无运行中循环后再改回 `WAITING`；或在修复期间临时停止容器。 | 在执行 `UPDATE ... SET status='WAITING'` 的修复脚本期间，`RUN_DUE_EXPORT_JOBS` 不得 claim 到任何被重置的 job | 规范已定 |

## 七十八、SKIPPED 统计口径与强制终止（10737–10740，本轮实现）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10737 | **空流程初始化不得标 SUCCESS**：`seedCDEUHierarchyTx` 对被判定为「占位」的 Process（无启用 JOB）及其所在阶段，由 `SUCCESS` 改为 `SKIPPED`，避免「没跑到该阶段就显示成功」。 | 新 run 的 `PROCESS_EXPORT_DECISION` / `PHASE_COLLECT` 初始状态为 `SKIPPED` 而非 `SUCCESS` | √ |
| 10738 | **上传动态实例的脏状态不得变成僵尸**：`GenerateUploadDynamicJobs` 把 `UPLOADING` 映射为 `PENDING`（原为 `RUNNING` 且不写 `gmt_start`），并在生成后追加清理「`RUNNING` 但 `gmt_start IS NULL`」的上传实例。 | 存在 `UPLOADING` 文件时生成的实例状态为 `PENDING`；库中不存在 `RUNNING AND gmt_start IS NULL` 的上传实例 | √ |
| 10739 | **新增强制终止能力**：`POST /CDEU/Workflow/Terminate`，把指定（或最近一次运行中的）`workflow_run` 下所有 `PENDING/READY/RUNNING` 的阶段、流程、JOB、步骤置为 `CANCELLED`，释放残留任务锁；前端工具栏新增「强制终止」按钮与确认对话框；终态运行重复终止返回 409。 | 对运行中的 run 调用接口返回 200 且子级未终态记录归零；再次调用返回 409；前端按钮仅在运行中可点 | √ |
| 10740 | **`SKIPPED` 必须计入完成数**：`cdeuChildCompleted` 原实现显式排除 `SKIPPED`，导致含占位流程的阶段（如 `PHASE_DECISION` 3 成功 + 1 跳过）被误判 `FAILED`。修复：`SKIPPED` 与 `SUCCESS` 同等视为已完成。 | `PHASE_DECISION` 在 3 个 SUCCESS + `PROCESS_EXPORT_DECISION` SKIPPED 时判 `SUCCESS` 而非 `FAILED` | √ |

### 备注：两个占位流程（无 JOB）

| 阶段 | 流程 | 说明 |
|---|---|---|
| `PHASE_COLLECT` | `PROCESS_HISTORY_INGEST` 历史行情数据采集 | 采集由外部程序负责，本地不产生 JOB |
| `PHASE_DECISION` | `PROCESS_EXPORT_DECISION` 导出条件判定 | 自 `V47` 起即为占位；实际判定在 `PROCESS_EXPORT_BATCH` 的 `JOB_FETCH_EXPORT_BATCH` 中完成 |

这两者没有 JOB，因此**不会有执行日志**（日志只由 JOB 执行产生），属设计行为。

## 七十九、BASE 文件名增加 _Base 后缀（10743）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10743 | BASE 导出文件名由 `{region}_{market}_{usc}_Min_{provider}_Mon_{yyyyMM}.mvsv` 改为 `..._Mon_{yyyyMM}_Base.mvsv`，与 PATCH（`_Incr_N`）、MANIFEST（`_Manifest`）的类型后缀保持一致。 | `select file_name from dedup.finv_dedup_export_file where file_type='BASE' order by file_id desc limit 1` 以 `_Base.mvsv` 结尾；`grep -F "_Base.mvsv" internal/meta/finv/quote/dedup/naming.go` 命中 | √ |

### 10743 影响说明

| 项 | 说明 |
|---|---|
| 历史文件 | GitHub 上已有 101 个旧名 BASE（无 `_Base`），**不会被自动删除**，与新建文件并存 |
| 同名覆盖语义 | 「BASE 无版本号、重写即同名覆盖」仅对新命名生效；旧名文件不再被覆盖 |
| MANIFEST | `manifest.base.file_name` 记录的是新名，与实际文件一致 |
| 数据库登记 | 新导出/重写的 BASE 在 `finv_dedup_export_file.file_name` / `target_file_name` 中均为新名 |

## 八十、占位流程配 JOB / 终态 started_at 回填（10741–10742）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10741 | 为两个「有流程定义但 0 个 JOB」的占位流程配置真实 JOB：`PROCESS_HISTORY_INGEST` ← `JOB_CHECK_INGEST_READINESS`（采集就绪检查），`PROCESS_EXPORT_DECISION` ← `JOB_EVALUATE_EXPORT_CONDITION`（导出条件判定）。两者均为只读诊断型 JOB，产出可观测结果与执行日志。 | `select count(*) from dedup.cdeu_process p where p.enabled and not exists (select 1 from dedup.cdeu_job j where j.process_id=p.process_id and j.enabled)` = 0；单独执行两个 JOB 均返回 SUCCESS 且 `finv_dedup_job_run_log_line` 有完整日志 | √ |
| 10742 | 终态流程/阶段若从未经过 `RUNNING`（子级一次性完成），`started_at` 为空导致界面「已完成但无开始时间」。修复：`EvaluateCDEUProcess` / `EvaluateCDEUPhase` 在写入终态时用**子级最早开始时间**兜底（`LEAST(min(job_run.started_at), min(job_dynamic.gmt_start), now())`）。**⚠️ 该实现中的 `now()` 兜底于 `10747` 被移除**：子级尚未开始时 `LEAST` 忽略 NULL 后落到 `now()`，把评估时刻写成了开始时间，再被 `COALESCE` 永久锁定——这是 10747 破绽的直接来源。现口径见 10747。 | `select count(*) from dedup.cdeu_process_run where started_at > finished_at` = 0；`select count(*) from dedup.cdeu_phase_run where started_at > finished_at` = 0 | √（经 10747 修正） |

## 八十一、本地导出文件清理配置 20011（10744）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10744 | 新增系统配置 **20011**（字符型）：归档复核阶段是否清理本地导出的 `.mvsv` / `Manifest.json`。`'1'` 清理（默认）/ `'0'` 不清理。启用后由 `PHASE_REVIEW` / `PROCESS_PURGE` 的 **`JOB_PURGE_LOCAL_FILES`** 执行，排在 `JOB_PURGE_EXPORTED_MONTH` 之后（`seq_no` = 2）。删除判据：该证券月 BASE 与全部 PATCH 均 `UPLOADED`、`remote_checksum_verified` 全为 true、最后一个文件上传完成已超 **24 小时**、无 `PENDING`/`RUNNING` 补丁。命中后删除该月全部本地文件并置 `cleanup_status='LOCAL_DELETED'`。原 `LocalRetention` 降级为孤儿文件兜底。 | `select char_value from sys.sys_config where config_no=20011` = `'1'`；`select job_code, seq_no from dedup.cdeu_job j join dedup.cdeu_process p on p.process_id=j.process_id where p.process_code='PROCESS_PURGE' order by seq_no` 返回 `JOB_PURGE_EXPORTED_MONTH:1` / `JOB_PURGE_LOCAL_FILES:2`；`grep -F "PURGE_LOCAL_EXPORT_FILES" internal/meta/finv/quote/dedup/model.go` 命中 | √ |

### 10744 用户决策

| # | 决策 |
|---|---|
| 1 | 20011 默认 **`'1'`**（清理） |
| 2 | MANIFEST `.json` 与 `.mvsv` **一起删除** |
| 3 | 上传成功后**超过 24 小时**才删，执行顺序排在 `JOB_PURGE_EXPORTED_MONTH` **之后** |
| 4 | `LocalRetention` **降级为孤儿兜底**（只处理登记记录已不存在的磁盘残留） |
| 5 | **`finv_dedup_export_file` 记录永久保留，不随本地文件删除而删除** |

### 10744 记录保留策略（决策 5 的说明）

`JOB_PURGE_LOCAL_FILES` 只做两件事：`os.Remove` 删物理文件、`UPDATE cleanup_status='LOCAL_DELETED'` 打标记。**记录一行不删**，理由：

| 用途 | 说明 |
|---|---|
| 幂等 | `cleanup_status <> 'LOCAL_DELETED'` 是判据之一，删过的不再重复扫描 |
| 审计 | 文件 ID / checksum / 上传时间 / 清理时间全留痕 |
| 补丁链 | `finv_dedup_export_file_patch.patch_file_id` 指向它，删记录会断链 |
| 去重 | `RegisterFile` 靠 checksum 判断「相同内容不重复上传」 |

**已知影响**：每轮导出产生约 404 条记录（BASE 101 + PATCH 101 + MANIFEST 202），永久保留意味着表会随运行次数线性增长（单条约 300 字节，1000 轮约 120 MB）。当前无清理机制，属**已接受的长期成本**。

## 八十二、合并 finv_dedup_usc_priority 到 finv_dedup_usc（10745）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10745 | 把 `dedup.finv_dedup_usc_priority`（39 个证券里仅 3 行配置的稀疏覆盖表）合并进 `dedup.finv_dedup_usc`：新增 `priority_rank` / `sla_days` / `max_wait_days` / `priority_gmt_update` 四列，三列同时为 NULL 表示未配置（按 100/5/7 兜底），`finv_dedup_usc_priority_group_check` 保证组一致性。前端把优先级从独立标签页改为证券列表内联编辑；`DELETE /Priority` 语义从「删行」改为「清空覆盖」，`PUT /Priority` 要求证券已存在（不存在返回 404）。 | `select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='dedup' and c.relname='finv_dedup_usc_priority'` = 0；`select count(*) from dedup.finv_dedup_usc where priority_rank is not null` = 3；`select conname from pg_constraint where conname='finv_dedup_usc_priority_group_check'` 命中；`grep -rn "finv_dedup_usc_priority" --include=*.go internal/` 仅剩注释 | √ |

### 10745 用户决策

| # | 决策 |
|---|---|
| 1 | **授权** `DROP TABLE dedup.finv_dedup_usc_priority` |
| 2 | 前端**合并进 USC 列表内联编辑**，取消独立「优先级 / SLA」标签页 |
| 3 | 需求编号用 `10745` |

### 10745 行为变化

| 变化 | 原 | 新 |
|---|---|---|
| 给未登记的证券配优先级 | `INSERT ... ON CONFLICT` 可创建孤儿行 | 返回 404，必须先登记证券 |
| `DELETE /Priority` | `DELETE` 一行 | `UPDATE SET 三列 = NULL`，证券行保留 |
| `GET /Priority/List` 返回字段 | `gmt_update` | `priority_gmt_update` |
| `gmt_update` 语义 | 优先级行自身更新时间 | 拆分为 `priority_gmt_update`（优先级）与 `gmt_update`（主数据） |

## 八十三、手动模式隐藏每日调度 + 流程开始时间改为真实值（10746–10747）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10746 | CDEU 概览页的「每日调度 HH:MM 时区」chip 只在 `20005=Auto` 时展示。手动模式下调度器根本不读该时间，展示出来会让运维误以为「到点会自动跑」。工作流详情 tooltip 同步增加「定时触发」一行，Manual 时明确写「未启用（20005=Manual，仅手动触发）」。 | `20005` 为 `Manual` 时页面顶部不出现「每日调度」chip；把 `20005` 改为 `Auto` 刷新后出现 | √ |
| 10747 | 流程与阶段的 `started_at` / `finished_at` 必须由其子级的真实执行时间推导，不得写入评估时刻。原实现 `COALESCE(pr.started_at, LEAST(min(job_run.started_at), min(dyn.gmt_start), now()))` 在子级尚未开始时落到 `now()`，随后被 `COALESCE` 永久锁定，导致同一阶段内先后执行的流程开始时间挤在几十毫秒内、甚至早于其子 JOB 数秒；`finished_at` 同理（暂停后恢复的 run 里会早于子 JOB 开始，物理上不可能）。新口径：`started_at = LEAST(最早子级开始)`、`finished_at = GREATEST(最晚子级结束, started_at, 原值)`；子级无痕迹时 `started_at = NULL`（不编造）。只读重放路径也不再写 `started_at = now()`。 | `select count(*) from dedup.cdeu_process_run where started_at > finished_at` = 0；`select count(*) from dedup.cdeu_phase_run where started_at > finished_at` = 0；对任一 run，`cdeu_process_run.started_at` 应等于其子级最早 `started_at`（`diff` 全为 0），且各流程按 `seq_no` 严格递增 | √ |

### 10747 现场证据（run 62，修复前）

| 流程 | 修复前 started_at | 子 JOB 最早 started_at | 偏差 |
|---|---|---|---|
| PROCESS_RECOVER | 19:44:24.766 | 19:44:34.459 | 早 9.7 秒 |
| PROCESS_UPLOAD | 19:44:24.759 | 19:44:34.336 | 早 9.6 秒 |
| PROCESS_EXPORT_BATCH | 19:44:24.803 | 19:44:30.062 | 早 5.3 秒 |
| PROCESS_INGEST_STATS | 19:44:24.739 | 19:44:24.977 | 早 238ms |
| PROCESS_SEAL | 19:44:24.745 | 19:44:24.851 | 早 106ms |

### 10747 修复后（run 62）

```
19:44:24.670  PROCESS_HISTORY_INGEST      ← PHASE_COLLECT
19:44:24.851  PROCESS_SEAL                ← PHASE_DEDUP
19:44:24.977  PROCESS_INGEST_STATS        ← PHASE_DECISION
19:44:26.387  PROCESS_MATURITY
19:44:29.664  PROCESS_BUDGET
19:44:29.875  PROCESS_EXPORT_DECISION
19:44:30.062  PROCESS_EXPORT_BATCH        ← PHASE_EXPORT
19:44:30.267  PROCESS_EXPORT_FILE_MON_BASE
19:44:30.628  PROCESS_EXPORT_FILE_MON_INCR
19:44:34.336  PROCESS_UPLOAD              ← PHASE_UPLOAD
19:44:34.459  PROCESS_RECOVER             ← PHASE_REVIEW
19:44:34.566  PROCESS_REVIEW
19:44:35.101  PROCESS_PURGE
19:44:35.171  PROCESS_STORAGE_CLEAN
19:44:35.296  PROCESS_RETENTION
```

严格单调递增，每个流程 `started_at` 与其子级最早开始时间完全相等。

### 10747 数据影响

| 项 | 数值 |
|---|---|
| `cdeu_process_run` 总数 | 354 |
| 重算后无开始时间 | 89（子级无执行痕迹，属正常） |
| `started_at > finished_at` 残留 | 0 |
| `cdeu_phase_run` 总数 | 150 |
| 重算后无开始时间 | 46 |
| `started_at > finished_at` 残留 | 0 |

## 八十四、阶段/流程进度百分比文本（10748）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10748 | 在 CDEU 主页面与全流程拓扑页的**全部**进度条右侧展示完成百分比文本，数值直接取自驱动进度条宽度的同一个 `phaseProgress()` / `processProgress()` 返回值（同源，不二次计算）。覆盖 8 个渲染点：CDEU 主页面 4 处（阶段拓扑卡片、Process 拓扑节点、阶段概览卡片、Process 块头部）、全流程拓扑 4 处（横版阶段、横版 Process 节点、竖版阶段纵向进度条、竖版 Process 节点）。 | 页面中每个阶段/流程进度条右侧均出现 `NN%` 文本；`grep -c "fmtPct(" Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` ≥ 4、`DeDupCDEUWorkflowView.vue` ≥ 4；未开始显示 `0%`、完成显示 `100%` | √ |

### 10748 实现说明

| 位置 | 改造方式 |
|---|---|
| CDEU 主页面 · 阶段拓扑卡片 | `v-progress-linear` 外包 `.workflow-node-progress` flex 容器，右侧 `.workflow-node-percent` |
| CDEU 主页面 · Process 拓扑节点 | 同上 |
| CDEU 主页面 · 阶段概览卡片 | 容器 `min-width` 120 → **168px**，进度条 `flex-grow-1`，右侧 `.progress-percent` |
| CDEU 主页面 · Process 块头部 | 容器 `width` 80 → **128px**，同上 |
| 全流程拓扑 · 横版阶段 | `.fulltopo-phase-progress` 由纯 `span` 高度条改为 flex（`.fulltopo-phase-progress-track` + `.fulltopo-phase-percent`），`top` 50 → **53px**，高度 2 → **12px**；定位避开阶段 header（52px）与首个节点（68px） |
| 全流程拓扑 · 横版 Process 节点 | `.fulltopo-node-progress` 改为 flex（track + percent），高度 2 → **12px**；节点高 80px，内容合计约 63px，`margin-top:auto` 仍贴底不溢出 |
| 全流程拓扑 · 竖版阶段 | 纵向进度条 `.fulltopo-vphase-divider` 不变，右侧新增绝对定位 `.fulltopo-vphase-percent`（`left = V_PHASE_TITLE_WIDTH + 6 = 162px`、`top: 6px`），落在标题区外、首批节点（`y = 16px` 起、`x = 172px` 起）之上的空白区 |
| 全流程拓扑 · 竖版 Process 节点 | 与横版节点同结构 |

数值渲染统一走 `fmtPct(value) = \`${Math.round(value)}%\``，`font-variant-numeric: tabular-nums` 保证位数变化时不抖动。

## 八十五、字典与市场初始数据补齐（10749–10750）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10749 | 补齐 `sys.sys_dict` 中 `dict_no=3120` 的主表记录。`V200027` 早已写入 3120 的 14 条子项（`0`–`9`、`A`–`D` → CNY/USD/HKD/EUR/GBP/AUD/CAD/SGD/JPY/MOP/DKK/GBX/MYR/TWD），但 `V200026` 的 19 条主表记录中没有 3120，形成「有子项、无主表」的孤儿字典。本迁移只补定义行、不动子项，并新增**全库孤儿字典为 0** 的收敛校验（后续再出现同类问题迁移会直接失败）。 | `select count(*) from sys.sys_dict where dict_no=3120` = 1；`select count(*) from sys.sys_dict_sub where dict_no=3120` = 14（不变）；`select count(distinct s.dict_no) from sys.sys_dict_sub s where not exists (select 1 from sys.sys_dict d where d.dict_no=s.dict_no)` = 0；`select version, success from public.schema_version where version='100053'` 为 `t` | √ |
| 10750 | 在 `public.finv_market` 新增美股 OTC Pink 市场：`market_code=1332` / `market_flag='PINK'`。PINK 指 OTC Markets Group 的 Pink 层级，与泛化的 `1331 US-OTC` 不同口径，需独立登记以便 metadata 刷新与导出目录按市场区分。 | `select market_code, market_flag, market_symbol, flag_enable from public.finv_market where market_code=1332` 返回 `1332 / PINK / PINK / 1`；`select count(*) from public.finv_market where market_flag='PINK'` = 1；`select version, success from public.schema_version where version='100054'` 为 `t` | √ |

### 10749 取值说明

| 字段 | 值 | 依据 |
|---|---|---|
| `dict_name` | `币种` | 子项是 14 种货币中英对照，且 3121「外汇交易币种」、3122「注册资本币种」均为其衍生字典 |
| `key_type` / `val_type` | `1` / `2` | 与 3103/3121/3122 等「1 键对 2 值」码表一致 |
| `access_level` / `manage_level` / `data_level` | `1` / `0` / `2` | 与 3121、3122 完全一致 |
| `dict_category` | `2` | 与 3101/3103/3110/3121/3122 同属码表类 |
| `gmt_create` / `gmt_modify` | `2024-12-19 15:28:10.900191` | 取 3120 子项自身写入时间，表示主表与子项同批导入 |
| `en_sub_sys` | `''`（空） | 3121=`act`、3122=`cic` 均为单一子系统归属；3120 子项 `en_sys_str` 为 `gat,gax,gcq,gaa,tms,pbs` 六子系统共用，无单一归属，故留空而非随意挑选 |

> `dict_name` 与 `en_sub_sys` 属推断值，已在迁移注释中标注依据。迁移用 `ON CONFLICT (dict_no) DO NOTHING`，运维可直接 `UPDATE` 修正且重跑不覆盖。

### 10750 取值说明

| 字段 | 值 | 依据 |
|---|---|---|
| `market_code` | `1332` | 紧随 1330→1331 的 US 场外编号段 |
| `market_flag` / `market_symbol` | `PINK` | 用户指定标识；`market_symbol` 与 `market_flag` 同标识便于追溯（`dedup.finv_dedup_usc.market` 取该列） |
| `market_abbr` | `US` | 与 1310/1315/1330/1331 等全部美股市场一致 |
| `market_name` | `美股 OTC Pink 场外交易` | 参照 1331「美股 OTC 场外交易」的命名口径 |
| `en_security_type` | `1313` | 与 1330/1331 同属美股场外/存托凭证类型 |
| `base_currency` | `USD` | 美股统一美元计价 |
| `flag_enable` | `1` | 1330/1331 均启用；不启用则 metadata 刷选不会取到 |
| `exchange_code` | `0` | 与 1330/1331 一致（无独立交易所编号） |

## 八十六、迁移脚本编号统一 +100000（10751）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10751 | 把 `Deploy/Migrations/` 下全部 131 个迁移脚本的 `V` 后编号统一加 100000（如 `V200029__sys_dict_sub_seed_common.sql` → `V300029__sys_dict_sub_seed_common.sql`），并保证**执行顺序与修改前完全一致**。编号分段随之调整为：结构/变更迁移 `V200001`～`V299999`（原 `V1`～`V76`，共 76 个），种子/数据迁移 `V300000+`（原 `V200000`～`V200054`，共 55 个）。同时同步 `public.schema_version` 已应用记录、代码注释与文档中全部迁移编号引用。 | `ls Deploy/Migrations/ \| grep -cE '^V[0-9]+__'` = 131；编号区间 `100001`～`200054` 严格递增且无重号；`select count(*), min(version::int), max(version::int), count(*) filter (where version::int < 100001) from public.schema_version` 返回 `131 / 100001 / 200054 / 0`；容器重启后启动日志**无任何迁移执行记录**（框架正确识别全部已应用） | √ |

### 10751 执行步骤与风险控制

| 步骤 | 操作 | 说明 |
|---|---|---|
| 1 | 停 `finvquant` 容器 | **必须**：旧二进制内嵌旧文件名，若先改名再重启，框架会把 131 个迁移全判为「未执行」并整体重跑 |
| 2 | 建立旧号→新号映射并校验单调性 | 断言 `新编号 - 100000 == 原始编号序列`，确保顺序不变 |
| 3 | 两阶段重命名（旧名 → `TMP__` 前缀 → 最终名） | 避开 `V42→V200042` 与尚未改名的 `V200042` 撞名 |
| 4 | 重建 `schema_version` 版本号 | 单条 `UPDATE +100000` 会撞主键（`1→100001` 与 `100001→200001` 冲突），改用「临时表备份 → `DELETE` → `INSERT (old+100000)`」，全程单事务 |
| 5 | 更新引用 | 文件名引用 `V<n>__` 250 处、裸版本引用 57 处、编号分段约定 6 处 |
| 6 | 构建部署并验证 | 启动日志无迁移执行；`schema_version` 131 条无遗留 |

### 10751 编号映射

| 段位 | 原编号 | 新编号 | 数量 |
|---|---|---|---|
| 结构/变更 | `V1` – `V76` | `V200001` – `V200076` | 76 |
| 种子/数据 | `V200000` – `V200054` | `V300000` – `V300054` | 55 |

### 10751 事故记录（执行中发生并已恢复）

第 3 步首次尝试用 `git mv` 时，因目标名被尚未改名的旧文件占用而在第 26 个文件失败；前 25 个已成功改成 `TMP__` 前缀。随后的脚本为「清理残留临时文件」对这些 `TMP__` 文件执行了 `os.remove`，误删 25 个已跟踪文件。

- **发现**：文件总数从 131 掉到 106
- **恢复**：这些文件状态为 `RD`（索引记录重命名、工作区已删除），用 `git restore --source=HEAD --staged --worktree` 精确恢复到旧名，随后重新参与两阶段改名
- **教训**：批量重命名脚本不得包含「无差别删除临时前缀文件」的逻辑；临时状态必须显式记录并只清理本次运行产生的文件

## 八十七、日志前缀 [exportarchive] 统一改为 [cdeu]（10752）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10752 | 把运行日志中的前缀 `[exportarchive]` 统一替换为 `[cdeu]`，覆盖 25 个文件共 97 处。理由：该日志域早已从「导出归档」演进为 CDEU 工作流（Workflow → Phase → Process → JOB → Step），前缀与模块语义不符。 | `grep -rn '\[exportarchive\]' --include=*.go .` 命中 0 处；`grep -rc '\[cdeu\]' --include=*.go .` 合计 97 处；容器重启后启动日志全部为 `[cdeu] …` 前缀，旧前缀条数 0 | √ |

### 10752 替换范围

| 文件（相对 `internal/meta/finv/quote/dedup/`） | 处数 |
|---|---|
| `job_export.go` | 19 |
| `cdeu.go` | 12 |
| `job_patch.go` | 9 |
| `job_upload.go` | 8 |
| `scheduler.go` | 7 |
| `service.go` | 5 |
| `job/purge_local_export_files.go` | 5 |
| `job/purge_exported_month.go` | 4 |
| `job/upload_pending_export_files.go` / `job/scan_post_seal_patches.go` / `job/check_export_job_health.go` | 各 3 |
| 其余 14 个文件 | 各 1–2 |

> 仅替换日志前缀字面量，不改任何变量名、结构体字段或接口路径。

## 八十八、新增 cdeu schema 域占位（10753）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10753 | 新增 `cdeu` schema 作为 **CDEU 工作流域的域占位**，并把 `cdeu` 纳入数据库级与应用连接串的 `search_path`（10 段：`public, quote, quant, futu, export, sche, dedup, cdeu, archive, sys`，紧随 `dedup` 之后、`archive` 之前）。本次**只建空域，表不搬迁**——12 张 `cdeu_*` 表仍在 `dedup`。 | `select nspname, pg_get_userbyid(nspowner) from pg_namespace where nspname='cdeu'` 返回 `cdeu / finv`；`select has_schema_privilege('finv','cdeu','USAGE'), has_schema_privilege('finv','cdeu','CREATE')` 均为 `t`；`select setconfig from pg_db_role_setting s join pg_database d on d.oid=s.setdatabase where d.datname='finv' and s.setrole=0` 含 `cdeu`；`show search_path` 为 10 段；`select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='cdeu'` = 0；`select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='dedup' and c.relname like 'cdeu%' and c.relkind='r'` = 12；`select version, success from public.schema_version where version='100077'` 为 `t` | √ |

### 10753 约束条款（用户 2026-10-28 明确要求）

> **后续把 `cdeu_*` 表从 `dedup` 迁入 `cdeu` 时，不是简单的同名迁移。**

| 禁止 | 要求 |
|---|---|
| `ALTER TABLE dedup.cdeu_xxx SET SCHEMA cdeu` 式的**同名机械搬移** | 必须重新审视并可能重构：表名去掉重复的 `cdeu_` 前缀、约束/索引命名、主键与外键策略、运行表与定义表的拆分粒度、历史数据兼容 |
| 把 `finv_dedup_export_*` 一并搬入 | 那批表属**去重/导出域**（导出产物元数据），不属于工作流域 |
| 无授权直接搬 | 属 `DbSchemaSpec §0` 红线范围，须本清单有条目或用户明确点名 |

**域边界**：`cdeu` = CDEU 工作流的定义与运行实例；`dedup` = 行情批内去重、跨源指纹、入库审计、导出/上传元数据、证券主数据。

约束已写入三处落地位置：`V200077` 迁移头注释、`COMMENT ON SCHEMA cdeu`、`Docs/DevSpec/DbSchemaSpec.md §1.1`、`schema_qualify_test.go` 的 `cdeuTables` 注释。

### 10753 实施记录

| 项 | 内容 |
|---|---|
| 迁移 | `V200077__create_cdeu_schema.sql`：幂等建 schema（属主 `finv`）→ 数据库级 `search_path` 纳入 `cdeu` → 三道收敛校验（属主、USAGE/CREATE、`search_path` 含 `cdeu`） |
| 代码 | `postgres.go` DSN `search_path` 加 `cdeu` + 域名归属注释；3 个集成测试的 `RuntimeParams["search_path"]` 同步 |
| 测试 | `schema_qualify_test.go` 新增 `cdeuTables` 清单（12 表名）与过渡期说明，不强制 `cdeu.` 前缀 |
| 文档 | `DbSchemaSpec.md` schema 表 9 → 10 行 + 新增 §1.1 搬迁约束 |
| 踩坑 | 首版用 `pg_database.setconfig` 校验，PG 18 已无该列（`pg_database` 仅 18 列到 `datacl`），数据库级配置在 `pg_db_role_setting.setconfig`（`setrole=0`）——修正后通过 |
| 顺带 | `Docs/DevSpec/DbSchemaSpec.md`（47 处）与 `BacktestStrategySpec.md`（9 处）的裸迁移编号引用一并 +100000 |

## 八十九、在 cdeu schema 建立与 dedup 对应的表结构（10754）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10754 | 参照 `dedup` schema 的表，在 `cdeu` schema 建立对应的表结构，命名规则：① `dedup_` 开头的表 → 前缀替换为 `finv_quote_cdeu_`；② `finv_dedup_` 开头的表 → 前缀替换为 `finv_quote_cdeu_`；③ 其他表名保持不变；④ `prep_bak` 开头的表不在 cdeu 中创建。**本次只建表结构，不发放任何初始数据**。 | `select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='cdeu' and c.relkind='r'` = 26；`select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='cdeu' and c.relname like 'prep_bak%'` = 0；`select sum(n_live_tup) from pg_stat_user_tables s where s.schemaname='cdeu'` = 0；`select version, success from public.schema_version where version='100078'` 为 `t` | √ |

### 10754 表名映射（26 张）

| 类别 | 原表（dedup） | 新表（cdeu） | 数量 |
|---|---|---|---|
| 前缀替换 | `cdeu_workflow` / `cdeu_phase` / `cdeu_process` / `cdeu_job` / `cdeu_job_step` / `cdeu_job_dynamic` / `cdeu_job_step_dynamic` / `cdeu_workflow_run` / `cdeu_phase_run` / `cdeu_process_run` / `cdeu_job_run` / `cdeu_job_step_run` | `finv_quote_cdeu_*`（**10755 修正**：初版误按原名保留） | 12 |
| 前缀替换 | `finv_dedup_daily_budget`、`finv_dedup_export_batch`、`finv_dedup_export_file`、`finv_dedup_export_file_patch`、`finv_dedup_export_file_upload_log`、`finv_dedup_export_job`、`finv_dedup_ingest_stats`、`finv_dedup_job_run_log_export_file`、`finv_dedup_job_run_log_line`、`finv_dedup_quote_maturity_mon`、`finv_dedup_scan_watermark`、`finv_dedup_trade_calendar`、`finv_dedup_usc` | `finv_quote_cdeu_*` | 13 |
| 表名不变 | `finv_quote_secu_kline_min` | 同名 | 1 |
| **排除** | `prep_bak_export_batch_20261027`、`prep_bak_export_file_20261027`、`prep_bak_export_job_20261027`、`prep_bak_export_patch_20261027`、`prep_bak_export_upload_log_20261027` | — | 5 |

### 10754 派生标识符规则

| 原命名 | 新命名 | 说明 |
|---|---|---|
| `finv_dedup_*`（约束/主键/唯一键/序列） | `finv_quote_cdeu_*` | 与表名保持一致 |
| `idx_dedup*` | `idx_quotecdeu*` | 按 `V200041` 的 `idx_<表名去finv_与下划线>_<用途>` 规范 |
| `uq_dedup*` | `uq_quotecdeu*` | 同上 |
| `cdeu_*` 系列 | 不变 | 表名未变 |

**唯一例外**：`finv_quote_cdeu_export_batch_batch_no_usc_biz_month_export_type_key` 加前缀后 67 字符，超 PostgreSQL 63 字符标识符上限（会被静默截断），改用短名 **`uq_finv_quote_cdeu_export_batch`**（33 字符），语义不变。

### 10754 与 dedup 的结构一致性核验

用 `information_schema.columns` 对 26 张表逐列比对（列名 / 类型 / 长度 / 可空性 / 默认值 / 精度）：

| 项 | 结果 |
|---|---|
| 表数量 | dedup 映射后 26 = cdeu 26 |
| 仅 dedup 有 / 仅 cdeu 有 | 无 |
| 列结构不一致的表 | 17 张，**全部为同一类差异**：`column_default` 里序列的 schema 限定方式 |

```sql
-- dedup（依赖 search_path 解析）
nextval('cdeu_step_step_id_seq'::regclass)
-- cdeu（显式限定，更安全）
nextval('cdeu.cdeu_step_step_id_seq'::regclass)
```

两者解析结果相同（都指向 cdeu schema 的序列），cdeu 侧显式限定符合仓库「所有 SQL 显式 schema 限定」规范，属**有意改进**而非缺陷。

### 10754 影响与注意事项

| 项 | 说明 |
|---|---|
| `finv_quote_secu_kline_min` | 现在有 **4 份同名副本**（`quote` / `dedup` / `export` / `cdeu`），`schema_qualify_test.go` 的 `exactTableSchema` 白名单已加入 `cdeu.` |
| 数据 | 全部为空表（`n_live_tup` 合计 0），本次不发放初始数据 |
| 代码读写 | **未切换**——应用仍读写 `dedup.*`，cdeu 侧仅为结构副本，等后续工作流改造接管 |
| 搬迁约束 | 仍适用 10753 的约束：切换时不是简单的 `SET SCHEMA`，须重新审视表名（去掉重复 `cdeu_` 前缀）、约束/索引命名、主外键策略等 |
| 序列 | 17 个，`nextval` 显式限定 `cdeu.` |

## 九十、修正 cdeu 表命名：cdeu_ 前缀也须替换（10755）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10755 | 修正 10754 的命名遗漏：`cdeu_` 开头的表**同样**要把 `cdeu_` 替换为 `finv_quote_cdeu_`。10754 初版误将 12 张 `cdeu_*` 按原名保留，本迁移对其及派生标识符（序列/约束/索引）执行 RENAME 矫正，**不 DROP、不重建**，表结构与数据不动。 | `select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='cdeu' and c.relkind in ('r','S','i') and c.relname like 'cdeu\_%'` = 0；`select count(*) from pg_constraint co join pg_namespace n on n.oid=co.connamespace where n.nspname='cdeu' and co.conname like 'cdeu\_%'` = 0；`select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='cdeu' and c.relkind='r'` = 26；`to_regclass('cdeu.finv_quote_cdeu_job_step_dynamic')` 非 NULL；`select version, success from public.schema_version where version='100079'` 为 `t` | √ |

### 10755 最终命名规则（修正后）

| 原前缀 | 新前缀 | 数量 |
|---|---|---|
| `cdeu_` | `finv_quote_cdeu_` | 12 |
| `finv_dedup_` | `finv_quote_cdeu_` | 13 |
| 其他（`finv_quote_secu_kline_min`） | 不变 | 1 |
| `prep_bak_` | **不创建** | （排除 5） |

### 10755 派生标识符映射（RENAME）

| 类型 | 原名 | 新名 |
|---|---|---|
| 表 | `cdeu_job_step_dynamic` | `finv_quote_cdeu_job_step_dynamic` |
| 序列 | `cdeu_job_step_dynamic_dynamic_step_id_seq` | `finv_quote_cdeu_job_step_dynamic_dynamic_step_id_seq` |
| 约束（PK/UNIQUE/CHECK/NOT NULL） | `cdeu_job_step_dynamic_pkey` | `finv_quote_cdeu_job_step_dynamic_pkey` |
| 独立索引 | `idx_cdeu_job_step_dynamic_job` | `idx_finv_quote_cdeu_job_step_dynamic_job` |

统一 +11 字符（`finv_quote_`）；改名后最长标识符 **61 字符**（`cdeu_job_step_dynamic_dynamic_job_id_step_code_key` 50 → 61），未超 PG 的 63 上限。

### 10755 矫正方式与核验

| 项 | 内容 |
|---|---|
| 迁移 | `V200079__rename_cdeu_tables_with_prefix.sql`：5 个 DO 块按系统表动态生成 RENAME（表 → 序列 → 约束 → 索引），逐类计数并输出 NOTICE |
| 为何不 DROP 重建 | 遵循「不修改已应用迁移」规范：`V200078` 已 `success=t`，保留其历史（记录了初版命名），由 `V200079` 矫正；且 RENAME 不动结构与数据，比 DROP+CREATE 更安全 |
| 不变项核验 | 表 26 / 序列 17 / 索引 80 / 约束（CHECK 26 + NOT NULL 249 + PK 26 + UNIQUE 18）全部与矫正前一致；数据行合计仍为 0；`dedup` 侧 31 张表未受影响 |
| 残留核验 | `cdeu_` 前缀的表/序列/索引/约束、`idx_cdeu_` 前缀的索引，全部为 0 |
| 测试同步 | `schema_qualify_test.go` 的 `cdeuTables` 清单改为 25 张 `finv_quote_cdeu_*` + 1 张 `finv_quote_secu_kline_min`，并标注两组来源 |

## 九十一、把 dedup 表数据迁入 cdeu 对应表（10756）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10756 | 把 `dedup` schema 中 26 张表的数据复制到 `cdeu` schema 对应表，为后续逐步清理淘汰 `dedup` 表做准备。映射：`dedup.cdeu_*` / `dedup.finv_dedup_*` → `cdeu.finv_quote_cdeu_*`；`dedup.finv_quote_secu_kline_min` → `cdeu.finv_quote_secu_kline_min`（表名不变）；`prep_bak_*` 5 张不迁。**只复制、不删源**，`dedup` 侧数据保留。 | 对 26 张表逐表执行 `select count(*) from dedup.<src>` 与 `select count(*) from cdeu.<dst>` 比对，全部相等（合计 133,260 行）；17 个序列 `last_value` 与对应表 `max(主键)` 相等；`select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='dedup' and c.relkind='r'` 仍为 31（源表未删）；`select version, success from public.schema_version where version='100080'` 为 `t` | √ |

### 10756 迁移规模（逐表行数）

| 表（cdeu 侧名） | 行数 |
|---|---|
| `finv_quote_cdeu_job_run_log_line` | 115,825 |
| `finv_quote_cdeu_job_run_log_export_file` | 8,059 |
| `finv_quote_cdeu_job_step_dynamic` | 2,940 |
| `finv_quote_cdeu_quote_maturity_mon` | 2,492 |
| `finv_quote_secu_kline_min` | 1,000 |
| `finv_quote_cdeu_job_dynamic` | 876 |
| `finv_quote_cdeu_job_run` | 383 |
| `finv_quote_cdeu_job_step_run` | 359 |
| `finv_quote_cdeu_process_run` | 354 |
| `finv_quote_cdeu_export_file` / `_export_file_upload_log` | 各 202 |
| `finv_quote_cdeu_phase_run` | 150 |
| `finv_quote_cdeu_export_batch` / `_export_job` / `_scan_watermark` | 各 101 |
| `finv_quote_cdeu_usc` | 39 |
| `finv_quote_cdeu_job` | 22 |
| `finv_quote_cdeu_process` | 17 |
| `finv_quote_cdeu_job_step` | 15 |
| `finv_quote_cdeu_workflow_run` | 11 |
| `finv_quote_cdeu_phase` | 6 |
| `finv_quote_cdeu_daily_budget` | 3 |
| `finv_quote_cdeu_workflow` / `_ingest_stats` | 各 1 |
| `finv_quote_cdeu_export_file_patch` / `_trade_calendar` | 0 |
| **合计** | **133,260** |

### 10756 实施要点

| 项 | 内容 |
|---|---|
| 迁移 | `V200080__copy_dedup_data_to_cdeu.sql`：前置校验（源 26 / 目标 26）→ 逐表 `INSERT ... ON CONFLICT DO NOTHING` → 17 个序列 `setval` → 逐表行数收敛校验 |
| 列映射 | 列清单从 `information_schema.columns` 按 `ordinal_position` 动态获取，不依赖 `SELECT *` 的列顺序假设 |
| 幂等 | 全部 `ON CONFLICT DO NOTHING`，重复执行不产生重复行 |
| 序列衔接 | 按 `pg_depend` 反查「序列 → 表.列」的真实依赖关系后 `setval`；空表回初始态（`is_called=false`） |
| 源表保留 | `dedup` 31 张表（含 5 张 `prep_bak_*`）数据一行未删 |

### 10756 已知遗留（继承自 dedup，非本次引入）

三个序列名与其实际服务的列错位，是 `V200050` 重构 CDEU 模型时的遗留，本次按实际依赖 `setval` 故值正确：

| 序列 | 实际服务 |
|---|---|
| `finv_quote_cdeu_process_process_id_seq` | `finv_quote_cdeu_workflow.workflow_id` |
| `finv_quote_cdeu_process_run_run_id_seq` | `finv_quote_cdeu_workflow_run.run_id` |
| `finv_quote_cdeu_step_step_id_seq` | `finv_quote_cdeu_process.process_id` |

### 10756 ⚠️ 关键状态说明

**应用代码仍读写 `dedup.*`。** 本迁移只完成一次性数据复制，因此：

- `cdeu` 侧数据从迁移那一刻起就是**快照**，会随 `dedup` 继续变化而逐渐过期；
- 真正切换需要改造代码（`store.go` / `cdeu_store.go` 等 200 处 `dedup.cdeu_*` 引用）并处理过渡期的双写或补差；
- 在切换完成前，**不得删除 `dedup` 表**，否则应用立即不可用。

## 九十二、代码读写切换到 cdeu schema（10757）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10757 | 把 `internal/` 与 `cmd/` 中所有使用 `dedup` schema 的代码迁移到 `cdeu` 对应表，确保切换到新表后查询/新增/修改/删除行为与切换前一致。映射：`dedup.cdeu_*` → `cdeu.finv_quote_cdeu_*`、`dedup.finv_dedup_*` → `cdeu.finv_quote_cdeu_*`、`dedup.finv_quote_secu_kline_min` → `cdeu.finv_quote_secu_kline_min`。 | `grep -rnE 'dedup\.[a-z][a-z0-9_]*' --include=*.go internal/ cmd/` 仅剩 4 处非表引用（1 条历史注释、1 条待淘汰说明、2 处独立测试临时表 `dedup.finv_quote_secu_kline_min_it`）；`grego -c 'cdeu\.' --include=*.go internal/ cmd/` 覆盖 19 个文件；`go build` / `go vet` / `go test ./...` 全绿；部署后 `CDEU/Overview`、`Status`、`Usc/Page`、`Job/List`、`File/List`、`RunLog/Page`、`Maturity/Page`、`Priority/List` 全部正常返回 | √ |

### 10757 替换规模

| 项 | 数量 |
|---|---|
| 精确替换处数 | **290** |
| 涉及文件 | **19** |
| 主要文件 | `cdeu_store.go`(98)、`migrate_integration_test.go`(46)、`service_integration_test.go`(39)、`store.go`(39)、`cdeu_admin.go`(17)、`store_integration_test.go`(16)、`model_admin.go`(11) |

### 10757 替换规则与边界处理

| 情形 | 处理 |
|---|---|
| `dedup.cdeu_X` / `dedup.finv_dedup_X` | 精确替换为 `cdeu.finv_quote_cdeu_X`，按表名长度降序 + 右边界 `(?![a-z0-9_])`，避免 `cdeu_workflow` 误伤 `cdeu_workflow_run` |
| Go 包引用 `dedup.Service` 等 | **不动**（首字母大写，正则不匹配） |
| 测试临时表 `dedup.finv_quote_secu_kline_min_it` | **不动**（独立临时表，非业务表） |
| 已删表注释 `dedup.finv_dedup_runtime_setting` | **不动**（历史说明） |
| 注释通配 `dedup.cdeu_*` / `dedup.finv_dedup_*` | 改为 `cdeu.finv_quote_cdeu_*` |

### 10757 关键常量变更

```go
// internal/meta/finv/quote/dedup/store.go
tableUsc  = "cdeu.finv_quote_cdeu_usc"
tableJob  = "cdeu.finv_quote_cdeu_export_job"
tableFile = "cdeu.finv_quote_cdeu_export_file"

// internal/meta/finv/quote/cdeu/kline/sql.go（K 线 API 写入/查询目标）
const Table = "cdeu.finv_quote_secu_kline_min"
```

### 10757 防回归闸门更新

`internal/database/schema_qualify_test.go`：

| 改动 | 说明 |
|---|---|
| `domainTablePrefixes` 新增 `"finv_quote_cdeu_": "cdeu."` | 新代码漏写前缀时会被 `TestSQLSchemaQualified` 拦下 |
| `unqualifiedDomainTables` 增加「最长前缀优先」 | 避免 `finv_quote_cdeu_*` 同时被 `finv_quote_` 报一次，产生「必须写成 quote.」的矛盾建议 |
| `exactTableSchema["finv_quote_secu_kline_min"]` 注释重写 | 四份副本职责重新表述：`quote.` 回测 / `cdeu.` 业务（代码读写）/ `export.` 中转 / `dedup.` 历史待清理 |

`internal/meta/finv/quote/cdeu/kline/sql_test.go`：`TestTableConstantQualified` 断言由 `dedup.` 改为 `cdeu.`。

### 10757 切换后的状态

| 项 | 状态 |
|---|---|
| 代码读写 | **全部指向 `cdeu.*`** |
| `dedup.*` 表 | 数据保留但**不再被读写**（只读历史） |
| 数据一致性 | 切换时点两边一致（`V200080` 迁入 133,260 行）；之后 `cdeu` 增长、`dedup` 停滞 |
| 删表前置条件 | 观察一个完整流程周期、确认无回滚需求、`dedup` 无外部只读依赖 |
| 回滚方式 | 重新部署切换前的镜像即可（`dedup` 数据仍在，只丢切换后新增到 `cdeu` 的行） |

## 九十三、前端文案与镜像构建修正（10758）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10758 | ① 前端 6 个视图共 18 处数据来源文案由 `dedup.<表>` 改为 `cdeu.<表>`，避免界面显示与真实数据源不符；② 修正 Dockerfile：`COPY . .` 会把工作区残留的旧前端产物带进镜像，随后的 `COPY --from=web-builder` 只叠加不清理，导致不同 hash 的 `index-*.js` 一同被 `go:embed` 打包（旧包内仍含已修复文案）。改为 COPY 前先 `RUN rm -rf /app/internal/webui/dist`。 | `grep -rnE 'dedup\.[a-z][a-z0-9_]*' --include=*.vue --include=*.ts Web/src/` = 0；容器内 `strings /usr/local/bin/finvquant \| grep -oE 'dedup\.(finv_[a-z0-9_]*\|cdeu_[a-z0-9_]*)'` = 0 行 | √ |

### 10758 前端替换明细

| 文件 | 处数 |
|---|---|
| `DeDupArchiveChainView.vue` | 9 |
| `DeDupKLineQueryView.vue` | 3 |
| `DeDupExportFileView.vue` / `DeDupExportJobView.vue` | 各 2 |
| `DeDupKLineStatsView.vue` / `DeDupExportOverviewView.vue` | 各 1 |

### 10758 为何 L1 grep 查不出

`grep` 只扫 `.go` 源码，漏掉两类：

| 漏点 | 发现方式 |
|---|---|
| 前端 `.vue` 文案 | 扩展 grep 到 `Web/src/` 才看到 |
| 镜像里叠加的旧 dist | 只有 `strings` 扫编译产物才暴露 —— 源码里早已没有该字符串 |

**结论**：静态验证必须覆盖「源码 grep → 编译产物 strings」两层，否则前端文案与镜像污染都会被漏掉。

## 九十四、dedup schema 重命名与完整流程验证（10759）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10759 | 把旧 `dedup` schema 重命名为 `dedup_frozen`（保留全部数据、可秒回滚），并同步 `search_path` 移除 `dedup`；在此「旧 schema 完全不可用」的严格条件下跑一轮完整 CDEU 流程，验证代码切换无遗漏。 | `select nspname from pg_namespace where nspname like 'dedup%'` 仅返回 `dedup_frozen`；`select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='dedup_frozen' and c.relkind='r'` = 31；16 个只读接口全部 `code:0`、0 失败；完整流程 `run_id=65` 终态 `SUCCESS`、6 个阶段全 `SUCCESS`；`dedup_frozen.cdeu_*` 行数在流程前后**完全不变**，所有增量落在 `cdeu.*`；容器日志 `does not exist`/`ERROR` 计数 = 0 | √ |

### 10759 重命名操作

```sql
BEGIN;
ALTER SCHEMA dedup RENAME TO dedup_frozen;
ALTER DATABASE finv SET search_path TO public, quote, quant, futu, export, sche, cdeu, archive, sys;
COMMIT;
```

| 项 | 说明 |
|---|---|
| 前置 | **必须先停容器**——重命名期间若有连接持有旧 schema，DDL 会阻塞 |
| 保留 | 31 张表（含 5 张 `prep_bak_*` 备份）与全部数据原样保留 |
| 回滚 | `ALTER SCHEMA dedup_frozen RENAME TO dedup;` 一条命令 |
| search_path | 移除 `dedup`，避免指向不存在的 schema（PG 虽静默忽略，但保留会误导） |

### 10759 fail-fast 验证结果

**只读接口（16 个，0 失败）**：`Status`、`CDEU/Overview`、`Job/List`、`File/List`、`Usc/Page`、`Priority/List`、`Calendar/Page`、`Maturity/Page`、`Stats/Page`、`Budget/Page`、`Watermark/List`、`RunLog/Page`、`UploadLog/Page`、`Patch/Page`、`Job/Runtime`、`Settings`

> `dedup` 已不存在，任一残留引用都会立刻返回 500 并报 `relation "dedup.xxx" does not exist`。全部通过 = 只读路径零遗漏。

### 10759 完整流程验证（run 65）

| 项 | 结果 |
|---|---|
| 终态 | `SUCCESS`（02:05:50 → 02:05:54，4 秒） |
| 6 个阶段 | `PHASE_COLLECT` 1/1、`PHASE_DEDUP` 1/1、`PHASE_DECISION` 4/4、`PHASE_EXPORT` 3/3、`PHASE_UPLOAD` 1/1、`PHASE_REVIEW` 5/5 全部 `SUCCESS` |
| 15 个流程 | 全部 `SUCCESS`，`started_at` 严格递增（10747 修正生效） |
| cdeu 写入 | `workflow_run` +1、`phase_run` +6、`process_run` +15、`job_run` +21、`job_step_run` +14、`job_run_log` +117、`job_run_log_line` +451 |
| dedup_frozen | **一行未变**（0 写入） |

### 10759 未覆盖的路径（已由 10760 补齐）

`run 65` 仅 4 秒结束，`job_dynamic` / `job_step_dynamic` 均为 0——因为主表只有 202610（当月、未成熟）的 1,000 行，无导出候选。导出动态 JOB、文件生成、上传三条路径当时未验证，**已由 10760（回填 2,802,911 行 + run 66）全部覆盖**。

## 九十五、回填历史数据并验证完整导出/上传/归档链路（10760）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10760 | 向 `cdeu.finv_quote_secu_kline_min` 回填 `quote.finv_quote_secu_kline_min` 的 93 个月 / 2,802,911 行历史数据，重置 101 个导出任务为 `WAITING` 并清空导出产物登记，在「`dedup` schema 已重命名为 `dedup_frozen`、完全不可用」的条件下跑一轮完整 CDEU 流程，覆盖导出动态 JOB → 文件生成 → 上传 → 归档复核（20007 删主表 / 20011 本地清理 / 动态实例清理）全部路径。 | `run_id=66` 终态 `SUCCESS`；6 个阶段全 `SUCCESS`；`select count(*) from cdeu.finv_quote_cdeu_job_dynamic where workflow_run_id=66` = 202（101 导出 + 101 上传）；`select count(*) from cdeu.finv_quote_cdeu_job_step_dynamic ...` = 505（101×5）且全部 `SUCCESS`；`select file_type, upload_status, count(*) from cdeu.finv_quote_cdeu_export_file group by 1,2` 返回 `BASE/UPLOADED=101` + `MANIFEST/SKIPPED=101`；主表 `2,803,911 → 1,000`（仅剩 202610 陷阱）；`dedup_frozen` 各表行数在流程前后完全不变；日志 `does not exist`/`ERROR` 计数 = 0 | √ |

### 10760 准备步骤

| 步骤 | 内容 |
|---|---|
| 1. 停容器 | 遵循 10732 教训：重置为 `WAITING` 后会被 `RUN_DUE_EXPORT_JOBS` 抢跑 |
| 2. 回填主表 | `INSERT ... SELECT ... FROM quote.finv_quote_secu_kline_min ON CONFLICT (ts, usc) DO NOTHING`，`gmt_create`/`gmt_update` 取 `now()`；耗时 **195.8 秒**，插入 2,802,911 行，结果 2,803,911 行 / 94 月 |
| 3. 清空产物登记 | `upload_log` 202 / `file` 202 / `batch` 202 / `watermark` 101 / `file_patch` 0 全部清空（否则 `RegisterFile` 会因已 `UPLOADED` 而不重新上传） |
| 4. 重置任务 | 101 个 job → `WAITING`，游标 `NULL`、`loaded_rows=0`、`base_version=1`、`patch_count=0`，清锁/心跳/`last_error` |
| 5. 清本地目录 | 删除 176.3 MB / 202 个旧导出文件（GitHub 已有备份） |
| 6. 起容器 | 无迁移重跑，无 SQL 错误 |

### 10760 run 66 执行结果

| 项 | 结果 |
|---|---|
| 终态 / 耗时 | `SUCCESS` / **18 分 52 秒** |
| 阶段 | `PHASE_COLLECT` 1/1、`PHASE_DEDUP` 1/1、`PHASE_DECISION` 4/4、`PHASE_EXPORT` 3/3、`PHASE_UPLOAD` 1/1、`PHASE_REVIEW` 5/5，全部 `SUCCESS` |
| 归档复核 5 流程 | `PROCESS_RECOVER` / `PROCESS_REVIEW` / `PROCESS_PURGE` / `PROCESS_STORAGE_CLEAN` / `PROCESS_RETENTION` 全 `SUCCESS` |
| 导出动态 JOB | **101 个 `EXPORT/BASE` 全部 `SUCCESS`** |
| 上传动态 JOB | **101 个 `UPLOAD` 全部 `SUCCESS`** |
| 动态 Step | **505 个全部 `SUCCESS`**：`STEP_LOAD` 101 / `STEP_EXPORT` 101 / `STEP_REGISTER` 101 / `STEP_CLEANUP` 101 / `STEP_UPLOAD` 101 |
| 证券月覆盖 | `GCMain` 93（201811–202607）、`NVDA` 4（202604–202607）、`XAUUSD` 4（202604–202607） |
| 文件 | `BASE/UPLOADED` **101**、`MANIFEST/SKIPPED` **101**（MANIFEST 按约定不上传） |
| 20007 删主表 | `2,803,911 → 1,000`，仅剩 202610 陷阱月 |
| 20011 本地清理 | 202 个文件仍在（176.3 MB），`cleanup_status=NONE` —— **符合预期**，需上传完成满 24 小时才删 |
| 导出任务终态 | 101 个全部 `EXPORTED` |
| `dedup_frozen` | `cdeu_workflow_run` 11 / `finv_dedup_export_file` 202 / `finv_quote_secu_kline_min` 1,000 —— **流程前后完全不变** |
| `cdeu` 侧 | `workflow_run` 13（含 run 65/66）、`export_file` 202 |
| 日志错误 | `does not exist` / `ERROR` / `panic` = **0** |

### 10760 结论

在 `dedup` schema **完全不存在**（已重命名为 `dedup_frozen`）的条件下，**全链路零错误跑通**：统计 → 成熟度 → 预算 → 导出决策 → 批次清单 → 101 个 BASE 导出 → 101 个上传 → 归档复核 → 主表释放 → 动态实例登记。

至此可确认：**代码已 100% 切换到 `cdeu`，不再有任何 `dedup` 依赖**。`dedup_frozen` 可安全进入观察期后清理。

## 九十六、登录认证与访问拦截（10761）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10761 | 基于 `sys.sys_user` 建立登录认证：① 创建用户 `finv`，口令 `finv@2026!finv` **不得明文入库**，采用 argon2id（m=64MB/t=3/p=2/keyLen=32）哈希，盐 16 字节随机 hex 存 `password_salt`、PHC 串存 `password`；② 登录成功后签发随机会话令牌，会话存 Redis 并按 `sys_config` 20012 控制有效期（默认 480 分钟，命中且剩余不足一半时滑动续期）；③ 除健康检查、版本、登录外的全部接口必须携带 `Authorization: Bearer <token>`，否则 401；④ 前端新增登录页并加路由守卫，未登录一律重定向到 `/login`，接口 401 时自动清理本地会话并回登录页。 | `select count(*) from sys.sys_user where password like '%finv@2026%'` = 0；`select password from sys.sys_user where uid='finv'` 以 `$argon2id$v=19$m=65536,t=3,p=2$` 开头且长度为 97；`select password_salt from sys.sys_user where uid='finv'` 匹配 `^[0-9a-f]{32}$`；无令牌 `GET /API/V1/Meta/Finv/Quote/DeDup/Status` = 401 且 `GET /API/V1/Health/Live` = 200；错误口令与不存在账号均返回 401「用户名或密码错误」；正确口令返回 `code=0` 与 64 字符 token，携带该 token 请求业务接口 = 200；调用 `/Meta/Finv/Auth/Logout` 后原 token 再访问 = 401；`grep -c 'isLoggedIn' Web/src/router.ts` ≥ 1 且 `test -f Web/src/views/Common/LoginView.vue` | √ |

### 10761 实现要点

| 项 | 说明 |
|---|---|
| 哈希算法 | argon2id，参数 `m=65536` `t=3` `p=2` `keyLen=32`；输出 PHC 串 97 字符，适配 `password` varchar(128) |
| 盐 | `crypto/rand` 16 字节 → hex 32 字符，独立存 `password_salt`（varchar(64)） |
| 校验 | `crypto/subtle.ConstantTimeCompare` 常量时间比较，防时序侧信道 |
| 防账号枚举 | 用户不存在时仍执行一次等价耗时的 argon2 计算，失败文案与密码错误完全相同 |
| 会话 | 令牌 32 字节随机 → hex 64 字符；Redis 键 `finv:auth:session:<token>`，值为 JSON 会话记录 |
| 有效期 | 每次登录读 `sys_config` 20012；`Touch` 用 `ExpiresAt-CreatedAt` 推算续期长度，运行期改配置不影响已签发会话 |
| 吊销 | 登出即 `DEL` Redis 键，服务端可即时失效（无状态 JWT 做不到） |
| 白名单 | `/Health/Live`、`/Health/Ready`、`/Version`、`/Meta/Finv/Auth/Login` 不校验令牌（容器探针必需） |

### 10761 迁移

| 迁移 | 内容 |
|---|---|
| `V200081__sys_user_uid_unique.sql` | 新增 `uk_sys_user_uid` 唯一索引（V74 建表时 uid 未加唯一约束，登录以 uid 为标识必须唯一） |
| `V300055__sys_user_seed_finv.sql` | 插入 `uid='finv'`，含盐与 PHC 哈希；`WHERE NOT EXISTS` 保证幂等且不覆盖已改口令 |
| `V300056__sys_config_seed_auth_session_ttl.sql` | 新增配置 20012（会话有效期，默认 480 分钟，合法区间 5~43200） |

### 10761 新增文件

| 文件 | 职责 |
|---|---|
| `internal/meta/finv/auth/password.go` | argon2id 哈希生成与校验、随机盐、PHC 解析 |
| `internal/meta/finv/auth/session.go` | Redis 会话存储：Create / Resolve / Revoke / Touch |
| `internal/meta/finv/auth/store.go` | 按 uid 查询 `sys.sys_user` |
| `internal/meta/finv/auth/service.go` | 登录编排、防枚举等时处理、资料查询 |
| `internal/api/middleware.go` | `RequireAuth` 中间件与 Bearer 解析 |
| `internal/api/handler/auth.go` | Login / Logout / Profile 处理器与 `SessionFrom` |
| `Web/src/auth.ts` | 会话状态、localStorage 持久化、失效事件 |
| `Web/src/views/Common/LoginView.vue` | 登录页（口令不留存前端） |

## 九十七、认证与配置模块目录归位（10762）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10762 | 新增与既有的两个通用模块按 `internal/meta/finv/<域>/` 结构归位：`internal/auth` → `internal/meta/finv/auth`，`internal/syscfg` → `internal/meta/finv/sys`。仅移动目录与更新 import 路径，包名、行为与导出符号一律不变。 | `test ! -d internal/auth && test ! -d internal/syscfg`；`ls internal/meta/finv/auth/*.go internal/meta/finv/sys/*.go` 各有 4 个与 2 个文件；`grep -rn 'finvquant/internal/auth"\|finvquant/internal/syscfg"' --include=*.go .` 无输出；`go build ./... && go vet ./... && go test ./...` 全绿 | √ |

## 九十八、登录后修改密码（10763）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10763 | 登录用户可自助修改密码：① 必须校验当前密码；② 新密码需满足强度规则（长度 8–64 位，且含大写/小写/数字/符号中至少 3 类）；③ 新密码不得与当前密码相同；④ 修改时重新生成盐与 argon2id 哈希（**不落明文**）；⑤ 修改成功后吊销该用户在其它设备上的全部会话，**保留当前会话**避免本人被踢出；⑥ 前端在顶栏用户菜单提供入口，对话框中三个口令输入框都支持右侧眼睛图标切换明文/密文。 | `POST /API/V1/Meta/Finv/Auth/ChangePassword`：旧密码错、仅小写 8 位、新旧相同三种情形均返回 400 且 `message` 指明原因；正常修改返回 `code=0` 与 `data.revoked_sessions`；改后旧密码登录 401、新密码登录 200；操作会话仍可访问 `/Meta/Finv/Auth/Profile`（200），另一并发会话变 401；`select password_salt from sys.sys_user where uid='finv'` 在改密前后取值不同且匹配 `^[0-9a-f]{32}$`；`select count(*) from sys.sys_user where password like '%finv@2026%'` = 0；`grep -c 'append-inner-icon' Web/src/views/Common/ChangePasswordDialog.vue` ≥ 3 | √ |

### 10763 口令强度规则

| 规则 | 取值 | 理由 |
|---|---|---|
| 长度 | 8–64 位 | 上限防止超长输入拖慢 argon2（每字节都参与计算） |
| 类别 | 大写/小写/数字/符号中至少 **3** 类 | 强制四类会逼出 `Passw0rd!` 这类可预测变形，3 类在可用性与强度间更平衡 |
| 新旧 | 不得相同 | 轮换应产生新凭据，而不是原地重复 |

服务端 `ValidatePasswordStrength` 与前端 `strengthOk` 使用同一口径，前端仅做即时反馈，**裁决权在服务端**。

### 10763 会话吊销设计

会话键是 `token → 会话`，无法按用户反查，因此新增反向索引：

```
finv:auth:user:<user_id>   (Set，保存该用户全部 token)
```

| 时机 | 动作 |
|---|---|
| 登录成功 | `SADD` 索引 |
| 登出 | `DEL` 会话键 + `SREM` 索引 |
| 修改密码 | 遍历索引，`DEL` 除当前 token 外的全部会话键并 `SREM`；索引项无论会话是否仍存在一律移除（顺带清理过期残留） |

> 索引不设 TTL，但每次吊销都会清理对应项，不会无限增长。

## 九十九、前端基础路由改为大驼峰（10764）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10764 | 前端基础路由 `/dashboard`、`/login` 统一改为大驼峰 `/Dashboard`、`/Login`，与既有 `/Meta/Finv/...` 风格一致；旧的小写路径保留归一化重定向，避免历史书签失效。路由 `name`（`dashboard` / `login`）保持不变，仅调整 URL 路径。 | `grep -c "path: '/Dashboard'\|path: '/Login'" Web/src/router.ts` ≥ 2；`grep -rn "'/dashboard'\|'/login'" Web/src --include=*.ts --include=*.vue` 仅出现在归一化映射表中；浏览器访问 `/login` 落到 `/Login`、访问 `/dashboard` 落到 `/Dashboard`（未登录时为 `/Login?redirect=/Dashboard`）；已登录访问 `/Login` 落到 `/Dashboard` | √ |

### 10764 为什么不能用路由级 redirect

`vue-router` 4 的路径匹配默认 **大小写不敏感**（`sensitive: false`），`/login` 会直接命中 `/Login` 路由，写在路由表里的 `{ path: '/login', redirect: '/Login' }` **永远匹配不到**，实测访问 `/login` 时 URL 停留在 `/login`。

改为在 `beforeEach` 守卫里做归一化——此时 `to.path` 仍是访问时的原始大小写，是唯一可靠的位置：

```ts
const LEGACY_PATH_MAP: Record<string, string> = {
  '/login': '/Login',
  '/dashboard': '/Dashboard',
}
if (LEGACY_PATH_MAP[to.path]) {
  return { path: LEGACY_PATH_MAP[to.path], query: to.query, hash: to.hash, replace: true }
}
```

## 一百、认证接口前缀统一到 /Meta/Finv/Auth（10765）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10765 | 认证接口由 `/API/V1/Auth/*` 统一调整为 `/API/V1/Meta/Finv/Auth/*`，与 `/Meta/Finv/Quote`、`/Meta/Finv/Quant` 同层级。同步修正前端调用与接口文档，并修复 `api.ts` 把登录接口的 401 误判为「会话失效」的问题。 | `grep -rn '"/Auth' internal --include=*.go` 无输出；`grep -rn "'/Auth/" Web/src` 无输出；`POST /API/V1/Meta/Finv/Auth/Login` 返回 `code=0`，旧路径 `POST /API/V1/Auth/Login` 返回 404；浏览器用错误口令登录时页面提示「用户名或密码错误」而非「未登录或登录已过期」，且路径停留在 `/Login`；正确口令登录落到 `/Dashboard`；改密对话框可正常打开 | √ |

### 10765 端点映射

| 旧路径 | 新路径 |
|---|---|
| `POST /API/V1/Auth/Login` | `POST /API/V1/Meta/Finv/Auth/Login` |
| `POST /API/V1/Auth/Logout` | `POST /API/V1/Meta/Finv/Auth/Logout` |
| `POST /API/V1/Auth/ChangePassword` | `POST /API/V1/Meta/Finv/Auth/ChangePassword` |
| `GET /API/V1/Auth/Profile` | `GET /API/V1/Meta/Finv/Auth/Profile` |

### 10765 顺带修掉的 401 误判

`api.ts` 原先对所有 401 一律执行「清空会话 + 广播失效事件 + 抛『未登录或登录已过期』」。登录接口本身用 401 表示「用户名或密码错误」，于是：

- 用户输错密码看到的是「未登录或登录已过期」，完全误导
- 还会误清一次本来就不存在的会话并触发一次无效的路由跳转

修正为按路径分流：`path.endsWith('/Auth/Login')` 的 401 走正常信封解析，把服务端 `message` 原样抛给用户；其余 401 才执行会话失效流程。

## 一百零一、Native 安装到统一根目录 /data/App/FinvQuant（10766）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10766 | Native（宿主直装）方式把程序、数据库脚本、核心配置、部署脚本与部署文件统一安装到 `/data/App/FinvQuant/`：① 二进制 `bin/finvquant`（软链 `/usr/local/bin/finvquant`）；② 迁移脚本 `Migrations/`；③ 配置 `etc/finvquant.env`（权限 600）；④ 部署文件 `Deploy/`（`Install.sh`/`Upgrade.sh`/`Uninstall.sh`、unit、env 与 Nginx 模板、手册）；⑤ 备份 `Backup/`、导出 `Export/`；⑥ systemd unit 由 `Deploy/FinvQuant.service` 的 `@PREFIX@` 渲染后注册。安装根目录可用 `--prefix` 覆盖，升级/卸载脚本自动从 unit 的 `ExecStart` 推断。 | `grep -c 'FINVQ_PREFIX' Deploy/Native/Install.sh` ≥ 1；`grep -rn '/opt/finvquant\|/var/backups/finvquant' Deploy/Native/` 无输出；在测试机执行 `sudo bash Deploy/Native/Install.sh` 后 `test -x /data/App/FinvQuant/bin/finvquant && test -f /data/App/FinvQuant/etc/finvquant.env && test -d /data/App/FinvQuant/Migrations && test -x /data/App/FinvQuant/Deploy/Upgrade.sh && test -d /data/App/FinvQuant/Backup && test -d /data/App/FinvQuant/Export`；`grep -E '^(WorkingDirectory\|EnvironmentFile\|ExecStart\|ReadWritePaths)=/data/App/FinvQuant' /etc/systemd/system/finvquant.service` 全部命中；`systemctl status finvquant` 为 `active (running)`；`curl -o /dev/null -w '%{http_code}' http://127.0.0.1:16001/API/V1/Health/Ready` = 200 | |

## 一百零二、一键下载脚本（10767）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10767 | 新增一键下载脚本 `Deploy/Script/Download.sh`（及 Windows 薄封装 `Download.cmd`）：自动检查操作系统与硬件架构，自动下载匹配的最新发行包（支持 `auto`/`release`/`snapshot`/`pr` 四种渠道，`auto` 包含快照版），校验 SHA256 后解压到指定目录（默认 `/data/App/FinvQuant`）；支持 `--version`、`--list`、`--dry-run`、`--no-extract`、`--keep-top`、`--force`、`--base-url`、`--token`。 | `bash -n Deploy/Script/Download.sh` 退出码 0；`bash Deploy/Script/Download.sh --help` 输出包含 `--channel` 与 `--base-url`；`bash Deploy/Script/Download.sh --list` 列出至少 1 个 `v` 开头版本；`bash Deploy/Script/Download.sh --dry-run -v <某快照 tag>` 输出 `FinvQuant-<tag>-ubuntu-amd64.tar.gz` 与 `/data/App/FinvQuant`；在本地镜像（`python3 -m http.server` + 伪造包）上执行 `Download.sh -v v9.9.9 --base-url <镜像> -d /tmp/t` 后 `test -f /tmp/t/finvquant && test -d /tmp/t/Deploy/Native` 且日志含「SHA256 校验通过」「包内 SHA256SUMS 校验通过」；对同一目录不加 `--force` 重跑返回非 0；篡改 `.sha256` 后重跑返回非 0；`file Deploy/Script/Download.cmd` 含 `CRLF` 且首行为 `@chcp 65001 >nul` | |

## 一百零三、发行包 Migrations 提到包顶层（10768）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10768 | 迁移目录与仓库结构保持一致：安装后为 `<PREFIX>/Deploy/Migrations`（默认 `/data/App/FinvQuant/Deploy/Migrations`），发行包内同样为 `Deploy/Migrations/`；`FINV_MIGRATIONS_DIR` 指向该目录，`Install.sh`/`Upgrade.sh` 按 `<PREFIX>/Deploy/Migrations` → `<repo>/Deploy/Migrations` → Native 脚本上一级/同级顺序探测，源即安装目标时跳过复制。 | `grep -c 'FINV_MIGRATIONS_DIR=/data/App/FinvQuant/Deploy/Migrations' Deploy/Native/FinvQuant.env.example` = 1；`grep -n 'cp -r Deploy/Migrations "${PKG_ROOT}/Deploy/Migrations"' Deploy/Script/PackSnapshot.sh` 命中；`bash Deploy/Script/PackSnapshot.sh --target linux/amd64 --keep-tmp` 后 `tar -tzf Dist/FinvQuant-*-ubuntu-amd64.tar.gz \| grep -cE '^FinvQuant-[^/]+/Deploy/Migrations/.*\.sql$'` > 0；在解压目录执行 `sudo bash Deploy/Native/Install.sh --prefix <dir>` 日志含「迁移目录即安装目录，无需复制」且 `test -x <dir>/Deploy/Migrations` 成立；`grep -c 'PREFIX}/Deploy/Migrations' Deploy/Native/Install.sh Deploy/Native/Upgrade.sh Deploy/Native/Uninstall.sh` 各 ≥ 1 | |

## 一百零四、核心配置放到安装根目录并修复安装摘要渲染（10769）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10769 | ① Native 方式的核心配置文件为 `<PREFIX>/finvquant.env`（默认 `/data/App/FinvQuant/finvquant.env`），不再放在 `<PREFIX>/etc/`；systemd unit 的 `EnvironmentFile` 指向该文件；`Install.sh`/`Upgrade.sh` 自动把旧布局 `<PREFIX>/etc/finvquant.env` 迁移到新位置并保留原密码；② 安装完成摘要用 `printf` 输出，ANSI 颜色正常渲染，不再出现字面 `\033[1;32m`。 | `grep -n 'EnvironmentFile=@PREFIX@/finvquant.env' Deploy/Native/FinvQuant.service` 命中；`grep -c 'ENV_FILE="${PREFIX}/finvquant.env"' Deploy/Native/Install.sh Deploy/Native/Upgrade.sh` 各 ≥ 1；在存在 `/data/App/FinvQuant/etc/finvquant.env` 的机器上重跑 `sudo bash /data/App/FinvQuant/Deploy/Native/Install.sh`，日志含「已迁移旧配置」且 `test -f /data/App/FinvQuant/finvquant.env` 成立、`test ! -d /data/App/FinvQuant/etc` 成立；`systemctl show -p EnvironmentFiles finvquant` 显示 `/data/App/FinvQuant/finvquant.env`；安装脚本尾部输出含彩色「安装完成」且不出现字面 `\033[1;32m`（`sudo bash Deploy/Native/Install.sh 2>&1 \| grep -c '\\033\[1;32m'` = 0） | |

## 一百零五、安装脚本支持备份后重建空库（10770）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10770 | `Install.sh` 对「已存在但非本程序初始化」的数据库只做**只读预检**：目标库非空且缺 `public.schema_version` 时输出撞表风险与「改用独立库名/独立 PG 实例」指引，**不执行 `dropdb`/`createdb`，不删除、不改名、不迁移任何已有表**。 | `grep -c 'recreate-db' Deploy/Native/Install.sh` = 0；在非空旧库上执行 `sudo bash Deploy/Native/Install.sh`，日志含「数据库 … 已存在」与撞表告警、且执行前后 `select count(*) from information_schema.tables where table_schema not in ('pg_catalog','information_schema')` 不变 | |

## 一百零六、首次部署兼容共享 public 同名表并修复迁移链（10771）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10771 | 让全新数据库在 `public` 已存在**其它服务**同名表（如 `public.finv_quote_secu_kline_min`）时也能完成首次部署：① 历史迁移中本程序自有表的建表/ALTER/DML 全部显式限定 `quote.`；② `V200032` 不得从 `public` 搬移非本程序创建的表（目标 schema 已有则跳过）；③ 修复全新库下 `V200080`（表数/列漂移）、`V300039`（空库断言）、`V300048`/`V300050`（硬编码 phase_id）导致的迁移中断；④ 新增 `V300057` 将 dedup 侧 CDEU 配置同步到 cdeu。 | 在临时 PG18 上先建 `public.finv_quote_secu_kline_min`（含注释、索引与 3 行数据），再执行全部迁移：`grep -cE 'ERROR\|FATAL'` = 0；该校验前后 `select md5(string_agg(t::text,'\|' order by id)) from public.finv_quote_secu_kline_min t` 结果一致且 `pg_dump --schema-only -t public.finv_quote_secu_kline_min` 除 `\restrict` 随机 token 外无差异；`select count(*) from pg_tables where schemaname='quote' and tablename in ('finv_quote_secu_kline_min','finv_quote_ingest_batches','finv_quote_revision_log')` = 3；`select count(*) from cdeu.finv_quote_cdeu_process` 与 `select count(*) from dedup.cdeu_process` 相等；`grep -c 'AND NOT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = '\''quote'\''' Deploy/Migrations/V200032__*.sql` ≥ 1；`grep -c 'phase_id = 12' Deploy/Migrations/V300048__*.sql Deploy/Migrations/V300050__*.sql` = 0 | |

## 一百零七、历史迁移不得删除 public 中非本程序表（10772）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10772 | 历史迁移中不得存在针对 `public` 中非本程序表的 `DROP TABLE` / `TRUNCATE` / `ALTER TABLE`：涉及 `finv_quote_secu_kline_min` 的删除操作一律带 `export.`/`dedup.` 前缀；`V200015` 重建 `finv_futu_mapping_market_code` 的 `CREATE/INSERT/DROP/ALTER/INDEX/TRIGGER` 全部显式加 `public.`；运行期 `TRUNCATE` 使用带 `export.` 前缀的常量。 | `grep -rnE 'DROP TABLE\|TRUNCATE' Deploy/Migrations/*.sql \| grep -i kline_min` 命中的语句均带 `export.` 或 `dedup.` 前缀；`grep -cE 'DROP TABLE( IF EXISTS)? +[a-z_]+ *;' Deploy/Migrations/*.sql` = 0；`grep -c 'public.finv_futu_mapping_market_code' Deploy/Migrations/V200015__*.sql` ≥ 6；`grep -n 'tableStaging' internal/meta/finv/quote/dedup/store.go` 显示值为 `export.finv_quote_secu_kline_min`；在 `public` 预置同名表后执行全部 139 个迁移，`public` 表数据与结构前后一致 | |

## 一百零八、Native 暴露与历史数据迁移（10773–10775）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10773 | 提供**一次性**数据迁移脚本 `Deploy/Upgrade/2610-004-MigratePublicKlineToCdeu.sql`，把目标库 `public.finv_quote_secu_kline_min` 的历史数据搬入 `cdeu.finv_quote_secu_kline_min`：源表只读；列名映射 `secu_code→usc`、`prev_close→last_close`，`market_code` 丢弃；同 `(ts,usc)` 多 `market_code` 去重保留最新；`ON CONFLICT (ts,usc) DO NOTHING` 幂等；单事务 + 计数校验，异常整体回滚；不进入迁移链。 | 在含旧结构 `public` 表（5 行，含 1 组重复 `ts`+`secu_code`）的库上执行全部迁移后再跑脚本：源表行数保持 5、目标表 4 行、重复组保留 `gmt_update` 最新值、`last_close` 等于源 `prev_close`；重复执行输出「新增 0 行」且目标仍为 4 行；`grep -c 'ON CONFLICT (ts, usc) DO NOTHING' Deploy/Upgrade/2610-004-MigratePublicKlineToCdeu.sql` ≥ 1 | |
| 10774 | `Nginx.conf.example` 改为**分阶段**模板：阶段一默认 HTTP/80 + `server_name _;` + `default_server`，无需域名/证书即可 `http://<IP>` 访问；阶段二（443 + `http2 on;` + 证书路径 + 80→443 跳转）以整段注释保留，取消注释即可启用；注释内含切换步骤与 nginx < 1.25.1 写法说明；文档说明需移除 Ubuntu 自带 `sites-enabled/default`。 | `grep -vE '^\s*#' Deploy/Native/Nginx.conf.example \| grep -cE 'listen\s+443\|ssl_certificate\|return 301 https'` = 0；`grep -c 'listen 80 default_server;'` ≥ 1；`grep -c 'server_name _;'` ≥ 1；`grep -cE '^#\s+listen 443 ssl;'` ≥ 1 且 `grep -cE '^#\s+ssl_certificate\s'` ≥ 1；删除自带 default 站点后 `nginx -t` 通过（阶段一）；取消注释块并按 1.24 写法替换后 `nginx -t` 通过（阶段二） | |
| 10775 | 在 `Docs/FinvQuant/` 提供两份文档：`Nginx-Deploy.md`（阶段一部署/验证/常见问题、阶段二切换/回滚、与 systemd 的关系）与 `Nginx.conf.example-ChangeRequest.md`（可直接作为上游 Issue 正文：问题现象、期望行为、逐项改动、验收标准、兼容性）。 | `ls Docs/FinvQuant/Nginx-Deploy.md Docs/FinvQuant/Nginx.conf.example-ChangeRequest.md` 均存在；前者含「阶段一」「阶段二」「常见问题」「回滚」章节；后者含「建议 Issue 标题」「验收标准」「兼容性」章节；`Deploy/Native/Install.md` §6 与 `Deploy/Native/README.md` 文件表描述已同步为分阶段 | |

## 一百零九、部署包包含一次性运维 SQL（10776）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10776 | 部署包（快照包 / 构建包 / 正式包）必须包含 `Deploy/Init/*.sql` 与 `Deploy/Upgrade/*.sql` 初始化与升级维护脚本；`Install.sh` 安装后需留存到 `<PREFIX>/Deploy/Init/` 与 `<PREFIX>/Deploy/Upgrade/`（权限 0644），且两目录**不进入** `Deploy/Migrations/` 迁移链。 | 执行 `bash Deploy/Script/PackSnapshot.sh --target linux/arm64` 后：`tar -tzf Dist/*.tar.gz \| grep 'Deploy/Upgrade/2610-004-MigratePublicKlineToCdeu.sql'` 与 `grep 'Deploy/Init/2610-001-CreateDatabase.sql'` 均命中；Windows zip 用 `unzip -l` 同样命中；包内该文件权限为 `0644`。安装后 `ls -l /data/App/FinvQuant/Deploy/Init/ /data/App/FinvQuant/Deploy/Upgrade/` 存在 SQL。源码目录与安装目录同布局时重复执行 `Install.sh` 不报错。 | |

## 一百一十、分钟行情 Upsert 端点迁移到 CDEU 前缀（10777）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10777 | 把分钟行情批量 Upsert 端点从 `/API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Upsert` 调整为 `/API/V1/Meta/Finv/Quote/Cdeu/Min/Upsert`；旧路径（含 legacy 前缀 `/API/V1/Meta/Finv/Quote/Secu/KLine/Min/Upsert`）不再注册；其它 K 线端点保持不变。 | `grep -c 'CdeuUpsertPath' internal/meta/finv/quote/cdeu/kline/model.go internal/api/router.go` ≥ 2；`grep -c '/Cdeu/Min/Upsert' internal/api/router_test.go` ≥ 1；`grep -c 'POST ...DeDup/Secu/KLine/Min/Upsert'` 在 router.go 中为 0；运行 `go test ./internal/api -run TestKLineRoutesRegistered` 通过（新路径已注册、两条旧路径未注册）；`curl -X POST http://127.0.0.1:16001/API/V1/Meta/Finv/Quote/Cdeu/Min/Upsert` 不再返回 404，旧路径返回 404 | |

## 一百一十一、Markdown 表格必须带表头（10778）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10778 | `Docs/` 下所有 Markdown 表格必须包含表头行与 `\|---\|` 分隔行，不得出现只有数据行的“伪表格”（本次修复 KLine 系列 11 处错误码表格）；并在 `internal/docs` 中增加自动校验防回归。 | `go test ./internal/docs -run TestMarkdownTablesHaveHeader` 通过；`Docs/API/Quote/Cdeu/Min/KLineUpsert.md` 中错误码表格首两行为 `\| 错误码 \| HTTP \| 说明 \|` 与 `\|---\|---\|---\|`；用任意 Markdown 渲染器（或 `pandoc -f gfm`）渲染该文件，错误码呈现为三列表格而非连续文本 | |

## 一百一十二、全仓 Markdown 表格结构合法（10779）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10779 | 仓库内所有 Markdown 文档（根目录、`Docs/`、`Deploy/`、`Archive/`）的表格必须：① 有表头与分隔行；② 分隔行、数据行列数与表头一致（单元格内 `\|` 才表示字面管道）；③ 表头列与真实表结构一致。由 `internal/docs` 自动校验。 | `go test ./internal/docs` 通过；全仓扫描脚本（跳过 `.git`/`node_modules`/`Dist`/`pkg`）输出 0 处「缺表头/列数不一致」；`grep -c 'market_code' Docs/DataDictMapping/FinvSecurity.md` 的表格行不含该列（`finv_security` 无此字段）；`go test ./internal/docs -run TestMarkdownTablesHaveHeader -v` 通过 | |

## 一百一十三、KLinePage 系列文档目录归位（10780）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10780 | 把 `KLinePage.md`、`KLinePageUni.md` 从 `Docs/API/KLine/` 移动到 `Docs/API/Quote/Cdeu/Min/`，并同步所有索引与链接（端点路径保持不变）；同时修复仓库内失效的 Markdown 相对链接。 | `ls Docs/API/Quote/Cdeu/Min/{KLinePage.md,KLinePageUni.md,KLineUpsert.md}` 均存在；`ls Docs/API/KLine/KLinePage*.md` 不存在；`grep -rn 'KLine/KLinePage' Docs/` 无输出；全仓 Markdown 相对链接校验 0 处失效；`go test ./internal/docs` 通过 | |

## 一百一十四、全部分钟行情端点统一 Cdeu 前缀（10782）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10782 | 把分钟行情全部端点（`Add`、`Batch` POST/PATCH/DELETE、`Upsert`、`Page`、`PageUni`、`Query`、`Count`、`Stats`、`Bounds`）统一到 `/API/V1/Meta/Finv/Quote/Cdeu/Min/*`；旧 `DeDup/Secu/KLine/Min` 与 legacy `Secu/KLine/Min` 不再注册；`Docs/API/KLine/` 下全部文档迁入 `Docs/API/Quote/Cdeu/Min/`；前端 API 调用同步（页面路由不变）。 | `grep -c 'Cdeu/Min' internal/meta/finv/quote/cdeu/kline/model.go` ≥ 1；`grep -c 'Secu/KLine/Min' internal/api/router.go` = 0（仅注释例外）；`go test ./internal/api -run TestKLineRoutesRegistered` 通过（新路径已注册、22 条旧路径未注册）；`ls Docs/API/Quote/Cdeu/Min/*.md \| wc -l` = 12 且 `Docs/API/KLine` 不存在；全仓 Markdown 相对链接 0 失效；`go test ./internal/docs` 通过；`curl -X POST http://127.0.0.1:16001/API/V1/Meta/Finv/Quote/Cdeu/Min/Batch` 非 404，旧路径返回 404 | |

## 一百一十五、接口文档按 OpenAPI 3.1 编写（10783）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10783 | `KLineUpsert.md` 按 OpenAPI 3.1 规范重写并作为后续接口文档样板：必须包含完整 `openapi` 代码块（paths/parameters/requestBody/responses/components）、逐字段约束表、行为语义、全量错误目录（code/HTTP/retryable/debug 键）、curl 与成功/错误示例；不得用「同 XXX 文档」省略契约。`ApiSpec` 增加 §7.7 样板约定。 | `grep -c 'openapi: 3.1.0' Docs/API/Quote/Cdeu/Min/KLineUpsert.md` = 1；YAML 块可被 `yaml.safe_load` 解析且含 `paths`/`components.schemas`；`grep -c '同 \[`KLineBatchInsert.md`'` = 0；错误码 `211001`–`211303` 共 15 个全部出现在错误表中；`grep -c '7.7 接口文档样板' Docs/DevSpec/ApiSpec.md` = 1；全仓 Markdown 链接 0 失效；`go test ./internal/docs` 通过 | |

## 一百一十六、OpenAPI 契约独立为同名 .yml（10784）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10784 | 接口文档的机器可读契约从 `.md` 中拆出为**同名 `.yml`**（如 `KLineUpsert.md` + `KLineUpsert.yml`）：`.md` 正文只保留人类可读说明与契约链接/命令，`.yml` 为完整 OpenAPI 3.1；并增加自动校验。 | `ls Docs/API/Quote/Cdeu/Min/KLineUpsert.{md,yml}` 均存在；`grep -c '```yaml' Docs/API/Quote/Cdeu/Min/KLineUpsert.md` = 0；`KLineUpsert.yml` 可被 YAML 解析且含 `openapi`/`paths`；`go test ./internal/docs -run TestOpenAPISpecsValid` 通过；`grep -c '同名 `.yml`' Docs/DevSpec/ApiSpec.md` ≥ 1；全仓 Markdown 相对链接 0 失效 | |

## 一百一十七、KLinePageUni 按 OpenAPI 样板重写（10785）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10785 | `KLinePageUni.md` 按 `KLineUpsert.md` 样板重写：补齐全部 Query 参数（`usc`/`provider`/`dateFrom`/`dateTo`/`tsFrom`/`tsTo`/`limit`/`offset` 及兼容旧名）、过滤/排序/分页语义、15 列定义与单元格取值规则、响应信封、全量错误码、示例；并拆出同名 `.yml` OpenAPI 3.1 契约。 | `ls Docs/API/Quote/Cdeu/Min/KLinePageUni.{md,yml}` 均存在；`KLinePageUni.yml` 可解析且 `paths` 为 `/API/V1/Meta/Finv/Quote/Cdeu/Min/PageUni`；`grep -c '```yaml' Docs/API/Quote/Cdeu/Min/KLinePageUni.md` = 0；`grep -c 'limit.*1–1000' Docs/API/Quote/Cdeu/Min/KLinePageUni.md` ≥ 1；`go test ./internal/docs` 通过；全仓 Markdown 链接 0 失效 | |

## 一百一十八、外部 HOOK 接入方案（10786）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10786 | 产出外部 HOOK 接入的方案文档（不含实现）：覆盖现状认证、方案对比、推荐方案的凭据/数据模型/中间件/路由/限流/审计/配置/安全、待确认决策点、验收标准与实施计划。 | `test -f Docs/Refactor/2610-009-ExternalHookApiKey.md`；文档含「方案对比」「已确认的决策」「验收标准」「实施计划」章节，决策点 ≥ 10 项且含最终表名 `sys.sys_open_api_key`；过程文档编号 `2610-009` 全局唯一（`go test ./internal/docs -run TestProcessDocNumberUnique` 通过）；`go test ./internal/docs` 通过 | |

## 一百一十九、外部 OpenAPI 示例端点（10787）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10787 | 新增外部 OpenAPI 示例端点 `POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert`，业务逻辑复用内部 `KLine.Upsert` handler；使用独立路由组与静态密钥 `FINV_OPENAPI_KEY`（支持 Bearer 与 `X-API-Key`），未配置时不注册。 | `grep -c '/OpenAPI/V1' internal/api/router.go` ≥ 1；`grep -c 'RequireOpenAPIKey' internal/api/router.go internal/api/middleware.go` ≥ 2；`go test ./internal/api -run TestOpenAPIExampleRoute -v` 通过（未配置不注册、错误密钥 401、Bearer/X-API-Key 正确密钥均进入 handler 返回 400/211001）；未设置 `FINV_OPENAPI_KEY` 时 `curl -X POST http://127.0.0.1:16001/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert` 返回 404 | |

## 一百二十、外部 OpenAPI 响应信封规范化（10788）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10788 | `/OpenAPI/V1` 下接口成功响应统一为 `{"code":0,"data":<原业务数据>,"message":"OK"}`：业务字段放入 `data`；成功码默认 `0`，`1`/`200`/`202` 兼容；非 2xx 与 `204` 原样透传。补充外部端点文档（`.md` + `.yml`）。 | `grep -c '"code": 0' internal/api/middleware.go` = 1；`go test ./internal/api -run TestWrapOpenAPIResponseEnvelope -v` 通过（断言 `code=0`、`data.affected`、`data.mode`、204 无 body、错误原样）；`grep -c 'code: 0' Docs/OpenAPI/Quote/Cdeu/Min/Upsert.yml` ≥ 1 且 `enum` 含 0/1/200/202；`ls Docs/OpenAPI/Quote/Cdeu/Min/Upsert.{md,yml}` 均存在；`go test ./internal/docs` 通过 | |

## 一百二十一、外部 OpenAPI PageUni 查询端点（10789）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10789 | 为内部 `/API/V1/Meta/Finv/Quote/Cdeu/Min/PageUni` 暴露外部端点 `GET /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni`：复用 `KLine.PageUni` handler；静态密钥认证；成功响应 `{code:0,data,message}`（内部列表信封解包后放入 `data`）；补 `.md` + `.yml` 文档。 | `grep -c 'OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni' internal/api/router.go` = 1；`go test ./internal/api -run TestOpenAPIPageUniRoute -v` 通过（注册、401、`limit=0` → 422）；`go test ./internal/api -run TestWrapOpenAPIResponseEnvelope -v` 通过（内部信封解包不嵌套）；`ls Docs/OpenAPI/Quote/Cdeu/Min/PageUni.{md,yml}` 均存在；`PageUni.yml` 可解析且 paths 正确；`go test ./internal/docs` 通过；全仓链接 0 失效 | |

## 一百二十二、外部 OpenAPI 暂不鉴权（10790）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10790 | 暂时放开 `/OpenAPI/V1/*` 的鉴权（公开访问），仅保留统一响应信封；`RequireOpenAPIKey` 与 `FINV_OPENAPI_KEY` 代码保留、待 API Key 体系落地后恢复；文档与契约同步标注「暂不鉴权 + 后续恢复」。 | `grep -c 'openGroup.Use(WrapOpenAPIResponse())' internal/api/router.go` = 1；`go test ./internal/api -run 'TestOpenAPIExampleRoute\|TestOpenAPIPageUniRoute' -v` 通过（**无密钥**调用分别返回 400/422，说明已进入 handler）；两份 `Docs/OpenAPI/**/*.yml` 含 `security: []` 且可解析；外部 `.md` 含「暂不鉴权」；`go test ./internal/docs` 通过；全仓 Markdown 链接 0 失效 | |

## 一百二十三、cdeu 成交量改为浮点（10791）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10791 | 将 `cdeu.finv_quote_secu_kline_min.volume` 由 `bigint` 改为浮点 `numeric(20,6)`，保留 `volume >= 0` 约束；幂等迁移，仅改 `cdeu` 表。 | `grep -c 'numeric(20,6)' Deploy/Migrations/V300058__cdeu_kline_min_volume_numeric.sql` ≥ 1；临时 PG 执行全部迁移后 `select data_type from information_schema.columns where table_schema='cdeu' and table_name='finv_quote_secu_kline_min' and column_name='volume'` = `numeric`；插入 `12.345678` 后读回一致；重复执行 V300058 输出「已是 numeric，跳过」；`go build ./...`、`go test ./internal/meta/finv/quote/cdeu/kline` 通过；`MVsvFileFormat.md` 中 `v` 类型为 `Decimal(20,6)` | |

## 一百二十四、volume 全链路浮点化（10792）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10792 | 把 `quote` / `export` / `dedup` 三张同名表的 `volume` 与 `cdeu` 统一为 `numeric(20,6)`；`internal/meta/finv/quote/mvsv` 解析保留小数文本；`internal/meta/finv/quote` 查询输出支持小数；保证导入/导出/补丁/回测链路不截断。 | 临时 PG 执行全部迁移后 `select table_schema, data_type from information_schema.columns where column_name='volume' and table_name='finv_quote_secu_kline_min' and table_schema in ('quote','dedup','export','cdeu')` 四行均为 `numeric`；`12.345678` 在四表写入/读回一致；重复执行 V300059 输出「已是 numeric，跳过」；`go test ./internal/meta/finv/quote/mvsv` 含 `TestParseNumericVolume` 通过；`go build ./...`、`go test ./internal/meta/finv/quote ./internal/meta/finv/quote/cdeu/kline` 通过；`HistoryQuote.md` volume 标注支持小数 | |

## 一百二十五、外部 OpenAPI 文档自包含（10793）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10793 | `Docs/OpenAPI/` 对外文档必须自包含：字段表、错误码、schema 全部内联，**不得引用 `Docs/API/` 内部文档或内部 `.yml`**；`ApiSpec §7.7` 增加该规则。 | `grep -rn '\.\./.*API/' Docs/OpenAPI/` 无输出；`grep -rn 'Docs/API/' Docs/OpenAPI/` 无输出；`grep -c 'API/Quote/Cdeu/Min/KLine' Docs/OpenAPI/Quote/Cdeu/Min/Upsert.yml` = 0；`Upsert.yml` 含 `KLineItem:` schema 且无跨文件 `$ref`；`Upsert.md` 含完整 17 字段表与 15 条错误码；`PageUni.md` 含 15 列定义与取值规则；`grep -c '内部/外部分离且自包含' Docs/DevSpec/ApiSpec.md` = 1；`go test ./internal/docs` 通过 | |

## 一百二十六、K 线字段改名与 last_close 落库（10794）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10794 | 分钟行情写入接口的 `paocd` 参数改为 `pacd`（库列仍 `paocd`）；`prev_close` 改为 `last_close` 并**落库**（不再仅回填 open），upsert/PATCH/查询/JSON 输出与文档同步；请求示例必须包含完整 17 字段，成功响应 JSON 格式化。 | `grep -c '"pacd"' internal/meta/finv/quote/cdeu/kline/request.go` = 1；`grep -c 'ColLastClose' internal/meta/finv/quote/cdeu/kline/model.go` ≥ 2；`go test ./internal/meta/finv/quote/cdeu/kline -run 'TestLastCloseCompat\|TestPatchLastClose\|TestRowJSON' -v` 通过；`grep -rn 'paocd\|prev_close' Docs/API/Quote/Cdeu/Min Docs/OpenAPI/Quote/Cdeu/Min` 仅剩「库列」说明；两份 `KLineUpsert.yml` 的 `KLineItem` 均为 17 字段且含 `pacd`/`last_close`；`go test ./internal/docs` 通过 | |

## 一百二十七、字段表类型/可空分列（10795）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10795 | 所有接口文档的字段表必须把「类型」与「可空」拆成两列，不得在类型列写 `X \| null`；嵌套可空用 `array<array<string>>` + 说明文字表达；`ApiSpec §7.7` 增加该约定。 | `grep -rn '\| null' Docs --include=*.md` 仅命中 `ApiSpec.md` 的规则示例；内部/外部 `KLineUpsert` 字段表表头含「类型」「可空」两列；`PageUni` 列定义同样拆分；`grep -c 'array<array<string>>' Docs/API/Quote/Cdeu/Min/KLinePageUni.md Docs/OpenAPI/Quote/Cdeu/Min/PageUni.md` ≥ 1；`go test ./internal/docs` 通过（表格列数一致） | |

## 一百二十八、Native 日志落盘（10796）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10796 | Native 方式支持把日志写入文件：`FINV_LOG_DIR`/`FINV_LOG_FILE` 控制路径（默认 `<PREFIX>/Log/FinvQuant.log`），标准库日志与 gin 访问日志同时写文件并保留 journald；安装脚本创建目录、渲染/补齐配置（切分/按日归档规则见 10797）。 | `grep -c 'FINV_LOG_DIR' internal/config/config.go Deploy/Native/FinvQuant.env.example Deploy/Native/Install.sh` 各 ≥ 1；`go test ./internal/logging -v` 通过（写文件、空目录 no-op、默认文件名）；`grep -c 'MultiWriter' internal/logging/logging.go` ≥ 2；安装后 `test -d /data/App/FinvQuant/Log` 且 `tail -f /data/App/FinvQuant/Log/FinvQuant.log` 有输出；`journalctl -u finvquant` 仍可用；`go build ./...` 通过 | |

## 一百二十九、日志切分与按日归档（10797）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10797 | 日志目录改为 `<PREFIX>/Log`，活动文件名 `FinvQuant.log`；单文件超过 10MB（`FINV_LOG_MAX_MB`）切分为 `FinvQuant-yyyyMMdd-NN.log`（NN 从 01 自增）；跨日把此前日期的切分文件打包为 `FinvQuant-yyyyMMdd.tar.gz` 并删除源 `.log`；启动时归档历史切分文件。 | `go test ./internal/logging -v` 通过（`TestRotateBySize` 断言 `FinvQuant-20261028-01.log`；`TestArchiveOnNextDay` 断言 `FinvQuant-20261028.tar.gz` 且 `.log` 已删除）；`grep -c 'FinvQuant-yyyyMMdd-NN.log\|FinvQuant-yyyyMMdd.tar.gz' internal/logging/logging.go` ≥ 2；`grep -c 'FINV_LOG_MAX_MB' internal/config/config.go Deploy/Native/FinvQuant.env.example` 各 = 1；`grep -rn 'Logs/' Deploy/Native` 无输出；安装后 `Log/FinvQuant.log` 存在；`go build ./...` 通过 | |

## 一百三十、cdeu 行情导出/导入工具方案（10798）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10798 | 产出 `cdeu.finv_quote_secu_kline_min` 导出/导入工具方案文档（不含实现）：命令设计、文件格式（CSV.gz/binary）、导入模式（append/upsert/replace）、一致性快照、校验与安全、性能预估、待确认决策点与实施计划。 | `test -f Docs/Refactor/2610-010-CdeuKlineDumpImport.md`；文档含「方案对比」「推荐方案设计」「待确认的决策点」「验收标准」「实施计划」章节且决策点 ≥ 8 项；过程文档编号 `2610-010` 全局唯一（`go test ./internal/docs -run TestProcessDocNumberUnique` 通过）；全仓 Markdown 链接 0 失效；`go test ./internal/docs` 通过 | |

## 一百三十一、qmio 数据搬运工具（10799）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10799 | 新增 `qmio` 数据搬运工具（与 `finvquant` 同目录 `<PREFIX>/bin/qmio`）：导出 `cdeu.finv_quote_secu_kline_min` 为 MVSV-2（默认 gzip、含 gmt、可 `--no-gmt`），导入默认 `upsert`（可选 append/replace，replace 需 `--force`），支持 `verify`（sha256/行数）；打包/安装/升级/卸载同步。 | `ls cmd/qmio`；`go build ./cmd/qmio` 通过；`go test ./internal/meta/finv/quote/mvsv -v` 含 `TestWriteV2RoundTripGmt/NoGmt` 通过；`grep -c 'qmio' Deploy/Script/PackSnapshot.sh Deploy/Native/Install.sh` 各 ≥ 1；临时 PG 导出→清空→导入→重复导入行数不变；`--no-gmt` 文件 `Field` 为 17 列且导入后 `gmt_create` 非空；`qmio verify` 通过；`go build ./...` 通过 | |

## 一百三十二、BRS 数据备份服务（10800）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10800 | 新增 `brs` schema 与 `brs.brs_bak_rdb_table` 备份登记表（`priority` 唯一）；`sys.sys_config 39001` 配置 6 段 cron；服务内每日按 `priority` 倒序对启用表执行 `pg_dump -Fc`（按 `compression` 压缩），备份到 `<PREFIX>/Backup/BRS/<yyyyMMdd>/`，只写日志；预置系统表与 CDEU 配置表种子（不含 `cdeu.finv_quote_secu_kline_min`）；对象存储通过 `internal/meta/base/oss` 抽象，首期不启用上传。 | `test -f Deploy/Migrations/V300060__brs_backup_table.sql`；临时 PG 执行全部 142 个迁移 0 错误，`select count(*) from brs.brs_bak_rdb_table` = 12，`select count(distinct priority)` = 12，`select str_value from sys.sys_config where config_no=39001` = `0 0 3 * * *`；`go test ./internal/meta/finv/brs ./internal/meta/base/cron ./internal/meta/base/oss/...` 通过；`go build ./...` 通过；`grep -c 'FINV_BRS_BACKUP_DIR' internal/config/config.go Deploy/Native/FinvQuant.env.example` 各 ≥ 1；`grep -rn 'minio-go' internal/` 无输出 | |

## 一百三十三、BRS 对象存储上传（10801）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10801 | BRS 二期：备份产物上传 S3 兼容对象存储（R2 等）。`brs_bak_rdb_table` 增加 `flag_upload`；`sys.sys_config 39002` 保存对象存储 JSON 配置（默认 `enabled=false`）；通过 `internal/meta/base/oss` 抽象 + `oss/s3` 适配器（`aws-sdk-go-v2`，**不使用 minio-go**），上传失败保留本地。 | `grep -c 'flag_upload' Deploy/Migrations/V300061__brs_upload.sql` ≥ 1；临时 PG 执行全部 143 个迁移 0 错误；`select json_value->>'enabled' from sys.sys_config where config_no=39002` = `false`；`go test ./internal/meta/base/oss/... ./internal/meta/finv/brs` 通过（含 s3 适配器 httptest 用例）；`grep -rn 'minio' internal/meta/base/oss internal/meta/finv/brs` 无官方库引用；`go build ./...` 通过 | |

## 一百三十四、BRS 对象存储配置说明（10802）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10802 | `sys.sys_config 39002` 的 `remark` 必须包含完整配置说明：字段含义、R2/AWS/MinIO 的 endpoint/region 差异、R2 API Token 获取步骤、示例 JSON、注意事项；不能只写一句话。`Brs.md` 同步配置步骤。 | `grep -c 'R2 示例' Deploy/Migrations/V300061__brs_upload.sql` ≥ 1；临时库执行后 `select length(remark) from sys.sys_config where config_no=39002` ≥ 500；`select remark from ...` 含 `enabled`、`endpoint`、`R2`、`flag_upload`；`go test ./internal/docs` 通过 | |

## 一百三十五、Web 左侧菜单四级化（10803–10807）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10803 | 左侧菜单改为四级树：一级为系统，其下为二/三/四级菜单；既有 29 个业务菜单按「旧层级 +1」映射到新层级（不新增业务菜单、不改路由）。 | `grep -c "key: '" Web/src/config/menuConfig.ts` ≥ 37；人工核对「数据采集 → 行情数据归档 → 归档统计与预算」上下共 4 级；`grep -c 'v-list-group' Web/src/App.vue` 为 0 | |
| 10804 | 顶栏提供一级系统切换器（金融投资 / 内容管理，内部 key `finvquant` / `cms`），切换后左侧菜单联动并回到该系统上次访问路由；「金融投资」即 CMS 一级菜单中的金融投资分系统。 | `grep -F "key: 'finvquant'" Web/src/config/menuConfig.ts` 命中；`grep -F "key: 'cms'" Web/src/config/menuConfig.ts` 命中；`grep -F "path: '/Meta/Finv', redirect: '/Dashboard'" Web/src/router.ts` 命中；人工切换后左侧菜单随之变化 | |
| 10805 | 内容管理系统本轮仅提供缺省占位：一级入口「内容管理」+ 二级叶子「内容管理控制台」（`/Meta/CMS`），页面为占位页且不实现业务功能。 | `grep -F "path: '/Meta/CMS'" Web/src/router.ts` 命中；`grep -F "'cms-home'" Web/src/views/Common/PlaceholderView.vue` 命中；人工点击后显示「规划中」占位页 | |
| 10806 | 菜单展开状态按系统分桶持久化到 localStorage（键 `finvquant.nav.openKeys.v1`），深链接直开时自动展开祖先链；脏数据回退到激活链且不报错。 | `grep -F 'finvquant.nav.openKeys.v1' Web/src/config/uiConfig.ts` 命中；人工：展开「元数据管理」→切到内容管理→切回，展开状态保留；直接访问 `/Meta/Finv/Quote/DeDup/Export/Stats` 时祖先链自动展开 | |
| 10807 | 导航视觉固定为 WebCMS 风格：当前菜单项左侧 3px 主色激活条 + 祖先分组标题主色；**不提供风格开关，也不使用 `?nav=` 参数**。抽屉顶部不展示系统标题块、内容区不展示面包屑（2026-10-29 已移除）。 | `grep -c 'navVisualStyle\|toggleNavVisualStyle\|WebCMS 视觉风格' Web/src Web/dist/assets/index-*.js` 为 0；`grep -c 'WebCMS 视觉风格' Web/dist/assets/index-*.js` 为 0；人工：用户菜单只有修改密码/退出登录，菜单激活项有 3px 主色条 | |

## 一百三十六、V300057 迁移兼容 dedup 缺失（10808）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10808 | `V300057__sync_cdeu_config_from_dedup.sql` 在 `dedup` schema 不存在时必须幂等跳过（输出 NOTICE），不得因 `dedup.cdeu_process` 不存在而中断迁移链。 | `grep -c "nspname = 'dedup'" Deploy/Migrations/V300057__sync_cdeu_config_from_dedup.sql` ≥ 1；在无 dedup schema 的库上重建容器后：`docker logs finvquant` 含 `dedup schema 不存在，跳过配置同步`，`select max(version) from schema_version` = `200061`，`/API/V1/Health/Ready` 返回 200 | |

## 一百三十七、登录后页面页脚（10809）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10809 | 登录后每个页面底部显示居中页脚 `© 2026 ACANX`；内容较短时贴底；登录页不显示。 | `grep -F 'finv-page-footer' Web/src/App.vue` 命中；`grep -F '© 2026 ACANX' Web/dist/assets/index-*.js` 命中；人工：仪表盘页底部可见页脚，登录页无页脚 | |

## 一百三十八、深色/浅色主题切换（10810–10811）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10810 | 用户可在右上角用户菜单内切换深色/浅色主题，选择保存在浏览器 `localStorage['finvquant.ui.theme']`；初始主题按「本地选择 > 系统偏好 > 深色」取。 | `grep -F "finvquant.ui.theme" Web/src/config/uiConfig.ts` 命中；`grep -F "resolveInitialTheme" Web/src/main.ts Web/src/config/uiConfig.ts` 各命中；人工：切换后全站配色立即变化、刷新后保持 | |
| 10811 | **待办（未实施）**：主题偏好做服务端持久化（存 `sys.sys_user` 或 `sys_config`，登录后跨设备同步）。 | 未实施；落地时须补迁移、接口与文档 | |

## 一百三十九、CDEU 前端路由调整（10812）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10812 | CDEU 页面路由由 `/Meta/Finv/Quote/DeDup/CDEU`、`/Meta/Finv/Quote/DeDup/CDEUWorkflow` 调整为 `/Meta/Finv/Quote/Cdeu/CdeuOverview`、`/Meta/Finv/Quote/Cdeu/CdeuWorkflow`；菜单项与页面内「全流程拓扑」跳转同步；后端 API 路径（`/API/V1/Meta/Finv/Quote/DeDup/CDEU/*`）保持不变。 | `grep -c '/Meta/Finv/Quote/Cdeu/CdeuOverview' Web/src/router.ts Web/src/config/menuConfig.ts` 各 ≥ 1；`grep -c '/Meta/Finv/Quote/Cdeu/CdeuWorkflow' Web/src/router.ts Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 各 ≥ 1；`grep -c "Quote/DeDup/CDEU'" Web/src/router.ts` 为 0；`cd Web && npm run build` 通过 | |

## 一百四十、CDEU 归档工作流设计文档（10813）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10813 | 在 `Docs/Design/` 下新增 CDEU（采集 → 去重 → 导出 → 上传）归档工作流设计文档，体例对齐 AHA `Docs/Design`（元信息 + 变更日志 + 编号章节 + 表格化结论）：覆盖五级层级模型、状态机与完成条件、六个阶段、17 个 Process、22 个 JOB、15 个静态 Step 的「输入 / 处理 / 输出 / 解决的问题」，以及数据契约、系统配置项、阈值清单、运维排障与 `CdeuOverview`、`CdeuWorkflow` 页面映射。 | `test -f Docs/Design/CdeuWorkflowDesign.md`；`grep -c '^## ' Docs/Design/CdeuWorkflowDesign.md` ≥ 10；`grep -o 'JOB_[A-Z_]*' Docs/Design/CdeuWorkflowDesign.md \| sort -u \| wc -l` ≥ 20；`go test ./internal/docs` 通过 | |

## 一百四十一、finv_security 扩展 type_secu_sub / provider / remark 三列（10814）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10814 | `public.finv_security` 增加 `type_secu_sub`、`provider`、`remark` 三列，均为 `TEXT NOT NULL DEFAULT ''`，并带列注释与末尾收敛校验（列存在性/类型/可空/默认值）。 | `test -f Deploy/Migrations/V200082__finv_security_add_type_secu_sub_provider_remark.sql`；文件内 `ADD COLUMN IF NOT EXISTS type_secu_sub`、`provider`、`remark` 三处均在；`grep -c 'RAISE EXCEPTION' Deploy/Migrations/V200082__finv_security_add_type_secu_sub_provider_remark.sql` ≥ 4 | |

## 一百四十二、Supabase 证券视图下载到本地中转目录（10815）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10815 | 提供 Python 下载脚本，把 Supabase `finv_quote_collect_futu_view` 中 `flag_enable='1'` 的记录以 JSONL（一行一条）落盘到 `FinvQuantDesign/Data/Secu/`，并写 manifest（上游总数、实拉行数、usc 唯一数、sha256）；分页采用游标方式并经上游 count 核对。 | `test -f ../FinvQuantDesign/Scripts/SecuSync/DownloadSecu.py`；`ls ../FinvQuantDesign/Data/Secu/finv_quote_collect_futu_view_*.jsonl` 命中；`ls ../FinvQuantDesign/Data/Secu/finv_quote_collect_futu_view_*.manifest.json` 命中；`grep -c 'gt\.' ../FinvQuantDesign/Scripts/SecuSync/DownloadSecu.py` ≥ 1 | |

## 一百四十三、富途 market_code 转换与幂等插入脚本生成（10816）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10816 | 提供 Python 转换脚本，把上游记录的富途 `market_code` 转换为 `finv_market.market_code`（非一一对应，由映射配置完成）后生成 `finv_security` 插入脚本；脚本对已有记录不更新（`ON CONFLICT DO NOTHING`），新行 `flag_enable='0'`、`provider='FTMM'`、`remark='20261030-SupabasePG-finv_quote_collect_futu_view批量导入'`。 | `test -f ../FinvQuantDesign/Scripts/SecuSync/ExportSecu.py`；`test -f ../FinvQuantDesign/Scripts/SecuSync/SecuMapping.json`；`grep -c 'ON CONFLICT DO NOTHING' ../FinvQuantDesign/Data/Secu/Out/finv_security_insert_*.sql` ≥ 1；`grep -c "'20261030-SupabasePG-finv_quote_collect_futu_view批量导入'" ../FinvQuantDesign/Data/Secu/Out/finv_security_insert_*.sql` ≥ 1；`grep -c "'FTMM'" ../FinvQuantDesign/Data/Secu/Out/finv_security_insert_*.sql` ≥ 1 | |

## 一百四十四、港股人民币柜台货币口径修正（10817）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10817 | 港股人民币柜台（港交所 8 开头 5 位代码，`security_type` 非指数/波动率/债券/商品指数类）的 `currency_type` 一律为 `CNY`；6 位 8 开头港元指数保持 `HKD`；导入脚本与已入库数据同步修正。 | `grep -c 'hkRmbCounter' ../FinvQuantDesign/Scripts/SecuSync/SecuMapping.json` ≥ 1；`test -f ../FinvQuantDesign/Scripts/SecuSync/FixSecuCurrency.py`；库内校验：`select count(*) from public.finv_security where exchange_code=21 and security_code ~ '^8[0-9]{4}$' and security_type not in ('StockIndex','VolatilityIndex','BondIndex','CommodityIndex') and currency_type <> 'CNY'` 为 0；且 `select count(*) from public.finv_security where exchange_code=21 and security_code ~ '^8[0-9]{5}$' and currency_type <> 'HKD'` 为 0 | |

## 一百四十五、富途原始属性镜像表 finv_futu_quote_collect（10818）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10818 | 在 `futu` schema 新建 `finv_futu_quote_collect` 表（`usc` 主键，含 `stock_id`/`futu_symbol`/`market_type`/`market_code`/`instrument_type`/`sub_instrument_type`/`req_section`/`quote_market`/`type_symbol`/`flag_enable`/`remark`/`gmt_create`/`gmt_update`），并支持从上游快照初始化与增量同步（幂等 upsert，可仅同步新增/变更行）。 | `test -f Deploy/Migrations/V200083__create_futu_finv_futu_quote_collect.sql`；`test -f ../FinvQuantDesign/Scripts/SecuSync/BuildFutuQuoteCollect.py`；`select count(*) from public.schema_version where version='100083'` 为 1；`select count(*) from information_schema.columns where table_schema='futu' and table_name='finv_futu_quote_collect'` 为 14；`select count(*) from futu.finv_futu_quote_collect` 为 17138 | |

## 一百四十六、初始数据发放以迁移脚本形式入库（10819）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10819 | `finv_security` 与 `futu.finv_futu_quote_collect` 的初始数据以种子/数据迁移形式纳入 `Deploy/Migrations`（`V300062` / `V300063`，均 `ON CONFLICT DO NOTHING`），并将港股人民币柜台币种修正固化为 `V300064`；全新库按版本顺序执行后两表行数分别为 17,199 / 17,138。 | `ls Deploy/Migrations/V20006[234]__*.sql` 命中3 个；`grep -c 'ON CONFLICT DO NOTHING' Deploy/Migrations/V300062__finv_security_seed_supabase.sql` ≥ 35；`grep -c 'ON CONFLICT DO NOTHING' Deploy/Migrations/V300063__finv_futu_quote_collect_seed_supabase.sql` ≥ 35；`select count(*) from public.schema_version where version in ('200062','200063','200064')` 为 3；`select count(*) from futu.finv_futu_quote_collect` 为 17138 | |

## 一百四十七、日增量行数模板与 day_expect_rows 自动补全（10820）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10820 | 新建日增量行数模板表 `cdeu.finv_quote_cdeu_usc_day_incr_row_tpl`（主键 `(region, market, category)`，三列均可为空串表示通配，含 `day_expect_rows` > 0 约束与 `gmt_update` 触发器）并预置市场模板；自动流程发现/刷新证券元数据时，若 `cdeu.finv_quote_cdeu_usc.day_expect_rows` 为空则按模板（特异性降序、模板未命中则不补）补全，已配置的非空值一律不覆盖。 | `test -f Deploy/Migrations/V200084__create_cdeu_usc_day_incr_row_tpl.sql`；`test -f Deploy/Migrations/V300065__cdeu_usc_day_incr_row_tpl_seed.sql`；`select count(*) from information_schema.columns where table_schema='cdeu' and table_name='finv_quote_cdeu_usc_day_incr_row_tpl'` 为 8；`select count(*) from cdeu.finv_quote_cdeu_usc_day_incr_row_tpl where flag_enable='1'` ≥ 10；`grep -c 'func (s *Store) BackfillUscDefaults' internal/meta/finv/quote/dedup/store.go` 为 1；`grep -c 'BackfillUscDefaults' internal/meta/finv/quote/dedup/job/refresh_quote_maturity.go internal/meta/finv/quote/dedup/job/export.go` 各 ≥ 1；库内验证：两轮 JOB 后 `select count(*) from cdeu.finv_quote_cdeu_usc u where u.day_expect_rows is null and exists (select 1 from cdeu.finv_quote_cdeu_usc_day_incr_row_tpl t where t.flag_enable = '1' and (t.region = '' or upper(t.region) = upper(coalesce(u.region, ''))) and (t.market = '' or upper(t.market) = upper(coalesce(u.market, ''))) and (t.category = '' or upper(t.category) = upper(coalesce(u.category, ''))))` 为 0 | |

## 一百四十八、自动发现时补全优先级 / SLA / 最大等待默认值（10821）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10821 | 自动发现新证券、刷新证券元数据时，`priority_rank` / `sla_days` / `max_wait_days` 三列若同时为 NULL（未配置）则统一补默认值 100 / 5 / 7，使 CDEU 流程无需人工预配置即可运行；人工设置过的值不被覆盖，且自动补全不写 `priority_gmt_update`（该列为 NULL 即表示默认值）。 | `grep -c 'defaultPriorityRank = 100' internal/meta/finv/quote/dedup/store.go` 为 1；`grep -c 'priority_rank IS NULL AND sla_days IS NULL AND max_wait_days IS NULL' internal/meta/finv/quote/dedup/store.go` 为 1；库内验证：`select count(*) from cdeu.finv_quote_cdeu_usc where priority_rank is null and sla_days is null and max_wait_days is null` 在两轮 JOB 后为 0；`go test ./internal/meta/finv/quote/dedup/` 通过（`TestBackfillUscDefaults` 在配备测试库时额外验证默认值/不覆盖/幂等） | |

## 一百四十九、Native 安装脚本 qmio 定位修正（10822）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10822 | `Deploy/Native/Install.sh` 能从发行包根目录（`qmio` 与 `finvquant` 并列）找到 qmio 并安装到 `<PREFIX>/bin/qmio` 与 `/usr/local/bin/qmio`，不再在标准包布局下报「未找到 qmio」警告；保留对历史布局（`Deploy/qmio`、`Deploy/Native/qmio`）的兼容。 | `bash -n Deploy/Native/Install.sh` 通过；`grep -c 'REPO_ROOT}/qmio' Deploy/Native/Install.sh` ≥ 1；构造 `FinvQuant-vX-ubuntu-arm64/{finvquant,qmio,Deploy/Native/Install.sh}` 后以 `SCRIPT_DIR=<pkg>/Deploy/Native`、`REPO_ROOT=<pkg>` 执行候选定位循环，`QMIO_SRC` 命中 `<pkg>/qmio` | |

## 一百五十、证券自动发现后的元数据与默认值补全优化（10823）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10823 | 自动发现的 `cdeu.finv_quote_cdeu_usc` 行必须能自动补齐 `category` / `region` / `market` / `name` / `timezone` / `currency` / `day_expect_rows`：① 回填 `finv_security` 时**不限 `flag_enable`**（禁用不等于无行情）；② `finv_security` 未收录时从 `futu.finv_futu_quote_collect` 兜底（`quote_market` → region/market、`type_symbol` → category）；③ 按 region 推导 timezone/currency；④ `SH######` / `SZ######` 前缀兜底为 A 股指数；⑤ 模板表补区域级兜底行，`market` 未维护时仍能给出 `day_expect_rows`；⑥ 补全后剩余缺口写日志（字段行数 + 证券样本），不静默失败。 | `grep -c 'FROM public.finv_security s' internal/meta/finv/quote/dedup/store.go` ≥ 1 且该 SQL 中**无** `flag_enable` 过滤；`grep -c 'finv_futu_quote_collect' internal/meta/finv/quote/dedup/store.go` ≥ 1；`grep -c 'func (s \*Store) UscMissingSample' internal/meta/finv/quote/dedup/store.go` 为 1；`test -f Deploy/Migrations/V300066__cdeu_usc_day_incr_row_tpl_region_fallback.sql`；`select count(*) from cdeu.finv_quote_cdeu_usc_day_incr_row_tpl` ≥ 17；库内验证：导入 497 行 usc 后跑一轮 `REFRESH_QUOTE_MATURITY`，`select count(*) from cdeu.finv_quote_cdeu_usc where day_expect_rows is null` ≤ 7（仅 `finv_security` 与富途镜像表都未收录的证券） | |

## 一百五十一、上传 JOB 日志可观测性与异常一致性（10824）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10824 | `UPLOAD_PENDING_EXPORT_FILES` 的执行日志必须能定位到具体环节：每个文件记录本地文件路径、登记/实际校验值、上传目标（网站 + 组织/用户 + 仓库 + 分支 + 目标路径 + 完整 URL）、远端 HEAD 结论、PUT 字节与耗时、远端校验结果；失败项以 `ERROR` 级别写入运行日志并附失败环节与下次重试时间；有失败时运行汇总的 `error` 字段与末条日志必须给出「首个失败 file_id 与原因」，与页面「异常」列一致；GitHub 响应解析失败必须带 HTTP 状态、URL、Content-Type/Content-Length、响应体片段与请求 ID。 | `grep -c 'describeUploadTarget' internal/meta/finv/quote/dedup/job_upload.go` ≥ 2；`grep -c 'reportRunLog(finishCtx, "ERROR"' internal/meta/finv/quote/dedup/job_upload.go` 为 1；`grep -c 'first_failed_err' internal/meta/finv/quote/dedup/job/upload_pending_export_files.go` ≥ 2；`grep -c 'responseDecodeError' internal/meta/finv/quote/dedup/uploader_github.go` ≥ 2；`go test ./internal/meta/finv/quote/dedup/ -run TestDescribeUploadTarget` 通过；库内验证：一轮上传（含一个失败文件）后 `select count(*) from cdeu.finv_quote_cdeu_job_run_log_line where run_id=<id> and level='ERROR'` ≥ 1，且 `select error from cdeu.finv_quote_cdeu_job_run_log_export_file where run_id=<id>` 含「本轮 N 个文件上传失败」 | |

## 一百五十二、GitHub 大文件上传与远端去重口径修正（10825）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10825 | GitHub 上传必须能处理大于 1MB 的归档文件：查询远端文件改走 Git Trees API（只取 blob sha 与 size，不下载 base64 内容），失败时回退 Contents API 且读取上限 ≥ 32MB、超限报明确错误；Job 层「上传前去重」必须按远端校验口径判断（github 为 git blob sha1，不可与本地 SHA256 混比）；上传动态 JOB 必须写入 `run_log_id`（含历史实例补齐与查询兜底），保证页面「执行日志」入口不消失。 | `grep -c 'git/trees/' internal/meta/finv/quote/dedup/uploader_github.go` ≥ 1；`grep -c 'githubContentsReadLimit' internal/meta/finv/quote/dedup/uploader_github.go` ≥ 2；`grep -c 'RemoteChecksumAlgorithm' internal/meta/finv/quote/dedup/uploader.go internal/meta/finv/quote/dedup/uploader_github.go` 各 ≥ 1；`grep -c 'run_log_id' internal/meta/finv/quote/dedup/store.go` ≥ 4；`go test ./internal/meta/finv/quote/dedup/ -run 'TestGitHubStatUsesTreeAPI\|TestSplitRepoPath'` 通过；库内验证：一轮上传后 `select count(*) from cdeu.finv_quote_cdeu_job_dynamic where job_category='UPLOAD' and run_log_id is null` 为 0 | |

## 一百五十三、补齐 day_expect_rows 后重算历史成熟度（10826）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10826 | 自动补全 `cdeu.finv_quote_cdeu_usc.day_expect_rows` 后，必须同步重算这些证券的历史成熟度行 `expected_rows` 与 `maturity_status`（非交易日=0/COMPLETE、无期望值=PARTIAL、`actual_rows >= day_expect_rows × 阈值` = COMPLETE，`FROZEN` 不改写），使页面「期望行数」不再停在 0；只处理刚补全的证券，不整表重扫。 | `grep -c 'func (s \*Store) recomputeMaturityExpected' internal/meta/finv/quote/dedup/store.go` 为 1；`grep -c 'recomputeMaturityExpected' internal/meta/finv/quote/dedup/store.go` ≥ 2；`grep -c 'RETURNING u.usc' internal/meta/finv/quote/dedup/store.go` 为 1；`go test ./internal/meta/finv/quote/dedup/ -run TestBackfillUscDefaults`（需 `FINV_TEST_PG_DSN`）通过；库内验证：将某证券 `day_expect_rows` 置空并使其成熟度行 `expected_rows=0`，跑一轮 `REFRESH_QUOTE_MATURITY` 后 `select expected_rows from cdeu.finv_quote_cdeu_quote_maturity_mon where usc=<该证券>` 等于模板值（不再是 0） | |

## 一百五十四、补全流程不得覆盖人工清空的优先级（10827）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10827 | 重复执行证券补全流程时，`priority_rank` / `sla_days` / `max_wait_days` 只能对「从未配置过」（三列全空且 `priority_gmt_update IS NULL`）的证券补默认值 100/5/7；`priority_gmt_update` 非空的空值行表示人工清空，不得再补；同时补全方法在无缺口时应快速返回（一次 count 判定，不执行后续大表 JOIN/UPDATE）。 | `grep -c 'AND priority_gmt_update IS NULL' internal/meta/finv/quote/dedup/store.go` ≥ 2（阶段 SQL 与缺口判定各一处）；`grep -c '稳态快速返回' internal/meta/finv/quote/dedup/store.go` 为 1；库内验证：将某证券三列清空并置 `priority_gmt_update = now()`，跑一轮 `REFRESH_QUOTE_MATURITY` 后该行三列仍为 NULL；将另一证券三列清空且 `priority_gmt_update = NULL`，同一轮后补成 100/5/7；再跑一轮 JOB 日志出现「优先级 0」 | |

## 一百五十五、业务月份改为自动生成的下拉列表（10828）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10828 | 「业务月份」不再手工输入，改为按年月自动生成的下拉列表（默认最近 10 年、含当前月、最新在前）；交易日历、成熟度查询、导出文件筛选、导出任务筛选四处统一使用；下拉可清空且不预选（空值 = 不按月筛选）。 | `grep -c 'export function bizMonthOptions' Web/src/utils.ts` 为 1；`grep -rc 'bizMonthOptions' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveConfigView.vue Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveStatsView.vue Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportFileView.vue Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportJobView.vue` 各 ≥ 2（import + 引用）；四文件内不再出现 `v-text-field ... label="业务月份"`；`cd Web && npx vue-tsc -b` 与 `npm run build` 通过 | |

## 一百五十六、CDEU 阶段按拓扑顺序执行（10829）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10829 | CDEU 的 6 个阶段（`COLLECT → DEDUP → DECISION → EXPORT → UPLOAD → REVIEW`）必须按拓扑顺序执行：调度器触发某阶段 JOB 前，若同一天活动运行中 `seq_no` 更小、`enabled` 且 `affects_completion=true` 的前序阶段「从未开始」或「已失败」，本轮不得触发；被拦时不得写「今天已跑」标记（前序完成后自动继续）；持续型前序阶段（已在跑）与旁路阶段、手动触发不受限制。 | `grep -c 'func (s \*Store) CDEUPhaseGateForJob' internal/meta/finv/quote/dedup/cdeu_store.go` 为 1；`grep -c 'CDEUJobGate' internal/meta/finv/quote/dedup/scheduler.go` ≥ 1 且门禁判断在 `s.lastRun[runtime.Kind] = now` 之前；`grep -c 'noteGateBlocked' internal/meta/finv/quote/dedup/scheduler.go` ≥ 2；`go test ./internal/meta/finv/quote/dedup/ -run TestCDEUPhaseGate`（需 `FINV_TEST_PG_DSN`）通过；库内验证：造活动运行并把 `PHASE_COLLECT` 置 PENDING（`started_at` 为空）后，`PHASE_DECISION` 的 JOB 调度日志出现「暂不执行：前序阶段 PHASE_COLLECT 尚未完成」 | |

## 一百五十七、CDEU 阶段内流程与流程内 JOB 按顺序执行（10830）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10830 | 除跨阶段门禁外，调度触发还需约束另外两层顺序：同一阶段内 `seq_no` 更小的流程未完成/已失败时，后序流程的 JOB 不得触发；同一流程内 `seq_no` 更小的 JOB 未完成/已失败时，后序 JOB 不得触发。门禁按 `job_kind` 匹配定义（与 `CDEUJobDefinitions` 一致）；一个 kind 覆盖多个 JOB 定义的编排型 Job（如 `RUN_DUE_EXPORT_JOBS` 覆盖 6 个）只受阶段级门禁约束，避免死锁。 | `grep -c "'PHASE' AS level" internal/meta/finv/quote/dedup/cdeu_store.go` 为 1；`grep -c "'PROCESS'" internal/meta/finv/quote/dedup/cdeu_store.go` ≥ 1；`grep -c "'JOB', j2.job_code" internal/meta/finv/quote/dedup/cdeu_store.go` 为 1；`grep -c 'kind_jobs' internal/meta/finv/quote/dedup/cdeu_store.go` ≥ 3；`grep -c 'j.job_kind = \$1' internal/meta/finv/quote/dedup/cdeu_store.go` 为 1；`go test ./internal/meta/finv/quote/dedup/ -run TestCDEUJobGate`（需 `FINV_TEST_PG_DSN`）通过 7 个场景 | |

## 一百五十八、动态 JOB 领取顺序：优先级为主、seq_no 为次序（10831）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10831 | 同一流程下同类型动态 JOB（导出 / 上传）的领取顺序：**优先级为主序**（导出 `FROZEN` 紧急度 → `priority_rank` → `oldest_pending_date`；上传无优先级维度则直接看拓扑），**同优先级时按 CDEU 定义的 `cdeu.finv_quote_cdeu_job_dynamic.seq_no` 决定先后**；不引入新的配置开关。 | `grep -c 'dj.seq_no' internal/meta/finv/quote/dedup/store.go` ≥ 1；`grep -c 'dj.seq_no' internal/meta/finv/quote/dedup/store_files.go` ≥ 1；`grep -c '2147483647' internal/meta/finv/quote/dedup/store.go internal/meta/finv/quote/dedup/store_files.go` 各 ≥ 1；库内验证（事务内造数后回滚）：同 `priority_rank`/`oldest_pending_date` 的两条任务 `seq_no=2/1` → 领取到 `seq_no=1`；把 `seq_no=2` 那条的 `priority_rank` 改成 1 → 领取到高优先级那条 | |

## 一百五十九、导出决策明细 JOB 与 provider 缺口修复（10832）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10832 | 导出决策阶段必须有一个 JOB 逐条说明「哪些证券月能导出、哪些不能、为什么不能」，不得只给汇总数量：不能导出时给出具体原因（缺失的元数据字段名、退市日期、当月、未成熟的交易日列表、已导出的 job_id 与文件名）；能导出时给出应有行数、实际行数、目标文件名、本地路径、远端路径；另需列出「行情表有数据但成熟度表无记录」的盲区月份。同时修复 `cdeu.finv_quote_cdeu_usc.provider` 恒空导致全部证券月被判「元数据不全」的缺口（有富途采集记录的证券补 `FTMM`）。 | `grep -c 'JobExplainExportDecision = "EXPLAIN_EXPORT_DECISION"' internal/meta/finv/quote/dedup/model.go` 为 1；`grep -c 'func (s \*Service) ExplainExportDecision' internal/meta/finv/quote/dedup/service_decision.go` 为 1；`grep -c 'func (s \*Service) DiagnoseEmptyExportBatch' internal/meta/finv/quote/dedup/service_decision.go` 为 1；`grep -c 'provider   = .FTMM.' internal/meta/finv/quote/dedup/store.go` ≥ 1；`go test ./internal/meta/finv/quote/dedup/ -run TestClassifyExportDecision` 通过 9 个场景；`go test ./internal/meta/finv/quote/dedup/job/ -run TestExplainExportDecisionEndToEnd`（需 `FINV_TEST_PG_DSN`）通过并且运行日志表 `cdeu.finv_quote_cdeu_job_run_log_line` 里该 run_id 存在 `message LIKE '%导出决策%'` 的明细行；`grep -c 'func (s \*Service) AppendJobLog' internal/meta/finv/quote/dedup/job_port.go` 为 1；库内 `SELECT count(*) FROM cdeu.finv_quote_cdeu_usc WHERE provider IS NULL AND EXISTS (SELECT 1 FROM futu.finv_futu_quote_collect f WHERE f.usc = cdeu.finv_quote_cdeu_usc.usc)` 为 0；服务启动日志含 `调度 EXPLAIN_EXPORT_DECISION` | |

## 一百六十、删除 dedup / dedup_frozen schema（10834）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10834 | 删除已淘汰的 `dedup` 与 `dedup_frozen` 两个 schema（`cdeu` 已完整接管），删除前必须做逻辑备份；同时建立代码闸门，禁止任何代码再引用这两个 schema 前缀（含集成测试里的 `CREATE SCHEMA` 误复活）。 | `psql -c "select nspname from pg_namespace where nspname in ('dedup','dedup_frozen')"` 返回 0 行；`ls Deploy/Backup/dedup_schemas_*.sql.gz` 存在且 `zcat` 后 `grep -c '^CREATE TABLE'` 为 32；`grep -c 'forbiddenSchemaPrefixes' internal/database/schema_qualify_test.go` ≥ 2；`grep -rn 'dedup\\.finv_quote_secu_kline_min' --include=*.go .` 无业务代码命中；`go test ./internal/database/ -run TestSQLSchemaQualified` 通过；`select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where c.relname='finv_quote_secu_kline_min' and n.nspname in ('quote','export','cdeu')` 为 3 | |

## 一百六十一、行情数据归档·月份存量清单（10835）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10835 | 「行情数据归档」菜单下新增「月份存量清单」：按业务月份 + 证券维度统计主表该月的存量，给出每个证券的存量行数、交易日数、成熟度应有/实际行数、导出状态、文件名（已生成或预计）与预计大小，并汇总证券数/总行数/预计总大小；主表已清理（已导出）的证券必须明确标注而不是显示为数据缺失。 | `grep -c '历史月份存量数据清点' Web/src/config/menuConfig.ts` 为 1（菜单名见需求 10838）；`grep -c 'Export/Stock/Page' internal/api/router.go` 为 1；`grep -c 'func (s \*Service) StockPage' internal/meta/finv/quote/dedup/service_decision.go` 为 1；`grep -c 'func (s \*Store) ExportStockPage' internal/meta/finv/quote/dedup/store_decision.go` 为 1；`go test ./internal/meta/finv/quote/dedup/ -run TestExportStockPage`（需 `FINV_TEST_PG_DSN`）通过并输出「已清理」计数；`grep -rn 'dedup\.' internal/meta/finv/quote/dedup/store_decision.go` 无命中（数据源只用 `cdeu.*`） | |

## 一百六十二、Download.sh 回显包内 commit（10836）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10836 | `Download.sh` 下载完成后必须回显该包对应的 commit：从包内 `BUILD-INFO.txt` 读取「提交」行（无需解压），`--no-extract` 与最终摘要都要打印；包内无该文件时只告警不中断。 | `grep -c 'read_package_commit' Deploy/Script/Download.sh` ≥ 3；`bash -n Deploy/Script/Download.sh` 通过；造含 `BUILD-INFO.txt` 的 `tar.gz` 与 `zip` 各一个，提取该函数执行后输出 commit 且与 `BUILD-INFO.txt` 中一致；对不存在的包与缺 `BUILD-INFO.txt` 的包，函数返回非 0 且不报错退出；`grep -c '包内提交号' Deploy/Script/README.md` 为 1 | |

## 一百六十三、证券元数据 JOB 归入导出批次流程（10837）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10837 | `JOB_FETCH_SECURITY_METADATA` 必须挂在 `PROCESS_EXPORT_BATCH`（导出批次清单获取流程）下执行，且排在 `JOB_FETCH_EXPORT_BATCH` 之后（`seq_no=2`）；原所在的 `PROCESS_EXPORT_FILE_MON_BASE` 剩余 JOB 的 `seq_no` 重排为连续且无空洞。 | `psql -c "select p.process_code, j.seq_no, j.job_code from cdeu.finv_quote_cdeu_job j join cdeu.finv_quote_cdeu_process p on p.process_id=j.process_id where p.process_code in ('PROCESS_EXPORT_BATCH','PROCESS_EXPORT_FILE_MON_BASE') order by 1,2"` 输出中 `PROCESS_EXPORT_BATCH` 为 1=JOB_FETCH_EXPORT_BATCH、2=JOB_FETCH_SECURITY_METADATA，`PROCESS_EXPORT_FILE_MON_BASE` 为连续 1..4；重复执行 `V200087` 打印「已在 PROCESS_EXPORT_BATCH，跳过（幂等）」且结果不变；`go test ./internal/meta/finv/quote/dedup/ -run TestCDEUJobGate`（需 `FINV_TEST_PG_DSN`）7 场景通过；「工作流控制与监控」页的 JOB 清单表格只出现一条水平滚动条：`grep -c 'job-table-scroll :deep(.v-table__wrapper)' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 为 1 且该规则为 `overflow-x: visible` | |

## 一百六十四、菜单改名与端点调整（10838）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10838 | 「行情数据归档」下的「月份存量清单」改名为「历史月份存量数据清点」，菜单端点由 `/Meta/Finv/Quote/DeDup/Export/Stock` 调整为 `/Meta/Finv/Quote/Cdeu/HisMonthExistSecuStat`。 | `grep -c "title: '历史月份存量数据清点'" Web/src/config/menuConfig.ts` 为 1；`grep -c '/Meta/Finv/Quote/Cdeu/HisMonthExistSecuStat' Web/src/router.ts Web/src/config/menuConfig.ts` 各 ≥ 1；`grep -rn '/Meta/Finv/Quote/DeDup/Export/Stock' Web/src` 无页面路由命中（仅后端 API 调用路径保留，见说明）；`npx vue-tsc --noEmit` 通过 | |

## 一百六十五、菜单新增二级「行情」并移动两个叶子（10839）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10839 | 一级系统「金融投资」下新增与「数据采集」同级的二级菜单「行情」（前缀 `/Meta/Finv/Quote/`），其下两个三级子菜单：「收剔导传ETL」（前缀 `/Meta/Finv/Quote/Cdeu/`）与「历史行情」（前缀 `/Meta/Finv/Quote/His/`）；「历史月份存量数据清点」（`/Meta/Finv/Quote/Cdeu/HisMonthExistSecuStat`）移入「收剔导传ETL」，「历史行情查询」（`/Meta/Finv/Quote/History/HistoryQuoteQuery`）移入「历史行情」，原二级「历史行情」分组删除。 | `grep -c "title: '行情'" Web/src/config/menuConfig.ts` 为 1；`grep -c "title: '收剔导传ETL'" Web/src/config/menuConfig.ts` 为 1；`grep -c "key: 'history-quote'" Web/src/config/menuConfig.ts` 为 0；`grep -c "key: 'cdeu-his-month-exist-secu-stat'" Web/src/config/menuConfig.ts` 为 1 且出现在 `quote-cdeu` 分组内；菜单内 path 与 `Web/src/router.ts` 的路由集合比对无缺失、无重复 key；菜单全部 `mdi-*` 图标名都能在 `Web/node_modules/@mdi/font/css/materialdesignicons.css` 的 `.mdi-xxx::before` 中找到（`mdi-chart-finance` 曾写错导致「行情」无图标，已改为 `mdi-chart-multiline`）；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百六十六、菜单改名与顺序调整（10840）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10840 | 二级菜单「元数据管理」改名为「元数据」，位置移到「仪表盘」之后；「数据采集」移到「配置管理」之后。节点 key 不变。 | 解析 `Web/src/config/menuConfig.ts` 的 `finvQuantMenu` 顶层顺序应为 `dashboard, metadata, quote, config-manage, data-collection, environment, template, account, fund, position, quant-backtest, strategy, backtest-analysis, simulation-data, simulation-paper, simulation-live-sim, live-trading`；`grep -c "title: '元数据'" ` 为 1 且 `grep -c "title: '元数据管理'"` 为 0；菜单图标全部可在 `@mdi/font` 检索到、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百六十七、新增二级菜单「量化」（占位）（10841）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10841 | 在「配置管理」之后新增同级别的二级菜单「量化」（用户选定方案 A：仅空节点占位，不移动现有量化节点）。 | 解析 `Web/src/config/menuConfig.ts` 的 `finvQuantMenu` 顶层顺序，`config-manage` 之后、`data-collection` 之前应为 `quant`；`grep -c "title: '量化'" ` 为 1 且该节点 `children` 为空数组；现有 `quant-backtest`/`strategy`/`backtest-analysis`/`simulation-*`/`live-trading` 仍在顶层（未被移入）；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百六十八、「配置管理」移到「行情」之前（10842）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10842 | 二级菜单「配置管理」移动到「行情」之前（即顺序变为 仪表盘 → 元数据 → 配置管理 → 行情 → 量化 → 数据采集 → 其余），节点 key 与子菜单不变。 | 解析 `Web/src/config/menuConfig.ts` 的 `finvQuantMenu` 顶层顺序，`config-manage` 的索引应小于 `quote`；完整顺序为 `dashboard, metadata, config-manage, quote, quant, data-collection, environment, template, account, fund, position, quant-backtest, strategy, backtest-analysis, simulation-data, simulation-paper, simulation-live-sim, live-trading`；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百六十九、「量化」下新增「MVP配置」「回测」三级子菜单（10843）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10843 | 二级菜单「量化」下新增两个三级子菜单「MVP配置」「回测」（当前无对应页面与路由，按占位分组处理，`children` 为空数组）；不移动现有回测相关节点。 | `grep -c "key: 'quant-mvp-config'" Web/src/config/menuConfig.ts` 为 1；`grep -c "key: 'quant-backtest-menu'" Web/src/config/menuConfig.ts` 为 1；两者均在 `key: 'quant'` 节点的 `children` 内且 `children: []`；顶层仍保留 `quant-backtest`「量化策略验证」与 `backtest-analysis`「回测分析」；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百七十、「历史行情数据导入」移入「历史行情」（10844）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10844 | 「历史行情数据导入」（`/meta/import`）从「元数据 → 业务元数据维护」移到「行情 → 历史行情」下作为叶子，key 保持 `meta-import`、标题与路径不变。 | `grep -c "key: 'meta-import'" Web/src/config/menuConfig.ts` 为 1；该行位于 `key: 'quote-history'` 之后且不在 `key: 'meta-maintenance'` 的 children 内；「业务元数据维护」下剩 3 个叶子（`meta-exchange`/`meta-market`/`meta-security`）；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百七十一、「工作流控制与监控」移入「收剔导传ETL」并改名（10845）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10845 | 「工作流控制与监控」从「数据采集 → 行情数据归档」移到「行情 → 收剔导传ETL」下，并改名为「CDEU 工作流控制面板」；key `dedup-export-cdeu` 与路径 `/Meta/Finv/Quote/Cdeu/CdeuOverview` 不变；路由 `meta.title` 与页面内标题同步。 | `grep -c "key: 'dedup-export-cdeu'" Web/src/config/menuConfig.ts` 为 1 且该行含 `title: 'CDEU 工作流控制面板'`，位置在 `key: 'quote-cdeu'` 之后、不在 `key: 'quote-archive'` 段内；`grep -c "title: 'CDEU 工作流控制面板'" Web/src/router.ts` 为 1；`grep -c "CDEU 工作流控制面板" Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` ≥ 2；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百七十二、5 个配置类叶子移入「量化 → MVP配置」（10846）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10846 | 「环境管理」「模板管理」「账户管理」「资金管理」「持仓管理」从二级（顶层）移动到「量化 → MVP配置」下作为四级叶子；key、标题、路径均不变。 | `grep -c "key: 'quant-mvp-config'" Web/src/config/menuConfig.ts` 为 1，且 `environment`/`template`/`account`/`fund`/`position` 各出现 1 次并位于该节点 `children` 内；顶层顺序为 `dashboard, metadata, config-manage, quote, quant, data-collection, quant-backtest, strategy, backtest-analysis, simulation-data, simulation-paper, simulation-live-sim, live-trading`；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百七十三、回测类叶子移入「量化 → 回测」（10847）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10847 | 「黄金期货合约回测验证」与「回测分析」移入「量化 → 回测」下作为四级叶子；原「量化策略验证」分组因叶子移空而删除；两叶子 key、标题、路径不变。 | `grep -c "key: 'quant-backtest-menu'" Web/src/config/menuConfig.ts` 为 1 且其 `children` 为 {`backtest-gold-futures`, `backtest-analysis`}、`icon: 'mdi-history'` 存在；`grep -c "key: 'quant-backtest'" ` 为 0（已撤除）；`backtest-analysis` 在配置中出现 1 次；顶层顺序为 `dashboard, metadata, config-manage, quote, quant, data-collection, strategy, simulation-data, simulation-paper, simulation-live-sim, live-trading`；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百七十四、新增「量化 → 验证」并迁入 3 个验证类叶子（10848）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10848 | 在「量化 → 回测」之后新增同级别的「验证」三级分组，并把「仿真数据验证」「模拟盘验证」「实盘仿真验证」从顶层移入该分组（key/标题/路径不变）。 | `grep -c "key: 'quant-verify'" Web/src/config/menuConfig.ts` 为 1，其 `children` 为 {`simulation-data`, `simulation-paper`, `simulation-live-sim`}；三个 key 在配置中各出现 1 次；顶层顺序为 `dashboard, metadata, config-manage, quote, quant, data-collection, strategy, live-trading`；量化下三级节点顺序为 {MVP配置, 回测, 验证}；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百七十五、新增「量化 → 实盘」并迁入「实盘交易」（10849）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10849 | 在「量化 → 验证」之后新增同级别的「实盘」三级分组，并把「实盘交易」从顶层移入该分组（key/标题/路径不变）。 | `grep -c "key: 'quant-live'" Web/src/config/menuConfig.ts` 为 1，其 `children` 为 {`live-trading`}；`live-trading` 在配置中出现 1 次；顶层顺序为 `dashboard, metadata, config-manage, quote, quant, data-collection, strategy`；量化下三级节点顺序为 {MVP配置, 回测, 验证, 实盘}；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百七十六、新增「量化 → 策略」并迁入「策略管理」（10850）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10850 | 在「量化 → 回测」之前新增「策略」三级分组，并把「策略管理」从顶层移入该分组（key/标题/路径不变）。 | `grep -c "key: 'quant-strategy'" Web/src/config/menuConfig.ts` 为 1，其 `children` 为 {`strategy`}，且该节点索引小于 `quant-backtest-menu`；`strategy` 在配置中出现 1 次；顶层顺序为 `dashboard, metadata, config-manage, quote, quant, data-collection`；量化下三级节点顺序为 {MVP配置, 策略, 回测, 验证, 实盘}；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百七十七、「量化 → 实盘」改名为「交易」（10851）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10851 | 「量化」下的三级菜单「实盘」改名为「交易」；节点 key（`quant-live`）、图标与其下叶子「实盘交易」保持不变。 | `grep -c "title: '交易'" Web/src/config/menuConfig.ts` ≥ 1 且 `grep -c "title: '实盘'" ` 为 0；`grep -c "key: 'quant-live'"` 为 1；量化下三级节点顺序为 {MVP配置, 策略, 回测, 验证, 交易}；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百七十八、「业务元数据维护」改名为「Finv业务元数据」（10852）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10852 | 「元数据」下的三级菜单「业务元数据维护」更名为「Finv业务元数据」；节点 key（`meta-maintenance`）与子菜单不变。 | `grep -c "title: 'Finv业务元数据'" Web/src/config/menuConfig.ts` 为 1；`grep -c "title: '业务元数据维护'" ` 为 0；`grep -c "key: 'meta-maintenance'" ` 为 1；菜单图标全部有效、`path` 全部有路由、无重复 key | |

## 一百七十九、「交易所下设市场信息维护」改名为「市场信息管理」（10853）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10853 | 「交易所下设市场信息维护」更名为「市场信息管理」；叶子 key（`meta-market`）、路由 `/meta/market` 不变，路由 `meta.title` 同步。 | `grep -c "title: '市场信息管理'" Web/src/config/menuConfig.ts` 为 1；`grep -rc "交易所下设市场信息维护" Web/src/config/menuConfig.ts Web/src/router.ts` 均为 0；`grep -c "key: 'meta-market'"` 为 1；菜单图标全部有效、`path` 全部有路由、无重复 key | |

## 一百八十、「交易所信息维护」改名为「交易所信息管理」（10854）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10854 | 「交易所信息维护」更名为「交易所信息管理」；叶子 key（`meta-exchange`）、路由 `/meta/exchange` 不变，路由 `meta.title` 同步。 | `grep -c "title: '交易所信息管理'" Web/src/config/menuConfig.ts` 为 1；`grep -rc "交易所信息维护" Web/src/config/menuConfig.ts Web/src/router.ts` 均为 0；`grep -c "key: 'meta-exchange'"` 为 1；菜单图标全部有效、`path` 全部有路由、无重复 key | |

## 一百八十一、「规范证券信息维护」改名为「规范证券信息管理」（10855）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10855 | 「规范证券信息维护」更名为「规范证券信息管理」；叶子 key（`meta-security`）、路由 `/meta/security` 不变，路由 `meta.title` 同步。 | `grep -c "title: '规范证券信息管理'" Web/src/config/menuConfig.ts` 为 1；`grep -rc "规范证券信息维护" Web/src/config/menuConfig.ts Web/src/router.ts` 均为 0；`grep -c "key: 'meta-security'"` 为 1；菜单图标全部有效、`path` 全部有路由、无重复 key | |

## 一百八十二、「去重分钟行情查询」改名并移入「收剔导传ETL」（10856）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10856 | 「去重分钟行情查询」更名为「K线数据查询」，并从「数据采集 → 行情数据去重」移入「行情 → 收剔导传ETL」；key `dedup-kline-query` 与路由不变，路由 `meta.title` 同步。 | `grep -c "title: 'K线数据查询'" Web/src/config/menuConfig.ts` 为 1 且该行在 `key: 'quote-cdeu'` 段内、不在 `key: 'quote-dedup'` 段内；`grep -rc "去重分钟行情查询" Web/src/config/menuConfig.ts` 为 0；`grep -c "title: 'K线数据查询'" Web/src/router.ts` 为 1；菜单图标全部有效、`path` 全部有路由、无重复 key | |

## 一百八十三、「去重分钟行情统计」改名并移入「收剔导传ETL」（10857）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10857 | 「去重分钟行情统计」更名为「K线数据统计」，并从「数据采集 → 行情数据去重」移入「行情 → 收剔导传ETL」；移空后的「行情数据去重」分组删除；key 与路由不变，路由 `meta.title` 与页面内标题同步。 | `grep -c "title: 'K线数据统计'" Web/src/config/menuConfig.ts` 为 1 且在 `quote-cdeu` 段内；`grep -c "key: 'quote-dedup'"` 为 0；ETL 下叶子为 {历史月份存量数据清点, CDEU 工作流控制面板, K线数据查询, K线数据统计}；`grep -rc "去重分钟行情统计" Web/src/views` 为 0；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百八十四、5 个归档运维页面移入「收剔导传ETL」（10858）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10858 | 「导出任务」「文件与上传」「归档统计与预算」「归档主数据与规则」「归档日志与补丁」从「数据采集 → 行情数据归档」移入「行情 → 收剔导传ETL」；key/标题/路径不变。 | 5 个 key（`dedup-export-job`/`dedup-export-file`/`dedup-export-stats`/`dedup-export-config`/`dedup-export-log`）在菜单配置中各出现 1 次且均位于 `key: 'quote-cdeu'` 段内；`key: 'quote-archive'` 段内只剩 {归档监控总览, 端到端链路}；ETL 下叶子共 9 项；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百八十五、「配置管理」改名为「系统配置」（10859）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10859 | 顶层二级菜单「配置管理」更名为「系统配置」；节点 key（`config-manage`）、图标、路由 `/Meta/Finv/Quant/Config` 不变，路由 `meta.title` 与页面内标题同步。 | `grep -c "title: '系统配置'" Web/src/config/menuConfig.ts` 为 1 且 `Web/src/router.ts` 中也为 1；`grep -rc "配置管理" Web/src/config/menuConfig.ts Web/src/router.ts Web/src/views/Meta/Finv/Quant/Backtest/ConfigManageView.vue` 均为 0；`grep -c "key: 'config-manage'"` 为 1；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百八十六、「行情数据归档」移入「行情」并在「历史行情」之后（10860）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10860 | 「行情数据归档」从「数据采集」移入「行情」下，排在「历史行情」之后作为第三个三级分组；移空后的「数据采集」二级分组删除；key/标题/路径不变。 | 解析 `menuConfig.ts` 顶层顺序为 `dashboard, metadata, config-manage, quote, quant`；`key: 'data-collection'` 为 0 次；「行情」下三级分组依次为 {收剔导传ETL, 历史行情, 行情数据归档}；`key: 'quote-archive'` 出现 1 次且位于 `key: 'quote'` 段内；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百八十七、K线数据查询/统计前置到 ETL 最前（10861）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10861 | 「K线数据查询」「K线数据统计」移动到「行情 → 收剔导传ETL」下叶子的最前面（key/标题/路径不变）。 | 解析 `menuConfig.ts` 中 `key: 'quote-cdeu'` 节点的 `children`，前两项 `title` 依次为 `K线数据查询`、`K线数据统计`，其后为 `历史月份存量数据清点`…；叶子总数 9；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百八十八、「导出任务」→「文件导出」、「文件与上传」→「文件上传」（10862）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10862 | 「导出任务」更名为「文件导出」、「文件与上传」更名为「文件上传」；两个叶子 key、图标、路由路径不变，路由 `meta.title` 与页面内标题同步。 | `grep -c "title: '文件导出'" Web/src/config/menuConfig.ts` 为 1、`title: '文件上传'` 为 1；`grep -rc "title: '导出任务'\|title: '文件与上传'" Web/src/config/menuConfig.ts` 为 0；`grep -c "meta: { title: '文件导出' }" Web/src/router.ts` 为 1、`meta: { title: '文件上传' }` 为 1；`grep -rc "月归档导出任务\|月归档导出文件与上传" Web/src` 为 0；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百八十九、系统显示名改为「FinvQuant策略回测量化交易」（10863）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10863 | 页面顶部标题「FinvQuant 量化策略交易平台」改为「FinvQuant策略回测量化交易」，并同步浏览器页签标题、`document.title` 兜底、登录页标题、一级系统副标题与占位页文案。 | `grep -rc "FinvQuant策略回测量化交易" Web/src/App.vue Web/index.html Web/src/router.ts Web/src/views/Common/LoginView.vue Web/src/views/Common/PlaceholderView.vue` 各 ≥ 1；`grep -rc "量化策略交易平台\|交易平台" Web/src Web/index.html` 为 0；`grep -c "subtitle: '策略回测量化交易'" Web/src/config/menuConfig.ts` 为 1；`npx vue-tsc --noEmit` 与 `npx vite build` 通过，且 `dist/index.html` 含新标题 | |

## 一百九十、「收剔导传ETL」新增「CDEU自动化」（10864）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10864 | 在「行情 → 收剔导传ETL」的「归档日志与补丁」之后新增「CDEU自动化」节点（当前无对应页面与路由，按占位分组处理，`children` 为空数组）。 | `grep -c "key: 'cdeu-automation'" Web/src/config/menuConfig.ts` 为 1 且位于 `key: 'quote-cdeu'` 段内、「归档日志与补丁」之后；该节点 `children: []`；菜单图标全部有效、`path` 全部有路由、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百九十一、「CDEU自动化」改为叶子菜单（10865）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10865 | 「CDEU自动化」由占位分组改为**叶子**菜单（`path=/Meta/Finv/Quote/Cdeu/Automation`），并新增对应路由（复用 `PlaceholderView` 占位页，`meta.title=CDEU自动化`），占位页内登记该模块规划说明。 | `grep -c "key: 'cdeu-automation'" Web/src/config/menuConfig.ts` 为 1 且该行为单行叶子（含 `path: '/Meta/Finv/Quote/Cdeu/Automation'`、无 `children`）；`grep -c "'/Meta/Finv/Quote/Cdeu/Automation'" Web/src/router.ts` 为 1；`grep -c "'cdeu-automation': {" Web/src/views/Common/PlaceholderView.vue` 为 1；菜单全部 `path` 有对应路由、图标有效、无重复 key；`npx vue-tsc --noEmit` 与 `npx vite build` 通过 | |

## 一百九十二、证券主数据与 CDEU 域收口（10866–10870）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10866 | `finv_security` 的 470 条历史种子（`V300002` / `V300009`，因 `market_code` 列后补而长期为 `0` 且 `flag_enable='0'`）回填 `market_code` 并启用；`finv_market` 的一批市场补 `market_symbol` 与 `base_currency`。取值依据：400 条按 `futu.finv_futu_quote_collect` + `SecuSync/SecuMapping.json` 的 `marketCodeByFutu` 口径，70 条按「交易所 + 证券类型 + 代码形态」推导；跨币种市场（`1251` / `2003` / `8000`）`base_currency` 保持空串。 | `select count(*) from public.finv_security where flag_enable='0'` = 0；`select count(*) from public.finv_security where market_code=0` = 0；`select count(*) from public.finv_market m join public.finv_security s on s.market_code=m.market_code where m.market_symbol=''` = 0；`select version, success from public.schema_version where version in ('200068','200069')` 两行 `success` 均为 `t` | √ |
| 10867 | 导出中转表从 `export` 域迁入 `cdeu.finv_quote_secu_kline_min_export`（与 `cdeu.finv_quote_secu_kline_min` 同构、允许清空），并删除源表 `export.finv_quote_secu_kline_min`；代码中转表常量与集成测试同步，`TestSQLSchemaQualified` 的 `exactTableSchema` 收敛为 `quote.` / `cdeu.`。 | `select to_regclass('export.finv_quote_secu_kline_min')` 为 NULL；`select count(*) from information_schema.tables where table_schema='cdeu' and table_name='finv_quote_secu_kline_min_export'` = 1；`grep -c 'cdeu.finv_quote_secu_kline_min_export' internal/meta/finv/quote/cdeu/store.go` ≥ 1；`go test ./internal/database -run TestSQLSchemaQualified` 通过；`select success from public.schema_version where version='100088'` 为 `t` | √ |
| 10868 | 原 `internal/meta/finv/quote/dedup` 包整体迁入 `internal/meta/finv/quote/cdeu`（包名 `dedup` → `cdeu`，子包 `job` / `jobmeta` 不变），外部引用（`cmd/server`、`internal/api`、`cdeu/job`）同步更新，不得残留旧路径引用。 | `test ! -d internal/meta/finv/quote/dedup`；`grep -rn 'quote/dedup' --include=*.go internal cmd` 无输出；`go build ./...` 与 `go vet ./...` 通过 | √ |
| 10869 | 市场码与货币口径收口：字典外重复码并入已登记等价码（`1313`→`1331`、`1314`→`1312`、`1150`→`1250`），港股人民币柜台（8 开头 5 位代码）由 `1201` 归入 `1205`，韩国 KOSPI / 印度 SENSEX 的 `base_currency` 与证券 `currency_type` 由 `USD` 改为 `KRW` / `INR`；删除已空的 `export` schema；仓库外 `SecuSync/SecuMapping.json` 与 `ExportSecu.py` 同步落 1331/1312/1250/1205，确保下次增量导入不回退。 | `select count(*) from public.finv_security s left join public.finv_market m on m.market_code=s.market_code where m.market_code is null` = 0；`select count(*) from public.finv_security where market_code=1205` = 26；`select base_currency from public.finv_market where market_code in (1508,1521)` 为 `KRW` / `INR`；`select count(*) from pg_namespace where nspname='export'` = 0；`select version, success from public.schema_version where version in ('100089','200070')` 均为 `t`；`grep -c '"13": 1331' ../FinvQuantDesign/Scripts/SecuSync/SecuMapping.json` = 1 且 `grep -c '"marketCode": 1205'` = 1 | √ |
| 10870 | 上生产前必须在**独立测试库**上完整跑一次「全链迁移 + 数据库集成测试」，确认 `V200086`/`V200088`/`V200089` 与 `V300068`–`V300070` 在全新库上按版本顺序执行无错、幂等重跑 0 变更。 | 在空库执行 `FINV_TEST_PG_DSN=... go test -p 1 ./internal/database -run TestMigrateIntegration -v` 通过；`grep -rn "schemaname = 'dedup'\|table_schema = 'dedup'" internal/database/migrate_integration_test.go` 无输出；全链跑完后 `select count(*) from pg_namespace where nspname in ('dedup','dedup_frozen','export')` = 0 | √（2026-10-31 在独立库 `finv_migtest` 验证通过，161 个迁移 0 失败；修复了 `V200086` 提前删 dedup、`V300057` 冲掉 `JOB_EXPLAIN_EXPORT_DECISION`、`V300070` 缺字典等 3 处阻塞） |
| 10871 | 让 `finv_market` 字典编号体系在「全新库」与「生产库」一致：仓库种子（`V300017`/`V300018`）用的是 `1150 HKFE-F` / `1313 US-OTC` / `1314 US-I2` / `1312 US-ARCA` / `1315 BMS` … 旧编号，生产库现行体系是 `1250 HKFE-F` / `1331 US-OTC` / `1312 US-I2` / `1317 US-ARCA` / `1315 US-NSDQ` / `1330 US-ADR` / `1340 BMS` …；全新库因此有 77 行证券的 `market_code` 落在字典外。需按生产库口径重编号字典并同步证券 `market_code`（或反向统一到种子口径并重生成 `V300062`）。 | 在独立库 `finv_migtest` 全链跑完后 `select count(*) from public.finv_security s left join public.finv_market m on m.market_code=s.market_code where m.market_code is null` = 0；`select count(*) from public.finv_market` 与生产库相同（当前生产库 63 行）；`select market_code,market_flag from public.finv_market where market_flag in ('HKFE-F','US-OTC','US-I2','US-ARCA','US-NSDQ')` 与生产库逐一相同 | √（2026-10-31 由 `V300073` 完成：全新库与生产库 `finv_market` 均 63 行且逐字段一致，两库证券字典外均 0 行） |

## 一百九十三、「备份与恢复」前端二级菜单（10872）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10872 | 前端「金融投资」系统在「量化」二级菜单之后新增「备份与恢复」二级分组（`key='brs'`，`icon=mdi-backup-restore`），下辖「备份管理」「恢复管理」两个三级分组与 4 个占位叶子：备份登记维护（`/Meta/Finv/Brs/Backup/Table`）、备份记录与产物（`/Meta/Finv/Brs/Backup/Record`）、对象存储配置（`/Meta/Finv/Brs/Backup/Storage`）、数据恢复（`/Meta/Finv/Brs/Restore/Run`）；路由前缀 `/Meta/Finv/Brs/`，复用 `PlaceholderView` 并登记各页规划说明，作为后续 BRS 服务（`internal/meta/finv/brs`）前端菜单的扩展位。 | `grep -c "key: 'brs'" Web/src/config/menuConfig.ts` 为 1 且该节点在 `key: 'quant'` 之后；`grep -c "'/Meta/Finv/Brs/" Web/src/router.ts` = 4；`grep -c "'brs-" Web/src/views/Common/PlaceholderView.vue` = 4；菜单全部 `path` 有对应路由、图标有效（`grep -c '\.mdi-backup-restore::before' Web/node_modules/@mdi/font/css/materialdesignicons.css` = 1）、无重复 key；`npm run build`（含 `vue-tsc -b`）通过 | √ |

## 一百九十四、SH 指数类 usc 8 位改 6 位（10873）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10873 | 上游证券代码口径调整后（`SH000001`~`SH000999` → `SH0001`~`SH0999`，去掉数字部分前两个 `0`），生产库历史数据需对齐且不得因主键/唯一约束冲突导致脚本失败：`V300074` 对 8 位 usc 只在**没有对应 6 位行**时就地改号（`usc` + `security_code` 同步，双 `NOT EXISTS` 避 `finv_security_pkey` 与 `uq_finv_security_exchange_code`），已存在 6 位行时以 6 位为准回填英文名（D1）、`flag_enable='1'` 优先（D2）、`gmt_create` 取较早值（D3）；`SH0985` 对齐为 `exchange_code=11` / `security_code='SH0985'`（D4）、`security_type` 保留 6 位现网值（D5）；`SH0001` 的 `security_code` 归一为 `'SH0001'`（D8）；`futu.finv_futu_mapping_security.finv_usc` 同步 6 位；其余 17 张含 `usc` 的表本期只巡检不处理（D6）、8 位重复行暂不删除（D7）。 | 独立库与生产库执行后 `select count(*) from public.finv_security where usc ~ '^SH[0-9]{4}$'` = 317、`select count(*) from public.finv_security where usc ~ '^SH[0-9]{6}$'` = 22（D7 保留）、`select count(*) from public.finv_security where usc ~ '^SH[0-9]{4}$' and security_code ~ '^SH[0-9]{6}$'` = 0、`select count(*) from futu.finv_futu_mapping_security where finv_usc ~ '^SH[0-9]{6}$'` = 0、`select concat(exchange_code, '/', security_code) from public.finv_security where usc='SH0985'` = `11/SH0985`、`select security_code from public.finv_security where usc='SH0001'` = `SH0001`；`select success from public.schema_version where version='200074'` 为 `t`；`V300074` 重跑全部 0 行影响；`go test -p 1 ./internal/database -run TestMigrateIntegration` 通过 | √ |
| 10874 | 【**待办**】`public.finv_security` 中 22 行 8 位 usc（`SH000002`…`SH000985`）是**为兼容保留**的重复记录（迁移 `V300074` 按决策 D7 未删除，6 位行为准、代码与采集均走 6 位）。暂不处理；待**下月初（2026-12 月初）上游数据更新完成后**再评估并清理：删除条件必须是「同 usc 的 6 位行已存在且字段完整」（`DELETE ... WHERE usc ~ '^SH[0-9]{6}$' AND EXISTS (同 usc 的 6 位行)`），脚本需幂等可重跑，执行前 `pg_dump` 备份到 `Deploy/Backup/`。 | 保留期：`select count(*) from public.finv_security where usc ~ '^SH[0-9]{6}$'` = 22 且 `select count(*) from public.finv_security where usc ~ '^SH[0-9]{4}$'` = 317；清理后：8 位行 = 0、6 位行 = 317，且两个库（`finv` / 全新库）一致；清理脚本重跑影响 0 行；`select success from public.schema_version where version='<清理迁移编号>'` 为 `t` | |
| 10875 | `public.finv_security` 中 `usc='USDIndex'`（美元指数）的 `type_secu_sub` 由 `CER` 修正为 `CIX`，与所属市场 1253（`market_abbr='FX-IDX'`、`market_symbol='CIX'`）的符号口径一致；不改 `market_code` / `currency_type` / `flag_enable`，不动小写的 `USDindex`（`V300003` 老种子、`type_secu_sub` 为空串）。迁移 `V300075`。 | `select type_secu_sub from public.finv_security where usc='USDIndex'` = `CIX`；`select count(*) from public.finv_security where market_code=1253 and type_secu_sub<>'' and type_secu_sub<>'CIX'` = 0；`select type_secu_sub from public.finv_security where usc='USDindex'` = 空串；`select success from public.schema_version where version='200075'` 为 `t`；`V300075` 重跑影响 0 行；`go test -p 1 ./internal/database -run TestMigrateIntegration` 通过 | √ |
| 10876 | `usc='USDIndex'`（美元指数）**固定归属市场 1253**（`market_abbr='FX-IDX'`、`market_symbol='CIX'`、`en_security_type='1100'`），后续新增/修改一律按 1253 定义。迁移 `V300076`：`V300062` 种子写的是 1251（生产库的 1253 来自早前手工回填、未落迁移），本迁移收口使「全新库」与「生产库」口径一致，避免出现「1251 市场 + `type_secu_sub=CIX`」的矛盾行；不动 `type_secu_sub`（`V300075` 已为 `CIX`）/`currency_type`/`flag_enable`，不动小写 `USDindex`。 | `select concat(market_code, '/', type_secu_sub) from public.finv_security where usc='USDIndex'` = `1253/CIX`；`select count(*) from public.finv_security where market_code=1253 and type_secu_sub<>'' and type_secu_sub<>'CIX'` = 0；`select count(*) from public.finv_security where market_code=1251 and type_secu_sub='CIX'` = 0；`select success from public.schema_version where version='200076'` 为 `t`；`V300076` 重跑影响 0 行；两库（`finv` / 全新库）`USDIndex` 均为 `1253/CIX`；`go test -p 1 ./internal/database -run TestMigrateIntegration` 通过 | √ |
| 10877 | `public.finv_region` 补两行并统一 US 口径：① 新增 `(idx=2, region='FX', name='外汇')` —— `finv_exchange` 中 100/101（FX、FX-CFD）的 `region='FX'` 此前在字典中不存在，`cdeu/store.go` 的 `LEFT JOIN finv_region r ON r.region = e.region` 永远落空、只能退化取 `finv_exchange.region` 文本；② 新增 `(idx=3, region='CRYPTO', name='加密货币')`（用户要求「顺便把加密货币的 3 也加上」）；③ `idx=103` 的 `region` 由 `USA` 改为 `US`，与生产库及 `finv_exchange.region`（9 个美股交易所）一致，否则全新库美股导出文件名 `region` 段为 `USA`、生产库为 `US`，两边对不上。`idx` 编码依据 `V300004`（0=UNK、1=GLOBAL、2~100 预留扩展段）。本期不改 `finv_exchange.region`（8 个加密货币交易所仍为 `GLOBAL`，`CRYPTO` 行属字典先行）、不处理 `N/A`（120/121/122/9003）。迁移 `V300077`。 | `select count(*) from public.finv_region` = 27；`select concat(idx, '/', region, '/', name) from public.finv_region where idx in (2,3,103) order by idx` 依次为 `2/FX/外汇`、`3/CRYPTO/加密货币`、`103/US/美国`；`select count(*) from public.finv_exchange e left join public.finv_region r on r.region=e.region where e.region in ('FX','US') and r.region is null` = 0；`select success from public.schema_version where version='200077'` 为 `t`；`V300077` 重跑影响 0 行；两库（`finv` / 全新库）`finv_region` 均 27 行且 2/3/103 一致；`go test -p 1 ./internal/database -run TestMigrateIntegration` 通过 | √ |
| 10878 | `public.finv_region` 中加密货币的 `region` 取值由 `CRYPTO` 修正为 **`CC`**：`V300077` 新增 `idx=3` 时按 `exchange_abbr`/`market_flag` 习惯推测写成了 `CRYPTO`，用户确认正确取值为两字母短码 `CC`（与 `finv_exchange.exchange_abbr='CC'`、`CN`/`HK`/`US` 等短码风格一致），`name` 保持 `加密货币`。迁移 `V300078`，含 idx=3 缺失时的幂等补插；不改 `finv_exchange.region`（8 个加密货币交易所 10000~10007 仍为 `GLOBAL`）、不改 `futu.finv_futu_mapping_region`。 | `select concat(idx, '/', region, '/', name) from public.finv_region where idx=3` = `3/CC/加密货币`；`select count(*) from public.finv_region where region='CRYPTO'` = 0；`select count(*) from public.finv_region where idx=3` = 1；`select success from public.schema_version where version='200078'` 为 `t`；`V300078` 重跑影响 0 行；两库（`finv` / 全新库）`idx=3` 均为 `CC/加密货币`；`go test -p 1 ./internal/database -run TestMigrateIntegration` 通过 | √ |
| 10879 | `public.finv_exchange` 中 8 个加密货币交易所（`10000 CC` ~ `10007 CCPT`）的 `region` 由 `GLOBAL` 改为 **`CC`**，使 `V300077`/`V300078` 新增的 `finv_region(3,'CC','加密货币')` 真正被引用。影响：`cdeu/store.go` 元数据回填对加密证券的 `region` 变为 `CC`（`cdeu.finv_quote_cdeu_usc` 当前无加密证券行，无需回刷）；加密证券导出文件名 `region` 段由 `GLOBAL_*` 变为 `CC_*`（`cdeu.finv_quote_cdeu_export_file` 中此类证券历史导出 0 条，远端无归档文件，无回溯包袱）。不改 `finv_security`/`cdeu.*`/`futu.*`，不改 `base_currency`/`flag_enable`/`exchange_abbr`。迁移 `V300079`。 | `select count(*) from public.finv_exchange where exchange_code between 10000 and 10007 and region='CC'` = 8；`select count(*) from public.finv_exchange where region='GLOBAL'` = 0；`select count(*) from public.finv_exchange e join public.finv_region r on r.region=e.region where e.exchange_code between 10000 and 10007 and r.idx=3` = 8；`select success from public.schema_version where version='200079'` 为 `t`；`V300079` 重跑影响 0 行；两库（`finv` / 全新库）该 8 行 region 均为 `CC`；`go test -p 1 ./internal/database -run TestMigrateIntegration` 通过 | √ |
| 10880 | `public.finv_security` 中 20 只加密货币证券（`BCHUSDC`/`BTC`/`BTCHKD`/`BTCUSDC`/`BTCUSDT`/`DOGE`/`ETH`/`ETHBTC`/`ETHUSDC`/`PAXG`/`PAXGUSD`/`SOL`/`SOLBTC`/`UNI`/`USDC`/`USDCUSD`/`USDCUSDT`/`USDT`/`USDTUSD`/`BTCUSD`）的 `exchange_code` 由 `100`（`FX` 外汇市场）修正为 **`10001`**（`CRYPTO` Cryptocurrency Exchange）。成因：`V300009` 增量种子（2026-08-05）即写 100，`V300062` 权威导入为 10001 但 `ON CONFLICT (usc) DO NOTHING` 未覆盖，`V300068` 只修了 `market_code`。`market_code` 不变（19 只 10001、`BTCUSD` 保持 10011）；因 `V300079` 已把加密交易所 region 切到 `CC`，这 20 只回填 `region` 由 `FX` 变 `CC`，导出文件名 `region` 段相应由 `FX_*` 变 `CC_*`（该批证券历史导出 0 条，无回溯包袱）。迁移 `V300080`，用明确 usc 清单限定，写入前做 `uq_finv_security_exchange_code` 冲突预检。 | `select count(*) from public.finv_security where usc in ('BCHUSDC','BTC','BTCHKD','BTCUSDC','BTCUSDT','DOGE','ETH','ETHBTC','ETHUSDC','PAXG','PAXGUSD','SOL','SOLBTC','UNI','USDC','USDCUSD','USDCUSDT','USDT','USDTUSD','BTCUSD') and exchange_code=10001` = 20；`select count(*) from public.finv_security where exchange_code=100 and market_code in (10001,10011)` = 0；`select count(*) from public.finv_security where market_code in (10001,10011,10013,10014) and exchange_code not between 10000 and 10007` = 0；`select count(*) from public.finv_security s join public.finv_exchange e using (exchange_code) where s.market_code in (10001,10011,10013,10014) and e.region='CC'` = 192；`select success from public.schema_version where version='200080'` 为 `t`；`V300080` 重跑影响 0 行；两库（`finv` / 全新库）该 20 行均为 10001；`go test -p 1 ./internal/database -run TestMigrateIntegration` 通过 | √ |
| 10881 | 迁移号段重排（为生产库重做基线做准备）：三段号段 —— `V100000`～`V199999` **基础准备**（创建角色 / 校验数据库 / 创建 schema）、`V200000`～`V299999` **表结构 DDL**、`V300000`～`V399999` **数据**（字典种子 / 回填 / 修正）。落地内容：① 原 `V1xxxxx` 段 89 个文件整体 +100000 → `V200001`～`V200089`，原 `V2xxxxx` 段 81 个文件整体 +100000 → `V300000`～`V300080`（执行顺序不变）；② 新增基础准备段 12 个脚本（一对象一脚本，需求 10882 ①）：`V100001`（角色 `finv`，幂等、不落密码）、`V100002`～`V100008`（`quote`/`quant`/`futu`/`sys`/`sche`/`cdeu`/`archive` 七个 schema 各一个文件，`AUTHORIZATION CURRENT_USER`）、`V100009`（`brs`，从 `V300060` 归位到基础准备段）、`V100010`～`V100012`（三项只读校验各一个文件：库名 / 角色对 `public` 的 `CREATE` 权限 / 8 个基础 schema 的 `USAGE`+`CREATE` 权限；`CREATE DATABASE` 不能进迁移事务，建库由部署侧负责）；③ 全仓版本引用同步替换（SQL/Go/文档 1678 处，含 `CHANGELOG`/`RequirementList`/`Refactor` 历史条目，另手工修 `Deploy/README.md`、`Deploy/Script`、`DbSchemaSpec` §4 号段约定）；④ 存量库 `public.schema_version` 一次性 +100000（`Deploy/Upgrade/2611-001-MigrationSegmentsShift.sql`，`public.schema_version_plan_shift` marker 保证只执行一次，分批 UPDATE 避免「旧 100077→200077 撞尚未处理的旧 200077」主键冲突，并同步 `description`）。 | 全新库全链 `select count(*) from public.schema_version` = 183（12 基础 + 89 DDL + 82 数据），`min(version::bigint)`=100001、`max`=300080；存量库执行平移脚本后 `select count(*), min(version::bigint), max(version::bigint) from public.schema_version` = `170 / 200001 / 300080`，再启动服务后 = 183；平移脚本重跑输出「此前已执行，跳过」且行数不变；`select count(*) from public.schema_version where version::bigint between 100013 and 199999` = 0；`select string_agg(nspname, ',') from pg_namespace where nspname in ('quote','quant','futu','sys','sche','cdeu','archive','brs')` 含全部 8 个；`ls Deploy/Migrations \| sed -E 's#.*/V([0-9]+)__.*#\1#' \| sort -n \| uniq -d` 无输出；`go test ./internal/docs ./internal/database` 通过、`TestMigrateIntegration` 通过；`go build ./...` 通过 | √ |

## 一百九十五、生产库基线重做（待办）（10882）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10882 | 【**待办**】生产库基线重做：等 2026 年 10 月数据导出完成后，按需求 10881 确立的三段号段（`V100000`～`V199999` 基础准备、`V200000`～`V299999` 表结构 DDL、`V300000`～`V399999` 数据），把当前 **173 个历史迁移**重做为一份干净基线后重建生产库。**已定方案**（用户确认，2026-11-01）：① 基线形态——**一对象一脚本**（✅ 已落地）：基础准备段即 `V100001` 角色 `finv`、`V100002`～`V100008` 七个 `schema_<name>`（`quote`/`quant`/`futu`/`sys`/`sche`/`cdeu`/`archive`）、`V100009__schema_brs_create`（`brs` 从数据段 `V300060` 归位到本段）、`V100010`～`V100012`（三项只读校验各一个文件：库名、角色对 `public` 的 `CREATE` 权限、8 个基础 schema 的 `USAGE`+`CREATE` 权限；`CREATE DATABASE` 不能在迁移事务内执行，建库仍归部署侧），每对象一个文件，迁移总数 174 → 183；建库由初始化脚本 `Deploy/Init/2610-001-CreateDatabase.sql`（`psql -v db=... -v owner=...`，幂等，不建角色不设密码）承担，不进迁移链。**待明确的两件事**（需在动工前与用户逐项确认）：② 历史遗留归位——把混在 DDL 段的数据脚本（`V200018` 交易所种子、`V200025`～`V200042` 系统字典与配置）重新归入 `V3xxxxx`，并合并 `V3xxxxx` 中大量一次性数据修正（`V300068` 起的 market_code / usc / region 修正）；③ 废弃对象清理——已删除的 `dedup` / `dedup_frozen` / `export` schema 相关脚本是否直接剔除（`brs` 已定纳入基础准备段，见 ①）。**执行前置**：10 月数据已完整导出并归档到 GitHub（`Data/Finv/SecuQuoteMon/FTMM/`），`finv_security` 等业务数据可重建（或按需从归档回灌）。 | 基线方案确定后细化；初步验收口径：① 全新库从零跑基线 **0 失败**，`select count(*) from public.schema_version` 等于基线脚本数且 `min(version::bigint) >= 100001`（基础准备段为 12 个脚本，`V100001`～`V100012` 每个版本号对应且仅对应一份文件）；② 基线重建后的库与当前生产库逐表结构一致（`information_schema.columns` 差集为空）、关键字典数据一致（`finv_region` 27 行、`finv_market` 63 行、`finv_exchange` 行数一致）；③ `go test ./internal/database -run TestMigrateIntegration` 与 `go test ./internal/docs` 通过；④ 旧库已 `pg_dump` 备份到 `Deploy/Backup/` 且保留期明确 | |


## 一百九十六、CDEU 运行树状态判定修复（10883）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10883 | CDEU 运行树状态误判修复：`decideCDEUStatus` 原把子级的 `PENDING`/`READY` 与 `RUNNING` 一起归入 `hasRunning`，导致「子 JOB 尚未建立运行记录」的流程被评估一次就写成 `RUNNING`（`started_at` 仍为 `NULL`，因为时间由子级推导）。界面表现：前序阶段还是 `PENDING`，后序阶段里的流程（如「每日入口流量统计」）却显示在跑，实际没有任何 `job_run` 在执行。修复口径：① `RUNNING` 单独判 `hasRunning`；② `PENDING`/`READY` 单独记 `hasPending`；③ 判定顺序 `RUNNING` → 关键失败 `FAILED` → `PAUSED` → **未满足完成条件且存在未开始子级时回写 `PENDING`** → `FAILED`/`PARTIAL`/`SUCCESS`；④ `ANY_OF`/`AT_LEAST` 已满足条件时不受未开始子级影响（仍按原逻辑判 `PARTIAL`/`SUCCESS`）。 | `go test ./internal/meta/finv/quote/cdeu -run TestDecideCDEUStatus -v` 全绿（含新增「全部未开始」「部分未开始」「READY 视为未开始」「关键失败优先于未开始」「运行中优先于未开始」「ANY_OF 满足后不被未开始拖住」6 个用例）；`go build ./...` 通过；生产库对 run 81 重新评估后 `select process_id,status,started_at from cdeu.finv_quote_cdeu_process_run where workflow_run_id=81` 中「每日入口流量统计」由 `RUNNING`（`started_at` 为空）变为 `PENDING`，且无 `RUNNING` 记录 | √ |

## 一百九十七、CDEU 历史残留运行记录收敛（10884）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10884 | 收敛「工作流已终态、子级仍非终态」的历史残留运行记录（`V300081`）：旧版本在运行被终止/失败/完成后不彻底收尾，留下孤儿子级 —— run 59（`FAILED`，2026-10-27）残留 2 个 `RUNNING` 的 `job_run`（`JOB_UPLOAD_FILES` / `JOB_EXPORT_LOAD`）、61 条 `GCMAIN` 上传动态实例、4 个未终态 `process_run`；run 55（`SUCCESS`）残留 1 个 `RUNNING` 的 `process_run`（`PROCESS_EXPORT_FILE_MON_BASE`）。处置口径：只把「所属 `workflow_run` 已终态而自身仍为 `PENDING`/`READY`/`RUNNING`」的 `job_step_run` / `job_run` / `job_dynamic` / `process_run` / `phase_run` 改为 `CANCELLED` 并补齐结束时间（`finished_at` / `gmt_finish`），**不删除任何记录**，不改动已终态行与 run/phase 的既定结论。 | ① 迁移执行输出 `UPDATE 5 / 5 / 61 / 5 / 1` + `NOTICE: V300081 收敛完成`，重复执行全部为 `UPDATE 0`（幂等）；② 收敛后 `select count(*) from cdeu.finv_quote_cdeu_job_run s join cdeu.finv_quote_cdeu_workflow_run r on r.run_id=s.workflow_run_id where r.status in ('SUCCESS','FAILED','PARTIAL','CANCELLED','PAUSED') and s.status in ('PENDING','READY','RUNNING')` = 0，`job_step_run` / `job_dynamic` / `process_run` / `phase_run` 同类查询同为 0；③ `select run_id,status from cdeu.finv_quote_cdeu_workflow_run where run_id in (55,59)` 仍为 `55=SUCCESS`、`59=FAILED`（查询结论未被改写）；④ `select count(*) from cdeu.finv_quote_cdeu_export_job where locked_by is not null` = 0（无残留锁）；⑤ `public.schema_version` 出现 `300081`，服务 Live/Web 200 | √ |


## 一百九十八、CDEU「JOB 进度」标题去重（10885）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10885 | CDEU 工作流视图「JOB 进度」卡片标题去掉重复计数：原为 `JOB 进度（{N} 个 JOB / {M} 个 Step）`，而卡片正文已展示 `成功 / 总数` 与「失败 · 待执行」，标题里的计数属重复描述。改为纯文本 `JOB 进度`，并删除因此不再被引用的 `jobStepStats` computed（避免 `vue-tsc` 未使用变量报错）。 | `grep -n "JOB 进度" Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 输出为 `<div class="text-caption text-medium-emphasis">JOB 进度</div>`（无 `个 JOB`/`个 Step`）；`grep -c jobStepStats` = 0；`cd Web && npm run build` 通过；页面「JOB 进度」卡片标题仅显示 `JOB 进度`，下方 `成功/总数`、`失败 · 待执行` 指标不变 | √ |


## 一百九十九、「当日累计均价」字段名调整（待办）（10886）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10886 | 【**待办**】「当日累计均价」字段名调整。**现状口径（三处并不一致）**：① 数据库列名 `paocd`，存在于 `cdeu.finv_quote_secu_kline_min`、`cdeu.finv_quote_secu_kline_min_export`、`quote.finv_quote_secu_kline_min` 三张表；② 对外 API 字段名 **`pacd`**（`internal/meta/finv/quote/cdeu/kline/model.go` 的 `ColPaocd = "paocd" // 库列名；对外 API 字段为 pacd`、`request.go` 的 `Pacd OptDec \`json:"pacd"\``）；③ 导出 `.mvsv` V2 头部字段名 `paocd`（`mvsv/writer_v2.go` 的 `v2FieldNames` / `parser_v2.go` 同时接受 `paocd` 与 `pacd`），中文名「累计均价」；④ 库注释为「当日实时累计均价」，API 文档写作「当日累计均价」。**动工前须确认三点**：① 调整范围——只改对外 API 名 / 只改库列名 / 库列+API+导出三处统一 / 连中文名一并调整；② 目标名称（如 `avg_price`、`acc_avg_price`、`pa` 等，由需求方给定）；③ 兼容策略——旧名是否保留（视图、别名列、`parser_v2.go` 继续接受旧名）、已导出的历史 `.mvsv` 文件是否需要兼容读取、`quote.` 表是否随 `cdeu.` 同步改名。 | 待方案确定后细化；初步验收口径：① 新旧名称在库列、Go 常量/结构体 tag、API 文档（`Docs/API/Quote/Cdeu/Min/*`、`Docs/OpenAPI/Quote/Cdeu/Min/*`）、`.mvsv` V2 字段名表四处保持一致（若决定改名）；② `grep -rn "<旧名>" internal/ Docs/ Deploy/` 无遗漏（若保留兼容则仅限显式白名单）；③ 迁移脚本幂等、单事务、末尾收敛校验；④ `go build ./...` 与 `go test ./internal/meta/finv/quote/...` 通过；⑤ 现有 277,295 行 `cdeu.finv_quote_secu_kline_min` 数据在改名后仍可正常读写（抽样 upsert + page 查询） | |


## 二百、CDEU 指标卡片顺序调整（10887）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10887 | CDEU 工作流视图指标卡片顺序调整：「运行时长」卡片由最右侧（第 4 位）移到最左侧（第 1 位），其余三张（工作流状态、阶段进度（参与完成判定）、JOB 进度）相对顺序不变。卡片内容、响应式断点（`cols="12" sm="6" lg="3"`）与统计口径均不改动。 | 提取 `Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 指标卡片区（约第 1303～1356 行）的四张卡片标题，顺序应为「运行时长、工作流状态、阶段进度、JOB 进度」；`grep -c` 该区间 `<v-col cols="12" sm="6" lg="3">` 计数 = 4（无卡片缺失）；`cd Web && npm run build` 通过；页面首屏四张指标卡顺序符合预期 | √ |
| 10888 | BRS 备份登记维护页面实现查询与管理：「备份登记维护」菜单（前端路由 `/Meta/BRS/Backup/Finv/Table`，原为占位页）对接 `brs.brs_bak_rdb_table`：① 查询——关键字（schema / 表名 / 备注）+ 启用状态筛选 + 分页，按 `priority` 倒序；② 新增/编辑——对话框维护 `rdb_type`/`schema_name`/`table_name`/`priority`/`bak_scope`/`bak_method`/`compression`/`flag_upload`/`where_clause`/`remark`；③ 启用/禁用——行内切换 `flag_enable`。接口 `GET\|POST /API/V1/Meta/BRS/Backup/Finv/Table/{List,Save,Toggle}`，URL 查询参数一律小驼峰（`page`/`pageSize`/`keyword`/`flagEnable`），JSON 体 snake_case；**不做删除**；唯一键冲突（表重复登记 / `priority` 重复）返回 400 且消息可读。 | `grep -c 'Meta/BRS/Backup/Finv/Table' internal/api/router.go` ≥ 3；`grep -c 'BrsBackupTableView' Web/src/router.ts` ≥ 1；`go test ./internal/api/...` 通过；运行态：`List` 返回 `{total,list}` 且 `priority` 倒序，`Save`（`idn=0`）后新行出现在列表，重复登记同一 `schema_name+table_name` 返回 400 且消息含「该表已登记」，`priority` 重复返回 400 且消息含「优先级」，`Toggle` 后 `flag_enable` 翻转且带 `flagEnable=1` 查询不再返回该行；`Docs/API/BRS/BackupTable.md` 存在且在 `Docs/API/APIs.md` 与 `Docs/API/README.md` 登记 | √ |
| 10889 | `brs.brs_bak_rdb_table` 增加 `db`（数据库名）列：`varchar(64) NOT NULL DEFAULT 'finv'`，存量登记由默认值回填；唯一约束由 `(rdb_type, schema_name, table_name)` 改为 `(rdb_type, db, schema_name, table_name)`（`rdb_type` 保留，不去除）；备份登记维护页面与接口同步该字段。**本次只加登记字段，备份执行链（`LoadTables` / `pg_dump`）仍连当前库，不按 `db` 分库执行**。 | 迁移 `V200090__brs_bak_rdb_table_add_db.sql` 在存量库执行后：`select character_maximum_length, is_nullable, column_default from information_schema.columns where table_schema='brs' and table_name='brs_bak_rdb_table' and column_name='db'` 为 `64 / NO / 'finv'::character varying`；`select count(*) from brs.brs_bak_rdb_table where db='finv'` 等于表总行数；`pg_get_constraintdef` 中 `uq_brs_bak_rdb_table` 为 `UNIQUE (rdb_type, db, schema_name, table_name)`；`POST .../Table/Save` 不传 `db` 时落库为 `finv`，同一 `db+schema_name+table_name` 重复登记返回 400；`GET .../Table/List` 返回行含 `db`；前端表格与编辑对话框含「数据库」字段；`npx vue-tsc -b` 与 `go test ./internal/api/...` 通过 | √ |
| 10890 | 「对象存储配置」菜单（前端路由 `/Meta/BRS/Backup/Finv/OSSConfig`，原为占位页）实现 `sys.sys_config` 39002（`data_type='4'`，JSON）的可视化展示与配置：① 读取——`enabled`/`provider`/`endpoint`/`bucket`/`region`/`access_key`/`prefix` + `secret_key_set` 标记 + 39002 的 `remark` 说明；② 保存——字段校验（`enabled=true` 时 `endpoint`/`bucket`/`access_key`/`secret_key` 必填；`provider` 首期仅 `s3`；`region` 空→`auto`；`prefix` 空→`brs/` 且自动补结尾 `/`），写回 `json_value` 并更新 `gmt_modify`/`modify_by`，下一轮备份生效（无需重启）；③ 安全——读取**永不回传 `secret_key` 明文**（只给 `secret_key_set`），保存时 `secret_key` 留空表示保持库内现值。 接口 `GET\|POST /API/V1/Meta/BRS/Backup/Finv/OSSConfig/{Get,Save}`。 | `grep -c 'OSSConfig' internal/api/router.go` ≥ 2；`grep -c 'BrsOssConfigView' Web/src/router.ts` ≥ 1；运行态：`Get` 返回 `code=0` 且 `data.secret_key` 为空串、`data.remark` 非空；`Save`（`enabled=false`，只改 `prefix`）返回 `code=0`，再 `Get` 能读回新 `prefix` 且 `secret_key_set` 保持原值；`Save` `enabled=true` 但缺 `bucket` 返回 400 且消息含「bucket」；`Save` `provider='local'` 返回 400 且消息含「只支持 s3」；`select json_value->>'prefix' from sys.sys_config where config_no=39002` 与页面值一致，且 `modify_by` 为当前登录用户；`grep -c 'secret_key' Docs/API/BRS/OSSConfig.md` ≥ 1 且文档声明不回传明文；`go test ./internal/api/...`、`npx vue-tsc -b` 通过 | √ |
| 10891 | `sys.sys_config` 39002（BRS 对象存储配置）的 JSON 内凭证键由 `access_key` / `secret_key` 改名为 `ak` / `sk`：迁移 `V300082` 就地改名（保留原值、仅当旧键存在时执行）；服务层读取新键优先、旧键兜底，保存只写 `ak` / `sk`；HTTP 接口 wire 字段与前端展示**保持** `access_key` / `secret_key`（标签仍是「Access Key ID」「Secret Access Key」）。 | `select json_value::jsonb ? 'access_key', json_value::jsonb ? 'ak', json_value->>'ak', json_value->>'sk' from sys.sys_config where config_no=39002` 结果为 `f / t / 原 AK / 原 SK`；迁移重跑不产生变化（UPDATE 0）；`go test ./internal/meta/finv/brs -run TestOSSSettings` 通过（新键 / 旧键 / 新旧并存 / 新键为空回退四例 + 序列化只写 ak/sk）；`POST .../OSSConfig/Save` 后库里 `json_value ? 'ak'` 为 `t` 且无 `access_key`；`GET .../OSSConfig/Get` 响应仍含 `access_key` 与 `secret_key_set`，`secret_key` 为空串；前端页面标签未变 | √ |
| 10892 | BRS 备份执行能力：① 「备份调度配置」菜单（`/Meta/BRS/Backup/Finv/Schedule`）可视化配置 `sys.sys_config` 39001 的 6 段 cron（秒 分 时 日 月 周），保存时服务端校验语法并返回未来 5 次执行时间，最多 1 分钟生效；② 保留自动模式（定时任务）的同时提供**手动模式**——「备份执行 → 手动备份与日志」菜单（`/Meta/BRS/Backup/Finv/Run`）可随时触发一轮备份；③ 手动与自动**共用同一执行链**，同一时刻只允许一个运行（重复触发 409）；④ 前端实时展示进度（已完成/总表数、当前表、耗时）与运行日志（info/warn/error 分级）；日志按 `seq` 增量拉取（`sinceSeq`），运行中 1 秒、空闲 5 秒轮询；⑤ 运行状态与日志仅存服务内存（重启清空），备份产物仍落磁盘/对象存储。 接口 `GET\|POST /API/V1/Meta/BRS/Backup/Finv/Schedule/{Get,Save}`、`POST .../Run/Manual`、`GET .../Run/Status`。 | `grep -c 'Meta/BRS/Backup/Finv/Schedule' internal/api/router.go` ≥ 2 且 `grep -c 'Meta/BRS/Backup/Finv/Run' internal/api/router.go` ≥ 2；`grep -c 'brs-schedule\|brs-run' Web/src/router.ts` ≥ 2 且 `grep -c "brs-run" Web/src/config/menuConfig.ts` ≥ 1；`go test ./internal/meta/finv/brs` 通过（`TestNextRuns`/`TestRunnerBusy`/`TestRunnerStatusSinceSeq`/`TestRunnerLogLimit`）；运行态：`Schedule/Get` 返回 5 条 `next_runs`；`Schedule/Save` 传 `0 0 3 * * *` 返回 `code=0` 且再 `Get` 一致，传 `0 0 3 * *` 返回 400；`Run/Manual` 返回 `status=running` 与 `run_id`，轮询 `Run/Status?sinceSeq=<last_seq>` 能看到 `done` 递增、日志追加，结束后 `status` 为 `succeeded`（或 `failed` 且 `error` 非空）；运行中再次调用 `Run/Manual` 返回 409；备份目录下生成当日 `*.dump`；`npx vue-tsc -b` 通过 | √ |
| 10893 | BRS 备份路径与文件命名调整：① 备份根目录不再由代码自作主张拼产品名段——默认值改为 `/data/App/Backup/BRS`（`FINV_BRS_BACKUP_DIR` 显式配置时以其为准），Docker compose 透传该变量并把 `/data/App/Backup` 挂载为宿主目录（默认 `FINV_BRS_HOST_DIR=./data/App/Backup`）；② 备份文件名去掉时间戳后缀，由 `<schema>.<table>_<HHmmss>.dump` 改为 `<schema>.<table>.dump`（本地与对象存储 key 同步）。同一张表当天重复备份会覆盖上一次，属预期行为。 | `grep -c 'FINV_BRS_BACKUP_DIR' Deploy/Docker/docker-compose.yml` ≥ 1；`grep -c 'FINV_BRS_HOST_DIR' Deploy/Docker/docker-compose.yml` ≥ 1；`grep -rn 'FinvQuant/Backup/BRS' internal/ cmd/` 无输出且 `grep -c 'envOr("FINV_BRS_BACKUP_DIR", "/data/App/Backup/BRS")' internal/config/config.go` = 1；`grep -c 'table).dump\|s):\\.\\.dump' internal/meta/finv/brs/brs.go` ≥ 1 且 `grep -c '150405' internal/meta/finv/brs/brs.go` = 0；运行态：手动备份后日志中的文件路径形如 `/data/App/Backup/BRS/<yyyyMMdd>/sys.sys_user.dump`（无产品名段、无时间戳），宿主 `FINV_BRS_HOST_DIR` 下可见同名文件；`go test ./internal/meta/finv/brs ./internal/docs` 通过 | √ |
| 10894 | 把 `public` 与 `futu` schema 中的表加入 BRS 备份清单：`futu` 6 张（`finv_futu_mapping_cs_market`/`mapping_exchange`/`mapping_market_code`/`mapping_region`/`mapping_security`、`finv_futu_quote_collect`，priority 800～750）、`public` 7 张（`finv_currency`/`finv_exchange`/`finv_market`/`finv_region`/`finv_security`、`schema_version`、`schema_version_plan_shift`，priority 740～680），均为 `bak_scope=both`、`pg_dump`、`zstd`、`flag_upload='1'`、`db='finv'`；登记总数 12 → 25。迁移幂等（`ON CONFLICT DO NOTHING`）；末尾**只校验本清单 13 张表已登记**，不要求「schema 下所有表都已登记」——各环境历史遗留表不同（如生产库 `public.finv_quote_secu_kline_min`），全表校验会让迁移在其它环境失败、服务无法启动（2026-11-01 线上事故）。 | `test -f Deploy/Migrations/V300083__brs_backup_table_public_futu.sql`；`select count(*) from brs.brs_bak_rdb_table where schema_name in ('public','futu')` = 13；`select count(*) from brs.brs_bak_rdb_table` = 25 且 `select count(distinct priority)` = 25；`select version, success from public.schema_version where version='300083'` 为 `t`；手动备份（`POST .../Run/Manual`）后 `Run/Status` 的 `total=25`、`failed=0`，`/data/App/Backup/BRS/<yyyyMMdd>/` 下出现 `public.*.dump` 与 `futu.*.dump` 各对应份数；迁移重跑不新增行（`INSERT 0`） | √ |
| 10895 | 退役号段平移脚本（需求 10881 的一次性存量库过渡）：删除 `Deploy/Upgrade/2611-001-MigrationSegmentsShift.sql`，并由迁移 `V300084__drop_schema_version_plan_shift.sql` 删除其防重跑 marker 表 `public.schema_version_plan_shift`，同时删除 `brs.brs_bak_rdb_table` 中该表的备份登记（备份总数 25 → 24）。迁移前置校验 `public.schema_version` 无 <100000 的旧号段记录，否则整事务回滚。**授权**：删除表属 `DbSchemaSpec` §0 红线操作，由用户在会话中明确点名（选项 B）。 | `test ! -f Deploy/Upgrade/2611-001-MigrationSegmentsShift.sql`；`test -f Deploy/Migrations/V300084__drop_schema_version_plan_shift.sql`；`select version, success from public.schema_version where version='300084'` 为 `t`；`select count(*) from pg_tables where schemaname='public' and tablename='schema_version_plan_shift'` = 0；`select count(*) from brs.brs_bak_rdb_table where table_name='schema_version_plan_shift'` = 0 且 `select count(*) from brs.brs_bak_rdb_table` = 24；手动备份 `Run/Status` 的 `total=24`、`failed=0`；`grep -rn '2611-001' Deploy/ Docs/DevSpec/` 仅剩 CHANGELOG 历史条目；迁移重跑无副作用 | √ |
| 10896 | 发放运行环境标识配置：新增一次性初始化脚本 `Deploy/Init/2610-002-AddRuntimeEnvConfig.sql`，在 `sys.sys_config` 写入 `config_no=1002`（`data_type='3'`，值存 `str_value`），由部署侧在目标库手工执行一次，用 `-v env=<dev\|test\|uat\|prod\|...>` 指定当前实例运行环境；`overwrite` 缺省 0（已存在时不覆盖，避免误改生产标识），传 1 才覆盖并刷新 `gmt_modify`/`modify_by`；取值校验 2~32 位字母/数字/下划线/短横线；参数非法或 `sys.sys_config` 不存在时以非 0 退出码中止。脚本不进入迁移链（环境取值随部署而定，不能由迁移统一套用）。 | `test -f Deploy/Init/2610-002-AddRuntimeEnvConfig.sql`；`psql -v env=prod -f Deploy/Init/2610-002-AddRuntimeEnvConfig.sql` 后 `select config_no,data_type,str_value,create_by from sys.sys_config where config_no=1002` 为 `1002 / 3 / prod / init`，`remark` 为 `当前实例的运行环境标识（如 Dev / Test / Uat / Prod）；由部署侧一次性写入，服务只读展示`；重复执行且不带 `overwrite` 时 `str_value` 不变且输出「已存在且未覆盖」；带 `-v overwrite=1 -v env=test` 后变为 `test`；`-v env=x` 退出码非 0 且输出「env 取值非法」；`grep -c 'quit 1' Deploy/Init/*.sql` 为 0 | √ |
| 10897 | BRS 对象存储上传路径增加运行环境层级：key 由 `<prefix>/<yyyyMMdd>/<schema>.<table>.dump` 改为 `<prefix>/<env>/<yyyyMMdd>/<schema>.<table>.dump`，其中 `prefix` 仍是 39002 里用户指定的路径前缀（如 `BRS/Finv/`），`env` 取 `sys_config` 1002 的 `str_value`（如 `dev`/`test`/`uat`/`prod`）；每轮备份开始时读取一次。1002 缺失、读取失败或取值不合法（非 2~32 位字母/数字/下划线/短横线）时**跳过对象存储上传**并记 warn，本地备份保留——避免不同环境的备份混到同一路径。 | `grep -c 'EnvConfigNo' internal/meta/finv/brs/brs.go` ≥ 2；`grep -c 'func validEnvName' internal/meta/finv/brs/brs.go` = 1；`go test ./internal/meta/finv/brs -run TestValidEnvName` 通过；运行态（1002=`prod`）：手动备份日志含「上传环境目录：prod」，上传行形如 `已上传 prod/20261101/sys.sys_user.dump`，R2 上对象路径为 `BRS/Finv/prod/20261101/sys.sys_user.dump`；删除 1002 后再备份：日志出现「未配置运行环境标识（sys_config 1002），跳过对象存储上传」且无上传行、本地 dump 仍生成；1002 设为 `prod/x` 时同样跳过上传 | √ |
| 10898 | BRS 备份文件名增加数据库名前缀：由 `<schema>.<table>.dump` 改为 **`<db>.<schema>.<table>.dump`**（如 `finv.sys.sys_config.dump`），本地文件名与对象存储 key 同步；`<db>` 取登记表 `brs.brs_bak_rdb_table.db`，为空时回落连接库 `FINV_PG_DATABASE`。 | `grep -c 'fmt.Sprintf("%s.%s.%s.dump"' internal/meta/finv/brs/brs.go` = 1；`grep -c 'SELECT idn, db, schema_name' internal/meta/finv/brs/brs.go` = 1；运行态：手动备份日志路径形如 `/data/App/Backup/BRS/<yyyyMMdd>/finv.sys.sys_user.dump`，上传行形如 `已上传 Local/20261101/finv.sys.sys_user.dump`，目录下所有文件均以 `finv.` 开头；`pg_restore -l` 可正常列出文件；`go test ./internal/meta/finv/brs ./internal/docs` 通过 | √ |
| 10899 | 清理历史遗留表 `public.finv_quote_secu_kline_min`（早期版本把分钟 K 线建在 `public`，数据已由 `Deploy/Upgrade/2610-004-MigratePublicKlineToCdeu.sql` 搬入 `cdeu`；该表不在 `DbSchemaSpec` §0.2 保护名单，代码无业务读写）。迁移 `V300085__drop_public_kline_min.sql` 按安全策略执行：源表不存在→跳过；目标表缺失或源表缺 `usc`/`secu_code` 列→保留并 `WARNING`；按 `(ts, usc)` 差集不为 0→保留并 `WARNING`；差集为 0→`DROP TABLE`；`DROP` 因视图/外键依赖失败→捕获异常、保留表、迁移仍成功（避免拖垮服务启动）。执行前需运维先 `pg_dump` 备份该表。 **授权**：删除表属 `DbSchemaSpec` §0 红线操作，由用户明确选择方案 A。 | `test -f Deploy/Migrations/V300085__drop_public_kline_min.sql`；本机三分支验证：源表存在且差集 2 行时输出 `WARNING ... 保留该表不删除` 且表仍在；补插 `cdeu` 对应行后重跑输出 `NOTICE ... 已删除 ... 差集 0` 且 `to_regclass` 为 NULL；源表不存在时输出 `NOTICE ... 不存在，跳过`；`select version, success from public.schema_version where version='300085'` 为 `t`；生产执行前 `pg_dump -t public.finv_quote_secu_kline_min -Fc` 产物存在于 `Deploy/Backup/`；`go test ./internal/docs` 通过 | √ |
| 10900 | 暂缓删除 `public.finv_quote_secu_kline_min`，先备份到对象存储（R2）留档：① 迁移 `V300085` 改为只在差集为 0 时输出 `NOTICE`，**不执行 `DROP`**；② 新增迁移 `V300086` 把该表加入 BRS 备份登记（priority 660、`both`/`pg_dump`/`zstd`/`flag_upload='1'`），**仅当表存在时登记**，避免全新库产生指向不存在表的登记；③ 新增一次性脚本 `Deploy/Upgrade/2610-003-DropPublicKlineMin.sql`：再次校验 `(ts, usc)` 差集，为 0 才 `DROP TABLE`，并同步删除 `brs.brs_bak_rdb_table` 中的该表登记；差集不为 0 时 `RAISE EXCEPTION` 整体回滚。后续「还原删除要求」= 备份完成后执行该 Upgrade 脚本（无需再改迁移）。 | `grep -c 'DROP TABLE' Deploy/Migrations/V300085__drop_public_kline_min.sql` = 0（迁移内不删表）且 `grep -c 'DROP TABLE' Deploy/Upgrade/2610-003-DropPublicKlineMin.sql` ≥ 1；`select count(*) from brs.brs_bak_rdb_table where table_name='finv_quote_secu_kline_min'` 在表存在时为 1、表被删除后为 0；表存在时手动备份日志出现 `finv.public.finv_quote_secu_kline_min.dump` 且上传成功；执行 Upgrade 脚本后 `to_regclass('public.finv_quote_secu_kline_min')` 为 NULL、登记被移除、后续备份不报错；`go test ./internal/docs` 通过 | √ |
| 10901 | BRS 备份目录在裸机部署下的可用性修复：① `FINV_BRS_BACKUP_DIR` 的代码默认值回到 Native 安装约定 `<PREFIX>/Backup/BRS`（即 `/data/App/FinvQuant/Backup/BRS`），不再落到 `/data/App/Backup/BRS`——裸机 `/data/App` 往往只读或不属于运行用户，首次备份会 `mkdir: read-only file system`；容器部署由 compose 显式传 `/data/App/Backup/BRS`，不受影响；② `Install.sh` 预建 `<PREFIX>/Backup/BRS`（属主运行用户，0750），并在新建/既有 `finvquant.env` 中补齐 `FINV_BRS_BACKUP_DIR`（不覆盖已有值）；③ `Upgrade.sh` 同样预建目录并补齐老 env 的缺失项（老版本 env 无该键正是本次线上故障场景）；④ 备份时创建目录失败的报错补充可操作提示（`请检查 FINV_BRS_BACKUP_DIR（当前 …）是否存在且运行用户可写`）。 | `grep -c 'envOr("FINV_BRS_BACKUP_DIR", "/data/App/FinvQuant/Backup/BRS")' internal/config/config.go` = 1；`grep -c 'defaultBackupDir = "/data/App/FinvQuant/Backup/BRS"' internal/meta/finv/brs/brs.go` = 1；`grep -c "BACKUP_DIR}/BRS" Deploy/Native/Install.sh Deploy/Native/Upgrade.sh` 各 ≥ 2；`bash -n` 两脚本通过；裸机验证：老 env（无 `FINV_BRS_BACKUP_DIR`）执行 `Upgrade.sh` 后日志含「已补齐 FINV_BRS_BACKUP_DIR=…」且 `ls -ld <PREFIX>/Backup/BRS` 属主为运行用户；手动备份不再出现 `创建备份目录失败`，产物落在 `<PREFIX>/Backup/BRS/<yyyyMMdd>/`；容器验证：`docker exec finvquant env \| grep FINV_BRS_BACKUP_DIR` 仍为 `/data/App/Backup/BRS` | √ |
| 10902 | 把 K 线字段 `paocd` / `pacd` 统一为 `vwap`（成交量加权平均价）：① 迁移 `V300087` 对 `quote.finv_quote_secu_kline_min`、`cdeu.finv_quote_secu_kline_min`、`cdeu.finv_quote_secu_kline_min_export` 执行 `RENAME COLUMN paocd TO vwap`（幂等 + 收敛校验，历史迁移里的 `paocd` 字样不动）；② API 出参 `/Add`、`/Query` 字段 `pacd` → `vwap`，`/Page`、`/PageUni` 第 13 项短名 `pa` → `vwa`（`column` 仍固定 15 项、顺序不变）；③ API 入参优先 `vwap`，未传或显式 `null` 时回退旧字段 `pacd`，`/Upsert`、`/Batch` 增量更新同样回退；④ MVSV-2 导出列名/`FieldName`/中文名改 `vwap`/`Vwap`/「成交量加权平均价」，导入兼容 `paocd`、`pacd`、`vwap` 三名额；⑤ 同步代码、文档（`Docs/API`、`Docs/OpenAPI`、`Docs/DataFormat`、`Docs/Design`、`Docs/FinvQuant/Brs.md` 旧备份恢复提示）与过程清单 [`Docs/Refactor/2610-013-PaocdToVwap.md`](Refactor/2610-013-PaocdToVwap.md)。 | `select column_name from information_schema.columns where table_name like 'finv_quote_secu_kline_min%' and column_name in ('paocd','vwap')` 只返回 `vwap`（3 行）；`grep -c '"pacd"' internal/meta/finv/quote/cdeu/kline/*.go` 只出现在兼容注释/字段定义；`go test ./internal/meta/finv/quote/...` 通过（含 `kline/vwap_compat_test.go`、`mvsv/vwap_compat_test.go`）；`/Page`、`/PageUni` 响应中 `column[12]` 为 `vwa`；`/Add`、`/Query` 响应含 `vwap`、不含 `pacd`；`pacd` 入参仍能落库为 `vwap`；`qmio export` 产物 `# Field` 含 `vwap`、不含 `paocd`；旧 `paocd`/`pacd` 文件可正常 `qmio import` | √ |
| 10903 | **待办**：在需求 10902 的兼容期结束后，移除 `pacd` / `paocd` 旧字段名的兼容分支——`internal/meta/finv/quote/cdeu/kline/request.go` 的 `Pacd` 入参、`internal/meta/finv/quote/mvsv/parser_v2.go` 与 `cmd/qmio/import.go` 的 `paocd` / `pacd` 分支。触发条件：前端与外部调用方全部切到 `vwap`（含历史 `.mvsv` 文件无需再导入）。 | `grep -rn 'json:"pacd"' internal` 无输出；`grep -rn '"paocd", "pacd"' internal cmd` 无输出；`go test ./internal/meta/finv/quote/...` 通过（此时 `vwap_compat_test.go` 的兼容用例应同步删除）；旧文件导入能力若保留，须在 `Docs/DataFormat/MvsvFileFormat.md` 明确写出口径 | 待办 |
| 10904 | 数据搬运工具二进制改名 `qmio` → `qiom`：① 源码目录 `cmd/qmio` → `cmd/qiom`（含 usage、注释、测试）；② 安装/升级/卸载脚本：二进制 `<PREFIX>/bin/qiom`、软链 `/usr/local/bin/qiom`、备份 `qiom.prev`，并在安装/升级时清理旧名残留（`bin/qmio`、`qmio.prev`、`/usr/local/bin/qmio`）；③ 打包脚本产物名 `qiom`；④ 文档改名 `Docs/FinvQuant/Qmio.md` → `Qiom.md`、`Docs/Design/QmioDesign.md` → `QiomDesign.md` 并同步全部引用（`Deploy/Native/Install.md`、`README.md`、`Docs/DataFormat/MvsvFileFormat.md`、`Docs/Design/BrsBackupDesign.md`、`Docs/Refactor/2610-013-PaocdToVwap.md`）。历史 CHANGELOG/RequirementList 条目与历史 Refactor 文档保持原样。 | `find . -name 'qmio*' -not -path './.git/*' -not -path './Web/node_modules/*'` 无输出（仅历史文档提及旧名不算文件）；`go build ./cmd/qiom` 通过且 `./qiom --help` 输出以 `qiom` 开头；`grep -rn 'cmd/qmio' --include='*.go' --include='*.sh' Deploy internal cmd` 无输出；`bash -n` 三个脚本通过；安装后 `ls -l <PREFIX>/bin/qiom /usr/local/bin/qiom` 存在且指向一致；升级/安装脚本输出包含「已清理旧名残留」且 `bin/qmio`、`/usr/local/bin/qmio` 不再存在；`grep -rn 'QmioDesign.md\|Qmio.md' --include='*.md' Docs Deploy` 只命中历史 CHANGELOG/Refactor；`go test ./internal/docs` 通过 | √ |
| 10905 | 大表增量（分片）备份：与小表全量备份**完全独立**的一套策略——① 新增登记表 `brs.brs_bak_rdb_table_incr`（迁移 `V300088` 建表、`V300090` 由 `_slice` 改名为 `_incr`，不修改 `brs_bak_rdb_table_full`），字段含 `slice_mode`/`slice_key_sql`/`slice_where_tpl`/`slice_file_tpl`/`min_rows`/`max_slices`/`redo_slices`；② 按分片维度切分导出（`date`/`week`/`month` 内置模板，`column`/`expr` 自定义），分片写成 **MVSV-2** 文件（复用 `mvsv.WriteV2`，与 `qiom export` 同格式，gzip 压缩；`pg_dump` 无行级过滤不能用），目标表须与 `cdeu.finv_quote_secu_kline_min` 同构并在保存登记时校验；③ 增量以产物存在为准（本地文件或远端对象已存在即跳过），末尾 `redo_slices` 个分片每轮重做，不新增状态表；④ 产物：本地 `<备份目录>/Incr/<db>.<schema>.<表>/<分片键>.mvsv.gz`，远端 `<prefix>/<env>/Incr/<db>.<schema>.<表>/<分片键>.mvsv.gz`；⑤ 接口 `Slice/Rule/{List,Save,Toggle}` + `Slice/Plan` + `Slice/Run/{Manual,Status}`，页面「备份管理 → 大表增量备份」；本期只做手动触发，定时待验收后再定。 | `select count(*) from brs.brs_bak_rdb_table_incr` ≥ 1 且含 `cdeu.finv_quote_secu_kline_min`（`slice_mode=date`）；`go test ./internal/meta/finv/brs/ ./internal/api/...` 通过；`npx vue-tsc -b` 通过；首轮手动备份产出 `20260930.mvsv.gz` 等分片文件并上传 `<env>/Incr/finv.cdeu.finv_quote_secu_kline_min/20260930.mvsv.gz`；第二轮仅重做末尾 `redo_slices` 个分片（日志出现「已备份，跳过」）；`qiom verify <分片>.mvsv.gz` 通过（`Format=MVSV-2`、`Count` 与源表该日行数一致）；`qiom import <分片>.mvsv.gz` 可恢复 | √ |
| 10906 | MVSV-2 数据区**字段转义**（允许文本列包含分隔符，不做禁用）：① 写侧对 `ext_field`/`remark`/`provider` 等字段做可逆转义——`\\`→`\\\\`、`\|`→`\\\|`、换行→`\n`、回车→`\r`，头部写出 `# Escape : "backslash"`；② 读侧**仅当头部声明该键**时按转义解析，历史文件（无该头）沿用 `Split("\|")`，行为不变；③ 历史文件若含裸 `\|` 导致列数不符，错误信息提示缺少 `# Escape` 声明；④ 未知转义序列按原样保留，避免吞掉旧数据里的裸反斜杠；⑤ 覆盖 `qiom export/import/verify`、CDEU 导出与 BRS 大表分片备份（共用 `mvsv.WriteV2`）。 | `go test ./internal/meta/finv/quote/mvsv/` 通过（含 `escape_test.go`：往返、未知转义、历史文件兼容、缺头提示）；导出含 `ext_field='source\|vendor'`、`remark` 含换行的数据后，分片文件头部含 `# Escape : "backslash"` 且数据行为 `source\\\|vendor`、`第一行\\n第二行`；`qiom verify <文件>` 通过且 `Count` 与统计一致；`qiom import <文件>` 后库内 `ext_field` 还原为 `source\|vendor`、`remark` 含真实换行；无 `# Escape` 头的历史文件仍能正常导入 | √ |
| 10907 | BRS 备份目录从 `<PREFIX>/Backup/BRS` 迁到 **`<PREFIX>/BRS`**（`/data/App/FinvQuant/BRS`）：`Backup` 是数据库逻辑备份目录（`Upgrade.sh` 写入，root 属主），把需要运行用户可写的 BRS 产物放在其下容易产生属主/权限互相干扰。① `FINV_BRS_BACKUP_DIR` 代码默认值改为 `/data/App/FinvQuant/BRS`；② `Install.sh`/`Upgrade.sh` 新增 `BRS_DIR=<PREFIX>/BRS`，预建 `BRS` 与 `BRS/Slice`（属主运行用户 0750），env 补齐/改写为 `${BRS_DIR}`；`Upgrade.sh` 检测到旧 `Backup/BRS` 有产物时打印迁移命令（不自动搬动生产数据）；③ 容器内路径与 Native 统一为 `/data/App/FinvQuant/BRS`（volume 宿主目录 `FINV_BRS_HOST_DIR` 不变）；④ 文档同步（`Brs.md`、`BrsBackupDesign.md`、`Install.md`、`API/BRS/{Run,Slice}.md`）。 | `grep -c 'envOr("FINV_BRS_BACKUP_DIR", "/data/App/FinvQuant/BRS")' internal/config/config.go` = 1；`grep -c 'defaultBackupDir = "/data/App/FinvQuant/BRS"' internal/meta/finv/brs/brs.go` = 1；`grep -c 'BRS_DIR="${PREFIX}/BRS"' Deploy/Native/Install.sh Deploy/Native/Upgrade.sh` 各 ≥ 1 且 `grep -c 'BACKUP_DIR}/BRS' Deploy/Native/Install.sh` = 0；`bash -n` 两脚本通过；`grep -c 'FINV_BRS_BACKUP_DIR: ${FINV_BRS_BACKUP_DIR:-/data/App/FinvQuant/BRS}' Deploy/Docker/docker-compose.yml` = 1；运行态：`Run/Status` 的 `backup_dir` = `/data/App/FinvQuant/BRS`（Native 与容器一致），`backup_dir_ok=true`；手动备份产物落在 `<BRS_DIR>/<yyyyMMdd>/`、分片产物落在 `<BRS_DIR>/Slice/`；`go test ./internal/docs ./internal/meta/finv/brs` 通过 | √ |
| 10908 | 小表全量备份登记表改名 `brs.brs_bak_rdb_table` → **`brs.brs_bak_rdb_table_full`**，与大表增量登记 `brs_bak_rdb_table_incr` 并列、职责一眼可辨（`_full` 全量 / `_incr` 增量）：① 迁移 `V300091` 改名表，并把约束（主键/唯一/CHECK/NOT NULL）、触发器 `trg_brs_bak_rdb_table_full_gmt_update`、identity 序列 `brs_bak_rdb_table_full_idn_seq` 一并改名，幂等；② 代码（`LoadTables`、登记维护 CRUD、唯一约束冲突识别）与现行文档同步；`V300060` 等历史迁移保持原样不动。 | `select to_regclass('brs.brs_bak_rdb_table_full')` 非空且 `select to_regclass('brs.brs_bak_rdb_table')` 为 NULL；`select count(*) from brs.brs_bak_rdb_table_full` = 24（种子行数不变）；`select count(*) from pg_constraint where conrelid='brs.brs_bak_rdb_table_full'::regclass and conname like '%brs_bak_rdb_table_%' and conname not like '%_full%'` = 0；`grep -rn 'brs_bak_rdb_table\b' --include='*.go' internal` 无输出；`grep -rn 'brs_bak_rdb_table\b' --include='*.md' Docs Deploy`（排除 RequirementList/CHANGELOG 历史条目）无输出；`go test ./internal/meta/finv/brs ./internal/api/... ./internal/docs` 通过；手动备份一轮仍按 24 张表执行 | √ |
| 10909 | 新增 BRS RDB 数据源登记表 **`brs.brs_bak_rdb_datasource`**（迁移 `V300092`），**只建表、业务不实现**：字段含 `code`（唯一，供备份登记引用）、`name`、`rdb_type`（首期仅 `postgresql`）、`host`、`port`、`database_name`、`username`、`password_cipher`（加密口令，空则回退 `FINV_PG_PASSWORD`）、`ssl_mode`、`connect_timeout`、`is_default`（全表最多一条 `1`，部分唯一索引）、`flag_enable`、`priority`、`remark`、`creator`、`gmt_*`；种子：由迁移 `V300093` 写入 1 条 `code=finv` 的示例数据源（`127.0.0.1:5432/finv`、`password_cipher` 留空回退 `FINV_PG_PASSWORD`、`is_default=1`），**仅为示例**，备份链路仍只读 `FINV_PG_*` 环境变量。 | `select version, success from public.schema_version where version='300092'` 为 `t`；`select to_regclass('brs.brs_bak_rdb_datasource')` 非空；`\d brs.brs_bak_rdb_datasource` 含上述 18 个字段、6 个 CHECK、唯一约束 `code`/`name`、部分唯一索引 `uq_brs_bak_rdb_datasource_default`、触发器 `trg_brs_bak_rdb_datasource_gmt_update`；插入两条 `is_default='1'` 时第二条报 `uq_brs_bak_rdb_datasource_default`；`port=70000` 报 `ck_brs_bak_rdb_datasource_port`；`UPDATE` 后 `gmt_update > gmt_create`；`select count(*) from brs.brs_bak_rdb_datasource` = 1 且 `code=finv`、`is_default=1`、`password_cipher IS NULL`；重复执行 `V300093` 不新增行；`go test ./internal/docs` 通过 | √ |
| 10910 | BRS 本地备份产物自动清理，避免「小表全量」按天累积占满磁盘：① 新增配置 `FINV_BRS_LOCAL_KEEP_DAYS`（默认 7，`<=0` 不清理）；② 每轮全量备份结束（成功或失败）自动清理 `<备份目录>/<yyyyMMdd>/` 下超过保留期的 `*.dump`；③ **只删除远端对象已存在（`Store.Stat` 命中）的文件**，去重键为 `<env>/<yyyyMMdd>/<file>`；远端不存在（上传曾失败）则保留，空目录一并删除；④ 对象存储未启用或运行环境标识（1002）缺失/非法时**整体跳过**（本地是唯一副本）；`Incr/` 分片目录与非 `yyyyMMdd` 目录不参与清理；⑤ 日志输出删除文件数、释放字节数、保留文件数与保留期；大表分片按分片键覆盖、文件数等于分片数不随时间增长，不做自动删除。 | `grep -c 'FINV_BRS_LOCAL_KEEP_DAYS' internal/config/config.go Deploy/Native/FinvQuant.env.example Deploy/Docker/docker-compose.yml` 各 ≥ 1；`go test ./internal/meta/finv/brs/` 通过（`cleanup_test.go` 5 个用例：已上传才删、保留期内不删、无 store 跳过、`keepDays=0` 跳过、`Incr/` 不受影响）；运行态：手工造一个 8 天前的日期目录并只把其中一个 dump 上传到远端，跑一轮后该 dump 被删、未上传的保留，日志出现「本地清理完成：删除 N 个文件」；`FINV_BRS_LOCAL_KEEP_DAYS=0` 时日志为「本地备份清理未启用」，文件不动 | √ |
| 10911 | BRS 磁盘水位告警：① 新增 `internal/meta/finv/brs/disk.go`（`checkDisk` + `existingAncestor`），Linux/macOS 用 `syscall.Statfs` 取备份目录所在文件系统的可用/总空间，目录尚未创建时沿父目录向上取最近已存在目录；Windows 返回「未知」不影响备份；② 阈值由 `FINV_BRS_DISK_MIN_FREE_GB` 控制（默认 5 GB，`<=0` 关闭）；③ 检测时机：运行器启动、每轮备份开始前与结束后各一次；低于阈值写 `warn` 日志（含可用/阈值/路径与清理建议）；④ 状态接口（`Run/Status`、`Slice/Run/Status`）新增 `disk_path`/`disk_free_bytes`/`disk_total_bytes`/`disk_min_free_bytes`/`disk_low`/`disk_error`；⑤ 页面「小表全量备份」「大表增量备份」顶部黄色告警条 + 状态行显示磁盘剩余与阈值。 | `grep -c 'FINV_BRS_DISK_MIN_FREE_GB' internal/config/config.go Deploy/Native/FinvQuant.env.example Deploy/Docker/docker-compose.yml` 各 ≥ 1；`go test ./internal/meta/finv/brs/` 通过（`disk_test.go`：可用/总空间非 0、阈值换算、超大阈值判低、`<=0` 关闭、目录不存在时回落父目录）；`npx vue-tsc -b` 通过；运行态：`Run/Status` 返回 `disk_free_bytes > 0` 且 `disk_low=false`；把 `FINV_BRS_DISK_MIN_FREE_GB` 调到远大于可用空间后重启并触发一轮，日志出现「磁盘剩余空间偏低」且接口 `disk_low=true`；`FINV_BRS_DISK_MIN_FREE_GB=0` 时 `disk_low` 恒为 false | √ |
| 10912 | 39002 对象存储配置格式改为 `{"enable":…,"protocol":"s3","provider":"cloudflare-r2",…,"ak":…,"sk":…}`：① 键改名 `enabled`→`enable`；② 原 `provider`（协议）拆为 `protocol`（协议：`s3`/`local`）+ `provider`（厂商标签：`cloudflare-r2`/`aws-s3`/自定义）；③ **保存一律写新键**，不再写出 `enabled`/`protocal`/`access_key`/`secret_key`；④ **升级兼容**：读取时 `enable` 优先 `enabled`、`protocol` 兼容误拼别名 `protocal`、旧格式 `"provider":"s3"` 视为只有协议并按 endpoint 推断厂商（含 `r2.cloudflarestorage.com`→`cloudflare-r2`，否则 `aws-s3`）、旧凭证键 `access_key`/`secret_key` 兜底；⑤ 适配器由 `protocol` 决定（`s3`→`oss/s3`，`local`→`oss/local`），`provider` 仅作标签；未知协议 400 拒绝；⑥ 迁移 `V300094` 只更新 39002 的 `remark` 说明，**不改 `json_value`**（避免升级时动用户配置）；前端表单改为 `enable`/`protocol`/`provider` 三项。 | `grep -c 'normalizeOSS' internal/meta/finv/brs/ossconfig.go` ≥ 3；`go test ./internal/meta/finv/brs/ -run NormalizeOSS` 与 `go test ./internal/meta/finv/brs/ -run Protocol` 均通过（归一化 8 例 + 兼容读取 4 例 + 序列化只写新键）；`npx vue-tsc -b` 通过；运行态：库里保持旧 JSON 时 `GET /OSSConfig/Get` 返回 `enable=true`/`protocol=s3`/`provider=cloudflare-r2` 且备份上传正常；`POST /OSSConfig/Save` 后 `SELECT json_value FROM sys.sys_config WHERE config_no=39002` 只剩新键（无 `enabled`/`access_key`/`secret_key`）；用旧格式（`enabled`+`provider:"s3"`）POST 仍能保存成功 | √ |
| 10913 | 前端操作反馈提示统一为悬浮 Toast：① 新增 `Web/src/ui/toast.ts`（全局队列 + `showToast(text, type, timeout=5000)` + `dismissToast`）与 `Web/src/components/AppToast.vue`（顶部居中悬浮、不占布局、5 秒自动消失、可手动关闭、FIFO 不堆叠、长文案折行），在 `App.vue` 挂载一次；② 所有页面把「保存成功/已启用/已开始备份」等一次性提示从内嵌 `v-alert` + `message` ref 改为 `showToast`（14 个页面，含 BRS 5 个页面），操作类失败（非 `load*` 函数内的 `error`）也改 `showToast(msg, 'error')`；③ 加载失败（`load*` 的 catch）、持续状态（运行中/磁盘水位/备份目录不可用）、表单字段校验仍保留内联 `v-alert`；④ 删除页面内自建 `v-snackbar`（`EnvironmentView` 与 `App.vue` 改密提示统一走 `AppToast`）；⑤ `Docs/DevSpec/UiSpec.md` 新增 §4「操作反馈提示（Toast / Snackbar）规范」（统一入口、禁止事项、内联适用场景、自检命令），并在 `AGENTS.md` 规范表登记。 | `grep -rn 'const message = ref' Web/src/views` 为 0 行；`grep -rn 'v-snackbar' Web/src/views` 为 0 行；`cd Web && npx vue-tsc -b` 通过；人工回归：保存对象存储配置后提示悬浮在顶部、约 5 秒自动消失且页面布局不跳动，点关闭按钮可立即消失，连续触发两次操作时两条提示依次展示不叠加；`Docs/DevSpec/UiSpec.md` 含「## 4. 操作反馈提示」章节 | √ |
| 10914 | 升级/安装脚本纠正 10907 之前的 BRS 备份目录旧默认值：`Install.sh` 与 `Upgrade.sh` 在 env 已有 `FINV_BRS_BACKUP_DIR` 时原先「不覆盖」，导致旧值 `<PREFIX>/Backup/BRS` 残留、页面继续显示旧路径；改为三态处理——① 无该键 → 补齐 `<PREFIX>/BRS`；② 值为旧默认 `<PREFIX>/Backup/BRS` → **自动改为 `<PREFIX>/BRS`** 并打印变更；③ 用户自定义的其它路径 → **保持不动**。同时修正遗留旧路径文本（`brs.go` 注释、`PlaceholderView` 备份记录说明）。 | `bash -n Deploy/Native/Install.sh` 与 `bash -n Deploy/Native/Upgrade.sh` 通过；三分支验证：env 无键 → 追加 `<PREFIX>/BRS`；值为 `<PREFIX>/Backup/BRS` → 改为 `<PREFIX>/BRS`；值为 `/mnt/bigdisk/brs` → 不变；`grep -rn 'Backup/BRS' --include=*.go --include=*.vue` 仅剩 Upgrade.sh 的迁移提示注释；升级后页面「备份目录」显示 `<PREFIX>/BRS`（旧产物仍在 `Backup/BRS` 时脚本已打印 `cp -a` 迁移提示） | √ |
| 10915 | 小表全量备份增加 `Full/` 目录段，与大表增量的 `Incr/` 对称：① 本地路径由 `<备份目录>/<yyyyMMdd>/` 改为 `<备份目录>/Full/<yyyyMMdd>/`；② 远端 key 由 `<prefix>/<env>/<yyyyMMdd>/<db>.<schema>.<table>.dump` 改为 `<prefix>/<env>/Full/<yyyyMMdd>/<db>.<schema>.<table>.dump`；③ 本地清理逻辑同时处理新位置 `Full/<yyyyMMdd>/` 与历史旧位置 `<备份目录>/<yyyyMMdd>/`（旧远端 key 无 `Full/` 段，按原 key 校验，避免把未上传文件当成已上传删掉）；④ `Install.sh`/`Upgrade.sh` 预建 `BRS/Full`；⑤ 文档同步（`Brs.md` 目录树与上传 key、`BrsBackupDesign.md`、`OSSConfig.md`、`Slice.md`、`MigratePublicKlineToCdeu.md`）。 | `grep -rn 'FullDirName' internal/meta/finv/brs/*.go` 含 `const FullDirName = \"Full\"` 与 key 拼接；`go test ./internal/meta/finv/brs/ -run Cleanup` 通过（含新增 `TestCleanupLocalDumpsFullDir`：远端带 `Full/` 才删、旧 key 无 `Full/` 则保留）；运行态：触发一轮小表全量备份后 `ls $FINV_BRS_BACKUP_DIR/Full/$(date +%Y%m%d)` 有产物，日志 `已上传 <env>/Full/<yyyyMMdd>/<file>`；`grep -rn 'BRS_DIR}/Full' Deploy/Native/*.sh` 各 1 处 | √ |
| 10916 | 大表分片增量由「末尾 N 个无条件重做」改为**内容指纹比对**：① 每个分片先算源端指纹 `(count(*), sum(hashtextextended(行文本,0)))`（一次范围扫描，不导出不上传）；② 备份成功后把指纹写入本地 sidecar `<分片文件>.fp` 与远端 `<key>.fp`（JSON：rows/hash/gmt）；③ 已存在分片：本地指纹优先、其次远端指纹，与源端比较——相同则跳过并记「内容无变化」，不同则重做并覆盖指纹；④ 无指纹（10916 之前的产物）时 `redo_slices` 退化为**兜底窗口**：窗口内重做、窗口外跳过并 `warn`（不再每轮整批重做）；⑤ 源端指纹计算失败时保守重做且不写指纹；远端指纹上传失败只告警（只影响下一轮差异判断）；⑥ 页面分片明细的 `done`/`backuped` 改为「产物存在即已备份」，`redo_from` 语义改为兜底窗口起点，文案同步。 | `go test ./internal/meta/finv/brs/ -run Fingerprint`、`-run DecideSkip`、`-run SaveFingerprints` 三段均通过（7 个用例：本地/远端读写、损坏视为无效、Equal 只比 rows+hash、四条跳过分支、双写、关闭上传只写本地）；运行态三轮验证：首轮旧产物无指纹 → `warn` 跳过、新分片写入 `.fp`；第二轮 → 日志「内容无变化」跳过；改一条数据后再跑 → 该分片「备份成功」并重新上传，其它分片仍「内容无变化」；`ls $FINV_BRS_BACKUP_DIR/Incr/<表>/` 出现 `<分片键>.mvsv.gz.fp` | √ |
| 10917 | 本地清理与指纹 sidecar 的关系固化为可验证行为：① 清理只扫描 `<备份目录>/Full/<yyyyMMdd>/`（及 10915 前的旧位置）里的 `*.dump`，**`Incr/` 分片目录与分片指纹 `<分片键>.mvsv.gz.fp` 完全不在清理范围内**；② 删除过期 dump 时**成对删除同名 `<文件>.dump.fp`**（防止将来小表也引入指纹后留下孤儿 sidecar），孤立的 `.fp` 不动；③ 新增两条回归测试固化该行为。 | `go test ./internal/meta/finv/brs/ -run Cleanup` 通过（新增 `TestCleanupLocalDumpsKeepsIncrFingerprint`：`Incr/<表>/20260101.mvsv.gz` 与 `.fp` 清理后仍在；`TestCleanupLocalDumpsRemovesPairedFingerprint`：过期 dump 与其 `.dump.fp` 一起删除、孤立 `keep.dump.fp` 保留）；`grep -n 'filepath.Ext(file.Name()) != ".dump"' internal/meta/finv/brs/cleanup.go` 命中 1 处 | √ |
| 10918 | 分片备份两处整改：① **旧产物缺指纹时改为自愈**（原实现窗口外只跳过 + `warn`，导致每轮都报同一批告警、永远不建立指纹）：缺指纹 → 重做一次并写回指纹，日志 `缺少指纹记录（10916 之前的产物），重做一次建立基线：N 行 / hash`；`redo_slices=0` 时仍跳过并 `warn`（显式关闭自愈）；`redo_slices` 语义同步改为「自愈开关」，迁移 `V300095` 更新字段注释；② **备份成功日志输出本地产物路径**：`备份成功 <表> 分片 <键>（N 行，大小，耗时）→ <备份目录>/Incr/<表>/<键>.mvsv.gz`（上传日志仍输出远端 key）。 | `go test ./internal/meta/finv/brs/ -run DecideSkip` 通过（无指纹+自愈开启 → 重做并输出「重做一次建立基线」；无指纹+自愈关闭 → 跳过并输出「自愈关闭」告警）；运行态两轮验证：先 `rm -f <备份目录>/Incr/<表>/*.fp` 模拟旧产物 → 跑一轮日志为「重做一次建立基线」+ 成功日志带本地路径 + 生成 9 个 `.fp`；再跑一轮 → 全部「内容无变化」跳过、`done=0`、`warn=0`；`grep -n '重做一次建立基线' internal/meta/finv/brs/slice.go` 命中 1 处 | √ |
| 10919 | 工作流界面阶段状态必须反映实际执行：阶段/流程内的子级（含动态导出 JOB 实例）正在执行时，阶段不得显示「待执行」。① 动态导出 JOB 启动时同步把工作流/阶段/流程置为 RUNNING（新增 `TouchCDEUHierarchyByProcess`，`startDynamicExportJob` 调用），结束后重新评估所在流程（`evaluateDynamicHierarchy` → `EvaluateCDEUProcess`）；② `CDEUOverview` 增加展示层派生 `deriveCDEURunning`：父级状态为 PENDING/READY 且（任一子级 RUNNING、或自身 started_at 非空 finished_at 为空）时输出 RUNNING，已终态与已 RUNNING 不受影响；③ 子流程同样派生，PENDING 的流程若自身有 RUNNING 的 JOB 也置 RUNNING。 | `go test ./internal/meta/finv/quote/cdeu/ -run DeriveCDEURunning` 通过（5 个分支：子 JOB RUNNING、子流程 RUNNING、已开始未结束、全未开始保持 PENDING、终态不被改写）；运行态验证：把 run 84 的 `PHASE_EXPORT` 的 phase_run 置 PENDING（其下有 RUNNING 的动态导出 JOB）后请求 `/API/V1/Meta/Finv/Quote/DeDup/CDEU/Overview?runId=84`，返回该阶段 `status=RUNNING`（前端显示「执行中」）而非 PENDING；`grep -n 'deriveCDEURunning' internal/meta/finv/quote/cdeu/cdeu_admin.go` 命中调用与定义；`grep -n 'TouchCDEUHierarchyByProcess' internal/meta/finv/quote/cdeu/job_export.go` 命中 | √ |
| 10920 | `cdeu.finv_quote_cdeu_usc` 的行序只能来自主表 `cdeu.finv_quote_secu_kline_min` 中真实出现过的证券，**不得从证券字典 `public.finv_security` 拷贝**；已登记的行默认永不删除（某证券当月/上月乃至连续数月无数据也保留）。① `RefreshSecurityMetadata` 由 `INSERT ... SELECT FROM public.finv_security` 改为纯 `UPDATE`：只刷新已登记 usc 的 `category`/`region`/`market`/`name`/`timezone`/`currency`，不再新建行；② `DiscoverUsc` 增加 `full` 参数：首次执行（无水位）全量扫描主表 `DISTINCT usc`，之后按 `gmt_update` 窗口增量；③ 全流程无任何 DELETE，人工 `PUT /Usc` 预建的行同样保留。 | `grep -n 'INSERT INTO `+tableUsc+`' internal/meta/finv/quote/cdeu/store.go` 只剩 `DiscoverUsc` 一处（原 `RefreshSecurityMetadata` 的 INSERT 已移除）；运行态：容器升级后触发 `RUN_DUE_EXPORT_JOBS`，`SELECT count(*) FROM cdeu.finv_quote_cdeu_usc` 行数不变（原实现每次都会把 `finv_security` 的 17199 行重放一遍）；向主表插入一行 `usc='TEST.USC.9999'` 后触发 `REFRESH_QUOTE_MATURITY`，该 usc 出现在 `finv_quote_cdeu_usc`（`flag_auto_discover=t`、`provider=FTMM`）；`SELECT count(*) FROM cdeu.finv_quote_cdeu_usc u WHERE NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_secu_kline_min m WHERE m.usc=u.usc)` 除人工预建证券外应为 0 | √ |
| 10921 | 清理 `cdeu.finv_quote_cdeu_usc` 存量脏行并让程序自动补齐：① 一次性脚本 `Deploy/Upgrade/2611-001-PurgeUscWithoutQuote.sql` 删除「主表 `cdeu.finv_quote_secu_kline_min` 中不存在任何一行」的 usc，按 `DbSchemaSpec` §0.7 先整行备份到 `archive.finv_quote_cdeu_usc_purge_20261102`（带 `src_schema`/`reason`/`backup_at`），单事务「备份 → 删除 → 校验」，校验用 `RAISE EXCEPTION`；② `DiscoverUsc` 增加缺口自愈：每天最多一次检测主表是否存在未登记 usc（水位 `DISCOVER_USC_GAP_CHECK`，走 `idx_finv_quote_usc_date_ts` 且命中即停），发现则本轮自动升级为全量扫描，无需人工清水位。 | 执行 `2611-001-PurgeUscWithoutQuote.sql`（本机）输出 `NOTICE: 完成：删除前 17199 行，备份 16714 行，删除 16714 行，删除后 485 行`；`SELECT count(*) FROM archive.finv_quote_cdeu_usc_purge_20261102` = 16714；随后触发 `REFRESH_QUOTE_MATURITY`，`SELECT count(*) FROM cdeu.finv_quote_cdeu_usc` = 498 且 `missing=0`（13 个历史证券自动补齐，JOB 日志「优先级/SLA 13 行」）；`grep -n 'uscHasUnregistered' internal/meta/finv/quote/cdeu/store.go` 命中定义与调用 | √ |
| 10922 | 用 SQL 发放补全 `cdeu.finv_quote_cdeu_usc` 的缺失元数据（`timezone`/`day_expect_rows`/`category`/`region`/`market`/`currency`/`name`），不改代码逻辑：① 模板表补 `('CC','FTC','Crypto',1440)` 与 `('CC','','',1440)`，使 `region=CC` 的加密货币能匹配 `day_expect_rows`；② `region=CC` 的行补 `timezone='UTC'`、`day_expect_rows=1440`；③ 上游字典（`public.finv_security`、`futu.finv_futu_quote_collect`、`futu.finv_futu_mapping_security`）未收录但行情确在采的证券，按显式清单补全（`AMAT`/`MRVL`/`NOW`/`PLTR`/`SMCI`、`XRP`、`899050`/`SH000688`/`SH000888`）；④ `name` 仍无来源的用 `usc` 兜底，不编造名称；全部列 `COALESCE(原值, 新值)`，不覆盖人工值；迁移末尾 `RAISE NOTICE` 输出收敛统计。 | `test -f Deploy/Migrations/V300096__cdeu_usc_fill_missing_metadata.sql` 退出码 0；换包启动后 `SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300096；`SELECT count(*) FROM cdeu.finv_quote_cdeu_usc WHERE timezone IS NULL OR currency IS NULL OR name IS NULL OR day_expect_rows IS NULL` = 0（本机补全后）；`SELECT usc, timezone, day_expect_rows FROM cdeu.finv_quote_cdeu_usc WHERE usc='BTCUSD'` 为 `UTC / 1440`，`AMAT` 为 `America/New_York / 390`（名称 `Applied Materials, Inc.`）；`SELECT * FROM cdeu.finv_quote_cdeu_usc_day_incr_row_tpl WHERE region='CC'` 返回 2 行 | √ |
| 10923 | 删除导入联调残留证券 `IT001`/`IT002`/`IT003`（单行、`date`/`time` 为空、`ts` 相同、证券字典与富途镜像均无记录，无来源可推导元数据），用一次性脚本 `Deploy/Upgrade/2611-002-PurgeTestUsc.sql` 执行：按 `DbSchemaSpec` §0.7 把行情行备份到 `archive.finv_quote_secu_kline_min_purge_20261102`、usc 行备份到 `archive.finv_quote_cdeu_usc_purge_20261102`（均带 `src_schema`/`reason`/`backup_at`），单事务「备份 → 删除 → 校验」，校验不符 `RAISE EXCEPTION` 回滚；脚本尾部附回滚 SQL。 | 执行脚本输出 `NOTICE: 完成：行情表 删除前 3 行 / 备份 3 / 删除 3；usc 表 删除前 3 行 / 备份 3 / 删除 3`；`SELECT count(*) FROM cdeu.finv_quote_secu_kline_min WHERE usc IN ('IT001','IT002','IT003')` = 0 且 `... FROM cdeu.finv_quote_cdeu_usc WHERE usc IN (...)` = 0；`SELECT count(*) FROM archive.finv_quote_secu_kline_min_purge_20261102` = 3；`SELECT count(*) FROM cdeu.finv_quote_cdeu_usc` 与 `SELECT count(DISTINCT usc) FROM cdeu.finv_quote_secu_kline_min WHERE usc <> ''` 均为 495；`SELECT count(*) FROM cdeu.finv_quote_cdeu_usc WHERE category IS NULL OR region IS NULL OR market IS NULL OR name IS NULL OR timezone IS NULL OR currency IS NULL OR day_expect_rows IS NULL` = 0 | √ |
| 10924 | `BackfillUscDefaults` 的 `region` → `timezone` 推导补齐 `CC` 区域（`WHEN 'CC' THEN 'UTC'`）：本库加密货币的 `region` 实际取值为 `CC`（`market=FTC`、`category=Crypto`），原推导只覆盖 `CN`/`HK`/`US`/`FX`/`GLOBAL`，导致新增加密货币永远补不上时区（`V300096` 只补了存量）。币种保持不推（加密货币计价币种不统一）。 | `grep -n "WHEN 'CC' THEN 'UTC'" internal/meta/finv/quote/cdeu/store.go` 命中 1 处；`go build ./...` 与 `go vet ./internal/meta/finv/quote/cdeu/` 通过；运行态：`UPDATE cdeu.finv_quote_cdeu_usc SET timezone=NULL WHERE usc IN ('BTCUSD','XRP')` 后触发 `REFRESH_QUOTE_MATURITY`，两条记录的 `timezone` 均被补回 `UTC`，`day_expect_rows=1440` 与 `currency=USD` 保持不变 | √ |
| 10925 | 补全 `cdeu.finv_quote_cdeu_usc.date_list`（上市/上线日期）：① 迁移 `V300097` 对上市日确定的美股 5 只（`AMAT`/`MRVL`/`NOW`/`PLTR`/`SMCI`）按公开 IPO/直接上市日发放；② 其余仍为空的（`XRP`、`899050`、`SH000680`/`SH000688`/`SH000888`）用主表最早行情交易日 `min(date)` 兜底（主表 `date` 为 integer `yyyyMMdd`，需 `to_date(...)` 且越界值不参与）；③ 代码侧 `DiscoverUsc` 在登记新 usc 时同步写入 `date_list`（全量与增量两个分支都是 `min(m.date)` 转 date），并在已有行 `date_list` 为空时 `ON CONFLICT DO UPDATE ... WHERE date_list IS NULL` 补上，不覆盖上游/人工值。 | `test -f Deploy/Migrations/V300097__cdeu_usc_fill_date_list.sql` 退出码 0；换包启动后 `SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300097；`SELECT count(*) FROM cdeu.finv_quote_cdeu_usc WHERE date_list IS NULL` = 0（本机 495 行）；`SELECT usc, date_list FROM cdeu.finv_quote_cdeu_usc WHERE usc IN ('AMAT','MRVL','NOW','PLTR','SMCI')` 依次为 `1972-10-12`/`2000-06-27`/`2012-06-29`/`2020-09-30`/`2007-03-29`；`SELECT date_list FROM cdeu.finv_quote_cdeu_usc WHERE usc='XRP'` 非空（取主表最早交易日）；向主表插入新的 `usc='TEST.DATE.0001'`（最早 `date=20260920`）后触发 `REFRESH_QUOTE_MATURITY`，`SELECT date_list ... WHERE usc='TEST.DATE.0001'` = `2026-09-20` | √ |
| 10926 | 证券主数据 usc 改名 `KC50` → `STAR50`：科创50 指数的助记名统一为 `STAR50`，与 `SHCI`（上证综指）、`CSIA500`（中证A500）、`CSI300`（沪深300）同一批命名规范。迁移 `V300098` 按 `DbSchemaSpec` §0.7 先整行备份到 `archive.finv_security_purge_20261102`，再单事务「备份 → 改名 → 校验」，`RAISE EXCEPTION` 不符即回滚，幂等。执行前已核对 `cdeu.finv_quote_cdeu_usc`、`cdeu.finv_quote_secu_kline_min`、`cdeu.finv_quote_cdeu_quote_maturity_mon` 均无 `KC50` 行，且无外键引用 `public.finv_security`。 | `test -f Deploy/Migrations/V300098__security_usc_kc50_to_star50.sql` 退出码 0；执行脚本输出 `NOTICE: 完成：改名前 1 行，备份 1 行，改名 1 行；STAR50 现存在 1 行`；`SELECT count(*) FROM public.finv_security WHERE usc='KC50'` = 0；`SELECT security_code, market_code, security_type FROM public.finv_security WHERE usc='STAR50'` 为 `000688 / 1110 / StockIndex`；`SELECT count(*) FROM archive.finv_security_purge_20261102 WHERE usc='KC50'` = 1；重复执行脚本影响 0 行 | √ |
| 10927 | 证券主数据 usc 助记名改名第一批：`SHI` 改为 `SHCI`（上证综指）、`A500` 改为 `CSIA500`（中证A500）、`HS300I` 改为 `CSI300`（沪深300），与已完成的 `KC50` 改为 `STAR50`（需求 10926）同一批命名规范。迁移 `V300099` 按 `DbSchemaSpec` §0.7 循环处理三条：整行备份到 `archive.finv_security_purge_20261102`，单事务「备份 → 改名 → 校验」，`RAISE EXCEPTION` 不符即回滚，幂等。执行前核对三个旧 usc 在主表行情与行情登记表中均 0 行，三个新 usc 均未占用，无外键引用。 | `test -f Deploy/Migrations/V300099__security_usc_rename_alias_batch1.sql` 退出码 0；执行输出三条 `NOTICE: <旧> → <新>：改名前 1 行 / 备份 1 行 / 改名 1 行`；`SELECT usc, security_code, market_code FROM public.finv_security WHERE usc IN ('SHCI','CSIA500','CSI300')` 返回 3 行，`security_code` 分别为 `000001`、`000510`、`000300`；`SELECT count(*) FROM public.finv_security WHERE usc IN ('SHI','A500','HS300I')` = 0；`SELECT count(*) FROM archive.finv_security_purge_20261102 WHERE reason LIKE '需求 10927%'` = 3；重复执行影响 0 行 | √ |
| 10928 | 证券主数据错位记录修正与字段互校（清单 M4、M6、M7）：① `000680` 的 Prod 错位记录（`security_type=StockIndex`、名称写成科创综指）改正为山推股份；② `SH0680` 逐字段互校（`security_code` 取 `000680`、`market_code` 取 1112、主名称取中文、补 `type_secu_sub` 与 `provider`、`remark`）；③ `SH0985` 补齐 `type_secu_sub`、`provider`、`remark`。迁移 `V300100` 按 `DbSchemaSpec` §0.7 先备份到 `archive.finv_security_purge_20261102`，M4 带 `security_type='StockIndex'` 条件保证正确记录不被误改，幂等。 | `test -f Deploy/Migrations/V300100__security_fix_misaligned_records.sql` 退出码 0；执行输出 `M4 000680 错位记录改正：0 行`（本地已是正确值）、`M6 SH0680 字段互校：1 行`、`M7 SH0985 补齐字段：1 行`；`SELECT security_type, security_name FROM public.finv_security WHERE usc='000680'` 为 `Stock / 山推股份`；`SELECT security_code, market_code, type_secu_sub, provider FROM public.finv_security WHERE usc='SH0680'` 为 `000680 / 1112 / StockIndex.SH / FTMM`；`SELECT count(*) FROM public.finv_security WHERE usc='SH0985' AND COALESCE(type_secu_sub,'')<>'' AND COALESCE(provider,'')<>''` = 1；重复执行影响 0 行 | √ |
| 10929 | 证券主数据 `flag_enable` 全量启用向 Prod 发放（清单 M8）：以 LocalDocker 为准，迁移 `V300101` 按 `finv_exchange.region` 分组、每组每批最多 1000 行循环更新，更新前把待更新行整行备份到 `archive.finv_security_purge_20261102`，全部在单事务内完成，幂等。发放后 `RefreshSecurityMetadata` 可覆盖的证券由 552 行扩大到 17199 行。 | `test -f Deploy/Migrations/V300101__security_enable_all_by_region.sql` 退出码 0；本机（已全启用）执行输出 `完成：共备份 0 行，共启用 0 行，剩余未启用 0 行`；`SELECT count(*) FROM public.finv_security WHERE flag_enable <> '1'` = 0；生产执行后同一查询为 0，且 `SELECT flag_enable, count(*) FROM public.finv_security GROUP BY 1` 只剩一行 | √ |
| 10930 | LocalDocker 独有的 6 只证券发放给 Prod（清单 M11）：`AMAT`、`MRVL`、`NOW`、`PLTR`、`SMCI`、`XRP` 由 `DiscoverUsc` 自动登记（`flag_auto_discover='t'`），迁移 `V300102` 用显式 `VALUES` 固化为 SQL 向 Prod 发放，`ON CONFLICT (usc) DO NOTHING` 不覆盖已有记录，末尾校验必须恰好 6 行。 | `test -f Deploy/Migrations/V300102__cdeu_usc_register_local_new_securities.sql` 退出码 0；执行输出 `完成：6 只证券在登记表中现存 6 行（应为 6）`；`SELECT count(*) FROM cdeu.finv_quote_cdeu_usc WHERE usc IN ('AMAT','MRVL','NOW','PLTR','SMCI','XRP')` = 6；`SELECT region, market, day_expect_rows FROM cdeu.finv_quote_cdeu_usc WHERE usc='XRP'` 为 `CC / FTC / 1440`；重复执行影响 0 行 | √ |
| 10931 | USC 补丁（Patch）机制：新增规则表 `cdeu.finv_quote_cdeu_usc_patch_rule`（`usc_old`、`usc_new`、`match_mode`、`flag_enable`、`priority`、`note`），维护「过时 usc → 新 usc」映射；执行时按 `ts` 区间分批，用 `INSERT ... ON CONFLICT (ts, usc) DO UPDATE` 规避主键冲突（冲突按 `gmt_update` 较新者胜覆盖），随后在同一语句内删除旧 usc 行；提供 `/API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/{List,Save,Enable,Delete,Preview,Run}` 六个接口与前端「USC 补丁规则」页面（`/Meta/Finv/Quote/Cdeu/Usc/Patch`）；定时 JOB `PATCH_OUTDATED_USC` 每日 05:00 执行。方案见 `Docs/Proposal/202611-0002-UscPatchSchemeProposal.md`。 | `test -f Deploy/Migrations/V300103__cdeu_usc_patch_rule.sql` 退出码 0；`SELECT count(*) FROM cdeu.finv_quote_cdeu_usc_patch_rule` = 25 且 `flag_enable='1'` 为 0；`GET .../Usc/Patch/List?page=1&pageSize=3` 返回 `code=0` 且 `total=25`；`GET .../Usc/Patch/Preview` 在无生效规则时 `total_rows=0`；启用 `idn=5`（`SH000008` → `SH0008`）后 `Preview` 返回 `rows=1205`、`conflict_rows=241`，停用后恢复 0；`POST .../Usc/Patch/Run` 在无生效规则时返回 `rule_count=0`、`moved_rows=0`；`go test ./internal/api -run TestExportArchiveRoutesRegistered` 通过；`SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300103 | √ |
| 10932 | 把 `PATCH_OUTDATED_USC`（USC 补丁，需求 10931）纳入 CDEU 工作流的「历史行情去重」阶段（`PHASE_DEDUP`）作为阶段内 JOB 运行，与独立每日 05:00 定时并存：① 新增流程 `PROCESS_USC_PATCH`（`phase_id=2`、`seq_no=1`，「过时 USC 补丁」），排在收口冻结 `PROCESS_SEAL` 之前，`PROCESS_SEAL.seq_no` 顺延为 2；② 新增 JOB 登记 `JOB_PATCH_OUTDATED_USC`（`job_kind='PATCH_OUTDATED_USC'`、`trigger_mode='SCHEDULED'`、`execution_mode='DIRECT'`）；③ 新增步骤 `STEP_PATCH_OUTDATED_USC`；④ 两种触发共用同一 `job_kind`，同一时刻只允许一个 RUNNING 实例，避免工作流触发与定时触发重复执行。设计与落地迁移号在提案 `Docs/Proposal/202611-0002-UscPatchSchemeProposal.md` 第 11 节。 | `SELECT 1 FROM cdeu.finv_quote_cdeu_process WHERE process_code='PROCESS_USC_PATCH' AND phase_id=2` 返回 1 行；`SELECT seq_no FROM cdeu.finv_quote_cdeu_process WHERE process_code IN ('PROCESS_USC_PATCH','PROCESS_SEAL') ORDER BY seq_no` 依次为 1、2；`SELECT job_kind FROM cdeu.finv_quote_cdeu_job WHERE job_code='JOB_PATCH_OUTDATED_USC'` 为 `PATCH_OUTDATED_USC`；`SELECT count(*) FROM cdeu.finv_quote_cdeu_job_step WHERE step_code='STEP_PATCH_OUTDATED_USC'` = 1；`SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300106；`GET /API/V1/Meta/Finv/Quote/DeDup/CDEU/Overview` 的 `PHASE_DEDUP` 阶段 JOB 列表包含 `JOB_PATCH_OUTDATED_USC`（`job_kind=PATCH_OUTDATED_USC`、名称「过时 USC 补丁」） | √ |
| 10933 | USC 补丁执行改为异步并展示逐批日志：新增 `UscPatchRunner`（内存状态 + 增量日志 + 单实例约束，模式同 `brs.Runner`）；`POST .../Usc/Patch/Run` 改为异步触发并立即返回 `running` 快照，新增 `GET .../Usc/Patch/Status?sinceSeq=` 返回状态与增量日志，已有实例运行时返回 HTTP 409；逐批日志包含可读时间区间、本批扫描行数、累计迁移/冲突/删除行数与本批耗时；前端页面按 BRS 手动备份的形态展示状态卡与日志框（1 秒轮询、自动滚动）；定时 JOB 执行前检查 runner 是否在跑，在跑则跳过。 | `POST .../Usc/Patch/Run` 返回 `code=0` 且 `data.status=running`；连续两次触发第二次返回 HTTP 409、`code=2009`；`GET .../Usc/Patch/Status` 返回 `logs` 数组且每批一行、`last_seq` 递增；插入 700 行测试行情并把 `batch_size` 设为 300 后执行，`Status` 日志出现 3 条批次记录（300/300/100 行）且结束行统计为 `3 批，扫描 700 行`；测试数据已清理（`SELECT count(*) FROM cdeu.finv_quote_secu_kline_min WHERE usc LIKE 'TEST.%'` = 0）；`go test ./internal/api -run TestExportArchiveRoutesRegistered` 通过 | √ |
| 10934 | 修复全局轻提示（Toast）不自动消失、关闭按钮无效的问题：原实现依赖 `v-snackbar` 的 `timeout`，队列切换到下一条时组件不会重新起表，导致第二条及之后的提示永久停留（跨页面也一直显示）。改为**每条消息在 `showToast` 时独立计时**（`setTimeout` 到期调用 `dismissToast(id)`），`v-snackbar` 固定 `timeout=-1` 只负责展示，关闭按钮同样走 `dismissToast`。 | `grep -n 'setTimeout(() => dismissToast(id)' Web/src/ui/toast.ts` 命中 1 处；`grep -n '":timeout="-1"' Web/src/components/AppToast.vue` 命中 1 处；连续点击「启用」多次（页面 `/Meta/Finv/Quote/Cdeu/Usc/Patch` 或任一使用 showToast 的页面），每条提示均在 5 秒内自动消失；点提示右侧 × 立即消失且队列继续播放；`npx vue-tsc -b` 与 `npx vite build` 通过 | √ |
| 10935 | USC 补丁启用前自动登记目标 usc：启用规则（`SetUscPatchRuleEnable(true)`）时把 `usc_new` 从 `usc_old` 复制登记到 `cdeu.finv_quote_cdeu_usc`（`ON CONFLICT DO NOTHING`，只补不覆盖），`PatchOutdatedUsc` 执行前再兜底同步一次并在日志输出「补齐目标 usc 登记 N 行」；一次性存量修正由迁移 `V300104` 完成，覆盖**全部规则**（含停用规则，便于将来启用时不出空档）。 | `SELECT count(*) FROM cdeu.finv_quote_cdeu_usc WHERE usc IN ('SH0888','SH0903','SH0922','SH0980')` = 4；执行 `V300104` 输出 `完成：旧 usc 已登记但目标 usc 未登记的数量 = 0`；调用 `POST .../Usc/Patch/Enable {idn:21,enable:true}` 后 `SELECT count(*) FROM cdeu.finv_quote_cdeu_usc WHERE usc='SH0888'` = 1；`SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300104 | √ |
| 10936 | USC 补丁联动成熟度表：`PatchOutdatedUsc` 在主表迁移完成后，把 `cdeu.finv_quote_cdeu_quote_maturity_mon` 里旧 usc 的月度统计合并到新 usc（同一 `trade_date` 取 `actual_rows` 较大者，状态跟随较大者），并删除旧 usc 记录；删除前整行备份到 `archive.finv_quote_cdeu_quote_maturity_mon_purge_20261102`（迁移 `V300105` 建表，带 `src_schema`/`reason`/`backup_at`）。执行统计新增 `maturity_moved`、`maturity_deleted` 并在日志输出。 | `SELECT count(*) FROM cdeu.finv_quote_cdeu_quote_maturity_mon WHERE usc IN ('SH000888','SH000903','SH000922','SH000980') AND biz_month=202610` = 0；`SELECT usc, count(*) FROM cdeu.finv_quote_cdeu_quote_maturity_mon WHERE usc IN ('SH0888','SH0903','SH0922','SH0980') AND biz_month=202610 GROUP BY usc` 各 5 天、合计 1205 行；`SELECT count(*) FROM archive.finv_quote_cdeu_quote_maturity_mon_purge_20261102` = 120；`GET /API/V1/Meta/Finv/Quote/DeDup/Export/Stock/Page?bizMonth=202610` 的 items 中不再出现 `SH000888` 等 6 位旧形态 usc，且出现 `SH0888`、`SH0903`、`SH0922`、`SH0980`；`SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300105 | √ |
| 10937 | 清理成熟度表中的测试残留：`cdeu.finv_quote_cdeu_quote_maturity_mon` 里仍留着 `TEST.USC.9999`（1 行）与 `TEST.DATE.0001`（2 行），是需求 10920、10925 联调时插入的测试证券——当时只清了主表与证券登记表，漏了成熟度表。按 `DbSchemaSpec` §0.7 先整行备份到 `archive.finv_quote_cdeu_quote_maturity_mon_purge_20261102`（`reason` 标注测试残留清理），单事务「备份 → 删除 → 校验」，校验不符 `RAISE EXCEPTION` 回滚。 | `SELECT count(*) FROM cdeu.finv_quote_cdeu_quote_maturity_mon WHERE usc LIKE 'TEST.%'` = 0；`SELECT count(*) FROM archive.finv_quote_cdeu_quote_maturity_mon_purge_20261102 WHERE reason LIKE '测试残留%'` = 3；主表、`cdeu.finv_quote_cdeu_usc`、`public.finv_security` 中 `usc LIKE 'TEST.%'` 均为 0；注意 `09999` 是港股网易的合法代码，不属测试数据 | √ |
| 10938 | 统一 USC 补丁相关名称：工作流流程、JOB、步骤名称与前端菜单名由「过时 USC 补丁」「USC 补丁规则」统一改为「过时USC映射到新USC」，与功能语义一致（不是打补丁，而是把过时 usc 的行情映射到新 usc）。迁移 `V300107` 更新 `finv_quote_cdeu_process.name`、`finv_quote_cdeu_job.name`、`finv_quote_cdeu_job_step.name`；代码侧同步 `jobmeta` 的 `Name`、前端 `menuConfig` 菜单标题、路由 `meta.title`、页面标题。只改显示名称，接口路径与 `job_kind`、`job_code` 均不变。 | `SELECT count(*) FROM cdeu.finv_quote_cdeu_process WHERE name='过时USC映射到新USC'` = 1；`SELECT count(*) FROM cdeu.finv_quote_cdeu_job WHERE name='过时USC映射到新USC'` = 1；`SELECT count(*) FROM cdeu.finv_quote_cdeu_job_step WHERE name='过时USC映射到新USC'` = 1；`grep -c '过时USC映射到新USC' Web/src/config/menuConfig.ts` = 1；`GET /API/V1/Meta/Finv/Quote/DeDup/CDEU/Overview` 中该 JOB 的 `name` 为「过时USC映射到新USC」 | √ |
| 10939 | 修复 `PATCH_OUTDATED_USC` 手动执行报「未知 Job」：该 JOB 的 `jobmeta` 定义写好了，但没有加入 `jobmeta.All()`，于是 `JobRuntimes()` 里没有它，`TriggerWithOptions` 判定为未知 Job 直接返回错误，调度器也不会按每日 05:00 调度。修复：把 `PatchOutdatedUsc` 加入 `jobmeta.All()`（排在收口冻结之前）；新增单测 `TestJobRuntimesCoverAllMeta` 校验 `jobmeta.All()` 的每一项都出现在 `JobRuntimes()` 里，防止同类遗漏重犯。 | `GET /API/V1/Meta/Finv/Quote/DeDup/Job/Runtime` 返回项中包含 `PATCH_OUTDATED_USC`、名称「过时USC映射到新USC」、`schedule` 为「每日 05:00」；`POST /API/V1/Meta/Finv/Quote/DeDup/Job/Run {"job_kind":"PATCH_OUTDATED_USC"}` 返回 `code=202` 与 `run_log_id`，不再返回「未知 Job」；容器日志出现 `调度 PATCH_OUTDATED_USC：每日 05:00` 与 `PATCH_OUTDATED_USC 完成 rules=24 ...`；`go test ./internal/meta/finv/quote/cdeu/` 中 `TestJobRuntimesCoverAllMeta`、`TestPatchOutdatedUscRegistered` 均通过 | √ |
| 10940 | 修复 CDEU 工作流阶段状态顺序倒挂：`attachCDEURun` 在把 PENDING 阶段按「子流程全部成功」回填 SUCCESS 时没有校验前置阶段，而「归档复核」阶段的子流程都是日级清理 JOB、由调度器独立触发，常常在「月度行情上传」还没跑完时就全部成功，于是出现「上传执行中、归档复核成功」的倒挂。修复：回填改为按 `seq_no` 顺序放行，任一前置阶段未成功（RUNNING、PENDING、READY、FAILED、PARTIAL）时后续阶段不再回填 SUCCESS；新增单测覆盖三种情形。 | `go test ./internal/meta/finv/quote/cdeu/` 中 `TestPhaseOrderingBlocksLaterSuccess`、`TestPhaseOrderingAllowsLaterSuccess`、`TestPhaseOrderingBlocksOnFailed` 均通过；`GET /API/V1/Meta/Finv/Quote/DeDup/CDEU/Overview?runId=86` 返回的阶段状态中 `PHASE_UPLOAD` 为 `RUNNING` 时 `PHASE_REVIEW` 不再是 `SUCCESS`；前置阶段全部成功的历史运行，后续阶段仍能回填 `SUCCESS` | √ |
| 10941 | CDEU 阶段顺序门禁补全：前一阶段未完成时，后续阶段不得被推进为 RUNNING、SUCCESS 或 SKIPPED。根因是三条写入路径都没有校验前置阶段：① `TouchCDEUHierarchy` 无条件把阶段置 RUNNING；② `EvaluateCDEUPhase` 在门禁命中时仍写 PENDING/SKIPPED；③ `SeedCDEUProcessRuns` 在「子流程全部 SUCCESS/SKIPPED」时直接批量把阶段标 SKIPPED。归档复核阶段下的清理 JOB（超时回收、PURGE、STORAGE_CLEAN、RETENTION、健康检查）都是独立定时任务，不等月度上传完成就会执行，于是每次触发都把这批阶段写成 RUNNING/SKIPPED，出现「上传执行中、归档复核成功或已跳过」。修复：三处统一加前置门禁——`TouchCDEUHierarchy` 命中门禁时不写、`EvaluateCDEUPhase` 命中且前序未完成时直接返回不改状态、`SeedCDEUProcessRuns` 的阶段 SKIPPED 语句加 `NOT EXISTS` 前置未完成判断；展示层（需求 10940）门禁时不派生 RUNNING。清理 JOB 本身照常执行，只是不再污染阶段状态。 | `GET /API/V1/Meta/Finv/Quote/DeDup/CDEU/Overview?runId=86` 在 `PHASE_UPLOAD=RUNNING` 时 `PHASE_REVIEW` 为 `PENDING`；连续触发两次 `CHECK_EXPORT_JOB_HEALTH`（属 PHASE_REVIEW）后该阶段仍为 `PENDING`；容器日志出现 `阶段 12 的前序阶段仍为 RUNNING，跳过层级状态推进`；`SELECT status FROM cdeu.finv_quote_cdeu_phase_run WHERE workflow_run_id=86 AND phase_code='PHASE_REVIEW'` 为 `PENDING`；`go test ./internal/meta/finv/quote/cdeu/` 中 `TestPhaseOrdering*` 与 `TestDeriveCDEURunning` 通过 | √ |
| 10942 | 修复「月度行情上传」阶段卡在执行中、剩余上传任务既无法自动执行也无法手动执行：① `sys_config` 20005（CDEU 工作流运行模式）为 `Manual` 时进程内调度器整体不触发，所有 JOB 都不会自动跑；② `JOB_UPLOAD_FILES` 的 `ui_visible=false`，工作流面板不显示这个 JOB，用户也找不到手动入口，于是阶段永久停在 RUNNING、无法进入归档复核。修复：迁移 `V300108` 把 `JOB_UPLOAD_FILES.ui_visible` 置为 true；同时清理 3 条历史残留的 `RUNNING` 上传运行日志（进程重启未收口）标记为 `FAILED`。手动执行上传后，21 个待传文件全部上传完成，阶段自动推进为 SUCCESS。 | `SELECT ui_visible FROM cdeu.finv_quote_cdeu_job WHERE job_code='JOB_UPLOAD_FILES'` 为 `t`；`GET /API/V1/Meta/Finv/Quote/DeDup/CDEU/Overview?runId=86` 的 `PHASE_UPLOAD` 阶段列出 `JOB_UPLOAD_FILES` 且状态为 `SUCCESS`；`SELECT upload_status, count(*) FROM cdeu.finv_quote_cdeu_export_file GROUP BY 1` 只剩 `UPLOADED` 与 `SKIPPED`，无 `PENDING_UPLOAD`；`SELECT count(*) FROM cdeu.finv_quote_cdeu_job_run_log_export_file WHERE job_kind='UPLOAD_PENDING_EXPORT_FILES' AND status='RUNNING'` = 0；`SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300108 | √ |
| 10943 | 把 USC 映射规则的启用状态与 `GDRnext` 映射以迁移脚本发放：① 生产环境「过时USC映射到新USC」列表查不到 `SH000888`、`SH000903`、`SH000922`、`SH000980` 四条规则——`V300103` 建表时四条均为 `flag_enable='0'`，之后只在 LocalDocker 通过接口手工启用，启用状态没有落到迁移脚本；② 富途把黄金下月期货 symbol 写成小写 `GDRnext`（`futu.finv_futu_mapping_security`、`futu.finv_futu_quote_collect` 均为 `('71009930','GDRnext','GDRNext')`），需要映射为大写形态 `GDRNext`，规则表里此前没有这条记录。新增迁移 `V300109`：已存在的规则校正 `usc_new`／`match_mode='exact'`／`priority=100` 并置 `flag_enable='1'`（同一 `usc_old` 多行时只更新「已启用优先、idn 最小」一行，避开部分唯一索引），不存在的规则补插；目标 usc 登记兜底（SH 系列从旧 usc 复制元数据，`GDRNext` 用显式口径兜底，均 `ON CONFLICT DO NOTHING` 只补不覆盖）；末尾校验 5 条规则生效且 5 个目标 usc 已登记，不符 `RAISE EXCEPTION` 回滚。幂等：重复执行更新 0 行、新增 0 行。 | `test -f Deploy/Migrations/V300109__cdeu_usc_patch_rule_deliver_enabled.sql` 退出码 0；构造「四条停用 + `GDRnext` 缺失」样例执行输出「更新规则 4 行，新增规则 1 行」，构造「四条规则缺失」样例输出「更新规则 0 行，新增规则 5 行」，两者校验均通过；本机重复执行输出「更新规则 0 行，新增规则 0 行」；`SELECT count(*) FROM cdeu.finv_quote_cdeu_usc_patch_rule WHERE usc_old IN ('SH000888','SH000903','SH000922','SH000980','GDRnext') AND flag_enable='1'` = 5；`SELECT usc_new, match_mode, priority FROM cdeu.finv_quote_cdeu_usc_patch_rule WHERE usc_old='GDRnext'` 为 `GDRNext / exact / 100`；`SELECT count(*) FROM cdeu.finv_quote_cdeu_usc WHERE usc IN ('SH0888','SH0903','SH0922','SH0980','GDRNext')` = 5；应用后 `SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300109 | √ |
| 10944 | 行情证券登记表 `region`/`market` 口径纠正为 `market_symbol`：生产「历史月份存量数据清点」页面与导出文件名出现 `CN_SSE-A_…`、`US_US-ARCA_…`、`FX_FX_BTC_…`、`UNKNOWN_UNKNOWN_GDRnext_…` 等段——登记表里 `market` 存的是 `public.finv_market.market_flag`（`SSE-A`/`SZSE-A`/`SSE-STAR`/`HKEX-H`/`US-I`/`US-II`/`US-ARCA`/`US-NYSE`/`FX`），而 `{region}_{market}` 目录段与文件名契约要求 `market` 取 `market_symbol`（`SH`/`SZ`/`HK`/`US`/`ARCA`/`NY`/`CER`/`FTC`）。口径：`region` 取 `public.finv_exchange.region`（经 `finv_region` 校验），`market` 取 `public.finv_market.market_symbol`（经 `public.finv_security.market_code` 关联）。迁移 `V300110`：① `finv_market` 新增 `CBOE`（`market_code=1323`、`market_symbol='CBOE'`、`exchange_code=34`）；② 修正期货 `finv_security.market_code`（`1OZMain`/`GCCurrent`/`GCMain`/`MGCMain`/`QOCurrent`/`QOMain`/`SGUMain`/`SICurrent`/`SIMain`→1320 COMEX、`ESMain`/`NQMain`→1321 CME、`YMMain`→1322 CBOT、`BZMain`/`CLMain`/`QMMain`→1319 NYMEX、`VXMain`→1323 CBOE）；③ 按主数据纠正登记表 `market`/`region`，只在现值不是合法 `market_symbol` 时生效；④ 主数据查不到时退回 `market_flag → market_symbol` 唯一映射；⑤ 显式清单：期货 16 条强制对齐、`NOW` 的 `NYSE→NY`、`GDRnext` 登记（`HK/HK`）、`SH0680` 名称纠正；⑥ 校验期货 16 条落点与「登记表无非 `market_symbol` 值」，不符 `RAISE EXCEPTION`。幂等：重复执行影响 0 行。 | `test -f Deploy/Migrations/V300110__cdeu_usc_region_market_symbol_norm.sql` 退出码 0；本机执行输出「期货 market_code 修正 14 行；按主数据纠正 0 行，按 market_flag 映射 0 行，显式清单 16 行」并提示「登记表已无非 market_symbol 口径的证券」，复跑输出「期货 market_code 修正 0 行……显式清单 0 行」；构造生产值样例（`AIQ=US/US-ARCA`、`SH0008=CN/SSE-A`、`688050=CN/SSE-STAR`、`000510=CN/SZSE-A`、`BK1191=HK/HKEX-H`、`BTC=FX/FX`、`EURUSD=FX/FX`、`USDIndex=FX/FX`、`XAUUSD=FX/FX`、`NOW=US/NYSE`、`1OZMain/ESMain/CLMain/YMMain/VXMain=US/COMEX`）演练后依次为 `US/US`、`CN/SH`、`CN/SH`、`CN/SZ`、`HK/HK`、`CC/FTC`、`FX/CER`、`FX/CIX`、`FX/CFD`、`US/NY`、`COMEX`/`CME`/`NYMEX`/`CBOT`/`CBOE`；`SELECT count(*) FROM cdeu.finv_quote_cdeu_usc u WHERE NOT EXISTS (SELECT 1 FROM public.finv_market m WHERE m.market_symbol=u.market)` = 0；`SELECT market_code, market_symbol FROM public.finv_market WHERE market_code=1323` 为 `1323 / CBOE`；应用后 `SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300110 | √ |
| 10945 | 补发本地独有证券的 `public.finv_security` 初始数据并纠正 Prod `000985` 错位：① `899050`/`AMAT`/`MRVL`/`NOW`/`PLTR`/`SMCI` 只有 `cdeu.finv_quote_cdeu_usc` 登记行，`public.finv_security` 无记录（`899050` 在 `V300062` 种子里有行，被唯一约束 `uq_finv_security_exchange_code` 挡掉：旧别名 `BSE50I` 占用 `(13,'899050')`），无法参与主数据维护与元数据刷新；② Prod `000985` 是错位记录（`V300100` 的 M5 挂账未处理），内容写成中证全指（`StockIndex`/`SH`），实际 `000985` 是深市个股「大庆华科」，中证全指的 usc 是 `SH0985`。迁移 `V300111`：① 整行备份 `BSE50I`/`000985` 到 `archive.finv_security_purge_20261102`，把 `BSE50I.security_code` 改为 `BSE50I` 以释放 `(13,'899050')`（不删除、不合并任何行）；② 插入 6 只证券初始数据（`899050`=13/1130/StockIndex、`AMAT`/`MRVL`/`PLTR`/`SMCI`=30/1315/Stock、`NOW`=30/1316/Stock，`flag_enable='1'`，上市日取需求 10925 已固化的公开日期，中文名上游无来源时用英文全称不编造译名）；③ 把 `000985` 逐字段纠正为「大庆华科 / `CN` / `SZ` / `Stock`」（`exchange_code=12`、`market_code=1120`、`type_secu_sub=Stock.SZ`）；④ 同步登记表：`000985` 纠正为个股，`SH0985`（中证全指）补齐登记（`CN`/`SH`/`StockIndex`，缺则插入只补不覆盖）；⑤ 校验 6 只证券落库与 `000985`/`SH0985` 归属，不符 `RAISE EXCEPTION`。幂等：重复执行影响 0 行。 | `test -f Deploy/Migrations/V300111__finv_security_local_only_issue_and_fix_000985.sql` 退出码 0；本机执行输出「备份 2 行；补发证券 6 行；000985 纠正 0 行；SH0985 登记已就绪」，复跑输出「备份 0 行；补发证券 0 行；000985 纠正 0 行」；`SELECT count(*) FROM public.finv_security WHERE usc IN ('899050','AMAT','MRVL','NOW','PLTR','SMCI') AND flag_enable='1'` = 6；`SELECT market_code FROM public.finv_security WHERE usc='NOW'` 为 `1316`，`AMAT`/`MRVL`/`PLTR`/`SMCI` 为 `1315`，`899050` 为 `1130`；`SELECT security_code FROM public.finv_security WHERE usc='BSE50I'` 为 `BSE50I`；构造 Prod 错位样例（`000985` 写成 `StockIndex`/`1110`/`11` 且删掉 `SH0985` 登记行）演练后 `000985` 为「大庆华科 / Stock / 1120 / 12」、登记表为 `CN`/`SZ`/`Stock`，`SH0985` 为 `CN`/`SH`/`StockIndex`；`SELECT count(*) FROM archive.finv_security_purge_20261102 WHERE usc IN ('BSE50I','000985')` = 2；应用后 `SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300111 | √ |
| 10946 | 补发 `XRP` 的 `public.finv_security` 初始数据：`XRP` 与 899050/AMAT/MRVL/NOW/PLTR/SMCI 同属「仅本地有」清单，但需求 10945 只发了后 6 只，`XRP` 至今没有主数据记录，主数据页面查不到、也无法参与 `RefreshSecurityMetadata` 与 `BackfillUscDefaults`。迁移 `V300112` 按登记表现值口径插入一行：`usc='XRP'`、`exchange_code=10001`、`market_code=10001`（`CRYPTO_10001` → `market_symbol=FTC`）、`security_type='Crypto'`、`type_secu_sub='Crypto.CC'`、`security_code='XRP'`、`currency_type='USD'`、`tz='+00:00'`/`timezone='UTC'`、`init_date=20000000`、`provider='FTMM'`、`flag_enable='1'`；末尾校验主数据口径与登记表 `CC`/`FTC`/`Crypto` 一致，不符 `RAISE EXCEPTION`。幂等：`ON CONFLICT DO NOTHING`，重复执行影响 0 行。 | `test -f Deploy/Migrations/V300112__finv_security_issue_xrp.sql` 退出码 0；执行输出「完成：XRP 主数据已就绪（CC/FTC/Crypto/UTC/USD），登记表口径一致」，复跑 `INSERT 0 0`；`SELECT exchange_code, market_code, security_type, type_secu_sub, currency_type, tz, timezone, flag_enable FROM public.finv_security WHERE usc='XRP'` 为 `10001 / 10001 / Crypto / Crypto.CC / USD / +00:00 / UTC / 1`；`SELECT category, region, market, name, timezone, currency FROM cdeu.finv_quote_cdeu_usc WHERE usc='XRP'` 为 `Crypto / CC / FTC / XRP / UTC / USD`；应用后 `SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300112 | √ |
| 10948 | CC 区域加密货币区分「现货币 FTC」与「加密货币交易对 CTP」：此前两侧共用一个 `market=FTC`，交易对在 `{region}_{market}` 目录段与导出文件名里无法与现货币区分。做法：① `finv_market` 新增 `CTP`（`market_code=10016`，`market_flag='CRYPTO_CTP'`、`market_abbr='CRYPTO'`、`market_name='加密货币交易对'`、`en_security_type='TradePair.CC'`、`base_currency='USD'`、`exchange_code=10001`、`market_symbol='CTP'`、`flag_enable='1'`）；② `public.finv_security` 中交易所 10001 下 `type_secu_sub='TradePair.CC'` 的 172 行改到 10016，另 12 个已登记交易对（`BCHUSDC`/`BTCHKD`/`BTCUSD`/`BTCUSDC`/`BTCUSDT`/`ETHBTC`/`ETHUSDC`/`PAXGUSD`/`SOLBTC`/`USDCUSD`/`USDCUSDT`/`USDTUSD`，历史种子未写 `type_secu_sub`，按显式清单 + 名称含 `/` 双重确认）改到 10016 并补 `type_secu_sub='TradePair.CC'`，合计 184 行；③ `cdeu.finv_quote_cdeu_usc` 上述 12 行 `market` 由 `FTC` 改 `CTP`（`region` 仍 `CC`）；④ 模板表补 `CC / CTP / Crypto = 1440`；⑤ 校验 CTP 行、184 条交易对 `market_code`、登记表 12 行 `CTP` 与 9 个现货币仍 `FTC`（`BTC`/`DOGE`/`ETH`/`PAXG`/`SOL`/`UNI`/`USDC`/`USDT`/`XRP`）、模板行，不符整事务回滚。只改分类与归属，不动行情数据；幂等。 | `test -f Deploy/Migrations/V300113__finv_market_add_ctp_and_regroup_crypto_tradepair.sql` 退出码 0；执行输出「finv_security 交易对归位 184 行；登记表 market 改 CTP 12 行；CTP 模板已就绪」，复跑输出「归位 0 行；改 CTP 0 行」；`SELECT market_code, market_flag, market_symbol, exchange_code, flag_enable FROM public.finv_market WHERE market_code=10016` 为 `10016 / CRYPTO_CTP / CTP / 10001 / 1`；`SELECT m.market_symbol, count(*) FROM public.finv_security s JOIN public.finv_market m ON m.market_code=s.market_code WHERE s.exchange_code=10001 GROUP BY 1` 为 `CTP=184`、`FTC=9`；`SELECT market, count(*) FROM cdeu.finv_quote_cdeu_usc WHERE region='CC' GROUP BY 1` 为 `CTP=12`、`FTC=9`；`SELECT count(*) FROM cdeu.finv_quote_cdeu_usc_day_incr_row_tpl WHERE region='CC' AND market='CTP' AND category='Crypto' AND day_expect_rows=1440` = 1；应用后 `SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300113 | √ |
| 10950 | 新建证券日 K 线表 `cdeu.finv_quote_secu_kline_day`（先在 `cdeu` schema，结构稳定后再按同结构扩展到 `quote`）：① 一天一行，主键 `(date, usc)`；`ts` 按用户要求保留为普通列（日K线时间戳，通常为收盘时刻），不参与主键，否则同一天写两次不同 `ts` 会产生重复日线；去掉分钟线的 `time`；② 价格量额列沿用分钟线同名同义（`open`/`high`/`low`/`close`/`last_close`/`change_price`/`change_ratio`/`vwap`/`volume`/`turnover`）；③ 新增日线常用量 `trade_count`/`turnover_rate`/`amplitude`/`adj_factor`/`suspend_flag`/`limit_up`/`limit_down`；④ 新增 4 个指标：`open_interest`（持仓量）、`hist_volatility`（历史波动率，小数、20 日窗口年化）、`implied_volatility`（隐含波动率，小数）、`pe_ratio`（市盈率 TTM，可为负）；⑤ 共 29 列，主键 `(date, usc)`、索引 `(usc, date)`、`gmt_update` 触发器复用 `public.vq_set_gmt_update()`，全列中文注释写入口径；⑥ 写入策略与分钟线一致（`INSERT ... ON CONFLICT (date, usc) DO UPDATE` 字段覆盖），不设不可变触发器；⑦ 迁移幂等（表/索引/触发器/注释均可重复执行），失败整事务回滚。 | `test -f Deploy/Migrations/V300114__cdeu_create_finv_quote_secu_kline_day.sql` 退出码 0；执行输出 `NOTICE: V300114 校验通过：29 列 / 主键(date,usc) / 索引 / 触发器 / 列注释齐全`；复跑输出 `relation "finv_quote_secu_kline_day" already exists, skipping` 且校验仍通过；`\d+ cdeu.finv_quote_secu_kline_day` 显示 29 列（含 `ts`、`open_interest`、`hist_volatility`、`implied_volatility`、`pe_ratio`）、`PRIMARY KEY (date, usc)`、索引 `idx_finv_quote_secu_kline_day_usc_date`、触发器 `trg_finv_quote_secu_kline_day_gmt_update`；冒烟：同一 `(date, usc)` 第二行插入报 `unique_violation`、负 `volume` 报 `check_violation`、跨事务 UPDATE 后 `gmt_update > gmt_create`；应用后 `SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300114 | √ |
| 10952 | 把日K同步做成 cdeu 菜单（前端选 usc → 请求接口 → upsert 到 `cdeu.finv_quote_secu_kline_day`），并补齐排障日志：① 后端新增三个接口——`POST /API/V1/Meta/Finv/Quote/Cdeu/DayKline/Sync`（body `{usc, from, to}`，`from`/`to` 为 yyyyMMdd、0 不限）、`GET .../DayKline/Stat?usc=`（行数/日期范围/来源）、`GET .../DayKline/UscList?keyword=&limit=`（候选 usc，取自 `futu.finv_futu_quote_collect` 且 `flag_enable='1'`，`Store.ListCollectUsc`）；② 同步链路每步输出单行 KV 日志，统一前缀 `[cdeu][daykline]`：`api sync begin/end/failed`、`params`、`fetch start/done`、`filter`、`upsert batch`、`sync done/failed(step=collect_params\|fetch\|upsert)`，线上用 `docker logs 容器 2>&1 \| grep '\[cdeu\]\[daykline\]'` 抓取；③ 前端新增菜单「日K线数据同步」（key `cdeu-daykline-sync`，路由 `/Meta/Finv/Quote/Cdeu/DayKline/Sync`，视图 `DayKlineSyncView.vue`）——usc 下拉（可手输）+ 日期区间 + 同步/查询存量按钮 + 本次同步结果 + 存量统计 + 原始响应折叠面板；④ `Web/src/api.ts` 的 `apiPost` 增加可选 `timeoutMs`，同步请求单独放宽到 180s（默认 30s 覆盖不了全量拉取 + 写库，且超时会误报失败）；⑤ 文档：3 篇接口文档 + 1 篇菜单文档 + `Docs/API/README.md`、`Docs/API/APIs.md`、`Docs/Menu/Menus.md` 索引登记。 | `go build ./...`、`go vet ./internal/...` 退出码 0；`go.exe test ./internal/api/` 通过（`router_test` 已含 `POST .../DayKline/Sync`、`GET .../DayKline/Stat`、`GET .../DayKline/UscList` 三条期望路由）；`Web/` 下 `npx vue-tsc --noEmit -p tsconfig.app.json` 与 `npm run build` 均退出码 0，且 `Web/dist/assets/index-*.js` 含「日K线数据同步」；`grep -c 'cdeu\]\[daykline\]' internal/meta/finv/quote/cdeu/daykline_sync.go` = 10；部署后访问菜单 `/Meta/Finv/Quote/Cdeu/DayKline/Sync`，同步 `GCMain` 返回 `code=0` 且 `data.total>0`，`SELECT count(*) FROM cdeu.finv_quote_secu_kline_day WHERE usc='GCMain'` 大于 0；失败时容器日志可按 `[cdeu][daykline]` 过滤到具体 `step` | √ |
| 10951 | 新增「moomoo 网页行情接口」公共抓取工具与 cdeu 日K同步方法，为后续 cdeu 菜单（前端选 usc → 请求 → upsert 日K）打底：① 新包 `internal/meta/component/quote/moomoo/quoteapiv2`——`Sign`/`SignParams`/`SignInput`/`SignedQuery` 复现 `quote-token` 算法（HMAC-SHA512→hex 前 10→SHA-256→hex 前 10，密钥 `quote_web`，空输入降级 `quote`；签名输入为紧凑 JSON、键序保真、GET 值全部字符串化），`Client.Kline(ctx, KlineRequest)` 调 `GET /quote-api/quote-v2/get-kline`（键序 `stockId,marketType,type,marketCode,instrumentType,subInstrumentType,_`），响应 `k/o/h/l/c/v/t/r/lc/cp` 用 `flexNum` 兼容字符串与数字混用，`-403` 通过 `ErrTokenRejected` 识别；② 请求参数一律取自 `futu.finv_futu_quote_collect`（`Store.CollectQuoteParams`，仅 `flag_enable='1'`，无记录返回 `ErrCollectParamsNotFound`），五个定位参数原样透传不做语义换算；③ `Service.SyncDayKlineFromMoomoo`——拉全量历史 → 按 `cdeu.finv_quote_cdeu_usc.timezone` 把秒级时间戳换算成市场本地日期 → 按 `from`/`to`（yyyyMMdd）过滤 → 每 2000 行一批 upsert 到 `cdeu.finv_quote_secu_kline_day`（`provider='FTMM'`）；`change_ratio` 不写接口 `r`（样本恒 null、口径未验证），改用 `change_price / last_close × 100` 与存量分钟线口径一致；④ `Store.UpsertDayKlines` 以 `jsonb_to_recordset` 整批写入、`(date, usc)` 冲突时 `COALESCE(EXCLUDED.x, t.x)` 只覆盖本次有值的列（避免抹掉其它来源回填的 `open_interest`/`pe_ratio`），`xmax = 0` 区分新增/更新；⑤ `Store.DayKlineStats` 查某 usc 的存量统计。 | `go build ./...`、`go vet ./internal/meta/...` 退出码 0；`go.exe test ./internal/meta/component/quote/moomoo/quoteapiv2/` 通过（含文档复现向量：`quote`→`58af97268c`、`{"a":"1","b":"2"}`→`39adef0837`、3A→`45671b44c1`、3B→`7feb1b1665`、1A→`055b0960ec`；httptest 校验查询键序、`quote-token`、csrf/cookie 头与样本响应解析、`-403` 识别）；设 `FINV_TEST_PG_DSN` 后 `go test -run TestUpsertDayKlinesIntegration` 与 `-run TestSyncDayKlineFromMoomooIntegration` 均 PASS（插入/覆盖幂等、其它来源字段保留、时区换算 `20261001`、区间过滤、请求参数透传、未登记 usc 报错、`change_ratio=2.941176`）；真实接口用 `FINV_MOOMOO_LIVE=1 go test -run TestLiveGCMainDayKline` 验证（需可访问 `www.moomoo.com`）；本机实测不可达：`curl -4/-6 https://www.moomoo.com/` 均返回 `000`（DNS 解析到 `43.174.250.223` 正常）、容器内 `wget` 报 `SSL routines::unexpected eof while reading / Connection reset by peer`，故真实拉取待可访问环境执行 | √ |
| 10949 | 「历史月份存量数据清点」页的「应有/实际行数」口径不一致：`应有` 取的是成熟度表的月累计 `sum(expected_rows)`，而该表按交易日每天一行、每行记的是**每日**应有行数，成熟度记录不全时（如 FX 只记 1 天）就退化成「日值」，与实际（整月累计）直接比较没有意义（`1,380 / 32,618`）。修复：`ExportStockPage` 明细查询改为 `应有 = 日应有 × 交易日`——`交易日` 优先取主表覆盖天数（`count(DISTINCT date)`），主表已清理时退回成熟度记录天数；证券主数据缺 `day_expect_rows` 时仍回退成熟度月累计。接口新增返回 `day_expect_rows`、`expected_days` 两个字段，页面在「应有/实际」下标注 `每日 X × N 交易日`（回退时标注「成熟度月累计」）。 | `go build ./...`、`go vet ./internal/meta/finv/quote/cdeu/` 退出码 0；`Web/` 下 `npx vue-tsc --noEmit -p tsconfig.app.json` 退出码 0；按同一段 SQL 在本机 2026-10 数据上核对：`BTC`/`BTCUSD`（`day_expect_rows=1440`、主表与成熟度各 5 天）`expected_rows=7,200`、`expected_days=5`，`SH0980`（241 × 5）`=1,205`，`899050`（主表已清理、成熟度 5 天）`=1,205`；页面「应有/实际」列显示 `7,200 / 7,195` 并在下方标注 `每日 1,440 × 5 交易日`；生产上 FX 类证券由 `1,380 / 32,618` 变为 `1,380 × 23 = 31,740 / 32,618` | √ |
| 10947 | 「过时USC映射到新USC」菜单默认只显示 20 条：规则表现有 26 条，页面按 `pageSize=20` 取数且没有「每页」选择器，第一页看不到第 21 条之后的规则，容易误判为「只查到 20 条」。修复：前端 `UscPatchView.vue` 增加「每页」下拉（20/50/100/200），默认取服务端上限 200（该表是小字典表，一页即可看全）；切换每页条数时回到第 1 页重新查询；`Docs/API/Cdeu/UscPatch.md` 的 `pageSize` 说明同步为「默认 200、上限 200、`<=0` 回退 20」。 | `grep -c '每页' Web/src/views/Meta/Finv/Quote/Cdeu/UscPatchView.vue` ≥ 1；`grep -n 'const pageSize = ref(200)' Web/src/views/Meta/Finv/Quote/Cdeu/UscPatchView.vue` 命中；`npx vue-tsc --noEmit -p tsconfig.app.json`（`Web/` 目录下）退出码 0；打开菜单「过时USC映射到新USC」时「共 26 条」且表格一次列出 26 行；`GET /API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/List?page=1&pageSize=200` 返回 `total=26` 且 `list` 长度 26 | √ |
| 10953 | moomoo 行情客户端与上游 MetaExt 对齐（只对齐 kline）：① `internal/meta/component/quote/moomoo/quoteapiv2` 按上游「每个接口一个源文件」拆分——新增 `kline.go`（`GetKLineRequest`／`KLineBar`／`Client.GetKLine`）与 `kline_test.go`，`client.go` 只保留配置、错误与请求发送；② 公共部分与上游同名同签名：`Config.Path` → `Config.KLinePath`（`normalizePath` 规整）、`get`／`url` 增加 `path` 参数、新增 `decode`、`flexNum` → 导出 `FlexNum`、`ErrTokenRejected` → `IsTokenRejected`、`APIError.Error()` 补 `Message != ""` 分支、`KlineTypeDay` → `KLineTypeDay`；③ 刻意裁剪（不搬上游其它接口）：无 `post`／`do`／`DefaultOrigin`、无 stocklist 等 7 个路径常量与 `Config` 字段、无 `IsParamsError`／`IsParamsMissing` 与 500／404 业务码、无 `defaultIfEmpty`／`intParam`，裁剪点写入 `client.go` 包注释；④ 调用方 `internal/meta/finv/quote/cdeu/daykline_sync.go` 的 `KlineFetcher` 接口与 `dayKlineChangeRatio` 改用具名类型；⑤ 需求 10951 条目原文保持不动，其 `Client.Kline`／`flexNum`／`ErrTokenRejected` 表述由本条取代。 | `go build ./...`、`go vet ./internal/meta/...` 退出码 0；`go test ./internal/meta/component/quote/moomoo/quoteapiv2/ -count=1` PASS（含上游 8 个 kline 用例：请求装配与解析、显式参数透传、HTTP 502、解析失败、空 stockId 不发请求、上下文取消、其它业务码、token 被拒）；`go test ./internal/meta/finv/quote/cdeu/ -run 'TestDayKline\|TestUpsertDayKlines\|TestSyncDayKline'` PASS；`diff` 校验 `kline.go`／`kline_test.go` 与 MetaExt `component/quote/moomoo/quoteapiv2/` 同路径文件逐字一致；真实接口已验证（2026-11-03，本机可访问 `www.moomoo.com`）：`GCMain`（stockId=70000294／marketType=2／marketCode=70／instrumentType=9／subInstrumentType=9008）返回 12819 根日K（19750102~20261102），经 `SyncDayKlineFromMoomoo` 落库 `cdeu.finv_quote_secu_kline_day` 12819 行、耗时 4.7s，二次同步 0 新增／12819 更新；全表 `change_ratio` 与 `change_price/last_close*100` 偏差为 0。 | √ |
| 10954 | moomoo 行情客户端由「仓库内副本」改为直接依赖上游模块：① 删除 `internal/meta/component/quote/moomoo/quoteapiv2`（5 个文件），改为 `require github.com/acnxo/MetaExt v0.1.0`（该模块 `go.mod` 零第三方依赖，`go mod tidy` 后为直接依赖）；② `cdeu` 侧 4 处 import（`daykline_sync.go`、`service.go`、`daykline_live_test.go`、`store_daykline_integration_test.go`）改指 `github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2`，调用方使用的 `GetKLine`／`GetKLineRequest`／`KLineBar`／`KLineTypeDay`／`IsTokenRejected`／`Config.KLinePath` 均为上游同名符号，无需本地适配层；③ 需求 10953 的包路径与 `go test ./internal/meta/component/quote/moomoo/quoteapiv2/` 核对口径由本条取代（10953 原文不改）；④ 上游 `v0.1.0` 的 `kline.go`／`client.go` 与 `main` 逐字一致，故以 tag 固定版本；⑤ 上游模块尚未被 sum.golang.org 收录，首次 `go get` 需 `GOSUMDB=off`，`go.sum` 落定后 CI 无需特殊配置。 | `go build ./...`、`go vet ./internal/meta/...` 退出码 0；`go list -m github.com/acnxo/MetaExt` 输出 `v0.1.0`；`grep -rn 'finvquant/internal/meta/component' internal cmd` 无结果；`go test ./internal/meta/finv/quote/cdeu/` 除既有 Windows 平台用例 `TestDirUploader` 外通过；真实接口实测（2026-11-03）`GCMain` 返回 12819 根日K、二次同步 0 新增／12819 更新。 | √ |
| 10955 | 依赖上游 MetaExt 升级到 `v0.1.1`：`go.mod` 的 `github.com/acnxo/MetaExt` 由 `v0.1.0` 升至 `v0.1.1`，`go.sum` 两行随之替换（`go mod tidy` 未触碰其它条目）。上游 `v0.1.1` 未改动本仓库依赖的 `component/quote/moomoo/quoteapiv2`（`git diff v0.1.0 v0.1.1 -- component/quote/moomoo/quoteapiv2` 为空），变更集中在新增 `base/version`（版本号由 `VERSION` 声明并可代码读取）、新增 `component/quote/sina/finance/cnatradecalendar`（原 `sdk/sina/tradecalendar` 移除）与文档/CI 扩充，`go.mod` 仍为零第三方依赖，故本仓库调用方零改动。 | `go list -m github.com/acnxo/MetaExt` 输出 `v0.1.1`；`go build ./...`、`go vet ./...` 退出码 0；`diff -r` 校验模块缓存中 `MetaExt@v0.1.1/component/quote/moomoo/quoteapiv2` 与上游 tag 逐字一致；`go test ./internal/meta/...` 除既有 Windows 平台用例 `TestDirUploader` 外通过（用 `git archive HEAD` 的 `v0.1.0` 检出对照，同一用例同样失败，非本次回归）；真实接口实测（2026-11-03）`GCMain` 返回 12819 根日K（19750102~20261102，首根收盘 178.2、末根收盘 3956.6、昨收 3884.9），时间升序且收盘价无空值。 | √ |
| 10956 | 新建证券实时行情快照表 `cdeu.finv_cdeu_secu_realtime_stock`：① 一行一证券，主键 `stock_id`（约束名 `finv_cdeu_secu_realtime_stock_pk`）；② 共 15 列：`usc`、`stock_id`、`name`、`stock_code`、`market_type`、`market_label`、`instrument_type`、`currency`（货币类型，ISO 4217 三位字母代码，如 `CNY`/`USD`）、`market_value`、`total_shares`、`circulation_total_shares`、`trade_trunover`、`trade_volumn`、`circulation_market_value`、`dt_update`（Unix 毫秒时间戳）；③ 列名 `trade_trunover`/`trade_volumn` 按用户给定 DDL 保留上游拼写，列注释标注其含义为 turnover（成交额）/ volume（成交量）；④ 表名沿用用户给定的 `finv_cdeu_` 前缀（cdeu 域常规前缀为 `finv_quote_cdeu_`，本条属刻意例外，已在 `DbSchemaSpec` §1 登记）；⑤ 表注释与 15 个列注释全部为中文，`currency` 注释引用 `ISO 4217`（`ISO 4217:2015`）；⑥ 迁移幂等（表/约束/注释可重复执行），末尾校验列清单、主键名与主键列、列注释数量，任一不符整事务回滚；⑦ 严格贴合给定 DDL：不新增 `gmt_create`/`gmt_update`、不建触发器、不加二级索引。 | `test -f Deploy/Migrations/V300115__cdeu_create_finv_cdeu_secu_realtime_stock.sql` 退出码 0；执行输出 `NOTICE: V300115 校验通过：15 列 / 主键(stock_id) / 列注释齐全`；复跑输出 `relation "finv_cdeu_secu_realtime_stock" already exists, skipping` 且校验仍通过；`\d+ cdeu.finv_cdeu_secu_realtime_stock` 显示 15 列（含 `currency`、`dt_update`）、`PRIMARY KEY (stock_id)`、约束名 `finv_cdeu_secu_realtime_stock_pk`；`SELECT col_description('cdeu.finv_cdeu_secu_realtime_stock'::regclass, 8)` 含 `ISO 4217`；同一 `stock_id` 插入第二行报 `unique_violation`；应用后 `SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300115 | |
| 10957 | 为证券实时行情快照表 `cdeu.finv_cdeu_secu_realtime_stock` 增加批量 Upsert 接口：① 内部 `POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/StockUpsert`（需登录）+ 对外 `POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/StockUpsert`（同一 handler，暂不鉴权，响应经 `WrapOpenAPIResponse` 包成 `{code,data,message}`）；② 请求体 `{"items":[...]}`（snake_case，严格解码，未定义字段 400），单请求 1–500 条（`MaxRealtimeStockBatchRows=500`）；③ 主键 `stock_id` 必填、1..2147483647、批内唯一（重复 400 并指出位置）；④ 冲突语义为字段级覆盖（`ON CONFLICT (stock_id) DO UPDATE`，除主键外 14 列均 `COALESCE(EXCLUDED.x, t.x)`，源端缺字段不清空旧值；空串等同未提供）；⑤ `dt_update` 例外：省略或 `null` 时按本次写入时刻刷新（新行与更新行都填 Unix 毫秒），显式传入按传入值落库；⑥ 数值字段只用 `json.Number` 承载原文以 text 入参、数据库转 `NUMERIC(24,6)`，不经 `float64`；⑦ 整批单条 `INSERT ... ON CONFLICT` 语句，失败整批回滚，成功返回 `{total,inserted,updated}`（`xmax = 0` 区分新增/更新）；⑧ 数据库码 `22003`/`22P02`/`22007`/`22008` 映射为 400（入参问题），其余写库错误 `2006`；⑨ 日志前缀 `[cdeu][realtimestock]`；⑩ 文档：内部 `Docs/API/Cdeu/RealtimeStockUpsert.md` + `.yml`、对外 `Docs/OpenAPI/Quote/Cdeu/Realtime/StockUpsert.md` + `.yml`（OpenAPI 3.1，各自自包含），并在 `Docs/API/README.md`、`Docs/API/APIs.md` 登记；⑪ `internal/database/schema_qualify_test.go` 增 `finv_cdeu_` → `cdeu.` 前缀映射。 | `grep -c 'Realtime/StockUpsert' internal/api/router.go` = 2；`test -f Docs/API/Cdeu/RealtimeStockUpsert.md`、`test -f Docs/API/Cdeu/RealtimeStockUpsert.yml`、`test -f Docs/OpenAPI/Quote/Cdeu/Realtime/StockUpsert.md`、`test -f Docs/OpenAPI/Quote/Cdeu/Realtime/StockUpsert.yml` 退出码均为 0；`go build ./...`、`go vet ./...` 退出码 0；`go test ./internal/api/ -run TestExportArchiveRoutesRegistered` PASS（期望路由含内部与 `/OpenAPI/V1` 两条）；`go test ./internal/database/` PASS；`curl -X POST 'http://127.0.0.1:16001/API/V1/Meta/Finv/Quote/Cdeu/Realtime/StockUpsert' -H 'Content-Type: application/json' -d '{"items":[{"stock_id":1001,"usc":"SH600000","currency":"CNY"}]}'` 返回 `code=0`、`data.inserted=1`；再提交 `{"items":[{"stock_id":1001,"market_value":100000000}]}` 返回 `data.updated=1`，且 `SELECT usc, market_value, dt_update FROM cdeu.finv_cdeu_secu_realtime_stock WHERE stock_id=1001` 中 `usc` 仍为 `SH600000`（缺字段未清空）、`market_value=100000000`、`dt_update` 已刷新；`items` 超过 500 条或批内重复 `stock_id` 返回 400；对外路径同请求返回 `{"code":0,"data":{...},"message":"OK"}` | |
| 10958 | 修正 `cdeu.finv_cdeu_secu_realtime_stock.stock_id` 的列宽设计：V300115 按给定 DDL 建成 `INTEGER`（int4，上限 2147483647），而该列承载的是上游富途 64 位 stockId（实测最大 `148739012639708`），源库 `RTQuote.db` 的 13045 行中有 9561 行装不下，被 API 的 int32 校验挡掉。① 新增迁移 `V300116` 把该列扩为 `BIGINT`（`ALTER COLUMN stock_id TYPE BIGINT`，仅放宽值域，不动数据/表名/归属/其它列；幂等，末尾校验列类型、主键、注释数，不符回滚）；② API 校验由 `math.MaxInt32` 放宽为 `stock_id > 0`（上限交给 int64 与数据库），`*int64` 字段不变；③ 内部/对外接口文档与 OpenAPI 契约同步（`format: int64`、`maximum: 9223372036854775807`）；④ 需求 10957 中「1..2147483647」的校验口径由本条取代（10957 原文不改）。 | `test -f Deploy/Migrations/V300116__cdeu_realtime_stock_id_bigint.sql` 退出码 0；`grep -c 'MaxInt32' internal/meta/finv/quote/cdeu/realtime_stock.go` = 0；`grep -c '9223372036854775807' Docs/API/Cdeu/RealtimeStockUpsert.yml Docs/OpenAPI/Quote/Cdeu/Realtime/StockUpsert.yml` 各 = 1；执行 V300116 输出 `NOTICE: V300116 校验通过：stock_id=bigint / 主键(stock_id) / 列注释 15`，复跑输出「已是 bigint，跳过」；`SELECT data_type FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_cdeu_secu_realtime_stock' AND column_name='stock_id'` 为 `bigint`；`SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300116；用 `SyncRealtimeStock.py` 全量提交后 `SELECT count(*), max(stock_id) FROM cdeu.finv_cdeu_secu_realtime_stock` 为 `13045 / 148739012639708` | √ |
| 10959 | 为 `cdeu.finv_cdeu_secu_realtime_stock` 增加 7 个结构化数值列，替代上游展示文本（含 `+/-`、`%`、万/亿/万亿）：`price_nominal`（名义价格/单价）、`change_ratio`（涨跌幅）、`trust_aum`（信托/基金资产规模，元）、`trade_change_ratio`（成交额变化率，由上游拼写 `trade_changeraio` 规范化命名）、`price_amplitude`（振幅）、`volumn_ratio`（量比）、`buysell_ratio`（买卖比）。① 百分数列存百分点数值（`1.03` 表示 1.03%）；`trust_aum` 存元（万=1e4、亿=1e8、万亿=1e12 换算）；全部 `NUMERIC`；② 列序紧随原 15 列，且 `price_nominal` 位于新增列首位（`ADD COLUMN` 顺序即列序）；③ 迁移 `V300117` 幂等，末尾校验新增列数量/类型/列序与注释数（22），不符回滚；④ API 模型（`RealtimeStockItem`）、写入 SQL（列清单与 `COALESCE(EXCLUDED.x, t.x)`）、严格解码同步支持 7 个字段；⑤ 内部与对外接口文档、OpenAPI 契约（`type: [number,'null']`，标注百分点/元口径）同步；⑥ 同步脚本 `SyncRealtimeStock.py` 按规则解析（去 `+`/`%`、万/亿/万亿换算、纯数直取）后发送。 | `test -f Deploy/Migrations/V300117__cdeu_realtime_stock_add_numeric_metrics.sql` 退出码 0；`go build ./...`、`go vet ./...` 退出码 0；执行 V300117 输出 `NOTICE: V300117 校验通过：7 个数值列已就绪（price_nominal 在前），列注释 22`，复跑无 ALTER 副作用；`SELECT string_agg(column_name, ',' ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_cdeu_secu_realtime_stock' AND ordinal_position > 15` = `price_nominal,change_ratio,trust_aum,trade_change_ratio,price_amplitude,volumn_ratio,buysell_ratio`；`SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300117；脚本全量提交后 `SELECT count(*) FILTER (WHERE price_nominal IS NOT NULL), count(*) FILTER (WHERE change_ratio IS NOT NULL), count(*) FILTER (WHERE trust_aum IS NOT NULL), count(*) FILTER (WHERE trade_change_ratio IS NOT NULL), count(*) FILTER (WHERE price_amplitude IS NOT NULL), count(*) FILTER (WHERE volumn_ratio IS NOT NULL), count(*) FILTER (WHERE buysell_ratio IS NOT NULL) FROM cdeu.finv_cdeu_secu_realtime_stock` 各为 13045；抽样 `SELECT price_nominal, change_ratio, trust_aum, trade_change_ratio, price_amplitude, volumn_ratio, buysell_ratio FROM cdeu.finv_cdeu_secu_realtime_stock WHERE stock_id=2` 为 `73.7 / 1.03 / 0 / 0.12 / 1.3 / 1.15 / 0`；`SELECT max(trust_aum)::text FROM cdeu.finv_cdeu_secu_realtime_stock` = `18446740000000000.000000`（对应源 `18446.74万亿`，上游脏值原样换算、未清洗） | √ |
| 10960 | 为 `cdeu.finv_cdeu_secu_realtime_stock` 增加 `change_price`（涨跌额）数值列，对应上游 `secu_realtime_quote.change`（带正负号的文本，如 `+0.750` / `-1.240`）：① 迁移 `V300118` 以 `NUMERIC(20,6)` 追加——PostgreSQL `ADD COLUMN` 只能追加到表尾，故落在第 23 列，幂等并校验 `data_type`/`ordinal_position`/注释数（23），不符回滚；② API 模型（`RealtimeStockItem`）、写入 SQL（列清单、`jsonb_to_recordset` 声明与 `COALESCE(EXCLUDED.x, t.x)`）、严格解码同步；③ 内部与对外接口文档、OpenAPI 契约（`type: [number,'null']`）同步；④ 同步脚本 `SyncRealtimeStock.py` 以 `to_number` 解析（去正号）后发送；⑤ 列名与日K表 `finv_quote_secu_kline_day.change_price` 同义。 | `test -f Deploy/Migrations/V300118__cdeu_realtime_stock_add_change_price.sql` 退出码 0；`go build ./...`、`go vet ./...` 退出码 0；执行输出 `NOTICE: V300118 校验通过：change_price 为 numeric 第 23 列，列注释 23`，复跑无副作用；`SELECT data_type\|\|'/'\|\|ordinal_position FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_cdeu_secu_realtime_stock' AND column_name='change_price'` = `numeric/23`；`SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300118；脚本全量提交后 `SELECT count(change_price), min(change_price), max(change_price) FROM cdeu.finv_cdeu_secu_realtime_stock` = `13045 / -248.040000 / 1511.154000`；抽样 `SELECT change_price FROM cdeu.finv_cdeu_secu_realtime_stock WHERE stock_id=2` = `0.750000`（源 `+0.750`） | √ |
| 10961 | 为 `cdeu.finv_cdeu_secu_realtime_stock` 增加 `price_direct`（涨跌方向）与 `leverage`（杠杆）两列：① `price_direct` 用 `CHAR(1)` 存**符号**——源 `up`→`+`、`down`→`-`、`flat`→`0`（三态完整，flat 不丢）；② `leverage` 用 `NUMERIC(20,6)` 直存数值（源值可为负；10659 行源为空则不写入）；③ 迁移 `V300119` 追加到表尾（第 24、25 列），幂等并校验各列 `data_type`/`ordinal_position` 与注释数（25），不符回滚；④ API 模型（`PriceDirect *string`、`Leverage *json.Number`）、写入 SQL（列清单与 `COALESCE(EXCLUDED.x, t.x)`）、严格解码同步；⑤ 内部与对外接口文档、OpenAPI 契约同步（`price_direct` 用 `enum: ['+','-','0',null]`）；⑥ 同步脚本新增 `to_direct`（up→+、down→-、flat→0）与 `leverage` 数值解析。 | `test -f Deploy/Migrations/V300119__cdeu_realtime_stock_add_price_direct_leverage.sql` 退出码 0；`go build ./...`、`go vet ./...` 退出码 0；执行输出 `NOTICE: V300119 校验通过：price_direct 第 24 列（char(1)）/ leverage 第 25 列（numeric）/ 注释 25`，复跑无副作用；`SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300119；脚本全量提交后 `SELECT price_direct, count(*) FROM cdeu.finv_cdeu_secu_realtime_stock GROUP BY 1 ORDER BY 1` 为 `+`=6058、`-`=6001、`0`=986；`SELECT count(leverage), min(leverage), max(leverage) FROM cdeu.finv_cdeu_secu_realtime_stock` = `2386 / -3.000000 / 4.000000`；抽样 `SELECT price_direct, leverage FROM cdeu.finv_cdeu_secu_realtime_stock WHERE stock_id=2` = `+ / NULL`（源 `up` / 空） | √ |
| 10962 | 在前端「行情」下、「历史行情」分组之后新增三级分组「实时行情」及其叶子菜单（key `quote-realtime-stock`，路由 `/Meta/Finv/Quote/Realtime/Stock`，视图 `Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue`），用于只读展示 `cdeu.finv_cdeu_secu_realtime_stock` 的最新快照：① 新增后端分页查询接口 `GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Page`——关键字（`usc`/`stock_code`/`name` 的 ILIKE）、`marketLabel`、`currency` 筛选，`orderBy` 白名单（10 列）+ `order`、`page`/`pageSize`（1–500），NULL 一律排最后并以 `stock_id` 兜底稳定次序；数值列以 `::text` 返回以保 `NUMERIC(24,6)` 精度；② 页面含筛选/排序/每页/查询·刷新·重置、23 列固定表头横向滚动表格、分页与总数，大额数值按万/亿/万亿折算、涨跌红绿着色、空值显示 `—`；③ 文档：接口 `Docs/API/Cdeu/RealtimeStockPage.md` + `.yml`、菜单 `Docs/Menu/Quote/Realtime/RealtimeStock.md`，并登记 `Docs/API/README.md`、`Docs/API/APIs.md`（113）、`Docs/Menu/Menus.md`（41）。 | `grep -c 'quote-realtime-stock' Web/src/config/menuConfig.ts Web/src/router.ts` 各 = 1；`grep -c 'Realtime/Stock/Page' internal/api/router.go` = 1；`go build ./...`、`go vet ./...` 退出码 0；`go test ./internal/api/ -run TestExportArchiveRoutesRegistered` PASS（期望路由含 `GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Page`）；`cd Web && npm run build` 退出码 0（`vue-tsc -b` 类型检查通过）；`test -f Docs/API/Cdeu/RealtimeStockPage.md`、`test -f Docs/API/Cdeu/RealtimeStockPage.yml`、`test -f Docs/Menu/Quote/Realtime/RealtimeStock.md` 均为 0；登录后 `GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Page?page=1&pageSize=5&orderBy=market_value&order=desc` 返回 `code=0`、`data.total=13045`、`data.list` 长度 5 且按市值降序；`?orderBy=bogus` 返回 400；`?pageSize=501` 返回 400；未登录返回 401；`?orderBy=market_value&order=desc&pageSize=1` 的 `data.list[0].stock_id` = 202597（英伟达，`5.06e12`，证明按 numeric 而非 text 排序） | √ |
| 10963 | 调整前端「实时行情」菜单（需求 10962）的路由与列表展示：① 路由 `/Meta/Finv/Quote/Realtime/Stock` 改为 `/Meta/Finv/Quote/Realtime/FTStockRank`，菜单 key 同步为 `quote-realtime-ftstockrank`（`Web/src/router.ts` 的 `path`/`name` 与 `Web/src/config/menuConfig.ts` 叶子两处一致）；② 表格取消「证券编号」列（仍保留为排序字段下拉项）与「方向」列；③ 涨跌方向改用**整行颜色**表达——`price_direct` 为 `+` 的整行显示红色、为 `-` 的整行显示绿色（`0`/空为默认色），即红涨绿跌；④ 文档同步：菜单文档路由/key、`Docs/Menu/Menus.md` 层级树与索引行、API 文档「已使用位置」。 | `grep -c 'Realtime/FTStockRank' Web/src/router.ts Web/src/config/menuConfig.ts` 各 = 1；`grep -rc 'Realtime/Stock' Web/src/router.ts Web/src/config/menuConfig.ts Docs/Menu/Menus.md` 均为 0；`grep -c '证券编号' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 1（仅排序下拉项，表格列已删除）；`grep -c 'row-rise\|row-fall' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` ≥ 3；`cd Web && npm run build` 退出码 0；部署后访问 `/Meta/Finv/Quote/Realtime/FTStockRank` 返回 SPA 页面，表格无「证券编号」「方向」两列，涨的证券整行为红色、跌的整行为绿色（对比 `price_direct` 字段确认） | √ |
| 10964 | 前端「实时行情」叶子菜单更名为「**证券行情排行**」（路由/`key`/视图不变，仍为 `/Meta/Finv/Quote/Realtime/FTStockRank`、`quote-realtime-ftstockrank`、`RealtimeStockView.vue`）：① `Web/src/config/menuConfig.ts` 叶子 `title`、② `Web/src/router.ts` 的 `meta.title`、③ 页面 `v-card-title` 文案、④ 菜单文档标题与所属导航、`Docs/Menu/Menus.md` 层级树与索引行、⑤ API 文档「已使用位置」文字同步。父分组名仍为「实时行情」。 | `grep -c "title: '证券行情排行'" Web/src/config/menuConfig.ts` = 1；`grep -c "meta: { title: '证券行情排行' }" Web/src/router.ts` = 1；`grep -c '证券行情排行' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` ≥ 1；`grep -c '\| 41 \| 证券行情排行 \|' Docs/Menu/Menus.md` = 1；`cd Web && npm run build` 退出码 0；部署后左侧菜单显示「实时行情 → 证券行情排行」，页面标题一致 | √ |
| 10965 | 调整前端「证券行情排行」页面布局：① 筛选控件（关键字/市场/币种/排序字段/方向）与「查询」「刷新」「重置」按钮**同处一行**——筛选区由 v-row 栅格改为 flex 换行布局，按钮紧跟筛选控件之后，仅在窄屏才换行，不再把「重置」挤到第二行；② 每页条数选择器从筛选行移到底部，固定显示在**页码控件左侧**；底部左侧仍显示「共 X 条，第 x / y 页」。 | `grep -c 'd-flex flex-wrap align-center ga-2 mb-3' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 1；`grep -c '每页条数固定在页码控件左侧' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 1；`grep -c 'label="每页"' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 1（已不在筛选区）；`cd Web && npm run build` 退出码 0；部署后页面「查询/刷新/重置」与筛选控件同一行，每页选择器位于底部页码左侧，切换每页条数回到第 1 页 | √ |
| 10966 | 「证券行情排行」涨跌行着色的红色由 Vuetify `error` 主题色（偏暗红 `#B00020`）改为**正红 `#FF0000`**，绿色维持 success 主题色；仅改 `.finv-rt-table` 的 `tr.row-rise` 颜色值，判定逻辑（`price_direct` 为 `+`/`-`）不变。 | `grep -c 'color: #ff0000' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 1；`grep -c 'rgb(var(--v-theme-error))' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 0；`cd Web && npm run build` 退出码 0；部署后涨的证券整行显示为正红 | √ |
| 10967 | 「证券行情排行」跌的整行颜色由 Vuetify `success` 主题绿（`#4CAF50`）改为**亮绿 `#00E676`**，与需求 10966 的正红 `#FF0000` 对称；判定逻辑不变。 | `grep -c 'color: #00e676' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 1；`grep -c 'var(--v-theme-success)' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 0；`cd Web && npm run build` 退出码 0；部署后跌的证券整行显示为亮绿 | √ |
| 10968 | 「证券行情排行」表格取消内部纵向滚动：去掉 `v-table` 的 `fixed-header` 与 `height="60vh"`，表格按内容自然展开、由页面整体滚动承载（横向滚动保留，列多时仍可左右查看）。 | `grep -c 'fixed-header' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 0；`grep -c 'height="60vh"' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 0；`cd Web && npm run build` 退出码 0；部署后页面出现 50 行时不出现表格内滚动条，滚动条属于整页 | √ |
| 10969 | 「证券行情排行」表格列宽优化：① 所有 `th`/`td` 一律 `white-space: nowrap`，列宽随内容撑开、不再把「2.51 万亿」「1.03%」这类值挤到第二行；② 单元格左右内边距由 compact 默认放宽到 12px；③ 指标列（价格/涨跌/市值/股本/成交/AUM/比率，带 `num` class）最小宽度 96px；列数超出视口时整体横向滚动。 | `grep -c 'white-space: nowrap' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` ≥ 1；`grep -c 'padding-inline: 12px' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 1；`grep -c 'text-right num' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 15；`cd Web && npm run build` 退出码 0；部署后任意数值列（含「万亿」单位）均单行显示，不出现折行 | √ |
| 10970 | 「证券行情排行」底部分页条数控件补齐可读文案：去掉 `v-select` 的浮动 `label="每页"`（88–110px 宽度下会被截断成「每…」），改为控件两侧独立文本「每页」与「条」，即底部显示为 `每页 [50] 条`，下拉值直接显示数字。 | `grep -c '>每页<' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 1；`grep -c '>条<' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 1；`grep -c 'label="每页"' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 0；`cd Web && npm run build` 退出码 0；部署后底部页码左侧显示「每页 50 条」，切换条数回到第 1 页 | √ |
| 10971 | `cdeu.finv_cdeu_secu_realtime_stock` 增加整型 `page` 字段（第 26 列，`INTEGER`，可空），含义为该证券最新快照由**市值排行接口分页请求**获取时的页码（从 1 开始），用于后续按页定位并手动刷新对应证券的快照；同时 `page_chk` 约束拒绝 ≤ 0（NULL 合法），历史 13045 行保持 NULL；`POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/StockUpsert` 与其 OpenAPI 外部端点支持表中**全部 26 个字段**（新增 `page`，入参校验 1 ≤ page ≤ 2147483647，越界/非正数返回 400，PG `23514` 亦映射 400）；迁移 `V300120__cdeu_realtime_stock_add_page.sql`；文档同步 4 份（内外各 `.md`+`.yml`）与 `DbSchemaSpec`。 | `grep -c 'ADD COLUMN IF NOT EXISTS page INTEGER' Deploy/Migrations/V300120__cdeu_realtime_stock_add_page.sql` = 1；迁移后 `SELECT ordinal_position FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_cdeu_secu_realtime_stock' AND column_name='page'` = 26；`SELECT count(*) FROM pg_constraint WHERE conname='finv_cdeu_secu_realtime_stock_page_chk'` = 1；POST 带 `{"items":[{"stock_id":2,"page":3}]}` 返回 `code=0` 且库中该行 `page=3`；POST `page:0` 返回 HTTP 400 且提示「page 非法」；`go build ./...` `go vet ./...` 通过 | √ |
| 10972 | 两个证券实时行情快照批量 Upsert 端点中的 `StockUpsert` 统一替换为 `FTStockUpsert`：`POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert`（内部，需登录）与 `POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert`（外部 OpenAPI，同一 handler）；handler、请求体/响应体、业务逻辑、`Stock/Page` 列表端点均不变；同步更新路由注册与期望路由、4 份接口文档（内外各 `.md`+`.yml`）中的路径、`Docs/API/APIs.md`、`Docs/API/README.md`、菜单文档与采集脚本 `SyncRealtimeStock.py` 的默认 URL。旧路径不再注册（返回 404）。 | `grep -c 'Realtime/FTStockUpsert' internal/api/router.go` = 2；`grep -rc 'Realtime/StockUpsert' internal/api/router.go internal/api/router_test.go` 均为 0（不含 `RealtimeStockUpsert` 类型名与 `.md`/`.yml` 文件名）；`curl -X POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert -d '{}'` 返回 400（路由存在）；同一请求打到旧路径 `/Realtime/StockUpsert` 返回 404；`POST /API/V1/.../Realtime/FTStockUpsert` 未登录返回 401；`go build ./...` 通过 | √ |
| 10973 | 证券实时行情快照 Upsert 响应体统计字段去掉过去式后缀：`data.inserted` → `data.insert`、`data.updated` → `data.update`（`total` 不变，含义不变），即响应为 `{"code":0,"data":{"total":1,"insert":0,"update":1}}`；改动点：`RealtimeStockUpsertStat` 的 JSON tag、handler 注释、内外 4 份接口文档（`.md` 示例/字段表 + `.yml` schema/示例/required）、采集脚本 `SyncRealtimeStock.py` 的取值与打印。Go 结构体字段名与 SQL 逻辑不动。 | `grep -c '"insert"' internal/meta/finv/quote/cdeu/realtime_stock.go` = 1；`grep -c '"update"' internal/meta/finv/quote/cdeu/realtime_stock.go` = 1；POST 新端点返回 `{"code":0,"data":{"total":?,..}` 且 `data` 的键为 `total`/`insert`/`update`（无 `inserted`/`updated`）；`grep -rc 'inserted\|updated' Docs/API/Cdeu/RealtimeStockUpsert.yml Docs/OpenAPI/Quote/Cdeu/Realtime/StockUpsert.yml` 均为 0；脚本 `python3 SyncRealtimeStock.py --dry-run --limit 1` 正常 | √ |
| 10974 | 日K 同步接口（`DayKlineSyncResult`）统计字段与需求 10973 统一：`inserted` → `insert`、`updated` → `update`；同步 `DayKlineUpsertStat`（内部 upsert 统计）JSON tag、handler 注释、`Docs/API/Cdeu/DayKlineSync.md` 的响应示例与字段表、前端 `DayKlineSyncView.vue` 的类型定义/提示文案/结果表格。Go 字段名、日志文案（`inserted=`/`updated=`）与 SQL 别名不变。 | `grep -c '"insert"' internal/meta/finv/quote/cdeu/daykline_sync.go` = 1；`grep -c '"update"' internal/meta/finv/quote/cdeu/daykline_sync.go` = 1；`grep -rc 'inserted\|updated' internal/meta/finv/quote/cdeu/store_daykline.go` 仅剩 SQL 别名 `RETURNING (xmax = 0) AS inserted`；`grep -n 'data.inserted\|result.inserted' Web/src/views/Meta/Finv/Quote/Cdeu/DayKlineSyncView.vue` 无输出；`grep -n '"inserted"' Docs/API/Cdeu/DayKlineSync.md` 无输出；`go build ./...` 与 `cd Web && npm run build` 通过；部署后调用日K 同步，响应 `data` 含 `insert`/`update` | √ |
| 10975 | （待办，尚未实施）把余下两个接口的统计字段与需求 10973/10974 统一为 `insert` / `update`：① `POST /API/V1/Meta/Finv/Quote/Cdeu/Min/KLineBatchInsert` 返回 `{"inserted": n}` → `{"insert": n}`；② 历史行情导入上传接口（`internal/api/handler` 内对应实现）响应中的 `inserted` / `updated` → `insert` / `update`；同步更新 `internal/api/handler/kline.go` 的 `gin.H` 键、相关文档（`Docs/API/Quote/Cdeu/Min/KLineBatchInsert.md\|.yml`、`Docs/API/HistoryQuote/ImportsUpload.md\|.yml`）与前端 `Web/src/views/Meta/Finv/Quote/History/HistoryQuoteImportView.vue` 的取值。Go 字段名与日志文案不变。 | （实施后核对）`grep -rn '"inserted"' internal/api/handler/` 无输出；`grep -rn 'data.inserted\|data.updated' Web/src/views/` 无输出；调用 `Min/KLineBatchInsert` 返回键为 `insert`；历史行情导入上传响应键为 `insert`/`update`；`go build ./...`、`cd Web && npm run build` 通过 | |
| 10976 | 新建采集配置表 `cdeu.finv_cdeu_realtime_sync_config`（迁移 `V300121`）：一行=一个「市场 + 证券品种」，9 列 `instrument_symbol / market_type / count_plan / count_exists / total_available / flag_enable / remark / gmt_create / gmt_update`，主键 `(market_type, instrument_symbol)`，`flag_enable` CHECK 只允许 `0`/`1`；初始 3 行（`stock` × marketType 1/2/4，`count_plan` 2500/6000/4500，与旧 `Stat.json` 的 targets 一致）。用途：替代 Python 采集脚本常量与 `Stat.json`，供网页菜单读配置触发采集；采集成功后回写 `count_exists`（接口 `pagination.total`）与 `total_available`；结果数据仍写 `cdeu.finv_cdeu_secu_realtime_stock`。 | `ls Deploy/Migrations/V300121__cdeu_create_realtime_sync_config.sql` 存在；`SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300121；`SELECT count(*) FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_cdeu_realtime_sync_config'` = 9；`SELECT string_agg(column_name, ',' ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_cdeu_realtime_sync_config'` 以 `instrument_symbol,market_type,count_plan,count_exists,total_available` 开头；`SELECT count(*) FROM cdeu.finv_cdeu_realtime_sync_config` = 3；`SELECT count_plan FROM ... WHERE market_type='2'` = 6000 | √ |
| 10977 | 实现行情列表股票快照同步服务与接口：采集逻辑集中在**单个文件** `internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go`（只依赖标准库与 `quoteapiv2`，落库/配置读写通过 `StockListSink` / `StockListConfigStore` 接口注入，便于后续整体迁往 MetaExt 第三方包）；采集范围与采集控制参数**运行时动态传入**（市场/品种过滤、计划数量覆盖、页大小、页码起始、页间间隔、重试次数与退避、请求超时、页数上限、失败续跑、`extra` 扩展位）；接口 `POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/StockListCollect`（需登录），结果写 `cdeu.finv_cdeu_secu_realtime_stock`（`page` 记来源页码），采集后回写配置表 `count_exists` / `total_available`。 | `ls internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go` 存在，且文件内无 `s.pool` / `UpsertRealtimeStocks`（只通过接口调用）；`grep -c 'StockListCollectRequest' internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go` ≥ 1；`grep -c 'Realtime/StockListCollect' internal/api/router.go` = 1；POST 实测（`{"market_type":"1","count_plan":100,"page_size":50,"interval_ms":300,"max_pages":2}`）返回 `code=0`、`rows_received=100`、`count_exists>0`；库中 `market_label='HK' AND page IS NOT NULL` 有数据且 `price_direct` ∈ {`+`,`-`,`0`}；配置表该行 `count_exists`/`total_available` 已更新；`go build ./...` 通过 | √ |
| 10978 | 行情列表股票快照同步端点改名：`POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/StockListCollect` → `POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync`（需求 10977 的路由口径由本条取代；handler、请求体、采集逻辑与结果均不变，旧路径不再注册返回 404）。同步更新 `router.go`、`router_test.go`、handler 注释、`Docs/API/Cdeu/StockListCollect.md`/`.yml`、`Docs/API/APIs.md`（#114）、`Docs/API/README.md`。 | `grep -c 'Realtime/FTStockQuoteSnapshotSync' internal/api/router.go` = 1；`grep -rc 'Realtime/StockListCollect' internal/api/router.go internal/api/router_test.go` 均为 0；POST 新路径返回 200/`code=0`；POST 旧路径返回 404；`go build ./...` 通过 | √ |
| 10979 | 行情列表股票快照同步接口的统计字段 `rows_received` 改为**动词原形** `rows_receive`（与需求 10973/10974 的 `insert`/`update` 口径一致），涉及 `StockListCollectResult` 的 JSON tag、handler 汇总键、接口文档 `.md` 示例与字段表、`.yml` 的 `properties`/`required`。 | `grep -rn 'rows_received' internal/ Docs/API/Cdeu/StockListCollect.md Docs/API/Cdeu/StockListCollect.yml` 无输出（历史需求条目除外）；`grep -c 'rows_receive' internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go` ≥ 1；POST `FTStockQuoteSnapshotSync` 响应含 `rows_receive` 且无 `rows_received`；两份 yml 通过 yaml 解析；`go build ./...` 通过 | √ |
| 10980 | 在开发规范中固化统计字段命名要求：`Docs/DevSpec/ApiSpec.md` 新增 §1.1「统计与动作类字段命名（强制）」——统计字段用动词原形（`insert`/`update`/`receive`/`delete`/`skip`），禁止过去式（`inserted`/`updated`/`received`/`deleted`/`skipped`）；行数类字段统一 `rows_` 前缀；规则覆盖 `.md` 示例与字段表、`.yml` 的 `properties`/`required`、前端 TS 类型；存量不合规字段按需求条目逐条迁移。 | `grep -c '### 1.1 统计与动作类字段命名（强制）' Docs/DevSpec/ApiSpec.md` = 1；`grep -c '禁止：`inserted`' Docs/DevSpec/ApiSpec.md` = 1；`grep -c 'rows_insert' Docs/DevSpec/ApiSpec.md` ≥ 1 | √ |
| 10981 | （待办，尚未实施）按 `Docs/DevSpec/ApiSpec.md` §1.1 清理**存量**统计字段命名（动词原形、禁用过去式）。清单分三组——① 接口响应顶层：`internal/api/handler/kline.go` 的 `{"inserted": n}` / `{"updated": n}` / `{"deleted": n}` → `insert` / `update` / `delete`；`internal/meta/finv/quote/service.go` 的 `Inserted` / `Updated`（历史行情导入结果）→ `insert` / `update`，前端 `Web/src/views/Meta/Finv/Quote/History/HistoryQuoteImportView.vue` 的 `data.inserted ?? 0` / `data.updated ?? 0` 同步；② Job 运行详情（details）：`internal/meta/finv/brs/slice.go` 的 `skipped` → `skip`；`store_patch_rule.go` 的 `deleted_rows` / `maturity_deleted`、`usc_patch_runner.go` 的 `deleted_rows`、`job/purge_history_runs.go` 与 `job/purge_local_export_files.go` 与 `job/purge_stale_dynamic.go` 的 `deleted`、`job/clean_export_storage.go` 的 `staging_deleted`、`job/upload_pending_export_files.go` 的 `manifests_skipped`、`quant/backtest/model.go` 的 `deleted_counts` → 统一改成 `rows_delete` / `rows_skip` 等动词原形（含调用方与文档）；③ 不在范围（时间/操作者字段，非统计）：`updated_at`、`deleted_at`、`deleted_by`。接口文档 `.md`/`.yml` 与前端类型同步。 | （实施后核对）`grep -rn '"inserted"\|"updated"\|"deleted"\|"skipped"' internal/ --include=*.go` 无输出；`grep -rn 'deleted_rows\|maturity_deleted\|staging_deleted\|manifests_skipped\|deleted_counts' internal/ --include=*.go` 无输出；`grep -rn 'data.inserted\|data.updated' Web/src/` 无输出；`grep -rn 'updated_at\|deleted_at\|deleted_by' internal/ --include=*.go` 仍有输出（豁免项）；`go build ./...`、`cd Web && npm run build` 通过 | |
| 10982 | 在「证券行情排行」之后新增前端菜单「行情快照手动同步」（`quote-realtime-snapshot-sync`，路由 `/Meta/Finv/Quote/Realtime/FTStockQuoteSnapshotSync`）：可查看采集配置表、动态设置市场/计划数量/每页条数/页间间隔/重试次数/退避/页数上限，点「开始同步」触发异步采集；界面展示状态、耗时、各市场结果（收到/新增/覆盖/市场总数/本轮可采）以及**每页翻页采集的日志**（`begin`/`page`/`retry`/`done`，INFO/WARN/ERROR）。后端新增 `POST .../Realtime/FTStockQuoteSnapshotSync/Run`（异步启动，返回 `run_id`，同一时间仅一个运行中任务）与 `GET .../Realtime/FTStockQuoteSnapshotSync/Run`（轮询运行状态 + 逐页日志 + 采集配置）；采集单文件新增事件回调（`StockListCollectHook`），运行状态与日志由 `stocklist_run.go` 在内存持有（最近 10 次 × 500 行）。 | `grep -c 'quote-realtime-snapshot-sync' Web/src/config/menuConfig.ts Web/src/router.ts` 各 = 1；`grep -c 'FTStockQuoteSnapshotSync/Run' internal/api/router.go` = 2；`ls Web/src/views/Meta/Finv/Quote/Realtime/StockSnapshotSyncView.vue` 存在；`POST .../Run` 返回 `run_id` 且 `status=RUNNING`；随后 `GET .../Run` 的 `data.run.logs` 含「第 N/M 页」行、结束时 `status` ∈ {`SUCCESS`,`PARTIAL`,`FAILED`}；重复 `POST` 在任务运行中返回 400；`go build ./...`、`cd Web && npm run build` 通过 | √ |
| 10983 | 实测并固化 moomoo `get-stock-list` 的 `page` 起始值：**从 1 开始**，`page=0` 时服务端返回错误「moomoo page 不能小于 1，实际 0」；`page=1` 为首页（港股第一条 HSBC HOLDINGS，`pagination{page:1, pageCount:668, total:3337}`），`page=2` / `page=3` 依次为后续页。采集侧 `StockListDefaultPageStart = 1` 正确；早期 Python 版 `page-1`（0 起）为错误实现，不得沿用。结论写入常量注释、live 测试与 5 份接口/菜单文档。 | `grep -c 'page 不能小于 1' internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go` = 1；`grep -rc 'page_start' internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go` ≥ 1；`FINV_MOOMOO_LIVE=1 go test -run TestLiveStockListPageBase -v ./internal/meta/finv/quote/cdeu/` 输出 page=0 报错、page=1 为首条港股；`grep -rn '实测若为 0 起' Docs/API Docs/Menu` 无输出 | √ |
| 10984 | 采集每页落库后输出**证券明细日志**：每个证券一格 `[标记] 代码 名称`，`[I]`=新增（`stock_id` 此前不在 `cdeu.finv_cdeu_secu_realtime_stock`）、`[U]`=更新；**每行 10 个**（每页 50 条即 5 行），每格固定 22 显示列（CJK 按 2 列计，超长截断、不足补空格）保证列对齐；同时输出到容器控制台（`docker logs`）与异步任务日志流（前端日志面板换行展示）。实现：`StockListSink.SaveStockListPage` 改为返回逐行标记（先按 `stock_id = ANY(...)` 查已存在集合），`quote_list_stock_snapshot_sync.go` 新增 `stockListFormatDetail` / `stockListDisplayWidth` / `stockListTruncateDisplay`。 | `grep -c 'stockListDetailPerLine   = 10' internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go` = 1；`grep -c 'stockListDetailCellWidth = 22' internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go` = 1；触发一次同步后 `docker logs finvquant 2>&1 \| grep -cE '^\[[IU]\]'` ≥ 1；明细每行含 10 个 `[I]`/`[U]` 标记且字符列对齐（对比相邻两行的标记列位置一致）；`GET .../Run` 的 `data.run.logs[].message` 含明细块（含换行）；`go build ./...` 通过 | √ |
| 10985 | `cdeu.finv_cdeu_realtime_sync_config` 更名为 `cdeu.finv_cdeu_secu_realtime_sync_config`（迁移 `V300122`）：只改表名与两个约束名（`..._pk`、`..._enable_chk`），列结构、数据行、注释与索引定义均不变；代码侧 `tableRealtimeSyncConfig` 常量、文档（`DbSchemaSpec`、`StockListCollect.md\|.yml`、`SnapshotSync.md`）、前端视图注释同步。V300121 保留原表名不动（历史迁移，全新部署时由 V300122 承接改名）。 | `SELECT to_regclass('cdeu.finv_cdeu_realtime_sync_config') IS NULL AND to_regclass('cdeu.finv_cdeu_secu_realtime_sync_config') IS NOT NULL` = true；`SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300122；`\d cdeu.finv_cdeu_secu_realtime_sync_config` 主键为 `finv_cdeu_secu_realtime_sync_config_pk`、CHECK 为 `finv_cdeu_secu_realtime_sync_config_enable_chk`、列数 9、`gmt_update` 保留；`grep -rc 'finv_cdeu_realtime_sync_config' internal/ Docs/API Docs/Menu Web/src` 仅在 `V300121`/`V300122`/历史需求与变更日志中出现；`GET .../FTStockQuoteSnapshotSync/Run` 仍能返回 3 行 configs（证明代码已读新表） | √ |
| 10986 | 把「行情快照手动同步」页面内容改为**按证券品种分 Tab**：Tab 列表由 `cdeu.finv_cdeu_secu_realtime_sync_config` 的 `instrument_symbol` 去重生成（当前仅 `stock`），并为 `etf` / `future` / `forex` / `option` / `crypto` 预留显示名与图标；每个 Tab 的配置表、市场下拉、结果表、汇总与日志均按当前品种过滤（日志按 `market` 前缀 `品种/市场` 匹配，任务级日志常显）；「开始同步」请求体带 `instrument_symbol`，只采当前 Tab 品种；切换 Tab 清空市场与计划数量覆盖。扩展新品类只需插一行配置，不改前端。 | `grep -c 'v-tabs' Web/src/views/Meta/Finv/Quote/Realtime/StockSnapshotSyncView.vue` ≥ 1；`grep -c "instrument_symbol: activeSymbol.value" Web/src/views/Meta/Finv/Quote/Realtime/StockSnapshotSyncView.vue` = 1；`grep -c 'crypto.*加密货币\|加密货币' Web/src/views/Meta/Finv/Quote/Realtime/StockSnapshotSyncView.vue` ≥ 1；部署后产物含「股票/ETF/期货/外汇/期权/加密货币」六个品种名；页面显示 1 个「股票」Tab（配置表仅 stock）；`cd Web && npm run build` 通过 | √ |
| 10987 | 「日K线数据同步」菜单补图标并优化页面展示：① 图标 `mdi-chart-candlestick` 在 `@mdi/font@7.4.47` 中不存在（渲染为空白），统一替换为存在的 `mdi-candle`（`menuConfig.ts`、页面标题、`Menus.md` 索引）；② 操作栏改为单行 flex（usc 组合框 + 起止日 + 同步/查询存量/重置），窄屏自动换行；③ 同步过程增加已耗时秒数与不确定进度条提示；④ 同步结果与存量统计改为**指标卡 + 明细表**两栏布局（返回根数/区间内/新增/更新、行数/起止日）；⑤ 请求参数透传、原始响应与日志命令合并进默认收起的「排障信息」折叠面板，不再占用主版面；⑥ 新增「重置」按钮。 | `grep -c 'mdi-candle' Web/src/config/menuConfig.ts Web/src/views/Meta/Finv/Quote/Cdeu/DayKlineSyncView.vue` 各 ≥ 1；`grep -rc 'mdi-chart-candlestick' Web/src Docs/Menu/Menus.md` 均为 0；`grep -c '排障信息' Web/src/views/Meta/Finv/Quote/Cdeu/DayKlineSyncView.vue` = 1；`grep -c '已耗时' Web/src/views/Meta/Finv/Quote/Cdeu/DayKlineSyncView.vue` = 1；`cd Web && npm run build` 通过；部署后菜单与页面标题图标显示为蜡烛图，页面无「docker logs」提示条占用首屏 | √ |
| 10988 | 采集逐页明细日志改为**只展示证券代码、不展示名称**：每格由 `[标记] 代码 名称` 简化为 `[标记] 代码`，格子宽度由 22 显示列收到 11 列（仍每行 10 个）；代码缺失时退回 `stock_id`。同步更新 `Docs/Menu/Quote/Realtime/SnapshotSync.md`、`Docs/API/Cdeu/StockSnapshotSyncRun.md`、`Docs/API/Cdeu/StockListCollect.md` 的示例与规则。 | `grep -c 'stockListDetailCellWidth = 11' internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go` = 1；`grep -c 'row.Name' internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go` = 0（明细不再拼接名称）；触发一次同步后 `docker logs finvquant 2>&1 \| grep -E '^\[[IU]\]' \| head -1` 每行只有 `[I]/[U]` + 代码；`go build ./...` 通过 | √ |
| 10989 | 列表采集支持英文单位口径并锁定语言：① `stockListAmount` 在原有 `万亿/亿/万/B/M` 基础上补充 `T`(1e12)、`K`(1e3)（大小写均可），保证海外 IP 返回的 `4.07T` 这类万亿级市值也能正确换算；② 列表采集的 HTTP 客户端固定携带 `accept-language: zh-CN,zh;q=0.9,en;q=0.8`（通过 `quoteapiv2.Config.HTTPClient` 注入，不改外部包），避免按出口 IP 返回英文口径；③ 每轮首页打印 `首页采样` 日志（代码\|名称\|市值原文），用于现场确认服务端语言与单位。 | `grep -c 'StockListAcceptLanguage' internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go` ≥ 1；`go test -run 'TestStockList.*' ./internal/meta/finv/quote/cdeu/` 通过（含单位换算/采样串/语言头三个用例）；对 `4.07T` 调用 `stockListAmount` 返回 `4070000000000`；采集日志出现 `首页采样` 且样本里能看出返回口径（`4.07万亿` 或 `4.07T`） | √ |
| 10990 | 「行情快照手动同步」页在服务刚重启、内存无运行记录时误报「该品种暂无启用的采集配置」：`GET .../FTStockQuoteSnapshotSync/Run` 的「暂无运行记录」分支原先直接 return，**不返回 `configs`**，前端拿不到启用配置列表；修复为两个分支都查询并返回 `configs`（与 `data.error`），前端在 `data.error` 非空时提示读取配置失败。 | 重启服务后（内存运行记录为空）调用 `GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run`，响应 `data.configs` 为数组且条数等于 `SELECT count(*) FROM cdeu.finv_cdeu_secu_realtime_sync_config WHERE flag_enable = '1'`；页面不再出现「该品种暂无启用的采集配置」，配置表正常渲染 | √ |
| 10992 | 列表采集补齐三处请求口径，修复「采不到 00700 等头部证券、名称为英文」：① **页码改 0-based**（`StockListDefaultPageStart = 0`），并绕开 MetaExt `GetStockList` 硬编码的 page 下限 1（`intParam(req.Page, \"page\", 1)`，报错「moomoo page 不能小于 1，实际 0」属客户端本地校验），改用本文件内 `StockListDirectFetcher` 自行请求、只复用 `quoteapiv2` 的签名与响应结构体；② **referer 按 marketType 指定为中文站市值排行页**（`/hans/quote/{hk,us,cn}/stock-list/.../top-market-cap`），替代 MetaExt 硬编码的站点首页（后者返回英文降级数据，且 `accept-language` 单独设置无效）；③ 请求固定携带 `_` 毫秒时间戳（参与 `quote-token` 签名）。 | 港股 `count_plan=2600` 跑完后 `SELECT count(*) FILTER (WHERE page IS NOT NULL) FROM cdeu.finv_cdeu_secu_realtime_stock WHERE market_type='1'` 等于接口 `pagination.total`（实测 3337）；`SELECT page FROM ... WHERE market_type='1' AND stock_code='00700'` 非空且为 1；首页日志 `[cdeu][stocklist] 首页采样` 样本为中文名中文单位（如 `00700\|腾讯控股\|3.65万亿`） | √ |
| 10993 | 列表采集自动补齐 `usc` 与 `currency`：接口不返回币种、`usc` 也不在响应里，落库前按 ① `stock_id` → `futu.finv_futu_quote_collect.usc`（缺失回落 `stock_code`）② `usc` → `cdeu.finv_quote_cdeu_usc.currency` ③ `security_code` → `public.finv_security.currency_type` ④ 规则兜底（港股 8 开头 5 位代码=人民币柜台 → `CNY`，其余 → `HKD`；美股 → `USD`；沪深 → `CNY`）顺序补值，每页批量查表。 | 对 `page IS NOT NULL AND (usc IS NULL OR currency IS NULL)` 的存量行（实测 1206 行）按上述顺序模拟补齐，`count(new_usc)` 与 `count(new_currency)` 均等于 1206；`go test -run 'TestStockListFallbackCurrency' -run 'TestStockListHKCurrency' ./internal/meta/finv/quote/cdeu/` 通过；采集一轮后 `SELECT count(*) FROM cdeu.finv_cdeu_secu_realtime_stock WHERE page IS NOT NULL AND (usc IS NULL OR currency IS NULL)` 为 0 | √ |
| 10994 | 收紧 10993 的补齐口径：① `usc` **只**取自 `futu.finv_futu_quote_collect`（按 `stock_id`），查不到保持 NULL，**禁止用 `stock_code` 兜底**（同码证券跨市场会映射错，如 SZ/SH 均有 000001）；② 币种规则兜底改为**只看 `market_label`**：`HK` 按 8 开头 5 位代码判人民币柜台（`CNY`）否则 `HKD`、`US` → `USD`、`SH`/`SZ`/`BJ` → `CNY`（兼容历史 `CN`），不再依赖 `marketType`。 | `grep -c 'stock_code' internal/meta/finv/quote/cdeu/store_stocklist.go` 在 `loadStockListSecurityMeta` 内不出现 usc 兜底赋值；`go test -run 'TestStockListFallbackCurrency' ./internal/meta/finv/quote/cdeu/` 通过（含 SH/SZ/BJ 与 HK R 柜台的柜面用例）；14 组用例真实 Go 运行全通过 | √ |
| 10995 | 「实时行情」菜单下新增「待录入证券」页，列出 `cdeu.finv_cdeu_secu_realtime_stock` 中 **`usc` 为空**的快照行（尚未录入证券主数据的证券，作为后续补录 `public.finv_security` + `futu.finv_futu_quote_collect` + `cdeu.finv_quote_cdeu_usc` 的依据）：新增接口 `GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending`（分页 + 关键字 + 市场 + 白名单排序，固定 `usc IS NULL`）、路由 `/Meta/Finv/Quote/Realtime/PendingSecu`、菜单 `quote-realtime-pending-secu`（图标 `mdi-database-plus-outline`）；页面含说明区、筛选、列表与分页，「一键录入」为规划功能（按钮 disabled + 提示，后端无写入接口）。 | `curl -H 'Authorization: Bearer <token>' '.../Realtime/Stock/Pending?page=1&pageSize=3'` 返回 `data.total > 0` 且每行 `usc` 为空；`SELECT count(*) FROM cdeu.finv_cdeu_secu_realtime_stock WHERE usc IS NULL` 与接口 `data.total`（不带筛选时）一致；菜单「行情 → 实时行情 → 待录入证券」可打开且表格有数据；`cd Web && npm run build` 通过 | √ |
| 10996 | 「待录入证券」列表在「快照时间」后新增「操作」列与「录入」按钮；点击弹出录入对话框，按 `public.finv_security` / `futu.finv_futu_quote_collect` / `cdeu.finv_quote_cdeu_usc` 三张表分区展示各自的登记字段（含每表 NOT NULL 且无 DEFAULT 的必填项标记），`usc` 三表共用、在顶部填一次；仅当全部必填都有值时才启用对话框的「录入」按钮，否则禁用并列出缺失项；字段预填只做快照同名字段直传，**不做推断**；三表写入接口与推断逻辑本轮**不实现**（点击仅提示待接入）。 | 列表页存在「操作」列且每行有「录入」按钮；点击弹出对话框，三张表字段均可见、必填项带 `*`；清空任一必填项后「录入」按钮为 disabled 且出现「必填未齐」提示；填齐后按钮启用；`grep -c 'PendingSecuEnrollDialog' Web/src/views/Meta/Finv/Quote/Realtime/PendingSecuView.vue` = 1；`cd Web && npm run build` 通过 | √ |
| 10997 | 依赖上游 MetaExt 升级到 `v0.1.2` 并移除列表采集为绕开上游口径而写的本地临时规避：① `go.mod` 的 `github.com/acnxo/MetaExt` 由 `v0.1.1` 升至 `v0.1.2`——上游该版本已修复 `Docs/Issue/Outside/ISSUE-O-0001` 记录的三处口径（`Config.Header` 自定义请求头、`DefaultReferer` 改为简体中文站并新增 `DefaultReferers` 按 `marketType` 选各市场中文站市值排行页且 `GetStockList` 改走 `getList`、`GetStockList` 的 page 下限由 1 改为 0）；② 删除 `internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go` 中的整套自建请求链路（`StockListDirectFetcher`／`NewStockListDirectFetcher`／`StockListEndpoint`／`StockListUserAgent`／`StockListCSRF`／`stockListReferer` 与 `StockListRefererHK/US/CN`／`StockListAcceptLanguage`／`stockListHTTPClient`／`stockListHeaderTransport`）及只服务于该 transport 的单测 `TestStockListHeaderTransport`；③ 新增 `NewStockListFetcher()` 直接返回 `quoteapiv2.New(quoteapiv2.Config{})`，`NewService` 与 `SetStockListFetcher` 改用它，采集侧不再注入 referer／`accept-language`／自定义 transport；④ 需求 10992 的 ② ③ 两项本地规避由本条取代（10992 原文不改）。| `go list -m github.com/acnxo/MetaExt` 输出 `v0.1.2`；`grep -rn 'StockListDirectFetcher\|stockListReferer\|StockListAcceptLanguage\|stockListHTTPClient\|stockListHeaderTransport\|StockListEndpoint\|StockListUserAgent\|StockListCSRF' internal/` 无输出；`go build ./...`、`go vet ./...` 退出码 0；`go test ./internal/meta/...` 除既有 Windows 平台用例 `TestDirUploader` 外通过；`FINV_MOOMOO_LIVE=1 go test -run TestLiveStockListChineseDefault -v ./internal/meta/finv/quote/cdeu/` PASS，三个市场首条均为中文名且市值为中文单位（实测 2026-11-08：港股 `腾讯控股 3.82万亿`、美股 `苹果 4.98万亿`、沪深 `建设银行 3.12万亿`）；`FINV_MOOMOO_LIVE=1 go test -run TestLiveStockListPageBase -v ./internal/meta/finv/quote/cdeu/` 输出 `page=0` 首条腾讯控股、`page=1` 首条汇丰控股 | √ |
| 10998 | 新建缺省汇率表 `public.finv_cer`（迁移 `V300123`），作为多货币环境把证券市值等金额统一到同一计价货币时的缺省汇率来源：① 列清单固定为 9 列——`currency_src`／`currency_dest`（均 `TEXT`，`CHECK ~ '^[A-Z]{3}$'`）、`cer`（`NUMERIC(24,10)`，`CHECK > 0`，语义「1 单位 `currency_src` = N `currency_dest`」）、`dt_quote`（`BIGINT`，Unix 毫秒，对应上游 `data.<pair>.time`）、`src`、`flag_derive`（`CHAR(1)`，`0`=直采／`1`=派生）、`remark`、`gmt_create`／`gmt_update`（`gmt_update` 由 `public.vq_set_gmt_update()` 触发器维护）；② 主键 `finv_cer_pk (currency_src, currency_dest)`，`CHECK currency_src <> currency_dest`，无物理外键；③ 只保最新、**不建历史表**；表/列注释齐全（9 条列注释），迁移幂等且末尾校验列清单/主键/6 个 CHECK/触发器/列注释数；④ 数据源为 metaext `GetFxOverview`（moomoo `get-fx-overview`）：72 条直采 + 16 条 CNY 派生（`src='derive:CNH'`、`flag_derive='1'`，由 `CNH` 对代理）= 88 行，`price`／`priceNominal` 只落一列 `cer`；⑤ 当前仅支持手动触发更新（定期更新本轮不实现），写入用 upsert 且「旧 `dt_quote` 不覆盖新、派生行不覆盖直采行」；表在采集前为空，不写种子数据。| `ls Deploy/Migrations/V300123__public_create_finv_cer.sql` 存在；`file` 显示 UTF-8 且无 CRLF；`grep -c 'CREATE TABLE IF NOT EXISTS public.finv_cer' Deploy/Migrations/V300123__public_create_finv_cer.sql` = 1；部署后 `SELECT string_agg(column_name, ',' ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema='public' AND table_name='finv_cer'` 输出 `currency_src,currency_dest,cer,dt_quote,src,flag_derive,remark,gmt_create,gmt_update`；`SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300123；`SELECT count(*) FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace WHERE n.nspname='public' AND t.relname='finv_cer' AND c.contype='c'` = 6；`SELECT count(*) FROM pg_description d JOIN pg_class t ON t.oid=d.objoid JOIN pg_namespace n ON n.oid=t.relnamespace WHERE n.nspname='public' AND t.relname='finv_cer' AND d.objsubid>0` = 9；`\d public.finv_cer` 显示主键 `currency_src,currency_dest` 与触发器 `trg_finv_cer_gmt_update`；`grep -c 'finv_cer' Docs/DevSpec/DbSchemaSpec.md` ≥ 1；`test -f Docs/DataDictMapping/FinvCer.md` | √ |
| 10999 | 缺省汇率表 `public.finv_cer` 的手动触发接口与前端菜单：① 后端新增 `POST /API/V1/Meta/Finv/Quote/Cdeu/Fx/CerSync`（需登录，同步执行）——一次 `GetFxOverview` 拉 72 个货币对，校验键（三位大写字母、源≠目标）与价格（优先 `priceNominal`、缺失回落 `price`、须为正数）与 `time`（须 > 0），按 CNH 对派生 16 条 CNY 行（`src='derive:CNH'`、`flag_derive='1'`；同批次已有 CNY 直采行则不派生），单事务全量 upsert，返回 `rows_receive`／`derive`／`insert`／`update`／`dt_quote_max`／`elapsed_ms`；upsert 跳过条件为「新 `dt_quote` 小于库中值」与「派生行遇到同对直采行」，统计以 `RETURNING (xmax = 0)` 为准；② 后端新增 `GET /API/V1/Meta/Finv/Quote/Cdeu/Fx/Cer`（需登录，不分页）返回 `{total, list}`，`cer` 以 `::text` 输出保精度、附 `gmt_update`；③ 前端新增菜单「行情 → 实时行情 → 缺省汇率」（key `quote-realtime-fx-cer`、路由 `/Meta/Finv/Quote/Realtime/FxCer`、图标 `mdi-cash-sync`），页面含「立即更新」/「刷新」、表为空与报价 ≥ 24h 的告警、直采/派生/最新报价/最近更新四个汇总 chip、88 行明细表（直采与派生用 chip 区分、汇率按量级调精度、时间按本地时区）；④ 文档：新增 `Docs/API/Cdeu/FxCer.md` + `.yml`、`Docs/Menu/Quote/Realtime/FxCer.md`，并登记 `Docs/API/APIs.md`（118/119）、`Docs/API/README.md`、`Docs/Menu/Menus.md`（44）。| `grep -c 'Fx/CerSync' internal/api/router.go` = 1；`grep -c 'Fx/Cer"' internal/api/router.go` = 1；`go test ./internal/api/ -run TestExportArchiveRoutesRegistered -count=1` PASS（期望路由含 `POST /API/V1/Meta/Finv/Quote/Cdeu/Fx/CerSync`、`GET /API/V1/Meta/Finv/Quote/Cdeu/Fx/Cer`）；`go test ./internal/meta/finv/quote/cdeu/ -run TestBuildFxCER -count=1` PASS（72 对 → 88 行、CNY 直采优先、非法键/价格/时间拒绝、`price` 回落）；`FINV_MOOMOO_LIVE=1 go test -run TestLiveFxCEROverview -v ./internal/meta/finv/quote/cdeu/` PASS（实测 72 对 → 88 行，`USDCNH = 6.67333`，`AUDCNY` 为派生行）；`grep -c 'quote-realtime-fx-cer' Web/src/config/menuConfig.ts Web/src/router.ts` 各 ≥ 1；`grep -c 'FxCerView' Web/src/router.ts` = 2；`cd Web && npm run build` 通过；部署后菜单「行情 → 实时行情 → 缺省汇率」可打开，`POST .../Fx/CerSync` 返回 `code=0` 且 `rows_receive=88`、`derive=16`，`GET .../Fx/Cer` 的 `data.total=88` 且 `flag_derive='1'` 行有 `remark` | √ |
| 11000 | 缺省汇率页面改为「矩阵/表格」双模式并补齐币种中文名、改路由：① 菜单路由由 `/Meta/Finv/Quote/Realtime/FxCer` 改为 `/Meta/Finv/Quote/Realtime/FXCEROverview`（菜单 key 与路由 name 同步为 `quote-realtime-fxcer-overview`）；② 默认展示**矩阵模式**：行=源币种、列=目标币种、单元格=「1 行首币种 = N 列首币种」，对角线显示 `—`，派生单元格带 `*` 且底色略深，悬浮显示币种全名/汇率/报价时间/来源，矩阵下方给出读法说明；行列币种顺序固定为 `CNY`、`CNH`、`USD`、`HKD`、`EUR`、`GBP`、`AUD`、`SGD`、`CAD`、`JPY`，未登记币种按字母序追加；③ 表格模式新增「币种名称」列并置于第一列，形如 `美元/离岸人民币`（报价时间/更新时间仍固定为最后两列）；④ 币种中文名由后端 `cdeu.FxCERCurrencyNames` 经 `GET .../Fx/Cer` 的 `data.currency_names` 下发（**不取** `public.finv_currency.currency_name`，该字典缺 `CNH`/`EUR`/`GBP`/`AUD`/`CAD` 且不区分在岸/离岸），`CNY`=在岸人民币、`CNH`=离岸人民币，未登记币种回落为代码；⑤ 同步更新 `Docs/Menu/Quote/Realtime/FxCer.md`（双模式与字段说明）、`Docs/Menu/Menus.md`（路由/key）、`Docs/API/Cdeu/FxCer.md`、`Docs/API/Cdeu/FxCer.yml`（`currency_names`）。| `grep -c 'FXCEROverview' Web/src/router.ts Web/src/config/menuConfig.ts` 各 = 1；`grep -c "quote-realtime-fx-cer\b" Web/src Docs/Menu` 无输出；`cd Web && npm run build` 通过；`grep -c 'currency_names' Docs/API/Cdeu/FxCer.yml` ≥ 1 且 YAML 可解析；`go build ./...` 通过；部署后 `GET .../Fx/Cer` 的 `data.currency_names.CNH` = `离岸人民币`、`.CNY` = `在岸人民币`；页面默认进入矩阵模式且矩阵行列含 `CNY`/`CNH`/`USD`…`JPY`，单元格悬浮显示报价时间；切到表格模式后第一列为「币种名称」且最后一列为「更新时间」 | √ |
| 11001 | 缺省汇率「立即更新」按当前展示模式分流到两个上游接口，两者互补写入同一张表：① 矩阵模式 → `POST /API/V1/Meta/Finv/Quote/Cdeu/Fx/CerSync?source=overview`，调 `GetFxOverview`（9 币种两两矩阵 72 对 + 16 条 CNY 派生）；② 表格模式 → 同一接口带 `source=rateList`，调 `GetFxRateList`（`listId=1`、`page=0`、`pageSize=500`，**不传 `_` 时间戳**），条目币种更多，用于补齐概览缺失的货币对；上游对大 `pageSize`（300/500）返回 `code=500`（`spreadTableCode`）且 `data` 为数组时，自动降级为 `pageSize=50` 逐页拉取（最多 20 页、页间 300ms、按货币对去重、中途失败用已取部分），响应 `data.pages` 记录实际请求页数；列表中的非货币对条目（`USDindex`／`EURindex` 等）与缺价条目逐条跳过并计入 `data.skipped`，仅当全部条目不可用时才报错；③ 两来源均单事务全量 upsert 到 `public.finv_cer` 且互不删除：同一来源内按「新 `dt_quote` 覆盖旧」，跨来源以最后一次采集为准（两个来源的时钟不可比），因此可先跑矩阵再跑表格补齐、也可反过来；④ `rateList` 上游条目不返回报价时间，写入行 `dt_quote` 取本次采集时刻；价格取 `priceNominal`，代码须为 6 位大写字母、源≠目标、价格须为正；⑤ 响应新增 `data.source`／`data.pages`／`data.skipped`；`source` 取值非 `overview`／`rateList` 返回 400；前端 toast 与按钮悬浮提示按模式显示对应上游接口。| `grep -c 'FxCERSourceRateList' internal/meta/finv/quote/cdeu/*.go` ≥ 2；`grep -c 'source=' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` ≥ 2；`go test ./internal/meta/finv/quote/cdeu/ -run 'TestBuildFxCER\|TestFxRateList' -count=1` PASS（含非货币对跳过、分页终止与去重）；`go build ./...`、`cd Web && npm run build` 通过；部署后 `POST .../Fx/CerSync?source=overview` 返回 `data.source=overview`、`rows_receive=88`；`POST .../Fx/CerSync?source=rateList` 返回 `data.source=rateList`、`data.pages` ≥ 1、`rows_receive` ≥ 282、`data.skipped` ≥ 2（`USDindex`/`EURindex`）；`POST .../Fx/CerSync?source=bogus` 返回 400；库内 `SELECT count(DISTINCT currency_src) FROM public.finv_cer` 在两次更新后大于仅跑 overview 时的 10，证明两来源互补 | √ |
| 11002 | 在 `public.finv_cer` 基础上新建 `public.finv_fxcer`（迁移 `V300124`）作为缺省汇率表 v2，并改造代码写入/读取新表：① 新增 `usc`（主键，`CHECK ^[A-Z]{6}$`：`get-fx-rate-list` 的 `stockCode`／`get-fx-overview` 的 map key，派生行取「源+目标」如 `CNYUSD`）与 `name`（`NOT NULL DEFAULT ''`，简体中文展示名：列表来源原样用上游 `name`，概览与派生行按币种中文名合成如「美元/离岸人民币」）；② `dt_quote` 改名 `ts_quote`、`cer` 改名 `rate`、`src` 改名 `source`；其余列（`currency_src`／`currency_dest`／`flag_derive`／`remark`／`gmt_create`／`gmt_update`）与约束语义不变；③ 主键由 `(currency_src, currency_dest)` 改为 `usc`，upsert 冲突键同步改为 `(usc)`，保留「同一来源内 `ts_quote` 只前进、跨来源以最后一次采集为准、派生不覆盖直采」；④ `GET .../Fx/Cer` 行字段改为 `usc`／`name`／`rate`／`ts_quote`／`source`，`ts_quote_max` 取代 `dt_quote_max`，`currency_names` 由行内 `name`（按「/」拆分）反推补齐长尾币种；⑤ 前端表格新增「代码（`usc`）」列、名称列改用行内 `name`，字段引用同步改名；⑥ `public.finv_cer`（v1）**保留在库、不再写入**，不删不改（删表需另行授权），文档标注其已被 v2 取代。| `ls Deploy/Migrations/V300124__public_create_finv_fxcer.sql` 存在且 UTF-8/LF；`grep -c 'public.finv_fxcer' internal/meta/finv/quote/cdeu/store_fx_cer.go` ≥ 2；`grep -c 'public.finv_cer\b' internal/meta/finv/quote/cdeu/*.go` 仅出现在注释/常量说明中；`go build ./...`、`go vet ./...` 通过；`go test ./internal/meta/finv/quote/cdeu/ -run TestBuildFxCER -count=1` PASS；`cd Web && npm run build` 通过；部署后 `SELECT string_agg(column_name, ',' ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema='public' AND table_name='finv_fxcer'` 输出 `usc,name,currency_src,currency_dest,rate,ts_quote,source,flag_derive,remark,gmt_create,gmt_update`；`SELECT version FROM public.schema_version ORDER BY version DESC LIMIT 1` = 300124；`POST .../Fx/CerSync?source=rateList` 与 `?source=overview` 均返回 `code=0`，随后 `GET .../Fx/Cer` 每行含非空 `usc`／`name`／`rate`，`total` 与 `SELECT count(*) FROM public.finv_fxcer` 一致；`SELECT count(*) FROM public.finv_cer` 保持不变（v1 不再写入） | √ |
| 11003 | 币种口径与矩阵顺序调整：① 中文名 `CNY` 由「在岸人民币」规范为「人民币」（`CNH` 仍为「离岸人民币」，两者靠币种代码区分、不混用）；② 矩阵模式行列币种顺序调整为 `USD`、`CNY`、`CNH`、`HKD`、`EUR`、`JPY`、`SGD`、`AUD`、`CAD`、`NZD`、`PHP`、`CHF`、`IDR`、`ZAR`、`RUB`、`THB`、`TWD`、`XAU`、`XAG`（需求文字中的「TWB」按数据实际存在的 `TWD` 处理），未列入的币种按字母序追加到末尾、保持原有相对顺序；③ `XAU`/`XAG`（黄金/白银）虽非货币但来自 `listId=1`，**保留不过滤**。| `grep -c 'FxCurrencyCNY: "人民币"' internal/meta/finv/quote/cdeu/fx_cer_sync.go` = 1；`grep -c '在岸人民币' internal/meta/finv/quote/cdeu/*.go Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 0；`grep -c 'NZD.*PHP.*CHF.*IDR.*ZAR.*RUB.*THB.*TWD.*XAU.*XAG' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` ≥ 1；`go build ./...`、`cd Web && npm run build` 通过；部署后 `GET .../Fx/Cer` 的 `data.currency_names.CNY` = `人民币`、`.CNH` = `离岸人民币`；矩阵模式首行/首列为 `USD`，其后依次为 `CNY`、`CNH`、`HKD`…`XAG`；表格模式中 `XAU`/`XAG` 行仍在且带中文名「黄金」「白银」 | √ |
| 11004 | 微调矩阵模式币种顺序：`GBP` 移到 `AUD` 之前、`CHF` 移到 `PHP` 之前，完整顺序为 `USD`、`CNY`、`CNH`、`HKD`、`EUR`、`JPY`、`SGD`、`GBP`、`AUD`、`CAD`、`NZD`、`CHF`、`PHP`、`IDR`、`ZAR`、`RUB`、`THB`、`TWD`、`XAU`、`XAG`，未列入的币种仍按字母序追加。 | `grep -c "SGD., .GBP., .AUD" Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` ≥ 1；`grep -c "NZD., .CHF., .PHP" Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` ≥ 1；`cd Web && npm run build` 通过；产物顺序字面量含 `SGD`,`GBP`,`AUD` 与 `NZD`,`CHF`,`PHP` | √ |
| 11005 | 缺省汇率页面把三段说明文本统一移到表格下方的独立卡片中：①「表口径」（`public.finv_fxcer`／`usc`／`name`／`rate`／只保最新）从标题下方副标题移到说明卡片；②「数据源」（`get-fx-overview` 72 对、CNH 派生 CNY、两模式各自来源、互不删除可互补）从表格上方 info 提示移到说明卡片；③「矩阵读法」（单元格读法示例、`*` 派生、`—` 无该货币对、悬浮看时间与来源）从矩阵表格下方移到说明卡片，且仍仅在矩阵模式显示；三段文本样式保持不变（副标题 / info 提示 / 小字说明），标题下方与表格前后不再出现这些说明。 | `grep -c '说明卡片' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 1；`grep -c 'read method\|读法' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 1；`cd Web && npm run build` 通过；部署后页面顶部只保留标题与按钮，表格下方出现独立说明卡片且含三段文本（矩阵模式下含「读法」段） | √ |
| 11006 | 优化缺省汇率「说明」卡片样式并加卡片标题：① 卡片加 `v-card-title`（图标 `mdi-information-outline` + 文案「说明」）；② 三段文本改为「分节标题 + 正文」结构——「表口径」「数据源与互补」「矩阵读法」，分节标题为 `0.875rem/500/primary`，正文 `text-body-2`（不透明度 0.85、行高 1.6），左侧 3px primary 色条；③ 段间用 `v-divider` 分隔；「数据源与互补」拆成两条正文（数据源与基差提醒 / 两模式互补）；④ 文本内容不变，「矩阵读法」仍仅在矩阵模式显示。 | `grep -c 'finv-fxcer-note-title' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` ≥ 4（CSS + 三处使用）；`grep -c 'mdi-information-outline' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 1；`grep -c '>说明<' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 1；`cd Web && npm run build` 通过；部署后表格下方说明卡片显示标题「说明」与三个分节标题，段间有分隔线 | √ |
| 11007 | 新增前端开发规范 `Docs/DevSpec/ExplainCardSpec.md`（说明卡片规范），并把「静态文字说明统一收进页面最底部唯一一张『说明』卡片」定为本项目默认风格：① 明确什么算静态说明（表口径、数据源、读法/图例、业务约束、与其它页面的关系）要进说明卡片；② 明确运行时状态（请求失败、空数据、过期/降级、一次性操作反馈、指标汇总）仍留在主体内容区，不搬进说明卡片；③ 规定位置（页面最底部、最后一张主卡片之后）、数量（每页唯一）、标题（`mdi-information-outline` + 「说明」）与标准结构片段；④ 统一分节样式（分节标题 0.875rem/500/primary、左侧 3px primary 色条、正文 `text-body-2` 0.85 不透明度、段间 `v-divider`）与分节切分建议；⑤ 列出反例（标题下副标题、表格前后 info 提示、多处重复说明）与存量页面迁移原则（改造时顺手迁移、文本不变）；⑥ 给出核对命令与人工判定点，并指向参考实现 `FxCerView.vue`（需求 11005/11006）。| `test -f Docs/DevSpec/ExplainCardSpec.md`；`grep -c '^## ' Docs/DevSpec/ExplainCardSpec.md` ≥ 8；`grep -c 'mdi-information-outline' Docs/DevSpec/ExplainCardSpec.md` ≥ 1；`grep -c 'finv-fxcer-note' Docs/DevSpec/ExplainCardSpec.md` ≥ 1；`grep -c '反例' Docs/DevSpec/ExplainCardSpec.md` ≥ 1；`file` 显示 UTF-8 且无 CRLF | √ |
| 11008 | 新增货币字典文档 `Meta/Dict/Currency.md`：① 覆盖缺省汇率页（`/Meta/Finv/Quote/Realtime/FXCEROverview`）表格模式「源币种」「目标币种」两列出现的全部代码，键为三位大写字母、值为规范中文名称，表格固定「键 / 值 / 备注」三列；② 行集合以 `public.finv_fxcer` 实际出现的代码为准（实测 36 个：`USD`…`TRY`），不臆造未落库代码；③ 备注标注特殊口径：`CNY` 为派生（由 `CNH` 同价代理）、`CNH` 与 `CNY` 存在基差不可混用、`XAU`/`XAG` 为贵金属现货而非货币（按需求保留）、`HKD`/`SGD` 等口语简称；④ 排序先按缺省汇率矩阵展示顺序、其余按字母序；⑤ 写明维护约定（以库内代码为准、名称以规范中文名为准、口语简称写备注、派生行不引入新代码）。 | `test -f Meta/Dict/Currency.md`；`grep -c '^\| \`' Meta/Dict/Currency.md` = 36；`grep -c '\| 键 \| 值 \| 备注 \|' Meta/Dict/Currency.md` ≥ 1；代码集合与 `SELECT currency_src FROM public.finv_fxcer UNION SELECT currency_dest` 去重集合完全一致（实测 36 个、零缺零多）；`file` 显示 UTF-8 且无 CRLF | √ |
| 11009 | 参照 `Meta/Dict/Currency.md` 新增区域字典文档 `Meta/Dict/FinvRegion.md`：① 键为 `public.finv_region.region` 区域简写、值为 `name` 规范中文名，表格固定「键 / 值 / 备注」三列；② 覆盖库内全部 27 条记录（`UNK`、`GLOBAL`、`FX`、`CC` 与 `101`~`123` 国家/地区段），按 `idx` 升序；③ 备注写明 `idx` 值、富途自选股 region 原值，以及 `FX`/`CC` 由 `V300077`/`V300078` 引入、`USA`→`US` 统一等口径变更；④ 记录 `idx` 编码规则（0 未知、1 全球、2~3 非地域类别、101+ 区域=原值+99）与维护约定（走迁移、改简写需跨表联动、业务判断以 `region` 为准、简称只写备注）。| `test -f Meta/Dict/FinvRegion.md`；`grep -c '^\| \`' Meta/Dict/FinvRegion.md` = 27；`grep -c '\| 键 \| 值 \| 备注 \|' Meta/Dict/FinvRegion.md` ≥ 1；键集合与 `SELECT region FROM public.finv_region` 完全一致、值与该表 `name` 逐条相同（实测 27/27，零差异）；`file` 显示 UTF-8 且无 CRLF | √ |
| 11010 | 同步 `Docs/DataDictMapping/FinvRegion.md` 到库内实际 27 条：① 清单标题「25 条」→「27 条」，补 `FX`（idx=2，`V300077`）与 `CC`（idx=3，`V300077` 以 `CRYPTO` 新增、`V300078` 改简写）两行；② 103 行 `USA`→`US`（`V300077` 统一），`对应地区`列注明变更来源；③ 表结构 `region` 示例与 §3 关联参考中的 `USA` 改为 `US`，并注明 `USA` 为历史值；④ §3 补「FX / CC」「USA → US」两条说明，idx 段位改为「0~1 保留、2~3 非地域类别、101~123 正式区域、4~100 预留」；⑤ 头部补 `V300077` / `V300078` 迁移链接与取值字典 `Meta/Dict/FinvRegion.md` 交叉引用。| 清单 `idx` 集合与 `SELECT idx FROM public.finv_region` 完全一致（27/27）；每行 `region`/`name` 与该表取值逐条相同（零差异）；`grep -c '25 条' Docs/DataDictMapping/FinvRegion.md` = 0；`grep -c 'FX' Docs/DataDictMapping/FinvRegion.md` ≥ 2；文件 UTF-8 且无 CRLF | √ |
| 11011 | `Meta/Dict/FinvRegion.md` 字典表在「键」「值」之后新增独立 `IDX` 列：① 表头改为「键 / 值 / IDX / 备注」，`IDX` 为右对齐数值列；② 原先写在备注里的 `idx=N` 前缀移到 `IDX` 列，备注只保留口径说明；③ §1 说明补 `IDX` 列语义（对应表字段 `idx`、地区编号、仅用于排序与兼容富途历史原值、业务判断以 `region` 为准），排序改为「按 `IDX` 升序」。| `grep -c '^\| 键 \| 值 \| IDX \| 备注 \|$' Meta/Dict/FinvRegion.md` = 1；`grep -c '^\| \`' Meta/Dict/FinvRegion.md` = 27 且每行 4 列；`grep -c '\`idx=' Meta/Dict/FinvRegion.md` = 0（备注不再含 idx 前缀）；`IDX`、`键`、`值` 三列与该表 `SELECT idx, region, name FROM public.finv_region` 逐条一致（实测 27/27，零差异） | √ |
| 11012 | 缺省汇率矩阵模式的角单元格由纯文本「源 \\ 目标」改为**对角线拆分**：① 用一条从单元格左上角连到右下角的对角线（内联 SVG `line` + `vector-effect="non-scaling-stroke"`，1px、`currentColor`、透明度 0.5）把单元格拆成两个三角区；② 右上三角显示「目标」（对应列方向目标币种），左下三角显示「源」（对应行方向源币种），字号 0.7rem、透明度 0.85；③ 角单元格尺寸固定 66×40，垂直居中；悬浮提示「行 = 源币种，列 = 目标币种」；④ 表头不再出现「源 \\ 目标」纯文本。| `grep -c 'finv-fxcer-corner-dest' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` ≥ 2（样式 + 模板）；`grep -c '源 \\\\ 目标' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 0；`grep -c 'vector-effect="non-scaling-stroke"' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 1；`cd Web && npm run build` 通过；部署后矩阵表头左上角出现对角线与「目标」（右上）/「源」（左下）两处文字 | √ |
| 11013 | 缺省汇率矩阵单元格改为卡片式并补全分割线与悬浮反馈：① 表格 `border-collapse: separate` + `border-spacing: 3px`，每个 `th`/`td` 带 `1px` 边框与 `6px` 圆角，同时具备行分割线与**列分割线**（原样式仅有行线）；② 表头行与行首列底色加深到 `surface-variant` 70%，单元格底色 30%，形成卡片标题栏/内容区层次；③ 悬浮单元格：背景 `primary` 16%、描边 `primary` 55%、`0 2px 10px` 抬升阴影、文字转 `primary` 且加粗，`z-index: 1` 浮于相邻卡片之上；④ 悬浮时同行行首 `th` 同步浅高亮（`primary` 10%），便于定位行；⑤ 进出均有 `0.15s` 过渡（背景/描边/阴影/文字色四项），离开不瞬切；派生单元格底色提高到 secondary 16% 以保证可辨；⑥ 选择器加 `.v-table__wrapper > table` 前缀，压过 Vuetify 自带单元格 padding/border 规则。| `grep -c 'border-spacing: 3px' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 1；`grep -c 'v-table__wrapper > table tbody td:hover' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 1；`grep -c 'border-radius: 6px' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` ≥ 1；`cd Web && npm run build` 通过；部署后产物 CSS 含 `border-spacing:3px`、`tbody td:hover{background-color:rgba(var(--v-theme-primary), .16)`、`box-shadow:0 2px 10px`、`transition:background-color .15s,border-color .15s,box-shadow .15s,color .15s`；页面矩阵行列方向均可见分割线，悬浮/离开有明显颜色与明暗变化 | √ |
| 11014 | 缺省汇率矩阵单元格改回**直角矩形 + 实线分割**，并修复悬浮提示换行失效：① 表格 `border-collapse: collapse`、`border-spacing: 0`，取消 3px 间隙；单元格去掉 `border-radius: 6px`，改为直角；相邻单元格共用 1px 实线边框（`rgba(on-surface, 0.18)`），行列两个方向都是实线，不再有间隙；② 悬浮反馈保留不变（`primary` 16% 底、55% 描边、`0 2px 10px` 阴影、文字转主色加粗、同行行首联动、0.15s 过渡）；③ 悬浮提示改为**逐行渲染**：`cellTooltip` → `cellTooltipLines` 返回 `string[]`，模板用 `v-tooltip` 默认插槽 + `v-for` 每行一个 `div`，外层 `.finv-fxcer-tip` 设 `line-height: 1.5` 与 `tabular-nums`；无该货币对时拆成两行（结论 / 原因）；④ 原 `v-tooltip :text` 的单行字符串（含 `\n`）废弃，避免换行被折叠成空格。| `grep -c 'border-collapse: collapse' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 1；`grep -c 'border-radius: 6px' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 0；`grep -c 'cellTooltipLines' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` ≥ 2；`grep -c ':text="cellTooltip' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 0；`cd Web && npm run build` 通过；部署后产物 CSS 含 `border-collapse:collapse`、矩阵单元格无 `border-radius`、含 `.finv-fxcer-tip`，产物 JS 无 `\\n` 单行提示残留；页面悬浮提示显示为 4 行（无数据时为 2 行） | √ |
| 11015 | 缺省汇率矩阵悬浮时**列首同步高亮**：① 单元格 `td` 绑定 `@mouseenter="setHover(src, dest)"` 与 `@mouseleave="clearHover()"`，用 `hoverSrc` / `hoverDest` 记录当前悬浮单元格的行（源）与列（目标）币种；② 列首 `th` 按 `hoverDest === dest` 加 `.finv-fxcer-axis-hover`，行首 `th` 按 `hoverSrc === src` 加同一类，两者样式一致（底色 `primary` 10%、描边 `primary` 35%、文字转 `primary`、字重 600），替代原先只能靠 `tbody tr:hover th` 实现的「仅行首」效果；③ 鼠标离开单元格、离开矩阵滚动容器（`.finv-fxcer-scroll` 上 `@mouseleave`）时清除高亮，避免残影；④ 鼠标直接落在行首/列首 `th` 上时也分别高亮对应维度（`setHover(src,'')` / `setHover('',dest)`）。| `grep -c 'finv-fxcer-axis-hover' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 4（两处 class + 样式 + 注释）；`grep -c '@mouseenter="setHover' …/FxCerView.vue` = 3；`grep -c 'tbody tr:hover th' …/FxCerView.vue` = 0；`cd Web && npm run build` 通过；部署后产物 JS 含 `finv-fxcer-axis-hover`、CSS 含 `th.finv-fxcer-axis-hover` 规则；页面悬浮任意单元格时其行首与列首同时变色（同一样式） | √ |
| 11016 | `public.finv_fxcer` 增加涨跌额/涨跌幅两列，并在表格模式采集时落库：① 迁移 `V300125__public_finv_fxcer_add_change_columns.sql` 追加 `change_price NUMERIC(24,10)`（涨跌额，带符号）与 `change_ratio NUMERIC(12,6)`（涨跌幅百分数数值），均可空、幂等（`ADD COLUMN IF NOT EXISTS`）、带列注释并校验列清单/类型/CHECK 数/注释数；② Go `FxCERRow` 增 `ChangePrice`/`ChangeRatio`，`BuildFxCERRowsFromRateList` 解析上游 `changeAmount`/`changePercent`（去千分位、去空白、去 `%`，非法值归一化为空 → 落库 NULL，不丢弃整条汇率），派生 CNY 行与源行汇率相同故继承这两列；③ `UpsertFxCER` 增两列写入，冲突更新用 `COALESCE(EXCLUDED.change_*, 现有值)`，使不提供涨跌字段的 `overview` 采集不清空已有值；`LoadFxCER` 用 `COALESCE(change_*::text,'')` 返回；④ 前端表格模式在「缺省汇率」后新增「涨跌额」「涨跌幅」两列（右对齐），涨跌额按量级调精度、涨跌幅补 `%` 保留两位，配色红涨绿跌（`error`/`success`）、0 与空值不着色，空值显示 `—`。| `psql -c '\d public.finv_fxcer'` 含 `change_price numeric(24,10)` 与 `change_ratio numeric(12,6)`；`SELECT max(version) FROM public.schema_version` = 300125；`go build ./...` 通过；`go test ./internal/meta/finv/quote/cdeu/ -run 'TestBuildFxCERRows\|TestNormalizeChangeValue'` 全 PASS（含新增 `TestBuildFxCERRowsFromRateListChangeFields`：`-0.018%`→`-0.018`、派生行继承、非法值置空、`1,234.5`→`1234.5`）；SQL 模拟一次「带涨跌值的 upsert」与一次「空涨跌值的 upsert」，前者写入新值、后者保留旧值；`cd Web && npm run build` 通过；页面表格模式可见两列且涨跌配色正确 | √ |
| 11017 | 矩阵模式按涨跌幅给汇率数值着色（红涨绿跌）：① 矩阵单元格的汇率 `span` 增加 `:class="changeClass(cell(src, dest)?.change_ratio)"`，`change_ratio > 0` → `.finv-fxcer-up`、`< 0` → `.finv-fxcer-down`、`= 0` 或缺失 → 不着色；② 色值改用「行情 → 实时行情 → 证券待录入」（`PendingSecuView.vue` 的 `.row-rise` / `.row-fall`）同一套：涨 `#ff0000`、跌 `#00e676`，替换原先的 `--v-theme-error` / `--v-theme-success`，表格模式的涨跌两列同步改为这套色值；③ 说明卡片「矩阵读法」补一句着色规则（含涨/跌两色示例与「无涨跌数据按默认色」）；④ 矩阵模式自身采集（`get-fx-overview`）不带涨跌字段，只有经过表格模式（`get-fx-rate-list`）采集过的行才着色。| `grep -c 'changeClass(cell(src, dest)?.change_ratio)' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 1；`grep -c '#ff0000' …/FxCerView.vue` = 1 且 `grep -c '#00e676' …/FxCerView.vue` = 1；`grep -c 'var(--v-theme-error)' …/FxCerView.vue` = 0；`cd Web && npm run build` 通过；部署后产物 CSS 含 `.finv-fxcer-up{color:red}` / `.finv-fxcer-down{color:#00e676}`，产物 JS 矩阵单元格类绑定调用 `changeClass`；表格模式采集一次后（`SELECT count(*) FROM public.finv_fxcer WHERE change_ratio IS NOT NULL` = 280，其中涨 143 / 跌 131 / 平 6）矩阵数值呈红/绿/默认三态 | √ |
| 11018 | 矩阵单元格悬浮提示增加涨跌额/涨跌幅：① `cellTooltipLines` 返回结构由 `string[]` 改为 `FxCERTipLine[]`（`{ text, tone?: 'up' \| 'down' }`），在「1 源 = N 目标」与「报价时间」之间插入一行 `涨跌额 X · 涨跌幅 Y%`；② 该行按 `change_ratio` 正负带 `tone`，模板据此加 `.finv-fxcer-up` / `.finv-fxcer-down`（与矩阵数值、表格涨跌列同一套红涨绿跌）；③ 无涨跌数据（矩阵模式采集的行）时显示「涨跌额 — · 涨跌幅 —（表格模式采集后才有）」，不着色，避免被误读成 0；④ 其余提示行（币种对、汇率、报价时间、类型·来源）与「无该货币对」「自身货币对」两种分支保持原样；⑤ 说明卡片「矩阵读法」同步补一句提示内容。| `grep -c 'FxCERTipLine' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` ≥ 3；`grep -c '涨跌额 \${changePriceText' …/FxCerView.vue` = 1；`grep -c 'line.tone' …/FxCerView.vue` = 1；`cd Web && npm run build` 通过；部署后产物 JS 含 `涨跌额`/`涨跌幅`/`表格模式采集后才有` 与 `tone` 三态判断；页面悬浮有涨跌数据的单元格时提示新增一行且按涨跌着色，悬浮无涨跌数据的单元格显示「—」提示 | √ |
| 11019 | 矩阵悬浮提示文本统一使用提示框默认文字色（撤销需求 11018 中擅自加入的涨跌着色）：① `cellTooltipLines` 返回值恢复为 `string[]`，删除 `FxCERTipLine` 的 `text` / `tone` 结构与 `tone` 三态判断；② 模板恢复为 `<div v-for="(line, i) in cellTooltipLines(src, dest)">{{ line }}</div>`，不再绑定 `.finv-fxcer-up` / `.finv-fxcer-down`；③ 提示内容仍保留 5 行（币种对 / 汇率 / 涨跌额 · 涨跌幅 / 报价时间 / 类型·来源），无涨跌数据时的「表格模式采集后才有」提示保持不变；④ 矩阵数值与表格涨跌列的涨跌配色（需求 11017）不受影响，仍按红涨绿跌显示。| `grep -c 'FxCERTipLine' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 0；`grep -c 'line.tone' Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue` = 0；`grep -c 'finv-fxcer-tip' …/FxCerView.vue` = 2（样式 + 模板）；`cd Web && npm run build` 通过；部署后产物 JS 中提示返回纯字符串数组、无 `tone` 字段；页面悬浮时提示框内 5 行文字颜色一致 | √ |
| 11020 | **取消按 `CNH` 同价派生 `CNY` 汇率**（避免派生值污染换算与回测口径）：① 后端删除 `appendDerivedCNY` 与 `FxCERSrcDeriveCNH` 常量，`BuildFxCERRows`（overview）与 `BuildFxCERRowsFromRateList`（rateList）只返回上游直采行，统一用新增的 `sortFxCERRows` 排序；② `FxCERRow.FlagDerive`、`FxCERSyncResult.Derive` 字段保留（接口向后兼容），采集只会写 `flag_derive='0'`、`derive` 恒为 0；③ 单测改为固化新口径：overview 由「72 直采 + 16 派生 = 88 行」改为 72 行且不含任何 `CNY` 行，rateList 用例去掉派生期望（3 行/2 行/4 行），`TestBuildFxCERRowsDirectCNYWins` 改名 `...DirectCNYPassthrough`（直采 CNY 原样保留、不派生）；④ 前端去掉「按 CNH 派生」表述：说明卡片数据源段改为「`CNY` 由 `get-fx-rate-list` 直采、不再派生」，矩阵读法与表格「类型」列把 `*`/「派生」标为历史标记，采集 toast 与统计 chip 标注「（已停用）」；⑤ 同步文档：`Docs/DataDictMapping/FinvFxcer.md`、`Docs/API/Cdeu/FxCer.{md,yml}`、`Docs/API/APIs.md`、`Docs/Menu/Quote/Realtime/FxCer.md`。| `grep -c 'appendDerivedCNY\|FxCERSrcDeriveCNH' internal/meta/finv/quote/cdeu/*.go` = 0；`go build ./...`、`go vet ./internal/meta/finv/quote/cdeu/` 通过；`go test ./internal/meta/finv/quote/cdeu/ -run TestBuildFxCERRows` 全 PASS；实测 overview 采集返回 `rows_receive=72, derive=0`，rateList 返回 `rows_receive=280, derive=0`；`SELECT count(*) FROM public.finv_fxcer WHERE flag_derive='1' OR source='derive:CNH'` = 0；`cd Web && npm run build` 通过，产物含「不再按 CNH 同价派生」「已停用」文案 | √ |
| 11021 | 从项目中清理 `finv_cer`（缺省汇率表 v1）的建表脚本、表结构与相关描述：① 删除迁移脚本 `Deploy/Migrations/V300123__public_create_finv_cer.sql`；② 删除数据字典文档 `Docs/DataDictMapping/FinvCer.md`，并从 `Docs/DataDictMapping/FinvFxcer.md` 移除「前身」引用与「§2 与 v1 的差异」整节（后续章节号前移）；③ 代码注释去掉 v1 相关说明（`internal/meta/finv/quote/cdeu/store_fx_cer.go`），`V300124` 迁移头注释改为「唯一来源」表述；④ `Docs/Menu/Quote/Realtime/FxCer.md`、`Docs/API/Cdeu/FxCer.md` 中指向 `FinvCer.md` 的链接改指 `FinvFxcer.md`；⑤ `Docs/DevSpec/DbSchemaSpec.md` 的 `public` 域表清单移除 `finv_cer` 条目（同时纠正 `finv_fxcer` 描述里已过时的「CNY 由 CNH 派生」与缺失的 `change_price`／`change_ratio` 两列）；⑥ **不改动数据库**：存量库里的 `public.finv_cer` 表与其 280 行历史数据原样保留（删表需另行明确授权），`schema_version` 中 300123 记录作为历史留存；⑦ 历史变更记录（`Docs/CHANGELOG.md`、`Docs/RequirementList.md` 中 10998/10999/11001/11002 条目）按「只增不改」原则保留原文。| `test ! -f Deploy/Migrations/V300123__public_create_finv_cer.sql`；`test ! -f Docs/DataDictMapping/FinvCer.md`；`grep -rn 'finv_cer\b' --include=*.go --include=*.md --include=*.sql --include=*.vue .` 仅剩历史变更记录与 `V300124` 的说明性注释；`grep -c 'FinvCer.md' Docs/API/Cdeu/FxCer.md Docs/Menu/Quote/Realtime/FxCer.md` 均为 0；`grep -c 'finv_cer\b' Docs/DevSpec/DbSchemaSpec.md` = 0；`go build ./...`、`go vet ./...` 通过；`go test ./internal/meta/finv/quote/cdeu/ -run TestBuildFxCERRows` PASS；容器重建后 `healthy`、`/Health/Ready` 返回 `obs ok`；库内 `to_regclass('public.finv_cer')` 仍为 `finv_cer` 且行数不变（280），`max(schema_version)=300125` | √ |
| 11022 | 修复「全新空库执行迁移链失败」，让空库能一路迁移到链尾并正常启动：① `V200090`（`brs.brs_bak_rdb_table` 加 `db` 列）版本号早于建表脚本 `V300060`，空库执行时表不存在 → 报 `42P01`；改为整体包进 `DO` 块，`to_regclass('brs.brs_bak_rdb_table') IS NULL` 时打印 `NOTICE` 并跳过（存量库行为不变）；② `V300060` 建表即带 `db varchar(64) NOT NULL DEFAULT 'finv'` 与四列唯一约束 `(rdb_type, db, schema_name, table_name)`，种子 `INSERT` 补 `db` 列与 `ON CONFLICT (rdb_type, db, schema_name, table_name)`，使空库一次建成与存量库一致的结构；③ `V300111`：`000985`/`SH0985` 为生产库既有数据的纠正校验，空库无这两行 → 改为「两行都不存在则 NOTICE 跳过」，且 `000985` 的 cdeu 登记校验改为「存在即必须是纠正后口径」（不再强制存在）；④ `V300113`：把按生产数据条数的硬校验（184/12/9 条）改为不变量校验——交易对不得留在非 `10016`、已登记交易对不得离开 `CC/CTP`、现货币不得离开 `CC/FTC`；无加密证券/登记数据时跳过并 NOTICE；⑤ 校验：新建空库用真实容器跑完整迁移链，`schema_version` 到 `300125`、共 55 张表、`public.finv_fxcer` 13 列、`public.finv_cer` 不再被创建、`brs_bak_rdb_table_full` 有 `db` 列、`000985` 纠正为 `Stock/1120`、`CTP` 市场已建、容器 `healthy`。| 空库上 `while read f; psql -v ON_ERROR_STOP=1 -f $f` 全量跑 228 个迁移文件零失败；`docker run` 用 `FINV_PG_DATABASE=<空库>` 启动镜像：日志无迁移错误、`/Health/Ready` 返回 `postgres/redis ok`；空库 `SELECT max(version) FROM public.schema_version` = `300125`；`SELECT count(*) FROM information_schema.tables WHERE table_schema IN ('public','cdeu','sys','brs','futu','quote')` = 55；`to_regclass('public.finv_cer')` 为空；`SELECT count(*) FROM information_schema.columns WHERE table_schema='brs' AND table_name='brs_bak_rdb_table_full' AND column_name='db'` = 1；存量库不受影响：容器重建后 `healthy`、`max(schema_version)` 与 `finv_fxcer` / `finv_cer` 行数均未变 | √ |
| 11023 | `cdeu.finv_cdeu_secu_realtime_stock` 增加 5 个人民币计价字段（迁移 `V300126`），为后续按人民币口径做证券排行做准备：① 列与类型 —— `rmb_exchange_rate NUMERIC(24,10)`（人民币转换汇率：1 单位本币折合的人民币数量，口径同 `public.finv_fxcer.rate`）、`rmb_market_value NUMERIC(24,6)`、`rmb_circulation_market_value NUMERIC(24,6)`、`rmb_trade_trunover NUMERIC(24,6)`、`rmb_price_nominal NUMERIC(20,6)`；② 全部可空，`NULL` 表示「尚未补全」而非 0；列序按指定顺序追加在原 26 列之后（共 31 列）；③ 加 CHECK 约束 `finv_cdeu_secu_realtime_stock_rmb_rate_chk`（`rmb_exchange_rate IS NULL OR > 0`）；④ 五条列注释写明换算口径（`rmb_* = 本币值 × rmb_exchange_rate`）与「NULL≠0」；⑤ 迁移幂等（`ADD COLUMN IF NOT EXISTS` + 约束存在性判断），末尾校验列清单/类型/列序/列总数/CHECK/注释数；⑥ 本轮只加字段，不改写入与接口（`RealtimeStockUpsert` 不接受这些字段，落库为 NULL），补全与排行后续实现。| `SELECT max(version) FROM public.schema_version` = `300126`；`SELECT string_agg(column_name \|\| ':' \|\| data_type, ',' ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_cdeu_secu_realtime_stock' AND ordinal_position>26` 输出 `rmb_exchange_rate:numeric,rmb_market_value:numeric,rmb_circulation_market_value:numeric,rmb_trade_trunover:numeric,rmb_price_nominal:numeric`；`SELECT count(*) FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_cdeu_secu_realtime_stock'` = 31；五列 `attnotnull` 均为 `false`；存在 CHECK `finv_cdeu_secu_realtime_stock_rmb_rate_chk`；五条列注释齐全；表行数在迁移前后不变；容器重建后 `healthy`、`/Health/Ready` ok | √ |
| 11024 | 「证券行情排行」页（`/Meta/Finv/Quote/Realtime/FTStockRank`）支持手动补全人民币计价指标：① 后端新增 `POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/RmbSync?step=all\|rate\|amount`（需登录，无请求体）：**第一步**按行内 `currency` 从 `public.finv_fxcer` 求「1 本币 = N 人民币」（`CNY` 恒为 1；回退顺序：直采 → 反向取倒数 → 经 USD 交叉，两腿各自允许取倒数）写入 `rmb_exchange_rate`；**第二步**用该汇率换算 `rmb_market_value` / `rmb_circulation_market_value` / `rmb_trade_trunover` / `rmb_price_nominal`（`round(...,6)`）；② 汇率乘除用 `math/big.Rat` 计算、按 10 位小数落库，不经 `float64`；查不到汇率的币种不写值并在响应 `missing_currencies` 列出，`currency` 为空的行计入 `rows_currency_missing`；本币值为 NULL 时对应 `rmb_` 字段保持 NULL（不写 0）；③ 两步均为差异更新（`IS DISTINCT FROM`），重复调用 `rate_update`/`amount_update` 为 0（幂等）；响应返回 `rows_total`/`rate_rows`/`rate_update`/`amount_rows`/`amount_update`/`rates`/`missing_currencies`/`elapsed_ms`；`step` 非法返回 400；④ 前端在筛选行「重置」之后新增**「人民币计价指标更新」**按钮（`mdi-currency-cny`、次级色、带悬浮说明），点击调 `step=all`，toast 提示汇率/金额行数与耗时、异常信息（无币种行、缺汇率币种），随后自动刷新列表；⑤ 文档：新增 `Docs/API/Cdeu/RealtimeStockRmbSync.md` + `.yml`，登记 `Docs/API/APIs.md`（120）与 `Docs/API/README.md`，更新 `Docs/Menu/Quote/Realtime/RealtimeStock.md`。| `grep -c 'Stock/RmbSync' internal/api/router.go` = 1；`grep -c 'RmbSync' internal/api/handler/cdeu_realtime_stock.go` ≥ 1；`grep -c '人民币计价指标更新' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 1；`go build ./...`、`go vet` 通过；`go test ./internal/meta/finv/quote/cdeu/ -run 'TestFxRateBookToCNY\|TestNormalizeRMBMetricsStep'` PASS；`cd Web && npm run build` 通过；实测 `POST ...?step=all` 返回 `code=0`、`rate_update=8671`、`amount_update=8671`、`rows_total=13906`、`rows_currency_missing=5235`、`missing_currencies=[]`；重复调用返回 `rate_update=0`/`amount_update=0`；`step=bogus` 返回 400；`SELECT count(*) FROM cdeu.finv_cdeu_secu_realtime_stock WHERE round(market_value*rmb_exchange_rate,6) IS DISTINCT FROM rmb_market_value AND market_value IS NOT NULL` = 0 | √ |
| 11025 | 「证券行情排行」按人民币口径展示与排行：① `GET .../Realtime/Stock/Page` 响应 `data.list[]` 增加 `rmb_exchange_rate` / `rmb_market_value` / `rmb_circulation_market_value` / `rmb_trade_trunover` / `rmb_price_nominal`（十进制文本，空串=未补全即库内 NULL，不是 0）；② 排序白名单增加上述 5 个字段，**`orderBy` 缺省值改为 `rmb_market_value`**（NULL 仍按 `NULLS LAST` 排最后），handler 与 store 两处默认值同步；③ 前端表格在「流通市值」后新增分组列：兑人民币汇率（按量级给 2~8 位小数）、人民币现价、人民币总市值、人民币流通市值、人民币成交额；市值/成交额用万/亿/万亿折算，未补全显示 `—`；`colspan` 21 → 26；④ 前端排序下拉新增 5 个人民币口径选项并置于最前，「人民币总市值」为默认排序，重置恢复该默认；⑤ 同步 `Docs/API/Cdeu/RealtimeStockPage.md` + `.yml`（响应字段、白名单、默认值、示例）与 `Docs/Menu/Quote/Realtime/RealtimeStock.md`（列与排序说明）。| `grep -c 'rmb_market_value' internal/meta/finv/quote/cdeu/realtime_stock.go` ≥ 2（白名单 + 行结构）；`grep -c 'rmb_market_value' internal/meta/finv/quote/cdeu/store_realtime_stock.go` ≥ 1（默认排序）；`grep -c '人民币总市值' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` ≥ 2；`go build ./...`、`go vet`、`cd Web && npm run build` 通过；实测不传 `orderBy` 返回按人民币总市值降序（首行 NVDA `rmb_market_value=33780560000000`）；`orderBy=rmb_trade_trunover`/`rmb_price_nominal` 排序生效；末页为未补全行（`rmb_market_value` 为空串）；响应每行含 5 个 `rmb_*` 字段 | √ |
| 11026 | **【待办·暂不实现】** 快照表 `cdeu.finv_cdeu_secu_realtime_stock` 中 `currency` 为空的 5235 行无法换算人民币计价指标，需先确定并落实币种回填口径：候选方案（待确认）——① 按 `market_label` 推断（`HK`→`HKD`、`SH`/`SZ`/`BJ`→`CNY`、`US`→`USD`）；② 按 `usc` 前缀推断（港股 `00005`、沪 `SH6xxxxx`、深 `SZ0xxxxx`、京 `BJ`，美股按 `public.finv_security.currency`）；③ 直接用 `public.finv_security.currency` 回填（但这些行大多 `usc` 也为空，属需求 10995「待录入证券」范围）。回填完成后重新执行「人民币计价指标更新」（需求 11024）即可覆盖剩余行。**本条仅登记待办，本轮不实现、不改代码、不改数据。** | 现状核对：`SELECT count(*) FROM cdeu.finv_cdeu_secu_realtime_stock WHERE currency IS NULL OR btrim(currency) = ''` = 5235；口径确认后另开需求编号实现，届时以 `SELECT count(*) FROM cdeu.finv_cdeu_secu_realtime_stock WHERE rmb_exchange_rate IS NULL` 归零作为验收；本轮只要求本行存在于 `Docs/RequirementList.md` | 待办 |
| 11027 | 从「证券行情排行」的排序白名单移除 `rmb_exchange_rate`：① 后端 `RealtimeStockOrderColumns` 删除 `rmb_exchange_rate`（保留 `rmb_market_value` / `rmb_circulation_market_value` / `rmb_trade_trunover` / `rmb_price_nominal` 四项指标）；② 前端排序下拉移除「人民币汇率」选项（该列仍作为展示列保留）；③ 文档同步：`Docs/API/Cdeu/RealtimeStockPage.md`/`.yml` 的白名单与枚举去掉该值并注明「汇率是换算参数，不参与排序」，`Docs/Menu/Quote/Realtime/RealtimeStock.md` 排序字段说明同步；④ 传 `orderBy=rmb_exchange_rate` 时按既有白名单校验返回 400（不再被接受）。| `grep -c 'rmb_exchange_rate' internal/meta/finv/quote/cdeu/realtime_stock.go` 仅出现在行结构与注释中、白名单无该键；`grep -c '人民币汇率' Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue` = 0（列标题为「兑人民币汇率」，排序项已删）；`go build ./...`、`cd Web && npm run build` 通过；`GET .../Stock/Page?orderBy=rmb_exchange_rate` 返回 400 `orderBy 非法`；`orderBy=rmb_market_value` / `rmb_trade_trunover` / `rmb_price_nominal` / `rmb_circulation_market_value` 仍生效 | √ |
| 11028 | `cdeu.finv_cdeu_secu_realtime_stock` 增加 `remark`（备注）列（迁移 `V300127`）：① 列定义 `remark TEXT` 可空，追加在原 31 列末尾（共 32 列），列注释写明「人工标注/采集端说明，不参与换算/过滤/排序；NULL 或空串表示无备注」；② 迁移幂等（`ADD COLUMN IF NOT EXISTS`），末尾校验列存在/类型 `text`/列序为第 32 列/列总数 32/注释存在；③ 写入接口 `POST .../Realtime/FTStockUpsert` 请求体新增可选字段 `remark`（`TEXT`，空串视同未提供），落库为 `COALESCE(EXCLUDED.remark, t.remark)`——**未提供不会清空已有备注**；④ 查询接口 `GET .../Realtime/Stock/Page` 每行返回 `remark`（空串=无备注）；⑤ 同步 `Docs/API/Cdeu/RealtimeStockUpsert.md`、`Docs/API/Cdeu/RealtimeStockPage.md`/`.yml` 与 `Docs/DevSpec/DbSchemaSpec.md`（列数 31→32）。| `SELECT max(version) FROM public.schema_version` = `300127`；`SELECT count(*) FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_cdeu_secu_realtime_stock'` = 32；`SELECT data_type FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_cdeu_secu_realtime_stock' AND column_name='remark'` = `text`；`go build ./...`、`go vet`、`go test ./internal/meta/finv/quote/cdeu/ -run TestBuildFxCERRows` 通过；实测 upsert 带 `remark` → 库内与 `GET .../Stock/Page` 均返回该备注；再 upsert 不带 `remark` → 备注保留不被清空；容器 `healthy` | √ |
| 11029 | `cdeu.finv_cdeu_secu_realtime_stock` 增加 `gmt_create` / `gmt_update` 审计时间列（迁移 `V300128`）：① `gmt_create TIMESTAMPTZ NOT NULL DEFAULT now()` —— 记录入库时刻，此后不再变化（存量行由 `ADD COLUMN` 的默认值回填为迁移执行时刻）；② `gmt_update TIMESTAMPTZ NOT NULL DEFAULT now()` —— 每次 `UPDATE` 由触发器 `trg_finv_cdeu_secu_realtime_stock_gmt_update` 调用既有函数 `public.vq_set_gmt_update()` 自动置为 `now()`（与 `finv_fxcer` 等同机制，不新建函数）；③ 两列追加在第 33、34 列（共 34 列），列注释写明口径；迁移幂等（`ADD COLUMN IF NOT EXISTS` + 触发器先 DROP 再建），末尾校验列存在/类型/可空性/默认值/列序/列总数/触发器/注释数；④ 写入接口不接受这两列（新增由 DEFAULT 取值、更新由触发器维护）；⑤ 查询接口 `GET .../Realtime/Stock/Page` 每行返回 `gmt_create` / `gmt_update`（RFC 3339）；⑥ 同步 `Docs/API/Cdeu/RealtimeStockPage.md`/`.yml`、`Docs/API/Cdeu/RealtimeStockUpsert.md`、`Docs/DevSpec/DbSchemaSpec.md`（列数 32→34）。| `SELECT max(version) FROM public.schema_version` = `300128`；`SELECT count(*) FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_cdeu_secu_realtime_stock'` = 34；两列 `data_type = timestamp with time zone`、`is_nullable = NO`、`column_default = now()`；存在非内部触发器 `trg_finv_cdeu_secu_realtime_stock_gmt_update`（`BEFORE UPDATE ... EXECUTE FUNCTION vq_set_gmt_update()`）；实测：`UPDATE` 一行后 `gmt_create` 不变、`gmt_update` 前进；经 `POST .../FTStockUpsert` 更新同一行同样只前进 `gmt_update`；`go build ./...`、`go vet`、`go test ./internal/meta/finv/quote/cdeu/ -run TestBuildFxCERRows` 通过；容器 `healthy` | √ |
| 11030 | 修正 3 条美股宽基股的 `public.finv_security.market_code`（迁移 `V300129`）：`V300068` 当年把无富途采集记录的 `BRK.B` / `EA` / `MTBLY` 统一回填为 `1312`（`finv_market` 语义是「美股指数（道琼斯系列）」，对个股错位），现按方案 A（交易所专码）修正为：`BRK.B` 1312→**1316**（NYSE，依据 `futu.finv_futu_quote_collect.market_code=10` 且上游快照 `BRK.A` 记为 `market=NYSE`）、`EA` 1312→**1315**（NSDAQ，按已知市场归属）、`MTBLY` 1312→**1316**（NYSE，按公司沿革推断，**发放前需业务确认**）。迁移只改 `market_code`，不动 `exchange_code`（三条均已是 30）与 `type_secu_sub` / `provider`；幂等（`market_code IS DISTINCT FROM 目标值`），前置校验目标码已在 `finv_market` 登记、三条 `usc` 存在且均为「exchange_code=30 的股票」，末尾收敛校验。| `SELECT max(version) FROM public.schema_version` ≥ `300129`；`SELECT usc, market_code FROM public.finv_security WHERE usc IN ('BRK.B','EA','MTBLY') ORDER BY usc` = `BRK.B=1316`、`EA=1315`、`MTBLY=1316`；复跑迁移 `UPDATE 0`（幂等）；`SELECT count(*) FROM public.finv_security WHERE usc IN ('BRK.B','EA','MTBLY') AND exchange_code=30` = 3（未误伤交易所） | √ |
| 11031 | 20261109 US 批次（4019 行中映射命中的）初始数据改以**迁移脚本**发放：`V300130__finv_security_seed_us_batch_20261109.sql` 与 `V300131__finv_futu_quote_collect_seed_us_batch_20261109.sql`（各 3603 行），随应用启动的迁移链自动应用并记入 `schema_version`，不再依赖一次性手工 SQL；生成器 `FinvQuantDesign/Scripts/SecuSync/BuildSecuInitFromCdeu.py` 增加 `--migration-dir`（自动取目录最大版本号 +1/+2 命名）。| `SELECT version FROM public.schema_version WHERE version IN ('300130','300131')` 两行均存在且 `success=t`；`SELECT count(*) FROM public.finv_security WHERE remark='20261109-stockCode=>usc批量映射-初始数据'` = 3597；`SELECT count(*) FROM futu.finv_futu_quote_collect WHERE remark='20261109-stockCode=>usc批量映射-初始数据'` = 3603；全新空库顺序执行 233 个迁移文件零失败；两条迁移在已部署库上重跑 `INSERT 0 0`（幂等） | √ |
| 11032 | 线上扫描缺失的 428 个 `stock_id` 中可补的 **417** 个，两表初始数据以迁移脚本发放：`V300132__finv_security_seed_us_missing428_20261109.sql` 与 `V300133__finv_futu_quote_collect_seed_us_missing428_20261109.sql`（各 417 行，`remark='20261109-线上缺失证券补齐'`）。数据来源：映射 CSV `Supabase Snippet finv_quote_secu_meta.csv` 命中 261（字段齐备）+ 本地 `cdeu` 快照 156（REIT/ETF 为主，`market_code` 置 0=未维护）；排除 11 个：3 个 `usc` 撞车（`XRP`/`BFLY`/`DJT` 与既有加密币或指数同名）、8 个本地任何来源都无（需上游按 `stock_id` 采集）。依据分析报告 `FinvQuantDesign/Data/Secu/Out/secu428_missing_analysis.md`（含 428 个分类明细）。| `SELECT count(*) FROM public.finv_security WHERE remark='20261109-线上缺失证券补齐'` = 417；`SELECT count(*) FROM futu.finv_futu_quote_collect WHERE remark='20261109-线上缺失证券补齐'` = 417；`SELECT count(*) FROM public.schema_version WHERE version IN ('300132','300133') AND success` = 2；428 个 id 中有 417 个在 `futu.finv_futu_quote_collect` 可查到（其余 11 个为已知排除项）；两表本批 `usc` 无重复、无 `(exchange_code, security_code)` / `(market_code, futu_symbol)` 冲突 | √ |
| 11033 | 线上缺失 428 个中剩余 **9 个**（8 个无任何外部来源 + 1 个 `usc` 撞车）改为**就地派生**发放：迁移 `V300134__finv_security_fix_missing_9_from_cdeu.sql` 不使用外部数据，直接 `INSERT ... SELECT FROM cdeu.finv_cdeu_secu_realtime_stock`按 `stock_id` 派生两表记录（`remark='20261109-线上缺失证券补齐-第二批'`）——线上 cdeu 有这几行即补出，没有则影响 0 行。口径：`market_code=0`（未维护，cdeu 无交易所码）、`exchange_code`/`market_type`/`currency_type`/时区按 `market_label` 换算（US/HK/SH/SZ/BJ，未登记市场落 100/0/''）、名称三列取 cdeu 中文名、`flag_enable='0'`；两条 INSERT 均带「usc 在 `finv_security` 与目标表都不存在」的守卫，**`usc` 已被其它证券占用（如 `XRP`）自动跳过**，不覆盖不报错；末尾逐条 NOTICE 输出未处理原因（cdeu 无此行 / usc 撞车）。| `SELECT version FROM public.schema_version WHERE version='300134' AND success` 存在；本地执行：cdeu 仅命中 `XRP` 一行且被守卫跳过（`finv_security=0`、`futu=0`），9 条 NOTICE 逐条给出原因；假行事务测试（`ZZTEST1`，已回滚）：两表各插入 1 行、`market_code=0`、`quote_market=US`、`timezone=-04:00`，字段映射与 NOT NULL 约束均通过；重跑 `INSERT 0 0`（幂等） | √ |
| 11034 | 为 Bitwise XRP ETF 分配独立 `usc = ETFXRP`，解决与加密币 `XRP` 的 usc 撞车（迁移 `V300135`）：① `cdeu.finv_cdeu_secu_realtime_stock` 中 `stock_id=87660282761853` 的 `usc` 由 `XRP` 更正为 `ETFXRP`；② `futu.finv_futu_quote_collect` 插入 `usc='ETFXRP'`（`futu_symbol='XRP'`、`market_code=12`、`instrument_type=4`、`sub_instrument_type=4002`、`type_symbol='etfs'`）；③ `public.finv_security` 插入 `usc='ETFXRP'`（`security_code='XRP'` 保留交易所原码、`security_type='ETF'`、`type_secu_sub='ETF.US'`、`market_code=0`、`flag_enable='0'`、`provider='FTMM'`）；④ 加密币 `XRP`（`market_code=10001`）不做任何修改；迁移幂等且带防串校验（`ETFXRP` 若已被其它证券占用则报错回滚）。| cdeu：`SELECT usc FROM cdeu.finv_cdeu_secu_realtime_stock WHERE stock_id=87660282761853` = `ETFXRP`；`SELECT count(*) FROM futu.finv_futu_quote_collect WHERE usc='ETFXRP' AND futu_symbol='XRP' AND market_code=12 AND instrument_type=4` = 1；`SELECT security_type, security_code, market_code, flag_enable FROM public.finv_security WHERE usc='ETFXRP'` = `ETF/XRP/0/0`；`SELECT security_type, market_code FROM public.finv_security WHERE usc='XRP'` 仍为 `Crypto/10001`（未被改动）；复跑迁移 `UPDATE 0` + 2 条 `INSERT 0 0`（幂等） | √ |
| 11035 | 按用户指定更正 5 个证券的 `usc` 并发放两表初始数据（迁移 `V300136`）：`80199924522296`→`BFLYN`（Butterfly Network，原 `BFLY` 属 Cboe 铁蝴蝶指数）、`85070417466455`→`DJTG`（特朗普媒体科技集团，原 `DJT` 属道琼斯交通运输指数）、`85611583331825`→`BTCMT`（Grayscale Bitcoin Mini Trust，原占位 `USBTC`）、`85577223574361`→`LETH`（Grayscale Ethereum Mini Trust，原占位 `USETH`）、`87660282761853`→`ETFXRP`（V300135 已完成，本迁移幂等跳过）。迁移内容：① 更正 `cdeu` 侧 `usc`；② 两表把上批别名行（`USBTC`/`USETH`）**改名为新 usc**（改名前要求 `security_name`/`futu_symbol` 与清单一致，避免误改同名指数行）并修正 `security_code`（`BTC`/`ETH`）；③ 补齐 `BFLYN`/`DJTG` 两表缺失行（`security_code` 用新 usc，因 `BFLY`/`DJT` 已被同名指数占用 `(exchange_code, security_code)`）；④ 市场码沿用方案 A（股票 1316/1315，ETF 置 0）。| `SELECT usc, security_type, security_code FROM public.finv_security WHERE usc IN ('BFLYN','DJTG','BTCMT','LETH','ETFXRP')` 依次为 `BFLYN/Stock/BFLYN`、`DJTG/Stock/DJTG`、`BTCMT/ETF/BTC`、`LETH/ETF/ETH`、`ETFXRP/ETF/XRP`；`futu` 侧对应 `BFLY/mc10`、`DJT/mc11`、`BTC/mc12`、`ETH/mc12`、`XRP/mc12`；同名标的未被误改：`BFLY`=StockIndex、`DJT`=StockIndex、`XRP`=Crypto 保持不变，`USBTC`/`USETH` 行已不存在；复跑迁移全部 `UPDATE 0` / `INSERT 0 0` | √ |
| 11036 | 新建证券键表 `futu.finv_quote_secu_key`（迁移 `V300137`），按用户提供的 Supabase DDL 落地：10 列 `instrument` / `market` / `code` / `ftmm_symbol` / `flag_enable`（SMALLINT 默认 1）/ `priority`（取值 1~1000000，默认 10）/ `status`（TEXT 默认 `PENDING`）/ `url`（可空）/ `gmt_create` / `gmt_update`；主键 `ftmm_symbol`，索引 `(instrument, market)`。适配本仓库约定：索引建在 `futu` schema（原 DDL 写 `public.` 属残留）、`gmt_update` 复用既有 `public.vq_set_gmt_update()` 而非新建专用函数、触发器名 `trg_finv_quote_secu_key_gmt_update`、补齐表与列注释；幂等且末尾收敛校验（列数 10 / 主键 / 索引 / 触发器 / 注释）。| `SELECT count(*) FROM information_schema.tables WHERE table_schema='futu' AND table_name='finv_quote_secu_key'` = 1；`SELECT count(*) FROM information_schema.columns WHERE table_schema='futu' AND table_name='finv_quote_secu_key'` = 10；主键为 `ftmm_symbol`、存在索引 `idx_finvquotesecukey_market`、存在触发器 `trg_finv_quote_secu_key_gmt_update`；实测：`priority=0` 触发 CHECK 报错、重复 `ftmm_symbol` 触发主键报错、`UPDATE` 后 `gmt_update` 自动前进；`schema_version` ≥ `300137`；重跑迁移幂等 | √ |
| 11037 | `futu.finv_quote_secu_key.status` 取值固定为 **PENDING / QUEUED / DONE / FAILED / SKIP** 并加 CHECK 约束（迁移 `V300138`）：约束名 `finv_quote_secu_key_status_check`，默认值仍为 `PENDING`；列注释写明 5 个合法值；存量若存在非约定值则迁移报错回滚。**按用户要求不写任何初始数据**（后续走同步）。| `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname='finv_quote_secu_key_status_check'` 含 5 个取值；逐一插入 5 个合法值均成功；插入 `BOGUS` 报 `violates check constraint`；`SELECT column_default … WHERE column_name='status'` 仍为 `'PENDING'::text`；重跑迁移幂等 | √ |
| 11038 | `futu.finv_quote_secu_key` 增加 `stock_id`（BIGINT）与 `usc`（TEXT）两列（迁移 `V300139`）：均**可空、无默认值（即 NULL）**，分别对应 `futu.finv_futu_quote_collect.stock_id` 与 `public.finv_security.usc`；NULL 表示「尚未解析到」，供同步任务回填；列注释写明口径，列数 10→12，迁移幂等且校验列存在/类型/可空/无默认值/列数。| `SELECT count(*) FROM information_schema.columns WHERE table_schema='futu' AND table_name='finv_quote_secu_key'` = 12；两列 `data_type` 分别为 `bigint`/`text`、`is_nullable='YES'`、`column_default IS NULL`；实测：只写前 10 列时两列为 NULL，写入后可按 `ftmm_symbol` 读回；重跑迁移跳过（`already exists`） | √ |
| 11039 | `futu.finv_futu_quote_collect` 两个列改名（迁移 `V300140`，用户明确点名，属 DbSchemaSpec §0 红线操作的授权例外）：`type_symbol` → **`instrument`**、`quote_market` → **`market_label`**；仅改名，类型/NOT NULL/数据不变（21,161 行零影响）。命名对齐：`instrument` 与键表 `finv_quote_secu_key.instrument`、上游 `finv_quote_secu_meta.instrument` 一致；`market_label` 与上游同名字段及 `cdeu.finv_cdeu_secu_realtime_stock.market_label` 一致。同步调整业务代码：Go（`store_daykline.go` 的 `FutuQuoteParams`/`FutuCollectRow` 结构与 SQL、`daykline_sync.go` 日志、`store.go` 富途兜底 SQL 与注释、两个集成测试）、前端（`DayKlineSyncView.vue`、`PendingSecuEnrollDialog.vue` 字段键与展示）、文档（`DayKlineSync.md`、`DayKlineUscList.md`、`QuoteDeDup/Usc.md`、`Menu/Quote/Realtime/PendingSecu.md`、`DbSchemaSpec.md`）与仓库外生成器（`BuildFutuQuoteCollect.py`、`BuildSecuInitFromCdeu.py`、`SecuSync/README.md`）。历史迁移（`V200083` 建表、`V300063`/`V300131`/`V300133`/`V300134`/`V300135`/`V300136` 写入）均在 `V300140` 之前执行，按惯例不回改。| `SELECT count(*) FROM information_schema.columns WHERE table_schema='futu' AND table_name='finv_futu_quote_collect' AND column_name IN ('market_label','instrument') AND is_nullable='NO'` = 2，且 `column_name IN ('quote_market','type_symbol')` = 0；`SELECT count(*) FROM futu.finv_futu_quote_collect` 仍为 21161（数据零损失），`market_label`/`instrument` 分布与改名前的 `quote_market`/`type_symbol` 一致；`go build ./...`、`go vet`、`npm run build` 通过；接口实测 `GET .../DayKline/UscList` 返回 `market_label`/`instrument`，`POST .../DayKline/Sync` 能正常读到采集参数（仅上游 moomoo 网络 EOF）；全仓库 `grep -rn 'quote_market\|type_symbol'` 仅剩历史迁移与 CHANGELOG/RequirementList | √ |
| 11040 | 表改名 `futu.finv_quote_secu_key` → **`futu.finv_futu_secu_key`**（迁移 `V300141`，用户明确点名，属 DbSchemaSpec §0 红线操作的授权例外）：前缀与同域 `finv_futu_mapping_*` / `finv_futu_quote_collect` 对齐；同时把派生对象一并改名——主键 `finv_futu_secu_key_pkey`、CHECK `finv_futu_secu_key_priority_check` / `finv_futu_secu_key_status_check`、索引 `idx_finvfutusecukey_market`、触发器 `trg_finv_futu_secu_key_gmt_update`，以及 PG18 登记进 `pg_constraint` 的 NOT NULL 约束名；表结构（12 列）/ 注释 / 数据不变。历史迁移 `V300137`–`V300139` 按惯例不回改。| `SELECT to_regclass('futu.finv_quote_secu_key') IS NULL` = t 且 `to_regclass('futu.finv_futu_secu_key') IS NOT NULL` = t；`SELECT count(*) FROM information_schema.columns WHERE table_schema='futu' AND table_name='finv_futu_secu_key'` = 12；`pg_constraint` 无 `conname LIKE 'finv_quote%'` 残留；索引/触发器均为新名；实测插入（含 `stock_id`/`usc`）成功、`status='BOGUS'` 按新约束名报错；重跑迁移为 no-op | √ |
| 11041 | `sys.sys_config` 新增系统配置 **20013「Supabase 数据库连接配置」**（迁移 `V300142`）：`data_type='3'`（字符串），取值存 `str_value`，格式固定为 **`{PROJECT_ID}\|{APIKEY}`**（半角竖线分隔 2 段；PROJECT_ID=Supabase 项目 ID，即 `https://<PROJECT_ID>.supabase.co` 的子域；APIKEY=Supabase API Key，如 `sb_publishable_...`）；**初始值留空 = 未配置（不连接 Supabase）**，由运维在系统配置里填写；`remark` 写明格式、留空语义、非法值按未配置处理与「敏感信息请勿在日志/前端明文回显」；`create_by='finv'`；仅 `INSERT ... ON CONFLICT (config_no) DO NOTHING`，**绝不覆盖用户已配置的值**，幂等。| `SELECT data_type, quote_nullable(str_value) FROM sys.sys_config WHERE config_no=20013` = `3` / `''`（交付态为未配置）；`SELECT remark FROM sys.sys_config WHERE config_no=20013` 含 `{PROJECT_ID}\|{APIKEY}`；把 `str_value` 人为改成 `myproj123\|sb_publishable_TESTVALUE` 后重跑迁移 → `INSERT 0 0` 且值不变（未覆盖）；`schema_version` ≥ `300142` | √ |
| 11042 | 在「缺省汇率」菜单之后新增菜单 **「证券参数同步」**（key `quote-realtime-secu-param-sync`，路由 `/Meta/Finv/Quote/Realtime/SecuParamSync`，视图 `Web/src/views/Meta/Finv/Quote/Realtime/SecuParamSyncView.vue`，图标 `mdi-database-sync-outline`，所属「行情 → 实时行情」）：页面自上而下四块——① **连接与目标表**（20013 是否已配置 + 项目 ID + 脱敏 key 前缀 / 目标表 `futu.finv_futu_secu_key` 行数与最近变更时间）；② **同步参数**（源表名输入 + 每批行数下拉 500/1000/2000/5000 + 「保存源表名」+ 「立即同步」）；③ **同步结果**（拉取/新增/更新/未变/跳过/status 归一 摘要 + 逐批日志表）；④ **说明**（页面最底部唯一「说明」卡片，按 ExplainCardSpec）。同步只由该页按钮手动触发，无定时任务。| `grep -c 'quote-realtime-secu-param-sync' Web/src/config/menuConfig.ts` = 1，且 `grep -A3 'quote-realtime-fxcer-overview' Web/src/config/menuConfig.ts \| grep -c 'secu-param-sync'` = 1（紧跟缺省汇率之后）；`test -f Web/src/views/Meta/Finv/Quote/Realtime/SecuParamSyncView.vue` 且 `grep -c 'SecuParamSync' Web/src/router.ts` = 2；`npm run build` 通过且产物 JS 含「证券参数同步」「SecuKey/Status」「SecuKey/Sync」；`docs/Menu/Menus.md` 与 `Docs/Menu/Quote/Realtime/SecuParamSync.md` 已登记 | √ |
| 11043 | 实现 **Supabase 证券键表 → 本地 `futu.finv_futu_secu_key` 的分批增量同步**（需求 11042 页面的按钮后端），代码在 `internal/meta/finv/quote/secukey/`（`client.go`/`store.go`/`service.go`）+ 处理器 `internal/api/handler/cdeu_secu_key.go`，三个接口：`GET /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Status`（只读本地快照）、`POST .../SecuKey/Config`（存源表名）、`POST .../SecuKey/Sync`（同步执行）。口径：按 `ftmm_symbol` 升序用 PostgREST `limit`/`offset` 分批拉取（每批 100~5000，默认 1000），每批一次 `INSERT ... ON CONFLICT (ftmm_symbol) DO UPDATE ... WHERE 目标列 IS DISTINCT FROM EXCLUDED`；**缺则插入、变了才更新、未变不改写**（不改写即不触发 `gmt_update` 触发器）；单向只读上游、不删本地行、不清空本地 `stock_id`/`usc`；字段映射 `instrument/market/code/ftmm_symbol` 直传、`enable→flag_enable`（NULL/非 0→1）、`priority`（NULL→10，越界收敛 1~1000000）、`status`（空/非法→`PENDING` 并计入 `status_reset`）、`url`（空白→NULL）；关键字段为空的行跳过并计入 `skipped`；同批同 `ftmm_symbol` 保留最后一条；schema 通过 `Accept-Profile` 头指定（写成 `/rest/v1/public.<表>` 会被 PostgREST 当成 `public.public.<表>` 报 `PGRST205`）；上游网络/429/5xx 重试 3 次（退避 500ms、1s），4xx 不重试，404/401/403 给出可读排查提示。| 首次同步（`batch_size=5000`）：`rows_total=73212`、`batches=15`、`insert=73212`、`update=0`、`elapsed_ms≈13148`；复跑：`insert=0`、`update=0`、`unchanged=73212` 且 `SELECT max(gmt_update) FROM futu.finv_futu_secu_key` 前后相同；本地把 `US.ANGU` 的 `priority` 改为 999 并写入 `stock_id`/`usc` 后再同步：`update=1`、`unchanged=73211`，该行 `priority` 复原为上游值且 `stock_id`/`usc` 保留；`POST .../SecuKey/Sync` 拉取不存在的表 → HTTP 500 + `code=2006` + `PGRST205` 原文；20013 清空后 `Status.configured=false`、`Sync` → HTTP 500「Supabase 连接不可用」；`go build ./...`、`go vet`、`go test ./internal/meta/finv/quote/secukey/`、`npm run build` 通过 | √ |
| 11044 | **Supabase 源表名支持在菜单里配置**：新增系统配置 **20014「Supabase 证券键源表名」**（迁移 `V300143`，`data_type='3'`，默认 **`finv_quote_secu_key`**，`create_by='finv'`，仅 `INSERT ... ON CONFLICT DO NOTHING` 不覆盖用户值）；「证券参数同步」页的源表名输入框读取该配置，点「保存源表名」经 `POST .../SecuKey/Config` 写回；同步接口也接受本次覆盖参数 `source_table`（不写库）；表名只允许 `[A-Za-z_][A-Za-z0-9_]*`（最多 `schema.table` 两段）以防 URL 注入，留空/非法按默认表名处理，非法入参返回 400。| `SELECT config_no, data_type, str_value FROM sys.sys_config WHERE config_no=20014` = `20014 / 3 / finv_quote_secu_key`；把该值改为 `my_table` 后重跑迁移 → `INSERT 0 0` 且值不变；`POST .../SecuKey/Config` 传 `public.finv_quote_secu_key` → `code=0` 且 `data.source_table=public.finv_quote_secu_key`、随后同步 `unchanged=73212`（`Accept-Profile` 生效）；传 `finv;drop table x` → HTTP 400「只允许字母、数字与下划线」；`schema_version` ≥ `300143` | √ |
| 11045 | 「证券参数同步」页面拆成**两个 Tab**（需求 11042 的单页改造）：① **「FT证券基础参数同步」**（现有 Supabase 证券键表 → `futu.finv_futu_secu_key` 的同步，功能不变）；② **「证券指标数据同步」**（证券指标数据的上游同步，当前为占位页：`待接入` chip + 明确「口径未确认、功能未接入」+ 已探测到的上游候选表 `public.finv_quote_secu_meta` 与待确认四项 + 「立即同步」按钮置灰）。页面底部仍只保留唯一「说明」卡片（ExplainCardSpec），按四段组织并对两个 Tab 都有交代。| 页面 `SecuParamSyncView.vue` 含两个 `v-tab`：`grep -c 'FT证券基础参数同步' 与 '证券指标数据同步'` 各 ≥ 1；产物 JS 同时含这两个字符串与「待接入」；Tab 2 的「立即同步」按钮 `disabled` 且页面文案含「同步口径尚未确认」；Tab 切换后说明卡片始终在页面最底部且仅一个 `mdi-information-outline` 标题「说明」 | √ |
| 11046 | 「FT证券基础参数同步」的同步改为**异步运行 + 实时日志**：新增 `POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run`（立刻返回 `run_id` 与首条日志，已有运行在执行返回 409）与 `GET .../SecuKey/Sync/Run?runId=`（返回 `status`/`progress`/累计计数/`logs[]`，`runId` 省略取最近一次、不匹配返回 404）；服务端 `secukey.Runner` 内存持有单次运行（状态只存内存、整体超时 60 分钟、日志上限 2000 行），每批回调写入一条日志（含本批差值与本批耗时）；前端点「立即同步」后立刻出现「同步进度与日志」卡片，每 800 ms 轮询并以**独立滚动日志区**逐行追加（自动滚到底、等宽字体、`ERROR` 行标红、运行中显示「等待下一批…」），不必等同步结束；原同步接口 `POST .../SecuKey/Sync` 保留（供脚本/接口直接调用）。| 启动实测：`POST .../Sync/Run` → HTTP 202 + `run_id` + `logs[0]=同步任务已启动`；运行中再次启动 → HTTP 409 且 message 带正在运行的 run_id；轮询实测 `progress` 依次 13.66%→27.32%→34.15%→47.81%→61.47%→75.12%→100%，`logs` 同步增长（3→5→6→8→10→12→17）；完成后 `status=DONE`、15 批、`logs=17`（启动 + 15 批 + 完成）、`unchanged=73212`、`elapsed_ms≈7854`；失败路径（源表不存在）→ `status=FAILED` + `error` 非空 + 最后一行 `ERROR 同步失败：…`；`runId` 不存在 → HTTP 404；`go build ./...`、`go test ./internal/meta/finv/quote/secukey/`、`npm run build` 通过 | √ |
| 11047 | **待办：Tab 2「证券指标数据同步」的口径待确认，本轮不实现、不建表、不写入任何数据。** 待确认四项：① 来源表名（已探测上游候选 `public.finv_quote_secu_meta`，10,910 行，含 `price`/`volume`/`turnover`/`pe_ttm`/`pb_ratio`/`total_market_cap`/`price_highest_week52` 等指标字段）；② 目标表（本地是否新建；现有 `cdeu.finv_cdeu_secu_realtime_stock` 只覆盖部分行情字段）；③ 字段映射与类型精度（金额/比率分开）；④ 增量键与「变了才更新」的判据列。确认后按 Tab 1 同款（异步启动 + 轮询 + 实时日志 + 分批增量 upsert）实现。| `grep -c '证券指标数据同步' Web/src/views/Meta/Finv/Quote/Realtime/SecuParamSyncView.vue` ≥ 1 且该 Tab 内「立即同步」按钮带 `disabled`；`SELECT count(*) FROM information_schema.tables WHERE table_name LIKE '%indicator%'` 不因本轮新增（本轮未建任何指标表）；本条目状态为**待办** | 待办 |
| 11048 | 在 **`cdeu` schema 新建证券指标表 `cdeu.finv_quote_secu_meta`**（迁移 `V300144`），结构按用户 2026-11-09 提供的上游建表脚本（Supabase `public.finv_quote_secu_meta`）同构创建：**131 列**（数值 80 列统一 `numeric(30,10)`、`smallint` 12、`integer` 7、`bigint` 5、`text` 24、`timestamptz` 3），主键 `usc`（`finv_quote_secu_meta_pkey`），`ftmm_symbol` NOT NULL，索引 `idx_finv_quote_secu_meta_ts (ts)` 与 `idx_finv_quote_secu_meta_dt_update (dt_update)`，触发器 `finv_quote_secu_meta_set_dt_update`（`BEFORE UPDATE` → 函数 `cdeu.finv_quote_secu_meta_set_dt_update()` 刷新 `dt_update=now()`）；表放 `cdeu` 域、触发器函数建在 `cdeu`（不复用 `public.vq_set_gmt_update()`，因列名是 `dt_update` 而非 `gmt_update`）；`dt_collect`/`dt_create`/`dt_update` 保持上游口径（可空、无默认值）；仅建表，**不写入任何数据、不改其他表**。| `SELECT count(*) FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_quote_secu_meta'` = 131；`SELECT count(*) FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_quote_secu_meta' AND data_type='numeric' AND numeric_precision=30 AND numeric_scale=10` = 80；`SELECT conname FROM pg_constraint WHERE conrelid='cdeu.finv_quote_secu_meta'::regclass AND contype='p'` = `finv_quote_secu_meta_pkey` 且列仅 `usc`；两个索引存在（`pg_indexes` 命中 2）；触发器存在且 UPDATE 后 `dt_update` 前进、`dt_create` 不变；迁移重跑幂等（`relation already exists, skipping`）；`SELECT count(*) FROM cdeu.finv_quote_secu_meta` = 0（仅建表未灌数） | √ |
| 11049 | 证券指标表改名 **`cdeu.finv_quote_secu_meta` → `cdeu.finv_quote_secu_indicator`**（迁移 `V300145`，用户明确点名，属 DbSchemaSpec §0 红线 RENAME 的授权例外）：列（131 列）/ 类型 / 可空性 / 数据全部不变；派生对象一并改名——主键 `finv_quote_secu_indicator_pkey`、索引 `idx_finv_quote_secu_indicator_ts` / `idx_finv_quote_secu_indicator_dt_update`、触发器 `finv_quote_secu_indicator_set_dt_update`、触发器函数 `cdeu.finv_quote_secu_indicator_set_dt_update()`，以及 PG18 登记进 `pg_constraint` 的 NOT NULL 约束名；表/列注释按新名重写。历史迁移 `V300144` 不回改。| `to_regclass('cdeu.finv_quote_secu_meta')` = NULL 且 `to_regclass('cdeu.finv_quote_secu_indicator')` 存在；`SELECT count(*) FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_quote_secu_indicator'` = 131；`SELECT conname FROM pg_constraint WHERE conrelid='cdeu.finv_quote_secu_indicator'::regclass AND contype='p'` = `finv_quote_secu_indicator_pkey`；两个索引与触发器/函数均为新名、无 `finv_quote_secu_meta%` 残留；UPDATE 后 `dt_update` 前进、`dt_create` 不变；迁移重跑幂等 | √ |
| 11050 | 实现「证券参数同步 → Tab 2 **证券指标数据同步**」：来源固定上游 **`public.finv_quote_secu_meta`**（10,910 行 / 145 列），**一条上游记录一次写三张本地表**——① `cdeu.finv_quote_secu_indicator`：**131 列同名同步**（`dt_update` 由触发器维护，不写入、不参与比对）；② `futu.finv_futu_quote_collect`：身份字段同名同步（`stock_id`/`market_type`/`market_code`/`instrument_type`/`sub_instrument_type`/`market_label`/`instrument`，`futu_symbol` 由 `ftmm_symbol` 去掉市场前缀派生），唯一键 `(market_code, futu_symbol)` 被别的 `usc` 占用时跳过该行；③ `futu.finv_futu_secu_key`：只补 `url`（来源 **`secu_url`**）与 `stock_id`，按 `ftmm_symbol` 对齐。**源记录 `usc` 为空时用 `ftmm_symbol` 占位**（上游 3,673 行为空）。接口三支：`GET .../SecuKey/Indicator/Status`、`POST .../SecuKey/Indicator/Sync/Run`（异步，立刻返回 `run_id`，已有运行返回 409）、`GET .../SecuKey/Indicator/Sync/Run?runId=`（`progress` + 逐批实时日志，`runId` 未知返回 404）；页面 Tab 2 与 Tab 1 同款「异步启动 + 800ms 轮询 + 独立滚动日志区」。增量口径：主键不存在则插入、字段与源不同才更新、一致的行不改写。| 首跑实测：`rows_total=10910`、3 批、指标表 `insert=10910`、采集属性表 `insert=3257` + `qc_skip=419`、键表 `key_update=10910`；复跑 `ind_insert=0 / ind_update=0 / ind_unchanged=10910`、`qc_*=0`、`key_update=0`；把 `00010` 的 `price` 改 999 后复跑 → `ind_update=1` 且恢复上游值；`SELECT count(*) FROM cdeu.finv_quote_secu_indicator` = 10910 且 `WHERE usc = ftmm_symbol` = 3673；`SELECT count(*) FROM futu.finv_futu_secu_key WHERE url IS NOT NULL` = 10910、`stock_id IS NOT NULL` = 10910；`go build`/`go vet`/`go test`/`npm run build` 通过，产物含 `Indicator/Sync/Run` | √ |
| 11051 | **`futu.finv_futu_quote_collect.flag_enable` 取值规则**（指标同步写入时）：记录 `usc` 用的是 **`ftmm_symbol` 占位** → `flag_enable='0'`（禁用采集）；`usc` 为上游真实值 → `flag_enable='1'`（启用采集）。**插入与更新都按此推导覆盖**（即该列是同步派生列，不是人工开关，人工改的值下次同步会被改正）。| 占位行（`usc = ftmm_symbol`）反例检查：`SELECT count(*) FROM futu.finv_futu_quote_collect q JOIN cdeu.finv_quote_secu_indicator i ON i.usc=q.usc WHERE i.usc=i.ftmm_symbol AND q.flag_enable <> '0'` = 0；且上述占位行 3,257 行均为 `'0'`、真实 usc 行均为 `'1'`；把占位行 `CC.AAVE` 改成 `'1'`、真实 usc 行 `00010` 改成 `'0'` 后复跑 → 日志 `qc_update=2`，两行分别恢复 `'0'` / `'1'` | √ |
| 11052 | Tab 1（Supabase 证券键表 → `futu.finv_futu_secu_key`）**不得清空 Tab 2 补全的 `url`**：源键表 `url` 全为 NULL，upsert 改为 `url = COALESCE(EXCLUDED.url, t.url)`，比对元组同口径（源为 NULL 时保留本地值）。背景：Tab 2 按 `secu_url` 补完 10,910 行 `url` 后，Tab 1 一跑就把它们全清空（`update=10910`）。| Tab 2 跑完后执行 Tab 1 同步：`insert=0`、`update=0`、`unchanged=73212`；随后 `SELECT count(*) FROM futu.finv_futu_secu_key WHERE url IS NOT NULL` 仍为 10910 | √ |
| 11053 | **`cdeu.finv_quote_secu_indicator` 全部 131 列补齐中文注释**（迁移 `V300146`）：建表时只有表与 8 个关键列有注释，其余 123 列为空。列说明取自上游采集脚本 `CodeSnippet/TypeScript/FtmmSecuMetaFetch/sql/schema.pg.sql` 的 `COMMENT ON COLUMN "finv_quote_secu_meta"` 段落（如 `price`=最新价、`pe_ttm`=市盈率 TTM、`total_market_cap`=总市值、`stat_h24_turnover`=24h 成交额、`change_d250`=250 日涨跌幅、`difference`=报价量纲基数（待确认））；本表专有 10 列（`usc` / `ftmm_symbol` / `ts` / `secu_url` / `source_channel` / `dt_collect` / `dt_create` / `dt_update` / `count_update` / `json_unmapped`）用更贴合本地语义的措辞覆盖（如 `usc` 注明「上游为空时由同步用 `ftmm_symbol` 占位」、`dt_update` 注明「由触发器刷新」）；表注释同步刷新。仅改注释，**不动列结构、不动数据**。| `SELECT count(*) FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_quote_secu_indicator'` = 131，且 `count(*) FILTER (WHERE COALESCE(col_description('cdeu.finv_quote_secu_indicator'::regclass, ordinal_position),'') <> '')` = 131（无空注释）；`\d+ cdeu.finv_quote_secu_indicator` 每列 Description 非空；抽查 `pe_ttm`=`市盈率 TTM`、`total_market_cap`=`总市值`、`secu_url` 含「同步到 futu.finv_futu_secu_key.url」；迁移重跑幂等（NOTICE 输出 131 列已有注释） | √ |
| 11054 | 新增「证券参数同步 → Tab 3 **上游 usc 回填**」（方向与 Tab 1/2 相反：**本地 → 上游**）：把本地 `futu.finv_futu_quote_collect.usc`（24,418 行）**按 `stock_id` 关联**写回上游 Supabase `public.finv_quote_secu_meta.usc`（上游该列原有 3,673 行为 NULL）。口径：关联键是 `stock_id`（**不是** `usc`，也不是「只更新 `usc IS NULL` 的行」）；本地按 `stock_id` 去重后升序分批（24,348 个）；上游该行 `usc` 已等于目标值 → 记「未变」且**不发请求**；为空 → 「填入」；有值但不同 → 「覆盖」；本地有、上游没有的 `stock_id` → 「无对应」跳过。写入逐行 `PATCH /rest/v1/<表>?stock_id=eq.<X>`，并发 8，`Prefer: return=minimal, count=exact` 且**校验受影响行数 = 1**（RLS 拒绝时必须报错，不能静默成功）。接口三支：`GET .../SecuKey/UscBackfill/Status`、`POST .../SecuKey/UscBackfill/Sync/Run`（异步，409 防并发）、`GET .../SecuKey/UscBackfill/Sync/Run?runId=`（进度 + 逐批日志，未知 runId 404）；页面 Tab 3 与 Tab 1/2 同款交互。| 首跑（`batch_size=500`，49 批，118,669 ms）：`rows_total=24348`、`fill=3673`、`overwrite=3`、`unchanged=7234`、`ambiguous=0`、`no_target=13438`，五项合计 = 24,348；上游 `usc IS NULL` 由 3,673 降为 **0**、非空由 7,237 升为 **10,910**；复跑（`batch_size=1000`，25 批，24,031 ms）：`fill=0 overwrite=0 unchanged=10910`（幂等），`no_target` 仍 13,438；运行中再次 POST → 409 且 message 带运行中 run_id；未知 runId → 404；`go build`/`go vet`/`go test`/`npm run build` 通过 | √ |
| 11055 | **本地同一 `stock_id` 对应多条 `usc` 时的取值规则**（上游 usc 回填用）：取**等于上游 `ftmm_symbol`** 的那条（上游行的身份标识）；都不相等（或上游 `ftmm_symbol` 为空）则**不写**该行并计入「歧义」，不做猜测性取第一条。背景：本地 70 组 `stock_id` 各有两条 usc（如 `12002553` → `CC.DASHTRY.FTUS` 与 `DASHTRY`），上游对应 `ftmm_symbol` 为 `CC.DASHTRY.FTUS`。| `SELECT count(*) FROM (SELECT stock_id FROM futu.finv_futu_quote_collect GROUP BY stock_id HAVING count(DISTINCT usc) > 1) t` = 70；上游回填运行 `ambiguous = 0` 且这 70 组上游 `usc` 均等于其 `ftmm_symbol`（如 `stock_id=12002553` → `CC.DASHTRY.FTUS`）；单测 `TestPickUSC` 覆盖「单条 / 多条命中 / 多条不命中 / 空输入」4 类分支 | √ |
| 11056 | **Tab 3「上游 usc 回填」同时回填 `public.finv_quote_secu_key` 的 `usc` 与 `stock_id`**（一次运行依次写两张上游表，阶段标识 `phase=meta/key`）：① 维持原口径 `public.finv_quote_secu_meta.usc`（按 `stock_id`）；② 新增 `public.finv_quote_secu_key`（上游主键 `ftmm_symbol`）的 `usc` + `stock_id` 两列，来源为本地 `futu.finv_futu_secu_key`（候选键集）⋈ `futu.finv_futu_quote_collect`（提供 `usc` + `stock_id`）；上游两列都为空 → 「填入」，任一已有且不同 → 「覆盖」，两列都已一致 → 「未变」不发请求（幂等）；本地无依据的键（无对应 `usc`/`stock_id`）在 `Status` 里给 `key_unresolved` 计数并跳过。接口与页面沿用 Tab 3 三支，`Status` 增加 `key_table`/`key_columns`/`key_key`/`key_local_rows`/`key_resolvable`/`key_unresolved`，运行快照增加 `phase` 与 `key_*` 一组计数，页面「回填进度与日志」增加键表 chips。| `GET .../UscBackfill/Status` 返回 `key_local_rows=73212`、`key_resolvable` > 0、`key_unresolved + key_resolvable = 73212`；运行完成后上游 `public.finv_quote_secu_key`：`usc` 非空行数 = `stock_id` 非空行数 = `key_fill + key_overwrite + key_unchanged`；抽样 `US.AAPL` 行 `usc=AAPL`、`stock_id=205189`，本地不存在对应品种的键（如 `AU..XFL`）仍保持 NULL；复跑 `key_fill=0 key_overwrite=0`，`key_unchanged` 等于可回填键数 | √ |
| 11057 | **键表回填的关联口径**：用本地 `market_label \|\| '.' \|\| futu_symbol` **重建** `ftmm_symbol` 与上游键表关联（上游 `ftmm_symbol = {market}.{code}`，本地把它拆成 `market_label` + `futu_symbol`），而**不是**只按 `stock_id` 关联。理由：按 `stock_id` 只能覆盖 10,910 行（本地键表 `stock_id` 仅由 Tab 2 从 meta 补到 1 万余行），按符号重建可覆盖 24,162 / 73,212 行，且按 `stock_id` 命中的集合是其**子集**（实测 `by_symbol=24162`、`union=24162`、`symbol_only=13252`）。| `SELECT count(DISTINCT k.ftmm_symbol) FROM futu.finv_futu_secu_key k JOIN futu.finv_futu_quote_collect q ON k.ftmm_symbol = q.market_label \|\| '.' \|\| q.futu_symbol` = 24162（与 `Status.key_resolvable` 一致）；同一表达式按 `stock_id` 关联的结果为 10910 且为其子集；运行后上游 `finv_quote_secu_key` 中 `usc`/`stock_id` 非空均为 24,162 行，`key_ambiguous=0`、`key_no_target=0` | √ |
| 11058 | **「证券参数同步」页面 Tab 2 名称由「证券指标数据同步」改为「FT证券指标数据同步」**（与 Tab 1「FT证券基础参数同步」命名一致；只改展示名称，接口路径、口径、行为均不变，三处外观同步：Tab 标签、日志/提示中的任务名（`同步任务已启动` 前的 label 与 Toast）、页面底部说明卡片的小标题）。| 浏览器打开 `/Meta/Finv/Quote/Realtime/SecuParamSync` 切到第二个 Tab：标签文字为「FT证券指标数据同步」；点「立即同步」后 Toast 文案为「FT证券指标数据同步完成：…」；说明卡片对应段落标题为「FT证券指标数据同步」；`grep -rn '证券指标数据同步' Web/src Docs internal \| grep -v 'FT证券指标数据同步' \| grep -v CHANGELOG` 无输出 | √ |
| 11059 | **「证券参数同步」页面 Tab 3 名称由「上游 usc 回填」改为「上游主键参数回填」**（该 Tab 实际回填的是上游两张表的**主键类参数**：`finv_quote_secu_meta.usc` 与 `finv_quote_secu_key.usc` + `stock_id`，不只是 `usc`，故按主键参数命名；只改展示名称与启动提示文案，接口路径 `.../SecuKey/UscBackfill/*`、请求/响应字段名、口径与行为均不变）。| 浏览器打开 `/Meta/Finv/Quote/Realtime/SecuParamSync` 切到第三个 Tab：标签文字为「上游主键参数回填」；点「立即回填」后 Toast 文案为「上游主键参数回填完成：…」，接口返回的 `message` 为「上游主键参数回填任务已启动」；说明卡片对应段落标题为「上游主键参数回填（方向相反：本地 → 上游，写两张表）」，且该段仍写明回填 `usc` 与 `stock_id` 两列；`grep -rn '上游 usc 回填' Web/src Docs internal \| grep -v CHANGELOG \| grep -v RequirementList` 无输出 | √ |
| 11060 | **`futu.finv_futu_quote_collect` 增加 `ftmm_symbol` 字段**（迁移 `V300147`）：上游键表/元数据表的主键口径是 `ftmm_symbol = {market}.{code}`，本表此前把它拆成 `market_label` + `futu_symbol`，跨表关联需现拼`market_label \|\| '.' \|\| futu_symbol`（易漏、走不上索引）。做法：**生成列** `ftmm_symbol text GENERATED ALWAYS AS (market_label \|\| '.' \|\| futu_symbol) STORED NOT NULL`，由 PG 自动维护、无需触发器与回填脚本，`ALTER` 时自动为存量行计算；另建普通索引 `idx_finv_futu_quote_collect_ftmm_symbol`。**该列非唯一**（`market_label` 与 `market_code` 非 1:1，实测 75 组、150 行重建后撞名，如 `CC.AAVEUSD` 同时来自 market_code 360 与 413），故不建唯一约束。写入约束：INSERT/UPDATE **不得显式赋值** `ftmm_symbol`（PG 会报 `cannot insert a non-DEFAULT value`）；仓库内所有写入（`V300063`/`V300131` 种子、指标同步 upsert、两个集成测试）均为显式列清单且不含该列，兼容。同时把 Tab 3 阶段 ② 的关联表达式由现拼改为直接引用该列。| `SELECT count(*) FROM information_schema.columns WHERE table_schema='futu' AND table_name='finv_futu_quote_collect'` = **15**（原 14）；`SELECT attgenerated FROM pg_attribute WHERE attrelid='futu.finv_futu_quote_collect'::regclass AND attname='ftmm_symbol'` = `s`；`SELECT count(*) FROM futu.finv_futu_quote_collect WHERE ftmm_symbol <> market_label \|\| '.' \|\| futu_symbol` = 0，且 `count(ftmm_symbol)` = `count(*)` = 24,418；`SELECT count(*) FROM (SELECT ftmm_symbol FROM futu.finv_futu_quote_collect GROUP BY 1 HAVING count(*)>1) t` = 75（证明非唯一）；`to_regclass('futu.idx_finv_futu_quote_collect_ftmm_symbol')` 非空；抽查 `usc='AAPL'` → `US.AAPL`、`usc='00700'` → `HK.00700`；迁移重跑幂等（`column already exists, skipping` + NOTICE 24418 行）；应用启动后 `schema_version` = 300147 | √ |
| 11061 | **`cdeu.finv_quote_secu_indicator` 补充 `stock_id` 字段并在 Tab 2 同步中写入**（迁移 `V300148`）：上游 `public.finv_quote_secu_meta` 已带 `stock_id`（bigint、全部非空），本地指标表建表时把它排除在 14 个身份列之外。现补列：`stock_id bigint`、**可空**（NULL = 尚未同步到，无占位口径）、带中文注释与索引 `idx_finv_quote_secu_indicator_stock_id`；迁移内先按本地已有映射回填（① `futu.finv_futu_secu_key` 按 `ftmm_symbol`；② `futu.finv_futu_quote_collect` 按其生成列 `ftmm_symbol`，同名多行取 `min(stock_id)`），均带 `IS DISTINCT FROM` 守卫，重复执行为 no-op。同步侧：`internal/meta/finv/quote/indicator/sql.go` 由 131 列改为 **132 列同名同步**，`stock_id` 进入 INSERT 列表、SELECT 列表、SET 列表与「变了才更新」的比对元组（`dt_update` 仍不参与比对）。| 迁移后 `SELECT count(*) FROM information_schema.columns WHERE table_schema='cdeu' AND table_name='finv_quote_secu_indicator'` = **132**；`SELECT data_type, is_nullable FROM information_schema.columns WHERE ... AND column_name='stock_id'` = `bigint` / `YES`；`grep -c 'stock_id' internal/meta/finv/quote/indicator/sql.go` ≥ 4 且 `indicatorColumns` 列数 = 132；实测迁移回填 11,339 / 11,339 行；跑一次 Tab 2 后 `SELECT count(*) - count(stock_id) FROM cdeu.finv_quote_secu_indicator` = **0**，且 `SELECT count(*) FROM cdeu.finv_quote_secu_indicator i JOIN futu.finv_futu_secu_key k ON k.ftmm_symbol=i.ftmm_symbol WHERE k.stock_id IS NOT NULL AND i.stock_id IS DISTINCT FROM k.stock_id` = 0；复跑指标同步 `ind_update=0`（stock_id 不漂移）；`schema_version` = 300148；`go build`/`go vet`/`go test ./...` 通过 | √ |
| 11062 | **Tab 1「FT证券基础参数同步」把 `stock_id`、`usc` 也纳入同步**：上游 `public.finv_quote_secu_key` 已补齐这两列，它们不再是本地专有列。同步列由 7 列扩为 9 列：`instrument` / `market` / `code` / `flag_enable`（源 `enable`）/ `priority` / `status` / `url` / **`stock_id`** / **`usc`**；拉取列（`sourceSelect`）、`SourceRow` / `KeyRow`、`jsonb_to_recordset` 类型、INSERT 列清单、SET 列表与「变了才更新」比对元组同步扩展。取值语义：`url` / `stock_id` / `usc` 三者均为 **`COALESCE(源, 本地)`**——上游该列为 NULL 时保留本地值、不清空（上游 73,212 行中 `stock_id` 仅约 1.6 万行非空，直接覆盖会清掉本地 2.2 万行）。| 同步前 `SELECT count(*), count(url), count(stock_id), count(usc) FROM futu.finv_futu_secu_key` 记快照，跑一次后：总行数不变（73,212）、`count(url)` 不减少（16,591 → 16,591）、`count(stock_id)` 不减少（22,393 → 22,437）、`count(usc)` 由 16,712 增至 73,212（上游 `usc` 已全量非空）；复跑 `insert=0`、`update` 仅为上游后续变化的个位数、`unchanged≈73,212`；`grep -c 'stock_id,usc' internal/meta/finv/quote/secukey/client.go` = 1；单测 `go test ./internal/meta/finv/quote/secukey/` 通过 | √ |
| 11063 | **Tab 2「FT证券指标数据同步」改为「一次写两张表」并调整三处口径**：① 指标表 `cdeu.finv_quote_secu_indicator` 按 `usc` 主键 upsert 增量同步 132 列（`dt_update` 由触发器维护、不参与比对）；② 采集属性表 `futu.finv_futu_quote_collect` 按 `usc` 主键 upsert 增量同步身份字段（`stock_id`/`market_type`/`market_code`/`instrument_type`/`sub_instrument_type`/`market_label`/`instrument`），**`futu_symbol` 改由上游 `stock_code`（缺失时用 `ftmm_symbol`）去掉市场前缀派生**，**`flag_enable` 不写**（新增行取表默认 `'0'`、已有行保持原值）——**取代需求 11051 的「按占位推导并覆盖」口径**；③ **不再同步 `futu.finv_futu_secu_key`**（上游已支持 `url` ← `secu_url`、`stock_id` 的回填，本地该表改由 Tab 1（上游→本地）与 Tab 3（本地→上游）维护）：SQL 去掉 `upd_key` 段与 `key_update` 返回值，`UpsertStat`/`Progress`/`Result`/`Run` 与前端 chips 一并移除该计数（接口 yml 里 `key_update` 标记 deprecated 以兼容旧前端）。保留项：`usc` 为空时用 `ftmm_symbol` 占位；唯一约束 `(market_code, futu_symbol)` 撞车时跳过并计入 `qc_skip`。| 跑一次指标同步（`batch_size=3000`）：日志无「键表补全」，`Run` 快照无 `key_update` 字段；`futu.finv_futu_secu_key` 在同步前后完全不变（`count(*)`=73,212、`count(url)`、`count(stock_id)` 三项一致）；`futu.finv_futu_quote_collect` 的 `flag_enable='1'` 行数同步前后不变（24,610 → 24,610），`'0'` 行数只随新增行增加（456 → 659，新增 203 行取默认值）；`SELECT count(*) FROM futu.finv_futu_quote_collect WHERE futu_symbol <> substring(ftmm_symbol FROM strpos(ftmm_symbol,'.')+1)` = 0（`stock_code` 派生与旧口径结果一致）；复跑无批量更新（仅上游新增/价格变化）；`go build`/`go vet`/`npm run build` 通过 | √ |
| 11064 | **Tab 2 的「本地目标表」卡片只展示本 Tab 真正写入的两张表**：去掉 `futu.finv_futu_secu_key` 的 chip 与行数（该表已不在本 Tab 同步，列在目标表里会误导）；卡片标题由「本地目标表（一次同步写三张）」改为「一次同步写两张」；`GET .../SecuKey/Indicator/Status` 的 `targets[]` 由 3 项降为 2 项（指标表 / 采集属性表）；页面底部「说明」卡片里原本的「③ `futu.finv_futu_secu_key`：不再同步……」一行也一并删除（不在本 Tab 提及不做的事）。| `GET .../SecuKey/Indicator/Status` 返回 `len(targets)==2`，且 `targets[].table` 只有 `cdeu.finv_quote_secu_indicator` 与 `futu.finv_futu_quote_collect`；浏览器 Tab 2 第一张卡片标题为「本地目标表（一次同步写两张）」，只有 2 个 chip、无 `futu.finv_futu_secu_key`；前端产物 `grep -c 'finv_futu_secu_key'` 在 Tab 2 段落内为 0；`go build` / `go vet` / `npm run build` 通过 | √ |
| 11065 | **BRS 全量备份登记补录第一批（17 张表）**：把「最近迁移新建/改名」与「quant 体积可控」的业务表登记进 `brs.brs_bak_rdb_table_full`，纳入每日全量备份。① 最近 8 张：`cdeu.finv_quote_secu_indicator`、`futu.finv_futu_secu_key`、`cdeu.finv_cdeu_secu_realtime_stock`、`cdeu.finv_cdeu_secu_realtime_sync_config`、`cdeu.finv_quote_secu_kline_day`、`cdeu.finv_quote_cdeu_usc_patch_rule`、`public.finv_fxcer`、`public.finv_cer`；② quant 9 张：`finv_quant_backtest_account`、`_del_log`、`_del_task`、`_run`、`_run_archive`、`_strategy`、`_trade`、`finv_quant_environment`、`finv_quant_template`。口径：`bak_scope='both'`、`bak_method='pg_dump'`、`compression='zstd'`、`flag_upload='1'`、`creator='migration'`；priority 取 690 以下步长 10 的空档并**跳过 660**（该值曾被 V300086 条件占用，避免 priority 唯一键冲突导致静默跳过）；每行带 `to_regclass` 守卫，缺表不登记（`public.finv_cer` 的建表脚本 V300123 已删除，新环境无此表）；迁移 `V300149` 幂等（`ON CONFLICT DO NOTHING`）+ 末尾收敛校验。| `SELECT count(*) FROM brs.brs_bak_rdb_table_full` = 41（原 24 + 17）；`priority<=680` 的 17 行逐条存在且 `flag_enable='1'`；迁移复跑 `INSERT 0 0`、`schema_version=300149`；触发一次手动全量备份 `POST .../BRS/Backup/Finv/Run/Manual` → `status=succeeded、total=41、done=41、failed=0`，产物目录下 17 张新表的 `.dump` 均生成，`pg_restore -l` 能看到 `TABLE` / `TABLE DATA` 条目 | √ |
| 11066 | **BRS 全量备份登记补录第二批（cdeu 4 张）**：在 `brs.brs_bak_rdb_table_full` 追加 `cdeu.finv_quote_cdeu_export_batch`、`cdeu.finv_quote_cdeu_export_job`、`cdeu.finv_quote_cdeu_usc_day_incr_row_tpl`、`cdeu.finv_quote_cdeu_ingest_stats` 共 4 张（迁移 `V300150`）。cdeu 下其余 15 张历史遗漏表经确认**暂不备份**，保持不登记（不写 `flag_enable='0'` 占位行）。priority 从 490 往下取 10 的整数（490/480/470/460，避开第一批占用的 510～680）；口径同第一批（`both` + `pg_dump` + `zstd` + 上传 `'1'` + `to_regclass` 守卫 + `ON CONFLICT DO NOTHING` + 收敛校验）。| `SELECT count(*) FROM brs.brs_bak_rdb_table_full` = 45（41 + 4）；四条登记行的 `priority` 分别为 490/480/470/460 且 `flag_enable='1'`；迁移复跑 `INSERT 0 0`、`schema_version=300150`；手动全量备份 `status=succeeded、total=45、done=45、failed=0`，产物目录 45 个 `.dump` 含这 4 张、且不含 `cdeu.finv_quote_cdeu_job_run_log_line` 等未登记表 | √ |
| 11067 | **撤销 `public.finv_cer` 的 BRS 全量备份登记**：该表是缺省汇率 v1，已被 `public.finv_fxcer`（V300124）取代，建表脚本 V300123 已按需求 11021 从版本库删除，Go/前端 0 引用、无写入路径；迁移 `V300151` 只删 `brs.brs_bak_rdb_table_full` 中 `public.finv_cer` 这一行（`db='finv'`），**不动该表本身及其 280 行数据**，其余 44 条登记（含 `futu.finv_futu_mapping_*` 5 张）全部保留；priority 600 随该行释放。| `SELECT count(*) FROM brs.brs_bak_rdb_table_full` = 44；`SELECT count(*) ... WHERE table_name='finv_cer'` = 0；`SELECT count(*) FROM public.finv_cer` 仍为 280（表与数据未被删）；迁移复跑 `DELETE 0` 不报错、`schema_version=300151`；手动全量备份 `total=44 / done=44 / failed=0`，产物目录无 `finv.public.finv_cer.dump`、仍有 `finv.public.finv_fxcer.dump` | √ |
| 11068 | **把 brs schema 自身的 3 张表登记进 BRS 全量备份清单**：`brs.brs_bak_rdb_table_full`（priority 600）、`brs.brs_bak_rdb_table_incr`（450）、`brs.brs_bak_rdb_datasource`（440），迁移 `V300152`。原因：备份系统自身的登记表此前不在备份范围，产物里没有任何一张表说明「哪些表该备份、怎么切分、备份源是什么」，恢复时只能人工回忆。cron 与对象存储配置存于 `sys.sys_config`（`39001` / `39002`，已登记 990），不重复登记。口径同前批（`both` + `pg_dump` + `zstd` + 上传 `'1'` + `to_regclass` 守卫 + `ON CONFLICT DO NOTHING` + 收敛校验）。| `SELECT count(*) FROM brs.brs_bak_rdb_table_full` = 47（44 + 3）；三条登记行 priority 分别为 600/450/440 且 `flag_enable='1'`；迁移复跑 `INSERT 0 0`、`schema_version=300152`；手动全量备份 `status=succeeded、total=47、done=47、failed=0`，产物含 `finv.brs.brs_bak_rdb_table_full.dump` / `_table_incr.dump` / `_datasource.dump`，`pg_restore -l` 对 `brs_bak_rdb_table_full.dump` 能看到 `TABLE DATA` 条目 | √ |
| 11069 | **按 `FinvQuantDesign/Mapping/finv_futu_query_stat.json` 补录证券初始数据**（迁移 `V300153`，480 行来源）：① `public.finv_security` 补 **480 行**（来源里 480 个 `usc` 在补录前一个都没有）；② `futu.finv_futu_quote_collect` 补 **252 行**（另 6 个 `usc` 早已存在：AENT / BUFC / SH.689009 / TAXX / UL / ULBI；另 222 个已按「占位 usc = `{market_label}.{futu_symbol}`」存在、被唯一约束 `(market_code, futu_symbol)` 挡住，本迁移不插入也不改动）。字段口径：以 `public.finv_quote_secu_meta` 补齐 `market_code` / `instrument` / `sub_instrument_type` / `name_en`（按 `usc` 命中 226 行），未命中的 254 行按本地既有主导写法派生（`exchange_code` HK 21 / SZ 12 / SH 11 / BJ 13 / US 30；`finv_security.market_code` HK 1201（GEM 1202、人民币柜台 1205）/ SZ 1120（30xxxx 1121）/ SH 1110（68xxxx 1112）/ BJ 1130 / US 1315；`security_type` 3→Stock、4→ETF、12→Stock；`type_secu_sub={security_type}.{market_label}`；富途 `market_code` HK 1（08xxxx GEM 2）/ SZ 31（30xxxx 35）/ SH 30（68xxxx 32）/ BJ 38 / US 11；`futu_symbol=stock_code`、`ftmm_symbol` 为生成列不写入；`flag_enable='1'`；`remark='finv_futu_query_stat'` 标来源）；幂等 `ON CONFLICT DO NOTHING`。| `SELECT count(*) FROM public.finv_security WHERE remark='finv_futu_query_stat'` = 480；`SELECT count(*) FROM futu.finv_futu_quote_collect WHERE remark='finv_futu_query_stat'` = 252；总行数 21,223 → 21,703、25,401 → 25,653；迁移随应用启动应用、`schema_version=300153`；复跑不新增行；抽样 `usc='00068'` → HK / exchange 21 / market 1201 / Stock / HKD，`usc='001246'` → SZ / 12 / 1120 / Stock / CNY | √ |
| 11070 | **按 `FinvQuantDesign/Mapping/finv_futu_query_stat-2.json` 补录 `public.finv_security` 初始数据**（迁移 `V300154`）：来源 489 行（结构同富途采集属性表，**不含证券名称与币种**），其中 210 个 `usc` 已存在；剩余 279 个里 11 个与既有登记仅大小写不同（HK 指数 `800000`→`HSI`、`800700`→`HSTI`，HK 期货主连 9 条 `CNCMain`/`GDRCurrent`/`GDRMain`/`GDRNext`/`GDUMain`/`HSIMain`/`HTIMain`/`METMain`/`MIUMain`）→ 跳过不重复登记；**实际补录 268 行**。名称/英文名由上游 `public.finv_quote_secu_meta` 补齐（按 `usc` 命中 84 行 + 按 `ftmm_symbol` 补齐共 480 行，本批插入行全部有名称）；口径：`security_type` 3→Stock、4→ETF、6→StockIndex、9→Futures；`market_code` HK 股票/ETF 1201（08xxxx 1202）、HK 期货 1250、SZ 1120（30xxxx 1121）；`type_secu_sub` = `{security_type}.{market_label}`（Futures 留空）；`security_code`=来源 `futu_symbol`；币种 HK HKD / SZ·SH·BJ CNY / US USD；`init_date` 20000000、`provider` 'FTMM'、`flag_enable` '1'、`remark` 'finv_futu_query_stat-2'；幂等 `ON CONFLICT DO NOTHING`。| `SELECT count(*) FROM public.finv_security WHERE remark='finv_futu_query_stat-2'` = 268（Futures 183 / ETF 67 / Stock 18）；总行数 21,703 → 21,971；`schema_version=300154`；名称/中文名空值 0 行；抽样 `A50Main` → HK / 1250 / Futures / HKD / Asia/Hong_Kong、`03001` → HK / 1201 / ETF / ETF.HK、`002710` → Stock；来源里 11 个大小写不同的既有证券未被重复插入 | √ |
| 11071 | **`HK.800700`（恒生科技指数）的 usc 统一为 `HTI`**（迁移 `V300155` + 修正 `V300156`）：该指数此前存在双写法与重复行 —— `public.finv_security` 有 `usc='HSTI'`（`security_code=800700`）与 `usc='HTI'`（`security_code='HTI'` 空壳行）两行、`cdeu.finv_quote_secu_indicator` 有 `usc='800700'` 与 `usc='HTI'` 两行同 `ftmm_symbol='HK.800700'`；上游 `public.finv_quote_secu_meta` / `public.finv_quote_secu_key` 均为 `usc='HTI'`。处理：删前按 DbSchemaSpec §0.7 整行备份到 `archive.finv_security_purge_20261110` / `archive.finv_quote_secu_indicator_purge_20261110`（带 `src_table`/`reason`/`backup_at`），再删两张表的重复行、把 `HSTI` 改为 `HTI`。**顺带修正**：首版 `V300155` 误把 `futu.finv_futu_secu_key` 中 `US.HSTI`（美股，code=`HSTI`）的 `usc` 改成 `HTI`，`V300156` 已恢复为 `HSTI`，`V300155` 同时收口为「只改 `market='HK'`」。`SH` 指数 149 行裸 code 按用户要求**不动**（生产数据双份，下月自然收敛）。| `SELECT count(*) FROM public.finv_security WHERE usc='HTI' AND security_code='800700'` = 1，`usc='HSTI'` = 0；`SELECT count(*) FROM cdeu.finv_quote_secu_indicator WHERE ftmm_symbol='HK.800700' AND usc='HTI'` = 1、`usc='800700'` = 0；`futu.finv_futu_secu_key` 中 `HK.800700→HTI`、`US.HSTI→HSTI` 各 1 行且总行数仍 73,212；两张 `archive.*_purge_20261110` 表分别留档 2 行 / 1 行；两迁移幂等复跑零改动、`schema_version` 到 300156 | √ |
| 11167 | 外部 OpenAPI（`/OpenAPI/V1`）鉴权建表：新建 schema `open`（迁移 `V100013`）与 3 张表（迁移 `V300183`），表名一律 `open_` 前缀 —— `open.open_api_client`（调用方）、`open.open_api_credential`（凭证，`auth_mode` 支持 `API_KEY`/`AK_SK`/`BEARER`/`MTLS`/`BASIC`，静态 Key 存 `secret_hash`、SK 存 `secret_cipher` 密文，两者 CHECK 互斥；`secret_cipher_prev`+`dt_secret_rotate` 支持轮换；吊销必须留 `dt_revoke`+`reason_revoke`）、`open.open_api_invoke_log`（调用审计，原名 `open_api_call_log`、同批定稿为 `invoke_log` 见 11170；只增不改，只存 `body_sha256` 不存原文）；nonce 存 Redis 不建表；配套 `sys_config` 19001～19006（鉴权模式开关/签名时间窗/日志保留天数/密钥 KID/密钥本体/兼容兜底密钥，后两项敏感且不预填）；`search_path` 末尾追加 `open` | `SELECT count(*) FROM information_schema.tables WHERE table_schema='open'` = 3；`SELECT count(*) FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace WHERE n.nspname='open' AND c.contype='f'` = 0（零物理外键）；`SELECT count(*) FROM sys.sys_config WHERE config_no BETWEEN 19001 AND 19006` = 6 且 `config_no IN (19005,19006)` 的 `str_value` 均为空串；`go test -p 1 ./internal/database -run TestMigrateIntegration` 通过（全新库全链 + 幂等复跑）；`go test ./internal/docs` 通过 | |
| 11168 | 外部 OpenAPI 鉴权中间件（`internal/api/openapi`）：`/OpenAPI/V1` 路由组挂 `Middleware.Handler`，先于 `WrapOpenAPIResponse`；支持模式一静态 API Key（`X-API-Key` / `Authorization: Bearer`，库内只存 sha256 哈希）与模式二 AK/SK 签名（`Authorization: FinvAK <ak>:<sig>` + `X-Finv-Timestamp` + `X-Finv-Nonce`，待签串 = `METHOD\nPATH\nSORTED_QUERY\nBODY_SHA256\nTIMESTAMP\nNONCE`）；校验顺序为 「凭证定位 → 解密 SK → 状态/时效 → IP 白名单 → 时间窗 → nonce（Redis SET NX PX）→ 签名 → 授权范围 → 限流/配额」，失败返回 `190xx`；`sys_config` 19001 控制 `0` 关闭 / `1` 启用 / `2` 宽松（校验并写审计但仍放行）；环境变量 `FINV_OPENAPI_KEY` 与 `sys_config` 19006 作为兼容兜底静态密钥；每次校验（含失败）同步写 `open.open_api_invoke_log`；请求体上限 64 MiB；原 `RequireOpenAPIKey` 过渡实现已删除 | `go test ./internal/api/openapi` 通过（错误码号段/HTTP 映射契约、待签串与验签、AES-GCM 往返、scope、静态 Key、AK/SK、超窗、宽松模式、兜底密钥、密钥未配置 19010）；`FINV_OPENAPI_LIVE=1` 的端到端测试通过（真服务：无凭证 401/19001、错 Key 401/19002、静态 Key 200、AK/SK 200、nonce 重放 401/19007、吊销后 403/19003、审计日志 pass/deny 均有、模式 0 无凭证 200） | √ |
| 11169 | 新增菜单「OpenAPI 调用方管理」（`/Meta/Finv/OpenAPI/Client`，key `openapi-client-manage`，二级叶子，位于「系统配置」之前）：三个 Tab 一一对应三张表 —— ① 调用方 `open.open_api_client`（分页/新增/编辑/启停，新增默认停用）；② 凭证 `open.open_api_credential`（分页/签发/启停/吊销/轮换，**密钥明文只在签发与轮换的响应里出现一次**，列表只回 `has_secret`）；③ 调用审计 `open.open_api_invoke_log`（结果/原因/公开标识筛选 + 分页）。顶部可读写鉴权配置 `sys_config` 19001～19003，并以只读 chip 显示 19004 KID 与 19005/19006 是否已配置（不回显密钥）。后端 11 个内部接口（`/API/V1/Meta/Finv/OpenAPI/*`）：`Settings`(GET/PUT)、`Client/Page`、`Client/Save`、`Client/Enable`、`Credential/Page`、`Credential/Create`、`Credential/Enable`、`Credential/Revoke`、`Credential/Rotate`、`InvokeLog/Page`；吊销原因必填、已吊销不可再启用/吊销(409)、19005 未配置时签发 AK/SK 返回 503 | 浏览器打开 `/Meta/Finv/OpenAPI/Client`：三个 Tab 存在且标题为「调用方 / 凭证 / 调用审计」；新增调用方后列表状态为「停用」，点「启用」后变「启用」；签发静态 Key 后弹窗显示密钥与「复制」按钮，关闭后再查看列表只能看到 `key_prefix…`、无完整密钥；轮换后 `access_key_id` 变化、`dt_secret_rotate` 有值；吊销必须填原因（空原因按钮禁用），吊销后该行操作列只剩「已吊销，只能新建」；`go test ./internal/meta/finv/openapi` 与 `go test ./internal/api/...` 通过；`npx vue-tsc -b --force` 与 `npm run build` 通过；`SELECT count(*) FROM open.open_api_credential WHERE secret_hash IS NULL AND secret_cipher IS NULL` = 0 | √ |
| 11170 | `open.open_api_call_log` 改名为 `open.open_api_invoke_log`（迁移 `V300184`）：表名、主键约束 `pk_open_api_invoke_log`、4 个 CHECK 约束、3 个索引名一并由 `*_call_log*` 改为 `*_invoke_log*`，列/数据/注释不动；`V300183` 同批回改为最终表名（尚未推送任何环境），`V300184` 负责把已应用旧版的库就地改名；只用「旧表存在且新表不存在」时才改名，两表并存只 `RAISE NOTICE` 交人工，重跑 0 变更 | `SELECT to_regclass('open.open_api_invoke_log')` 非空且 `SELECT to_regclass('open.open_api_call_log')` 为空；`SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='open' AND c.relname LIKE '%\_call\_log%'` = 0（索引/表名无残留）；`SELECT count(*) FROM pg_constraint c JOIN pg_class r ON r.oid=c.conrelid JOIN pg_namespace n ON n.oid=r.relnamespace WHERE n.nspname='open' AND c.conname LIKE '%\_call\_log%'` = 0；`\d open.open_api_invoke_log` 的主键/CHECK/索引名均为 `*_invoke_log*`；`go test -p 1 ./internal/database -run TestMigrateIntegration` 通过（全新库重建的是 `invoke_log`，`call_log` 为空） | |
| 11171 | 菜单 `/Meta/Finv/OpenAPI/Client`（OpenAPI 调用方管理）由**二级叶子**改为**四级叶子**：新增二级分组「系统管理」（key `sys-manage`，`mdi-shield-lock-outline`，本身不可点）与三级分组「开放接口」（key `sys-manage-openapi`，`mdi-api`），叶子挂在三级分组下；key `openapi-client-manage`、路由 `/Meta/Finv/OpenAPI/Client`、页面组件、接口路径全部不变（历史深链接继续可用）。约束：二级菜单不再直接挂业务叶子 | 浏览器展开侧边菜单：「系统管理 → 开放接口 → OpenAPI 调用方管理」三级链路存在，点击叶子进入 `/Meta/Finv/OpenAPI/Client` 且页面标题为「OpenAPI 调用方管理」；`grep -c "key: 'openapi-client-manage'" Web/src/config/menuConfig.ts` = 1；构建产物中「系统管理」「开放接口」「OpenAPI 调用方管理」三个标题依次出现（字节位置递增）；`npx vue-tsc -b --force` 与 `npm run build` 通过；`go test ./internal/docs` 通过 | √ |
| 11172 | 菜单二级分组「系统管理」改名为「**开放平台**」（key `sys-manage`、路由前缀、三级分组「开放接口」、叶子与接口路径全部不变；只改 `menuConfig.ts` 的 `title`），并同步注释中的分组定位说明 | 浏览器展开侧边菜单二级标题显示「开放平台」，其下仍是「开放接口 → OpenAPI 调用方管理」；`grep -c "key: 'sys-manage'" Web/src/config/menuConfig.ts` = 1 且 `grep -c "title: '开放平台'"` = 1；`grep -rn '系统管理' Web/src Docs \| grep -v CHANGELOG \| grep -v RequirementList` 无输出；`npx vue-tsc -b --force` 与 `npm run build` 通过 | √ |
| 11173 | 「开放平台」下按功能拆分菜单，不再把调用方/凭证/审计/配置塞在一个页面：新建三个三级分组 —— **接入管理**（`openapi-access`：OpenAPI 调用方 `openapi-client` / OpenAPI 凭证 `openapi-credential`）、**运行监控**（`openapi-monitor`：OpenAPI 调用审计 `openapi-invoke-log`）、**安全设置**（`openapi-security`：OpenAPI 鉴权配置 `openapi-settings`）；原单页 `ClientManageView.vue`（三 Tab + 顶部配置）拆成 4 个视图，每页只写一张表、只放一块说明卡片；路由分别为 `/Meta/Finv/OpenAPI/{Client,Credential,InvokeLog,Settings}`，共用枚举与展示辅助抽到 `OpenAPI/shared.ts`；原 `/Meta/Finv/OpenAPI/Client` 路径保留（仍指向调用方页，深链接不断） | 侧边菜单展开「开放平台」：三个三级分组各含对应叶子（共 4 个）；逐个点击检查 URL 与页面卡片标题一致；`ls Web/src/views/Meta/Finv/OpenAPI/` 含 `ClientView.vue`/`CredentialView.vue`/`InvokeLogView.vue`/`SettingsView.vue`/`shared.ts`，且 `ClientManageView.vue` 不存在；`grep -rn 'ClientManageView' Web/src` 无输出；`npx vue-tsc -b --force` 与 `npm run build` 通过；产物中「开放平台→接入管理→OpenAPI 调用方→OpenAPI 凭证→运行监控→OpenAPI 调用审计→安全设置→OpenAPI 鉴权配置」八个标题字节位置递增；`go test ./internal/docs` 通过 | √ |
| 11174 | 「OpenAPI 调用方」新增对话框补默认值与到期下拉：① 负责人账号默认当前登录账号（只读展示，服务端 `owner_uid` 为空时同样用它兜底）；② 联系邮箱默认取当前登录账号的资料邮箱（前端 `GET /Auth/Profile` 预填，服务端 `contact_email` 为空时按 `owner_uid` 查 `sys.sys_user.email` 兜底）；③ 授权到期改为下拉预设「长期 / 1天 / 7天 / 15天 / 1个月 / 3个月 / 半年 / 1年 / 18个月 / 3年」（编辑时多一项「保持当前到期日」），选中后自动换算成具体日期并显示「到期日：YYYY-MM-DD」，提交的仍是绝对时间；月份加法溢出回退到当月最后一天 | 打开「开放平台 → 接入管理 → OpenAPI 调用方」点「新增调用方」：负责人账号已填当前登录账号且不可编辑、联系邮箱已预填登录用户邮箱；到期下拉含 10 个预设（编辑时 11 个），选「1 个月」后下方提示显示对应日期；保存后列表「到期」列与该日期一致，`SELECT dt_expire FROM open.open_api_client WHERE idn=?` 为对应日志时间；`FINV_OPENAPI_ADMIN_LIVE=1 go test ./internal/meta/finv/openapi -run TestLiveAdminClientDefaults` 通过；`npx vue-tsc -b --force` 与 `npm run build` 通过 | √ |
| 11175 | 新增菜单「OpenAPI 密钥配置」（`/Meta/Finv/OpenAPI/Secret`，key `openapi-secret`，开放平台 → 安全设置 下的四级叶子）：解决 19005（32 字节 AES-256 Base64）无法手填的问题 ——页面可一键用浏览器 `crypto.getRandomValues` 生成 32 字节密钥并复制备份；可改 19004 KID；可配/清空 19006 兜底密钥；换密钥时勾选「重加密现有凭证」会在**同一事务**内用旧密钥解、新密钥加密并更新 `secret_kid`；解不开的历史密文跳过并计数提示。后端新增 `GET /Meta/Finv/OpenAPI/Secret/Status` 与 `POST /Meta/Finv/OpenAPI/Secret/Save`（空值表示不改，清空兜底密钥用独立开关；密钥只进不出）；「OpenAPI 鉴权配置」页的提示改为引导到本页 | 打开「开放平台 → 安全设置 → OpenAPI 密钥配置」：未配置时顶部黄色提示；点「生成 32 字节密钥」后输入框出现 44 字符 Base64；保存后 chips 显示「19005 已配置」，「OpenAPI 凭证」页可签发 AK/SK（此前 503）；把 KID 改为 `k2` 并勾选重加密保存后：`SELECT secret_kid FROM open.open_api_credential WHERE secret_cipher IS NOT NULL` 全为 `k2`，且该凭证的 AK/SK 请求仍能通过鉴权；不勾选重加密时前端二次确认并警告 `19010`；`curl -X POST .../OpenAPI/Secret/Save -d '{"secret_key_base64":"c2hvcnQ="}'` 返回 400；响应体 `grep -c 'secret_key_base64'` = 0（不回显）；`FINV_OPENAPI_ADMIN_LIVE=1 go test ./internal/meta/finv/openapi -run TestLiveAdminSecretRotation` 通过 | √ |
| 11176 | 凭证号 / 静态 API Key / AK / SK 中的英文字母一律大写：生成侧前缀改为 `FK_LIVE_` / `AK_LIVE_` / `SK_LIVE_`、随机十六进制部分转大写；凭证号入参 `ToUpper` 后入库（正则同时收紧为 `^[A-Z0-9][A-Z0-9._-]{1,47}$`），前端输入框实时转大写、自动生成的凭证号也大写；校验侧按大写归一后查库与比对哈希（`secret.NormalizeKey` + `MatchKey` 先大写再原样兜底，兼容改口径前签发的小写密钥） | 新增凭证后 `SELECT credential_no, access_key_id FROM open.open_api_credential` 两列均无小写字母；签发的静态 Key 匹配 `^FK_LIVE_[0-9A-F]{32}$`、AK 匹配 `^AK_LIVE_[0-9A-F]{32}$`、SK 匹配 `^SK_LIVE_[0-9A-F]{48}$`；用全小写的凭证号调用签发接口，落库仍为大写；用全小写静态 Key 发请求仍能通过（服务端归一）；`go test ./internal/meta/finv/openapi/... ./internal/api/openapi/` 通过（含 `TestIssuedKeyUppercase`）；`npx vue-tsc -b --force` 与 `npm run build` 通过 | √ |


## 汇总

| 分类 | 编号区间 | 条目数 |
|---|---|---|
| 变更日志文档 | 10001–10015 | 15 |
| 开发规范条款 | 10016–10034 | 19 |
| 跨平台换行与仓库卫生 | 10035–10050 | 16 |
| 需求清单自身 | 10051–10055 | 5 |
| 改造计划文档 | 10056–10065 | 10 |
| 改造项 A 验收 | 10066–10079 | 14 |
| 改造项 B 验收 | 10080–10091 | 12 |
| 改造项 C 验收 | 10092–10107 | 16 |
| 改造项 D 验收 | 10108–10120 | 13 |
| SQL schema 限定防回归 | 10121–10124 | 4 |
| 部署目录结构与 Native 说明 | 10125–10131 | 7 |
| Native 部署配套与生成物命名 | 10132–10140 | 9 |
| Deploy 目录总览文档 | 10141–10144 | 4 |
| Archive 归档目录 | 10145–10148 | 4 |
| 数据库名与角色名统一为 finv | 10149–10158 | 10 |
| 新增 export / sche 两个 schema | 10159–10166 | 8 |
| export 行情导出转存表 | 10167–10175 | 9 |
| 新增 dedup 去重 schema | 10176–10183 | 8 |
| 新增 archive 归档 schema | 10184–10191 | 8 |
| feat/integration 快照构建 | 10192–10202 | 11 |
| 本地快照打包脚本 | 10203–10215 | 13 |
| 打包脚本文档 | 10216–10222 | 7 |
| 产物命名规范调整 | 10223–10233 | 11 |
| VeritasQuant 归档 | 10234–10240 | 7 |
| 分钟行情 K 线 API 融合 | 10241–10263 | 23 |
| 涨跌字段补全规则 | 10264–10269 | 6 |
| 去重行情前端菜单与统计 | 10270–10275 | 6 |
| 数据采集菜单层级调整 | 10276 | 1 |
| 数字展示格式 | 10277 | 1 |
| 前端默认查询与时间展示 | 10278–10280 | 3 |
| 分钟行情日期索引 | 10281 | 1 |
| 统计页布局与坐标轴格式 | 10282 | 1 |
| 侧边导航缩进优化 | 10283 | 1 |
| 统计图表拆分对齐 | 10284 | 1 |
| 侧边导航缩进再减半 | 10285 | 1 |
| 菜单图标标题间距 | 10286 | 1 |
| 前端入口缓存与内嵌产物 | 10287 | 1 |
| 每日成交额图表 | 10288 | 1 |
| 日期边界接口 | 10289 | 1 |
| 数据采集菜单位置 | 10290 | 1 |
| 行情月归档导出与上传 | 10291–10310 | 20 |
| 三种发布包的 CI/CD | 10311–10320 | 10 |
| 前端月归档导出控制台 | 10321–10330 | 10 |
| 归档运维台（全表查询与管理员编辑） | 10331–10354 | 24 |
| 构建与工程化修复 | 10355 | 1 |
| 依赖安全升级 | 10356 | 1 |
| 归档 Job 执行可视化与全链路流程图 | 10357–10474 | 118 |
| 新增 sys 系统域 schema | 10475–10482 | 8 |
| sys 系统配置表 | 10483–10490 | 8 |
| sys 数据字典表 | 10491–10501 | 11 |
| sys 系统用户表 | 10502–10512 | 11 |
| sys_config 初始配置数据 | 10513–10518 | 6 |
| sys_dict 初始字典定义 | 10519–10524 | 6 |
| sys_dict_sub 初始字典子项 | 10525–10531 | 7 |
| sys_dict_sub 地区/国籍子项 | 10532–10538 | 7 |
| sys_dict_sub 通用标志/分页/数据类型子项 | 10539–10544 | 6 |
| sys_dict_sub 国籍增量补齐 | 10545–10550 | 6 |
| sys_config 运行/收集模式配置 | 10551–10556 | 6 |
| sys_config 创建人修正为 fqe | 10557–10560 | 4 |
| sys_config 20001 备注文案修订 | 10561–10564 | 4 |
| sys_config 归档上传开关与 PAT | 10565–10570 | 6 |
| sys_config 文件上传仓库配置 20004 | 10571–10576 | 6 |
| 月归档上传 GitHub 仓库（Contents API） | 10577–10586 | 10 |
| 上传范围收窄为仅 .mvsv | 10587–10592 | 6 |
| 远端归档目录按证券分层 | 10593–10598 | 6 |
| finv_market 新增 market_symbol | 10599–10604 | 6 |
| 证券主数据 region/market 口径修正 | 10605–10610 | 6 |
| 归档导出状态重置 | 10611–10615 | 5 |
| 证券主数据 currency 刷新修正 | 10616–10620 | 5 |
| 清理 GCMain 误插入月K线 | 10621–10625 | 5 |
| 恢复 quote 回测隔离副本 | 10626–10631 | 6 |
| 分钟行情双 schema 读写分工 | 10632–10637 | 6 |
| 数据库表保护红线 | 10638–10645 | 8 |
| 系统配置 20005：CDEU 运行模式 | 10646–10652 | 7 |
| 导出低峰窗口改为 20006（含删表授权） | 10653–10660 | 8 |
| 待办：sys_config 通用管理接口 | 10661 | 1 |
| 预算与窗口仅约束自动调度 | 10662–10666 | 5 |
| CDEU 概览展示动态 JOB 实例 | 10667–10670 | 4 |
| 系统配置 20007：导出后删主表 | 10671–10678 | 8 |
| CDEU 运行实例表去自增代理键 | 10679–10686 | 8 |
| 动态 JOB 按运行隔离 | 10687–10693 | 7 |
| 系统配置 20008：动态 JOB 保留天数 | 10694–10698 | 5 |
| 20007 沉淀月补偿清理 | 10699–10704 | 6 |
| CDEU 页面展示运行模式 | 10705–10706 | 2 |
| CDEU 父级状态修复与动效 | 10707–10712 | 6 |
| 进度条百分比展示（由 10748 落地） | 10713 | 1 |
| 清理历史备份表 | 10714 | 1 |
| CDEU 动态 Step 表改名 | 10715 | 1 |
| 20007 清理改为归档复核独立 JOB | 10716 | 1 |
| 收尾阶段 JOB 重分配与解耦 | 10717–10722 | 6 |
| 按 workflow_run_id 清理历史运行数据 | 10723 | 1 |
| run 59 暴露的 5 个缺陷 | 10724–10728 | 5 |
| 阶段内流程串行与统计列拆分 | 10729–10731 | 3 |
| 数据修复与运行循环的竞态 | 10732 | 1 |
| 阶段级串行/失败传播与按钮/取消状态 | 10733–10735 | 3 |
| 空转循环免疫超时回收 | 10736 | 1 |
| SKIPPED 统计口径与强制终止 | 10737–10740 | 4 |
| 占位流程配 JOB | 10741 | 1 |
| 终态流程 started_at 回填 | 10742 | 1 |
| BASE 文件名加 _Base 后缀 | 10743 | 1 |
| 本地导出文件清理配置 20011 | 10744 | 1 |
| 合并 usc_priority 到 usc | 10745 | 1 |
| 手动模式隐藏每日调度 | 10746 | 1 |
| 流程开始时间改为真实值 | 10747 | 1 |
| 阶段/流程进度百分比文本 | 10748 | 1 |
| 补齐字典 3120 主表记录 | 10749 | 1 |
| 新增市场 1332 PINK | 10750 | 1 |
| 迁移脚本编号统一 +100000 | 10751 | 1 |
| 日志前缀改为 [cdeu] | 10752 | 1 |
| 新增 cdeu schema 域占位 | 10753 | 1 |
| cdeu 建表（对应 dedup 结构） | 10754 | 1 |
| 修正 cdeu 表命名（cdeu_ 也替换） | 10755 | 1 |
| dedup 数据迁入 cdeu | 10756 | 1 |
| 代码读写切换到 cdeu | 10757 | 1 |
| 前端文案与镜像构建修正 | 10758 | 1 |
| dedup 重命名与流程验证 | 10759 | 1 |
| 回填数据与完整链路验证 | 10760 | 1 |
| 登录认证与访问拦截 | 10761 | 1 |
| 认证与配置模块目录归位 | 10762 | 1 |
| 登录后修改密码 | 10763 | 1 |
| 前端基础路由改大驼峰 | 10764 | 1 |
| 认证接口前缀统一 | 10765 | 1 |
| Native 安装到 /data/App/FinvQuant | 10766 | 1 |
| 一键下载脚本 | 10767 | 1 |
| 发行包 Migrations 提到包顶层 | 10768 | 1 |
| 配置放安装根目录 + 摘要渲染修复 | 10769 | 1 |
| 安装脚本支持备份后重建空库（已移除） | 10770 | 1 |
| 首次部署兼容共享 public 同名表 | 10771 | 1 |
| 历史迁移不删除 public 非本程序表 | 10772 | 1 |
| 一次性 K 线数据迁移脚本 | 10773 | 1 |
| Nginx 分阶段模板（80 默认 / 443 注释） | 10774 | 1 |
| Nginx 部署文档与上游 Issue 草案 | 10775 | 1 |
| 部署包包含一次性运维 SQL | 10776 | 1 |
| Upsert 端点迁移到 CDEU 前缀 | 10777 | 1 |
| Markdown 表格必须带表头 | 10778 | 1 |
| 全仓 Markdown 表格结构合法 | 10779 | 1 |
| KLinePage 系列文档目录归位 | 10780 | 1 |
| 全部分钟行情端点统一 Cdeu | 10782 | 1 |
| 接口文档按 OpenAPI 3.1 编写 | 10783 | 1 |
| OpenAPI 契约独立为同名 .yml | 10784 | 1 |
| KLinePageUni 按 OpenAPI 样板重写 | 10785 | 1 |
| 外部 HOOK 接入方案文档 | 10786 | 1 |
| 外部 OpenAPI 示例端点 | 10787 | 1 |
| 外部 OpenAPI 响应信封规范化 | 10788 | 1 |
| 外部 OpenAPI PageUni 查询端点 | 10789 | 1 |
| 外部 OpenAPI 暂不鉴权 | 10790 | 1 |
| cdeu 成交量改为浮点 | 10791 | 1 |
| volume 全链路浮点化 | 10792 | 1 |
| 外部 OpenAPI 文档自包含 | 10793 | 1 |
| K 线字段改名与 last_close 落库 | 10794 | 1 |
| 字段表类型/可空分列 | 10795 | 1 |
| Native 日志落盘 | 10796 | 1 |
| 日志切分与按日归档 | 10797 | 1 |
| cdeu 行情导出/导入工具方案 | 10798 | 1 |
| qmio 数据搬运工具 | 10799 | 1 |
| BRS 数据备份服务 | 10800 | 1 |
| BRS 对象存储上传 | 10801 | 1 |
| BRS 对象存储配置说明 | 10802 | 1 |
| Web 左侧菜单四级化 | 10803–10807 | 5 |
| V300057 迁移兼容 dedup 缺失 | 10808 | 1 |
| 登录后页面页脚 | 10809 | 1 |
| 深色/浅色主题切换 | 10810–10811 | 2 |
| CDEU 前端路由调整 | 10812 | 1 |
| CDEU 归档工作流设计文档 | 10813 | 1 |
| finv_security 扩展三列 | 10814 | 1 |
| Supabase 证券视图下载到中转目录 | 10815 | 1 |
| 富途 market_code 转换与插入脚本 | 10816 | 1 |
| 港股人民币柜台货币口径修正 | 10817 | 1 |
| 富途原始属性镜像表 | 10818 | 1 |
| 初始数据发放迁移入库 | 10819 | 1 |
| 日增量行数模板与自动补全 | 10820 | 1 |
| 自动发现补全优先级/SLA 默认值 | 10821 | 1 |
| Native 安装脚本 qmio 定位修正 | 10822 | 1 |
| 证券元数据与默认值补全优化 | 10823 | 1 |
| 上传 JOB 日志可观测性 | 10824 | 1 |
| GitHub 大文件上传与去重口径 | 10825 | 1 |
| 补齐 day_expect_rows 后重算成熟度 | 10826 | 1 |
| 补全流程不覆盖人工清空的优先级 | 10827 | 1 |
| 业务月份自动下拉列表 | 10828 | 1 |
| CDEU 阶段按拓扑顺序执行 | 10829 | 1 |
| CDEU 流程与 JOB 按顺序执行 | 10830 | 1 |
| 动态 JOB 领取顺序（优先级 + seq_no） | 10831 | 1 |
| 导出决策明细 JOB 与 provider 修复 | 10832 | 1 |
| 删除 dedup / dedup_frozen schema | 10834 | 1 |
| 行情数据归档·月份存量清单 | 10835 | 1 |
| Download.sh 回显包内 commit | 10836 | 1 |
| 证券元数据 JOB 归入批次流程 | 10837 | 1 |
| 菜单改名与端点调整 | 10838 | 1 |
| 菜单新增二级「行情」并移动叶子 | 10839 | 1 |
| 菜单改名「元数据」与顺序调整 | 10840 | 1 |
| 新增二级菜单「量化」（占位） | 10841 | 1 |
| 「配置管理」移到「行情」之前 | 10842 | 1 |
| 「量化」下新增两个三级子菜单 | 10843 | 1 |
| 「历史行情数据导入」移入「历史行情」 | 10844 | 1 |
| CDEU 工作流面板移入 ETL 并改名 | 10845 | 1 |
| 5 个配置类叶子移入 MVP配置 | 10846 | 1 |
| 回测类叶子移入「量化 → 回测」 | 10847 | 1 |
| 新增「量化 → 验证」并迁入 3 叶子 | 10848 | 1 |
| 新增「量化 → 实盘」并迁入实盘交易 | 10849 | 1 |
| 新增「量化 → 策略」并迁入策略管理 | 10850 | 1 |
| 「量化 → 实盘」改名「交易」 | 10851 | 1 |
| 「业务元数据维护」改名 Finv业务元数据 | 10852 | 1 |
| 「交易所下设市场信息维护」改名市场信息管理 | 10853 | 1 |
| 「交易所信息维护」改名交易所信息管理 | 10854 | 1 |
| 「规范证券信息维护」改名规范证券信息管理 | 10855 | 1 |
| K线数据查询改名并移入 ETL | 10856 | 1 |
| K线数据统计改名并移入 ETL | 10857 | 1 |
| 5 个归档运维页面移入 ETL | 10858 | 1 |
| 「配置管理」改名系统配置 | 10859 | 1 |
| 「行情数据归档」移入「行情」 | 10860 | 1 |
| K线数据查询/统计前置到 ETL 最前 | 10861 | 1 |
| 导出任务→文件导出、文件与上传→文件上传 | 10862 | 1 |
| 系统显示名改为 FinvQuant策略回测量化交易 | 10863 | 1 |
| 收剔导传ETL 新增「CDEU自动化」 | 10864 | 1 |
| 「CDEU自动化」改为叶子菜单 | 10865 | 1 |
| 证券主数据 market_code 回填与启用 | 10866 | 1 |
| 导出中转表迁入 cdeu 域 | 10867 | 1 |
| dedup 包迁入 cdeu | 10868 | 1 |
| 市场码与货币口径收口 | 10869 | 1 |
| 生产前独立库全链迁移验证 | 10870 | 1 |
| finv_market 字典编号体系对齐 | 10871 | 1 |
| 「备份与恢复」前端二级菜单 | 10872 | 1 |
| SH 指数类 usc 8 位改 6 位 | 10873 | 1 |
| 兼容期 8 位 usc 重复行延后清理（待办） | 10874 | 1 |
| USDIndex 的 type_secu_sub 修正为 CIX | 10875 | 1 |
| USDIndex 固定归属市场 1253 | 10876 | 1 |
| finv_region 补 FX/CRYPTO 并统一 US | 10877 | 1 |
| 加密货币 region 取值修正为 CC | 10878 | 1 |
| 加密货币交易所 region 切到 CC | 10879 | 1 |
| 20 只加密货币证券 exchange_code 修正为 10001 | 10880 | 1 |
| 迁移号段重排（1/2/3 三段，为基线重做准备） | 10881 | 1 |
| 生产库基线重做（待办） | 10882 | 1 |
| CDEU 运行树状态误判修复（PENDING 不再显示 RUNNING） | 10883 | 1 |
| CDEU 历史残留运行记录收敛（已终态 run 的孤儿子级） | 10884 | 1 |
| CDEU「JOB 进度」标题去掉重复计数 | 10885 | 1 |
| 「当日累计均价」字段名调整（待办） | 10886 | 1 |
| CDEU 指标卡片顺序调整（运行时长移到最左） | 10887 | 1 |
| moomoo 行情客户端对齐上游 | 10953 | 1 |
| moomoo 行情客户端改为依赖上游模块 | 10954 | 1 |
| MetaExt 依赖升级到 v0.1.1 | 10955 | 1 |
| 证券实时行情快照表（cdeu） | 10956 | 1 |
| 证券实时行情快照批量 Upsert 接口 | 10957 | 1 |
| 证券实时行情快照 stock_id 扩为 BIGINT | 10958 | 1 |
| 实时行情快照新增 7 个结构化数值列 | 10959 | 1 |
| 实时行情快照新增 change_price（涨跌额） | 10960 | 1 |
| 实时行情快照新增 price_direct / leverage | 10961 | 1 |
| 前端「实时行情」菜单与分页查询接口 | 10962 | 1 |
| 实时行情路由改 FTStockRank 与红涨绿跌行着色 | 10963 | 1 |
| 实时行情菜单更名为证券行情排行 | 10964 | 1 |
| 证券行情排行筛选行与底部分页布局调整 | 10965 | 1 |
| 涨跌着色红色改为正红 | 10966 | 1 |
| 涨跌着色绿色改为亮绿 | 10967 | 1 |
| 证券行情排行表格取消内部纵向滚动 | 10968 | 1 |
| 证券行情排行表格列宽优化 | 10969 | 1 |
| 分页条数控件补齐「每页…条」文案 | 10970 | 1 |
| 实时行情表增加 page 字段并接入 Upsert | 10971 | 1 |
| Upsert 端点改名为 FTStockUpsert | 10972 | 1 |
| Upsert 响应统计字段改为 insert/update | 10973 | 1 |
| 日K 同步响应统计字段改为 insert/update | 10974 | 1 |
| 分钟K线批量插入与历史行情导入响应统计字段改名（待办） | 10975 | 1 |
| 实时行情列表采集配置表 | 10976 | 1 |
| 行情列表股票快照同步服务与接口 | 10977 | 1 |
| 列表快照同步端点改名 FTStockQuoteSnapshotSync | 10978 | 1 |
| 列表快照同步统计字段改为 rows_receive | 10979 | 1 |
| ApiSpec 固化统计字段命名规范 | 10980 | 1 |
| 存量统计字段按 ApiSpec §1.1 迁移（待办） | 10981 | 1 |
| 前端「行情快照手动同步」菜单与异步采集任务 | 10982 | 1 |
| 实测并固化列表接口 page 起始值为 1 | 10983 | 1 |
| 采集每页输出证券明细日志（含新增/更新标记） | 10984 | 1 |
| 采集配置表更名为 finv_cdeu_secu_realtime_sync_config | 10985 | 1 |
| 行情快照手动同步改为按证券品种分 Tab | 10986 | 1 |
| 日K同步菜单补图标与页面展示优化 | 10987 | 1 |
| 采集明细日志只展示证券代码 | 10988 | 1 |
| 采集支持英文单位口径并锁定语言 | 10989 | 1 |
| 无运行记录时接口也返回采集配置 | 10990 | 1 |
| 列表采集修正页码基准与 referer | 10992 | 1 |
| 列表采集自动补齐 usc 与币种 | 10993 | 1 |
| 收紧 usc 与币种的补齐口径 | 10994 | 1 |
| 新增「待录入证券」菜单与查询接口 | 10995 | 1 |
| 待录入证券增加录入对话框（字段登记） | 10996 | 1 |
| MetaExt 依赖升级到 v0.1.2 并移除列表采集临时规避 | 10997 | 1 |
| 新建缺省汇率表 public.finv_cer | 10998 | 1 |
| 缺省汇率手动触发接口与前端菜单 | 10999 | 1 |
| 缺省汇率矩阵/表格双模式、中文名称与路由改名 | 11000 | 1 |
| 缺省汇率按模式分流双来源（overview / rateList） | 11001 | 1 |
| 新建 public.finv_fxcer（v2：usc/name 与列改名） | 11002 | 1 |
| 人民币名称规范与矩阵币种顺序调整 | 11003 | 1 |
| 矩阵顺序微调（GBP 前移、CHF 前移） | 11004 | 1 |
| 缺省汇率说明文本统一移到表格下方独立卡片 | 11005 | 1 |
| 说明卡片加标题与分节样式 | 11006 | 1 |
| 新增前端规范 ExplainCardSpec（说明卡片） | 11007 | 1 |
| 新增货币字典 Meta/Dict/Currency.md | 11008 | 1 |
| 新增区域字典 Meta/Dict/FinvRegion.md | 11009 | 1 |
| 同步 FinvRegion 数据字典映射到 27 条 | 11010 | 1 |
| 区域字典表新增独立 IDX 列 | 11011 | 1 |
| 矩阵角单元格改为对角线拆分「源/目标」 | 11012 | 1 |
| 矩阵单元格卡片化 + 列分割线 + 悬浮反馈 | 11013 | 1 |
| 矩阵改直角实线分割 + 修复提示换行 | 11014 | 1 |
| 矩阵悬浮时列首同步高亮 | 11015 | 1 |
| 缺省汇率表增涨跌额/涨跌幅并采集落库 | 11016 | 1 |
| 矩阵模式按涨跌幅红涨绿跌着色 | 11017 | 1 |
| 矩阵悬浮提示增加涨跌额/涨跌幅 | 11018 | 1 |
| 撤销悬浮提示的涨跌着色，统一默认文字色 | 11019 | 1 |
| 取消 CNH→CNY 同价派生 | 11020 | 1 |
| 清理 finv_cer（v1）脚本/文档/代码描述 | 11021 | 1 |
| 修复全新空库迁移链失败 | 11022 | 1 |
| 实时快照表增加 5 个人民币计价字段 | 11023 | 1 |
| 证券行情排行：人民币计价指标手动更新 | 11024 | 1 |
| 证券行情排行：人民币口径展示与默认排行 | 11025 | 1 |
| 待办：空币种行币种回填口径 | 11026 | 0 |
| 排序白名单移除人民币汇率（换算参数不排序） | 11027 | 1 |
| 实时快照表增加 remark 备注列 | 11028 | 1 |
| 实时快照表增加 gmt_create/gmt_update 审计时间 | 11029 | 1 |
| 修正 3 条美股宽基股 market_code | 11030 | 1 |
| 20261109 批次初始数据改以迁移脚本发放 | 11031 | 1 |
| 线上缺失 428 个 stock_id 的可补部分发放 | 11032 | 1 |
| 线上缺失剩余 9 个改为 cdeu 就地派生 | 11033 | 1 |
| XRP ETF 分配独立 usc=ETFXRP | 11034 | 1 |
| 5 个证券 usc 更正与初始数据发放 | 11035 | 1 |
| 新建证券键表 futu.finv_quote_secu_key | 11036 | 1 |
| 键表 status 取值加 CHECK 约束 | 11037 | 1 |
| 键表增加 stock_id / usc 列 | 11038 | 1 |
| 采集表列改名 type_symbol→instrument、quote_market→market_label | 11039 | 1 |
| 键表改名 finv_quote_secu_key → finv_futu_secu_key | 11040 | 1 |
| 新增系统配置 20013（Supabase 连接配置） | 11041 | 1 |
| 证券参数同步菜单与页面（缺省汇率之后） | 11042 | 1 |
| Supabase 证券键表分批增量同步（三个接口） | 11043 | 1 |
| Supabase 源表名可配（系统配置 20014） | 11044 | 1 |
| 证券参数同步页面拆两个 Tab | 11045 | 1 |
| 同步改异步运行 + 页面实时日志区 | 11046 | 1 |
| 待办：Tab 2 证券指标数据同步口径待确认 | 11047 | 1 |
| 新建 cdeu.finv_quote_secu_meta 证券指标表（131 列） | 11048 | 1 |
| 证券指标表改名 finv_quote_secu_meta → finv_quote_secu_indicator | 11049 | 1 |
| Tab 2 证券指标数据同步（写三张表） | 11050 | 1 |
| flag_enable 按 usc 是否占位推导（占位 0 / 真实 1） | 11051 | 1 |
| Tab 1 不清空 Tab 2 补的 url | 11052 | 1 |
| 指标表 131 列补齐中文注释 | 11053 | 1 |
| Tab 3 上游 usc 回填（本地 → 上游，按 stock_id） | 11054 | 1 |
| 多条 usc 取等于 ftmm_symbol 的那条 | 11055 | 1 |
| Tab 3 追加回填 finv_quote_secu_key 的 usc+stock_id | 11056 | 1 |
| 键表按 market_label.futu_symbol 重建 ftmm_symbol 关联 | 11057 | 1 |
| Tab 2 改名「FT证券指标数据同步」 | 11058 | 1 |
| Tab 3 改名「上游主键参数回填」 | 11059 | 1 |
| 采集属性表增加生成列 ftmm_symbol + 索引 | 11060 | 1 |
| 指标表补回 stock_id 并纳入 Tab 2 同步（132 列） | 11061 | 1 |
| Tab 1 也同步 stock_id / usc（9 个同步列） | 11062 | 1 |
| Tab 2 改一次写两张表（不写键表）+ flag_enable 不覆盖 + futu_symbol 取 stock_code | 11063 | 1 |
| Tab 2 目标表卡片去掉证券键表（3 → 2） | 11064 | 1 |
| BRS 全量备份登记补录第一批（17 张） | 11065 | 1 |
| BRS 全量备份登记补录第二批（cdeu 4 张） | 11066 | 1 |
| 撤销 public.finv_cer 的备份登记 | 11067 | 1 |
| brs schema 3 张表纳入全量备份 | 11068 | 1 |
| 按 finv_futu_query_stat.json 补录证券初始数据（480 + 252） | 11069 | 1 |
| 按 finv_futu_query_stat-2.json 补录 finv_security（268 行） | 11070 | 1 |
| HK.800700 usc 统一为 HTI（含 V300156 修正） | 11071 | 1 |
| USC 映射规则结构化建表（4 表 + 1 视图 + 归档表 + 689 条种子 + cron 20015） | 11072 | 1 |
| USC 映射规则维护页（三 Tab + 菜单 + 接口文档）与默认关闭定时同步（V300158） | 11073 | 1 |
| 修复候选分层排序 + P22 提为前处理（319 支 SH 指数形态统一，V300159） | 11074 | 1 |
| P40 补 US/XRP → ETFXRP（美股 XRP ETF 不占用裸 XRP 读法，V300160） | 11075 | 1 |
| 预演明细支持分页浏览（Diff/Page + 页面分页/类别过滤，不再只显示 200 行） | 11076 | 1 |
| 指数代码统一去前导点（P16 前处理）+ 14 条同市场撞名例外改用 IDX. 前缀（V300161/V300162） | 11077 | 1 |
| 临时停用 P50 批量例外整层（flag_enable=0，条目保留可恢复，V300163） | 11078 | 1 |
| P50 条目逐条停用（status=RETIRED，页面可见；层开关同时保持关闭，V300164） | 11079 | 1 |
| USC 字段名去 -ed 形式（API/前端/结果表列改名，V300165）+ 审计日志「变更」可展开查看完整 JSON | 11080 | 1 |
| 定时同步支持页面配置 cron 与「立即执行一次」（预演+应用一键完成，无需等定时） | 11081 | 1 |
| finv_futu_usc_map 时间列改为 dt_ 前缀（dt_retire / dt_compute / dt_first_assign，V300166） | 11082 | 1 |
| 复用归档表时间列改 dt_ 前缀（archive.finv_usc_reuse_snapshot.dt_snapshot，V300167） | 11083 | 1 |
| SecuParamSync Tab 3 重构为「本地 usc 回推上游」（futu.finv_futu_secu_key.usc → public.finv_quote_secu_key.usc，按 ftmm_symbol，只写 usc 一列；删除 meta 表与 stock_id 回填旧逻辑） | 11084 | 1 |
| usc 一次性对齐（V300168）：quote_collect 按 ftmm_symbol、indicator 按 stock_id 对齐到 futu.finv_futu_secu_key，改前整行归档到 archive.*_usc_align_20261110；Tab 2 落库改用键表 usc（A1） | 11085 | 1 |
| usc 对齐收尾（V300169）：删除 indicator 3,649 行陈旧重复 + quote_collect 88 行重复 + 1 行孤立 US.LTC（占住 CC.LTC 的目标 usc），删前整行归档、删后两表与键表差异 0 | 11086 | 1 |
| 清理 indicator 无键表依据的重复 stock_id（V300170）：503 组各留 1 行（以 quote_collect.usc 仲裁，保留最新行；1 组 `FX.EURindex` 改名 `EURindex`），删 503 行、改名 1 行 | 11087 | 1 |
| 孤立行清理（V300171 + V300172）：indicator 826 行按 usc 回填 ftmm_symbol、保留原 stock_id；quote_collect 64 行按 usc 对齐 market_label/futu_symbol；删除无下游引用的 4 行（US.SMCF / USDINDEX / .DJUSNS / 000117），有下游引用的 127 行保留留痕 | 11088 | 1 |
| 清理无键表依据的证券（V300173）：public.finv_security 删 126 行并归档、quote_collect 删 126 行、indicator 删 1 行（EURindex），两表无依据行归零 | 11089 | 1 |
| 按「有数据才建」重建 public.finv_security（V300174）：stock/etf/index 且 US/SH/SZ/HK 的 1,676 个证券补登记（31 个撞唯一键跳过）；crypto 2,974 / futures 108 / forex 4 按口径未纳入 | 11090 | 1 |
| 纳入外汇/货币指数证券（V300175）：补 EURIndex（Forex/CIX/1253）+ 4 个有数据的外汇对（MYRKRW/NZDTHB/THBNZD/XNGUSD，Forex/CER/1251）；USDIndex 本已在册 | 11091 | 1 |
| 删除 finv_security 的重复垃圾行 USDindex（小写 i，零引用，已归档；V300176） | 11092 | 1 |
| 全新库部署可用（V300177 + 兼容修订）：数据修复类迁移在键表为空/引擎无行时跳过存量断言，V300168–V300176 不在种子数据上做清洗；移除 USC 条目物理外键（约定全库 0 外键） | 11093 | 1 |
| 迁移收敛校验只断言脚本自身可保证的结构性事实（计划删除集已删净、重复 0 组、与键表差异 0），「剩余无键表依据行数」这类数据质量计数一律只 `RAISE NOTICE`，不得因环境数据差异阻断服务启动 | 11094 | 1 |
| 回推上游的 PostgREST `in.(...)` 过滤值必须做「PostgREST 引号 + RFC 3986 百分号编码」，并按字符预算分片（长键/含 `&`、`,`、`"`、空格、中文的键不得把 URL 打坏） | 11095 | 1 |
| 加密 XRP 现货 `CC.XRP` 的 `usc=XRP` 必须由规则表 P40 精确例外条目给出（历史条目曾为 FTMM 全码且已 RETIRED，不得只靠 P20 市场默认顺带得出）；美股 `US.XRP→ETFXRP`、加拿大 `CA.XRP→CA.XRP` 的口径保持不变 | 11096 | 1 |
| 美股 XRP ETF（`US.XRP`，Bitwise XRP ETF）的 `usc` 由自造读法 `ETFXRP` 改为富途全码 `US.XRP`，规则层对应 `P40 / US / XRP → FTMM`；结果表、键表、采集/指标/证券/实时 6 表同步改名，`ETFXRP` 归零 | 11097 | 1 |
| USC 条目保存：`idn>0` 必须走 UPDATE（改键也算编辑），`idn=0` 走 INSERT；自然键重复返回 409 并给出已有条目 `idn`/状态，不得把数据库 23505 原始报错透出 | 11098 | 1 |
| 前端枚举类下拉与状态标签统一用 `中文[值]` 风格（如 `待生效[PENDING]`）：USC 条目的动作/状态/品种必须有下拉选项并带中文，菜单其他页面的同类枚举一并调整 | 11099 | 1 |
| USC 预演必须能发现「键表 `usc` 与规则结果不一致」（手工改值/外部同步覆盖）并以 `key_update` 分类列出，应用时回写键表；该类行结果表未变，不得计为 `update`、也不得出现「预演无变更」的假象 | 11100 | 1 |
| USC 条目维护页的「市场」必须支持下拉选择并带中文字面值（16 种市场，取自 `futu.finv_futu_secu_key.market`，中文对齐 `public.finv_market`），顶部筛选同样用该下拉，列表市场列显示 `中文[值]` | 11101 | 1 |
| USC 条目编辑对话框的取值输入必须随「动作」联动：仅 `LITERAL` 显示「固定取值」、仅 `REWRITE` 显示「代码正则+替换模板」，其余动作不显示取值输入并给出该动作的人话说明；字段标题不得使用 LITERAL/REWRITE 之类的英文术语 | 11102 | 1 |
| 「预演与应用」页必须支持按 `ftmm_symbol` / `code` / `usc` 关键字检索预演明细，并提供**不依赖预演**的结果表（`futu.finv_futu_usc_map`）随时查询（`GET /Map/Page`，可按状态过滤），避免几万条差异只能翻页找 | 11103 | 1 |
| USC 维护页的「规则」与「条目」必须拆成两个独立页签：规则页只读展示规则层（含生效/全部条目数）并可跳转到条目页带规则过滤；条目页提供规则/市场/状态/关键字过滤 | 11104 | 1 |
| USC 维护页「规则」页签表格的「说明」列不得摊开长文本，改用图标 + 鼠标悬浮提示展示完整说明与备注，避免撑高变形 | 11105 | 1 |
| 新增开发规范 `Docs/DevSpec/TableTextSpec.md`：表格单元格禁止摊开长文本，一律 ⓘ 图标 + 悬浮提示，并给出判定口径、图标配色、悬浮内容结构、自检清单与反例 | 11106 | 1 |
| USC 维护页「条目」页签的说明列（`item_desc` + `source_ref` + `remark`）同样改为 ⓘ 悬浮，与规则页签口径一致 | 11107 | 1 |
| 字典值展示口径：**下拉选项**用 `中文[值]`；**表格列只显示中文值**，不得带键（如显示 `生效` 而非 `生效[ACTIVE]`、`美股` 而非 `美股[US]`）；未登记值原样显示 | 11108 | 1 |
| 新增开发规范 `Docs/DevSpec/DictFilterSpec.md`：字典/可枚举的**查询筛选控件**选项统一 `中文[值]`，含适用判定、首项「全部 X」空值语义、触发时机、选项来源核对、布局一致性、自检清单与反例 | 11109 | 1 |
| 新增开发规范 `Docs/DevSpec/DictColumnSpec.md`：字典/可枚举的**表格列**只显示中文（不带键），含适用判定、空值与未登记值口径、状态列 chip 语义色、需要看键时的三处出口、自检清单与反例（与 `DictFilterSpec.md` 各自独立成篇） | 11110 | 1 |
| 新增开发规范 `Docs/CheckList/CommitPreCheckListSpec.md`：代码递交前的检查清单（C1–C28 分组条目 + 一键自检命令 + 红线 + 豁免写法 + 条目新增规则） | 11111 | 1 |
| 新增开发规范 `Docs/CheckList/MergeCommitPreCheckListSpec.md`：PR 合并前的校验复核与功能验收清单（M1–M6：需求/代码/迁移/文档一致性/功能验收证据/合并动作 + 打回准则 + PR 模板） | 11112 | 1 |
| 两份清单补入「高频事故防守表（`R1…R16`，逐条挂真实事故与判据）」与「按变更类型取最小必查集」路由（提交侧 `C`/`R` 编号，合并侧复核范围与必验判据）；不拆前后端两份清单 | 11113 | 1 |
| 检查清单文档统一存放 `Docs/CheckList/` 并以 `Pre` 前缀命名：`CommitPreCheckListSpec.md`、`MergeCommitPreCheckListSpec.md`（原 `Docs/DevSpec/CommitCheckListSpec.md`、`MergeCheckListSpec.md`） | 11114 | 1 |
| 新增 `Docs/CheckList/FrontendCommitPreCheckListSpec.md`：前端改动专项清单（FE1–FE15，含枚举口径、长文本、三态、产物一致性与本类高频失手） | 11115 | 1 |
| 新增 `Docs/CheckList/BackendCommitPreCheckListSpec.md`：后端改动专项清单（BE1–BE16，含错误映射、URL 编码与分片、幂等、资源释放、性能证据） | 11116 | 1 |
| 新增 `Docs/CheckList/DbChangeCommitPreCheckListSpec.md`：数据库修改专项清单（DB1–DB20：授权、脚本写法、空库/真实库验证、回滚、事故对照） | 11117 | 1 |
| 新增 `Docs/CheckList/ProposalDocCommitPreCheckListSpec.md`：提案设计清单（十章骨架 + PP1–PP10 + 批准/打回/待定判定 + 存放命名） | 11118 | 1 |
| 新增 `Docs/CheckList/TechDesignDocCommitPreCheckListSpec.md`：技术方案设计清单（十二章骨架 + TD1–TD13 + 常见问题） | 11119 | 1 |
| 新增 `Docs/CheckList/CodeRefactorCommitPreCheckListSpec.md`：代码重构清单（四条原则 + RF1–RF13 + 步骤清单骨架） | 11120 | 1 |
| `CommitPreCheckListSpec.md` §4 路由表改为指向六份专项清单，本文件只保留通用 C 组与事故 R 组 | 11121 | 1 |
| 检查清单文件名后缀统一为 `CheckListSpec.md`（原 `CheckSpec.md`）：`PreCommit` / `PreMerge` / `PreFrontend` / `PreBackend` / `PreDbChange` / `PreProposal` / `PreTechDesign` / `PreRefactor` 共 8 份 | 11122 | 1 |
| 代码重构清单文件名由 `PreRefactorCheckListSpec.md` 改为 `CodeRefactorCommitPreCheckListSpec.md`；命名口径：`Pre*` 用于环节门槛（提交前 / 合并前），`Code*` 用于代码类工作类型 | 11123 | 1 |
| 提案设计清单文件名由 `PreProposalCheckListSpec.md` 改为 `ProposalDocCommitPreCheckListSpec.md`（`Doc` 标识「产出文档」类清单，取代需求 11122 中的 `PreProposal` 命名） | 11124 | 1 |
| 代码重构清单文件名由 `CodeRefactorCheckListSpec.md` 改为 `CodeRefactorCommitPreCheckListSpec.md`（补齐 `Commit` 段，改为需求 11123 中的 `CodeRefactor` 命名） | 11125 | 1 |
| 后端清单文件名由 `PreBackendCheckListSpec.md` 改为 `BackendCommitPreCheckListSpec.md`（取代需求 11122 中的 `PreBackend` 命名） | 11126 | 1 |
| 合并前清单文件名由 `PreMergeCheckListSpec.md` 改为 `MergeCommitPreCheckListSpec.md`（取代需求 11122 中的 `PreMerge` 命名） | 11127 | 1 |
| 技术方案设计清单文件名由 `PreTechDesignCheckListSpec.md` 改为 `TechDesignDocCommitPreCheckListSpec.md`（取代需求 11122 中的 `PreTechDesign` 命名） | 11128 | 1 |
| 数据库修改清单文件名由 `PreDbChangeCheckListSpec.md` 改为 `DbChangeCommitPreCheckListSpec.md`（取代需求 11122 中的 `PreDbChange` 命名） | 11129 | 1 |
| 前端清单文件名由 `PreFrontendCheckListSpec.md` 改为 `FrontendCommitPreCheckListSpec.md`（取代需求 11122 中的 `PreFrontend` 命名） | 11130 | 1 |
| 提案设计清单文件名由 `ProposalDocCheckListSpec.md` 改为 `ProposalDocCommitPreCheckListSpec.md`（统一 `Commit` 段，取代需求 11124 中的 `ProposalDoc` 命名） | 11131 | 1 |
| 检查清单文件名统一为 `<领域>[Doc]CommitPreCheckListSpec.md`（`Pre` 由前导前缀改为置于 `CheckListSpec` 之前）：`CommitPre`、`MergeCommitPre`、`FrontendCommitPre`、`BackendCommitPre`、`DbChangeCommitPre`、`ProposalDocCommitPre`、`TechDesignDocCommitPre`、`CodeRefactorCommitPre` 共 8 份 | 11132 | 1 |
| 新增 `Docs/CheckList/README.md`：清单目录说明（八份清单一览与使用时机、三步使用流程、命名规则、通用约定、新增或修改清单的流程与校验命令、与 DevSpec/Proposal/Design/Refactor 的关系） | 11133 | 1 |
| P50 条目只发放「当前生效的 4 条单点例外」（`CC.ANT` / `CC.ATOM` / `US.SPTSX` / `THA.SET`），其余 637 条批量例外从 `V300157` 种子中删除（需求 11079 已整层停用，不再随脚本发放；`V300164` 改为只停用批量例外、保留这 4 条 ACTIVE，校验改为「恰有 4 条 ACTIVE」） | 11134 | 1 |
| 修复 `/Meta/Finv/Quote/Realtime/UscMapping` 条目编辑框「市场」「品种」保存后落库为 `[object Object]`：`v-combobox` 默认 `return-object=true` 把选项对象写回 `v-model`，`nullable()` 用 `String(obj)` 转成字符串；改为显式 `item-title`/`item-value`/`:return-object="false"`，并把 `nullable()` 换成对象取 `value`/`title` 的归一函数 | 11135 | 1 |
| USC 条目编辑框的「条目说明」改为**默认自动生成**（按 市场/品种 + 匹配键 + 动作效果 + 依据规则 拼一句，随表单联动），用户可直接改写（改写后不再覆盖），并提供「重新生成」按钮；`item_desc` 不再是必须手填的空白项 | 11136 | 1 |
| 审计日志「主键」列拆开：后端从快照解析返回 `src_idn` / `src_rule_no` / `src_ftmm`，页面改为「主键」「所属规则」两列（原先把 `idn=689 P50` 挤在一格） | 11137 | 1 |
| 规则页签支持修改：新增 `POST /Rule/Save`（仅开放 `flag_enable` / `default_action` / `rule_desc` / `remark`，写 `src_table='rule'` 审计日志）；前端「启用」改 `v-switch` 点击即切层、「编辑」弹窗改说明与默认动作；点行不再跳转，跳转只由「查看条目」按钮触发 | 11138 | 1 |
| 规则表格「条目数（生效/全部）」拆成「生效条目数」「全部条目数」两列 | 11139 | 1 |
| 恢复启用 P50 批量例外层：`V300180` 把 `futu.finv_futu_usc_rule.flag_enable` 由 `'0'` 改回 `'1'`（生效条目仅 4 条单点例外）；结果表落库仍走页面「预演 → 应用」 | 11140 | 1 |
| 审计日志「变更摘要」列改为两个图标按钮（「看摘要」字段级差异表 / 「看对比」前后 JSON 并排），点该行其他列默认打开「对比」视图，弹窗内可用页签切换；原「详情」列合并进该列 | 11141 | 1 |
| `/Meta/Finv/Quote/Realtime/SecuParamSync` Tab 3「本地 usc 回推上游」同时回推 `priority`：本地读 `futu.finv_futu_secu_key.usc`/`.priority`，上游 `public.finv_quote_secu_key` 同名列同一行一次 PATCH；判定改为两列都一致才 `keep`，任一为空 `fill`，都不为空但有差异 `overwrite` | 11142 | 1 |
| `ftmm_symbol` 入库英文字母统一大写：写入侧覆盖 批量 upsert / 应用 / 指标批量（`upper(btrim(...))`）+ 新增 `internal/meta/finv/quote/symbol.Normalize` 供 Go 侧调用；存量由 `V300181` 一次性改写（键表 / 结果表 / 指标表），撞名先中止 | 11143 | 1 |
| 采集属性表三条 GDR / 外汇指数键与 usc 修正：`ftmm_symbol` 是生成列（不可 UPDATE），改为修正基列 `market_label`/`futu_symbol` + `usc`，落到 `HK.GDRNEXT`→`GDRNext`、`FX.EURINDEX`→`EURIndex`、`FX.USDINDEX`→`USDIndex`；`V300182` 按 `upper(...)` 兼容大小写变体匹配，主键/唯一键冲突先中止 | 11144 | 1 |
| 菜单 `/Meta/Finv/Quote/Realtime/PendingSecu` 改口径：先用派生键 `ftmm_symbol = upper(market_label \|\| '.' \|\| stock_code)` 左连 `futu.finv_futu_secu_key` 补一次 `usc`，**补完仍为空**的快照行才列出；响应新增 `ftmm_symbol`，关键字可搜派生键，页面新增「派生键」列 | 11145 | 1 |
| 待录入证券页面在「查询」按钮右侧新增「手动补录USC」按钮，调用 `POST /Meta/Finv/Quote/Cdeu/Realtime/Stock/USCBackfill`：按派生键 `ftmm_symbol = upper(market_label \|\| '.' \|\| stock_code)` 关联 `futu.finv_futu_secu_key`，把 `usc` 为空（NULL / 空白）的 `cdeu.finv_cdeu_secu_realtime_stock` 行就地写回；键表无对应证券的行不动，幂等（只写空值行） | 11146 | 1 |
| 「录入证券元数据」对话框改为**关联自动预填**：新增 `GET /Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Prefill?stockId=`，按快照行 + `public.finv_exchange` 字典 + `futu.finv_futu_quote_collect` 同市场同品种参考值算出两张表字段并标注来源（快照/字典/参考/缺省/待填）；新增 `POST .../Stock/Pending/Enroll` 单事务写 `public.finv_security` + `futu.finv_futu_quote_collect` 并回写快照 `usc`（三处均幂等），两张表 `remark` 写入 `[AUTO-FILL]` 标记与时间便于溯源 | 11147 | 1 |
| 「录入证券元数据」对话框移除 `cdeu.finv_quote_cdeu_usc` 分区：该表数据由运行时采集链路动态生成，页面不再展示、补录不再写入该表 | 11148 | 1 |
| 待录入证券表格首列增加 `usc` 字段展示（快照真实值，空值显示 `—` 并在悬浮说明中指向「派生键」列，避免与本页「补全后仍为空」的口径冲突） | 11149 | 1 |
| 修复「录入证券元数据」对话框一打开就报 `预填失败：stockId 必须是 1..0 的整数`：`stockId` 改为按 int64 解析且只校验下界（bigint 主键无业务上界），不得再套 `min..max` 闭区间 | 11150 | 1 |
| 待录入证券表格「品种类型」列按 `instrument_type` 字典翻译为中文（3/12→股票、4→ETF、6→指数、7→板块、9→期货、11→加密货币；未登记值原样、空值 `—`） | 11151 | 1 |
| 待录入证券表格列名「派生键 ftmm_symbol」改为「唯一标识」（表头悬浮说明保留 `upper(市场标签.代码)` 推导公式） | 11152 | 1 |
| 待录入证券操作列新增「入队」：按快照派生 `ftmm_symbol` / `market` / `code` / `stock_id` 与 `instrument`（`instrument_type` 主、`market_type` 兜底，词表与键表一致），新增 `GET /Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enqueue?stockId=` 预检、`POST .../Stock/Pending/Enqueue` 写 `futu.finv_futu_secu_key`（`status=PENDING` / `flag_enable=1` / `priority=10`，`usc` 留空）；键表已存在则不插入并提示去「USC 映射规则」补 USC | 11153 | 1 |
| 「入队」必须两步走：点按钮只打开确认对话框（预检只读），人工核对并可改正 `instrument` 后，再点对话框里的「确认入队」才写库；不得一键直接提交 | 11154 | 1 |
| 入队新增探测接口 `POST /Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enqueue/Probe`（只读，返回 `planned`/`existed`/`current`/`hint`），并按场景区分按钮：键不存在显示可点击的「确认入队」、已存在显示「更新键」（`POST .../Enqueue/Update` 改非主键字段）；未探测或键字段改动后按钮禁用，每次提交前服务端再验证一次存在性 | 11155 | 1 |
| 入队对话框写入 `futu.finv_futu_secu_key` 的字段支持手工修改：`market`/`code`/`stock_id`/`instrument`/`status`/`flag_enable`/`priority`，主键 `ftmm_symbol` 由 `market`+`code` 派生只读；携带词表与范围校验（`instrument`、`status` 词表，`flag_enable` 0/1，`priority` 1..1000000） | 11156 | 1 |
| 「证券参数同步」Tab 3（本地 usc 回推上游）改为 upsert：本地有、上游 `public.finv_quote_secu_key` 没有的键要**插入新记录**（按本地 `instrument`/`market`/`code`/`usc`/`priority`/`stock_id`/`status`/`flag_enable`（上游列名 `enable`）组行，`on_conflict=ftmm_symbol` + `resolution=merge-duplicates`），`note` 写 `[AUTO-FILL]` 标记便于溯源；已有行仍按 usc+priority 两列 PATCH；本地缺 `instrument`/`market`/`code` 的行计「无法插入」不插入 | 11157 | 1 |
| 「行情快照手动同步」菜单改名为「行情快照同步」（菜单标题、路由 `meta.title`、页面卡片标题、菜单文档标题同步改；菜单 key 与路由路径不变，历史深链接继续可用） | 11158 | 1 |
| 「缺省汇率」菜单改名为「外汇汇率」，并调整到「行情快照同步」菜单**之前**（`menuConfig.ts` 中「实时行情」分组下的顺序改为：证券行情排行 → 外汇汇率 → 行情快照同步 → 待采证券录入 → 证券参数补全 → 证券参数同步 → USC 映射规则；菜单 key / 路由 / 接口路径不变，数据表 `public.finv_fxcer` 与数据字典键不变） | 11159 | 1 |
| 「待录入证券」菜单改名为「待采证券录入」（菜单标题、路由 `meta.title`、页面卡片标题、菜单文档标题同步改；页面空态文案与 info 提示同步改；菜单 key / 路由路径不变） | 11160 | 1 |
| 新增菜单「证券参数补全」（`/Meta/Finv/Quote/Realtime/SecuParamComplete`，key `quote-realtime-secu-param-complete`），承接原「待录入证券」表格「操作」列「录入」按钮的能力，并按目标表分两个 Tab：「规范证券参数」（`public.finv_security`）与「采集参数」（`futu.finv_futu_quote_collect`）；每个 Tab 独立查询缺口清单、独立预填、独立写入（只写该 Tab 那一张表，按主键 `usc` 做 `ON CONFLICT DO NOTHING`，已存在返回 `created=false` 不覆盖）；缺口判定键 = `COALESCE(快照 usc, upper(市场标签.代码))`，存在性同时接受该键与派生键两种形态；写入前须在对话框中人工确认（两步走），预填字段带来源标记 | 11161 | 1 |
| 「待采证券录入」页表格「操作」列只保留「入队」按钮，移除「录入」按钮与对应对话框组件（能力迁至「证券参数补全」）；后端整体录入接口 `POST .../Stock/Pending/Enroll` 保留供脚本调用，页面不再引用 | 11162 | 1 |
| 「证券参数同步」Tab 3（本地 usc 回推上游）的写入列改为可配置：请求体新增 `fields: {usc, priority}`，前端「回推参数」卡片给出两个复选框（默认都勾）；判定与写入**只看勾选的列**（未勾的列不比对、不写、连值都不放进 payload），两个都不勾返回 400；只勾 `priority` 时上游缺记录的行不插入（计 `insert_skipped`）；运行快照回显 `fields` | 11163 | 1 |
| 修复「证券参数补全 → 采集参数」必填校验必然失败：快照 `instrument_type` 是 TEXT 且 `12`/`3` 都表示股票、采集表只有 `3`，参考行匹配按 `q.instrument_type::text = btrim(t.instrument_type)` 直接比对导致 `12` 的行取不到 `market_code`（=0）；改为按归一后的整型匹配（`12`→`3`）；`market_code` 取不到参考值时预填**返回空串**并提示必须人工填写（不再返回 `0`）；前端把数值字段的 `0` 也算「未填」并逐字段标红；后端必填校验错误消息逐项列出字段与实际值 | 11164 | 1 |
| 「证券参数补全 → 规范证券参数」对话框的 `timezone`（UTC 偏移）/ `tz`（IANA 标识）预填缺省值，按「市场 → 交易所 → 区域」三层推断：① 同 `exchange_code` 已有 `public.finv_security` 行 `(timezone, tz)` 的众数（来源=参考）；② 同 `finv_exchange.region` 已有行的众数（来源=参考）；③ 内置区域 → IANA 时区映射、偏移按该时区**当前时刻**推算（来源=字典，二进制内嵌 `time/tzdata`）；三层都拿不到时留空 + 来源=待填 + 风险提示 | 11165 | 1 |
| 「证券参数补全 → 采集参数」对话框补缺省推断：`market_type` 改为「快照 → 同市场已有配置行众数（参考）→ 内置映射 HK=1/US=2/A股=4/FX=11/CC=17（字典）」；`req_section` 改为「同市场同品种众数（参考），众数为空则留空并说明选填」；`sub_instrument_type` 的兜底改按**归一后** `instrument_type` 计算（`12` 按 `3` 算，避免算出 12002）；`instrument_type` 仍写快照原值但说明里点明「12 与 3 同为股票、存量以 3 为主」 | 11166 | 1 |
| **合计** | **10001–11176** | **1172**（含 4 条待办） |

> 验收方式：按编号顺序逐条核对，`√` = 满足，`×` = 不满足并记录差异。
> 编号连续、不跳号；新增需求从 `10803` 起往后追加，不插入既有区间。
