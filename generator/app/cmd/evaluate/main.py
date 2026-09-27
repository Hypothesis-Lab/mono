from __future__ import annotations

import argparse
import json
import os
from collections.abc import Iterable
from contextlib import ExitStack
from dataclasses import asdict
from pathlib import Path
from time import perf_counter
from uuid import uuid4

from langchain_core.runnables import RunnableConfig
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.checkpoint.postgres import PostgresSaver
from langgraph.checkpoint.sqlite import SqliteSaver

from app.config import ModelConfig
from app.db import connect
from app.language_model_client.client import ModelClient
from app.observability import Measurement, RunCollector
from app.pipelines.audiences.graph import build_graph
from app.pipelines.audiences.models import InputBundle
from app.pipelines.audiences.stages import AudienceStages
from app.pipelines.audiences.stages.input import validate_input


def main() -> None:
    args = _parser().parse_args()
    bundle = InputBundle.model_validate_json(args.input.read_text(encoding="utf-8"))
    observer = RunCollector()
    model_config = ModelConfig.from_environment(
        base_url=args.base_url,
        api_key=args.api_key,
        model=args.model,
        reasoning_effort=args.reasoning_effort,
    )
    stages = AudienceStages(ModelClient(model_config, observer))
    validate_input(bundle)

    with ExitStack() as stack:
        checkpointer, thread_id = _checkpointer(args, stack)
        graph = build_graph(stages, checkpointer=checkpointer, observer=observer)
        config: RunnableConfig | None = {"configurable": {"thread_id": thread_id}} if thread_id else None
        if args.resume and (config is None or not graph.get_state(config).values):
            raise ValueError("--resume requires an existing checkpointed evaluation thread")

        started = perf_counter()
        outcome = "success"
        try:
            if args.resume:
                result = graph.invoke(None, config)
            else:
                result = graph.invoke({"input": bundle.model_dump(mode="json"), "outputs": {}}, config)
        except Exception:
            outcome = "error"
            raise
        finally:
            observer.observe(
                Measurement(
                    kind="run",
                    name="audiences",
                    model=None,
                    duration_ms=(perf_counter() - started) * 1000,
                    outcome=outcome,
                )
            )

    args.output.write_text(json.dumps(result["result"], ensure_ascii=False, indent=2), encoding="utf-8")
    measurements = observer.snapshot()
    calls = [item for item in measurements if item.kind == "model_call"]
    report = {
        "thread_id": thread_id,
        "model": model_config.model,
        "measurements": [asdict(item) for item in measurements],
        "totals": {
            "model_calls": len(calls),
            "input_tokens": _known_sum(item.input_tokens for item in calls),
            "output_tokens": _known_sum(item.output_tokens for item in calls),
            "missing_usage_calls": sum(item.input_tokens is None or item.output_tokens is None for item in calls),
        },
    }
    report_path = args.report or args.output.with_suffix(".metrics.json")
    report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")


def _checkpointer(args: argparse.Namespace, stack: ExitStack) -> tuple[object | None, str | None]:
    if args.checkpointer == "none":
        if args.resume:
            raise ValueError("--resume requires a persistent checkpointer")

        return None, None

    thread_id = args.thread_id or str(uuid4())
    if args.checkpointer == "memory":
        if args.resume:
            raise ValueError("in-memory checkpoints cannot be resumed by a new CLI invocation")

        return InMemorySaver(), thread_id

    if args.checkpointer == "sqlite":
        if args.sqlite_path is None:
            raise ValueError("--sqlite-path is required with --checkpointer sqlite")

        saver = stack.enter_context(SqliteSaver.from_conn_string(str(args.sqlite_path)))
        saver.setup()
        return saver, thread_id

    dsn = args.postgres_dsn or os.environ.get("EVALUATION_DATABASE_URL")
    if not dsn:
        raise ValueError("--postgres-dsn or EVALUATION_DATABASE_URL is required for PostgreSQL checkpoints")

    connection = stack.enter_context(connect(dsn, autocommit=True))
    saver = PostgresSaver(connection)
    saver.setup()
    return saver, thread_id


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Evaluate one audience-pipeline input")
    parser.add_argument("input", type=Path)
    parser.add_argument("--model")
    parser.add_argument("--base-url")
    parser.add_argument("--api-key")
    parser.add_argument("--reasoning-effort")
    parser.add_argument("--checkpointer", choices=("none", "memory", "sqlite", "postgres"), default="none")
    parser.add_argument("--sqlite-path", type=Path)
    parser.add_argument("--postgres-dsn")
    parser.add_argument("--thread-id")
    parser.add_argument("--resume", action="store_true")
    parser.add_argument("--output", type=Path, default=Path("audience-result.json"))
    parser.add_argument("--report", type=Path)
    return parser


def _known_sum(values: Iterable[int | None]) -> int | None:
    tokens = [value for value in values if value is not None]
    return sum(tokens) if tokens else None


if __name__ == "__main__":
    main()
