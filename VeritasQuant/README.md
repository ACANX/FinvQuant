# VeritasQuant

VeritasQuant 是一个严格事件驱动的量化交易平台（Event Sourcing + 投影），覆盖从账户账本、基金定投、信号参考与人工审核、券商仿真适配，到实盘安全控制、审计恢复与离线优化建模的完整能力链。金额与身份路径禁止浮点误差，所有状态流转以事件为唯一事实来源。

> ⚠️ **实盘交易默认禁用**：仅支持 `PAPER`（模拟盘）与 `SIMULATION`（券商仿真）环境；任何实盘启用必须先走 Change 流程并满足 [TechSpec 13 阶段 Gate](Docs/VeritasQuantTechSpec.md)。

## 能力概览

| 阶段 | 能力 |
| --- | --- |
| P2 平台核心 | 事件驱动账户/订单/资金/份额账本、基金定投（计划/日历/金额模式/受限 DSL）、API 服务（统一错误码/命令幂等/乐观并发/RBAC/SSE/调度）、Prometheus 指标与 SLO、端到端多账户集成 |
| P3 信号参考 | 信号生成与幂等发布、人工审核与人工成交闭环、通知路由、端到端延迟 SLI、信号偏差分析 |
| P4 券商适配 | 统一 Broker 端口、会话安全（凭据打码/令牌哈希/轮换）、订单网关（超时不盲目重发/限频/幂等）、回报处理与对账、执行诊断校准与 A/B 模型批准 |
| P5 实盘安全 | 环境隔离、密钥服务、短期会话令牌、双人审批、白名单与账户硬上限、紧急停止、影子运行冻结、上线评审、每日 Go/No-Go、不可变审计、备份恢复、Runbook、供应链冻结 |
| P6 离线优化 | 实验跟踪（训练/验证/留出三段隔离）、确定性超参数搜索、优化门禁（留出集达标 + 双人批准） |

## 快速开始

### 1. 服务端（Windows 11 + Docker Desktop，推荐）

```powershell
# 准备环境变量（复制后修改 VQ_POSTGRES_PASSWORD）
Copy-Item Docker\.env.deploy.example Docker\.env.deploy

# 检查并启动（默认拉取 ghcr.io/acanx/veritasquant:latest）
python3 scripts/DeployServer.py check
python3 scripts/DeployServer.py start

# 验证
curl.exe http://localhost:18000/Health/Live
curl.exe http://localhost:18000/api/v1/version
```

详细教程与 FAQ（含代理导致 pip 失败、PowerShell curl 乱码等排错）见
[Docker/Windows11Deployment.md](Docker/Windows11Deployment.md)。

### 2. 客户端（GUI / 回测）

```powershell
python3 -m venv .venv-client
.\.venv-client\Scripts\Activate.ps1
python3 -m pip install -e .

vq-gui --api-url http://localhost:18000 --serve   # GUI（--serve 才真正启动）
vq-run-backtest --help                            # 回测
```

### 3. 本地开发（Linux/macOS）

```bash
python3 -m venv .venv
source .venv/bin/activate
python3 -m pip install -e ".[dev]"

pytest                       # 全量测试
ruff check src tests         # 静态检查
mypy src                     # 类型检查
python3 scripts/Preflight.py # 工程基线（UTF-8/命名/YAML 契约等）
```

## 文档

- [技术规格](Docs/VeritasQuantTechSpec.md)：权威架构设计与阶段 Gate 定义
- [开发计划](Docs/VeritasQuantDevelopmentPlan.md)：工作项与验收标准
- [开发工作流](Docs/VeritasQuantDevelopmentWorkflow.md)：流程、状态机与 CI 自动化要求
- [Windows 11 部署](Docker/Windows11Deployment.md)：Docker 部署教程与 FAQ

## 版本与镜像

- 当前版本：**0.1.2**（Git tag `V0.1.2`）
- 服务端镜像：`ghcr.io/acanx/veritasquant`（`latest` / `0.1.2`，由 GitHub Actions 自动构建发布）
- 版本升级只需同步两处：`pyproject.toml` 的 `version` 与 `.github/workflows/Ci.yml` 的 `VQ_VERSION`
