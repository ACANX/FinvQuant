"""行情校验命令入口。"""

from __future__ import annotations

from collections.abc import Sequence

from veritasquant.application.Entrypoints import runOfflineEntrypoint


def main(argv: Sequence[str] | None = None) -> int:
    """返回行情校验命令离线参数校验的明确退出码。"""
    return runOfflineEntrypoint("vq-validate-market-data", "校验 VeritasQuant 行情数据", argv)
