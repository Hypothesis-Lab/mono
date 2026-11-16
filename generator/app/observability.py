from __future__ import annotations

from collections.abc import Iterator
from contextlib import contextmanager
from contextvars import ContextVar
from dataclasses import dataclass
from threading import Lock
from time import perf_counter
from typing import Protocol

from prometheus_client import Counter, Histogram

_operation_id: ContextVar[str] = ContextVar("generator_operation_id", default="unknown")


@dataclass(frozen=True)
class Measurement:
    kind: str
    name: str
    model: str | None
    duration_ms: float
    operation_id: str | None = None
    input_tokens: int | None = None
    output_tokens: int | None = None
    provider_cost: float | None = None
    outcome: str = "success"


class Observer(Protocol):
    def observe(self, measurement: Measurement) -> None: ...


class RunCollector:
    def __init__(self) -> None:
        self._measurements: list[Measurement] = []
        self._lock = Lock()

    def observe(self, measurement: Measurement) -> None:
        with self._lock:
            self._measurements.append(measurement)

    def snapshot(self) -> list[Measurement]:
        with self._lock:
            return list(self._measurements)


class PrometheusObserver:
    def __init__(self) -> None:
        self.calls = Counter("generator_calls_total", "Generator model calls", ("model", "outcome"))
        self.tokens = Counter("generator_tokens_total", "Provider-reported model tokens", ("model", "direction"))
        self.failures = Counter("generator_failures_total", "Failed generator operations", ("kind", "stage", "model"))
        self.durations = Histogram(
            "generator_operation_duration_ms",
            "Generator operation duration in milliseconds",
            ("kind", "stage", "model", "outcome"),
        )

    def observe(self, measurement: Measurement) -> None:
        model = measurement.model or "none"
        self.durations.labels(measurement.kind, measurement.name, model, measurement.outcome).observe(
            measurement.duration_ms
        )
        if measurement.outcome == "error":
            self.failures.labels(measurement.kind, measurement.name, model).inc()

        if measurement.kind == "model_call":
            self.calls.labels(model, measurement.outcome).inc()
            if measurement.input_tokens is not None:
                self.tokens.labels(model, "input").inc(measurement.input_tokens)

            if measurement.output_tokens is not None:
                self.tokens.labels(model, "output").inc(measurement.output_tokens)


@contextmanager
def observe_operation(observer: Observer | None, operation_id: str) -> Iterator[None]:
    context_token = _operation_id.set(operation_id)
    started = perf_counter()
    outcome = "success"
    try:
        yield
    except Exception:
        outcome = "error"
        raise
    finally:
        _operation_id.reset(context_token)
        if observer is not None:
            observer.observe(
                Measurement(
                    kind="node",
                    name=operation_id.split(":", maxsplit=1)[0],
                    model=None,
                    duration_ms=(perf_counter() - started) * 1000,
                    operation_id=operation_id,
                    outcome=outcome,
                )
            )


def current_operation_id() -> str:
    return _operation_id.get()
