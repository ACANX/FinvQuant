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
| 10063 | 改造项 B 的重命名迁移与回填迁移各有一条清单条目。 | 命中 `V30__finv_quote_kline_rename_prev_close_to_last_close` 与 `V31__finv_quote_kline_backfill_open_from_last_close` 各 1 处 | |
| 10064 | `Docs/Refactor/` 下的过程文档文件名均带 `yyMM-` 四位前缀。 | `ls Docs/Refactor/*.md` 结果全部匹配 `^[0-9]{4}-` | |
| 10065 | `FileNamingSpec.md` 记录了 `yyMM-` 前缀规则及其例外清单。 | `grep -c "yyMM-" Docs/DevSpec/FileNamingSpec.md` 输出不小于 2 | |

## 六、改造项 A 验收（10066–10079）

> 对应清单 [`Refactor/2610-001-SecuCodeToUsc.md`](Refactor/2610-001-SecuCodeToUsc.md)：`secu_code` → `usc` 全库统一。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10066 | 存在 `V29` 改名迁移且为单事务脚本。 | `grep -c "^BEGIN;" Deploy/Migrations/V29__rename_secu_code_to_usc.sql` 与 `grep -c "^COMMIT;"` 均为 `1` | |
| 10067 | `V29` 覆盖 6 张表的 `secu_code` → `usc` 改名。 | 脚本内表名数组包含 `finv_quote_secu_kline_min`、`finv_quote_ingest_batches`、`finv_quote_revision_log`、`finv_quant_backtest_strategy`、`finv_quant_backtest_run`、`finv_quant_backtest_run_archive` 共 6 项 | |
| 10068 | `V29` 对 5 个依赖索引执行改名。 | `grep -c "idx_finv_quote_usc_ts\|idx_finv_quote_ingest_batches_usc\|idx_finv_quote_revision_log_usc\|idx_finv_quant_backtest_strategy_usc\|idx_finv_quant_backtest_run_usc" Deploy/Migrations/V29__rename_secu_code_to_usc.sql` 输出不小于 `5` | |
| 10069 | `V29` 不重建主键。 | 脚本内无 `DROP CONSTRAINT` 与 `ADD CONSTRAINT ... PRIMARY KEY` | |
| 10070 | Go 代码中不再出现旧字段名。 | `grep -rn "secu_code\|SecuCode\|secuCode" internal/ --include="*.go"` 输出为空 | |
| 10071 | 前端源码中不再出现旧字段名。 | `grep -rn "secu_code\|SecuCode\|secuCode" Web/src/` 输出为空 | |
| 10072 | 行情查询接口的查询参数与响应字段均为 `usc`。 | `grep -c "usc" internal/api/handler/query.go` 不小于 `5`，且该文件无旧名 | |
| 10073 | 文档中除计划文档、变更日志与本清单外无旧字段名。 | `grep -rn "secu_code\|secuCode" Docs/ Prompt.md \| grep -v "Docs/Refactor/\|Docs/CHANGELOG.md\|Docs/RequirementList.md"` 输出为空 | |
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
| 10080 | 存在 `V30` 改名迁移且为单事务脚本。 | `grep -c "^BEGIN;" Deploy/Migrations/V30__finv_quote_kline_rename_prev_close_to_last_close.sql` 与 `grep -c "^COMMIT;"` 均为 `1` | |
| 10081 | `V30` 以幂等方式把 `prev_close` 改名为 `last_close`。 | 脚本内同时含 `information_schema.columns`、`column_name = 'prev_close'` 与 `RENAME COLUMN prev_close TO last_close` | |
| 10082 | `V30` 为 `last_close` 写入口径注释。 | `grep -c "与 open 独立" Deploy/Migrations/V30__finv_quote_kline_rename_prev_close_to_last_close.sql` 输出 `1` | |
| 10083 | `V31` 回填条件完整（缺失/0 值 + `last_close` 非空非 0）。 | 脚本内出现 `open IS NULL OR open = 0`、`last_close IS NOT NULL`、`last_close <> 0` 三处条件 | |
| 10084 | `V31` 输出回填行数。 | `grep -c "RAISE NOTICE" Deploy/Migrations/V31__finv_quote_kline_backfill_open_from_last_close.sql` 不小于 `2` | |
| 10085 | `V31` 向 `finv_quote_revision_log` 写入留痕记录。 | 脚本内含 `MIGRATION_V31` 与 `INSERT INTO finv_quote_revision_log` | |
| 10086 | `V31` 明确声明回填不可逆。 | `grep -c "不可逆" Deploy/Migrations/V31__finv_quote_kline_backfill_open_from_last_close.sql` 不小于 `1` | |
| 10087 | Go 代码中不再用旧列名（MVSV 文件头文本与 ATR 局部变量除外）。 | `grep -rn "prev_close" internal/ --include="*.go"` 输出为空 | |
| 10088 | 导入链路按同一规则增量兜底 `open`。 | `grep -c "fillOpenFromLastClose" internal/quote/service.go` 不小于 `2` | |
| 10089 | 数据格式与接口文档已改用 `last_close`。 | `grep -c "last_close" Docs/DataFormat/MvsvFileFormat.md Docs/API/HistoryQuote/HistoryQuote.md` 各不小于 `1` | |
| 10090 | 前端源码不含该字段引用。 | `grep -rn "prev_close\|prevClose" Web/src/` 输出为空 | |
| 10091 | `CHANGELOG` 记录改名、回填与不可逆提示。 | `grep -c "last_close" Docs/CHANGELOG.md` 不小于 `2` | |

## 八、改造项 C 验收（10092–10107）

> 对应清单 [`Refactor/2610-003-MoveTablesToSchemas.md`](Refactor/2610-003-MoveTablesToSchemas.md)：`finv_quote_*` → `quote`、`finv_quant_*` → `quant` schema。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10092 | 存在 `V32` 迁移且创建两个业务 schema。 | `grep -c "CREATE SCHEMA IF NOT EXISTS" Deploy/Migrations/V32__move_finv_tables_to_quote_quant_schemas.sql` 输出 `2` | |
| 10093 | `V32` 覆盖 16 张表的迁移（3 张 quote + 13 张 quant，通过数组循环执行 `SET SCHEMA`）。 | `grep -oE "'finv_(quote|quant)_[a-z_]+" Deploy/Migrations/V32__move_finv_tables_to_quote_quant_schemas.sql \| sort -u \| wc -l` 输出 `16` | |
| 10094 | `V32` 设置数据库默认 `search_path`。 | 脚本内含 `ALTER DATABASE` 与 `current_database()` | |
| 10095 | `V32` 具备收敛校验，失败整体回滚。 | `grep -c "RAISE EXCEPTION" Deploy/Migrations/V32__move_finv_tables_to_quote_quant_schemas.sql` 不小于 `2` | |
| 10096 | 存在 `V100024` 收尾迁移，处理种子段新建的 3 张 quant 表。 | `test -f Deploy/Migrations/V100024__finv_quant_seed_tables_move_to_quant_schema.sql` | |
| 10097 | 应用连接串携带 `search_path`。 | `grep -c "search_path=public,quote,quant" internal/database/postgres.go` 输出 `1` | |
| 10098 | Go 代码中表名均带 schema 前缀（无裸表名）。 | `grep -rn "FROM finv_quote_\|FROM finv_quant_\|INTO finv_quote_\|INTO finv_quant_\|UPDATE finv_quant_\|UPDATE finv_quote_" internal/ --include="*.go"` 输出为空 | |
| 10099 | 文档中表名均带 schema 前缀。 | `grep -rn "FROM finv_quote_\|FROM finv_quant_" Docs/ Prompt.md \| grep -v "Docs/Refactor/\|Docs/CHANGELOG.md\|Docs/RequirementList.md"` 输出为空 | |
| 10100 | 生产库 `quote` schema 含 3 张行情表。 | `SELECT count(*) FROM pg_tables WHERE schemaname='quote'` 为 `3` | |
| 10101 | 生产库 `quant` schema 含 13 张量化表。 | `SELECT count(*) FROM pg_tables WHERE schemaname='quant'` 为 `13` | |
| 10102 | 生产库 `public` 不再有业务域表。 | `SELECT count(*) FROM pg_tables WHERE schemaname='public' AND tablename ~ '^finv_(quote|quant)_'` 为 `0` | |
| 10103 | 数据库与连接 `search_path` 为 `public, quote, quant`。 | `SHOW search_path;` 输出含 `public, quote, quant` | |
| 10104 | 表迁移后 `gmt_update` 触发器仍生效。 | 更新一行后 `gmt_update` 大于 `gmt_create` | |
| 10105 | 存在数据库 schema 规范文档。 | `test -f Docs/DevSpec/DbSchemaSpec.md` | |
| 10106 | `CHANGELOG` 记录本次 schema 迁移与连接要求。 | `grep -c "quote" Docs/CHANGELOG.md` 不小于 `2` | |
| 10107 | 历史种子迁移的列名例外已登记。 | `grep -c "V100019" Docs/DevSpec/DbSchemaSpec.md Docs/CHANGELOG.md` 各不小于 `1` | |

## 九、改造项 D 验收（10108–10120）

> 对应清单 [`Refactor/2610-004-MoveFutuTablesToSchema.md`](Refactor/2610-004-MoveFutuTablesToSchema.md)：`finv_futu_*` → `futu` schema。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10108 | 存在 `V33` 迁移且创建 `futu` schema。 | `grep -c "CREATE SCHEMA IF NOT EXISTS futu" Deploy/Migrations/V33__move_finv_futu_tables_to_futu_schema.sql` 输出 `1` | |
| 10109 | `V33` 覆盖 5 张富途映射表。 | `grep -oE "'finv_futu_[a-z_]+" Deploy/Migrations/V33__move_finv_futu_tables_to_futu_schema.sql \| sort -u \| wc -l` 输出 `5` | |
| 10110 | `V33` 更新数据库默认 `search_path` 含 `futu`。 | 脚本含 `public, quote, quant, futu` | |
| 10111 | `V33` 具备收敛校验与回滚。 | `grep -c "RAISE EXCEPTION" Deploy/Migrations/V33__move_finv_futu_tables_to_futu_schema.sql` 不小于 `2` | |
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
| 10131 | 面向使用者的文档不再出现旧的 Docker 部署路径。 | `grep -rn "Deploy/\.env\|Deploy/docker-compose.yml\|Deploy/Win11Docker" README.md Prompt.md` 输出为空 | |

## 十二、Native 部署配套与生成物命名（10132–10140）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10132 | Native 目录下脚本与配置文件采用大驼峰文件名。 | `ls Deploy/Native/` 中 `.sh` 与品牌名文件均以大写字母开头（`Install.sh`、`Upgrade.sh`、`Uninstall.sh`、`FinvQuant.service`、`FinvQuant.env.example`） | |
| 10133 | systemd unit 源文件名按品牌写作 `FinvQuant.service`（无小写 s 变体）。 | `ls Deploy/Native/ \| grep -x FinvQuant.service` 输出 1 行；`ls Deploy/Native/ \| grep -cx Finvquant.service` 输出 `0`（Windows 大小写不敏感，以 `ls` 实际输出为准） | |
| 10134 | unit 含运行所需关键指令。 | `grep -cE "^(ExecStart|EnvironmentFile|Restart|User|WantedBy)=" Deploy/Native/FinvQuant.service` 输出 `5` | |
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
| 10159 | 存在创建两个 schema 的迁移且可重复执行。 | `test -f Deploy/Migrations/V34__create_export_and_sche_schemas.sql`；文件内含 `CREATE SCHEMA IF NOT EXISTS export` 与 `... sche` | |
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
| 10167 | 存在创建转存表的迁移。 | `test -f Deploy/Migrations/V35__create_export_staging_kline_table.sql` | |
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
| 10176 | 存在创建 dedup schema 的迁移且幂等。 | `test -f Deploy/Migrations/V36__create_dedup_schema.sql`；含 `CREATE SCHEMA IF NOT EXISTS dedup` | |
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
| 10184 | 存在创建 archive schema 的迁移且幂等。 | `test -f Deploy/Migrations/V37__create_archive_schema.sql`；含 `CREATE SCHEMA IF NOT EXISTS archive` | |
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
| 10217 | 文档覆盖全部参数并标出默认值。 | `grep -c '^| `--' Deploy/Script/README.md` 不小于 `9` | |
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
| 10237 | 文档路径引用已更新。 | `grep -c 'Archive/VeritasQuant' README.md AGENTS.md Prompt.md Archive/README.md` 均不小于 `1`；根 README 目录树无裸 `VeritasQuant/` | |
| 10238 | `Archive/README.md` 登记该归档项。 | 含 `\| \`VeritasQuant/\` \|` 行，说明归档日期与「不参与现行构建」 | |
| 10239 | 构建上下文排除归档目录。 | `.dockerignore` 含 `Archive/`，且无独立 `VeritasQuant/` 行 | |
| 10240 | 可执行示例与代码注释路径同步。 | `grep -rn 'Archive/VeritasQuant' Docs/API/HistoryQuote/ImportsUpload.md internal/mvsv/parser.go` 各不小于 `1` | |

## 二十五、分钟行情 K 线 API 融合（10241–10263）

> 来源：`FinvQuoteDataDeDup`（Python/FastAPI）退役，接口融合进 FinvQuant Go 服务端。
> 关联文档：`Docs/API/KLine/*.md`、`Docs/API/APIs.md`、`Docs/DevSpec/ErrorCodeSpec.md`、`Docs/CHANGELOG.md`。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10241 | 分钟行情 API 注册新前缀 `/API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min`。 | `grep -c "Meta/Finv/Quote/DeDup/Secu/KLine/Min" internal/api/router.go` 输出不小于 `1` | |
| 10242 | 旧前缀 `/API/V1/Meta/Finv/Quote/Secu/KLine/Min` 作为过渡别名同时注册。 | `grep -c "LegacyAPIPrefix" internal/api/router.go` 输出不小于 `1` | |
| 10243 | 提供 `GET /Health` 探活端点（含 `/healthz` 过渡别名）。 | `grep -c 'GET("/Health"' internal/api/router.go` 与 `grep -c 'GET("/healthz"' internal/api/router.go` 均为 `1` | |
| 10244 | 分钟行情主表位于 `dedup.finv_quote_secu_kline_min`。 | `grep -c "const Table = \"dedup.finv_quote_secu_kline_min\"" internal/kline/sql.go` 输出 `1` | |
| 10245 | 迁移 V38 将主表从 `quote` 迁入 `dedup` 并可重复执行。 | `ls Deploy/Migrations/V38__finv_quote_kline_move_to_dedup.sql` 不存在时退出码非 0；脚本含 `SET SCHEMA dedup` 与 `IF EXISTS` 幂等判断 | |
| 10246 | 迁移 V38 补齐 `change_price`/`change_ratio`/`provider` 三列。 | `grep -c "ADD COLUMN IF NOT EXISTS change_price" Deploy/Migrations/V38__finv_quote_kline_move_to_dedup.sql` 输出 `1` | |
| 10247 | `export.finv_quote_secu_kline_min` 按 dedup 新结构重建。 | `grep -c "LIKE dedup.finv_quote_secu_kline_min" Deploy/Migrations/V38__finv_quote_kline_move_to_dedup.sql` 输出 `1` | |
| 10248 | `ts` 契约与库内均为秒级；13 位毫秒默认拒绝。 | `go test ./internal/kline/ -run TestTSUnit` 通过；用例覆盖秒级通过与 13 位拒绝 | |
| 10249 | `FINV_KLINE_TS_UNIT=ms` 时毫秒入参换算为秒。 | `go test ./internal/kline/ -run TestTSUnit` 通过（含 ms 分支） | |
| 10250 | 具体错误使用 `211xxx` 编号且全局唯一。 | `go test ./internal/kline/ -run TestCatalogIntegrity` 通过 | |
| 10251 | 错误目录共 15 条具体码，覆盖校验/资源/容量/存储四类。 | 同上；`internal/kline/apierr.go` 中 `Code` 常量计数为 15 | |
| 10252 | 具体码的 HTTP 映射与接口文档一致。 | `grep -c "211101" Docs/API/KLine/KLineAdd.md` 与 `grep -c "409" Docs/API/KLine/KLineAdd.md` 均不小于 `1` | |
| 10253 | 未归类的错误兜底复用 HTTP 错误码（500 用 `2006`）。 | `go test ./internal/api/handler/ -run TestErrorResponderUnknownErrorMasked` 通过 | |
| 10254 | 存量接口旧号 `4001/4004/4009` 已停用。 | `grep -rn '"code": *\(4001\|4004\|4009\)' internal/ cmd/` 输出为空 | |
| 10255 | 错误响应默认（safe）不含 `debug` 与内部细节。 | `go test ./internal/api/handler/ -run TestErrorResponderSafeByDefault` 通过 | |
| 10256 | `admin` 模式 + 正确 `X-Debug-Token` 才输出白名单 `debug`。 | `go test ./internal/api/handler/ -run TestErrorResponderAdminWithToken` 通过 | |
| 10257 | 令牌缺失/错误时静默回退 safe 且不提示令牌错误。 | 同上用例覆盖 | |
| 10258 | 来源 IP 命中 `FINV_ERROR_DETAIL_ALLOWED_CIDRS` 视为高权限。 | `grep -c "AllowedCIDRs" internal/config/config.go` 不小于 `1`；`grep -c "Contains(ip)" internal/api/handler/errresp.go` 输出 `1` | |
| 10259 | `request_id` 为大写 UUID4。 | `go test ./internal/api/handler/ -run TestErrorResponderSafeByDefault` 通过 | |
| 10260 | `/Page` 返回 `{code:1,data:{cnt,ps,pn,list,request_id,ts},message:"OK"}`。 | `grep -c '"pn":         kline.PageNumber' internal/api/handler/kline.go` 输出 `1`；手工调用 `/Page` 校验字段 | |
| 10261 | `/Page` 的 `cr` 为两位小数文本（HALF_UP），NULL 为 `null`。 | `go test ./internal/kline/ -run TestRatioText` 通过 | |
| 10262 | `/PageUni` 的 `column` 固定 15 项且 `row` 值全部字符串化/去尾零。 | `go test ./internal/kline/ -run TestRowUniRow` 通过 | |
| 10263 | `PATCH /Batch` 区分「未传」与「显式 null」。 | `go test ./internal/kline/ -run TestPatchExplicitNull` 通过 | |

## 二十六、涨跌字段补全规则（10264–10269）

> 来源：导入/写入时 `change_price`、`change_ratio` 缺失的自动补全规则（ACANX 2026-10-24）。
> 关联：`internal/calc/change.go`、`internal/mvsv/parser.go`、`internal/quote/service.go`、`internal/kline/request.go`。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10264 | MVSV 文件的 `cp`/`cr` 列有值时按文件值落库，不做重算。 | `go test ./internal/mvsv/ -run TestParseChangeFields` 通过；导入含 `cp/cr` 的文件后直查两列等于文件值 | |
| 10265 | `cp`/`cr` 均缺失且 `last_close` 非空非 0 时，用 `last_close` 作为参考价计算。 | `go test ./internal/quote/ -run TestComputeChangeFields` 通过（用例 2） | |
| 10266 | `last_close` 缺失或为 0 且 `open` 非空非 0 时，用 `open` 作为参考价计算。 | 同上（用例 3、4）通过 | |
| 10267 | `last_close` 与 `open` 都不可用时保持 NULL，不报错。 | 同上（用例 5）通过 | |
| 10268 | KLine API 写入未提供 `change_price`/`change_ratio` 时按 `open` 补全；任一列有值时不修改。 | `grep -c "calc.Change" internal/kline/request.go` 输出不小于 `1`；`go test ./internal/kline/` 通过 | |
| 10269 | 涨跌计算使用十进制精确运算（big.Rat）并保留 6 位小数。 | `go test ./internal/calc/ -run TestChangeRule` 通过（含 6 位小数用例） | |

## 二十七、去重行情前端菜单与统计（10270–10275）

> 来源：去重行情（`dedup` schema）前端查询与统计可视化需求（ACANX 2026-10-24）。
> 关联：`Web/src/App.vue`、`Web/src/router.ts`、`Web/src/views/Meta/Finv/Quote/DeDup/*`、`internal/kline/stats.go`、`internal/api/handler/kline.go`。

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
| 10291 | 迁移 `V41` 在 `dedup` schema 新增 12 张归档链路表。 | `grep -c 'CREATE TABLE IF NOT EXISTS dedup.finv_dedup_' Deploy/Migrations/V41__finv_dedup_export_archive_tables.sql` 输出 `12` | |
| 10292 | 新增表全部位于 `dedup` schema 且全库无外键，不修改既有两张业务表。 | `grep -c 'REFERENCES' Deploy/Migrations/V41__finv_dedup_export_archive_tables.sql` 输出 `0`；`grep -c 'ALTER TABLE dedup.finv_quote_secu_kline_min' Deploy/Migrations/V41__finv_dedup_export_archive_tables.sql` 输出 `0` | |
| 10293 | 12 个普通索引按 `idx_<表名去 finv_ 前缀并去下划线>_<用途>` 命名。 | `grep -c 'CREATE INDEX IF NOT EXISTS idx_dedup' Deploy/Migrations/V41__finv_dedup_export_archive_tables.sql` 输出 `12` | |
| 10294 | 导出文件为 MVsv：16 行 `# ` 文件头 + 1 行空行 + 数据行。 | `go test ./internal/meta/finv/quote/dedup -run TestWriteMVsvStructure` 通过 | |
| 10295 | `# Field` 行固定为 `ts|da|ti|o|h|l|c|v|t|cp|cr|lc`。 | `go test ./internal/meta/finv/quote/dedup -run TestWriteMVsvStructure` 通过（断言 Field 行） | |
| 10296 | `# Count` 等于实际数据行数，不一致时写出失败且不留半成品。 | `go test ./internal/meta/finv/quote/dedup -run 'TestWriteMVsvRowCountMismatch|TestVerifyMVsvRejectsBrokenFiles'` 通过 | |
| 10297 | BASE / PATCH / MANIFEST 文件名符合方案稿约定。 | `go test ./internal/meta/finv/quote/dedup -run TestFileNames` 通过 | |
| 10298 | 上传与本地同名（`.mvsv` / `.json`），不生成、不支持 `.csv`。 | `grep -rn '\.csv' internal/meta/finv/quote/dedup` 无输出；`go test ./internal/meta/finv/quote/dedup -run TestFileNames` 通过 | |
| 10299 | 导出任务以 `(usc, biz_month)` 唯一，保证幂等。 | `grep -F 'UNIQUE (usc, biz_month)' Deploy/Migrations/V41__finv_dedup_export_archive_tables.sql` 命中 | |
| 10300 | LOAD 按日 keyset 分页、`ON CONFLICT (ts, usc) DO NOTHING` 幂等、按 `usc` + 日期范围清理。 | `grep -F 'ON CONFLICT (ts, usc) DO NOTHING' internal/meta/finv/quote/dedup/store.go` 命中；`grep -F 'DELETE FROM ' internal/meta/finv/quote/dedup/store.go` 命中 | |
| 10301 | 窗口耗尽时保存游标转 `PAUSED`（预算只限制启动新任务，不中断已启动任务），续跑前校验 `loaded_rows` 与实际行数不一致则重置游标。 | `grep -F 'ResetLoadCursor' internal/meta/finv/quote/dedup/job_export.go` 命中；`grep -F 'JobStatusPaused' internal/meta/finv/quote/dedup/job_export.go` 命中 | |
| 10302 | 多 worker 用行锁 + 租约 + 心跳安全领取（`FOR UPDATE SKIP LOCKED`）。 | `grep -F 'FOR UPDATE SKIP LOCKED' internal/meta/finv/quote/dedup/store.go` 命中；`grep -F 'make_interval(secs' internal/meta/finv/quote/dedup/store.go` 命中 | |
| 10303 | 日预算按公式计算并落库（`factor = clamp(1.20 + 0.20*sla + 0.05*min(backlog,5), 1.20, 2.00)`）。 | `go test ./internal/meta/finv/quote/dedup -run TestComputeBudget` 通过 | |
| 10304 | 导出候选要求该月所有已观测日均为 `COMPLETE`/`FROZEN` 且月份已结束。 | `grep -F "HAVING bool_and(m.maturity_status IN ('COMPLETE', 'FROZEN'))" internal/meta/finv/quote/dedup/store.go` 命中；`grep -F 'm.biz_month < $1' internal/meta/finv/quote/dedup/store.go` 命中 | |
| 10305 | 每个物理文件登记到 `finv_dedup_export_file`，唯一键 `(usc, biz_month, file_type, version)`。 | `grep -F 'UNIQUE (usc, biz_month, file_type, version)' Deploy/Migrations/V41__finv_dedup_export_archive_tables.sql` 命中 | |
| 10306 | MANIFEST 在同一证券月 BASE / PATCH 全部上传完成后才领取。 | `grep -F "f.file_type <> 'MANIFEST' OR NOT EXISTS" internal/meta/finv/quote/dedup/store_files.go` 命中 | |
| 10307 | 上传前后校验大小与 SHA256，每次尝试写上传审计日志。 | `grep -F 's.uploader.Verify' internal/meta/finv/quote/dedup/job_upload.go` 命中；`grep -F 'InsertUploadLog' internal/meta/finv/quote/dedup/job_upload.go` 命中 | |
| 10308 | 收口后数据按 L1_HOT / L2_WARM / L3_COLD 分级补丁，并登记 `finv_dedup_export_file_patch`。 | `grep -F 'PatchLevelL1Hot' internal/meta/finv/quote/dedup/job_patch.go` 命中；`grep -F 'InsertPatchTask' internal/meta/finv/quote/dedup/job_patch.go` 命中 | |
| 10309 | 生命周期清理默认不删主表数据，需显式开关；本地文件需「上传成功 + 远端校验 + 网盘归档」才清理。 | `grep -F 'FINV_QUOTE_DEDUB_PURGE_MAIN_ENABLED' internal/config/config.go` 命中；`grep -F "network_status = 'ARCHIVED'" internal/meta/finv/quote/dedup/store_files.go` 命中 | |
| 10310 | 健康检查输出指标与告警，并登记 10 个 Job 的调度表。 | `go test ./internal/meta/finv/quote/dedup -run 'TestBuildAlerts|TestJobRuntimes'` 通过 | |

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
| **合计** | **10001–10320** | **320** |

> 验收方式：按编号顺序逐条核对，`√` = 满足，`×` = 不满足并记录差异。
> 编号连续、不跳号；新增需求从 `10321` 起往后追加，不插入既有区间。
