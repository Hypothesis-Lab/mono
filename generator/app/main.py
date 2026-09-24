from __future__ import annotations

from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.config import Settings
from app.migrations import apply as apply_migrations
from app.observability import PrometheusObserver
from app.pipelines.audiences.llm import ModelClient, ModelConfig
from app.pipelines.audiences.operations import AudienceOperations
from app.repositories.jobs import JobRepository
from app.routes import router
from app.worker import AudienceWorker


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = Settings.from_environment()
    apply_migrations(settings.database_url)
    repository = JobRepository(settings.database_url)
    observer = PrometheusObserver()
    llm = ModelClient(
        ModelConfig(
            base_url=settings.openai_base_url,
            api_key=settings.openai_api_key,
            model=settings.openai_model,
            reasoning_effort=settings.openai_reasoning_effort,
        ),
        observer,
    )
    worker = AudienceWorker(
        repository,
        AudienceOperations(llm),
        observer,
        settings.worker_concurrency,
        settings.graph_max_concurrency,
    )
    app.state.repository = repository
    app.state.prometheus_observer = observer
    worker.start()
    try:
        yield
    finally:
        worker.stop()


app = FastAPI(title="Generator", lifespan=lifespan)
app.include_router(router)
