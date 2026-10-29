# 变更日志（CHANGELOG）

> 所属：FinvQuant · 存放：`Docs/`
> 文件名**全大写** `CHANGELOG.md`（不写作 `ChangeLog.md`）。
> 版本号唯一来源：仓库根目录 [`VERSION`](../VERSION)；每次发布须与 Git tag 一一对应。

## 记录规则

- **强制**：任何使用者或部署者可感知的修改（功能、修复、接口、数据库、配置、部署、开发规范）必须在同一次提交中更新本文件，详见 [`DevSpec/DocSpec.md`](DevSpec/DocSpec.md) 第 4 节与 [`DevSpec/GitSpec.md`](DevSpec/GitSpec.md) 第 3 节。
- 按 `新增` / `变更` / `修复` / `移除` / `安全` 五类分组记录，无内容的分组不保留。
- 尚未发布的变更记入「未发布」小节；发布时整体改为对应版本号并补上发布日期。
- 只记录**面向使用者与部署者**可见的变化；纯内部重构可合并为一条描述。
- 日期格式 `YYYY-MM-DD`；提交信息规范见 [`DevSpec/GitSpec.md`](DevSpec/GitSpec.md)，文件命名规范见 [`DevSpec/FileNamingSpec.md`](DevSpec/FileNamingSpec.md)。

---

## [未发布]

> 0.1.4 开发中。变更按 `新增` / `变更` / `修复` / `移除` / `安全` 分类追加，发布时整体改为版本号并补日期。

### 变更

- 版本号由 `0.1.3` 切换至 `0.1.4`（`VERSION`），开启下一轮开发；`v0.1.3` 已作为正式版本打 tag。
- **CDEU 前端路由迁移到 `Quote/Cdeu` 段**
  - 页面路由：`/Meta/Finv/Quote/DeDup/CDEU` → **`/Meta/Finv/Quote/Cdeu/CdeuOverview`**；`/Meta/Finv/Quote/DeDup/CDEUWorkflow` → **`/Meta/Finv/Quote/Cdeu/CdeuWorkflow`**（`Web/src/router.ts`）。
  - 菜单项 `dedup-export-cdeu` 的 `path` 与 CDEU 页面内「全流程拓扑」跳转 `href` 同步更新；菜单 `key` 不变，展开状态与上次访问记忆不受影响。
  - 后端 API 路径保持 `/API/V1/Meta/Finv/Quote/DeDup/CDEU/{Overview,Workflow/Run,Workflow/Terminate}` 不变，页面内 `apiGet`/`apiPost` 调用零改动。
  - 文档：`Docs/Menu/Menus.md` 路由列、`Docs/RequirementList.md`（10389、10432 核对命令）同步；历史 CHANGELOG 与 `Docs/Refactor/2610-012` 中的旧路径按原样保留。
  - 注意：旧路径未保留 redirect，浏览器书签/历史链接需手动改为新路径。

- **计算与 MVSV 解析包归入 `meta/finv` 域，`internal/` 顶层收敛**
  - `internal/calc`（`change.go`、`change_test.go`）→ **`internal/meta/finv/quant/calc`**；`internal/mvsv`（10 个文件：MVSV-1/2 解析与写出、流式读、样例验证）→ **`internal/meta/finv/quote/mvsv`**；包名 `calc` / `mvsv` 保持不变。
  - import 同步：`internal/api/handler/import.go`、`internal/meta/finv/quote/{service.go,change_test.go}`、`internal/meta/finv/quote/cdeu/kline/request.go`、`cmd/qmio/{export.go,import.go,verify.go}`。
  - `mvsv/sample_files_verify_test.go` 的示例文件相对路径按新目录深度修正为 `../../../../../Docs/DataFormat/...`，`TestSampleFilesParse` 仍通过（2 个样例各 20 条记录）。
  - 文档同步：`Docs/DataFormat/MvsvFileFormat.md`（2 处）、`Docs/FinvQuant/Qmio.md`（1 处）、`Docs/RequirementList.md`（8 处）；历史 CHANGELOG/Refactor 记录保留。
  - 至此 `internal/` 顶层只剩 `api`、`config`、`database`、`docs`、`logging`、`meta`、`redisclient`、`static`、`webui`（业务域全部收敛到 `internal/meta/{base,finv}`）。
  - 验证：`go build ./...`、`go vet ./...`、`go test ./internal/meta/finv/quant/calc ./internal/meta/finv/quote/mvsv ./internal/meta/finv/quote ./cmd/qmio ./internal/api` 全部通过。
- **行情导入/查询服务迁入 `internal/meta/finv/quote`**
  - `internal/quote`（`service.go`、`service_test.go`、`change_test.go`，包名 `quote`）迁移到 **`internal/meta/finv/quote/`**，与同目录的 `cdeu/`、`dedup/` 子包并列，形成完整的 quote 域（导入/查询 + 去重 + CDEU 加工）。
  - `internal/api/handler/{import.go,query.go}`、`internal/api/router.go` 的 import 路径同步更新；源码内容、导出符号、接口路径与 schema 归属（`quote.`）零改动。
  - 文档引用同步：`Docs/DevSpec/DbSchemaSpec.md`（2 处）、`Docs/RequirementList.md`（9 处核对命令与描述）；`Docs/CHANGELOG.md`、`Docs/Refactor/*` 中的历史记录按原样保留。
  - 验证：`go build ./...`、`go vet ./...`、`go test ./internal/meta/finv/quote ./internal/api` 全部通过。
- **K 线领域服务迁入 `internal/meta/finv/quote/cdeu` 子域**
  - `internal/kline`（13 个 Go 文件：领域服务、请求校验、格式化、SQL、统计、错误目录 + 单测）整体迁移到 **`internal/meta/finv/quote/cdeu/kline`**，包名仍为 `kline`；`internal/meta/finv/quote/cdeu/` 为新建的 CDEU（去重/导出/上传）业务域目录，与同级的 `quote/dedup` 并列。
  - `internal/api/handler/{errresp.go,errresp_test.go,kline.go,kline_ops.go}`、`internal/api/{router.go,router_test.go}` 的 import 路径同步更新；`internal/meta/finv/quote/service.go` 与包内集成测试注释中的路径同步。
  - 源码内容、导出符号、接口路径与数据库 schema（`cdeu.finv_quote_secu_kline_min`）零改动。
  - 文档引用同步：`Docs/API/Quote/Cdeu/Min/{KLinePageUni,KLineUpsert}.md`、`Docs/DevSpec/DbSchemaSpec.md`、`Docs/DevSpec/ErrorCodeSpec.md`、`Docs/RequirementList.md`（25 处核对命令与描述）；`Docs/CHANGELOG.md` 历史条目按原样保留。
  - 验证：`go build ./...`、`go vet ./...`、`go test ./internal/meta/finv/quote/cdeu/kline ./internal/api` 全部通过。
- **回测引擎迁入 `internal/meta/finv/quant` 子域**
  - `internal/backtest`（16 个 Go 文件，引擎/表达式/指标/服务/校验/模型 + 单测）整体迁移到 **`internal/meta/finv/quant/backtest`**，包名仍为 `backtest`，与 `auth/`、`brs/`、`quote/`、`sys/` 的组织方式统一。
  - `internal/api/handler/backtest.go`、`internal/api/router.go` 的 import 路径同步更新；源码内容与导出符号零改动，接口与回测行为不变。
  - 文档引用同步：`Docs/Asset/Backtest/README.md`、`Docs/DevSpec/BacktestStrategySpec.md`、`Docs/DevSpec/DbSchemaSpec.md`（schema 归属表）、`Docs/FinvQuant/V1/Prompt.md`、`Docs/RequirementList.md`（10633 描述与核对命令）；`Docs/CHANGELOG.md` 与 `Docs/Refactor/*` 中的历史记录按原样保留。
  - 验证：`go build ./...`、`go vet ./...`、`go test ./internal/meta/finv/quant/backtest` 全部通过。
- **BRS 包迁入 `internal/meta/finv` 子域**
  - `internal/brs`（`brs.go`、`brs_test.go`）整体迁移到 **`internal/meta/finv/brs`**（包名仍为 `brs`），与 `internal/meta/finv/{auth,quote,sys}` 的组织方式统一；`cmd/server/main.go` 的 import 路径同步更新。
  - 源码内容与导出符号零改动，服务行为不变；`go build ./...`、`go vet ./...`、`go test ./internal/meta/finv/brs` 全部通过。
  - 文档引用同步：`Docs/FinvQuant/Brs.md`（源码位置）、`Docs/Refactor/2610-011-BrsBackup.md`（路径表与验证命令）、`Docs/RequirementList.md`（10800、10801 核对命令）；历史 CHANGELOG 条目中的旧路径按历史记录保留。
- **文档归档与处理器文件重命名**
  - `Prompt.md` 移至 **`Docs/FinvQuant/V1/Prompt.md`**（`git mv` 保留历史），根目录不再保留；`AGENTS.md`、`README.md` 的目录树/索引与 `Docs/RequirementList.md` 核对命令（10073、10099、10131、10237）中的路径同步更新；历史 CHANGELOG 条目与 `Docs/Refactor/*` 过程文档中的旧路径按历史记录保留。
  - `internal/api/handler/exportarchive.go` → **`cdeu.go`**，`internal/api/handler/exportarchive_admin.go` → **`cdeu_admin.go`**：Go 构建不依赖文件名，包名与导出符号不变，接口行为零变化；`Docs/API/QuoteDeDup/Usc.md` 与 `Docs/RequirementList.md`（10340、10350、10379、10470 核对命令）中的引用同步更新。

## [0.1.3] - 2026-10-29

> 本版本内容：四级菜单导航改造、登录与文案修复、BRS 上传与配置、去重/导出链路修复、深色/浅色主题切换等。

### 新增

- **设计文档撰写指南（参考性）**
  - 新增 **`Docs/Guide/DesignDocGuide.md`**：基于 AHA `Docs/Design` 22 篇的体例统计（命名、6 行元信息、4 列变更日志、章节数 8–12、篇幅中位 271 行、表格化表达、ASCII 图、`✅`/`⛔`/`⚠` 状态标记、12 篇「为什么」小节），归纳为撰写参考：文件命名与放置、文档骨架、章节顺序、叙述习惯、表格/代码块/状态标记用法、链接、篇幅拆分、维护更新。
  - 附可复制模板与 13 项写完后的自检建议、常见毛病对照表，以及统计基线附录。
  - **性质说明**：这是写作参考，不进入 `Docs/DevSpec/` 规范体系，也不占用需求编号；与既有规范冲突时以 `Docs/DevSpec/` 为准。

- **CDEU 归档工作流设计文档（需求 10813）**
  - 新增 **`Docs/Design/CdeuWorkflowDesign.md`**，体例对齐 AHA `Docs/Design`：文档元信息 + 变更日志 + 编号章节 + 表格化结论。
  - 内容覆盖：五级层级（Workflow → Phase → Process → JOB → Step）、定义/运行/动态实例分离、9 种运行状态、完成条件、触发与执行模式、六个阶段（收集 / 去重 / 决策 / 导出 / 上传 / 复核）、17 个 Process、22 个 JOB（含 1 个已停用）、15 个静态 Step。
  - 每个阶段给出「目标—输入—输出—流程—关键规则—失败边界」，每个 JOB 给出「输入—处理—输出—解决的问题」，另含数据契约（关键表、文件命名与 16 行文件头、校验规则）、系统配置 20001–20012、阈值清单、运维排障表与两个页面（`CdeuOverview`、`CdeuWorkflow`）的监控映射。

- **深色 / 浅色主题切换（需求 10810）**
  - 右上角用户菜单新增「深色主题」开关，即时切换 Vuetify `light` / `dark` 主题；选择保存在 `localStorage['finvquant.ui.theme']`，**不做服务端持久化**（待办 10811）。
  - 初始主题优先级：本地选择 > 浏览器 `prefers-color-scheme` > 深色默认（`Web/src/main.ts` + `Web/src/config/uiConfig.ts`）。
  - 图表适配：历史行情查询（ECharts 画布/坐标轴/图例/tooltip/布林中轨线）与去重分钟行情统计（坐标轴分割线）改为随主题取值，并在主题切换时重绘；登录页保持独立深色视觉。
  - 文档：`Docs/DevSpec/UiSpec.md` 新增 §3「主题（深色/浅色）切换」。

- **BRS 对象存储配置说明（39002 remark，需求 10802）**
  - `V200061` 中 `sys.sys_config 39002` 的 `remark` 补全为完整配置说明（`enabled/provider/endpoint/bucket/region/access_key/secret_key/prefix` 字段含义、R2/AWS/MinIO 差异、R2 API Token 获取步骤、注意事项、示例 JSON），约 1KB。
  - `Docs/FinvQuant/Brs.md` §6.1 增加 R2 配置步骤与验证命令。

- **BRS 对象存储上传（二期，需求 10801）**
  - `brs.brs_bak_rdb_table` 增加 `flag_upload char(1)`（默认 `'1'`：上传 / `'0'` 只留本地）。
  - `sys.sys_config` 新增 **39002**（JSON）：`enabled/provider/endpoint/bucket/region/access_key/secret_key/prefix`，默认 `enabled=false`。
  - 新增 `internal/meta/base/oss/s3` 适配器（`aws-sdk-go-v2/service/s3`，path-style），**不使用 minio-go 及 MinIO 官方库**；通过 `oss.Store` 抽象接入，切换客户端只换适配器。
  - 备份成功后按 `flag_upload` 上传到 `<prefix>/<yyyyMMdd>/<schema>.<table>_<HHmmss>.dump`；失败保留本地文件并记日志，不影响下一张表。
  - 文档：`Docs/FinvQuant/Brs.md` §6 更新为已实现；`2610-011` 实施记录补充二期。
  - 验证：143 个迁移 0 错误；39002 默认 `enabled=false`、`provider=s3`；`go build ./...`、`go test ./internal/brs ./internal/meta/base/oss/...` 通过。

- **BRS 数据备份服务（首期：PostgreSQL 表级备份，需求 10800）**
  - 新增 `brs` schema 与 `brs.brs_bak_rdb_table`：`idn`、`rdb_type`、`schema_name`、`table_name`、`flag_enable`（char(1)）、`priority`（**必填且唯一**）、`bak_scope`、`bak_method`、`compression`（none/gzip/zstd）、`where_clause`、`remark`、`creator`（uid）、`gmt_create`、`gmt_update`；唯一键 `(rdb_type, schema_name, table_name)`、`UNIQUE (priority)`。
  - `sys.sys_config` 新增 **39001**：6 段 cron（秒 分 时 日 月 周），默认 `0 0 3 * * *`。
  - 新增独立包 `internal/meta/base/cron`（手写 6 段解析/匹配/Next，仅标准库）与 `internal/brs`（登记读取、cron 调度、`pg_dump` 执行、日志）；`cmd/server` 启动调度。
  - 新增 `internal/meta/base/oss` 对象存储抽象（`Store` 接口 + `Config` + 注册式 `New`），首期实现 `oss/local`；`oss/s3` 二期使用 `aws-sdk-go-v2`，**不使用 minio-go 及 MinIO 官方库**，切换客户端只改适配器。
  - 备份文件：`<PREFIX>/Backup/BRS/<yyyyMMdd>/<schema>.<table>_<HHmmss>.dump`，`-Fc` + `--compress=zstd:3`；失败继续下一张表。
  - 预置 12 条登记（sys 系统表 + cdeu 配置表），**不含** `cdeu.finv_quote_secu_kline_min`。
  - 文档：`Docs/FinvQuant/Brs.md`（运维/恢复/二期）、`2610-011` 实施记录、`Install.md` 与 `FINV_BRS_BACKUP_DIR`。

- **qmio 数据搬运工具：MVSV-2 导出/导入（需求 10799）**
  - 新增 `cmd/qmio`（`export` / `import` / `verify`），安装到 **`<PREFIX>/bin/qmio`（与 `finvquant` 同目录）**，软链 `/usr/local/bin/qmio`。
  - `internal/mvsv` 新增 **MVSV-2**：单文件全表、含 `usc` 列、19 列（含 `gmt_*`）或 17 列（`--no-gmt`），导入按列名映射；MVSV-1 解析保持不变。
  - 导出：`REPEATABLE READ READ ONLY` 快照、gzip、临时文件原子改名、行数校验、`.sha256`。
  - 导入：默认 `upsert`（冲突覆盖业务列）；`append` / `replace`（需 `--force`）；批量 1000 行/批；`--skip-verify` 可跳过 sha256。
  - `PackSnapshot.sh` 包根目录同时产出 `finvquant` 与 `qmio`；`Install.sh`/`Upgrade.sh`/`Uninstall.sh` 同步安装/替换/删除。
  - 文档：`Docs/FinvQuant/Qmio.md`、`MvsvFileFormat.md` 增加 MVSV-2 章节、`Install.md`/`README.md` 目录与用法、`2610-010` 实施记录。
  - 验证：临时 PG 导出→清空→导入→重复导入（幂等）通过；`--no-gmt` 17 列通过；`go build ./...`、`go test ./internal/mvsv` 通过。

- **cdeu 分钟行情导出/导入工具方案文档（需求 10798）**
  - 新增 `Docs/Refactor/2610-010-CdeuKlineDumpImport.md`（状态：方案草案，未实施）：交付形态/文件格式/导入模式对比，推荐「psql 脚本 + CSV.gz + append 幂等导入」，含命令设计、一致性快照导出、三种导入模式、sha256/gzip 校验、性能预估、10 项待确认决策点与验收标准。

- **Native 日志落盘支持（`FINV_LOG_DIR`，需求 10796）**
  - 新增 `internal/logging`：标准库日志与 gin 访问日志**同时**写入 `<FINV_LOG_DIR>/<FINV_LOG_FILE>`（默认 `<PREFIX>/Log/FinvQuant.log`），journald 仍保留。
  - `FINV_LOG_DIR` 为空时保持仅 stdout/stderr（容器方式不受影响）；目录/文件创建失败仅告警，不阻断启动。
  - `Install.sh`：创建 `<PREFIX>/Log`（`finvquant:finvquant 0750`）、新装渲染/旧配置补齐 `FINV_LOG_DIR`，摘要输出日志路径。
  - 文档：`Install.md` 新增「日志与归档」、`README.md` 目录树与权限表、`FinvQuant.env.example`、unit 注释、`Uninstall.sh` 保留提示。

- **外部 OpenAPI：PageUni 查询端点（需求 10789）**
  - `/OpenAPI/V1` 新增 `GET /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni`，业务逻辑复用内部 `KLine.PageUni` handler；认证与响应信封同外部 Upsert。
  - 成功响应 `{"code":0,"data":{"cnt","ps","pn","column","row","request_id","ts"},"message":"OK"}`；包装中间件识别内部 `{code,data,message}` 信封并解包内层 `data`，避免信封嵌套。
  - 新增 `Docs/OpenAPI/Quote/Cdeu/Min/PageUni.md` + 同名 `.yml`（OpenAPI 3.1，8 个 query 参数、15 列定义、成功码 0/1/200/202）；`APIs.md`、`README.md` 同步。
  - 单测覆盖：注册、未授权 401、正确密钥进入 handler（`limit=0` → 422 / 211004）、内部信封解包。

- **外部 OpenAPI 响应规范化为 `{code,data,message}`（需求 10788）**
  - `/OpenAPI/V1` 路由组新增响应包装：成功时输出 `{"code":0,"data":<原 handler JSON>,"message":"OK"}`，业务字段统一收敛到 `data` 节点；`204` 与 `401`、业务错误保持原样。
  - 成功码默认 **`0`**；`1` / `200` / `202` 同样视为正常响应（兼容），服务端默认输出 `0`。
  - 新增外部端点文档 `Docs/OpenAPI/Quote/Cdeu/Min/Upsert.md` + 同名 `.yml`（OpenAPI 3.1，请求体 schema 通过相对 `$ref` 复用内部 `KLineUpsert.yml`）；`Docs/API/APIs.md`、`README.md`、方案文档 §5.5 同步。
  - 单测覆盖信封结构、`code=0`、`204` 不包装、错误原样透传。

- **外部 OpenAPI 示例端点（需求 10787）**
  - 新增独立路由组 `/OpenAPI/V1`（不使用登录会话），示例端点 `POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert` 直接复用内部 `KLine.Upsert` handler。
  - 过渡鉴权：配置 `FINV_OPENAPI_KEY` 后启用，同时接受 `Authorization: Bearer <key>` 与 `X-API-Key: <key>`（常量时间比较）；未配置时不注册（404），避免无认证暴露。
  - `FinvQuant.env.example` 增加 `FINV_OPENAPI_KEY`；`router_test` 覆盖「未配置不注册」「错误密钥 401」「两种正确传递方式进入 handler」。
  - 后续由 `sys.sys_open_api_key` API Key 体系替换静态密钥（见 `2610-009-ExternalHookApiKey.md`）。

- **外部 HOOK 接入方案文档（API Key，需求 10786）**
  - 新增 `Docs/Refactor/2610-009-ExternalHookApiKey.md`（状态：**方案已确认，未实施**）：现状认证分析、4 种方案对比、推荐「API Key + 独立 `/API/V1/Hook` 路由组」、`sys.sys_open_api_key` DDL 草案（默认 180 天、key 级限流、多 scope）、`RequireOpenAPIKey` 中间件流程、端点开放清单（待接入方确定）、管理接口、限流/审计/配置、13 项已确认决策与 4 项后续待决策、验收标准与分阶段计划。
  - 仅文档，不改动现有认证与路由。

- **一次性数据迁移脚本：`public.finv_quote_secu_kline_min` → `cdeu.finv_quote_secu_kline_min`（需求 10773）**
  - 新增 `Deploy/Script/MigratePublicKlineToCdeu.sql`：早期版本把分钟 K 线建在 `public`，新版本只读写 `cdeu`，脚本把历史数据一次性搬入。
  - 特性：对源表**只读**（不改名/删除/TRUNCATE）；幂等（`ON CONFLICT (ts, usc) DO NOTHING`，可重复执行）；列名映射（`secu_code → usc`、`prev_close → last_close`，`market_code` 丢弃）；同键多 `market_code` 去重保留最新；单事务 + 计数校验，异常整体回滚。
  - 文档：`Docs/FinvQuant/MigratePublicKlineToCdeu.md`（执行、验证、回滚、注意事项）；脚本不进入 `Deploy/Migrations/` 迁移链，验证通过后可作废。
- **Nginx 分阶段部署文档（需求 10775）**
  - `Docs/FinvQuant/Nginx-Deploy.md`：阶段一（HTTP/80 + IP 直连）部署、验证、常见问题；阶段二（HTTPS/443 + HTTP/2）切换与回滚。
  - `Docs/FinvQuant/Nginx.conf.example-ChangeRequest.md`：可**直接作为上游 Issue 正文**提交的模板改造建议（问题现象、期望行为、逐项改动、验收标准、兼容性）。

- ~~`Install.sh --recreate-db`：备份后重建数据库（需求 10770）~~ **该方案已移除**：目标库常与其它服务共用，重建库风险不可接受；现改为只读预检 + 撞表告警，并让迁移脚本兼容 `public` 中他人同名表（见需求 10771）。

- **新增一键下载脚本 `Deploy/Script/Download.sh`（需求 10767）**
  - **一键到位**：自动识别操作系统（Linux / Windows）与 CPU 架构（amd64 / arm64）→ 选对应官方资产 → 下载 → 双层 SHA256 校验（资产 `.sha256` + 包内 `SHA256SUMS`）→ 安全解压到指定目录（默认 `/data/App/FinvQuant`，与 Native 安装根目录一致）。
  - **版本渠道**：`--channel auto`（默认，最新且**包含快照/构建包**）/ `release`（仅正式版）/ `snapshot`（prerelease）/ `pr`（PR 构建包），可用 `--version` 固定 tag、`--list` 列最近 10 个版本、`--dry-run` 预览下载地址。
  - **取源方式**：未提供 token 时走公开 `releases.atom`（近 10 条、含 prerelease、不含 draft）；有 `--token` / `GH_TOKEN` / `GITHUB_TOKEN` 时走 Releases API（近 100 条，排除 draft），支持私有仓库；`--base-url` 可指向内网镜像/加速代理，`-v` 模式不联网解析列表。
  - **安全与防误操作**：解压前扫描包内路径（拒绝绝对路径与 `..`）、先解到临时目录再落盘、目标目录非空需 `--force`；`--force` 下同名文件覆盖、同名目录合并，**不删除**目标已有文件。
  - **两种落盘**：默认铺平（`finvquant` + `Deploy/` 直接进目标目录，可直接接 `Install.sh`），`--keep-top` 保留 `FinvQuant-<版本>-<os>-<arch>/`。
  - **Windows 封装**：新增 `Deploy/Script/Download.cmd`（定位 Git Bash 后转调 `.sh`，不含下载逻辑，排除 System32 的 WSL 转发器）。
  - **文档**：`Deploy/Script/README.md` 新增 §13（渠道/平台映射/校验解压/参数/与 PackSnapshot 的关系/常见问题），`Deploy/README.md` 与 `Deploy/Native/Install.md` 同步新增入口。

- **把 `dedup` 表数据迁入 `cdeu` 对应表（需求 10756，已部署 V100080）**
  - **范围**：26 张表、合计 **133,260 行**。映射 `dedup.cdeu_*` / `dedup.finv_dedup_*` → `cdeu.finv_quote_cdeu_*`，`dedup.finv_quote_secu_kline_min` → `cdeu.finv_quote_secu_kline_min`；`prep_bak_*` 5 张不迁。
  - **只复制不删源**：`dedup` 侧 31 张表数据一行未删，供后续逐步清理淘汰。
  - **迁移 `V100080`**：前置校验（源 26 / 目标 26）→ 逐表 `INSERT ... ON CONFLICT DO NOTHING`（列清单从 `information_schema.columns` 按 `ordinal_position` 动态获取，不依赖 `SELECT *` 顺序）→ 17 个序列按 `pg_depend` 反查真实依赖后 `setval`（空表回初始态）→ 逐表行数收敛校验。
  - **幂等**：全部 `ON CONFLICT DO NOTHING`，重复执行不产生重复行。
  - **序列衔接**：抽查 `job` 50/50、`job_step_dynamic` 132270/132270、`export_file` 1673/1673、`job_run_log_line` 115832/115832（`max(主键)` == `last_value`）。
  - **已知遗留**：三个序列名与实际服务列错位（`finv_quote_cdeu_process_process_id_seq` → `workflow.workflow_id` 等），是 `V100050` 重构 CDEU 模型时的遗留，本次按实际依赖 `setval` 故值正确。
  - **⚠️ 关键状态**：应用代码仍读写 `dedup.*`，`cdeu` 数据是**快照**，会随 `dedup` 变化而逐渐过期。真正切换需改造 200 处 `dedup.cdeu_*` 引用并处理双写/补差；切换完成前**不得删除 `dedup` 表**。
  - **验证**：`V100080 success=t`；26 张表逐表行数与 dedup 完全一致；`dedup` 仍 31 张表；容器 healthy。

- **在 `cdeu` schema 建立与 `dedup` 对应的 26 张表结构（需求 10754，已部署 V100078）**
  - **命名规则**：`dedup_` / `finv_dedup_` 开头 → `finv_quote_cdeu_`；其他表名不变；`prep_bak` 开头**不创建**（本库 5 张全部排除）。
  - **结果**：26 张表 = 12 张 `cdeu_*`（表名不变）+ 13 张 `finv_quote_cdeu_*`（前缀替换）+ 1 张 `finv_quote_secu_kline_min`（表名不变）。**本次不发放任何初始数据**，全部为空表。
  - **派生标识符**：`finv_dedup_*`→`finv_quote_cdeu_*`、`idx_dedup*`→`idx_quotecdeu*`、`uq_dedup*`→`uq_quotecdeu*`，`cdeu_*` 系列不变。
  - **标识符长度**：`finv_quote_cdeu_export_batch_batch_no_usc_biz_month_export_type_key` 加前缀后 67 字符超 PG 的 63 上限（会被静默截断），改用短名 `uq_finv_quote_cdeu_export_batch`（33 字符）。
  - **结构来源**：`pg_dump --schema-only -n dedup`（PostgreSQL 18.4），保证列/类型/默认值/行内 CHECK/NOT NULL 与 dedup 完全一致，不做人工增减。产出 26 表 / 17 序列 / 44 ALTER 约束 / 36 索引 / 1 触发器 / 385 注释。
  - **一致性核验**：逐列比对（列名/类型/长度/可空性/默认值/精度）后，表数量与列集合完全对应；17 张表存在唯一一类差异——`column_default` 中序列的 schema 限定方式（dedup 为 `nextval('seq')` 依赖 search_path，cdeu 为 `nextval('cdeu.seq')` 显式限定），解析结果相同且更安全，属有意改进。
  - **测试同步**：`schema_qualify_test.go` 的 `exactTableSchema["finv_quote_secu_kline_min"]` 白名单加入 `cdeu.`（该表现有 4 份副本：`quote`/`dedup`/`export`/`cdeu`）；`cdeuTables` 清单扩到 26 个表名。
  - **状态**：应用仍读写 `dedup.*`，cdeu 侧仅为结构副本，等后续工作流改造接管；切换仍受 10753 约束（不得 `SET SCHEMA` 机械搬移）。
  - **验证**：`V100078 success=t`；cdeu 下 26 表 / 17 序列 / 80 索引 / 265 约束；`n_live_tup` 合计 0；`prep_bak` 泄漏 0；`go build` / `go vet` / `go test ./...` 全绿。

- **新增 `cdeu` schema 作为 CDEU 工作流域域占位（需求 10753，已部署 V100077）**
  - **范围**：只建空 schema，**表不搬迁**。12 张 `cdeu_*` 表仍在 `dedup`。
  - **`search_path`**：9 段 → 10 段，`cdeu` 紧随 `dedup` 之后、`archive` 之前（`public, quote, quant, futu, export, sche, dedup, cdeu, archive, sys`），数据库级与应用连接串同步。
  - **迁移 `V100077`**：幂等建 schema（属主 `finv`）→ `ALTER DATABASE ... SET search_path` → 三道收敛校验（属主、`USAGE`/`CREATE`、`search_path` 含 `cdeu`）。
  - **代码**：`postgres.go` DSN 加 `cdeu` 并补域名归属注释；3 个集成测试的 `RuntimeParams["search_path"]` 同步；`schema_qualify_test.go` 新增 `cdeuTables` 清单（12 表名）与过渡期说明。
  - **约束（用户明确要求，写入 4 处）**：后续把 `cdeu_*` 表迁入 `cdeu` 时**不是简单的同名迁移**——禁止 `ALTER TABLE ... SET SCHEMA` 式机械搬移，须重新审视表名（去掉重复 `cdeu_` 前缀）、约束/索引命名、主键与外键策略、运行表与定义表拆分粒度、历史数据兼容。落地位置：迁移头注释、`COMMENT ON SCHEMA cdeu`、`DbSchemaSpec §1.1`、`schema_qualify_test.go`。
  - **域边界**：`cdeu` = CDEU 工作流定义与运行实例；`dedup` = 行情批内去重、跨源指纹、入库审计、导出/上传元数据、证券主数据。`finv_dedup_export_*` **不迁**。
  - **踩坑**：首版迁移用 `pg_database.setconfig` 校验失败——PG 18 的 `pg_database` 已无该列（仅 18 列到 `datacl`），数据库级配置实际在 `pg_db_role_setting.setconfig`（`setrole=0`）。修正后 `success=t`。
  - **顺带修正**：`Docs/DevSpec/DbSchemaSpec.md`（47 处）、`BacktestStrategySpec.md`（9 处）里的裸迁移编号引用一并 +100000，与新文件名对齐。
  - **验证**：`cdeu` 属主 `finv`、`USAGE`/`CREATE` 均为 `t`、库内对象 0、`dedup` 下 `cdeu_*` 表仍 12 张、`show search_path` 10 段；`go build` / `go vet` / `go test ./...` 全绿；容器 healthy。

- **补齐字典 3120「币种」主表记录（需求 10749，已部署 V100053）**
  - **问题**：`V100027` 早已写入 `sys.sys_dict_sub` 中 `dict_no=3120` 的 14 条子项（`0`–`9`、`A`–`D` → CNY/USD/HKD/EUR/GBP/AUD/CAD/SGD/JPY/MOP/DKK/GBX/MYR/TWD），但 `V100026` 的 19 条 `sys.sys_dict` 主表记录中没有 3120，形成「有子项、无主表」的孤儿字典。
  - **修复**：`V100053` 只补定义行，不动子项。`dict_name='币种'`、`key_type='1'`、`val_type='2'`、`access_level=1`、`manage_level='0'`、`data_level='2'`、`dict_category='2'`、`gmt_create`/`gmt_modify='2024-12-19 15:28:10.900191'`（与子项同批）；`en_sub_sys` 留空——3121=`act`、3122=`cic` 均为单一子系统归属，而 3120 子项的 `en_sys_str` 是 `gat,gax,gcq,gaa,tms,pbs` 六子系统共用，无单一归属。
  - **新增护栏**：收敛校验断言**全库孤儿字典为 0**（`sys_dict_sub` 中存在但 `sys_dict` 无对应 `dict_no`），后续再出现同类问题迁移会直接失败并列出 `dict_no`。
  - **结果**：字典主表 19 → 20，子项 606 不变，孤儿字典 1 → 0。
  - **可调**：`dict_name` 与 `en_sub_sys` 属推断值（依据已写入迁移注释），迁移用 `ON CONFLICT (dict_no) DO NOTHING`，运维可直接 `UPDATE` 修正且重跑不覆盖。

- **新增美股 OTC Pink 市场 1332（需求 10750，已部署 V100054）**
  - **背景**：原有美股场外市场只有 `1330 US-ADR`（`market_symbol=ADR`）与 `1331 US-OTC`（`market_symbol=USOTC`）；PINK 指 OTC Markets Group 的 Pink 层级，与泛化的 US-OTC 不同口径，需独立登记以便 metadata 刷新与导出目录按市场区分。
  - **取值**：`market_code=1332`、`market_flag='PINK'`、`market_abbr='US'`、`market_name='美股 OTC Pink 场外交易'`、`en_security_type='1313'`、`base_currency='USD'`、`flag_enable='1'`、`exchange_code=0`、`market_symbol='PINK'`。
  - **影响面**：`market_symbol` 会进入 metadata 刷新链路与导出命名（`{region}_{market}_{usc}_Min_{provider}_Mon_{yyyyMM}_Base.mvsv` → `US_PINK_XXXX_...`）；若某证券的 Pink 归属要落到该市场，还需在 `dedup.finv_dedup_usc` 中把 `market` 绑到 `PINK`。
  - **收敛校验**：断言 1332 存在、`market_flag='PINK'` 全库唯一、`market_symbol='PINK'`、`flag_enable='1'` 且 `base_currency='USD'`；幂等 `ON CONFLICT (market_code) DO NOTHING`。
  - **结果**：市场总数 61 → 62（启用 41）。

- **阶段/流程进度条右侧展示完成百分比（需求 10748，落地原 10713）**
  - **范围**：CDEU 主页面 4 处 + 全流程拓扑页 4 处，共 8 个进度条渲染点，全部在右侧加 `NN%` 文本。
  - **同源**：文本值与进度条宽度都取同一个 `phaseProgress()` / `processProgress()` 返回值，不二次计算，不会出现「条走了 62% 文字写 63%」。
  - **CDEU 主页面**：阶段拓扑卡片与 Process 拓扑节点外包 `.workflow-node-progress` flex 容器；阶段概览容器 `min-width` 120 → 168px、Process 块头部 `width` 80 → 128px，进度条改 `flex-grow-1`。
  - **全流程拓扑**：横版阶段进度条由纯 `span` 细条改为 flex（track + percent），`top` 50 → 53px、高 2 → 12px，定位避开 52px 的 header 与 68px 起的首批节点；Process 节点进度条同改，节点高 80px 不溢出；竖版阶段的纵向进度条右侧新增绝对定位百分比标签，落在标题区外、节点之上的空白区。
  - **样式**：`fmtPct()` 统一渲染，`font-variant-numeric: tabular-nums` 保证 `0%` → `100%` 位数变化时不抖动。
  - **验证**：`vue-tsc --noEmit` 无错；前端构建通过；`grep -c "fmtPct("` 在两个视图各命中 5 处（1 定义 + 4 渲染）。

- **手动模式隐藏每日调度 + 流程开始时间改为真实值（需求 10746–10747，已部署 V100052）**
  - **10746**：`DeDupCDEUView.vue` 的「每日调度 HH:MM 时区」chip 加 `v-if="overview?.cdeu_mode === 'Auto'"`。手动模式下调度器不读该时间，展示即误导。工作流详情 tooltip 增加「定时触发」一行，Manual 时明确写「未启用（20005=Manual，仅手动触发）」。
  - **10747 根因**：`EvaluateCDEUProcess` 原写 `started_at = COALESCE(pr.started_at, LEAST(min(job_run.started_at), min(dyn.gmt_start), now()))`。首次评估时子级尚未开始，`LEAST` 忽略 NULL 后落到 `now()`，把「评估时刻」写成流程开始时间；随后 `pr.started_at` 非空，`COALESCE` 永久锁定，子级真实时间再也进不来。`finished_at` 同病：暂停后恢复的 run 里 16:00 判终态、23:44 子 JOB 才跑，导致 `started_at > finished_at`（物理上不可能）。
  - **10747 修复**：Process 层 `started_at = LEAST(min(job_run.started_at), min(job_dynamic.gmt_start))`、`finished_at = GREATEST(max(job_run.finished_at), max(job_dynamic.gmt_finish), started_at, 原值)`；Phase 层取子流程的最早开始 / 最晚结束。子级全无痕迹时 `started_at = NULL`——没有执行痕迹就不写时间。
  - **10747 只读重放路径**：`UpsertReadOnlyCDEUJobRun` 不再写 `started_at = now()`（该 JOB 实际未执行），新行保持 NULL，已有行保留真实开始时间。
  - **V100052**：按新口径重算全部历史 `cdeu_process_run` / `cdeu_phase_run` 的起止时间，含三道收敛校验（process 自洽、phase 自洽、phase 开始不得晚于其子流程）。
  - **前端**：Process 表「开始时间」改用 `processStartedText()`，无开始时间且非 PENDING 时显示「未执行（无子级执行记录）」，避免与「刚创建还没到点」混淆。
  - **验证**：`go build`/`go vet`/`go test ./...` 全绿；`vue-tsc --noEmit` 无错；`V100052` `success=t`；`started_at > finished_at` 残留 process 0 / phase 0；run 62 各流程开始时间严格递增且等于子级最早 `started_at`。

- **合并 `finv_dedup_usc_priority` 到 `finv_dedup_usc`（需求 10745，已部署 V100051）**
  - **动机**：原表是稀疏覆盖表（39 个证券仅 3 行配置），维护需两个界面、两个 API，用户决定合并统一维护。
  - **结构**：`finv_dedup_usc` 新增 `priority_rank` / `sla_days` / `max_wait_days` / `priority_gmt_update` 四列；三列同时为 NULL = 未配置，分别按 **100 / 5 / 7** 兜底，语义与原 `COALESCE` 逻辑完全一致。
  - **约束**：`finv_dedup_usc_priority_group_check` 保证三列同 NULL 或同有值，杜绝半配置态。
  - **迁移 V100051**：前置校验孤儿行 → 加列 → 搬数据 → 加约束 → 收敛校验（搬入数 = 源表数）→ `DROP TABLE`（用户授权）→ 最终确认表已消失。
  - **后端**：`store.go` 删除 `tablePriority` 常量与 2 处 `LEFT JOIN`，`GROUP BY` / `ORDER BY` 改用 `u.priority_rank` / `u.sla_days`；`UpsertPriority` / `DeletePriority` 合并为 `SetUscPriority(ctx, usc, *PriorityUpsertInput)`（`nil` = 清空覆盖）；`UscRow` 与 `ListUscs` 增加 4 列；新增 `ErrUscNotFound`，handler 据此返回 404。
  - **前端**：`DeDupArchiveConfigView.vue` 取消「优先级 / SLA」标签页，证券列表新增「优先级 / SLA 天数 / 最大等待」三列（未配置显示默认值并置灰），行尾「优先级」按钮打开对话框，含「清除覆盖」。
  - **行为收紧（3 处）**：① 不能再给未登记证券单独建优先级行（404）；② `DELETE /Priority` 改为清空覆盖而非删行；③ `GET /Priority/List` 的 `gmt_update` 改名 `priority_gmt_update`。
  - **测试**：`migrate_integration_test.go` 归档表清单 12 → 11；两个 integration test 的插入 SQL 改为 `UPDATE finv_dedup_usc SET ...`。
  - **文档**：`Docs/API/QuoteDeDup/Priority.md` 重写、`Docs/DevSpec/DbSchemaSpec.md` 归档表 12 → 11、`Docs/Refactor/2610-005` 表清单同步。
  - **验证**：`go build`/`go vet`/`go test ./...` 全绿；`vue-tsc --noEmit` 无错。

- **新增系统配置 20011 + 本地导出文件清理 JOB（需求 10744，已部署 V100050）**
  - **配置**：`20011 归档复核是否清理本地导出文件`，`data_type='2'`（char），`char_value='1'`（清理）/`'0'`（不清理），缺行兜底 `'1'`。
  - **JOB 归属**：`PHASE_REVIEW` / `PROCESS_PURGE`，`seq_no=2`，排在 `JOB_PURGE_EXPORTED_MONTH`（seq 1，删主表）之后——先释放库里数据，再释放本地文件。
  - **删除判据（4 条全满足）**：该月 BASE+全部 PATCH 均 `UPLOADED`；`remote_checksum_verified` 全为 true；最后一个文件 `gmt_upload_finish` 早于 `now() - 24h`；该月无 `PENDING`/`RUNNING` 补丁。
  - **删除范围**：该月全部本地文件——`.mvsv`（BASE + 所有 `_Incr_N`）与 `Manifest.json`，随后 `cleanup_status='LOCAL_DELETED'`。文件不存在视为已清理（幂等）。
  - **职责收缩**：`JOB_CLEAN_STORAGE` 移除本地文件段，只保留 export schema 中转表残留清理；`LocalRetention` 降级为「孤儿文件兜底」（只处理登记已不存在的磁盘残留）。
  - **记录保留**：`finv_dedup_export_file` 记录**永久保留**（只改 `cleanup_status`），理由是幂等判据、审计留痕、补丁链引用（`file_patch.patch_file_id`）与 checksum 去重。已知成本：每轮约 404 条记录、单条约 300 字节，随运行轮次线性增长。
  - **新增代码**：`syscfg.KeyPurgeLocalExportFiles=20011` + `PurgeLocalExportFilesDelay=24h` + `Values.PurgeLocalExportFiles`；`Service.PurgeLocalExportFilesEnabled`；`JobPurgeLocalExportFiles="PURGE_LOCAL_EXPORT_FILES"`；`Store.LocalFilesForSettledMonths` + `LocalFileToPurge`；`job/purge_local_export_files.go`；`jobmeta.PurgeLocalExportFiles`。
  - **验证**：`gofmt` 干净、`go build`/`go vet`/`go test ./...` 全绿；`V100050` `success=t`；`20011` = `'1'`；`PROCESS_PURGE` 下 `JOB_PURGE_EXPORTED_MONTH:1` → `JOB_PURGE_LOCAL_FILES:2`，均 `affects_completion=true`。

- **BASE 文件名增加 `_Base` 后缀（需求 10743，已实现）**
  - **变更**：`{region}_{market}_{usc}_Min_{provider}_Mon_{yyyyMM}.mvsv` → `..._Mon_{yyyyMM}_Base.mvsv`，与 PATCH（`_Incr_N`）、MANIFEST（`_Manifest`）的类型后缀统一。
  - **代码**：`naming.go` 的 `FileMeta.BaseFileName()` 加 `_Base`；`naming_test.go` 期望值同步。
  - **影响**：
    - GitHub 上已有 101 个旧名 BASE（无 `_Base`）**不会被自动删除**，与新建文件并存；
    - 「BASE 无版本号、重写即同名覆盖」只对新命名生效，旧名文件不再被覆盖；
    - MANIFEST 的 `base.file_name` 与实际文件一致（同源 `BaseFileName()`）。
  - **验证**：`gofmt` 干净、`go build` / `go vet` / `go test ./...` 全绿。

- **修复空转循环靠刷心跳免疫超时回收（需求 10736，已部署）**
  - **现象**：`JOB_EXPORT_BASE_GCMain_201901` 卡 47 分钟不结束、也不超时；日志 2,078 行全是 `job_id=10 date=-155839383 已装载 0 行` 这类逐次变小的负日期。
  - **根因（两个缺陷叠加）**：
    1. `loadJob` 外层 `for date := cursorDate; date <= lastDate; date = nextDate(date)` 在游标异常时空转——`nextDate` 产生负数后 `date <= lastDate` 永远成立；
    2. 每轮循环都调 `UpdateLoadProgress`，它无条件刷新 `gmt_heartbeat = now()` 与 `locked_until = now() + lease`。而 `RECOVER_TIMEOUT_EXPORT_JOBS` 的判据是 `status IN ('LOADING','LOADED','EXPORTING') AND locked_until < now()`——**租约永远是新的，回收永远认不到它**。程序无法区分「真的在干活」与「空转刷心跳」。
  - **修复（`job_export.go`）**：
    - `loadJob` 外层：`nextDate(date)` 必须严格大于 `date`，否则立即报错；并加总轮数上限 `maxLoadDays = 62`（正常一个月最多 31 天，留一倍余量），超出报「装载游标异常：已循环 N 轮仍未覆盖 …」；
    - `loadDate` 内层 keyset 分页：`MaxTS` 在 `PageRows > 0` 时必须严格大于上一轮 `prevTS`，否则报「装载分页游标不前进」。
  - **效果**：同类异常从「无限空转 + 永不超时」变为「数十轮内报错退出」，随后由 `FAILED` → 重试/超时机制接管。
  - **验证**：`gofmt` 干净、`go build` / `go vet` / `go test ./...` 全绿。

- **修复阶段级失败传播失效、异常态隐藏手动执行按钮、误标 CANCELLED（需求 10733–10735，已部署）**
  - **10733 阶段（Phase）级缺串行与失败传播门禁**
    - 现象：run 59 中 `PHASE_EXPORT` = `FAILED`，但 `PHASE_UPLOAD`、`PHASE_REVIEW` 仍显示 `SUCCESS`。
    - 根因：上一轮（10729）只补了 **Process 层**门禁 `cdeuProcessHasEarlierUnfinished`，**Phase 层完全没做**——`EvaluateCDEUPhase` 仍然只看自己子级，`PHASE_UPLOAD` 的子级恰好全 `SUCCESS`（那 2 个被竞态导出的文件确实上传成功了），于是判 `SUCCESS` 并向上传染。
    - 修复（`cdeu_store.go`）：新增 `cdeuPhasePredecessorState(ctx, runID, workflowID, seqNo)`，`EvaluateCDEUPhase` 在评估前检查同工作流内 `seq_no` 更小且非成功的阶段：
      - 前序 `PENDING`/`READY`/`RUNNING` → 本阶段保持 `PENDING`；
      - 前序 `FAILED`/`PARTIAL`/`CANCELLED` → 本阶段标 `SKIPPED`（业务上不会真正执行，不能显示「成功」）；
      - 旁路阶段（`affects_completion=false`）不参与门禁。
  - **10734 异常态 JOB 隐藏「手动执行」按钮**
    - 根因：`showRunButton` 对 `EMBEDDED` 步骤只在 `JOB_EXPORT_LOAD` 上放行，`JOB_EXPORT_EXPORT`/`REGISTER`/`CLEANUP` 全部无按钮；`manual_rerun_enabled=false` 的 JOB 亦一律无按钮。JOB 失败后用户没有任何重试入口。
    - 修复（`DeDupCDEUView.vue`）：`FAILED`/`PARTIAL`/`PAUSED`/`CANCELLED` 一律显示手动执行按钮，优先于 `EMBEDDED` 与 `manual_rerun_enabled` 的限制；动态 JOB 的允许状态补上 `PARTIAL`。
  - **10735 误标 CANCELLED（操作失误，已记录规范）**
    - 经过：`fix_run59.sql` 把 run 59 的 phase/process/job 未终态行批量更新为 `CANCELLED`（phase 2 条、process 4 条、job_run 4 条、job_step_run 4 条、job_dynamic 99 条、job_step_dynamic 396 条），导致界面「状态」列出现「已取消」而用户不知情。
    - 规范：批量改写运行状态前必须征得用户同意；确需取消时应写入变更日志并说明影响范围。
  - **验证**：`gofmt` 干净、`go build` / `go vet` / `go test ./...` 全绿；二进制确认 `cdeuPhasePredecessorState` 已编译进去；镜像重建部署后容器 healthy、`/Health/Ready` = 200。

- **修复阶段内流程并行为、统计列拆分、执行中开始时间为空（需求 10729–10731，已部署）**
  - **10729 同阶段流程未串行**
    - 现象：`PROCESS_EXPORT_FILE_MON_BASE` 与 `PROCESS_EXPORT_FILE_MON_INCR` 同时 `RUNNING`；且两者 `started_at` 均为空。
    - 定性：**状态显示与逻辑控制都有缺陷**。
      - 显示层：`EvaluateCDEUProcess` / `EvaluateCDEUPhase` 的 UPDATE 只写 `status` 与 `finished_at`，**从不写 `started_at`**，导致执行中的流程开始时间列为空。
      - 控制层：评估逻辑只看自身子级——BASE 的子级是动态导出实例、INCR 的子级是 `JOB_SCAN_PATCH` 补丁扫描，两者由不同触发点独立驱动，因此并行状态被如实反映出来；**从不检查同阶段前序流程是否已完成**。
    - 修复（`cdeu_store.go`）：
      - 新增 `cdeuProcessHasEarlierUnfinished(ctx, runID, phaseID, seqNo)`：同 Phase 内存在 `seq_no` 更小、`affects_completion=true` 且未终态的前序流程时返回 `true`；
      - `EvaluateCDEUProcess` 开头执行门禁：被前序阻塞时强制 `PENDING` 并清空 `started_at`/`finished_at`，不再进入 `RUNNING`/`SUCCESS`（旁路的 `PROCESS_RECOVER` 因 `affects_completion=false` 不受阻塞）；
      - `EvaluateCDEUProcess` / `EvaluateCDEUPhase` 的 UPDATE 补写 `started_at = CASE WHEN $n = 'RUNNING' THEN COALESCE(started_at, now()) ELSE started_at END`。
  - **10730 / 10731 流程表格统计拆列**（`DeDupCDEUView.vue`）
    - 改前：`状态 | JOB(1/102 · 执行中 1) | 批次(5/10 · 失败 2)`
    - 改后：`状态 | JOB总数 | 成功 | 失败 | 执行中 | 批次总数 | 批次失败`，各自独立成列，空值显示 `-`；空表 `colspan` 由 10 → 12 → 14 同步调整。
  - **验证**：`gofmt` 干净、`go build` / `go vet` / `go test ./...` 全绿；二进制中确认 `cdeuProcessHasEarlierUnfinished` 与 `started_at = CASE WHEN` 均已编译进去；前端产物含「JOB总数」「批次总数」「批次失败」三列。

- **竞态事故记录（需求 10732）：数据修复脚本与运行中调度循环**
  - **经过**：`fix_run59.sql` 把 job 8/9 从 `FAILED` 直接 UPDATE 回 `WAITING` 的那一刻，旧容器内仍在循环的 `RUN_DUE_EXPORT_JOBS`（低峰窗口内每 60 秒一轮）立刻 claim 到这两个 job：
    ```
    17:35  fix_run59.sql 重置 job 8/9 为 WAITING
    17:36:28  job 8 开始装载 → 17:36:31 导出 20,381 行 → BASE v2 上传成功 → 20007 删主表
    17:36:43  job 9 开始装载 → 17:36:43 导出 27,600 行 → BASE v2 上传成功 → 20007 删主表
    ```
    主表因此少 47,981 行（2,803,911 → 2,755,930），并生成 4 条文件登记与 4 条上传日志。
  - **不是 Manual 模式失效**：`20005=Manual` 的拦截仍然有效，是「修复动作主动把 job 放回可 claim 状态」触发了已在循环中的 worker。
  - **恢复**：从 `quote` 回填 47,981 行（201811=20,381 / 201812=27,600），job 8/9 重置为 `WAITING` + 游标 `NULL` + `base_version=1`，删除文件登记 4 条 / 上传日志 4 条 / 批次 2 条，清理本地导出目录 `GCMain/201811`、`GCMain/201812`。主表回到 2,803,911 行 / 94 月。
  - **操作规范（已登记为需求）**：修复脚本必须先把 job 置为 `FROZEN`/`PAUSED` 等不可 claim 状态、确认无运行中循环后再改回 `WAITING`；或修复期间临时停止容器。
  - **副作用**：GitHub 仓库中 `US_COMEX_GCMain_Min_FTMM_Mon_201811.mvsv` / `_201812.mvsv` 已是 v2 内容；因 BASE 文件名无版本号，重跑会同名覆盖，不影响后续验证。

- **修复 run 59 暴露的 5 个缺陷（需求 10724–10728，已部署）**
  - **10724 导出 LOAD 游标越界导致 int4 溢出、JOB 卡死**（根因级修复）
    - 现象：`JOB_EXPORT_BASE_GCMain_201811` 报文 `failed to encode args[1]: unable to encode -2147539797 into binary format for int4`；日志刷出 `date=0`、`date=-8799`、`date=-79475`、`date=-158882`… 逐次放大的负数日期。
    - 根因：准备脚本把 `load_cursor_date` 重置为 **`0`（非 NULL）**，`loadJob` 把「非 NULL 的 0」当作有效游标采纳；`nextDate(0)` 调 `time.Date(0, time.Month(0), 0)` 被归一化成负数日期并逐日下探，游标一路跌到 `-2147469187`，绑定 int4 参数时溢出。
    - 修复（`job_export.go`）：
      - 游标必须满足 `firstDate <= cursorDate <= lastDate` 才采纳，否则回退当月 1 日；`LoadCursorTS` 增加 `>= 0` 校验；
      - `nextDate` 增加年月日合法性防御（`year < 1900 || month ∉ [1,12] || day ∉ [1,31]` 时原值返回），杜绝非法输入进入 `time.Date`。
    - 数据修复：`load_cursor_date` / `load_cursor_ts` 全部置 **NULL**（"未开始装载"的唯一正确表示），101 个 job 回到 `WAITING`，清锁清心跳清 `last_error`；清导出决策遗留批次 101 条与水印 1 条；run 59 及其 2/4/4/4/99/396 条子级记录标记 `CANCELLED`。主表 2,803,911 行未受影响。
  - **10725 空流程被误判 SUCCESS（父级提前完成）**
    - 现象：`PHASE_EXPORT`/`PHASE_UPLOAD` 仍 `RUNNING`，`PHASE_REVIEW`（归档复核）已 `SUCCESS`。
    - 根因：`PROCESS_REVIEW` 有流程定义但 `cdeu_job` 中 0 条 JOB，`cdeuProcessChildren` 返回空集 → `decideCDEUStatus(total=0)` 判 `SUCCESS`；该状态再向上传染使 `PHASE_REVIEW` 也提前完成。上一轮只修了「job_run 未创建 → 子级为空」，没覆盖「根本没有任何 JOB 定义」这种真正的空流程。
    - 修复（`cdeu_store.go`）：`EvaluateCDEUProcess` / `EvaluateCDEUPhase` 在子级集合为空时**不得判完成**——非终态保持 `PENDING`；已是终态但定义被清空的回退为 `PENDING` 并清空 `finished_at`。
  - **10726 流程概览 JOB 列混展示两个指标**
    - 现象：JOB 列显示 `1/102 · 执行中 1`。
    - 修复（`DeDupCDEUView.vue`）：JOB 列只显示 `成功/总数`；新增「执行中」「失败」两个独立列（表头顺序：状态 → JOB → 执行中 → 失败 → 批次），空值显示 `-`；空表提示 `colspan` 由 10 调整为 12。
  - **10727 运行中的动态 JOB 无法查看执行日志**
    - 现象：`dynamic_job_id=6542`（`JOB_EXPORT_BASE_GCMain_201812`）状态 `RUNNING` 但 `run_log_id IS NULL`，前端 `v-if="job.run_log_id"` 因此不渲染「执行日志」按钮；而日志本身已在写入（`run_id=7470` 有 13,254 行）。
    - 根因：`job/export.go` 在标记 `RUNNING` 时传 `runLogID = 0`，`COALESCE(NULLIF(0,0), run_log_id)` 于是保持 `NULL`，只有终态（FAILED/SUCCESS）才带上 `outcome.RunLogID`。
    - 修复：`MarkDynamicJobStatus(..., CDEUStatusRunning, "", dedup.RunLogIDFromContext(ctx))`——运行中立即回写日志 ID。
  - **10728 跑马灯加在了错误页面**
    - 现象：动效只加在「全流程拓扑」（`DeDupCDEUWorkflowView.vue`），用户在「流程概览」（`DeDupCDEUView.vue`）看不到。
    - 修复：流程概览补齐青色跑马灯——`Phase` 卡片（`.phase-card-running`，`v-card` 加 `phase-card` 类 + `:class` 绑定）与 `Process` 区块头部（`.process-header-running::after`）均叠加 115° 斜向流光带（`cdeuMarquee`，2.2s 循环）；内容经 `z-index: 1` 抬到光带之上；`running-dot`、选中行左边条、列宽拖拽线由蓝色 `#1976d2` 统一改为青色 `rgb(0,229,255)`；`prefers-reduced-motion: reduce` 下关闭动效仅留静态描边。
  - **验证**：`gofmt` 干净、`go build ./...` / `go vet ./...` / `go test ./...` 全绿；镜像重建部署后容器 healthy、`/Health/Ready` = 200；前端产物校验 `cdeuMarquee` 3 处、`phase-card-running` 4 处、`process-header-running` 4 处、青色 `#00e5ff` 37 处；修复后状态：101 个 job 全 `WAITING` 且游标全 `NULL`、锁 0、批次 0、水印 0、中转 0、文件 0、主表 2,803,911 行 / 94 月不变。

- **新增待办需求 10723（暂不实现）：按 workflow_run_id 清理历史流程运行数据**
  - **只登记不动代码**。
  - **两种可选策略**：`Days`（删超过 N 天，默认 30）或 `Count`（只保留最近 N 个 `workflow_run_id`，默认 30），支持 `Off` 关闭；配置建议走 `sys_config` 20009（策略）+ 20010（阈值）。
  - **清理范围 9 张表**，按子→父顺序单事务删除：`cdeu_job_step_dynamic` → `cdeu_job_dynamic` → `cdeu_job_step_run` → `cdeu_job_run` → `cdeu_process_run` → `cdeu_phase_run` → `finv_dedup_job_run_log_line` → `finv_dedup_job_run_log_export_file` → `cdeu_workflow_run`。
  - **关联陷阱**：`finv_dedup_job_run_log_line.run_id` / `finv_dedup_job_run_log_export_file.run_id` 是 **JOB 运行日志批次 ID**（对应 `cdeu_job_run.run_log_id`），不是 `workflow_run_id`，必须经子查询间接关联。
  - **安全约束**：只删终态 run；永不删最近 1 个 run；孤儿 run（`cdeu_job_run` 有 `workflow_run_id` 但 `cdeu_workflow_run` 无对应行，当前有 13 个）按 `cdeu_job_run` 扫描而非依赖父表；逐 run 记录各表删除行数。
  - **与 20008 的关系待定**：按 run 级联清理覆盖后，20008（动态实例保留天数）可降级为兜底或直接由 20009/20010 取代。
  - **数据量基线**：`finv_dedup_job_run_log_line` 39,094 行 / 8.0 MB、`finv_dedup_job_run_log_export_file` 7,100 行 / 2.8 MB 为占用大头。

- **新增待办需求 10717–10722（暂不实现）：收尾阶段 JOB 重分配与解耦**
  - **只登记不动代码**。10716 的详细设计，用户已批准全部 4 项决策。
  - **批准结论**：① 拆分四个新流程；② `JOB_RECOVER_TIMEOUT` 移入 `PHASE_REVIEW`；③ 执行顺序认可；④ 收尾 5 个 JOB **全部为关键 JOB**（`affects_completion = true`）。
  - **撤销 `JOB_CLEAN_EXPIRED`**（原一肩挑四类不相关职责），拆为三个独立 JOB：`JOB_PURGE_EXPORTED_MONTH`（删 dedup 主表）、`JOB_CLEAN_STORAGE`（中转表残留 + 本地过期文件）、`JOB_PURGE_STALE_DYNAMIC`（历史动态实例，20008）。
  - **新 `PHASE_REVIEW` 结构**（5 个流程按序）：`PROCESS_RECOVER`(1) → `PROCESS_REVIEW`(2) → `PROCESS_PURGE`(3) → `PROCESS_STORAGE_CLEAN`(4) → `PROCESS_RETENTION`(5)。
  - **`PHASE_UPLOAD` 精简**为 `PROCESS_UPLOAD` → `JOB_UPLOAD_FILES` 一条，内联主表清理移除；`PROCESS_LIFECYCLE_CLEAN` / `PROCESS_HEALTH` 一并撤销或迁出。
  - **关键解耦**：`JOB_PURGE_EXPORTED_MONTH` 改扫描式判据——按 `(usc, biz_month)` 聚合 `finv_dedup_export_file`，凡「全部文件 `UPLOADED` 且该月主表仍有数据」即删，不再区分本次上传与历史遗留。原「沉淀月补偿清理」概念随之消失（它就是同一件事的补丁写法）。

- **新增待办需求 10716（暂不实现）：20007 主表清理改为「归档复核」阶段独立 JOB**
  - **只登记不动代码**，下一次开发实施。
  - **现状**：清理逻辑内联在 `UPLOAD_PENDING_EXPORT_FILES`（`job/upload_pending_export_files.go:80-92`），每上传成功一个 BASE/PATCH 即删该 `(usc, biz_month)` 的 `dedup` 主表数据。
  - **核心问题**：删除发生在**归档复核（`PHASE_REVIEW`）之前**——若复核发现文件异常，源数据已不可恢复，只能依赖 `quote` 副本（与 `dedup` 不保证逐行一致）。
  - **目标**：拆为独立 JOB `JOB_PURGE_EXPORTED_MONTH`，定义在 `PHASE_REVIEW` → `PROCESS_REVIEW`（该阶段当前已有流程定义但 0 个 JOB，正好承载）。
  - **待定 5 点**：复核门禁口径、20007 语义是否改写、切换方式、失败重试策略、历史补删方案。

- **表名对齐：`cdeu_job_dynamic_step` → `cdeu_job_step_dynamic`（迁移 `V100047`，已部署 `success=t`）**
  - **决策依据**：用户明确要求「`cdeu_job_dynamic_step` 表需要重命名为 `cdeu_job_step_dynamic`」。
  - **命名理由**：CDEU 的 Step 相关表有 `cdeu_job_step`（定义）与 `cdeu_job_step_run`（运行），动态 JOB 的 Step 表原名把 `dynamic` 放在 `step` 之前，与两者都不对齐；改名后统一为 `cdeu_job_step_*` 前缀。
  - **改名范围（表 + 7 类附属对象全部对齐）**：
    | 类型 | 旧名 | 新名 |
    |---|---|---|
    | 表 | `cdeu_job_dynamic_step` | `cdeu_job_step_dynamic` |
    | 序列 | `cdeu_dynamic_job_step_dynamic_step_id_seq` | `cdeu_job_step_dynamic_dynamic_step_id_seq` |
    | 主键 | `cdeu_dynamic_job_step_pkey` | `cdeu_job_step_dynamic_pkey` |
    | 唯一 | `cdeu_dynamic_job_step_dynamic_job_id_seq_no_key` | `cdeu_job_step_dynamic_dynamic_job_id_seq_no_key` |
    | 唯一 | `cdeu_dynamic_job_step_dynamic_job_id_step_code_key` | `cdeu_job_step_dynamic_dynamic_job_id_step_code_key` |
    | 检查 | `cdeu_dynamic_job_step_status_check` | `cdeu_job_step_dynamic_status_check` |
    | 非空 | `cdeu_dynamic_job_step_<col>_not_null` ×8 | `cdeu_job_step_dynamic_<col>_not_null` |
    | 索引 | `idx_cdeu_job_dynamic_step_job` | `idx_cdeu_job_step_dynamic_job` |
  - **安全性**：幂等（已是新名则跳过）；约束/序列改名用 `DO` 块逐个探测存在性，缺名不报错；全程只改名字，不动数据、不动列定义、不重建表。
  - **收敛校验（8 项）**：旧表不存在 / 新表存在 / 新序列存在 / 无 `cdeu_dynamic_job_step_%` 残留约束 / 新索引存在 / 主键为新名 / 唯一约束数=2（`seq_no` 与 `step_code` 双唯一，`ON CONFLICT DO NOTHING` 依赖）/ 序列归属列仍为 `dynamic_step_id`。
  - **代码同步（6 处）**：`store.go` 的 `tableDynamicStep` 常量、`cdeu_admin.go` 的 Overview Step 查询、`service_integration_test.go`（3 处 SQL）、`migrate_integration_test.go`（表清单 + 索引清单）。
  - **验证**：`dedup.cdeu_job_step_dynamic` 存在且旧名 `to_regclass` 为 NULL，行数 **1,345 不变**，`\d` 显示序列/PK/2 个 UNIQUE/CHECK/索引全部新名，8 个非空约束全部新名；`go build ./...`、`go vet ./...`、`go test ./...` 全绿。
  - **未改**：`V63` / `V100045` 为历史已应用迁移，按规范不得修改，其内部旧表名保持原样。

- **清理全部历史备份/快照表（用户明确授权，17 张）**
  - **授权依据**：用户明确指示「A B C D 都删」（A=测试过程产物、B=迁移 DDL 快照、C=CDEU 脏数据清理备份、D=行情数据清理备份）。
  - **删除前校验**：17 张表全部 `dependent_objects = 0`（无视图/外键依赖）；三张主表 `dedup` / `quote` / `export` 均不在删除清单内。
  - **删除清单（17 张，释放 2,240 kB）**：
    - 迁移 DDL 快照（5）：`cdeu_process_run_bak_v100044`、`cdeu_job_run_bak_v100044`、`cdeu_job_step_run_bak_v100044`、`cdeu_job_dynamic_bak_v100045`、`cdeu_job_dynamic_step_bak_v100045`
    - CDEU 脏数据备份（2）：`cdeu_job_run_purge`(589)、`cdeu_job_step_run_purge`(433)
    - 导出测试准备快照（8）：`finv_dedup_export_job_bak_full`(101)、`finv_dedup_export_file_bak_full`(206)、`finv_dedup_export_file_upload_log_bak_full`(700)、`finv_dedup_export_file_patch_bak_full`、`finv_dedup_export_file_bak_r2`、`finv_dedup_export_file_patch_bak_r2`、`finv_dedup_export_file_bak_201811`、`finv_dedup_export_file_upload_log_bak_201811`
    - 行情数据清理备份（2）：`finv_quote_secu_kline_min_purge_day`(2,690)、`finv_quote_secu_kline_min_kline_purge`(73)
  - **结果**：`pg_class` 中已无 `%_bak%` / `%_purge%` 残留表；主表行数不变（`dedup` 1,000 / `quote` 2,802,911 / `export` 0）。
  - **影响**：`V100039`（月 K 清理）与日 K 清理的操作文档中引用的 purge 备份表已不存在，后续如需回滚只能依赖 `quote` 副本；后续同类清理若需回滚证据，应在删除前先导出 CSV 留档。

- **修复 CDEU 父级状态「提前完成」缺陷 + 运行中节点跑马灯动效**
  - **缺陷现象**：父级 Process / Phase 在子级 JOB 还在运行时已显示 `SUCCESS`，后续阶段未按预期显示「待执行」。
  - **数据铁证（run 58）**：
    ```
    PROCESS_EXPORT_BATCH  finished_at = 15:43:50.848
      └─ JOB_FETCH_EXPORT_BATCH  started_at = 15:43:56.853   ← 父级"完成"6 秒后子级才开跑
    PHASE_EXPORT  finished_at = 15:43:50.850
      └─ PROCESS_EXPORT_FILE_MON_BASE  finished_at = 15:50:37.692  ← 父级提前 6 分 47 秒
    PHASE_UPLOAD  finished_at = 15:43:50.828
      └─ PROCESS_UPLOAD  finished_at = 15:59:21.534  ← 父级提前 15 分 31 秒
    ```
  - **根因**：子级集合从**运行记录**取（`cdeuProcessChildren` 查 `cdeu_job_run`、`cdeuPhaseChildren` 查 `cdeu_process_run`、`cdeuWorkflowChildren` 查 `cdeu_phase_run`）。而运行记录只在子级真正开始执行时才创建——`SeedCDEUProcessRuns` 之后、`job_run` 建立之前，子级集合为空，`decideCDEUStatus(total == 0)` 直接返回 `SUCCESS`，父级被提前判完成；叠加 `TouchCDEUHierarchy` 的 `WHERE status IN ('PENDING','READY','RUNNING')` 拒绝回退，状态永久停在 `SUCCESS`。
  - **修复**：
    1. `cdeuProcessChildren` 改为 `FROM cdeu_job j LEFT JOIN cdeu_job_run r`，缺失运行记录按 `PENDING`；
    2. `cdeuPhaseChildren` 改为 `FROM cdeu_process ps LEFT JOIN cdeu_process_run r`（签名由 `phaseRunID` 改为 `(runID, phaseID)`）；
    3. `cdeuWorkflowChildren` 改为 `FROM cdeu_phase ph LEFT JOIN cdeu_phase_run r`；
    4. `TouchCDEUHierarchy` 允许从 `SUCCESS`/`SKIPPED` 回退为 `RUNNING`（并清空 `finished_at`），仍不覆盖 `FAILED`/`PARTIAL`/`CANCELLED`；
    5. `EvaluateCDEUProcess` / `EvaluateCDEUPhase` 对已终态父级，新增 `cdeuProcessHasActiveChild` / `cdeuPhaseHasActiveChild` 判断：仍有 `PENDING/READY/RUNNING` 子级时重新评估，从而把错误完成标记改回。
  - **效果验证（纯 SQL 模拟缺失运行记录）**：
    ```
    旧逻辑：process_id=5 子级数 = 0        → decideCDEUStatus 判 SUCCESS
    新逻辑：process_id=5 子级 5 条全 PENDING → 判 RUNNING/PENDING
            phase_id=3 下 4 个 Process 全 PENDING；工作流下 6 个 Phase 全 PENDING
    ```
  - **新增动效（需求 10710）**：`DeDupCDEUWorkflowView.vue` 全流程拓扑中，`RUNNING` 的**阶段虚线框**与 **Process 矩形框**叠加青色（`rgb(0,229,255)`）斜向流光带跑马灯（`fulltopo-marquee`，2.2s 循环），并配青色边框脉动；阶段/节点内容通过 `z-index` 抬到光带之上。JOB 级不在拓扑图展示，未加动效。
  - **无障碍**：`@media (prefers-reduced-motion: reduce)` 下关闭跑马灯与脉动，仅保留静态青色高亮。

- **CDEU 页面展示运行模式（20005）**
  - **后端**：`CDEUOverview` 新增 `cdeu_mode`（`Manual`/`Auto`）与 `cdeu_schedule_enabled`（仅 `Auto` 为 `true`）；`Service.CDEUOverview` 读取 `syscfg` 20005 填充，读取失败或未配置时按 `Manual` 兜底（与调度器安全侧一致），前端无需额外请求 `Status`。
  - **前端**：`DeDupCDEUView.vue` 页头原有的「运行模式」chip 原先展示的是 `workflow.run_mode`（工作流定义模式，与 20005 不是同一概念），现**替换为 20005 的取值**：`Auto` 显示「自动」（绿色 + `mdi-robot-outline`）、`Manual` 显示「手动」（灰色 + `mdi-hand-back-right-outline`），并附悬浮说明「系统配置 20005」。
  - **不受影响**：`runModeText()` 仍用于 Phase 级 `run_mode` 展示（阶段明细与卡片视图），保持不变。
  - **验证**：`GET /CDEU/Overview` 返回 `cdeu_mode=Manual`、`cdeu_schedule_enabled=false`；容器重建后库内准备状态不变（主表 2,803,911 行、202610 陷讲 1,000 行、201811 job 仍为 `WAITING`）。

- **修复 20007「死水区」：新增沉淀月补偿清理（方案 B）**
  - **缺陷现象**：主表 `dedup.finv_quote_secu_kline_min` 长期残留 12 个证券月 / 346,413 行；重新跑流程时**既不清理、也不生成 INCR 增量**，且流程汇总层没有任何提示。
  - **根因链**：
    - `RegisterFile` 在 **checksum 未变**且原状态 `UPLOADED` 时**保持 `UPLOADED`**（避免重复上传同一内容）；
    - 而 `20007` 的删除原先只绑定在「**本次实际发生上传动作并校验通过**」之后；
    - 于是「重新导出内容相同的月份」→ 不进上传队列 → 无上传动作 → 不触发删除；
    - 同时水位线（`finv_dedup_scan_watermark`）已推进到数据最后修改时间之后 → `CountMainDelta` 恒为 `0` → 永不生成 INCR。
    - 形成「已导出 / 未删除 / 无增量」的死水区。实测：数据 `gmt_update = 2026-10-24 14:13`，水位线 `2026-10-27 14:26`，而 BASE 文件 `gmt_create = 2026-10-27 03:04`、`row_count` 与主表逐月完全一致（证明数据早已完整导出）。
  - **修复（方案 B，用户指定）**：`CLEAN_EXPIRED_EXPORT_ARTIFACTS` 新增「沉淀月补偿清理」步骤，**仅当 20007=1 时执行**：
    - 新增 `Store.SettledSecurityMonths(ctx, limit)`：判定「BASE 已 `EXPORTED` + 已 `UPLOADED` + 主表该月有数据 + **BASE 生成之后该月主表无任何 `gmt_create`/`gmt_update` 晚于 `BASE.gmt_create` 的行**」；
    - 新增 `job.purgeSettledMainData`：逐月按 `50000` 行一批循环删空（复用 `PurgeMainSecurityMonth`），单轮上限 `500` 个月（`settledPurgeBatch`）；
    - `details` 新增 `settled_months_purged` / `settled_rows_purged`，逐月日志 `20007 沉淀月清理 usc=… biz_month=… rows=… file_id=…`；
    - **不做行级备份**（需求方约定）：数据在远端 BASE 文件与 `quote` 副本中各有一份完整拷贝；删除只作用于 `dedup`。
  - **验证**：
    - 实跑清理：`dedup` 主表 `346,413 → 0`，`quote` 主表保持 `2,802,911` 不变；`details` = `settled_months=12 settled_rows=346413`；
    - 闭环验证（事务内）：主表清空后 `delta = 0`；写入 1 行新数据（`gmt_create=now()`）后 `delta = 1 > 0`（证明 SCAN 会重新生成 INCR）；同一场景下该月**不会**被判定为「已沉淀」（不误删）；验证后 `ROLLBACK`，数据未污染。
  - **文档**：`Docs/Refactor/2610-005-FinvQuoteDeDubExportUpload.md` 新增 **§4.6 主表数据释放（系统配置 20007）**，明确两条触发路径、路径②的三重判定条件、删除方式与观测字段、以及与增量链路的关系；`Docs/API/QuoteDeDup/JobRun.md` 补 `CLEAN_EXPIRED_EXPORT_ARTIFACTS` 的 `details` 字段表；`DbSchemaSpec` 同步 20007 语义。

- **新增系统配置 20008「历史运行动态 JOB 保留天数」（`V100046`）**
  - `20008`（`data_type='1'`，取值存 `int_value`，默认 `30` 天）：配合 `V100045` 的动态实例按运行隔离（每次流程运行各持有一条实例），把原先固化为代码常量的推进保留期改为可在线配置。
  - **合法区间 `1~3650`**（约 10 年）；越界（含 `0`、负数）或读取失败时，运行期统一回退默认 `30` 天，保证清理逻辑始终有界。
  - **不覆盖用户已配置值**：`INSERT ... ON CONFLICT (config_no) DO NOTHING`；迁移内对越界值只 `RAISE WARNING`，不阻断。
  - **生效位置**：`CLEAN_EXPIRED_EXPORT_ARTIFACTS` 调用 `PurgeStaleDynamicJobRuns(retentionDays, 5000)`，`details` 新增 `stale_dynamic_deleted` 与 `dynamic_retention_days`，日志打印 `retention_days=`。
  - **API**：`GET .../Settings` 与 `GET .../Status` 的 `options` 均新增 `dynamic_job_retention_days`。
  - **代码**：`syscfg.KeyDynamicJobRetentionDays`、`DynamicJobRetentionDefault`、`DynamicJobRetentionMax`、`NormalizeDynamicJobRetentionDays`；`selectConfigSQL` 增加读取 `int_value`（参数扩到 `$7`）；`Values.DynamicJobRetentionDays/Raw`；`Service.DynamicJobRetentionDays(ctx)`（读配置失败回退 30）。
  - **验证**：迁移 `100046` `success=t`，20008 = `30`（`create_by=finv`）；改 `7` 后 `Settings` 返回 `7`，改 `0`（越界）后返回 `30`，改回 `30` 后返回 `30`；清理 Job 运行日志 `progress_detail` 显示「清理历史动态 JOB 实例（保留 30 天）」，完成日志含 `retention_days=30 stale_dynamic_deleted=0`（当前无超过 30 天的运行数据）。

- **动态 JOB 改为「每次运行一条实例」（`V100045`）**
  - **问题**：`cdeu_job_dynamic` 原唯一键为 `(usc, biz_month)`（EXPORT）/ `(file_id)`（UPLOAD），与运行次数无关。`EnsureDynamicExportJob` 在第二次运行时会 `UPDATE` 上一轮的记录：覆盖 `workflow_run_id`、重置状态为 `PENDING`——即「上一次跑的 JOB，下一次跑流程还关联上」，上一轮运行的历史被直接抹掉。这也是「按 run 筛选动态实例」一直无法正确实现（早前只能靠“不过滤 + `from_current_run` 标记”绕过）的根因。
  - **迁移 `V100045`**：
    - 删除 `uq_cdeu_job_dynamic_export` / `uq_cdeu_job_dynamic_upload`；
    - 新建 `uq_cdeu_job_dynamic_export_run (workflow_run_id, usc, biz_month)`、`uq_cdeu_job_dynamic_upload_run (workflow_run_id, file_id)`，均带 `NULLS NOT DISTINCT`（PostgreSQL 15+），保证手动单实例执行（`workflow_run_id = NULL`）时同样按唯一处理；
    - DDL 前快照 `dedup.cdeu_job_dynamic_bak_v100045` / `cdeu_job_dynamic_step_bak_v100045`；
    - **不删除任何存量数据**（507 条各带自己的 `workflow_run_id`，全部保留）。
  - **代码改造**：
    - `EnsureDynamicExportJob`：删掉「跨运行 UPDATE 复用」逻辑，改为 `INSERT ... ON CONFLICT (workflow_run_id, usc, biz_month) WHERE job_category='EXPORT' DO UPDATE`（同一运行内幂等重置，不同运行各自新记录）；
    - `GenerateDynamicJobs` / `GenerateUploadDynamicJobs`：`ON CONFLICT` 目标改为新唯一键，Step 生成限定当前运行；
    - `MarkDynamicJobStatus` / `MarkDynamicJobStepStatus` / `FailDynamicJobSteps` / `CompleteDynamicJobSteps`：新增 `dynamicJobSelectionSQL(runParam)` 统一按「优先当前运行、缺失回退最新一条」定位目标实例；
    - `MarkDynamicUploadJobStatus` / `MarkDynamicUploadJobStepStatus`：同样加运行维度；
    - `runSubsequentDynamicExportJobs`：续跑限定同一 `workflow_run_id`，不再跨运行续跑旧实例；
    - `cdeu_admin.go attachCDEURun`：**恢复严格按 `workflow_run_id` 过滤**动态实例（数据模型支持后，过滤才是正确语义）。
  - **保留策略**：新增 `Store.PurgeStaleDynamicJobRuns(keepDays, limit)`（默认保留 **30 天**，只删「所属运行已终态 且 实例自身已终态」的记录及其 Step），由 `CLEAN_EXPIRED_EXPORT_ARTIFACTS` 调用，`details` 新增 `stale_dynamic_deleted`。
  - **验证**：
    - `OK-1` 同一 `(usc, biz_month)` 在两次不同运行下各 1 条（跨运行独立）；
    - `OK-2` 同一运行内重复插入被唯一约束拒绝；
    - `OK-3` `workflow_run_id = NULL` 的重复插入同样被拒绝（`NULLS NOT DISTINCT` 生效）；
    - 迁移后存量 507 条动态实例未变（`run 49` 101 EXPORT + 202 UPLOAD，`run 51/52/53` 共 204 UPLOAD）；
    - `CDEU/Overview?runId=54` 只显示本运行的 15 个静态 JOB、无动态实例；`?runId=49` 显示 90（BASE，含 89 动态）+ 12（INCR）+ 202（UPLOAD），证明「每次运行只看自己的实例」已成立。

- **CDEU 运行实例表去除自增代理键（`V100043` / `V100044`）**
  - **背景**：`cdeu_process_run` / `cdeu_job_run` 原先各用 `bigserial` 主键（`process_run_id` / `job_run_id`），导致「按一次运行筛 JOB」必须 JOIN 三层（`job_run → process_run → phase_run → workflow_run_id`）、且 `cdeu_job_run.batch_key` 从未写入，752 行历史数据中有 589 行是孤儿或无法归属的批次重复。
  - **`V100043`（加列 + 回填）**：`cdeu_process_run` / `cdeu_job_run` 新增 `workflow_run_id`，从父链回填（`210/210`、`641/797`），并建索引 `idx_cdeu_process_run_workflow`、`idx_cdeu_job_run_workflow`。此后按运行筛选只需 `WHERE workflow_run_id = ?`。
  - **历史脏数据处理（已授权删除，备份保留）**：156 行孤儿（父 `cdeu_process_run` 已不存在）+ 433 行批次重复（`run_log_id` 关联的日志 `usc`/`biz_month` 全为空，无法还原证券月）→ 备份至 `dedup.cdeu_job_run_purge`(589) / `dedup.cdeu_job_step_run_purge`(433) 后删除；`cdeu_job_run` 797→208，`cdeu_job_step_run` 641→208。
  - **`V100044`（删代理键）**：
    - `cdeu_process_run` 主键改为 `(workflow_run_id, process_id)`，删除 `process_run_id`；
    - `cdeu_job_run` 主键改为 `(workflow_run_id, process_id, job_id)`，删除 `job_run_id`、`process_run_id`；
    - `cdeu_job_step_run` 保留自增 `step_run_id`，`job_run_id` 改为 `(workflow_run_id, process_id, job_id)` 三元组引用（唯一键同步改为 `(workflow_run_id, process_id, job_id, step_code)`）；
    - `cdeu_job_run` 新增 `dynamic_job_id`（可空，指向 `cdeu_job_dynamic`，非批次 JOB 为 NULL）；
    - `cdeu_workflow_run.run_id` 保持自增（唯一运行入口流水号）；
    - 清理 V100050 改名遗留的序列 `cdeu_step_run_step_run_id_seq` / `cdeu_job_run_job_run_id_seq`，并把 `cdeu_workflow_run` 上误占的 `cdeu_process_run_pkey` 改名为 `cdeu_workflow_run_pkey`；
    - DDL 前整表快照 `dedup.cdeu_{process,job,job_step}_run_bak_v100044`（不参与运行时）。
  - **代码重构**：新增 `cdeuRunKey{WorkflowRunID, ProcessID, JobID}` 取代所有 `int64` 运行 ID 传参；`CDEUPhaseProcessRunIDs` → `CDEUPhaseRunID`；`InsertCDEUJobRun` / `StartCDEUJobRun` / `FinishCDEUJobRun` / `InsertCDEUJobStepRuns` / `ResetCDEUJobStepRuns` / `UpsertReadOnlyCDEUJobRun` 全部改用运行键；`EvaluateCDEUProcess` 入参改为 `(runID, processID)`；`cdeu_store.go` / `cdeu.go` / `cdeu_admin.go` 共 55 处引用同步调整。
  - **API 变更**：`CDEU/Overview` 不再返回 `job_run_id` / `process_run_id`（`CDEUJobView.JobRunID`、`CDEUProcessView.ProcessRunID` 删除），新增 `dynamic_job_id`；前端 `DeDupCDEUView.vue` 的「运行编号」改为显示 `第 N 次`（依据 `attempt`）。
  - **验证**：迁移 `100043`/`100044` 均 `success=t`；三表行数不变（`210`/`208`/`208`）；手动 `CHECK_EXPORT_JOB_HEALTH` 后新 `cdeu_job_run` 15 行按新主键写入、`cdeu_process_run` 14 行、`cdeu_job_step_run` 正常；`CDEU/Overview?runId=54` 返回 200 且 11 个 Process 的 JOB 状态正常（`PROCESS_EXPORT_FILE_MON_BASE` 90 项含 89 个动态实例）。
  - **未解决（遗留议题）**：`cdeu_job_dynamic` 仍以 `UNIQUE (usc, biz_month)` 跨运行复用同一条记录并覆盖 `workflow_run_id`，即「上一次跑的动态 JOB 被下一次运行接管」，需单独决策是否改为每次运行一条。

- **新增系统配置 20007「导出成功后立即删除主表行情数据标志」，并取代原延迟清理机制**（迁移 `V100042`）：
  - `20007`（`data_type='2'`，`char_value`，默认 `1`）：
    - **`1` 立即删除（默认）**：文件**远端上传成功后**删除 `dedup.finv_quote_secu_kline_min` 中该 `(usc, biz_month)` 的行情数据；此后同一证券月表内再次插入历史数据时按增量导出，INCR 文件名中的序号 `_Incr_N` 的 N 递增（沿用现有 `patch_count` 机制）。
    - **`0` 保留数据**：不删主表；重复导出直接**重写同名 BASE 文件覆盖**，**不生成 INCR**。
  - **删除时机 = 远端上传成功后**（不是本地写盘成功）：`UPLOAD_PENDING_EXPORT_FILES` 在单个文件上传并校验通过后触发，上传失败分支不会删除，避免「远端失败 + 主表已删」。
  - **BASE 文件没有版本号**：文件名 `{region}_{market}_{usc}_Min_{provider}_Mon_{yyyyMM}.mvsv` 不含版本，重写即同名覆盖，不存在递增；递增的只有 INCR 的 `_Incr_N`。
  - **两种模式互斥**：`job_patch.go` 中 `20007=0` 强制走 `rewriteBase`（覆盖 BASE）；`20007=1` 保持原分级逻辑（BASE 未上传成功时重写、已上传后生成 INCR，因为主表可能已被删）。
  - **取代旧机制**：`CLEAN_EXPIRED_EXPORT_ARTIFACTS` 的「延迟按月清理主表」下线，环境变量 `FINV_QUOTE_DEDUB_PURGE_MAIN_ENABLED` / `_PURGE_MAIN_MONTHS` 废弃（已从 `config`、`docker-compose.yml`、`.env.example` 移除）。
  - **影响范围仅 dedup schema**：`quote.finv_quote_secu_kline_min` 是回测/查询用的独立副本，不参与该开关的删除动作。
  - 删除不做行级备份（需求方明确），但记录删除行数到运行进度与 upload job 的 `details.main_rows_purged`。

- **导出低峰窗口改由系统配置 `20006` 承载，删除为两个时间单独建的键值表**（迁移 `V100041`）：
  - 新增 `sys.sys_config` `20006`「导出低峰窗口」，`data_type='3'`，格式 **`HH:MM:SS|HH:MM:SS`**（精确到秒、半角竖线分隔、支持跨零点、`start==end` 视为全天），初始值 `00:30:00|06:30:00`。
  - **删除 `dedup.finv_dedup_runtime_setting`**（`V43` 建的 3 列键值表）：它仅为承载 `EXPORT_WINDOW_START`/`EXPORT_WINDOW_END` 两个时间而存在，与已有的 `sys.sys_config` 配置体系重复。删除前硬校验「表内仅含这两个键」——发现其他键即 `RAISE EXCEPTION` 中止；20006 以**规范初始值 `00:30:00|06:30:00`** 发放，不从旧表搬运现值——初始发放值与部署后实际运行值不一致属正常，实际窗口由运维通过接口自行配置。
  - **废弃环境变量** `FINV_QUOTE_DEDUB_WINDOW_START` / `_END`：窗口一律取 20006，缺失或非法时按内置默认 `00:30:00|06:30:00` 降级（不再回读环境变量）；`docker-compose.yml`、`.env.example` 已移除这两项。
  - 代码：`syscfg` 新增 `KeyExportWindow`/`ExportWindow`/`ParseExportWindow`/`ParseClockSeconds`/`FormatExportWindow`/`FormatClockSeconds` 与 `Store.WriteString`（写后自动失效缓存）；`dedup.Service` 的 `Window(ctx)`/`InWindow(ctx, at)` 改为读 20006、`SetWindow` 写 20006；删除 `LoadRuntimeSettings`、`Store.RuntimeSettings`、`Store.SaveRuntimeWindow`、`SettingWindowStart/End`；`inWindowAt` 与 `ParseClock` 统一改秒级；`PUT /Settings/Window` 改为**严格接受 `HH:MM:SS`**（拒绝 `HH:MM`）；`GET /Settings` 的 `source` 取值改为 `sysconfig` / `default`。
  - 前端：窗口对话框时间控件加 `step="1"`（精确到秒），来源文案改为「系统配置 20006 / 内置默认值」。
  - 验证：写入 `01:15:30|02:45:15` 落库与回读一致、`in_window` 立即变 false；`HH:MM` 请求被 400 拒绝；启动日志 `导出窗口 00:00:00-23:59:00，来源 sysconfig`。

- **新增系统配置 20005「CDEU工作流运行模式」，把「是否自动跑」从低峰窗口里独立出来**（迁移 `V100040`）：`sys.sys_config` 新增 `20005`（`data_type='3'` 字符串，`str_value` 默认 `Manual`）——`Manual`=手动模式（默认），进程内调度器不自动触发任何 CDEU Job，只有 `POST .../Job/Run` 手动触发才执行；`Auto`=自动模式，每天按定时任务设定的时间（`cdeu_job.schedule_value`）自动跑。
  - **背景**：原有实现中「调度触发」与「手动触发」共用同一条 Job 领取路径（`Store.ClaimJob` 只按 `status` + 到期时间领取，`CDEUTrigger*` 仅写入运行日志），所以只要处于低峰窗口且 Job 到期，调度器就一定会自动执行，运维无法只跑手动流程。现在窗口（`EXPORT_WINDOW_*`）控制「什么时段可以跑」，`20005` 控制「是否允许自动跑」，两者独立。
  - **实现**：`internal/meta/finv/sys`（需求 10762 归位前为 `internal/syscfg`）新增 `KeyCDEUMode`/`CDEUModeManual`/`CDEUModeAuto`/`NormalizeCDEUMode`/`Values.CDEUScheduleEnabled()`；`dedup.Service` 新增 `CDEUMode(ctx)` 与 `CDEUScheduleEnabled(ctx)`；`Scheduler.fire` 入口在窗口判断**之前**先判 `20005`，非 `Auto` 直接返回（手动 `Job/Run` 不经过该函数，不受限）；`GET .../DeDup/Status` 的 `options` 新增 `cdeu_mode`、`cdeu_schedule_enabled`、`cdeu_mode_error`。读配置失败/空值/未知取值一律按 `Manual` 兜底（安全侧）。
  - **验证**：部署后日志输出 `CDEU 运行模式 = Manual（默认）：调度器不自动触发任何 Job`；间隔类 Job（`UPLOAD_PENDING_EXPORT_FILES` 每 300 秒，部署前 02:32:49 刚跑过）在应到期的 02:37:56 未触发，部署后 6 分钟零新增运行日志，101 个导出 job 全部保持 `FROZEN`，本地导出目录 0 文件。

- **证券行情月归档导出与上传自动化链路**：新增 `internal/meta/finv/quote/dedup`，按「成熟度 → 候选 → LOAD（主表→`export` 中转表）→ EXPORT（→ MVsv 文件）→ CLEANUP（清理中转）→ 上传独享存储 → 收口补丁 → 生命周期清理」实现 10 个 Job（`CALC_DAILY_INGEST_STATS`、`CALC_DAILY_EXPORT_BUDGET`、`REFRESH_QUOTE_MATURITY`、`MARK_SEALED_JOBS_FROZEN`、`RUN_DUE_EXPORT_JOBS`、`UPLOAD_PENDING_EXPORT_FILES`、`SCAN_POST_SEAL_PATCHES`、`CLEAN_EXPIRED_EXPORT_ARTIFACTS`、`RECOVER_TIMEOUT_EXPORT_JOBS`、`CHECK_EXPORT_JOB_HEALTH`）；每日预算自适应（`factor = clamp(1.20 + 0.20*sla + 0.05*min(backlog,5), 1.20, 2.00)`）、keyset 分页、行锁 + 租约 + 心跳、暂停续跑、指数退避。
- CDEU 流程控制页默认展示「总体视图」，支持切换「阶段详情」；阶段节点悬浮展示状态、步骤/作业/批次计数、开始时间与耗时；新增「启动流程」按钮与确认对话框。
- 总体视图的「阶段概览」改为**所选阶段的步骤清单表格**：点击阶段流转节点切换阶段，表格展示步骤状态、作业完成数、批次进度、开始时间、耗时与异常；阶段汇总信息保留在悬浮提示中。
- 「阶段流转」改为**画布式工作流**：点阵画布上悬浮阶段节点，节点之间用带箭头的有向连线从前序阶段指向后续阶段；点击节点切换下方步骤清单。
- 新增迁移 **`V100045__finv_dedup_cdeu_split_ops_steps.sql`**：删除尾部 `PHASE_OPS`，将 `STEP_RECOVER` 并入导出阶段、`STEP_HEALTH` 并入上传阶段，作为并行旁路步骤（`affects_completion=false`）；流程画布由 5 个阶段调整为 4 个阶段。
- 「阶段概览」步骤表格支持点击选中步骤，选中行高亮并在表格下方展示该步骤定义的 JOB 清单；作业行支持直接触发（历史运行只读）。
- 运行中的阶段在画布节点上增加脉冲高亮、运行小圆点与指向该阶段的流光箭头；运行中的步骤在步骤表格行与详情头部增加背景呼吸动画与左侧高亮条。
- 「运行历史」由右侧卡片改为「最新运行」位置的下拉列表：默认展示最近一次运行，支持选择历史运行切换；启动流程后自动回到最新运行并展示当前执行数据。
- 执行中状态图标统一改为旋转的 `mdi-loading`（流程 KPI、阶段画布、步骤表格、阶段详情、JOB 清单、历史运行列表），并支持 `prefers-reduced-motion`。
- 状态图例移入「阶段流转」画布卡片底部，通过卡片右上角「图例」开关显示/隐藏；移除右侧独立状态图例卡片。
- 移除独立「流程规则」卡片：完成条件与失败策略并入页头摘要，业务时区、查询时间及内嵌阶段说明收进信息悬浮提示；阶段概览与 JOB 清单改为全宽布局。
- 阶段流转画布增加「开始」「结束」首尾节点，并用有向箭头与首个/最后一个阶段连接；开始节点显示已开始/未开始，结束节点显示已完成/未完成/等待结束。
- 新增迁移 **`V100046__finv_dedup_cdeu_history_ingest_step.sql`**：数据采集阶段增加非关键「历史行情数据采集」占位步骤 `STEP_HISTORY_INGEST`，无 JOB；CDEU 运行初始化时自动将无 JOB 的步骤置为 SUCCESS。
- 阶段步骤表拆分「步骤名称 / 步骤 KEY」两列，JOB 表拆分「作业名称 / 调度标识」两列；四列增加拖拽列宽手柄，表格按最小宽度自动横向滚动。
- 新增迁移 **`V100047__finv_dedup_cdeu_decision_phase.sql`**：去重收口与导出之间增加 `PHASE_DECISION`「导出判定」阶段及 `STEP_EXPORT_DECISION` 占位步骤，当前无 JOB；空步骤/空阶段初始化自动成功。
- 重构 JOB 清单列：作业名称、作业编码、调度标识、触发方式、执行次数、异常分别独立成列；作业名称/编码/调度标识支持拖拽列宽。
- JOB 清单「执行阶段」列文字颜色调整为暖色 `rgb(251, 140, 0)`，阶段详情中的执行阶段标签同步。
- 新增迁移 **`V100048__finv_dedup_cdeu_move_stats_to_decision.sql`**：将 `STEP_INGEST_STATS`、`STEP_BUDGET` 从数据采集阶段移入 `PHASE_DECISION` 导出判定阶段（seq 1/2），JOB 随步骤迁移。
- 新增迁移 **`V100049__finv_dedup_cdeu_move_maturity_to_decision.sql`**：将 `STEP_MATURITY` 从去重收口阶段移入 `PHASE_DECISION` 导出判定阶段（seq 3），JOB 随步骤迁移；去重收口仅保留收口冻结。
- **CDEU 工作流模型重构（V50）**：层级由 `Process → Phase → Step → JOB` 调整为 `Workflow → Phase → Process → JOB → Step`；表名/字段、`GET /CDEU/Overview` 响应、启动接口与前端表述同步调整，新增 `cdeu_job_step` / `cdeu_job_step_run` 并为现有 JOB seed 默认 Step；启动主路径改为 `POST /CDEU/Workflow/Run`，旧 `/CDEU/Process/Run` 保留兼容。
- 新增迁移 **`V100051__finv_dedup_cdeu_rename_seal_process.sql`**：将 `PROCESS_SEAL` / `JOB_MARK_SEALED` / `STEP_MARK_SEALED` 展示名称统一为「收口冻结与差异报告生成」，并同步运行快照名称。
- 新增迁移 **`V100052__finv_dedup_cdeu_rename_decision_phase.sql`**：将 `PHASE_DECISION` 阶段名称由「导出判定」调整为「导出决策」。
- 新增迁移 **`V100053__finv_dedup_cdeu_phase_rename_review.sql`**：将 `PHASE_COLLECT` / `PHASE_DEDUP` 命名为「历史行情采集」/「历史行情去重」，并在上传后新增 `PHASE_REVIEW`「归档复核」阶段（占位 Process，初始化自动成功）。
- 「阶段流转」标题改为「CDEU工作流」；阶段间距由 24px 扩大为 48px，并缩小阶段间箭头，避免箭头过大、线段过短。
- 画布「开始」「结束」节点改为 icon 在左、文字在右；标题与状态上下两行展示，终端节点宽度由 72px 调整为 96px。
- 新增迁移 **`V100054__finv_dedup_cdeu_rename_export_upload_phases.sql`**：将 `PHASE_EXPORT` / `PHASE_UPLOAD` 名称调整为「月度行情导出」/「月度行情上传」。
- 前端「阶段概览」标题改为「流程概览」；JOB 清单标题改为「JOB 清单 · `<Process 名称>`」。
- Process 表格新增独立「标识」列展示「关键」/「旁路」，不再挤在 Process 名称单元格；移除流程概览右侧的操作提示文字。
- 工作流页头元信息改为带图标的彩色 chip 可视化展示：工作流名称、运行模式、每日调度、完成条件、失败策略。
- 新增迁移 **`V100055__finv_dedup_cdeu_rename_export_process.sql`**：将 `PROCESS_EXPORT_JOB` 名称由「单证券月导出」调整为「证券历史月度行情文件导出」。
- 新增迁移 **`V100056__finv_dedup_cdeu_rename_export_file_processes.sql`**：`PROCESS_EXPORT_JOB` 编码改为 `PROCESS_EXPORT_FILE_MON`；`PROCESS_PATCH` 编码改为 `PROCESS_EXPORT_FILE_MON_INCR`，名称改为「证券历史月度行情增量文件导出」。
- 新增迁移 **`V100057__finv_dedup_cdeu_add_security_metadata_job.sql`**：在 `PROCESS_EXPORT_FILE_MON` 下新增 `JOB_FETCH_SECURITY_METADATA`「证券元数据获取」JOB（META 阶段），导出前从证券字典刷新 `finv_dedup_usc` 缺失元数据。
- 新增迁移 **`V100058__finv_dedup_cdeu_export_batch_process.sql`**：新增 `PROCESS_EXPORT_BATCH`「证券导出批次清单获取」及 `JOB_FETCH_EXPORT_BATCH`，生成 `usc + biz_month + BASE/INCR` 批次清单表 `dedup.finv_dedup_export_batch`；BASE/INCR 导出流程分别消费对应条目。
- 新增迁移 **`V100059__finv_dedup_cdeu_rename_base_export_process.sql`**：将 BASE 导出 Process 编码改为 `PROCESS_EXPORT_FILE_MON_BASE`；同时修复历史已完成运行新增 Process 后 CDEU 只读登记找不到运行树的问题。
- 新增迁移 **`V100060__finv_dedup_cdeu_rename_collect_phase.sql`**：将 `PHASE_COLLECT` 名称由「历史行情采集」调整为「历史行情收集」。
- 新增迁移 **`V100061__finv_dedup_cdeu_dynamic_export_jobs.sql`**：按批次清单动态生成单证券导出 JOB（`(usc, biz_month)` 全局唯一）；固定 LOAD/EXPORT/REGISTER/CLEANUP 与补丁扫描 JOB 转为内部执行壳并在前端隐藏，页面展示动态 JOB 的证券、年月、BASE/INCR 类型及状态。
- 新增迁移 **`V100062__finv_dedup_cdeu_dynamic_job_steps.sql`**：为每个动态单证券导出 JOB 生成 `LOAD → EXPORT → REGISTER → CLEANUP` 四个串行 Step，运行时同步 Step 状态并展示在前端。
- 新增迁移 **`V100063__finv_dedup_cdeu_rename_job_dynamic_and_upload.sql`**：`cdeu_dynamic_job` / `cdeu_dynamic_job_step` 改名为 `cdeu_job_dynamic` / `cdeu_job_dynamic_step`；`PROCESS_UPLOAD` 按已导出未完成上传的文件动态生成上传 JOB（按 `file_id` 唯一），并同步 `STEP_UPLOAD`。
- 点击阶段后，在工作流图下方新增该阶段 Process 拓扑顺序图：按 `seq_no` 串联，起点连接上一阶段输出箭头，终点连接下一阶段输入箭头；Process 节点可点击选中。
- 导出决策阶段依赖调整为：入口流量统计与行情成熟度扫描并行，每日导出预算计算在其后；调度调整为 00:05 启动统计/成熟度、00:10 启动预算，拓扑图同步展示并行汇合关系。
- 画布阶段节点与 Process 拓扑节点采用不同视觉样式（阶段实线大卡片、Process 虚线紧凑卡片）；「开始 / 结束」节点由圆形改为矩形。
- 新增「全流程拓扑」菜单与页面 `/Meta/Finv/Quote/DeDup/CDEUWorkflow`：阶段以虚线框圈定，框内展开全部 Process，并绘制阶段内与跨阶段的依赖连接。
- 「全流程拓扑」图例移入拓扑画布内部，随画布滚动。
- 「全流程拓扑」阶段内部并行分叉点收进虚线框内；所有阶段虚线框统一高度，宽度按流程列数自适应。
- 「全流程拓扑」移除阶段虚线框左侧绿色实边；加大分叉点到 Process 入口间距，避免箭头与入口重叠。
- 修正阶段 2 → 阶段 3 入口连线为完整 Z 字折线，解决箭头与竖向线重叠问题。
- 「全流程拓扑」增加运行状态实时同步：执行中每 2 秒刷新、空闲每 10 秒刷新，页面恢复可见时立即刷新，并高亮运行中阶段/流程。
- 「全流程拓扑」移出侧边栏菜单，改为「工作流控制与监控」页头链接，新窗口打开。
- 「全流程拓扑」下方新增竖版全流程拓扑对比卡片，阶段纵向堆叠、Process 纵向执行，并复用实时状态。
- 竖版全流程拓扑的阶段标题改为左侧贴边标题栏，流程节点在其右侧自上而下排列。
- 竖版全流程拓扑所有阶段虚线框宽度统一，流程较少的阶段内容在框内水平居中。
- 竖版全流程拓扑加大阶段 3/4 的入口分叉空间和并行分支行距，避免折线箭头挤压。
- 全流程拓扑 Process 节点编号改用 `process_id`，横版与竖版同步。
- 竖版全流程拓扑阶段 3 入口分叉的两条水平折线段上移 10px。
- 全流程拓扑 Process 节点宽度按标题/编码/JOB 文案自适应，标题不再省略。
- 竖版全流程拓扑将阶段进度改为左侧标题区与流程区之间的竖直进度线。
- 新增迁移 **`V100065__finv_dedup_cdeu_dynamic_job_name.sql`**：导出动态 JOB 名称统一为 `EXPORT_MON_{TYPE}_{USC}_{YYYYMM}` 并回填已有记录。
- 新增迁移 **`V100066__finv_dedup_job_run_log_line.sql`** 与 `GET .../RunLog/Detail`；CDEU JOB 行新增「执行日志」弹窗，展示逐行阶段、进度与错误日志，运行中自动刷新。
- 低峰窗口支持 `00:00:00` ~ `23:59:59` 全天候测试：兼容秒输入、按分钟判定并包含 `23:59` 分钟。
- CDEU JOB 清单增加横向滚动容器，列宽超出时可查看右侧列。
- 新增迁移 **`V100067__finv_dedup_cdeu_reseed_jobs.sql`**：人工清理 CDEU JOB 定义后幂等重建 JOB / Job Step，修复无法启动工作流。
- 新增迁移 **`V100068__finv_dedup_cdeu_upload_dynamic_name.sql`**：上传动态 JOB 编码/名称统一为 `JOB_UPLOAD_{TYPE}_V{VERSION}_{USC}_{YYYYMM}`，调度标识改为 `DYNAMIC_UPLOAD`。
- 新增迁移 **`V100069__finv_dedup_cdeu_upload_dynamic_display_name.sql`**：上传动态 JOB 名称改为 `UPLOAD_{TYPE}_V{VERSION}_{USC}_{YYYYMM}`，与 `JOB_UPLOAD_*` 编码分离。
- 新增迁移 **`V100070__finv_dedup_cdeu_dynamic_job_run_link.sql`**：动态 JOB 关联 `run_log_id` / `workflow_run_id`，支持查看执行日志且只展示当前工作流本轮生成的动态 JOB。
- 补丁扫描每个证券月动态 JOB 单独登记运行日志，避免多个补丁 JOB 共用父任务日志。
- CDEU JOB 清单的 Step 列改为只展示当前执行到的 Step，并显示当前序号/总 Step 数。
- 修复导出/上传目录与 Docker 持久化挂载不一致：默认统一到 `Data/ExportArchive` 与 `Data/ExportArchive/Upload`。
- 无增量数据的 INCR 补丁动态 JOB 标记为 `SKIPPED`，执行日志说明未生成补丁文件。
- INCR 动态 JOB 改为确认 `delta > 0` 后创建，零增量不再生成 `JOB_EXPORT_INCR_*` 任务。
- 导出目录调整为 `/data/App/FinvQuant/Export`（上传目录 `/data/App/FinvQuant/Export/Upload`），Docker 宿主机映射为 `D:/Data/App/FinvQuant/Export`。
- Docker 宿主机导出目录调整为 `D:/Dev/Docker/HostFileSystem/FinvQuant/Export`，容器路径保持 `/data/App/FinvQuant/Export`。
- CDEU Process/Phase/Workflow 完成判定纳入动态 JOB；存在待执行动态 JOB 时保持运行中，动态状态变化后重新评估工作流。
- 修复动态 JOB Step 回填；动态导出 JOB 增加「手动执行」按钮与 `POST /Job/Dynamic/Run` 单任务执行接口。
- JOB 执行日志增加 Step 开始/结束、JOB/Step 编码与名称、MVsv 绝对路径、中转表名。
- 归档 JOB 定义拆分到 `internal/meta/finv/quote/dedup/job/`，每个 JOB 一个文件，动态 BASE/INCR/上传复用父 JOB 定义。
- 归档 JOB 业务执行逻辑迁移到 `dedup/job` 子包，通过执行器注册表接入调度器；动态 BASE/INCR/上传复用对应父 JOB 执行文件。
- CDEU 手动执行 JOB 增加确认框：默认仅执行当前 JOB，可勾选「自动续跑同流程下的后续 JOB」，勾选后按工作流定义顺序续跑。
- 手动触发静态/动态 JOB 时接口立即返回 `run_log_id`，前端可直接打开执行日志弹窗；动态导出 JOB 改为异步执行。
- JOB 执行日志弹窗支持全屏 / 退出全屏切换，便于查看大量日志。
- 重置 GCMain 201811–201910 导出状态并重新导出 12 个 BASE 文件到新的持久化目录。
- 阶段内 Process 拓扑增加「流程拓扑」显示/隐藏开关（默认显示）；导出阶段把 `PROCESS_RECOVER` 与 `PROCESS_EXPORT_FILE_MON_BASE` 并行展示，分别汇入下一阶段。
- 新增 `POST /API/V1/Meta/Finv/Quote/DeDup/CDEU/Process/Run`：无活动流程时创建 `MANUAL` 运行并在后台按阶段顺序执行全部直接作业；已有运行中的流程返回 409。
- 新增 CDEU 流程控制与监控页面（菜单「数据采集 → 行情数据归档 → 流程控制与监控」，路由 `/Meta/Finv/Quote/DeDup/CDEU`）：按 Process → Phase → Step → Job 展示当前/历史运行状态、阶段与作业进度、作业耗时与处理量、运行历史；作业行支持复用 `POST .../Job/Run` 手动触发。后端新增 `GET /API/V1/Meta/Finv/Quote/DeDup/CDEU/Overview` 聚合定义树、运行树与历史运行。
- 新增迁移 **`V100044__finv_dedup_cdeu_process_model.sql`**：在 `dedup` schema 新增 8 张 `cdeu_*` 表（流程/阶段/步骤/作业定义 + 流程/阶段/步骤/作业运行），全部字段带中文注释、无外键；seed `CDEU_DAILY` 流程、5 个阶段、10 个步骤、13 个作业（含 `RUN_DUE_EXPORT_JOBS` 的 LOAD / EXPORT / REGISTER / CLEANUP 四个可手动重跑内嵌阶段）。
- **CDEU 结构化运行层**：`Service.RunWithTrigger` 执行 Job 时双写 `cdeu_job_run`，按 `ALL` 完成条件逐级评估 Step / Phase / Process；同一天多个 Job 复用同一 `cdeu_process_run`，流程终态后的手动补跑只登记只读作业运行，不重开已完成流程；调度器启动时读取 `cdeu_job.schedule_kind/value` 覆盖默认调度时刻（导出 Job 仍使用窗口内固定间隔）。
- 新增 CDEU 完成条件单元测试（ALL / CRITICAL_ALL / AT_LEAST / ANY_OF / PARTIAL / 非参与项）；集成测试 `TestServiceIntegration` 已验证 13 个作业全部登记到 CDEU 运行树。
- 新增迁移 **`V100041__finv_dedup_export_archive_tables.sql`**：在 `dedup` schema 新建 12 张 `finv_dedup_*` 表（入口统计、日预算、扫描水位、优先级、成熟度、交易日历、运行日志、证券主数据、导出任务、导出文件、上传审计、补丁任务）与 12 个规范命名索引，全库无外键，末尾做收敛校验；**不修改** `dedup.finv_quote_secu_kline_min` 与 `export.finv_quote_secu_kline_min`。
- 新增导出格式实现：MVsv「16 行文件头 + 1 行空行 + `ts|da|ti|o|h|l|c|v|t|cp|cr|lc` 数据行」、临时文件 + SHA256 + 原子 `rename`、结构与 `Count` 校验、可选 gzip；BASE / PATCH / MANIFEST 命名与 Manifest 字段按方案稿落地。
- 新增可插拔上传适配器（`Uploader`：`Stat` / `Put` / `Verify`）：内置 `none`（不启用）、`dir`（目录 / NAS 挂载）、`http`（HTTP PUT 网关）三种实现；上传前本地校验、远端查重、上传后远端 checksum 校验，每次尝试写 `finv_dedup_export_file_upload_log`。
- 新增 6 个运维接口（`/API/V1/Meta/Finv/Quote/DeDup/*`）：`Status`、`Job/List`、`File/List`、`Job/Retry`、`File/Retry`、`Job/Run`，均为一接口一文档（`Docs/API/QuoteDeDup/*.md`），并在 `Docs/API/README.md` 与 `Docs/API/APIs.md` 登记。
- 新增配置项（均在 `config.Config` 中集中解析）：`FINV_QUOTE_DEDUB_ENABLED`、`_TIMEZONE`、`_OUTPUT_DIR`、`_WORKER_ID`、`_PAGE_SIZE`、`_WINDOW_START`、`_WINDOW_END`、`_LEASE_SECONDS`、`_MAX_ATTEMPTS`、`_UPLOAD_MODE`、`_UPLOAD_DIR`、`_UPLOAD_HTTP_BASE`、`_UPLOAD_HTTP_TOKEN`、`_UPLOAD_PREFIX`、`_LOCAL_RETENTION_DAYS`、`_PURGE_MAIN_ENABLED`、`_PURGE_MAIN_MONTHS`、`_MATURITY_THRESHOLD`、`_PATCH_BUDGET_RATIO`、`_COMPRESSION`、`_FETCH_LIMIT`、`_MAX_JOBS_PER_RUN`。
- 新增落地文档 [`Docs/Refactor/2610-005-FinvQuoteDeDubExportUpload.md`](Refactor/2610-005-FinvQuoteDeDubExportUpload.md)（流程、配置、调度、运维要点、与方案稿的差异说明、P0–P7 阶段对照）。
- 新增设计文档 [`Docs/Refactor/2610-007-CDEUProcessModelDesign.md`](Refactor/2610-007-CDEUProcessModelDesign.md)：定义 CDEU（采集/去重/导出/上传）的 Process → Phase → Step → Job 四层抽象、每日定时/手动运行模式、阶段/步骤/作业顺序与批处理、完成条件（全部/关键/至少 N 个）、8 张登记表结构及 13 个作业执行单元的结构化登记映射。
- **前端月归档导出控制台**（`Web/src/views/Meta/Finv/Quote/DeDup/Export/`）：新增 3 个页面，挂到「数据采集 → 行情数据去重 → 月归档导出」菜单：
  - **归档监控总览**（`/Meta/Finv/Quote/DeDup/Export/Overview`）：指标卡（待装载行数、积压天数、预算因子与消耗、FROZEN 缺失、近 7 天补丁、心跳超时、上传重试耗尽、待清理文件、MVsv 异常）、预算使用率与完整性进度条、任务/上传状态分布、告警列表（`alerts`）、运行参数摘要、10 个 Job 调度表与「立即运行」，支持 30 秒自动刷新；
  - **月归档导出任务**（`.../Export/Job`）：`finv_dedup_export_job` 分页表（状态、已装载行数、BASE 版本、补丁数、收口/冻结日、执行者、心跳、下次运行、最近错误），支持状态 / 证券 / 业务月过滤与人工重试；
  - **月归档导出文件与上传**（`.../Export/File`）：`finv_dedup_export_file` 分页表（类型、版本、行数、大小、上传状态、尝试次数、上传起止、远端校验、网盘归档、清理状态、下次重试），支持上传状态 / 文件类型过滤、「仅看已上传」快捷筛选与重试上传。
  - 复用 `Web/src/api.ts`（`apiGet` / `apiPost`）与 `Web/src/utils.ts`（新增 `fmtISO` / `fmtBytes` / `fmtMonth`），仅调用已有 6 个后端接口，无后端改动。
- **归档运维台（第二批：全表查询 + 管理员编辑）**：补齐 12 张 `finv_dedup_*` 表的运维能力：
  - 新增 10 个查询接口：`Stats/Page`、`Budget/Page`、`Maturity/Page`、`Watermark/List`、`Priority/List`、`Calendar/Page`、`Usc/Page`、`RunLog/Page`、`UploadLog/Page`、`Patch/Page`；
  - 新增 11 个管理员写接口：`PUT Usc`、`PUT|DELETE Priority`、`PUT|DELETE Calendar`、`PUT Budget/Override`、`PUT Watermark`（重置）、`POST Patch/Retry`、`POST Job/Pause`、`POST Job/Cancel`、`POST Maturity/Refresh`；
  - 可编辑白名单 5 张表：`finv_dedup_usc`（含决定能否导出的 `day_expect_rows` 校准值）、`finv_dedup_usc_priority`、`finv_dedup_trade_calendar`、`finv_dedup_daily_budget`、`finv_dedup_scan_watermark`；其余 7 张表只读；
  - 所有写操作追加审计（`finv_dedup_job_run_log_export_file`，`job_kind=MANUAL_ADMIN_EDIT`）；
  - 前端新增 3 个页面：**归档统计与预算**（`.../Export/Stats`）、**归档主数据与规则**（`.../Export/Config`）、**归档日志与补丁**（`.../Export/Log`）；`Web/src/api.ts` 新增 `apiPut` / `apiDelete`；
  - `Docs/API/QuoteDeDup/` 新增 11 篇文档（Usc / Priority / Calendar / Maturity / Stats / Budget / Watermark / RunLog / UploadLog / Patch / JobControl），`Docs/API/APIs.md` 登记 58–84 号接口并回填前端页面。
- **归档 Job 执行可视化与全链路流程图**：
  - 新增接口 `GET /API/V1/Meta/Finv/Quote/DeDup/Job/Runtime`（85 号）：合并进程内运行状态与运行日志，返回 10 个 Job 是否在跑、当前阶段 / 进度 / 心跳、最近一次结果、最近成功 / 失败时间与 24 小时成功 / 失败 / 暂停统计，文档见 `Docs/API/QuoteDeDup/JobRuntime.md`；
  - 新增迁移 **`V100042__finv_dedup_job_run_log_progress.sql`**：为 `dedup.finv_dedup_job_run_log_export_file` 增加 `stage` / `progress_percent` / `progress_detail` / `gmt_heartbeat` 四列，并建「未结束运行记录」部分索引；
  - 10 个 Job 上报阶段与心跳：导出任务按 `LOAD → EXPORT → CLEANUP` 上报（写 MVsv 文件时每 2 万行刷新心跳），上传 / 补丁类按文件 / 证券月上报，维护与统计类上报 `CLEANUP` / `RECOVER` / `HEALTH` / `STATS` / `BUDGET` / `MATURITY` / `SEAL`；心跳超过「租约 2 倍且不少于 2 分钟」未更新即标记为「疑似卡住」；
  - 10 个 Job 均提供中文名称与任务说明（功能 / 流程 / 工作细节 / 注意事项），由 `/Status` 与 `/Job/Runtime` 的 `jobs[]` 返回，前端在 Job 标识处悬浮展示，不新增表格列；
  - 前端新增独立菜单 **「端到端链路」**（`/Meta/Finv/Quote/DeDup/Export/Chain`，`Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue`）：以流程图展示「去重主表 → 成熟度/预算 → 收口冻结 → 装载 LOAD → 导出 EXPORT → 中转清理 CLEANUP → 文件上传 → 网盘归档与清理 → 补丁收口 → 保障任务」全链路节点状态与进度（执行中 / 心跳超时 / 需要关注 / 正常 / 未运行，含当前阶段、已运行时长、心跳与业务数字）；运行快照 5 秒、业务指标 20 秒自动刷新；支持**总体视角**（SVG 数据流向图：①–⑪ 节点、箭头连线与节点内数据流文案，支持点击联动详情，步骤状态与卡点定位）与**任务视角**（10 个 Job 的说明、状态、最近运行与所在链路位置，可跳转总体视角），节点内的 Job 中文名称支持悬浮查看功能 / 流程 / 工作细节 / 注意事项；旁路任务均连接到下游步骤（补丁 → 上传、超时回收 → 重新排队、健康检查 → 调度反馈）；导出调度 / 上传 / 超时回收 / 健康检查等定时循环任务以节点自环箭头 + 调度周期标识；连接线统一改为正交折线（实线主流程 / 虚线旁路 / 紫色定时循环）以减少重叠；紫色自环统一从节点底部中点（出口）绕外侧回到顶部中点（入口）；旁路任务使用 ⑫ / ⑬ / ⑭ 独立序号；⑫ 纳入「导出任务批次（本月）」范围，补丁数据先进入 ⑤ 中转表，再走 ⑥→⑦→⑧→⑨（与正常导出同一套 LOAD→EXPORT→CLEANUP），大数据量补丁不再按补丁小预算截断；批次进度合并 ⑫ 补丁任务计数（成功 / 失败 / 运行中 / 待处理）；⑤–⑧ 包裹为「导出任务批次（本月）」大节点，展示成功 / 失败 / 总数与分段进度条（成功绿 / 失败红 / 进行中蓝），子步骤各自展示装载 / 导出 / 登记完成度；节点第三行改为关键指标，表名与完整数据流仅在悬浮或右侧详情展示；某一步骤存在失败 / 心跳超时 / 业务阈值告警时，在总体视角以红黄边框与「链路卡点」列表直接定位到具体环节。
- **低峰窗口支持在线修改（保存后立即生效）**：
  - 新增迁移 **`V100043__finv_dedup_runtime_setting.sql`**：创建 `dedup.finv_dedup_runtime_setting` 键值表，保存 `EXPORT_WINDOW_START` / `EXPORT_WINDOW_END`；
  - 服务启动时加载数据库设置，未配置时回退环境变量 `FINV_QUOTE_DEDUB_WINDOW_START` / `_END`；调度器与导出 Job 使用内存中的当前生效值，在线保存后原子更新，无需重启；
  - 新增接口 `GET /Settings`（当前窗口、来源、更新时间、是否在窗口内）与 `PUT /Settings/Window`（校验 HH:MM、支持跨零点、写审计），文档见 `Docs/API/QuoteDeDup/Settings.md`；
  - 「端到端链路」页新增低峰窗口编辑对话框（开始 / 结束时间、来源与更新时间展示），保存后立即生效。
- **新增 `sys` schema（系统域）**（`V71`）：幂等建 schema、以 `current_user` 设定属主并授予 `USAGE`/`CREATE`、追加到 `search_path` 末尾（`public,quote,quant,futu,export,sche,dedup,archive,sys`，不改变既有解析优先级）、末尾收敛校验；应用连接串（`internal/database/postgres.go`）与 `DbSchemaSpec.md` 同步为九段。
  - 定位：`sys` 只承载用户/角色/权限、系统参数、审计留痕等平台级对象，不放行情/量化/映射业务表；表结构由后续迁移创建，本迁移**不建表**。
  - 约束：`sys` 对象读写必须写 `sys.` 前缀；`search_path` 仅作为历史种子迁移与手工 `psql` 的兜底。
- **新增 `sys.sys_config`（系统配置表）**（`V72`）：`integer` 主键 `config_no`，`data_type` 区分整型/字符/字符串/JSON 四类值列（`int_value` / `char_value` / `str_value` / `json_value`），带 `access_level`、`create_by`/`modify_by` 与 `gmt_create`/`gmt_modify`（`timestamp(6)`）；仿 MySQL `ON UPDATE CURRENT_TIMESTAMP(6)` 用 BEFORE UPDATE 触发器 `trg_sys_config_set_gmt_modify` 维护 `gmt_modify`；表 + 13 列中文注释齐备，迁移幂等并在末尾校验表/列数/主键/触发器/函数/注释。
- **新增 `sys.sys_dict` / `sys.sys_dict_sub`（数据字典主表与子表）**（`V73`）：`sys_dict`（主键 `dict_no`，16 列，含 `key_type`/`val_type`、`access_level`/`manage_level`/`data_level`、`ref_spec`、`enum_class`、`dict_category`）；`sys_dict_sub`（主键 `(dict_no, sub_key)`，16 列，含 `flag_enable` 启用标志与 `create_by`/`modify_by`、`gmt_create`/`gmt_modify`）；两表均 `timestamp(6)` 时间列、表 + 16 列中文注释齐备、无物理外键，各带一个 `BEFORE UPDATE` 触发器（`trg_sys_dict_touch` / `trg_sys_dict_sub_touch`），更新时**只刷新 `gmt_modify`**，`gmt_create` 仅插入时生成、之后保持不变（与 `V72` 口径一致）；迁移幂等，末尾校验表/列数/主键/触发器/函数/注释，并断言触发器函数体不含 `gmt_create` 赋值。
- **新增 `sys.sys_user`（系统用户表）**（`V74`）：24 列，主键 `idn`（`bigint GENERATED BY DEFAULT AS IDENTITY`，初值对齐原库 `AUTO_INCREMENT=1028`，空表时 `RESTART WITH 1028`、已有数据时接续 `max(idn)+1`）、`email` / `mobile_phone_no` 各自唯一、`flag_delete` 逻辑删除、`authority_str varchar(6000)`、`language` / `time_zone` 默认 `zh-CN` / `UTC+08:00`；表 + 24 列中文注释齐备；`BEFORE UPDATE` 触发器 `trg_sys_user_set_gmt_modify` 只刷新 `gmt_modify`（`gmt_create` 仅插入时生成）；无物理外键，迁移幂等并在末尾校验表/列数/主键/唯一约束/identity/触发器/函数/注释。
- **新增 `sys.sys_config` 初始配置数据（5 条）**（`V100025`，种子段）：`1010`「系统前后端分离标志」、`1020`「默认最大单页展示记录数」（`data_type='1'`、`int_value=2000`）、`1031`「1001配置」、`5001`「后端自定义分页插件启用标志」、`5002`「后端MyBatis分页缓存启用标志」（后四条 `data_type='2'`、`char_value='1'`）；`gmt_create` / `gmt_modify` 保留原库时间；`INSERT ... ON CONFLICT (config_no) DO NOTHING` 幂等——重跑不报错也不覆盖生产已调整的配置值；末尾校验 5 条齐备且 `data_type` 与取值列匹配。
- **新增 `sys.sys_dict` 初始字典定义（19 条）**（`V100026`，种子段）：`1001` 普通标志、`1002` Meta业务字段类型、`1003` 存取级别、`1004` 分页模式、`1005` 字段释义键序列、`1006` 数据类型（字典？）、`1007` 字典分类、`1008` 管理级别、`1009` 数据类型、`1301` 系统类型、`3004` 邮件类型、`3101` 民族、`3103` 国籍/地区字典、`3104` 国籍/地区字母代码对应电话区号字典、`3105` 国籍、`3107` 证件类别、`3110` 国标性别代码、`3121` 外汇交易币种、`3122` 注册资本币种；`gmt_create` / `gmt_modify` 保留原库时间；`ON CONFLICT (dict_no) DO NOTHING` 幂等（不覆盖生产已调整定义），末尾校验 19 条齐备且关键字段（`key_type`、`val_type`、`enum_class`、`dict_category`、`3105.manage_level` 为 `NULL`）匹配；字典子项数据待后续种子迁移。
- **新增 `sys.sys_dict_sub` 初始字典子项（89 条）**（`V100027`，种子段）：`3120` 币种（数字/字母键）14 条（`0`-`9`、`A`-`D`）、`3121` 外汇交易币种 11 条、`3122` 注册资本币种 64 条（含 `OTH` 其它，其 `en_sys_str` 为 `NULL`）；`sub_val` / `name_en` / `en_sys_str` 与时间戳原样保留（`3120` 的 `en_sys_str` 存在 `gat,gax,…` 与 `gat,gcq,…` 两种顺序）；`ON CONFLICT (dict_no, sub_key) DO NOTHING` 幂等，末尾校验三组条数与关键字段；`3120` 主表记录尚未提供，需补齐后与 `sys_dict` 建立对应。
- **新增 `sys.sys_dict_sub` 地区/国籍子项（387 条）**（`V100028`，种子段）：`3103` 国籍/地区字典 249 条（`ABW` 阿鲁巴 … `ZWE` 津巴布韦，ISO 三字母键；多数 `en_sys_str='trade'`，特例为 `NULL` 或 `cic,gft,gaa,act` / `act,cic`，`access_level` 为 1/2）、`3104` 国籍/地区字母代码对应电话区号 5 条（`CHN`=86 / `CTN`=886 / `HKG`=852 / `MAC`=853 / `USA`=001）、`3105` 国籍 133 条（ISO 数字码 `004` 阿富汗 … `470` 马耳他）；`ON CONFLICT (dict_no, sub_key) DO NOTHING` 幂等，末尾按「不少于」校验条数并抽查例外行；⚠ `3105` 在 `470` 处数据截断，后续编号（484 起）待补增量种子。
- **新增 `sys.sys_dict_sub` 通用标志/分页/数据类型子项（28 条）**（`V100029`，种子段，原 MySQL `cms_sys_dict_sub` 转写）：`1001` 普通标志 2 条（`0` 否 / `1` 是）、`1002` Meta业务字段类型 2 条（原库两值均为「否」）、`1003` 存取级别 3 条（可修改/只读/隐藏）、`1004` 分页模式 2 条（带 `sub_sort` 与 `description`，`en_sys_str` 为空串）、`1005` 字段释义键序列 14 条（`0`-`9`/`A`-`D`，`sub_sort` 1-14）、`1006` 数据类型 5 条（字符/整数/浮点数/布尔/字符串，`flag_enable='0'`）；`ON CONFLICT (dict_no, sub_key) DO NOTHING` 幂等，末尾校验六组条数与关键值（`1005` 的 `sub_sort` 比较改为数值比较，避免文本序误判）。
- **`sys.sys_dict_sub` 国籍（`3105`）补齐至完整 235 条**（`V100030`，种子段增量）：新增 `474` 马提尼克 … `894` 赞比亚共 102 条，与 `V100028` 的 `004`–`470` 衔接，使 `3105` 覆盖完整 ISO 3166-1 numeric 列表；同批提供的 `3103`（`MAC`–`ZWE`）与 `3104`（5 条）与 `V100028` 重复，不重复插入；`ON CONFLICT (dict_no, sub_key) DO NOTHING` 幂等，末尾校验总数不少于 235、本批 102 条齐全与首尾值；原库笔误（`624` 几内亚比招、`795` 土库存曼斯坦）原样保留。
- **新增两条 FinvQuote 运行/收集模式配置**（`V100031`，种子段）：`20000`「FinvQuote服务运行模式」（字符型，默认 `'0'`，备注 `0-云端模式（默认） 1-本地模式`）与 `20001`「历史行情收集模式」（字符型，默认 `'0'`，备注 `0 被动收集（默认，外部程序通过暴露的API接口添加），1-主动采集模式，服务主动从外部源采集入库保存`）；`ON CONFLICT (config_no) DO NOTHING` 幂等，末尾校验两项类型/默认值与备注。
- **`sys_config` 20000/20001 创建人修正为 `fqe`**（`V100032`）：`V100031` 首次落库时 `create_by` 为 `NULL`，本迁移仅修正 `create_by='fqe'`（不动类型/值/备注）；`gmt_create` 保持原插入时间、`gmt_modify` 由触发器刷新；只更新尚非 `fqe` 的行，重跑影响 0 行。
- **`sys_config` 20001 备注文案修订**（`V100033`）：`0 被动收集（默认，外部程序通过暴露的API接口添加），1-主动采集模式…` 改为 `0-被动收集（默认），外部程序通过暴露的API接口添加；1-主动采集模式，服务主动从外部源采集入库保存`（结构更清晰：`0-`/`1-` 两档用「；」分隔）；只改 `remark`，`create_by='fqe'`、类型与默认值不变，`IS DISTINCT FROM` 保证重跑影响 0 行。
- **新增月归档自动上传 Git 的两条配置**（`V100034`，种子段）：`20002`「启用月归档数据自动上传Git仓库标志」（`data_type='2'`，`char_value='0'`，备注 `0-关闭(默认) 1-启用`）与 `20003`「归档数据上传GitHub仓库的PAT值」（`data_type='3'`，`str_value` **无默认值（NULL）**，由用户自行配置）；因 `char_value` 为 `char(1)` 装不下 PAT，令牌走 `varchar(6000)` 的 `str_value`；**只 DO NOTHING、不 DO UPDATE**，并用迁移前临时快照硬校验「已有值未被覆盖」（已实测：用户改 `20002='1'` + 写入模拟 PAT 后重跑，`INSERT 0 0` 且两值原样保留）。
- **新增文件上传仓库配置**（`V100035`，种子段）：`20004`「文件上传仓库配置」，`data_type='3'`、`str_value` **无默认值（NULL）**，取值格式 `{owner}:{repo}:{branch}:{pre_path_key}`（owner 仓库所有者、repo 仓库名、branch 分支名、pre_path_key 目录前缀）；与 `20002`（是否上传）/`20003`（PAT）三件套配合；沿用 `DO NOTHING` + 快照守值，另对**用户已配置**的值做只读格式检查——不合约定只 `RAISE WARNING`不阻断部署（避免用户配错导致升级失败），由运行时应用层校验报错。
- **月归档上传 GitHub 仓库（Contents API）**：新增 `internal/meta/finv/sys`（需求 10762 归位前为 `internal/syscfg`）读取 `sys.sys_config` 的 `20002` 开关 / `20003` PAT / `20004` 仓库定位（`{owner}:{repo}:{branch}:{pre_path_key}`），带 30 秒 TTL 缓存，**在线改配置无需重启**；新增 `internal/meta/finv/quote/dedup/uploader_github.go`，用 GitHub Contents API（纯 `net/http`，**未引入任何第三方 SDK**）实现 `Uploader`：流式 base64 构造 PUT 请求体（不把大文件读进内存）、远端内容一致时**不重复提交**（幂等）、内容变更时携带远端 sha 覆盖、上传后按大小与 git blob sha 做内容级校验；新增 `uploader_dynamic.go` 按 `20002` 实时路由到 GitHub 或原 `none/dir/http`；配置缺失/非法时返回可读错误并写入上传审计日志（不静默回退到其它目标）。`GET /API/V1/Meta/Finv/Quote/DeDup/Status` 的 `upload_mode` 实时反映生效模式（`github` / `dir` / `http` / `none`）。
- **新增配置项 `FINV_QUOTE_DEDUB_UPLOAD_GITHUB_API_BASE`**（可选，默认 `https://api.github.com`）：供 GitHub Enterprise 覆盖 API 端点；已同步 `Deploy/Docker/docker-compose.yml` 与 `.env.example`。
- **证券主数据 region/market 口径修正**（迁移 `V100036` + `RefreshSecurityMetadata`）：`dedup.finv_dedup_usc.market` 改为关联 `finv_market.market_symbol` 取值（`SH`/`SZ`/`BJ`/`HK`/`NSDQ`/`CFD` 等市场级符号），不再直接抄板块级 `market_flag`（`SSE-A`/`US-NSDQ`）；`region` 改为以 `finv_region` 字典规范化（`finv_exchange.region='USA'` 统一为 `US`），字典无对应项时回退原值。`RefreshSecurityMetadata` 对这两列改为「以字典为准覆盖」（新值为空时保留旧值），其余列仍只补缺。已重绑 39 行：`CN→SH/SZ/BJ`、`HK→HK/HKR`、`FX→CFD`、`US→BMS/COMEX/NSDQ`；**影响**：新导出文件的 MVsv 文件名与归档目录随之变化（如 `CN_SH_600519_…mvsv`，目录 `CN_SH/600519/`）。
- **`finv_market` 新增 `market_symbol` 市场符号列**（迁移 `V75`）：类型 `TEXT NOT NULL DEFAULT ''`，用于表达市场的对外标识（如 `US-NSDQ` / `FX-CFD` / `SZ`）并与行情链路的 market 取值对齐；空串表示未维护。`GET/POST .../Metadata/Market/List|Save` 同步返回与写入该字段，列表 `keyword` 检索覆盖该列；前端「元数据 → 交易市场」列表新增「市场符号」列与编辑项。`ADD COLUMN IF NOT EXISTS` 幂等，末尾校验类型/可空性/默认值与列注释。

### 变更

- **Web 左侧菜单四级化 + 顶栏系统切换（需求 10803–10807）**
  - 导航改为四级树：一级为系统（顶栏切换器：`金融投资` / `内容管理`），其下二/三/四级菜单；现有 29 个业务菜单按「旧层级 +1」映射（三级叶子落到四级），**路由路径与视图组件全部不变**。
  - 菜单数据从 `Web/src/App.vue` 抽离到 `Web/src/config/menuConfig.ts`（系统注册表 + 递归菜单）；新增 `Web/src/components/menu/` 下 `SystemSwitcher` / `NavMenuTree` / `NavMenuTreeNode`（递归）。
  - 新增一级系统「内容管理」（内部 key `cms`，路由 `/Meta/CMS`）：本轮仅占位（内容管理控制台），不实现业务；`/Meta/Finv` 新增 redirect 到 `/Dashboard`。
  - 菜单展开状态按系统分桶持久化（`localStorage['finvquant.nav.openKeys.v1']`），深链接直开自动展开祖先链；切换系统回到该系统上次访问路由（`finvquant.nav.lastPathBySystem.v1`）。
  - 导航视觉固定为 WebCMS 风格（当前菜单项左侧 3px 主色激活条 + 祖先标题主色）；早期版本的风格开关与 `?nav=` 参数已按 2026-10-29 评审意见移除。
  - 按 2026-10-29 反馈调整视觉：移除抽屉顶部系统标题块（与顶栏切换器重复）、移除面包屑、移除风格开关（固定 WebCMS 视觉）；新增页脚 `© 2026 ACANX`（内容较短时贴底，登录页不显示）；抽屉宽度 300px、三/四级字号 13px、激活色跟随主题色保持不变。
  - 抽屉宽度 260px → 300px；二/三/四级菜单均保留图标。
  - 顶栏布局：平台标题**完整展示不省略**（`flex: none`、不限宽；仅 900px 以下窄屏限宽）并与一级系统切换器居左，**登录用户菜单保持在顶栏最右侧**（不随切换器搬动）。
  - 文档：`Docs/Menu/Menus.md` 重写为四级索引（新增「所属系统」列）、`Docs/Menu/CMS/CMSHome.md` 新增、`Docs/DevSpec/MenuSpec.md` §1/§1.2 更新、`Docs/Refactor/2610-012-WebMenuFourLevelRefactor.md` 增加实施记录。
  - 验证：`cd Web && npm run build`（vue-tsc + vite）通过；菜单 31 条路径全部命中 `router.ts`；深链接 `/Meta/Finv/Quote/DeDup/Export/Stats` 节点链为「金融投资 / 数据采集 / 行情数据归档 / 归档统计与预算」。

- **日志目录与切分/归档规则调整（需求 10797）**
  - 目录 `Logs/` 改为 **`Log/`**，活动文件名改为 **`FinvQuant.log`**。
  - 单文件超过 `FINV_LOG_MAX_MB`（默认 **10MB**）自动切分为 `FinvQuant-yyyyMMdd-NN.log`（`NN` 从 `01` 自增）。
  - 跨日把此前日期的切分文件打包为 `FinvQuant-yyyyMMdd.tar.gz`，打包成功后删除对应 `.log`；服务启动时也会归档停机期间遗留的历史切分文件。
  - 移除 `FinvQuant.logrotate.example`（改由应用内轮转，避免两套机制冲突）；`Install.sh`/`Uninstall.sh`/`env.example`/unit/文档同步。

- **字段表「类型 / 可空」拆分（需求 10795）**
  - 全部字段表不再在类型列写 `X \| null`：新增「可空」列（是/否），类型列只保留基础类型（`number` / `integer` / `string` / `object` 等）。
  - 涉及内部/外部 Upsert（§2.3/§3.3）、PageUni 列定义、`QuoteDeDup` 的 `CDEUOverview` / `JobRuntime` / `Settings` 等 7 张表。
  - 嵌套可空（`array<array<string \| null>>`）改为 `array<array<string>>`，由说明文字注明「单元格可为 null」。
  - `Docs/DevSpec/ApiSpec.md` §7.7 新增约定：字段表「类型」与「可空」必须分两列，类型列不得写 `X \| null`。

- **K 线 API 字段调整：`paocd` → `pacd`、`prev_close` → `last_close`（并落库，需求 10794）**
  - 请求/响应 JSON 字段 `paocd` 改为 `pacd`（数据库列仍为 `paocd`，内部映射）；`prev_close` 改为 `last_close`。
  - `last_close` 不再是“仅回填 open、不落库”：写入 `last_close` 列（`InsertColumns` 17 列），`upsert`/`PATCH`/查询/JSON 输出全部支持；`open` 缺失时仍用 `last_close` 回填；涨跌自动计算的参考价优先 `last_close`。
  - 文档：内部/外部 `KLineUpsert`（字段表、完整 17 字段请求示例、§4.1 成功响应格式化）、`KLineAdd`/`KLineBatchPatch`/`KLineQuery`/`KLinePage`/`KLinePageUni` 同步；两份 `KLineUpsert.yml` 的 `KLineItem` schema 更新为 17 字段（含 `pacd`/`last_close`）。
  - 验证：`go build`、`go test ./internal/kline` 通过（新增 `last_close` 落库与 PATCH 断言）；数据库无需迁移（`last_close` 列已存在）。

- **外部 OpenAPI 文档改为自包含，不再引用内部 API 文档（需求 10793）**
  - `Docs/OpenAPI/Quote/Cdeu/Min/Upsert.md`：内联 `KLineItem` 17 字段表与 15 条错误目录；删除对 `Docs/API/.../KLineUpsert.md`、`KLineUpsert.yml` 的引用。
  - `Docs/OpenAPI/Quote/Cdeu/Min/Upsert.yml`：`KLineItem` schema 内联，移除跨文件 `$ref`，对外契约独立可分发。
  - `Docs/OpenAPI/Quote/Cdeu/Min/PageUni.md`：内联 15 列定义与单元格取值规则，删除内部文档引用，修正章节层级（4.2/4.3/4.4）。
  - `Docs/DevSpec/ApiSpec.md` §7.7：新增「内部/外部分离且自包含」规则——`Docs/API/` 为内部文档、`Docs/OpenAPI/` 为对外契约，对外文档不得引用内部文档或内部 `.yml`。

- **分钟行情 volume 全链路统一为浮点 `numeric(20,6)`（需求 10792）**
  - 新增迁移 `V200059__kline_min_volume_numeric_all_schemas.sql`：把 `quote` / `export` / `dedup` 三张同名表的 `volume` 从 `bigint` 改为 `numeric(20,6)`（与 `V200058` 的 `cdeu` 一致），重建 `CHECK (volume >= 0)`，幂等、无损。
  - `internal/mvsv/parser.go`：`Row.Volume` 由 `*int64` 改为 `*string`（`numericTextOrNil` 原样保留小数文本），修复含小数 MVsv 导入时 volume 被静默丢弃的问题。
  - `internal/quote/service.go`：查询结果 `Bar.Volume` 由 `*int64` 改为 `*float64`，JSON 输出保持数字并支持小数。
  - 文档：`HistoryQuote.md` 标注 volume 支持小数；`MvsvFileFormat.md` 已为 `Decimal(20,6)`。
  - 验证：临时 PG18 上 141 个迁移 0 错误；四表 volume 均为 `numeric(20,6)`；`12.345678` 在四表写入/读回一致；重复执行 V200059 全部跳过；`go build`、`mvsv/quote/kline` 测试通过（新增小数解析用例）。

- **cdeu 分钟行情成交量改为浮点（`numeric(20,6)`，需求 10791）**
  - 新增迁移 `V200058__cdeu_kline_min_volume_numeric.sql`：`cdeu.finv_quote_secu_kline_min.volume` 由 `bigint` 改为 `numeric(20,6)`，重建 `CHECK (volume >= 0)`；幂等（已是 numeric 直接跳过），`bigint → numeric` 无损。
  - 仅调整 `cdeu` 表；`public` / `quote` / `dedup` / `export` 的同名表不动。
  - 代码无需改动：`internal/kline` 已按 numeric 文本读写（`volume::text`、`CellText` 去尾零），集成测试表本为 `numeric(20,6)`；`MVsvFileFormat.md` 中 `v` 的类型由 `Long` 更新为 `Decimal(20,6)`。
  - 验证：临时 PG18 上 140 个迁移 0 错误；`12.345678` 写入/读回正常；重复执行 V200058 输出「已是 numeric，跳过」。

- **外部 OpenAPI 暂不鉴权（需求 10790）**
  - `/OpenAPI/V1` 路由组移除 `RequireOpenAPIKey`，公开访问；仅保留 `WrapOpenAPIResponse` 统一信封。
  - `RequireOpenAPIKey`、`FINV_OPENAPI_KEY` 与 `config.OpenAPIKey` 保留（当前不生效），待 API Key（`sys.sys_open_api_key`）落地后恢复。
  - 文档/契约同步：外部 Upsert/PageUni 的认证章节改为「暂不鉴权」，两份 `.yml` 的 `security` 置空、securitySchemes 标注预留；`FinvQuant.env.example`、方案文档 §5.5、`APIs.md`/`README.md` 同步。
  - **风险提示**：公开端点可写入/查询分钟行情，部署时必须用防火墙/安全组或内网限制来源。

- **`KLinePageUni.md` 按 OpenAPI 3.1 样板重写并拆分契约（需求 10785）**
  - 正文补齐：Header/Query 参数表（`usc`、`provider`、`dateFrom/dateTo`、`tsFrom/tsTo`、`limit`、`offset` 及兼容旧名）、过滤/排序（`usc,date,ts` 升序）/分页语义、15 列定义与单元格取值规则（去尾零、负零归一、`cr` 两位小数 ROUND_HALF_UP、NULL→null）、响应信封、15 个错误码及本接口触发场景、curl 与错误示例。
  - 新增同名 `KLinePageUni.yml`（OpenAPI 3.1：8 个 query 参数、`PageUniResponse`/`PageUniData`/`ErrorResponse`/`ErrorObject`）。
  - `Docs/API/README.md` 标注其伴生契约文件。

- **OpenAPI 契约拆分为同名 `.yml` 文件（需求 10784）**
  - 新增 `Docs/API/Quote/Cdeu/Min/KLineUpsert.yml`：从 `.md` 内嵌代码块抽出的完整 OpenAPI 3.1 契约，供 lint、`preview-docs`、`openapi-generator` 直接消费。
  - `KLineUpsert.md` 第 8 节只保留契约链接与校验/生成命令，不再重复 YAML（避免两处维护漂移）。
  - `Docs/DevSpec/ApiSpec.md` §7.7：约定接口文档由「同名 `.md`（人读）+ `.yml`（机读）」成对组成。
  - 新增 `internal/docs/openapi_test.go`：校验 `Docs/**/*.yml|yaml` 可解析且含 `openapi`/`paths`；`github.com/goccy/go-yaml` 由间接依赖转为直接依赖。

- **接口文档样板：`KLineUpsert.md` 按 OpenAPI 3.1 规范重写（需求 10783）**
  - 正文按「人先看」组织（概述 → 参数表 → 行为语义 → 错误目录 → 示例 → 使用登记 → 编写约定），完整 `openapi: 3.1.0` 契约作为末尾附录（`paths`、`parameters`、`requestBody`、`responses`、`components`，含 `KLineItem` 17 字段 schema、`UpsertResponse`、`ErrorResponse`/`ErrorObject`、Bearer 认证）；该契约随后在需求 10784 拆为同名 `.yml`。
  - 补齐此前被省略的内容：请求体不再写「同 KLineBatchInsert.md」，改为逐字段表；补充字段约束（`usc` 1–255、`ts` 0–1e11、`date`/`time` 范围、`volume`/`turnover` ≥ 0、`provider` ≤ 255）、行为语义（skip/update、整行覆盖、`prev_close` 回填、`change_*` 自动计算、单事务/幂等）、全量错误目录（15 个 code + HTTP + retryable + debug 键）、curl/成功/错误示例。
  - `Docs/DevSpec/ApiSpec.md` 新增 §7.7「接口文档样板（OpenAPI 3.1）」；`Docs/API/README.md` 将本文标注为样板。

- **全部分钟行情端点统一到 `/API/V1/Meta/Finv/Quote/Cdeu/Min/*`（需求 10782）**
  - 端点：`Add`、`Batch`（POST/PATCH/DELETE）、`Upsert`、`Page`、`PageUni`、`Query`、`Count`、`Stats`、`Bounds` 全部改挂 CDEU 前缀；旧 `/API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/*` 与 Python 服务前缀 `/API/V1/Meta/Finv/Quote/Secu/KLine/Min/*` **不再注册**（返回 404）。
  - 代码：`kline.APIPrefix` 改为 CDEU 前缀，router 单前缀注册全部端点（不再循环两套前缀），`router_test` 断言 22 条旧路径均不存在。
  - 前端：3 处 API 调用（`Bounds`×2、`Stats`）改为 CDEU；Vue 页面路由（`/Meta/Finv/Quote/DeDup/Secu/KLine/Min/*`）保持不变。
  - 文档：`Docs/API/KLine/` 剩余 9 个文档全部迁入 `Docs/API/Quote/Cdeu/Min/`（共 12 个），标题/路径/旧路径说明同步；`Docs/API/KLine` 目录移除；`APIs.md`、`README.md`、`Docs/Menu/*`、`DbSchemaSpec`、`ApiSpec` 同步。
  - **破坏性变更**：所有调用方需把 K 线 API 路径前缀改为 `/API/V1/Meta/Finv/Quote/Cdeu/Min`（`GET /Health`、`GET /healthz` 运维端点不受影响）。

- **KLinePage / KLinePageUni 文档迁入 `Docs/API/Quote/Cdeu/Min/`（需求 10780）**
  - `Docs/API/KLine/KLinePage.md`、`KLinePageUni.md` → `Docs/API/Quote/Cdeu/Min/`（当时端点未变，随后由需求 10781 迁到 `/API/V1/Meta/Finv/Quote/Cdeu/Min/Page(Uni)`）。
  - 同步 `APIs.md` 索引、`README.md` 接口索引（新增「分钟行情 CDEU 目录」小节）、`Docs/Menu/HistoryQuote/DeDupKLineQuery.md` 链接。
  - 顺带修复 4 处失效相对链接：`Deploy/Docker/Win11DockerUpgrade.md` 的「迁移目录」改为 `../Migrations/`；`Archive/.../Windows11Deployment.md` 3 处 `Docs/...` 改为 `../Docs/...`。

- **分钟行情批量 Upsert 端点迁移到 CDEU 前缀（需求 10777）**
  - 路径：`/API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Upsert` → **`/API/V1/Meta/Finv/Quote/Cdeu/Min/Upsert`**。
  - 旧路径（含旧前缀别名的 `/API/V1/Meta/Finv/Quote/Secu/KLine/Min/Upsert`）**不再注册**；其它 K 线端点（`Add`/`Batch`/`Page`/`PageUni`/`Query`/`Count`/`Stats`/`Bounds`）不变。
  - 新增常量 `kline.CdeuAPIPrefix`、`kline.CdeuUpsertPath`；`router_test.go` 增加「旧路径必须不存在」断言。
  - 文档同步：`Docs/API/APIs.md`、`Docs/API/Quote/Cdeu/Min/KLineUpsert.md`、`Docs/API/README.md`、`Docs/DevSpec/DbSchemaSpec.md`。
  - **破坏性变更**：外部采集端/调用方需在升级前把 URL 改为新路径。

- **`Nginx.conf.example` 改为「阶段一 HTTP/80 + 阶段二 HTTPS/443」分阶段模板（需求 10774）**
  - 阶段一默认启用：`listen 80 default_server;` + `server_name _;`，无需域名与证书，`http://<服务器IP>` 直接访问；前端 16002、API `/API/` → 16001、上传体积/超时/安全头/ACME 目录均在此块。
  - 阶段二（443 + `http2 on;` + 证书 + 80→443 跳转）以**整段注释**放在文件末尾，取消注释即可切换；注释内含启用步骤与 nginx < 1.25.1 的写法说明。
  - 明确 Ubuntu/Debian 需先 `rm -f /etc/nginx/sites-enabled/default`（其 `default_server` 会冲突或导致 IP 访问落到欢迎页）。
  - `Deploy/Native/Install.md` §6、`Deploy/Native/README.md` 文件表同步。

- **核心配置文件移到安装根目录 `<PREFIX>/finvquant.env`（需求 10769）**
  - 由 `<PREFIX>/etc/finvquant.env` 改为 `<PREFIX>/finvquant.env`，与 `bin/`、`Deploy/` 同级；systemd unit 的 `EnvironmentFile` 同步为 `@PREFIX@/finvquant.env`。
  - **兼容旧布局**：`Install.sh` / `Upgrade.sh` 检测到 `<PREFIX>/etc/finvquant.env` 且新路径不存在时，自动 `mv` 到新位置并保留原数据库密码（不重新生成），随后清理空 `etc/`。
  - **Uninstall**：`--purge` 删除新路径的 `finvquant.env`（并兜底清理旧 `etc/`）。
  - 文档 `Native/README.md`（§6 布局与权限表、§9）、`Install.md`（§0、§4 手工步骤、§5 自检）、`Upgrade.md` 同步。

- **Migrations 目录统一为 `<PREFIX>/Deploy/Migrations`（需求 10768）**
  - **原因**：一处是安装后的 `FINV_MIGRATIONS_DIR=<PREFIX>/Migrations`，一处是包内的 `Deploy/Migrations`，两套路径容易混淆且需要额外复制才能对齐。
  - **结论**：与仓库结构保持一致，安装后即 `<PREFIX>/Deploy/Migrations`（默认 `/data/App/FinvQuant/Deploy/Migrations`）。
  - **包结构**：`PackSnapshot.sh` 保持 `cp -r Deploy/Migrations "${PKG_ROOT}/Deploy/Migrations"`，Linux `tar.gz` / Windows `zip` 同构。
  - **安装/升级脚本**：`MIGRATIONS_DIR=<PREFIX>/Deploy/Migrations`；探测顺序 `<PREFIX>/Deploy/Migrations` → `<repo>/Deploy/Migrations` → Native 脚本上一级/同级；源即目标时**跳过复制**；升级时自动补建目录。
  - **模板与文档**：`FINV_MIGRATIONS_DIR=/data/App/FinvQuant/Deploy/Migrations`，Native README/Install/Upgrade、`Deploy/README.md`、`Script/README.md` §5 全部对齐。

- **Native 安装统一到 `/data/App/FinvQuant/`（需求 10766）**
  - **目录布局**：`bin/finvquant`（软链 `/usr/local/bin/finvquant`）、`Migrations/`、`etc/finvquant.env`（600）、`Deploy/`（`Install.sh`/`Upgrade.sh`/`Uninstall.sh`、`finvquant.service`、env 与 Nginx 模板、手册）、`Backup/`、`Export/` 全部收拢到同一安装根目录；原 `/opt/finvquant`、`/etc/finvquant`、`/var/backups/finvquant` 不再使用。
  - **可自定义根目录**：`Install.sh --prefix <dir>`（或环境变量 `FINVQ_PREFIX`）可改装到其它绝对路径；`Upgrade.sh` / `Uninstall.sh` 默认从 systemd unit 的 `ExecStart` 推断，无需重复指定。
  - **unit 模板化**：`FinvQuant.service` 改用 `@PREFIX@` 占位（`WorkingDirectory` / `EnvironmentFile` / `ExecStart` / `ReadWritePaths` / `Documentation`），安装或升级时渲染为 `<PREFIX>/Deploy/finvquant.service` 并注册到 `/etc/systemd/system/finvquant.service`。
  - **升级脚本**：备份写入 `<PREFIX>/Backup`（由 root 重定向，避免 `postgres` 对目录无写权限，失败时清理空文件）；同步替换二进制、迁移脚本、部署脚本/模板与 unit 并 `daemon-reload`；拒绝「新二进制 == 已安装二进制」的误操作。
  - **卸载脚本**：`--purge` 删除 `bin`/`etc`/`Migrations`/`Deploy`，**保留** `Backup` 与 `Export` 数据；`--purge-data` 只打印删库与清理备份/导出的指引。
  - **环境变量模板**：`FINV_MIGRATIONS_DIR=/data/App/FinvQuant/Migrations`，新增 `FINV_QUOTE_DEDUB_OUTPUT_DIR`/`FINV_QUOTE_DEDUB_UPLOAD_DIR`/`FINV_QUOTE_DEDUB_UPLOAD_MODE`（与容器方式 `/data/App/FinvQuant/Export` 口径一致），安装脚本按实际根目录改写。
  - **文档同步**：`Deploy/Native/{README,Install,Upgrade}.md` 与 `Deploy/README.md` 的路径、权限矩阵、手工安装步骤、定时备份示例全部改为安装根目录口径。

- **前端基础路由改为大驼峰（需求 10764）**
  - `/dashboard` → `/Dashboard`、`/login` → `/Login`，与 `/Meta/Finv/...` 风格统一。路由 `name`（`dashboard` / `login`）保持不变，仅调整 URL。
  - **旧路径归一化不走路由级 redirect**：`vue-router` 4 的路径匹配默认大小写不敏感（`sensitive: false`），`/login` 会直接命中 `/Login` 路由，路由表里的 `{ path: '/login', redirect: '/Login' }` **永远匹配不到**。改在 `beforeEach` 守卫里用 `LEGACY_PATH_MAP` 归一化——此时 `to.path` 仍是访问时的原始大小写，是唯一可靠的位置。
  - 同步修正 `App.vue` 菜单项路径、`LoginView.vue` 默认回跳目标、`App.vue` 退出登录跳转。

- **认证接口前缀统一到 /Meta/Finv/Auth（需求 10765）**
  - `/API/V1/Auth/{Login,Logout,ChangePassword,Profile}` → `/API/V1/Meta/Finv/Auth/*`，与 `/Meta/Finv/Quote`、`/Meta/Finv/Quant` 同层级。旧路径返回 404（不保留别名）。
  - 同步前端三处调用与 `Docs/API/{Auth/Auth.md,APIs.md,README.md}`。
  - **顺带修掉一个真实 bug**：`api.ts` 原先对所有 401 一律执行「清空会话 + 广播失效事件 + 抛『未登录或登录已过期』」。登录接口本身用 401 表示口令错误，导致用户输错密码时看到的是「未登录或登录已过期」，并且误清一次本就不存在的会话。改为按路径分流：登录接口的 401 走正常信封解析，服务端 `message`（如「用户名或密码错误」）原样抛给用户。

- **登录后修改密码（需求 10763）**
  - **接口**：新增 `POST /API/V1/Meta/Finv/Auth/ChangePassword`，请求体 `{current_password, new_password}`，返回 `{revoked_sessions}`。
  - **校验顺序**：当前密码 → 新旧是否相同 → 新密码强度。任一失败返回 400 且 `message` 指明原因（「当前密码不正确」/「新密码不能与当前密码相同」/「新密码不符合强度要求：…」）。
  - **强度规则**：长度 8–64 位，且含大写/小写/数字/符号中至少 3 类。前端 `strengthOk` 与服务端 `ValidatePasswordStrength` 同口径，但裁决权在服务端。
  - **哈希轮换**：每次改密重新 `GenerateSalt` 并计算 argon2id 哈希，`modify_by` 记当前 uid；旧盐与旧哈希不保留。
  - **会话吊销**：新增反向索引 `finv:auth:user:<user_id>`（Redis Set）。改密后遍历索引吊销该用户**除当前 token 外**的全部会话，避免「口令已改但旧会话仍可访问」的安全缺口，同时不把刚改完密码的用户自己踢出。索引项无论会话是否仍存在一律移除，顺带清理过期残留。
  - **前端**：新增 `ChangePasswordDialog.vue`，顶栏用户菜单加「修改密码」入口；三个口令输入框均支持右侧眼睛图标切换明文/密文（复用登录页交互）；新密码输入即时显示强度进度条，两次输入不一致时行内报错；成功后提示吊销的其它设备数量。
  - **验证**：旧密码错/弱口令/新旧相同均 400；正常修改 200 且 `revoked_sessions=1`；改后旧口令登录 401、新口令登录 200；操作会话仍 200，另一并发会话变 401；改密后 `password_salt` 与初始值不同且仍为 32 位 hex；明文命中 0。

- **登录认证与访问拦截（需求 10761）**
  - **用户与口令**：`sys.sys_user` 新增初始账号 `finv`（idn=1028）。口令原文**不入库**：argon2id（`m=65536,t=3,p=2,keyLen=32`）哈希为 97 字符 PHC 串存 `password`，16 字节随机盐 hex 32 字符存 `password_salt`；校验用 `crypto/subtle` 常量时间比较。
  - **防账号枚举**：用户不存在时仍执行一次等价耗时的 argon2 计算，两条失败路径统一返回 401「用户名或密码错误」。
  - **会话**：令牌 32 字节随机 → hex 64 字符，存 Redis `finv:auth:session:<token>`；有效期读 `sys_config` 20012（默认 480 分钟），命中且剩余不足一半时滑动续期；登出即删键，服务端可即时吊销。
  - **接口拦截**：新增 `RequireAuth` 中间件，除 `/Health/Live`、`/Health/Ready`、`/Version`、`/Meta/Finv/Auth/Login` 外全部要求 `Authorization: Bearer <token>`；新增 `POST /Meta/Finv/Auth/Login`、`POST /Meta/Finv/Auth/Logout`、`GET /Meta/Finv/Auth/Profile`。
  - **前端**：新增登录页 `LoginView.vue` 与 `auth.ts`（localStorage 持久化）；`router.ts` 加 `beforeEach` 守卫，未登录重定向 `/login?redirect=<来源>`；`api.ts` 自动附加令牌并在 401 时清空会话、广播 `finvquant:unauthorized`；`App.vue` 登录页不渲染导航框架，顶栏新增用户菜单与「退出登录」。
  - **迁移**：`V100081` 补 `uk_sys_user_uid` 唯一索引（V74 建表时 uid 无唯一约束，登录以 uid 为标识必须唯一）；`V200055` 幂等插入 `finv`（`WHERE NOT EXISTS`，不覆盖已改口令）；`V200056` 新增配置 20012（会话有效期，合法区间 5~43200）。
  - **验证**：明文命中 0；无令牌访问业务接口 401、健康检查 200；错误口令与不存在账号同为 401 且文案一致；正确口令返回 64 字符 token 且可访问业务接口；伪造令牌 401；登出后原 token 401；前端 `/login` 深链可直达。

- **认证与配置模块目录归位（需求 10762）**
  - `internal/auth` → `internal/meta/finv/auth`，`internal/syscfg` → `internal/meta/finv/sys`，与 `internal/meta/finv/quote/dedup` 同构。
  - 仅移动目录与更新 import 路径，包名（`auth` / `syscfg`）与导出符号不变；同步更新 `Docs/RequirementList.md` 中 10577/10578/10594/10650 的核对命令路径，保证历史需求仍可复核。

- **回填历史数据并验证完整导出/上传/归档链路（需求 10760）**
  - **准备**：停容器 → 从 `quote.finv_quote_secu_kline_min` 回填 93 个月 / **2,802,911 行**到 `cdeu.finv_quote_secu_kline_min`（耗时 195.8 秒，`gmt_create` 取 `now()`）→ 清空产物登记（`upload_log` 202 / `file` 202 / `batch` 202 / `watermark` 101）→ 101 个 job 重置 `WAITING` → 清本地 176.3 MB / 202 个旧文件 → 起容器。
  - **执行（run 66）**：`SUCCESS`，耗时 **18 分 52 秒**；6 阶段全 `SUCCESS`；归档复核 5 流程全 `SUCCESS`。
  - **覆盖的路径**：101 个 `EXPORT/BASE` 动态 JOB、101 个 `UPLOAD` 动态 JOB、**505 个动态 Step**（`STEP_LOAD`/`STEP_EXPORT`/`STEP_REGISTER`/`STEP_CLEANUP` 各 101、`STEP_UPLOAD` 101）全部 `SUCCESS`；证券月 `GCMain` 93（201811–202607）、`NVDA` 4、`XAUUSD` 4。
  - **结果**：`BASE/UPLOADED` 101、`MANIFEST/SKIPPED` 101（按约定不上传）；20007 删主表 `2,803,911 → 1,000`（仅剩 202610 陷阱）；20011 本地清理未触发（202 文件仍在，需上传满 24 小时，符合预期）；101 个 job 终态 `EXPORTED`。
  - **隔离证明**：`dedup_frozen` 的 `cdeu_workflow_run` 11 / `finv_dedup_export_file` 202 / `finv_quote_secu_kline_min` 1,000 在流程前后**完全不变**，所有写入落在 `cdeu`。
  - **日志**：`does not exist` / `ERROR` / `panic` = 0。
  - **结论**：在 `dedup` schema 完全不可用的条件下全链路零错误跑通，确认代码 100% 切换到 `cdeu`，`dedup_frozen` 可进入观察期后清理。

- **前端文案与镜像构建修正（需求 10758）**
  - **前端**：6 个视图共 18 处数据来源文案由 `dedup.<表>` 改为 `cdeu.<表>`（`DeDupArchiveChainView` 9、`DeDupKLineQueryView` 3，其余各 1–2）。
  - **镜像构建**：`COPY . .` 会把工作区残留的旧前端产物带进镜像，随后 `COPY --from=web-builder /web/dist /app/internal/webui/dist` 只叠加不清理，导致不同 hash 的 `index-*.js` 一同被 `go:embed` 打包，旧包里仍含已修复文案。Dockerfile 改为 COPY 前 `RUN rm -rf /app/internal/webui/dist`。
  - **发现方式**：`grep` 只扫 `.go` 源码，前端文案需扩到 `Web/src/` 才可见；镜像叠加问题只有 `strings /usr/local/bin/finvquant` 扫编译产物才暴露。静态验证须覆盖「源码 grep → 编译产物 strings」两层。
  - **验证**：`grep -rnE 'dedup\.[a-z][a-z0-9_]*' Web/src/` = 0；容器内二进制 `dedup.(finv_*|cdeu_*)` 表名残留 = 0。

- **`dedup` schema 重命名为 `dedup_frozen` 并完成 fail-fast 验证（需求 10759）**
  - **操作**：停容器 → `ALTER SCHEMA dedup RENAME TO dedup_frozen` → `search_path` 移除 `dedup` → 起容器。31 张表（含 5 张 `prep_bak_*`）与全部数据原样保留，回滚只需一条 `ALTER SCHEMA dedup_frozen RENAME TO dedup`。
  - **fail-fast 验证（只读）**：`dedup` 完全不可用后，16 个只读接口全部 `code:0`、**0 失败**。任一残留引用都会立刻报 `relation "dedup.xxx" does not exist`。
  - **完整流程验证（run 65）**：`SUCCESS`，6 个阶段全 `SUCCESS`（`PHASE_COLLECT` 1/1、`PHASE_DEDUP` 1/1、`PHASE_DECISION` 4/4、`PHASE_EXPORT` 3/3、`PHASE_UPLOAD` 1/1、`PHASE_REVIEW` 5/5）；15 个流程 `started_at` 严格递增（10747 修正生效）。
  - **写入归属证据**：`cdeu` 侧 `workflow_run` +1 / `phase_run` +6 / `process_run` +15 / `job_run` +21 / `job_step_run` +14 / `job_run_log` +117 / `job_run_log_line` +451；`dedup_frozen` **一行未变**。
  - **日志**：`does not exist` / `ERROR` 计数 = 0。
  - **未覆盖路径**：run 65 仅 4 秒结束，`job_dynamic` / `job_step_dynamic` 为 0——主表只有 202610（当月未成熟）1,000 行，无导出候选。导出动态 JOB、文件生成、上传三条路径尚未在"旧 schema 不可用"条件下验证，需回填历史月份数据后再跑一轮。

- **代码读写切换到 `cdeu` schema（需求 10757）**
  - **范围**：`internal/` 与 `cmd/` 中 290 处 `dedup.*` 表引用改为 `cdeu.*`，涉及 19 个文件。映射 `dedup.cdeu_*` / `dedup.finv_dedup_*` → `cdeu.finv_quote_cdeu_*`，`dedup.finv_quote_secu_kline_min` → `cdeu.finv_quote_secu_kline_min`。
  - **替换规则**：按表名长度降序 + 右边界 `(?![a-z0-9_])` 精确匹配，避免 `cdeu_workflow` 误伤 `cdeu_workflow_run`。
  - **边界保留（4 处）**：Go 包引用 `dedup.Service`（首字母大写不匹配）、测试临时表 `dedup.finv_quote_secu_kline_min_it`（2 处，独立表）、历史注释 `dedup.finv_dedup_runtime_setting`（已删表说明）。
  - **关键常量**：`store.go` 的 `tableUsc`/`tableJob`/`tableFile`；`kline/sql.go` 的 `const Table`（K 线 API 写入目标）全部改为 `cdeu.*`。
  - **防回归闸门**：`schema_qualify_test.go` 的 `domainTablePrefixes` 新增 `"finv_quote_cdeu_": "cdeu."`；`unqualifiedDomainTables` 增加「最长前缀优先」避免 `finv_quote_cdeu_*` 被 `finv_quote_` 重复报错并给出矛盾建议；`exactTableSchema` 注释重写四份 `finv_quote_secu_kline_min` 副本职责。
  - **测试**：`kline/sql_test.go` 的 `TestTableConstantQualified` 断言由 `dedup.` 改为 `cdeu.`。
  - **切换后状态**：代码全部读写 `cdeu.*`；`dedup.*` 数据保留但不再被读写（只读历史）；两边在切换时点一致（`V100080` 迁入的 133,260 行），之后 `cdeu` 增长、`dedup` 停滞。
  - **回滚**：重新部署切换前镜像即可（`dedup` 数据仍在，仅丢切换后写入 `cdeu` 的新行）。
  - **验证**：`go build` / `go vet` / `go test ./...` 全绿；部署后 8 个只读接口（`CDEU/Overview`、`Status`、`Usc/Page`、`Job/List` 101 条、`File/List` 202 条、`RunLog/Page` 8059 条、`Maturity/Page` 2492 条、`Priority/List` 3 条）全部正常返回，启动日志无 SQL 错误。

- **修正 cdeu schema 的命名规则：`cdeu_` 前缀同样替换为 `finv_quote_cdeu_`（需求 10755，已部署 V100079）**
  - **问题**：10754（`V100078`）把命名规则理解成「只有 `finv_dedup_` 前缀需要替换」，12 张 `cdeu_*` 表沿用了原名。正确规则是三者统一：`cdeu_` / `finv_dedup_` → `finv_quote_cdeu_`，其他不变，`prep_bak_` 不建。
  - **矫正方式**：`V100079` 用 RENAME 而非 DROP+CREATE —— `V100078` 已 `success=t`，遵循「不修改已应用迁移」规范，保留其历史并由新迁移矫正；RENAME 不动结构与数据，比重建更安全。
  - **实施**：5 个 DO 块按系统表动态生成 RENAME 语句，顺序为 表 → 序列 → 约束（PK/UNIQUE/CHECK/NOT NULL，`RENAME CONSTRAINT` 连带重命名底层索引）→ 独立索引（`idx_cdeu_*` → `idx_finv_quote_cdeu_*`），逐类计数并输出 NOTICE。
  - **标识符长度**：统一 +11 字符（`finv_quote_`），改名后最长 61 字符（`cdeu_job_step_dynamic_dynamic_job_id_step_code_key` 50 → 61），未超 PostgreSQL 的 63 上限。
  - **不变项**：表 26 / 序列 17 / 索引 80 / 约束（CHECK 26 + NOT NULL 249 + PK 26 + UNIQUE 18）与矫正前完全一致；数据行合计仍 0；`dedup` 侧 31 张表未受影响。
  - **残留核验**：`cdeu_` 前缀的表/序列/索引/约束、`idx_cdeu_` 前缀的索引全部为 0。
  - **测试同步**：`schema_qualify_test.go` 的 `cdeuTables` 改为 25 张 `finv_quote_cdeu_*` + 1 张 `finv_quote_secu_kline_min`，按「原 cdeu_*」「原 finv_dedup_*」「表名不变」三组注释。
  - **验证**：`V100079 success=t`；`go build` / `go vet` / `go test ./...` 全绿；容器 healthy。

- **日志前缀 `[exportarchive]` 统一改为 `[cdeu]`（需求 10752）**
  - **范围**：25 个文件共 97 处 `log.Printf` 前缀字面量，仅换前缀，不改变量名/结构体字段/接口路径。
  - **理由**：该日志域早已从「导出归档」演进为 CDEU 工作流（Workflow → Phase → Process → JOB → Step），前缀与模块语义不符；运维按前缀过滤日志时也容易漏掉 `cdeu.go`、`scheduler.go` 等非归档文件。
  - **主要文件**：`job_export.go`(19)、`cdeu.go`(12)、`job_patch.go`(9)、`job_upload.go`(8)、`scheduler.go`(7)、`service.go`(5)、`job/purge_local_export_files.go`(5)、`job/purge_exported_month.go`(4)，其余 17 个文件各 1–3 处。
  - **验证**：`grep -rn '\[exportarchive\]' --include=*.go .` = 0；容器重启后启动日志全部为 `[cdeu] …`，旧前缀条数 0；`go build` / `go vet` / `go test ./...` 全绿。

- **迁移脚本编号统一 +100000，编号分段重新划分（需求 10751）**
  - **范围**：`Deploy/Migrations/` 下全部 **131** 个脚本的 `V` 后编号统一加 100000，执行顺序与修改前完全一致。
  - **新分段**：结构/变更迁移 `V100001`～`V199999`（原 `V1`～`V76`，76 个）；种子/数据迁移 `V200000+`（原 `V100000`～`V100054`，55 个）。
  - **示例**：`V100029__sys_dict_sub_seed_common.sql` → `V200029__sys_dict_sub_seed_common.sql`；`V71__create_sys_schema.sql` → `V100071__create_sys_schema.sql`。
  - **`schema_version` 同步**：已应用的 131 条记录版本号一并 +100000。单条 `UPDATE` 会撞主键（`1→100001` 与 `100001→200001` 冲突），改用「临时表备份 → `DELETE` → `INSERT (old+100000)`」单事务重建。
  - **引用同步**：文件名引用 `V<n>__` 250 处、裸版本引用 57 处、编号分段约定 6 处（`Prompt.md`、`Deploy/README.md`、`Deploy/Docker/Win11DockerUpgrade.md`、`Docs/DevSpec/DbSchemaSpec.md`、`Docs/DataDictMapping/FinvExchange.md`、`AGENTS.md`）。
  - **执行纪律**：改名前**必须先停容器**——旧二进制内嵌旧文件名，若先改名再重启，框架会把 131 个迁移全判为「未执行」并整体重跑。
  - **验证**：131 个文件编号 `100001`～`200054` 严格递增无重号；`schema_version` 131 条、范围 `100001`～`200054`、无 `< 100001` 遗留；容器重启后启动日志**无任何迁移执行记录**；`go build` / `go vet` / `go test ./...` 全绿；git 识别 66 个重命名 + 65 个新增。
  - **过程事故（已恢复）**：首次重命名用 `git mv` 在第 26 个文件失败（目标名被尚未改名的旧文件占用），前 25 个已变成 `TMP__` 前缀；随后脚本的「清理残留临时文件」逻辑对这些文件执行了 `os.remove`，误删 25 个已跟踪文件。用 `git restore --source=HEAD --staged --worktree` 从 `RD` 状态精确恢复后重新完成改名。教训：批量重命名脚本不得包含无差别删除临时前缀文件的逻辑。

- **修正 `RequirementList` 中与实际不符的状态标记（需求 10707–10732）**
  - **过期标记清除**：`10716`、`10723` 原写「（暂不实现，下一次开发）」，实际已随 `V200048__cdeu_review_phase_job_reorg.sql` 全部落地，改为正常状态并标 `√`；`10717–10722` 节首的「本节……统一暂不实现，下一次开发落地」改为「已随 V100048 全部落地」，并补注 `10744` 起本地文件清理由 `JOB_PURGE_LOCAL_FILES` 承担。
  - **结果列补勾（6 条）**：`10707`–`10712`（定义表 LEFT JOIN 取子级、已终态父级可回退、跑马灯动效、`prefers-reduced-motion`、只作用于 Phase/Process 两级）代码早已实现，结果列却为空，补 `√`。
  - **结果列补勾（2 条）**：`10714`（删除 17 张历史备份表）、`10715`（`cdeu_job_dynamic_step` → `cdeu_job_step_dynamic` 改名）。
  - **`10742` 加口径修正备注**：它的 `LEAST(..., now())` 兜底正是 `10747` 破绽的来源，标注「现口径见 10747」，并把核对方式换成 `started_at > finished_at` 残留为 0（原核对方式在 10747 之后已不适用）。
  - **`10732` 标注为「规范项·非功能需求」**：它是事故教训（修复脚本被 `RUN_DUE_EXPORT_JOBS` 抢跑），结果列由空改为「规范已定」。
  - **`10713` 标注由「暂不实现」改为「未实现」**：核实 `phaseProgress` / `processProgress` 仅驱动 `v-progress-linear` 的宽度，无任何百分比文本渲染，确认功能确实缺失。
  - **顺带整理**：`purgeMainSecurityMonth` 从 `job/upload_pending_export_files.go` 迁到 `job/purge_exported_month.go`（唯一调用方），使 `10722` 的核对条件「上传 JOB 文件不含主表清理代码」成立。
  - **验证**：`10707`–`10747` 共 41 条中，除 `10713` 外全部 `√` / 规范已定；`gofmt` 干净、`go build` / `go vet` / `go test ./...` 全绿。

- **归档导出状态重置（迁移 `V100037`，配套 `V100036` 的 region/market 口径变更）**：清除旧口径的 226 条文件登记（`finv_dedup_export_file` / `_patch`）与 202 条批次（`finv_dedup_export_batch`），101 个 `EXPORTED` 任务回退为 `FROZEN` 并清空装载游标/版本/补丁计数/锁与错误，当日预算清零消耗并抬到 500 万行（硬顶 600 万），使完整工作流可重新导出为 `CN_SH` / `US_NSDQ` 等新命名与新目录。成熟度 `FROZEN`、扫描水位、`finv_dedup_usc`、CDEU 历史记录均保留；上传审计日志 `finv_dedup_export_file_upload_log` 保留作历史凭据。**迁移只改数据表**：本地导出文件与远端仓库旧路径对象需运维另行清理。重跑保护：仅当仍存在 `EXPORTED` 任务时才执行，避免导出进行中被二次清空。
- **归档上传范围收窄为仅 `.mvsv` 数据文件**：`xx_..._Manifest.json` 不再推送远端（GitHub 或其他目标）——`PickUploadFile` 只领取 `BASE`/`PATCH`；待上传的 MANIFEST 由上传 Job 统一标记为 `SKIPPED` 并写审计日志（`MANIFEST 按业务约定不上传（仅上传 .mvsv 数据文件）`），避免其滞留待上传队列阻塞本地清理判定与监控指标；`RegisterFile` 的 upsert 分支在「内容未变 + 已是 SKIPPED + 类型为 MANIFEST」时保持 `SKIPPED`，不反复入队；本地清理条件放宽为「`UPLOADED` 且远端已校验」或「MANIFEST 的 `UPLOADED`/`SKIPPED`」，保证 MANIFEST 本地文件仍能按「网盘归档 + 保留期」正常释放；上传 Job 结果新增 `manifests_skipped` 计数。
- **远端归档目录改为按证券分层**：上传相对路径由 `{文件名}` 改为 `{region}_{market}/{usc}/{文件名}`（最终位置 = `sys_config` 20004 的 `pre_path_key` + 分层路径），例：`Data/Finv/SecuQuoteMon/FTMM/USA_US-NSDQ/NVDA/USA_US-NSDQ_NVDA_Min_FTMM_Mon_202607.mvsv`。**分层规则写在代码而非塞进 20004**：`pre_path_key` 是静态仓库前缀，而 `{region}_{market}/{usc}` 逐文件变化且与文件名 `{region}_{market}_{usc}_...` 同属命名契约（`naming.go`），两者必须同源维护；20004 保持「仓库定位」语义不被结构逻辑污染。新增环境变量 `FINV_QUOTE_DEDUB_UPLOAD_LAYOUT`（`by_security` 默认 / `flat` 回退旧行为）；`region/market` 从证券主数据 `dedup.finv_dedup_usc` 读取，缺失时退回扁平并告警；`upload_target` 登记与实际上传复用同一个 `Options.ArchiveTargetPath` 计算，保证两处一致。
- **过程文档编号规范**：`Docs/DevSpec/FileNamingSpec.md` 第 4 节明确 `yyMM-NNN-` 三位序号在 `Docs/` 下**全局唯一、只增不减、不得重号或复用**，新增/改名需同步更新所有引用；`Docs/DevSpec/DocSpec.md` 第 3 节同步增加强制条款。
- 新增自动校验 `internal/docs/naming_test.go`：扫描 `Docs/` 下过程文档编号，重号时 `go test ./internal/docs`（及 `go test ./...`）直接失败，CI 自动拦截。

- `GET .../DeDup/RunLog/Page` 响应新增 `stage` / `progress_percent` / `progress_detail` / `gmt_heartbeat` 字段；`RunLog.md` 同步说明。
- **归档监控总览保持原设计**：Job 调度表仍只展示调度与手动触发，不内联运行状态；运行状态 / 进度改由独立「端到端链路」页面承载。
- `Docs/API/APIs.md` 与 `Docs/API/README.md` 登记第 85–87 号接口（`Job/Runtime`、`Settings`、`Settings/Window`）；`Docs/RequirementList.md` 新增 10357–10387 共 31 条需求。
- `GET .../Status` 与 `GET .../Job/Runtime` 的 `jobs[]` 新增 `name` / `summary` / `flow` / `details` / `cautions` 字段，前端用于 Job 中文名与悬浮说明；两者同时返回当前月批次进度 `job_progress`（成功 / 失败 / 总数 / 进行中 / 待执行 / 暂停 / 冻结 / 装载中 / 已装载 / 导出中）。
- 服务启动流程（`cmd/server/main.go`）：始终创建归档服务（运维接口可用），仅当 `FINV_QUOTE_DEDUB_ENABLED=true` 时启动进程内调度器（每 30 秒 tick；多实例由数据库行锁 + 租约防重）。
- `internal/api/router.go`：新增 `Deps.ExportArchive` 与 `/Meta/Finv/Quote/DeDup/**` 路由组（服务未注入时不注册）。
- `Docs/DevSpec/DbSchemaSpec.md`：`dedup` schema 归属表登记更新为归档链路 13 张表（新增 `finv_dedup_runtime_setting`）。
- `Docs/RequirementList.md`：新增归档链路需求条目（自 `10291` 起）。
- 默认不修改既有两张业务表，不新增 `export` / `sche` schema，导出/上传文件统一 `.mvsv`（不生成、不支持 `.csv`）。
- **CI/CD 拆分为三种发布包**（`.github/workflows/CI.yml`）：新增 `pr-binaries` / `pr-publish` 与 `release-binaries` / `release-publish`，与既有 `snapshot-binaries` / `snapshot-publish` 同级平行：
  - **快照包（snapshot）**：push 代码到分支（非 `main`，且提交未关联已合并 PR）→ `v{VERSION}.{PPPPP}.{yymmddHHmm}`（prerelease，时间戳保证同一 PR 多次 push 唯一）；
  - **构建包（pr）**：PR 合并 → `v{VERSION}.{PPPPP}`（prerelease；**B 策略**：同名 release/tag 已存在时删除后重建）；
  - **正式版包（release）**：代码合并到 `main` → `v{VERSION}`（正式 release、`--latest`；tag 已存在则跳过，需先 bump `VERSION`）。
  - 三种场景互斥（提交已关联已合并 PR 时只出构建包），`push` 分支范围扩展为 `dev` / `main` / `FinvQuant` / `feat/**`，镜像构建仍仅限 `dev` / `main` / `FinvQuant` 与 `v*` tag。
- `Deploy/Script/PackSnapshot.sh` 支持三种包版本格式（`v{BASE}`、`v{BASE}.{PPPPP}`、`v{BASE}.{PPPPP}.{yymmddHHmm}`）与 `--kind snapshot|pr|release`，包内 `BUILD-INFO.txt` 增加包类型；`workflow_dispatch` 增加 `kind` 输入，便于手动构建指定类型包。
- `Docs/DevSpec/GitSpec.md` §4 补充三种包的版本/tag 与发布规则。
- **包目录调整**：`internal/exportarchive` 迁移到 `internal/meta/finv/quote/dedup`（包名 `dedup`），`cmd/server/main.go`、`internal/api`（handler / router / 测试）与 `Docs`（CHANGELOG、RequirementList、Refactor 落地文档）中的引用同步更新；接口路径、配置项与功能行为不变。
- `Deploy/Docker/Dockerfile` 移除 Go 构建阶段冗余的 `--platform=$TARGETPLATFORM`（BuildKit 默认即目标平台），消除 `RedundantTargetPlatform` 构建检查告警；前端构建阶段的 `$BUILDPLATFORM` 保留（跨平台复用产物所需）。
- **依赖安全升级（Dependabot 24 个告警清零）**：
  - Go：`golang.org/x/crypto` 0.48.0 → **0.52.0**、`golang.org/x/net` 0.51.0 → **0.55.0**、`github.com/quic-go/quic-go` 0.59.0 → **0.59.1**（连带 `x/sys` 0.45.0、`x/text` 0.37.0、`x/sync` 0.20.0）；
  - 前端：`vite` 8.2.0 → **8.3.5**、`vue` 3.5.40 → **3.5.44**（含 `@vue/server-renderer` 3.5.44）、`source-map-js` → **1.2.2**、`nanoid` → **3.3.20**；`package.json` 的 `vite` / `vue` 最低版本同步提升，避免回退；
  - 验证：`go build/vet/test` 全绿、`npm audit` 输出 `found 0 vulnerabilities`、`npm run build` 通过、Docker 镜像重建并实跑健康。

### 修复

- **左侧菜单树渲染为空（需求 10803 修复）**
  - 根因：`NavMenuTree.vue` 模板使用 `<NavMenuTreeNode>` 但未 `import`（项目未启用自定义组件自动注册），编译为运行时 `resolveComponent('NavMenuTreeNode')`；组件未进入 bundle，左侧菜单整体渲染为空。
  - 修复：`NavMenuTree.vue` 显式导入 `NavMenuTreeNode`；递归组件 `NavMenuTreeNode.vue` 显式自导入。
  - 防回归：`Docs/DevSpec/MenuSpec.md` 新增 §1.3（自定义组件必须显式导入 + 构建产物自检命令）。
  - 验证：重建后产物含 `finv-nav-node`、`finv-nav-item`、`finv-nav-indent`，`resolveComponent` 计数 0；容器 healthy，前端 16002 正常。

- **V200057 迁移在 dedup schema 缺失时阻塞启动（需求 10808）**
  - `Deploy/Migrations/V200057__sync_cdeu_config_from_dedup.sql` 在 `DO` 块开头增加 `pg_namespace` 存在性判断：`dedup` schema 不存在（已被冻结/迁移为 `dedup_frozen` 等）时直接 `RETURN` 并输出 NOTICE。
  - 修复前该迁移会在 `SELECT count(*) FROM dedup.cdeu_process` 处报 `42P01`，导致后续 V200058–V200061 全部阻塞、容器重启循环。
  - 注入 dedup 的存量库行为不变（仍按「有运行记录 / 行数不落后」判断跳过或同步）。
  - 验证：本机 Docker（dedup 不存在、cdeu 26 表、cdeu 配置 17 行且 425 条运行记录）重建镜像后迁移一次跑通至 **200061**，容器 `healthy`，`/API/V1/Health/Ready` 返回 `postgres/redis ok`。

- **全仓 Markdown 表格渲染修复与校验范围扩展（需求 10779）**
  - 扫描全仓 `.md`（根目录、`Docs/`、`Deploy/`、`Archive/`），修复 **23 处**表格问题：
    - 单元格内裸 `|` 未转义导致列错位：`ts|da|ti|...`、正则 `(quote|quant)`、shell `||`、`2>&1 | grep`、`3120|14` 等（`RequirementList.md`、`Prompt.md`、`MvsvFileFormat.md`）；
    - `Docs/DataDictMapping/FinvSecurity.md` 表头多出 `finv_security` 表并不存在的 `market_code` 列（与 `V100005` 表结构不符），表头 12 → 11 列；
    - `Archive/.../DevelopmentDocumentIndex.md` 1 处表格缺表头。
  - `internal/docs/table_test.go`：扫描根目录从 `Docs/` 扩展到仓库根（跳过 `.git`/`node_modules`/`Dist`/`pkg`），并新增「分隔行/数据行列数必须等于表头列数」校验。

- **错误码表格缺表头导致文档渲染异常（需求 10778）**
  - `Docs/API/Quote/Cdeu/Min/KLineUpsert.md` 与 `Docs/API/KLine/*.md` 共 **11 处**错误码表格只有数据行，缺少 `| 错误码 | HTTP | 说明 |` 表头与 `|---|` 分隔行，Markdown 渲染为普通文本。
  - 全部补齐表头并清理表格行尾空格；涉及 `KLineAdd`、`KLineBatchDelete`、`KLineBatchInsert`、`KLineBatchPatch`、`KLineBounds`、`KLineCount`、`KLinePage`、`KLinePageUni`、`KLineQuery`、`KLineStats`、`KLineUpsert`。
  - 新增 `internal/docs/table_test.go::TestMarkdownTablesHaveHeader`：扫描 `Docs/**/*.md`（跳过 ``` 代码块），任何表格块无表头/分隔行即失败，防止同类问题回归。

- **部署包漏发一次性运维 SQL 脚本（`Deploy/Script/*.sql`，需求 10776）**
  - 现象：快照包/构建包/正式包只打包 `Deploy/Migrations`、`Deploy/Native`、`Deploy/Docker`，`Deploy/Script/` 下的 `MigratePublicKlineToCdeu.sql` 下载后不存在。
  - `PackSnapshot.sh`：所有平台包新增 `Deploy/Script/`，拷入 `*.sql` 与 `README.md` 并统一 `chmod 0644`。
  - `Install.sh`：安装时把 `Deploy/Script/*.sql` 留存到 `<PREFIX>/Deploy/Script/`（源码目录与目标目录同一布局时自动跳过）。
  - 文档同步：`Deploy/Script/README.md`（随包分发说明）、`Deploy/Native/Install.md`（安装目录清单）、`Docs/FinvQuant/MigratePublicKlineToCdeu.md`（分发路径）。
  - 注意：已发布的旧包不含该文件，需要重新触发一次快照/正式构建并重新下载。

- **历史迁移中未限定的 `DROP TABLE` 加固（需求 10772）**
  - 核查全部 139 个迁移与运行期代码：**不存在** `DROP TABLE` / `TRUNCATE` / `ALTER TABLE` 作用到 `public.finv_quote_secu_kline_min` 的语句（涉及该表的 `DROP` 均带 `export.` / `dedup.` 前缀；运行期 `tableStaging = "export.finv_quote_secu_kline_min"` 亦带前缀）。
  - 唯一同类隐患：`V100015` 重建 `finv_futu_mapping_market_code` 时 `CREATE/INSERT/DROP/ALTER/INDEX/TRIGGER` 全部未限定 schema，`search_path` 首位为 `public` 时 `DROP` 会作用到 public 中的同名表。已全部显式加 `public.` 前缀（该表按设计常驻 public，语义不变）。

- **全新库首次部署：与共享 `public` 同名表冲突、以及 V200xxx 迁移链断裂（需求 10771）**
  - **public 冲突（核心）**：`V100001` 的 3 张表 / 2 个索引 / 触发器改为显式 `quote.`；`V100021`、`V100029`、`V100030`、`V100031` 同步限定；`V100032` 搬表增加「目标 schema 已有则跳过」，收敛断言改为白名单 + 目标 schema 存在性（`public` 中其它服务的同名表只 `NOTICE`）。**全程不对 `public` 中他人表执行任何 DDL/DML**。
  - **`V100080`**：前置校验由「dedup 必须 26 张」改为「≥26 且 cdeu=26」；无对应目标表的源表跳过；复制列清单改为「源 ∩ 目标」交集（修复 `workflow_run_id` 不存在）。
  - **`V200039`**：库内无 GCMain 行情（全新库）时跳过清理校验，避免误报「备份表没有 GCMain 月K记录」。
  - **`V200048` / `V200050`**：去掉硬编码 `phase_id = 12/4/3`，改为按 `phase_code` 解析（全新库中 PHASE_REVIEW 的 id 由序列分配，不等于 12）。
  - **`V200057`（新增）**：把 dedup 侧 CDEU 配置（phase/process/job/job_step）同步到 cdeu，修复「迁移按版本号执行 → cdeu 配置落后于 dedup」的结构/数据漂移；只在 cdeu 无运行记录且配置落后时执行，升级库自动跳过。
  - **验证**：临时 PG18 中预置他人 `public.finv_quote_secu_kline_min`（含注释/索引/3 行数据）后执行全部 139 个迁移——0 错误；`public` 表数据 md5 与 schema dump 前后**完全一致**；`quote` 三表就位；`dedup` 与 `cdeu` 四张配置表内容逐表 md5 一致；空库场景同样 0 错误。
  - **安装脚本**：移除此前误加的 `--recreate-db`（会 `dropdb`），非空旧库只告警并给出「换库名/换实例」指引，**不删除、不改名、不迁移任何已有表**。

- **安装完成摘要输出显示字面 `\033[1;32m`（需求 10769 修复）**
  - 摘要块原用 `cat <<EOF` 输出，未解释 ANSI 转义；改为 `printf` 逐行输出，终端颜色正常渲染。

- **已存在但未被本程序初始化的数据库缺少启动告警（需求 10766 修复）**
  - `Install.sh` 第 8 步：当目标库已存在、库内有表却缺 `public.schema_version` 时，明确提示“迁移将从 V100001 起执行并可能撞已有表（`relation ... already exists`）”，并给出重建库命令与“先备份、勿手工插 `schema_version`”的指引。

- **Native 安装脚本无法创建用户、Redis 误报未启动（需求 10766 修复）**
  - **`useradd` 失败**：脚本先 `groupadd --system finvquant`，随后 `useradd --system ...` 未指定主组；Debian/Ubuntu 的 `useradd` 默认会再建同名组，遇到已存在的组直接报错退出（`group 'finvquant' exists - ... use -g`），导致用户/目录/建库/systemd 全未执行。改为 `useradd --system --gid finvquant ...`；已存在用户时用 `usermod -g` 补正主组（手工调建组场景）。
  - **Redis 选错 unit**：原逻辑优先 `redis.service`，而它在 Ubuntu 上只是 `redis-server.service` 的 alias，alias 不能 `enable`，`systemctl enable --now redis` 整体失败 → 仅打 WARN、Redis 实际未启动。改为**优先 `redis-server`**（不存在才用 `redis`），`enable` / `start` 分开执行，用 `systemctl is-active` 判定真实状态，未运行时自动打印最近 15 行 journal。
  - **文档**：`Deploy/Native/Install.md` 手工安装步骤的 `useradd` 同步补 `--gid`。

- **CDEU 概览不再因「最近一次运行没处理实例」而显示 0 个 JOB**（修正展示缺陷）：
  - **根因**：`cdeu_job_dynamic` 是**跨运行累积的工作项**（每个 `usc+biz_month` 只有一行，状态随每次运行更新，无历史快照），但 `attachCDEURun` 按 `workflow_run_id == 当前展示 run` 过滤实例。当最新一次运行恰好没有处理任何实例时（如预算耗尽那一轮），整棵树显示 0 个 JOB，看起来像「实例没生成」——实际库里 BASE 89 / INCR 12 / UPLOAD 317 条都在。
  - **修正**：`attachCDEURun` 不再按 run 过滤动态实例，改为保留全部并标记 `from_current_run`（是否属于当前展示的这次运行）；`CDEUJobView` 新增 `from_current_run` 字段。前端 `processJobText` 改为「`x/y JOB · N 实例（成功 k）`」，节点悬浮提示列出前 8 条实例明细（`usc 月份 类型 [状态]`）。
  - **实测**：概览接口 `PROCESS_EXPORT_FILE_MON_BASE` 实例=89（成功 89）、`PROCESS_EXPORT_FILE_MON_INCR` 实例=12、`PROCESS_UPLOAD` 实例=317；切到历史 run 49 同样可见（本次 run 内 89/12/202，其余为在办存量）。

- **预算与低峰窗口只约束自动调度，手动触发不再受限**（修正设计缺陷）：
  - **背景**：`RUN_DUE_EXPORT_JOBS` 原实现无差别检查低峰窗口与当日预算，导致「手动触发」也被卡住——日预算是按「近 30 天入口流量 × 因子」算出的**增量保护**（`finv_dedup_ingest_stats` 为空时兜底仅 20 万行），历史全量重导（上百个月、数百万行）根本塞不进去，实测手动触发只跑了 12 个月就被预算耗尽挡停。
  - **修正**：`Result` 增加 `Trigger`（`CDEUTriggerManual`/`CDEUTriggerSchedule`），由 `runWithTriggerRunID` 填入；同时通过 context 传递（`withTrigger`/`TriggerFromContext`/`IsManualTriggerFromContext`）下沉到 LOAD 分页循环。`job/export.go` 的窗口与预算判断、`job_export.go` 的 `loadDate` 窗口判断均改为 **仅对调度触发生效**；手动触发仍写入 `finv_dedup_daily_budget.consumed_rows`（保留审计与 Auto 模式基线），进度详情标注「手动触发：不限制」。
  - **实测**：把当日预算人为置为 `remaining=0`，手动触发后 60 秒内 `EXPORTED` 由 12 → 45、本地文件 12 → 90 个，运行详情 `已处理 33/200 个证券月`，未因预算停下。
  - 文档：`Docs/API/QuoteDeDup/JobRun.md` 新增「手动触发与调度触发的约束差异」对照表。

- **数据库表保护红线入规范（防止擅自删表/迁表/合并同名表）**：
  - `Docs/DevSpec/DbSchemaSpec.md` 新增 **§0「表的存在性与归属：不可擅自变更（红线）」**，优先级高于全文其它章节，位于 §1 之前。内容包括：绝对禁止操作清单（`DROP TABLE`/`DROP SCHEMA`/`SET SCHEMA`/`RENAME`/`TRUNCATE`/无条件 `DELETE`/合并或废弃同名表）、**同名表保护名单**（`quote.`/`dedup.`/`export.` 三份 `finv_quote_secu_kline_min` 必须并存、禁止合并，并说明消费者/加工者分离的设计理由）、授权判定标准（仅「用户明确点名」或「`RequirementList` 有条目」算授权，沉默/推断/旧清单均不算）、文档冲突仲裁顺序（本规范 §0/§1 → `RequirementList` → `CHANGELOG` → `Refactor` 旧清单）、改库前 6 问自检清单、删除数据的强制要求（整行备份到 `purge` 表 + `reason`/`backup_at` + 单事务 + `RAISE EXCEPTION` 校验）、以及 `V38` 事故记录（`SET SCHEMA` 致 `quote` 副本消失、同日已验收的改造清单未作废）作为反例。
  - `Docs/Refactor/2610-003-MoveTablesToSchemas.md` 加**时效声明横幅**，并把受影响的 `C-10`/`C-11`/`C-16`/`C-18`/`C-20` 五条状态改为「⚠ 作废」（保留原文，按该文件「只增不改、作废标注」规则处理），避免后来者把 2026-10-24 的旧验收标记当作当前事实。
  - 仓库级 `AGENTS.md`「工作约定」首条新增数据库红线，明确「同名表通常是刻意的隔离设计，不是冗余」，并指向 `DbSchemaSpec.md` §0。

- **分钟行情双 schema 读写分工落地**：`internal/quote`（`Import/Upload` 文件导入 + `History/QuoteQuery` 查询）与 `internal/backtest` 全部改为读写 `quote.finv_quote_secu_kline_min`；`internal/kline`（`DeDup/Secu/KLine/Min/{Add,Batch,Upsert,Patch,BatchDelete}` 等 K 线 API）与去重/导出/上传链路继续读写 `dedup.finv_quote_secu_kline_min`。回测/查询属消费侧、K 线 API 与归档属加工侧，靠 schema 前缀强制隔离。实测验证：K 线 API `/Add` 写入后 `dedup=1 / quote=0`，`History/QuoteQuery` 查同一 `usc` 返回 `total=0`。相关单测断言与 `Docs/API/HistoryQuote/ImportsUpload.md`、`Docs/API/Backtest/RunCreate.md`、`Docs/DevSpec/DbSchemaSpec.md` 已同步。

- **恢复 `quote.finv_quote_secu_kline_min`（回测隔离副本）**（迁移 `V76`）：同名表采用**双 schema 隔离设计**——`quote.` 供量化回测只读（SQL 必须写 `quote.` 前缀，避免漏写前缀命中 `search_path` 中另一张表）、`dedup.` 供去重/导出/上传链路、`export.` 为导出中转快照。`V38`（2026-10-24）曾把 `quote.` 副本 `SET SCHEMA dedup` 迁走，使回测侧隔离手段失效；本迁移按原设计恢复：结构/主键 `(ts, usc)`/5 个 CHECK/3 个业务索引/`gmt_update` 触发器/表注释全部与 `dedup` 对齐，并一次性从 `dedup` 回填 **2,804,256 行**（实测耗时约 4 分钟）。同时把启动迁移超时从 30 秒放宽到 10 分钟（`cmd/server/main.go`），避免大表回填类迁移超时导致启动失败；`schema_qualify_test.go` 的 `exactTableSchema` 已把 `quote.` 加入白名单。**两表此后各自演进，不再自动同步**。

- **清理 GCMain 误插入的月K线数据**（迁移 `V100039`）：GCMain 分钟表里混入了历史月K线（时间戳落在每月 1 号 `00:00:00`，字段值是整月 OHLC 与成交量），与正常分钟数据相差 4~5 个数量级，会污染 MVsv 导出。判定口径：`time=0` + `date%100=1` + `volume >= 100000` + `high-low >= 20`（三者同时满足）；共清理 **73 行**（`GCMain` 2,729,329 → 2,729,256），同位置 4 条正常分钟数据（`20260401/20260501/20260601/20260701`，volume 14~45）已保留。删除前整行备份到 `dedup.finv_quote_secu_kline_min_kline_purge`（含 `reason` 与 `backup_at`，主键 `(usc, ts)`），可回滚、可审计。**同表的日K线（`time=0` 且非 1 号，约 1326 行）与 `XAUUSD` 的 44 行 `time=0` 记录本次未处理**，待业务确认后另行清理。

- **证券主数据 `currency` 刷新缺陷修复**（迁移 `V100038`）：`RefreshSecurityMetadata` 原先对 `dedup.finv_dedup_usc.currency` 采用「只补 NULL」策略，主数据（`finv_security.currency_type`）修正后 usc 长期停留在旧币种，会把错值写进 MVsv 文件头 `# Currency` 行。现改为「以主数据为准覆盖」（新值为 NULL 时保留旧值），与 region / market 同策略；`name` / `timezone` 仍只补缺，避免覆盖人工维护的名称与时区别名。已重刷数据：`81810`（小米集团-WR 人民币柜台）`HKD → CNY`，全库 `CNY 25 / HKD 9 / USD 5`，无空值、与主数据零偏差。

- 修复动态导出 JOB 勾选「自动续跑」后只执行静态后续 JOB、未继续同 Process 后续动态 JOB 的问题：现在先按 `dynamic_job_id` 顺序续跑后续动态导出 JOB，再执行补丁、上传、清理、健康检查等静态后续 JOB；日志弹窗增加自动续跑状态提示，JOB 清单在续跑期间使用 1 秒刷新。
- 修复 CDEU JOB 清单「处理量」列为空：动态导出 JOB 从运行日志读取行数/字节数，动态上传 JOB 从 `finv_dedup_export_file` 读取文件行数/大小；静态 JOB 在 CDEU 运行记录为 0 时回退运行日志。
- CDEU Process / JOB 清单「耗时」列在不超过 10 秒时改为毫秒精度展示（不足 1 秒为 `xxxms`，1–10 秒为 `x.xxxs`），超过 10 秒继续按秒 / 分 / 小时展示。
- 修正 `internal/meta/finv/quote/dedup` 集成测试迁移目录路径（`../../Deploy/Migrations` → 上溯至仓库根），使 `TestStoreIntegration` / `TestServiceIntegration` 可按文档命令实际运行。
- 修正文档编号重复：`Docs/Refactor/2610-006-CDEUProcessModelDesign.md` 与既有 `2610-006-RenameDatabaseAndRole.md` 重号，已重命名为 **`Docs/Refactor/2610-007-CDEUProcessModelDesign.md`**，并同步更新 `CHANGELOG` 与 `RequirementList` 引用。

## [0.1.2] - 2026-10-24

### 新增

- **分钟行情 K 线 API 融合（FinvQuoteDataDeDup → FinvQuant）**：新增 `internal/kline`（领域服务/校验/格式化/错误目录）与 `internal/api/handler/kline*.go`，在 16001 进程内实现原 Python 服务的 8 类业务接口（`Add`、`Batch`、`Upsert`、`Page`、`PageUni`、`Query`、`PATCH Batch`、`DELETE Batch`、`Count`），统一前缀 **`/API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min`**；旧前缀 `/API/V1/Meta/Finv/Quote/Secu/KLine/Min` 作为切流期过渡别名同时注册。
- 新增运维端点 **`GET /Health`**（探数据库，替代旧 `/healthz`，`/healthz` 保留为过渡别名）与 `GET /` 服务标识（返回 `service/prefix/docs/version`）。
- 新增迁移 **`V100038__finv_quote_kline_move_to_dedup.sql`**：分钟行情主表 `quote.finv_quote_secu_kline_min` 迁入 `dedup` schema（`SET SCHEMA`，不重写数据），补齐 `change_price`/`change_ratio`/`provider` 三列，并按 `dedup` 新结构重建 `export.finv_quote_secu_kline_min` 转存表（含触发器与收敛校验）。
- 新增配置项：`FINV_KLINE_MAX_BATCH_ROWS`（默认 5000）、`FINV_KLINE_MAX_LIST_SIZE`（默认 5000）、`FINV_KLINE_TS_UNIT`（`s`/`ms`）、`FINV_ERROR_DETAIL_MODE`（`safe`/`admin`）、`FINV_ERROR_DEBUG_TOKEN`、`FINV_ERROR_DETAIL_ALLOWED_CIDRS`。
- 新增 10 篇接口文档（`Docs/API/KLine/*.md`）并在 `Docs/API/README.md`、`Docs/API/APIs.md` 登记；`Docs/DevSpec/ErrorCodeSpec.md` 增补 `21xxxx` 号段与复用规则；`Docs/DevSpec/ApiSpec.md` 登记兼容例外。
- **数据采集前端菜单**：新增一级菜单「数据采集」→ 二级菜单「行情数据去重」→ 两个三级子菜单：「去重分钟行情查询」（按 `usc` + 交易日期范围分页查询 `dedup.finv_quote_secu_kline_min`）与「去重分钟行情统计」（汇总指标、字段完整性、每日趋势与证券分布可视化）。
- 新增迁移 **`V100040__finv_quote_kline_date_indexes.sql`**：为 `dedup.finv_quote_secu_kline_min` 增加 `(usc, date, ts)` 与 `(date, usc, ts)` 复合索引，支撑「证券 + 交易日期范围」分页查询与按日聚合（此前仅 `(usc, ts)`，日期过滤需顺扫历史 2.8M 行，单次分页可达 8~17 秒）。
- 新增接口 **`GET /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Stats`**：返回汇总统计（行数/证券数/日期范围/成交量与成交额合计/收盘价区间与均值/各列空值行数）、按日期聚合趋势、证券记录数 Top 20；文档见 `Docs/API/KLine/KLineStats.md`。

### 变更

- **错误码口径（DC-05）**：分钟行情 API 使用 `211xxx` 具体错误码（`211001`~`211303`，15 条，编号/HTTP/安全文案见 `internal/kline/apierr.go` 与接口文档）；其余/兜底错误复用原 HTTP 错误码（`400/404/409/413/422/503`，`500` 沿用 `2006`）。
- **破坏性变更（接口）**：存量接口旧业务号 `4001/4004/4009` 停用，分别收敛为 `400`/`404`/`211104`（前端仅判断 `code !== 0`，不受影响；调用方如依赖具体码需同步调整）。
- **错误详情分级（DC-06）**：错误响应默认只返回安全文案；`FINV_ERROR_DETAIL_MODE=admin` 且携带有效 `X-Debug-Token` 或来源 IP 命中内网 CIDR 时，才附加白名单 `debug` 字段；DSN、完整 SQL、堆栈、令牌、内部主机名永不进入响应；服务端日志始终记录全量并可用 `request_id` 关联。
- `request_id` 统一为**大写 UUID4**（DC-07），与 `internal/backtest` 现有写法一致。
- 分钟行情表归属调整为 `dedup.finv_quote_secu_kline_min`；`internal/quote`、`internal/backtest`、`Docs/API/HistoryQuote/*`、`Docs/DevSpec/DbSchemaSpec.md` 等引用同步改为 `dedup.`；`TestSQLSchemaQualified` 对该表强制 `dedup.`/`export.` 前缀。
- **去重行情页面数字展示**：默认取消千分位分隔符，查询表格数值与总数、统计 KPI/图表提示均输出纯数字文本；大额统计按 `万/亿/万亿` 缩写展示（不再叠加千分位）。
- **去重行情默认查询条件**：证券代码默认 `GCMain`；开始/结束日期默认最近一个月（以库内该证券最新交易日期为结束日回推 1 个月，若取不到则回退本机当天），不再默认缺省。
- **侧边导航缩进优化**：一级→二级、二级→三级每级缩进在上一版 28px 基础上再减半至 **14px**（计算样式：二级 22px、三级 36px），三级菜单标题完整展示且留白充足；菜单图标与标题的水平间距由 32px 调整为 **约 1/3（10.67px）**；样式在 `App.vue` 内全局生效。
- **侧边导航顺序**：「数据采集」菜单移动到「历史行情」之前，紧跟仪表盘。
- **前端入口缓存**：`internal/static` 对 `index.html` 与 SPA 回退统一返回 `Cache-Control: no-cache, no-store, must-revalidate`（附 `Pragma/Expires`），避免前端重建后浏览器继续使用旧 `index.html` 引用旧 JS；同步刷新 `internal/webui/dist` 内嵌产物，保证非 Docker 构建也使用最新前端。
- **去重行情统计页成交额图表**：在「每日成交量」之后新增「每日成交额」独立图表，与「每日记录数」「每日成交量」「每日收盘价均值」共用时间轴与绘图区网格（四图纵向排列、时间轴对齐）。
- 新增轻量接口 **`GET /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Bounds`**：返回过滤范围 `date_from/date_to`，供前端初始化默认日期范围；实测 `usc=GCMain` 约 17ms（原用 `/Stats` 全量聚合约 8s）。
- **去重行情统计页布局**：「每日记录数与成交量」拆分为「每日记录数」「每日成交量」两个独立图表，与「每日收盘价均值」共三个图表；三者使用同一绘图区网格与时间轴刻度配置，横向完全对齐；「字段完整性（空值行数）」独立一行；图表坐标轴显式关闭千分位（ECharts 默认格式同步覆盖）。
- **时间展示修正**：`time=0` 按合法值展示为 `00:00:00`，仅 `null/undefined` 展示为 `-`（全局 `fmtTime` 同步修正）。
- 分钟行情分页/查询排序调整为 `usc, date, ts`（与新增日期索引一致，`date` 与 `ts` 单调对应，结果顺序对调用方等价）。
- 前端菜单层级调整（未发布版内）：原「去重行情」一级菜单更名为「数据采集」，其下新增二级菜单「行情数据去重」，去重查询/统计调整为该二级菜单下的三级子菜单（`Docs/RequirementList.md` 10276）。
- 接口错误码变更、表结构变更与文档索引更新同批提交（参见 `Docs/RequirementList.md` 10241 起）。

### 变更（涨跌字段补全规则）

- **MVSV 导入**：`cp`/`cr` 列由“解析后忽略”改为**有值时落库**；两列均缺失时由服务端自动补全——参考价 `last_close`（非 0）优先，缺失/为 0 时回退 `open`（非 0），两者都不可用则保持 `NULL`；计算式 `change_price = close - ref`、`change_ratio = (close - ref) / ref * 100`（百分比，6 位小数，`big.Rat` 精确运算）。
- **分钟行情 KLine API**：`/Add`、`/Batch`、`/Upsert` 在请求未提供 `change_price`/`change_ratio`（两列都为空）时按同一规则补全；API 不写 `last_close`，因此参考价回退 `open`。**任一列已有值时不做修改。**
- 导入/写入链路补全不改变 `FIELD`/`ROW` 覆盖语义：`FIELD` 模式下补全值为 NULL 时保留库内旧值，`ROW` 模式仍整行覆盖。

### 修复

- **分钟行情按日期范围分页性能**：新增日期索引并调整排序后，`GCMain + 最近一个月` 分页查询由约 7 秒降至约 0.03 秒，消除 503/超时。
- **修复去重行情页面报错 `OK` 且无数据**：前端 `apiRequest` 原先只接受 `code=0`，而分钟行情 KLine API 成功码为 `1`，导致 `/Page`、`/Stats` 被误判为失败；现按 `ApiSpec` 统一接受成功码 `{0,1,200,202}`。
- 修正分钟行情 `ts` 精度口径：库内与接口契约均为**秒级**（原 Python 代码/文档中的「毫秒」为文档缺陷）；13 位毫秒默认拒绝（`211004`），仅 `FINV_KLINE_TS_UNIT=ms` 时换算。
- `/Page`、`/PageUni` 的 `change_ratio` 统一按 `ROUND_HALF_UP` 输出两位小数文本；`/PageUni` 数值去尾零、`-0` 归一为 `0`。

## [0.1.1] - 2026-10-24

### 新增

- 新增本变更日志 `Docs/CHANGELOG.md`，汇总 0.1.0 及历史 VeritasQuant 版本的对外变更。
- 开发规范新增「变更日志」强制要求：[`DevSpec/DocSpec.md`](DevSpec/DocSpec.md) 第 4 节、[`DevSpec/GitSpec.md`](DevSpec/GitSpec.md) 第 3 节，并在 [`DevSpec/FileNamingSpec.md`](DevSpec/FileNamingSpec.md) 中明确本文件名全大写。
- 新增需求清单 [`RequirementList.md`](RequirementList.md)，采用五位数编号（自 `10001` 起），记录 2026-10-24 起的新增需求，首版 65 条；`AGENTS.md` 工作约定同步要求新增需求必须登记。
- 新增两份改造清单：改造项 A [`Refactor/2610-001-SecuCodeToUsc.md`](Refactor/2610-001-SecuCodeToUsc.md)（`secu_code` → `usc` 全库统一，`A-01`～`A-37`）、改造项 B [`Refactor/2610-002-PrevCloseToLastClose.md`](Refactor/2610-002-PrevCloseToLastClose.md)（`prev_close` → `last_close` 重命名 + `open` 兜底回填，`B-01`～`B-24`），共 61 个可勾选检查项与 4 项待决策；B 的删除动作暂缓，留待生产数据核实后另议。

### 变更

- 统一跨平台换行策略：新增根目录 [`.gitattributes`](../.gitattributes)，所有文本文件以 LF 存储并检出，`*.cmd` / `*.bat` / `*.ps1` 保留 CRLF；规范见 [`DevSpec/FileEncodingSpec.md`](DevSpec/FileEncodingSpec.md) 第 4 节。
- 按上述规则重规范化全库行尾，消除 Windows / Linux 开发环境间的纯行尾差异。
- **破坏性变更（数据库）**：证券代码列 `secu_code` 统一正名为 `usc`，覆盖 6 张表（`finv_quote_secu_kline_min`、`finv_quote_ingest_batches`、`finv_quote_revision_log`、`finv_quant_backtest_strategy`、`finv_quant_backtest_run`、`finv_quant_backtest_run_archive`），5 个依赖索引同步改名；由迁移 `V29` 幂等执行，主键 `finv_quote_secu_kline_min_pkey` 名称不变、不重建。
- **破坏性变更（接口）**：接口与表单中的证券代码字段统一为 `usc`，查询参数由 `secuCode` 改为 `usc`，不再接受旧名（`Quote/History/QuoteQuery`、`Imports/Upload`、`Backtest/Run/Create|List`、`Strategy/Save|List` 等）；`secu_name` 等其余字段不受影响。
- **升级要求**：新旧镜像不可共存（旧版本无法读写新列名），必须先停旧容器再启动新版本，让服务启动时自动执行 `V29`；升级前请按备份流程做一次 `pg_dump`。
- **破坏性变更（数据库）**：`finv_quote_secu_kline_min.prev_close` 重命名为 `last_close`（迁移 `V30`），口径为数据源的「前收盘价」，与 `open` 独立；涨跌额/涨跌幅仍按 `close - last_close` 计算，**计算逻辑与接口字段不变**，前端零改动。
- **数据修复**：`open` 为 `NULL` 或 `0` 且 `last_close` 有效时，用 `last_close` 兜底回填（迁移 `V31`），并写入 `finv_quote_revision_log`（`ingest_batch_id = MIGRATION_V31`）留痕；新导入数据由导入链路按同一规则实时兜底，不再产生缺 `open` 的行。
- **升级提示**：该回填**不可逆**（覆盖了原始缺失状态），升级前务必完成 `pg_dump` 备份；如需还原只能从备份恢复。
- **破坏性变更（数据库）**：表按业务域迁入独立 schema —— `finv_quote_*`（3 张）→ `quote`，`finv_quant_*`（13 张）→ `quant`；字典表与 `schema_version` 保留 `public`。由迁移 `V32` 幂等执行，只改对象归属、不改表名/列/数据，索引与触发器自动跟随。
- **连接要求**：应用连接串固定携带 `search_path=public,quote,quant`（`internal/database/postgres.go`）；手工 `psql` 未设置 `search_path` 时须使用全限定名（如 `quote.finv_quote_secu_kline_min`）。
- **全新库兼容修复**：历史种子迁移 `V100019`（INSERT 列名）与 `V100023`（建表列定义）已把 `secu_code` 改为 `usc`——这两支在 `V29` 之后执行，不改会导致全新库首次部署失败；已应用库不受影响（`schema_version` 跳过）。新增 `V100024` 把 `V100023` 在 `public` 新建的 3 张表移入 `quant`。
- 新增规范 [`DevSpec/DbSchemaSpec.md`](DevSpec/DbSchemaSpec.md)：schema 划分、表命名、`search_path` 约定、迁移例外规则。
- **破坏性变更（数据库）**：富途映射表 `finv_futu_mapping_*`（5 张：`cs_market`、`exchange`、`market_code`、`region`、`security`，共 641 行）迁入 `futu` schema，由迁移 `V33` 幂等执行；索引与触发器自动跟随，无表名/列/数据变化。
- **连接要求更新**：`search_path` 扩展为 `public,quote,quant,futu`（应用连接串与数据库默认值同步）；Go 代码对本组表无引用，无需改后端 SQL。
- **防回归校验**：新增 `TestSQLSchemaQualified`（`internal/database/schema_qualify_test.go`），扫描 `internal/`、`cmd/` 全部 Go 源码，要求涉及 `finv_quote_*` / `finv_quant_*` / `finv_futu_*` 的 CRUD 语句必须带 schema 全限定名（`quote.` / `quant.` / `futu.`）；漏写会让 `go test ./...` 失败，为后续系统融合的同名表场景兜底。
- 规范补充：`DbSchemaSpec.md` 新增「融合前的同名表风险」——`public` 在 `search_path` 首位，新增同名表必须先移出首位并核验历史种子迁移。
- **部署目录重组**：`Deploy/` 按部署方式拆分为 `Deploy/Docker/`（容器方式：`docker-compose.yml`、`Dockerfile`、`.env.example`、`deploy/upgrade/rollback` 脚本、三份 Windows Docker 文档）与 `Deploy/Native/`（宿主直装方式，当前仅说明文档）；两种方式共用的 `Migrations/`、`Backup/` 保留在 `Deploy/` 根下。
- **目录命名**：备份目录由 `Deploy/backup/` 调整为 `Deploy/Backup/`（与 PascalCase 目录命名一致）；升级脚本、文档与 `.gitignore` 同步更新。
- **构建路径调整**：`Dockerfile` 迁入 `Deploy/Docker/`，compose 的 `build.context` 改为 `../..`、`dockerfile` 为 `Deploy/Docker/Dockerfile`；CI workflow 的 `file` 同步更新。
- 新增 [`Deploy/Native/README.md`](../Deploy/Native/README.md)：明确 Native 方式的**适用场景与不适用场景**、采用前自检清单、与容器方式的责任边界，避免误用。
- **Native 部署配套落地**：新增 `Deploy/Native/{README,Install,Upgrade}.md` 与 `Install.sh`、`Upgrade.sh`、`Uninstall.sh`、`FinvQuant.service`、`FinvQuant.env.example`、`Nginx.conf.example`；服务由 systemd 守护（`Restart=on-failure`、`EnvironmentFile`、安全加固、journald），安装脚本幂等并自动生成随机数据库密码、建库授权、健康检查；升级脚本按「备份 → 停服 → 备份旧二进制 → 替换 → 启动 → 健康检查」执行，支持 `--rollback` 回退程序。
- **文件名按品牌大小写**：`FinvQuant.service`、`FinvQuant.env.example`（安装后注册为 `finvquant.service`，运维命令仍用 `systemctl ... finvquant`）。
- 新增 [`Deploy/README.md`](../Deploy/README.md)：部署目录总览——两种方式的目录边界与选择建议、共用 `Migrations/` 与 `Backup/` 的说明、两种方式共同遵守的约束（禁止新旧共存、升级前备份、schema 限定）、目录命名约定。
- **`VeritasQuant/` 子项目整体移入 `Archive/VeritasQuant/`**（`git mv`，保留提交历史，590 个文件 / 6.4 MB）：该 Python 量化平台为 FinvQuant 前身，现已不参与构建、测试与部署；其 `.github/workflows/` 从未生效（GitHub 只识别仓库根），保留仅为追溯。同步更新根 `README.md`、`AGENTS.md`、`Prompt.md` 的目录结构与路径、`.dockerignore`（改为整体排除 `Archive/`）、`Archive/README.md`（新增归档登记表）、`Docs/API/HistoryQuote/ImportsUpload.md` 示例命令、`internal/mvsv/parser.go` 注释与 `Docs/CHANGELOG.md` 历史章节的链接。
- **快照产物命名改用连字符**：`FinvQuant-v0.1.1.00000.2610240325-ubuntu-amd64.tar.gz`（原 `FinvQuant_v0.1.1.00000.2610240325_ubuntu_amd64.tar.gz`），包内顶层目录名同步；快照 release 标题改为 `{快照版本}-FinvQuant`（如 `v0.1.1.00000.2610240325-FinvQuant`），git tag 仍为 `{快照版本}`；release 说明删除「产物」列表整节与「BUILD-INFO 时间戳一致」「SHA256SUMS 校验方式」两句冗余描述（附件已由 release assets 列表呈现，正文无需重复）。
- **`BUILD-INFO.txt` 补全 CI 溯源信息**：CI 构建时写入运行号（`CI run <id>`）、仓库地址与流水线链接（经 `CI_RUN_ID`/`BUILD_REPO`/`BUILD_RUN_URL` 环境变量传入），本地构建时这几行自动省略。
- **快照产物命名规范调整**（`PackSnapshot.sh` / CI 同步）：产物目录 `dist/` → **`Dist/`**；包名前缀 `finvquant_` → **`FinvQuant_`**；平台段 `ubuntu2604` → **`ubuntu`**、`win11` → **`windows`**。新命名示例：`FinvQuant_v0.1.1.00123.2610241530_ubuntu_arm64.tar.gz`。本次仅改命名与输出目录，包内容与打包流程不变。
- **新增 [`Deploy/Script/README.md`](../Deploy/Script/README.md)（打包脚本文档）**：文件职责与「改打包只改 .sh」的维护约定、快速开始、全部参数表（含默认值）、版本三段来源与一致性保证、两种包的目录树与 `SHA256SUMS` 用法、环境要求、native/WSL/Git Bash 三环境差异与 zip 后端回退链、6 个常用配方、产物校验命令（架构字节/时间戳一致性/zip 正斜杠）、9 条常见问题、与 CI 三个 job 的调用关系；`Deploy/README.md` §2.3 相应精简为概览并链接该文档。
- **新增 `Deploy/Script/PackSnapshot.cmd`（Windows 封装）**：仅负责定位 Git for Windows 的 `bash.exe` 并转调 `PackSnapshot.sh`，打包逻辑仍只有一份。
  - 定位顺序：`git.exe` 位置推导 `<GitRoot>\bin\bash.exe` → Git 默认安装目录 → PATH；**主动跳过 `C:\Windows\System32\bash.exe`**（WSL 转发器，无法处理 Windows 路径）
  - 找不到 bash 时给出明确指引（装 Git for Windows / MSYS2，或改用 WSL）
  - `PackSnapshot.sh` 同步增强：识别 native / WSL / Git Bash 三种环境（WSL 用 `WSLENV` + `wslpath`，Git Bash 用 `cygpath`）；zip 打包按 `zip` → Windows 自带 `tar.exe` → `powershell.exe` → `python3` 依次回退（PowerShell 的 `Compress-Archive` 会写成反斜杠路径，故排在 bsdtar 之后）
- **新增 `Deploy/Script/PackSnapshot.sh`（快照打包的唯一实现）**：CI 的 `snapshot-binaries` 与本地手工出包调用同一脚本，避免两套逻辑漂移。
  - 用法：`bash Deploy/Script/PackSnapshot.sh [--version vX.Y.Z.PPPPP.yymmddHHmm] [--target linux/amd64|linux/arm64|windows/amd64|all] [--pr N] [--timestamp yymmddHHmm] [--build-web] [--keep-tmp]`
  - 默认 `--target all`，一次产出 Ubuntu 26.04（amd64/arm64）与 Windows 11（amd64）三个包到 `dist/`，附 `.sha256`
  - 版本三段由脚本统一解析/补零；同一快照多产物共用同一时间戳（可直接传 `--version` 复现 CI 产物）
  - WSL 兼容：自动探测 `go`/`go.exe`，Windows 版 Go 经 `WSLENV` 传递 `GOOS`/`GOARCH`，输出路径经 `wslpath` 转换
  - CI 侧改为单步 `BuildAndPack` 调用该脚本，删除原先内联的 `go build` 与打包脚本（减少约 90 行重复逻辑）
- **CI 增量支持 `feat/integration` 快照构建**（不改变既有镜像规则）：
  - 触发：`push` 到 `feat/integration`（并加入 PR 目标分支以便验证）
  - 版本标识：`v{VERSION}.{PPPPP}.{yymmddHHmm}` —— `PPPPP` 为关联 PR 号（5 位左补零，无 PR 为 `00000`），`yymmddHHmm` 为 Asia/Shanghai 构建时间戳；**由 `snapshot-meta` job 只算一次**，经 job outputs 传给所有产物，保证同一快照不同产物时间戳一致
  - 产物：Ubuntu 26.04 二进制包 `tar.gz`（`linux/amd64` + `linux/arm64`）、Windows 11 二进制包 `zip`（`windows/amd64`）；包内含二进制、`Deploy/Migrations`、部署脚本与文档、`VERSION`、`BUILD-INFO.txt`、`SHA256SUMS`（Linux 包带 Native 全套，Windows 包带 Docker 编排且不含 Dockerfile）
  - 发布：以快照版本为 git tag 建 **prerelease**，附三个包与 `.sha256`
  - 隔离：`docker` job 显式排除 `feat/integration`，集成分支不构建镜像、不污染 `latest`
  - 复用：`build-web` 上传 `web-dist` artifact 供二进制打包复用，避免二次 `npm ci`
  - 规范：`GitSpec.md` 新增第 4 节「版本号与构建标识」
- **新增 `archive` schema（冷数据归档域）**（`V37`）：幂等建 schema、以 `current_user` 设定属主并授予 `USAGE`/`CREATE`、追加到 `search_path` 末尾（`public,quote,quant,futu,export,sche,dedup,archive`）、末尾收敛校验；生产库已应用并验证（`V37 success=t`、属主 `finv`、权限齐备）。
  - 注意区分：`archive` **schema** 是数据库冷数据归档（历史行情下沉），仓库根 `Archive/` 是**文件**归档（废弃脚本/文档），两者无关。
  - 风险提示：归档表沿用业务表名会形成 `quote` / `export` / `archive` 三处同名，任何读写必须写 `archive.` 前缀。
- **新增 `dedup` schema（去重域）**（`V36`）：幂等建 schema、以 `current_user` 设定属主并授予 `USAGE`/`CREATE`、追加到 `search_path` 末尾（`public,quote,quant,futu,export,sche,dedup`）、末尾收敛校验；生产库已应用并验证（`V36 success=t`、属主 `finv`、权限齐备）。
  - 适用边界已在 `DbSchemaSpec.md` 写明：跨批次幂等由 `quote.finv_quote_secu_kline_min` 主键 `(ts, usc)` + `INSERT ... ON CONFLICT DO NOTHING` 保证，**无需**额外去重表；`dedup` 只承载「批内去重暂存」「跨源内容指纹比对」「入库审计留痕」三类对象。
- **`export` schema 新增行情导出转存表**（`V35`）：`export.finv_quote_secu_kline_min`，与 `quote.finv_quote_secu_kline_min` **同名同结构**（`LIKE ... INCLUDING DEFAULTS/CONSTRAINTS/INDEXES/STORAGE/COMMENTS` 建表 + 单独补 `gmt_update` 触发器），用于某证券月度/日历史行情导出前转存；迁移含双向列差集与主键/索引/触发器校验，幂等可重复执行。
  - ⚠ 同名风险：`quote` 在 `search_path` 中先于 `export`，裸写表名会命中业务表，转存表读写**必须**写 `export.` 前缀（`DbSchemaSpec.md` §2/§3.1 已补规则与实例）。
  - 验证：源/目标同期行数一致（`NVDA` 2026-06 转存 7,220 行）、`gmt_update` 触发器生效、`TRUNCATE` 后可复用，转存表现为空。
- **新增 `export` / `sche` 两个 schema**（融合前准备）：`export` 用于数据导出域、`sche` 用于定时任务调度域；`V34` 幂等建 schema、以 `current_user` 动态设定属主并授予 `USAGE`/`CREATE`、把两者追加到数据库与连接串的 `search_path` 末尾（`public,quote,quant,futu,export,sche`，不改变既有解析优先级）、末尾做收敛校验；生产库已应用并验证（`V34 success=t`、两 schema 为空、属主 `finv`、权限齐备）。
- **数据库名与角色名统一为 `finv`**（融合前准备，**破坏性**）：PostgreSQL 数据库 `finvquant`→`finv`、登录角色 `finvquant`→`finv`，**密码不变**；生产库已执行 `ALTER ROLE` / `ALTER DATABASE` 改名并验证（数据 2,804,329 行不变、`search_path` 随 OID 保留、授权无需重跑）。
  - 配置：`internal/config` 默认值、`docker-compose.yml`（`FINV_PG_USER`/`FINV_PG_DATABASE`/`POSTGRES_USER`/`POSTGRES_DB`/`pg_isready`）、`.env.example`、Native `FinvQuant.env.example`、`Install.sh` 兜底值、`deploy.cmd`/`upgrade.cmd` 的 `psql`/`pg_dump` 参数
  - 不改：产品/镜像/容器/项目名 `finvquant`、Linux 系统用户与 `/opt/finvquant`、二进制名、备份文件名前缀
  - 新增 [`.dockerignore`](../.dockerignore)：构建上下文排除 `Deploy/Backup/`、`Web/node_modules/`、`Docs/`、`Archive/` 等，构建耗时降至 ~38s
  - 改造清单：新增 [`Docs/Refactor/2610-006-RenameDatabaseAndRole.md`](Refactor/2610-006-RenameDatabaseAndRole.md)（`E-01`～`E-30`）
- 新增 [`Archive/README.md`](../Archive/README.md)：仓库级历史归档目录的用途与边界（放什么、不放什么、按来源建子目录、与原位置对照命名、须随替换提交一并入库），并在根 `README.md` 目录结构与本文件中登记；`Deploy/README.md` 增加「废弃文件去处」指引。
- **备份文件命名规范化**：数据库备份统一为 `finvquant_YYYYMMDD_HHMMSS.dump`（纯 ASCII，按名称即可排序）；
  Docker 升级脚本改用 PowerShell 生成时间戳（不再截取 `%date%`，避免中文系统出现「周五」等字符与区域格式错乱）；
  `Deploy/Backup/` 下 9 个历史备份已按实际生成时间重命名；规范写入 `FileNamingSpec.md` 第 5 节。

## [0.1.0] - 2026-08-08

FinvQuant 首个正式版本：Go 服务端 + Vue3 前端前后端分离骨架、历史行情、元数据字典、通用回测引擎与容器化交付链路落地。

### 新增

- **服务端**：Go 1.25.3 + Gin + pgx/v5（PostgreSQL 18）+ go-redis v9 骨架，默认端口 16001，提供 `/API/V1/Health/Live|Ready` 与 `/API/V1/Version`。
- **前端**：Vue3 + Vite 8 + Vuetify 4（TypeScript），默认端口 16002，开发态 `/api` 代理、生产态反代至 16001。
- **All-in-One 单镜像**：前端构建产物经 `go:embed` 内嵌进 Go 二进制，单进程双端口，无需独立 Nginx 容器。
- **历史行情**：MVSV 文件上传导入 PostgreSQL（field 两种列布局自动识别、字段语义与溯源头部键校验）、K 线查询（分钟级 / 5 日、`ts` 范围查询、日历点选、红涨绿跌、MA/BOLL 图例显隐、富途风格悬停详情）。
- **元数据与数据字典**：`finv_exchange`、`finv_market`、`finv_currency`、`finv_region`、`finv_security` 表及其富途映射表（security / market_code / cs_market / region / exchange），含种子数据与 `flag_enable` 启用标志。
- **元数据管理 API 与维护页面**：字典分页查询（启用项优先排序）、`Security/Lookup` 双策略匹配（`usc` 或源证券代码）、前端多级菜单与元数据维护界面。
- **回测引擎**：通用量化回测引擎（以黄金期货合约完成验证）、多用户 / 环境 / 模板模型、异步任务删除与审计留痕、分批建仓减仓与滑动窗口交易限制。
- **回测报告**：独立报告页（回测背景信息卡片、图表全宽与全屏、拒绝原因可读化、指标与报告导出）。
- **部署交付**：Docker Compose 编排、一键部署 / 版本检测升级 / 回滚脚本及 Windows 11 使用手册。
- **CI/CD**：GitHub Actions 构建并推送 `ghcr.io/acanx/finvquant`；镜像 tag 按分支区分（`dev` 时间戳 / `main` 版本 tag），多平台构建与 `linux-amd64` / `linux-arm64` 平台专属 tag，拉取时输出 manifest digest。
- **版本管理**：新增 `VERSION` 文件作为全局版本唯一来源。

### 变更

- API 路径统一为大写 PascalCase `/API/V1/`，Meta 类接口前缀迁移为 `/API/V1/Meta/Finv/Quant/`。
- 数据库迁移与表名统一 `finv_` 前缀，种子数据段位调整为 `V100000`（十万级）。
- 仓库结构重组：原项目代码整体移入 `VeritasQuant/` 子目录，根目录重建为「外壳 + 子项目」结构，`deploy/` 重命名为 `Deploy/`。
- Docker Compose 顶层项目名统一为 `finvquant`，卷名不再带 `deploy` 前缀，容器名支持自定义（默认 `finvquant`）。
- PostgreSQL 默认暴露宿主机 5432 端口，数据目录支持宿主机文件系统映射。
- 前端 View 目录按菜单分层重组（`Web/src/views/Meta/Finv/`），元数据模块独立为菜单分支。

### 修复

- 生产模式 16002 前端 `/API` 请求正确反向代理到服务端 16001。
- PostgreSQL 18 镜像数据目录挂载点修正（`/var/lib/postgresql/data` → `/var/lib/postgresql`）。
- 部署脚本中文乱码修复（GBK → UTF-8 with BOM），镜像拉取增加网络波动重试与显式 compose 配置文件。
- MVSV 解析修复：文件头引号去除导致证券匹配失败、布局 B 更正为 12 列并容忍行尾空段、`ExchangeCode` 修正。
- 回测修复：每日计数死锁、可空列 NULL 扫描崩溃、时间区间与版本断言同步。
- 迁移修复：`INSERT` 末尾缺分号导致 `COMMIT` 语法错误、种子数据冲突处理改为 `ON CONFLICT DO NOTHING`、重建表迁移列顺序调整。

---

## 历史版本（VeritasQuant 子项目）

> 以下为 `VeritasQuant/` 独立阶段的历史版本，仅为追溯保留；其事件语义与验收门禁见 [`Archive/VeritasQuant/AGENTS.md`](../Archive/VeritasQuant/AGENTS.md)（该子项目已于 2026-10-24 移入归档目录）。

### VeritasQuant 0.1.2 - 2026-08-03

- 新增 FastAPI 应用、统一响应中间件与异常边界。
- 新增命令 API（202 受理、幂等键、乐观并发、取消与失败快照）。
- 新增账户 / 策略 / 数据 / 回测 / 基金计划与报告 API，基础 RBAC、请求追踪 ID、审计与限频。
- 新增鉴权 SSE 状态流与有界 replay cursor。
- 新增 Streamlit 控制台框架与数据导入、策略、定投计划、回测、账户、结果分析、监控页面。
- 新增调度计划与 JobRun 状态机、可观测性指标与 SLO 计算、端到端集成测试。

### VeritasQuant 0.1.1 - 2026-08-03

- 新增 PostgreSQL 事实表 / 投影表 / 索引与首版迁移，数据库端 inbox/outbox、租约与 fencing token。
- 新增共享事件持久化与确定性分区扇出、账户组 worker、风险快照屏障、纸上交易与对账。
- 新增基金定投核心逻辑与业绩报告（TWR、XIRR、本金与规则贡献）。
- 新增 M1 / M2A 门禁检查、崩溃演练报告、每周复核工具与容量预演。
- 完成 P3 信号参考闭环、P4 券商仿真适配层、P5 实盘安全核心与运行保障、P6 离线优化试验追踪。
