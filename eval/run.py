from __future__ import annotations

import argparse
import asyncio
import os
import uuid
from pathlib import Path

from tempo_eval.runner import load_models, load_sources, run_models


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Run the Tempo product extraction workflow against OpenRouter models.")
    parser.add_argument(
        "--models", nargs="*", default=[], help="OpenRouter model IDs, for example google/gemini-2.5-flash"
    )
    parser.add_argument("--models-file", type=Path, help="Text file (one model per line) or JSON list/object")
    parser.add_argument("--input-dir", type=Path, default=Path(__file__).resolve().parents[1] / "n8n/product")
    parser.add_argument("--output", type=Path, default=Path("results.json"))
    parser.add_argument("--checkpoints", type=Path, default=Path("checkpoints"))
    parser.add_argument("--target-product", default="Tempo")
    parser.add_argument("--as-of-date")
    parser.add_argument("--api-key", default=os.environ.get("OPENROUTER_API_KEY"))
    parser.add_argument("--base-url", default=os.environ.get("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"))
    parser.add_argument("--timeout", type=float, default=15.0)
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
    configured_model = os.environ.get("OPENROUTER_MODEL")
    models = load_models(args.models or ([configured_model] if configured_model else []), args.models_file)
    sources = load_sources(args.input_dir)
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
        )
    )


if __name__ == "__main__":
    main()
