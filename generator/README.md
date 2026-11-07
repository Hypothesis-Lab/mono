# Generator

## Prepare the exploratory audience dataset

From `generator/`, with the generator PostgreSQL URL in `DATABASE_URL`, run:

```sh
uv run python -m app.cmd.prepare_segments
```

This runs both notebooks in `../eval/farida_oct/` on
`../eval/События аудиторий (датасет).xlsx` and saves a new versioned segment set
and its private membership to PostgreSQL. It uses `../eval/.venv/bin/python`,
which can be prepared with `uv sync --group notebooks` from `eval/`.
Use `--python PATH` to select another interpreter and
`--dataset PATH` to select another copy of the same synthetic workbook. Other
datasets are rejected until their provenance and preparation settings can be
configured explicitly.

The notebooks flag this dataset as synthetic and exploratory. Their reports and
feature settings require review before the segments can be treated as validated
or used for activation. This command does not create an advertising audience.
