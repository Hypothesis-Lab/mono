ALTER TABLE ad_campaign
    ADD COLUMN hypothesis_id TEXT REFERENCES hypothesis(id) ON DELETE CASCADE;

UPDATE ad_campaign AS campaign
SET hypothesis_id = product.hypothesis_id
FROM product
WHERE product.id = campaign.product_id;

ALTER TABLE ad_campaign
    ALTER COLUMN hypothesis_id SET NOT NULL,
    DROP CONSTRAINT ad_campaign_product_id_provider_key,
    DROP COLUMN product_id,
    ADD CONSTRAINT ad_campaign_hypothesis_id_provider_key UNIQUE (hypothesis_id, provider);
