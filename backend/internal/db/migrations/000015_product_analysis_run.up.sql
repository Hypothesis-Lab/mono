CREATE TABLE product_analysis_run (
    id TEXT PRIMARY KEY,
    product_id TEXT NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    input_snapshot JSONB NOT NULL,
    segment_set_id UUID NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'blocked', 'failed', 'completed')),
    current_stage TEXT NOT NULL CHECK (current_stage IN ('product', 'audience', 'hypotheses')),
    product_job_id TEXT,
    audience_job_id TEXT,
    hypothesis_job_id TEXT,
    product_result JSONB,
    audience_result JSONB,
    hypothesis_result JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX product_analysis_run_active_product_idx
    ON product_analysis_run (product_id) WHERE status IN ('queued', 'running');
