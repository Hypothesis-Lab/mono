from __future__ import annotations

import asyncio
import json
import re
import sqlite3
import uuid
from dataclasses import asdict
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from langgraph.checkpoint.sqlite import SqliteSaver

from .client import ModelClient
from .models import Evaluation, Passport, Source, Validation
from .workflow import Workflow


class ResultWriter:
    def __init__(self, path: Path, metadata: dict[str, Any], payload: dict[str, Any] | None = None) -> None:
        self.path = path
        self.payload: dict[str, Any] = payload or {**metadata, "results": []}
        self.lock = asyncio.Lock()

    async def append(self, result: dict[str, Any]) -> None:
        async with self.lock:
            self.payload["results"].append(result)
            self._write()

    async def append_evaluation(self, evaluation: dict[str, Any]) -> None:
        async with self.lock:
            self.payload.setdefault("evaluations", []).append(evaluation)
            self._write()

    def _write(self) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        temporary = self.path.with_suffix(self.path.suffix + ".tmp")
        temporary.write_text(json.dumps(self.payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        temporary.replace(self.path)


async def run_models(
    models: list[str],
    sources: list[Source],
    *,
    target_product: str,
    as_of_date: str | None,
    api_key: str,
    base_url: str,
    checkpoint_dir: Path,
    output_path: Path,
    timeout_seconds: float,
    step_retries: int,
    app_url: str | None,
    app_name: str | None,
    run_id: str,
) -> None:
    writer = ResultWriter(
        output_path,
        {
            "run_id": run_id,
            "created_at": datetime.now(UTC).isoformat(),
            "target_product": target_product,
            "as_of_date": as_of_date,
            "source_names": [source.source for source in sources],
        },
    )
    checkpoint_dir.mkdir(parents=True, exist_ok=True)
    await asyncio.gather(
        *(
            run_one_model(
                model,
                sources,
                target_product=target_product,
                as_of_date=as_of_date,
                api_key=api_key,
                base_url=base_url,
                checkpoint_dir=checkpoint_dir,
                timeout_seconds=timeout_seconds,
                step_retries=step_retries,
                app_url=app_url,
                app_name=app_name,
                run_id=run_id,
                writer=writer,
            )
            for model in models
        )
    )


async def run_one_model(
    model: str,
    sources: list[Source],
    *,
    target_product: str,
    as_of_date: str | None,
    api_key: str,
    base_url: str,
    checkpoint_dir: Path,
    timeout_seconds: float,
    step_retries: int,
    app_url: str | None,
    app_name: str | None,
    run_id: str,
    writer: ResultWriter,
    run_number: int | None = None,
    checkpoint_path: Path | None = None,
    resume: bool = False,
    source_order: list[str] | None = None,
) -> None:
    started = datetime.now(UTC)
    client: ModelClient | None = None
    checkpoint_path = checkpoint_path or checkpoint_dir / f"{slugify(model)}.sqlite"
    record: dict[str, Any] = {
        "model": model,
        "checkpoint": str(checkpoint_path),
        "status": "failed",
        **({"run_number": run_number} if run_number is not None else {}),
        **({"source_order": source_order} if source_order is not None else {}),
    }
    try:
        client = ModelClient(
            model,
            api_key,
            base_url,
            timeout_seconds,
            app_url,
            app_name,
            correction_retries=step_retries,
        )
        with SqliteSaver.from_conn_string(str(checkpoint_path)) as checkpointer:
            checkpointer.setup()
            graph = Workflow(client, step_retries=step_retries).compile(checkpointer)
            thread_id = read_thread_id(checkpoint_path) if resume else None
            final_state = await asyncio.to_thread(
                graph.invoke,
                None
                if resume
                else {
                    "model": model,
                    "target_product": target_product,
                    "as_of_date": as_of_date,
                    "sources": sources,
                },
                {
                    "configurable": {
                        "thread_id": thread_id or f"{run_id}:{slugify(model)}",
                    }
                },
            )
        record.update(
            {
                "status": "completed",
                "result": {
                    "passport": Passport.model_validate(final_state["passport"]).model_dump(mode="json"),
                    "validation": Validation.model_validate(final_state["validation"]).model_dump(mode="json"),
                    "evaluation": Evaluation.model_validate(final_state["evaluation"]).model_dump(mode="json"),
                },
            }
        )
    except Exception as error:
        record["error"] = f"{type(error).__name__}: {error}"
    record["duration_seconds"] = (datetime.now(UTC) - started).total_seconds()
    calls = [asdict(call) for call in client.calls] if client else []
    record["calls"] = calls
    record["call_count"] = len(calls)
    record["input_tokens"] = sum(call["input_tokens"] or 0 for call in calls)
    record["output_tokens"] = sum(call["output_tokens"] or 0 for call in calls)
    await writer.append(record)


def load_sources(input_dir: Path) -> list[Source]:
    files = sorted(path for path in input_dir.glob("*.md") if path.name != "README.md")
    if not files:
        return [Source(source="no-source.md", content="")]
    return [Source(source=path.name, content=path.read_text(encoding="utf-8")) for path in files]


def read_thread_id(checkpoint_path: Path) -> str | None:
    connection = sqlite3.connect(f"file:{checkpoint_path.resolve()}?mode=ro", uri=True)
    try:
        row = connection.execute("SELECT thread_id FROM checkpoints ORDER BY rowid DESC LIMIT 1").fetchone()
    finally:
        connection.close()
    return row[0] if row else None


def load_models(values: list[str], model_file: Path | None) -> list[str]:
    models = list(values)
    if model_file:
        text = model_file.read_text(encoding="utf-8")
        if model_file.suffix == ".json":
            loaded = json.loads(text)
            models.extend(loaded if isinstance(loaded, list) else loaded["models"])
        else:
            models.extend(line.strip() for line in text.splitlines() if line.strip() and not line.startswith("#"))
    result: list[str] = []
    for model in models:
        for item in model.split(","):
            if item.strip() and item.strip() not in result:
                result.append(item.strip())
    if not result:
        raise ValueError("provide models with --models or --models-file")
    return result


def slugify(value: str) -> str:
    slug = re.sub(r"[^a-zA-Z0-9_.-]+", "-", value).strip("-")
    return slug[:100] or str(uuid.uuid4())
