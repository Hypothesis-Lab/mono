CREATE TABLE product_feedback_survey_result (
    persona_id   TEXT PRIMARY KEY,
    persona_name TEXT NOT NULL,
    response     JSONB NOT NULL,
    updated_at   TIMESTAMP NOT NULL DEFAULT NOW()
);
