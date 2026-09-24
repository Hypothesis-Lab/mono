from pathlib import Path

import psycopg
from alembic import command
from alembic.config import Config
from langgraph.checkpoint.postgres import PostgresSaver

from app.db import connect


def apply(database_url: str) -> None:
    # A second service instance can start while the first is upgrading the schema.
    with psycopg.connect(database_url) as connection, connection.cursor() as cursor:
        cursor.execute("SELECT pg_advisory_lock(768249013)")
        config = Config(str(_project_root() / "alembic.ini"))
        config.set_main_option("sqlalchemy.url", _sqlalchemy_url(database_url))
        command.upgrade(config, "head")
        with connect(database_url, autocommit=True) as saver_connection:
            PostgresSaver(saver_connection).setup()


def _project_root() -> Path:
    return Path(__file__).resolve().parents[1]


def _sqlalchemy_url(database_url: str) -> str:
    if database_url.startswith("postgres://"):
        return "postgresql+psycopg://" + database_url.removeprefix("postgres://")

    if database_url.startswith("postgresql://"):
        return "postgresql+psycopg://" + database_url.removeprefix("postgresql://")

    return database_url
