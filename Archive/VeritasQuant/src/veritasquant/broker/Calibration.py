"""P4-008 执行校准数据集和候选参数生成 Job。

对齐 TechSpec 7.4/13 阶段 4：
- 延迟、滑点、成交率、部分成交按标的/时段可重算；
- 数据和代码版本齐全（数据集 + 生成器版本可复现）。

- `CalibrationSampleV1`：单次执行的校准样本（延迟/滑点/成交率/部分成交）；
- `CalibrationDatasetV1`：按标的/时段聚合的数据集；
- `CalibrationDatasetBuilderV1`：从诊断报告构建数据集；
- `CandidateParameterSetV1`：候选校准参数（版本化）；
- `CandidateParameterGeneratorV1`：确定性生成候选参数（不依赖随机状态）。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal
from typing import Iterable

from veritasquant.broker.Diagnostics import DiagnosticCollectorV1, ExecutionPhase


class CalibrationError(ValueError):
    """校准数据集或候选参数不满足契约时抛出。"""


@dataclass(frozen=True, slots=True)
class CalibrationSampleV1:
    """单次执行校准样本（Decimal 字符串，禁止 float）。"""

    sampleId: str
    symbol: str
    sessionBucket: str  # OPEN / INTRADAY / CLOSE
    latencySeconds: float
    slippage: str  # Decimal 字符串（相对基准的偏差）
    fillRate: str  # Decimal 字符串 0~1
    partialFillCount: int

    def __post_init__(self) -> None:
        if not self.sampleId or not self.symbol or not self.sessionBucket:
            raise CalibrationError("样本标识字段不能为空")
        fillRate = Decimal(self.fillRate)
        if not 0 <= fillRate <= 1:
            raise CalibrationError("成交率必须在 0~1 区间")


@dataclass(frozen=True, slots=True)
class CalibrationBucketV1:
    """按标的+时段聚合的校准指标。"""

    symbol: str
    sessionBucket: str
    sampleCount: int
    latencyP50: float | None
    latencyP95: float | None
    avgSlippage: str  # Decimal 字符串
    avgFillRate: str  # Decimal 字符串
    totalPartialFills: int

    def __post_init__(self) -> None:
        if not self.symbol or not self.sessionBucket:
            raise CalibrationError("校准桶标识字段不能为空")

    @property
    def adequate(self) -> bool:
        """样本充足性（阶段 4 gate：不少于 90% 落入预注册区间需要样本）。"""
        return self.sampleCount >= 10


@dataclass(frozen=True, slots=True)
class CalibrationDatasetV1:
    """执行校准数据集：数据 + 代码版本齐全。"""

    datasetId: str
    builderVersion: str
    dataVersion: str
    buckets: tuple[CalibrationBucketV1, ...]
    samples: tuple[CalibrationSampleV1, ...] = ()

    def bucketFor(self, symbol: str, sessionBucket: str) -> CalibrationBucketV1 | None:
        for bucket in self.buckets:
            if bucket.symbol == symbol and bucket.sessionBucket == sessionBucket:
                return bucket
        return None


@dataclass(frozen=True, slots=True)
class CandidateParameterSetV1:
    """候选校准参数（版本化，供 A/B 回测）。"""

    candidateId: str
    version: str
    modelType: str  # 例如 IDEAL / LIQUIDITY_POOL / EXECUTION_MODEL
    slippageBps: str  # Decimal 字符串（基点）
    fillRateMultiplier: str  # Decimal 字符串
    latencyBudgetSeconds: float
    approved: bool = False

    def __post_init__(self) -> None:
        if not self.candidateId or not self.version or not self.modelType:
            raise CalibrationError("候选参数标识字段不能为空")


class CalibrationDatasetBuilderV1:
    """从诊断报告构建校准数据集。"""

    def __init__(self, *, builderVersion: str = "V1", dataVersion: str = "2026-08-03") -> None:
        if not builderVersion or not dataVersion:
            raise CalibrationError("构建器与数据版本不能为空")
        self._builderVersion = builderVersion
        self._dataVersion = dataVersion

    def build(
        self,
        *,
        datasetId: str,
        samples: list[CalibrationSampleV1],
    ) -> CalibrationDatasetV1:
        """聚合样本为按标的+时段的桶。"""
        grouped: dict[tuple[str, str], list[CalibrationSampleV1]] = {}
        for sample in samples:
            grouped.setdefault((sample.symbol, sample.sessionBucket), []).append(sample)
        buckets: list[CalibrationBucketV1] = []
        for (symbol, sessionBucket), group in grouped.items():
            latencies = sorted(s.latencySeconds for s in group)
            p50 = _percentile(latencies, 0.50)
            p95 = _percentile(latencies, 0.95)
            avgSlippage = _avgDecimal(s.slippage for s in group)
            avgFillRate = _avgDecimal(s.fillRate for s in group)
            buckets.append(
                CalibrationBucketV1(
                    symbol=symbol,
                    sessionBucket=sessionBucket,
                    sampleCount=len(group),
                    latencyP50=p50,
                    latencyP95=p95,
                    avgSlippage=avgSlippage,
                    avgFillRate=avgFillRate,
                    totalPartialFills=sum(s.partialFillCount for s in group),
                )
            )
        return CalibrationDatasetV1(
            datasetId=datasetId,
            builderVersion=self._builderVersion,
            dataVersion=self._dataVersion,
            buckets=tuple(buckets),
            samples=tuple(samples),
        )

    @staticmethod
    def sampleFromDiagnostics(
        *,
        sampleId: str,
        symbol: str,
        sessionBucket: str,
        diagnostics: DiagnosticCollectorV1,
        clientOrderId: str,
        referencePrice: str,
        benchmarkSlippageBps: str = "0",
    ) -> CalibrationSampleV1:
        """从诊断报告提取一次校准样本。"""
        report = diagnostics.report(clientOrderId)
        if report is None:
            raise CalibrationError(f"诊断报告不存在: {clientOrderId}")
        latency = report.latencyBetween(ExecutionPhase.Submitted, ExecutionPhase.Filled)
        if latency is None:
            latency = 0.0
        partialCount = sum(
            1 for t in report.timestamps if t.phase is ExecutionPhase.PartialFill
        )
        fillRate = _fillRateFromTimeline(report)
        slippage = _slippageFromReference(
            report=report,
            referencePrice=referencePrice,
            benchmarkBps=benchmarkSlippageBps,
        )
        return CalibrationSampleV1(
            sampleId=sampleId,
            symbol=symbol,
            sessionBucket=sessionBucket,
            latencySeconds=latency,
            slippage=slippage,
            fillRate=fillRate,
            partialFillCount=partialCount,
        )


class CandidateParameterGeneratorV1:
    """确定性候选参数生成（相同输入产生相同候选；不依赖随机状态）。"""

    def __init__(self, *, generatorVersion: str = "V1") -> None:
        if not generatorVersion:
            raise CalibrationError("生成器版本不能为空")
        self._version = generatorVersion

    @property
    def version(self) -> str:
        return self._version

    def generate(
        self,
        *,
        modelType: str,
        baseSlippageBps: str = "2",
        baseFillRate: str = "0.98",
        baseLatencyBudget: float = 0.5,
        variants: int = 3,
    ) -> tuple[CandidateParameterSetV1, ...]:
        """生成 variants 个候选参数（含基线与 ± 变体）。"""
        if variants < 1:
            raise CalibrationError("候选数量必须为正")
        baseBps = Decimal(baseSlippageBps)
        candidates: list[CandidateParameterSetV1] = []
        for index in range(variants):
            step = index - (variants - 1) / 2
            candidates.append(
                CandidateParameterSetV1(
                    candidateId=f"cand-{modelType}-{index + 1:02d}",
                    version=f"{self._version}.{index + 1}",
                    modelType=modelType,
                    slippageBps=str(baseBps + Decimal(step)),
                    fillRateMultiplier=str(Decimal(baseFillRate) + Decimal(step) * Decimal("0.005")),
                    latencyBudgetSeconds=baseLatencyBudget + step * 0.1,
                    approved=False,
                )
            )
        return tuple(candidates)


def _percentile(sortedValues: list[float], quantile: float) -> float:
    if not sortedValues:
        raise CalibrationError("空序列无法计算百分位")
    if len(sortedValues) == 1:
        return sortedValues[0]
    position = (len(sortedValues) - 1) * quantile
    lower = int(position)
    upper = min(lower + 1, len(sortedValues) - 1)
    weight = position - lower
    return sortedValues[lower] * (1 - weight) + sortedValues[upper] * weight


def _avgDecimal(values: Iterable[str]) -> str:
    items = list(values)
    total = sum((Decimal(v) for v in items), Decimal("0"))
    count = len(items)
    if count == 0:
        return "0"
    return str(total / count)


def _fillRateFromTimeline(report: object) -> str:
    """从时间线估算成交率：FILLED=1，PARTIAL 按最后累计量/订单量（占位 0.5）。"""
    for timestamp in getattr(report, "timestamps", ()):
        if timestamp.phase is ExecutionPhase.Filled:
            return "1"
        if timestamp.phase is ExecutionPhase.PartialFill:
            return "0.5"
    return "0"


def _slippageFromReference(report: object, referencePrice: str, benchmarkBps: str) -> str:
    """滑点 = (成交价 - 基准价) / 基准价（基准点差基准可配置）。"""
    lastPrice = None
    for timestamp in getattr(report, "timestamps", ()):
        if timestamp.phase is ExecutionPhase.Filled and timestamp.detail:
            lastPrice = timestamp.detail
    if lastPrice is None:
        return benchmarkBps
    reference = Decimal(referencePrice)
    if reference == 0:
        return benchmarkBps
    price = Decimal(lastPrice)
    slippage = (price - reference) / reference
    return str(slippage * 10000)  # 转基点
