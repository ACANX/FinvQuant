"""P2-020 受限基金 DSL：解析、类型检查与中间表示。

验收标准：
- 仅白名单表达式/动作（变量、函数、运算符白名单）；
- 禁止 eval、文件、网络和未来变量（只做语法分析，绝不执行任意代码）；
- 错误返回注册业务码（DSL-1001 语法、DSL-1002 未知函数/变量、
  DSL-1003 类型错误、DSL-1004 禁用构造）。
"""

from __future__ import annotations

import ast
from dataclasses import dataclass
from datetime import date
from decimal import Decimal
from enum import StrEnum
from typing import Any

# 白名单变量：只用当时可用数据，禁止未来变量
_ALLOWED_VARIABLES = frozenset(
    {"nav", "nav_prev", "date", "amount", "budget", "balance", "nav_ma", "drawdown"}
)
# 白名单函数：无副作用纯函数
_ALLOWED_FUNCTIONS = frozenset(
    {"abs", "min", "max", "round", "percentile", "avg", "clamp"}
)
# 白名单运算符（ast 节点类名小写）：算术/比较/逻辑
_ALLOWED_OPERATORS = frozenset(
    {
        "add", "sub", "mult", "div", "mod",
        "lt", "lte", "gt", "gte", "eq", "noteq",
        "and", "or",
    }
)


class DslErrorCode(StrEnum):
    SyntaxError = "DSL-1001"
    UnknownSymbol = "DSL-1002"
    TypeError = "DSL-1003"
    DisallowedConstruct = "DSL-1004"


class FundDslError(ValueError):
    """DSL 解析/类型检查失败，携带注册业务码。"""

    def __init__(self, code: DslErrorCode, message: str) -> None:
        super().__init__(f"{code.value} {message}")
        self.code = code
        self.message = message


@dataclass(frozen=True, slots=True)
class DslContextV1:
    """求值上下文：只用当时可用数据。"""

    nav: Decimal | None = None
    navPrev: Decimal | None = None
    planDate: date | None = None
    amount: Decimal | None = None
    budget: Decimal | None = None
    balance: Decimal | None = None
    navMa: Decimal | None = None
    drawdown: Decimal | None = None

    def variableValue(self, name: str) -> Decimal | date | None:
        mapping = {
            "nav": self.nav,
            "nav_prev": self.navPrev,
            "date": self.planDate,
            "amount": self.amount,
            "budget": self.budget,
            "balance": self.balance,
            "nav_ma": self.navMa,
            "drawdown": self.drawdown,
        }
        return mapping.get(name)


class DslEvaluatorV1:
    """白名单表达式求值器：禁止 eval/exec，仅遍历受控 AST。"""

    @staticmethod
    def parse(source: str) -> ast.Expression:
        """解析源码为 AST；语法错误映射 DSL-1001。"""
        try:
            tree = ast.parse(source, mode="eval")
        except SyntaxError as error:
            raise FundDslError(DslErrorCode.SyntaxError, f"语法错误: {error.msg}") from error
        return tree

    @staticmethod
    def validate(node: ast.AST) -> None:
        """白名单校验：未知符号/禁用构造映射 DSL-1002/1004。"""
        for child in ast.walk(node):
            if isinstance(child, ast.Name):
                # 函数名不作为变量求值（Call 分支直接取 func.id），豁免检查
                if child.id not in _ALLOWED_VARIABLES and child.id not in _ALLOWED_FUNCTIONS:
                    if not isinstance(child.ctx, ast.Store):
                        raise FundDslError(DslErrorCode.UnknownSymbol, f"未知变量: {child.id}")
            elif isinstance(child, ast.Call):
                if not isinstance(child.func, ast.Name):
                    raise FundDslError(DslErrorCode.DisallowedConstruct, "函数必须是白名单名称")
                if child.func.id not in _ALLOWED_FUNCTIONS:
                    raise FundDslError(DslErrorCode.UnknownSymbol, "只允许白名单函数调用")
            elif isinstance(child, ast.Attribute):
                raise FundDslError(DslErrorCode.DisallowedConstruct, "禁止属性访问")
            elif isinstance(child, ast.Subscript):
                raise FundDslError(DslErrorCode.DisallowedConstruct, "禁止下标访问")
            elif isinstance(child, (ast.Lambda, ast.Import, ast.ImportFrom)):
                raise FundDslError(DslErrorCode.DisallowedConstruct, "禁止 lambda/import")
            elif isinstance(child, ast.BinOp):
                operatorName = type(child.op).__name__.lower()
                if operatorName not in _ALLOWED_OPERATORS:
                    raise FundDslError(DslErrorCode.UnknownSymbol, f"未知运算符: {operatorName}")
            elif isinstance(child, ast.BoolOp):
                operatorName = type(child.op).__name__.lower()
                if operatorName not in _ALLOWED_OPERATORS:
                    raise FundDslError(DslErrorCode.UnknownSymbol, f"未知逻辑运算符: {operatorName}")

    @staticmethod
    def evaluate(source: str, context: DslContextV1) -> Decimal | bool | date:
        """解析 + 校验 + 求值；全程不执行任意代码。"""
        tree = DslEvaluatorV1.parse(source)
        DslEvaluatorV1.validate(tree)
        return DslEvaluatorV1._evalNode(tree.body, context)

    @staticmethod
    def _evalNode(node: ast.AST, context: DslContextV1) -> Any:  # noqa: ANN401
        if isinstance(node, ast.Constant):
            value: Any = node.value
            if isinstance(value, bool):
                return value
            if isinstance(value, (int, float)):
                return Decimal(str(value))
            raise FundDslError(DslErrorCode.TypeError, f"不支持的常量: {value!r}")
        if isinstance(node, ast.Name):
            value = context.variableValue(node.id)
            if value is None:
                raise FundDslError(DslErrorCode.UnknownSymbol, f"变量 {node.id} 不可用")
            return value
        if isinstance(node, ast.BinOp):
            left = DslEvaluatorV1._evalNode(node.left, context)
            right = DslEvaluatorV1._evalNode(node.right, context)
            if not isinstance(left, Decimal) or not isinstance(right, Decimal):
                raise FundDslError(DslErrorCode.TypeError, "算术操作数必须为数值")
            return _applyBinOp(type(node.op).__name__, left, right)
        if isinstance(node, ast.BoolOp):
            values = [DslEvaluatorV1._evalNode(value, context) for value in node.values]
            if not all(isinstance(value, bool) for value in values):
                raise FundDslError(DslErrorCode.TypeError, "逻辑操作数必须为布尔")
            if type(node.op).__name__ == "And":
                return all(values)
            return any(values)
        if isinstance(node, ast.Compare):
            left = DslEvaluatorV1._evalNode(node.left, context)
            if len(node.ops) != 1:
                raise FundDslError(DslErrorCode.DisallowedConstruct, "禁止链式比较")
            right = DslEvaluatorV1._evalNode(node.comparators[0], context)
            return _applyCompare(type(node.ops[0]).__name__, left, right)
        if isinstance(node, ast.UnaryOp):
            operand = DslEvaluatorV1._evalNode(node.operand, context)
            if type(node.op).__name__ == "Not" and isinstance(operand, bool):
                return not operand
            if type(node.op).__name__ == "USub" and isinstance(operand, Decimal):
                return -operand
            raise FundDslError(DslErrorCode.TypeError, "不支持的一元运算")
        if isinstance(node, ast.Call):
            functionNode = node.func
            assert isinstance(functionNode, ast.Name)  # validate 已保证
            functionName = functionNode.id
            arguments = [DslEvaluatorV1._evalNode(argument, context) for argument in node.args]
            return _callFunction(functionName, arguments)
        raise FundDslError(DslErrorCode.DisallowedConstruct, f"禁止的表达式节点: {type(node).__name__}")


def _applyBinOp(operator: str, left: Decimal, right: Decimal) -> Decimal:
    if operator == "Add":
        return left + right
    if operator == "Sub":
        return left - right
    if operator == "Mult":
        return left * right
    if operator == "Div":
        if right == 0:
            raise FundDslError(DslErrorCode.TypeError, "除数为零")
        return left / right
    if operator == "Mod":
        if right == 0:
            raise FundDslError(DslErrorCode.TypeError, "取模除数为零")
        return left % right
    raise FundDslError(DslErrorCode.UnknownSymbol, f"未知运算符 {operator}")


def _applyCompare(operator: str, left: Any, right: Any) -> bool:  # noqa: ANN401
    if operator == "Lt":
        return left < right
    if operator == "LtE":
        return left <= right
    if operator == "Gt":
        return left > right
    if operator == "GtE":
        return left >= right
    if operator == "Eq":
        return left == right
    if operator == "NotEq":
        return left != right
    raise FundDslError(DslErrorCode.UnknownSymbol, f"未知比较运算符 {operator}")


def _callFunction(name: str, arguments: list[Any]) -> Any:  # noqa: ANN401
    if name == "abs" and len(arguments) == 1 and isinstance(arguments[0], Decimal):
        return abs(arguments[0])
    if name in {"min", "max"} and arguments and all(isinstance(item, Decimal) for item in arguments):
        return min(arguments) if name == "min" else max(arguments)
    if name == "round" and len(arguments) == 1 and isinstance(arguments[0], Decimal):
        return arguments[0].quantize(Decimal("0.01"))
    if name == "avg" and arguments and all(isinstance(item, Decimal) for item in arguments):
        return sum(arguments, Decimal("0")) / Decimal(len(arguments))
    if name == "clamp" and len(arguments) == 3 and all(isinstance(item, Decimal) for item in arguments):
        return min(max(arguments[0], arguments[1]), arguments[2])
    if name == "percentile" and len(arguments) == 2 and isinstance(arguments[0], Decimal) and isinstance(arguments[1], Decimal):
        return arguments[0] * arguments[1]  # 分位近似：value * percentile
    raise FundDslError(DslErrorCode.TypeError, f"函数 {name} 参数不合法")
