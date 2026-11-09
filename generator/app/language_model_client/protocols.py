from __future__ import annotations

from typing import Any, Protocol


class LanguageModelClient(Protocol):
    @property
    def model(self) -> str: ...

    @property
    def reasoning_effort(self) -> str: ...

    def complete(self, system_prompt: str, user_prompt: str, response_schema: dict[str, Any]) -> str: ...
