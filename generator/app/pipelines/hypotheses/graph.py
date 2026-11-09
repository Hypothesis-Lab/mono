from __future__ import annotations

import json
from pathlib import Path
from typing import Any, Literal, TypedDict

from langgraph.graph import END, START, StateGraph

from app.language_model_client.structured_generation import complete_with_repair
from app.pipelines.audiences.stages import AudienceStages
from app.pipelines.audiences.stages.execution import ModelCallContext
from app.pipelines.hypotheses.models import (
    Card,
    Claim,
    Draft,
    DraftBatch,
    ExcludedCandidate,
    Factor,
    HypothesisInput,
    InputRef,
    PipelineResult,
    Priority,
    Ranking,
    RankingItem,
    Validation,
)

# Prompt text intentionally contains long natural-language instruction lines.
# ruff: noqa: E501
MODULE_PROMPT = Path(__file__).resolve().parents[4] / "input_prompts" / "Промпт_модуля_гипотез.md"


class HypothesisState(TypedDict, total=False):
    input: dict[str, Any]
    errors: list[str]
    input_limitations: list[str]
    candidates: list[dict[str, Any]]
    ranking: dict[str, Any]
    ordered_ids: list[str]
    validations: list[dict[str, Any]]
    accepted: list[str]
    cursor: int
    generation_missing_data: list[str]
    generation_limitations: list[str]
    result: dict[str, Any]


def build_graph(stages: AudienceStages, *, checkpointer: Any = None, observer: Any = None):
    builder = StateGraph(HypothesisState)

    def validate_input(state: HypothesisState) -> dict[str, Any]:
        data = HypothesisInput.model_validate(state["input"])
        audience = data.audience_result.get("audience", data.audience_result)
        errors: list[str] = []
        limitations: list[str] = []
        if not data.product_passport.get("records"):
            errors.append("В product result нет записей паспорта продукта.")
        if not audience.get("segments"):
            errors.append("В audience result нет переданного кандидата аудитории.")
        if data.product_passport.get("extraction_status") != "complete":
            limitations.append("Паспорт продукта имеет статус не complete.")
        if audience.get("status") != "complete":
            limitations.append("Результат аудитории имеет статус не complete.")
        return {"errors": errors, "input_limitations": limitations}

    def route_input(state: HypothesisState) -> Literal["generate", "finalize"]:
        return "finalize" if state.get("errors") else "generate"

    def generate(state: HypothesisState) -> dict[str, Any]:
        data = HypothesisInput.model_validate(state["input"])
        product_context, audience_context = _prompt_context(data)
        prompt = f"""{_module_text()}

Это первая стадия отдельного evaluation-пайплайна. Сгенерируй ровно {data.candidate_count} содержательно различных кандидатов.
На этой стадии НЕ ранжируй и НЕ проводи детальную проверку: поле кандидата `source_refs` должно ссылаться только на переданные материалы, а неизвестные значения сохраняй как unknown. Не выдумывай результаты экспериментов.
Заполни все поля `HypothesisDraft`; priority и checks не возвращай, потому что они появятся на следующих стадиях.
Все текстовые значения — на русском. Верни только JSON по схеме `HypothesisDraftBatch`.

ЦЕЛЕВОЙ ПРОДУКТ:
{product_context.get("product_name")}

ПАСПОРТ ПРОДУКТА:
{json.dumps(product_context, ensure_ascii=False, indent=2)}

РЕЗУЛЬТАТ АУДИТОРИИ:
{json.dumps(audience_context, ensure_ascii=False, indent=2)}
"""
        batch = complete_with_repair(
            stages.model_client,
            ModelCallContext(),
            DraftBatch,
            prompt,
            None,
            validator=lambda value: _validate_segment_refs(value, data),
        )
        candidates = [
            candidate.model_copy(update={"candidate_id": f"hyp_{index:03d}"}).model_dump(mode="json")
            for index, candidate in enumerate(batch.candidates[: data.candidate_count], 1)
        ]
        return {
            "candidates": candidates,
            "generation_missing_data": batch.missing_data,
            "generation_limitations": batch.limitations,
            "validations": [],
            "accepted": [],
            "cursor": 0,
        }

    def rank(state: HypothesisState) -> dict[str, Any]:
        candidates = [Draft.model_validate(item) for item in state.get("candidates", [])]
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

        prompt = f"""Ты — отдельный агент ранжирования гипотез.

Сначала рассмотри ВСЕ кандидаты ниже. Верни их идентификаторы в полном порядке от наиболее приоритетного к наименее приоритетному, без пропусков и дублей. Используй качественное сравнение: evidence, business_value, feasibility, novelty, measurability, risk. Не подменяй неизвестность низкой оценкой. Для каждого кандидата дай краткое rationale, объясняющее его место в порядке. Это обоснование приоритета, а не внутренний ход рассуждений.

На этой стадии не проводи детальную валидацию и не меняй содержание кандидатов. Верни только JSON по схеме `HypothesisRanking`.

ПОЛИТИКА ИЗ МОДУЛЯ:
{_module_text()}

КАНДИДАТЫ:
{json.dumps([candidate.model_dump(mode="json") for candidate in candidates], ensure_ascii=False, indent=2)}
"""
        ranking = complete_with_repair(stages.model_client, ModelCallContext(), Ranking, prompt, None)
        by_id = {candidate.candidate_id: candidate for candidate in candidates}
        ordered: list[str] = []
        ranking_items: dict[str, RankingItem] = {}
        for item in ranking.ordered:
            if item.candidate_id in by_id and item.candidate_id not in ordered:
                ordered.append(item.candidate_id)
                ranking_items[item.candidate_id] = item
        for candidate in candidates:
            if candidate.candidate_id not in ordered:
                ordered.append(candidate.candidate_id)

        normalized = []
        for rank_number, candidate_id in enumerate(ordered, 1):
            item = ranking_items.get(candidate_id) or _fallback_priority(candidate_id, rank_number)
            priority = item.priority.model_copy(update={"rank": rank_number})
            normalized.append({"candidate_id": candidate_id, "priority": priority.model_dump(mode="json")})

        ranking_payload = ranking.model_dump(mode="json")
        ranking_payload["ordered"] = normalized
        return {"ranking": ranking_payload, "ordered_ids": ordered}

    def validate_next(state: HypothesisState) -> dict[str, Any]:
        ordered_ids = state.get("ordered_ids", [])
        cursor = state.get("cursor", 0)
        candidate_id = ordered_ids[cursor]
        candidate = next(item for item in state.get("candidates", []) if item["candidate_id"] == candidate_id)
        data = HypothesisInput.model_validate(state["input"])
        product_context, audience_context = _prompt_context(data)
        prompt = f"""Ты — валидатор одной продуктовой гипотезы после её ранжирования.

Проверь только переданными материалами: логику перехода от изменения к поведению, опору на источники, соответствие аудитории, ограничения продукта/бренда/права, тестируемость и измеримость. Для неизвестных внешних фактов ставь unknown, а не fail. Fail используй только для конкретного нарушения или противоречия. Не симулируй синтетических персон, человеческое подтверждение или реальный эксперимент: если такой результат не передан, укажи unknown и limitation.

Верни проверки по восьми именам из схемы: logic, evidence, audience, constraints, testability, synthetic, human_confirmation, real_experiment. Для synthetic, human_confirmation и real_experiment ставь unknown, если соответствующий результат не был передан. `overall_status=fail` только если есть blocking issue, который нельзя снять уточнением; unknown допускается для кандидата и позволяет продвинуть его с ограничениями. Верни только JSON по схеме `HypothesisValidation`.

ПОЛИТИКА ИЗ МОДУЛЯ:
{_module_text()}

ПАСПОРТ ПРОДУКТА:
{json.dumps(product_context, ensure_ascii=False, indent=2)}

РЕЗУЛЬТАТ АУДИТОРИИ:
{json.dumps(audience_context, ensure_ascii=False, indent=2)}

КАНДИДАТ:
{json.dumps(candidate, ensure_ascii=False, indent=2)}
"""
        result = complete_with_repair(stages.model_client, ModelCallContext(), Validation, prompt, None)
        result = result.model_copy(update={"candidate_id": candidate_id})
        accepted = state.get("accepted", []) + ([candidate_id] if result.overall_status != "fail" else [])
        return {
            "validations": [*state.get("validations", []), result.model_dump(mode="json")],
            "accepted": accepted,
            "cursor": cursor + 1,
        }

    def route_validation(state: HypothesisState) -> Literal["validate_next", "finalize"]:
        data = HypothesisInput.model_validate(state["input"])
        if len(state.get("accepted", [])) >= data.validation_target:
            return "finalize"
        if state.get("cursor", 0) >= len(state.get("ordered_ids", [])):
            return "finalize"
        return "validate_next"

    def finalize(state: HypothesisState) -> dict[str, Any]:
        data = HypothesisInput.model_validate(state["input"])
        candidates = [Draft.model_validate(item) for item in state.get("candidates", [])]
        ranking = state.get("ranking", {})
        priorities = {
            item["candidate_id"]: Priority.model_validate(item["priority"]) for item in ranking.get("ordered", [])
        }
        attempts = [Validation.model_validate(item) for item in state.get("validations", [])]
        attempt_by_id = {item.candidate_id: item for item in attempts}
        accepted = set(state.get("accepted", []))
        cards: list[Card] = []
        excluded: list[ExcludedCandidate] = []
        for index, candidate in enumerate(candidates, 1):
            priority = priorities.get(
                candidate.candidate_id, _fallback_priority(candidate.candidate_id, index).priority
            )
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
                        ExcludedCandidate(
                            title=candidate.title,
                            reason="; ".join(validation.blocking_issues) or "Гипотеза не прошла проверку.",
                            source_refs=candidate.source_refs,
                        )
                    )
            cards.append(
                Card(
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
        audience = data.audience_result
        audience_payload = audience.get("audience", audience)
        selected_ids = [candidate_id for candidate_id in state.get("ordered_ids", []) if candidate_id in accepted]
        result = PipelineResult(
            schema_version="0.1",
            request_id="hypothesis-run",
            workspace_id=None,
            product_id=data.product_passport.get("product_name") or "unknown",
            status=_result_status(data, selected_ids, attempts),
            input_refs=[
                InputRef(
                    artifact_type="product_result",
                    artifact_id="product-result",
                    version=None,
                    source_refs=sorted(
                        {
                            str(evidence.get("source"))
                            for item in data.product_passport.get("records", [])
                            for evidence in item.get("evidence", [])
                            if evidence.get("source")
                        }
                    ),
                ),
                InputRef(
                    artifact_type="audience_result",
                    artifact_id="audience-result",
                    version="0.1",
                    source_refs=[
                        str(item) for item in audience_payload.get("data_profile", {}).get("snapshot_refs", [])
                    ],
                ),
            ],
            context_summary=_context_summary(data),
            hypotheses=cards,
            excluded_candidates=excluded,
            conflicts=[str(item.get("description", item)) for item in data.product_passport.get("conflicts", [])],
            missing_data=_missing_data(data, state, audience_payload),
            questions=_questions(attempts),
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

    builder.add_node("validate_input", validate_input)
    builder.add_node("generate", generate)
    builder.add_node("rank", rank)
    builder.add_node("validate_next", validate_next)
    builder.add_node("finalize", finalize)
    builder.add_edge(START, "validate_input")
    builder.add_conditional_edges("validate_input", route_input, ["generate", "finalize"])
    builder.add_edge("generate", "rank")
    builder.add_edge("rank", "validate_next")
    builder.add_conditional_edges("validate_next", route_validation, ["validate_next", "finalize"])
    builder.add_edge("finalize", END)
    return builder.compile(checkpointer=checkpointer)


def _module_text() -> str:
    try:
        return MODULE_PROMPT.read_text(encoding="utf-8")
    except OSError:
        return "Используй строгий режим доказательности, сохранение неизвестности и проверяемые гипотезы."


def _prompt_context(data: HypothesisInput) -> tuple[dict[str, Any], dict[str, Any]]:
    passport = data.product_passport
    product_context = {
        "product_name": passport.get("product_name"),
        "extraction_status": passport.get("extraction_status"),
        "records": passport.get("records", []),
        "conflicts": passport.get("conflicts", []),
        "gaps": passport.get("gaps", []),
        "warnings": passport.get("warnings", []),
    }
    audience_result = data.audience_result
    audience_context = {
        "audience": audience_result.get("audience", audience_result),
        "calculation": audience_result.get("calculation"),
        "review": audience_result.get("review"),
        "query": audience_result.get("query"),
    }
    return product_context, audience_context


def _validate_segment_refs(batch: DraftBatch, data: HypothesisInput) -> None:
    audience = data.audience_result.get("audience", data.audience_result)
    allowed_ids = {segment.get("segment_id") for segment in audience.get("segments", [])}
    for candidate in batch.candidates:
        if not candidate.segment_refs or any(ref.segment_id not in allowed_ids for ref in candidate.segment_refs):
            raise ValueError("hypothesis references a segment outside the selected audience")


def _fallback_priority(candidate_id: str, rank: int) -> RankingItem:
    factors = [
        Factor(name=name, rating="unknown", score=None, rationale="Рейтинг не был передан моделью.", source_refs=[])
        for name in ("evidence", "business_value", "feasibility", "novelty", "measurability", "risk")
    ]
    return RankingItem(
        candidate_id=candidate_id,
        priority=Priority(
            rank=rank,
            method="qualitative-fallback",
            provisional=True,
            total_score=None,
            factors=factors,
            rationale="Порядок использован как стабильный fallback из-за неполного ответа ранжировщика.",
        ),
    )


def _context_summary(data: HypothesisInput) -> dict[str, Any]:
    product = data.product_passport
    records = product.get("records", [])
    product_name = product.get("product_name")
    purpose = next((item for item in records if item.get("category") in {"purpose", "jtbd"}), None)
    audience = data.audience_result.get("audience", data.audience_result)
    segments = audience.get("segments", [])
    constraints = [
        Claim(
            text=item.get("statement"),
            kind="fact",
            source_refs=[evidence.get("source", "") for evidence in item.get("evidence", [])],
            rationale="Ограничение передано из паспорта продукта.",
        )
        for item in records
        if item.get("category") in {"limitations", "terms", "communication_rules"}
    ]
    return {
        "product": Claim(
            text=product_name,
            kind="fact" if product_name else "unknown",
            source_refs=[],
            rationale="Название взято из входного product result."
            if product_name
            else "Название продукта отсутствует.",
        ),
        "business_goal": Claim(
            text=purpose.get("statement") if purpose else None,
            kind="fact" if purpose else "unknown",
            source_refs=[evidence.get("source", "") for evidence in (purpose or {}).get("evidence", [])],
            rationale=(
                "Бизнес-контекст ограничен переданными записями product result."
                if purpose
                else "Бизнес-цель не была передана."
            ),
        ),
        "audience_ids": [str(segment.get("segment_id")) for segment in segments if segment.get("segment_id")],
        "constraints": constraints,
    }


def _missing_data(data: HypothesisInput, state: HypothesisState, audience: dict[str, Any]) -> list[str]:
    missing = [item for item in [*state.get("errors", []), *state.get("generation_missing_data", [])] if item]
    if audience.get("status") != "complete":
        missing.append("Результат audience pipeline имеет статус не complete.")
    if not audience.get("segments"):
        missing.append("В audience result нет сохранённого сегмента.")
    return list(dict.fromkeys(missing))


def _questions(attempts: list[Validation]) -> list[str]:
    questions = [item for validation in attempts for item in validation.limitations if item]
    return list(dict.fromkeys(questions))[:3]


def _result_status(
    data: HypothesisInput, selected_ids: list[str], attempts: list[Validation]
) -> Literal["complete", "partial", "blocked"]:
    if not selected_ids:
        return "blocked"
    if len(selected_ids) < data.validation_target:
        return "partial"
    if any(attempt.overall_status == "unknown" for attempt in attempts if attempt.candidate_id in selected_ids):
        return "partial"
    if data.product_passport.get("extraction_status") != "complete":
        return "partial"
    audience = data.audience_result.get("audience", data.audience_result)
    if audience.get("status") != "complete":
        return "partial"
    return "complete"
