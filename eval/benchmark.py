from __future__ import annotations

import argparse
import asyncio
import json
import os
import random
import shutil
from pathlib import Path
from typing import Any

from tempo_eval.client import ModelClient
from tempo_eval.models import Source, StabilityEvaluation
from tempo_eval.runner import (
    ResultWriter,
    load_audience_context,
    load_models,
    load_sources,
    run_one_model,
    slugify,
)


async def run_benchmark(args: argparse.Namespace) -> None:
    if not args.api_key:
        raise SystemExit("OPENROUTER_API_KEY or --api-key is required")
    payload = load_payload(args.output)
    if args.retry_failed and not args.models and not args.models_file:
        models = sorted({result.get("model") for result in payload.get("results", []) if result.get("model")})
        if not models:
            raise SystemExit("no models with existing benchmark results to retry")
    else:
        models = load_models(args.models, args.models_file)
    models = [model for model in models if model not in set(args.exclude_model)]
    if not models:
        raise SystemExit("no models remain after exclusions")
    sources = load_sources(args.input_dir) if args.pipeline == "product" else []
    audience_context = (
        load_audience_context(args.audience_context, args.product_result, args.product_model)
        if args.pipeline == "audience"
        else None
    )
    if args.force:
        reset_models(payload, models, args.checkpoints)
    elif args.retry_failed:
        reset_failed(payload, models, args.checkpoints)
    payload.setdefault("results", [])
    payload.setdefault("evaluations", [])
    payload.setdefault(
        "benchmark",
        {
            "max_runs": args.runs,
            "target_product": args.target_product,
            "as_of_date": args.as_of_date,
            "pipeline": args.pipeline,
        },
    )
    payload["pipeline"] = args.pipeline
    writer = ResultWriter(args.output, {}, payload)
    await asyncio.gather(
        *(run_model_slots(model, sources, audience_context, args=args, writer=writer) for model in models)
    )


async def run_model_slots(
    model: str,
    sources: list[Source],
    audience_context: dict[str, Any] | None,
    *,
    args: argparse.Namespace,
    writer: ResultWriter,
) -> None:
    run_tasks = []
    for run_number in range(1, args.runs + 1):
        if has_run(writer.payload, model, run_number):
            continue
        checkpoint_path = args.checkpoints / slugify(model) / f"run-{run_number}.sqlite"
        checkpoint_path.parent.mkdir(parents=True, exist_ok=True)
        ordered_sources = sources_for_run(sources, run_number)
        ordered_context = context_for_run(audience_context, run_number)
        input_order = (
            (
                [source.source for source in ordered_sources]
                if args.pipeline == "product"
                else [source.get("source_id", "unknown") for source in ordered_context.get("audience_sources", [])]
            )
            if ordered_context
            else [source.source for source in ordered_sources]
        )
        run_tasks.append(
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
                source_order=input_order,
                input_order=audience_input_order(ordered_context) if args.pipeline == "audience" else None,
                pipeline=args.pipeline,
                audience_context=ordered_context,
                rate_limit_wait_budget_seconds=args.rate_limit_budget,
            )
        )
    if run_tasks:
        await asyncio.gather(*run_tasks)
    completed = completed_runs(writer.payload, model, args.runs, args.pipeline)
    if len(completed) == args.runs and not has_evaluation(writer.payload, model, args.pipeline):
        await evaluate_stability(model, completed, sources, audience_context, args=args, writer=writer)


async def evaluate_stability(
    model: str,
    runs: list[dict[str, Any]],
    sources: list[Source],
    audience_context: dict[str, Any] | None,
    *,
    args: argparse.Namespace,
    writer: ResultWriter,
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
            "benchmark_evaluation",
            benchmark_prompt(model, runs, sources, audience_context, args.pipeline),
            StabilityEvaluation,
        )
        entry.update({"status": "completed", "evaluation": evaluation.model_dump(mode="json")})
    except Exception as error:
        entry["error"] = f"{type(error).__name__}: {error}"
    entry["calls"] = [call.__dict__ for call in client.calls]
    await writer.append_evaluation(entry)


def benchmark_prompt(
    model: str,
    runs: list[dict[str, Any]],
    sources: list[Source],
    audience_context: dict[str, Any] | None,
    pipeline: str,
) -> str:
    if pipeline == "audience":
        return audience_benchmark_prompt(model, runs, audience_context or {})
    passports = {
        f"run_{run['run_number']}": {
            "input_order": run.get("input_order", {"audience_sources": run.get("source_order", [])}),
            "passport": run["result"]["passport"],
            "validation": run["result"]["validation"],
        }
        for run in runs
    }
    return f"""You are an independent stability evaluator for product extraction.

This is an external benchmark over three completed runs of model `{model}`.
Evaluate only C9, `Устойчивость результата`. Do not score or reproduce the
general extraction criteria, mandatory checks, or test scenarios from the
single-run evaluation.

Compare the runs for stability of substance, conditions, numbers, evidence,
conflicts, gaps, and product boundaries. Source order was intentionally varied.
Differences in local IDs, wording, or record ordering are acceptable when
meaning is preserved. Identify any material drift and explain it in the run
comparison.

Return only one JSON object with exactly these fields:
evaluation_status, mode, overall_score, acceptance, score_basis, criterion,
run_comparison, summary, limitations.

Use `mode: "benchmark"`. The `criterion` must have id `c9`, name
`Устойчивость результата`, weight 5, a score from 0 to 5 or null, status
`assessed` or `not_tested`, weighted_score, findings, and recommendations.
Set acceptance to `not_tested` unless the evidence supports acceptance; do not
claim acceptance merely because the outputs look similar.

RUNS:
{json.dumps(passports, ensure_ascii=False, indent=2)}

VALIDATIONS:
{json.dumps({f"run_{run['run_number']}": run["result"]["validation"] for run in runs}, ensure_ascii=False, indent=2)}

SOURCE MATERIALS:
{json.dumps([source.model_dump(mode="json") for source in sources], ensure_ascii=False, indent=2)}
    """


def audience_benchmark_prompt(model: str, runs: list[dict[str, Any]], context: dict[str, Any]) -> str:
    results = {
        f"run_{run['run_number']}": {
            "input_order": run.get("input_order", {"audience_sources": run.get("source_order", [])}),
            "audience": run["result"]["audience"],
            "validation": run["result"]["validation"],
        }
        for run in runs
    }
    comparison = audience_persistence_comparison(runs)
    return f"""You are an independent stability evaluator for an audience segmentation pipeline.

Evaluate only C9-style prompt/output persistence across three completed runs of `{model}`.
The same product handoff, trusted configuration, and precomputed analytical
artifacts were supplied; only the order of equivalent input collections was
varied. Compare segment inventory, observed/proposed origin, inclusion and
exclusion rules, rule and membership artifacts, sizes and populations,
features, checks, source references, claim kinds, status/mode, and
interpretation boundaries.

The following must not drift merely because inputs are reordered: populations,
rules, counts, shares, periods, source references, check statuses, and segment
meaning. Local IDs, textual phrasing, and record ordering may differ only when
their meaning and measured values are preserved. Report material drift clearly.

The application computed the canonical material comparison below. Treat it as
authoritative for structural fields: do not claim structural drift absent from
it. Claim-boundary drift is material only when the canonical comparison reports
it. Apply this fixed C9 rubric: score 5 for no reported drift; score 3 for
claim-boundary-only drift; score 0 for any structural drift. Set overall_score
to score * 20 and weighted_score equal to overall_score.

This benchmark is not a temporal or sample stability calculation. Do not mark
the audience result's `stability` check as pass based only on three LLM runs;
that check still requires comparable periods/samples and a supplied analytical
result.

All three runs supplied here completed their contract validation, so C9 prompt
persistence is testable. You MUST return criterion.status="assessed", a numeric
criterion.score from 0 to 5, numeric criterion.weighted_score and
overall_score from 0 to 100. Use criterion.weight=100 and set weighted_score
equal to overall_score. Unknown activation, channel availability, relevance,
or empirical temporal/sample stability are limitations of the audience
candidate; they MUST NOT make this prompt-persistence score null or
not_tested. `acceptance` is not a deployment approval; if no system acceptance
threshold was supplied, leave it as `not_tested` while still scoring C9.

`run_comparison` MUST contain at least three concrete entries: Run 1 ↔ Run 2,
Run 1 ↔ Run 3, and Run 2 ↔ Run 3. Each entry must state whether material drift
was found in definitions, artifacts, numbers, check statuses, refs, or meaning.

Return one JSON object matching the StabilityEvaluation schema. Use mode=benchmark and evaluate
only the C9-like stability criterion with id c9 and name `Устойчивость результата`. Do not score
product extraction criteria. Do not claim a calculation, stability, or activation status that is
not present in the supplied results.

RUNS:
{json.dumps(results, ensure_ascii=False, indent=2)}

INPUT METADATA:
{json.dumps({key: context.get(key) for key in ("request_id", "product_id", "config")}, ensure_ascii=False, indent=2)}

CANONICAL MATERIAL COMPARISON:
{json.dumps(comparison, ensure_ascii=False, indent=2)}
"""


def audience_persistence_comparison(runs: list[dict[str, Any]]) -> dict[str, Any]:
    canonical = {run["run_number"]: _canonical_audience_result(run["result"]["audience"]) for run in runs}
    comparisons = []
    for left, right in ((1, 2), (1, 3), (2, 3)):
        differences = _difference_keys(canonical[left], canonical[right])
        structural = [key for key in differences if key != "claim_boundaries"]
        comparisons.append(
            {
                "runs": [left, right],
                "structural_drift": structural,
                "claim_boundary_drift": "claim_boundaries" in differences,
            }
        )
    has_structural_drift = any(item["structural_drift"] for item in comparisons)
    has_claim_drift = any(item["claim_boundary_drift"] for item in comparisons)
    return {
        "comparisons": comparisons,
        "classification": (
            "structural_drift"
            if has_structural_drift
            else "claim_boundary_only"
            if has_claim_drift
            else "no_material_drift"
        ),
    }


def _canonical_audience_result(audience: dict[str, Any]) -> dict[str, Any]:
    return {
        "status": audience.get("status"),
        "mode": audience.get("mode"),
        "data_profile": _canonical_data_profile(audience.get("data_profile", {})),
        "features": {
            "used": _sorted(_canonical_feature(feature) for feature in audience.get("features_used", [])),
            "excluded": sorted(feature.get("name") for feature in audience.get("features_excluded", [])),
        },
        "segments": _sorted(_canonical_segment(segment) for segment in audience.get("segments", [])),
        "overlaps": _canonical_overlaps(audience.get("overlaps", {})),
        "claim_boundaries": _sorted(
            [_canonical_claim(audience.get("data_profile", {}).get("coverage", {}))]
            + [
                _canonical_claims(segment.get(field, {}))
                for segment in audience.get("segments", [])
                for field in ("characteristics", "interpretation")
            ]
        ),
    }


def _canonical_data_profile(value: dict[str, Any]) -> dict[str, Any]:
    return {
        "unit": value.get("unit"),
        "cutoff_at": value.get("cutoff_at"),
        "observation_window": value.get("observation_window"),
        "snapshot_refs": sorted(value.get("snapshot_refs", [])),
    }


def _canonical_feature(value: dict[str, Any]) -> tuple[Any, ...]:
    window = value.get("window", {})
    return (
        value.get("name"),
        window.get("start"),
        window.get("end"),
        tuple(sorted(value.get("source_refs", []))),
    )


def _canonical_segment(value: dict[str, Any]) -> tuple[Any, ...]:
    definition = value.get("definition", {})
    size = value.get("size", {})
    checks = sorted(
        (
            check.get("name"),
            check.get("status"),
            tuple(sorted(check.get("source_refs", []))),
        )
        for check in value.get("checks", [])
    )
    return (
        value.get("origin"),
        tuple(sorted(definition.get("inclusion_rules", []))),
        tuple(sorted(definition.get("exclusion_rules", []))),
        definition.get("method"),
        definition.get("rule_or_model_ref"),
        definition.get("membership_ref"),
        size.get("count"),
        size.get("base_count"),
        size.get("share"),
        size.get("population"),
        tuple(sorted(size.get("source_refs", []))),
        tuple(checks),
    )


def _canonical_overlaps(value: dict[str, Any]) -> tuple[Any, ...]:
    return value.get("status"), tuple(sorted(value.get("source_refs", [])))


def _canonical_claims(value: object) -> tuple[Any, ...]:
    if isinstance(value, dict):
        if "kind" in value:
            return _canonical_claim(value)
        return tuple(_sorted((key, _canonical_claims(item)) for key, item in value.items()))
    if isinstance(value, list):
        return tuple(_sorted(_canonical_claims(item) for item in value))
    return ("value", value)


def _canonical_claim(value: object) -> tuple[Any, ...]:
    if not isinstance(value, dict):
        return ("invalid",)
    return value.get("kind"), tuple(sorted(value.get("source_refs", [])))


def _difference_keys(left: dict[str, Any], right: dict[str, Any]) -> list[str]:
    return [key for key in left if left[key] != right.get(key)]


def _sorted(values: Any) -> list[Any]:
    return sorted(values, key=repr)


def sources_for_run(sources: list[Source], run_number: int) -> list[Source]:
    if run_number == 1:
        return list(sources)
    if run_number == 2:
        return list(reversed(sources))
    shuffled = list(sources)
    random.Random(42).shuffle(shuffled)
    return shuffled


def context_for_run(context: dict[str, Any] | None, run_number: int) -> dict[str, Any] | None:
    if context is None:
        return None
    result = json.loads(json.dumps(context, ensure_ascii=False))
    _reorder_context_collection(result, "audience_sources", run_number)
    _reorder_context_mapping(result, "calculation_results", run_number)
    passport = result.get("product_passport")
    if isinstance(passport, dict):
        _reorder_context_collection(passport, "records", run_number)
    config = result.get("config")
    if isinstance(config, dict):
        _reorder_context_collection(config, "allowed_features", run_number)
        _reorder_context_collection(config, "prohibited_features", run_number)
    return result


def audience_input_order(context: dict[str, Any] | None) -> dict[str, list[str]]:
    if context is None:
        return {}
    passport = context.get("product_passport", {})
    return {
        "audience_sources": [source.get("source_id", "unknown") for source in context.get("audience_sources", [])],
        "calculation_results": list(context.get("calculation_results", {})),
        "product_records": [record.get("id", "unknown") for record in passport.get("records", [])],
        "allowed_features": list(context.get("config", {}).get("allowed_features", [])),
    }


def _reorder_context_collection(context: dict[str, Any], key: str, run_number: int) -> None:
    value = context.get(key)
    if not isinstance(value, list) or run_number == 1:
        return
    reordered = list(value)
    if run_number == 2:
        reordered.reverse()
    else:
        random.Random(42).shuffle(reordered)
    context[key] = reordered


def _reorder_context_mapping(context: dict[str, Any], key: str, run_number: int) -> None:
    value = context.get(key)
    if not isinstance(value, dict) or run_number == 1:
        return
    items = list(value.items())
    if run_number == 2:
        items.reverse()
    else:
        random.Random(42).shuffle(items)
    context[key] = dict(items)


def load_payload(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise SystemExit(f"cannot read benchmark output {path}: {error}") from error
    if not isinstance(payload, dict):
        raise SystemExit(f"benchmark output must contain a JSON object: {path}")
    for result in payload.get("results", []):
        if "run_number" not in result:
            result["run_number"] = 1
    return payload


def reset_models(payload: dict[str, Any], models: list[str], checkpoints: Path) -> None:
    selected = set(models)
    payload["results"] = [result for result in payload.get("results", []) if result.get("model") not in selected]
    payload["evaluations"] = [
        evaluation for evaluation in payload.get("evaluations", []) if evaluation.get("model") not in selected
    ]
    for model in models:
        shutil.rmtree(checkpoints / slugify(model), ignore_errors=True)


def reset_failed(payload: dict[str, Any], models: list[str], checkpoints: Path) -> None:
    selected = set(models)
    payload["results"] = [
        result
        for result in payload.get("results", [])
        if not (result.get("model") in selected and result.get("status") == "failed")
    ]
    payload["evaluations"] = [
        evaluation
        for evaluation in payload.get("evaluations", [])
        if not (evaluation.get("model") in selected and evaluation.get("status") == "failed")
    ]


def has_run(payload: dict[str, Any], model: str, run_number: int) -> bool:
    return any(
        result.get("model") == model and result.get("run_number") == run_number for result in payload.get("results", [])
    )


def completed_runs(payload: dict[str, Any], model: str, max_runs: int, pipeline: str) -> list[dict[str, Any]]:
    return sorted(
        [
            result
            for result in payload.get("results", [])
            if result.get("model") == model
            and result.get("run_number", 1) <= max_runs
            and result.get("status") == "completed"
            and isinstance(result.get("result", {}).get("audience" if pipeline == "audience" else "passport"), dict)
        ],
        key=lambda result: result["run_number"],
    )


def has_evaluation(payload: dict[str, Any], model: str, pipeline: str) -> bool:
    for evaluation in payload.get("evaluations", []):
        if evaluation.get("model") != model or evaluation.get("status") != "completed":
            continue
        if pipeline != "audience":
            return True
        value = evaluation.get("evaluation", {})
        criterion = value.get("criterion", {})
        if (
            value.get("overall_score") is not None
            and criterion.get("score") is not None
            and len(value.get("run_comparison", [])) >= 3
        ):
            return True
    return False


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Run a selected pipeline up to three times and benchmark stability.")
    parser.add_argument("--pipeline", choices=("product", "audience"), default="product")
    parser.add_argument("--models", nargs="*", default=[])
    parser.add_argument("--models-file", type=Path)
    parser.add_argument("--exclude-model", action="append", default=[], help="Model ID to skip; may be repeated")
    parser.add_argument("--input-dir", type=Path, default=Path(__file__).resolve().parent.parent / "n8n/product")
    parser.add_argument("--output", type=Path)
    parser.add_argument("--checkpoints", type=Path)
    parser.add_argument(
        "--audience-context",
        type=Path,
        default=Path(__file__).resolve().parent.parent / "n8n/audience/context.json",
        help="Pre-generated JSON context for the audience pipeline",
    )
    parser.add_argument(
        "--product-result", type=Path, help="Required product pipeline JSON whose passport is handed to audience"
    )
    parser.add_argument("--product-model", help="Model to select from a product benchmark result")
    parser.add_argument("--runs", type=int, default=3)
    parser.add_argument("--force", action="store_true", help="Reset selected model slots and checkpoints")
    parser.add_argument(
        "--retry-failed",
        action="store_true",
        help="Remove failed slots and retry them while preserving completed runs",
    )
    parser.add_argument("--target-product", default="Tempo")
    parser.add_argument("--as-of-date")
    parser.add_argument("--api-key", default=os.environ.get("OPENROUTER_API_KEY"))
    parser.add_argument("--base-url", default=os.environ.get("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"))
    parser.add_argument(
        "--evaluation-model", default=os.environ.get("OPENROUTER_EVALUATION_MODEL", os.environ.get("OPENROUTER_MODEL"))
    )
    parser.add_argument("--timeout", type=float, default=15.0)
    parser.add_argument("--rate-limit-budget", type=float, default=45.0, help="Maximum seconds to wait on 429 retries")
    parser.add_argument("--step-retries", type=int, default=3)
    parser.add_argument("--app-url", default=os.environ.get("OPENROUTER_APP_URL"))
    parser.add_argument("--app-name", default="Tempo extraction benchmark")
    parser.add_argument("--run-id", default="benchmark")
    return parser


def main() -> None:
    args = build_parser().parse_args()
    if not args.models and not args.models_file and Path("models.txt").exists():
        args.models_file = Path("models.txt")
    args.output = args.output or (
        Path("results/audience-benchmark.json") if args.pipeline == "audience" else Path("results/benchmark.json")
    )
    args.checkpoints = args.checkpoints or (
        Path("checkpoints/audience-benchmark") if args.pipeline == "audience" else Path("checkpoints/benchmark")
    )
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
