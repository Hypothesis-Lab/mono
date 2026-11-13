ALTER TABLE product_analysis_review
    ADD COLUMN ratings JSONB NOT NULL DEFAULT '{}'::jsonb;
