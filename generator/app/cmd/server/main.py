from __future__ import annotations

from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI

from app.config import ServerConfig, Settings
from app.language_model_client.client import ModelClient
from app.migrations import apply as apply_migrations
from app.observability import PrometheusObserver
from app.pipelines.audiences.stages import AudienceStages
from app.repositories.jobs import JobRepository
from app.routes import router
from app.worker import AudienceWorker


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = Settings.from_environment()
    apply_migrations(settings.database_url)
    repository = JobRepository(settings.database_url)
    observer = PrometheusObserver()
    llm = ModelClient(settings.model_config, observer)
    worker = AudienceWorker(
        repository,
        AudienceStages(llm),
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


def main() -> None:
    config = ServerConfig.from_environment()
    uvicorn.run(app, host=config.host, port=config.port)
