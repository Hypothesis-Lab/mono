from alembic import op

revision = "0006"
down_revision = "0005"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.execute(
        """
        UPDATE generator.job
        SET input = input || jsonb_build_object('custom_category', 'Кредит в банке')
        WHERE product_id = '2bb54b6a-6c11-4207-903f-7f27b164096d'
            AND input->>'category' = 'other'
            AND input->>'custom_category' IS NULL
        """
    )


def downgrade() -> None:
    op.execute(
        """
        UPDATE generator.job
        SET input = input - 'custom_category'
        WHERE input ? 'custom_category'
        """
    )
