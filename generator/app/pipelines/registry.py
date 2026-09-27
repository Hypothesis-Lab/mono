from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass
from typing import Any

from pydantic import BaseModel

from app.pipelines.audience_questions.graph import build_graph as build_questions_graph
from app.pipelines.audiences.graph import build_graph
from app.pipelines.audiences.models import InputBundle, Product


@dataclass(frozen=True)
class PipelineDefinition:
    kind: str
    input_model: type[BaseModel]
    graph_builder: Callable[..., Any]

    def validate_input(self, input_data: dict[str, Any]) -> dict[str, Any]:
        return self.input_model.model_validate(input_data).model_dump(mode="json")


class UnknownPipelineError(ValueError):
    pass


PIPELINES = {
    "audiences": PipelineDefinition(
        kind="audiences",
        input_model=InputBundle,
        graph_builder=build_graph,
    ),
    "audience_questions": PipelineDefinition(
        kind="audience_questions",
        input_model=Product,
        graph_builder=build_questions_graph,
    ),
}


def get_pipeline(kind: str) -> PipelineDefinition:
    try:
        return PIPELINES[kind]
    except KeyError as error:
        raise UnknownPipelineError(f"unknown pipeline kind {kind!r}") from error
