"""P2-032 操作页逻辑测试：表单校验、DSL 校验与主流程。"""

from __future__ import annotations

import httpx

from veritasquant.apps.guiclient.ApiClient import ApiClient
from veritasquant.apps.guiclient.Pages import (
    BacktestRequest,
    ImportRequest,
    PlanDraft,
    StrategyDraft,
    UploadImportRequest,
    submitImport,
    submitUploadImport,
    validateDsl,
)


class TestImportRequest:
    def test_valid_import(self) -> None:
        request = ImportRequest(
            source="cn-feed",
            instrumentId="510300.SH",
            startDate="2026-01-01",
            endDate="2026-06-30",
            importMode="INCREMENTAL",
        )
        assert request.validate() == []

    def test_missing_source_and_instrument(self) -> None:
        request = ImportRequest(
            source="  ",
            instrumentId="",
            startDate="2026-01-01",
            endDate="2026-06-30",
            importMode="INCREMENTAL",
        )
        errors = request.validate()
        assert any("数据源" in e for e in errors)
        assert any("标的" in e for e in errors)

    def test_inverted_date_range(self) -> None:
        request = ImportRequest(
            source="s", instrumentId="i",
            startDate="2026-06-30", endDate="2026-01-01",
            importMode="FULL",
        )
        assert any("开始日期" in e for e in request.validate())

    def test_invalid_mode(self) -> None:
        request = ImportRequest(
            source="s", instrumentId="i",
            startDate="2026-01-01", endDate="2026-06-30",
            importMode="SNAPSHOT",
        )
        assert any("导入模式" in e for e in request.validate())


class TestSubmitImport:
    def test_submit_sends_command(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:  # noqa: ANN001
            import json

            body = json.loads(request.content)
            assert body["command_type"] == "DATA_IMPORT"
            assert body["payload"]["source"] == "cn-feed"
            return httpx.Response(
                202,
                json={
                    "code": 202,
                    "message": "受理",
                    "data": {"command_id": "cmd-1", "status": "PENDING"},
                },
            )

        client = ApiClient("http://test", transport=httpx.MockTransport(handler))
        request = ImportRequest(
            source="cn-feed", instrumentId="510300.SH",
            startDate="2026-01-01", endDate="2026-06-30", importMode="FULL",
        )
        result = submitImport(client, request)
        assert result["command_id"] == "cmd-1"
        assert result["status"] == "PENDING"


class TestUploadImportRequest:
    def test_valid_upload(self) -> None:
        request = UploadImportRequest(fileName="a.mvsv", source="cn-feed", upsertMode="FIELD")
        assert request.validate() == []

    def test_missing_file(self) -> None:
        request = UploadImportRequest(fileName="", source="cn-feed", upsertMode="FIELD")
        assert any("文件" in e for e in request.validate())

    def test_missing_source(self) -> None:
        request = UploadImportRequest(fileName="a.mvsv", source="  ", upsertMode="FIELD")
        assert any("数据源" in e for e in request.validate())

    def test_invalid_mode(self) -> None:
        request = UploadImportRequest(fileName="a.mvsv", source="cn-feed", upsertMode="SNAPSHOT")
        assert any("覆盖模式" in e for e in request.validate())


class TestSubmitUploadImport:
    def test_upload_sends_file(self) -> None:
        captured: dict = {}

        def handler(request: httpx.Request) -> httpx.Response:  # noqa: ANN001
            captured["url"] = str(request.url)
            captured["content"] = request.content
            return httpx.Response(
                200,
                json={
                    "code": 0,
                    "message": "行情导入完成",
                    "data": {
                        "batch_id": "import_518880_20260804120000",
                        "secu_code": "518880",
                        "market_code": 1,
                        "record_count": 3,
                        "content_sha256": "a" * 64,
                        "mode": "FIELD",
                    },
                },
            )

        client = ApiClient("http://test", transport=httpx.MockTransport(handler))
        request = UploadImportRequest(fileName="NVDA.mvsv", source="cn-feed", upsertMode="FIELD")
        result = submitUploadImport(client, request, b"mvsv-content")

        assert "imports/upload" in captured["url"]
        assert result["secu_code"] == "518880"
        assert result["record_count"] == 3


class TestStrategyDraft:
    def test_valid_python_strategy(self) -> None:
        draft = StrategyDraft(name="动量", kind="PYTHON", source="class Momentum(BaseStrategy): pass")
        assert draft.validate() == []

    def test_missing_name(self) -> None:
        draft = StrategyDraft(name="", kind="PYTHON", source="x")
        assert any("名称" in e for e in draft.validate())

    def test_invalid_kind(self) -> None:
        draft = StrategyDraft(name="s", kind="LUA", source="x")
        assert any("类型" in e for e in draft.validate())

    def test_dsl_validation_errors_included(self) -> None:
        draft = StrategyDraft(name="s", kind="DSL", source="PlanType: UnknownType\nFundScope: []")
        errors = draft.validate()
        assert any("PlanType" in e for e in errors)


class TestValidateDsl:
    def test_valid_dsl(self) -> None:
        assert validateDsl("PlanType: FixedAmountSchedule\nFundScope: [FUND-A]") == []

    def test_invalid_yaml(self) -> None:
        errors = validateDsl("PlanType: [unclosed")
        assert any("解析失败" in e for e in errors)

    def test_missing_required_fields(self) -> None:
        errors = validateDsl("PlanType: FixedAmountSchedule")
        assert any("FundScope" in e for e in errors)

    def test_not_object(self) -> None:
        errors = validateDsl("- a\n- b")
        assert any("顶层" in e for e in errors)

    def test_unsupported_plan_type(self) -> None:
        errors = validateDsl("PlanType: Lottery\nFundScope: [FUND-A]")
        assert any("PlanType" in e for e in errors)


class TestPlanDraft:
    def test_valid_plan(self) -> None:
        draft = PlanDraft(
            name="沪深300定投", fundSymbol="FUND-A", frequency="Daily",
            amountMode="Fixed", baseAmount="1000.00", cashSource="AccountCash",
        )
        assert draft.validate() == []

    def test_invalid_frequency_and_amount(self) -> None:
        draft = PlanDraft(
            name="p", fundSymbol="FUND-A", frequency="Hourly",
            amountMode="Fixed", baseAmount="-5", cashSource="AccountCash",
        )
        errors = draft.validate()
        assert any("周期" in e for e in errors)
        assert any("正数" in e for e in errors)

    def test_non_numeric_amount(self) -> None:
        draft = PlanDraft(
            name="p", fundSymbol="FUND-A", frequency="Daily",
            amountMode="Fixed", baseAmount="abc", cashSource="AccountCash",
        )
        assert any("数字" in e for e in draft.validate())

    def test_invalid_cash_source(self) -> None:
        draft = PlanDraft(
            name="p", fundSymbol="FUND-A", frequency="Daily",
            amountMode="Fixed", baseAmount="100", cashSource="CreditCard",
        )
        assert any("资金来源" in e for e in draft.validate())


class TestBacktestRequest:
    def test_valid_backtest(self) -> None:
        request = BacktestRequest(
            strategyId="st-1", accountId="acc-1",
            startDate="2026-01-01", endDate="2026-06-30",
            initialCash="1000000", mode="REALISTIC",
        )
        assert request.validate() == []

    def test_missing_strategy_and_account(self) -> None:
        request = BacktestRequest(
            strategyId="", accountId="",
            startDate="2026-01-01", endDate="2026-06-30",
            initialCash="100", mode="IDEAL",
        )
        errors = request.validate()
        assert any("策略" in e for e in errors)
        assert any("账户" in e for e in errors)

    def test_invalid_mode(self) -> None:
        request = BacktestRequest(
            strategyId="s", accountId="a",
            startDate="2026-01-01", endDate="2026-06-30",
            initialCash="100", mode="FANTASY",
        )
        assert any("模式" in e for e in request.validate())

    def test_negative_initial_cash(self) -> None:
        request = BacktestRequest(
            strategyId="s", accountId="a",
            startDate="2026-01-01", endDate="2026-06-30",
            initialCash="-100", mode="IDEAL",
        )
        assert any("初始资金" in e for e in request.validate())


class TestBacktestApiFlow:
    def test_create_start_cancel_flow(self) -> None:
        """创建 -> 启动 -> 取消 主流程可用。"""
        calls: list[str] = []

        def handler(request: httpx.Request) -> httpx.Response:  # noqa: ANN001
            calls.append(f"{request.method} {request.url.path}")
            if request.method == "POST" and request.url.path == "/api/v1/backtests":
                return httpx.Response(202, json={"code": 202, "message": "受理", "data": {"run_id": "run-1", "status": "CREATED"}})
            if request.url.path == "/api/v1/backtests/run-1/start":
                return httpx.Response(200, json={"code": 0, "message": "ok", "data": {"status": "RUNNING"}})
            if request.url.path == "/api/v1/backtests/run-1/cancel":
                return httpx.Response(200, json={"code": 0, "message": "ok", "data": {"status": "CANCELLED"}})
            return httpx.Response(200, json={"code": 0, "message": "ok", "data": {"backtests": [{"run_id": "run-1", "status": "CREATED"}]}})

        client = ApiClient("http://test", transport=httpx.MockTransport(handler))
        created = client.createBacktest({"strategy_id": "s", "account_id": "a"})
        assert created["run_id"] == "run-1"
        started = client.startBacktest("run-1")
        assert started["status"] == "RUNNING"
        cancelled = client.cancelBacktest("run-1")
        assert cancelled["status"] == "CANCELLED"
        assert calls == [
            "POST /api/v1/backtests",
            "POST /api/v1/backtests/run-1/start",
            "POST /api/v1/backtests/run-1/cancel",
        ]
