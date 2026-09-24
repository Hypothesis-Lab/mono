from typing import Any, cast

import psycopg
from psycopg.rows import dict_row


def connect(database_url: str, *, autocommit: bool = False) -> psycopg.Connection[Any]:
    return cast(
        psycopg.Connection[Any],
        psycopg.connect(database_url, autocommit=autocommit, row_factory=cast(Any, dict_row)),
    )
