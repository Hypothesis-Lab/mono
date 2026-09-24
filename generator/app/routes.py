from uuid import UUID

from fastapi import APIRouter, HTTPException, Request, Response, status
from prometheus_client import CONTENT_TYPE_LATEST, generate_latest

from app.api_models import (
    CompletedJobResponse,
    CreateJobRequest,
    CreateJobResponse,
    JobResponse,
    ProductGenerationStatusResponse,
    WorkerStatusResponse,
)
from app.pipelines.audiences.models import InputBundle
from app.repositories.jobs import Job, JobAlreadyQueuedError, JobRepository

router = APIRouter()


@router.get("/metrics", include_in_schema=False)
def metrics() -> Response:
    return Response(generate_latest(), media_type=CONTENT_TYPE_LATEST)


@router.get("/healthz")
def healthz() -> None:
    return None


@router.post("/jobs", response_model=CreateJobResponse, status_code=status.HTTP_201_CREATED)
def create_job(request: Request, body: CreateJobRequest) -> CreateJobResponse:
    repository = _repository(request)
    try:
        job = repository.create(body.product_id, body.kind, body.input.model_dump(mode="json"))
    except JobAlreadyQueuedError as error:
        raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail="product already queued") from error

    return CreateJobResponse(id=str(job.id), kind="audiences", status="queued")


@router.get("/jobs/{job_id}", response_model=JobResponse)
def get_job(request: Request, job_id: UUID) -> JobResponse:
    job = _repository(request).get(job_id)
    if job is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="job not found")

    return _job_response(job)


@router.get(
    "/products/{product_id}/generation-status",
    response_model=ProductGenerationStatusResponse,
)
def product_generation_status(request: Request, product_id: str) -> ProductGenerationStatusResponse:
    generation_status = _repository(request).generation_status(product_id)
    if generation_status is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="generation job not found")

    return ProductGenerationStatusResponse(status=generation_status)


@router.post(
    "/products/{product_id}/generation-retry",
    response_model=ProductGenerationStatusResponse,
)
def retry_product_generation(request: Request, product_id: str) -> ProductGenerationStatusResponse:
    if not _repository(request).retry(product_id):
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="failed generation job not found")

    return ProductGenerationStatusResponse(status="created")


@router.get(
    "/completed-jobs/next",
    response_model=CompletedJobResponse,
    responses={status.HTTP_204_NO_CONTENT: {"description": "no completed jobs"}},
)
def next_completed_job(request: Request) -> CompletedJobResponse | Response:
    job = _repository(request).next_completed()
    if job is None:
        return Response(status_code=status.HTTP_204_NO_CONTENT)

    return CompletedJobResponse(id=str(job.id), product_id=job.product_id, kind=job.kind, result=job.result)


@router.post("/completed-jobs/{job_id}/persisted", status_code=status.HTTP_204_NO_CONTENT)
def mark_completed_job_persisted(request: Request, job_id: UUID) -> Response:
    if not _repository(request).mark_persisted(job_id):
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="completed job not found")

    return Response(status_code=status.HTTP_204_NO_CONTENT)


@router.get("/status", response_model=WorkerStatusResponse)
def worker_status(request: Request) -> WorkerStatusResponse:
    repository = _repository(request)
    queued_jobs, failed_jobs = repository.counts()
    return WorkerStatusResponse(
        current_job_id=repository.current_job_id(),
        queued_jobs=queued_jobs,
        failed_jobs=failed_jobs,
    )


def _repository(request: Request) -> JobRepository:
    return request.app.state.repository


def _job_response(job: Job) -> JobResponse:
    return JobResponse(
        id=str(job.id),
        product_id=job.product_id,
        kind=job.kind,
        input=InputBundle.model_validate(job.input),
        result=job.result,
        status=job.status,
        created_at=job.created_at.isoformat(),
        finished_at=job.finished_at.isoformat() if job.finished_at else None,
    )
