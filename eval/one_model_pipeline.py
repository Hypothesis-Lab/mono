"""Run the product pipeline and the dependent audience workflow for one model."""

from __future__ import annotations

import argparse
import asyncio
import logging
import os
from pathlib import Path
from types import SimpleNamespace

from audience_benchmark import run_benchmark
from report import ReportBuilder
from tempo_eval.products import PRODUCT_SLUGS, product_name, product_source_dir
from tempo_eval.runner import compact_product_passport, load_product_passport, load_sources, run_models


def build_parser() -> argparse.ArgumentParser:
    root = Path(__file__).resolve().parent
    parser = argparse.ArgumentParser(
        description=(
            "Run one product pass, one dependent audience pass, then a three-run "
            "audience consistency benchmark for one model."
        )
    )
    parser.add_argument(
        "--model",
        default=os.environ.get("OPENROUTER_MODEL"),
        required=not os.environ.get("OPENROUTER_MODEL"),
    )
    parser.add_argument("--product", choices=PRODUCT_SLUGS, default="cafe", help="Product source set to run")
    parser.add_argument("--input-dir", type=Path, help="Override the selected product source directory")
    parser.add_argument("--audience-data", type=Path, default=root / "События аудиторий (датасет).xlsx")
    parser.add_argument("--audience-db", type=Path, default=root / "audience.sqlite")
    parser.add_argument("--semantic-index", type=Path, default=root / "semantic-index")
    parser.add_argument("--output-dir", type=Path)
    parser.add_argument("--checkpoints-dir", type=Path)
    parser.add_argument("--reports-dir", type=Path)
    parser.add_argument("--target-product", help="Override the selected product name")
    parser.add_argument("--as-of-date")
    parser.add_argument("--api-key", default=os.environ.get("OPENROUTER_API_KEY"))
    parser.add_argument("--base-url", default=os.environ.get("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"))
    parser.add_argument("--timeout", type=float, default=120.0)
    parser.add_argument("--run-timeout", type=float, default=600.0)
    parser.add_argument("--rate-limit-budget", type=float, default=45.0)
    parser.add_argument("--step-retries", type=int, default=1)
    parser.add_argument("--app-url", default=os.environ.get("OPENROUTER_APP_URL"))
    parser.add_argument("--app-name", default="Tempo one-model dependent pipeline")
    return parser


def _benchmark_args(
    args: argparse.Namespace, product_result: Path, output: Path, checkpoints: Path
) -> argparse.Namespace:
    """Adapt the shared audience benchmark runner to the one-model orchestration."""

    return SimpleNamespace(
        api_key=args.api_key,
        runs=3,
        models=[args.model],
        models_file=None,
        product_result=product_result,
        product_model=args.model,
        output=output,
        checkpoints=checkpoints,
        semantic_index=args.semantic_index,
        audience_db=args.audience_db,
        audience_data=args.audience_data,
        target_product=args.target_product,
        as_of_date=args.as_of_date,
        base_url=args.base_url,
        timeout=args.timeout,
        run_timeout=args.run_timeout,
        rate_limit_budget=args.rate_limit_budget,
        step_retries=args.step_retries,
        app_url=args.app_url,
        app_name=args.app_name,
        run_id="audience-three-run",
    )


async def run_pipeline(args: argparse.Namespace) -> None:
    if not args.api_key:
        raise SystemExit("OPENROUTER_API_KEY or --api-key is required")
    if args.step_retries < 1:
        raise SystemExit("--step-retries must be at least 1")
    if args.run_timeout < 0:
        raise SystemExit("--run-timeout must be non-negative")
    if args.rate_limit_budget < 0:
        raise SystemExit("--rate-limit-budget must be non-negative")

    output_dir = args.output_dir
    checkpoints_dir = args.checkpoints_dir
    reports_dir = args.reports_dir
    output_dir.mkdir(parents=True, exist_ok=True)
    checkpoints_dir.mkdir(parents=True, exist_ok=True)
    reports_dir.mkdir(parents=True, exist_ok=True)

    product_result = output_dir / "product-one.json"
    product_checkpoints = checkpoints_dir / "product"
    audience_single_result = output_dir / "audience-one.json"
    audience_single_checkpoints = checkpoints_dir / "audience-one"
    audience_three_result = output_dir / "audience-three.json"
    audience_three_checkpoints = checkpoints_dir / "audience-three"

    # Phase 1: the product passport is the only input passed to the audience graph.
    await run_models(
        [args.model],
        load_sources(args.input_dir),
        target_product=args.target_product,
        as_of_date=args.as_of_date,
        api_key=args.api_key,
        base_url=args.base_url,
        checkpoint_dir=product_checkpoints,
        output_path=product_result,
        timeout_seconds=args.timeout,
        step_retries=args.step_retries,
        app_url=args.app_url,
        app_name=args.app_name,
        run_id="product-one",
        pipeline="product",
        rate_limit_wait_budget_seconds=args.rate_limit_budget,
        run_timeout_seconds=args.run_timeout or None,
    )
    product_passport = compact_product_passport(load_product_passport(product_result, args.model))

    # Phase 2: one dependent audience run and its immediate report.
    await run_models(
        [args.model],
        [],
        target_product=args.target_product,
        as_of_date=args.as_of_date,
        api_key=args.api_key,
        base_url=args.base_url,
        checkpoint_dir=audience_single_checkpoints,
        output_path=audience_single_result,
        timeout_seconds=args.timeout,
        step_retries=args.step_retries,
        app_url=args.app_url,
        app_name=args.app_name,
        run_id="audience-one",
        pipeline="audience",
        audience_dataset=args.audience_data,
        audience_db=args.audience_db,
        product_passport=product_passport,
        semantic_index_dir=args.semantic_index,
        rate_limit_wait_budget_seconds=args.rate_limit_budget,
        run_timeout_seconds=args.run_timeout or None,
    )
    single_report = ReportBuilder(output_dir, checkpoints_dir, pipeline="auto")
    single_report.max_runs = 1
    single_report.write(reports_dir / "single-run.html")

    # Phase 3: three fresh audience runs plus the existing deterministic stability check.
    await run_benchmark(_benchmark_args(args, product_result, audience_three_result, audience_three_checkpoints))
    ReportBuilder(output_dir, checkpoints_dir, pipeline="auto").write(reports_dir / "three-run-consistency.html")

    print(f"Product result: {product_result}")
    print(f"Single audience report: {reports_dir / 'single-run.html'}")
    print(f"Three-run audience result: {audience_three_result}")
    print(f"Three-run consistency report: {reports_dir / 'three-run-consistency.html'}")


def main() -> None:
    logging.basicConfig(
        level=os.environ.get("TEMPO_LOG_LEVEL", "INFO").upper(),
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
        force=True,
    )
    args = build_parser().parse_args()
    root = Path(__file__).resolve().parent
    args.input_dir = args.input_dir or product_source_dir(args.product)
    args.target_product = args.target_product or product_name(args.product)
    args.output_dir = args.output_dir or root / "results" / "one-model-pipeline" / args.product
    args.checkpoints_dir = args.checkpoints_dir or root / "checkpoints" / "one-model-pipeline" / args.product
    args.reports_dir = args.reports_dir or root / "reports" / "one-model-pipeline" / args.product
    asyncio.run(run_pipeline(args))


if __name__ == "__main__":
    main()
