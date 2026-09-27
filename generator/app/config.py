import os
from dataclasses import dataclass


@dataclass(frozen=True)
class ModelConfig:
    base_url: str
    api_key: str
    model: str
    reasoning_effort: str
    timeout_seconds: float = 120.0
    max_retries: int = 2

    @classmethod
    def from_environment(
        cls,
        *,
        base_url: str | None = None,
        api_key: str | None = None,
        model: str | None = None,
        reasoning_effort: str | None = None,
    ) -> "ModelConfig":
        return cls(
            base_url=base_url or _required_environment("OPENAI_BASE_URL"),
            api_key=api_key or _required_environment("OPENAI_API_KEY"),
            model=model or _required_environment("OPENAI_MODEL"),
            reasoning_effort=reasoning_effort or _required_environment("OPENAI_REASONING_EFFORT"),
        )


@dataclass(frozen=True)
class Settings:
    database_url: str
    model_config: ModelConfig
    worker_concurrency: int
    graph_max_concurrency: int

    @classmethod
    def from_environment(cls) -> "Settings":
        database_url = os.environ.get("DATABASE_URL")
        if not database_url:
            raise RuntimeError("DATABASE_URL is required")

        worker_concurrency = int(os.environ.get("GENERATOR_WORKER_CONCURRENCY", "2"))
        if worker_concurrency < 1:
            raise ValueError("GENERATOR_WORKER_CONCURRENCY must be positive")

        graph_max_concurrency = int(os.environ.get("GENERATOR_GRAPH_MAX_CONCURRENCY", "2"))
        if graph_max_concurrency < 1:
            raise ValueError("GENERATOR_GRAPH_MAX_CONCURRENCY must be positive")

        return cls(
            database_url=database_url,
            model_config=ModelConfig.from_environment(),
            worker_concurrency=worker_concurrency,
            graph_max_concurrency=graph_max_concurrency,
        )


def _required_environment(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        raise RuntimeError(f"{name} is required")

    return value
