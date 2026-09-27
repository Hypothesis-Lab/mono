from dataclasses import dataclass


@dataclass
class ModelCallContext:
    raw_response: str | None = None
