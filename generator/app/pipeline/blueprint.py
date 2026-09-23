"""The generator's business pipeline: what each stage does and needs.

Read PIPELINE from top to bottom. The code below it expands this one definition
for the catalog personas and, after ranking, the selected personas.
"""

from dataclasses import dataclass
from enum import StrEnum
from typing import Any

from app.catalog import PERSONAS
from app.pipeline.graph import NodeRef, PipelineNode


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


class Scope(StrEnum):
    PER_PERSONA = "per persona"
    ONCE = "once"
    PER_SELECTED_ANGLE = "per selected persona and angle"


class HypothesisAngle(StrEnum):
    PAIN_RELIEF = "pain_relief"
    NEED_SOLUTION = "need_solution"
    DECISION_SUPPORT = "decision_support"


@dataclass(frozen=True)
class StageDefinition:
    stage: Stage
    purpose: str
    scope: Scope
    needs: tuple[Stage, ...] = ()
    needs_previous_angle: bool = False


PIPELINE = (
    StageDefinition(Stage.ELIGIBILITY, "Check product rules against a synthetic persona", Scope.PER_PERSONA),
    StageDefinition(Stage.INTERVIEW_PRE, "Ask Q01-Q05 before showing the product", Scope.PER_PERSONA),
    StageDefinition(
        Stage.INTERVIEW_POST,
        "Show the product; ask Q06-Q10 with the pre-product answers in context",
        Scope.PER_PERSONA,
        needs=(Stage.INTERVIEW_PRE,),
    ),
    StageDefinition(
        Stage.INSIGHTS,
        "Extract pains, needs and other insights with quotes from the answers",
        Scope.PER_PERSONA,
        needs=(Stage.INTERVIEW_PRE, Stage.INTERVIEW_POST),
    ),
    StageDefinition(
        Stage.SCORE,
        "Score product fit using persona research and product facts",
        Scope.PER_PERSONA,
        needs=(Stage.INSIGHTS,),
    ),
    StageDefinition(
        Stage.RANKING,
        "Rank all personas and select up to three",
        Scope.ONCE,
        needs=(Stage.ELIGIBILITY, Stage.SCORE),
    ),
    StageDefinition(
        Stage.HYPOTHESIS,
        "Draft three distinct pain-and-need experiments for each selected persona",
        Scope.PER_SELECTED_ANGLE,
        needs=(Stage.RANKING, Stage.INSIGHTS),
        needs_previous_angle=True,
    ),
    StageDefinition(
        Stage.DIVERSITY,
        "Flag similar experiments across selected personas",
        Scope.ONCE,
        needs=(Stage.HYPOTHESIS,),
    ),
    StageDefinition(
        Stage.RESULT,
        "Assemble and verify the human-reviewable result",
        Scope.ONCE,
        needs=(Stage.RANKING, Stage.DIVERSITY),
    ),
)


class PipelineGraph:
    """Expand PIPELINE into concrete nodes without choosing how they execute."""

    def __init__(self) -> None:
        self.definitions = {definition.stage: definition for definition in PIPELINE}

    def nodes(self, outputs: dict[str, dict[str, Any]]) -> list[PipelineNode]:
        nodes = []
        for definition in PIPELINE:
            if definition.scope == Scope.PER_SELECTED_ANGLE and Stage.RANKING not in outputs:
                break

            for ref in self._references(definition, outputs):
                dependencies = [
                    dependency for stage in definition.needs for dependency in self._dependencies(stage, ref, outputs)
                ]
                if definition.needs_previous_angle and ref.variant:
                    dependencies.append(NodeRef(definition.stage, ref.entity_id, ref.variant - 1))

                nodes.append(PipelineNode(ref, tuple(dependencies)))

        return nodes

    @staticmethod
    def _references(definition: StageDefinition, outputs: dict[str, dict[str, Any]]) -> tuple[NodeRef, ...]:
        if definition.scope == Scope.PER_PERSONA:
            return tuple(NodeRef(definition.stage, persona.persona_id) for persona in PERSONAS)

        if definition.scope == Scope.PER_SELECTED_ANGLE:
            return tuple(
                NodeRef(definition.stage, row["persona_id"], slot)
                for row in outputs[Stage.RANKING]["top_3"]
                for slot in range(len(HypothesisAngle))
            )

        return (NodeRef(definition.stage),)

    def _dependencies(self, stage: Stage, target: NodeRef, outputs: dict[str, dict[str, Any]]) -> tuple[NodeRef, ...]:
        definition = self.definitions[stage]
        if definition.scope == Scope.PER_PERSONA:
            personas = (
                (target.entity_id,)
                if target.entity_id is not None
                else tuple(persona.persona_id for persona in PERSONAS)
            )
            return tuple(NodeRef(stage, persona_id) for persona_id in personas)

        if definition.scope == Scope.PER_SELECTED_ANGLE:
            return self._references(definition, outputs)

        return (NodeRef(stage),)
