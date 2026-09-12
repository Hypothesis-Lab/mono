from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Any, cast
from uuid import UUID, uuid4

import psycopg
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb


class JobAlreadyQueuedError(Exception):
    pass


@dataclass(frozen=True)
class Job:
    id: UUID
    product_id: str
    input: dict[str, Any]
    progress: dict[str, Any]
    result: dict[str, Any] | None
    finished: bool
    created_at: datetime
    finished_at: datetime | None


@dataclass(frozen=True)
class CompletedJob:
    id: UUID
    product_id: str
    result: dict[str, Any]


class JobRepository:
    def __init__(self, database_url: str) -> None:
        self.database_url = database_url

    def create(self, product_id: str, input_data: dict[str, Any]) -> Job:
        job_id = uuid4()
        try:
            with (
                self._connection() as connection,
                connection.cursor(row_factory=dict_row) as cursor,
            ):
                cursor.execute(
                    """
                    INSERT INTO generator.job (id, product_id, input)
                    VALUES (%s, %s, %s)
                    RETURNING id, product_id, input, progress, result, finished, created_at, finished_at
                    """,
                    (job_id, product_id, Jsonb(input_data)),
                )
                row = cursor.fetchone()
        except psycopg.errors.UniqueViolation as error:
            raise JobAlreadyQueuedError from error
        if row is None:
            raise RuntimeError("creating generator job returned no row")
        return self._job(dict(row))

    def get(self, job_id: UUID) -> Job | None:
        with (
            self._connection() as connection,
            connection.cursor(row_factory=dict_row) as cursor,
        ):
            cursor.execute(
                """
                SELECT id, product_id, input, progress, result, finished, created_at, finished_at
                FROM generator.job
                WHERE id = %s
                """,
                (job_id,),
            )
            row = cursor.fetchone()
        return self._job(dict(row)) if row else None

    def generation_status(self, product_id: str) -> str | None:
        with self._connection() as connection, connection.cursor() as cursor:
            cursor.execute(
                """
                SELECT jsonb_path_exists(
                    progress,
                    '$.nodes.* ? (@.completed == false && @.retry_count >= 2)'
                )
                FROM generator.job
                WHERE product_id = %s AND NOT finished
                """,
                (product_id,),
            )
            row = cursor.fetchone()
        if row is None:
            return None
        if row[0]:
            return "failed"
        return "created"

    def next_completed(self) -> CompletedJob | None:
        with (
            self._connection() as connection,
            connection.cursor(row_factory=dict_row) as cursor,
        ):
            cursor.execute(
                """
                SELECT id, product_id, result
                FROM generator.job
                WHERE finished AND NOT persisted
                ORDER BY finished_at
                LIMIT 1
                """
            )
            row = cursor.fetchone()
        if row is None:
            return None
        return CompletedJob(
            id=cast(UUID, row["id"]),
            product_id=cast(str, row["product_id"]),
            result=cast(dict[str, Any], row["result"]),
        )

    def mark_persisted(self, job_id: UUID) -> bool:
        with self._connection() as connection, connection.cursor() as cursor:
            cursor.execute(
                """
                UPDATE generator.job
                SET persisted = TRUE
                WHERE id = %s AND finished
                """,
                (job_id,),
            )
            return cursor.rowcount > 0

    def next_eligible(self) -> Job | None:
        with (
            self._connection() as connection,
            connection.cursor(row_factory=dict_row) as cursor,
        ):
            cursor.execute(
                """
                SELECT id, product_id, input, progress, result, finished, created_at, finished_at
                FROM generator.job
                WHERE NOT finished
                  AND NOT jsonb_path_exists(
                      progress,
                      '$.nodes.* ? (@.completed == false && @.retry_count >= 2)'
                  )
                ORDER BY created_at
                LIMIT 1
                """
            )
            row = cursor.fetchone()
        return self._job(dict(row)) if row else None

    def save_progress(self, job_id: UUID, progress: dict[str, Any]) -> None:
        with self._connection() as connection, connection.cursor() as cursor:
            cursor.execute(
                "UPDATE generator.job SET progress = %s WHERE id = %s AND NOT finished",
                (Jsonb(progress), job_id),
            )

    def finish(self, job_id: UUID, result: dict[str, Any]) -> None:
        with self._connection() as connection, connection.cursor() as cursor:
            cursor.execute(
                """
                UPDATE generator.job
                SET result = %s, finished = TRUE, finished_at = NOW()
                WHERE id = %s AND NOT finished
                """,
                (Jsonb(result), job_id),
            )

    def counts(self) -> tuple[int, int]:
        with self._connection() as connection, connection.cursor() as cursor:
            cursor.execute(
                """
                SELECT
                    COUNT(*) FILTER (
                        WHERE NOT jsonb_path_exists(
                            progress,
                            '$.nodes.* ? (@.completed == false && @.retry_count >= 2)'
                        )
                    ),
                    COUNT(*) FILTER (
                        WHERE jsonb_path_exists(
                            progress,
                            '$.nodes.* ? (@.completed == false && @.retry_count >= 2)'
                        )
                    )
                FROM generator.job
                WHERE NOT finished
                """
            )
            row = cursor.fetchone()
        if row is None:
            raise RuntimeError("counting generator jobs returned no row")
        queued, blocked = row
        return int(queued), int(blocked)

    def _connection(self) -> psycopg.Connection[Any]:
        return cast(psycopg.Connection[Any], psycopg.connect(self.database_url))

    @staticmethod
    def _job(row: dict[str, Any]) -> Job:
        return Job(
            id=cast(UUID, row["id"]),
            product_id=cast(str, row["product_id"]),
            input=cast(dict[str, Any], row["input"]),
            progress=cast(dict[str, Any], row["progress"]),
            result=cast(dict[str, Any] | None, row["result"]),
            finished=cast(bool, row["finished"]),
            created_at=cast(datetime, row["created_at"]),
            finished_at=cast(datetime | None, row["finished_at"]),
        )
