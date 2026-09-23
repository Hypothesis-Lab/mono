from __future__ import annotations

from time import perf_counter
from typing import Any, Protocol

import httpx

from app.pipeline.telemetry import record_call


class LLM(Protocol):
    model: str
    reasoning_effort: str

    def complete(self, system_prompt: str, user_prompt: str, response_schema: dict[str, Any]) -> str: ...


class LLMClient:
    def __init__(self, base_url: str, api_key: str, model: str, reasoning_effort: str) -> None:
        self.base_url = base_url
        self.api_key = api_key
        self.model = model
        self.reasoning_effort = reasoning_effort

    def complete(
        self,
        system_prompt: str,
        user_prompt: str,
        response_schema: dict[str, Any],
    ) -> str:
        started = perf_counter()
        usage: dict[str, int] | None = None
        try:
            response = httpx.post(
                f"{self.base_url.rstrip('/')}/v1/chat/completions",
                headers={
                    "Authorization": f"Bearer {self.api_key}",
                    "Content-Type": "application/json",
                },
                json={
                    "model": self.model,
                    "messages": [
                        {"role": "system", "content": system_prompt},
                        {"role": "user", "content": user_prompt},
                    ],
                    "reasoning_effort": self.reasoning_effort,
                    "response_format": {
                        "type": "json_schema",
                        "json_schema": {
                            "name": "generation_output",
                            "schema": response_schema,
                            "strict": True,
                        },
                    },
                },
                timeout=httpx.Timeout(120.0, connect=10.0),
            )
            if response.is_error:
                body = response.text[: 64 << 10].strip()
                message = f"LLM returned {response.status_code} {response.reason_phrase}"
                if body:
                    message += f": {body}"

                raise RuntimeError(message)

            body = response.json()
            if isinstance(body.get("usage"), dict):
                usage = body["usage"]

            try:
                content = body["choices"][0]["message"]["content"]
            except (IndexError, KeyError, TypeError) as error:
                raise ValueError("OpenAI-compatible response did not contain message content") from error

            if not isinstance(content, str):
                raise ValueError("OpenAI-compatible response content was not a string")

            record_call(self.model, (perf_counter() - started) * 1000, usage, None)
            return content
        except Exception as error:
            record_call(self.model, (perf_counter() - started) * 1000, usage, type(error).__name__)
            raise
