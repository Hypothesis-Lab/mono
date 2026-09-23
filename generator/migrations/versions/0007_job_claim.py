from alembic import op

revision = "0007"
down_revision = "0006"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.execute("ALTER TABLE generator.job ADD COLUMN claim_token UUID, ADD COLUMN claim_until TIMESTAMPTZ")


def downgrade() -> None:
    op.execute("ALTER TABLE generator.job DROP COLUMN claim_token, DROP COLUMN claim_until")
