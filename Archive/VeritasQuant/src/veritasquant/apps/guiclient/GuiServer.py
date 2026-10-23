"""P2-031 Streamlit GUI 服务入口。

- 默认离线参数校验（无副作用，供 packaging 契约测试）；
- `--serve` 时启动 Streamlit（子进程运行 `streamlit run` 等价逻辑）；
- 导入模块不连接 API、不启动服务。
"""

from __future__ import annotations

import argparse
import subprocess
import sys
from collections.abc import Sequence

from veritasquant.application.Entrypoints import configureStandardStreams


def main(argv: Sequence[str] | None = None) -> int:
    """解析 GUI 参数；--serve 时启动 Streamlit。"""
    configureStandardStreams()
    parser = argparse.ArgumentParser(prog="vq-gui", description="启动 VeritasQuant Streamlit GUI")
    parser.add_argument("--api-url", default="http://127.0.0.1:8000", help="后端 API 基地址")
    parser.add_argument("--token", default=None, help="API 访问令牌（Bearer）")
    parser.add_argument("--host", default="127.0.0.1", help="Streamlit 监听地址")
    parser.add_argument("--port", type=int, default=8501, help="Streamlit 监听端口")
    parser.add_argument(
        "--serve",
        action="store_true",
        help="实际启动 GUI；缺省时仅做离线参数校验",
    )
    try:
        arguments = parser.parse_args(argv)
    except SystemExit as error:
        return error.code if isinstance(error.code, int) else 1
    if not arguments.serve:
        return 0
    return _serve(arguments)


def _serve(arguments: argparse.Namespace) -> int:
    """以子进程启动 Streamlit 入口脚本。"""
    import os

    from veritasquant.apps.guiclient.GuiApp import serve as guiServe

    entryScript = _writeEntryScript(guiServe, arguments.api_url, arguments.token)
    command = [
        sys.executable,
        "-m",
        "streamlit",
        "run",
        entryScript,
        "--server.address",
        arguments.host,
        "--server.port",
        str(arguments.port),
        "--server.headless",
        "true",
    ]
    try:
        subprocess.run(command, check=False)
    except KeyboardInterrupt:
        return 0
    finally:
        try:
            os.unlink(entryScript)
        except OSError:
            pass
    return 0


def _writeEntryScript(guiServe: object, apiUrl: str, token: str | None) -> str:
    """写入临时 Streamlit 入口脚本（延迟导入避免离线校验副作用）。"""
    import tempfile

    tokenLiteral = repr(token) if token else "None"
    script = (
        "import sys\n"
        "sys.path.insert(0, 'src')\n"
        f"from veritasquant.apps.guiclient.GuiApp import serve\n"
        f"serve({apiUrl!r}, {tokenLiteral})\n"
    )
    handle, path = tempfile.mkstemp(suffix=".py", prefix="vq_gui_entry_")
    with open(handle, "w", encoding="utf-8") as file:
        file.write(script)
    return path
