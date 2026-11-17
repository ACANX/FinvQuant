"""正式 console script 共用的无副作用参数边界。"""

from __future__ import annotations

import argparse
from collections.abc import Sequence
import sys


def configureStandardStreams() -> None:
    """确保 Windows 非 UTF-8 控制台也能输出中文 CLI 文案。"""
    for stream in (sys.stdout, sys.stderr):
        reconfigure = getattr(stream, "reconfigure", None)
        if not callable(reconfigure):
            continue
        try:
            reconfigure(encoding="utf-8", errors="backslashreplace")
        except (OSError, ValueError):
            # 测试替身或已关闭流可能不支持重配置；参数解析仍应保持可用。
            continue


def runOfflineEntrypoint(
    commandName: str, description: str, argv: Sequence[str] | None = None
) -> int:
    """解析统一的离线入口参数；实际服务组装留给后续应用任务。"""
    configureStandardStreams()
    parser = argparse.ArgumentParser(prog=commandName, description=description)
    parser.add_argument("--config", help="显式绝对配置文件路径")
    parser.add_argument("--resource-dir", help="显式绝对部署资源目录")
    parser.add_argument("--runtime-dir", help="显式绝对运行产物目录")
    try:
        parser.parse_args(argv)
    except SystemExit as error:
        # argparse 的 --help 与非法参数都通过明确退出码回到 console script 包装器。
        return error.code if isinstance(error.code, int) else 1
    return 0
