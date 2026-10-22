from __future__ import annotations

import argparse
import asyncio
import os
import uuid
from pathlib import Path

from tempo_eval.products import PRODUCT_SLUGS, product_name, product_source_dir
from tempo_eval.runner import (
    compact_product_passport,
    load_completed_pipeline_result,
    load_models,
    load_product_passport,
    load_sources,
    run_models,
)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Run a Tempo evaluation pipeline against OpenRouter models.")
    parser.add_argument("--pipeline", choices=("product", "audience", "hypothesis"), default="product")
    parser.add_argument("--product", choices=PRODUCT_SLUGS, default="cafe", help="Product source set to use")
    parser.add_argument(
        "--models", nargs="*", default=[], help="OpenRouter model IDs, for example google/gemini-2.5-flash"
    )
    parser.add_argument(
        "--semantic-index",
        type=Path,
        default=Path("semantic-index"),
        help="Local persistent vector index for safe audience feature cards",
    )
    parser.add_argument(
        "--audience-db",
        type=Path,
        default=Path("audience.sqlite"),
        help="Private SQLite database imported from the audience XLSX",
    )
    parser.add_argument("--models-file", type=Path, help="Text file (one model per line) or JSON list/object")
    parser.add_argument("--input-dir", type=Path, help="Override the selected product source directory")
    parser.add_argument("--output", type=Path)
    parser.add_argument("--checkpoints", type=Path)
    parser.add_argument(
        "--audience-data",
        type=Path,
        default=Path(__file__).resolve().parent / "События аудиторий (датасет).xlsx",
        help="Private XLSX event dataset for the audience pipeline; raw rows are never sent to the model",
    )
    parser.add_argument(
        "--product-result", type=Path, help="Required product pipeline JSON whose passport is handed to audience"
    )
    parser.add_argument(
        "--audience-result", type=Path, help="Required audience pipeline JSON for the hypothesis pipeline"
    )
    parser.add_argument("--product-model", help="Model to select from a product benchmark result")
    parser.add_argument("--audience-model", help="Model to select from an audience benchmark result")
    parser.add_argument("--hypothesis-count", type=int, default=5, help="Candidate hypotheses to generate")
    parser.add_argument("--validate-top", type=int, default=3, help="Accepted hypotheses required after validation")
    parser.add_argument("--target-product", help="Override the selected product name")
    parser.add_argument("--as-of-date")
    parser.add_argument("--api-key", default=os.environ.get("OPENROUTER_API_KEY"))
    parser.add_argument("--base-url", default=os.environ.get("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"))
    parser.add_argument("--timeout", type=float, default=120.0)
    parser.add_argument(
        "--run-timeout",
        type=float,
        default=600.0,
        help="Maximum wall-clock seconds per model run; zero disables the overall limit",
    )
    parser.add_argument("--rate-limit-budget", type=float, default=45.0, help="Maximum seconds to wait on 429 retries")
    parser.add_argument("--step-retries", type=int, default=3, help="Corrective retries after invalid step output")
    parser.add_argument("--app-url", default=os.environ.get("OPENROUTER_APP_URL"))
    parser.add_argument("--app-name", default="Tempo extraction eval")
    parser.add_argument("--run-id")
    return parser


def main() -> None:
    args = build_parser().parse_args()
    if not args.api_key:
        raise SystemExit("OPENROUTER_API_KEY or --api-key is required")
    if args.step_retries < 1:
        raise SystemExit("--step-retries must be at least 1")
    if args.rate_limit_budget < 0:
        raise SystemExit("--rate-limit-budget must be non-negative")
    if args.run_timeout < 0:
        raise SystemExit("--run-timeout must be non-negative")
    if args.pipeline in {"audience", "hypothesis"} and not args.product_result:
        raise SystemExit("--product-result is required for the audience and hypothesis pipelines")
    if args.pipeline == "hypothesis" and not args.audience_result:
        raise SystemExit("--audience-result is required for the hypothesis pipeline")
    if args.hypothesis_count < 1:
        raise SystemExit("--hypothesis-count must be at least 1")
    if args.validate_top < 1:
        raise SystemExit("--validate-top must be at least 1")
    args.input_dir = args.input_dir or product_source_dir(args.product)
    args.target_product = args.target_product or product_name(args.product)
    configured_model = os.environ.get("OPENROUTER_MODEL")
    models = load_models(args.models or ([configured_model] if configured_model else []), args.models_file)
    pipeline_name = args.pipeline
    args.output = args.output or Path(f"results/{args.product}-{pipeline_name}.json")
    args.checkpoints = args.checkpoints or Path(f"checkpoints/{args.product}-{pipeline_name}")
    sources = load_sources(args.input_dir) if args.pipeline == "product" else []
    product_passport = None
    audience_result = None
    if args.pipeline == "audience":
        product_passport = compact_product_passport(load_product_passport(args.product_result, args.product_model))
    elif args.pipeline == "hypothesis":
        product_passport = load_product_passport(args.product_result, args.product_model)
        audience_result = load_completed_pipeline_result(
            args.audience_result, "audience", args.audience_model or args.product_model
        )
    run_id = args.run_id or uuid.uuid4().hex[:12]
    asyncio.run(
        run_models(
            models,
            sources,
            target_product=args.target_product,
            as_of_date=args.as_of_date,
            api_key=args.api_key,
            base_url=args.base_url,
            checkpoint_dir=args.checkpoints,
            output_path=args.output,
            timeout_seconds=args.timeout,
            step_retries=args.step_retries,
            app_url=args.app_url,
            app_name=args.app_name,
            run_id=run_id,
            pipeline=args.pipeline,
            audience_dataset=args.audience_data if args.pipeline == "audience" else None,
            audience_db=args.audience_db if args.pipeline == "audience" else None,
            product_passport=product_passport,
            audience_result=audience_result,
            product_result_ref=str(args.product_result) if args.product_result else None,
            audience_result_ref=str(args.audience_result) if args.audience_result else None,
            hypothesis_candidate_count=args.hypothesis_count,
            hypothesis_validation_target=args.validate_top,
            semantic_index_dir=args.semantic_index if args.pipeline == "audience" else None,
            rate_limit_wait_budget_seconds=args.rate_limit_budget,
            run_timeout_seconds=args.run_timeout or None,
        )
    )


if __name__ == "__main__":
    main()
