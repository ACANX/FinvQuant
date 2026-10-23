# VeritasQuant 智能体协作指南

## 适用范围与唯一事实来源

本文件适用于整个仓库。项目采用“方案优先”原则；[Docs/VeritasQuantTechSpec.md](Docs/VeritasQuantTechSpec.md) 是唯一的权威技术设计。修改架构、事件契约、风险控制、执行行为或技术文档前，必须先阅读其中对应章节。

`Archive/` 保存已合并到技术方案中的历史源文档。除非任务明确要求维护归档，否则不得修改、移动或删除其中的文件。设计决策变更时，必须同步更新 `Docs/VeritasQuantTechSpec.md`。

## 项目目标

VeritasQuant 是一个面向多资产的严格事件驱动量化交易平台，支持多种基金智能定投方案和用户自定义定投规则的历史回测。在进入模拟盘、券商仿真和受控实盘前，必须先产出可复现的研究和回测结果。

规划技术栈为 Python 3.13+、Pydantic/YAML、Pandas/NumPy、PostgreSQL/TimescaleDB、Redis Streams、FastAPI/Streamlit、Prometheus/Grafana、Optuna/MLflow 和 Docker Compose。组件应按核心工作流逐步引入，不得在对应业务流程尚未建立前过早增加基础设施。

- 所有 Python 相关的开发、测试、构建、运行和部署命令统一使用 `python3` 执行；当前默认开发、测试、运行和部署环境为 Windows 11 上的 Python 3.13。

## 不可违反的事件语义

- 统一使用 UTC 时间。`Event.ts` 表示平台首次可以使用该信息的时刻，而不一定是底层事实发生的时刻。运行级 `TsPrecision` 默认为 `Second`，需要毫秒时整个运行统一使用 `Millisecond`；禁止把毫秒时间隐式取整、截断或四舍五入为秒。事件可用时间字段必须统一命名为 `ts`，不得保留其他同义字段。
- 可用时必须保留 `occurred_at`、`published_at` 和 `ingested_at`。回测中的外部信息必须按记录的实际可用时间排序。
- 对时间为 `T` 的行情事件，依次执行：撮合此前已生效订单、立即更新订单和账户状态、分发当前事件、由策略生成订单意图、运行 `RiskEngine`、将获批订单加入未来撮合队列。
- 策略不得用当前 Bar 为基于当前 Bar 创建的订单成交，不得访问尚未消费的历史数据，也不得通过辅助 API 查询未来数据。
- 事件必须按版本化排序键 `ts + phase + priority + source_rank + source_sequence + event_id` 形成全序。`phase` 固定为撮合旧订单、记账、分发、策略、风控和入队六个阶段；诊断时间不得参与因果排序。每次运行均须记录 `TsPrecision`、`EventOrderingVersion`、随机种子、数据版本、配置版本和策略版本。
- 核心事件必须使用版本化强类型信封和注册的 Pydantic 载荷，携带运行、生产者、因果链、账户作用域、完整排序元数据和内容哈希；未知主版本、校验失败或同幂等键不同哈希必须隔离，禁止以无约束 `dict` 猜测处理。

## 风控信号、预警与控制

必须严格区分以下职责：

1. `RiskSignal` 是检测器或适配器产生的不可变原始事实。必须持久化来源载荷、证据、观测/检测时间、规则 ID 和规则版本；它本身不得改变交易权限。
2. `AlertNormalizer` 负责校验并将风险信号转换为 `AlertEvent`；`AlertCorrelator` 负责去重、关联、抑制、严重度升级和生命周期更新。
3. `AlertEvent` 是跨模块分发的标准化风控预警事件。其生命周期使用 `alert.created`、`alert.updated` 和 `alert.resolved`；业务风险类别存放于 `alert_type`，例如 `market.extreme_volatility`。
4. `RiskEngine` 是唯一可结合预警、账户、订单、持仓和策略状态作出风险判断的模块。它必须产出可审计的 `RiskDecisionEvent` 和 `TradingControlEvent`；内部 `AlertPolicyEngine` 只能执行无副作用纯规则求值并返回候选建议，无事件发布权。
5. OMS 或券商适配器只能执行已授权的控制事件。通知投递独立于交易控制；通知失败不得导致保护措施失效。

每个 `AlertEvent` 必须包含不可变的 `event_id`、生命周期标识 `alert_id`、稳定的 `dedupe_key`、作用范围、严重度、证据、来源引用及过期或明确解除规则。必须保留原始信号和标准化失败记录，禁止静默丢弃。多个活动控制冲突时，以最严格的控制为准；确认预警不得自动解除 P0/P1 保护。

## 模块边界

- `src/veritasquant/core/`：不可变事件契约、逻辑时钟、确定性分发和事件循环。
- `src/veritasquant/data/`：流式接入、多源归并、数据校验和已完成 Bar 聚合；不得向策略泄漏未来数据。
- `src/veritasquant/accounts/`：多账户不可变平衡账本、资金、持仓、结算和对账契约；成交及入出金、费用税款、公司行为、估值、汇兑和更正均须使用可重放 journal，历史错误只能冲正。
- `src/veritasquant/strategy/`：仅负责信号、定投金额/分配决定和订单或申赎意图；策略不得修改账户、虚构入金、绕过风控或直接调用券商 API。
- `src/veritasquant/execution/`：订单状态迁移、撮合、滑点、基金申购/赎回与份额确认、券商适配器和执行回报；回测、模拟盘、仿真和实盘适配器必须遵循同一契约，场外基金不得套用股票撮合规则。
- `src/veritasquant/risk/`：`RiskSignal` 接口、检测器、预警标准化与关联、风险策略、`RiskEngine`、资金计算及持久化风险状态。
- `src/veritasquant/monitoring/`：指标、结构化运行日志、通知路由、看板和运行可观测性；不得作出交易决策。
- `src/veritasquant/application/`：跨模块业务用例和事务编排；不得包含具体数据库、消息队列或 GUI 实现。
- `src/veritasquant/infrastructure/`：数据库、消息、文件和第三方服务适配；通过端口注入业务模块。
- `src/veritasquant/apps/`：可安装的服务端和 GUI 入口；负责依赖组装、健康检查和优雅停机，不得复制核心业务逻辑。GUI 仅调用服务 API，不得直接访问交易内核或数据库。
- `src/veritasquant/jobs/`：可安装、可独立执行、可重试且幂等的定时任务入口；实际业务逻辑必须复用 `application`。
- `src/veritasquant/cli/`：可安装的正式人工、CI 和运维命令入口；由 `pyproject.toml` 声明 console script。
- `Apps/`、`Jobs/`：只保存 UTF-8 `.yml` 部署或任务注册清单，不得包含 Python 文件、`__init__.py` 或业务逻辑，也不进入 wheel。
- `scripts/`：只保存源码树维护、打包验证和短期诊断脚本，不是正式生产命令；`scripts/temporary/` 不得被生产代码、任务或测试导入。

## 工程目录与依赖

- 项目固定使用 `src` 布局并采用单 wheel 的方案 A，`src/veritasquant/` 是唯一可导入的项目包。不得在仓库根目录、`Apps/`、`Jobs/` 或 `scripts/` 下建立平行业务实现。
- 依赖方向固定为 `veritasquant.apps / veritasquant.jobs / veritasquant.cli -> application -> 领域模块`；包内入口负责注入 `infrastructure` 实现，领域模块不得依赖入口包、FastAPI、Streamlit、调度器或具体数据库客户端。
- 所有正式 Python 入口必须位于 `src/veritasquant/apps/`、`src/veritasquant/jobs/` 或 `src/veritasquant/cli/`，暴露无导入副作用的 `main() -> int`，并由 `pyproject.toml` 的 `[project.scripts]` 映射。
- 根级 `Apps/` 和 `Jobs/` 清单只能引用已安装 console script，不得使用 `python Apps/...`、`python Jobs/...`、仓库相对模块路径或修改 `sys.path`。
- 临时脚本必须注明负责人、用途、创建日期和清理日期；需要长期保留的逻辑应移入核心包并补充测试。
- 随 wheel 分发的内置资源放入 `src/veritasquant/resources/` 并通过 `importlib.resources` 读取；根级 `Configs/` 和 `Resources/` 必须使用显式绝对路径挂载。禁止依赖当前工作目录或用 `__file__` 向上寻找仓库资源。
- 打包测试必须在大小写敏感的 Linux 环境构建 wheel，安装到仓库外的全新虚拟环境，在不设置 `PYTHONPATH` 且切换工作目录后验证所有正式命令、包数据和退出码。
- 首期以 `account_group_id` 为确定性事件循环分区键，组内按冻结的 `account_rank` 串行、组间并行；实盘不得与非实盘账户混组。共享事件按固定 `partition_rank` 扇出，跨组风险只能读取同一 `barrier_event_id` 的完整不可变快照集合。

## 数据、账户与执行

- 分钟级历史行情优先使用 Parquet。回放前必须校验时间顺序、重复数据、OHLC 一致性、成交量、交易时段对齐、缺口和标的映射。
- 规范化分钟行情使用技术方案 `MinuteBarSchemaV1` 和不可变 `DataManifestV1`；原始、规范化、隔离层分开，供应商修订生成新数据版本，禁止覆盖旧回测数据。
- 导入 `.mvsv` 历史行情必须遵循技术方案第 5.2 节 `MVSV-1` 契约。源字段只在适配器边界保留；未由来源契约确认 `BarLabelMeaning` 时必须拒绝导入，未确认 `TurnoverScale` 时不得猜测成交额缩放因子。
- 市场和标的元数据必须版本化，包括时区、日历、币种、合约乘数、保证金、结算、最小价格变动、费率和交易限制。
- 成交与部分成交必须立即更新账户。持续跟踪可用/冻结资金、结算限制、保证金、多币种敞口和已实现/未实现盈亏。
- 基金定投必须遵循技术方案第 7.5、8.1 和 9.2 节。净值只能在实际可用时间后进入规则上下文；申购、等待净值、份额确认和赎回结算使用专用状态机。定投资金流必须形成独立 `DEPOSIT` journal，禁止把入金计为收益或在回测中虚构现金。
- 内置与用户自定义定投规则统一编译到受限策略中间表示，只能使用当时已发布的净值、估值、账户和计划状态。自定义 DSL 字段使用 PascalCase，不得执行 `eval`、访问未来数据或绕过 RiskEngine。
- 基金定投周期必须支持 `Daily`。日频至少覆盖固定金额、规则动态金额和版本化逐日不同额度三种模式；休市日的跳过或累计行为必须显式配置。场外基金在未知价申购时只决定金额，实际份额由后续净值和确认事件计算。
- 必须区分用于诊断信号逻辑的理想执行与用于评估可行性的真实执行。真实执行至少应建模延迟、成交概率、部分成交、成交量参与率、滑点、过期和市场限制。
- 分钟 Bar 撮合必须使用版本化 Bar 内路径、触价/跳空和 tick/手数规则；所有账户共享标的-Bar 流动性池，总分配不得超过全局参与率。

## 测试与验证

- 为每个事件契约、运行级 `TsPrecision` 一致性与非法降精度拒绝、订单/基金申赎状态迁移、定投日历和金额规则、风险规则、`RiskSignal -> AlertEvent` 转换、预警生命周期迁移和控制动作添加聚焦的单元测试。
- 添加覆盖完整事件循环的集成测试：从行情/净值和计划事件，经定投决定、入金、申购/成交、份额或账户更新、策略回调、风险决定，到审计记录持久化。
- 使用固定数据、配置和随机种子的回归基准。任何基准绩效或成交清单变化都必须调查原因。
- 覆盖未来数据探针、重复投递与幂等、数据质量失败、券商断连、陈旧行情、部分成交，以及活动 P0/P1 控制的重启恢复。
- 测试体系必须包含 property-based、model-based、crash-injection、contract compatibility、数据库迁移和策略沙箱安全套件，并按技术方案第 12.2 节追踪矩阵覆盖 R-001 至 R-017；测试跳过或证据缺失视为验收证据不足。
- 修改后运行最小相关测试集。尚无可运行测试命令时，必须明确说明验证仅限文档或静态检查。

## 日志与可观测性

- 程序入口和核心模块必须使用统一的结构化日志配置，不得使用零散的 `print` 充当运行日志。
- 日志时间字段统一使用 UTC 的 `ts`，并与当前运行的 `TsPrecision` 保持一致；存在上下文时必须携带 `run_id`、`account_id`、`event_id`、`order_id`、`execution_id`、`alert_id` 等关联标识。
- 运行日志用于诊断，不能替代不可变事件、订单迁移、风险决定或账户账本等领域审计记录。
- 禁止记录密码、令牌、私钥、完整券商凭据和未脱敏个人信息；高频事件应聚合或采样，日志故障不得阻断 P0/P1 保护动作。

## API 与业务错误码

- 所有 REST JSON 响应顶层固定输出数值 `code` 和文本 `message`，其他 `data`、`error`、`details`、`request_id`、`trace_id` 均为按语义可选字段；所有 wire 字段使用 snake_case。成功码集合固定为 `{0, 1, 200, 202}`，成功及非错误业务状态响应不得携带 `error`，所有错误必须携带 `error`。
- 除固定成功码外，`2-999` 仅允许显式注册为非错误业务状态；`1000-2999` 为平台、安全和依赖错误；项目自定义业务错误码必须大于或等于 3000，并严格遵循技术方案第 10.2 节的领域号段与后三位分类。所有错误的 `error` 对象必须携带 `code`、`catalog_version` 和 `retryable`。
- 完整统一代码目录固定为 `src/veritasquant/resources/Schemas/ApiErrorCodes.yml`，所有字段使用 PascalCase。数值码和符号码全局唯一，已发布号码不得删除、改变语义或复用于其他错误；第三方错误必须在适配器边界映射。
- 领域代码只通过统一 `BusinessException` 抛出已注册业务码，应用边界统一映射顶层 `code`、HTTP、嵌套 `error`、重试属性、本地化消息和公开详情。未注册业务码必须使启动校验或 CI 失败，敏感详情和堆栈不得进入 API 响应。
- API 契约测试必须覆盖固定成功码、特定业务状态、3000 以上业务码的唯一性和号段归属、目录版本兼容、异常映射、命令失败快照和敏感详情过滤。

## 文档、命名与编码

- 默认使用简体中文编写文档、界面文案、注释和用户可见的技术说明；仅在代码标识符、事件名、配置键、第三方协议或术语准确性需要时使用英文。
- Git 提交信息使用 Conventional Commit 风格前缀，标题格式为 `<type>: 简体中文摘要`；允许的首选 `type` 为 `fix`、`feat`、`docs`、`ci`、`test`、`refactor`、`build` 和 `chore`。标题和正文默认使用简体中文；仅当外部协议、固定术语或第三方要求必须使用英文时例外。
- 开发时须为关键契约字段、参数校验和非直观操作补充简体中文注释；避免对显而易见的赋值或语言惯例作重复说明。
- 项目中的程序文件、资源文件、文档文件和非 Python 包的组件目录优先使用大驼峰（PascalCase）命名，例如 `EventLoop.py`、`Apps/GuiClient/`、`VeritasQuantOverallArchitecture.svg` 和 `VeritasQuantTechSpec.md`。根级目录 `Apps/`、`Jobs/`、`Migrations/`、`Docker/`、`Configs/` 和 `Resources/` 固定使用首字母大写。Python 包目录使用小写 `snake_case` 以保持导入一致；仅当语言、框架、工具链或第三方协议规定固定名称时保留其约定，例如 `__init__.py`、`pyproject.toml` 和 `docker-compose.yml`，不得为追求形式统一而破坏生态兼容性。
- `Configs/` 的子目录和项目自有程序配置文件均使用 PascalCase；配置文件优先采用 UTF-8 编码的 `.yml`，例如 `Configs/Environments/Live.yml`，仅当第三方工具明确要求其他扩展名时允许例外。
- `Configs/**/*.yml` 中的项目自有字段名必须使用大驼峰（PascalCase），包括嵌套对象中的字段，例如 `ExecutionMode`、`RiskPolicyVersion` 和 `DataSource`。标的代码等动态映射键不视为字段名；第三方协议要求保留原始键名时必须在适配边界转换，不得让例外扩散到内部配置模型。
- 策略 DSL 不属于程序配置，固定存放于 `Strategies/Dsl/*.yml`，文件名以及所有项目自有 YAML 字段和嵌套字段均使用 PascalCase。项目事件 YAML 示例也使用 PascalCase，其中 `Ts` 唯一映射到内部 `ts`，禁止使用 `Timestamp`；不得把 DSL 文件移入 `Configs/` 或套用程序配置模型。
- 项目自定义 Python 类名使用 PascalCase；方法、函数、参数、局部变量和模型字段优先使用小驼峰（lowerCamelCase），例如 `createOrder()`、`sourceSequence` 和 `riskPolicyVersion`。既定核心字段 `ts` 保持不变。该规则只约束 Python 标识符，不改写版本化事件、API、数据库或文件协议的 wire 字段；风格不一致时使用唯一显式 alias。Python 包目录仍遵循前述导入约定，不因标识符风格调整包路径。
- 标准库、第三方依赖、框架回调、魔术方法和外部协议中的方法或字段必须保持原名称，例如 `datetime.fromtimestamp()`、`BaseModel.model_validate()`、`__init__()`、`main()` 和 Pydantic 的 `validation_alias`。项目自有 PascalCase YAML 字段通过唯一显式 alias 映射到 lowerCamelCase Python 字段；`.mvsv` 等第三方来源字段仅在适配器边界保留，不得为统一外观修改第三方 API。
- 所有新建或修改的文本文件优先使用 UTF-8 编码。不得因本地编辑器或终端默认代码页而将中文内容写成系统区域编码。
- 对包含中文的 `.bat` 或 `.cmd` 文件，使用 UTF-8 编码，首个有效命令必须为 ASCII 的 `@chcp 65001 >nul`，且其前不得出现中文输出、中文注释或其他非 ASCII 内容；避免依赖调用终端的默认代码页。
- 对包含中文的 `.sh` 或 Bash 文件，保持 ASCII shebang，并使用 UTF-8 编码；涉及中文输入、输出或文件名的命令须在目标环境验证 locale/终端编码，不能假定 CI 或远程主机已正确配置。
- 代码标识符、事件名和配置键使用清晰的英文；代码注释默认使用简体中文，除非周边文件已有明确约定。
- 使用明确的领域名称，避免泛化消息名称。预警生命周期（`alert.created`）必须与业务类别（`market.extreme_volatility`）分离。
- 不得基于理想模式收益宣称策略已具备实盘条件。每份结果均须记录数据版本、配置哈希、执行模型、随机种子和风险策略版本。
- 实盘交易默认禁用。未来接入实盘必须显式配置，采用最小权限凭据、幂等下单、订单对账、监控告警和经过演练的人工紧急停止机制。
