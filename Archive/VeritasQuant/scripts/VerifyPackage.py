"""P0-006/P0-009 仓库外 wheel 安装和正式命令验证器。"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
import tempfile
import tomllib
from pathlib import Path


def configureStandardStreams() -> None:
    """确保 Windows CI 使用非 UTF-8 代码页时仍可输出诊断信息。"""
    for stream in (sys.stdout, sys.stderr):
        reconfigure = getattr(stream, "reconfigure", None)
        if not callable(reconfigure):
            continue
        try:
            reconfigure(encoding="utf-8", errors="backslashreplace")
        except (OSError, ValueError):
            continue


def venvPython(venvRoot: Path) -> Path:
    """按平台返回虚拟环境解释器路径。"""
    return venvRoot / ("Scripts/python.exe" if os.name == "nt" else "bin/python")


def run(command: list[str], cwd: Path, environment: dict[str, str]) -> None:
    """失败时保留命令和输出，方便 CI 精确定位入口或打包问题。"""
    result = subprocess.run(command, cwd=cwd, env=environment, text=True, capture_output=True, check=False)
    if result.returncode != 0:
        raise RuntimeError(f"命令失败 ({result.returncode}): {' '.join(command)}\n{result.stdout}\n{result.stderr}")


def verify(wheel: Path, projectPath: Path) -> None:
    """在仓库外的临时虚拟环境安装唯一 wheel，拒绝 PYTHONPATH 泄漏。"""
    project = tomllib.loads(projectPath.read_text(encoding="utf-8"))["project"]
    commands = sorted(project["scripts"])
    with tempfile.TemporaryDirectory(prefix="VeritasQuantWheel-") as temporary:
        temporaryPath = Path(temporary)
        venvRoot = temporaryPath / "venv"
        run([sys.executable, "-m", "venv", str(venvRoot)], temporaryPath, os.environ.copy())
        pythonPath = venvPython(venvRoot)
        environment = dict(os.environ)
        environment.pop("PYTHONPATH", None)
        run([str(pythonPath), "-m", "pip", "install", "--disable-pip-version-check", str(wheel.resolve())], temporaryPath, environment)
        binDirectory = pythonPath.parent
        for command in commands:
            executable = binDirectory / (f"{command}.exe" if os.name == "nt" else command)
            run([str(executable), "--help"], temporaryPath, environment)
        run(
            [
                str(pythonPath),
                "-c",
                "from importlib import resources; assert resources.files('veritasquant.resources').joinpath('Schemas', 'ApiErrorCodes.yml').is_file()",
            ],
            temporaryPath,
            environment,
        )


def main(argv: list[str] | None = None) -> int:
    """验证失败返回非零；临时虚拟环境始终由系统清理。"""
    configureStandardStreams()
    parser = argparse.ArgumentParser(description="在仓库外验证 VeritasQuant wheel 与 console script")
    parser.add_argument("--wheel", type=Path, required=True)
    parser.add_argument("--project", type=Path, default=Path("pyproject.toml"))
    arguments = parser.parse_args(argv)
    if not arguments.wheel.is_file() or not arguments.project.is_file():
        print("wheel 或 pyproject.toml 不存在")
        return 1
    try:
        verify(arguments.wheel, arguments.project)
    except (OSError, RuntimeError, tomllib.TOMLDecodeError) as error:
        print(f"package verification failed: {error}")
        return 1
    print("package verification passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
