from __future__ import annotations

import argparse
import json
import os
import sqlite3
from concurrent.futures import ThreadPoolExecutor, as_completed
from contextlib import ExitStack
from datetime import UTC, datetime
from html import escape
from pathlib import Path
from time import perf_counter
from typing import Any, cast
from uuid import uuid4

from langchain_core.runnables import RunnableConfig
from langgraph.checkpoint.sqlite import SqliteSaver

from app.config import ModelConfig
from app.language_model_client.client import CodexClient
from app.observability import Measurement, RunCollector
from app.pipelines.audience_questions.graph import build_graph
from app.pipelines.audiences.models import Product
from app.pipelines.audiences.stages import AudienceStages

MODELS = ("gpt-6-luna", "gpt-5.6-luna")
PIPELINE = "audience_questions"


class EvaluationStore:
    def __init__(self, path: Path) -> None:
        self.path = path

    def initialize(self) -> None:
        with sqlite3.connect(self.path) as connection:
            connection.execute("PRAGMA journal_mode=WAL")
            self._configure(connection)
            self._setup(connection)

    def save(
        self,
        run_id: str,
        model: str,
        thread_id: str,
        product: Product,
        result: dict[str, object] | None,
        outcome: str,
        measurements: list[Measurement],
    ) -> None:
        timestamp = datetime.now(UTC).isoformat()
        run_measurement = next(item for item in measurements if item.kind == "run")
        with sqlite3.connect(self.path) as connection:
            self._configure(connection)
            self._setup(connection)
            connection.execute(
                """
                INSERT INTO evaluation_results
                    (run_id, pipeline, model, thread_id, input_json, result_json, outcome, duration_ms, created_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    run_id,
                    PIPELINE,
                    model,
                    thread_id,
                    json.dumps(product.model_dump(mode="json"), ensure_ascii=False),
                    json.dumps(result, ensure_ascii=False) if result is not None else None,
                    outcome,
                    run_measurement.duration_ms,
                    timestamp,
                ),
            )
            connection.executemany(
                """
                INSERT INTO evaluation_measurements
                    (run_id, kind, name, model, duration_ms, operation_id,
                     input_tokens, output_tokens, outcome, created_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                [
                    (
                        run_id,
                        item.kind,
                        item.name,
                        item.model,
                        item.duration_ms,
                        item.operation_id,
                        item.input_tokens,
                        item.output_tokens,
                        item.outcome,
                        timestamp,
                    )
                    for item in measurements
                ],
            )

    def render_html(self, path: Path, run_ids: list[str]) -> None:
        with sqlite3.connect(self.path) as connection:
            connection.row_factory = sqlite3.Row
            results = []
            measurements = []
            for run_id in run_ids:
                results.extend(
                    dict(row)
                    for row in connection.execute("SELECT * FROM evaluation_results WHERE run_id = ?", (run_id,))
                )
                measurements.extend(
                    dict(row)
                    for row in connection.execute("SELECT * FROM evaluation_measurements WHERE run_id = ?", (run_id,))
                )

        path.write_text(_render_html(results, measurements), encoding="utf-8")

    @staticmethod
    def _configure(connection: sqlite3.Connection) -> None:
        connection.execute("PRAGMA busy_timeout=30000")

    @staticmethod
    def _setup(connection: sqlite3.Connection) -> None:
        connection.executescript(
            """
            CREATE TABLE IF NOT EXISTS evaluation_results (
                run_id TEXT PRIMARY KEY,
                pipeline TEXT NOT NULL,
                model TEXT NOT NULL,
                thread_id TEXT NOT NULL,
                input_json TEXT NOT NULL,
                result_json TEXT,
                outcome TEXT NOT NULL,
                duration_ms REAL NOT NULL,
                created_at TEXT NOT NULL
            );
            CREATE TABLE IF NOT EXISTS evaluation_measurements (
                measurement_id INTEGER PRIMARY KEY AUTOINCREMENT,
                run_id TEXT NOT NULL REFERENCES evaluation_results(run_id),
                kind TEXT NOT NULL,
                name TEXT NOT NULL,
                model TEXT,
                duration_ms REAL NOT NULL,
                operation_id TEXT,
                input_tokens INTEGER,
                output_tokens INTEGER,
                outcome TEXT NOT NULL,
                created_at TEXT NOT NULL
            );
            """
        )


def main() -> None:
    args = _parser().parse_args()
    product = Product.model_validate_json(args.input.read_text(encoding="utf-8"))
    store = EvaluationStore(args.sqlite_path)
    store.initialize()
    _initialize_checkpointer(args.sqlite_path)
    run_ids = {model: str(uuid4()) for model in MODELS}
    errors: list[tuple[str, Exception]] = []
    with ThreadPoolExecutor(max_workers=len(MODELS), thread_name_prefix="evaluation") as executor:
        futures = {
            executor.submit(_run_model, product, model, args.sqlite_path, store, run_ids[model]): model
            for model in MODELS
        }
        for future in as_completed(futures):
            model = futures[future]
            try:
                future.result()
            except Exception as error:
                errors.append((model, error))

    if args.html:
        store.render_html(args.html_path, [run_ids[model] for model in MODELS])

    if errors:
        models = ", ".join(model for model, _ in errors)
        raise RuntimeError(f"{len(errors)} evaluation run(s) failed: {models}") from errors[0][1]


def _run_model(product: Product, model: str, sqlite_path: Path, store: EvaluationStore, run_id: str) -> None:
    thread_id = str(uuid4())
    observer = RunCollector()
    started = perf_counter()
    outcome = "success"
    result: dict[str, object] | None = None
    try:
        model_config = _model_config(model)
        stages = AudienceStages(CodexClient(model_config, observer))
        with ExitStack() as stack:
            checkpointer = stack.enter_context(SqliteSaver.from_conn_string(str(sqlite_path)))
            checkpointer.conn.execute("PRAGMA busy_timeout=30000")
            graph = build_graph(stages, checkpointer=checkpointer, observer=observer)
            config: RunnableConfig = {"configurable": {"thread_id": thread_id}}
            state = graph.invoke({"input": product.model_dump(mode="json"), "outputs": {}}, config)
            result = state["result"]
    except Exception:
        outcome = "error"
        raise
    finally:
        observer.observe(
            Measurement(
                kind="run",
                name=PIPELINE,
                model=model,
                duration_ms=(perf_counter() - started) * 1000,
                operation_id=run_id,
                outcome=outcome,
            )
        )
        store.save(run_id, model, thread_id, product, result, outcome, observer.snapshot())


def _initialize_checkpointer(sqlite_path: Path) -> None:
    with SqliteSaver.from_conn_string(str(sqlite_path)) as checkpointer:
        checkpointer.conn.execute("PRAGMA busy_timeout=30000")
        checkpointer.setup()


def _render_html(results: list[dict[str, Any]], measurements: list[dict[str, Any]]) -> str:
    results_by_model = {row["model"]: row for row in results}
    measurements_by_run: dict[str, list[dict[str, Any]]] = {}
    for measurement in measurements:
        measurements_by_run.setdefault(measurement["run_id"], []).append(measurement)

    summaries = {model: _summary(results_by_model.get(model), measurements_by_run) for model in MODELS}
    questions, audiences = _result_dimensions(results_by_model)
    metric_rows = "".join(
        f"<tr><th>{escape(label)}</th>"
        f"{''.join(f'<td>{escape(_display(summaries[model][key]))}</td>' for model in MODELS)}</tr>"
        for key, label in (
            ("outcome", "Outcome"),
            ("duration_ms", "Duration (ms)"),
            ("model_calls", "Model calls"),
            ("input_tokens", "Input tokens"),
            ("output_tokens", "Output tokens"),
        )
    )
    answer_rows = "".join(
        "<tr>"
        f"<th><strong>{escape(audience['label'])}</strong><br>{escape(question['text'])}</th>"
        + "".join(
            f"<td>{escape(_answer(results_by_model.get(model), audience['id'], question['id']))}</td>"
            for model in MODELS
        )
        + "</tr>"
        for audience in audiences
        for question in questions
    )

    headers = "".join(f"<th>{escape(model)}</th>" for model in MODELS)
    return f"""<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Audience questions evaluation</title>
  <style>
    :root {{ color-scheme: light; font-family: Inter, ui-sans-serif, system-ui, sans-serif; }}
    body {{ background: #f4f5f7; color: #20242a; margin: 0; padding: 32px; }}
    main {{ margin: 0 auto; max-width: 1440px; }}
    h1 {{ margin: 0 0 8px; font-size: 28px; }}
    h2 {{ margin: 32px 0 12px; font-size: 20px; }}
    p {{ color: #626b76; margin: 0 0 24px; }}
    table {{ background: white; border-collapse: collapse; box-shadow: 0 1px 3px #17203314; width: 100%; }}
    th, td {{ border: 1px solid #e1e5ea; padding: 12px 14px; text-align: left; vertical-align: top; }}
    thead th {{ background: #20242a; color: white; }}
    tbody th {{ background: #f8f9fa; min-width: 220px; }}
    .answers td {{ line-height: 1.5; white-space: pre-wrap; width: 35%; }}
    .answers tbody th {{ width: 30%; }}
    .missing {{ color: #9b3b32; font-style: italic; }}
    @media (max-width: 800px) {{ body {{ padding: 16px; }} table {{ font-size: 14px; }} th, td {{ padding: 9px; }} }}
  </style>
</head>
<body>
  <main>
    <h1>Audience questions evaluation</h1>
    <p>Pipeline: {escape(PIPELINE)}. Results and measurements are stored in SQLite.</p>
    <h2>Metrics comparison</h2>
    <table>
      <thead><tr><th>Metric</th>{headers}</tr></thead>
      <tbody>{metric_rows}</tbody>
    </table>
    <h2>Answer comparison</h2>
    <table class="answers">
      <thead><tr><th>Audience / question</th>{headers}</tr></thead>
      <tbody>{answer_rows or '<tr><td colspan="3" class="missing">No result rows found.</td></tr>'}</tbody>
    </table>
  </main>
</body>
</html>
"""


def _summary(result: dict[str, Any] | None, measurements_by_run: dict[str, list[dict[str, Any]]]) -> dict[str, Any]:
    if result is None:
        return {
            "outcome": "missing",
            "duration_ms": None,
            "model_calls": None,
            "input_tokens": None,
            "output_tokens": None,
        }

    calls = [item for item in measurements_by_run.get(result["run_id"], []) if item["kind"] == "model_call"]
    return {
        "outcome": result["outcome"],
        "duration_ms": result["duration_ms"],
        "model_calls": len(calls),
        "input_tokens": _token_sum(calls, "input_tokens"),
        "output_tokens": _token_sum(calls, "output_tokens"),
    }


def _result_dimensions(
    results_by_model: dict[str, dict[str, Any]],
) -> tuple[list[dict[str, str]], list[dict[str, str]]]:
    for model in MODELS:
        result = results_by_model.get(model)
        if result is None or result["result_json"] is None:
            continue

        payload = json.loads(result["result_json"])
        questions = [{"id": item["question_id"], "text": item["text"]} for item in payload["questions"]]
        seen: set[str] = set()
        audiences = []
        for answer in payload["answers"]:
            if answer["audience_id"] not in seen:
                audiences.append({"id": answer["audience_id"], "label": answer["audience_label"]})
                seen.add(answer["audience_id"])

        return questions, audiences

    return [], []


def _answer(result: dict[str, Any] | None, audience_id: str, question_id: str) -> str:
    if result is None or result["result_json"] is None:
        return "No result"

    payload = json.loads(result["result_json"])
    for answer in payload["answers"]:
        if answer["audience_id"] == audience_id and answer["question_id"] == question_id:
            return answer["answer"]

    return "No answer"


def _token_sum(measurements: list[dict[str, Any]], field: str) -> int | None:
    values = [item[field] for item in measurements if item[field] is not None]
    return sum(values) if values else None


def _display(value: object) -> str:
    return "unknown" if value is None else str(value)


def _model_config(model: str) -> ModelConfig:
    return ModelConfig(
        base_url=os.environ["OPENAI_BASE_URL"],
        api_key=os.environ["OPENAI_API_KEY"],
        model=model,
        # None omits reasoning_effort from the request and uses the provider default.
        reasoning_effort=cast(str, None),
        timeout_seconds=float(os.environ["OPENAI_TIMEOUT_SECONDS"]),
        max_retries=int(os.environ["OPENAI_MAX_RETRIES"]),
    )


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Evaluate audience questions with both Luna models")
    parser.add_argument("input", type=Path)
    parser.add_argument("--sqlite-path", type=Path, default=Path("evaluation.sqlite3"))
    parser.add_argument("--html", action="store_true", help="save a side-by-side HTML comparison")
    parser.add_argument("--html-path", type=Path, default=Path("evaluation.html"))
    return parser


if __name__ == "__main__":
    main()
