ALTER TABLE product
    ADD COLUMN goal_origin TEXT NOT NULL DEFAULT 'user_provided' CHECK (goal_origin IN ('demo_assumption', 'user_provided')),
    ADD COLUMN primary_metric TEXT NOT NULL DEFAULT 'target_action_rate',
    ADD COLUMN metric_direction TEXT NOT NULL DEFAULT 'increase' CHECK (metric_direction IN ('increase', 'decrease')),
    ADD COLUMN metric_numerator TEXT NOT NULL DEFAULT 'Unique users completing the target action',
    ADD COLUMN metric_denominator TEXT NOT NULL DEFAULT 'Unique randomized users',
    ADD COLUMN success_criterion TEXT,
    ADD COLUMN key_fact_ids TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN facts_checked_at TEXT,
    ADD COLUMN unknowns TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN contactable_persona_ids TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN campaign_note TEXT NOT NULL DEFAULT 'Channels are not used for research or ranking.';

UPDATE product
SET key_fact_ids = ARRAY(SELECT value->>'fact_id' FROM jsonb_array_elements(facts) AS value)
WHERE key_fact_ids = '{}';

ALTER TABLE product
    ALTER COLUMN price TYPE JSONB
    USING jsonb_build_object(
        'status', 'unknown',
        'description', COALESCE(NULLIF(price, ''), 'Стоимость не указана; не считать продукт бесплатным.'),
        'source_fact_ids', '[]'::jsonb
    );

ALTER TABLE product
    ALTER COLUMN price SET NOT NULL,
    ALTER COLUMN price SET DEFAULT '{"status":"unknown","description":"Стоимость не указана; не считать продукт бесплатным.","source_fact_ids":[]}'::jsonb;
