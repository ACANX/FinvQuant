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
| 10063 | 改造项 B 的重命名迁移与回填迁移各有一条清单条目。 | 命中 `V100030__finv_quote_kline_rename_prev_close_to_last_close` 与 `V100031__finv_quote_kline_backfill_open_from_last_close` 各 1 处 | |
| 10064 | `Docs/Refactor/` 下的过程文档文件名均带 `yyMM-` 四位前缀。 | `ls Docs/Refactor/*.md` 结果全部匹配 `^[0-9]{4}-` | |
| 10065 | `FileNamingSpec.md` 记录了 `yyMM-` 前缀规则及其例外清单。 | `grep -c "yyMM-" Docs/DevSpec/FileNamingSpec.md` 输出不小于 2 | |

## 六、改造项 A 验收（10066–10079）

> 对应清单 [`Refactor/2610-001-SecuCodeToUsc.md`](Refactor/2610-001-SecuCodeToUsc.md)：`secu_code` → `usc` 全库统一。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10066 | 存在 `V29` 改名迁移且为单事务脚本。 | `grep -c "^BEGIN;" Deploy/Migrations/V100029__rename_secu_code_to_usc.sql` 与 `grep -c "^COMMIT;"` 均为 `1` | |
| 10067 | `V29` 覆盖 6 张表的 `secu_code` → `usc` 改名。 | 脚本内表名数组包含 `finv_quote_secu_kline_min`、`finv_quote_ingest_batches`、`finv_quote_revision_log`、`finv_quant_backtest_strategy`、`finv_quant_backtest_run`、`finv_quant_backtest_run_archive` 共 6 项 | |
| 10068 | `V29` 对 5 个依赖索引执行改名。 | `grep -c "idx_finv_quote_usc_ts\|idx_finv_quote_ingest_batches_usc\|idx_finv_quote_revision_log_usc\|idx_finv_quant_backtest_strategy_usc\|idx_finv_quant_backtest_run_usc" Deploy/Migrations/V100029__rename_secu_code_to_usc.sql` 输出不小于 `5` | |
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
| 10080 | 存在 `V30` 改名迁移且为单事务脚本。 | `grep -c "^BEGIN;" Deploy/Migrations/V100030__finv_quote_kline_rename_prev_close_to_last_close.sql` 与 `grep -c "^COMMIT;"` 均为 `1` | |
| 10081 | `V30` 以幂等方式把 `prev_close` 改名为 `last_close`。 | 脚本内同时含 `information_schema.columns`、`column_name = 'prev_close'` 与 `RENAME COLUMN prev_close TO last_close` | |
| 10082 | `V30` 为 `last_close` 写入口径注释。 | `grep -c "与 open 独立" Deploy/Migrations/V100030__finv_quote_kline_rename_prev_close_to_last_close.sql` 输出 `1` | |
| 10083 | `V31` 回填条件完整（缺失/0 值 + `last_close` 非空非 0）。 | 脚本内出现 `open IS NULL OR open = 0`、`last_close IS NOT NULL`、`last_close <> 0` 三处条件 | |
| 10084 | `V31` 输出回填行数。 | `grep -c "RAISE NOTICE" Deploy/Migrations/V100031__finv_quote_kline_backfill_open_from_last_close.sql` 不小于 `2` | |
| 10085 | `V31` 向 `finv_quote_revision_log` 写入留痕记录。 | 脚本内含 `MIGRATION_V31` 与 `INSERT INTO finv_quote_revision_log` | |
| 10086 | `V31` 明确声明回填不可逆。 | `grep -c "不可逆" Deploy/Migrations/V100031__finv_quote_kline_backfill_open_from_last_close.sql` 不小于 `1` | |
| 10087 | Go 代码中不再用旧列名（MVSV 文件头文本与 ATR 局部变量除外）。 | `grep -rn "prev_close" internal/ --include="*.go"` 输出为空 | |
| 10088 | 导入链路按同一规则增量兜底 `open`。 | `grep -c "fillOpenFromLastClose" internal/meta/finv/quote/service.go` 不小于 `2` | |
| 10089 | 数据格式与接口文档已改用 `last_close`。 | `grep -c "last_close" Docs/DataFormat/MvsvFileFormat.md Docs/API/HistoryQuote/HistoryQuote.md` 各不小于 `1` | |
| 10090 | 前端源码不含该字段引用。 | `grep -rn "prev_close\|prevClose" Web/src/` 输出为空 | |
| 10091 | `CHANGELOG` 记录改名、回填与不可逆提示。 | `grep -c "last_close" Docs/CHANGELOG.md` 不小于 `2` | |

## 八、改造项 C 验收（10092–10107）

> 对应清单 [`Refactor/2610-003-MoveTablesToSchemas.md`](Refactor/2610-003-MoveTablesToSchemas.md)：`finv_quote_*` → `quote`、`finv_quant_*` → `quant` schema。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10092 | 存在 `V32` 迁移且创建两个业务 schema。 | `grep -c "CREATE SCHEMA IF NOT EXISTS" Deploy/Migrations/V100032__move_finv_tables_to_quote_quant_schemas.sql` 输出 `2` | |
| 10093 | `V32` 覆盖 16 张表的迁移（3 张 quote + 13 张 quant，通过数组循环执行 `SET SCHEMA`）。 | `grep -oE "'finv_(quote\|quant)_[a-z_]+" Deploy/Migrations/V100032__move_finv_tables_to_quote_quant_schemas.sql \| sort -u \| wc -l` 输出 `16` | |
| 10094 | `V32` 设置数据库默认 `search_path`。 | 脚本内含 `ALTER DATABASE` 与 `current_database()` | |
| 10095 | `V32` 具备收敛校验，失败整体回滚。 | `grep -c "RAISE EXCEPTION" Deploy/Migrations/V100032__move_finv_tables_to_quote_quant_schemas.sql` 不小于 `2` | |
| 10096 | 存在 `V100024` 收尾迁移，处理种子段新建的 3 张 quant 表。 | `test -f Deploy/Migrations/V200024__finv_quant_seed_tables_move_to_quant_schema.sql` | |
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
| 10107 | 历史种子迁移的列名例外已登记。 | `grep -c "V100019" Docs/DevSpec/DbSchemaSpec.md Docs/CHANGELOG.md` 各不小于 `1` | |

## 九、改造项 D 验收（10108–10120）

> 对应清单 [`Refactor/2610-004-MoveFutuTablesToSchema.md`](Refactor/2610-004-MoveFutuTablesToSchema.md)：`finv_futu_*` → `futu` schema。

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10108 | 存在 `V33` 迁移且创建 `futu` schema。 | `grep -c "CREATE SCHEMA IF NOT EXISTS futu" Deploy/Migrations/V100033__move_finv_futu_tables_to_futu_schema.sql` 输出 `1` | |
| 10109 | `V33` 覆盖 5 张富途映射表。 | `grep -oE "'finv_futu_[a-z_]+" Deploy/Migrations/V100033__move_finv_futu_tables_to_futu_schema.sql \| sort -u \| wc -l` 输出 `5` | |
| 10110 | `V33` 更新数据库默认 `search_path` 含 `futu`。 | 脚本含 `public, quote, quant, futu` | |
| 10111 | `V33` 具备收敛校验与回滚。 | `grep -c "RAISE EXCEPTION" Deploy/Migrations/V100033__move_finv_futu_tables_to_futu_schema.sql` 不小于 `2` | |
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
| 10159 | 存在创建两个 schema 的迁移且可重复执行。 | `test -f Deploy/Migrations/V100034__create_export_and_sche_schemas.sql`；文件内含 `CREATE SCHEMA IF NOT EXISTS export` 与 `... sche` | |
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
| 10167 | 存在创建转存表的迁移。 | `test -f Deploy/Migrations/V100035__create_export_staging_kline_table.sql` | |
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
| 10176 | 存在创建 dedup schema 的迁移且幂等。 | `test -f Deploy/Migrations/V100036__create_dedup_schema.sql`；含 `CREATE SCHEMA IF NOT EXISTS dedup` | |
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
| 10184 | 存在创建 archive schema 的迁移且幂等。 | `test -f Deploy/Migrations/V100037__create_archive_schema.sql`；含 `CREATE SCHEMA IF NOT EXISTS archive` | |
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
| 10245 | 迁移 V38 将主表从 `quote` 迁入 `dedup` 并可重复执行。 | `ls Deploy/Migrations/V100038__finv_quote_kline_move_to_dedup.sql` 不存在时退出码非 0；脚本含 `SET SCHEMA dedup` 与 `IF EXISTS` 幂等判断 | |
| 10246 | 迁移 V38 补齐 `change_price`/`change_ratio`/`provider` 三列。 | `grep -c "ADD COLUMN IF NOT EXISTS change_price" Deploy/Migrations/V100038__finv_quote_kline_move_to_dedup.sql` 输出 `1` | |
| 10247 | `export.finv_quote_secu_kline_min` 按 dedup 新结构重建。 | `grep -c "LIKE dedup.finv_quote_secu_kline_min" Deploy/Migrations/V100038__finv_quote_kline_move_to_dedup.sql` 输出 `1` | |
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
| 10291 | 迁移 `V41` 在 `dedup` schema 新增 12 张归档链路表。 | `grep -c 'CREATE TABLE IF NOT EXISTS dedup.finv_dedup_' Deploy/Migrations/V100041__finv_dedup_export_archive_tables.sql` 输出 `12` | |
| 10292 | 新增表全部位于 `dedup` schema 且全库无外键，不修改既有两张业务表。 | `grep -c 'REFERENCES' Deploy/Migrations/V100041__finv_dedup_export_archive_tables.sql` 输出 `0`；`grep -c 'ALTER TABLE dedup.finv_quote_secu_kline_min' Deploy/Migrations/V100041__finv_dedup_export_archive_tables.sql` 输出 `0` | |
| 10293 | 12 个普通索引按 `idx_<表名去 finv_ 前缀并去下划线>_<用途>` 命名。 | `grep -c 'CREATE INDEX IF NOT EXISTS idx_dedup' Deploy/Migrations/V100041__finv_dedup_export_archive_tables.sql` 输出 `12` | |
| 10294 | 导出文件为 MVsv：16 行 `# ` 文件头 + 1 行空行 + 数据行。 | `go test ./internal/meta/finv/quote/dedup -run TestWriteMVsvStructure` 通过 | |
| 10295 | `# Field` 行固定为 `ts\|da\|ti\|o\|h\|l\|c\|v\|t\|cp\|cr\|lc`。 | `go test ./internal/meta/finv/quote/dedup -run TestWriteMVsvStructure` 通过（断言 Field 行） | |
| 10296 | `# Count` 等于实际数据行数，不一致时写出失败且不留半成品。 | `go test ./internal/meta/finv/quote/dedup -run 'TestWriteMVsvRowCountMismatch\|TestVerifyMVsvRejectsBrokenFiles'` 通过 | |
| 10297 | BASE / PATCH / MANIFEST 文件名符合方案稿约定。 | `go test ./internal/meta/finv/quote/dedup -run TestFileNames` 通过 | |
| 10298 | 上传与本地同名（`.mvsv` / `.json`），不生成、不支持 `.csv`。 | `grep -rn '\.csv' internal/meta/finv/quote/dedup` 无输出；`go test ./internal/meta/finv/quote/dedup -run TestFileNames` 通过 | |
| 10299 | 导出任务以 `(usc, biz_month)` 唯一，保证幂等。 | `grep -F 'UNIQUE (usc, biz_month)' Deploy/Migrations/V100041__finv_dedup_export_archive_tables.sql` 命中 | |
| 10300 | LOAD 按日 keyset 分页、`ON CONFLICT (ts, usc) DO NOTHING` 幂等、按 `usc` + 日期范围清理。 | `grep -F 'ON CONFLICT (ts, usc) DO NOTHING' internal/meta/finv/quote/dedup/store.go` 命中；`grep -F 'DELETE FROM ' internal/meta/finv/quote/dedup/store.go` 命中 | |
| 10301 | 窗口耗尽时保存游标转 `PAUSED`（预算只限制启动新任务，不中断已启动任务），续跑前校验 `loaded_rows` 与实际行数不一致则重置游标。 | `grep -F 'ResetLoadCursor' internal/meta/finv/quote/dedup/job_export.go` 命中；`grep -F 'JobStatusPaused' internal/meta/finv/quote/dedup/job_export.go` 命中 | |
| 10302 | 多 worker 用行锁 + 租约 + 心跳安全领取（`FOR UPDATE SKIP LOCKED`）。 | `grep -F 'FOR UPDATE SKIP LOCKED' internal/meta/finv/quote/dedup/store.go` 命中；`grep -F 'make_interval(secs' internal/meta/finv/quote/dedup/store.go` 命中 | |
| 10303 | 日预算按公式计算并落库（`factor = clamp(1.20 + 0.20*sla + 0.05*min(backlog,5), 1.20, 2.00)`）。 | `go test ./internal/meta/finv/quote/dedup -run TestComputeBudget` 通过 | |
| 10304 | 导出候选要求该月所有已观测日均为 `COMPLETE`/`FROZEN` 且月份已结束。 | `grep -F "HAVING bool_and(m.maturity_status IN ('COMPLETE', 'FROZEN'))" internal/meta/finv/quote/dedup/store.go` 命中；`grep -F 'm.biz_month < $1' internal/meta/finv/quote/dedup/store.go` 命中 | |
| 10305 | 每个物理文件登记到 `finv_dedup_export_file`，唯一键 `(usc, biz_month, file_type, version)`。 | `grep -F 'UNIQUE (usc, biz_month, file_type, version)' Deploy/Migrations/V100041__finv_dedup_export_archive_tables.sql` 命中 | |
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
| 10358 | 运行日志表新增 `stage` / `progress_percent` / `progress_detail` / `gmt_heartbeat` 四列并建未结束记录部分索引。 | `grep -F 'progress_percent' Deploy/Migrations/V100042__finv_dedup_job_run_log_progress.sql` 命中；`grep -F 'idx_dedupjobrunlogexportfile_running' Deploy/Migrations/V100042__finv_dedup_job_run_log_progress.sql` 命中 | |
| 10359 | 导出 Job 在 LOAD / EXPORT / CLEANUP 阶段上报进度与心跳（写文件时按行数周期性刷新）。 | `grep -F '"LOAD"' internal/meta/finv/quote/dedup/job_export.go` 与 `grep -F 'written%20000' internal/meta/finv/quote/dedup/job_export.go` 均命中 | |
| 10360 | 上传 / 补丁 / 维护 / 统计类 Job 上报阶段进度（UPLOAD / PATCH / CLEANUP / RECOVER / HEALTH / STATS / BUDGET / MATURITY / SEAL）。 | `grep -rlF 'reportProgress(ctx' internal/meta/finv/quote/dedup/job_*.go \| wc -l` 输出 `7` | |
| 10361 | 侧边菜单新增「端到端链路」独立入口与路由。 | `grep -F 'dedup-export-chain' Web/src/App.vue Web/src/router.ts` 命中 2 处；`test -f Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 退出码为 0 | |
| 10362 | 流程页以流程图展示去重 → 成熟度/预算 → 收口 → 装载 → 导出 → 清理 → 上传 → 归档 → 补丁 → 保障的全链路节点状态与进度。 | `grep -F '总体视角 · 数据流向' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 命中；`grep -c "kind: 'main'," Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 输出 `11` | |
| 10363 | 流程页自动刷新运行快照（5 秒）与业务指标（20 秒），并汇总当前执行环节与告警。 | `grep -F 'setInterval(loadRuntime, 5000)' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` 与 `grep -F 'statusTimer = setInterval(() =>'` 均命中 | |
| 10364 | 归档监控总览页的 Job 调度表保持「调度 + 手动触发」，不内联运行状态与历史记录。 | `grep -c 'Job/Runtime' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportOverviewView.vue` 输出 `0` | |
| 10365 | 低峰窗口支持运行时修改并立即生效（新增运行时设置表 + 服务端热更新）。 | `test -f Deploy/Migrations/V100043__finv_dedup_runtime_setting.sql` 退出码为 0；`grep -F 'SetWindow' internal/meta/finv/quote/dedup/service.go` 命中 | |
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
| 10388 | 按 CDEU 模型新增 8 张 `dedup.cdeu_*` 表并 seed 1 个流程 / 4 个阶段 / 10 个步骤 / 13 个作业（V45 将旁路步骤并入导出/上传阶段）；现有 Job 执行时双写 CDEU 运行树并按完成条件逐级评估。 | `test -f Deploy/Migrations/V100044__finv_dedup_cdeu_process_model.sql` 退出码为 0；`grep -F 'CDEU_DAILY' Deploy/Migrations/V100044__finv_dedup_cdeu_process_model.sql` 命中；`go test ./internal/meta/finv/quote/dedup` 通过 | |
| 10389 | 前端新增「流程控制与监控」菜单，按 Process / Phase / Step / Job 展示 CDEU 运行状态、历史运行与手动触发；后端新增 `GET /CDEU/Overview`。 | `grep -F '/Meta/Finv/Quote/Cdeu/CdeuOverview' Web/src/router.ts Web/src/config/menuConfig.ts` 命中；`grep -F 'CDEU/Overview' internal/api/router.go` 命中；`cd Web && npm run build` 通过 | |
| 10390 | 流程控制页默认「总体视图」，阶段悬浮展示概要；支持启动完整 CDEU 流程（`POST /CDEU/Process/Run`）。 | `grep -F 'viewMode' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'v-tooltip' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'CDEU/Process/Run' internal/api/router.go` 命中；`cd Web && npm run build` 通过 | |
| 10391 | 总体视图点击阶段后，在「阶段概览」位置展示该阶段的步骤清单表格（步骤状态、作业/批次进度、开始时间、耗时、异常），不再重复展示阶段汇总表。 | `grep -F 'selectedPhase' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'selectedPhase.steps' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10392 | 「阶段流转」改为画布式工作流：悬浮节点 + 点阵画布 + 阶段间有向箭头，箭头由前序阶段指向后续阶段，点击节点切换步骤清单。 | `grep -F 'workflow-canvas' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'marker-end' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10393 | 删除尾部「保障（旁路）」阶段，将超时回收步骤并入导出阶段、健康检查步骤并入上传阶段，与正常步骤并行展示且不参与完成判定。 | `test -f Deploy/Migrations/V100045__finv_dedup_cdeu_split_ops_steps.sql` 退出码为 0；`grep -F 'PHASE_OPS' Deploy/Migrations/V100045__finv_dedup_cdeu_split_ops_steps.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10394 | 在「阶段概览」步骤表格中点击某一步骤，下方展示该步骤定义的 JOB 清单（作业、调度标识、执行阶段、状态、开始时间、耗时、处理量、操作）。 | `grep -F 'selectedStep' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'selectedStep.jobs' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10395 | 运行中的阶段在画布节点上显示脉冲高亮/流光连线，运行中的步骤在步骤行与详情头部显示高亮与呼吸动效，便于一眼定位当前执行位置。 | `grep -F 'workflow-node-running' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'step-row-running' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10396 | 将「运行历史」改为「最新运行」位置的下拉列表，支持选择历史运行切换；默认展示最近一次运行，启动流程后展示当前运行。 | `grep -F 'run-select-btn' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'previousRuns' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10397 | 所有执行中的状态图标改为旋转动效（阶段画布、步骤表格、详情、JOB 清单、历史列表），并遵循 reduced-motion 设置。 | `grep -F 'spin-icon' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F "mdi-loading" Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10398 | 状态图例移入「阶段流转」画布卡片，支持显示/隐藏，并移除右侧独立图例卡片。 | `grep -F 'legendVisible' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'workflow-legend' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10399 | 移除独立「流程规则」卡片，将完成条件/失败策略并入页面头部摘要，时区/查询时间与内嵌阶段说明收进信息悬浮提示；步骤/ JOB 区域改为全宽展示。 | `grep -F '流程规则' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 应无输出；`grep -F '完成条件' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10400 | 阶段流转画布增加「开始」「结束」首尾节点，并用有向箭头与首个/最后一个阶段连接；开始节点随运行存在显示已开始，结束节点随流程终态显示完成/失败/等待。 | `grep -F 'workflowStart' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'workflowEnd' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10401 | 数据采集阶段增加非关键「历史行情数据采集」占位步骤，步骤下无 JOB，流程初始化时自动成功，并保留后续接入历史采集 Job 的位置。 | `test -f Deploy/Migrations/V100046__finv_dedup_cdeu_history_ingest_step.sql` 退出码为 0；`grep -F 'STEP_HISTORY_INGEST' Deploy/Migrations/V100046__finv_dedup_cdeu_history_ingest_step.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10402 | 阶段步骤表将「步骤名称」「步骤 KEY」拆为两列，JOB 表将「作业名称」「调度标识」拆为两列；四列均支持拖拽调整列宽。 | `grep -F 'stepNameWidth' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'col-resizer' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10403 | 在去重收口与导出之间增加 `PHASE_DECISION`「导出判定」阶段及 `STEP_EXPORT_DECISION` 占位步骤，完成导出前检查判定；当前无 JOB，初始化自动成功。 | `test -f Deploy/Migrations/V100047__finv_dedup_cdeu_decision_phase.sql` 退出码为 0；`grep -F 'PHASE_DECISION' Deploy/Migrations/V100047__finv_dedup_cdeu_decision_phase.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10404 | JOB 清单不再把名称/触发方式/执行次数/异常堆叠在「作业名称」单元格：拆为作业名称、作业编码、调度标识、触发方式、执行次数、异常等独立列；作业名称/编码/调度标识支持列宽拖拽。 | `grep -F 'jobCodeWidth' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F '执行次数' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10405 | JOB 清单「执行阶段」列文字颜色统一为暖色 `rgb(251, 140, 0)`，阶段详情中的执行阶段标签同步。 | `grep -F 'rgb(251, 140, 0)' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10406 | 将数据采集阶段的 `STEP_INGEST_STATS`、`STEP_BUDGET` 调整到 `PHASE_DECISION` 导出判定阶段执行，JOB 随步骤迁移。 | `test -f Deploy/Migrations/V100048__finv_dedup_cdeu_move_stats_to_decision.sql` 退出码为 0；`grep -F 'STEP_INGEST_STATS' Deploy/Migrations/V100048__finv_dedup_cdeu_move_stats_to_decision.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10407 | 将 `STEP_MATURITY` 从去重收口阶段调整到 `PHASE_DECISION` 导出判定阶段执行，JOB 随步骤迁移；去重收口保留收口冻结。 | `test -f Deploy/Migrations/V100049__finv_dedup_cdeu_move_maturity_to_decision.sql` 退出码为 0；`grep -F 'STEP_MATURITY' Deploy/Migrations/V100049__finv_dedup_cdeu_move_maturity_to_decision.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10408 | CDEU 工作流模型重构为 `Workflow → Phase → Process → JOB → Step`：旧 Process 改为 Workflow、旧 Step 改为 Process、JOB 改为 JOB，并在 JOB 下新增串行 Step；数据库表/字段、API JSON、前端表述与启动路径同步调整。 | `test -f Deploy/Migrations/V100050__finv_dedup_cdeu_workflow_process_jobstep.sql` 退出码为 0；`grep -F 'cdeu_job_step' Deploy/Migrations/V100050__finv_dedup_cdeu_workflow_process_jobstep.sql` 命中；`grep -F 'workflow' internal/meta/finv/quote/dedup/cdeu_admin.go` 命中；`grep -F 'Process KEY' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup ./internal/api` 通过 | |
| 10409 | 将「收口冻结与差异报告」统一改名为「收口冻结与差异报告生成」，覆盖 Process、JOB、JOB Step 与运行快照展示名称。 | `test -f Deploy/Migrations/V100051__finv_dedup_cdeu_rename_seal_process.sql` 退出码为 0；`grep -F '收口冻结与差异报告生成' Deploy/Migrations/V100051__finv_dedup_cdeu_rename_seal_process.sql` 命中；`grep -F '收口冻结与差异报告生成' internal/meta/finv/quote/dedup/service.go` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10410 | 将 `PHASE_DECISION` 阶段名称由「导出判定」调整为「导出决策」。 | `test -f Deploy/Migrations/V100052__finv_dedup_cdeu_rename_decision_phase.sql` 退出码为 0；`grep -F "name = '导出决策'" Deploy/Migrations/V100052__finv_dedup_cdeu_rename_decision_phase.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10411 | 将 `PHASE_COLLECT` 命名为「历史行情采集」、`PHASE_DEDUP` 命名为「历史行情去重」，并在上传后新增 `PHASE_REVIEW`「归档复核」阶段（占位 Process，初始化自动成功）。 | `test -f Deploy/Migrations/V100053__finv_dedup_cdeu_phase_rename_review.sql` 退出码为 0；`grep -F "PHASE_REVIEW" Deploy/Migrations/V100053__finv_dedup_cdeu_phase_rename_review.sql` 命中；`grep -F "历史行情采集" Deploy/Migrations/V100053__finv_dedup_cdeu_phase_rename_review.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10412 | 「阶段流转」标题改为「CDEU工作流」；阶段间距扩大一倍，同时缩小阶段间箭头，使箭头之间的线段有足够展示宽度。 | `grep -F 'CDEU工作流' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'const WORKFLOW_GAP = 48' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'markerWidth="5"' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10413 | 画布「开始」「结束」节点的 icon 放在文字左侧，文字改为上下两行（标题 / 状态）；终端节点宽度同步扩大以容纳横向布局。 | `grep -F 'workflow-terminal-text' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'const WORKFLOW_TERMINAL_WIDTH = 96' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10414 | 将 `PHASE_EXPORT` 名称改为「月度行情导出」、`PHASE_UPLOAD` 名称改为「月度行情上传」。 | `test -f Deploy/Migrations/V100054__finv_dedup_cdeu_rename_export_upload_phases.sql` 退出码为 0；`grep -F "PHASE_EXPORT" Deploy/Migrations/V100054__finv_dedup_cdeu_rename_export_upload_phases.sql` 命中；`grep -F "月度行情导出" Deploy/Migrations/V100054__finv_dedup_cdeu_rename_export_upload_phases.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10415 | 前端「阶段概览」改为「流程概览」；Process 下的 JOB 标题改为「JOB 清单 · `<Process 名称>`」顺序。 | `grep -F '流程概览' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'JOB 清单 · {{ selectedProcess.name }}' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10416 | Process 表格将「关键」/「旁路」标识从 Process 名称单元格拆出，单独增加「标识」列展示。 | `grep -F '>标识<' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F '>关键<' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10417 | 移除流程概览标题右侧的「点击上方阶段切换 Process · 点击 Process 查看 JOB 清单 · 悬浮查看阶段概要」描述文字。 | `grep -F '点击上方阶段切换 Process' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 无输出；`cd Web && npm run build` 通过 | |
| 10418 | 工作流页头元信息由单行文字改为带图标的彩色 chip 可视化：工作流名称、运行模式、每日调度、完成条件、失败策略。 | `grep -F 'workflow-meta-row' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F '完成条件' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10419 | 将 `PROCESS_EXPORT_JOB` 名称由「单证券月导出」改为「证券历史月度行情文件导出」。 | `test -f Deploy/Migrations/V100055__finv_dedup_cdeu_rename_export_process.sql` 退出码为 0；`grep -F "证券历史月度行情文件导出" Deploy/Migrations/V100055__finv_dedup_cdeu_rename_export_process.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10420 | 将 `PROCESS_PATCH` 编码改为 `PROCESS_EXPORT_FILE_MON_INCR`、名称改为「证券历史月度行情增量文件导出」；将 `PROCESS_EXPORT_JOB` 编码改为 `PROCESS_EXPORT_FILE_MON`。 | `test -f Deploy/Migrations/V100056__finv_dedup_cdeu_rename_export_file_processes.sql` 退出码为 0；`grep -F "PROCESS_EXPORT_FILE_MON_INCR" Deploy/Migrations/V100056__finv_dedup_cdeu_rename_export_file_processes.sql` 命中；`grep -F "PROCESS_EXPORT_FILE_MON" Deploy/Migrations/V100056__finv_dedup_cdeu_rename_export_file_processes.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10421 | 在 `PROCESS_EXPORT_FILE_MON` 下新增 JOB `JOB_FETCH_SECURITY_METADATA`「证券元数据获取」，作为导出 JOB 的 META 阶段，在 LOAD 前刷新证券元数据。 | `test -f Deploy/Migrations/V100057__finv_dedup_cdeu_add_security_metadata_job.sql` 退出码为 0；`grep -F "JOB_FETCH_SECURITY_METADATA" Deploy/Migrations/V100057__finv_dedup_cdeu_add_security_metadata_job.sql` 命中；`grep -F "RefreshSecurityMetadata" internal/meta/finv/quote/dedup/store.go` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10422 | 在导出阶段新增 `PROCESS_EXPORT_BATCH`「证券导出批次清单获取」Process 及 `JOB_FETCH_EXPORT_BATCH` JOB，生成当批次 `usc + biz_month + BASE/INCR` 清单并写入 `dedup.finv_dedup_export_batch`；BASE/INCR 导出流程分别读取对应条目。 | `test -f Deploy/Migrations/V100058__finv_dedup_cdeu_export_batch_process.sql` 退出码为 0；`grep -F "finv_dedup_export_batch" Deploy/Migrations/V100058__finv_dedup_cdeu_export_batch_process.sql` 命中；`grep -F "GenerateExportBatch" internal/meta/finv/quote/dedup/store.go` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10423 | 将 BASE 导出 Process 编码由 `PROCESS_EXPORT_FILE_MON` 改为 `PROCESS_EXPORT_FILE_MON_BASE`；同时修复历史已完成运行新增 Process 后 CDEU 只读登记找不到运行树的问题。 | `test -f Deploy/Migrations/V100059__finv_dedup_cdeu_rename_base_export_process.sql` 退出码为 0；`grep -F "PROCESS_EXPORT_FILE_MON_BASE" Deploy/Migrations/V100059__finv_dedup_cdeu_rename_base_export_process.sql` 命中；`grep -F "SeedCDEUProcessRuns" internal/meta/finv/quote/dedup/cdeu.go` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10424 | 将 `PHASE_COLLECT` 名称由「历史行情采集」改为「历史行情收集」。 | `test -f Deploy/Migrations/V100060__finv_dedup_cdeu_rename_collect_phase.sql` 退出码为 0；`grep -F "历史行情收集" Deploy/Migrations/V100060__finv_dedup_cdeu_rename_collect_phase.sql` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10425 | BASE/INCR 导出 Process 按批次清单动态生成单证券 JOB：以 `(usc, biz_month)` 全局唯一，写入 `dedup.cdeu_dynamic_job`；前端隐藏固定 LOAD/EXPORT/REGISTER/CLEANUP 执行壳 JOB，并展示证券、年月、BASE/INCR 类型及动态状态。 | `test -f Deploy/Migrations/V100061__finv_dedup_cdeu_dynamic_export_jobs.sql` 退出码为 0；`grep -F "cdeu_dynamic_job" Deploy/Migrations/V100061__finv_dedup_cdeu_dynamic_export_jobs.sql` 命中；`grep -F "GenerateDynamicJobs" internal/meta/finv/quote/dedup/store.go` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10426 | 每个动态单证券导出 JOB 内部拆为 `STEP_LOAD → STEP_EXPORT → STEP_REGISTER → STEP_CLEANUP` 四个串行 Step，运行时更新 Step 状态，前端展示对应 Step 信息。 | `test -f Deploy/Migrations/V100062__finv_dedup_cdeu_dynamic_job_steps.sql` 退出码为 0；`grep -F "cdeu_dynamic_job_step" Deploy/Migrations/V100062__finv_dedup_cdeu_dynamic_job_steps.sql` 命中；`grep -F "MarkDynamicJobStepStatus" internal/meta/finv/quote/dedup/store.go` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10427 | `cdeu_dynamic_job` 表改名为 `cdeu_job_dynamic`（Step 表同步为 `cdeu_job_dynamic_step`）；`PROCESS_UPLOAD` 基于已导出且未完成上传的月归档文件动态生成上传 JOB，每个文件一条并按 `file_id` 唯一，运行时同步 `STEP_UPLOAD` 状态。 | `test -f Deploy/Migrations/V100063__finv_dedup_cdeu_rename_job_dynamic_and_upload.sql` 退出码为 0；`grep -F "cdeu_job_dynamic" Deploy/Migrations/V100063__finv_dedup_cdeu_rename_job_dynamic_and_upload.sql` 命中；`grep -F "GenerateUploadDynamicJobs" internal/meta/finv/quote/dedup/store.go` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
| 10428 | 点击阶段后，在工作流画布下方展示该阶段内 Process 的拓扑顺序图；顺序按 `seq_no` 生成，起点连接上一阶段输出、终点连接下一阶段输入，Process 节点可点击选中。 | `grep -F 'process-topology' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F '上一阶段输出' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10429 | 阶段内 Process 拓扑支持「流程拓扑」开关显示/隐藏，默认显示；导出阶段将 `PROCESS_RECOVER` 作为与 `PROCESS_EXPORT_FILE_MON_BASE` 并行的支路展示。 | `grep -F 'processTopologyVisible' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'processTopologyHasParallel' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10430 | 导出决策阶段按依赖调整：入口流量统计与行情成熟度扫描并行，每日导出预算计算在其后；调度改为 00:05 启动统计/成熟度、00:10 启动预算，拓扑图展示并行汇合关系。 | `test -f Deploy/Migrations/V100064__finv_dedup_cdeu_decision_parallel_schedule.sql` 退出码为 0；`grep -F "JOB_CALC_BUDGET" Deploy/Migrations/V100064__finv_dedup_cdeu_decision_parallel_schedule.sql` 命中；`grep -F "processTopologyMode" Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`go test ./internal/database ./internal/meta/finv/quote/dedup` 通过 | |
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
| 10447 | 导出动态 JOB 的 `name` 字段统一为 `EXPORT_MON_{BASE/INCR}_{usc}_{biz_month}` 格式（如 `EXPORT_MON_BASE_GCMain_201912`），并回填已有 `cdeu_job_dynamic` 记录。 | `grep -F "EXPORT_MON_" internal/meta/finv/quote/dedup/store.go` 命中；`test -f Deploy/Migrations/V100065__finv_dedup_cdeu_dynamic_job_name.sql` 退出码为 0；`grep -F "EXPORT_MON_" Deploy/Migrations/V100065__finv_dedup_cdeu_dynamic_job_name.sql` 命中；`go test ./internal/meta/finv/quote/dedup ./internal/database` 通过 | |
| 10448 | 每个 JOB 运行提供详细执行日志：新增运行日志行表、`GET .../RunLog/Detail?runId=` 接口，CDEU JOB 行提供「执行日志」入口，弹窗展示阶段/进度/错误等逐行日志；运行中自动刷新。 | `test -f Deploy/Migrations/V100066__finv_dedup_job_run_log_line.sql` 退出码为 0；`grep -F 'RunLog/Detail' internal/api/router.go` 命中；`grep -F '执行日志' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`go test ./...` 通过；`cd Web && npm run build` 通过 | |
| 10449 | 低峰窗口支持 `00:00:00` ~ `23:59:59` 全天候测试；窗口按分钟粒度判定，`23:59` 分钟包含在窗口内，`HH:MM:SS` 输入兼容并归一为分钟保存。 | `grep -F '00:00:00 ~ 23:59:59' internal/meta/finv/quote/dedup/options.go` 命中；`grep -F 'fullDay' internal/meta/finv/quote/dedup/options_test.go` 命中；`go test ./internal/meta/finv/quote/dedup` 通过 | |
| 10450 | CDEU 控制台的 JOB 清单支持水平滚动：列总宽超出容器时出现横向滚动条，可查看右侧的执行阶段、状态、异常、操作等列。 | `grep -F 'job-table-scroll' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`grep -F 'overflow-x: auto' Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` 命中；`cd Web && npm run build` 通过 | |
| 10451 | 修复人工清理 CDEU 定义数据后 `StartCDEUWorkflow` 报「未登记 CDEU_DAILY 工作流定义」：新增幂等迁移重建缺失的 15 个 JOB 与 Job Step 定义。 | `test -f Deploy/Migrations/V100067__finv_dedup_cdeu_reseed_jobs.sql` 退出码为 0；`grep -F 'JOB_FETCH_EXPORT_BATCH' Deploy/Migrations/V100067__finv_dedup_cdeu_reseed_jobs.sql` 命中；`go test ./internal/database` 通过 | |
| 10452 | 上传动态 JOB 的编码/名称统一为 `JOB_UPLOAD_{FILE_TYPE}_V{VERSION}_{USC大写}_{YYYYMM}`（如 `JOB_UPLOAD_BASE_V1_GCMAIN_201910`）；调度标识由 `DYNAMIC_EXPORT` 改为 `DYNAMIC_UPLOAD`，并回填已有上传动态 JOB。 | `grep -F "JOB_UPLOAD_' \|\| f.file_type" internal/meta/finv/quote/dedup/store.go` 命中；`grep -F 'DYNAMIC_UPLOAD' internal/meta/finv/quote/dedup/cdeu_admin.go` 命中；`test -f Deploy/Migrations/V100068__finv_dedup_cdeu_upload_dynamic_name.sql` 退出码为 0；`go test ./internal/meta/finv/quote/dedup ./internal/database` 通过 | |
| 10453 | 上传动态 JOB 的 `name` 与 `job_code` 分离：`job_code` 为 `JOB_UPLOAD_BASE_V1_GCMAIN_201910`，`name` 为 `UPLOAD_BASE_V1_GCMain_201910`；回填已有上传动态 JOB。 | `grep -F "'UPLOAD_' \|\| f.file_type" internal/meta/finv/quote/dedup/store.go` 命中；`test -f Deploy/Migrations/V100069__finv_dedup_cdeu_upload_dynamic_display_name.sql` 退出码为 0；`grep -F "UPLOAD_' \|\| file_type" Deploy/Migrations/V100069__finv_dedup_cdeu_upload_dynamic_display_name.sql` 命中；`go test ./internal/meta/finv/quote/dedup ./internal/database` 通过 | |
| 10454 | 动态 JOB 关联执行日志与工作流运行：新增 `run_log_id` / `workflow_run_id` 字段；动态 JOB 完成后可在 JOB 清单查看执行日志；当前工作流 JOB 清单只展示本轮生成的动态 JOB，避免历史任务混入。 | `grep -F 'run_log_id = COALESCE' internal/meta/finv/quote/dedup/store.go` 命中；`grep -F 'workflow_run_id' internal/meta/finv/quote/dedup/cdeu_admin.go` 命中；`test -f Deploy/Migrations/V100070__finv_dedup_cdeu_dynamic_job_run_link.sql` 退出码为 0；`go test ./internal/meta/finv/quote/dedup ./internal/database` 通过 | |
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
| 10475 | 存在创建 `sys` schema 的迁移且幂等。 | `test -f Deploy/Migrations/V100071__create_sys_schema.sql`；含 `CREATE SCHEMA IF NOT EXISTS sys` | |
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
| 10483 | 存在创建 `sys.sys_config` 的迁移且幂等。 | `test -f Deploy/Migrations/V100072__sys_config.sql`；含 `CREATE TABLE IF NOT EXISTS sys.sys_config` 与 `CREATE OR REPLACE FUNCTION` | |
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
| 10491 | 存在创建 `sys.sys_dict` / `sys.sys_dict_sub` 的迁移且幂等。 | `test -f Deploy/Migrations/V100073__sys_dict.sql`；含两个 `CREATE TABLE IF NOT EXISTS sys.sys_` 与两个 `CREATE OR REPLACE FUNCTION` | |
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
| 10502 | 存在创建 `sys.sys_user` 的迁移且幂等（建表、序列初值、触发器均不因重跑而失效）。 | `test -f Deploy/Migrations/V100074__sys_user.sql`；含 `CREATE TABLE IF NOT EXISTS sys.sys_user`、`pg_get_serial_sequence` 与 `CREATE OR REPLACE FUNCTION` | |
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
| 10513 | 初始配置数据以种子段迁移形式提供（编号 `V200000+`）且幂等。 | `test -f Deploy/Migrations/V200025__sys_config_seed.sql`；含 `INSERT INTO sys.sys_config` 与 `ON CONFLICT (config_no) DO NOTHING` | |
| 10514 | 数据库存在 `1010` / `1020` / `1031` / `5001` / `5002` 五个配置项。 | `select count(*) from sys.sys_config where config_no in (1010,1020,1031,5001,5002)` 输出 `5` | |
| 10515 | `1020`（默认最大单页展示记录数）为整型配置：`data_type='1'`、`int_value=2000`。 | `select data_type, int_value from sys.sys_config where config_no=1020` 输出 `1`、`2000` | |
| 10516 | `1010` / `1031` / `5001` / `5002` 为字符配置：`data_type='2'`、`char_value='1'`。 | `select count(*) from sys.sys_config where config_no in (1010,1031,5001,5002) and data_type='2' and char_value='1'` 输出 `4` | |
| 10517 | 重跑不影响生产已调整的配置值（不覆盖）。 | 先 `update sys.sys_config set str_value='x' where config_no=1031`，再重新执行 `Deploy/Migrations/V200025__sys_config_seed.sql`，`select str_value from sys.sys_config where config_no=1031` 仍输出 `x` | |
| 10518 | 迁移末尾收敛校验通过并登记 `V100025`。 | `select success from schema_version where version='100025'` 输出 `t` | |

## 三十八、sys_dict 初始字典定义（10519–10524）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10519 | 字典定义数据以种子段迁移形式提供（`V200000+`）且幂等。 | `test -f Deploy/Migrations/V200026__sys_dict_seed.sql`；含 `INSERT INTO sys.sys_dict` 与 `ON CONFLICT (dict_no) DO NOTHING` | |
| 10520 | 数据库存在 `1001`–`1009`、`1301`、`3004`、`3101`、`3103`、`3104`、`3105`、`3107`、`3110`、`3121`、`3122` 共 19 个字典定义。 | `select count(*) from sys.sys_dict where dict_no in (1001,1002,1003,1004,1005,1006,1007,1008,1009,1301,3004,3101,3103,3104,3105,3107,3110,3121,3122)` 输出 `19` | |
| 10521 | 19 条字典的 `key_type` 全为 `'1'` 且 `gmt_create` / `gmt_modify` 均非空。 | `select count(*) from sys.sys_dict where key_type='1' and gmt_create is not null and gmt_modify is not null` 不小于 `19` | |
| 10522 | 关键字段抽查正确：`1005`（`val_type='2'`、`dict_category='5'`）、`3004`（`enum_class='mnp'`、`dict_category='F'`）、`3104`（`en_sub_sys='foc'`、`dict_category='T'`）、`3105`（`manage_level IS NULL`、`enum_class='ses'`）、`3121`（`en_sub_sys='act'`）。 | `select dict_no from sys.sys_dict where (dict_no=1005 and not (val_type='2' and dict_category='5')) or (dict_no=3004 and not (enum_class='mnp' and dict_category='F')) or (dict_no=3104 and not (en_sub_sys='foc' and dict_category='T')) or (dict_no=3105 and not (manage_level is null and enum_class='ses')) or (dict_no=3121 and en_sub_sys<>'act')` 输出 `0 行` | |
| 10523 | 重跑不影响生产已调整的字典定义（不覆盖）。 | 先 `update sys.sys_dict set remark='x' where dict_no=1001`，再重新执行 `Deploy/Migrations/V200026__sys_dict_seed.sql`，`select remark from sys.sys_dict where dict_no=1001` 仍输出 `x` | |
| 10524 | 迁移末尾收敛校验通过并登记 `V100026`。 | `select success from schema_version where version='100026'` 输出 `t` | |

## 三十九、sys_dict_sub 初始字典子项（10525–10531）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10525 | 字典子项数据以种子段迁移形式提供（`V200000+`）且幂等。 | `test -f Deploy/Migrations/V200027__sys_dict_sub_seed.sql`；含 `INSERT INTO sys.sys_dict_sub` 与 `ON CONFLICT (dict_no, sub_key) DO NOTHING` | |
| 10526 | 三组字典子项条数正确：`3120` = 14、`3121` = 11、`3122` = 64，共 89 条。 | `select dict_no, count(*) from sys.sys_dict_sub where dict_no in (3120,3121,3122) group by dict_no order by dict_no` 输出 `3120\|14`、`3121\|11`、`3122\|64` | |
| 10527 | `3120` 子键完整（`0`–`9`、`A`–`D`）且 `en_sys_str` 两种顺序正确：`0/1/2/8/9/A/B/C/D` 为 `gat,gax,gcq,gaa,tms,pbs`，`3/4/5/6/7` 为 `gat,gcq,gax,gaa,tms,pbs`。 | `select count(*) from sys.sys_dict_sub where dict_no=3120 and sub_key in ('0','1','2','8','9','A','B','C','D') and en_sys_str='gat,gax,gcq,gaa,tms,pbs'` 输出 `9`；`select count(*) from sys.sys_dict_sub where dict_no=3120 and sub_key in ('3','4','5','6','7') and en_sys_str='gat,gcq,gax,gaa,tms,pbs'` 输出 `5` | |
| 10528 | `3121` 共 11 条，`en_sys_str` 均为 `act`，`name_en` 均为 `NULL`，且包含 `HKD`→`香港元`。 | `select count(*) from sys.sys_dict_sub where dict_no=3121 and en_sys_str='act' and name_en is null` 输出 `11`；`select sub_val from sys.sys_dict_sub where dict_no=3121 and sub_key='HKD'` 输出 `香港元` | |
| 10529 | `3122` 共 64 条，含 `OTH`→`其它` 且其 `en_sys_str` 为 `NULL`，其余 `en_sys_str` 均为 `cic`。 | `select count(*) from sys.sys_dict_sub where dict_no=3122` 输出 `64`；`select en_sys_str from sys.sys_dict_sub where dict_no=3122 and sub_key='OTH'` 输出空值；`select count(*) from sys.sys_dict_sub where dict_no=3122 and sub_key<>'OTH' and en_sys_str='cic'` 输出 `63` | |
| 10530 | 89 条均满足 `flag_enable='0'`、`access_level=2`、`gmt_create` / `gmt_modify` 非空。 | `select count(*) from sys.sys_dict_sub where dict_no in (3120,3121,3122) and flag_enable='0' and access_level=2 and gmt_create is not null and gmt_modify is not null` 输出 `89` | |
| 10531 | 迁移末尾收敛校验通过并登记 `V100027`。 | `select success from schema_version where version='100027'` 输出 `t` | |

## 四十、sys_dict_sub 地区/国籍子项（10532–10538）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10532 | 地区/国籍子项数据以种子段迁移形式提供（`V200000+`）且幂等。 | `test -f Deploy/Migrations/V200028__sys_dict_sub_seed_area.sql`；含 `ON CONFLICT (dict_no, sub_key) DO NOTHING` | |
| 10533 | 三组条数不少于预期：`3103` = 249、`3104` = 5、`3105` = 133（共 387）。 | `select dict_no, count(*) from sys.sys_dict_sub where dict_no in (3103,3104,3105) group by dict_no order by dict_no` 输出 `3103\|249`、`3104\|5`、`3105\|133` | |
| 10534 | `3103` 首尾为 `ABW` 阿鲁巴 / `ZWE` 津巴布韦，且 `en_sys_str` 只出现 `trade`、`cic,gft,gaa,act`、`act,cic` 或 `NULL`。 | `select count(*) from sys.sys_dict_sub where dict_no=3103 and en_sys_str is not null and en_sys_str not in ('trade','cic,gft,gaa,act','act,cic')` 输出 `0` | |
| 10535 | `3103` 例外行正确：`ANT/ATF/BVT/COD/COK/DMA/ERI/EST/FLK/FSM/GEO/GLP/HMD/IOT/KGZ/LVA/MDA/MKD/MNP/MTQ/MYT/PLW/REU/SGS/SHN/SJM/SPM/TCA/TJK/UMI/UZB/VAT/VCT/WLF/WSM` 为 `access_level=2` 且 `en_sys_str IS NULL`；`CCK` 为 `1/NULL`；`CSR/PST/TMP` 为 `2/cic,gft,gaa,act`；`GGY` 为 `2/act,cic`。 | `select count(*) from sys.sys_dict_sub where dict_no=3103 and sub_key in ('ANT','ATF','BVT','COD','COK','DMA','ERI','EST','FLK','FSM','GEO','GLP','HMD','IOT','KGZ','LVA','MDA','MKD','MNP','MTQ','MYT','PLW','REU','SGS','SHN','SJM','SPM','TCA','TJK','UMI','UZB','VAT','VCT','WLF','WSM') and not (access_level=2 and en_sys_str is null)` 输出 `0` | |
| 10536 | `3104` 五条电话区号正确（`CHN`=86、`CTN`=886、`HKG`=852、`MAC`=853、`USA`=001），且 `en_sys_str` 为 `NULL`、`access_level=1`。 | `select sub_key, sub_val from sys.sys_dict_sub where dict_no=3104 order by sub_key` 输出五行；`select count(*) from sys.sys_dict_sub where dict_no=3104 and en_sys_str is null and access_level=1` 输出 `5` | |
| 10537 | `3105` 为 `004` 阿富汗 至 `470` 马耳他，且全部 `en_sys_str='trade'`、`access_level=2`。 | `select count(*) from sys.sys_dict_sub where dict_no=3105 and not (en_sys_str='trade' and access_level=2)` 输出 `0`；`select sub_val from sys.sys_dict_sub where dict_no=3105 and sub_key='470'` 输出 `马耳他` | |
| 10538 | 迁移末尾收敛校验通过并登记 `V100028`。 | `select success from schema_version where version='100028'` 输出 `t` | |

## 四十一、sys_dict_sub 通用标志/分页/数据类型子项（10539–10544）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10539 | 数据以种子段迁移形式提供（`V200000+`）且幂等。 | `test -f Deploy/Migrations/V200029__sys_dict_sub_seed_common.sql`；含 `ON CONFLICT (dict_no, sub_key) DO NOTHING` | |
| 10540 | 六组条数正确：`1001`=2、`1002`=2、`1003`=3、`1004`=2、`1005`=14、`1006`=5（共 28）。 | `select dict_no, count(*) from sys.sys_dict_sub where dict_no between 1001 and 1006 group by dict_no order by dict_no` 输出六行对应条数 | |
| 10541 | `1001` 为 `0`→否 / `1`→是；`1002` 的 `0` 与 `1` 均为「否」（原库如此）。 | `select count(*) from sys.sys_dict_sub where dict_no=1001 and ((sub_key='0' and sub_val='否') or (sub_key='1' and sub_val='是'))` 输出 `2`；`select count(*) from sys.sys_dict_sub where dict_no=1002 and sub_val='否'` 输出 `2` | |
| 10542 | `1003` 为可修改/只读/隐藏；`1004` 两条带 `sub_sort`（1/2）与非空 `description`，且 `en_sys_str` 为空串（非 `NULL`）。 | `select count(*) from sys.sys_dict_sub where dict_no=1003` 输出 `3`；`select count(*) from sys.sys_dict_sub where dict_no=1004 and sub_sort in ('1','2') and coalesce(description,'')<>'' and en_sys_str=''` 输出 `2` | |
| 10543 | `1005` 共 14 条，`sub_val = sub_key`、`sub_sort` 为 1–14（按数值比较）、`en_sys_str='ftc'`。 | `select count(*) from sys.sys_dict_sub where dict_no=1005 and sub_val=sub_key and sub_sort::int between 1 and 14 and en_sys_str='ftc'` 输出 `14` | |
| 10544 | `1006` 共 5 条（字符/整数/浮点数/布尔/字符串），`flag_enable='0'`；迁移登记 `V100029` 成功。 | `select count(*) from sys.sys_dict_sub where dict_no=1006 and flag_enable='0' and en_sys_str='trade,pbs'` 输出 `5`；`select success from schema_version where version='100029'` 输出 `t` | |

## 四十二、sys_dict_sub 国籍（3105）增量补齐（10545–10550）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10545 | 3105 增量以种子段迁移形式提供（`V200000+`）且幂等。 | `test -f Deploy/Migrations/V200030__sys_dict_sub_seed_nationality_increment.sql`；含 `ON CONFLICT (dict_no, sub_key) DO NOTHING` | |
| 10546 | `3105` 国籍已补齐至 235 条，区间为 `004`–`894`。 | `select min(sub_key), max(sub_key), count(*) from sys.sys_dict_sub where dict_no=3105` 输出 `004` / `894` / `235` | |
| 10547 | 本批新增的 102 个 `sub_key`（`474`–`894`）全部存在。 | `select count(*) from sys.sys_dict_sub where dict_no=3105 and sub_key >= '474'` 输出 `102` | |
| 10548 | 新增首尾正确：`474`→马提尼克、`894`→赞比亚。 | `select sub_val from sys.sys_dict_sub where dict_no=3105 and sub_key='474'` 输出 `马提尼克`；`... sub_key='894'` 输出 `赞比亚` | |
| 10549 | `3105` 全量 235 条均为 `flag_enable='0'`、`access_level=2`、`en_sys_str='trade'`。 | `select count(*) from sys.sys_dict_sub where dict_no=3105 and not (flag_enable='0' and access_level=2 and en_sys_str='trade')` 输出 `0` | |
| 10550 | `3103`（249）与 `3104`（5）不受本次增量影响；迁移登记 `V100030` 成功。 | `select dict_no, count(*) from sys.sys_dict_sub where dict_no in (3103,3104) group by dict_no order by dict_no` 输出 `3103\|249`、`3104\|5`；`select success from schema_version where version='100030'` 输出 `t` | |

## 四十三、sys_config 新增运行/收集模式配置（10551–10556）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10551 | 配置项以种子段迁移形式提供（`V200000+`）且幂等。 | `test -f Deploy/Migrations/V200031__sys_config_seed_quote_mode.sql`；含 `ON CONFLICT (config_no) DO NOTHING` | |
| 10552 | 存在 `config_no=20000`，`config_name='FinvQuote服务运行模式'`、`data_type='2'`、`char_value='0'`。 | `select config_name, data_type, char_value from sys.sys_config where config_no=20000` 输出对应三值 | |
| 10553 | `20000` 的 `remark` 为 `0-云端模式（默认） 1-本地模式`。 | `select remark from sys.sys_config where config_no=20000` 输出与预期一致 | |
| 10554 | 存在 `config_no=20001`，`config_name='历史行情收集模式'`、`data_type='2'`、`char_value='0'`。 | `select config_name, data_type, char_value from sys.sys_config where config_no=20001` 输出对应三值 | |
| 10555 | `20001` 的 `remark` 说明被动/主动两种收集模式。 | `select remark from sys.sys_config where config_no=20001` 包含 `被动收集` 与 `主动采集模式` | |
| 10556 | 两条配置时间戳非空；迁移登记 `V100031` 成功。 | `select success from schema_version where version='100031'` 输出 `t`；`select count(*) from sys.sys_config where config_no in (20000,20001) and gmt_create is not null and gmt_modify is not null` 输出 `2` | |

## 四十四、sys_config 创建人修正为 fqe（10557–10560）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10557 | 修正以独立种子段迁移形式提供（`V200000+`）且幂等（不回改已应用的 `V100031`）。 | `test -f Deploy/Migrations/V200032__sys_config_set_create_by_fqe.sql`；含 `SET create_by = 'fqe'` 与 `AND (create_by IS NULL OR create_by <> 'fqe')` | |
| 10558 | `20000` 与 `20001` 的 `create_by` 均为 `fqe`。 | `select count(*) from sys.sys_config where config_no in (20000,20001) and create_by='fqe'` 输出 `2` | |
| 10559 | 修正不改写 `gmt_create`（仍为插入时间），`gmt_modify` 由触发器刷新。 | `select config_no, gmt_create, gmt_modify from sys.sys_config where config_no in (20000,20001)` 中 `gmt_modify >= gmt_create` 且 `gmt_create` 仍为初次插入时间 | |
| 10560 | 其余配置的 `create_by` 不受影响；迁移登记 `V100032` 成功。 | `select count(*) from sys.sys_config where config_no in (1010,1020,1031,5001,5002) and create_by is null` 输出 `5`；`select success from schema_version where version='100032'` 输出 `t` | |

## 四十五、sys_config 20001 备注文案修订（10561–10564）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10561 | 文案修订以独立种子段迁移形式提供（`V200000+`）且幂等（`IS DISTINCT FROM`）。 | `test -f Deploy/Migrations/V200033__sys_config_update_20001_remark.sql`；含 `remark IS DISTINCT FROM` | |
| 10562 | `20001` 的 `remark` 为 `0-被动收集（默认），外部程序通过暴露的API接口添加；1-主动采集模式，服务主动从外部源采集入库保存`。 | `select remark from sys.sys_config where config_no=20001` 输出与预期一致 | |
| 10563 | `20001` 的 `config_name` / `data_type` / `char_value` / `create_by` 未被改动，`gmt_create` 不变。 | `select config_name, data_type, char_value, create_by from sys.sys_config where config_no=20001` 输出 `历史行情收集模式` / `2` / `0` / `fqe` | |
| 10564 | `20000` 的备注不受影响；迁移登记 `V100033` 成功。 | `select remark from sys.sys_config where config_no=20000` 输出 `0-云端模式（默认） 1-本地模式`；`select success from schema_version where version='100033'` 输出 `t` | |

## 四十六、sys_config 归档自动上传 Git 开关与 PAT（10565–10570）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10565 | 两项配置以种子段迁移形式提供（`V200000+`），仅 `ON CONFLICT DO NOTHING`，无针对 20002/20003 的 `UPDATE` 语句。 | `test -f Deploy/Migrations/V200034__sys_config_seed_archive_upload.sql`；`grep -cE '^[[:space:]]*UPDATE[[:space:]]+sys\.sys_config' 文件` 输出 `0`；`grep -cE '^ON CONFLICT \(config_no\) DO NOTHING;' 文件` 输出 `1`（排除注释中的提及） | |
| 10566 | `20002`「启用月归档数据自动上传Git仓库标志」存在，`data_type='2'`、`char_value='0'`、`remark='0-关闭(默认) 1-启用'`。 | `select config_name, data_type, char_value, remark from sys.sys_config where config_no=20002` 输出对应四值 | |
| 10567 | `20003`「归档数据上传GitHub仓库的PAT值」存在，`data_type='3'`、`char_value=''`、**`str_value IS NULL`（无默认值）**。 | `select config_name, data_type, char_value, str_value from sys.sys_config where config_no=20003` 输出 `... / 3 / (空) / (NULL)` | |
| 10568 | PAT 存储容量足够（`str_value` 为 `varchar(6000)`），未误用 `char(1)` 的 `char_value`。 | `select character_maximum_length from information_schema.columns where table_schema='sys' and table_name='sys_config' and column_name='str_value'` 输出 `6000` | |
| 10569 | 交付的初始种子**不覆盖**用户后配值：已存在的行在重跑时值保持不变。 | 实测：先用 `update ... set char_value='1' where config_no=20002` 与模拟 PAT 写入 `20003.str_value`，重跑 `V100034` 输出 `INSERT 0 0`，两值仍为 `1` / 模拟 PAT（验证后已回置为 `0` / `NULL`） | |
| 10570 | 两条配置 `create_by='finv'`；迁移登记 `V100034` 成功。 | `select count(*) from sys.sys_config where config_no in (20002,20003) and create_by='finv'` 输出 `2`；`select success from schema_version where version='100034'` 输出 `t` | |

## 四十七、sys_config 文件上传仓库配置 20004（10571–10576）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10571 | 配置以种子段迁移形式提供（`V200000+`），仅 `ON CONFLICT DO NOTHING`，无针对 20004 的 `UPDATE` 语句。 | `test -f Deploy/Migrations/V200035__sys_config_seed_archive_repo.sql`；`grep -cE '^[[:space:]]*UPDATE[[:space:]]+sys\.sys_config' 文件` 输出 `0` | |
| 10572 | `20004`「文件上传仓库配置」存在，`data_type='3'`、`char_value=''`、`create_by='finv'`。 | `select config_name, data_type, char_value, create_by from sys.sys_config where config_no=20004` 输出对应四值 | |
| 10573 | **不发放默认值**：`str_value IS NULL`（待用户自行配置）。 | `select str_value is null from sys.sys_config where config_no=20004` 输出 `t` | |
| 10574 | `remark` 明确给出 `{owner}:{repo}:{branch}:{pre_path_key}` 格式及各段含义。 | `select remark from sys.sys_config where config_no=20004` 包含 `{owner}:{repo}:{branch}:{pre_path_key}` 与 `pre_path_key=文件保存的目录前缀` | |
| 10575 | 交付的初始种子**不覆盖**用户已配置值。 | 实测：写入 `ACANX:FinvQuant-Archive:main:Archive/FinvQuant/Monthly` 后重跑 `V100035` 输出 `INSERT 0 0`，值原样保留（验证后已回置为 `NULL`） | |
| 10576 | 用户配置格式不合约定时只告警、不阻断迁移；迁移登记 `V100035` 成功。 | 实测：置为 3 段非法值 `ACANX:x:main` 后重跑，输出 `WARNING ... 当前 3 段` 且 `COMMIT` 成功；`select success from schema_version where version='100035'` 输出 `t` | |

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
| 10610 | 校验：无 `USA` 残留、`market` 与 `market_symbol` 对齐、`region/market` 均非空；迁移 `V100036` 登记成功。 | 迁移内置 `DO` 块四条断言均通过；`select success from schema_version where version='100036'` 输出 `t` | |

## 五十三、归档导出状态重置（10611–10615）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10611 | 清除旧口径的文件登记与批次（否则 `RegisterFile` 会因已 `UPLOADED` 而不重新上传）。 | `select count(*) from dedup.finv_dedup_export_file` 输出 `0`；`select count(*) from dedup.finv_dedup_export_batch` 输出 `0`（迁移日志：文件 226、批次 202 已清） | |
| 10612 | 101 个导出任务回退为 `FROZEN` 并清空游标/版本/补丁计数/锁/错误。 | `select status, count(*) from dedup.finv_dedup_export_job group by status` 仅输出 `FROZEN \| 101` | |
| 10613 | 当日预算清零消耗并抬高上限（一轮覆盖全量重导）。 | `select budget_rows, hard_cap_rows, consumed_rows from dedup.finv_dedup_daily_budget where biz_date=current_date` 输出 `5000000 / 6000000 / 0` | |
| 10614 | 重跑保护：已重置后再次执行迁移不会二次清空（避免导出进行中被破坏）。 | 重跑输出 `无 EXPORTED 任务（已重置过或尚未导出），跳过` 且 `COMMIT` | |
| 10615 | 前置输入保留：成熟度 `FROZEN`（2491）、扫描水位、`usc` 主数据、CDEU 历史与上传审计日志均不改；迁移 `V100037` 登记成功。 | `select count(*) from dedup.finv_dedup_quote_maturity_mon where maturity_status='FROZEN'` 输出 `2491`；`select count(*) from dedup.finv_dedup_usc` 输出 `39`；`select success from schema_version where version='100037'` 输出 `t` | |

## 五十四、证券主数据 currency 刷新修正（10616–10620）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10616 | `RefreshSecurityMetadata` 对 `currency` 改为「以主数据为准覆盖」（新值为 NULL 时保留旧值），其余列仍只补缺。 | `grep -n 'COALESCE(EXCLUDED.currency' internal/meta/finv/quote/dedup/store.go` 有输出；`go test ./...` 通过 | |
| 10617 | 存量数据已重刷：`81810`（小米集团-WR 人民币柜台）`HKD → CNY`。 | `select currency from dedup.finv_dedup_usc where usc='81810'` 输出 `CNY`；迁移首次执行 `UPDATE 1` | |
| 10618 | 全库 `currency` 与 `finv_security.currency_type` 零偏差且无空值。 | `select count(*) from dedup.finv_dedup_usc u join public.finv_security s on s.usc=u.usc where nullif(s.currency_type,'') is not null and coalesce(u.currency,'')<>s.currency_type` 输出 `0`；空值数 `0` | |
| 10619 | 幂等：重跑迁移影响 0 行。 | 重跑输出 `UPDATE 0` 且 `COMMIT` | |
| 10620 | 迁移 `V100038` 登记成功，服务重启后数据完好（经历一次 PG 崩溃恢复）。 | `select success from schema_version where version='100038'` 输出 `t`；`select currency from dedup.finv_dedup_usc where usc='81810'` 仍为 `CNY` | |

## 五十五、清理 GCMain 误插入月K线（10621–10625）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10621 | 判定口径可复现：`time=0` + `date%100=1` + `volume>=100000` + `high-low>=20` 三条同时成立。 | `select count(*) from dedup.finv_quote_secu_kline_min where usc='GCMain' and time=0 and date%100=1 and volume>=100000 and (high-low)>=20` 执行前输出 `73`、执行后 `0` | |
| 10622 | 删除前整行备份到 `dedup.finv_quote_secu_kline_min_kline_purge`（含 `reason` / `backup_at`，主键 `(usc,ts)`），可回滚。 | `select count(*) from dedup.finv_quote_secu_kline_min_kline_purge` 输出 `73`；`select distinct reason from ...` 含 `MONTH_KLINE` | |
| 10623 | 仅删误插行，同位置正常分钟数据不得误伤。 | `select count(*) from dedup.finv_quote_secu_kline_min where usc='GCMain' and time=0 and date%100=1` 输出 `4`（`20260401/20260501/20260601/20260701`，volume 14~45） | |
| 10624 | GCMain 行数按预期减少 73。 | 执行前 `2729329` → 执行后 `2729256` | |
| 10625 | 幂等：重跑影响 0 行；迁移 `V100039` 登记成功。 | 重跑输出 `INSERT 0 0` / `DELETE 0`；`select success from schema_version where version='100039'` 输出 `t` | |

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
| 10647 | 迁移 `V100040` 幂等且**不覆盖用户已配值**（`ON CONFLICT DO NOTHING` + 快照硬校验）；重跑影响 0 行。 | 重跑后 `schema_version` 仍 `success=t`；已有值被改写时抛 `RAISE EXCEPTION` | |
| 10648 | `Manual`（默认）时进程内调度器**不自动触发任何 CDEU Job**；手动 `POST .../Job/Run` 不受限。 | 部署后日志含「CDEU 运行模式 = Manual（默认）：调度器不自动触发任何 Job」；间隔类 Job 到期后 6 分钟零新增 `finv_dedup_job_run_log_export_file` 记录 | |
| 10649 | `Auto` 时按定时任务时间自动跑（行为同改造前）。 | 将 20005 改为 `Auto` 并重启后，日志含「CDEU 运行模式 = Auto」，间隔类 Job 到期后出现新运行日志 | |
| 10650 | 取值大小写不敏感；空值/未知值/未配置一律按 `Manual` 兜底（安全侧不自动跑）。 | `go test ./internal/meta/finv/sys -run 'TestNormalizeCDEUMode\|TestValuesCDEUScheduleEnabled'` 通过 | |
| 10651 | `GET .../DeDup/Status` 的 `options` 暴露 `cdeu_mode` / `cdeu_schedule_enabled` / `cdeu_mode_error`。 | 调用接口返回 `"cdeu_mode":"Manual"`、`"cdeu_schedule_enabled":false` | |
| 10652 | 文档同步：`DbSchemaSpec` 种子清单、`CHANGELOG`、`Docs/API/QuoteDeDup/Status.md` 均描述 20005。 | 三处 `grep -c '20005'` 均 ≥ 1；`Status.md` 含 `cdeu_mode` 字段说明 | |

## 六十、导出低峰窗口改为系统配置 20006（10653–10660）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10653 | **【删除表授权记录】** 用户于 2026-10-27 明确授权删除 `dedup.finv_dedup_runtime_setting`（原话：“表删除，留着过年？”）。删除前硬校验表内仅含 `EXPORT_WINDOW_START`/`EXPORT_WINDOW_END` 两个键，发现其他键即中止。 | `select count(*) from pg_tables where schemaname='dedup' and tablename='finv_dedup_runtime_setting'` 输出 `0`；迁移 `V100041` 日志含“已删除” | |
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
| 10671 | `sys.sys_config` 新增 `20007`，`data_type='2'`，`char_value` 默认 `1`；迁移 `V100042` 幂等且不覆盖用户已配值。 | `select data_type, char_value from sys.sys_config where config_no=20007` 输出 `2 \| 1`；重跑影响 0 行 | |
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
| 10694 | `sys.sys_config` 新增 `20008`「历史运行动态JOB保留天数」，`data_type='1'`，`int_value` 默认 `30`；迁移 `V100046` 幂等且不覆盖用户值。 | `select data_type, int_value from sys.sys_config where config_no=20008` 输出 `1 \| 30`；重跑影响 0 行 | |
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

> 本节为 10716 的详细设计，**已随 `V200048__cdeu_review_phase_job_reorg.sql` 全部落地**（2026-10-27）。
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
| 10749 | 补齐 `sys.sys_dict` 中 `dict_no=3120` 的主表记录。`V100027` 早已写入 3120 的 14 条子项（`0`–`9`、`A`–`D` → CNY/USD/HKD/EUR/GBP/AUD/CAD/SGD/JPY/MOP/DKK/GBX/MYR/TWD），但 `V100026` 的 19 条主表记录中没有 3120，形成「有子项、无主表」的孤儿字典。本迁移只补定义行、不动子项，并新增**全库孤儿字典为 0** 的收敛校验（后续再出现同类问题迁移会直接失败）。 | `select count(*) from sys.sys_dict where dict_no=3120` = 1；`select count(*) from sys.sys_dict_sub where dict_no=3120` = 14（不变）；`select count(distinct s.dict_no) from sys.sys_dict_sub s where not exists (select 1 from sys.sys_dict d where d.dict_no=s.dict_no)` = 0；`select version, success from public.schema_version where version='100053'` 为 `t` | √ |
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
| 10751 | 把 `Deploy/Migrations/` 下全部 131 个迁移脚本的 `V` 后编号统一加 100000（如 `V100029__sys_dict_sub_seed_common.sql` → `V200029__sys_dict_sub_seed_common.sql`），并保证**执行顺序与修改前完全一致**。编号分段随之调整为：结构/变更迁移 `V100001`～`V199999`（原 `V1`～`V76`，共 76 个），种子/数据迁移 `V200000+`（原 `V100000`～`V100054`，共 55 个）。同时同步 `public.schema_version` 已应用记录、代码注释与文档中全部迁移编号引用。 | `ls Deploy/Migrations/ \| grep -cE '^V[0-9]+__'` = 131；编号区间 `100001`～`200054` 严格递增且无重号；`select count(*), min(version::int), max(version::int), count(*) filter (where version::int < 100001) from public.schema_version` 返回 `131 / 100001 / 200054 / 0`；容器重启后启动日志**无任何迁移执行记录**（框架正确识别全部已应用） | √ |

### 10751 执行步骤与风险控制

| 步骤 | 操作 | 说明 |
|---|---|---|
| 1 | 停 `finvquant` 容器 | **必须**：旧二进制内嵌旧文件名，若先改名再重启，框架会把 131 个迁移全判为「未执行」并整体重跑 |
| 2 | 建立旧号→新号映射并校验单调性 | 断言 `新编号 - 100000 == 原始编号序列`，确保顺序不变 |
| 3 | 两阶段重命名（旧名 → `TMP__` 前缀 → 最终名） | 避开 `V42→V100042` 与尚未改名的 `V100042` 撞名 |
| 4 | 重建 `schema_version` 版本号 | 单条 `UPDATE +100000` 会撞主键（`1→100001` 与 `100001→200001` 冲突），改用「临时表备份 → `DELETE` → `INSERT (old+100000)`」，全程单事务 |
| 5 | 更新引用 | 文件名引用 `V<n>__` 250 处、裸版本引用 57 处、编号分段约定 6 处 |
| 6 | 构建部署并验证 | 启动日志无迁移执行；`schema_version` 131 条无遗留 |

### 10751 编号映射

| 段位 | 原编号 | 新编号 | 数量 |
|---|---|---|---|
| 结构/变更 | `V1` – `V76` | `V100001` – `V100076` | 76 |
| 种子/数据 | `V100000` – `V100054` | `V200000` – `V200054` | 55 |

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

约束已写入三处落地位置：`V100077` 迁移头注释、`COMMENT ON SCHEMA cdeu`、`Docs/DevSpec/DbSchemaSpec.md §1.1`、`schema_qualify_test.go` 的 `cdeuTables` 注释。

### 10753 实施记录

| 项 | 内容 |
|---|---|
| 迁移 | `V100077__create_cdeu_schema.sql`：幂等建 schema（属主 `finv`）→ 数据库级 `search_path` 纳入 `cdeu` → 三道收敛校验（属主、USAGE/CREATE、`search_path` 含 `cdeu`） |
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
| `idx_dedup*` | `idx_quotecdeu*` | 按 `V100041` 的 `idx_<表名去finv_与下划线>_<用途>` 规范 |
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
| 迁移 | `V100079__rename_cdeu_tables_with_prefix.sql`：5 个 DO 块按系统表动态生成 RENAME（表 → 序列 → 约束 → 索引），逐类计数并输出 NOTICE |
| 为何不 DROP 重建 | 遵循「不修改已应用迁移」规范：`V100078` 已 `success=t`，保留其历史（记录了初版命名），由 `V100079` 矫正；且 RENAME 不动结构与数据，比 DROP+CREATE 更安全 |
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
| 迁移 | `V100080__copy_dedup_data_to_cdeu.sql`：前置校验（源 26 / 目标 26）→ 逐表 `INSERT ... ON CONFLICT DO NOTHING` → 17 个序列 `setval` → 逐表行数收敛校验 |
| 列映射 | 列清单从 `information_schema.columns` 按 `ordinal_position` 动态获取，不依赖 `SELECT *` 的列顺序假设 |
| 幂等 | 全部 `ON CONFLICT DO NOTHING`，重复执行不产生重复行 |
| 序列衔接 | 按 `pg_depend` 反查「序列 → 表.列」的真实依赖关系后 `setval`；空表回初始态（`is_called=false`） |
| 源表保留 | `dedup` 31 张表（含 5 张 `prep_bak_*`）数据一行未删 |

### 10756 已知遗留（继承自 dedup，非本次引入）

三个序列名与其实际服务的列错位，是 `V100050` 重构 CDEU 模型时的遗留，本次按实际依赖 `setval` 故值正确：

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
| 数据一致性 | 切换时点两边一致（`V100080` 迁入 133,260 行）；之后 `cdeu` 增长、`dedup` 停滞 |
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
| `V100081__sys_user_uid_unique.sql` | 新增 `uk_sys_user_uid` 唯一索引（V74 建表时 uid 未加唯一约束，登录以 uid 为标识必须唯一） |
| `V200055__sys_user_seed_finv.sql` | 插入 `uid='finv'`，含盐与 PHC 哈希；`WHERE NOT EXISTS` 保证幂等且不覆盖已改口令 |
| `V200056__sys_config_seed_auth_session_ttl.sql` | 新增配置 20012（会话有效期，默认 480 分钟，合法区间 5~43200） |

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
| 10771 | 让全新数据库在 `public` 已存在**其它服务**同名表（如 `public.finv_quote_secu_kline_min`）时也能完成首次部署：① 历史迁移中本程序自有表的建表/ALTER/DML 全部显式限定 `quote.`；② `V100032` 不得从 `public` 搬移非本程序创建的表（目标 schema 已有则跳过）；③ 修复全新库下 `V100080`（表数/列漂移）、`V200039`（空库断言）、`V200048`/`V200050`（硬编码 phase_id）导致的迁移中断；④ 新增 `V200057` 将 dedup 侧 CDEU 配置同步到 cdeu。 | 在临时 PG18 上先建 `public.finv_quote_secu_kline_min`（含注释、索引与 3 行数据），再执行全部迁移：`grep -cE 'ERROR\|FATAL'` = 0；该校验前后 `select md5(string_agg(t::text,'\|' order by id)) from public.finv_quote_secu_kline_min t` 结果一致且 `pg_dump --schema-only -t public.finv_quote_secu_kline_min` 除 `\restrict` 随机 token 外无差异；`select count(*) from pg_tables where schemaname='quote' and tablename in ('finv_quote_secu_kline_min','finv_quote_ingest_batches','finv_quote_revision_log')` = 3；`select count(*) from cdeu.finv_quote_cdeu_process` 与 `select count(*) from dedup.cdeu_process` 相等；`grep -c 'AND NOT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = '\''quote'\''' Deploy/Migrations/V100032__*.sql` ≥ 1；`grep -c 'phase_id = 12' Deploy/Migrations/V200048__*.sql Deploy/Migrations/V200050__*.sql` = 0 | |

## 一百零七、历史迁移不得删除 public 中非本程序表（10772）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10772 | 历史迁移中不得存在针对 `public` 中非本程序表的 `DROP TABLE` / `TRUNCATE` / `ALTER TABLE`：涉及 `finv_quote_secu_kline_min` 的删除操作一律带 `export.`/`dedup.` 前缀；`V100015` 重建 `finv_futu_mapping_market_code` 的 `CREATE/INSERT/DROP/ALTER/INDEX/TRIGGER` 全部显式加 `public.`；运行期 `TRUNCATE` 使用带 `export.` 前缀的常量。 | `grep -rnE 'DROP TABLE\|TRUNCATE' Deploy/Migrations/*.sql \| grep -i kline_min` 命中的语句均带 `export.` 或 `dedup.` 前缀；`grep -cE 'DROP TABLE( IF EXISTS)? +[a-z_]+ *;' Deploy/Migrations/*.sql` = 0；`grep -c 'public.finv_futu_mapping_market_code' Deploy/Migrations/V100015__*.sql` ≥ 6；`grep -n 'tableStaging' internal/meta/finv/quote/dedup/store.go` 显示值为 `export.finv_quote_secu_kline_min`；在 `public` 预置同名表后执行全部 139 个迁移，`public` 表数据与结构前后一致 | |

## 一百零八、Native 暴露与历史数据迁移（10773–10775）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10773 | 提供**一次性**数据迁移脚本 `Deploy/Script/MigratePublicKlineToCdeu.sql`，把目标库 `public.finv_quote_secu_kline_min` 的历史数据搬入 `cdeu.finv_quote_secu_kline_min`：源表只读；列名映射 `secu_code→usc`、`prev_close→last_close`，`market_code` 丢弃；同 `(ts,usc)` 多 `market_code` 去重保留最新；`ON CONFLICT (ts,usc) DO NOTHING` 幂等；单事务 + 计数校验，异常整体回滚；不进入迁移链。 | 在含旧结构 `public` 表（5 行，含 1 组重复 `ts`+`secu_code`）的库上执行全部迁移后再跑脚本：源表行数保持 5、目标表 4 行、重复组保留 `gmt_update` 最新值、`last_close` 等于源 `prev_close`；重复执行输出「新增 0 行」且目标仍为 4 行；`grep -c 'ON CONFLICT (ts, usc) DO NOTHING' Deploy/Script/MigratePublicKlineToCdeu.sql` ≥ 1 | |
| 10774 | `Nginx.conf.example` 改为**分阶段**模板：阶段一默认 HTTP/80 + `server_name _;` + `default_server`，无需域名/证书即可 `http://<IP>` 访问；阶段二（443 + `http2 on;` + 证书路径 + 80→443 跳转）以整段注释保留，取消注释即可启用；注释内含切换步骤与 nginx < 1.25.1 写法说明；文档说明需移除 Ubuntu 自带 `sites-enabled/default`。 | `grep -vE '^\s*#' Deploy/Native/Nginx.conf.example \| grep -cE 'listen\s+443\|ssl_certificate\|return 301 https'` = 0；`grep -c 'listen 80 default_server;'` ≥ 1；`grep -c 'server_name _;'` ≥ 1；`grep -cE '^#\s+listen 443 ssl;'` ≥ 1 且 `grep -cE '^#\s+ssl_certificate\s'` ≥ 1；删除自带 default 站点后 `nginx -t` 通过（阶段一）；取消注释块并按 1.24 写法替换后 `nginx -t` 通过（阶段二） | |
| 10775 | 在 `Docs/FinvQuant/` 提供两份文档：`Nginx-Deploy.md`（阶段一部署/验证/常见问题、阶段二切换/回滚、与 systemd 的关系）与 `Nginx.conf.example-ChangeRequest.md`（可直接作为上游 Issue 正文：问题现象、期望行为、逐项改动、验收标准、兼容性）。 | `ls Docs/FinvQuant/Nginx-Deploy.md Docs/FinvQuant/Nginx.conf.example-ChangeRequest.md` 均存在；前者含「阶段一」「阶段二」「常见问题」「回滚」章节；后者含「建议 Issue 标题」「验收标准」「兼容性」章节；`Deploy/Native/Install.md` §6 与 `Deploy/Native/README.md` 文件表描述已同步为分阶段 | |

## 一百零九、部署包包含一次性运维 SQL（10776）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10776 | 部署包（快照包 / 构建包 / 正式包）必须包含 `Deploy/Script/*.sql` 一次性运维脚本及其 `README.md`；`Install.sh` 安装后需留存到 `<PREFIX>/Deploy/Script/`（权限 0644），且该目录**不进入** `Deploy/Migrations/` 迁移链。 | 执行 `bash Deploy/Script/PackSnapshot.sh --target linux/arm64` 后：`tar -tzf Dist/*.tar.gz \| grep 'Deploy/Script/MigratePublicKlineToCdeu.sql'` 命中；Windows zip 用 `unzip -l` 同样命中；包内该文件权限为 `0644`。安装后 `ls -l /data/App/FinvQuant/Deploy/Script/` 存在 SQL 与 README。源码目录与安装目录同布局时重复执行 `Install.sh` 不报错。 | |

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
| 10791 | 将 `cdeu.finv_quote_secu_kline_min.volume` 由 `bigint` 改为浮点 `numeric(20,6)`，保留 `volume >= 0` 约束；幂等迁移，仅改 `cdeu` 表。 | `grep -c 'numeric(20,6)' Deploy/Migrations/V200058__cdeu_kline_min_volume_numeric.sql` ≥ 1；临时 PG 执行全部迁移后 `select data_type from information_schema.columns where table_schema='cdeu' and table_name='finv_quote_secu_kline_min' and column_name='volume'` = `numeric`；插入 `12.345678` 后读回一致；重复执行 V200058 输出「已是 numeric，跳过」；`go build ./...`、`go test ./internal/meta/finv/quote/cdeu/kline` 通过；`MVsvFileFormat.md` 中 `v` 类型为 `Decimal(20,6)` | |

## 一百二十四、volume 全链路浮点化（10792）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10792 | 把 `quote` / `export` / `dedup` 三张同名表的 `volume` 与 `cdeu` 统一为 `numeric(20,6)`；`internal/meta/finv/quote/mvsv` 解析保留小数文本；`internal/meta/finv/quote` 查询输出支持小数；保证导入/导出/补丁/回测链路不截断。 | 临时 PG 执行全部迁移后 `select table_schema, data_type from information_schema.columns where column_name='volume' and table_name='finv_quote_secu_kline_min' and table_schema in ('quote','dedup','export','cdeu')` 四行均为 `numeric`；`12.345678` 在四表写入/读回一致；重复执行 V200059 输出「已是 numeric，跳过」；`go test ./internal/meta/finv/quote/mvsv` 含 `TestParseNumericVolume` 通过；`go build ./...`、`go test ./internal/meta/finv/quote ./internal/meta/finv/quote/cdeu/kline` 通过；`HistoryQuote.md` volume 标注支持小数 | |

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
| 10800 | 新增 `brs` schema 与 `brs.brs_bak_rdb_table` 备份登记表（`priority` 唯一）；`sys.sys_config 39001` 配置 6 段 cron；服务内每日按 `priority` 倒序对启用表执行 `pg_dump -Fc`（按 `compression` 压缩），备份到 `<PREFIX>/Backup/BRS/<yyyyMMdd>/`，只写日志；预置系统表与 CDEU 配置表种子（不含 `cdeu.finv_quote_secu_kline_min`）；对象存储通过 `internal/meta/base/oss` 抽象，首期不启用上传。 | `test -f Deploy/Migrations/V200060__brs_backup_table.sql`；临时 PG 执行全部 142 个迁移 0 错误，`select count(*) from brs.brs_bak_rdb_table` = 12，`select count(distinct priority)` = 12，`select str_value from sys.sys_config where config_no=39001` = `0 0 3 * * *`；`go test ./internal/meta/finv/brs ./internal/meta/base/cron ./internal/meta/base/oss/...` 通过；`go build ./...` 通过；`grep -c 'FINV_BRS_BACKUP_DIR' internal/config/config.go Deploy/Native/FinvQuant.env.example` 各 ≥ 1；`grep -rn 'minio-go' internal/` 无输出 | |

## 一百三十三、BRS 对象存储上传（10801）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10801 | BRS 二期：备份产物上传 S3 兼容对象存储（R2 等）。`brs_bak_rdb_table` 增加 `flag_upload`；`sys.sys_config 39002` 保存对象存储 JSON 配置（默认 `enabled=false`）；通过 `internal/meta/base/oss` 抽象 + `oss/s3` 适配器（`aws-sdk-go-v2`，**不使用 minio-go**），上传失败保留本地。 | `grep -c 'flag_upload' Deploy/Migrations/V200061__brs_upload.sql` ≥ 1；临时 PG 执行全部 143 个迁移 0 错误；`select json_value->>'enabled' from sys.sys_config where config_no=39002` = `false`；`go test ./internal/meta/base/oss/... ./internal/meta/finv/brs` 通过（含 s3 适配器 httptest 用例）；`grep -rn 'minio' internal/meta/base/oss internal/meta/finv/brs` 无官方库引用；`go build ./...` 通过 | |

## 一百三十四、BRS 对象存储配置说明（10802）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10802 | `sys.sys_config 39002` 的 `remark` 必须包含完整配置说明：字段含义、R2/AWS/MinIO 的 endpoint/region 差异、R2 API Token 获取步骤、示例 JSON、注意事项；不能只写一句话。`Brs.md` 同步配置步骤。 | `grep -c 'R2 示例' Deploy/Migrations/V200061__brs_upload.sql` ≥ 1；临时库执行后 `select length(remark) from sys.sys_config where config_no=39002` ≥ 500；`select remark from ...` 含 `enabled`、`endpoint`、`R2`、`flag_upload`；`go test ./internal/docs` 通过 | |

## 一百三十五、Web 左侧菜单四级化（10803–10807）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10803 | 左侧菜单改为四级树：一级为系统，其下为二/三/四级菜单；既有 29 个业务菜单按「旧层级 +1」映射到新层级（不新增业务菜单、不改路由）。 | `grep -c "key: '" Web/src/config/menuConfig.ts` ≥ 37；人工核对「数据采集 → 行情数据归档 → 归档统计与预算」上下共 4 级；`grep -c 'v-list-group' Web/src/App.vue` 为 0 | |
| 10804 | 顶栏提供一级系统切换器（金融投资 / 内容管理，内部 key `finvquant` / `cms`），切换后左侧菜单联动并回到该系统上次访问路由；「金融投资」即 CMS 一级菜单中的金融投资分系统。 | `grep -F "key: 'finvquant'" Web/src/config/menuConfig.ts` 命中；`grep -F "key: 'cms'" Web/src/config/menuConfig.ts` 命中；`grep -F "path: '/Meta/Finv', redirect: '/Dashboard'" Web/src/router.ts` 命中；人工切换后左侧菜单随之变化 | |
| 10805 | 内容管理系统本轮仅提供缺省占位：一级入口「内容管理」+ 二级叶子「内容管理控制台」（`/Meta/CMS`），页面为占位页且不实现业务功能。 | `grep -F "path: '/Meta/CMS'" Web/src/router.ts` 命中；`grep -F "'cms-home'" Web/src/views/Common/PlaceholderView.vue` 命中；人工点击后显示「规划中」占位页 | |
| 10806 | 菜单展开状态按系统分桶持久化到 localStorage（键 `finvquant.nav.openKeys.v1`），深链接直开时自动展开祖先链；脏数据回退到激活链且不报错。 | `grep -F 'finvquant.nav.openKeys.v1' Web/src/config/uiConfig.ts` 命中；人工：展开「元数据管理」→切到内容管理→切回，展开状态保留；直接访问 `/Meta/Finv/Quote/DeDup/Export/Stats` 时祖先链自动展开 | |
| 10807 | 导航视觉固定为 WebCMS 风格：当前菜单项左侧 3px 主色激活条 + 祖先分组标题主色；**不提供风格开关，也不使用 `?nav=` 参数**。抽屉顶部不展示系统标题块、内容区不展示面包屑（2026-10-29 已移除）。 | `grep -c 'navVisualStyle\|toggleNavVisualStyle\|WebCMS 视觉风格' Web/src Web/dist/assets/index-*.js` 为 0；`grep -c 'WebCMS 视觉风格' Web/dist/assets/index-*.js` 为 0；人工：用户菜单只有修改密码/退出登录，菜单激活项有 3px 主色条 | |

## 一百三十六、V200057 迁移兼容 dedup 缺失（10808）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10808 | `V200057__sync_cdeu_config_from_dedup.sql` 在 `dedup` schema 不存在时必须幂等跳过（输出 NOTICE），不得因 `dedup.cdeu_process` 不存在而中断迁移链。 | `grep -c "nspname = 'dedup'" Deploy/Migrations/V200057__sync_cdeu_config_from_dedup.sql` ≥ 1；在无 dedup schema 的库上重建容器后：`docker logs finvquant` 含 `dedup schema 不存在，跳过配置同步`，`select max(version) from schema_version` = `200061`，`/API/V1/Health/Ready` 返回 200 | |

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
| 10814 | `public.finv_security` 增加 `type_secu_sub`、`provider`、`remark` 三列，均为 `TEXT NOT NULL DEFAULT ''`，并带列注释与末尾收敛校验（列存在性/类型/可空/默认值）。 | `test -f Deploy/Migrations/V100082__finv_security_add_type_secu_sub_provider_remark.sql`；文件内 `ADD COLUMN IF NOT EXISTS type_secu_sub`、`provider`、`remark` 三处均在；`grep -c 'RAISE EXCEPTION' Deploy/Migrations/V100082__finv_security_add_type_secu_sub_provider_remark.sql` ≥ 4 | |

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
| 10818 | 在 `futu` schema 新建 `finv_futu_quote_collect` 表（`usc` 主键，含 `stock_id`/`futu_symbol`/`market_type`/`market_code`/`instrument_type`/`sub_instrument_type`/`req_section`/`quote_market`/`type_symbol`/`flag_enable`/`remark`/`gmt_create`/`gmt_update`），并支持从上游快照初始化与增量同步（幂等 upsert，可仅同步新增/变更行）。 | `test -f Deploy/Migrations/V100083__create_futu_finv_futu_quote_collect.sql`；`test -f ../FinvQuantDesign/Scripts/SecuSync/BuildFutuQuoteCollect.py`；`select count(*) from public.schema_version where version='100083'` 为 1；`select count(*) from information_schema.columns where table_schema='futu' and table_name='finv_futu_quote_collect'` 为 14；`select count(*) from futu.finv_futu_quote_collect` 为 17138 | |

## 一百四十六、初始数据发放以迁移脚本形式入库（10819）

| 编号 | 需求描述 | 核对方式 | 结果 |
|---|---|---|---|
| 10819 | `finv_security` 与 `futu.finv_futu_quote_collect` 的初始数据以种子/数据迁移形式纳入 `Deploy/Migrations`（`V200062` / `V200063`，均 `ON CONFLICT DO NOTHING`），并将港股人民币柜台币种修正固化为 `V200064`；全新库按版本顺序执行后两表行数分别为 17,199 / 17,138。 | `ls Deploy/Migrations/V20006[234]__*.sql` 命中3 个；`grep -c 'ON CONFLICT DO NOTHING' Deploy/Migrations/V200062__finv_security_seed_supabase.sql` ≥ 35；`grep -c 'ON CONFLICT DO NOTHING' Deploy/Migrations/V200063__finv_futu_quote_collect_seed_supabase.sql` ≥ 35；`select count(*) from public.schema_version where version in ('200062','200063','200064')` 为 3；`select count(*) from futu.finv_futu_quote_collect` 为 17138 | |

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
| V200057 迁移兼容 dedup 缺失 | 10808 | 1 |
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
| **合计** | **10001–10819** | **819** |

> 验收方式：按编号顺序逐条核对，`√` = 满足，`×` = 不满足并记录差异。
> 编号连续、不跳号；新增需求从 `10803` 起往后追加，不插入既有区间。
