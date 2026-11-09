ALTER TABLE product
    ADD COLUMN facts JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN key_fact_ids TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN facts_checked_at TEXT,
    ADD COLUMN eligibility_rules JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN contactable_persona_ids TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN campaign_note TEXT NOT NULL DEFAULT 'Channels are not used for research or ranking.';

ALTER TABLE product
    ALTER COLUMN price DROP DEFAULT;

UPDATE product
SET price = price || '{"source_fact_ids":[]}'::jsonb;

ALTER TABLE product
    ALTER COLUMN price SET DEFAULT '{"status":"unknown","description":"Стоимость не указана; не считать продукт бесплатным.","source_fact_ids":[]}'::jsonb;
