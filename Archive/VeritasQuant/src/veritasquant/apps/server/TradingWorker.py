"""交易工作进程 console script 入口。"""

from __future__ import annotations

from collections.abc import Sequence

from veritasquant.application.Entrypoints import runOfflineEntrypoint


def main(argv: Sequence[str] | None = None) -> int:
    """返回交易工作进程离线参数校验的明确退出码。"""
    return runOfflineEntrypoint("vq-trading-worker", "启动 VeritasQuant 交易工作进程", argv)
