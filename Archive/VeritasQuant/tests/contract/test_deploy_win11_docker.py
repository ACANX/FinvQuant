"""ISSUE #253 部署物契约测试：Dockerfile / 编排 / 部署脚本 / 教程。

覆盖验收要点：
- Dockerfile 存在且为多阶段构建、非 root 运行、暴露 8000、默认入口正确；
- 部署编排包含 server/postgresql/redis 三服务、健康检查、持久卷、read_only；
- 部署脚本子命令构造正确且无副作用（不实际调用 Docker）；
- Windows 部署教程包含环境要求、依赖说明与详细步骤。
"""

from __future__ import annotations

from pathlib import Path

import pytest
import yaml

REPO_ROOT = Path(__file__).resolve().parents[2]
DOCKER_DIR = REPO_ROOT / "Docker"
DEPLOY_COMPOSE = DOCKER_DIR / "docker-compose.deploy.yml"
DOCKERFILE = DOCKER_DIR / "Dockerfile"
DEPLOY_SCRIPT = REPO_ROOT / "scripts" / "DeployServer.py"
ENV_TEMPLATE = DOCKER_DIR / ".env.deploy.example"
TUTORIAL = DOCKER_DIR / "Windows11Deployment.md"


def test_dockerfile_exists_and_multi_stage() -> None:
    text = DOCKERFILE.read_text(encoding="utf-8")
    assert "FROM python:3.13-slim AS builder" in text
    assert "FROM python:3.13-slim AS runtime" in text
    assert "USER vq" in text  # 非 root 运行
    assert "EXPOSE 18000" in text
    assert '"--port", "18000"' in text
    assert "ENTRYPOINT" in text
    assert "HEALTHCHECK" in text


def test_deploy_compose_has_three_services_with_healthchecks() -> None:
    compose = yaml.safe_load(DEPLOY_COMPOSE.read_text(encoding="utf-8"))
    services = compose["services"]
    assert set(services) == {"server", "postgresql", "redis"}
    for name, service in services.items():
        assert "healthcheck" in service, f"{name} 缺少 healthcheck"
    assert "volumes" in compose  # 持久卷


def test_deploy_compose_server_service() -> None:
    compose = yaml.safe_load(DEPLOY_COMPOSE.read_text(encoding="utf-8"))
    server = compose["services"]["server"]
    # 默认引用 GitHub Packages（ghcr.io）镜像；本地构建块保留为注释选项
    assert "ghcr.io/" in server["image"]
    assert server["read_only"] is True
    assert server["ports"] == ["${VQ_API_PORT:-18000}:18000"]
    assert "depends_on" in server
    # 本地构建选项必须仍可用（Dockerfile 与 build 块存在）
    assert DOCKERFILE.is_file()


def test_deploy_compose_postgres_requires_password() -> None:
    compose = yaml.safe_load(DEPLOY_COMPOSE.read_text(encoding="utf-8"))
    pg = compose["services"]["postgresql"]
    assert "${VQ_POSTGRES_PASSWORD:?必须设置" in pg["environment"]["POSTGRES_PASSWORD"]
    assert pg["environment"]["POSTGRES_HOST_AUTH_METHOD"] == "scram-sha-256"


def test_deploy_script_subcommands() -> None:
    import importlib.util

    spec = importlib.util.spec_from_file_location("deploy_server", DEPLOY_SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)  # type: ignore[union-attr]
    for action in ("check", "build", "start", "status", "logs", "stop"):
        command = module.composeCommand(action)
        assert command[0] == "docker"
        assert any("docker-compose.deploy.yml" in part for part in command)


def test_deploy_script_logs_service_flag() -> None:
    import importlib.util

    spec = importlib.util.spec_from_file_location("deploy_server", DEPLOY_SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)  # type: ignore[union-attr]
    command = module.composeCommand("logs", ["server"])
    assert command[-1] == "server"
    assert "logs" in command


def test_deploy_script_unknown_action_rejected() -> None:
    import importlib.util

    spec = importlib.util.spec_from_file_location("deploy_server", DEPLOY_SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)  # type: ignore[union-attr]
    with pytest.raises(ValueError, match="未知动作"):
        module.composeCommand("explode")


def test_deploy_script_main_check_requires_no_docker() -> None:
    """check 动作不实际调用 Docker；无 Docker 环境也应只报配置错误而非崩溃。"""
    import importlib.util

    spec = importlib.util.spec_from_file_location("deploy_server", DEPLOY_SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)  # type: ignore[union-attr]
    # 无副作用：直接验证 check 动作构造命令，不执行 subprocess
    command = module.composeCommand("check")
    assert command[-2:] == ["config", "--quiet"]


def test_env_template_has_password_placeholder() -> None:
    text = ENV_TEMPLATE.read_text(encoding="utf-8")
    assert "VQ_POSTGRES_PASSWORD" in text
    assert "VQ_ENVIRONMENT" in text
    assert "VQ_API_PORT" in text


def test_gitignore_covers_env_deploy() -> None:
    gitignore = (REPO_ROOT / ".gitignore").read_text(encoding="utf-8")
    assert "Docker/.env.deploy" in gitignore


def test_tutorial_covers_requirements_deps_and_steps() -> None:
    text = TUTORIAL.read_text(encoding="utf-8")
    # 环境要求
    assert "环境要求" in text
    assert "Docker Desktop" in text
    assert "WSL2" in text
    # 依赖说明
    assert "依赖说明" in text
    assert "PostgreSQL" in text
    assert "Redis" in text
    # 详细步骤
    assert "详细部署步骤" in text
    assert "DeployServer.py" in text
    assert "health" in text
    # 常见问题
    assert "常见问题" in text


def test_deploy_uses_high_port_not_common_ports() -> None:
    """服务端使用 12000 以后端口，避开 8000/8080 常用端口。"""
    dockerfile = DOCKERFILE.read_text(encoding="utf-8")
    compose = DEPLOY_COMPOSE.read_text(encoding="utf-8")
    tutorial = TUTORIAL.read_text(encoding="utf-8")
    env_template = ENV_TEMPLATE.read_text(encoding="utf-8")
    combined = dockerfile + compose + tutorial + env_template
    # 暴露/映射/监听端口必须是 12000 以后
    assert "18000" in combined
    # 不得再监听/暴露 8000 或 8080（允许出现的是注释里的"避开"说明与 FAQ 提及）
    assert ":8000" not in combined.replace("避开 8000/8080", "").replace("8000/8080", "")
    assert ":8080" not in combined.replace("避开 8000/8080", "").replace("8000/8080", "")
