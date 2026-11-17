# 文件命名规范（FileNamingSpec）

> 所属：FinvQuant 开发规范 · 存放：`Docs/DevSpec/`
> 适用范围：全部文件、目录与 Python 标识符命名。

## 1. 文件与目录命名

- 项目中的程序文件、资源文件、文档文件和非 Python 包的组件目录优先使用**大驼峰（PascalCase）**命名，例如 `EventLoop.py`、`Apps/GuiClient/`。
- **例外（全大写固定名）**：生态约定俗成的全大写文件名保持全大写，包括 `CHANGELOG.md`、`README.md`、`LICENSE`、`AGENTS.md`、`Dockerfile` 等。
- 其中**变更日志文件名固定为 `Docs/CHANGELOG.md`（全大写）**，不得写作 `ChangeLog.md`、`CHANGELOG.MD` 等变体；新增或重命名文件前须先确认是否已存在同类文件，避免大小写不同的同名文件并存。

## 2. Python 标识符

- 项目自定义 Python 类名使用 **PascalCase**。
- 方法、函数、参数、局部变量和模型字段优先使用**小驼峰（lowerCamelCase）**，例如 `createOrder()`、`sourceSequence`、`riskPolicyVersion`。
- 既定核心字段 `ts` 保持不变。
- 该规则只约束 Python 标识符，**不改写**版本化事件、API、数据库或文件协议的 wire 字段；风格不一致时使用唯一显式 alias。
- Python 包目录仍遵循导入约定，不因标识符风格调整包路径。

## 3. 标识符语言

- 代码标识符、事件名和配置键使用清晰的**英文**。

## 5. 生成物命名（备份 / 导出 / 日志归档）

- 由脚本自动生成的文件（数据库备份、导出、日志归档）必须使用**纯 ASCII 的固定时间格式**：
  - 数据库逻辑备份：`finvquant_YYYYMMDD_HHMMSS.dump`（例 `finvquant_20261024_012329.dump`）
  - 其它生成物：`<业务名>_YYYYMMDD_HHMMSS.<扩展名>`
- **禁止**在文件名中使用中文（含「周一/周五」等日期本地化文案）与本地化日期格式（如 `2026/08/07`）。
- 时间戳必须由程序按固定格式生成，**不得截取系统区域化输出**：
  - Windows Batch：`for /f %%i in ('powershell -NoProfile -Command "Get-Date -Format yyyyMMdd_HHmmss"') do set "TS=%%i"`（不要用 `%date:~0,4%`，中文系统下 `%date%` 会包含「周五」等字符）；
  - Shell：`date +%Y%m%d_%H%M%S`。
- 时间取**执行机本地时区**，同一目录内按名称即天然按时间排序。
- 备份文件不入库（`.gitignore` 覆盖 `Deploy/Backup/`），但仍须遵守本命名规则，便于跨平台拷贝与排错。

## 4. 过程文档的时间前缀（yyMM-）

- 按阶段/批次新增的**过程文档**（计划、检查清单、改造方案、评审与验收报告等）文件名必须带 `yyMM-` 四位前缀：`yyMM` 为文档创建年月，后接大驼峰名称，例如 `Docs/Refactor/2610-SecuCodeToUsc.md`。
- 同一批交付的多份文档共用一个前缀；前缀一经确定不随后续修改月份变更。
- 例外（**不加**前缀）：
  - 长期固定文档：`CHANGELOG.md`、`RequirementList.md`、`README.md`、`LICENSE`、`AGENTS.md`；
  - 按内容命名的规范/接口/菜单/数据字典文档：`Docs/DevSpec/`、`Docs/API/`、`Docs/Menu/`、`Docs/DataDictMapping/`、`Docs/DataFormat/`。
- 新增过程文档时先确认是否已有同类目录（如 `Docs/Refactor/`），同类文档放入同一目录内，不得平行新建重复目录。
