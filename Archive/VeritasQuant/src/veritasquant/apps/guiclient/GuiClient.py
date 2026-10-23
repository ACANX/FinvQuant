"""GUI 客户端 console script 入口。"""

from __future__ import annotations

from collections.abc import Sequence

from veritasquant.application.Entrypoints import runOfflineEntrypoint


def main(argv: Sequence[str] | None = None) -> int:
    """返回 GUI 客户端离线参数校验的明确退出码。"""
    return runOfflineEntrypoint("vq-gui-client", "启动 VeritasQuant GUI 客户端", argv)
