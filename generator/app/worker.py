from __future__ import annotations

import logging
import threading
from concurrent.futures import Future, ThreadPoolExecutor
from time import perf_counter
from typing import Any

import psycopg
from langchain_core.runnables import RunnableConfig
from langgraph.checkpoint.postgres import PostgresSaver

from app.observability import Measurement, Observer
from app.pipelines.audiences.graph import build_graph
from app.pipelines.audiences.stages import AudienceStages
from app.repositories.jobs import Job, JobRepository

logger = logging.getLogger(__name__)


class AudienceWorker:
    def __init__(
        self,
        repository: JobRepository,
        stages: AudienceStages,
        observer: Observer,
        concurrency: int,
        graph_max_concurrency: int,
    ) -> None:
        self.repository = repository
        self.stages = stages
        self.observer = observer
        self.concurrency = concurrency
        self.graph_max_concurrency = graph_max_concurrency
        self._stop = threading.Event()
        self._condition = threading.Condition()
        self._thread = threading.Thread(target=self._run, name="audiences-dispatcher", daemon=True)

    def start(self) -> None:
        self._thread.start()

    def stop(self) -> None:
        self._stop.set()
        with self._condition:
            self._condition.notify_all()

        self._thread.join()

    def _run(self) -> None:
        active: set[Future[None]] = set()
        with ThreadPoolExecutor(max_workers=self.concurrency, thread_name_prefix="audiences-worker") as pool:
            while not self._stop.is_set():
                active = {future for future in active if not future.done()}
                capacity = self.concurrency - len(active)
                if capacity:
                    try:
                        for job_id in self.repository.candidate_ids():
                            if capacity == 0:
                                break

                            claimed = self.repository.try_lock(job_id)
                            if claimed is None:
                                continue

                            connection, job = claimed
                            future = pool.submit(self._run_job, connection, job)
                            future.add_done_callback(self._wake_dispatcher)
                            active.add(future)
                            capacity -= 1
                    except Exception:
                        logger.exception("audiences dispatcher scan failed")

                with self._condition:
                    if self._stop.is_set():
                        break

                    self._condition.wait(timeout=5 if capacity else None)

    def _wake_dispatcher(self, _: Future[None]) -> None:
        with self._condition:
            self._condition.notify_all()

    def _run_job(self, connection: psycopg.Connection[Any], job: Job) -> None:
        started = perf_counter()
        outcome = "success"
        try:
            checkpointer = PostgresSaver(connection)
            graph = build_graph(self.stages, checkpointer=checkpointer, observer=self.observer)
            config: RunnableConfig = {
                "configurable": {"thread_id": str(job.id), "retry_generation": job.retry_generation},
                "max_concurrency": self.graph_max_concurrency,
            }
            checkpoint = graph.get_state(config)
            if checkpoint.values:
                state = graph.invoke(None, config)
            else:
                state = graph.invoke({"input": job.input, "outputs": {}}, config)

            result = state.get("result")
            if not isinstance(result, dict):
                raise RuntimeError("audience graph completed without a result")

            JobRepository.finish_locked(connection, job.id, result)
        except Exception:
            outcome = "error"
            logger.exception("audience generation job failed", extra={"job_id": str(job.id)})

            try:
                JobRepository.fail_locked(connection, job.id)
            except Exception:
                logger.exception("could not persist audience job failure", extra={"job_id": str(job.id)})
        finally:
            self.observer.observe(
                Measurement(
                    kind="job",
                    name=job.kind,
                    model=None,
                    duration_ms=(perf_counter() - started) * 1000,
                    outcome=outcome,
                )
            )
            connection.close()
