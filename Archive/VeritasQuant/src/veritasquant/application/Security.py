"""P2-029 基础 RBAC、请求/追踪 ID、审计与限频（应用层端口与模型）。

本模块只定义领域/应用层的安全模型与决策逻辑，不依赖 FastAPI；
HTTP 接线（中间件）放在 apps.server.SecurityMiddleware。
导入本模块不连接外部服务、不启动线程。
"""

from __future__ import annotations

import time
from collections import deque
from dataclasses import dataclass
from enum import Enum
from typing import Protocol


class Role(Enum):
    """服务端最小权限 RBAC 角色（TechSpec 10.2 固定集合）。"""

    Viewer = "Viewer"
    Researcher = "Researcher"
    Operator = "Operator"
    RiskOperator = "RiskOperator"
    LiveApprover = "LiveApprover"
    Administrator = "Administrator"
    Auditor = "Auditor"


class Permission(Enum):
    """动作权限；与命令类型/API 路由映射。"""

    AccountRead = "account:read"
    AccountWrite = "account:write"
    StrategyRead = "strategy:read"
    StrategyWrite = "strategy:write"
    DataRead = "data:read"
    DataImport = "data:import"
    BacktestRun = "backtest:run"
    FundPlanRead = "fund_plan:read"
    FundPlanWrite = "fund_plan:write"
    ReportRead = "report:read"
    ReportGenerate = "report:generate"
    RiskRead = "risk:read"
    RiskControl = "risk:control"
    CommandSubmit = "command:submit"
    CommandCancel = "command:cancel"
    AuditRead = "audit:read"
    Admin = "admin:*"


# 角色 -> 权限矩阵（默认拒绝：未列出即无权限）
_ROLE_PERMISSIONS: dict[Role, frozenset[Permission]] = {
    Role.Viewer: frozenset(
        {
            Permission.AccountRead,
            Permission.StrategyRead,
            Permission.DataRead,
            Permission.FundPlanRead,
            Permission.ReportRead,
            Permission.RiskRead,
        }
    ),
    Role.Researcher: frozenset(
        {
            Permission.AccountRead,
            Permission.StrategyRead,
            Permission.StrategyWrite,
            Permission.DataRead,
            Permission.BacktestRun,
            Permission.FundPlanRead,
            Permission.ReportRead,
            Permission.ReportGenerate,
        }
    ),
    Role.Operator: frozenset(
        {
            Permission.AccountRead,
            Permission.AccountWrite,
            Permission.StrategyRead,
            Permission.StrategyWrite,
            Permission.DataRead,
            Permission.DataImport,
            Permission.BacktestRun,
            Permission.FundPlanRead,
            Permission.FundPlanWrite,
            Permission.ReportRead,
            Permission.ReportGenerate,
            Permission.CommandSubmit,
            Permission.CommandCancel,
        }
    ),
    Role.RiskOperator: frozenset(
        {
            Permission.AccountRead,
            Permission.StrategyRead,
            Permission.DataRead,
            Permission.FundPlanRead,
            Permission.ReportRead,
            Permission.RiskRead,
            Permission.RiskControl,
        }
    ),
    Role.LiveApprover: frozenset(
        {
            Permission.AccountRead,
            Permission.StrategyRead,
            Permission.DataRead,
            Permission.FundPlanRead,
            Permission.ReportRead,
            Permission.RiskRead,
            Permission.CommandSubmit,
            Permission.CommandCancel,
        }
    ),
    Role.Administrator: frozenset(
        {
            Permission.AccountRead,
            Permission.AccountWrite,
            Permission.StrategyRead,
            Permission.StrategyWrite,
            Permission.DataRead,
            Permission.DataImport,
            Permission.BacktestRun,
            Permission.FundPlanRead,
            Permission.FundPlanWrite,
            Permission.ReportRead,
            Permission.ReportGenerate,
            Permission.RiskRead,
            Permission.RiskControl,
            Permission.CommandSubmit,
            Permission.CommandCancel,
            Permission.AuditRead,
            Permission.Admin,
        }
    ),
    Role.Auditor: frozenset({Permission.AccountRead, Permission.ReportRead, Permission.AuditRead}),
}


class AccessDeniedError(Exception):
    """越权：主体无账户或动作权限（映射 2002 FORBIDDEN）。"""

    def __init__(self, principalId: str, permission: Permission, detail: str = "") -> None:
        super().__init__(f"principal={principalId} 无权限 {permission.value}")
        self.principalId = principalId
        self.permission = permission
        self.detail = detail


class UnauthenticatedError(Exception):
    """身份缺失或无效（映射 2001 UNAUTHENTICATED）。"""

    def __init__(self, reason: str = "身份凭据缺失") -> None:
        super().__init__(reason)
        self.reason = reason


class RateLimitExceededError(Exception):
    """限频拒绝（映射 2004 RATE_LIMITED，携带 Retry-After）。"""

    def __init__(self, scope: str, retryAfterSeconds: int) -> None:
        super().__init__(f"rate limit exceeded: {scope}")
        self.scope = scope
        self.retryAfterSeconds = retryAfterSeconds


@dataclass(frozen=True, slots=True)
class Principal:
    """已鉴权主体：身份 + 角色 + 可访问账户范围。"""

    principalId: str
    roles: tuple[Role, ...]
    accountGroupIds: frozenset[str] = frozenset()
    accountIds: frozenset[str] = frozenset()
    environment: str = "PAPER"

    def hasPermission(self, permission: Permission) -> bool:
        """任一角色含该权限即允许（默认拒绝）。"""
        return any(permission in _ROLE_PERMISSIONS.get(role, frozenset()) for role in self.roles)

    def canAccessAccount(self, accountId: str) -> bool:
        """账户范围校验：空集合表示无限制（仅管理员/审计）；否则必须显式包含。"""
        if not self.accountIds:
            return self.hasPermission(Permission.Admin) or self.hasPermission(Permission.AuditRead)
        return accountId in self.accountIds


@dataclass(frozen=True, slots=True)
class RequestContext:
    """一次请求的上下文：request_id / trace_id / 主体。"""

    requestId: str
    traceId: str | None = None
    principal: Principal | None = None

    @property
    def isAuthenticated(self) -> bool:
        return self.principal is not None


class PrincipalProvider(Protocol):
    """从凭据解析主体；凭据无效抛 UnauthenticatedError。"""

    def resolve(self, credential: str | None) -> Principal: ...


class RequestIdGenerator(Protocol):
    """生成请求/追踪 ID。"""

    def newRequestId(self) -> str: ...

    def newTraceId(self) -> str: ...


@dataclass(frozen=True, slots=True)
class AuditRecord:
    """一条不可变的结构化审计记录。"""

    timestampIso: str
    requestId: str
    traceId: str | None
    principalId: str | None
    action: str
    resource: str
    outcome: str  # ALLOWED / DENIED / FAILED
    permission: str | None = None
    detail: str = ""
    sensitive: bool = False

    def toWire(self) -> dict[str, str | None]:
        return {
            "timestamp": self.timestampIso,
            "request_id": self.requestId,
            "trace_id": self.traceId,
            "principal_id": self.principalId,
            "action": self.action,
            "resource": self.resource,
            "outcome": self.outcome,
            "permission": self.permission,
            "detail": self.detail if not self.sensitive else "[已脱敏]",
        }


class AuditSink(Protocol):
    """审计落点：测试用内存实现，生产接审计存储。"""

    def append(self, record: AuditRecord) -> None: ...


class InMemoryAuditSink:
    """进程内审计收集（模拟盘默认）；可查询最近记录。"""

    def __init__(self, maxRecords: int = 10000) -> None:
        self._records: deque[AuditRecord] = deque(maxlen=maxRecords)

    def append(self, record: AuditRecord) -> None:
        self._records.append(record)

    def records(self) -> tuple[AuditRecord, ...]:
        return tuple(self._records)

    def count(self) -> int:
        return len(self._records)


class TokenBucketRateLimiter:
    """令牌桶限频：按 (scope) 独立桶，固定速率补充，容量封顶。"""

    def __init__(self, capacity: int, refillPerSecond: float) -> None:
        if capacity <= 0 or refillPerSecond <= 0:
            raise ValueError("capacity 与 refillPerSecond 必须为正")
        self._capacity = capacity
        self._refillPerSecond = refillPerSecond
        self._buckets: dict[str, tuple[float, float]] = {}  # scope -> (tokens, last_refill_ts)
        self._lock = _threadingLock()

    def consume(self, scope: str, now: float | None = None) -> bool:
        """尝试消费 1 个令牌；不足时返回 False 并给出建议 Retry-After。"""
        now = time.monotonic() if now is None else now
        with self._lock:
            tokens, lastRefill = self._buckets.get(scope, (self._capacity, now))
            elapsed = max(0.0, now - lastRefill)
            tokens = min(self._capacity, tokens + elapsed * self._refillPerSecond)
            if tokens < 1.0:
                self._buckets[scope] = (tokens, now)
                return False
            self._buckets[scope] = (tokens - 1.0, now)
            return True

    def retryAfterSeconds(self, scope: str, now: float | None = None) -> int:
        """下一次可消费前需等待的秒数（向上取整，至少 1）。"""
        now = time.monotonic() if now is None else now
        with self._lock:
            tokens, lastRefill = self._buckets.get(scope, (self._capacity, now))
            elapsed = max(0.0, now - lastRefill)
            tokens = min(self._capacity, tokens + elapsed * self._refillPerSecond)
            missing = 1.0 - tokens
            if missing <= 0:
                return 0
            return max(1, int(missing / self._refillPerSecond) + 1)

    def reset(self, scope: str) -> None:
        with self._lock:
            self._buckets.pop(scope, None)


def _threadingLock():
    """延迟导入 threading，保持导入无副作用。"""
    import threading

    return threading.Lock()


class SimpleRequestIdGenerator:
    """进程内请求/追踪 ID 生成（模拟盘默认；生产可替换为分布式 ID）。"""

    _counter = 0

    def newRequestId(self) -> str:
        return f"req_{time.time_ns():x}_{self._next()}"

    def newTraceId(self) -> str:
        return f"trc_{time.time_ns():x}_{self._next()}"

    @classmethod
    def _next(cls) -> int:
        cls._counter = (cls._counter + 1) % 1_000_000
        return cls._counter


@dataclass(frozen=True, slots=True)
class RoutePermission:
    """API 路由 -> 所需权限与限频档位。"""

    permission: Permission
    rateLimitScope: str
    rateLimitCapacity: int = 60
    rateLimitRefillPerSecond: float = 1.0


class SecurityService:
    """授权用例：解析主体、校验权限与账户范围、记录审计。"""

    def __init__(
        self,
        principalProvider: PrincipalProvider,
        auditSink: AuditSink,
        requestIdGenerator: RequestIdGenerator,
        rateLimiter: TokenBucketRateLimiter | None = None,
    ) -> None:
        self._principalProvider = principalProvider
        self._auditSink = auditSink
        self._requestIdGenerator = requestIdGenerator
        self._rateLimiter = rateLimiter
        self._routeLimiters: dict[str, TokenBucketRateLimiter] = {}
        self._routeLock = _threadingLock()

    def _limiterFor(self, route: RoutePermission) -> TokenBucketRateLimiter | None:
        """按路由获取/创建独立限频器（容量与速率来自路由定义）。"""
        if self._rateLimiter is None:
            return None
        with self._routeLock:
            limiter = self._routeLimiters.get(route.rateLimitScope)
            if limiter is None:
                limiter = TokenBucketRateLimiter(
                    route.rateLimitCapacity, route.rateLimitRefillPerSecond
                )
                self._routeLimiters[route.rateLimitScope] = limiter
            return limiter

    def authenticate(self, credential: str | None) -> Principal:
        """解析主体；失败抛 UnauthenticatedError。"""
        return self._principalProvider.resolve(credential)

    def newRequestContext(self, credential: str | None, traceId: str | None = None) -> RequestContext:
        """建立请求上下文；凭据缺失时仍创建匿名上下文（供审计与限频）。"""
        requestId = self._requestIdGenerator.newRequestId()
        try:
            principal = self.authenticate(credential)
        except UnauthenticatedError:
            return RequestContext(requestId=requestId, traceId=traceId)
        return RequestContext(requestId=requestId, traceId=traceId, principal=principal)

    def authorize(
        self,
        context: RequestContext,
        route: RoutePermission,
        resource: str,
        accountId: str | None = None,
    ) -> None:
        """强制授权：未鉴权 2001、无权限/越权账户 2002、限频 2004。"""
        requestId = context.requestId
        if context.principal is None:
            self._auditSink.append(
                AuditRecord(
                    timestampIso=_utcNowIso(),
                    requestId=requestId,
                    traceId=context.traceId,
                    principalId=None,
                    action=route.permission.value,
                    resource=resource,
                    outcome="DENIED",
                    permission=route.permission.value,
                    detail="unauthenticated",
                )
            )
            raise UnauthenticatedError("身份凭据缺失或无效")

        principal = context.principal
        if not principal.hasPermission(route.permission):
            self._auditSink.append(
                AuditRecord(
                    timestampIso=_utcNowIso(),
                    requestId=requestId,
                    traceId=context.traceId,
                    principalId=principal.principalId,
                    action=route.permission.value,
                    resource=resource,
                    outcome="DENIED",
                    permission=route.permission.value,
                    detail="missing_permission",
                )
            )
            raise AccessDeniedError(principal.principalId, route.permission, "missing_permission")

        if accountId is not None and not principal.canAccessAccount(accountId):
            self._auditSink.append(
                AuditRecord(
                    timestampIso=_utcNowIso(),
                    requestId=requestId,
                    traceId=context.traceId,
                    principalId=principal.principalId,
                    action=route.permission.value,
                    resource=resource,
                    outcome="DENIED",
                    permission=route.permission.value,
                    detail=f"account_out_of_scope:{accountId}",
                )
            )
            raise AccessDeniedError(principal.principalId, route.permission, "account_out_of_scope")

        if self._rateLimiter is not None:
            limiter = self._limiterFor(route)
            scope = f"{route.rateLimitScope}:{principal.principalId}"
            if limiter is not None and not limiter.consume(scope):
                retryAfter = limiter.retryAfterSeconds(scope)
                self._auditSink.append(
                    AuditRecord(
                        timestampIso=_utcNowIso(),
                        requestId=requestId,
                        traceId=context.traceId,
                        principalId=principal.principalId,
                        action=route.permission.value,
                        resource=resource,
                        outcome="DENIED",
                        permission=route.permission.value,
                        detail=f"rate_limited:{scope}",
                    )
                )
                raise RateLimitExceededError(scope, retryAfter)

        self._auditSink.append(
            AuditRecord(
                timestampIso=_utcNowIso(),
                requestId=requestId,
                traceId=context.traceId,
                principalId=principal.principalId,
                action=route.permission.value,
                resource=resource,
                outcome="ALLOWED",
                permission=route.permission.value,
            )
        )


def _utcNowIso() -> str:
    from datetime import datetime, timezone

    return datetime.now(timezone.utc).isoformat()
