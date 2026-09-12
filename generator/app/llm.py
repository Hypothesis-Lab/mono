from __future__ import annotations

import httpx

from app.config import Settings


class LLMClient:
    def __init__(self, settings: Settings) -> None:
        self.base_url = settings.openai_base_url
        self.api_key = settings.openai_api_key
        self.model = settings.openai_model
        self.reasoning_effort = settings.openai_reasoning_effort

    def complete(
        self,
        system_prompt: str,
        user_prompt: str,
        temperature: float,
    ) -> str:
        if not self.base_url or not self.api_key or not self.model:
            raise RuntimeError(
                "OPENAI_BASE_URL, OPENAI_API_KEY, and OPENAI_MODEL are required"
            )

        response = httpx.post(
            self.base_url,
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
                "temperature": temperature,
                "reasoning_effort": self.reasoning_effort,
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
            raise ValueError(
                "OpenAI-compatible response did not contain message content"
            ) from error

        if not isinstance(content, str):
            raise ValueError("OpenAI-compatible response content was not a string")
        return content
