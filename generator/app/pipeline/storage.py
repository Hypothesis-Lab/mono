"""Checkpoint contract and ephemeral implementation for the pipeline runner."""

from copy import deepcopy
from dataclasses import dataclass, replace
from datetime import UTC, datetime
from threading import Lock
from typing import Any, Protocol
from uuid import UUID, uuid4

from app.models import InputBundle


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


class JobStore(Protocol):
    def claim_next(self, max_attempts: int) -> tuple[Job, UUID] | None: ...

    def save_progress(self, job_id: UUID, token: UUID, progress: dict[str, Any]) -> None: ...

    def finish(self, job_id: UUID, token: UUID, result: dict[str, Any]) -> None: ...


class InMemoryJobStore:
    """One job in process memory; no database or disk writes."""

    def __init__(self, bundle: InputBundle) -> None:
        self.job = Job(
            id=uuid4(),
            product_id="in-memory",
            input=bundle.model_dump(mode="json"),
            progress={"nodes": {}},
            result=None,
            finished=False,
            created_at=datetime.now(UTC),
            finished_at=None,
        )
        self._token: UUID | None = None
        self._lock = Lock()

    def claim_next(self, max_attempts: int) -> tuple[Job, UUID] | None:
        with self._lock:
            if self.job.finished or self._token is not None:
                return None

            for state in self.job.progress["nodes"].values():
                if not state.get("completed") and state.get("retry_count", 0) >= max_attempts:
                    return None

            self._token = uuid4()
            return replace(self.job, progress=deepcopy(self.job.progress)), self._token

    def save_progress(self, job_id: UUID, token: UUID, progress: dict[str, Any]) -> None:
        with self._lock:
            self._check_claim(job_id, token)
            self.job = replace(self.job, progress=deepcopy(progress))
            self._token = None

    def finish(self, job_id: UUID, token: UUID, result: dict[str, Any]) -> None:
        with self._lock:
            self._check_claim(job_id, token)
            self.job = replace(self.job, result=deepcopy(result), finished=True, finished_at=datetime.now(UTC))
            self._token = None

    def _check_claim(self, job_id: UUID, token: UUID) -> None:
        if self.job.id != job_id or self._token != token:
            raise RuntimeError("lost in-memory job claim")
