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
| 10056 | 两个改造清单文件分别位于 `Docs/Refactor/2610-SecuCodeToUsc.md` 与 `Docs/Refactor/2610-PrevCloseToLastClose.md`。 | 两个路径 `test -f` 退出码均为 0 | |
| 10057 | 改造项 A 清单编号自 `A-01` 起连续无缺号。 | `2610-SecuCodeToUsc.md` 中提取 `A-\d{2}` 编号后相邻差值恒为 1 | |
| 10058 | 改造项 B 清单编号自 `B-01` 起连续无缺号。 | `2610-PrevCloseToLastClose.md` 中提取 `B-\d{2}` 编号后相邻差值恒为 1 | |
| 10059 | 两个文件合计包含 4 项待决策（`DA-01`、`DB-01`～`DB-03`）。 | 两文件中 `DA-0[0-9]` 与 `DB-0[0-9]` 表格行合计为 4 | |
| 10060 | `README.md` 文档列表包含指向两个改造清单的链接。 | `grep -c 'Docs/Refactor/' README.md` 输出不小于 2 | |
| 10061 | 改造项 B 清单已声明示例数据仅供参考、权威判定以生产库 SQL 为准。 | `grep -c "示例文件不是生产数据，不能作为删除依据" Docs/Refactor/2610-PrevCloseToLastClose.md` 输出 `1` | |
| 10062 | 改造项 B 的 `DB-01` 记录了「本次只改名、不删列」的结论。 | `grep -c "本次只改名，不删列" Docs/Refactor/2610-PrevCloseToLastClose.md` 输出 `1` | |
| 10063 | 改造项 B 的重命名迁移与回填迁移各有一条清单条目。 | 命中 `V30__finv_quote_kline_rename_prev_close_to_last_close` 与 `V31__finv_quote_kline_backfill_open_from_last_close` 各 1 处 | |
| 10064 | `Docs/Refactor/` 下的过程文档文件名均带 `yyMM-` 四位前缀。 | `ls Docs/Refactor/*.md` 结果全部匹配 `^[0-9]{4}-` | |
| 10065 | `FileNamingSpec.md` 记录了 `yyMM-` 前缀规则及其例外清单。 | `grep -c "yyMM-" Docs/DevSpec/FileNamingSpec.md` 输出不小于 2 | |

## 六、改造项 A 验收（10066–10079）

> 对应清单 [`Refactor/2610-SecuCodeToUsc.md`](Refactor/2610-SecuCodeToUsc.md)：`secu_code` → `usc` 全库统一。

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
| 10076 | 升级文档声明新旧镜像不可共存。 | `grep -c "禁止新旧镜像共存" Deploy/Win11DockerUpgrade.md` 输出 `1` | |
| 10077 | 前端构建产物中不含旧字段名。 | `grep -rc "secu_code" Web/dist/assets/*.js` 全为 `0` | |
| 10078 | `V29` 同时覆盖两个审计表的索引改名（执行中新发现，原清单 `A-02` 只列 3 个）。 | 脚本内包含 `idx_finv_quote_ingest_batches_usc` 与 `idx_finv_quote_revision_log_usc` | |
| 10079 | All-in-One 内嵌产物 `internal/webui/dist` 已按新前端重新生成（执行中新发现，清单原未列）。 | `grep -rc "secu_code" internal/webui/dist/assets/*.js` 全为 `0` | |

## 七、改造项 B 验收（10080–10091）

> 对应清单 [`Refactor/2610-PrevCloseToLastClose.md`](Refactor/2610-PrevCloseToLastClose.md)：`prev_close` → `last_close` 重命名 + `open` 兜底回填。

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

> 对应清单 [`Refactor/2610-MoveTablesToSchemas.md`](Refactor/2610-MoveTablesToSchemas.md)：`finv_quote_*` → `quote`、`finv_quant_*` → `quant` schema。

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

> 对应清单 [`Refactor/2610-MoveFutuTablesToSchema.md`](Refactor/2610-MoveFutuTablesToSchema.md)：`finv_futu_*` → `futu` schema。

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
| **合计** | **10001–10124** | **124** |

> 验收方式：按编号顺序逐条核对，`√` = 满足，`×` = 不满足并记录差异。
> 编号连续、不跳号；新增需求从 `10125` 起往后追加，不插入既有区间。
