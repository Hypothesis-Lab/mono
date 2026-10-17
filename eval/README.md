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

The default model-call timeout is 120 seconds. Override it with `--timeout` if
an evaluation model needs more time. HTTP 429 responses are retried with short
exponential backoff (and `Retry-After` when provided), capped by a 45-second
wait budget per structured call. Override that cap with `--rate-limit-budget`.
The default overall model-run timeout is 600 seconds; override it with
`--run-timeout` when the audience feedback loop needs more or less time.

## Audience pipeline

The audience pipeline matches a product passport against a private XLSX event
dataset. Raw rows and `client_id` stay in the local analytics process. The
model receives only a safe data catalog and aggregate results from a
constrained client-level query.

Field meanings, time semantics, null semantics, and interpretation limits are
supplied through the versioned application prompt
`tempo_eval/prompts/audience-data-contract.md`. The Excel file contains event
data only; it does not carry the data dictionary or a hidden expected segment.

Audience quality policy is kept in
`tempo_eval/audience_config.py`: the default minimum segment size is 100,
the query agent may perform up to ten feedback iterations, and
`ad_platform` is currently configured as an allowed activation channel.
The channel setting means the platform is permitted by pipeline policy; actual
consent and audience matching remain a pre-campaign check. Product-manager
confirmation is recorded as an external post-pipeline action and is never
simulated by the model.

```text
product passport + private XLSX
  -> safe dataset profile
  -> model-selected query
  -> SQLite aggregate result
  -> model decides use_result or refine (up to ten iterations)
  -> deterministic client-level calculation
  -> product interpretation of aggregate result
  -> AI critic + AI validator + AI privacy review
  -> deterministic validation and safety gate
```

The product passport may be handed off from a completed product run:

```sh
uv run python run.py \
  --pipeline audience \
  --models-file models.txt \
  --audience-data "События аудиторий (датасет).xlsx" \
  --product-result results/product.json \
  --output results/audience-data.json \
  --checkpoints checkpoints/audience-data
```

`--product-result` is required for the audience pipeline. The model cannot use
raw rows, `client_id`, demographics, locations, balances, or campaign outcomes.
It proposes only exact filters from the safe catalog; the local calculator
executes them and returns count, share, feature provenance, and a private
membership reference.

Before choosing filters, the pipeline uses a local multilingual ONNX embedding
model to retrieve relevant **safe feature cards** from the dataset catalog. The
cards contain field definitions, observed values, and exact filters only; they
never contain event rows or client identifiers. The first run downloads the
embedding model and builds `semantic-index/`; later runs reuse the local index.
Use `--semantic-index PATH` to keep that cache on a persistent SSD volume.

The XLSX is imported into a private SQLite database on first run. Use
`--audience-db PATH` to choose its location. The database is an application
storage detail and its path is never included in the model prompt.

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
  --results results/audience-data.json \
  --checkpoints checkpoints/audience-data \
  --output audience-report.html
```

Open `report.html` in a browser. The report has separate views for the
passport result, evaluation, latest checkpoint, raw model entry, and (for
multi-model audience results) an algorithmic global comparison tab.

Run the audience graph three times per model and compare run stability:

```sh
uv run python audience_benchmark.py \
  --models minimax/minimax-m3 deepseek/deepseek-v4.1-flash \
  --runs 3 \
  --product-result results/additional-models.json \
  --product-model minimax/minimax-m3 \
  --output results/audience-benchmark.json \
  --checkpoints checkpoints/audience-benchmark
```

The global tab algorithmically selects the first completed run for each model.
An optional model-based cross-model evaluator can be invoked separately:

```sh
uv run python global_audience_eval.py \
  --results results/audience-benchmark.json \
  --evaluator-model minimax/minimax-m3
```

It writes the comparison into the benchmark JSON; regenerate the report to show
the evaluator findings in the global tab.

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

Audience JSON entries contain `result.data_profile`, `result.query`,
`result.calculation`, `result.audience`, `result.validation`, and
`result.review`. The calculation contains no membership list or raw events.

`result.review` is a deterministic review layer: segment relevance, permitted
features/bias evidence, segment count, interpretability, leakage, data quality,
absence of raw-record fields in LLM context, minimum size, stability, channel
availability, and contract validity. It never simulates product-manager
approval; that confirmation remains `pending` until supplied externally.
