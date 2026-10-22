from __future__ import annotations

import json
from pathlib import Path
from typing import Any, Literal, TypedDict

from langgraph.graph import END, START, StateGraph
from langgraph.types import RetryPolicy

from .client import ModelClient
from .models import (
    HypothesisCard,
    HypothesisClaim,
    HypothesisDraft,
    HypothesisDraftBatch,
    HypothesisExcludedCandidate,
    HypothesisFactor,
    HypothesisInputRef,
    HypothesisPipelineResult,
    HypothesisPriority,
    HypothesisRanking,
    HypothesisRankingItem,
    HypothesisValidation,
)

# Prompt text intentionally contains long natural-language instruction lines.
# ruff: noqa: E501

# The product and audience pipelines deliberately keep their prompts in the
# eval package. This module is different: the supplied module specification is
# the source document for this pipeline, so keep it as the prompt authority.
MODULE_PROMPT = Path(__file__).resolve().parents[2] / "input_prompts" / "Промпт_модуля_гипотез.md"


class HypothesisWorkflowState(TypedDict, total=False):
    pipeline: str
    model: str
    request_id: str
    target_product: str
    product_passport: dict[str, Any]
    audience_result: dict[str, Any]
    product_result_ref: str
    audience_result_ref: str
    candidate_count: int
    validation_target: int
    candidates: list[dict[str, Any]]
    candidate_ids: list[str]
    generation_missing_data: list[str]
    generation_limitations: list[str]
    input_errors: list[str]
    input_limitations: list[str]
    ranking: dict[str, Any]
    ordered_ids: list[str]
    validation_attempts: list[dict[str, Any]]
    accepted_candidate_ids: list[str]
    validation_cursor: int
    result: dict[str, Any]


class HypothesisWorkflow:
    """Generate, rank, and selectively validate product hypotheses."""

    def __init__(
        self,
        client: ModelClient,
        *,
        candidate_count: int = 5,
        validation_target: int = 3,
    ) -> None:
        if candidate_count < 1:
            raise ValueError("candidate_count must be at least 1")
        if validation_target < 1:
            raise ValueError("validation_target must be at least 1")
        self.client = client
        self.candidate_count = candidate_count
        self.validation_target = validation_target

    def compile(self, checkpointer: Any):
        retry_policy = RetryPolicy(max_attempts=1)
        builder = StateGraph(HypothesisWorkflowState)
        builder.add_node("validate_input", self.validate_input)
        builder.add_node("generate", self.generate, retry_policy=retry_policy)
        builder.add_node("rank", self.rank, retry_policy=retry_policy)
        builder.add_node("validate_next", self.validate_next, retry_policy=retry_policy)
        builder.add_node("finalize", self.finalize)
        builder.add_edge(START, "validate_input")
        builder.add_conditional_edges("validate_input", self.route_input, ["generate", "finalize"])
        builder.add_edge("generate", "rank")
        builder.add_edge("rank", "validate_next")
        builder.add_conditional_edges(
            "validate_next",
            self.route_validation,
            ["validate_next", "finalize"],
        )
        builder.add_edge("finalize", END)
        return builder.compile(checkpointer=checkpointer)

    def validate_input(self, state: HypothesisWorkflowState) -> dict[str, Any]:
        product = state.get("product_passport", {})
        audience = state.get("audience_result", {}).get("audience", state.get("audience_result", {}))
        errors: list[str] = []
        limitations: list[str] = []
        if not product.get("records"):
            errors.append("В product result нет записей паспорта продукта.")
        if not audience.get("segments"):
            errors.append("В audience result нет переданного кандидата аудитории.")
        if product.get("extraction_status") != "complete":
            limitations.append("Паспорт продукта имеет статус не complete.")
        if audience.get("status") != "complete":
            limitations.append("Результат аудитории имеет статус не complete.")
        return {"input_errors": errors, "input_limitations": limitations}

    def route_input(self, state: HypothesisWorkflowState) -> Literal["generate", "finalize"]:
        return "finalize" if state.get("input_errors") else "generate"

    async def generate(self, state: HypothesisWorkflowState) -> dict[str, Any]:
        prompt = _generation_prompt(state)
        batch = await self.client.complete_structured("hypothesis_generate", prompt, HypothesisDraftBatch)
        candidates: list[HypothesisDraft] = []
        for index, candidate in enumerate(batch.candidates[: self.candidate_count], start=1):
            candidates.append(candidate.model_copy(update={"candidate_id": f"hyp_{index:03d}"}))
        return {
            "candidates": [candidate.model_dump(mode="json") for candidate in candidates],
            "candidate_ids": [candidate.candidate_id for candidate in candidates],
            "generation_missing_data": batch.missing_data,
            "generation_limitations": batch.limitations,
            "validation_cursor": 0,
            "validation_attempts": [],
            "accepted_candidate_ids": [],
        }

    async def rank(self, state: HypothesisWorkflowState) -> dict[str, Any]:
        candidates = [HypothesisDraft.model_validate(item) for item in state.get("candidates", [])]
        if not candidates:
            return {
                "ranking": {
                    "ordered": [],
                    "method": "qualitative",
                    "provisional": True,
                    "rationale": "Гипотезы не были сформированы.",
                    "missing_data": [],
                },
                "ordered_ids": [],
            }
        prompt = _ranking_prompt(state, candidates)
        ranking = await self.client.complete_structured("hypothesis_rank", prompt, HypothesisRanking)
        by_id = {candidate.candidate_id: candidate for candidate in candidates}
        ordered: list[str] = []
        ranking_items: dict[str, Any] = {}
        for item in ranking.ordered:
            if item.candidate_id in by_id and item.candidate_id not in ordered:
                ordered.append(item.candidate_id)
                ranking_items[item.candidate_id] = item
        for candidate in candidates:
            if candidate.candidate_id not in ordered:
                ordered.append(candidate.candidate_id)
        normalized = []
        for rank, candidate_id in enumerate(ordered, start=1):
            item = ranking_items.get(candidate_id)
            if item is None:
                item = _fallback_priority(candidate_id, rank)
            priority = item.priority.model_copy(update={"rank": rank})
            normalized.append({"candidate_id": candidate_id, "priority": priority.model_dump(mode="json")})
        ranking_payload = {
            "ordered": normalized,
            "method": ranking.method,
            "provisional": ranking.provisional,
            "rationale": ranking.rationale,
            "missing_data": ranking.missing_data,
        }
        return {"ranking": ranking_payload, "ordered_ids": ordered}

    async def validate_next(self, state: HypothesisWorkflowState) -> dict[str, Any]:
        ordered_ids = state.get("ordered_ids", [])
        cursor = state.get("validation_cursor", 0)
        if cursor >= len(ordered_ids):
            return {}
        candidate_id = ordered_ids[cursor]
        candidates = {
            item["candidate_id"]: HypothesisDraft.model_validate(item) for item in state.get("candidates", [])
        }
        candidate = candidates[candidate_id]
        prompt = _validation_prompt(state, candidate)
        validation = await self.client.complete_structured(
            f"hypothesis_validate_{candidate_id}", prompt, HypothesisValidation
        )
        validation = validation.model_copy(update={"candidate_id": candidate_id})
        attempts = [*state.get("validation_attempts", []), validation.model_dump(mode="json")]
        accepted = list(state.get("accepted_candidate_ids", []))
        if validation.overall_status != "fail":
            accepted.append(candidate_id)
        return {
            "validation_attempts": attempts,
            "accepted_candidate_ids": accepted,
            "validation_cursor": cursor + 1,
        }

    def route_validation(self, state: HypothesisWorkflowState) -> Literal["validate_next", "finalize"]:
        if len(state.get("accepted_candidate_ids", [])) >= state.get("validation_target", self.validation_target):
            return "finalize"
        if state.get("validation_cursor", 0) >= len(state.get("ordered_ids", [])):
            return "finalize"
        return "validate_next"

    def finalize(self, state: HypothesisWorkflowState) -> dict[str, Any]:
        candidates = [HypothesisDraft.model_validate(item) for item in state.get("candidates", [])]
        ranking = state.get("ranking", {})
        priorities = {
            item["candidate_id"]: HypothesisPriority.model_validate(item["priority"])
            for item in ranking.get("ordered", [])
        }
        attempts = [HypothesisValidation.model_validate(item) for item in state.get("validation_attempts", [])]
        attempt_by_id = {item.candidate_id: item for item in attempts}
        accepted = set(state.get("accepted_candidate_ids", []))
        cards: list[HypothesisCard] = []
        excluded: list[HypothesisExcludedCandidate] = []
        for index, candidate in enumerate(candidates, start=1):
            priority = priorities.get(candidate.candidate_id, _fallback_priority(candidate.candidate_id, index))
            validation = attempt_by_id.get(candidate.candidate_id)
            if validation is None:
                selection_status = "backup"
                validation_status = "not_attempted"
                checks = []
                issues: list[str] = []
            else:
                validation_status = validation.overall_status
                checks = validation.checks
                issues = validation.blocking_issues
                selection_status = "selected" if candidate.candidate_id in accepted else "rejected_validation"
                if validation.overall_status == "fail":
                    excluded.append(
                        HypothesisExcludedCandidate(
                            title=candidate.title,
                            reason="; ".join(validation.blocking_issues) or "Гипотеза не прошла проверку.",
                            source_refs=candidate.source_refs,
                        )
                    )
            cards.append(
                HypothesisCard(
                    **candidate.model_dump(mode="python"),
                    hypothesis_id=None,
                    hypothesis_version=None,
                    checks=checks,
                    priority=priority,
                    selection_status=selection_status,
                    validation_status=validation_status,
                    validation_issues=issues,
                )
            )
        cards.sort(key=lambda card: card.priority.rank)
        product = state.get("product_passport", {})
        audience = state.get("audience_result", {})
        audience_payload = audience.get("audience", audience)
        selected_ids = [candidate_id for candidate_id in state.get("ordered_ids", []) if candidate_id in accepted]
        status = _result_status(state, selected_ids, attempts)
        result = HypothesisPipelineResult(
            schema_version="0.1",
            request_id=state.get("request_id", "hypothesis-run"),
            workspace_id=None,
            product_id=state.get("target_product", product.get("product_name") or "unknown"),
            status=status,
            input_refs=[
                HypothesisInputRef(
                    artifact_type="product_result",
                    artifact_id=state.get("product_result_ref", "product-result"),
                    version=None,
                    source_refs=sorted(
                        {
                            str(evidence.get("source"))
                            for item in product.get("records", [])
                            for evidence in item.get("evidence", [])
                            if evidence.get("source")
                        }
                    ),
                ),
                HypothesisInputRef(
                    artifact_type="audience_result",
                    artifact_id=state.get("audience_result_ref", "audience-result"),
                    version="0.1",
                    source_refs=[
                        str(item) for item in audience_payload.get("data_profile", {}).get("snapshot_refs", [])
                    ],
                ),
            ],
            context_summary=_context_summary(state),
            hypotheses=cards,
            excluded_candidates=excluded,
            conflicts=[str(item.get("description", item)) for item in product.get("conflicts", [])],
            missing_data=_missing_data(state, audience_payload),
            questions=_questions(state, attempts),
            limitations=[
                item
                for item in [
                    *state.get("input_limitations", []),
                    *state.get("generation_limitations", []),
                    *ranking.get("missing_data", []),
                    *audience_payload.get("limitations", []),
                ]
                if item
            ],
            review_status="pending",
            human_decision="pending",
            validation_attempts=attempts,
            selected_candidate_ids=selected_ids,
        )
        return {"result": result.model_dump(mode="json")}


def _module_text() -> str:
    try:
        return MODULE_PROMPT.read_text(encoding="utf-8")
    except OSError:
        return "Используй строгий режим доказательности, сохранение неизвестности и проверяемые гипотезы."


def _generation_prompt(state: HypothesisWorkflowState) -> str:
    product_context, audience_context = _prompt_context(state)
    return f"""{_module_text()}

Это первая стадия отдельного evaluation-пайплайна. Сгенерируй ровно {state.get("candidate_count", 5)} содержательно различных кандидатов.
На этой стадии НЕ ранжируй и НЕ проводи детальную проверку: поле кандидата `source_refs` должно ссылаться только на переданные материалы, а неизвестные значения сохраняй как unknown. Не выдумывай результаты экспериментов.
Заполни все поля `HypothesisDraft`; priority и checks не возвращай, потому что они появятся на следующих стадиях.
Все текстовые значения — на русском. Верни только JSON по схеме `HypothesisDraftBatch`.

ЦЕЛЕВОЙ ПРОДУКТ:
{state.get("target_product")}

ПАСПОРТ ПРОДУКТА:
{json.dumps(product_context, ensure_ascii=False, indent=2)}

РЕЗУЛЬТАТ АУДИТОРИИ:
{json.dumps(audience_context, ensure_ascii=False, indent=2)}
"""


def _ranking_prompt(state: HypothesisWorkflowState, candidates: list[HypothesisDraft]) -> str:
    return f"""Ты — отдельный агент ранжирования гипотез.

Сначала рассмотри ВСЕ кандидаты ниже. Верни их идентификаторы в полном порядке от наиболее приоритетного к наименее приоритетному, без пропусков и дублей. Используй качественное сравнение: evidence, business_value, feasibility, novelty, measurability, risk. Не подменяй неизвестность низкой оценкой. Для каждого кандидата дай краткое rationale, объясняющее его место в порядке. Это обоснование приоритета, а не внутренний ход рассуждений.

На этой стадии не проводи детальную валидацию и не меняй содержание кандидатов. Верни только JSON по схеме `HypothesisRanking`.

ПОЛИТИКА ИЗ МОДУЛЯ:
{_module_text()}

КАНДИДАТЫ:
{json.dumps([candidate.model_dump(mode="json") for candidate in candidates], ensure_ascii=False, indent=2)}
"""


def _validation_prompt(state: HypothesisWorkflowState, candidate: HypothesisDraft) -> str:
    product_context, audience_context = _prompt_context(state)
    return f"""Ты — валидатор одной продуктовой гипотезы после её ранжирования.

Проверь только переданными материалами: логику перехода от изменения к поведению, опору на источники, соответствие аудитории, ограничения продукта/бренда/права, тестируемость и измеримость. Для неизвестных внешних фактов ставь unknown, а не fail. Fail используй только для конкретного нарушения или противоречия. Не симулируй синтетических персон, человеческое подтверждение или реальный эксперимент: если такой результат не передан, укажи unknown и limitation.

Верни проверки по восьми именам из схемы: logic, evidence, audience, constraints, testability, synthetic, human_confirmation, real_experiment. Для synthetic, human_confirmation и real_experiment ставь unknown, если соответствующий результат не был передан. `overall_status=fail` только если есть blocking issue, который нельзя снять уточнением; unknown допускается для кандидата и позволяет продвинуть его с ограничениями. Верни только JSON по схеме `HypothesisValidation`.

ПОЛИТИКА ИЗ МОДУЛЯ:
{_module_text()}

ПАСПОРТ ПРОДУКТА:
{json.dumps(product_context, ensure_ascii=False, indent=2)}

РЕЗУЛЬТАТ АУДИТОРИИ:
{json.dumps(audience_context, ensure_ascii=False, indent=2)}

КАНДИДАТ:
{json.dumps(candidate.model_dump(mode="json"), ensure_ascii=False, indent=2)}
"""


def _fallback_priority(candidate_id: str, rank: int) -> HypothesisRankingItem:
    factors = [
        HypothesisFactor(
            name=name, rating="unknown", score=None, rationale="Рейтинг не был передан моделью.", source_refs=[]
        )
        for name in ("evidence", "business_value", "feasibility", "novelty", "measurability", "risk")
    ]
    return HypothesisRankingItem(
        candidate_id=candidate_id,
        priority=HypothesisPriority(
            rank=rank,
            method="qualitative-fallback",
            provisional=True,
            total_score=None,
            factors=factors,
            rationale="Порядок использован как стабильный fallback из-за неполного ответа ранжировщика.",
        ),
    )


def _prompt_context(state: HypothesisWorkflowState) -> tuple[dict[str, Any], dict[str, Any]]:
    """Keep repeated validation prompts focused on the two pipeline artifacts."""

    passport = state.get("product_passport", {})
    product_context = {
        "product_name": passport.get("product_name"),
        "extraction_status": passport.get("extraction_status"),
        "records": passport.get("records", []),
        "conflicts": passport.get("conflicts", []),
        "gaps": passport.get("gaps", []),
        "warnings": passport.get("warnings", []),
    }
    audience_result = state.get("audience_result", {})
    audience_context = {
        "audience": audience_result.get("audience", audience_result),
        "calculation": audience_result.get("calculation"),
        "review": audience_result.get("review"),
        "query": audience_result.get("query"),
    }
    return product_context, audience_context


def _context_summary(state: HypothesisWorkflowState) -> dict[str, Any]:
    product = state.get("product_passport", {})
    records = product.get("records", [])
    product_name = product.get("product_name") or state.get("target_product")
    purpose = next((item for item in records if item.get("category") in {"purpose", "jtbd"}), None)
    audience = state.get("audience_result", {}).get("audience", state.get("audience_result", {}))
    segments = audience.get("segments", [])
    constraints = [
        HypothesisClaim(
            text=item.get("statement"),
            kind="fact",
            source_refs=[evidence.get("source", "") for evidence in item.get("evidence", [])],
            rationale="Ограничение передано из паспорта продукта.",
        )
        for item in records
        if item.get("category") in {"limitations", "terms", "communication_rules"}
    ]
    return {
        "product": HypothesisClaim(
            text=product_name,
            kind="fact" if product_name else "unknown",
            source_refs=[],
            rationale="Название взято из входного product result."
            if product_name
            else "Название продукта отсутствует.",
        ),
        "business_goal": HypothesisClaim(
            text=purpose.get("statement") if purpose else None,
            kind="fact" if purpose else "unknown",
            source_refs=[evidence.get("source", "") for evidence in (purpose or {}).get("evidence", [])],
            rationale="Бизнес-контекст ограничен переданными записями product result."
            if purpose
            else "Бизнес-цель не была передана.",
        ),
        "audience_ids": [str(segment.get("segment_id")) for segment in segments if segment.get("segment_id")],
        "constraints": constraints,
    }


def _missing_data(state: HypothesisWorkflowState, audience: dict[str, Any]) -> list[str]:
    missing = [item for item in [*state.get("input_errors", []), *state.get("generation_missing_data", [])] if item]
    if audience.get("status") != "complete":
        missing.append("Результат audience pipeline имеет статус не complete.")
    if not audience.get("segments"):
        missing.append("В audience result нет сохранённого сегмента.")
    return list(dict.fromkeys(missing))


def _questions(state: HypothesisWorkflowState, attempts: list[HypothesisValidation]) -> list[str]:
    questions = [item for validation in attempts for item in validation.limitations if item]
    return list(dict.fromkeys(questions))[:3]


def _result_status(
    state: HypothesisWorkflowState,
    selected_ids: list[str],
    attempts: list[HypothesisValidation],
) -> Literal["complete", "partial", "blocked"]:
    if not selected_ids:
        return "blocked"
    if len(selected_ids) < state.get("validation_target", 3):
        return "partial"
    if any(attempt.overall_status == "unknown" for attempt in attempts if attempt.candidate_id in selected_ids):
        return "partial"
    if state.get("product_passport", {}).get("extraction_status") != "complete":
        return "partial"
    audience = state.get("audience_result", {}).get("audience", state.get("audience_result", {}))
    if audience.get("status") != "complete":
        return "partial"
    return "complete"
