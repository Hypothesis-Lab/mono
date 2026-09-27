from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from enum import StrEnum
from typing import Annotated, Any, TypedDict


class OperationOutput(TypedDict):
    completed: bool
    output: Any


class RepairContext(TypedDict):
    attempt: int
    last_error: str
    raw_response: str | None
    retry_generation: int


class Stage(StrEnum):
    ELIGIBILITY = "eligibility"
    INTERVIEW_PRE = "interview_pre"
    INTERVIEW_POST = "interview_post"
    INSIGHTS = "insights"
    SCORE = "score"
    RANKING = "ranking"
    HYPOTHESIS = "hypothesis"
    DIVERSITY = "diversity"
    RESULT = "result"


@dataclass(frozen=True)
class AudienceOperation:
    stage: Stage
    audience_id: str | None = None
    angle: int | None = None

    @property
    def id(self) -> str:
        if self.angle is not None:
            return f"{self.stage}:{self.audience_id}:{self.angle}"

        if self.audience_id is not None:
            return f"{self.stage}:{self.audience_id}"

        return str(self.stage)


class AudiencePipelineState(TypedDict, total=False):
    input: dict[str, object]
    outputs: Annotated[dict[str, OperationOutput], merge_operation_outputs]
    repairs: Annotated[dict[str, RepairContext], merge_repair_contexts]
    operation: dict[str, Any]
    result: dict[str, object]


def merge_operation_outputs(
    left: Mapping[str, OperationOutput], right: Mapping[str, OperationOutput]
) -> dict[str, OperationOutput]:
    return {**left, **right}


def merge_repair_contexts(
    left: Mapping[str, RepairContext], right: Mapping[str, RepairContext]
) -> dict[str, RepairContext]:
    return {**left, **right}
