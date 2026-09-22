from __future__ import annotations

from typing import Any

import httpx


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

        try:
            content = body["choices"][0]["message"]["content"]
        except (IndexError, KeyError, TypeError) as error:
            raise ValueError("OpenAI-compatible response did not contain message content") from error

        if not isinstance(content, str):
            raise ValueError("OpenAI-compatible response content was not a string")

        return content
