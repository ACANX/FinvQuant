"""P4-008 执行校准数据集和候选参数生成 Job 测试。"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from veritasquant.broker.Calibration import (
    CalibrationBucketV1,
    CalibrationDatasetBuilderV1,
    CalibrationDatasetV1,
    CalibrationError,
    CalibrationSampleV1,
    CandidateParameterGeneratorV1,
    CandidateParameterSetV1,
)

_T0 = datetime(2026, 8, 3, 2, 0, 0, tzinfo=timezone.utc)


def _sample(sampleId: str, latency: float, **overrides: object) -> CalibrationSampleV1:
    values: dict[str, object] = {
        "sampleId": sampleId,
        "symbol": "518880",
        "sessionBucket": "INTRADAY",
        "latencySeconds": latency,
        "slippage": "2.0000",
        "fillRate": "1",
        "partialFillCount": 0,
    }
    values.update(overrides)
    return CalibrationSampleV1(**values)


class TestCalibrationSample:
    def test_valid(self) -> None:
        sample = _sample("s-001", 0.3)
        assert sample.symbol == "518880"
        assert sample.sessionBucket == "INTRADAY"

    def test_requires_identity(self) -> None:
        with pytest.raises(CalibrationError):
            _sample("", 0.3)

    def test_fill_rate_range(self) -> None:
        with pytest.raises(CalibrationError):
            _sample("s-001", 0.3, fillRate="1.5")


class TestCalibrationDatasetBuilder:
    def test_build_buckets(self) -> None:
        builder = CalibrationDatasetBuilderV1()
        samples = [
            _sample("s-001", 0.1, sessionBucket="OPEN"),
            _sample("s-002", 0.2, sessionBucket="OPEN"),
            _sample("s-003", 0.3, sessionBucket="INTRADAY"),
            _sample("s-004", 0.4, sessionBucket="INTRADAY"),
        ]
        dataset = builder.build(datasetId="ds-001", samples=samples)
        assert isinstance(dataset, CalibrationDatasetV1)
        assert dataset.builderVersion == "V1"
        assert len(dataset.buckets) == 2
        openBucket = dataset.bucketFor("518880", "OPEN")
        assert openBucket is not None
        assert openBucket.sampleCount == 2
        assert openBucket.latencyP50 == pytest.approx(0.15)
        assert openBucket.latencyP95 == pytest.approx(0.195)

    def test_bucket_adequacy(self) -> None:
        builder = CalibrationDatasetBuilderV1()
        samples = [_sample(f"s-{i:03d}", 0.1) for i in range(10)]
        dataset = builder.build(datasetId="ds-001", samples=samples)
        bucket = dataset.bucketFor("518880", "INTRADAY")
        assert bucket is not None
        assert bucket.adequate is True

    def test_sample_from_diagnostics(self) -> None:
        from veritasquant.broker.Diagnostics import DiagnosticCollectorV1, ExecutionPhase

        collector = DiagnosticCollectorV1()
        collector.record(
            clientOrderId="co-001",
            brokerOrderId="broker-001",
            phase=ExecutionPhase.Submitted,
            sourceTs=_T0,
        )
        collector.record(
            clientOrderId="co-001",
            brokerOrderId="broker-001",
            phase=ExecutionPhase.Filled,
            sourceTs=_T0 + timedelta(milliseconds=500),
            detail="5.0100",
        )
        sample = CalibrationDatasetBuilderV1.sampleFromDiagnostics(
            sampleId="s-001",
            symbol="518880",
            sessionBucket="INTRADAY",
            diagnostics=collector,
            clientOrderId="co-001",
            referencePrice="5.0000",
        )
        assert sample.latencySeconds == pytest.approx(0.5)
        assert sample.fillRate == "1"
        assert sample.partialFillCount == 0
        # 滑点 (5.01-5.00)/5.00 * 10000 = 20 基点
        assert sample.slippage == "20.000"

    def test_missing_diagnostics_rejected(self) -> None:
        from veritasquant.broker.Diagnostics import DiagnosticCollectorV1

        collector = DiagnosticCollectorV1()
        with pytest.raises(CalibrationError, match="不存在"):
            CalibrationDatasetBuilderV1.sampleFromDiagnostics(
                sampleId="s-001",
                symbol="518880",
                sessionBucket="OPEN",
                diagnostics=collector,
                clientOrderId="co-unknown",
                referencePrice="5.0000",
            )


class TestCandidateParameterGenerator:
    def test_generate_variants(self) -> None:
        generator = CandidateParameterGeneratorV1()
        candidates = generator.generate(modelType="IDEAL", variants=3)
        assert len(candidates) == 3
        assert candidates[0].approved is False
        assert candidates[0].version == "V1.1"
        # 中间候选为基线
        assert candidates[1].slippageBps == "2"

    def test_deterministic(self) -> None:
        generator = CandidateParameterGeneratorV1()
        first = generator.generate(modelType="IDEAL", variants=3)
        second = generator.generate(modelType="IDEAL", variants=3)
        assert [c.slippageBps for c in first] == [c.slippageBps for c in second]

    def test_invalid_variants(self) -> None:
        generator = CandidateParameterGeneratorV1()
        with pytest.raises(CalibrationError):
            generator.generate(modelType="IDEAL", variants=0)

    def test_candidate_requires_identity(self) -> None:
        with pytest.raises(CalibrationError):
            CandidateParameterSetV1(
                candidateId="",
                version="1.0",
                modelType="IDEAL",
                slippageBps="2",
                fillRateMultiplier="1",
                latencyBudgetSeconds=0.5,
            )

    def test_bucket_requires_fields(self) -> None:
        with pytest.raises(CalibrationError):
            CalibrationBucketV1(
                symbol="",
                sessionBucket="OPEN",
                sampleCount=0,
                latencyP50=None,
                latencyP95=None,
                avgSlippage="0",
                avgFillRate="0",
                totalPartialFills=0,
            )
