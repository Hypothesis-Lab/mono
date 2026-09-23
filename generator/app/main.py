from __future__ import annotations

import logging
import threading
from concurrent.futures import ThreadPoolExecutor
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.config import Settings
from app.db import JobRepository
from app.llm_client import LLMClient
from app.migrations import apply as apply_migrations
from app.pipeline.runner import PipelineRunner
from app.routes import router
from app.workflow import Workflow

logger = logging.getLogger(__name__)


class Worker:
    def __init__(self, runner: PipelineRunner, concurrency: int) -> None:
        self.runner = runner
        self.concurrency = concurrency
        self._stop = threading.Event()
        self._thread = threading.Thread(target=self._run, name="generator-supervisor", daemon=True)

    def start(self) -> None:
        self._thread.start()

    def stop(self) -> None:
        self._stop.set()
        self._thread.join(timeout=5)

    def _run(self) -> None:
        with ThreadPoolExecutor(max_workers=self.concurrency, thread_name_prefix="generator-worker") as pool:
            tasks = [pool.submit(self._work) for _ in range(self.concurrency)]
            for task in tasks:
                task.result()

    def _work(self) -> None:
        while not self._stop.is_set():
            try:
                if self.runner.execute_next() is None:
                    self._stop.wait(0.5)
            except Exception:
                logger.exception("generator worker iteration failed")
                self._stop.wait(1)


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = Settings.from_environment()
    apply_migrations(settings.database_url)
    repository = JobRepository(settings.database_url)
    llm = LLMClient(
        base_url=settings.openai_base_url,
        api_key=settings.openai_api_key,
        model=settings.openai_model,
        reasoning_effort=settings.openai_reasoning_effort,
    )
    worker = Worker(PipelineRunner(Workflow(llm), repository), settings.worker_concurrency)
    app.state.repository = repository
    worker.start()
    try:
        yield
    finally:
        worker.stop()


app = FastAPI(title="Generator", lifespan=lifespan)
app.include_router(router)
