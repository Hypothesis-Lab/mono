# Audience pipeline input

`context.json` is a pre-generated, synthetic input fixture for the audience
pipeline. The runner consumes it; it does not profile raw clients, calculate
features, or build segment membership itself.

The file deliberately contains only aggregate data and calculation artifacts:

- `config` is trusted application configuration;
- `product_passport` is not duplicated here: it must be handed off from the
  product pipeline with `--product-result`;
- `audience_sources` describes the available snapshot and observation window;
- `calculation_results` contains pre-computed quality, feature, privacy, time,
  and segmentation results;
- `selected_channel` is the activation context.

For a real request, replace the synthetic values with versioned artifacts from
the data/analytics services. Never add raw client records, `client_id`, or
membership lists to the LLM context.

Example using the checked-in fixture:

```sh
cd eval
uv run python run.py \
  --pipeline audience \
  --models-file models.txt \
  --audience-context ../n8n/audience/context.json \
  --output results/audience.json \
  --checkpoints checkpoints/audience
```

To hand off a passport produced by the product pipeline, add
`--product-result results/product.json` and optionally `--product-model` when
the file contains multiple model runs. The audience pipeline refuses to run
without this product handoff, preventing stale or invented product evidence.
