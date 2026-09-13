from alembic import op

revision = "0005"
down_revision = "0004"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.execute(
        """
        UPDATE generator.job
        SET input = (input - 'geography') || jsonb_build_object(
            'country', 'Россия',
            'town', input->'geography'
        )
        WHERE input ? 'geography'
        """
    )


def downgrade() -> None:
    op.execute(
        """
        UPDATE generator.job
        SET input = (input - 'country' - 'town') || jsonb_build_object(
            'geography', COALESCE(input->'town', input->'country')
        )
        WHERE input ? 'country' OR input ? 'town'
        """
    )
