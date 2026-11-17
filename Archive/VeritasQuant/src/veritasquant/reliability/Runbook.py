"""P5-013 启动、停机、断连、对账、账本异常、密钥泄漏 Runbook。

对齐 TechSpec 12.3/13 阶段 5 与 ISSUE #209 验收标准：
- 每个 Runbook 必须包含：触发条件、所需权限、执行步骤、验证、回退、证据和升级联系人；
- 覆盖六类场景：启动、停机、断连、对账、账本异常、密钥泄漏；
- Runbook 注册表校验完整性：缺失必填段、无升级联系人或步骤为空均拒绝登记。

- `RunbookKind`：六类 Runbook 枚举；
- `RunbookStepV1`：单个执行步骤（动作 + 期望结果）；
- `RunbookV1`：结构化 Runbook（触发/权限/步骤/验证/回退/证据/升级联系人）；
- `RunbookRegistryV1`：Runbook 注册表（完整性校验 + 按场景检索）。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import StrEnum


def _utcNowMillisecond() -> datetime:
    """UTC 当前时间，截断到毫秒（对齐 TsPrecision.Millisecond）。"""
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1_000) * 1_000)


class RunbookKind(StrEnum):
    Startup = "STARTUP"  # 启动
    Shutdown = "SHUTDOWN"  # 停机
    Disconnect = "DISCONNECT"  # 断连
    Reconciliation = "RECONCILIATION"  # 对账
    LedgerAnomaly = "LEDGER_ANOMALY"  # 账本异常
    SecretLeak = "SECRET_LEAK"  # 密钥泄漏


class RunbookSeverity(StrEnum):
    S0 = "S0"  # 立即处理（危及交易安全/资金）
    S1 = "S1"  # 高优先级（服务降级/差异）
    S2 = "S2"  # 常规操作


@dataclass(frozen=True, slots=True)
class RunbookStepV1:
    """Runbook 执行步骤。"""

    order: int  # 执行顺序（从 1 开始）
    action: str  # 动作描述
    expectedResult: str  # 期望结果（验证步骤完成）

    def __post_init__(self) -> None:
        if self.order <= 0:
            raise ValueError("步骤序号必须为正")
        if not self.action or not self.expectedResult:
            raise ValueError("步骤动作与期望结果不能为空")


@dataclass(frozen=True, slots=True)
class EscalationContactV1:
    """升级联系人：24x7 联系树节点。"""

    role: str  # 角色（如 OnCall-SRE / OnCall-TL）
    name: str  # 姓名/别名
    channel: str  # 联系渠道（电话/飞书/短信）
    priority: int  # 联系优先级（1 最先）

    def __post_init__(self) -> None:
        if not self.role or not self.name or not self.channel:
            raise ValueError("升级联系人角色/姓名/渠道不能为空")
        if self.priority <= 0:
            raise ValueError("联系优先级必须为正")


@dataclass(frozen=True, slots=True)
class RunbookV1:
    """结构化 Runbook：完整覆盖验收要求八要素。"""

    kind: RunbookKind
    title: str
    trigger: str  # 触发条件
    requiredPermissions: tuple[str, ...]  # 所需权限（RBAC 角色）
    steps: tuple[RunbookStepV1, ...]  # 执行步骤（有序）
    verification: str  # 完成验证
    rollback: str  # 回退方案
    evidence: str  # 证据记录（保存位置/格式）
    escalationContacts: tuple[EscalationContactV1, ...]  # 升级联系人（24x7）
    severity: RunbookSeverity = RunbookSeverity.S1
    version: str = "V1"
    updatedAt: datetime = field(default_factory=_utcNowMillisecond)

    def __post_init__(self) -> None:
        if not self.title or not self.trigger:
            raise ValueError("Runbook 标题与触发条件不能为空")
        if not self.steps:
            raise ValueError("Runbook 必须包含执行步骤")
        if not self.verification or not self.rollback or not self.evidence:
            raise ValueError("Runbook 必须包含验证、回退和证据")
        if not self.escalationContacts:
            raise ValueError("Runbook 必须包含升级联系人")
        if not self.requiredPermissions:
            raise ValueError("Runbook 必须声明所需权限")

    def complete(self) -> bool:
        """验收标准完整性：八要素齐全。"""
        return (
            bool(self.trigger)
            and bool(self.requiredPermissions)
            and len(self.steps) >= 1
            and bool(self.verification)
            and bool(self.rollback)
            and bool(self.evidence)
            and len(self.escalationContacts) >= 1
        )

    def orderedSteps(self) -> tuple[RunbookStepV1, ...]:
        """按顺序返回步骤；乱序视为契约违规。"""
        orders = [s.order for s in self.steps]
        if orders != sorted(orders) or orders != list(range(1, len(self.steps) + 1)):
            raise ValueError("Runbook 步骤顺序必须从 1 连续递增")
        return self.steps


class RunbookRegistryV1:
    """Runbook 注册表：六类场景全覆盖 + 完整性校验。"""

    REQUIRED_KINDS = frozenset(RunbookKind)

    def __init__(self) -> None:
        self._runbooks: dict[RunbookKind, RunbookV1] = {}

    def register(self, runbook: RunbookV1) -> None:
        """登记 Runbook；不完整（缺要素）或重复登记拒绝。"""
        if not runbook.complete():
            raise ValueError(f"Runbook {runbook.kind.value} 不完整：必须含触发/权限/步骤/验证/回退/证据/升级联系人")
        runbook.orderedSteps()  # 校验步骤顺序
        if runbook.kind in self._runbooks:
            raise ValueError(f"Runbook 已登记: {runbook.kind.value}")
        self._runbooks[runbook.kind] = runbook

    def get(self, kind: RunbookKind) -> RunbookV1 | None:
        return self._runbooks.get(kind)

    def all(self) -> tuple[RunbookV1, ...]:
        return tuple(self._runbooks[k] for k in RunbookKind if k in self._runbooks)

    def coverageComplete(self) -> bool:
        """六类场景全部登记。"""
        return self.REQUIRED_KINDS <= set(self._runbooks.keys())

    def missingKinds(self) -> tuple[RunbookKind, ...]:
        return tuple(k for k in RunbookKind if k not in self._runbooks)


# ---------------------------------------------------------------------------
# 六类标准 Runbook 工厂（启动/停机/断连/对账/账本异常/密钥泄漏）
# ---------------------------------------------------------------------------

_ONCALL = (
    EscalationContactV1(role="OnCall-SRE", name="SRE-值班", channel="电话", priority=1),
    EscalationContactV1(role="OnCall-TL", name="TL-值班", channel="飞书", priority=2),
)


def buildStartupRunbook() -> RunbookV1:
    """启动 Runbook：冷启动/重启受控启动。"""
    return RunbookV1(
        kind=RunbookKind.Startup,
        title="受控启动",
        trigger="计划维护后启动或崩溃恢复后重启实盘服务",
        requiredPermissions=("Operator", "Administrator"),
        steps=(
            RunbookStepV1(1, "确认 trading-readiness 门禁全部检查项可执行", "门禁清单完整且无未执行项"),
            RunbookStepV1(2, "启动核心服务并等待健康检查通过", "liveness/readiness 通过"),
            RunbookStepV1(3, "核对账本/订单/持仓与券商权威对账", "差异为 0"),
            RunbookStepV1(4, "确认活动 P0/P1 控制已恢复", "控制恢复完整率 100%"),
        ),
        verification="trading-readiness 通过且首次发单前对账干净",
        rollback="停止服务并回退到上一已知良好配置版本",
        evidence="启动时间线、门禁结果、对账报告写入审计存储",
        escalationContacts=_ONCALL,
        severity=RunbookSeverity.S1,
    )


def buildShutdownRunbook() -> RunbookV1:
    """停机 Runbook：受控停机（先停发单再停服务）。"""
    return RunbookV1(
        kind=RunbookKind.Shutdown,
        title="受控停机",
        trigger="计划维护、版本升级或决策层要求停止交易",
        requiredPermissions=("Operator", "Administrator"),
        steps=(
            RunbookStepV1(1, "设置交易控制为禁止新订单并撤单活动订单", "无活动订单"),
            RunbookStepV1(2, "等待 in-flight 订单结果确认（不盲目重发）", "所有订单终态或进入对账"),
            RunbookStepV1(3, "停止服务进程并确认端口/队列退出", "进程退出、队列清空"),
            RunbookStepV1(4, "执行最终账本快照与备份", "快照哈希记录"),
        ),
        verification="停机后账本/订单/持仓快照完整且与券商一致",
        rollback="按启动 Runbook 受控恢复",
        evidence="停机时间线、撤单记录、最终快照哈希写入审计存储",
        escalationContacts=_ONCALL,
        severity=RunbookSeverity.S1,
    )


def buildDisconnectRunbook() -> RunbookV1:
    """断连 Runbook：券商/行情断连处理。"""
    return RunbookV1(
        kind=RunbookKind.Disconnect,
        title="券商/行情断连恢复",
        trigger="券商连接或行情流中断超过阈值（订单受理超时进入查询/对账）",
        requiredPermissions=("Operator", "RiskOperator"),
        steps=(
            RunbookStepV1(1, "确认断连范围（券商/行情/全部）", "明确影响面"),
            RunbookStepV1(2, "停止新发单（trading-readiness 自动 FAIL）", "发单被门禁阻止"),
            RunbookStepV1(3, "对 in-flight 订单执行查询（不盲目重发）", "未知结果进入 TIMEOUT_UNKNOWN/对账"),
            RunbookStepV1(4, "重连并按回报序列缺口请求重放", "序列补齐、无重复副作用"),
            RunbookStepV1(5, "断连恢复后重新对账再恢复发单", "对账差异为 0"),
        ),
        verification="连接恢复、序列无缺口、对账差异 0 且控制恢复",
        rollback="无法恢复时按紧急停止 Runbook 进入 REDUCE_ONLY/STOP_ALL",
        evidence="断连时间线、查询记录、重放结果、对账报告写入审计存储",
        escalationContacts=_ONCALL,
        severity=RunbookSeverity.S0,
    )


def buildReconciliationRunbook() -> RunbookV1:
    """对账 Runbook：每日/事件对账差异处理。"""
    return RunbookV1(
        kind=RunbookKind.Reconciliation,
        title="账本/订单/持仓对账",
        trigger="每日对账、恢复后对账或券商回报与本地不一致",
        requiredPermissions=("Operator", "RiskOperator", "Auditor"),
        steps=(
            RunbookStepV1(1, "拉取券商权威快照与本地账本/订单/持仓", "数据齐全"),
            RunbookStepV1(2, "逐项比对现金/持仓/订单/成交并分类差异", "差异分类（缺失/状态/金额）"),
            RunbookStepV1(3, "对未解释差异发起券商查询", "差异可解释或进入隔离"),
            RunbookStepV1(4, "恢复交易前确认差异为 0", "差异归零"),
        ),
        verification="未解释对账差异为 0；差异未清零不得恢复发单",
        rollback="差异无法解释时保持交易停止并升级 S0",
        evidence="对账报告（差异明细、查询记录、处置）写入审计存储",
        escalationContacts=_ONCALL,
        severity=RunbookSeverity.S0,
    )


def buildLedgerAnomalyRunbook() -> RunbookV1:
    """账本异常 Runbook：账本不平/事务中断处理。"""
    return RunbookV1(
        kind=RunbookKind.LedgerAnomaly,
        title="账本异常处置",
        trigger="账本借贷不平、分录丢失/重复或账本事务中断",
        requiredPermissions=("RiskOperator", "Administrator", "Auditor"),
        steps=(
            RunbookStepV1(1, "立即停止交易并隔离相关账户组", "交易停止"),
            RunbookStepV1(2, "定位异常分录（事件序列/快照重放）", "根因定位"),
            RunbookStepV1(3, "按事实序列重放投影，禁止手工改账", "投影重建且哈希一致"),
            RunbookStepV1(4, "发起冲正/更正命令（需双人授权）", "更正入账且留痕"),
            RunbookStepV1(5, "重新对账确认差异 0 后恢复", "对账干净"),
        ),
        verification="账本重新平衡、投影哈希一致、对账差异 0",
        rollback="无法重建时从最近已验证备份恢复（RPO<=5min）",
        evidence="异常根因、重放结果、冲正命令、审计条目写入审计存储",
        escalationContacts=_ONCALL,
        severity=RunbookSeverity.S0,
    )


def buildSecretLeakRunbook() -> RunbookV1:
    """密钥泄漏 Runbook：凭据/令牌泄露响应。"""
    return RunbookV1(
        kind=RunbookKind.SecretLeak,
        title="密钥泄漏响应",
        trigger="检测到凭据/令牌/私钥可能泄露（日志泄露、外部通报、异常访问）",
        requiredPermissions=("Administrator", "Auditor"),
        steps=(
            RunbookStepV1(1, "立即撤销疑似泄露的令牌/凭据", "撤销即时生效"),
            RunbookStepV1(2, "轮换相关密钥并保留版本历史", "新凭据生效、旧版留痕"),
            RunbookStepV1(3, "审计检索泄露来源与受影响范围", "影响面确认"),
            RunbookStepV1(4, "检查异常命令/访问并冻结相关账户", "无未授权命令"),
            RunbookStepV1(5, "更新 SecretService 最小权限与轮换策略", "策略版本更新"),
        ),
        verification="泄露凭据全部撤销、系统无未授权命令、密钥已轮换",
        rollback="受影响范围扩大时按紧急停止 Runbook 全面停止",
        evidence="撤销记录、轮换历史、审计检索结果写入审计存储",
        escalationContacts=_ONCALL,
        severity=RunbookSeverity.S0,
    )


def buildStandardRunbooks() -> dict[RunbookKind, RunbookV1]:
    """构建六类标准 Runbook。"""
    return {
        RunbookKind.Startup: buildStartupRunbook(),
        RunbookKind.Shutdown: buildShutdownRunbook(),
        RunbookKind.Disconnect: buildDisconnectRunbook(),
        RunbookKind.Reconciliation: buildReconciliationRunbook(),
        RunbookKind.LedgerAnomaly: buildLedgerAnomalyRunbook(),
        RunbookKind.SecretLeak: buildSecretLeakRunbook(),
    }
