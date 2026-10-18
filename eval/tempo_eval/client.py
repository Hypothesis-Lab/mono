from __future__ import annotations

import asyncio
import copy
import json
import logging
from dataclasses import dataclass, field
from threading import Lock
from time import perf_counter
from typing import Any, TypeVar

from langchain_core.messages import AIMessage, HumanMessage, SystemMessage
from langchain_openai import ChatOpenAI
from pydantic import BaseModel

ModelT = TypeVar("ModelT", bound=BaseModel)
LOGGER = logging.getLogger("tempo_eval.model")


@dataclass
class CallMetrics:
    operation: str
    duration_seconds: float
    input_tokens: int | None
    output_tokens: int | None
    outcome: str = "success"
    error: str | None = None


@dataclass
class ModelClient:
    model: str
    api_key: str
    base_url: str
    timeout_seconds: float
    app_url: str | None = None
    app_name: str | None = None
    correction_retries: int = 3
    rate_limit_wait_budget_seconds: float = 45.0
    heartbeat_seconds: float = 15.0
    _calls: list[CallMetrics] = field(default_factory=list, init=False)
    _lock: Lock = field(default_factory=Lock, init=False)

    def __post_init__(self) -> None:
        headers = {
            key: value
            for key, value in {
                "HTTP-Referer": self.app_url,
                "X-Title": self.app_name,
            }.items()
            if value
        }
        self._model = ChatOpenAI(
            api_key=self.api_key,
            base_url=self.base_url.rstrip("/"),
            model=self.model,
            timeout=self.timeout_seconds,
            max_retries=0,
            default_headers=headers or None,
        )

    @property
    def calls(self) -> list[CallMetrics]:
        with self._lock:
            return list(self._calls)

    async def complete_structured(self, operation: str, prompt: str, output_model: type[ModelT]) -> ModelT:
        schema = strict_schema(output_model.model_json_schema())
        attempt_prompt = prompt
        rate_limit_waited = 0.0
        for attempt in range(self.correction_retries + 1):
            started = perf_counter()
            response: AIMessage | None = None
            LOGGER.info(
                "model_call_start operation=%s attempt=%d/%d model=%s prompt_chars=%d schema=%s",
                operation,
                attempt + 1,
                self.correction_retries + 1,
                self.model,
                len(attempt_prompt),
                _schema_name(output_model),
            )
            try:
                async with asyncio.timeout(self.timeout_seconds):
                    response = await self._invoke_with_heartbeat(
                        self._model.bind(
                            response_format={
                                "type": "json_schema",
                                "json_schema": {
                                    "name": _schema_name(output_model),
                                    "schema": schema,
                                    "strict": True,
                                },
                            },
                        ).ainvoke(
                            [
                                SystemMessage(content="Return one valid JSON object without Markdown or explanation."),
                                HumanMessage(content=attempt_prompt),
                            ]
                        ),
                        operation,
                        attempt,
                        started,
                    )
                if not isinstance(response, AIMessage) or not isinstance(response.content, str):
                    raise ValueError("model returned a non-text response")
                result = output_model.model_validate(parse_json(response.content))
            except Exception as error:
                if isinstance(error, TimeoutError) and not str(error):
                    error = TimeoutError(f"request exceeded {self.timeout_seconds:.1f}s wall-clock deadline")
                self._record_call(self._metrics(operation, started, response, outcome="error", error=error))
                if attempt >= self.correction_retries or not is_retryable(error):
                    raise
                if is_rate_limit(error):
                    delay = retry_delay_seconds(error, attempt)
                    remaining = self.rate_limit_wait_budget_seconds - rate_limit_waited
                    if delay > remaining:
                        raise RateLimitBudgetExceeded(
                            f"rate-limit wait budget exhausted ({self.rate_limit_wait_budget_seconds:.1f}s); "
                            f"next retry requested {delay:.1f}s"
                        ) from error
                    await asyncio.sleep(delay)
                    rate_limit_waited += delay
                    continue
                attempt_prompt = correction_prompt(prompt, response.content if response else None, error)
            else:
                self._record_call(self._metrics(operation, started, response))
                return result
        raise RuntimeError("structured model call exhausted its correction retries")

    def _record_call(self, metrics: CallMetrics) -> None:
        with self._lock:
            self._calls.append(metrics)
        LOGGER.info(
            "model_call_end operation=%s outcome=%s duration=%.1fs input_tokens=%s output_tokens=%s error=%s",
            metrics.operation,
            metrics.outcome,
            metrics.duration_seconds,
            metrics.input_tokens,
            metrics.output_tokens,
            metrics.error or "-",
        )

    async def _invoke_with_heartbeat(
        self, request: Any, operation: str, attempt: int, started: float
    ) -> AIMessage:
        task = asyncio.create_task(request)
        try:
            while not task.done():
                done, _ = await asyncio.wait({task}, timeout=self.heartbeat_seconds)
                if not done:
                    LOGGER.info(
                        "model_call_waiting operation=%s attempt=%d elapsed=%.1fs",
                        operation,
                        attempt + 1,
                        perf_counter() - started,
                    )
            return task.result()
        finally:
            if not task.done():
                task.cancel()

    @staticmethod
    def _metrics(
        operation: str,
        started: float,
        response: AIMessage | None,
        *,
        outcome: str = "success",
        error: Exception | None = None,
    ) -> CallMetrics:
        usage = response.usage_metadata if response is not None else None
        usage = usage or {}
        return CallMetrics(
            operation=operation,
            duration_seconds=perf_counter() - started,
            input_tokens=usage.get("input_tokens"),
            output_tokens=usage.get("output_tokens"),
            outcome=outcome,
            error=_error_text(error) if error else None,
        )


def parse_json(content: str) -> Any:
    cleaned = content.strip()
    if cleaned.startswith("```"):
        cleaned = cleaned.removeprefix("```json").removeprefix("```").removesuffix("```").strip()
    return json.loads(cleaned)


def strict_schema(schema: dict[str, Any]) -> dict[str, Any]:
    result = copy.deepcopy(schema)
    values: list[Any] = [result]
    while values:
        value = values.pop()
        if isinstance(value, dict):
            if "properties" in value:
                value["required"] = list(value["properties"])
                value["additionalProperties"] = False
            values.extend(value.values())
        elif isinstance(value, list):
            values.extend(value)
    return result


def correction_prompt(original: str, raw_response: str | None, error: Exception) -> str:
    return f"""The previous response did not satisfy the requested output.

Original request:
{original[:30000]}

Previous response:
{(raw_response or "[no response returned]")[:16000]}

Validation or parsing error:
{_error_text(error)}

Correct the previous response. Return the complete response again as one valid JSON object only.
Do not explain the correction and do not use Markdown fences."""


def is_retryable(error: Exception) -> bool:
    if isinstance(error, TimeoutError):
        return False
    status_code = getattr(error, "status_code", None)
    return not isinstance(status_code, int) or status_code == 408 or status_code == 429 or status_code >= 500


class RateLimitBudgetExceeded(RuntimeError):
    pass


def is_rate_limit(error: Exception) -> bool:
    return getattr(error, "status_code", None) == 429 or error.__class__.__name__ == "OpenAIRateLimitError"


def retry_delay_seconds(error: Exception, attempt: int) -> float:
    response = getattr(error, "response", None)
    headers = getattr(response, "headers", {}) if response is not None else {}
    retry_after = headers.get("retry-after") if headers else None
    try:
        if retry_after is not None:
            return max(0.0, min(float(retry_after), 30.0))
    except (TypeError, ValueError):
        pass
    return float(min(2**attempt, 8))


def _error_text(error: Exception) -> str:
    return str(error)[:4000]


def _schema_name(output_model: type[BaseModel]) -> str:
    return output_model.__name__.lower()
