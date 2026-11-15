ALTER TABLE hypothesis_launch_resource
    DROP CONSTRAINT hypothesis_launch_resource_kind_check,
    DROP COLUMN external_key,
    DROP COLUMN payload,
    ADD CONSTRAINT hypothesis_launch_resource_kind_check
        CHECK (kind IN ('campaign', 'retargeting_list', 'ad_group', 'audience_target'));

ALTER TABLE product DROP COLUMN preparation_config;
