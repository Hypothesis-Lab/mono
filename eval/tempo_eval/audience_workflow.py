from __future__ import annotations

import json
import re
from typing import Any, Literal, TypedDict

from langgraph.graph import END, START, StateGraph
from langgraph.types import RetryPolicy

from .client import ModelClient
from .models import (
    AudiencePlan,
    AudienceProfile,
    AudienceResult,
    Validation,
)
from .prompts import load_prompt


class AudienceWorkflowState(TypedDict, total=False):
    model: str
    context: dict[str, Any]
    profile: AudienceProfile
    plan: AudiencePlan
    audience: AudienceResult
    validation: Validation
    repair_attempted: bool


class AudienceWorkflow:
    """Profile, segment, and interpret an audience with compact stage contracts."""

    def __init__(self, client: ModelClient, *, step_retries: int = 3) -> None:
        self.client = client
        self.step_retries = step_retries

    def compile(self, checkpointer: Any):
        retry_policy = RetryPolicy(max_attempts=1)
        builder = StateGraph(AudienceWorkflowState)
        builder.add_node("profile", self.profile, retry_policy=retry_policy)
        builder.add_node("segment", self.segment, retry_policy=retry_policy)
        builder.add_node("interpret", self.interpret, retry_policy=retry_policy)
        builder.add_node("validate", self.validate)
        builder.add_node("repair", self.repair, retry_policy=retry_policy)
        builder.add_node("finalize", self.finalize)
        builder.add_edge(START, "profile")
        builder.add_edge("profile", "segment")
        builder.add_edge("segment", "interpret")
        builder.add_edge("interpret", "validate")
        builder.add_conditional_edges("validate", self.route_validation, ["repair", "finalize"])
        builder.add_edge("repair", "finalize")
        builder.add_edge("finalize", END)
        return builder.compile(checkpointer=checkpointer)

    def profile(self, state: AudienceWorkflowState) -> dict[str, AudienceProfile]:
        prompt = _stage_prompt(
            "profile",
            {"CONTEXT_JSON": json.dumps(state["context"], ensure_ascii=False, indent=2)},
        )
        return {"profile": self.client.complete_structured("audience_profile", prompt, AudienceProfile)}

    def segment(self, state: AudienceWorkflowState) -> dict[str, AudiencePlan]:
        prompt = _stage_prompt(
            "segment",
            {"PROFILE_JSON": json.dumps(state["profile"].model_dump(mode="json"), ensure_ascii=False, indent=2)},
        )
        return {"plan": self.client.complete_structured("audience_segment", prompt, AudiencePlan)}

    def interpret(self, state: AudienceWorkflowState) -> dict[str, AudienceResult]:
        prompt = _stage_prompt(
            "interpret",
            {
                "PROFILE_JSON": json.dumps(state["profile"].model_dump(mode="json"), ensure_ascii=False, indent=2),
                "PLAN_JSON": json.dumps(state["plan"].model_dump(mode="json"), ensure_ascii=False, indent=2),
            },
        )
        return {"audience": self.client.complete_structured("audience_interpret", prompt, AudienceResult)}

    def validate(self, state: AudienceWorkflowState) -> dict[str, Validation]:
        result = AudienceResult.model_validate(state["audience"])
        return {"validation": Validation(valid=not validate_audience(result, state["context"]), errors=validate_audience(result, state["context"]))}

    def route_validation(self, state: AudienceWorkflowState) -> Literal["repair", "finalize"]:
        validation = Validation.model_validate(state["validation"])
        if validation.valid or state.get("repair_attempted", False):
            return "finalize"
        return "repair"

    def repair(self, state: AudienceWorkflowState) -> dict[str, object]:
        result = AudienceResult.model_validate(state["audience"])
        validation = Validation.model_validate(state["validation"])
        prompt = _stage_prompt(
            "repair",
            {
                "RESULT_JSON": json.dumps(result.model_dump(mode="json"), ensure_ascii=False, indent=2),
                "ERRORS_JSON": json.dumps(validation.errors, ensure_ascii=False),
            },
        )
        repaired = self.client.complete_structured("audience_repair", prompt, AudienceResult)
        return {"audience": repaired, "repair_attempted": True}

    def finalize(self, state: AudienceWorkflowState) -> dict[str, Validation]:
        result = AudienceResult.model_validate(state["audience"])
        errors = validate_audience(result, state["context"])
        return {"validation": Validation(valid=not errors, errors=errors)}


def _stage_prompt(stage: str, values: dict[str, str]) -> str:
    prompt = load_prompt("audience.md")
    sections = {
        "profile": """
ЭТАП PIPELINE: ПРОФИЛИРОВАНИЕ
Сформируй только внутренний контракт AudienceProfile. Не создавай сегменты и не интерпретируй мотивы.
Проверь идентичность продукта, единицу анализа, cutoff, периоды, покрытие, качество данных, доступность и
допустимость признаков. Сохрани все существенные наблюдаемые факты компактно в observations с source_refs,
чтобы следующие этапы не получали повторно весь исходный контекст. Не придумывай расчёты.

КОНТЕКСТ:
{{CONTEXT_JSON}}
""",
        "segment": """
ЭТАП PIPELINE: СЕГМЕНТАЦИЯ
Сформируй только внутренний контракт AudiencePlan. Используй только переданный профиль; не выдумывай данные,
источники, размеры, membership или выполненные расчёты. Если данных недостаточно, верни mode=none или
hypothesis_only по правилам основного промпта. Не добавляй interpretation: это делает следующий этап.

ПРОФИЛЬ:
{{PROFILE_JSON}}
""",
        "interpret": """
ЭТАП PIPELINE: ИНТЕРПРЕТАЦИЯ И ФИНАЛИЗАЦИЯ
Сформируй полный AudienceResult по основному контракту. Используй только профиль и план. Заполни interpretation
для каждого сегмента, но маркируй inference и unknown корректно. Не добавляй факты, числа, source_refs или
проверки, которых нет в этих контрактах. Верни review_status=pending.

ПРОФИЛЬ:
{{PROFILE_JSON}}

ПЛАН СЕГМЕНТАЦИИ:
{{PLAN_JSON}}
""",
        "repair": """
ЭТАП PIPELINE: ПРОВЕРКА И ОГРАНИЧЕННЫЙ РЕМОНТ
Исправь только перечисленные ошибки контракта. Не добавляй новые факты, сегменты, числа, источники или
подтверждённые проверки. Сохрани неизвестность и смысл результата. Верни полный AudienceResult, включая все поля.

ТЕКУЩИЙ РЕЗУЛЬТАТ:
{{RESULT_JSON}}

ОШИБКИ ПРОГРАММНОЙ ПРОВЕРКИ:
{{ERRORS_JSON}}
""",
    }
    text = prompt + "\n" + sections[stage]
    for key, value in values.items():
        text = text.replace("{{" + key + "}}", value)
    return text


def validate_audience(result: AudienceResult, context: dict[str, Any]) -> list[str]:
    errors: list[str] = []
    known_refs = _known_refs(context)
    if result.mode == "none" and (result.status != "blocked" or result.segments):
        errors.append("mode=none requires status=blocked and segments=[]")
    if result.mode == "hypothesis_only":
        if result.status != "partial":
            errors.append("mode=hypothesis_only requires status=partial")
        if any(segment.origin != "proposed" for segment in result.segments):
            errors.append("hypothesis_only segments must have origin=proposed")
    if result.status == "complete" and result.mode != "data_based":
        errors.append("status=complete requires mode=data_based")
    if len(result.questions) > 3:
        errors.append("questions must contain at most three items")

    ids: set[str] = set()
    for segment in result.segments:
        if not re.fullmatch(r"seg_[0-9]{3,}", segment.segment_id):
            errors.append(f"Invalid segment_id: {segment.segment_id}")
        if segment.segment_id in ids:
            errors.append(f"Duplicate segment_id: {segment.segment_id}")
        ids.add(segment.segment_id)
        if result.mode == "hypothesis_only" and (
            segment.size.count is not None
            or segment.size.base_count is not None
            or segment.size.share is not None
            or segment.size.source_refs
        ):
            errors.append(f"Proposed segment has observed size or refs: {segment.segment_id}")
        if segment.size.count is not None and segment.size.base_count is not None and segment.size.count > segment.size.base_count:
            errors.append(f"Segment count exceeds base_count: {segment.segment_id}")
        if segment.size.share is not None and segment.size.count is not None and segment.size.base_count:
            expected = segment.size.count / segment.size.base_count
            if abs(segment.size.share - expected) > 0.01:
                errors.append(f"Segment share does not match count/base_count: {segment.segment_id}")
        check_names = [check.name for check in segment.checks]
        if sorted(check_names) != sorted(_CHECK_NAMES) or len(set(check_names)) != len(_CHECK_NAMES):
            errors.append(f"Segment must contain each of the eight checks exactly once: {segment.segment_id}")
        if result.status == "complete" and any(check.status != "pass" for check in segment.checks):
            errors.append(f"Complete result requires all segment checks to pass: {segment.segment_id}")
        for claim in _claims(segment):
            errors.extend(_validate_claim(claim, known_refs, f"{segment.segment_id} claim"))
        for check in segment.checks:
            errors.extend(_validate_refs(check.source_refs, known_refs, f"{segment.segment_id} check {check.name}"))

    errors.extend(_validate_claim(result.data_profile.coverage, known_refs, "data_profile.coverage"))
    for feature in result.features_used:
        errors.extend(_validate_refs(feature.source_refs, known_refs, f"feature {feature.name}"))
    for claim in _claims_from_result(result):
        errors.extend(_validate_claim(claim, known_refs, "result claim"))
    return list(dict.fromkeys(errors))


_CHECK_NAMES = [
    "data_quality",
    "relevance",
    "reproducibility",
    "min_size",
    "stability",
    "privacy_bias",
    "leakage",
    "activation",
]


def _claims(segment: Any) -> list[Any]:
    interpretation = segment.interpretation
    values = list(segment.characteristics)
    for field in ("context", "jtbd", "pains", "needs", "motivations", "triggers", "barriers", "doubts"):
        value = getattr(interpretation, field)
        values.extend(value if isinstance(value, list) else [value])
    return values


def _claims_from_result(result: AudienceResult) -> list[Any]:
    return [segment.size for segment in result.segments if False]


def _validate_claim(claim: Any, known_refs: set[str], label: str) -> list[str]:
    if not hasattr(claim, "kind"):
        return []
    errors: list[str] = []
    if claim.kind == "unknown" and (claim.text is not None or claim.source_refs):
        errors.append(f"Unknown claim must have null text and no source_refs: {label}")
    if claim.kind in {"fact", "inference"} and not claim.source_refs:
        errors.append(f"{claim.kind} claim requires source_refs: {label}")
    if claim.kind == "inference" and not claim.rationale:
        errors.append(f"Inference claim requires rationale: {label}")
    errors.extend(_validate_refs(claim.source_refs, known_refs, label))
    return errors


def _validate_refs(refs: list[str], known_refs: set[str], label: str) -> list[str]:
    errors = []
    for ref in refs:
        if not any(ref == known or ref.startswith(known + "#") for known in known_refs):
            errors.append(f"Unknown source_ref {ref}: {label}")
    return errors


def _known_refs(context: dict[str, Any]) -> set[str]:
    refs: set[str] = set()
    for key in ("product_id", "request_id"):
        if isinstance(context.get(key), str):
            refs.add(context[key])
    for source in context.get("audience_sources", []):
        if isinstance(source, dict):
            for key in ("source_id", "snapshot_ref", "source_ref"):
                if isinstance(source.get(key), str):
                    refs.add(source[key])
    for key in ("tool_results", "calculation_results", "results"):
        values = context.get(key)
        if isinstance(values, dict):
            refs.update(str(item) for item in values)
    passport = context.get("product_passport")
    if isinstance(passport, dict):
        for fact in passport.get("facts", []):
            if isinstance(fact, dict):
                for key in ("source_ref", "source_id"):
                    if isinstance(fact.get(key), str):
                        refs.add(fact[key])
    return refs
