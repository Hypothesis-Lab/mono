from __future__ import annotations

# The generated HTML template intentionally contains long CSS and JavaScript lines.
# ruff: noqa: E501
import argparse
import html
import json
import sqlite3
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from langgraph.checkpoint.serde.jsonplus import JsonPlusSerializer

ALLOWED_CHECKPOINT_TYPES = [
    ("tempo_eval.models", "Source"),
    ("tempo_eval.models", "Draft"),
    ("tempo_eval.models", "Passport"),
    ("tempo_eval.models", "Validation"),
    ("tempo_eval.models", "Evaluation"),
    ("tempo_eval.models", "AudienceClaim"),
    ("tempo_eval.models", "AudienceWindow"),
    ("tempo_eval.models", "AudienceFeature"),
    ("tempo_eval.models", "ExcludedAudienceFeature"),
    ("tempo_eval.models", "AudienceDataProfile"),
    ("tempo_eval.models", "AudienceDefinition"),
    ("tempo_eval.models", "AudienceSize"),
    ("tempo_eval.models", "AudienceCheck"),
    ("tempo_eval.models", "AudienceInterpretation"),
    ("tempo_eval.models", "AudienceSegment"),
    ("tempo_eval.models", "AudienceOverlaps"),
    ("tempo_eval.models", "AudienceResult"),
    ("tempo_eval.models", "AudienceReviewCheck"),
    ("tempo_eval.models", "ProductManagerConfirmation"),
    ("tempo_eval.models", "AudienceReview"),
    ("tempo_eval.models", "AudienceFilter"),
    ("tempo_eval.models", "AudienceQueryCondition"),
    ("tempo_eval.models", "AudienceQuery"),
    ("tempo_eval.models", "AudienceQueryDecision"),
    ("tempo_eval.models", "AudienceNarrativeClaim"),
    ("tempo_eval.models", "AudienceNarrative"),
    ("tempo_eval.models", "AudienceJTBDAlignment"),
    ("tempo_eval.models", "AudienceAgentCheck"),
    ("tempo_eval.models", "AudienceAgentReview"),
    ("tempo_eval.models", "AudienceEvidenceRequest"),
    ("tempo_eval.models", "AudienceReviewDecision"),
    ("tempo_eval.models", "AudiencePrivacyReview"),
]


@dataclass
class CheckpointSnapshot:
    model: str
    pipeline: str
    path: str
    step: int | None
    state: dict[str, Any]
    metadata: dict[str, Any]


class ReportBuilder:
    def __init__(self, results_dir: Path, checkpoints_dir: Path, pipeline: str = "auto") -> None:
        self.results_dir = results_dir
        self.checkpoints_dir = checkpoints_dir
        self.pipeline = pipeline
        self.max_runs = 3
        self.global_evaluation: dict[str, Any] | None = None

    def build(self) -> dict[str, Any]:
        models = self._load_results()
        for checkpoint in self._load_checkpoints():
            if self.pipeline != "auto" and checkpoint.pipeline != self.pipeline:
                continue
            key = f"{checkpoint.pipeline}:{checkpoint.model}"
            item = models.setdefault(
                key, {"model": checkpoint.model, "pipeline": checkpoint.pipeline, "checkpoints": []}
            )
            snapshot = {
                "path": checkpoint.path,
                "step": checkpoint.step,
                "state": checkpoint.state,
                "metadata": checkpoint.metadata,
            }
            item.setdefault("checkpoints", []).append(snapshot)
            item["checkpoint"] = snapshot
            item.setdefault("status", "checkpointed")
            item.setdefault("result", self._result_from_checkpoint(checkpoint.state))
        return {
            "generated_at": datetime.now(UTC).isoformat(),
            "benchmark": {"max_runs": self.max_runs},
            "models": list(models.values()),
            "global_evaluation": self.global_evaluation,
        }

    def write(self, output: Path) -> None:
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(render_html(self.build()), encoding="utf-8")

    def _load_results(self) -> dict[str, dict[str, Any]]:
        models: dict[str, dict[str, Any]] = {}
        paths = [self.results_dir] if self.results_dir.is_file() else sorted(self.results_dir.glob("*.json"))
        for path in paths:
            try:
                payload = json.loads(path.read_text(encoding="utf-8"))
            except (OSError, json.JSONDecodeError):
                continue
            benchmark = payload.get("benchmark", {})
            pipeline = (
                payload.get("pipeline")
                or (benchmark.get("pipeline") if isinstance(benchmark, dict) else None)
                or "product"
            )
            if self.pipeline != "auto" and pipeline != self.pipeline:
                continue
            if isinstance(payload.get("global_evaluation"), dict):
                self.global_evaluation = payload["global_evaluation"]
            if isinstance(benchmark, dict) and isinstance(benchmark.get("max_runs"), int):
                self.max_runs = benchmark["max_runs"]
            elif self.results_dir.is_file() and all(
                "run_number" not in result for result in payload.get("results", [])
            ):
                self.max_runs = 1
            for result in payload.get("results", []):
                model = result.get("model")
                if isinstance(model, str):
                    key = f"{pipeline}:{model}"
                    item = models.setdefault(
                        key, {"model": model, "pipeline": pipeline, "runs": [], "benchmark_evaluations": []}
                    )
                    if "run_number" in result:
                        item["runs"].append({**result, "result_file": str(path)})
                        item.update(
                            {
                                "status": result.get("status", "unknown"),
                                "result": result.get("result", {}),
                                "run_number": result.get("run_number"),
                                "result_file": str(path),
                                "call_count": result.get("call_count"),
                            }
                        )
                    else:
                        item["legacy_run"] = {**result, "run_number": 1, "result_file": str(path)}
                        item.update({**result, "result_file": str(path)})
            for evaluation in payload.get("evaluations", []):
                model = evaluation.get("model")
                if isinstance(model, str):
                    key = f"{pipeline}:{model}"
                    item = models.setdefault(
                        key, {"model": model, "pipeline": pipeline, "runs": [], "benchmark_evaluations": []}
                    )
                    item.setdefault("benchmark_evaluations", []).append(evaluation)
        for item in models.values():
            if not item.get("runs") and item.get("legacy_run"):
                item["runs"] = [item["legacy_run"]]
            item["runs"].sort(key=lambda run: (run.get("run_number", 1), run.get("result_file", "")))
        return models

    def _load_checkpoints(self) -> list[CheckpointSnapshot]:
        snapshots: list[CheckpointSnapshot] = []
        for path in sorted(self.checkpoints_dir.glob("**/*.sqlite")):
            snapshot = read_checkpoint(path)
            if snapshot is not None:
                snapshots.append(snapshot)
        return snapshots

    @staticmethod
    def _result_from_checkpoint(state: dict[str, Any]) -> dict[str, Any]:
        return {
            key: state[key]
            for key in (
                "passport",
                "audience",
                "data_profile",
                "semantic_cards",
                "query",
                "calculation",
                "validation",
                "review",
                "evaluation",
            )
            if key in state
        }


def read_checkpoint(path: Path) -> CheckpointSnapshot | None:
    try:
        connection = sqlite3.connect(f"file:{path.resolve()}?mode=ro", uri=True)
        row = connection.execute(
            "SELECT thread_id, type, checkpoint, metadata FROM checkpoints ORDER BY rowid DESC LIMIT 1"
        ).fetchone()
        connection.close()
        if row is None:
            return None
        thread_id, checkpoint_type, checkpoint_blob, metadata_blob = row
        serde = JsonPlusSerializer(allowed_msgpack_modules=ALLOWED_CHECKPOINT_TYPES)
        checkpoint = serde.loads_typed((checkpoint_type, checkpoint_blob))
        metadata = json.loads(metadata_blob.decode("utf-8"))
        state = to_jsonable(checkpoint.get("channel_values", {}))
        model = state.get("model") or _model_from_thread_id(thread_id, path)
        return CheckpointSnapshot(
            model=model,
            pipeline=state.get("pipeline") or ("audience" if "audience" in state else "product"),
            path=str(path),
            step=metadata.get("step"),
            state=state,
            metadata=metadata,
        )
    except (OSError, sqlite3.Error, ValueError, TypeError, json.JSONDecodeError):
        return None


def to_jsonable(value: Any) -> Any:
    if hasattr(value, "model_dump"):
        return to_jsonable(value.model_dump(mode="json"))
    if isinstance(value, dict):
        return {str(key): to_jsonable(item) for key, item in value.items()}
    if isinstance(value, (list, tuple)):
        return [to_jsonable(item) for item in value]
    if isinstance(value, (str, int, float, bool)) or value is None:
        return value
    return str(value)


def render_html(payload: dict[str, Any]) -> str:
    data = json.dumps(payload, ensure_ascii=False).replace("</", "<\\/")
    fallback = render_fallback(payload)
    return f"""<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
   <title>Pipeline evaluation report</title>
  <style>
    :root {{ color-scheme: dark; font-family: Inter, ui-sans-serif, system-ui, sans-serif; }}
    body {{ margin: 0; background: #101318; color: #e8edf4; }}
    header {{ padding: 28px 34px 20px; border-bottom: 1px solid #2a303b; }}
    h1, h2, h3, p {{ margin-top: 0; }}
    h1 {{ margin-bottom: 8px; font-size: 24px; }}
    .muted {{ color: #91a0b5; }}
    .layout {{ display: grid; grid-template-columns: 280px 1fr; min-height: calc(100vh - 100px); }}
    aside {{ padding: 18px; border-right: 1px solid #2a303b; }}
    main {{ min-width: 0; padding: 28px 34px; }}
    button {{ width: 100%; padding: 12px; margin-bottom: 8px; border: 1px solid #303846; border-radius: 8px; background: #171c25; color: inherit; text-align: left; cursor: pointer; }}
    button.active {{ background: #263754; border-color: #5d9cff; }}
    .badge {{ display: inline-block; padding: 3px 8px; border-radius: 999px; font-size: 12px; background: #303846; }}
    .completed {{ background: #164f3b; color: #9ef0c8; }}
    .checkpointed {{ background: #594616; color: #ffe29a; }}
    .failed {{ background: #642d37; color: #ffc2ca; }}
    .cards {{ display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 12px; margin: 20px 0 28px; }}
    .card, section {{ background: #171c25; border: 1px solid #2a303b; border-radius: 10px; padding: 16px; }}
    .card strong {{ display: block; font-size: 22px; margin-top: 5px; }}
     .tabs {{ display: flex; gap: 8px; margin-bottom: 14px; }}
     .tabs button {{ width: auto; margin: 0; padding: 8px 14px; }}
     .tabs button.active {{ background: #263754; border-color: #5d9cff; color: #ffffff; box-shadow: 0 0 0 1px #5d9cff inset; }}
     .subtabs {{ display: flex; gap: 8px; margin: 16px 0; }}
     .subtabs button {{ width: auto; margin: 0; padding: 8px 14px; }}
     .subtabs button.active {{ background: #263754; border-color: #5d9cff; color: #ffffff; box-shadow: 0 0 0 1px #5d9cff inset; }}
    .panel-header {{ display: flex; align-items: center; justify-content: space-between; gap: 16px; }}
    .panel-header h3 {{ margin-bottom: 0; }}
    .view-toggle {{ display: flex; gap: 6px; }}
    .view-toggle button {{ width: auto; margin: 0; padding: 7px 12px; }}
    .view-toggle button.active {{ background: #263754; border-color: #5d9cff; }}
    .run-selector {{ display: flex; flex-wrap: wrap; gap: 8px; margin: 12px 0 16px; }}
    .run-selector button {{ width: auto; margin: 0; padding: 7px 12px; }}
    .run-selector button.active {{ background: #263754; border-color: #5d9cff; }}
    .hero {{ display: flex; flex-wrap: wrap; gap: 10px 24px; align-items: baseline; margin: 10px 0 20px; }}
    .hero strong {{ font-size: 22px; }}
    .record-grid {{ display: grid; gap: 12px; }}
    .record, .criterion {{ padding: 16px; border: 1px solid #303846; border-radius: 8px; background: #12161e; }}
    .record h4, .criterion h4 {{ margin: 0 0 8px; }}
    .record-meta {{ display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 10px; }}
    .tag {{ display: inline-block; padding: 3px 8px; border-radius: 5px; background: #293449; color: #bcd5ff; font-size: 12px; }}
    .statement {{ margin: 8px 0 12px; font-size: 16px; line-height: 1.5; }}
    .evidence {{ margin-top: 12px; padding: 10px 12px; border-left: 3px solid #4c78b8; background: #171e2b; }}
    .evidence blockquote {{ margin: 5px 0 0; white-space: pre-wrap; color: #d2dbea; }}
    .callout {{ margin: 16px 0; padding: 14px 16px; border-radius: 8px; background: #1b2330; }}
    .callout.warn {{ border-left: 3px solid #d1a73c; }}
     .callout.error {{ border-left: 3px solid #d15b6c; }}
     .table-wrap {{ overflow-x: auto; margin: 12px 0; }}
     .data-table {{ width: 100%; border-collapse: collapse; min-width: 620px; }}
     .data-table th, .data-table td {{ padding: 9px 10px; border-bottom: 1px solid #303846; text-align: left; vertical-align: top; }}
     .data-table th {{ color: #bcd5ff; font-size: 12px; text-transform: uppercase; letter-spacing: .04em; }}
     .data-table td:nth-child(3), .data-table td:nth-child(4), .data-table td:nth-child(5) {{ white-space: nowrap; text-align: right; }}
     .criterion-list {{ display: grid; gap: 10px; }}
    .criterion-top {{ display: flex; justify-content: space-between; gap: 12px; }}
    .score {{ font-size: 20px; font-weight: 700; color: #9ef0c8; }}
    .check-list {{ display: grid; gap: 8px; padding: 0; list-style: none; }}
    .check-list li {{ padding: 9px 12px; border-radius: 6px; background: #12161e; }}
    pre {{ max-height: 650px; overflow: auto; padding: 16px; border-radius: 8px; background: #0d1015; color: #cbd5e1; white-space: pre-wrap; word-break: break-word; }}
    .hidden {{ display: none; }}
    .ok {{ color: #9ef0c8; }}
    .warn {{ color: #ffe29a; }}
    .error {{ color: #ffc2ca; }}
    .fallback {{ max-width: 1100px; margin: 28px 34px; }}
    .fallback pre {{ max-height: 420px; }}
    @media (max-width: 760px) {{ .layout {{ display: block; }} aside {{ border-right: 0; border-bottom: 1px solid #2a303b; }} main {{ padding: 22px 18px; }} header {{ padding: 22px 18px; }} }}
  </style>
</head>
<body>
  <header>
    <h1>Pipeline evaluation report</h1>
    <p class="muted">Snapshot generated at <span id="generated"></span></p>
  </header>
  <div class="layout">
    <aside id="models"></aside>
    <main>
      {fallback}
      <div id="empty" class="muted">No model results or checkpoints found.</div>
      <div id="content" class="hidden">
         <div class="tabs global-tabs">
           <button class="active" data-global-tab="models">Model reports</button>
           <button data-global-tab="global">Global comparison</button>
         </div>
         <div id="model-content">
           <h2 id="title"></h2>
           <div class="tabs">
             <button data-tab="result">Result</button>
             <button data-tab="evaluation">Evaluation</button>
             <button data-tab="checkpoint">Checkpoint</button>
             <button data-tab="raw">Raw model entry</button>
           </div>
           <section id="result-panel"></section>
           <section id="evaluation-panel" class="hidden"></section>
           <section id="checkpoint-panel" class="hidden"></section>
           <section id="raw-panel" class="hidden"></section>
         </div>
         <section id="global-panel" class="hidden"></section>
      </div>
    </main>
  </div>
  <script id="report-data" type="application/json">{data}</script>
  <script>
    const report = JSON.parse(document.getElementById('report-data').textContent);
    const models = report.models || [];
    const modelNav = document.getElementById('models');
    const empty = document.getElementById('empty');
    const content = document.getElementById('content');
    const maxRuns = (report.benchmark && report.benchmark.max_runs) || 3;
    document.getElementById('generated').textContent = report.generated_at || 'unknown';
    let selected = 0;
    const selectedRuns = {{}};

    function pretty(value) {{ return JSON.stringify(value ?? null, null, 2); }}
    function esc(value) {{ return String(value ?? '').replace(/[&<>]/g, c => ({{'&':'&amp;','<':'&lt;','>':'&gt;'}}[c])); }}
    function badge(status) {{ return '<span class="badge ' + esc(status) + '">' + esc(status) + '</span>'; }}
    function runProgress(item) {{
      const runs = item.runs || [];
      const completed = runs.filter(run => run.status === 'completed').length;
      return completed + ' / ' + maxRuns;
    }}
    function renderNav() {{
      modelNav.innerHTML = models.map((item, index) =>
        '<button class="' + (index === selected ? 'active' : '') + '" data-model="' + index + '">' +
        '<strong>' + esc(item.model) + '</strong><br>' + badge(item.status || 'checkpointed') +
        ' <span class="muted">completed: ' + runProgress(item) + '</span>' +
        '</button>'
      ).join('');
      modelNav.querySelectorAll('[data-model]').forEach(button => button.onclick = () => {{ selected = Number(button.dataset.model); render(); }});
    }}
    function render() {{
      if (!models.length) {{ empty.classList.remove('hidden'); content.classList.add('hidden'); return; }}
      document.getElementById('static-fallback').classList.add('hidden');
      empty.classList.add('hidden'); content.classList.remove('hidden');
      const item = models[selected];
      const runs = item.runs || [];
       const runIndex = selectedRuns[item.model] ?? defaultRunIndex(item, runs);
      const selectedRun = runs[runIndex];
      const result = selectedRun ? (selectedRun.result || {{}}) : (item.result || {{}});
       const checkpoint = item.checkpoint || {{}};
       const latestState = checkpoint.state || {{}};
       const selectedCheckpoint = selectedRun && (item.checkpoints || []).find(snapshot =>
         String(snapshot.path || '').endsWith('/run-' + selectedRun.run_number + '.sqlite')
       );
       const state = selectedCheckpoint ? (selectedCheckpoint.state || {{}}) : latestState;
       const passport = result.passport || state.passport;
       const audience = result.audience || state.audience;
        const audienceDataProfile = result.data_profile || state.data_profile;
        const audienceSemanticCards = result.semantic_cards || state.semantic_cards;
        const audienceQuery = result.query || state.query;
        const audienceCalculation = result.calculation || state.calculation;
       const audienceReview = result.review || state.review;
      const validation = result.validation || state.validation;
      const benchmarkEvaluations = item.benchmark_evaluations || [];
      const latestBenchmark = benchmarkEvaluations.length ? benchmarkEvaluations[benchmarkEvaluations.length - 1].evaluation : null;
       const graphEvaluation = result.evaluation || state.evaluation;
      document.getElementById('title').innerHTML = esc(item.model) + ' ' + badge(item.status || 'checkpointed');
       document.getElementById('result-panel').innerHTML = (runs.length ? runSelector(runs, runIndex) : '') +
        summaryCards(item, state, selectedRun, passport, audience, validation, latestBenchmark, audienceCalculation) +
          panelHeader(item.pipeline === 'audience' ? 'Audience result' + (selectedRun ? ' · Run ' + selectedRun.run_number : '') : 'Passport result' + (selectedRun ? ' · Run ' + selectedRun.run_number : ''), 'result') +
          '<div data-pane="result" data-mode="rendered">' + (item.pipeline === 'audience' ? audienceRendered(audience, validation, audienceCalculation) : resultRendered(passport, validation)) + '</div>' +
         '<div data-pane="result" data-mode="json" class="hidden"><pre>' + esc(pretty(runs.length ? {{ selected_run: selectedRun, runs }} : {{ passport, audience, validation }})) + '</pre></div>';
       const runEvaluations = runs.map(run => ({{ run: run.run_number, evaluation: run.result && run.result.evaluation }})).filter(item => item.evaluation);
       const graphJson = item.pipeline === 'audience'
          ? {{ run: selectedRun ? selectedRun.run_number : null, data_profile: audienceDataProfile, semantic_cards: audienceSemanticCards, query: audienceQuery, calculation: audienceCalculation, audience, validation, review: audienceReview }}
         : {{ run: selectedRun ? selectedRun.run_number : null, evaluation: graphEvaluation, all_runs: runEvaluations }};
       const stabilityJson = {{ benchmark: stabilityView(latestBenchmark), runs: runs.map(run => ({{ run: run.run_number, validation: run.result && run.result.validation }})) }};
       document.getElementById('evaluation-panel').innerHTML = panelHeader('Evaluation', 'evaluation') +
         '<div class="subtabs">' +
         '<button class="active" data-evaluation-tab="graph">Graph evaluation</button>' +
         '<button data-evaluation-tab="stability">Stability evaluation (C9)</button>' +
         '</div>' +
         '<div data-evaluation-pane="graph">' +
          (item.pipeline === 'audience' ? '<div class="callout"><strong>Audience pipeline validation</strong><p class="muted">Safe data profile → constrained local query → aggregate calculation → interpretation.</p></div>' : '<div class="callout"><strong>Evaluation inside the graph</strong><p class="muted">Evaluates the selected extraction run independently.</p></div>') +
         (runs.length ? '<div class="callout"><strong>Per-run validation</strong><ul class="check-list">' + runs.map(run => '<li>Run ' + esc(run.run_number) + ': ' + validationLabel(run.result && run.result.validation) + '</li>').join('') + '</ul></div>' : '') +
           '<div data-pane="evaluation" data-mode="rendered">' + (item.pipeline === 'audience' ? audiencePipelineRendered(audienceDataProfile, audienceSemanticCards, audienceQuery, audienceCalculation, audience, validation, audienceReview) : evaluationRendered(graphEvaluation)) +
         (item.pipeline !== 'audience' && runEvaluations.length ? '<h3>Individual run evaluations</h3><div class="criterion-list">' + runEvaluations.map(item => '<article class="criterion"><h4>Run ' + esc(item.run) + '</h4>' + evaluationSummary(item.evaluation) + '</article>').join('') + '</div>' : '') +
         '</div>' +
         '<div data-pane="evaluation" data-mode="json" class="hidden"><pre>' + esc(pretty(graphJson)) + '</pre></div>' +
         '</div>' +
         '<div data-evaluation-pane="stability" class="hidden">' +
         '<div class="callout"><strong>Stability evaluation across runs</strong><p class="muted">C9 compares the completed extraction runs. It is separate from the graph evaluation and per-run validation.</p></div>' +
         '<div data-pane="evaluation" data-mode="rendered">' + stabilityEvaluationRendered(latestBenchmark) + '</div>' +
         '<div data-pane="evaluation" data-mode="json" class="hidden"><pre>' + esc(pretty(stabilityJson)) + '</pre></div>' +
         '</div>';
       document.getElementById('checkpoint-panel').innerHTML = '<h3>Latest checkpoint</h3><pre>' + esc(pretty({{ path: checkpoint.path, step: checkpoint.step, metadata: checkpoint.metadata, state: latestState }})) + '</pre>';
      document.getElementById('raw-panel').innerHTML = '<h3>Raw model entry</h3><pre>' + esc(pretty(item)) + '</pre>';
       bindViewToggle('result');
       bindViewToggle('evaluation');
       bindEvaluationTabs();
       bindRunSelector(item.model);
       renderGlobal();
       renderNav();
     }}
    function runSelector(runs, runIndex) {{
       return '<div class="run-selector"><strong>Runs (' + runs.length + '/' + maxRuns + '):</strong>' + runs.map((run, index) =>
         '<button class="' + (index === runIndex ? 'active' : '') + '" data-run-index="' + index + '">Run ' +
         esc(run.run_number) + ' · ' + esc(run.status || 'unknown') + ' · validation: ' + validationLabel(run.result && run.result.validation) + '</button>').join('') + '</div>';
     }}
     function bindRunSelector(model) {{
       document.querySelectorAll('[data-run-index]').forEach(button => button.onclick = () => {{
         selectedRuns[model] = Number(button.dataset.runIndex);
         render();
       }});
     }}
     function defaultRunIndex(item, runs) {{
       if (item.pipeline === 'product') {{
         for (let index = runs.length - 1; index >= 0; index -= 1) {{
           const passport = runs[index].result && runs[index].result.passport;
           if (runs[index].status === 'completed' && passport && passport.product_name) return index;
         }}
       }}
       return Math.max(runs.length - 1, 0);
     }}
    function evaluationSummary(evaluation) {{
      if (!evaluation) return '<p class="muted">No evaluation.</p>';
      return '<p><strong>' + esc(evaluation.overall_score == null ? 'Not scored' : evaluation.overall_score + ' / 100') +
        '</strong> · ' + esc(evaluation.evaluation_status) + ' · acceptance: ' + esc(evaluation.acceptance) + '</p>' +
        '<p>' + esc(evaluation.summary || '') + '</p>';
    }}
    function validationLabel(validation) {{
      if (!validation) return 'pending';
      return validation.valid ? 'passed' : 'failed';
    }}
     function summaryCards(item, state, selectedRun, passport, audience, validation, latestBenchmark, calculation) {{
       const c9Status = latestBenchmark ? (latestBenchmark.acceptance || latestBenchmark.evaluation_status || 'available') : 'pending';
       const c9Score = latestBenchmark && latestBenchmark.overall_score != null ? latestBenchmark.overall_score + ' / 100' : c9Status;
       const runValidation = validation ? (validation.valid ? '<span class="ok">passed</span>' : '<span class="error">failed</span>') : 'pending';
        const sourceDrafts = item.pipeline === 'audience' ? ((audience && audience.segments) || []).length : (Array.isArray(state.drafts) ? state.drafts.length : '—');
        const resultCount = item.pipeline === 'audience' ? ((audience && audience.segments) || []).length : (passport && passport.records ? passport.records.length : 0);
       const identifiedCount = item.pipeline === 'audience' && resultCount === 0 && calculation && calculation.count > 0 ? 1 : resultCount;
       return '<div class="cards">' +
         card('Pipeline', item.pipeline || 'product') +
         card('Stage', stage(item, state, selectedRun)) +
         card('Selected run', selectedRun ? 'Run ' + selectedRun.run_number : '—') +
         card(item.pipeline === 'audience' ? 'Segments' : 'Source drafts', sourceDrafts) +
         card(item.pipeline === 'audience' ? 'Audience segments' : 'Passport records', resultCount) +
         (item.pipeline === 'audience' ? card('Identified candidates', identifiedCount) : '') +
        card('Run validation', runValidation) +
        card('C9 stability', c9Score) +
        card('Completed runs', runProgress(item)) +
        card('Calls', selectedRun ? (selectedRun.call_count ?? '—') : (item.call_count ?? '—')) + '</div>';
    }}
    function panelHeader(title, kind) {{
      return '<div class="panel-header"><h3>' + title + '</h3>' +
        '<div class="view-toggle">' +
        '<button class="active" data-view-kind="' + kind + '" data-view-mode="rendered">Rendered</button>' +
        '<button data-view-kind="' + kind + '" data-view-mode="json">JSON</button>' +
        '</div></div>';
    }}
    function bindViewToggle(kind) {{
      const panel = document.getElementById(kind + '-panel');
      panel.querySelectorAll('[data-view-kind="' + kind + '"]').forEach(button => button.onclick = () => {{
        panel.querySelectorAll('[data-view-kind="' + kind + '"]').forEach(item => item.classList.toggle('active', item === button));
        panel.querySelectorAll('[data-pane="' + kind + '"]').forEach(pane => pane.classList.toggle('hidden', pane.dataset.mode !== button.dataset.viewMode));
      }});
    }}
     function bindEvaluationTabs() {{
      const panel = document.getElementById('evaluation-panel');
      panel.querySelectorAll('[data-evaluation-tab]').forEach(button => button.onclick = () => {{
        panel.querySelectorAll('[data-evaluation-tab]').forEach(tab => tab.classList.toggle('active', tab === button));
        panel.querySelectorAll('[data-evaluation-pane]').forEach(pane => pane.classList.toggle('hidden', pane.dataset.evaluationPane !== button.dataset.evaluationTab));
      }});
    }}
     function resultRendered(passport, validation) {{
      if (!passport) return '<p class="muted">No passport has been produced yet.</p>';
      const records = passport.records || [];
      const productJtbd = records.filter(record => record.category === 'jtbd');
      const recordsHtml = records.map((record, index) =>
        '<article class="record"><h4>' + esc(record.statement || ('Record ' + (index + 1))) + '</h4>' +
        '<div class="record-meta"><span class="tag">' + esc(record.category) + '</span>' +
        '<span class="tag">' + esc(record.statement_type) + '</span>' +
        '<span class="tag">' + esc(record.evidence_status) + '</span>' +
        (record.subject ? '<span class="muted">' + esc(record.subject) + '</span>' : '') + '</div>' +
        (record.conditions && record.conditions.length ? '<p><strong>Conditions:</strong> ' + esc(record.conditions.join('; ')) + '</p>' : '') +
        (record.time_scope ? '<p><strong>Time:</strong> ' + esc(record.time_scope) + '</p>' : '') +
        (record.evidence || []).map(evidence => '<div class="evidence"><strong>' + esc(evidence.source) +
          (evidence.locator ? ' · ' + esc(evidence.locator) : '') + '</strong><blockquote>' + esc(evidence.quote) + '</blockquote></div>').join('') +
        '</article>'
      ).join('');
      const conflicts = (passport.conflicts || []).map(conflict => '<li><strong>' + esc(conflict.record_ids.join(', ')) +
        ':</strong> ' + esc(conflict.description) + ' <span class="muted">' + esc(conflict.needed_to_resolve) + '</span></li>').join('');
      const gaps = (passport.gaps || []).map(gap => '<li><strong>' + esc(gap.category) + ':</strong> ' + esc(gap.missing) + '</li>').join('');
      const warnings = (passport.warnings || []).map(warning => '<li>' + esc(warning) + '</li>').join('');
       const validationErrors = validation && validation.errors ? validation.errors : [];
       return '<div class="hero"><strong>' + esc(passport.product_name || 'Unknown product') + '</strong>' +
         '<span class="tag">' + esc(passport.extraction_status) + '</span>' +
         (validation ? '<span class="tag">run validation: ' + validationLabel(validation) + '</span>' : '') + '</div>' +
         '<h3>Product JTBD (' + productJtbd.length + ')</h3>' +
         (productJtbd.length ? '<div class="record-grid">' + productJtbd.map(record => '<article class="record"><h4>' + esc(record.statement) + '</h4><div class="record-meta"><span class="tag">' + esc(record.id) + '</span><span class="tag">product JTBD</span></div>' + (record.evidence || []).map(evidence => '<div class="evidence"><strong>' + esc(evidence.source) + '</strong><blockquote>' + esc(evidence.quote) + '</blockquote></div>').join('') + '</article>').join('') + '</div>' : '<p class="muted">Product JTBD were not extracted in this run.</p>') +
         '<h3>Records (' + records.length + ')</h3><div class="record-grid">' + recordsHtml + '</div>' +
         (validationErrors.length ? '<div class="callout error"><h3>Run validation errors</h3><ul>' + validationErrors.map(error => '<li>' + esc(error) + '</li>').join('') + '</ul></div>' : '') +
        (conflicts ? '<div class="callout error"><h3>Conflicts</h3><ul>' + conflicts + '</ul></div>' : '') +
        (gaps ? '<div class="callout warn"><h3>Gaps</h3><ul>' + gaps + '</ul></div>' : '') +
        (warnings ? '<div class="callout warn"><h3>Warnings</h3><ul>' + warnings + '</ul></div>' : '');
     }}
     function bindGlobalTabs() {{
       document.querySelectorAll('[data-global-tab]').forEach(button => button.onclick = () => {{
         const global = button.dataset.globalTab === 'global';
         document.querySelectorAll('[data-global-tab]').forEach(tab => tab.classList.toggle('active', tab === button));
         document.getElementById('model-content').classList.toggle('hidden', global);
         document.getElementById('global-panel').classList.toggle('hidden', !global);
       }});
     }}
      function audienceRendered(audience, validation, calculation) {{
        if (!audience) return '<p class="muted">No audience result has been produced yet.</p>';
        const segments = (audience.segments || []).map(segment => {{
          const size = segment.size || {{}};
          const checks = (segment.checks || []).map(check => '<li><span class="tag">' + esc(check.status) + '</span> ' + esc(check.name) + ': ' + esc(check.detail) + '</li>').join('');
          const characteristics = (segment.characteristics || []).map(item => '<li><span class="tag">' + esc(item.kind) + '</span> ' + esc(item.text) + '</li>').join('');
          const interpretation = segment.interpretation || {{}};
          const segmentJtbd = Array.isArray(interpretation.jtbd) ? interpretation.jtbd : (interpretation.jtbd ? [interpretation.jtbd] : []);
          const jtbdCard = (ref, kind, text, rationale) =>
            '<article class="record"><h4>' + esc(text || 'Not established') + '</h4>' +
            '<div class="record-meta"><span class="tag">' + esc(ref || 'no ref') + '</span><span class="tag">' + esc(kind) + '</span></div>' +
            (rationale ? '<p class="muted"><strong>Rationale:</strong> ' + esc(rationale) + '</p>' : '') + '</article>';
          const productJtbd = (interpretation.jtbd_alignment || []).filter((item, index, items) =>
            item.product_jtbd_ref && items.findIndex(candidate => candidate.product_jtbd_ref === item.product_jtbd_ref) === index
          );
          const productJtbdHtml = productJtbd.map(item => jtbdCard(item.product_jtbd_ref, 'product JTBD', item.product_jtbd_text, '')).join('');
          const segmentJtbdHtml = segmentJtbd.map((item, index) => jtbdCard(
            item.claim_ref || ('seg_jtbd_' + String(index + 1).padStart(3, '0')),
            item.kind,
            item.text,
            item.rationale
          )).join('');
          const jtbdAlignment = (interpretation.jtbd_alignment || []).map(item => '<li><span class="tag">' + esc(item.relation) + '</span>' +
            '<p><strong>Product JTBD — pipeline 1 (' + esc(item.product_jtbd_ref || 'no ref') + ')</strong><br>' + esc(item.product_jtbd_text || 'Not available') + '</p>' +
            '<p><strong>Segment JTBD — pipeline 2 (' + esc(item.segment_jtbd_ref || 'no ref') + ')</strong><br>' + esc(item.segment_jtbd_text || 'Not established') + '</p>' +
            '<p><strong>Comparison rationale:</strong> ' + esc(item.rationale || 'Not provided') + '</p></li>').join('');
          return '<article class="record"><h4>' + esc(segment.name) + '</h4>' +
            '<div class="record-meta"><span class="tag">' + esc(segment.segment_id) + '</span><span class="tag">' + esc(segment.origin) + '</span></div>' +
            '<p>' + esc((segment.definition && segment.definition.inclusion_rules || []).join('; ') || 'No inclusion rule') + '</p>' +
            '<p><strong>Size:</strong> ' + esc(size.count == null ? 'unknown' : size.count) + (size.share == null ? '' : ' (' + esc(size.share * 100) + '%)') + '</p>' +
            aggregateProfileRendered(calculation) +
            (characteristics ? '<p><strong>Characteristics</strong></p><ul>' + characteristics + '</ul>' : '') +
            (interpretation.context ? '<p><strong>Context:</strong> <span class="tag">' + esc(interpretation.context.kind) + '</span> ' + esc(interpretation.context.text) + '</p>' : '') +
            '<p><strong>Product JTBD</strong></p>' + (productJtbdHtml ? '<div class="record-grid">' + productJtbdHtml + '</div>' : '<p class="muted">Product JTBD were not extracted in this run.</p>') +
            '<p><strong>Segment JTBD</strong></p>' + (segmentJtbdHtml ? '<div class="record-grid">' + segmentJtbdHtml + '</div>' : '<p class="muted">Segment JTBD were not established.</p>') +
            '<p><strong>Product JTBD ↔ segment JTBD comparison</strong></p>' + (jtbdAlignment ? '<ul>' + jtbdAlignment + '</ul>' : '<p class="muted">No product JTBD comparison was produced.</p>') +
            '<p><strong>Checks</strong></p><ul class="check-list">' + checks + '</ul>' +
            '<p><strong>Risks</strong></p><ul>' + (segment.risks || []).map(risk => '<li>' + esc(risk) + '</li>').join('') + '</ul></article>';
        }}).join('');
        const candidate = !segments && calculationCandidateRendered(calculation);
        const segmentHeading = segments
          ? (audience.status === 'complete' ? 'Segments (' : 'Potential segments (not approved) (') + ((audience.segments || []).length) + ')'
          : 'Approved segments (0)';
        const profile = audience.data_profile || {{}};
        const errors = validation && validation.errors ? validation.errors : [];
        return '<div class="hero"><strong>' + esc(audience.product_id || 'Audience result') + '</strong><span class="tag">' + esc(audience.status) + '</span><span class="tag">' + esc(audience.mode) + '</span>' + (validation ? '<span class="tag">validation: ' + validationLabel(validation) + '</span>' : '') + '</div>' +
          '<div class="callout"><strong>Data profile</strong><p>' + esc(profile.unit || 'unknown unit') + ' · cutoff: ' + esc(profile.cutoff_at || 'unknown') + '</p><p>' + esc(profile.coverage && profile.coverage.text || 'Coverage unknown') + '</p></div>' +
          '<h3>' + segmentHeading + '</h3><div class="record-grid">' + segments + '</div>' +
          (candidate ? '<h3>Identified candidates (not approved)</h3><div class="record-grid">' + candidate + '</div>' : '') +
          (errors.length ? '<div class="callout error"><h3>Validation errors</h3><ul>' + errors.map(error => '<li>' + esc(error) + '</li>').join('') + '</ul></div>' : '') +
          ((audience.missing_data || []).length ? '<div class="callout warn"><h3>Missing data</h3><ul>' + audience.missing_data.map(item => '<li>' + esc(item) + '</li>').join('') + '</ul></div>' : '');
      }}
      function calculationCandidateRendered(calculation) {{
        if (!calculation || calculation.count == null || calculation.count <= 0) return '';
        const rules = (calculation.feature_summaries || []).map(feature =>
          '<li><strong>' + esc(feature.name) + '</strong>: ' + esc((feature.filters || []).map(filter => filter.field + ' = ' + filter.value).join(' AND ')) +
          (feature.monthly_client_counts ? '<br><span class="muted">Monthly matched clients: ' + esc(feature.monthly_client_counts.map(item => item.month + ' = ' + item.client_count).join(', ')) + '</span>' : '') + '</li>'
        ).join('');
        const query = calculation.query || {{}};
        return '<article class="record"><h4>Potential segment: ' + esc(query.name || 'unnamed') + '</h4>' +
          '<div class="record-meta"><span class="tag">potential segment</span><span class="tag">not approved</span></div>' +
          '<p><strong>Size:</strong> ' + esc(calculation.count) + ' / ' + esc(calculation.base_count) + (calculation.share == null ? '' : ' (' + esc(calculation.share * 100) + '%)') + '</p>' +
          '<p><strong>Window:</strong> ' + esc(query.window_start || 'unknown') + ' → ' + esc(query.window_end || 'unknown') + '</p>' +
          '<p><strong>Rules</strong></p><ul>' + rules + '</ul>' +
          '<p class="muted">This candidate was identified by the private aggregate calculation. It is shown for inspection but is not an approved activation segment because the final gate did not pass.</p>' +
          aggregateProfileRendered(calculation) +
          '</article>';
      }}
     function aggregateProfileRendered(calculation) {{
        const aggregate = calculation && calculation.aggregate_summary;
        if (!aggregate) return '';
        const distributionRows = (aggregate.value_distributions || []).flatMap(distribution => (distribution.values || []).map(item => ({{
          field: distribution.field,
          value: item.value,
          clients: item.unique_client_count,
          events: item.event_count,
          eventShare: Number(item.event_share) || 0
        }}))).sort((left, right) => right.eventShare - left.eventShare || String(left.field).localeCompare(String(right.field)) || String(left.value).localeCompare(String(right.value))).map(item =>
          '<tr><td>' + esc(item.field) + '</td><td>' + esc(item.value) + '</td><td>' + esc(item.clients) + '</td><td>' + esc(item.events) + '</td><td>' + esc((item.eventShare * 100).toFixed(1)) + '%</td></tr>'
        ).join('');
        const monthly = (aggregate.monthly_event_counts || []).map(item => esc(item.month) + ': ' + esc(item.event_count)).join(', ');
        return '<div class="callout"><h4>Common and average activity</h4>' +
          '<p><strong>Events in this potential segment:</strong> ' + esc(aggregate.event_count) + '</p>' +
          '<p><strong>Average events per client:</strong> ' + esc(Number(aggregate.average_events_per_client).toFixed(1)) + '</p>' +
          '<p><strong>Average events per client per month:</strong> ' + esc(Number(aggregate.average_events_per_client_per_month).toFixed(1)) + '</p>' +
          '<p><strong>Monthly event volume:</strong> ' + monthly + '</p>' +
          (distributionRows ? '<p><strong>Most common categorical values among segment events</strong></p><div class="table-wrap"><table class="data-table"><thead><tr><th>Field</th><th>Value</th><th>Clients</th><th>Events</th><th>Event share</th></tr></thead><tbody>' + distributionRows + '</tbody></table></div>' : '') +
          (aggregate.excluded_event_types && aggregate.excluded_event_types.length ? '<p class="muted">Campaign event types excluded from activity characteristics: ' + esc(aggregate.excluded_event_types.join(', ')) + '</p>' : '') +
          '<p class="muted">These are aggregate characteristics only. They describe events associated with the qualifying clients, never individual people or membership.</p></div>';
      }}
      function firstCompletedRun(item) {{
        const runs = (item.runs || []).filter(run => run.status === 'completed').sort((left, right) => (left.run_number || 1) - (right.run_number || 1));
        if (runs.length) return runs[0];
        if (item.status === 'completed' && item.result) return {{ ...item, run_number: item.run_number || 1 }};
        return null;
      }}
      function globalModelSummary(item) {{
        const run = firstCompletedRun(item);
        if (!run) return {{ model: item.model, run: null, result: null }};
        const result = run.result || {{}};
        const calculation = result.calculation || {{}};
        const audience = result.audience || {{}};
        const segment = (audience.segments || [])[0] || {{}};
        const query = calculation.query || result.query || {{}};
        const rules = (segment.definition && segment.definition.inclusion_rules) || (calculation.feature_summaries || []).map(feature =>
          (feature.filters || []).map(filter => filter.field + ' = ' + filter.value).join(' AND '));
        return {{
          model: item.model,
          run: run.run_number || 1,
          result,
          calculation,
          audience,
          query,
          rules,
          signature: JSON.stringify({{ query: {{ ...query, name: undefined }}, rules, count: calculation.count, base_count: calculation.base_count }})
        }};
      }}
      function globalComparisonRendered() {{
        const summaries = models.map(globalModelSummary);
        const available = summaries.filter(item => item.result);
        if (!available.length) return '<p class="muted">No completed model results are available for comparison.</p>';
        const globalEvaluation = report.global_evaluation && report.global_evaluation.status === 'completed' ? report.global_evaluation.evaluation : null;
        const signatures = new Set(available.map(item => item.signature));
        const windows = new Set(available.map(item => (item.query.window_start || 'unknown') + '..' + (item.query.window_end || 'unknown')));
        const counts = new Set(available.map(item => String(item.calculation.count == null ? 'unknown' : item.calculation.count)));
        const rows = summaries.map(item => {{
          if (!item.result) return '<tr><td>' + esc(item.model) + '</td><td>—</td><td><span class="tag">no completed run</span></td><td>—</td><td>—</td><td>—</td></tr>';
          const share = item.calculation.share == null ? '—' : (item.calculation.share * 100).toFixed(1) + '%';
          return '<tr><td>' + esc(item.model) + '</td><td>' + esc(item.run) + '</td><td><span class="tag">' + esc(item.audience.status || 'unknown') + '</span></td><td>' + esc((item.calculation.count == null ? '—' : item.calculation.count) + ' / ' + (item.calculation.base_count == null ? '—' : item.calculation.base_count)) + '</td><td>' + esc(share) + '</td><td>' + esc((item.query.window_start || 'unknown') + ' → ' + (item.query.window_end || 'unknown')) + '</td></tr>';
        }}).join('');
        const differences = [];
        if (signatures.size > 1) differences.push('Models produced different rules, windows, or counts.');
        if (windows.size > 1) differences.push('Observation windows differ between models.');
        if (counts.size > 1) differences.push('Calculated segment sizes differ between models.');
        available.forEach(item => {{
          if (!item.audience.segments || !item.audience.segments.length) differences.push(item.model + ' has no retained segment.');
        }});
        const ruleCards = available.map(item => '<article class="record"><h4>' + esc(item.model) + '</h4><p><strong>Rules</strong></p><ul>' + item.rules.map(rule => '<li>' + esc(rule) + '</li>').join('') + '</ul><p><strong>Candidate facts:</strong> ' + esc((item.calculation.count == null ? 'unknown' : item.calculation.count) + ' / ' + (item.calculation.base_count == null ? 'unknown' : item.calculation.base_count) + ', ' + (item.query.window_start || 'unknown') + ' → ' + (item.query.window_end || 'unknown')) + '</p></article>').join('');
        const evaluatorPanel = globalEvaluation ? '<div class="callout"><h3>Cross-model evaluator</h3><p>' + esc(globalEvaluation.summary) + '</p>' +
          '<p><strong>Consensus</strong></p><ul>' + (globalEvaluation.consensus || []).map(item => '<li>' + esc(item) + '</li>').join('') + '</ul>' +
          '<p><strong>Model-specific differences</strong></p><ul>' + (globalEvaluation.model_specific_differences || []).map(item => '<li>' + esc(item) + '</li>').join('') + '</ul>' +
          '<p><strong>Inconsistencies</strong></p><ul>' + (globalEvaluation.inconsistencies || []).map(item => '<li>' + esc(item) + '</li>').join('') + '</ul>' +
          '<p><strong>Recommended follow-ups</strong></p><ul>' + (globalEvaluation.recommended_follow_ups || []).map(item => '<li>' + esc(item) + '</li>').join('') + '</ul></div>' : '<div class="callout"><p class="muted">Cross-model evaluator has not been run yet. Run global_audience_eval.py to add it.</p></div>';
        return '<div class="hero"><strong>Global model comparison</strong><span class="tag">' + esc(available.length) + ' completed model outputs</span></div>' +
          '<div class="cards">' + card('Candidate definitions', signatures.size === 1 ? '一致' : signatures.size + ' variants') + card('Windows', windows.size) + card('Sizes', counts.size) + card('First completed runs', available.length) + '</div>' +
          evaluatorPanel +
          '<h3>Model comparison</h3><div class="table-wrap"><table class="data-table"><thead><tr><th>Model</th><th>Run</th><th>Audience status</th><th>Size</th><th>Share</th><th>Window</th></tr></thead><tbody>' + rows + '</tbody></table></div>' +
          '<div class="callout ' + (differences.length ? 'warn' : '') + '"><h3>' + (differences.length ? 'Differences and anomalies' : 'Consensus') + '</h3><ul>' + (differences.length ? differences.map(item => '<li>' + esc(item) + '</li>').join('') : '<li>Completed models agree on the normalized candidate facts.</li>') + '</ul></div>' +
          '<h3>Candidate definitions</h3><div class="record-grid">' + ruleCards + '</div>';
      }}
      function renderGlobal() {{
        document.getElementById('global-panel').innerHTML = globalComparisonRendered();
      }}
      function audiencePipelineRendered(dataProfile, semanticCards, query, calculation, audience, validation, review) {{
        const retrieved = (semanticCards || []).map(card => '<li><strong>' + esc(card.id) + '</strong>: ' + esc(card.description) + '</li>').join('');
        const features = (calculation && calculation.feature_summaries || []).map(feature =>
          '<li><strong>' + esc(feature.name) + '</strong><br><span class="muted">Columns: ' + esc((feature.source_columns || []).join(', ')) + '</span><br>' +
          esc((feature.filters || []).map(filter => filter.field + ' = ' + filter.value).join(' AND ')) + '</li>').join('');
         return '<div class="callout"><h3>Stages</h3><ul class="check-list"><li>Safe data profile: ' + (dataProfile ? 'complete' : 'missing') + '</li><li>Local semantic feature retrieval: ' + (semanticCards ? 'complete' : 'missing') + '</li><li>Constrained data query: ' + (query ? 'complete' : 'missing') + '</li><li>Local aggregate calculation: ' + (calculation ? 'complete' : 'missing') + '</li><li>Final interpretation: ' + (audience ? 'complete' : 'missing') + '</li><li>AI critic, validator and privacy review: ' + (review ? 'complete' : 'missing') + '</li><li>Contract validation and safety gate: ' + validationLabel(validation) + '</li></ul></div>' +
          (retrieved ? '<div class="callout"><h3>Retrieved feature cards</h3><ul>' + retrieved + '</ul></div>' : '') +
          (calculation ? '<div class="callout"><h3>How this segment was calculated</h3><p><strong>Dataset:</strong> ' + esc(calculation.dataset_ref) + ' · <strong>Rule:</strong> ' + esc(calculation.rule_ref) + '</p><p><strong>Membership:</strong> stored privately; never included in this report.</p><ul>' + features + '</ul></div>' : '') +
           audienceReviewRendered(review) + audienceRendered(audience, validation, calculation);
      }}
     function audienceReviewRendered(review) {{
        if (!review) return '<p class="muted">Critic, validator and privacy findings have not been produced yet.</p>';
       const checks = items => items.map(check => '<li><span class="tag">' + esc(check.status) + '</span> <strong>' + esc(check.name) + '</strong>: ' + esc(check.detail) + '</li>').join('');
       const pm = review.product_manager_confirmation || {{}};
       return '<div class="callout"><h3>Critic</h3><ul class="check-list">' + checks(review.critic_checks || []) + '</ul></div>' +
         '<div class="callout"><h3>Validators</h3><ul class="check-list">' + checks(review.validator_checks || []) + '</ul></div>' +
         '<div class="callout warn"><h3>Product-manager confirmation</h3><p><span class="tag">' + esc(pm.status || 'pending') + '</span> ' + esc(pm.detail || '') + '</p></div>';
     }}
    function evaluationRendered(evaluation) {{
      if (!evaluation) return '<p class="muted">Evaluation has not been produced yet.</p>';
      const score = evaluation.overall_score == null ? 'Not scored' : evaluation.overall_score + ' / 100';
      const criteria = (evaluation.criteria || []).map(criterion => '<article class="criterion"><div class="criterion-top"><h4>' +
        esc(criterion.name) + ' <span class="muted">(' + esc(criterion.id) + ')</span></h4><span class="score">' +
        (criterion.score == null ? '—' : esc(criterion.score) + ' / 5') + '</span></div>' +
        '<p class="muted">Weight: ' + esc(criterion.weight) + ' · ' + esc(criterion.status) + '</p>' +
        '<p><strong>Findings</strong></p><ul>' + (criterion.findings || []).map(item => '<li>' + esc(item) + '</li>').join('') + '</ul>' +
        '<p><strong>Recommendations</strong></p><ul>' + (criterion.recommendations || []).map(item => '<li>' + esc(item) + '</li>').join('') + '</ul></article>').join('');
      const checks = (evaluation.mandatory_checks || []).map(check => '<li><span class="tag">' + esc(check.status) + '</span> ' + esc(check.name) + ': ' + esc(check.finding) + '</li>').join('');
      const scenarios = (evaluation.scenarios || []).map(scenario => '<li><span class="tag">' + esc(scenario.status) + '</span> Scenario ' + esc(scenario.number) + ': ' + esc(scenario.finding) + '</li>').join('');
      const limitations = (evaluation.limitations || []).map(item => '<li>' + esc(item) + '</li>').join('');
      return '<div class="hero"><strong>' + esc(score) + '</strong><span class="tag">' + esc(evaluation.evaluation_status) +
        '</span><span class="tag">acceptance: ' + esc(evaluation.acceptance) + '</span></div>' +
        '<div class="callout"><h3>Summary</h3><p>' + esc(evaluation.summary) + '</p><p class="muted">' + esc(evaluation.score_basis) + '</p></div>' +
        '<h3>Criteria</h3><div class="criterion-list">' + criteria + '</div>' +
        '<div class="callout"><h3>Mandatory checks</h3><ul class="check-list">' + checks + '</ul></div>' +
        '<div class="callout"><h3>Scenarios</h3><ul class="check-list">' + scenarios + '</ul></div>' +
         (limitations ? '<div class="callout warn"><h3>Limitations</h3><ul>' + limitations + '</ul></div>' : '');
    }}
    function stabilityView(evaluation) {{
      if (!evaluation) return null;
      const criterion = evaluation.criterion || (evaluation.criteria || []).find(item => item.id === 'c9');
      return {{
        evaluation_status: evaluation.evaluation_status,
        mode: 'benchmark',
        overall_score: evaluation.overall_score,
        acceptance: evaluation.acceptance,
        score_basis: evaluation.score_basis,
        criterion,
        run_comparison: evaluation.run_comparison || [],
        summary: evaluation.summary,
        limitations: evaluation.limitations || []
      }};
    }}
    function stabilityEvaluationRendered(evaluation) {{
      const view = stabilityView(evaluation);
      if (!view) return '<p class="muted">Stability evaluation has not been produced yet.</p>';
      const score = view.overall_score == null ? 'Not scored' : view.overall_score + ' / 100';
      const criterion = view.criterion;
      const criterionHtml = criterion ? '<article class="criterion"><div class="criterion-top"><h4>' +
        esc(criterion.name) + ' <span class="muted">(' + esc(criterion.id) + ')</span></h4><span class="score">' +
        (criterion.score == null ? '—' : esc(criterion.score) + ' / 5') + '</span></div>' +
        '<p class="muted">Weight: ' + esc(criterion.weight) + ' · ' + esc(criterion.status) + '</p>' +
        '<p><strong>Findings</strong></p><ul>' + (criterion.findings || []).map(item => '<li>' + esc(item) + '</li>').join('') + '</ul>' +
        '<p><strong>Recommendations</strong></p><ul>' + (criterion.recommendations || []).map(item => '<li>' + esc(item) + '</li>').join('') + '</ul></article>' : '';
      const comparison = (view.run_comparison || []).map(item => '<li>' + esc(item) + '</li>').join('');
      return '<div class="hero"><strong>' + esc(score) + '</strong><span class="tag">' + esc(view.evaluation_status) +
        '</span><span class="tag">acceptance: ' + esc(view.acceptance) + '</span></div>' +
        '<div class="callout"><h3>Summary</h3><p>' + esc(view.summary) + '</p><p class="muted">' + esc(view.score_basis) + '</p></div>' +
        (criterionHtml ? '<h3>C9 · Stability</h3><div class="criterion-list">' + criterionHtml + '</div>' : '') +
        (comparison ? '<div class="callout"><h3>Run comparison</h3><ul>' + comparison + '</ul></div>' : '') +
        (view.limitations.length ? '<div class="callout warn"><h3>Limitations</h3><ul>' + view.limitations.map(item => '<li>' + esc(item) + '</li>').join('') + '</ul></div>' : '');
    }}
    function card(label, value) {{ return '<div class="card"><span class="muted">' + label + '</span><strong>' + value + '</strong></div>'; }}
    function stage(item, state, selectedRun) {{
      if (selectedRun && selectedRun.status) return selectedRun.status;
      if (item.status === 'completed') return 'completed';
      if (state.evaluation) return 'evaluation complete';
      if (state.passport) return 'passport ready';
      if ((state.drafts || []).length >= (state.sources || []).length && (state.drafts || []).length) return 'consolidation';
      return 'source extraction';
    }}
      document.querySelectorAll('[data-tab]').forEach(button => button.onclick = () => {{
       document.querySelectorAll('main section').forEach(panel => panel.classList.add('hidden'));
       document.querySelectorAll('[data-tab]').forEach(tab => tab.classList.toggle('active', tab === button));
       document.getElementById(button.dataset.tab + '-panel').classList.remove('hidden');
     }});
     render();
     bindGlobalTabs();
     document.querySelector('[data-tab="result"]').click();
  </script>
</body>
</html>
"""


def render_fallback(payload: dict[str, Any]) -> str:
    models = payload.get("models", [])
    if not models:
        return '<div id="static-fallback" class="fallback muted">No model results or checkpoints found.</div>'
    selected = next(
        (item for item in models if item.get("status") == "completed" and item.get("result", {}).get("passport")),
        models[0],
    )
    max_runs = payload.get("benchmark", {}).get("max_runs", 3)
    result = selected.get("result", {})
    benchmark_evaluations = selected.get("benchmark_evaluations", [])
    evaluation = benchmark_evaluations[-1].get("evaluation") if benchmark_evaluations else result.get("evaluation")
    runs = selected.get("runs", [])
    model_list = "".join(
        f"<li><strong>{html.escape(str(item.get('model', 'unknown')))}</strong> "
        f'<span class="badge {html.escape(str(item.get("status", "checkpointed")))}">'
        f"{html.escape(str(item.get('status', 'checkpointed')))}</span> "
        f'<span class="muted">completed: {sum(run.get("status") == "completed" for run in item.get("runs", []))} / {max_runs}</span></li>'
        for item in models
    )
    result_json = html.escape(
        json.dumps(
            {"passport": result.get("passport"), "validation": result.get("validation")},
            ensure_ascii=False,
            indent=2,
        )
    )
    evaluation_json = html.escape(json.dumps(evaluation, ensure_ascii=False, indent=2))
    return f"""<section id="static-fallback" class="fallback">
  <h2>{html.escape(str(selected.get("model", "unknown")))}</h2>
  <p class="muted">Static preview · completed runs: {sum(run.get("status") == "completed" for run in runs)} / {max_runs}. Open in a browser for run selectors and rendered views.</p>
  <h3>Models</h3>
  <ul>{model_list}</ul>
  <h3>Passport result (JSON)</h3>
  <pre>{result_json}</pre>
  <h3>Evaluation (JSON)</h3>
  <pre>{evaluation_json}</pre>
</section>"""


def _model_from_thread_id(thread_id: str, path: Path) -> str:
    if ":" in thread_id:
        candidate = thread_id.split(":", 1)[1]
        if candidate:
            if candidate.startswith("gpt-"):
                return candidate
            return candidate.replace("-", "/", 1)
    return path.stem


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Build a static HTML report from eval results and checkpoints.")
    parser.add_argument("--results", type=Path, default=Path("results"))
    parser.add_argument("--checkpoints", type=Path, default=Path("checkpoints"))
    parser.add_argument("--pipeline", choices=("auto", "product", "audience"), default="auto")
    parser.add_argument("--output", type=Path, default=Path("report.html"))
    return parser


def main() -> None:
    args = build_parser().parse_args()
    ReportBuilder(args.results, args.checkpoints, args.pipeline).write(args.output)
    print(f"Wrote {args.output}")


if __name__ == "__main__":
    main()
