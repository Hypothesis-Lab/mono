ALTER TABLE product
    DROP CONSTRAINT product_category_check,
    ADD COLUMN brand TEXT,
    ADD COLUMN geography VARCHAR(255),
    ADD COLUMN experiment_goal TEXT,
    ADD COLUMN target_action TEXT,
    ADD COLUMN facts JSONB,
    ADD COLUMN eligibility_rules JSONB,
    ADD COLUMN allowed_channels TEXT[],
    ADD COLUMN forbidden_claims TEXT[],
    ADD COLUMN known_constraints TEXT[];

ALTER TABLE product
    ADD CONSTRAINT product_category_check CHECK (category IN ('electronics', 'clothing', 'food', 'furniture', 'beauty', 'sports', 'toys', 'books', 'automotive', 'health', 'home', 'office', 'garden', 'pets', 'music', 'banking_card', 'other'));

UPDATE product
SET
    description = COALESCE(description, ''),
    brand = name,
    geography = '',
    experiment_goal = '',
    target_action = '',
    facts = '[]'::JSONB,
    eligibility_rules = '[]'::JSONB,
    allowed_channels = '{}',
    forbidden_claims = '{}',
    known_constraints = '{}';

ALTER TABLE product
    ALTER COLUMN description SET NOT NULL,
    ALTER COLUMN brand SET NOT NULL,
    ALTER COLUMN geography SET NOT NULL,
    ALTER COLUMN experiment_goal SET NOT NULL,
    ALTER COLUMN target_action SET NOT NULL,
    ALTER COLUMN facts SET NOT NULL,
    ALTER COLUMN eligibility_rules SET NOT NULL,
    ALTER COLUMN allowed_channels SET NOT NULL,
    ALTER COLUMN forbidden_claims SET NOT NULL,
    ALTER COLUMN known_constraints SET NOT NULL;
