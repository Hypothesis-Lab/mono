"""LangGraph version of the generator's audience research pipeline."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Annotated, Any, NotRequired, TypedDict

from app.catalog import PERSONAS
from app.models import InputBundle
from app.pipeline.telemetry import at_stage
from app.workflow import Workflow
from langgraph.checkpoint.sqlite import SqliteSaver
from langgraph.graph import END, START, StateGraph
from langgraph.types import RetryPolicy

ELIGIBILITY = "eligibility"
INTERVIEW_PRE = "interview_pre"
INTERVIEW_POST = "interview_post"
INSIGHTS = "insights"
SCORE = "score"
RANKING = "ranking"
HYPOTHESIS = "hypothesis"
DIVERSITY = "diversity"
RESULT = "result"
HYPOTHESIS_ANGLES = 3


@dataclass(frozen=True)
class Operation:
    """One concrete workflow operation, identified by stage and audience."""

    stage: str
    audience_id: str | None = None
    angle: int | None = None

    @property
    def id(self) -> str:
        if self.angle is not None:
            return f"{self.stage}:{self.audience_id}:{self.angle}"

        if self.audience_id is not None:
            return f"{self.stage}:{self.audience_id}"

        return self.stage

    @property
    def ref(self) -> Operation:
        return self

    @property
    def entity_id(self) -> str | None:
        return self.audience_id

    @property
    def variant(self) -> int | None:
        return self.angle


class PipelineState(TypedDict):
    input: dict[str, Any]
    progress: Annotated[dict[str, Any], _merge_progress]
    result: NotRequired[dict[str, Any] | None]


def build_graph(workflow: Workflow, *, max_attempts: int = 2, checkpointer: Any = None):
    """Build a graph whose checkpoints and retries are scoped to named operations."""
    if max_attempts < 1:
        raise ValueError("max_attempts must be positive")

    builder = StateGraph(PipelineState)
    audience_ids = [persona.persona_id for persona in PERSONAS]
    retry_policy = RetryPolicy(max_attempts=max_attempts, retry_on=Exception)

    def add_operation(operation: Operation) -> None:
        builder.add_node(
            _graph_node_name(operation),
            _run_operation(workflow, operation),
            retry_policy=retry_policy,
        )

    for audience_id in audience_ids:
        eligibility = Operation(ELIGIBILITY, audience_id)
        pre_interview = Operation(INTERVIEW_PRE, audience_id)
        post_interview = Operation(INTERVIEW_POST, audience_id)
        insights = Operation(INSIGHTS, audience_id)
        score = Operation(SCORE, audience_id)

        for operation in (eligibility, pre_interview, post_interview, insights, score):
            add_operation(operation)

        builder.add_edge(START, _graph_node_name(eligibility))
        builder.add_edge(START, _graph_node_name(pre_interview))
        builder.add_edge(_graph_node_name(pre_interview), _graph_node_name(post_interview))
        builder.add_edge(_graph_node_name(post_interview), _graph_node_name(insights))
        builder.add_edge(_graph_node_name(insights), _graph_node_name(score))

    ranking = Operation(RANKING)
    add_operation(ranking)
    ranking_node = _graph_node_name(ranking)
    builder.add_edge(
        [
            _graph_node_name(operation)
            for audience_id in audience_ids
            for operation in (Operation(ELIGIBILITY, audience_id), Operation(SCORE, audience_id))
        ],
        ranking_node,
    )

    final_hypotheses = []
    for audience_id in audience_ids:
        angles = [Operation(HYPOTHESIS, audience_id, angle) for angle in range(HYPOTHESIS_ANGLES)]
        for operation in angles:
            add_operation(operation)

        builder.add_edge(ranking_node, _graph_node_name(angles[0]))
        builder.add_edge(_graph_node_name(angles[0]), _graph_node_name(angles[1]))
        builder.add_edge(_graph_node_name(angles[1]), _graph_node_name(angles[2]))
        final_hypotheses.append(_graph_node_name(angles[-1]))

    diversity = Operation(DIVERSITY)
    result = Operation(RESULT)
    add_operation(diversity)
    add_operation(result)
    builder.add_edge(final_hypotheses, _graph_node_name(diversity))
    builder.add_edge(_graph_node_name(diversity), _graph_node_name(result))
    builder.add_edge(_graph_node_name(result), END)

    return builder.compile(checkpointer=checkpointer)


def sqlite_checkpointer(path: str):
    """Open a local durable checkpointer for prototype runs."""
    return SqliteSaver.from_conn_string(path)


def _graph_node_name(operation: Operation) -> str:
    parts = ["operation", operation.stage]
    if operation.audience_id is not None:
        parts.append(operation.audience_id)

    if operation.angle is not None:
        parts.append(str(operation.angle))

    return "_".join(parts)


def _run_operation(workflow: Workflow, operation: Any):
    def run(state: PipelineState) -> dict[str, Any]:
        progress = state["progress"]
        node_states = progress.get("nodes", {})

        if operation.stage == HYPOTHESIS:
            ranking = node_states.get(RANKING, {}).get("output", {})
            selected = {row["persona_id"] for row in ranking.get("top_3", [])}
            if operation.audience_id not in selected:
                return {}

        bundle = InputBundle.model_validate(state["input"])
        workflow.validate_input(bundle)
        with at_stage(operation.id):
            output = workflow.execute(bundle, node_states, operation, None)

        if operation.stage == RESULT:
            return {"result": output}

        return {
            "progress": {
                "nodes": {
                    operation.id: {
                        "completed": True,
                        "output": output,
                    }
                }
            }
        }

    return run


def _merge_progress(left: dict[str, Any], right: dict[str, Any]) -> dict[str, Any]:
    merged = {**left, **right}
    merged["nodes"] = {**left.get("nodes", {}), **right.get("nodes", {})}
    return merged
