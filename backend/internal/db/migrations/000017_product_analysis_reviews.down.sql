DROP TABLE product_analysis_review;

DROP INDEX product_analysis_run_active_product_idx;
CREATE UNIQUE INDEX product_analysis_run_active_product_idx
    ON product_analysis_run (product_id) WHERE status IN ('queued', 'running');

ALTER TABLE product_analysis_run
    DROP CONSTRAINT product_analysis_run_status_check,
    ADD CONSTRAINT product_analysis_run_status_check
        CHECK (status IN ('queued', 'running', 'blocked', 'failed', 'completed'));
