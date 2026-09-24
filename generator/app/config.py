import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    database_url: str
    openai_base_url: str
    openai_api_key: str
    openai_model: str
    openai_reasoning_effort: str
    worker_concurrency: int
    graph_max_concurrency: int

    @classmethod
    def from_environment(cls) -> "Settings":
        database_url = os.environ.get("DATABASE_URL")
        if not database_url:
            raise RuntimeError("DATABASE_URL is required")

        openai_base_url = os.environ.get("OPENAI_BASE_URL")
        openai_api_key = os.environ.get("OPENAI_API_KEY")
        openai_model = os.environ.get("OPENAI_MODEL")
        openai_reasoning_effort = os.environ.get("OPENAI_REASONING_EFFORT")
        if not openai_base_url:
            raise RuntimeError("OPENAI_BASE_URL is required")

        if not openai_api_key:
            raise RuntimeError("OPENAI_API_KEY is required")

        if not openai_model:
            raise RuntimeError("OPENAI_MODEL is required")

        if not openai_reasoning_effort:
            raise RuntimeError("OPENAI_REASONING_EFFORT is required")

        worker_concurrency = int(os.environ.get("GENERATOR_WORKER_CONCURRENCY", "2"))
        if worker_concurrency < 1:
            raise ValueError("GENERATOR_WORKER_CONCURRENCY must be positive")

        graph_max_concurrency = int(os.environ.get("GENERATOR_GRAPH_MAX_CONCURRENCY", "2"))
        if graph_max_concurrency < 1:
            raise ValueError("GENERATOR_GRAPH_MAX_CONCURRENCY must be positive")

        return cls(
            database_url=database_url,
            openai_base_url=openai_base_url,
            openai_api_key=openai_api_key,
            openai_model=openai_model,
            openai_reasoning_effort=openai_reasoning_effort,
            worker_concurrency=worker_concurrency,
            graph_max_concurrency=graph_max_concurrency,
        )
