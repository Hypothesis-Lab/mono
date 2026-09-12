import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    database_url: str
    openai_base_url: str | None
    openai_api_key: str | None
    openai_model: str | None
    openai_reasoning_effort: str | None

    @classmethod
    def from_environment(cls) -> "Settings":
        database_url = os.environ.get("DATABASE_URL")
        if not database_url:
            raise RuntimeError("DATABASE_URL is required")

        return cls(
            database_url=database_url,
            openai_base_url=os.environ.get("OPENAI_BASE_URL"),
            openai_api_key=os.environ.get("OPENAI_API_KEY"),
            openai_model=os.environ.get("OPENAI_MODEL"),
            openai_reasoning_effort=os.environ.get("OPENAI_REASONING_EFFORT"),
        )
