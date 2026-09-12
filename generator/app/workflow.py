from __future__ import annotations

import json
import logging
import re
from copy import deepcopy
from typing import Any, TypeVar

from pydantic import BaseModel

from app.catalog import INTERVIEW_QUESTIONS, PERSONA_SCORE_WEIGHTS, PERSONAS
from app.db import Job, JobRepository
from app.llm import LLMClient
from app.models import (
    HypothesisBatch,
    PersonaMetricScores,
    PersonaResearch,
    Product,
    SyntheticPersona,
)

logger = logging.getLogger(__name__)
ModelT = TypeVar("ModelT", bound=BaseModel)


class Workflow:
    def __init__(self, repository: JobRepository, llm: LLMClient) -> None:
        self.repository = repository
        self.llm = llm
        self.personas = {persona.persona_id: persona for persona in PERSONAS}

    def process_one_node(self, job: Job) -> None:
        product = Product.model_validate(job.input)
        progress = deepcopy(job.progress)
        node_id = self._next_node(progress)
        if node_id is None:
            self.repository.finish(job.id, self._result(product, progress))
            return

        nodes = self._nodes(progress)
        previous = self._node_state(nodes, node_id)
        retry_count = int(previous.get("retry_count", 0))
        try:
            output = self._run_node(
                product, progress, node_id, previous.get("last_error")
            )
        except Exception as error:
            nodes[node_id] = {
                "completed": False,
                "retry_count": retry_count + 1,
                "last_error": str(error)[:1000],
            }
            self.repository.save_progress(job.id, progress)
            logger.exception(
                "generation node failed",
                extra={"job_id": str(job.id), "node_id": node_id},
            )
            return

        nodes[node_id] = {
            "completed": True,
            "retry_count": retry_count,
            "output": output,
        }
        self.repository.save_progress(job.id, progress)

    def _next_node(self, progress: dict[str, Any]) -> str | None:
        nodes = self._nodes(progress)
        for persona in PERSONAS:
            node_id = f"eligibility:{persona.persona_id}"
            if not self._completed(nodes, node_id):
                return node_id

        for persona in PERSONAS:
            node_id = f"interview:{persona.persona_id}"
            if not self._completed(nodes, node_id):
                return node_id

        for persona in PERSONAS:
            node_id = f"score:{persona.persona_id}"
            if not self._completed(nodes, node_id):
                return node_id

        if not self._completed(nodes, "ranking"):
            return "ranking"

        ranking = self._output(nodes, "ranking")
        for item in ranking["top_3"]:
            node_id = f"hypotheses:{item['persona_id']}"
            if not self._completed(nodes, node_id):
                return node_id

        if not self._completed(nodes, "diversity"):
            return "diversity"
        return None

    def _run_node(
        self,
        product: Product,
        progress: dict[str, Any],
        node_id: str,
        previous_error: object,
    ) -> dict[str, Any]:
        kind, _, persona_id = node_id.partition(":")
        nodes = self._nodes(progress)
        retry_message = previous_error if isinstance(previous_error, str) else None

        if kind == "eligibility":
            return self._eligibility(product, self.personas[persona_id])
        if kind == "interview":
            return self._interview(product, self.personas[persona_id], retry_message)
        if kind == "score":
            interview = self._output(nodes, f"interview:{persona_id}")
            eligibility = self._output(nodes, f"eligibility:{persona_id}")
            return self._score(
                product,
                self.personas[persona_id],
                interview,
                eligibility,
                retry_message,
            )
        if kind == "ranking":
            return self._ranking(nodes)
        if kind == "hypotheses":
            interview = self._output(nodes, f"interview:{persona_id}")
            ranking = self._output(nodes, "ranking")
            ranking_item = next(
                item for item in ranking["top_3"] if item["persona_id"] == persona_id
            )
            return self._hypotheses(
                product,
                self.personas[persona_id],
                interview,
                ranking_item,
                retry_message,
            )
        if kind == "diversity":
            return {"warnings": self._diversity_warnings(nodes)}
        raise ValueError(f"unknown workflow node {node_id}")

    def _interview(
        self,
        product: Product,
        persona: SyntheticPersona,
        previous_error: str | None,
    ) -> dict[str, Any]:
        questions = [
            {"question_id": question_id, "text": text}
            for question_id, text in INTERVIEW_QUESTIONS
        ]
        prompt = self._prompt(
            """
You are Synthetic Persona Interviewer. Simulate answers from exactly the supplied persona.
Use only the persona and confirmed product facts. Do not invent biographical facts, product
 features, rates, terms, or approval guarantees. Write in Russian and return only JSON.
 Answer every question exactly once and in the supplied order. Then summarize the findings.
 Return exactly 10 interview_answers. Include 2 to 5 items each for problems, pains, needs,
 desires, and decision_factors; include 1 to 4 items each for objections, decision_triggers,
 and relevant_use_cases.
""",
            product=product,
            persona=persona,
            questions=questions,
            output_fields=list(PersonaResearch.model_fields),
        )
        research = self._model_output(PersonaResearch, prompt, previous_error)
        if research.persona_id != persona.persona_id:
            raise ValueError(
                f"expected persona_id {persona.persona_id}, got {research.persona_id}"
            )
        question_ids = [answer.question_id for answer in research.interview_answers]
        expected_ids = [question_id for question_id, _ in INTERVIEW_QUESTIONS]
        if question_ids != expected_ids:
            raise ValueError(
                f"expected interview question ids {expected_ids}, got {question_ids}"
            )
        return research.model_dump(mode="json")

    def _score(
        self,
        product: Product,
        persona: SyntheticPersona,
        interview: dict[str, Any],
        eligibility: dict[str, Any],
        previous_error: str | None,
    ) -> dict[str, Any]:
        prompt = self._prompt(
            """
You are Persona-Product Critic. Score exactly one persona for exactly one product from 0 to 100.
Do not compare personas or imitate credit approval. Gender, age, profession, and income cannot
be grounds for approval. Use only confirmed product facts. Write in Russian and return only JSON.
""",
            product=product,
            persona=persona,
            interview=interview,
            eligibility=eligibility,
            output_fields=list(PersonaMetricScores.model_fields),
        )
        scores = self._model_output(PersonaMetricScores, prompt, previous_error)
        if scores.persona_id != persona.persona_id:
            raise ValueError(
                f"expected persona_id {persona.persona_id}, got {scores.persona_id}"
            )
        return scores.model_dump(mode="json")

    def _hypotheses(
        self,
        product: Product,
        persona: SyntheticPersona,
        interview: dict[str, Any],
        ranking_item: dict[str, Any],
        previous_error: str | None,
    ) -> dict[str, Any]:
        expected_ids = [
            f"{persona.persona_id}_hyp_{number:02d}" for number in range(1, 4)
        ]
        prompt = self._prompt(
            """
You are Marketing Hypothesis Generator. Create exactly three different testable marketing
hypotheses for the supplied persona and product. Every hypothesis must cite confirmed fact IDs,
use an allowed channel, include a measurable primary metric, and use an if-then-because statement.
Do not create unsupported product claims, numbers, approval guarantees, or prohibited claims.
Write in Russian and return only JSON with a hypotheses array.
""",
            product=product,
            persona=persona,
            interview=interview,
            ranking=ranking_item,
            expected_hypothesis_ids=expected_ids,
            output_fields=list(HypothesisBatch.model_fields),
        )
        batch = self._model_output(HypothesisBatch, prompt, previous_error)
        self._validate_hypotheses(product, persona, batch, expected_ids)
        return batch.model_dump(mode="json")

    def _model_output(
        self,
        model: type[ModelT],
        prompt: str,
        previous_error: str | None,
    ) -> ModelT:
        retry_instruction = ""
        if previous_error:
            retry_instruction = (
                "\n\nYour previous output was invalid. Return the full JSON object again and fix this "
                f"validation error: {previous_error[:1000]}"
            )
        schema = self._strict_schema(model.model_json_schema())
        schema_instruction = (
            "\n\nReturn a JSON object that conforms exactly to this schema, including all array length limits:\n"
            + json.dumps(schema, ensure_ascii=False)
        )
        response = self.llm.complete(
            "Return a single valid JSON object without Markdown or explanation.",
            prompt + retry_instruction + schema_instruction,
            schema,
        )
        return model.model_validate(self._parse_json(response))

    def _eligibility(
        self, product: Product, persona: SyntheticPersona
    ) -> dict[str, Any]:
        checks = []
        persona_data = persona.model_dump()
        for rule in product.eligibility_rules:
            if rule.field not in persona_data:
                raise ValueError(
                    f"eligibility rule {rule.rule_id} references unknown field {rule.field}"
                )
            checks.append(
                {
                    "rule_id": rule.rule_id,
                    "description": rule.description,
                    "passed": self._matches_rule(
                        persona_data[rule.field], rule.operator, rule.value
                    ),
                }
            )
        return {
            "persona_id": persona.persona_id,
            "hard_eligible": all(check["passed"] for check in checks),
            "has_required_card_now": persona.has_cifra_debit_or_salary_card,
            "checks": checks,
        }

    @staticmethod
    def _matches_rule(actual: object, operator: str, expected: object) -> bool:
        if operator == "eq":
            return actual == expected
        if operator == "in":
            return isinstance(expected, list) and actual in expected
        if operator == "gte":
            return (
                isinstance(actual, int)
                and isinstance(expected, int)
                and actual >= expected
            )
        if operator == "lte":
            return (
                isinstance(actual, int)
                and isinstance(expected, int)
                and actual <= expected
            )
        if operator == "between":
            return (
                isinstance(actual, int)
                and isinstance(expected, list)
                and len(expected) == 2
                and all(isinstance(value, int) for value in expected)
                and expected[0] <= actual <= expected[1]
            )
        if operator == "truthy":
            return bool(actual) is bool(expected)
        raise ValueError(f"unknown eligibility operator {operator}")

    def _ranking(self, nodes: dict[str, Any]) -> dict[str, Any]:
        rows = []
        for persona in PERSONAS:
            scores = self._output(nodes, f"score:{persona.persona_id}")
            eligibility = self._output(nodes, f"eligibility:{persona.persona_id}")
            final_score = round(
                sum(
                    float(scores[metric]) * weight
                    for metric, weight in PERSONA_SCORE_WEIGHTS.items()
                ),
                2,
            )
            rows.append(
                {
                    "persona_id": persona.persona_id,
                    "label": persona.label,
                    "hard_eligible": eligibility["hard_eligible"],
                    "has_required_card_now": eligibility["has_required_card_now"],
                    "metrics": scores,
                    "final_score": final_score,
                }
            )
        rows.sort(
            key=lambda item: (
                not bool(item["hard_eligible"]),
                -float(item["final_score"]),
                str(item["persona_id"]),
            )
        )
        eligible_rank = 0
        for row in rows:
            if row["hard_eligible"]:
                eligible_rank += 1
                row["rank"] = eligible_rank
            else:
                row["rank"] = None
        top_3 = [row for row in rows if row["hard_eligible"]][:3]
        if len(top_3) != 3:
            raise ValueError(
                f"expected at least three eligible personas, got {len(top_3)}"
            )
        return {"ranking": rows, "top_3": top_3}

    def _validate_hypotheses(
        self,
        product: Product,
        persona: SyntheticPersona,
        batch: HypothesisBatch,
        expected_ids: list[str],
    ) -> None:
        hypotheses = batch.hypotheses
        if [hypothesis.hypothesis_id for hypothesis in hypotheses] != expected_ids:
            raise ValueError(f"expected hypothesis ids {expected_ids}")
        if any(
            hypothesis.persona_id != persona.persona_id for hypothesis in hypotheses
        ):
            raise ValueError("hypothesis batch contains another persona")
        if len({hypothesis.value_angle for hypothesis in hypotheses}) != 3:
            raise ValueError("hypothesis value angles must differ")
        if len({hypothesis.headline.lower().strip() for hypothesis in hypotheses}) != 3:
            raise ValueError("hypothesis headlines must differ")

        allowed_fact_ids = {fact.fact_id for fact in product.facts}
        allowed_channels = set(product.allowed_channels)
        allowed_numbers = {
            self._compact_number(number)
            for fact in product.facts
            for number in self._numbers(fact.text)
        }
        for hypothesis in hypotheses:
            if not set(hypothesis.selected_fact_ids).issubset(allowed_fact_ids):
                raise ValueError(
                    f"{hypothesis.hypothesis_id} references unknown fact IDs"
                )
            if hypothesis.channel not in allowed_channels:
                raise ValueError(
                    f"{hypothesis.hypothesis_id} uses an unavailable channel"
                )
            text = self._hypothesis_text(hypothesis).lower()
            if any(claim.lower() in text for claim in product.forbidden_claims):
                raise ValueError(
                    f"{hypothesis.hypothesis_id} contains a forbidden claim"
                )
            if "потому что" not in hypothesis.hypothesis_statement.lower():
                raise ValueError(f"{hypothesis.hypothesis_id} lacks a because clause")
            used_numbers = {
                self._compact_number(number) for number in self._numbers(text)
            }
            if used_numbers - allowed_numbers:
                raise ValueError(
                    f"{hypothesis.hypothesis_id} contains unsupported numbers"
                )

    def _diversity_warnings(self, nodes: dict[str, Any]) -> list[str]:
        ranking = self._output(nodes, "ranking")
        hypotheses = [
            hypothesis
            for item in ranking["top_3"]
            for hypothesis in self._output(nodes, f"hypotheses:{item['persona_id']}")[
                "hypotheses"
            ]
        ]
        warnings = []
        for index, left in enumerate(hypotheses):
            for right in hypotheses[index + 1 :]:
                left_tokens = self._tokens(left)
                right_tokens = self._tokens(right)
                union = left_tokens | right_tokens
                similarity = (
                    len(left_tokens & right_tokens) / len(union) if union else 0.0
                )
                if similarity >= 0.8:
                    warnings.append(
                        f"{left['hypothesis_id']} and {right['hypothesis_id']} may duplicate meaning: {similarity:.2f}"
                    )
        return warnings

    def _result(
        self, product: Product, progress: dict[str, Any]
    ) -> dict[str, Any]:
        nodes = self._nodes(progress)
        ranking = self._output(nodes, "ranking")
        top_3 = ranking["top_3"]
        return {
            "pipeline_version": 1,
            "synthetic_data_warning": "Results use synthetic personas and require validation with real customers.",
            "product": product.model_dump(mode="json"),
            "personas": [persona.model_dump(mode="json") for persona in PERSONAS],
            "persona_research": [
                self._output(nodes, f"interview:{persona.persona_id}")
                for persona in PERSONAS
            ],
            "eligibility": {
                persona.persona_id: self._output(
                    nodes, f"eligibility:{persona.persona_id}"
                )
                for persona in PERSONAS
            },
            "ranking": ranking["ranking"],
            "top_3": top_3,
            "hypotheses_by_persona": {
                item["persona_id"]: self._output(
                    nodes, f"hypotheses:{item['persona_id']}"
                )["hypotheses"]
                for item in top_3
            },
            "diversity_warnings": self._output(nodes, "diversity")["warnings"],
        }

    @staticmethod
    def _prompt(instructions: str, **context: object) -> str:
        rendered = {
            key: value.model_dump(mode="json")
            if isinstance(value, BaseModel)
            else value
            for key, value in context.items()
        }
        return f"{instructions.strip()}\n\nCONTEXT:\n{json.dumps(rendered, ensure_ascii=False, indent=2)}"

    @staticmethod
    def _parse_json(content: str) -> dict[str, Any]:
        cleaned = (
            content.strip()
            .removeprefix("```json")
            .removeprefix("```")
            .removesuffix("```")
            .strip()
        )
        try:
            value = json.loads(cleaned)
        except json.JSONDecodeError as error:
            raise ValueError(f"model output was not valid JSON: {error.msg}") from error
        if not isinstance(value, dict):
            raise ValueError("model output JSON must be an object")
        return value

    @staticmethod
    def _strict_schema(schema: dict[str, Any]) -> dict[str, Any]:
        values: list[object] = [schema]
        while values:
            value = values.pop()
            if isinstance(value, dict):
                if "properties" in value:
                    value["additionalProperties"] = False
                values.extend(value.values())
            elif isinstance(value, list):
                values.extend(value)
        return schema

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

    def _completed(self, nodes: dict[str, Any], node_id: str) -> bool:
        return self._node_state(nodes, node_id).get("completed") is True

    def _output(self, nodes: dict[str, Any], node_id: str) -> dict[str, Any]:
        state = self._node_state(nodes, node_id)
        output = state.get("output")
        if state.get("completed") is not True or not isinstance(output, dict):
            raise ValueError(f"workflow node {node_id} has no completed output")
        return output

    @staticmethod
    def _numbers(text: str) -> set[str]:
        return set(re.findall(r"(?<![A-Za-zА-Яа-я_])\d[\d\s]*(?:[.,]\d+)?\s*%?", text))

    @staticmethod
    def _compact_number(value: str) -> str:
        return re.sub(r"\s+", "", value).replace(",", ".")

    @staticmethod
    def _hypothesis_text(hypothesis: Any) -> str:
        fields = [
            "hypothesis_statement",
            "headline",
            "message_frame",
            "cta",
            "experiment_design",
            "why_for_persona",
        ]
        return " ".join(str(getattr(hypothesis, field)) for field in fields)

    def _tokens(self, hypothesis: dict[str, Any]) -> set[str]:
        fields = (
            "hypothesis_statement",
            "headline",
            "message_frame",
            "why_for_persona",
        )
        text = " ".join(str(hypothesis[field]) for field in fields).lower()
        return {
            token
            for token in re.findall(r"[a-zа-яё]{4,}", text)
            if token not in {"если", "потому", "который", "которая", "через", "этого"}
        }
