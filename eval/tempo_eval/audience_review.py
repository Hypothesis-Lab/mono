from __future__ import annotations

from typing import Any

from .models import AudienceResult, AudienceReview, AudienceReviewCheck, Validation


def build_audience_review(result: AudienceResult, validation: Validation, context: dict[str, Any]) -> AudienceReview:
    checks = {check.name: check for segment in result.segments for check in segment.checks}
    config = context.get("config", {})
    critic_checks = [
        _from_segment_check("segment_relevance", checks.get("relevance")),
        _sensitive_features_and_bias(result, config, context),
        _segment_count(result, config),
        _interpretability(result),
        _from_segment_check("target_information_leakage", checks.get("leakage")),
    ]
    validator_checks = [
        _from_segment_check("data_availability_and_quality", checks.get("data_quality")),
        _pii_deidentification(context),
        _from_segment_check("minimum_group_size", checks.get("min_size")),
        _from_segment_check("stability_across_periods_or_samples", checks.get("stability")),
        _from_segment_check("channel_availability", checks.get("activation")),
        AudienceReviewCheck(
            name="result_contract",
            status="pass" if validation.valid else "fail",
            detail="Детерминированная проверка контракта пройдена."
            if validation.valid
            else "Детерминированная проверка контракта не пройдена: " + "; ".join(validation.errors),
            source_refs=[],
        ),
    ]
    critic_checks.extend(_agent_checks(context.get("ai_critic"), "ai_critic"))
    validator_checks.extend(_agent_checks(context.get("ai_validator"), "ai_validator"))
    privacy = context.get("ai_privacy")
    if privacy:
        privacy = privacy if isinstance(privacy, dict) else privacy.model_dump(mode="json")
        validator_checks.append(
            AudienceReviewCheck(
                name="ai_privacy_review",
                status=privacy["status"],
                detail=privacy["detail"],
                source_refs=[],
            )
        )
    return AudienceReview(
        critic_checks=critic_checks,
        validator_checks=validator_checks,
        product_manager_confirmation={
            "status": "pending",
            "detail": "Подтверждение продакт-менеджера — внешнее событие; модель и валидаторы его не имитируют.",
        },
    )


def _agent_checks(value: Any, source: str) -> list[AudienceReviewCheck]:
    if not value:
        return []
    payload = value if isinstance(value, dict) else value.model_dump(mode="json")
    checks = [
        AudienceReviewCheck(
            name=f"{source}:{check['name']}",
            status=check["status"],
            detail=check["detail"] + (f" Причина: {check['rationale']}" if check.get("rationale") else ""),
            source_refs=[],
        )
        for check in payload.get("checks", [])
    ]
    if payload.get("overall_status") == "fail" and not any(check.status == "fail" for check in checks):
        checks.append(
            AudienceReviewCheck(
                name=f"{source}:overall",
                status="fail",
                detail="AI-агент сообщил о блокирующей проблеме: " + "; ".join(payload.get("blocking_issues", [])),
                source_refs=[],
            )
        )
    return checks


def _from_segment_check(name: str, check: Any) -> AudienceReviewCheck:
    if check is None:
        return AudienceReviewCheck(
            name=name,
            status="unknown",
            detail="Проверка не была передана для сформированных сегментов.",
            source_refs=[],
        )
    return AudienceReviewCheck(name=name, status=check.status, detail=check.detail, source_refs=check.source_refs)


def _sensitive_features_and_bias(
    result: AudienceResult, config: dict[str, Any], context: dict[str, Any]
) -> AudienceReviewCheck:
    allowed = set(config.get("allowed_features", []))
    prohibited = set(config.get("prohibited_features", []))
    used = {feature.name for feature in result.features_used}
    if used - allowed or used & prohibited:
        return AudienceReviewCheck(
            name="sensitive_features_and_bias",
            status="fail",
            detail="В сегментации использован запрещённый или неразрешённый признак.",
            source_refs=[],
        )
    bias_results = [
        value
        for value in context.get("calculation_results", {}).values()
        if isinstance(value, dict) and "bias_check" in value
    ]
    if any(result.get("bias_check") == "fail" for result in bias_results):
        return AudienceReviewCheck(
            name="sensitive_features_and_bias",
            status="fail",
            detail="Переданный расчёт проверки смещения завершился со статусом fail.",
            source_refs=[],
        )
    if not any(result.get("bias_check") == "pass" for result in bias_results):
        return AudienceReviewCheck(
            name="sensitive_features_and_bias",
            status="unknown",
            detail="Разрешённость признаков соблюдена, но отдельный расчёт смещения не передан.",
            source_refs=[],
        )
    return AudienceReviewCheck(
        name="sensitive_features_and_bias",
        status="pass",
        detail="Использованы разрешённые признаки; переданный расчёт смещения имеет статус pass.",
        source_refs=[],
    )


def _segment_count(result: AudienceResult, config: dict[str, Any]) -> AudienceReviewCheck:
    maximum = config.get("max_segments")
    if not isinstance(maximum, int):
        return AudienceReviewCheck(
            name="segment_count", status="unknown", detail="Лимит max_segments не задан.", source_refs=[]
        )
    status = "pass" if len(result.segments) <= maximum else "fail"
    return AudienceReviewCheck(
        name="segment_count",
        status=status,
        detail=f"Сформировано сегментов: {len(result.segments)}; max_segments: {maximum}.",
        source_refs=[],
    )


def _interpretability(result: AudienceResult) -> AudienceReviewCheck:
    if not result.segments:
        return AudienceReviewCheck(
            name="interpretability", status="unknown", detail="Сегменты не сформированы.", source_refs=[]
        )
    for segment in result.segments:
        definition = segment.definition
        if not definition.method or not definition.rule_or_model_ref or not definition.membership_ref:
            return AudienceReviewCheck(
                name="interpretability",
                status="fail",
                detail=f"У сегмента {segment.segment_id} нет полного воспроизводимого определения.",
                source_refs=[],
            )
        if not any(claim.kind == "fact" and claim.source_refs for claim in segment.characteristics):
            return AudienceReviewCheck(
                name="interpretability",
                status="fail",
                detail=f"У сегмента {segment.segment_id} нет подтверждённой наблюдаемой характеристики.",
                source_refs=[],
            )
    return AudienceReviewCheck(
        name="interpretability",
        status="pass",
        detail="У всех сегментов есть правило, ссылка на состав и подтверждённая характеристика.",
        source_refs=[],
    )


def _pii_deidentification(context: dict[str, Any]) -> AudienceReviewCheck:
    forbidden_keys = {
        "client_records",
        "raw_records",
        "membership_list",
        "contacts",
        "emails",
        "phones",
        "personal_data",
    }
    found = _find_forbidden_keys(context, forbidden_keys)
    if found:
        return AudienceReviewCheck(
            name="pii_and_deidentification",
            status="fail",
            detail="В LLM-контексте обнаружены запрещённые поля: " + ", ".join(sorted(found)),
            source_refs=[],
        )
    return AudienceReviewCheck(
        name="pii_and_deidentification",
        status="pass",
        detail="В переданном LLM-контексте нет полей сырых записей, контактов или списков участников.",
        source_refs=[],
    )


def _find_forbidden_keys(value: Any, forbidden: set[str]) -> set[str]:
    if isinstance(value, dict):
        return set(value).intersection(forbidden) | set().union(
            *(_find_forbidden_keys(item, forbidden) for item in value.values())
        )
    if isinstance(value, list):
        return set().union(*(_find_forbidden_keys(item, forbidden) for item in value)) if value else set()
    return set()
