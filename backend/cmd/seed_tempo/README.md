# Seed the Tempo test pair

From `backend/`, original fixture seeding remains available:

```sh
go run ./cmd/seed_tempo --product cafe
go run ./cmd/seed_tempo --product bank
```

For a paired pilot, first create one complete source hypothesis/product and
generate its audience snapshot once. The generator flow starts at
`POST /api/v1/product/{id}/analysis`; its review/continue endpoints are
`PUT /api/v1/product/{id}/analysis/review` and
`POST /api/v1/product/{id}/analysis/continue`. Once the source product has its
single intended snapshot, clone it without changing the source:

```sh
DATABASE_URL="$HYPOTHESIS_AB_DATABASE_URL" go run ./cmd/seed_tempo --clone-source SOURCE_HYPOTHESIS_ID --ab-pair
```

Pair mode requires an explicit `DATABASE_URL` (point it at the isolated
`hypothesis_ab` database). If the source has no audience snapshot, pair creation stops unless explicitly
asked to add fixed synthetic engineering-pilot audience data:

```sh
DATABASE_URL="$HYPOTHESIS_AB_DATABASE_URL" go run ./cmd/seed_tempo --clone-source SOURCE_HYPOTHESIS_ID --ab-pair --pilot-audience
```

The synthetic option inserts three deterministic personas (мужчина/30,
женщина/30, мужчина/40), with integer ages and stable persona IDs, matching the
launch demographic dictionary inputs. These same three audience payloads are
inserted into both arms, labeled in research/metrics/hypotheses as synthetic;
it is not generated research and must not be treated as evidence. Pair creation
does not call the generator, LLMs, or advertising APIs. It creates new hypothesis
and product IDs, copies source product fields, hypothesis description, file
objects/content and audience payloads, and gives each copy fresh database,
generator-job, and S3 identifiers. Existing source and cafe/bank fixtures are
not overwritten or deleted.

To add the fully automatic **Tempo** draft-ad fixture to both copies, after the
unified-preparation migration has been applied:

```sh
DATABASE_URL="$HYPOTHESIS_AB_DATABASE_URL" go run ./cmd/seed_tempo \
  --clone-source SOURCE_HYPOTHESIS_ID --ab-pair --pilot-audience \
  --draft-landing-url https://example.org/tempo-pilot
```

This adds the same `unified_draft` preparation snapshot to both products: three
audiences with one keyword and three ads each. The nine distinct 600×600 PNG
calendar illustrations are deterministic synthetic fixtures, not generated
marketing assets; titles/texts are labeled test variants. The reserved
`example.org` URL is only a placeholder for draft preparation, not a real Tempo
landing page. Supply a real approved landing URL for a real advertising project.
The command never calls Yandex and does not start moderation or serving.

The draft search-campaign plan preserves gender/age exclusions as well as keyword
conditions. Exclusions are not proven equivalent to CPM positive audience lists,
particularly for users whose demographics are unknown. Identical image uploads
may be deduplicated by Yandex across A/B; record reuse separately from creation.

The command uses local Compose database/S3 defaults unless configured. Set `DATABASE_URL`, `S3_ENDPOINT`,
`S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, or `S3_SECRET_ACCESS_KEY` to target
another environment. The `--product` flag accepts `cafe` (the default) or
`bank`. It inserts the matching hypothesis and product for the existing `a@a`
account and attaches every Markdown document from the matching
`n8n/product/<product>/` directory except `README.md`. Each run creates a new
pair with the selected product content and document set.
