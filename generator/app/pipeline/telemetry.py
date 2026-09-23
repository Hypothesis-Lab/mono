"""Per-run LLM measurements; context-local so concurrent jobs do not mix calls."""

from collections.abc import Iterator
from contextlib import contextmanager
from contextvars import ContextVar
from dataclasses import dataclass


@dataclass(frozen=True)
class LLMCall:
    stage: str | None
    model: str
    milliseconds: float
    input_tokens: int | None
    output_tokens: int | None
    error: str | None


_calls: ContextVar[list[LLMCall] | None] = ContextVar("llm_calls", default=None)
_stage: ContextVar[str | None] = ContextVar("pipeline_stage", default=None)


@contextmanager
def record_calls() -> Iterator[list[LLMCall]]:
    calls: list[LLMCall] = []
    token = _calls.set(calls)
    try:
        yield calls
    finally:
        _calls.reset(token)


@contextmanager
def at_stage(node_id: str) -> Iterator[None]:
    token = _stage.set(node_id)
    try:
        yield
    finally:
        _stage.reset(token)


def record_call(model: str, milliseconds: float, usage: dict[str, int] | None, error: str | None) -> None:
    calls = _calls.get()
    if calls is not None:
        calls.append(
            LLMCall(
                stage=_stage.get(),
                model=model,
                milliseconds=milliseconds,
                input_tokens=usage.get("prompt_tokens") if usage else None,
                output_tokens=usage.get("completion_tokens") if usage else None,
                error=error,
            )
        )
