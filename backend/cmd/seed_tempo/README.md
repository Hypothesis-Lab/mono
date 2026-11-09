# Seed the Tempo test pair

From `backend/`, run:

```sh
go run ./cmd/seed_tempo
```

The command uses the local Compose database at `localhost:5433` and local S3
storage at `localhost:8333` by default. Set `DATABASE_URL`, `S3_ENDPOINT`,
`S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, or `S3_SECRET_ACCESS_KEY` to target
another environment. It inserts a fixed Tempo hypothesis and product for the
existing `a@a` account and attaches the café product documents from
`n8n/product/cafe/` (excluding `README.md`). Each run creates a new pair with
the same product content and document set.
