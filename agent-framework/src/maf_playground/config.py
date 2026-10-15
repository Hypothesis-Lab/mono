"""Application configuration loaded from the environment and project .env file."""

from __future__ import annotations

import math
import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Literal

from dotenv import load_dotenv

PROJECT_ROOT = Path(__file__).resolve().parents[2]
load_dotenv(PROJECT_ROOT / ".env", override=False)

Provider = Literal["openrouter", "opencode"]


@dataclass(frozen=True)
class Settings:
    """Parsed settings shared by the launcher and workflow."""

    llm_provider: Provider
    openrouter_api_key: str | None = field(repr=False)
    openrouter_model: str
    openrouter_chat_completions_url: str
    opencode_model: str
    llm_timeout_seconds: float


def _positive_float(name: str, default: str) -> float:
    raw_value = os.environ.get(name, default).strip()
    try:
        value = float(raw_value)
    except ValueError as error:
        raise ValueError(f"{name} must be a positive number") from error
    if not math.isfinite(value) or value <= 0:
        raise ValueError(f"{name} must be a positive number")
    return value


def load_settings() -> Settings:
    """Read and validate provider, model, credential, and timeout settings."""
    provider = os.environ.get("LLM_PROVIDER", "openrouter").strip().lower()
    if provider not in {"openrouter", "opencode"}:
        raise ValueError("LLM_PROVIDER must be either 'openrouter' or 'opencode'")

    api_key = os.environ.get("OPENROUTER_API_KEY", "").strip() or None
    return Settings(
        llm_provider=provider,  # type: ignore[arg-type]
        openrouter_api_key=api_key,
        openrouter_model=os.environ.get("OPENROUTER_MODEL", "inclusionai/ling-3.0-flash").strip(),
        openrouter_chat_completions_url=os.environ.get(
            "OPENROUTER_CHAT_COMPLETIONS_URL",
            "https://openrouter.ai/api/v1/chat/completions",
        ).strip(),
        opencode_model=os.environ.get("OPENCODE_MODEL", "opencode/cat-jam-free").strip(),
        llm_timeout_seconds=_positive_float("LLM_TIMEOUT_SECONDS", "300"),
    )


settings = load_settings()
