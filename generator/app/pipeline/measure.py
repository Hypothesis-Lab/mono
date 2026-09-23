"""Opt-in, database-free pipeline measurements using the production graph."""

from dataclasses import dataclass
from typing import Any

from app.llm_client import LLM
from app.models import InputBundle
from app.pipeline.runner import PipelineRunner
from app.pipeline.storage import InMemoryJobStore
from app.pipeline.telemetry import LLMCall, record_calls
from app.workflow import Workflow


@dataclass(frozen=True)
class Measurement:
    result: dict[str, Any]
    calls: list[LLMCall]


def measure(bundle: InputBundle, llm: LLM, *, max_attempts: int = 1) -> Measurement:
    """Compare LLM configurations without DB writes; one attempt disables retries."""
    runner = PipelineRunner(Workflow(llm), InMemoryJobStore(bundle), max_attempts=max_attempts)
    last_error = None
    with record_calls() as calls:
        while (outcome := runner.execute_next()) is not None:
            if outcome.result is not None:
                return Measurement(result=outcome.result, calls=calls)

            if outcome.error:
                last_error = outcome.error

    raise RuntimeError(f"generation did not complete: {last_error or 'no runnable job'}")
