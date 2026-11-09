from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass
from typing import Any

from pydantic import BaseModel

from app.pipelines.audience_selection.graph import build_graph as build_selection_graph
from app.pipelines.audience_selection.models import AudienceSelectionInput
from app.pipelines.hypotheses.graph import build_graph as build_hypothesis_graph
from app.pipelines.hypotheses.models import HypothesisInput
from app.pipelines.product.graph import build_graph as build_product_graph
from app.pipelines.product.models import ProductInput


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
    "product_analysis": PipelineDefinition(
        kind="product_analysis",
        input_model=ProductInput,
        graph_builder=build_product_graph,
    ),
    "audience_selection": PipelineDefinition(
        kind="audience_selection",
        input_model=AudienceSelectionInput,
        graph_builder=build_selection_graph,
    ),
    "hypothesis_generation": PipelineDefinition(
        kind="hypothesis_generation",
        input_model=HypothesisInput,
        graph_builder=build_hypothesis_graph,
    ),
}


def get_pipeline(kind: str) -> PipelineDefinition:
    try:
        return PIPELINES[kind]
    except KeyError as error:
        raise UnknownPipelineError(f"unknown pipeline kind {kind!r}") from error
