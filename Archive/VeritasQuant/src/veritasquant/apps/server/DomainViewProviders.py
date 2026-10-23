"""生产环境领域视图提供者（PAPER/仿真模式最小实现）。

背景：P2-028 领域 API 路由与契约测试已完成，但生产入口（ApiServer）此前未
注入 domainApis，导致生产镜像中 /api/v1/accounts、/strategies、/backtests 等
领域端点全部 404（1002 RESOURCE_NOT_FOUND），GUI 客户端无法使用。

本模块提供无外部存储依赖的最小实现：
- 账户列表来自 VQ_ACCOUNTS 环境变量（逗号分隔，默认空）；
- 账户详情返回基础元信息（运行模式取自 VQ_ENVIRONMENT）；
- 账本/现金流/份额/分析在无投影数据时返回空集合；
- 策略/标的/基金目录当前返回空列表（后续阶段接入注册表/配置源）。

实盘（LIVE）不适用：任何实盘启用必须先走 Change 并满足 TechSpec 13 gate。
"""

from __future__ import annotations

import os
from typing import Any

from veritasquant.apps.server.DomainRoutes import (
    AccountViewProvider,
    FundViewProvider,
    InstrumentViewProvider,
    ResourceNotFound,
    StrategyViewProvider,
)


def _configuredAccounts() -> tuple[str, ...]:
    """从 VQ_ACCOUNTS 环境变量解析账户列表（逗号分隔，忽略空项）。"""
    raw = os.environ.get("VQ_ACCOUNTS", "")
    return tuple(item.strip() for item in raw.split(",") if item.strip())


def _environment() -> str:
    """运行环境：PAPER（模拟盘）或 SIMULATION（券商仿真）；禁止 LIVE。"""
    environment = os.environ.get("VQ_ENVIRONMENT", "PAPER").strip().upper()
    if environment not in ("PAPER", "SIMULATION"):
        raise ValueError(f"非法运行环境: {environment}（仅允许 PAPER/SIMULATION）")
    return environment


def _requireAccount(accountId: str) -> None:
    """账户必须存在于配置列表，否则视为资源不存在。"""
    if accountId not in _configuredAccounts():
        raise ResourceNotFound(f"账户不存在: {accountId}")


class ServerAccountViewV1(AccountViewProvider):
    """账户视图：列表来自 VQ_ACCOUNTS，详情为账户基础元信息。"""

    def accounts(self) -> tuple[dict[str, Any], ...]:
        return tuple(
            {"account_id": aid, "execution_mode": _environment(), "run_id": None}
            for aid in _configuredAccounts()
        )

    def account(self, accountId: str, runId: str) -> dict[str, Any]:
        _requireAccount(accountId)
        return {
            "account_id": accountId,
            "execution_mode": _environment(),
            "run_id": runId or None,
            "snapshot": {},
        }

    def ledgerEntries(self, accountId: str, runId: str) -> tuple[dict[str, Any], ...]:
        _requireAccount(accountId)
        return ()

    def cashFlows(self, accountId: str, runId: str) -> tuple[dict[str, Any], ...]:
        _requireAccount(accountId)
        return ()

    def sharePositions(self, accountId: str, runId: str) -> tuple[dict[str, Any], ...]:
        _requireAccount(accountId)
        return ()

    def analysis(self, accountId: str, runId: str) -> dict[str, Any]:
        _requireAccount(accountId)
        return {"twr": None, "xirr": None, "principal": "0.00"}


class ServerStrategyViewV1(StrategyViewProvider):
    """策略目录：当前返回空（策略注册后续阶段接入）。"""

    def strategies(self) -> tuple[dict[str, Any], ...]:
        return ()


class ServerInstrumentViewV1(InstrumentViewProvider):
    """标的目录：当前返回空（标的注册后续阶段接入）。"""

    def instruments(self) -> tuple[dict[str, Any], ...]:
        return ()


class ServerFundViewV1(FundViewProvider):
    """基金目录：当前返回空（基金注册后续阶段接入）。"""

    def funds(self) -> tuple[dict[str, Any], ...]:
        return ()
