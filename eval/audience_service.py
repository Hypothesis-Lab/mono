from __future__ import annotations

import asyncio
import json
import os
import re
import tempfile
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any

from tempo_eval.runner import compact_product_passport, load_product_passport, run_models

ROOT = Path(__file__).resolve().parent
DEFAULT_PRODUCT_RESULT = ROOT / "results/additional-models.json"
DEFAULT_DATASET = ROOT / "События аудиторий (датасет).xlsx"
DEFAULT_DB = ROOT / "audience.sqlite"
DEFAULT_SEMANTIC_INDEX = ROOT / "semantic-index"
RUNS_DIR = Path(os.environ.get("TEMPO_AUDIENCE_SERVICE_RUNS", tempfile.gettempdir())) / "tempo-audience-runs"


def _path(name: str, default: Path) -> Path:
    return Path(os.environ.get(name, str(default))).expanduser().resolve()


def run_audience(request: dict[str, Any]) -> dict[str, Any]:
    model = request.get("model", os.environ.get("OPENROUTER_MODEL", "minimax/minimax-m3"))
    if not isinstance(model, str) or not re.fullmatch(r"[A-Za-z0-9._/-]+", model):
        raise ValueError("model must be an OpenRouter model ID")
    api_key = os.environ.get("OPENROUTER_API_KEY")
    if not api_key:
        raise ValueError("OPENROUTER_API_KEY is required")
    product_result = _path("TEMPO_AUDIENCE_PRODUCT_RESULT", DEFAULT_PRODUCT_RESULT)
    product_model = request.get("product_model") or os.environ.get("TEMPO_AUDIENCE_PRODUCT_MODEL", "minimax/minimax-m3")
    product_passport = compact_product_passport(load_product_passport(product_result, product_model))
    run_id = uuid.uuid4().hex[:12]
    output_dir = RUNS_DIR / run_id
    output_path = output_dir / "result.json"
    checkpoint_dir = output_dir / "checkpoints"
    asyncio.run(
        run_models(
            [model],
            [],
            target_product=str(request.get("target_product", "Tempo")),
            as_of_date=request.get("as_of_date"),
            api_key=api_key,
            base_url=os.environ.get("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
            checkpoint_dir=checkpoint_dir,
            output_path=output_path,
            timeout_seconds=float(request.get("timeout", 120)),
            step_retries=int(request.get("step_retries", 1)),
            app_url=os.environ.get("OPENROUTER_APP_URL"),
            app_name="Tempo n8n audience visual mirror",
            run_id=run_id,
            pipeline="audience",
            audience_dataset=_path("TEMPO_AUDIENCE_DATASET", DEFAULT_DATASET),
            audience_db=_path("TEMPO_AUDIENCE_DB", DEFAULT_DB),
            product_passport=product_passport,
            semantic_index_dir=_path("TEMPO_AUDIENCE_SEMANTIC_INDEX", DEFAULT_SEMANTIC_INDEX),
            run_timeout_seconds=float(request.get("run_timeout", 600)),
        )
    )
    payload = json.loads(output_path.read_text(encoding="utf-8"))
    result = payload.get("results", [{}])[0]
    return {"run_id": run_id, **result}


class Handler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:  # noqa: N802
        if self.path != "/health":
            self.send_error(404)
            return
        self._write(200, {"status": "ok", "service": "tempo-audience-python"})

    def do_POST(self) -> None:  # noqa: N802
        if self.path != "/audience/run":
            self.send_error(404)
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
            request = json.loads(self.rfile.read(length) or b"{}")
            response = run_audience(request)
            self._write(200, response)
        except Exception as error:  # noqa: BLE001
            self._write(500, {"status": "failed", "error": f"{type(error).__name__}: {error}"})

    def _write(self, status: int, payload: dict[str, Any]) -> None:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format: str, *args: object) -> None:
        return


def main() -> None:
    host = os.environ.get("TEMPO_AUDIENCE_SERVICE_HOST", "127.0.0.1")
    port = int(os.environ.get("TEMPO_AUDIENCE_SERVICE_PORT", "8098"))
    RUNS_DIR.mkdir(parents=True, exist_ok=True)
    server = ThreadingHTTPServer((host, port), Handler)
    print(f"Tempo audience Python service listening on http://{host}:{port}")
    server.serve_forever()


if __name__ == "__main__":
    main()
