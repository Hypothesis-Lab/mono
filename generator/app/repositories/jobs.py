from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Any, Literal, cast
from uuid import UUID, uuid4

import psycopg
from psycopg.types.json import Jsonb

from app.db import connect

JobStatus = Literal["queued", "running", "failed", "completed"]
JobKind = Literal["audiences"]


@dataclass(frozen=True)
class Job:
    id: UUID
    product_id: str
    kind: JobKind
    input: dict[str, Any]
    status: JobStatus
    retry_generation: int
    result: dict[str, Any] | None
    created_at: datetime
    finished_at: datetime | None


@dataclass(frozen=True)
class CompletedJob:
    id: UUID
    product_id: str
    kind: JobKind
    result: dict[str, Any]


class JobAlreadyQueuedError(Exception):
    pass


class JobRepository:
    def __init__(self, database_url: str) -> None:
        self.database_url = database_url

    def create(self, product_id: str, kind: str, input_data: dict[str, Any]) -> Job:
        try:
            with (
                connect(self.database_url) as connection,
                connection.cursor() as cursor,
            ):
                cursor.execute(
                    """
                    INSERT INTO generator.job (id, product_id, kind, input)
                    VALUES (%s, %s, %s, %s)
                    RETURNING id, product_id, kind, input, status, retry_generation, result, created_at, finished_at
                    """,
                    (uuid4(), product_id, kind, Jsonb(input_data)),
                )
                row = cursor.fetchone()
        except psycopg.errors.UniqueViolation as error:
            raise JobAlreadyQueuedError from error

        if row is None:
            raise RuntimeError("creating generator job returned no row")

        return self._job(row)

    def get(self, job_id: UUID) -> Job | None:
        with connect(self.database_url) as connection, connection.cursor() as cursor:
            cursor.execute(
                """
                SELECT id, product_id, kind, input, status, retry_generation, result, created_at, finished_at
                FROM generator.job WHERE id = %s
                """,
                (job_id,),
            )
            row = cursor.fetchone()

        return self._job(row) if row else None

    def retry(self, product_id: str, kind: str = "audiences") -> bool:
        with connect(self.database_url) as connection, connection.cursor() as cursor:
            cursor.execute(
                """
                UPDATE generator.job SET status = 'queued', retry_generation = retry_generation + 1
                WHERE product_id = %s AND kind = %s AND status = 'failed'
                """,
                (product_id, kind),
            )
            return cursor.rowcount == 1

    def generation_status(
        self, product_id: str, kind: str = "audiences"
    ) -> Literal["created", "working", "completed", "failed"] | None:
        with connect(self.database_url) as connection, connection.cursor() as cursor:
            cursor.execute(
                """SELECT status FROM generator.job
                   WHERE product_id = %s AND kind = %s
                   ORDER BY created_at DESC LIMIT 1""",
                (product_id, kind),
            )
            row = cursor.fetchone()

        if row is None:
            return None

        status = {"queued": "created", "running": "working", "completed": "completed", "failed": "failed"}[
            row["status"]
        ]
        return cast(Literal["created", "working", "completed", "failed"], status)

    def next_completed(self) -> CompletedJob | None:
        with (
            connect(self.database_url) as connection,
            connection.cursor() as cursor,
        ):
            cursor.execute(
                """
                SELECT id, product_id, kind, result FROM generator.job
                WHERE status = 'completed' AND NOT persisted ORDER BY finished_at LIMIT 1
                """
            )
            row = cursor.fetchone()

        if row is None:
            return None

        return CompletedJob(row["id"], row["product_id"], row["kind"], row["result"])

    def mark_persisted(self, job_id: UUID) -> bool:
        with connect(self.database_url) as connection, connection.cursor() as cursor:
            cursor.execute(
                "UPDATE generator.job SET persisted = TRUE WHERE id = %s AND status = 'completed'", (job_id,)
            )
            return cursor.rowcount == 1

    def candidate_ids(self) -> list[UUID]:
        with connect(self.database_url) as connection, connection.cursor() as cursor:
            cursor.execute(
                """
                SELECT id FROM generator.job WHERE kind = 'audiences' AND status IN ('queued', 'running')
                ORDER BY (status = 'running'), created_at
                """
            )
            return [row["id"] for row in cursor.fetchall()]

    def try_lock(self, job_id: UUID) -> tuple[psycopg.Connection[Any], Job] | None:
        connection = connect(self.database_url, autocommit=True)
        try:
            lock_key = job_id.int & ((1 << 63) - 1)
            with connection.cursor() as cursor:
                cursor.execute("SELECT pg_try_advisory_lock(%s) AS locked", (lock_key,))
                lock_row = cursor.fetchone()
                if lock_row is None or not lock_row["locked"]:
                    connection.close()
                    return None

                cursor.execute(
                    """
                    SELECT id, product_id, kind, input, status, retry_generation, result, created_at, finished_at
                    FROM generator.job WHERE id = %s
                    """,
                    (job_id,),
                )
                row = cursor.fetchone()
                if row is None or row["status"] not in {"queued", "running"}:
                    connection.close()
                    return None

                cursor.execute("UPDATE generator.job SET status = 'running' WHERE id = %s", (job_id,))

            return connection, self._job(row)
        except Exception:
            connection.close()
            raise

    @staticmethod
    def finish_locked(connection: psycopg.Connection[Any], job_id: UUID, result: dict[str, Any]) -> None:
        with connection.cursor() as cursor:
            cursor.execute(
                """UPDATE generator.job SET result = %s, status = 'completed', finished_at = NOW()
                   WHERE id = %s AND status = 'running'""",
                (Jsonb(result), job_id),
            )
            if cursor.rowcount != 1:
                raise RuntimeError(f"could not complete generator job {job_id}")

    @staticmethod
    def fail_locked(connection: psycopg.Connection[Any], job_id: UUID) -> None:
        with connection.cursor() as cursor:
            cursor.execute("UPDATE generator.job SET status = 'failed' WHERE id = %s AND status = 'running'", (job_id,))

    def counts(self) -> tuple[int, int]:
        with connect(self.database_url) as connection, connection.cursor() as cursor:
            cursor.execute(
                """SELECT COUNT(*) FILTER (WHERE status IN ('queued', 'running')) AS active_count,
                          COUNT(*) FILTER (WHERE status = 'failed') AS failed_count
                   FROM generator.job WHERE kind = 'audiences'"""
            )
            row = cursor.fetchone()

        if row is None:
            raise RuntimeError("counting generator jobs returned no row")

        return int(row["active_count"]), int(row["failed_count"])

    def current_job_id(self) -> str | None:
        with connect(self.database_url) as connection, connection.cursor() as cursor:
            cursor.execute(
                """
                SELECT id FROM generator.job
                WHERE kind = 'audiences' AND status = 'running'
                ORDER BY created_at LIMIT 1
                """
            )
            row = cursor.fetchone()

        return str(row["id"]) if row else None

    @staticmethod
    def _job(row: dict[str, Any]) -> Job:
        return Job(
            id=cast(UUID, row["id"]),
            product_id=cast(str, row["product_id"]),
            kind=cast(JobKind, row["kind"]),
            input=cast(dict[str, Any], row["input"]),
            status=cast(JobStatus, row["status"]),
            retry_generation=cast(int, row["retry_generation"]),
            result=cast(dict[str, Any] | None, row["result"]),
            created_at=cast(datetime, row["created_at"]),
            finished_at=cast(datetime | None, row["finished_at"]),
        )
