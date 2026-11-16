from __future__ import annotations

import argparse
import hashlib
import json
import os
import sqlite3
import uuid
from dataclasses import asdict
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from dotenv import load_dotenv

from app.config import CodexConfig
from app.language_model_client.client import CodexClient
from app.observability import Measurement, RunCollector
from app.pipelines.product.models import Passport, ProductInput
from app.pipelines.registry import get_pipeline
from app.pipelines.stages import PipelineStages


class PipelineRunner:
    def __init__(self, arguments: argparse.Namespace) -> None:
        self.arguments = arguments

    def run(self) -> Path:
        input_data, product_name, input_artifacts = self._load_input()
        pipeline = get_pipeline(self.arguments.pipeline)
        validated_input = pipeline.validate_input(input_data)
        run_id = str(uuid.uuid4())
        started_at = datetime.now(UTC).isoformat()
        config = CodexConfig(
            proxy_base_url=self.arguments.base_url,
            proxy_key=self.arguments.api_key,
            model=self.arguments.model,
            reasoning_effort=self.arguments.reasoning_effort or "",
            timeout_seconds=self.arguments.timeout,
            max_retries=self.arguments.max_retries,
        )
        collector = RunCollector()
        stages = PipelineStages(CodexClient(config, collector))
        graph = pipeline.graph_builder(stages)
        output_path = self._output_path(run_id, product_name)
        try:
            state = graph.invoke({"input": validated_input, "outputs": {}})
        except Exception:
            report = self._report(
                None,
                collector.snapshot(),
                run_id,
                started_at,
                product_name,
                "failed",
                input_artifacts,
            )
            self._save_run(report, output_path)
            self._print_summary(output_path, report)
            raise

        report = self._report(
            state["result"],
            collector.snapshot(),
            run_id,
            started_at,
            product_name,
            "completed",
            input_artifacts,
        )
        self._save_run(report, output_path)
        self._print_summary(output_path, report)
        return output_path

    def _load_input(self) -> tuple[dict[str, Any], str, dict[str, Any]]:
        if self.arguments.pipeline == "product_analysis":
            product_input, source_files = self._load_product_input()
            return (
                product_input.model_dump(mode="json"),
                product_input.target_product,
                {"source_files": source_files},
            )

        passport, product_name, passport_artifact = self._load_passport(self.arguments.passport)
        if self.arguments.pipeline == "audience_selection":
            segments, segment_set_id, limitations, segment_artifacts = self._load_segments()
            return (
                {
                    "product_passport": passport,
                    "segment_set_id": segment_set_id,
                    "segments": segments,
                    "limitations": limitations,
                },
                product_name,
                {"passport": passport_artifact, "segments": segment_artifacts},
            )

        audience_result, audience_artifact = self._load_audience_result(self.arguments.audience_result)
        return (
            {
                "product_passport": passport,
                "audience_result": audience_result,
                "candidate_count": self.arguments.candidate_count,
                "validation_target": self.arguments.validation_target,
            },
            product_name,
            {"passport": passport_artifact, "audience_result": audience_artifact},
        )

    def _load_product_input(self) -> tuple[ProductInput, list[str]]:
        if self.arguments.input:
            raw_input = self._read_json(self.arguments.input)
            product_input = ProductInput.model_validate(raw_input)
            return product_input, [source.source for source in product_input.sources]

        repo_root = Path(__file__).resolve().parents[3]
        source_directory = repo_root / "n8n" / "product" / "cafe"
        if not source_directory.is_dir():
            raise ValueError(f"product source directory not found: {source_directory}")

        form = {
            "name": "Tempo",
            "description": "Сервис для создания и распространения рабочих расписаний небольших кафе.",
            "category": "food",
            "custom_category": None,
            "brand": "Tempo",
            "country": "Россия",
            "town": None,
            "experiment_goal": (
                "Проверить интерес владельцев и управляющих небольших кафе к пробному использованию Tempo "
                "для планирования смен."
            ),
            "goal_origin": "user_provided",
            "target_action": "Запросить демонстрацию или начать пробное использование Tempo.",
            "primary_metric": "tempo_trial_interest",
            "metric_direction": "increase",
            "metric_numerator": (
                "Уникальные владельцы или управляющие, запросившие демонстрацию или начавшие пробное использование"
            ),
            "metric_denominator": "Уникальные подходящие участники, которым было предложено попробовать Tempo",
            "success_criterion": None,
            "price": {"status": "unknown", "description": "Цена не указана."},
            "allowed_channels": [],
            "forbidden_claims": [
                "Не утверждать, что Tempo автоматически планирует смены.",
                "Не утверждать, что Tempo прогнозирует спрос.",
            ],
            "known_constraints": ["Спрос и готовность платить не подтверждены."],
            "keywords": ["кафе", "расписание смен"],
            "unknowns": [
                "Фактическая цена и условия пробного периода не указаны.",
                "Наличие интереса и готовность платить не подтверждены.",
            ],
            "link": None,
        }
        source_paths = [path for path in sorted(source_directory.glob("*.md")) if path.name.lower() != "readme.md"]
        sources = [{"source": "product_form", "content": json.dumps(form, ensure_ascii=False)}]
        sources.extend(
            {"source": f"document:{path.name}", "content": path.read_text(encoding="utf-8")} for path in source_paths
        )
        product_input = ProductInput.model_validate(
            {
                "target_product": "Tempo",
                "as_of_date": None,
                "sources": sources,
                "unavailable_sources": [],
            }
        )
        return product_input, ["product_form", *(f"document:{path.name}" for path in source_paths)]

    def _load_passport(self, path: Path) -> tuple[dict[str, Any], str, dict[str, Any]]:
        document = self._read_json(path)
        if "status" in document and document["status"] != "completed":
            raise ValueError(f"product report {path} has status {document['status']!r}, expected 'completed'")

        result = document.get("result", document)
        raw_passport = result.get("passport", result)
        passport = Passport.model_validate(raw_passport)
        if passport.extraction_status == "blocked" or not passport.records:
            raise ValueError(f"product passport {path} is blocked or contains no records")

        product_name = passport.product_name or document.get("product") or "unknown product"
        artifact = {"path": str(path), "run_id": document.get("run_id")}
        return passport.model_dump(mode="json"), product_name, artifact

    def _load_segments(self) -> tuple[list[dict[str, Any]], str, list[str], dict[str, Any]]:
        candidate_path = self.arguments.segments
        metadata_path = self.arguments.segment_metadata
        candidates_document = self._read_json(candidate_path)
        metadata_document = self._read_json(metadata_path)
        metadata = metadata_document.get("report", metadata_document)
        segments = candidates_document.get("segments")
        if not isinstance(segments, list) or not segments:
            raise ValueError(f"prepared segment file {candidate_path} must contain a non-empty segments array")

        segment_ids = [segment.get("segment_id") for segment in segments if isinstance(segment, dict)]
        if len(segment_ids) != len(segments) or any(not isinstance(item, str) or not item for item in segment_ids):
            raise ValueError(f"prepared segment file {candidate_path} has a segment without a segment_id")

        if len(set(segment_ids)) != len(segment_ids):
            raise ValueError(f"prepared segment file {candidate_path} contains duplicate segment IDs")

        provenance = candidates_document.get("data_provenance") or metadata.get("data_provenance")
        metadata_provenance = metadata.get("data_provenance")
        if provenance and metadata_provenance and provenance != metadata_provenance:
            raise ValueError("segment candidate and metadata files have different data provenance")

        if not provenance:
            raise ValueError("prepared segment files must declare data_provenance")

        limitations = ["Сегменты исследовательские; проверьте качество, происхождение и активацию данных."]
        warnings = metadata.get("warnings", [])
        if isinstance(warnings, list):
            limitations.extend(item for item in warnings if isinstance(item, str))

        status = candidates_document.get("status")
        if status:
            limitations.append(f"Статус набора сегментов: {status}.")

        if provenance == "synthetic_demo":
            limitations.append("Данные синтетические: размеры сегментов не отражают реальных клиентов.")

        digest = hashlib.sha256(candidate_path.read_bytes() + metadata_path.read_bytes()).hexdigest()
        segment_set_id = candidates_document.get("segment_set_id")
        if not isinstance(segment_set_id, str) or not segment_set_id:
            segment_set_id = f"local:{digest}"

        artifact = {
            "candidates_path": str(candidate_path),
            "metadata_path": str(metadata_path),
            "segment_set_id": segment_set_id,
            "status": status,
            "data_provenance": provenance,
            "source_archive": candidates_document.get("source_archive"),
        }
        return segments, segment_set_id, list(dict.fromkeys(limitations)), artifact

    def _load_audience_result(self, path: Path) -> tuple[dict[str, Any], dict[str, Any]]:
        document = self._read_json(path)
        if "status" in document and document["status"] != "completed":
            raise ValueError(f"audience report {path} has status {document['status']!r}, expected 'completed'")

        result = document.get("result", document)
        audience = result.get("audience", result)
        if not isinstance(audience, dict) or not audience.get("segments"):
            raise ValueError(f"audience report {path} has no selected segments; hypothesis generation requires them")

        artifact = {"path": str(path), "run_id": document.get("run_id")}
        return {"audience": audience}, artifact

    def _read_json(self, path: Path) -> dict[str, Any]:
        value = json.loads(path.read_text(encoding="utf-8"))
        if not isinstance(value, dict):
            raise ValueError(f"JSON input {path} must contain an object")

        return value

    def _output_path(self, run_id: str, product_name: str) -> Path:
        if self.arguments.output:
            return self.arguments.output

        timestamp = datetime.now(UTC).strftime("%Y%m%dT%H%M%SZ")
        product_slug = "".join(character.lower() if character.isalnum() else "-" for character in product_name)
        product_slug = "-".join(part for part in product_slug.split("-") if part)
        pipeline_slug = self.arguments.pipeline.removesuffix("_generation").replace("_", "-")
        return Path("runs") / f"{pipeline_slug}-{product_slug}-{timestamp}-{run_id[:8]}.json"

    def _report(
        self,
        result: dict[str, Any] | None,
        measurements: list[Measurement],
        run_id: str,
        started_at: str,
        product_name: str,
        status: str,
        input_artifacts: dict[str, Any],
    ) -> dict[str, Any]:
        input_tokens = sum(item.input_tokens or 0 for item in measurements)
        output_tokens = sum(item.output_tokens or 0 for item in measurements)
        costs = [item.provider_cost for item in measurements]
        total_cost = (
            sum(cost for cost in costs if cost is not None)
            if costs and all(cost is not None for cost in costs)
            else None
        )
        report: dict[str, Any] = {
            "run_id": run_id,
            "started_at": started_at,
            "finished_at": datetime.now(UTC).isoformat(),
            "status": status,
            "pipeline": self.arguments.pipeline,
            "product": product_name,
            "provider": self.arguments.provider,
            "model": self.arguments.model,
            "reasoning_effort": self.arguments.reasoning_effort,
            "input_artifacts": input_artifacts,
            "usage": {
                "calls": len(measurements),
                "input_tokens": input_tokens,
                "output_tokens": output_tokens,
                "total_tokens": input_tokens + output_tokens,
                "openrouter_cost_credits": total_cost,
                "calls_detail": [asdict(item) for item in measurements],
            },
            "result": result,
        }
        return report

    def _save_run(self, report: dict[str, Any], output_path: Path) -> None:
        output_path.parent.mkdir(parents=True, exist_ok=True)
        report_json = json.dumps(report, ensure_ascii=False, indent=2)
        output_path.write_text(report_json + "\n", encoding="utf-8")
        database_path = self.arguments.database
        database_path.parent.mkdir(parents=True, exist_ok=True)
        usage = report["usage"]
        with sqlite3.connect(database_path) as connection:
            connection.execute(
                """
                CREATE TABLE IF NOT EXISTS pipeline_runs (
                    run_id TEXT PRIMARY KEY,
                    started_at TEXT NOT NULL,
                    finished_at TEXT NOT NULL,
                    status TEXT NOT NULL,
                    pipeline TEXT NOT NULL,
                    product TEXT NOT NULL,
                    provider TEXT NOT NULL,
                    model TEXT NOT NULL,
                    reasoning_effort TEXT,
                    input_tokens INTEGER NOT NULL,
                    output_tokens INTEGER NOT NULL,
                    total_tokens INTEGER NOT NULL,
                    openrouter_cost_credits TEXT,
                    report_path TEXT NOT NULL,
                    report_json TEXT NOT NULL
                )
                """
            )
            connection.execute(
                """
                INSERT INTO pipeline_runs (
                    run_id, started_at, finished_at, status, pipeline, product, provider, model,
                    reasoning_effort, input_tokens, output_tokens, total_tokens,
                    openrouter_cost_credits, report_path, report_json
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    report["run_id"],
                    report["started_at"],
                    report["finished_at"],
                    report["status"],
                    report["pipeline"],
                    report["product"],
                    report["provider"],
                    report["model"],
                    report["reasoning_effort"],
                    usage["input_tokens"],
                    usage["output_tokens"],
                    usage["total_tokens"],
                    str(usage["openrouter_cost_credits"]) if usage["openrouter_cost_credits"] is not None else None,
                    str(output_path),
                    report_json,
                ),
            )

    def _print_summary(self, output_path: Path, report: dict[str, Any]) -> None:
        usage = report["usage"]
        print(f"Saved result: {output_path}")
        print(f"Saved run history: {self.arguments.database}")
        print(f"Provider/model: {self.arguments.provider} / {self.arguments.model}")
        print(
            f"Status: {report['status']}; calls: {usage['calls']}; input tokens: {usage['input_tokens']}; "
            f"output tokens: {usage['output_tokens']}; total tokens: {usage['total_tokens']}"
        )
        cost = usage["openrouter_cost_credits"]
        if cost is not None:
            print(f"OpenRouter-reported cost: {cost:.8f} credits")
        else:
            print("OpenRouter did not report cost for every call; see per-call usage in the result file.")


def _parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run a generator pipeline locally with validated artifact inputs.")
    parser.add_argument(
        "--pipeline",
        choices=("product_analysis", "audience_selection", "hypothesis_generation"),
        default="product_analysis",
    )
    parser.add_argument("--input", type=Path, help="ProductInput JSON; only for product_analysis")
    parser.add_argument("--passport", type=Path, help="product report JSON (or raw product passport)")
    parser.add_argument(
        "--segments", type=Path, help="prepared segment_candidates.json; required for audience_selection"
    )
    parser.add_argument(
        "--segment-metadata", type=Path, help="matching segmentation_report.json; required for audience_selection"
    )
    parser.add_argument("--audience-result", type=Path, help="audience report JSON; required for hypothesis_generation")
    parser.add_argument("--candidate-count", type=int, default=5)
    parser.add_argument("--validation-target", type=int, default=3)
    parser.add_argument("--provider", default="openrouter", help="provider label for the saved report")
    parser.add_argument(
        "--base-url",
        default=os.environ.get("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
        help="OpenAI-compatible API base URL",
    )
    parser.add_argument("--api-key", default=os.environ.get("OPENROUTER_API_KEY"))
    parser.add_argument(
        "--model",
        default=os.environ.get("OPENROUTER_MODEL", "deepseek/deepseek-v4.1-flash-1017"),
    )
    parser.add_argument("--reasoning-effort", choices=("low", "medium", "high"))
    parser.add_argument("--timeout", type=float, default=120.0)
    parser.add_argument("--max-retries", type=int, default=2)
    parser.add_argument("--output", type=Path, help="result JSON path (default: runs/ with timestamp)")
    parser.add_argument("--database", type=Path, default=Path("runs/pipeline_runs.sqlite3"))
    arguments = parser.parse_args()
    if arguments.pipeline == "product_analysis":
        if arguments.passport or arguments.segments or arguments.segment_metadata or arguments.audience_result:
            parser.error(
                "--passport, --segments, --segment-metadata and --audience-result are for downstream pipelines"
            )

    elif arguments.pipeline == "audience_selection":
        if arguments.input or arguments.audience_result:
            parser.error("audience_selection uses --passport, --segments and --segment-metadata, not --input")

        for name in ("passport", "segments", "segment_metadata"):
            if getattr(arguments, name) is None:
                parser.error(f"--{name.replace('_', '-')} is required for audience_selection")

    else:
        if arguments.input or arguments.segments or arguments.segment_metadata:
            parser.error("hypothesis_generation uses --passport and --audience-result, not --input/--segments")

        for name in ("passport", "audience_result"):
            if getattr(arguments, name) is None:
                parser.error(f"--{name.replace('_', '-')} is required for hypothesis_generation")

    if not arguments.api_key:
        parser.error("set --api-key or OPENROUTER_API_KEY in the repo-root .env file")

    return arguments


def main() -> None:
    repo_root = Path(__file__).resolve().parents[3]
    load_dotenv(repo_root / ".env", override=False)
    PipelineRunner(_parse_arguments()).run()


if __name__ == "__main__":
    main()
