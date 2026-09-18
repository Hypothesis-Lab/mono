ALTER TABLE ad_campaign
    ADD COLUMN product_id TEXT REFERENCES product(id) ON DELETE CASCADE;

UPDATE ad_campaign AS campaign
SET product_id = product.id
FROM product
WHERE product.hypothesis_id = campaign.hypothesis_id;

ALTER TABLE ad_campaign
    ALTER COLUMN product_id SET NOT NULL,
    DROP CONSTRAINT ad_campaign_hypothesis_id_provider_key,
    DROP COLUMN hypothesis_id,
    ADD CONSTRAINT ad_campaign_product_id_provider_key UNIQUE (product_id, provider);
