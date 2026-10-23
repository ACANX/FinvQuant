## 平台命名与项目结构规划

### 一、平台名称建议

**推荐名称：VeritasQ**（简写 VQ）

- **含义**：Veritas 源自拉丁语“真理”，呼应本平台的核心设计理念——“在严格因果时序下，追求策略在真实市场环境中的可信验证，拒绝未来信息与虚假繁荣”。后缀 Q 代表 Quantitative（量化）与 Quest（探索、追求）。
- **备选方案**：ChronoQuant（强调时间事件驱动）、EventForge（事件锻造工坊）、StrictFlow（严格流式架构）。可根据团队偏好选用。

### 二、项目目录结构规划

以下为 VeritasQ 平台的完整工程目录，遵循模块化设计，各阶段复用度高、可独立测试。

```
VeritasQ/
├── README.md                          # 项目总览、快速开始
├── LICENSE
├── pyproject.toml                     # 项目依赖与构建配置（Poetry/Pip）
├── configs/                           # 所有配置文件（YAML）
│   ├── settings.yaml                  # 全局设置（日志、数据库、事件总线）
│   ├── instruments.yaml               # 标的元数据定义（合约乘数、保证金、交易时段等）
│   ├── data_sources.yaml              # 数据源配置（文件路径、行情适配器）
│   ├── execution.yaml                 # 执行器配置（撮合模型参数、滑点、延迟）
│   └── strategies/                    # 各策略配置
│       ├── ma_cross.yaml
│       └── momentum.yaml
├── core/                              # 核心引擎（事件、时钟、上下文）
│   ├── __init__.py
│   ├── event.py                       # Event 基类、MarketEvent、GenericEvent 等
│   ├── event_bus.py                   # 事件总线（Redis Stream / 内存队列）
│   ├── clock.py                       # 统一时钟，维持逻辑时间
│   └── context.py                     # 全局上下文（当前时间、运行模式等）
├── data/                              # 数据网关与存储
│   ├── __init__.py
│   ├── file_feeder.py                 # 历史文件流式回放器（多文件归并）
│   ├── live_adapter.py                # 实时行情适配器（WebSocket 等）
│   ├── event_loader.py                # 异构事件库加载器（新闻、宏观等）
│   ├── daily_aggregator.py            # 日线数据合成器
│   └── db/                            # 数据库操作层
│       ├── market_data.py             # 行情读写（TimescaleDB）
│       └── business_data.py           # 订单、成交、快照（PostgreSQL）
├── instruments/                       # 标的定义与管理
│   ├── __init__.py
│   ├── instrument.py                  # Instrument 数据类
│   ├── registry.py                    # 标的注册表（按市场分类）
│   └── schedules.py                   # 各市场交易时段日历
├── strategy/                          # 策略引擎
│   ├── __init__.py
│   ├── base.py                        # BaseStrategy 基类
│   ├── manager.py                     # StrategyManager 多策略调度
│   ├── indicators.py                  # 内置常用指标库
│   ├── examples/                      # 示例策略
│   │   ├── ma_cross.py                # 小时级均线交叉
│   │   └── momentum.py                # 日频动量
│   └── models/                        # 外置 ML 模型文件（可选）
├── execution/                         # 执行与撮合
│   ├── __init__.py
│   ├── order.py                       # OrderEvent、订单状态机
│   ├── matcher.py                     # 撮合引擎（多层级概率模型）
│   ├── brokers/                       # 不同模式的执行器
│   │   ├── backtest_broker.py         # 回测撮合器
│   │   ├── paper_broker.py            # 纸上交易器
│   │   ├── simulation_broker.py       # 仿真交易网关
│   │   └── live_broker.py             # 实盘交易网关
│   └── slippage.py                    # 滑点与冲击模型
├── risk/                              # 风险控制
│   ├── __init__.py
│   ├── engine.py                      # RiskEngine 主控
│   ├── rules/                         # 风控规则集
│   │   ├── position_limit.py
│   │   ├── max_drawdown.py
│   │   └── blacklist.py
│   └── capital.py                     # 资金计算（多币种、保证金）
├── events/                            # 异构事件体系
│   ├── __init__.py
│   ├── news.py                        # 新闻/公告事件
│   ├── macro.py                       # 宏观经济/政策事件
│   ├── alert.py                       # 极端事件检测与预警
│   └── adapters/                      # 第三方数据适配器
│       ├── reuters_adapter.py
│       └── economic_calendar.py
├── optimization/                      # 策略优化与训练
│   ├── __init__.py
│   ├── optuna_search.py               # 超参数搜索
│   ├── ml_pipeline.py                 # ML 模型训练流水线
│   └── replay.py                      # 复盘模块（数据回流、回测重演）
├── monitoring/                        # 监控与告警
│   ├── __init__.py
│   ├── metrics.py                     # Prometheus 指标暴露
│   ├── alerting.py                    # 告警规则（钉钉/微信/邮件）
│   └── dashboard/                     # Streamlit/Grafana 配置
├── web/                               # Web 服务与信号终端
│   ├── __init__.py
│   ├── api.py                         # FastAPI REST 接口
│   ├── signal_app.py                  # 信号审核与手动干预
│   └── templates/                     # 前端模板
├── tests/                             # 自动化测试
│   ├── unit/                          # 单元测试（撮合、风控、数据读取）
│   ├── integration/                   # 集成测试（事件循环、策略运行）
│   └── regression/                    # 回归测试（固定数据+基准绩效对比）
├── scripts/                           # 运维与辅助脚本
│   ├── download_data.py               # 历史数据下载
│   ├── run_backtest.py                # 回测启动入口
│   ├── run_paper.py                   # 模拟盘启动
│   └── run_live.py                    # 实盘启动（慎用）
└── docker/                            # 容器化部署
    ├── Dockerfile
    ├── docker-compose.yml
    └── .env.example
```

### 三、结构说明与使用指引

- **核心模块 (`core`)**：定义了平台的骨架，所有其他模块依赖 `core.event` 和 `core.event_bus`。更换事件队列实现（内存/Redis）只需修改配置，不影响上层逻辑。
- **数据层 (`data`)**：实现了流式文件读取和实时适配器，均生成统一 `MarketEvent`；异构事件加载器同样走同一总线，保证回放时序。
- **策略与执行分离**：`strategy` 只产生订单事件；`execution` 负责订单生命周期和撮合，两者通过事件总线解耦。
- **多阶段复用**：同一策略文件在 `configs/execution.yaml` 中切换 `mode` 即可在回测、模拟、仿真、实盘间切换，无需改动策略代码。
- **优化闭环**：`optimization` 利用 `data/file_feeder` 和 `execution/backtest_broker` 进行严格回测优化，结果可推送到 `monitoring` 进行绩效对比。

这一结构保证了 VeritasQ 平台具备高内聚、低耦合的特性，支持团队协作开发与持续迭代。
