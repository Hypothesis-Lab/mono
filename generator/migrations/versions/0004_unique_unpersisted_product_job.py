from alembic import op

revision = "0004"
down_revision = "0003"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.execute("DROP INDEX generator.generator_job_active_product_id_key")
    op.execute(
        """
        CREATE UNIQUE INDEX generator_job_unpersisted_product_id_key
        ON generator.job (product_id)
        WHERE NOT persisted
        """
    )


def downgrade() -> None:
    op.execute("DROP INDEX generator.generator_job_unpersisted_product_id_key")
    op.execute(
        """
        CREATE UNIQUE INDEX generator_job_active_product_id_key
        ON generator.job (product_id)
        WHERE NOT finished
        """
    )
