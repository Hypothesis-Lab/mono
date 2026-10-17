from __future__ import annotations

import argparse
import asyncio
import json
import os
import uuid
from pathlib import Path
from typing import Any

from tempo_eval.models import EvaluationCriterion, StabilityEvaluation
from tempo_eval.runner import (
    ResultWriter,
    compact_product_passport,
    load_models,
    load_product_passport,
    run_one_model,
    slugify,
)


async def run_benchmark(args: argparse.Namespace) -> None:
    if not args.api_key:
        raise SystemExit("OPENROUTER_API_KEY or --api-key is required")
    if args.runs < 1:
        raise SystemExit("--runs must be at least 1")
    models = load_models(args.models, args.models_file)
    if not models:
        raise SystemExit("at least one model is required")
    product_passport = compact_product_passport(load_product_passport(args.product_result, args.product_model))
    payload = load_payload(args.output)
    payload.update(
        {
            "pipeline": "audience",
            "target_product": args.target_product,
            "as_of_date": args.as_of_date,
            "benchmark": {
                "max_runs": args.runs,
                "pipeline": "audience",
                "stability_method": "deterministic aggregate comparison",
            },
        }
    )
    payload.setdefault("results", [])
    payload.setdefault("evaluations", [])
    writer = ResultWriter(args.output, {}, payload)
    await asyncio.gather(
        *(
            run_model_slots(model, args=args, writer=writer, product_passport=product_passport)
            for model in models
        )
    )


async def run_model_slots(
    model: str,
    *,
    args: argparse.Namespace,
    writer: ResultWriter,
    product_passport: dict[str, Any],
) -> None:
    tasks = []
    for run_number in range(1, args.runs + 1):
        if has_run(writer.payload, model, run_number):
            continue
        checkpoint_path = args.checkpoints / slugify(model) / f"run-{run_number}.sqlite"
        checkpoint_path.parent.mkdir(parents=True, exist_ok=True)
        tasks.append(
            run_one_model(
                model,
                [],
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
                pipeline="audience",
                audience_dataset=args.audience_data,
                audience_db=args.audience_db,
                product_passport=product_passport,
                semantic_index_dir=args.semantic_index,
                rate_limit_wait_budget_seconds=args.rate_limit_budget,
                run_timeout_seconds=args.run_timeout or None,
            )
        )
    if tasks:
        await asyncio.gather(*tasks)
    runs = completed_runs(writer.payload, model, args.runs)
    if len(runs) == args.runs and not has_evaluation(writer.payload, model):
        await append_stability_evaluation(model, runs, writer)


async def append_stability_evaluation(
    model: str, runs: list[dict[str, Any]], writer: ResultWriter
) -> None:
    evaluation = build_stability_evaluation(runs)
    await writer.append_evaluation(
        {
            "model": model,
            "evaluation_model": "deterministic-audience-stability",
            "run_numbers": [run["run_number"] for run in runs],
            "status": "completed",
            "evaluation": evaluation.model_dump(mode="json"),
        }
    )


def build_stability_evaluation(runs: list[dict[str, Any]]) -> StabilityEvaluation:
    summaries = [_run_summary(run) for run in runs]
    query_signatures = {json.dumps(item["query"], ensure_ascii=False, sort_keys=True) for item in summaries}
    result_signatures = {json.dumps(item["candidate_facts"], ensure_ascii=False, sort_keys=True) for item in summaries}
    same_query = len(query_signatures) == 1
    same_result = len(result_signatures) == 1
    all_have_segment = all(item["segment_count"] > 0 for item in summaries)
    score = 5.0 if same_query and same_result and all_have_segment else 3.0 if same_result else 2.0
    status = "complete" if same_result and all_have_segment else "partial"
    acceptance = "accepted" if score >= 4 else "rejected"
    comparison = [_comparison_line(item) for item in summaries]
    if not same_query:
        comparison.append("Query definitions or windows changed between runs.")
    if not same_result:
        comparison.append("Calculated counts, shares, or segment presence changed between runs.")
    if same_query and same_result and all_have_segment:
        summary = (
            "All three audience runs produced the same candidate definition, size, monthly profile, "
            "and aggregate characteristics."
        )
        limitations = [
            "This compares three independent model runs, not an alternative time period or resampled population.",
            "Stability does not establish product relevance, consent, or activation availability.",
        ]
    else:
        summary = "The audience output varied across the three runs; treat the segment as unstable."
        limitations = [
            "Run-to-run drift requires investigation before treating the audience as reproducible.",
            "This stability check does not establish product relevance, consent, or activation availability.",
        ]
    criterion = EvaluationCriterion(
        id="c9",
        name="Устойчивость сегмента",
        weight=5,
        score=score,
        status="assessed",
        weighted_score=score * 20,
        findings=[summary],
        recommendations=(
            ["Use the shared query and calculation as the reproducible audience definition."]
            if score >= 4
            else ["Inspect query, window, and feature drift before using the audience."]
        ),
    )
    return StabilityEvaluation(
        evaluation_status=status,
        mode="benchmark",
        overall_score=score * 20,
        acceptance=acceptance,
        score_basis="Deterministic comparison of query definitions, windows, counts, shares, and segment presence.",
        criterion=criterion,
        run_comparison=comparison,
        summary=summary,
        limitations=limitations,
    )


def _run_summary(run: dict[str, Any]) -> dict[str, Any]:
    result = run.get("result", {})
    calculation = result.get("calculation", {})
    query = calculation.get("query") or result.get("query") or {}
    audience = result.get("audience") or {}
    segment = (audience.get("segments") or [None])[0] or {}
    aggregate = calculation.get("aggregate_summary") or {}
    definition = segment.get("definition") or {}
    size = segment.get("size") or {}
    interpretation = segment.get("interpretation") or {}
    candidate_facts = {
        "query": {key: value for key, value in query.items() if key != "name"},
        "rules": definition.get("inclusion_rules", []),
        "size": {
            "count": calculation.get("count", size.get("count")),
            "base_count": calculation.get("base_count", size.get("base_count")),
            "share": calculation.get("share", size.get("share")),
        },
        "monthly_client_counts": [
            {
                "name": feature.get("name"),
                "counts": feature.get("monthly_client_counts", []),
            }
            for feature in calculation.get("feature_summaries", [])
        ],
        "aggregate": {
            "event_count": aggregate.get("event_count"),
            "average_events_per_client": aggregate.get("average_events_per_client"),
            "average_events_per_client_per_month": aggregate.get("average_events_per_client_per_month"),
            "monthly_event_counts": aggregate.get("monthly_event_counts", []),
            "value_distributions": aggregate.get("value_distributions", []),
        },
    }
    return {
        "run_number": run.get("run_number"),
        "query_name": query.get("name", "unknown"),
        "query": query,
        "window": f"{query.get('window_start', 'unknown')}..{query.get('window_end', 'unknown')}",
        "count": calculation.get("count"),
        "base_count": calculation.get("base_count"),
        "share": calculation.get("share"),
        "segment_count": len(audience.get("segments", [])),
        "candidate_facts": candidate_facts,
        "segment_name": segment.get("name", "unknown"),
        "characteristics": [item.get("text") for item in segment.get("characteristics", [])],
        "context": (interpretation.get("context") or {}).get("text"),
        "jtbd_status": (interpretation.get("jtbd") or {}).get("kind", "unknown"),
        "risks": segment.get("risks", []),
    }


def _comparison_line(item: dict[str, Any]) -> str:
    aggregate = item["candidate_facts"]["aggregate"]
    rules = "; ".join(item["candidate_facts"]["rules"]) or "none"
    monthly = ", ".join(
        f"{feature['name']}={[(entry['month'], entry['client_count']) for entry in feature['counts']]}"
        for feature in item["candidate_facts"]["monthly_client_counts"]
    )
    return (
        f"Run {item['run_number']}: candidate={item['segment_name']}; window={item['window']}; "
        f"size={item['count']}/{item['base_count']} ({item['share']}); segments={item['segment_count']}; "
        f"rules=[{rules}]; monthly_clients=[{monthly}]; "
        f"events={aggregate.get('event_count')}, avg_events_per_client={aggregate.get('average_events_per_client')}, "
        f"avg_events_per_client_per_month={aggregate.get('average_events_per_client_per_month')}; "
        f"context={item['context'] or 'unknown'}; jtbd_status={item['jtbd_status']}; "
        f"risks={item['risks'] or 'none'}."
    )


def load_payload(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {"results": [], "evaluations": []}
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise SystemExit(f"cannot read benchmark output {path}: {error}") from error
    if not isinstance(payload, dict):
        raise SystemExit(f"benchmark output must contain a JSON object: {path}")
    return payload


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
            and isinstance(item.get("result", {}).get("audience"), dict)
        ],
        key=lambda item: item["run_number"],
    )


def has_evaluation(payload: dict[str, Any], model: str) -> bool:
    return any(
        item.get("model") == model and item.get("status") == "completed"
        for item in payload.get("evaluations", [])
    )


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Run the audience pipeline three times and assess stability.")
    parser.add_argument("--models", nargs="*", default=[])
    parser.add_argument("--models-file", type=Path)
    parser.add_argument("--runs", type=int, default=3)
    parser.add_argument("--output", type=Path, default=Path("results/audience-benchmark.json"))
    parser.add_argument("--checkpoints", type=Path, default=Path("checkpoints/audience-benchmark"))
    parser.add_argument("--semantic-index", type=Path, default=Path("semantic-index"))
    parser.add_argument("--audience-db", type=Path, default=Path("audience.sqlite"))
    parser.add_argument(
        "--audience-data",
        type=Path,
        default=Path(__file__).resolve().parent / "События аудиторий (датасет).xlsx",
    )
    parser.add_argument("--product-result", type=Path, required=True)
    parser.add_argument("--product-model")
    parser.add_argument("--target-product", default="Tempo")
    parser.add_argument("--as-of-date")
    parser.add_argument("--api-key", default=os.environ.get("OPENROUTER_API_KEY"))
    parser.add_argument("--base-url", default=os.environ.get("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"))
    parser.add_argument("--timeout", type=float, default=120.0)
    parser.add_argument("--run-timeout", type=float, default=600.0)
    parser.add_argument("--rate-limit-budget", type=float, default=45.0)
    parser.add_argument("--step-retries", type=int, default=1)
    parser.add_argument("--app-url", default=os.environ.get("OPENROUTER_APP_URL"))
    parser.add_argument("--app-name", default="Tempo audience stability eval")
    parser.add_argument("--run-id", default=uuid.uuid4().hex[:12])
    return parser


if __name__ == "__main__":
    asyncio.run(run_benchmark(build_parser().parse_args()))
