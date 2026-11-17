"""ISSUE #253 服务端部署脚本（Windows 11 + Docker Desktop 封装）。

提供子命令：
- `check`   ：检查 Docker/Compose 可用性与编排文件合法性（无副作用）；
- `build`   ：构建服务端镜像（多阶段 Dockerfile）；
- `start`   ：构建并启动全部服务，等待健康检查通过；
- `status`  ：查看服务状态（compose ps）；
- `logs`    ：跟踪服务端日志（可选 --service 指定服务）；
- `stop`    ：停止并删除容器与网络（保留数据卷）。

用法（PowerShell）：
    python3 scripts/DeployServer.py check
    python3 scripts/DeployServer.py start
    python3 scripts/DeployServer.py status
    python3 scripts/DeployServer.py logs --service server
    python3 scripts/DeployServer.py stop
"""

from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path

COMPOSE_FILE = Path(__file__).resolve().parents[1] / "Docker" / "docker-compose.deploy.yml"
ENV_FILE = Path(__file__).resolve().parents[1] / "Docker" / ".env.deploy"


def configureStandardStreams() -> None:
    """确保 Windows 非 UTF-8 控制台也能输出中文 CLI 文案。"""
    for stream in (sys.stdout, sys.stderr):
        reconfigure = getattr(stream, "reconfigure", None)
        if not callable(reconfigure):
            continue
        try:
            reconfigure(encoding="utf-8", errors="backslashreplace")
        except (OSError, ValueError):
            continue


def composeBase() -> list[str]:
    """Compose 基础命令；优先使用 .env.deploy 提供环境变量。"""
    base = ["docker", "compose", "-f", str(COMPOSE_FILE)]
    if ENV_FILE.is_file():
        base.extend(["--env-file", str(ENV_FILE)])
    return base


def composeCommand(action: str, extra: list[str] | None = None) -> list[str]:
    """按动作构造 Compose 命令。"""
    base = composeBase()
    extra = extra or []
    if action == "check":
        return [*base, "config", "--quiet"]
    if action == "build":
        return [*base, "build", *extra]
    if action == "start":
        return [*base, "up", "--detach", "--build", "--wait", *extra]
    if action == "status":
        return [*base, "ps", *extra]
    if action == "logs":
        return [*base, "logs", "--follow", *extra]
    if action == "stop":
        return [*base, "down", "--remove-orphans", *extra]
    raise ValueError(f"未知动作: {action}")


def dockerAvailable() -> tuple[bool, str]:
    """检查 Docker CLI 是否可用；返回 (可用, 说明)。"""
    try:
        result = subprocess.run(
            ["docker", "--version"], capture_output=True, text=True, timeout=10
        )
    except FileNotFoundError:
        return False, "未找到 docker 命令：请先安装 Docker Desktop（启用 WSL2 后端）"
    except subprocess.TimeoutExpired:
        return False, "docker --version 超时：Docker Desktop 可能未启动"
    if result.returncode != 0:
        return False, f"docker 不可用: {result.stderr.strip()}"
    return True, result.stdout.strip()


def main(argv: list[str] | None = None) -> int:
    """执行部署动作；Docker 不可用时明确失败而不回退到本机服务。"""
    configureStandardStreams()
    parser = argparse.ArgumentParser(description="VeritasQuant 服务端部署（Docker Compose 封装）")
    parser.add_argument(
        "action",
        choices=("check", "build", "start", "status", "logs", "stop"),
        help="执行动作",
    )
    parser.add_argument("--service", default=None, help="logs 时指定服务名（server/postgresql/redis）")
    arguments = parser.parse_args(argv)

    if not COMPOSE_FILE.is_file():
        print(f"编排文件不存在: {COMPOSE_FILE}")
        return 1

    if arguments.action != "check":
        available, message = dockerAvailable()
        if not available:
            print(message)
            return 1

    extra: list[str] = []
    if arguments.action == "logs" and arguments.service:
        extra.append(arguments.service)
    command = composeCommand(arguments.action, extra)
    try:
        result = subprocess.run(command, check=False)
    except FileNotFoundError:
        print("Docker Compose 不可用；未启动或修改任何本机服务")
        return 1
    return result.returncode


if __name__ == "__main__":
    raise SystemExit(main())
