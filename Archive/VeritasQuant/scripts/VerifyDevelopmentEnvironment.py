"""P0-008 管理临时 Docker Compose 开发依赖，禁止保留卷和秘密。"""

from __future__ import annotations

import argparse
import subprocess
from pathlib import Path


def composeCommand(composePath: Path, action: str) -> list[str]:
    """统一 Compose 调用；start 等待 healthcheck，stop 删除所有局部运行状态。"""
    base = ["docker", "compose", "-f", str(composePath)]
    if action == "start":
        return [*base, "up", "--detach", "--wait", "--remove-orphans"]
    if action == "stop":
        return [*base, "down", "--volumes", "--remove-orphans"]
    return [*base, "ps"]


def main(argv: list[str] | None = None) -> int:
    """执行启动、状态查看或清理；Docker 不可用时明确失败而不回退到本机服务。"""
    parser = argparse.ArgumentParser(description="管理 VeritasQuant 临时开发依赖")
    parser.add_argument("--action", choices=("start", "stop", "check"), default="check")
    parser.add_argument("--compose", type=Path, default=Path("Docker/docker-compose.yml"))
    arguments = parser.parse_args(argv)
    if not arguments.compose.is_file():
        print(f"compose 文件不存在: {arguments.compose}")
        return 1
    try:
        result = subprocess.run(composeCommand(arguments.compose, arguments.action), check=False)
    except FileNotFoundError:
        print("Docker Compose 不可用；未启动或修改任何本机服务")
        return 1
    return result.returncode


if __name__ == "__main__":
    raise SystemExit(main())
