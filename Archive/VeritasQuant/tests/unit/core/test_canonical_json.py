from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from veritasquant.core.CanonicalJson import canonicalHash, canonicalJsonBytes
from veritasquant.core.Time import TsPrecision


def test_canonical_json_sorts_fields_and_normalizes_decimal_utc_and_null() -> None:
    first = {"z": None, "amount": Decimal("10.00"), "ts": datetime(2026, 7, 31, 8, 15, 30, tzinfo=timezone.utc)}
    second = {"ts": datetime(2026, 7, 31, 8, 15, 30, tzinfo=timezone.utc), "amount": Decimal("10"), "z": None}
    assert canonicalJsonBytes(first, TsPrecision.Second) == b'{"amount":"10","ts":"2026-07-31T08:15:30Z","z":null}'
    assert canonicalHash(first, TsPrecision.Second) == canonicalHash(second, TsPrecision.Second)


def test_float_is_not_allowed_in_identity_hash_paths() -> None:
    with pytest.raises(TypeError):
        canonicalHash({"amount": 1.25})
