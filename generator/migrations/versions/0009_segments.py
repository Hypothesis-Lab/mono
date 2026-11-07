from alembic import op

revision = "0009"
down_revision = "0008"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.execute("""
        CREATE TABLE generator.segment_set (
            id UUID PRIMARY KEY,
            source_digest TEXT NOT NULL,
            metadata JSONB NOT NULL,
            created_at TIMESTAMPTZ NOT NULL
        )
    """)
    op.execute("""
        CREATE TABLE generator.segment (
            segment_set_id UUID NOT NULL REFERENCES generator.segment_set(id),
            segment_id TEXT NOT NULL,
            candidate JSONB NOT NULL,
            PRIMARY KEY (segment_set_id, segment_id)
        )
    """)
    op.execute("""
        CREATE TABLE generator.segment_membership (
            segment_set_id UUID NOT NULL,
            segment_id TEXT NOT NULL,
            client_id TEXT NOT NULL,
            PRIMARY KEY (segment_set_id, client_id),
            FOREIGN KEY (segment_set_id, segment_id) REFERENCES generator.segment(segment_set_id, segment_id)
        )
    """)


def downgrade() -> None:
    op.execute("DROP TABLE generator.segment_membership")
    op.execute("DROP TABLE generator.segment")
    op.execute("DROP TABLE generator.segment_set")
