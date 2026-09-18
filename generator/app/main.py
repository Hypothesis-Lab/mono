from __future__ import annotations

import logging
import threading
from contextlib import asynccontextmanager
from uuid import UUID

from fastapi import FastAPI, HTTPException, Request, Response, status

from app.config import Settings
from app.db import Job, JobAlreadyQueuedError, JobRepository
from app.llm import LLMClient
from app.migrations import apply as apply_migrations
from app.models import (
    CompletedJobResponse,
    CreateJobRequest,
    CreateJobResponse,
    InputBundle,
    JobResponse,
    ProductGenerationStatusResponse,
    WorkerStatusResponse,
)
from app.workflow import Workflow

logger = logging.getLogger(__name__)


class Worker:
    def __init__(self, repository: JobRepository, workflow: Workflow) -> None:
        self.repository = repository
        self.workflow = workflow
        self.current_job_id: str | None = None
        self.current_product_id: str | None = None
        self._stop = threading.Event()
        self._thread = threading.Thread(target=self._run, name="generator-worker", daemon=True)

    def start(self) -> None:
        self._thread.start()

    def stop(self) -> None:
        self._stop.set()
        self._thread.join(timeout=5)

    def _run(self) -> None:
        while not self._stop.is_set():
            try:
                job = self.repository.next_eligible()
                if job is None:
                    self._stop.wait(0.5)
                    continue
                self.current_job_id = str(job.id)
                self.current_product_id = job.product_id
                self.workflow.process_one_node(job)
            except Exception:
                logger.exception("generator worker iteration failed")
                self._stop.wait(1)
            finally:
                self.current_job_id = None
                self.current_product_id = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = Settings.from_environment()
    apply_migrations(settings.database_url)
    repository = JobRepository(settings.database_url)
    worker = Worker(repository, Workflow(repository, LLMClient(settings)))
    app.state.repository = repository
    app.state.worker = worker
    worker.start()
    try:
        yield
    finally:
        worker.stop()


app = FastAPI(title="Generator", lifespan=lifespan)


@app.get("/healthz")
def healthz() -> None:
    return None


@app.post("/jobs", response_model=CreateJobResponse, status_code=status.HTTP_201_CREATED)
def create_job(request: Request, body: CreateJobRequest) -> CreateJobResponse:
    repository = _repository(request)
    try:
        job = repository.create(body.product_id, body.input.model_dump(mode="json"))
    except JobAlreadyQueuedError as error:
        raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail="product already queued") from error
    return CreateJobResponse(id=str(job.id), finished=job.finished)


@app.get("/jobs/{job_id}", response_model=JobResponse)
def get_job(request: Request, job_id: UUID) -> JobResponse:
    job = _repository(request).get(job_id)
    if job is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="job not found")
    return _job_response(job)


@app.get(
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


@app.post(
    "/products/{product_id}/generation-retry",
    response_model=ProductGenerationStatusResponse,
)
def retry_product_generation(request: Request, product_id: str) -> ProductGenerationStatusResponse:
    if not _repository(request).retry(product_id):
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="failed generation job not found")
    return ProductGenerationStatusResponse(status="created")


@app.get(
    "/completed-jobs/next",
    response_model=CompletedJobResponse,
    responses={status.HTTP_204_NO_CONTENT: {"description": "no completed jobs"}},
)
def next_completed_job(request: Request) -> CompletedJobResponse | Response:
    job = _repository(request).next_completed()
    if job is None:
        return Response(status_code=status.HTTP_204_NO_CONTENT)
    return CompletedJobResponse(id=str(job.id), product_id=job.product_id, result=job.result)


@app.post("/completed-jobs/{job_id}/persisted", status_code=status.HTTP_204_NO_CONTENT)
def mark_completed_job_persisted(request: Request, job_id: UUID) -> Response:
    if not _repository(request).mark_persisted(job_id):
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="completed job not found")
    return Response(status_code=status.HTTP_204_NO_CONTENT)


@app.get("/status", response_model=WorkerStatusResponse)
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


def _worker(request: Request) -> Worker:
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
