"""Execute the same graph with or without a durable job store."""

import logging
from copy import deepcopy
from dataclasses import dataclass
from typing import Any

from app.models import InputBundle
from app.pipeline.blueprint import PipelineGraph, Stage
from app.pipeline.graph import PipelineNode
from app.pipeline.storage import JobStore
from app.pipeline.telemetry import at_stage
from app.workflow import Workflow

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class StepOutcome:
    progress: dict[str, Any]
    result: dict[str, Any] | None = None
    error: str | None = None


class PipelineRunner:
    def __init__(
        self, workflow: Workflow, store: JobStore, graph: PipelineGraph | None = None, max_attempts: int = 2
    ) -> None:
        if max_attempts < 1:
            raise ValueError("max_attempts must be positive")

        self.workflow = workflow
        self.store = store
        self.graph = graph or PipelineGraph()
        self.max_attempts = max_attempts

    def step(self, bundle: InputBundle, progress: dict[str, Any]) -> StepOutcome:
        progress = deepcopy(progress)
        states = progress.setdefault("nodes", {})
        outputs = {
            node_id: state["output"]
            for node_id, state in states.items()
            if state.get("completed") is True and isinstance(state.get("output"), dict)
        }
        node = self._next_ready_node(outputs)
        state = states.get(node.id, {})
        attempts = int(state.get("retry_count", 0))
        if attempts >= self.max_attempts:
            raise ValueError(f"node {node.id} is blocked after {attempts} failures")

        try:
            self.workflow.validate_input(bundle)
            with at_stage(node.id):
                output = self.workflow.execute(bundle, states, node, state.get("last_error"))
        except Exception as error:
            states[node.id] = {
                "completed": False,
                "retry_count": attempts + 1,
                "last_error": str(error)[:1000],
            }
            logger.exception("generation node failed", extra={"node_id": node.id})
            return StepOutcome(progress=progress, error=str(error))

        if node.ref.stage == Stage.RESULT:
            return StepOutcome(progress=progress, result=output)

        states[node.id] = {"completed": True, "retry_count": attempts, "output": output}
        return StepOutcome(progress=progress)

    def _next_ready_node(self, outputs: dict[str, dict[str, Any]]) -> PipelineNode:
        for node in self.graph.nodes(outputs):
            if node.id in outputs:
                continue

            missing = [ref.id for ref in node.dependencies if ref.id not in outputs]
            if missing:
                raise ValueError(f"{node.id} is missing dependencies: {missing}")

            return node

        raise ValueError("generation result was already produced")

    def execute_next(self) -> StepOutcome | None:
        """Advance one claimed job by one graph node; return None if none are available."""
        claim = self.store.claim_next(self.max_attempts)
        if claim is None:
            return None

        job, token = claim
        outcome = self.step(InputBundle.model_validate(job.input), job.progress)
        if outcome.result is None:
            self.store.save_progress(job.id, token, outcome.progress)
        else:
            self.store.finish(job.id, token, outcome.result)

        return outcome
