from __future__ import annotations

from collections.abc import Mapping
from typing import Annotated, Any, TypedDict

from langchain_core.runnables import RunnableConfig
from langgraph.checkpoint.base import BaseCheckpointSaver
from langgraph.graph import END, START, StateGraph
from langgraph.types import Send

from app.observability import Observer, observe_operation
from app.pipelines.audience_questions.catalog import QUESTION_AUDIENCES, QUESTIONS
from app.pipelines.audience_questions.models import QuestionsResult
from app.pipelines.audience_questions.stages import AudienceQuestionsStage
from app.pipelines.audiences.graph.state import RepairContext
from app.pipelines.audiences.models import Product
from app.pipelines.audiences.stages.errors import CorrectionRequired


class OperationOutput(TypedDict):
    completed: bool
    output: Any


class QuestionsOperation(TypedDict):
    audience_id: str | None


class QuestionsPipelineState(TypedDict, total=False):
    input: dict[str, object]
    outputs: Annotated[dict[str, OperationOutput], _merge_outputs]
    repairs: Annotated[dict[str, RepairContext], _merge_repairs]
    operation: QuestionsOperation
    result: dict[str, object]


def build_graph(stages: Any, *, checkpointer: BaseCheckpointSaver | None = None, observer: Observer | None = None):
    builder = StateGraph(QuestionsPipelineState)
    question_stage = AudienceQuestionsStage(stages.model_client)

    def validate_input(state: QuestionsPipelineState) -> dict[str, Any]:
        input_data = state.get("input")
        if input_data is None:
            raise ValueError("audience questions pipeline input is missing")

        Product.model_validate(input_data)
        return {}

    def execute(state: QuestionsPipelineState, config: RunnableConfig) -> dict[str, Any]:
        operation = state.get("operation")
        if operation is None:
            raise ValueError("audience questions pipeline operation is missing")

        audience_id = operation["audience_id"]
        if audience_id is None:
            return {"result": _result(state.get("outputs", {}))}

        persona = next(persona for persona in QUESTION_AUDIENCES if persona.persona_id == audience_id)
        repair = state.get("repairs", {}).get(_operation_id(audience_id))
        retry_generation = config.get("configurable", {}).get("retry_generation", 0)
        if repair is not None and retry_generation > repair["retry_generation"]:
            return {
                "repairs": {
                    _operation_id(audience_id): {
                        "attempt": 0,
                        "last_error": repair["last_error"],
                        "raw_response": repair["raw_response"],
                        "retry_generation": retry_generation,
                    }
                }
            }

        if repair is not None and repair["attempt"] >= 3:
            raise ValueError(repair["last_error"])

        input_data = state.get("input")
        if input_data is None:
            raise ValueError("audience questions pipeline input is missing")

        product = Product.model_validate(input_data)
        with observe_operation(observer, _operation_id(audience_id)):
            try:
                output = question_stage.answer(
                    product,
                    persona,
                    repair["last_error"] if repair is not None else None,
                )
            except CorrectionRequired as error:
                return {
                    "repairs": {
                        _operation_id(audience_id): {
                            "attempt": (repair["attempt"] if repair else 0) + 1,
                            "last_error": str(error)[:1000],
                            "raw_response": error.raw_response,
                            "retry_generation": retry_generation,
                        }
                    }
                }

        return {"outputs": {_operation_id(audience_id): {"completed": True, "output": output}}}

    def schedule(state: QuestionsPipelineState) -> dict[str, Any]:
        return {}

    def next_operations(state: QuestionsPipelineState) -> list[Send] | str:
        outputs = state.get("outputs", {})
        ready = [
            persona.persona_id for persona in QUESTION_AUDIENCES if _operation_id(persona.persona_id) not in outputs
        ]
        if ready:
            return [
                Send(
                    "execute",
                    {
                        "input": state.get("input"),
                        "outputs": outputs,
                        "repairs": state.get("repairs", {}),
                        "operation": {"audience_id": audience_id},
                    },
                )
                for audience_id in ready
            ]

        if "result" not in state:
            return [
                Send(
                    "execute",
                    {
                        "input": state.get("input"),
                        "outputs": outputs,
                        "repairs": state.get("repairs", {}),
                        "operation": {"audience_id": None},
                    },
                )
            ]

        return END

    builder.add_node("validate_input", validate_input)
    builder.add_node("schedule", schedule)
    builder.add_node("execute", execute)
    builder.add_edge(START, "validate_input")
    builder.add_edge("validate_input", "schedule")
    builder.add_conditional_edges("schedule", next_operations, ["execute", END])
    builder.add_edge("execute", "schedule")
    return builder.compile(checkpointer=checkpointer)


def _operation_id(audience_id: str) -> str:
    return f"answer:{audience_id}"


def _result(outputs: Mapping[str, OperationOutput]) -> dict[str, object]:
    answers = []
    for persona in QUESTION_AUDIENCES:
        output = outputs[_operation_id(persona.persona_id)]["output"]
        for answer in output["answers"]:
            answers.append(
                {
                    "audience_id": persona.persona_id,
                    "audience_label": persona.label,
                    "question_id": answer["question_id"],
                    "answer": answer["answer"],
                }
            )

    return QuestionsResult(questions=QUESTIONS, answers=answers).model_dump(mode="json")


def _merge_outputs(
    left: Mapping[str, OperationOutput], right: Mapping[str, OperationOutput]
) -> dict[str, OperationOutput]:
    return {**left, **right}


def _merge_repairs(left: Mapping[str, RepairContext], right: Mapping[str, RepairContext]) -> dict[str, RepairContext]:
    return {**left, **right}
