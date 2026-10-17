# Tempo pipeline eval

This is a local evaluation runner extracted from the expanded n8n workflow in
`../n8n/workflows/tempo-product-extraction.json`.

The runner supports two independent pipelines. The default `product` pipeline
runs the product passport workflow:

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
an evaluation model needs more time. HTTP 429 responses are retried with short
exponential backoff (and `Retry-After` when provided), capped by a 45-second
wait budget per structured call. Override that cap with `--rate-limit-budget`.

## Audience pipeline

The audience pipeline consumes pre-generated, aggregate input from
`../n8n/audience/context.json`; it does not generate profiles, calculate
features, or build membership itself. Its stages are:

```text
trusted config + product passport + audience data artifacts
  -> data profile
  -> segment plan
  -> segment interpretation
  -> deterministic contract validation
  -> targeted repair only when validation fails
  -> deterministic critic and validator findings
  -> product-manager confirmation pending
```

The product passport may be handed off from a completed product run:

```sh
uv run python run.py \
  --pipeline audience \
  --models-file models.txt \
  --audience-context ../n8n/audience/context.json \
  --product-result results/product.json \
  --output results/audience.json \
  --checkpoints checkpoints/audience
```

`--product-result` is required for the audience pipeline. The checked-in
audience fixture contains only synthetic aggregate calculation results, never
raw client records or membership lists.

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

For an audience report, select the pipeline and its separate artifacts:

```sh
uv run python report.py \
  --pipeline audience \
  --results results/audience.json \
  --checkpoints checkpoints/audience \
  --output audience-report.html
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

The same three-run benchmark can target the audience pipeline:

```sh
uv run --env-file .env python benchmark.py \
  --pipeline audience \
  --models-file models.txt \
  --audience-context ../n8n/audience/context.json \
  --product-result results/benchmark.json \
  --product-model gpt-4o-mini \
  --output results/audience-benchmark.json \
  --checkpoints checkpoints/audience-benchmark
```

For audience runs, the benchmark varies only the order of equivalent audience
sources, calculation artifacts, product records, and allowed-feature lists.
Its C9 result tests prompt/output persistence: observed segment inventory,
rules, artifacts, numbers, periods, refs, statuses, and meaning must not drift.
It does **not** replace the audience result's data-stability check: stability
across comparable periods or samples still needs an analytical artifact and
remains `unknown` until one is provided.

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

Audience JSON entries instead contain `result.audience`, `result.profile`,
`result.plan`, `result.validation`, and `result.review`.

`result.review` is a deterministic review layer: segment relevance, permitted
features/bias evidence, segment count, interpretability, leakage, data quality,
absence of raw-record fields in LLM context, minimum size, stability, channel
availability, and contract validity. It never simulates product-manager
approval; that confirmation remains `pending` until supplied externally.
