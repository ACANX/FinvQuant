"""回测命令入口。"""

from __future__ import annotations

from collections.abc import Sequence

from veritasquant.application.Entrypoints import runOfflineEntrypoint


def main(argv: Sequence[str] | None = None) -> int:
    """返回回测命令离线参数校验的明确退出码。"""
    return runOfflineEntrypoint("vq-run-backtest", "运行 VeritasQuant 回测", argv)
