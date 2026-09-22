from uuid import UUID

from fastapi import APIRouter, HTTPException, Request, Response, status

from app.db import Job, JobAlreadyQueuedError, JobRepository
from app.models import (
    CompletedJobResponse,
    CreateJobRequest,
    CreateJobResponse,
    InputBundle,
    JobResponse,
    ProductGenerationStatusResponse,
    WorkerStatusResponse,
)

router = APIRouter()


@router.get("/healthz")
def healthz() -> None:
    return None


@router.post("/jobs", response_model=CreateJobResponse, status_code=status.HTTP_201_CREATED)
def create_job(request: Request, body: CreateJobRequest) -> CreateJobResponse:
    repository = _repository(request)
    try:
        job = repository.create(body.product_id, body.input.model_dump(mode="json"))
    except JobAlreadyQueuedError as error:
        raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail="product already queued") from error

    return CreateJobResponse(id=str(job.id), finished=job.finished)


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
    if _worker(request).current_product_id == product_id:
        return ProductGenerationStatusResponse(status="working")

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

    return CompletedJobResponse(id=str(job.id), product_id=job.product_id, result=job.result)


@router.post("/completed-jobs/{job_id}/persisted", status_code=status.HTTP_204_NO_CONTENT)
def mark_completed_job_persisted(request: Request, job_id: UUID) -> Response:
    if not _repository(request).mark_persisted(job_id):
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="completed job not found")

    return Response(status_code=status.HTTP_204_NO_CONTENT)


@router.get("/status", response_model=WorkerStatusResponse)
def worker_status(request: Request) -> WorkerStatusResponse:
    repository = _repository(request)
    queued_jobs, blocked_jobs = repository.counts()
    return WorkerStatusResponse(
        current_job_id=_worker(request).current_job_id,
        queued_jobs=queued_jobs,
        blocked_jobs=blocked_jobs,
    )


def _repository(request: Request) -> JobRepository:
    return request.app.state.repository


def _worker(request: Request):
    return request.app.state.worker


def _job_response(job: Job) -> JobResponse:
    return JobResponse(
        id=str(job.id),
        product_id=job.product_id,
        input=InputBundle.model_validate(job.input),
        progress=job.progress,
        result=job.result,
        finished=job.finished,
        created_at=job.created_at.isoformat(),
        finished_at=job.finished_at.isoformat() if job.finished_at else None,
    )
