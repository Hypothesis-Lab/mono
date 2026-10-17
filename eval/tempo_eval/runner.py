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

from langgraph.checkpoint.sqlite.aio import AsyncSqliteSaver

from .audience_workflow import AudienceWorkflow
from .client import ModelClient
from .models import (
    AudiencePlan,
    AudienceProfile,
    AudienceResult,
    AudienceReview,
    Evaluation,
    Passport,
    Source,
    Validation,
)
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
    pipeline: str = "product",
    audience_context: dict[str, Any] | None = None,
    rate_limit_wait_budget_seconds: float = 45.0,
) -> None:
    writer = ResultWriter(
        output_path,
        {
            "run_id": run_id,
            "created_at": datetime.now(UTC).isoformat(),
            "target_product": target_product,
            "as_of_date": as_of_date,
            "source_names": [source.source for source in sources],
            "pipeline": pipeline,
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
                pipeline=pipeline,
                audience_context=audience_context,
                rate_limit_wait_budget_seconds=rate_limit_wait_budget_seconds,
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
    input_order: dict[str, list[str]] | None = None,
    pipeline: str = "product",
    audience_context: dict[str, Any] | None = None,
    rate_limit_wait_budget_seconds: float = 45.0,
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
        **({"input_order": input_order} if input_order is not None else {}),
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
            rate_limit_wait_budget_seconds=rate_limit_wait_budget_seconds,
        )
        async with AsyncSqliteSaver.from_conn_string(str(checkpoint_path)) as checkpointer:
            await checkpointer.setup()
            workflow = (
                AudienceWorkflow(client, step_retries=step_retries)
                if pipeline == "audience"
                else Workflow(client, step_retries=step_retries)
            )
            graph = workflow.compile(checkpointer)
            thread_id = read_thread_id(checkpoint_path) if resume else None
            final_state = await graph.ainvoke(
                None
                if resume
                else _initial_state(
                    pipeline,
                    model=model,
                    target_product=target_product,
                    as_of_date=as_of_date,
                    sources=sources,
                    audience_context=audience_context,
                ),
                {
                    "configurable": {
                        "thread_id": thread_id or f"{run_id}:{slugify(model)}",
                    }
                },
            )
        record.update({"status": "completed", "result": _result_from_state(final_state, pipeline)})
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


def load_audience_context(
    path: Path, product_result: Path | None = None, product_model: str | None = None
) -> dict[str, Any]:
    try:
        context = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise ValueError(f"cannot read audience context {path}: {error}") from error
    if not isinstance(context, dict):
        raise ValueError(f"audience context must contain a JSON object: {path}")
    if product_result:
        context["product_passport"] = compact_product_passport(load_product_passport(product_result, product_model))
    if not isinstance(context.get("product_passport"), dict):
        raise ValueError("audience context requires a product passport; pass --product-result")
    return context


def load_product_passport(path: Path, model: str | None = None) -> dict[str, Any]:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise ValueError(f"cannot read product result {path}: {error}") from error
    candidates = payload.get("results", []) if isinstance(payload, dict) else []
    if isinstance(payload, dict) and payload.get("result", {}).get("passport"):
        candidates = [payload]
    for item in candidates:
        if model and item.get("model") != model:
            continue
        if item.get("status") in (None, "completed") and item.get("result", {}).get("passport"):
            return item["result"]["passport"]
    raise ValueError(f"no completed product passport found in {path}")


def compact_product_passport(passport: dict[str, Any]) -> dict[str, Any]:
    categories = {
        "identity",
        "purpose",
        "audience",
        "features",
        "benefits",
        "pricing",
        "terms",
        "limitations",
        "offers_channels",
        "communication_rules",
    }
    return {
        "product_name": passport.get("product_name"),
        "extraction_status": passport.get("extraction_status"),
        "records": [record for record in passport.get("records", []) if record.get("category") in categories],
        "warnings": passport.get("warnings", []),
    }


def _initial_state(
    pipeline: str,
    *,
    model: str,
    target_product: str,
    as_of_date: str | None,
    sources: list[Source],
    audience_context: dict[str, Any] | None,
) -> dict[str, Any]:
    if pipeline == "audience":
        if audience_context is None:
            raise ValueError("audience context is required for the audience pipeline")
        return {"pipeline": pipeline, "model": model, "context": audience_context}
    return {
        "pipeline": pipeline,
        "model": model,
        "target_product": target_product,
        "as_of_date": as_of_date,
        "sources": sources,
    }


def _result_from_state(state: dict[str, Any], pipeline: str) -> dict[str, Any]:
    validation = Validation.model_validate(state["validation"]).model_dump(mode="json")
    if pipeline == "audience":
        return {
            "audience": AudienceResult.model_validate(state["audience"]).model_dump(mode="json"),
            "profile": AudienceProfile.model_validate(state["profile"]).model_dump(mode="json"),
            "plan": AudiencePlan.model_validate(state["plan"]).model_dump(mode="json"),
            "validation": validation,
            "review": AudienceReview.model_validate(state["review"]).model_dump(mode="json"),
        }
    return {
        "passport": Passport.model_validate(state["passport"]).model_dump(mode="json"),
        "validation": validation,
        "evaluation": Evaluation.model_validate(state["evaluation"]).model_dump(mode="json"),
    }


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
