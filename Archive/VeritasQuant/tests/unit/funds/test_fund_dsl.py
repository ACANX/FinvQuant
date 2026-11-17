"""P2-020 受限基金 DSL 单元测试。"""

from __future__ import annotations

from datetime import date
from decimal import Decimal

import pytest

from veritasquant.funds.FundDsl import (
    DslContextV1,
    DslErrorCode,
    FundDslError,
)


def _context() -> DslContextV1:
    return DslContextV1(
        nav=Decimal("1.20"),
        navPrev=Decimal("1.10"),
        planDate=date(2026, 8, 3),
        amount=Decimal("1000"),
        budget=Decimal("2000"),
        balance=Decimal("5000"),
        navMa=Decimal("1.15"),
        drawdown=Decimal("0.1"),
    )


class TestDslEvaluation:
    def test_arithmetic(self) -> None:
        assert FundDslError  # 引用避免未使用
        from veritasquant.funds.FundDsl import DslEvaluatorV1

        assert DslEvaluatorV1.evaluate("nav * 2", _context()) == Decimal("2.4")
        assert DslEvaluatorV1.evaluate("nav - nav_prev", _context()) == Decimal("0.1")
        assert DslEvaluatorV1.evaluate("budget / 2", _context()) == Decimal("1000")

    def test_comparison_and_logic(self) -> None:
        from veritasquant.funds.FundDsl import DslEvaluatorV1

        assert DslEvaluatorV1.evaluate("nav < 1.5 and budget > 1000", _context()) is True
        assert DslEvaluatorV1.evaluate("nav > 2 or drawdown > 0.05", _context()) is True

    def test_whitelist_functions(self) -> None:
        from veritasquant.funds.FundDsl import DslEvaluatorV1

        assert DslEvaluatorV1.evaluate("min(amount, budget)", _context()) == Decimal("1000")
        assert DslEvaluatorV1.evaluate("max(nav, nav_prev)", _context()) == Decimal("1.2")
        assert DslEvaluatorV1.evaluate("clamp(nav, 1.0, 1.3)", _context()) == Decimal("1.2")

    def test_conditional_amount_rule(self) -> None:
        from veritasquant.funds.FundDsl import DslEvaluatorV1

        expression = "amount * 2 if nav < nav_ma else amount"
        tree = DslEvaluatorV1.parse(expression)
        DslEvaluatorV1.validate(tree)
        # 求值器不支持三元表达式（if-exp）——应映射为禁用构造
        with pytest.raises(FundDslError) as error:
            DslEvaluatorV1._evalNode(tree.body, _context())  # noqa: SLF001
        assert error.value.code is DslErrorCode.DisallowedConstruct


class TestDslSecurity:
    def test_unknown_function_rejected(self) -> None:
        from veritasquant.funds.FundDsl import DslEvaluatorV1

        with pytest.raises(FundDslError) as error:
            DslEvaluatorV1.evaluate("eval('__import__(\"os\")')", _context())
        assert error.value.code is DslErrorCode.UnknownSymbol

    def test_unknown_variable_rejected(self) -> None:
        from veritasquant.funds.FundDsl import DslEvaluatorV1

        with pytest.raises(FundDslError) as error:
            DslEvaluatorV1.evaluate("nav_future + 1", _context())  # 未来变量禁止
        assert error.value.code is DslErrorCode.UnknownSymbol

    def test_attribute_access_rejected(self) -> None:
        from veritasquant.funds.FundDsl import DslEvaluatorV1

        with pytest.raises(FundDslError) as error:
            DslEvaluatorV1.evaluate("nav.__class__", _context())
        assert error.value.code is DslErrorCode.DisallowedConstruct

    def test_subscript_rejected(self) -> None:
        from veritasquant.funds.FundDsl import DslEvaluatorV1

        with pytest.raises(FundDslError):
            DslEvaluatorV1.evaluate("nav[0]", _context())

    def test_import_rejected(self) -> None:
        from veritasquant.funds.FundDsl import DslEvaluatorV1

        with pytest.raises(FundDslError):
            DslEvaluatorV1.evaluate("__import__('os').system('ls')", _context())

    def test_syntax_error_maps_to_registered_code(self) -> None:
        from veritasquant.funds.FundDsl import DslEvaluatorV1

        with pytest.raises(FundDslError) as error:
            DslEvaluatorV1.evaluate("nav +", _context())
        assert error.value.code is DslErrorCode.SyntaxError

    def test_division_by_zero_type_error(self) -> None:
        from veritasquant.funds.FundDsl import DslEvaluatorV1

        with pytest.raises(FundDslError) as error:
            DslEvaluatorV1.evaluate("nav / 0", _context())
        assert error.value.code is DslErrorCode.TypeError

    def test_lambda_rejected(self) -> None:
        from veritasquant.funds.FundDsl import DslEvaluatorV1

        with pytest.raises(FundDslError) as error:
            DslEvaluatorV1.evaluate("(lambda x: x)(nav)", _context())
        assert error.value.code is DslErrorCode.DisallowedConstruct


class TestDslDeterminism:
    def test_same_input_same_result(self) -> None:
        from veritasquant.funds.FundDsl import DslEvaluatorV1

        first = DslEvaluatorV1.evaluate("nav * budget / 100", _context())
        second = DslEvaluatorV1.evaluate("nav * budget / 100", _context())
        assert first == second == Decimal("24.0")
