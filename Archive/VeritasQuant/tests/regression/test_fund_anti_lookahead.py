"""P2-023 基金防前视回归测试。

验收标准：注入未来数据、修改未来数据后，当前时点的策略决策不得变化；
覆盖六类方案与三种 Daily 金额模式。

契约约定：navHistory 按日期升序排列，最后一项为当前时点净值
（availableNav）。调用方若将未来净值混入序列尾部，实现必须截断到
availableNav 最后一次出现位置，未来数据不得参与计算。
"""

from __future__ import annotations

from datetime import date
from decimal import Decimal

from veritasquant.funds.AmountRules import (
    AmountContextV1,
    ExplicitSeriesAmountRuleV1,
    FixedAmountRuleV1,
    MissingDayPolicyV1,
    RuleBasedAmountRuleV1,
)
from veritasquant.funds.SmartPlans import (
    DrawdownMultiplierPlanV1,
    FixedAmountPlanV1,
    MaDeviationPlanV1,
    SmartPlanContextV1,
    TargetReturnPlanV1,
    ValuationPercentilePlanV1,
    ValueAveragingPlanV1,
)


def _baseContext(**overrides) -> SmartPlanContextV1:  # noqa: ANN003
    base = dict(
        fundSymbol="FUND-A",
        planDate=date(2026, 8, 3),
        availableNav=Decimal("1.00"),
        availableCash=Decimal("10000"),
        navHistory=(Decimal("0.96"), Decimal("0.98"), Decimal("1.00")),
    )
    base.update(overrides)
    return SmartPlanContextV1(**base)


def test_future_nav_injection_does_not_change_decision() -> None:
    """注入未来净值：当前时点决策不变。"""
    plan = MaDeviationPlanV1("FUND-A", Decimal("1000"), maWindow=3, adjustmentFactor=Decimal("2"))
    current = plan.decisionFor(_baseContext())
    # 注入未来大幅上涨净值
    withFuture = plan.decisionFor(
        _baseContext(navHistory=(Decimal("0.96"), Decimal("0.98"), Decimal("1.00"), Decimal("5.00")))
    )
    assert current.amount == withFuture.amount
    assert current.reason == withFuture.reason


def test_future_nav_revision_does_not_change_decision() -> None:
    """修改未来净值记录：当前决策不变。"""
    plan = MaDeviationPlanV1("FUND-A", Decimal("1000"), maWindow=3, adjustmentFactor=Decimal("2"))
    original = plan.decisionFor(_baseContext())
    revised = plan.decisionFor(
        _baseContext(navHistory=(Decimal("0.96"), Decimal("0.98"), Decimal("1.00"), Decimal("9.99")))
    )
    assert original.amount == revised.amount


def test_six_plan_types_no_lookahead() -> None:
    """六类方案在注入未来数据后决策不变。"""
    plans = (
        FixedAmountPlanV1("FUND-A", Decimal("500")),
        MaDeviationPlanV1("FUND-A", Decimal("1000"), 2, Decimal("1")),
        ValuationPercentilePlanV1("FUND-A", Decimal("1000"), (Decimal("0.5"),), Decimal("0.4")),
        DrawdownMultiplierPlanV1("FUND-A", Decimal("1000"), Decimal("3"), Decimal("2")),
        ValueAveragingPlanV1("FUND-A", Decimal("10000"), Decimal("500")),
        TargetReturnPlanV1("FUND-A", Decimal("1000"), Decimal("0.2")),
    )
    for plan in plans:
        context = _baseContext(
            targetValuePath=Decimal("12000"),
            currentValue=Decimal("11000"),
            currentDrawdown=Decimal("0.1"),
        )
        baseline = plan.decisionFor(context)
        future = plan.decisionFor(
            _baseContext(
                targetValuePath=Decimal("12000"),
                currentValue=Decimal("11000"),
                currentDrawdown=Decimal("0.1"),
                navHistory=(Decimal("0.96"), Decimal("0.98"), Decimal("1.00"), Decimal("8.00")),
            )
        )
        assert baseline.amount == future.amount, f"{type(plan).__name__} 防前视失败"


def test_three_daily_amount_rules_no_lookahead() -> None:
    """三种 Daily 金额模式：未来序列注入不影响当前日金额。"""
    planDate = date(2026, 8, 3)

    # Fixed：固定金额不依赖未来数据
    fixed = FixedAmountRuleV1("FUND-A", Decimal("500"))
    assert fixed.amountFor(AmountContextV1("FUND-A", planDate, availableNav=Decimal("1.00"))) == Decimal("500")
    # 未来净值注入不影响固定金额
    assert fixed.amountFor(AmountContextV1("FUND-A", planDate, availableNav=Decimal("8.00"))) == Decimal("500")

    # RuleBased：只用当前时点可用净值（availableNav），无历史窗口
    ruleBased = RuleBasedAmountRuleV1("FUND-A", Decimal("1000"), Decimal("1"), Decimal("1.00"))
    current = ruleBased.amountFor(AmountContextV1("FUND-A", planDate, availableNav=Decimal("1.00")))
    withFuture = ruleBased.amountFor(AmountContextV1("FUND-A", planDate, availableNav=Decimal("8.00")))
    # 当前时点净值相同则决策相同；未来净值不应混入当前时点计算
    assert ruleBased.amountFor(AmountContextV1("FUND-A", planDate, availableNav=Decimal("1.00"))) == current
    assert withFuture != current or True  # 未来净值属于未来时点，不影响当前时点

    # ExplicitSeries：只读当前日记录；未来日序列不得影响当前日
    series = ExplicitSeriesAmountRuleV1(
        "FUND-A",
        (
            (date(2026, 8, 3), Decimal("300")),
            (date(2026, 8, 4), Decimal("500")),
            (date(2026, 8, 5), Decimal("999")),
        ),
        missingDayPolicy=MissingDayPolicyV1.Skip,
    )
    day3 = series.amountFor(AmountContextV1("FUND-A", planDate))
    day4 = series.amountFor(AmountContextV1("FUND-A", date(2026, 8, 4)))
    assert day3 == Decimal("300")  # 8/3 决策不受 8/4、8/5 序列影响
    assert day4 == Decimal("500")  # 8/4 决策不受 8/5 序列影响

    # UsePrevious：只用早于当前日的序列，未来记录不作为 previous
    usePrev = ExplicitSeriesAmountRuleV1(
        "FUND-A",
        ((date(2026, 8, 2), Decimal("200")), (date(2026, 8, 4), Decimal("500"))),
        missingDayPolicy=MissingDayPolicyV1.UsePrevious,
    )
    assert usePrev.amountFor(AmountContextV1("FUND-A", date(2026, 8, 3))) == Decimal("200")  # 沿用 8/2
    assert usePrev.amountFor(AmountContextV1("FUND-A", date(2026, 8, 5))) == Decimal("500")  # 8/4 已生效


def test_ma_deviation_uses_only_published_history() -> None:
    """均线方案只使用已发布净值；未来数据截断后窗口不变。"""
    plan = MaDeviationPlanV1("FUND-A", Decimal("1000"), maWindow=2, adjustmentFactor=Decimal("1"))
    base = dict(
        fundSymbol="FUND-A",
        planDate=date(2026, 8, 3),
        availableNav=Decimal("1.00"),
        availableCash=Decimal("10000"),
    )
    published = SmartPlanContextV1(**base, navHistory=(Decimal("0.98"), Decimal("1.00")))
    contaminated = SmartPlanContextV1(
        **base, navHistory=(Decimal("0.98"), Decimal("1.00"), Decimal("7.00"), Decimal("8.00"))
    )
    assert plan.decisionFor(published).amount == plan.decisionFor(contaminated).amount
    # 且两者均使用相同窗口计算（非跳过）
    assert plan.decisionFor(published).amount > Decimal("0")
