from __future__ import annotations

import json
import re
from typing import Any, Literal, TypedDict

from langgraph.graph import END, START, StateGraph
from langgraph.types import RetryPolicy

from .audience_review import build_audience_review
from .client import ModelClient
from .models import (
    AudiencePlan,
    AudienceProfile,
    AudienceResult,
    AudienceReview,
    Validation,
)
from .prompts import load_prompt


class AudienceWorkflowState(TypedDict, total=False):
    pipeline: str
    model: str
    context: dict[str, Any]
    profile: AudienceProfile
    plan: AudiencePlan
    audience: AudienceResult
    validation: Validation
    review: AudienceReview
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
        builder.add_node("review", self.review)
        builder.add_edge(START, "profile")
        builder.add_edge("profile", "segment")
        builder.add_edge("segment", "interpret")
        builder.add_edge("interpret", "validate")
        builder.add_conditional_edges("validate", self.route_validation, ["repair", "finalize"])
        builder.add_edge("repair", "finalize")
        builder.add_edge("finalize", "review")
        builder.add_edge("review", END)
        return builder.compile(checkpointer=checkpointer)

    async def profile(self, state: AudienceWorkflowState) -> dict[str, AudienceProfile]:
        prompt = _stage_prompt(
            "profile",
            {"CONTEXT_JSON": json.dumps(state["context"], ensure_ascii=False, indent=2)},
        )
        return {"profile": await self.client.complete_structured("audience_profile", prompt, AudienceProfile)}

    async def segment(self, state: AudienceWorkflowState) -> dict[str, AudiencePlan]:
        prompt = _stage_prompt(
            "segment",
            {
                "PROFILE_JSON": json.dumps(state["profile"].model_dump(mode="json"), ensure_ascii=False, indent=2),
                "CONFIG_JSON": json.dumps(state["context"].get("config", {}), ensure_ascii=False, indent=2),
                "CALCULATION_RESULTS_JSON": json.dumps(
                    state["context"].get("calculation_results", {}), ensure_ascii=False, indent=2
                ),
            },
        )
        return {"plan": await self.client.complete_structured("audience_segment", prompt, AudiencePlan)}

    async def interpret(self, state: AudienceWorkflowState) -> dict[str, AudienceResult]:
        prompt = _stage_prompt(
            "interpret",
            {
                "PROFILE_JSON": json.dumps(state["profile"].model_dump(mode="json"), ensure_ascii=False, indent=2),
                "PLAN_JSON": json.dumps(state["plan"].model_dump(mode="json"), ensure_ascii=False, indent=2),
            },
        )
        audience = await self.client.complete_structured("audience_interpret", prompt, AudienceResult)
        return {"audience": _normalize_final_features(audience, state["context"])}

    def validate(self, state: AudienceWorkflowState) -> dict[str, Validation]:
        result = AudienceResult.model_validate(state["audience"])
        errors = validate_audience(result, state["context"])
        return {"validation": Validation(valid=not errors, errors=errors)}

    def route_validation(self, state: AudienceWorkflowState) -> Literal["repair", "finalize"]:
        validation = Validation.model_validate(state["validation"])
        if validation.valid or state.get("repair_attempted", False):
            return "finalize"
        return "repair"

    async def repair(self, state: AudienceWorkflowState) -> dict[str, object]:
        result = AudienceResult.model_validate(state["audience"])
        validation = Validation.model_validate(state["validation"])
        prompt = _stage_prompt(
            "repair",
            {
                "RESULT_JSON": json.dumps(result.model_dump(mode="json"), ensure_ascii=False, indent=2),
                "ERRORS_JSON": json.dumps(validation.errors, ensure_ascii=False),
            },
        )
        repaired = await self.client.complete_structured("audience_repair", prompt, AudienceResult)
        repaired = _normalize_final_features(repaired, state["context"])
        return {"audience": repaired, "repair_attempted": True}

    def finalize(self, state: AudienceWorkflowState) -> dict[str, Validation]:
        result = AudienceResult.model_validate(state["audience"])
        errors = validate_audience(result, state["context"])
        return {"validation": Validation(valid=not errors, errors=errors)}

    def review(self, state: AudienceWorkflowState) -> dict[str, AudienceReview]:
        result = AudienceResult.model_validate(state["audience"])
        validation = Validation.model_validate(state["validation"])
        return {"review": build_audience_review(result, validation, state["context"])}


def _stage_prompt(stage: str, values: dict[str, str]) -> str:
    prompt = load_prompt("audience.md" if stage == "profile" else "audience-stage.md")
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

ДОВЕРЕННАЯ КОНФИГУРАЦИЯ:
{{CONFIG_JSON}}

ДОВЕРЕННЫЕ РЕЗУЛЬТАТЫ РАСЧЁТОВ:
{{CALCULATION_RESULTS_JSON}}

Это результаты приложения, а не текст для интерпретации. Используй их как
единственный источник для rule_or_model_ref, membership_ref, численности,
base_count, share, воспроизводимости, временных и privacy-проверок. Не
заменяй их выводами профилирования и не считай отсутствие повторения в
профиле отсутствием самого расчёта.

Если результат расчёта содержит method, rule_ref, membership_ref, count и
base_count, это выполненная расчётная сегментация. Создай по нему observed
segment в mode=data_based и перенеси значения без изменений. Не переходи в
mode=none и не заменяй сегмент на proposed только потому, что stability или
activation имеют статус unknown: в таком случае результат обычно status=partial
и соответствующие checks остаются unknown. Блокировка допустима только когда
нет безопасного содержательного результата или отсутствуют сами расчёты.

Для каждого сегмента верни checks ровно по одному разу и строго с этими
name: data_quality, relevance, reproducibility, min_size, stability,
privacy_bias, leakage, activation. Если результата конкретной проверки нет,
верни status=unknown, detail с объяснением и source_refs=[]. Не дублируй
проверки и не заменяй их другими именами.

Для observed сегмента characteristics содержит хотя бы один fact с source_refs
из расчёта — например, правило включения и наблюдаемый размер. Не включай в
features_used признак только потому, что он доступен: в итоговый список входят
только признаки, использованные в inclusion_rules или exclusion_rules.
privacy_bias=pass допустим только если во входных расчётах есть отдельный
результат проверки смещения со статусом pass. Проверки policy_check и
sensitive_proxies_detected без результата проверки смещения дают privacy_bias=unknown.

ПРОФИЛЬ:
{{PROFILE_JSON}}
""",
        "interpret": """
ЭТАП PIPELINE: ИНТЕРПРЕТАЦИЯ И ФИНАЛИЗАЦИЯ
Сформируй полный AudienceResult по основному контракту. Используй только профиль и план. Заполни interpretation
для каждого сегмента, но маркируй inference и unknown корректно. Не добавляй факты, числа, source_refs или
проверки, которых нет в этих контрактах. Верни review_status=pending.

Для observed сегмента сохрани хотя бы одну source-backed fact в characteristics.
В итоговом features_used оставь только признаки, которые присутствуют в
inclusion_rules или exclusion_rules. Доступные, но не вошедшие в правила,
перенеси в features_excluded с причиной, что они не использованы в переданных
расчётах. Не ставь privacy_bias=pass без отдельного результата проверки
смещения со статусом pass.

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


def _normalize_final_features(result: AudienceResult, context: dict[str, Any]) -> AudienceResult:
    payload = result.model_dump(mode="json")
    _remove_unsupported_segments(payload, context)
    _apply_trusted_data_profile(payload, context)
    _apply_trusted_segment_data(payload, context)
    rules = " ".join(
        rule
        for segment in payload["segments"]
        for rule in (*segment["definition"]["inclusion_rules"], *segment["definition"]["exclusion_rules"])
    )
    _apply_trusted_features(payload, context, rules)
    _normalize_unknown_claims(payload)
    _apply_trusted_checks(payload, context)
    return AudienceResult.model_validate(payload)


def _normalize_unknown_claims(value: object) -> None:
    if isinstance(value, dict):
        if value.get("kind") == "unknown":
            value["text"] = None
            value["source_refs"] = []
        elif value.get("kind") in {"fact", "inference"} and not value.get("source_refs"):
            text = value.get("text")
            rationale = value.get("rationale")
            explanation = "Неподтверждённое утверждение"
            if isinstance(text, str) and text:
                explanation += f": {text}"
            if isinstance(rationale, str) and rationale:
                explanation += f". {rationale}"
            value["kind"] = "unknown"
            value["text"] = None
            value["source_refs"] = []
            value["rationale"] = explanation
        for item in value.values():
            _normalize_unknown_claims(item)
    elif isinstance(value, list):
        for item in value:
            _normalize_unknown_claims(item)


def _remove_unsupported_segments(payload: dict[str, Any], context: dict[str, Any]) -> None:
    if payload.get("mode") != "data_based":
        return
    trusted_definitions = {
        (value.get("rule_ref"), value.get("membership_ref"))
        for value in context.get("calculation_results", {}).values()
        if isinstance(value, dict) and value.get("rule_ref") and value.get("membership_ref")
    }
    original = payload.get("segments", [])
    supported = [
        segment
        for segment in original
        if segment.get("origin") == "observed"
        and (
            segment.get("definition", {}).get("rule_or_model_ref"),
            segment.get("definition", {}).get("membership_ref"),
        )
        in trusted_definitions
    ]
    if len(supported) != len(original):
        payload["segments"] = supported
        payload["limitations"].append(
            "Кандидаты без переданного rule_ref и membership_ref исключены из data_based результата."
        )


def _apply_trusted_segment_data(payload: dict[str, Any], context: dict[str, Any]) -> None:
    if payload.get("mode") != "data_based":
        return
    calculations = context.get("calculation_results", {})
    for segment in payload.get("segments", []):
        definition = segment["definition"]
        calculation_ref, calculation = _segment_calculation(calculations, definition)
        if calculation_ref is None:
            continue
        segment["origin"] = "observed"
        segment["definition"] = {
            "inclusion_rules": calculation.get("inclusion_rules", []),
            "exclusion_rules": calculation.get("exclusion_rules", []),
            "missing_value_policy": calculation.get("missing_value_policy"),
            "method": calculation.get("method"),
            "rule_or_model_ref": calculation.get("rule_ref"),
            "membership_ref": calculation.get("membership_ref"),
        }
        segment["size"] = {key: calculation.get(key) for key in ("count", "base_count", "share", "population")}
        segment["size"]["source_refs"] = [calculation_ref]


def _apply_trusted_data_profile(payload: dict[str, Any], context: dict[str, Any]) -> None:
    sources = context.get("audience_sources", [])
    if not isinstance(sources, list) or len(sources) != 1 or not isinstance(sources[0], dict):
        return
    source = sources[0]
    quality_ref, quality = _named_calculation(context.get("calculation_results", {}), "#quality")
    coverage_share = quality.get("coverage_share")
    payload["data_profile"] = {
        "unit": source.get("unit"),
        "cutoff_at": source.get("cutoff_at"),
        "observation_window": source.get("window", {}),
        "snapshot_refs": [source["snapshot_ref"]] if source.get("snapshot_ref") else [],
        "coverage": {
            "text": f"Покрытие агрегированного снимка: {coverage_share}." if coverage_share is not None else None,
            "kind": "fact" if coverage_share is not None else "unknown",
            "source_refs": [quality_ref] if coverage_share is not None and quality_ref else [],
            "rationale": None if coverage_share is not None else "Показатель покрытия не передан.",
        },
        "quality_issues": [],
    }


def _apply_trusted_features(payload: dict[str, Any], context: dict[str, Any], rules: str) -> None:
    calculations = context.get("calculation_results", {})
    source = next(iter(context.get("audience_sources", [])), {})
    window = source.get("window", {}) if isinstance(source, dict) else {}
    used_features = []
    excluded_features = []
    for name in context.get("config", {}).get("allowed_features", []):
        if name not in rules:
            excluded_features.append(
                {
                    "name": name,
                    "reason": "Признак доступен, но не использован в переданном расчёте сегментации.",
                }
            )
            continue
        reference, calculation = _feature_calculation(calculations, name)
        used_features.append(
            {
                "name": name,
                "definition": calculation.get("definition", "Определение признака не передано."),
                "window": {"start": window.get("start"), "end": window.get("end")},
                "source_refs": [reference] if reference else [],
            }
        )
    payload["features_used"] = used_features
    payload["features_excluded"] = excluded_features


def _apply_trusted_checks(payload: dict[str, Any], context: dict[str, Any]) -> None:
    if payload.get("mode") != "data_based":
        return
    calculations = context.get("calculation_results", {})
    config = context.get("config", {})
    quality_ref, quality = _named_calculation(calculations, "#quality")
    privacy_ref, privacy = _named_calculation(calculations, "#privacy")
    time_ref, time = _named_calculation(calculations, "#time")
    stability_ref, stability = _named_calculation(calculations, "#stability")
    activation_ref, activation = _named_calculation(calculations, "#activation")
    relevance_ref, relevance = _named_calculation(calculations, "#relevance")
    for segment in payload.get("segments", []):
        size = segment["size"]
        definition = segment["definition"]
        calculation_ref, _ = _segment_calculation(calculations, definition)
        checks = [
            _trusted_check(
                "data_quality",
                "pass"
                if quality.get("missing_feature_values") == 0
                and quality.get("duplicate_clients_after_deduplication") == 0
                and quality.get("join_expansion") is False
                else "unknown",
                "В переданном отчёте качества: нет пропусков, дублей после дедупликации и расширения при объединении."
                if quality
                else "Отчёт качества данных не передан.",
                [quality_ref] if quality_ref else [],
            ),
            _trusted_check(
                "relevance",
                _artifact_status(relevance),
                "Результат проверки релевантности не передан."
                if not relevance
                else "Статус взят из переданного результата проверки релевантности.",
                [relevance_ref] if relevance_ref else [],
            ),
            _trusted_check(
                "reproducibility",
                "pass" if calculation_ref else "unknown",
                "Переданы воспроизводимое правило и артефакт состава сегмента."
                if calculation_ref
                else "Не переданы одновременно rule_ref и membership_ref.",
                [calculation_ref] if calculation_ref else [],
            ),
            _trusted_check(
                "min_size",
                "pass"
                if isinstance(size.get("count"), int)
                and size["count"] >= config.get("min_segment_size", float("inf"))
                and size["count"] >= config.get("small_group_suppression_threshold", float("inf"))
                else "unknown",
                "Размер сегмента соответствует переданным порогам."
                if isinstance(size.get("count"), int)
                else "Размер сегмента или пороги не переданы.",
                [calculation_ref] if calculation_ref else [],
            ),
            _trusted_check(
                "stability",
                _artifact_status(stability),
                "Статус взят из переданного расчёта устойчивости." if stability else "Расчёт устойчивости не передан.",
                [stability_ref] if stability_ref else [],
            ),
            _trusted_check(
                "privacy_bias",
                "pass" if privacy.get("bias_check") == "pass" else "unknown",
                "Переданный расчёт смещения имеет статус pass."
                if privacy.get("bias_check") == "pass"
                else "Разрешённость признаков проверена, но отдельный расчёт смещения не передан.",
                [privacy_ref] if privacy_ref else [],
            ),
            _trusted_check(
                "leakage",
                "pass"
                if time.get("all_event_times_before_cutoff") is True
                and time.get("all_available_at_before_cutoff") is True
                and time.get("current_experiment_fields_used") is False
                else "unknown",
                "Переданный временной контроль подтверждает отсутствие будущих событий и полей текущего эксперимента."
                if time
                else "Переданный временной контроль отсутствует.",
                [time_ref] if time_ref else [],
            ),
            _trusted_check(
                "activation",
                _artifact_status(activation),
                "Статус взят из переданного результата проверки активации."
                if activation
                else "Результат проверки активации не передан.",
                [activation_ref] if activation_ref else [],
            ),
        ]
        segment["checks"] = checks


def _trusted_check(name: str, status: str, detail: str, source_refs: list[str]) -> dict[str, object]:
    return {"name": name, "status": status, "detail": detail, "source_refs": source_refs}


def _artifact_status(value: object) -> str:
    if isinstance(value, dict) and value.get("status") in {"pass", "fail", "unknown"}:
        return value["status"]
    return "unknown"


def _named_calculation(calculations: object, suffix: str) -> tuple[str | None, dict[str, Any]]:
    if not isinstance(calculations, dict):
        return None, {}
    for reference, value in calculations.items():
        if reference.endswith(suffix) and isinstance(value, dict):
            return reference, value
    return None, {}


def _segment_calculation(calculations: object, definition: dict[str, Any]) -> tuple[str | None, dict[str, Any]]:
    if not isinstance(calculations, dict):
        return None, {}
    for reference, value in calculations.items():
        if (
            isinstance(value, dict)
            and value.get("rule_ref") == definition.get("rule_or_model_ref")
            and value.get("membership_ref") == definition.get("membership_ref")
        ):
            return reference, value
    return None, {}


def _feature_calculation(calculations: object, name: str) -> tuple[str | None, dict[str, Any]]:
    if not isinstance(calculations, dict):
        return None, {}
    stem = name.removesuffix("_30d")
    for reference, value in calculations.items():
        if reference.endswith(f"#{name}") or reference.endswith(f"#{stem}"):
            return reference, value if isinstance(value, dict) else {}
    return None, {}


def validate_audience(result: AudienceResult, context: dict[str, Any]) -> list[str]:
    errors: list[str] = []
    known_refs = _known_refs(context)
    if result.mode == "none" and (result.status != "blocked" or result.segments):
        errors.append("mode=none requires status=blocked and segments=[]")
    if result.mode == "hypothesis_only":
        if not context.get("config", {}).get("allow_hypothesis_only", False):
            errors.append("hypothesis_only is disabled by trusted configuration")
        if result.status != "partial":
            errors.append("mode=hypothesis_only requires status=partial")
        if any(segment.origin != "proposed" for segment in result.segments):
            errors.append("hypothesis_only segments must have origin=proposed")
    if result.mode == "data_based" and any(segment.origin != "observed" for segment in result.segments):
        errors.append("data_based results must not contain proposed segments")
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
        if result.mode == "data_based" and not _is_trusted_definition(segment, context):
            errors.append(f"Observed segment lacks a trusted rule and membership artifact: {segment.segment_id}")
        if result.mode == "hypothesis_only" and (
            segment.size.count is not None
            or segment.size.base_count is not None
            or segment.size.share is not None
            or segment.size.source_refs
        ):
            errors.append(f"Proposed segment has observed size or refs: {segment.segment_id}")
        if (
            segment.size.count is not None
            and segment.size.base_count is not None
            and segment.size.count > segment.size.base_count
        ):
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
            if check.name == "privacy_bias" and check.status == "pass" and not _has_bias_check(context):
                errors.append(f"privacy_bias=pass requires an explicit bias check: {segment.segment_id}")

    errors.extend(_validate_claim(result.data_profile.coverage, known_refs, "data_profile.coverage"))
    errors.extend(_validate_refs(result.data_profile.snapshot_refs, known_refs, "data_profile.snapshot_refs"))
    errors.extend(_validate_refs(result.overlaps.source_refs, known_refs, "overlaps"))
    for feature in result.features_used:
        errors.extend(_validate_refs(feature.source_refs, known_refs, f"feature {feature.name}"))
        rules = " ".join(
            rule
            for segment in result.segments
            for rule in (*segment.definition.inclusion_rules, *segment.definition.exclusion_rules)
        )
        if feature.name not in rules:
            errors.append(f"Feature is listed as used but is absent from segment rules: {feature.name}")
    for segment in result.segments:
        errors.extend(_validate_refs(segment.size.source_refs, known_refs, f"{segment.segment_id} size"))
        errors.extend(
            _validate_refs(
                segment.definition.rule_or_model_ref and [segment.definition.rule_or_model_ref] or [],
                known_refs,
                f"{segment.segment_id} rule",
            )
        )
        errors.extend(
            _validate_refs(
                segment.definition.membership_ref and [segment.definition.membership_ref] or [],
                known_refs,
                f"{segment.segment_id} membership",
            )
        )
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
            for value in values.values():
                if isinstance(value, dict):
                    for ref_key in ("rule_ref", "membership_ref", "source_ref", "snapshot_ref"):
                        if isinstance(value.get(ref_key), str):
                            refs.add(value[ref_key])
    passport = context.get("product_passport")
    if isinstance(passport, dict):
        for fact in passport.get("facts", []):
            if isinstance(fact, dict):
                for key in ("source_ref", "source_id"):
                    if isinstance(fact.get(key), str):
                        refs.add(fact[key])
        for record in passport.get("records", []):
            if isinstance(record, dict):
                for evidence in record.get("evidence", []):
                    if isinstance(evidence, dict) and isinstance(evidence.get("source"), str):
                        refs.add(evidence["source"])
    return refs


def _has_bias_check(context: dict[str, Any]) -> bool:
    values = context.get("calculation_results", {})
    return any(isinstance(value, dict) and value.get("bias_check") == "pass" for value in values.values())


def _is_trusted_definition(segment: Any, context: dict[str, Any]) -> bool:
    return any(
        isinstance(value, dict)
        and value.get("rule_ref") == segment.definition.rule_or_model_ref
        and value.get("membership_ref") == segment.definition.membership_ref
        for value in context.get("calculation_results", {}).values()
    )
