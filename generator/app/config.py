import os
from dataclasses import dataclass


@dataclass(frozen=True)
class ServerConfig:
    host: str
    port: int

    @classmethod
    def from_environment(cls) -> "ServerConfig":
        port = int(_required_environment("PORT"))
        if not 1 <= port <= 65535:
            raise ValueError("PORT must be between 1 and 65535")

        return cls(host=_required_environment("HOST"), port=port)


@dataclass(frozen=True)
class CodexConfig:
    proxy_base_url: str
    proxy_key: str
    model: str
    reasoning_effort: str
    timeout_seconds: float
    max_retries: int

    @classmethod
    def from_environment(cls) -> "CodexConfig":
        return cls(
            proxy_base_url=_required_environment("CODEX_PROXY_BASE_URL"),
            proxy_key=_required_environment("CODEX_PROXY_KEY"),
            model=_required_environment("CODEX_MODEL"),
            reasoning_effort=_required_environment("CODEX_REASONING_EFFORT"),
            timeout_seconds=float(_required_environment("CODEX_TIMEOUT_SECONDS")),
            max_retries=int(_required_environment("CODEX_MAX_RETRIES")),
        )


@dataclass(frozen=True)
class Settings:
    database_url: str
    codex_config: CodexConfig
    worker_concurrency: int
    graph_max_concurrency: int

    @classmethod
    def from_environment(cls) -> "Settings":
        database_url = _required_environment("DATABASE_URL")

        worker_concurrency = int(_required_environment("WORKER_CONCURRENCY"))
        if worker_concurrency < 1:
            raise ValueError("WORKER_CONCURRENCY must be positive")

        graph_max_concurrency = int(_required_environment("GRAPH_MAX_CONCURRENCY"))
        if graph_max_concurrency < 1:
            raise ValueError("GRAPH_MAX_CONCURRENCY must be positive")

        return cls(
            database_url=database_url,
            codex_config=CodexConfig.from_environment(),
            worker_concurrency=worker_concurrency,
            graph_max_concurrency=graph_max_concurrency,
        )


def _required_environment(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        raise RuntimeError(f"{name} is required")

    return value
