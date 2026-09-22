from pathlib import Path

from alembic import command
from alembic.config import Config


def apply(database_url: str) -> None:
    config = Config(str(_project_root() / "alembic.ini"))
    config.set_main_option("sqlalchemy.url", _sqlalchemy_url(database_url))
    command.upgrade(config, "head")


def _project_root() -> Path:
    return Path(__file__).resolve().parents[1]


def _sqlalchemy_url(database_url: str) -> str:
    if database_url.startswith("postgres://"):
        return "postgresql+psycopg://" + database_url.removeprefix("postgres://")

    if database_url.startswith("postgresql://"):
        return "postgresql+psycopg://" + database_url.removeprefix("postgresql://")

    return database_url
