from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Any, cast
from uuid import UUID, uuid4

import psycopg
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb


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


class JobRepository:
    def __init__(self, database_url: str) -> None:
        self.database_url = database_url

    def initialize(self) -> None:
        with self._connection() as connection, connection.cursor() as cursor:
            cursor.execute("CREATE SCHEMA IF NOT EXISTS generator")
            cursor.execute(
                """
                CREATE TABLE IF NOT EXISTS generator.job (
                    id UUID PRIMARY KEY,
                    product_id TEXT NOT NULL,
                    input JSONB NOT NULL,
                    progress JSONB NOT NULL DEFAULT '{"pipeline_version": 1, "nodes": {}}',
                    result JSONB,
                    finished BOOLEAN NOT NULL DEFAULT FALSE,
                    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                    finished_at TIMESTAMPTZ
                )
                """
            )
            cursor.execute(
                """
                CREATE INDEX IF NOT EXISTS generator_job_unfinished_created_at_idx
                ON generator.job (created_at)
                WHERE NOT finished
                """
            )
            cursor.execute(
                """
                CREATE INDEX IF NOT EXISTS generator_job_progress_idx
                ON generator.job USING GIN (progress jsonb_path_ops)
                """
            )

    def create(self, product_id: str, input_data: dict[str, Any]) -> Job:
        job_id = uuid4()
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
