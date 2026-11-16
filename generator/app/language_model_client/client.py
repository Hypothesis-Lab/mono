from __future__ import annotations

from time import perf_counter
from typing import Any, cast
from urllib.parse import urlsplit, urlunsplit

from langchain_core.callbacks import BaseCallbackHandler
from langchain_core.messages import AIMessage, HumanMessage, SystemMessage
from langchain_core.outputs import LLMResult
from langchain_openai import ChatOpenAI

from app.config import CodexConfig
from app.observability import Measurement, Observer, current_operation_id


class ResponseUsageCapture(BaseCallbackHandler):
    def __init__(self) -> None:
        self.token_usage: dict[str, Any] = {}

    def on_llm_end(self, response: LLMResult, **kwargs: Any) -> None:
        if response.llm_output is None:
            return

        usage = response.llm_output.get("token_usage")
        if isinstance(usage, dict):
            self.token_usage = usage


class CodexClient:
    def __init__(self, config: CodexConfig, observer: Observer | None = None) -> None:
        self.config = config
        self.observer = observer
        self.reasoning_effort = config.reasoning_effort
        model_options: dict[str, Any] = {}
        if config.reasoning_effort:
            model_options["reasoning_effort"] = config.reasoning_effort

        self._model = ChatOpenAI(
            base_url=_codex_base_url(config.proxy_base_url),
            api_key=cast(Any, config.proxy_key),
            model=config.model,
            timeout=config.timeout_seconds,
            max_retries=config.max_retries,
            **model_options,
        )

    @property
    def model(self) -> str:
        return self.config.model

    def complete(self, system_prompt: str, user_prompt: str, response_schema: dict[str, Any]) -> str:
        started = perf_counter()
        response: AIMessage | None = None
        usage_capture = ResponseUsageCapture()
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
                [SystemMessage(content=system_prompt), HumanMessage(content=user_prompt)],
                config={"callbacks": [usage_capture]},
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
                cost = usage_capture.token_usage.get("cost")
                self.observer.observe(
                    Measurement(
                        kind="model_call",
                        name=current_operation_id().split(":", maxsplit=1)[0],
                        model=self.config.model,
                        duration_ms=(perf_counter() - started) * 1000,
                        operation_id=current_operation_id(),
                        input_tokens=usage.get("input_tokens") if usage else None,
                        output_tokens=usage.get("output_tokens") if usage else None,
                        provider_cost=cost if isinstance(cost, (int, float)) else None,
                        outcome=outcome,
                    )
                )


def _codex_base_url(value: str) -> str:
    parts = urlsplit(value.rstrip("/"))
    if parts.path.endswith("/chat/completions"):
        return urlunsplit(
            (parts.scheme, parts.netloc, parts.path[: -len("/chat/completions")], parts.query, parts.fragment)
        )

    if parts.path.endswith("/v1"):
        return value.rstrip("/")

    return urlunsplit((parts.scheme, parts.netloc, f"{parts.path}/v1", parts.query, parts.fragment))
