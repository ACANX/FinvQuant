from __future__ import annotations

import time

from veritasquant.strategy.Sandbox import SandboxQuotaV1
from veritasquant.strategy.SandboxSecurity import (
    SandboxProbeKind,
    SandboxSecurityReportV1,
    StrategySandboxSecuritySuiteV1,
)


def _suite(**overrides: object) -> StrategySandboxSecuritySuiteV1:
    return StrategySandboxSecuritySuiteV1(quota=SandboxQuotaV1(**overrides))  # type: ignore[call-arg]


def test_dangerous_import_probe_blocked() -> None:
    suite = _suite()
    result = suite.scanSource("import subprocess\nimport os\n")
    assert result.blocked
    assert result.probeKind is SandboxProbeKind.Subprocess


def test_network_probe_blocked() -> None:
    suite = _suite()
    result = suite.scanSource("import socket\nimport requests\n")
    assert result.blocked
    assert result.probeKind is SandboxProbeKind.Network


def test_filesystem_probe_blocked() -> None:
    suite = _suite()
    result = suite.scanSource("open('/etc/passwd')\n")
    assert result.blocked
    assert result.probeKind is SandboxProbeKind.FileSystem


def test_nondeterminism_probe_blocked() -> None:
    suite = _suite()
    result = suite.scanSource("import random\nimport time\n")
    assert result.blocked
    assert result.probeKind is SandboxProbeKind.Nondeterminism


def test_safe_source_passes_scan() -> None:
    suite = _suite()
    result = suite.scanSource("from decimal import Decimal\nclose = Decimal('1.2')\n")
    assert not result.blocked


def test_resource_exhaustion_probe_blocks_timeout() -> None:
    suite = _suite(callbackWallSeconds=0.01)

    def slow() -> None:
        time.sleep(0.1)

    result = suite.probeExecution(slow)
    assert result.blocked
    assert result.probeKind is SandboxProbeKind.ResourceExhaustion


def test_cross_account_probe_blocks_exception() -> None:
    suite = _suite()

    def crossAccount() -> None:
        raise RuntimeError("attempt to touch other account")

    result = suite.probeExecution(crossAccount)
    assert result.blocked
    assert result.probeKind is SandboxProbeKind.CrossAccount


def test_full_suite_blocks_all_hostile_sources() -> None:
    suite = _suite()
    report = suite.runFullSuite(
        {
            "hostile-1": "import subprocess\n",
            "hostile-2": "import socket\n",
            "hostile-3": "import random\n",
            "hostile-4": "open('secret')\n",
        }
    )
    assert isinstance(report, SandboxSecurityReportV1)
    assert report.allBlocked
    assert len(report.results) == 4


def test_full_suite_accepts_safe_strategies() -> None:
    suite = _suite()
    report = suite.runFullSuite({"safe-1": "from decimal import Decimal\nvalue = Decimal('1')\n"})
    assert not report.allBlocked
