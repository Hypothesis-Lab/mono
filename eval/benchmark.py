from __future__ import annotations

import argparse
import asyncio
import json
import logging
import os
import random
import shutil
from pathlib import Path
from typing import Any

from tempo_eval.client import ModelClient
from tempo_eval.models import Source, StabilityEvaluation
from tempo_eval.runner import ResultWriter, load_models, load_sources, run_one_model, slugify


async def run_benchmark(args: argparse.Namespace) -> None:
    if not args.api_key:
        raise SystemExit("OPENROUTER_API_KEY or --api-key is required")
    payload = load_payload(args.output)
    models = load_models(args.models, args.models_file)
    models = [model for model in models if model not in set(args.exclude_model)]
    if not models:
        raise SystemExit("no models remain after exclusions")
    if args.force:
        reset_models(payload, models, args.checkpoints)
    elif args.retry_failed:
        reset_failed(payload, models)
    payload.setdefault("results", [])
    payload.setdefault("evaluations", [])
    payload["pipeline"] = "product"
    payload["benchmark"] = {
        "max_runs": args.runs,
        "target_product": args.target_product,
        "as_of_date": args.as_of_date,
        "pipeline": "product",
    }
    writer = ResultWriter(args.output, {}, payload)
    sources = load_sources(args.input_dir)
    await asyncio.gather(*(run_model_slots(model, sources, args=args, writer=writer) for model in models))


async def run_model_slots(model: str, sources: list[Source], *, args: argparse.Namespace, writer: ResultWriter) -> None:
    tasks = []
    for run_number in range(1, args.runs + 1):
        if has_run(writer.payload, model, run_number):
            continue
        checkpoint_path = args.checkpoints / slugify(model) / f"run-{run_number}.sqlite"
        checkpoint_path.parent.mkdir(parents=True, exist_ok=True)
        ordered_sources = sources_for_run(sources, run_number)
        tasks.append(
            run_one_model(
                model,
                ordered_sources,
                target_product=args.target_product,
                as_of_date=args.as_of_date,
                api_key=args.api_key,
                base_url=args.base_url,
                checkpoint_dir=args.checkpoints,
                timeout_seconds=args.timeout,
                step_retries=args.step_retries,
                app_url=args.app_url,
                app_name=args.app_name,
                run_id=f"{args.run_id}:{slugify(model)}:run-{run_number}",
                writer=writer,
                run_number=run_number,
                checkpoint_path=checkpoint_path,
                resume=checkpoint_path.exists(),
                source_order=[source.source for source in ordered_sources],
                pipeline="product",
                rate_limit_wait_budget_seconds=args.rate_limit_budget,
            )
        )
    if tasks:
        await asyncio.gather(*tasks)
    runs = completed_runs(writer.payload, model, args.runs)
    if len(runs) == args.runs and not has_evaluation(writer.payload, model):
        await evaluate_stability(model, runs, sources, args=args, writer=writer)


async def evaluate_stability(
    model: str, runs: list[dict[str, Any]], sources: list[Source], *, args: argparse.Namespace, writer: ResultWriter
) -> None:
    client = ModelClient(
        args.evaluation_model,
        args.api_key,
        args.base_url,
        args.timeout,
        args.app_url,
        args.app_name,
        correction_retries=args.step_retries,
        rate_limit_wait_budget_seconds=args.rate_limit_budget,
    )
    entry: dict[str, Any] = {
        "model": model,
        "evaluation_model": args.evaluation_model,
        "run_numbers": [run["run_number"] for run in runs],
        "status": "failed",
    }
    try:
        evaluation = await client.complete_structured(
            "benchmark_evaluation", product_benchmark_prompt(model, runs, sources), StabilityEvaluation
        )
        entry.update({"status": "completed", "evaluation": evaluation.model_dump(mode="json")})
    except Exception as error:
        entry["error"] = f"{type(error).__name__}: {error}"
    entry["calls"] = [call.__dict__ for call in client.calls]
    await writer.append_evaluation(entry)


def product_benchmark_prompt(model: str, runs: list[dict[str, Any]], sources: list[Source]) -> str:
    passports = {
        f"run_{run['run_number']}": {
            "source_order": run.get("source_order", []),
            "passport": run["result"]["passport"],
            "validation": run["result"]["validation"],
        }
        for run in runs
    }
    return f"""You are an independent stability evaluator for product extraction.

Evaluate C9, `Устойчивость результата`, across three completed runs of `{model}`.
Source order was intentionally varied. Compare substance, conditions, numbers,
evidence, conflicts, gaps, and product boundaries. Local IDs, wording, and
record order may differ only when their meaning is preserved.

Return a StabilityEvaluation JSON object. Use mode=benchmark; criterion id=c9,
name=`Устойчивость результата`, and weight=5. Explain all material drift in
run_comparison. Do not claim acceptance merely because outputs look similar.

RUNS:
{json.dumps(passports, ensure_ascii=False, indent=2)}

SOURCE MATERIALS:
{json.dumps([source.model_dump(mode="json") for source in sources], ensure_ascii=False, indent=2)}
"""


def sources_for_run(sources: list[Source], run_number: int) -> list[Source]:
    if run_number == 1:
        return list(sources)
    if run_number == 2:
        return list(reversed(sources))
    shuffled = list(sources)
    random.Random(42).shuffle(shuffled)
    return shuffled


def load_payload(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise SystemExit(f"cannot read benchmark output {path}: {error}") from error
    if not isinstance(payload, dict):
        raise SystemExit(f"benchmark output must contain a JSON object: {path}")
    return payload


def reset_models(payload: dict[str, Any], models: list[str], checkpoints: Path) -> None:
    selected = set(models)
    payload["results"] = [item for item in payload.get("results", []) if item.get("model") not in selected]
    payload["evaluations"] = [item for item in payload.get("evaluations", []) if item.get("model") not in selected]
    for model in models:
        shutil.rmtree(checkpoints / slugify(model), ignore_errors=True)


def reset_failed(payload: dict[str, Any], models: list[str]) -> None:
    selected = set(models)
    payload["results"] = [
        item
        for item in payload.get("results", [])
        if not (item.get("model") in selected and item.get("status") == "failed")
    ]
    payload["evaluations"] = [
        item
        for item in payload.get("evaluations", [])
        if not (item.get("model") in selected and item.get("status") == "failed")
    ]


def has_run(payload: dict[str, Any], model: str, run_number: int) -> bool:
    return any(
        item.get("model") == model and item.get("run_number") == run_number for item in payload.get("results", [])
    )


def completed_runs(payload: dict[str, Any], model: str, max_runs: int) -> list[dict[str, Any]]:
    return sorted(
        [
            item
            for item in payload.get("results", [])
            if item.get("model") == model
            and item.get("run_number", 1) <= max_runs
            and item.get("status") == "completed"
            and isinstance(item.get("result", {}).get("passport"), dict)
        ],
        key=lambda item: item["run_number"],
    )


def has_evaluation(payload: dict[str, Any], model: str) -> bool:
    return any(
        item.get("model") == model and item.get("status") == "completed" for item in payload.get("evaluations", [])
    )


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Run the product pipeline up to three times and benchmark stability.")
    parser.add_argument("--models", nargs="*", default=[])
    parser.add_argument("--models-file", type=Path)
    parser.add_argument("--exclude-model", action="append", default=[])
    parser.add_argument("--input-dir", type=Path, default=Path(__file__).resolve().parent.parent / "n8n/product")
    parser.add_argument("--output", type=Path, default=Path("results/benchmark.json"))
    parser.add_argument("--checkpoints", type=Path, default=Path("checkpoints/benchmark"))
    parser.add_argument("--runs", type=int, default=3)
    parser.add_argument("--force", action="store_true")
    parser.add_argument("--retry-failed", action="store_true")
    parser.add_argument("--target-product", default="Tempo")
    parser.add_argument("--as-of-date")
    parser.add_argument("--api-key", default=os.environ.get("OPENROUTER_API_KEY"))
    parser.add_argument("--base-url", default=os.environ.get("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"))
    parser.add_argument(
        "--evaluation-model", default=os.environ.get("OPENROUTER_EVALUATION_MODEL", os.environ.get("OPENROUTER_MODEL"))
    )
    parser.add_argument("--timeout", type=float, default=15.0)
    parser.add_argument("--rate-limit-budget", type=float, default=45.0)
    parser.add_argument("--step-retries", type=int, default=3)
    parser.add_argument("--app-url", default=os.environ.get("OPENROUTER_APP_URL"))
    parser.add_argument("--app-name", default="Tempo extraction benchmark")
    parser.add_argument("--run-id", default="benchmark")
    return parser


def main() -> None:
    logging.basicConfig(
        level=os.environ.get("TEMPO_LOG_LEVEL", "INFO").upper(),
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
        force=True,
    )
    args = build_parser().parse_args()
    if not args.models and not args.models_file and Path("models.txt").exists():
        args.models_file = Path("models.txt")
    if not 1 <= args.runs <= 3:
        raise SystemExit("--runs must be between 1 and 3")
    if args.force and args.retry_failed:
        raise SystemExit("use either --force or --retry-failed, not both")
    if not args.evaluation_model:
        raise SystemExit("--evaluation-model or OPENROUTER_MODEL is required")
    if args.rate_limit_budget < 0:
        raise SystemExit("--rate-limit-budget must be non-negative")
    asyncio.run(run_benchmark(args))


if __name__ == "__main__":
    main()
