# Local pipeline CLI

Run from `generator/` with `OPENROUTER_API_KEY` set in the repository-root `.env`.
Reports are written under `runs/`; token and provider-cost history is stored in
`runs/pipeline_runs.sqlite3`. Each run gets a unique report filename.

## Product analysis

The product pipeline takes a `ProductInput` with a product name and one or more
source documents. By default, the CLI assembles the Tempo input from the product
form in `app/cmd/run_product_pipeline.py` and the Markdown files in
`../n8n/product/cafe/` (excluding `README.md`): `api-reference.md`,
`brand-book.md`, `features-and-pricing.md`, `glossary.md`, `policies.md`,
`product-brief.md`, `support-cases.md`, and `website-copy.md`. To use a different
explicit input, pass `--input PATH` with JSON in this shape:

```json
{
  "target_product": "Tempo",
  "as_of_date": null,
  "sources": [
    {"source": "product_brief.md", "content": "...source text..."}
  ],
  "unavailable_sources": []
}
```

Run the default Tempo product analysis with:

```sh
uv run python -m app.cmd.run_product_pipeline \
  --pipeline product_analysis \
  --provider openrouter \
  --model deepseek/deepseek-v4.1-flash-1017
```

The report contains the generated passport at `result.passport`; use that
completed report as `--passport` for audience selection and hypothesis
generation.

## Audience selection

The audience pipeline requires a completed product report, prepared segment
candidates, and their matching segmentation report. The current exploratory
inputs are:

- Product passport: `runs/product-analysis-cafe-20261115T163505Z-a1f31869.json`
  (`result.passport`, Tempo, complete, 113 records).
- Segment candidates:
  `inputs/tempo-audience-segment-set-95142c91-eaf1-416c-b32e-2d330c34341a/segment_candidates.json`
- Matching segment metadata:
  `inputs/tempo-audience-segment-set-95142c91-eaf1-416c-b32e-2d330c34341a/segmentation_report.json`
- Go database segment-set ID: `95142c91-eaf1-416c-b32e-2d330c34341a` (2 segments).
- Segment source: `../eval/События аудиторий (датасет).xlsx`, processed by
  `app.cmd.prepare_segments`; provenance is `synthetic_demo`, and the report
  marks the result `exploratory_not_ground_truth`.

Run one audience selection:

```sh
uv run python -m app.cmd.run_product_pipeline \
  --pipeline audience_selection \
  --passport runs/product-analysis-cafe-20261115T163505Z-a1f31869.json \
  --segments inputs/tempo-audience-segment-set-95142c91-eaf1-416c-b32e-2d330c34341a/segment_candidates.json \
  --segment-metadata inputs/tempo-audience-segment-set-95142c91-eaf1-416c-b32e-2d330c34341a/segmentation_report.json \
  --provider openrouter \
  --model deepseek/deepseek-v4.1-flash-1017
```

To compare three independent runs, invoke the command three times in parallel.
The report preserves input paths, segment-set ID, provenance, and limitations.
These are the available prepared inputs for exploratory testing; the synthetic
finance behaviors are not empirical evidence about café customers.

## Hypothesis generation

Hypothesis generation uses the same product passport plus a completed audience
report that contains at least one selected segment. For the runs recorded above,
use one of these audience report files (all selected `seg_02`):

- `runs/audience-selection-tempo-20261115T171329Z-980a2208.json`
- `runs/audience-selection-tempo-20261115T171329Z-c8d86f46.json`
- `runs/audience-selection-tempo-20261115T171329Z-811f35b8.json`

Example using the second report:

```sh
uv run python -m app.cmd.run_product_pipeline \
  --pipeline hypothesis_generation \
  --passport runs/product-analysis-cafe-20261115T163505Z-a1f31869.json \
  --audience-result runs/audience-selection-tempo-20261115T171329Z-c8d86f46.json \
  --provider openrouter \
  --model deepseek/deepseek-v4.1-flash-1017
```

The CLI rejects audience reports without selected segments. These reports have
one selected segment and nested audience status `partial`; the hypothesis graph
accepts them but carries that partial status and synthetic-data limitations
forward. Use the same passport and audience report together to preserve
provenance.
