# Tempo extraction eval

This is a local evaluation runner extracted from the expanded n8n workflow in
`../n8n/workflows/tempo-product-extraction.json`.

It runs the same stages for every requested OpenRouter model:

```text
source documents (parallel)
  -> source extraction
  -> consolidation
  -> deterministic validation
  -> conditional structural repair
  -> final passport
  -> source audit evaluation
```

Each model runs concurrently with the others. Every model has its own SQLite
checkpoint file so a failed run can be inspected without SQLite writers
competing for one database. A result is written incrementally
to the output JSON file as each model finishes.

Structured-output failures receive up to three corrective retries. Each retry
includes the original request, the raw model response, and the parse or
validation error. Non-retryable provider errors, such as guardrail rejections,
are recorded immediately.

The default model-call timeout is 15 seconds. Override it with `--timeout` if
an evaluation model needs more time.

## Run

```sh
cd eval
uv sync
export OPENROUTER_API_KEY=...
uv run python run.py \
  --models google/gemini-2.5-flash openai/gpt-4o-mini \
  --input-dir ../n8n/product \
  --output results/tempo.json
```

Or put one model ID per line in a file:

```sh
uv run python run.py --models-file models.txt --output results/tempo.json
```

## Build a report

Generate a static HTML snapshot from completed result JSON files and the latest
checkpoint for every model, including incomplete runs:

```sh
uv run python report.py --output report.html
```

Open `report.html` in a browser. The report has separate views for the
passport result, evaluation, latest checkpoint, and raw model entry.

## Run the C9 benchmark

The benchmark CLI gives each model up to three run slots. It resumes an
unfinished checkpoint, skips occupied slots, and runs the external C9
stability evaluation only after three successful passports exist:

```sh
uv run --env-file .env python benchmark.py \
  --models-file models.txt \
  --output results/benchmark.json \
  --checkpoints checkpoints/benchmark
```

Run 1 uses the normal source order, run 2 reverses it, and run 3 uses a
deterministic shuffle. Use `--force` to reset the selected models' benchmark
slots and checkpoints.

### Current benchmark models

The current evaluation set contains:

```text
deepseek/deepseek-v4.1-flash
inclusionai/ling-3.0-flash
z-ai/glm-5.3-flash
xiaomi/mimo-v2.6-flash
tencent/hy4-preview
openai/gpt-6-luna
z-ai/glm-5.3
xiaomi/mimo-v2.6-pro
minimax/minimax-m3
z-ai/glm-5.2
gpt-4o-mini
```

The benchmark evaluator currently uses `gpt-4o-mini` through the same
OpenRouter key. Override it with `--evaluation-model` when needed.

`meta/muse-spark-1.3-contributor` is intentionally excluded because the
current OpenRouter account guardrails reject its available endpoint. Use
`--exclude-model MODEL_ID` for additional temporary exclusions.

The JSON contains `model`, `result.passport`, `result.validation`, and
`result.evaluation` for successful runs, plus errors, timings, token counts,
and checkpoint paths for every model.
