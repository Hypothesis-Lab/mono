from __future__ import annotations

from typing import Any

from langchain_core.runnables import RunnableConfig
from langgraph.graph import END, START, StateGraph
from langgraph.types import Send

from app.observability import Observer, observe_operation
from app.pipelines.audiences.catalog import PERSONAS
from app.pipelines.audiences.input import validate_input
from app.pipelines.audiences.models import InputBundle
from app.pipelines.audiences.operations import AudienceOperations
from app.pipelines.audiences.state import AudienceOperation, AudiencePipelineState, RepairContext, Stage


def build_graph(operations: AudienceOperations, *, checkpointer: Any = None, observer: Observer | None = None):
    """Build the resumable audience pipeline with caller-owned model and persistence dependencies."""
    builder = StateGraph(AudiencePipelineState)

    def validate_input(state: AudiencePipelineState) -> dict[str, Any]:
        input_data = state.get("input")
        if input_data is None:
            raise ValueError("audience pipeline input is missing")

        validate_input(InputBundle.model_validate(input_data))
        return {}

    def run_operation(state: Any, config: RunnableConfig) -> dict[str, Any]:
        operation_value = state["operation"]
        operation = AudienceOperation(
            Stage(operation_value["stage"]), operation_value.get("audience_id"), operation_value.get("angle")
        )
        outputs = state.get("outputs", {})
        repairs: dict[str, RepairContext] = state.get("repairs", {})
        retry_generation = config.get("configurable", {}).get("retry_generation", 0)
        repair = repairs.get(operation.id)
        if repair is not None and retry_generation > repair["retry_generation"]:
            return {
                "repairs": {
                    operation.id: {
                        "attempt": 0,
                        "last_error": repair["last_error"],
                        "raw_response": repair["raw_response"],
                        "retry_generation": retry_generation,
                    }
                }
            }

        node_states = {key: {"completed": True, "output": value["output"]} for key, value in outputs.items()}
        bundle = InputBundle.model_validate(state["input"])
        operations.clear_last_raw_response()
        with observe_operation(observer, operation.id):
            if repair is not None and repair["attempt"] >= 3:
                raise ValueError(repair["last_error"])

            try:
                output = operations.execute_operation(
                    bundle,
                    node_states,
                    operation,
                    _repair_feedback(repair) if repair else None,
                )
            except ValueError as error:
                if operation.stage not in {
                    Stage.INTERVIEW_PRE,
                    Stage.INTERVIEW_POST,
                    Stage.INSIGHTS,
                    Stage.SCORE,
                    Stage.HYPOTHESIS,
                }:
                    raise

                attempt = (repair["attempt"] if repair else 0) + 1
                return {
                    "repairs": {
                        operation.id: {
                            "attempt": attempt,
                            "last_error": str(error)[:1000],
                            "raw_response": operations.last_raw_response,
                            "retry_generation": retry_generation,
                        }
                    }
                }

        if operation.stage == Stage.RESULT:
            return {"result": output}

        return {"outputs": {operation.id: {"completed": True, "output": output}}}

    def schedule(state: AudiencePipelineState) -> dict[str, Any]:
        return {}

    def next_operations(state: AudiencePipelineState) -> list[Send] | str:
        if state.get("result") is not None:
            return END

        input_data = state.get("input")
        if input_data is None:
            raise ValueError("audience pipeline input is missing")

        outputs = state.get("outputs", {})
        ready = _ready_operations(outputs)
        return [
            Send(
                "execute",
                {
                    "input": input_data,
                    "outputs": outputs,
                    "repairs": state.get("repairs", {}),
                    "operation": {
                        "stage": operation.stage.value,
                        "audience_id": operation.audience_id,
                        "angle": operation.angle,
                    },
                },
            )
            for operation in ready
        ] or END

    builder.add_node("validate_input", validate_input)
    builder.add_node("schedule", schedule)
    builder.add_node("execute", run_operation)
    builder.add_edge(START, "validate_input")
    builder.add_edge("validate_input", "schedule")
    builder.add_conditional_edges("schedule", next_operations, ["execute", END])
    builder.add_edge("execute", "schedule")
    return builder.compile(checkpointer=checkpointer)


def _repair_feedback(repair: RepairContext) -> str:
    raw_response = repair["raw_response"]
    if raw_response is None:
        return repair["last_error"]

    return f"Prior raw response:\n{raw_response[:12000]}\nValidation feedback:\n{repair['last_error']}"


def _ready_operations(outputs: dict[str, Any]) -> list[AudienceOperation]:
    operations: list[AudienceOperation] = []
    audience_ids = [persona.persona_id for persona in PERSONAS]

    for audience_id in audience_ids:
        for stage in (Stage.ELIGIBILITY, Stage.INTERVIEW_PRE):
            operation = AudienceOperation(stage, audience_id)
            if operation.id not in outputs:
                operations.append(operation)

        pre = AudienceOperation(Stage.INTERVIEW_PRE, audience_id)
        post = AudienceOperation(Stage.INTERVIEW_POST, audience_id)
        insights = AudienceOperation(Stage.INSIGHTS, audience_id)
        score = AudienceOperation(Stage.SCORE, audience_id)
        if pre.id in outputs and post.id not in outputs:
            operations.append(post)

        if post.id in outputs and insights.id not in outputs:
            operations.append(insights)

        if insights.id in outputs and score.id not in outputs:
            operations.append(score)

    ranking = AudienceOperation(Stage.RANKING)
    all_scored = all(
        AudienceOperation(stage, audience_id).id in outputs
        for audience_id in audience_ids
        for stage in (Stage.ELIGIBILITY, Stage.SCORE)
    )
    if all_scored and ranking.id not in outputs:
        operations.append(ranking)

    if ranking.id not in outputs:
        return operations

    selected_ids = [row["persona_id"] for row in outputs[ranking.id]["output"]["top_3"]]
    for audience_id in selected_ids:
        for angle in range(3):
            operation = AudienceOperation(Stage.HYPOTHESIS, audience_id, angle)
            previous = AudienceOperation(Stage.HYPOTHESIS, audience_id, angle - 1) if angle else None
            if operation.id not in outputs and (previous is None or previous.id in outputs):
                operations.append(operation)

    diversity = AudienceOperation(Stage.DIVERSITY)
    hypotheses_completed = all(
        AudienceOperation(Stage.HYPOTHESIS, audience_id, 2).id in outputs for audience_id in selected_ids
    )
    if hypotheses_completed and diversity.id not in outputs:
        operations.append(diversity)

    result = AudienceOperation(Stage.RESULT)
    if diversity.id in outputs and result.id not in outputs:
        operations.append(result)

    return operations
