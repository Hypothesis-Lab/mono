from alembic import op

revision = "0002"
down_revision = "0001"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.execute(
        """
        CREATE UNIQUE INDEX generator_job_active_product_id_key
        ON generator.job (product_id)
        WHERE NOT finished
        """
    )


def downgrade() -> None:
    op.execute("DROP INDEX IF EXISTS generator.generator_job_active_product_id_key")
