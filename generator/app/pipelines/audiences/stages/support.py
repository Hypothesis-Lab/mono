from __future__ import annotations

import hashlib
import json
import re
from typing import Any

from pydantic import BaseModel

from app.pipelines.audiences.catalog import INTERVIEW_QUESTIONS, PERSONAS
from app.pipelines.audiences.models import Product, SyntheticPersona
from app.pipelines.audiences.protocols import LanguageModelClient

PIPELINE_VERSION = "5.0_pain_need_grounded"


class AudienceStageSupport:
    def __init__(self, model_client: LanguageModelClient) -> None:
        self.model_client = model_client
        self.personas = {persona.persona_id: persona for persona in PERSONAS}

    def _research_signature(self, product: Product, persona: SyntheticPersona) -> str:
        return self._fingerprint(
            {
                "pipeline": PIPELINE_VERSION,
                "product": self._product_for_research(product),
                "persona": persona,
                "questions": INTERVIEW_QUESTIONS,
            }
        )

    @staticmethod
    def _product_for_research(product: Product) -> dict[str, Any]:
        return product.model_dump(
            include={
                "brand",
                "name",
                "description",
                "category",
                "custom_category",
                "country",
                "town",
                "price",
                "facts",
                "key_fact_ids",
                "known_constraints",
                "unknowns",
            },
            mode="json",
        )

    @staticmethod
    def _prompt(instructions: str, **context: object) -> str:
        rendered = {
            key: value.model_dump(mode="json") if isinstance(value, BaseModel) else value
            for key, value in context.items()
        }
        return instructions.strip() + "\n\nCONTEXT:\n" + json.dumps(rendered, ensure_ascii=False, indent=2)

    @staticmethod
    def _fingerprint(value: object) -> str:
        if isinstance(value, BaseModel):
            value = value.model_dump(mode="json")

        return hashlib.sha256(json.dumps(value, ensure_ascii=False, sort_keys=True, default=str).encode()).hexdigest()

    @staticmethod
    def _norm(value: str) -> str:
        return re.sub(r"\s+", " ", value.lower()).strip()

    def _word_similarity(self, left: str, right: str) -> float:
        left_words = set(re.findall(r"[а-яёa-z]{4,}", self._norm(left)))
        right_words = set(re.findall(r"[а-яёa-z]{4,}", self._norm(right)))
        return len(left_words & right_words) / len(left_words | right_words) if left_words | right_words else 0.0

    @staticmethod
    def _nodes(progress: dict[str, Any]) -> dict[str, Any]:
        nodes = progress.setdefault("nodes", {})
        if not isinstance(nodes, dict):
            raise ValueError("job progress nodes must be an object")

        return nodes

    @staticmethod
    def _node_state(nodes: dict[str, Any], node_id: str) -> dict[str, Any]:
        state = nodes.get(node_id, {})
        if not isinstance(state, dict):
            raise ValueError(f"job progress node {node_id} must be an object")

        return state

    def _output(self, nodes: dict[str, Any], node_id: str) -> dict[str, Any]:
        state = self._node_state(nodes, node_id)
        output = state.get("output")
        if state.get("completed") is not True or not isinstance(output, dict):
            raise ValueError(f"workflow node {node_id} has no completed output")

        return output
