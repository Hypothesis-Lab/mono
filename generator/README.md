# Generator

## Prepare the exploratory audience dataset

From `generator/`, with the generator PostgreSQL URL in `DATABASE_URL`, run:

```sh
uv run python -m app.cmd.prepare_segments
```

This runs both notebooks in `../eval/farida_oct/` and saves a new versioned
segment set and its private membership to PostgreSQL. It uses
`../eval/.venv/bin/python`, which can be prepared with `uv sync --group notebooks`
from `eval/`. Use `--python PATH` to select another interpreter. The default
dataset is the synthetic workbook; `--dataset PATH` also accepts the supported
T-Bank investments CSV and configures its `id`, `date`, decimal-comma values,
and real-client provenance. Its documented daily payment, turnover, trade-count,
and social-activity fields are rolled up over 30- and 90-day windows; portfolio
and account balances use the latest observation. Inputs are checked by SHA-256,
so modified copies are not accepted as either supported dataset.

The notebooks mark generated segments as exploratory. Review the reports and
feature settings before treating them as validated or using them for activation.
This command does not create an advertising audience.
