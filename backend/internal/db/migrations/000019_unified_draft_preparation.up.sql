ALTER TABLE product ADD COLUMN preparation_config JSONB;

ALTER TABLE hypothesis_launch_resource
    DROP CONSTRAINT hypothesis_launch_resource_kind_check,
    ADD COLUMN payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN external_key TEXT,
    ADD CONSTRAINT hypothesis_launch_resource_kind_check
        CHECK (kind IN (
            'campaign', 'retargeting_list', 'ad_group', 'audience_target',
            'campaign_suspend', 'unified_ad_group', 'demographic_exclusion',
            'keyword', 'ad_image', 'responsive_ad'
        ));
