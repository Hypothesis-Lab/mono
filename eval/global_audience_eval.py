from __future__ import annotations

import argparse
import asyncio
import json
import os
from pathlib import Path
from typing import Any

from tempo_eval.client import ModelClient
from tempo_eval.models import GlobalAudienceComparison


def first_completed_results(payload: dict[str, Any]) -> list[dict[str, Any]]:
    selected: dict[str, dict[str, Any]] = {}
    for item in payload.get("results", []):
        if item.get("status") != "completed" or not isinstance(item.get("result"), dict):
            continue
        model = item.get("model")
        if not isinstance(model, str):
            continue
        current = selected.get(model)
        if current is None or item.get("run_number", 1) < current.get("run_number", 1):
            selected[model] = item
    return [selected[model] for model in sorted(selected)]


def comparison_input(item: dict[str, Any]) -> dict[str, Any]:
    result = item.get("result", {})
    return {
        "model": item.get("model"),
        "run_number": item.get("run_number", 1),
        "audience": result.get("audience"),
        "query": result.get("query"),
        "calculation": result.get("calculation"),
        "validation": result.get("validation"),
        "review": result.get("review"),
    }


def build_prompt(outputs: list[dict[str, Any]]) -> str:
    return f"""Ты сравниваешь предоставленные результаты аудиторного пайплайна разных моделей.

Оцени согласованность и различия по правилам запроса, окнам наблюдения,
размеру и доле сегмента, помесячным счётчикам, агрегированным характеристикам,
статусам проверок, ограничениям и интерпретациям кандидатов.

Детерминированные поля расчёта являются фактической основой сравнения.
Различай общие факты, модельные интерпретации, существенные расхождения и
неподтверждённые выводы. Выделяй одинаковые кандидатские правила, уникальные
правила моделей, несогласованные размеры или периоды, коллинеарные признаки,
точное попадание в minimum_segment_size и необычные статусы проверок.

Верни GlobalAudienceComparison с кратким резюме, консенсусом, различиями между
моделями, несогласованностями, общими ограничениями и следующими действиями.

РЕЗУЛЬТАТЫ:
{json.dumps(outputs, ensure_ascii=False, indent=2)}
"""


async def evaluate(args: argparse.Namespace) -> None:
    if not args.api_key:
        raise SystemExit("OPENROUTER_API_KEY or --api-key is required")
    try:
        payload = json.loads(args.results.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise SystemExit(f"cannot read results: {error}") from error
    selected = first_completed_results(payload)
    if len(selected) < 2:
        raise SystemExit("at least two models need completed results")
    outputs = [comparison_input(item) for item in selected]
    client = ModelClient(
        args.evaluator_model,
        args.api_key,
        args.base_url,
        args.timeout,
        args.app_url,
        args.app_name,
        correction_retries=args.step_retries,
        rate_limit_wait_budget_seconds=args.rate_limit_budget,
    )
    evaluation = await client.complete_structured(
        "audience_global_comparison",
        build_prompt(outputs),
        GlobalAudienceComparison,
    )
    payload["global_evaluation"] = {
        "status": "completed",
        "evaluator_model": args.evaluator_model,
        "selected_models": [item["model"] for item in outputs],
        "selected_run_numbers": [item["run_number"] for item in outputs],
        "evaluation": evaluation.model_dump(mode="json"),
        "calls": [call.__dict__ for call in client.calls],
    }
    output = args.output or args.results
    output.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"Wrote {output}")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Compare the first completed audience result from each model.")
    parser.add_argument("--results", type=Path, required=True)
    parser.add_argument("--output", type=Path)
    parser.add_argument(
        "--evaluator-model",
        default=os.environ.get("OPENROUTER_EVALUATION_MODEL", os.environ.get("OPENROUTER_MODEL")),
    )
    parser.add_argument("--api-key", default=os.environ.get("OPENROUTER_API_KEY"))
    parser.add_argument("--base-url", default=os.environ.get("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"))
    parser.add_argument("--timeout", type=float, default=120.0)
    parser.add_argument("--rate-limit-budget", type=float, default=45.0)
    parser.add_argument("--step-retries", type=int, default=1)
    parser.add_argument("--app-url", default=os.environ.get("OPENROUTER_APP_URL"))
    parser.add_argument("--app-name", default="Tempo global audience comparison")
    return parser


if __name__ == "__main__":
    asyncio.run(evaluate(build_parser().parse_args()))
