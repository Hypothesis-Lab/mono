from __future__ import annotations

import logging
import threading
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.config import Settings
from app.db import JobRepository
from app.llm_client import LLMClient
from app.migrations import apply as apply_migrations
from app.routes import router
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
    worker = Worker(
        repository,
        Workflow(
            repository,
            LLMClient(
                base_url=settings.openai_base_url,
                api_key=settings.openai_api_key,
                model=settings.openai_model,
                reasoning_effort=settings.openai_reasoning_effort,
            ),
        ),
    )
    app.state.repository = repository
    app.state.worker = worker
    worker.start()
    try:
        yield
    finally:
        worker.stop()


app = FastAPI(title="Generator", lifespan=lifespan)
app.include_router(router)
