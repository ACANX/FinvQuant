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

### 新增

- 新增本变更日志 `Docs/CHANGELOG.md`，汇总 0.1.0 及历史 VeritasQuant 版本的对外变更。
- 开发规范新增「变更日志」强制要求：[`DevSpec/DocSpec.md`](DevSpec/DocSpec.md) 第 4 节、[`DevSpec/GitSpec.md`](DevSpec/GitSpec.md) 第 3 节，并在 [`DevSpec/FileNamingSpec.md`](DevSpec/FileNamingSpec.md) 中明确本文件名全大写。
- 新增需求清单 [`RequirementList.md`](RequirementList.md)，采用五位数编号（自 `10001` 起），记录 2026-10-24 起的新增需求，首版 55 条；`AGENTS.md` 工作约定同步要求新增需求必须登记。
- 新增两份改造清单：改造项 A [`Refactor/2610-SecuCodeToUsc.md`](Refactor/2610-SecuCodeToUsc.md)（`secu_code` → `usc` 全库统一，`A-01`～`A-37`）、改造项 B [`Refactor/2610-PrevCloseToLastClose.md`](Refactor/2610-PrevCloseToLastClose.md)（`prev_close` → `last_close` 重命名 + `open` 兜底回填，`B-01`～`B-24`），共 61 个可勾选检查项与 4 项待决策；B 的删除动作暂缓，留待生产数据核实后另议。

### 变更

- 统一跨平台换行策略：新增根目录 [`.gitattributes`](../.gitattributes)，所有文本文件以 LF 存储并检出，`*.cmd` / `*.bat` / `*.ps1` 保留 CRLF；规范见 [`DevSpec/FileEncodingSpec.md`](DevSpec/FileEncodingSpec.md) 第 4 节。
- 按上述规则重规范化全库行尾，消除 Windows / Linux 开发环境间的纯行尾差异。

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

> 以下为 `VeritasQuant/` 独立阶段的历史版本，仅为追溯保留；其事件语义与验收门禁见 [`VeritasQuant/AGENTS.md`](../VeritasQuant/AGENTS.md)。

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
