from alembic import op

revision = "0001"
down_revision = None
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.execute("CREATE SCHEMA IF NOT EXISTS generator")
    op.execute(
        """
        CREATE TABLE IF NOT EXISTS generator.job (
            id UUID PRIMARY KEY,
            product_id TEXT NOT NULL,
            input JSONB NOT NULL,
            progress JSONB NOT NULL DEFAULT '{"pipeline_version": 1, "nodes": {}}',
            result JSONB,
            finished BOOLEAN NOT NULL DEFAULT FALSE,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            finished_at TIMESTAMPTZ
        )
        """
    )
    op.execute(
        """
        CREATE INDEX IF NOT EXISTS generator_job_unfinished_created_at_idx
        ON generator.job (created_at)
        WHERE NOT finished
        """
    )
    op.execute(
        """
        CREATE INDEX IF NOT EXISTS generator_job_progress_idx
        ON generator.job USING GIN (progress jsonb_path_ops)
        """
    )


def downgrade() -> None:
    op.execute("DROP TABLE IF EXISTS generator.job")
