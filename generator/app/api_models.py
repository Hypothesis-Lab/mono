from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, ConfigDict, Field

from app.pipelines.audiences.models import InputBundle


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)


class CreateJobRequest(StrictModel):
    product_id: str = Field(min_length=1)
    kind: Literal["audiences"]
    input: InputBundle


class CreateJobResponse(StrictModel):
    id: str
    kind: Literal["audiences"]
    status: Literal["queued"]


class JobResponse(StrictModel):
    id: str
    product_id: str
    kind: Literal["audiences"]
    input: InputBundle
    result: dict[str, object] | None
    status: Literal["queued", "running", "failed", "completed"]
    created_at: str
    finished_at: str | None


class CompletedJobResponse(StrictModel):
    id: str
    product_id: str
    kind: Literal["audiences"]
    result: dict[str, object]


class WorkerStatusResponse(StrictModel):
    current_job_id: str | None
    queued_jobs: int
    failed_jobs: int


class ProductGenerationStatusResponse(StrictModel):
    status: Literal["created", "working", "failed"]
