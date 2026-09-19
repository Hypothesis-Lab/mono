ALTER TABLE hypothesis_launch_resource
    DROP CONSTRAINT hypothesis_launch_resource_status_check,
    ADD CONSTRAINT hypothesis_launch_resource_status_check CHECK (status IN ('pending', 'creating', 'created', 'deleted'));
