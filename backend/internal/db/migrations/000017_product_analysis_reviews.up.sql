ALTER TABLE product_analysis_run
    DROP CONSTRAINT product_analysis_run_status_check,
    ADD CONSTRAINT product_analysis_run_status_check
        CHECK (status IN ('queued', 'running', 'waiting_for_user_input', 'blocked', 'failed', 'completed'));

DROP INDEX product_analysis_run_active_product_idx;
CREATE UNIQUE INDEX product_analysis_run_active_product_idx
    ON product_analysis_run (product_id) WHERE status IN ('queued', 'running', 'waiting_for_user_input');

CREATE TABLE product_analysis_review (
    run_id TEXT NOT NULL REFERENCES product_analysis_run(id) ON DELETE CASCADE,
    stage TEXT NOT NULL CHECK (stage IN ('audience', 'hypotheses')),
    selected_candidate_ids TEXT[] NOT NULL,
    custom_candidates JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (run_id, stage)
);
