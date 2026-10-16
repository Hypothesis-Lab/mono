from __future__ import annotations

from pathlib import Path

PROMPT_DIR = Path(__file__).parent / "prompts"


def load_prompt(name: str) -> str:
    return (PROMPT_DIR / name).read_text(encoding="utf-8")


def render(prompt: str, values: dict[str, str]) -> str:
    rendered = prompt
    for key, value in values.items():
        rendered = rendered.replace("{{" + key + "}}", value)
    return rendered
