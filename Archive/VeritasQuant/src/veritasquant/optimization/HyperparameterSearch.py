"""P6-007b 确定性超参数搜索（网格/随机/顺序）。

对齐 TechSpec 13（优化仅在离线可复现的严格回测中进行）：
- 记录所有试验：搜索器每次评估都经过试验追踪器，参数、数据版本、种子、实现版本
  全部进入试验身份，同输入同输出；
- 搜索只使用训练/验证段成绩选优，禁止触碰留出段（隔离观察由 ExperimentTracker 保证）；
- 确定性：固定种子下网格/随机搜索产生完全相同的参数序列（可复现）；
- 优化结果不能自动晋级：搜索产出候选，是否采用由 Gate（P6-007c）决定。

- `SearchSpaceV1`：超参搜索空间（名称 + 候选值/范围）；
- `HyperparameterSearchV1`：搜索器（网格/随机/顺序三种策略）；
- `SearchResultV1`：搜索结果（候选参数 + 验证成绩 + 搜索元数据）。
"""

from __future__ import annotations

import hashlib
import itertools
import random
from dataclasses import dataclass, field
from datetime import datetime, timezone
from decimal import Decimal
from enum import StrEnum
from typing import Any, Callable

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.optimization.ExperimentTracker import ExperimentTrackerV1


def _utcNowMillisecond() -> datetime:
    """UTC 当前时间，截断到毫秒（对齐 TsPrecision.Millisecond）。"""
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1_000) * 1_000)


class SearchStrategy(StrEnum):
    Grid = "GRID"  # 全组合网格
    Random = "RANDOM"  # 固定种子随机采样
    Sequential = "SEQUENTIAL"  # 按参数顺序扫描


@dataclass(frozen=True, slots=True)
class SearchSpaceV1:
    """超参搜索空间。"""

    name: str  # 参数名（须与策略参数一致）
    candidates: tuple[Any, ...]  # 离散候选值（Grid/Sequential）
    minValue: Any | None = None  # 随机搜索下界
    maxValue: Any | None = None  # 随机搜索上界
    isInteger: bool = False  # 随机搜索整数模式

    def __post_init__(self) -> None:
        if not self.name:
            raise ValueError("参数名不能为空")
        # 随机搜索（有 min/max）允许空候选；网格/顺序必须提供候选
        if not self.candidates and (self.minValue is None or self.maxValue is None):
            raise ValueError("候选值不能为空（随机搜索需提供 minValue/maxValue）")


@dataclass(frozen=True, slots=True)
class SearchResultV1:
    """一次搜索的最终结果。"""

    searchId: str
    strategy: SearchStrategy
    bestParameters: dict[str, Any]
    bestValidationScore: Decimal
    trialsEvaluated: int
    dataVersionId: str
    randomSeed: int
    implementationVersion: str
    searchHash: str
    createdAt: datetime = field(default_factory=_utcNowMillisecond)

    def computeHash(self) -> str:
        """搜索身份哈希：只含确定性输入/输出，不含创建时间（保证同输入同哈希）。"""
        payload = {
            "search_id": self.searchId,
            "strategy": self.strategy.value if isinstance(self.strategy, SearchStrategy) else self.strategy,
            "best_parameters": self.bestParameters,
            "best_validation_score": str(self.bestValidationScore),
            "trials_evaluated": self.trialsEvaluated,
            "data_version_id": self.dataVersionId,
            "random_seed": self.randomSeed,
            "implementation_version": self.implementationVersion,
        }
        return hashlib.sha256(canonicalHash(payload).encode("utf-8")).hexdigest()

    def verify(self) -> bool:
        return self.computeHash() == self.searchHash


class HyperparameterSearchV1:
    """超参搜索器：产出候选参数序列，评估走试验追踪器（隔离留出）。"""

    def __init__(
        self,
        *,
        tracker: ExperimentTrackerV1,
        evaluator: Callable[[dict[str, Any], int], Decimal],
        dataVersionId: str,
        implementationVersion: str,
        randomSeed: int = 42,
    ) -> None:
        """evaluator 接收 (参数, 种子) 返回验证段成绩（确定性）。"""
        self._tracker = tracker
        self._evaluator = evaluator
        self._dataVersionId = dataVersionId
        self._implementationVersion = implementationVersion
        self._seed = randomSeed
        self._counter = 0

    def gridSearch(
        self,
        *,
        space: dict[str, SearchSpaceV1],
        experimentId: str,
    ) -> SearchResultV1:
        """网格搜索：全组合评估，按验证成绩选优。"""
        names = list(space.keys())
        candidateLists = [space[n].candidates for n in names]
        results: list[tuple[Decimal, dict[str, Any]]] = []
        for combo in itertools.product(*candidateLists):
            parameters = dict(zip(names, combo, strict=True))
            score = self._evaluate(parameters, experimentId)
            results.append((score, parameters))
        return self._finalize(
            strategy=SearchStrategy.Grid,
            experimentId=experimentId,
            results=results,
        )

    def randomSearch(
        self,
        *,
        space: dict[str, SearchSpaceV1],
        experimentId: str,
        iterations: int,
    ) -> SearchResultV1:
        """随机搜索：固定种子确定性采样。"""
        if iterations <= 0:
            raise ValueError("迭代次数必须为正")
        rng = random.Random(self._seed)
        results: list[tuple[Decimal, dict[str, Any]]] = []
        for _ in range(iterations):
            parameters = {}
            for name, dim in space.items():
                parameters[name] = self._sampleDimension(rng, dim)
            score = self._evaluate(parameters, experimentId)
            results.append((score, parameters))
        return self._finalize(
            strategy=SearchStrategy.Random,
            experimentId=experimentId,
            results=results,
        )

    def sequentialSearch(
        self,
        *,
        space: dict[str, SearchSpaceV1],
        experimentId: str,
        order: tuple[str, ...] | None = None,
    ) -> SearchResultV1:
        """顺序扫描：外层参数变化最慢（按声明顺序）。"""
        names = list(space.keys())
        if order is not None:
            if set(order) != set(names) or len(order) != len(names):
                raise ValueError("order 必须恰好包含全部参数名一次")
            names = list(order)
        candidateLists = [space[n].candidates for n in names]
        results: list[tuple[Decimal, dict[str, Any]]] = []
        for combo in itertools.product(*candidateLists):
            parameters = dict(zip(names, combo, strict=True))
            score = self._evaluate(parameters, experimentId)
            results.append((score, parameters))
        return self._finalize(
            strategy=SearchStrategy.Sequential,
            experimentId=experimentId,
            results=results,
        )

    def _evaluate(self, parameters: dict[str, Any], experimentId: str) -> Decimal:
        """评估单组参数：经试验追踪器记录（训练/验证成绩；留出保持锁定）。"""
        self._counter += 1
        trial = self._tracker.createTrial(
            experimentId=experimentId,
            parameters=parameters,
            dataVersionId=self._dataVersionId,
            randomSeed=self._seed,
            implementationVersion=self._implementationVersion,
        )
        validationScore = self._evaluator(parameters, self._seed)
        trainingScore = validationScore  # 训练段成绩同口径（确定性评估器）
        self._tracker.completeTrial(
            trialId=trial.trialId,
            trainingScore=trainingScore,
            validationScore=validationScore,
        )
        return validationScore

    def _sampleDimension(self, rng: random.Random, dim: SearchSpaceV1) -> Any:
        """随机采样单维参数（整数/浮点/离散）。"""
        if dim.minValue is not None and dim.maxValue is not None:
            if dim.isInteger:
                return rng.randint(int(dim.minValue), int(dim.maxValue))
            return rng.uniform(float(dim.minValue), float(dim.maxValue))
        return rng.choice(dim.candidates)

    def _finalize(
        self,
        *,
        strategy: SearchStrategy,
        experimentId: str,
        results: list[tuple[Decimal, dict[str, Any]]],
    ) -> SearchResultV1:
        """汇总结果：按验证成绩选优，产出不可变 SearchResult。

        searchId 由 experimentId + strategy + 种子确定性派生（同输入同 ID，
        保证可复现）；不依赖全局计数器。
        """
        if not results:
            raise ValueError("搜索没有产生任何评估")
        bestScore, bestParams = max(results, key=lambda r: r[0])
        identity = canonicalHash({"experiment": experimentId, "strategy": strategy.value, "seed": self._seed})
        searchId = f"search-{experimentId}-{identity[:12]}"
        result = SearchResultV1(
            searchId=searchId,
            strategy=strategy,
            bestParameters=bestParams,
            bestValidationScore=bestScore,
            trialsEvaluated=len(results),
            dataVersionId=self._dataVersionId,
            randomSeed=self._seed,
            implementationVersion=self._implementationVersion,
            searchHash="",
        )
        result = SearchResultV1(
            searchId=result.searchId,
            strategy=strategy,
            bestParameters=bestParams,
            bestValidationScore=bestScore,
            trialsEvaluated=len(results),
            dataVersionId=result.dataVersionId,
            randomSeed=result.randomSeed,
            implementationVersion=result.implementationVersion,
            searchHash=result.computeHash(),
            createdAt=result.createdAt,
        )
        return result


def buildNumericSpace(
    name: str,
    candidates: tuple[int | float | Decimal, ...],
) -> SearchSpaceV1:
    """便捷构造：数值候选空间。"""
    return SearchSpaceV1(name=name, candidates=candidates)
