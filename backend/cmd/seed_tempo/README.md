# Seed the Tempo test pair

From `backend/`, run:

```sh
go run ./cmd/seed_tempo --product cafe
go run ./cmd/seed_tempo --product bank
```

The command uses the local Compose database at `localhost:5433` and local S3
storage at `localhost:8333` by default. Set `DATABASE_URL`, `S3_ENDPOINT`,
`S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, or `S3_SECRET_ACCESS_KEY` to target
another environment. The `--product` flag accepts `cafe` (the default) or
`bank`. It inserts the matching hypothesis and product for the existing `a@a`
account and attaches every Markdown document from the matching
`n8n/product/<product>/` directory except `README.md`. Each run creates a new
pair with the selected product content and document set.
