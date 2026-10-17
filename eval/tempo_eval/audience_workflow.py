from __future__ import annotations

import json
from pathlib import Path
from typing import Any, Literal, TypedDict

from langgraph.graph import END, START, StateGraph
from langgraph.types import RetryPolicy

from .audience_config import DEFAULT_AUDIENCE_CONFIG, AudiencePipelineConfig
from .audience_data import AudienceDataset, SegmentCalculation
from .audience_review import build_audience_review
from .client import ModelClient
from .models import (
    AudienceAgentReview,
    AudienceNarrative,
    AudiencePrivacyReview,
    AudienceQuery,
    AudienceQueryDecision,
    AudienceResult,
    AudienceReview,
    AudienceReviewDecision,
    Validation,
)
from .prompts import load_prompt
from .semantic import LocalFeatureRetriever, product_text

# Prompts intentionally contain long natural-language instruction lines.
# ruff: noqa: E501


class AudienceWorkflowState(TypedDict, total=False):
    pipeline: str
    model: str
    product_passport: dict[str, Any]
    data_profile: dict[str, Any]
    semantic_cards: list[dict[str, Any]]
    query: AudienceQuery
    calculation: dict[str, Any]
    review_evidence: dict[str, Any]
    query_iteration: int
    query_history: list[dict[str, Any]]
    query_error: str
    repair_attempted: bool
    query_decision: AudienceQueryDecision
    narrative: AudienceNarrative
    audience: AudienceResult
    ai_critic: AudienceAgentReview
    ai_validator: AudienceAgentReview
    ai_privacy: AudiencePrivacyReview
    validation: Validation
    review: AudienceReview


class AudienceWorkflow:
    """Match a product to a private event dataset through a constrained query."""

    def __init__(
        self,
        client: ModelClient,
        dataset_path: Path,
        *,
        db_path: Path | None = None,
        max_segments: int = 1,
        semantic_index_dir: Path,
        config: AudiencePipelineConfig = DEFAULT_AUDIENCE_CONFIG,
    ) -> None:
        self.client = client
        self.dataset = AudienceDataset(dataset_path, db_path=db_path)
        self.max_segments = max_segments
        self.config = config
        self.retriever = LocalFeatureRetriever(self.dataset.profile(), semantic_index_dir)

    def compile(self, checkpointer: Any):
        retry_policy = RetryPolicy(max_attempts=1)
        builder = StateGraph(AudienceWorkflowState)
        builder.add_node("choose_query", self.choose_query, retry_policy=retry_policy)
        builder.add_node("calculate", self.calculate)
        builder.add_node("repair_query", self.repair_query, retry_policy=retry_policy)
        builder.add_node("assess_query", self.assess_query, retry_policy=retry_policy)
        builder.add_node("refine_query", self.refine_query)
        builder.add_node("interpret", self.interpret, retry_policy=retry_policy)
        builder.add_node("blocked", self.blocked)
        builder.add_node("finalize", self.finalize)
        builder.add_node("ai_critic", self.ai_critic, retry_policy=retry_policy)
        builder.add_node("ai_validator", self.ai_validator, retry_policy=retry_policy)
        builder.add_node("ai_privacy", self.ai_privacy, retry_policy=retry_policy)
        builder.add_node("validate", self.validate)
        builder.add_node("review", self.review)
        builder.add_node("gate", self.gate)
        builder.add_edge(START, "choose_query")
        builder.add_edge("choose_query", "calculate")
        builder.add_conditional_edges(
            "calculate", self.route_calculation, ["repair_query", "assess_query", "interpret", "blocked"]
        )
        builder.add_edge("repair_query", "calculate")
        builder.add_conditional_edges(
            "assess_query", self.route_query_decision, ["refine_query", "interpret", "blocked"]
        )
        builder.add_edge("refine_query", "calculate")
        builder.add_edge("interpret", "finalize")
        builder.add_edge("blocked", "validate")
        builder.add_conditional_edges("finalize", self.route_ai_review, ["ai_critic", "validate"])
        builder.add_edge("ai_critic", "ai_validator")
        builder.add_edge("ai_validator", "ai_privacy")
        builder.add_edge("ai_privacy", "validate")
        builder.add_edge("validate", "review")
        builder.add_edge("review", "gate")
        builder.add_edge("gate", END)
        return builder.compile(checkpointer=checkpointer)

    async def choose_query(self, state: AudienceWorkflowState) -> dict[str, Any]:
        cards = self.retriever.search(product_text(state["product_passport"]))
        prompt = _query_prompt(state["product_passport"], state["data_profile"], self.max_segments, cards, self.config)
        queries = []
        queries.append(await self.client.complete_structured("audience_query", prompt, AudienceQuery))
        return {
            "semantic_cards": cards,
            "query": queries[0],
            "query_iteration": 1,
            "query_history": [],
        }

    def calculate(self, state: AudienceWorkflowState) -> dict[str, Any]:
        try:
            primary_query = AudienceQuery.model_validate(state["query"])
            calculation = self.dataset.calculate(primary_query)
        except ValueError as error:
            return {"query_error": str(error)}
        calculation_json = _calculation_json(calculation)
        evidence = self.dataset.review_evidence(primary_query)
        calculation_json["review_evidence"] = evidence
        calculation_json["window_diagnostics"] = _window_diagnostics(calculation_json)
        history = [
            *state.get("query_history", []),
            {
                "iteration": state.get("query_iteration", 1),
                "query": primary_query.model_dump(mode="json"),
                "result": calculation_json,
            },
        ]
        return {
            "calculation": calculation_json,
            "review_evidence": evidence,
            "query_error": "",
            "query_history": history,
        }

    def route_calculation(
        self, state: AudienceWorkflowState
    ) -> Literal["repair_query", "assess_query", "interpret", "blocked"]:
        if not state.get("query_error"):
            if state.get("query_iteration", 1) >= self.config.max_query_iterations:
                return "interpret" if state["calculation"]["count"] >= self.config.minimum_segment_size else "blocked"
            return "assess_query"
        return "blocked" if state.get("repair_attempted") else "repair_query"

    async def repair_query(self, state: AudienceWorkflowState) -> dict[str, Any]:
        cards = self.retriever.search(product_text(state["product_passport"]))
        prompt = _query_prompt(state["product_passport"], state["data_profile"], self.max_segments, cards, self.config)
        previous_query = state.get("query")
        prompt += "\n\nПРЕДЫДУЩИЙ ЗАПРОС:\n"
        prompt += json.dumps(
            previous_query.model_dump(mode="json") if isinstance(previous_query, AudienceQuery) else previous_query,
            ensure_ascii=False,
            indent=2,
        )
        prompt += "\n\nОШИБКИ ПРОВЕРКИ:\n" + state.get("query_error", "")
        prompt += "\nИсправь только запрос. Используй только точные field и value из value_catalog."
        repaired_query = await self.client.complete_structured("audience_query_repair", prompt, AudienceQuery)
        return {
            "semantic_cards": cards,
            "query": repaired_query,
            "repair_attempted": True,
            "query_history": state.get("query_history", []),
        }

    async def assess_query(self, state: AudienceWorkflowState) -> dict[str, AudienceQueryDecision]:
        prompt = _query_result_prompt(
            state["product_passport"],
            state["data_profile"],
            state["query"],
            state["calculation"],
            state.get("query_history", []),
            state.get("semantic_cards", []),
            self.config,
        )
        decision = await self.client.complete_structured("audience_query_result_review", prompt, AudienceQueryDecision)
        if state["calculation"]["count"] < self.config.minimum_segment_size and decision.decision == "use_result":
            decision = await self.client.complete_structured(
                "audience_query_result_required_refinement",
                prompt + "\nОБЯЗАТЕЛЬНО: текущий результат ниже minimum_segment_size. Верни decision=refine "
                "и конкретный next_query из разрешённых полей; use_result недопустим.",
                AudienceQueryDecision,
            )
        return {"query_decision": decision}

    def route_query_decision(self, state: AudienceWorkflowState) -> Literal["refine_query", "interpret", "blocked"]:
        decision = AudienceQueryDecision.model_validate(state["query_decision"])
        if decision.decision == "refine" and decision.next_query is not None:
            return "refine_query"
        if state["calculation"]["count"] < self.config.minimum_segment_size:
            return "blocked"
        return "interpret"

    def refine_query(self, state: AudienceWorkflowState) -> dict[str, Any]:
        decision = AudienceQueryDecision.model_validate(state["query_decision"])
        if decision.decision != "refine" or decision.next_query is None:
            raise ValueError("query refinement must include next_query")
        return {
            "query": decision.next_query,
            "query_iteration": state.get("query_iteration", 1) + 1,
            "query_error": "",
            "repair_attempted": False,
        }

    async def interpret(self, state: AudienceWorkflowState) -> dict[str, AudienceNarrative]:
        prompt = _interpretation_prompt(state["product_passport"], state["data_profile"], state["calculation"])
        return {
            "narrative": await self.client.complete_structured("audience_interpretation", prompt, AudienceNarrative)
        }

    def blocked(self, state: AudienceWorkflowState) -> dict[str, AudienceResult]:
        profile = state["data_profile"]
        return {
            "audience": AudienceResult(
                schema_version="0.1",
                request_id=None,
                product_id=state["product_passport"].get("product_name"),
                status="blocked",
                mode="none",
                data_profile=_data_profile(profile),
                features_used=[],
                features_excluded=_excluded_features(profile),
                segments=[],
                overlaps={"status": "not_applicable", "detail": "Сегменты не рассчитаны.", "source_refs": []},
                missing_data=["Не удалось сформировать допустимый запрос к набору данных."],
                questions=["Какие признаки и период должны использоваться для сегментации?"],
                limitations=[state.get("query_error", "Запрос к данным не выполнен.")],
                review_status="pending",
                configuration=self.config.as_dict(),
            )
        }

    def finalize(self, state: AudienceWorkflowState) -> dict[str, AudienceResult]:
        calculation = state["calculation"]
        profile = state["data_profile"]
        narrative = AudienceNarrative.model_validate(state["narrative"])
        if calculation["count"] == 0:
            return {
                "audience": AudienceResult(
                    schema_version="0.1",
                    request_id=None,
                    product_id=state["product_passport"].get("product_name"),
                    status="complete",
                    mode="data_based",
                    data_profile=_data_profile(profile),
                    features_used=[],
                    features_excluded=_excluded_features(profile),
                    segments=[],
                    overlaps={
                        "status": "not_applicable",
                        "detail": "Подходящих клиентов не найдено.",
                        "source_refs": [calculation["rule_ref"]],
                    },
                    missing_data=[],
                    questions=narrative.questions,
                    limitations=["Выполненный воспроизводимый запрос не вернул кандидатов."],
                    review_status="pending",
                    configuration=self.config.as_dict(),
                )
            }
        segment = {
            "segment_id": "seg_001",
            "name": narrative.name,
            "origin": "observed",
            "definition": {
                "inclusion_rules": [_rule_text(item) for item in calculation["feature_summaries"]],
                "exclusion_rules": [],
                "missing_value_policy": "Событие без требуемых значений не соответствует условию.",
                "method": "client_month_rules",
                "rule_or_model_ref": calculation["rule_ref"],
                "membership_ref": calculation["membership_ref"],
            },
            "size": {
                "count": calculation["count"],
                "base_count": calculation["base_count"],
                "share": calculation["share"],
                "population": calculation["population"],
                "source_refs": [calculation["dataset_ref"], calculation["rule_ref"]],
            },
            "characteristics": [
                {
                    "text": (
                        f"{calculation['count']} из {calculation['base_count']} клиентов "
                        "соответствуют воспроизводимому правилу."
                    ),
                    "kind": "fact",
                    "source_refs": [calculation["rule_ref"]],
                    "rationale": None,
                }
            ],
            "interpretation": _interpretation(narrative, calculation["rule_ref"]),
            "checks": _checks(calculation, self.config),
            "risks": narrative.risks,
        }
        return {
            "audience": AudienceResult(
                schema_version="0.1",
                request_id=None,
                product_id=state["product_passport"].get("product_name"),
                status="partial",
                mode="data_based",
                data_profile=_data_profile(profile),
                features_used=[
                    {
                        "name": item["name"],
                        "definition": _rule_text(item),
                        "window": {
                            "start": calculation["query"]["window_start"],
                            "end": calculation["query"]["window_end"],
                        },
                        "source_refs": [calculation["dataset_ref"], calculation["rule_ref"]],
                    }
                    for item in calculation["feature_summaries"]
                ],
                features_excluded=_excluded_features(profile),
                segments=[segment],
                overlaps={
                    "status": "not_applicable",
                    "detail": "Сформирован один сегмент.",
                    "source_refs": [calculation["rule_ref"]],
                },
                missing_data=[
                    "Фактическое согласие и сопоставление клиентов с каналом проверяются перед кампанией; текущая политика только разрешает канал.",
                    "Независимые продуктовые или конверсионные результаты не входят в текущий набор событий.",
                ],
                questions=narrative.questions,
                limitations=narrative.limitations,
                review_status="pending",
                configuration=self.config.as_dict(),
            )
        }

    def route_ai_review(self, state: AudienceWorkflowState) -> Literal["ai_critic", "validate"]:
        return "ai_critic" if state.get("calculation", {}).get("count", 0) > 0 else "validate"

    async def ai_critic(self, state: AudienceWorkflowState) -> dict[str, AudienceAgentReview]:
        review = await self._review_until_final(state, role="critic")
        return {"ai_critic": review}

    async def ai_validator(self, state: AudienceWorkflowState) -> dict[str, AudienceAgentReview]:
        review = await self._review_until_final(state, role="validator")
        return {"ai_validator": review}

    async def _review_until_final(
        self, state: AudienceWorkflowState, *, role: Literal["critic", "validator"]
    ) -> AudienceAgentReview:
        evidence = state["calculation"].get("review_evidence", {})
        inspection_history: list[dict[str, Any]] = []
        for iteration in range(self.config.max_review_iterations):
            if role == "critic":
                prompt = _critic_prompt(
                    state["product_passport"],
                    state["data_profile"],
                    state["audience"],
                    state["calculation"],
                    evidence,
                    inspection_history,
                    self.config,
                )
                operation = "audience_critic" if iteration == 0 else "audience_critic_inspection"
            else:
                prompt = _validator_prompt(
                    state["data_profile"],
                    state["audience"],
                    state["calculation"],
                    evidence,
                    inspection_history,
                    self.config,
                )
                operation = "audience_validator" if iteration == 0 else "audience_validator_inspection"
            decision = await self.client.complete_structured(operation, prompt, AudienceReviewDecision)
            if decision.decision == "final" and decision.review is not None:
                return decision.review
            if decision.decision != "inspect" or decision.request is None:
                break
            try:
                evidence = self.dataset.inspect_evidence(state["query"], decision.request)
            except ValueError as error:
                evidence = {"error": str(error)}
            inspection_history.append(
                {
                    "request": decision.request.model_dump(mode="json"),
                    "response": evidence,
                }
            )
        return AudienceAgentReview(
            overall_status="unknown",
            checks=[
                {
                    "name": f"{role}_review",
                    "status": "unknown",
                    "detail": "AI review budget exhausted before a final assessment was returned.",
                    "rationale": None,
                }
            ],
            blocking_issues=[],
        )

    async def ai_privacy(self, state: AudienceWorkflowState) -> dict[str, AudiencePrivacyReview]:
        prompt = _privacy_prompt(state["audience"], state["calculation"])
        return {
            "ai_privacy": await self.client.complete_structured(
                "audience_privacy_review", prompt, AudiencePrivacyReview
            )
        }

    def validate(self, state: AudienceWorkflowState) -> dict[str, Validation]:
        result = AudienceResult.model_validate(state["audience"])
        errors = []
        if result.mode == "data_based" and len(result.segments) > self.max_segments:
            errors.append(f"segment count exceeds max_segments={self.max_segments}")
        for segment in result.segments:
            if (
                segment.size.count is None
                or segment.size.base_count is None
                or not segment.definition.rule_or_model_ref
            ):
                errors.append(f"segment lacks a deterministic calculation: {segment.segment_id}")
            if segment.size.count and segment.size.count > segment.size.base_count:
                errors.append(f"segment count exceeds base count: {segment.segment_id}")
        return {"validation": Validation(valid=not errors, errors=errors)}

    def review(self, state: AudienceWorkflowState) -> dict[str, AudienceReview]:
        context = {
            "config": {
                "max_segments": self.max_segments,
                "allowed_features": [
                    item["name"] for item in state.get("calculation", {}).get("feature_summaries", [])
                ],
            },
            "dataset_profile": state["data_profile"],
            "audience_config": self.config.as_dict(),
            "ai_critic": state.get("ai_critic"),
            "ai_validator": state.get("ai_validator"),
            "ai_privacy": state.get("ai_privacy"),
        }
        return {
            "review": build_audience_review(
                AudienceResult.model_validate(state["audience"]),
                Validation.model_validate(state["validation"]),
                context,
            )
        }

    def gate(self, state: AudienceWorkflowState) -> dict[str, AudienceResult]:
        result = AudienceResult.model_validate(state["audience"])
        review = AudienceReview.model_validate(state["review"])
        failures = [
            check.detail for check in [*review.critic_checks, *review.validator_checks] if check.status == "fail"
        ]
        if not failures or result.status == "blocked":
            return {}
        return {
            "audience": AudienceResult(
                schema_version="0.1",
                request_id=None,
                product_id=result.product_id,
                status="blocked",
                mode=result.mode,
                data_profile=result.data_profile,
                features_used=result.features_used,
                features_excluded=result.features_excluded,
                segments=result.segments,
                overlaps=result.overlaps,
                missing_data=[
                    *result.missing_data,
                    "Исправьте замечания критика или валидатора перед использованием сегмента.",
                ],
                questions=result.questions,
                limitations=[*result.limitations, *failures],
                review_status="pending",
                configuration=result.configuration,
            )
        }


def _query_prompt(
    product_passport: dict[str, Any],
    data_profile: dict[str, Any],
    max_segments: int,
    retrieved_cards: list[dict[str, Any]],
    config: AudiencePipelineConfig,
) -> str:
    return f"""Ты выбираешь один воспроизводимый сегмент клиентов для продукта.

{load_prompt("audience-data-contract.md")}

    Ты не видишь сырые записи, client_id или персональные данные. Ты можешь вернуть только один запрос с условиями по полям и точным значениям из value_catalog. Каждое условие должно встречаться хотя бы раз в каждом календарном месяце выбранного окна. window_start и window_end обязаны быть первым днём календарного месяца; window_end является исключающей границей. Если последний event_timestamp приходится на середину месяца, последний месяц неполный: используй первым днём этого неполного месяца как window_end (например, при последнем событии 2026-09-28 используй window_end=2026-09-01), а не первый день следующего месяца. Не используй forbidden_fields, кампании, баланс, демографию или географию. Не выдумывай поля, значения или период.

    Это начальная версия: max_segments={max_segments}; верни один лучший сегмент, а не несколько.

ПОЛИТИКА ПАЙПЛАЙНА:
{json.dumps(config.as_dict(), ensure_ascii=False)}
Минимальный размер сегмента учитывается отдельным валидатором. Канал
`ad_platform` сейчас объявлен доступным конфигурацией; это не доказывает
наличие согласия или фактическое сопоставление клиентов с каналом.

СЕМАНТИЧЕСКИ НАЙДЕННЫЕ КАРТОЧКИ ПРИЗНАКОВ:
{json.dumps(retrieved_cards, ensure_ascii=False)}

Карточки помогают выбрать релевантные признаки, но не доказывают существование сегмента. Используй только точные filters из карточек или точные field/value из value_catalog. Расчёт состава и размера выполнит отдельный локальный инструмент.

ПАСПОРТ ПРОДУКТА:
{json.dumps(product_passport, ensure_ascii=False)}

    БЕЗОПАСНЫЙ ПРОФИЛЬ ДАННЫХ:
{json.dumps(data_profile, ensure_ascii=False)}
"""


def _interpretation_prompt(
    product_passport: dict[str, Any], data_profile: dict[str, Any], calculation: dict[str, Any]
) -> str:
    return f"""Интерпретируй уже рассчитанный сегмент для продукта и ответь на вопросы аудитории: кто потенциально релевантен гипотезе, в каком контексте используется продукт, какой JTBD или боль наблюдается, какие реальные сигналы это подтверждают, какие сигналы могут указывать на готовность, какие потребности и мотивации возможны, а также какие есть триггеры, барьеры и сомнения. Не меняй правило, размер, долю, период, признаки или источники. Не утверждай, что поведение доказывает мотив, владение бизнесом, потребность или готовность купить. Не раскрывай персональные данные.

{load_prompt("audience-data-contract.md")}

Для неподтверждённого утверждения верни kind=unknown и text=null. Для интерпретации верни kind=inference с кратким rationale. product relevance обычно является inference, если нет отдельного подтверждённого исследования.
Поле questions используй только для нерешённых вопросов к продукту или данным,
например для отсутствующих сигналов; это не подтверждение факта и не запрос к
пользователю внутри текущего запуска.

ПАСПОРТ ПРОДУКТА:
{json.dumps(product_passport, ensure_ascii=False)}

ПРОФИЛЬ ДАННЫХ:
{json.dumps(data_profile, ensure_ascii=False)}

РЕЗУЛЬТАТ РАСЧЁТА:
{json.dumps(calculation, ensure_ascii=False)}
"""


def _query_result_prompt(
    product_passport: dict[str, Any],
    data_profile: dict[str, Any],
    query: AudienceQuery,
    calculation: dict[str, Any],
    query_history: list[dict[str, Any]],
    semantic_cards: list[dict[str, Any]],
    config: AudiencePipelineConfig,
) -> str:
    return f"""Ты оцениваешь результат уже выполненного запроса к SQLite.

Реши, достаточно ли результат полезен для продуктовой гипотезы. Верни
decision=use_result, если текущий сегмент релевантен и его можно интерпретировать.
Верни decision=refine, если нужно попробовать другой разрешённый сигнал,
период или комбинацию фильтров. При refine обязательно верни next_query.
Используй только точные поля и значения из безопасного профиля и текущих
карточек/запроса. Не выдумывай размер, мотив или персональные данные.

Учитывай конфигурацию: минимальный размер сегмента — это обязательный порог;
если он не достигнут, обычно предложи refinement, если в каталоге есть
разумная альтернатива. Не ослабляй правило только ради размера и не добавляй
запрещённые признаки. Если данных недостаточно для уверенного решения,
выбери refine только при наличии конкретного допустимого следующего запроса;
иначе выбери use_result и объясни ограничение в reason. Нулевой или меньший
минимального порога результат нельзя считать пригодным сегментом.

Предпочитай несколько независимых, взаимно усиливающих поведенческих сигналов
из разных полей или типов событий, когда они поддерживают одну продуктовую
гипотезу. Один общий потребительский сигнал сам по себе недостаточен.

Текущая итерация не должна повторять тот же запрос без изменения.
Если window_diagnostics содержит suggested_window_end, сначала проверь refinement
с этой исключающей границей: trailing zero months делают recurring-сегмент пустым.

КОНФИГУРАЦИЯ:
{json.dumps(config.as_dict(), ensure_ascii=False)}

ПАСПОРТ ПРОДУКТА:
{json.dumps(product_passport, ensure_ascii=False)}

ПРОФИЛЬ ДАННЫХ:
{json.dumps(data_profile, ensure_ascii=False)}

ТЕКУЩИЙ ЗАПРОС:
{json.dumps(query.model_dump(mode="json"), ensure_ascii=False)}

РЕЗУЛЬТАТ SQLITE:
{json.dumps(calculation, ensure_ascii=False)}

ИСТОРИЯ ПРЕДЫДУЩИХ ИТЕРАЦИЙ:
{json.dumps(query_history, ensure_ascii=False)}

ДОСТУПНЫЕ КАРТОЧКИ ПРИЗНАКОВ:
{json.dumps(semantic_cards, ensure_ascii=False)}
"""


def _critic_prompt(
    product_passport: dict[str, Any],
    data_profile: dict[str, Any],
    audience: AudienceResult,
    calculation: dict[str, Any],
    evidence: dict[str, Any],
    inspection_history: list[dict[str, Any]],
    config: AudiencePipelineConfig,
) -> str:
    return f"""Ты независимый AI-критик результата сегментации аудитории.

Работай итеративно. Верни decision=inspect, если для проверки нужен один
допустимый агрегатный срез, и заполни request. Верни decision=final, когда
доказательств достаточно, и заполни review проверками по категориям relevance,
sensitive_features_and_bias, interpretability и target_information_leakage.
Статус fail ставь только при конкретном противоречии продукту, запрещённом
признаке, утечке или другой проблеме, которая делает результат непригодным.
Если данные не доказывают роль человека, намерение или JTBD, но поведение
остаётся совместимым с продуктовой гипотезой, верни unknown и объясни
ограничение. Не превращай отсутствие доказательства в fail и не придумывай
отсутствующие данные.

Проверь в том числе косвенные чувствительные признаки: географию, демографию,
доход, баланс, личность, контакты и очевидные поведенческие прокси. Не
повторяй потенциальные персональные значения в detail или rationale.

ПАСПОРТ ПРОДУКТА:
{json.dumps(product_passport, ensure_ascii=False)}

ПРОФИЛЬ ДАННЫХ:
{json.dumps(data_profile, ensure_ascii=False)}

РЕЗУЛЬТАТ:
{json.dumps(audience.model_dump(mode="json"), ensure_ascii=False)}

РАСЧЁТ:
{json.dumps(calculation, ensure_ascii=False)}

ДОПОЛНИТЕЛЬНОЕ АГРЕГАТНОЕ ДОКАЗАТЕЛЬСТВО:
{json.dumps(evidence, ensure_ascii=False)}

ИСТОРИЯ ИНСПЕКЦИЙ:
{json.dumps(inspection_history, ensure_ascii=False)}

Разрешённые виды request.kind: condition_breakdown, monthly_breakdown,
value_distribution, condition_overlap. Один request за итерацию; не проси
сырые строки, client_id или membership list.
"""


def _validator_prompt(
    data_profile: dict[str, Any],
    audience: AudienceResult,
    calculation: dict[str, Any],
    evidence: dict[str, Any],
    inspection_history: list[dict[str, Any]],
    config: AudiencePipelineConfig,
) -> str:
    return f"""Ты независимый AI-валидатор результата сегментации.

    Работай итеративно. Верни decision=inspect, если для проверки нужен один
    допустимый агрегатный срез, и заполни request. Верни decision=final, когда
    доказательств достаточно, и заполни review с проверками data_quality,
    minimum_group_size, stability_across_periods_or_samples и
    channel_availability. Используй переданные детерминированные результаты
    SQLite и конфигурацию как источник истины. Не оценивай и не имитируй
    подтверждение продакт-менеджера: оно происходит после пайплайна человеком.

Если данных для проверки нет, верни unknown, а не pass. Для
channel_availability различай: разрешение канала политикой — pass для
конфигурационного чека; фактическое согласие, identity resolution и
контактируемость клиентов — внешнее ограничение, а не причина блокировать
результат. Для stability_across_periods_or_samples месячная повторяемость
внутри одного окна не равна проверке на альтернативном периоде или выборке:
без такого расчёта верни unknown.

КОНФИГУРАЦИЯ:
{json.dumps(config.as_dict(), ensure_ascii=False)}

ПРОФИЛЬ ДАННЫХ:
{json.dumps(data_profile, ensure_ascii=False)}

РЕЗУЛЬТАТ:
{json.dumps(audience.model_dump(mode="json"), ensure_ascii=False)}

РАСЧЁТ И ДЕТЕРМИНИРОВАННЫЕ ПРОВЕРКИ:
{json.dumps(calculation, ensure_ascii=False)}

ДОПОЛНИТЕЛЬНОЕ АГРЕГАТНОЕ ДОКАЗАТЕЛЬСТВО:
{json.dumps(evidence, ensure_ascii=False)}

ИСТОРИЯ ИНСПЕКЦИЙ:
{json.dumps(inspection_history, ensure_ascii=False)}

Разрешённые виды request.kind: condition_breakdown, monthly_breakdown,
value_distribution, condition_overlap. Один request за итерацию; не проси
сырые строки, client_id или membership list.
"""


def _privacy_prompt(audience: AudienceResult, calculation: dict[str, Any]) -> str:
    return f"""Ты выполняешь отдельную проверку приватности результата сегментации.

Проверь только опубликованный результат и расчёт на наличие client_id,
идентификаторов, контактов, raw records, membership lists, имён, телефонов,
email, адресов или иных персональных значений. Не считай dataset_ref,
rule_ref и membership_ref персональными данными: это закрытые ссылки. Само
название поля client_id в описании политики также не является утечкой; ищи
реальные идентификаторы, значения и списки.

Верни fail, если обнаружена конкретная утечка; unknown, если проверить нельзя.
Не цитируй обнаруженные персональные значения и указывай только категорию
утечки. Эта проверка не раскрывает и не запрашивает состав сегмента.

РЕЗУЛЬТАТ:
{json.dumps(audience.model_dump(mode="json"), ensure_ascii=False)}

РАСЧЁТ:
{json.dumps(calculation, ensure_ascii=False)}
"""


def _calculation_json(calculation: SegmentCalculation) -> dict[str, Any]:
    return {
        "dataset_ref": calculation.dataset_ref,
        "rule_ref": calculation.rule_ref,
        "membership_ref": calculation.membership_ref,
        "query": calculation.query,
        "count": calculation.count,
        "base_count": calculation.base_count,
        "share": calculation.share,
        "population": calculation.population,
        "feature_summaries": calculation.feature_summaries,
        "aggregate_summary": calculation.aggregate_summary,
        "quality": calculation.quality,
    }


def _window_diagnostics(calculation: dict[str, Any]) -> dict[str, Any]:
    first_zero_by_feature = {}
    for feature in calculation["feature_summaries"]:
        first_zero = next(
            (item["month"] for item in feature["monthly_client_counts"] if item["client_count"] == 0),
            None,
        )
        if first_zero is not None:
            first_zero_by_feature[feature["name"]] = first_zero
    candidates = [month for month in first_zero_by_feature.values() if month > calculation["query"]["window_start"]]
    return {
        "first_zero_month_by_feature": first_zero_by_feature,
        "suggested_window_end": min(candidates) if candidates else None,
        "interpretation": (
            "A zero month can make an otherwise useful recurring pattern disappear; "
            "consider the first zero month as an exclusive window_end."
            if candidates
            else "No trailing zero month was detected in the returned feature summaries."
        ),
    }


def _data_profile(profile: dict[str, Any]) -> dict[str, Any]:
    return {
        "unit": profile["unit"],
        "cutoff_at": profile["date_range"]["end"],
        "observation_window": profile["date_range"],
        "snapshot_refs": [profile["dataset_ref"]],
        "coverage": {
            "text": f"{profile['event_count']} событий и {profile['unique_client_count']} уникальных клиентов.",
            "kind": "fact",
            "source_refs": [profile["dataset_ref"]],
            "rationale": None,
        },
        "quality_issues": [],
    }


def _excluded_features(profile: dict[str, Any]) -> list[dict[str, str]]:
    return [
        {"name": name, "reason": "Поле исключено из сегментации по политике приватности или предотвращения утечки."}
        for name in profile["forbidden_fields"]
    ]


def _rule_text(feature: dict[str, Any]) -> str:
    filters = " AND ".join(f"{item['field']} = {item['value']}" for item in feature["filters"])
    return f"{feature['name']}: {filters}; хотя бы одно событие в каждом календарном месяце окна."


def _claim(claim: Any, reference: str) -> dict[str, Any]:
    if claim.kind == "unknown":
        return {
            "text": None,
            "kind": "unknown",
            "source_refs": [],
            "rationale": claim.rationale or "Недостаточно данных.",
        }
    return {"text": claim.text, "kind": claim.kind, "source_refs": [reference], "rationale": claim.rationale}


def _interpretation(narrative: AudienceNarrative, reference: str) -> dict[str, Any]:
    return {
        "context": _claim(narrative.context, reference),
        "jtbd": _claim(narrative.jtbd, reference),
        "pains": [_claim(item, reference) for item in narrative.pains],
        "needs": [_claim(item, reference) for item in narrative.needs],
        "readiness": [_claim(item, reference) for item in narrative.readiness],
        "motivations": [_claim(item, reference) for item in narrative.motivations],
        "triggers": [_claim(item, reference) for item in narrative.triggers],
        "barriers": [_claim(item, reference) for item in narrative.barriers],
        "doubts": [_claim(item, reference) for item in narrative.doubts],
    }


def _checks(calculation: dict[str, Any], config: AudiencePipelineConfig) -> list[dict[str, Any]]:
    missing = calculation["quality"]["missing_required_event_values"]
    quality_status = "pass" if not any(missing.values()) else "unknown"
    stability = calculation.get("stability", {})
    stability_status = stability.get("status", "unknown")
    minimum_size_status = "pass" if calculation["count"] >= config.minimum_segment_size else "fail"
    return [
        {
            "name": "data_quality",
            "status": quality_status,
            "detail": "Проверены пропуски в полях правила.",
            "source_refs": [calculation["dataset_ref"]],
        },
        {
            "name": "relevance",
            "status": "unknown",
            "detail": "Нужна отдельная проверка связи сегмента с продуктовой задачей.",
            "source_refs": [],
        },
        {
            "name": "reproducibility",
            "status": "pass",
            "detail": "Правило, период и закрытая ссылка на состав сегмента сохранены.",
            "source_refs": [calculation["rule_ref"]],
        },
        {
            "name": "min_size",
            "status": minimum_size_status,
            "detail": (
                f"Размер сегмента: {calculation['count']}; минимальный размер по конфигурации: "
                f"{config.minimum_segment_size}."
            ),
            "source_refs": [],
        },
        {
            "name": "stability",
            "status": stability_status,
            "detail": stability.get("detail", "Повторный расчёт устойчивости не выполнен."),
            "source_refs": [],
        },
        {
            "name": "privacy_bias",
            "status": "unknown",
            "detail": "Чувствительные и финансовые поля исключены, но отдельная проверка смещения не выполнена.",
            "source_refs": [],
        },
        {
            "name": "leakage",
            "status": "pass",
            "detail": "Запрос не допускает кампании, будущие события или целевые отклики.",
            "source_refs": [calculation["rule_ref"]],
        },
        {
            "name": "activation",
            "status": "pass" if config.allowed_channels else "fail",
            "detail": (
                "Канал ad_platform разрешён конфигурацией пайплайна; фактическое согласие и сопоставление "
                "клиентов с каналом должны проверяться перед запуском кампании."
            ),
            "source_refs": [],
        },
    ]
