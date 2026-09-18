ALTER TABLE product
    ALTER COLUMN price DROP DEFAULT,
    ALTER COLUMN price TYPE TEXT USING price->>'description',
    DROP COLUMN campaign_note,
    DROP COLUMN contactable_persona_ids,
    DROP COLUMN unknowns,
    DROP COLUMN facts_checked_at,
    DROP COLUMN key_fact_ids,
    DROP COLUMN success_criterion,
    DROP COLUMN metric_denominator,
    DROP COLUMN metric_numerator,
    DROP COLUMN metric_direction,
    DROP COLUMN primary_metric,
    DROP COLUMN goal_origin;
