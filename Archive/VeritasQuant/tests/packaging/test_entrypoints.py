from __future__ import annotations

import importlib
import os
import subprocess
import sys
import tomllib
from importlib import resources
from pathlib import Path

import pytest


# pyproject.toml 中的外部命令名和内部入口模块必须一一对应且可离线调用。
ENTRYPOINTS = {
    "vq-api-server": "veritasquant.apps.server.ApiServer",
    "vq-trading-worker": "veritasquant.apps.server.TradingWorker",
    "vq-scheduler-service": "veritasquant.apps.server.SchedulerService",
    "vq-gui-client": "veritasquant.apps.guiclient.GuiClient",
    "vq-gui": "veritasquant.apps.guiclient.GuiServer",
    "vq-job-data-ingestion": "veritasquant.jobs.DataIngestionJob",
    "vq-job-account-reconciliation": "veritasquant.jobs.AccountReconciliationJob",
    "vq-job-execution-calibration": "veritasquant.jobs.ExecutionCalibrationJob",
    "vq-job-report-generation": "veritasquant.jobs.ReportGenerationJob",
    "vq-import-market-data": "veritasquant.cli.ImportMarketData",
    "vq-validate-market-data": "veritasquant.cli.ValidateMarketData",
    "vq-run-backtest": "veritasquant.cli.RunBacktest",
    "vq-run-paper-trading": "veritasquant.cli.RunPaperTrading",
}


def test_project_scripts_match_all_formal_entrypoint_modules() -> None:
    project = tomllib.loads(Path("pyproject.toml").read_text(encoding="utf-8"))
    scripts = project["project"]["scripts"]
    assert set(scripts) == set(ENTRYPOINTS)
    for command, moduleName in ENTRYPOINTS.items():
        assert scripts[command] == f"{moduleName}:main"


@pytest.mark.parametrize("command,moduleName", ENTRYPOINTS.items())
def test_formal_entrypoint_help_is_offline_and_has_explicit_exit_code(command: str, moduleName: str, capsys: pytest.CaptureFixture[str]) -> None:
    module = importlib.import_module(moduleName)
    assert module.main(["--help"]) == 0
    assert command in capsys.readouterr().out
    assert module.main(["--unknown-option"]) == 2


def test_packaged_error_catalog_is_available_without_repo_relative_path() -> None:
    catalog = resources.files("veritasquant.resources").joinpath("Schemas", "ApiErrorCodes.yml")
    assert "ErrorCatalogVersion" in catalog.read_text(encoding="utf-8")


def test_formal_entrypoint_help_supports_cp1252_output() -> None:
    environment = dict(os.environ)
    environment["PYTHONIOENCODING"] = "cp1252"
    environment["PYTHONPATH"] = str(Path("src").resolve())
    result = subprocess.run(
        [
            sys.executable,
            "-c",
            "from veritasquant.apps.server.ApiServer import main; raise SystemExit(main(['--help']))",
        ],
        env=environment,
        capture_output=True,
        check=False,
    )

    assert result.returncode == 0
