from __future__ import annotations

from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)


class CreateJobRequest(StrictModel):
    product_id: str = Field(min_length=1)
    kind: str = Field(min_length=1, max_length=64)
    input: dict[str, Any]


class CreateJobResponse(StrictModel):
    id: str
    kind: str
    status: Literal["queued"]


class JobResponse(StrictModel):
    id: str
    product_id: str
    kind: str
    input: dict[str, Any]
    result: dict[str, Any] | None
    status: Literal["queued", "running", "failed", "completed"]
    created_at: str
    finished_at: str | None


class CompletedJobResponse(StrictModel):
    id: str
    product_id: str
    kind: str
    result: dict[str, Any]


class WorkerStatusResponse(StrictModel):
    current_job_id: str | None
    queued_jobs: int
    failed_jobs: int


class ProductGenerationStatusResponse(StrictModel):
    status: Literal["created", "working", "completed", "failed"]
