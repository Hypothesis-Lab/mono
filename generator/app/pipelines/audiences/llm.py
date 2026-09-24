from __future__ import annotations

from dataclasses import dataclass
from time import perf_counter
from typing import Any, Protocol, cast
from urllib.parse import urlsplit, urlunsplit

from langchain_core.messages import AIMessage, HumanMessage, SystemMessage
from langchain_openai import ChatOpenAI

from app.observability import Measurement, Observer, current_operation_id


@dataclass(frozen=True)
class ModelConfig:
    base_url: str
    api_key: str
    model: str
    reasoning_effort: str
    timeout_seconds: float = 120.0
    max_retries: int = 2


class AudienceModel(Protocol):
    @property
    def model(self) -> str: ...

    @property
    def reasoning_effort(self) -> str: ...

    def complete(self, system_prompt: str, user_prompt: str, response_schema: dict[str, Any]) -> str: ...


class ModelClient:
    def __init__(self, config: ModelConfig, observer: Observer | None = None) -> None:
        self.config = config
        self.observer = observer
        self.reasoning_effort = config.reasoning_effort
        self._model = ChatOpenAI(
            base_url=_openai_base_url(config.base_url),
            api_key=cast(Any, config.api_key),
            model=config.model,
            reasoning_effort=config.reasoning_effort,
            timeout=config.timeout_seconds,
            max_retries=config.max_retries,
        )

    @property
    def model(self) -> str:
        return self.config.model

    def complete(self, system_prompt: str, user_prompt: str, response_schema: dict[str, Any]) -> str:
        started = perf_counter()
        response: AIMessage | None = None
        outcome = "success"
        try:
            response_format = {
                "type": "json_schema",
                "json_schema": {
                    "name": "generation_output",
                    "schema": response_schema,
                    "strict": True,
                },
            }
            message = self._model.bind(response_format=response_format).invoke(
                [SystemMessage(content=system_prompt), HumanMessage(content=user_prompt)]
            )
            if not isinstance(message, AIMessage):
                raise TypeError("chat model returned a non-assistant message")

            response = message
            if not isinstance(message.content, str):
                raise ValueError("model returned non-text structured output")

            return message.content
        except Exception:
            outcome = "error"
            raise
        finally:
            if self.observer is not None:
                usage = response.usage_metadata if response is not None else None
                self.observer.observe(
                    Measurement(
                        kind="model_call",
                        name=current_operation_id().split(":", maxsplit=1)[0],
                        model=self.config.model,
                        duration_seconds=perf_counter() - started,
                        operation_id=current_operation_id(),
                        input_tokens=usage.get("input_tokens") if usage else None,
                        output_tokens=usage.get("output_tokens") if usage else None,
                        outcome=outcome,
                    )
                )


def _openai_base_url(value: str) -> str:
    parts = urlsplit(value.rstrip("/"))
    if parts.path.endswith("/v1"):
        return value.rstrip("/")

    return urlunsplit((parts.scheme, parts.netloc, f"{parts.path}/v1", parts.query, parts.fragment))
