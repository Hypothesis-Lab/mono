from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Any, Literal, cast
from uuid import UUID, uuid4

import psycopg
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb

from app.pipeline.storage import Job


class JobAlreadyQueuedError(Exception):
    pass


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

    def generation_status(self, product_id: str) -> Literal["created", "working", "failed"] | None:
        with self._connection() as connection, connection.cursor() as cursor:
            cursor.execute(
                """
                SELECT
                    jsonb_path_exists(progress, '$.nodes.* ? (@.completed == false && @.retry_count >= 2)'),
                    claim_until > NOW()
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

        return "working" if row[1] else "created"

    def retry(self, product_id: str) -> bool:
        with (
            self._connection() as connection,
            connection.cursor(row_factory=dict_row) as cursor,
        ):
            cursor.execute(
                """
                SELECT id, progress
                FROM generator.job
                WHERE product_id = %s
                  AND NOT finished
                  AND (claim_until IS NULL OR claim_until <= NOW())
                  AND jsonb_path_exists(
                      progress,
                      '$.nodes.* ? (@.completed == false && @.retry_count >= 2)'
                  )
                FOR UPDATE
                """,
                (product_id,),
            )
            row = cursor.fetchone()
            if row is None:
                return False

            progress = cast(dict[str, Any], row["progress"])
            nodes = cast(dict[str, dict[str, Any]], progress["nodes"])
            for node in nodes.values():
                if not node.get("completed") and int(node.get("retry_count", 0)) >= 2:
                    node.clear()
                    node.update({"completed": False, "retry_count": 0})
                    break

            cursor.execute(
                """UPDATE generator.job SET progress = %s, claim_token = NULL, claim_until = NULL
                   WHERE id = %s AND NOT finished""",
                (Jsonb(progress), row["id"]),
            )
            return cursor.rowcount > 0

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

    def claim_next(self, max_attempts: int) -> tuple[Job, UUID] | None:
        token = uuid4()
        with (
            self._connection() as connection,
            connection.cursor(row_factory=dict_row) as cursor,
        ):
            cursor.execute(
                """
                WITH next_job AS (
                    SELECT id FROM generator.job
                    WHERE NOT finished
                      AND (claim_until IS NULL OR claim_until <= NOW())
                      AND NOT jsonb_path_exists(
                          progress,
                          '$.nodes.* ? (@.completed == false && @.retry_count >= $limit)',
                          jsonb_build_object('limit', %s)
                      )
                    ORDER BY created_at
                    LIMIT 1
                    FOR UPDATE SKIP LOCKED
                )
                UPDATE generator.job AS job
                SET claim_token = %s, claim_until = NOW() + INTERVAL '5 minutes'
                FROM next_job
                WHERE job.id = next_job.id
                RETURNING job.id, job.product_id, job.input, job.progress, job.result,
                          job.finished, job.created_at, job.finished_at
                """,
                (max_attempts, token),
            )
            row = cursor.fetchone()

        return (self._job(dict(row)), token) if row else None

    def save_progress(self, job_id: UUID, token: UUID, progress: dict[str, Any]) -> None:
        with self._connection() as connection, connection.cursor() as cursor:
            cursor.execute(
                """UPDATE generator.job SET progress = %s, claim_token = NULL, claim_until = NULL
                   WHERE id = %s AND claim_token = %s AND NOT finished""",
                (Jsonb(progress), job_id, token),
            )
            if cursor.rowcount != 1:
                raise RuntimeError(f"lost claim while saving generator job {job_id}")

    def finish(self, job_id: UUID, token: UUID, result: dict[str, Any]) -> None:
        with self._connection() as connection, connection.cursor() as cursor:
            cursor.execute(
                """
                UPDATE generator.job
                SET result = %s, finished = TRUE, finished_at = NOW(), claim_token = NULL, claim_until = NULL
                WHERE id = %s AND claim_token = %s AND NOT finished
                """,
                (Jsonb(result), job_id, token),
            )
            if cursor.rowcount != 1:
                raise RuntimeError(f"lost claim while finishing generator job {job_id}")

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

    def current_job_id(self) -> str | None:
        with self._connection() as connection, connection.cursor() as cursor:
            cursor.execute(
                """SELECT id FROM generator.job
                   WHERE NOT finished AND claim_until > NOW()
                   ORDER BY created_at LIMIT 1"""
            )
            row = cursor.fetchone()

        return str(row[0]) if row else None

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
