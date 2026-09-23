"""Node references and dependency edges shared by pipeline graphs."""

from dataclasses import dataclass


@dataclass(frozen=True)
class NodeRef:
    stage: str
    entity_id: str | None = None
    variant: int | None = None

    @property
    def id(self) -> str:
        if self.variant is not None:
            return f"{self.stage}:{self.entity_id}:{self.variant}"

        if self.entity_id is not None:
            return f"{self.stage}:{self.entity_id}"

        return str(self.stage)


@dataclass(frozen=True)
class PipelineNode:
    ref: NodeRef
    dependencies: tuple[NodeRef, ...]

    @property
    def id(self) -> str:
        return self.ref.id
