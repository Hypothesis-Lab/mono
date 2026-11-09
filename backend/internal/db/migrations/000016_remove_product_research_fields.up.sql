ALTER TABLE product
    DROP COLUMN facts,
    DROP COLUMN key_fact_ids,
    DROP COLUMN facts_checked_at,
    DROP COLUMN eligibility_rules,
    DROP COLUMN contactable_persona_ids,
    DROP COLUMN campaign_note;

ALTER TABLE product
    ALTER COLUMN price DROP DEFAULT;

UPDATE product
SET price = price - 'source_fact_ids';

ALTER TABLE product
    ALTER COLUMN price SET DEFAULT '{"status":"unknown","description":"Цена не указана."}'::jsonb;
