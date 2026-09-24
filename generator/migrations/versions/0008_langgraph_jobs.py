from alembic import op

revision = "0008"
down_revision = "0007"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.execute("ALTER TABLE generator.job ADD COLUMN kind TEXT NOT NULL DEFAULT 'audiences'")
    op.execute("ALTER TABLE generator.job ADD COLUMN status TEXT NOT NULL DEFAULT 'queued'")
    op.execute("ALTER TABLE generator.job ADD COLUMN retry_generation INTEGER NOT NULL DEFAULT 0")
    op.execute("UPDATE generator.job SET status = CASE WHEN finished THEN 'completed' ELSE 'queued' END")
    op.execute("DROP INDEX IF EXISTS generator.generator_job_unpersisted_product_id_key")
    op.execute("DROP INDEX IF EXISTS generator.generator_job_progress_idx")
    op.execute("DROP INDEX IF EXISTS generator.generator_job_unfinished_created_at_idx")
    op.execute("ALTER TABLE generator.job DROP COLUMN progress, DROP COLUMN claim_token, DROP COLUMN claim_until")
    op.execute("ALTER TABLE generator.job DROP COLUMN finished")
    op.execute("CREATE INDEX generator_job_claim_candidates_idx ON generator.job (kind, status, created_at)")
    op.execute(
        "CREATE UNIQUE INDEX generator_job_unpersisted_product_kind_key "
        "ON generator.job (product_id, kind) WHERE NOT persisted"
    )


def downgrade() -> None:
    op.execute("DROP INDEX IF EXISTS generator.generator_job_unpersisted_product_kind_key")
    op.execute("DROP INDEX IF EXISTS generator.generator_job_claim_candidates_idx")
    op.execute("ALTER TABLE generator.job ADD COLUMN finished BOOLEAN NOT NULL DEFAULT FALSE")
    op.execute("UPDATE generator.job SET finished = status = 'completed'")
    op.execute("ALTER TABLE generator.job ADD COLUMN progress JSONB NOT NULL DEFAULT '{\"nodes\": {}}'")
    op.execute("ALTER TABLE generator.job ADD COLUMN claim_token UUID, ADD COLUMN claim_until TIMESTAMPTZ")
    op.execute("ALTER TABLE generator.job DROP COLUMN retry_generation, DROP COLUMN status, DROP COLUMN kind")
    op.execute("CREATE INDEX generator_job_unfinished_created_at_idx ON generator.job (created_at) WHERE NOT finished")
    op.execute("CREATE INDEX generator_job_progress_idx ON generator.job USING GIN (progress jsonb_path_ops)")
    op.execute(
        "CREATE UNIQUE INDEX generator_job_unpersisted_product_id_key ON generator.job (product_id) WHERE NOT persisted"
    )
