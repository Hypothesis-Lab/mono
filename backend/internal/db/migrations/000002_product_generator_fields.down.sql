ALTER TABLE product
    DROP CONSTRAINT product_category_check,
    ALTER COLUMN description DROP NOT NULL,
    DROP COLUMN known_constraints,
    DROP COLUMN forbidden_claims,
    DROP COLUMN allowed_channels,
    DROP COLUMN eligibility_rules,
    DROP COLUMN facts,
    DROP COLUMN target_action,
    DROP COLUMN experiment_goal,
    DROP COLUMN geography,
    DROP COLUMN brand;

ALTER TABLE product
    ADD CONSTRAINT product_category_check CHECK (category IN ('electronics', 'clothing', 'food', 'furniture', 'beauty', 'sports', 'toys', 'books', 'automotive', 'health', 'home', 'office', 'garden', 'pets', 'music', 'other'));
