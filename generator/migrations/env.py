from alembic import context
from sqlalchemy import create_engine, pool, text

config = context.config


def run_migrations_offline() -> None:
    context.configure(
        url=config.get_main_option("sqlalchemy.url"),
        dialect_opts={"paramstyle": "named"},
        literal_binds=True,
        version_table="alembic_version",
        version_table_schema="generator",
    )

    with context.begin_transaction():
        context.run_migrations()


def run_migrations_online() -> None:
    database_url = config.get_main_option("sqlalchemy.url")
    if database_url is None:
        raise RuntimeError("sqlalchemy.url is required")

    engine = create_engine(
        database_url,
        poolclass=pool.NullPool,
    )
    with engine.connect() as connection:
        connection.execute(text("CREATE SCHEMA IF NOT EXISTS generator"))
        connection.commit()
        context.configure(
            connection=connection,
            version_table="alembic_version",
            version_table_schema="generator",
        )

        with context.begin_transaction():
            context.run_migrations()


if context.is_offline_mode():
    run_migrations_offline()
else:
    run_migrations_online()
