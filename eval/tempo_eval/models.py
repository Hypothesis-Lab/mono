from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, ConfigDict, Field, model_validator

Category = Literal[
    "identity",
    "purpose",
    "jtbd",
    "audience",
    "features",
    "benefits",
    "pricing",
    "terms",
    "limitations",
    "economics",
    "funnel",
    "offers_channels",
    "communication_rules",
]
StatementType = Literal["description", "claim", "reported_result", "plan", "target", "rule"]
ExtractionStatus = Literal["complete", "partial", "blocked"]


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid")


class Evidence(StrictModel):
    source: str
    locator: str | None
    quote: str


class Record(StrictModel):
    id: str
    category: Category
    subject: str
    statement: str
    statement_type: StatementType
    conditions: list[str]
    time_scope: str | None
    evidence_status: Literal["supported", "conflicted"]
    evidence: list[Evidence] = Field(min_length=1)


class Conflict(StrictModel):
    record_ids: list[str] = Field(min_length=2)
    description: str
    needed_to_resolve: str


class Gap(StrictModel):
    category: str
    missing: str


class Passport(StrictModel):
    product_name: str | None
    extraction_status: ExtractionStatus
    records: list[Record]
    conflicts: list[Conflict]
    gaps: list[Gap]
    warnings: list[str]


class EvaluationCriterion(StrictModel):
    id: str
    name: str
    weight: float
    score: float | None = Field(ge=0, le=5, default=None)
    status: Literal["assessed", "not_tested"]
    weighted_score: float | None = Field(ge=0, le=100, default=None)
    findings: list[str]
    recommendations: list[str]


class MandatoryCheck(StrictModel):
    id: str
    name: str
    status: Literal["pass", "fail", "not_tested"]
    finding: str


class ScenarioResult(StrictModel):
    number: int = Field(ge=1, le=12)
    status: Literal["pass", "fail", "partial", "not_tested"]
    finding: str


class Evaluation(StrictModel):
    evaluation_status: ExtractionStatus
    mode: Literal["single_run_source_audit", "benchmark"]
    overall_score: float | None = Field(ge=0, le=100, default=None)
    acceptance: Literal["accepted", "rejected", "not_tested"]
    score_basis: str
    criteria: list[EvaluationCriterion] = Field(min_length=9)
    mandatory_checks: list[MandatoryCheck] = Field(min_length=6)
    scenarios: list[ScenarioResult] = Field(min_length=12)
    summary: str
    limitations: list[str]


class StabilityEvaluation(StrictModel):
    evaluation_status: ExtractionStatus
    mode: Literal["benchmark"]
    overall_score: float | None = Field(ge=0, le=100, default=None)
    acceptance: Literal["accepted", "rejected", "not_tested"]
    score_basis: str
    criterion: EvaluationCriterion
    run_comparison: list[str] = Field(min_length=3)
    summary: str
    limitations: list[str]


class GlobalAudienceComparison(StrictModel):
    overall_status: Literal["complete", "partial", "blocked"]
    summary: str
    consensus: list[str]
    model_specific_differences: list[str]
    inconsistencies: list[str]
    shared_limitations: list[str]
    recommended_follow_ups: list[str]


class AudienceClaim(StrictModel):
    text: str | None
    kind: Literal["fact", "inference", "unknown"]
    source_refs: list[str]
    rationale: str | None
    claim_ref: str | None = None


class AudienceWindow(StrictModel):
    start: str | None
    end: str | None


class AudienceFeature(StrictModel):
    name: str
    definition: str
    window: AudienceWindow
    source_refs: list[str]


class ExcludedAudienceFeature(StrictModel):
    name: str
    reason: str


class AudienceDataProfile(StrictModel):
    unit: str | None
    cutoff_at: str | None
    observation_window: AudienceWindow
    snapshot_refs: list[str]
    coverage: AudienceClaim
    quality_issues: list[str]


class AudienceDefinition(StrictModel):
    inclusion_rules: list[str]
    exclusion_rules: list[str]
    missing_value_policy: str | None
    method: str | None
    rule_or_model_ref: str | None
    membership_ref: str | None


class AudienceSize(StrictModel):
    count: int | None = Field(ge=0, default=None)
    base_count: int | None = Field(ge=0, default=None)
    share: float | None = Field(ge=0, le=1, default=None)
    population: str | None
    source_refs: list[str]


AudienceCheckName = Literal[
    "data_quality",
    "relevance",
    "reproducibility",
    "min_size",
    "stability",
    "privacy_bias",
    "leakage",
    "activation",
]
AUDIENCE_CHECK_NAMES = frozenset(
    {
        "data_quality",
        "relevance",
        "reproducibility",
        "min_size",
        "stability",
        "privacy_bias",
        "leakage",
        "activation",
    }
)


class AudienceCheck(StrictModel):
    name: AudienceCheckName
    status: Literal["pass", "fail", "unknown"]
    detail: str
    source_refs: list[str]


def _normalized_checks(value: object) -> object:
    if not isinstance(value, list):
        return value
    by_name: dict[str, dict[object, object]] = {}
    for check in value:
        if isinstance(check, dict) and isinstance(check.get("name"), str) and check["name"] in AUDIENCE_CHECK_NAMES:
            by_name.setdefault(check["name"], check)
    return [
        by_name.get(
            name,
            {
                "name": name,
                "status": "unknown",
                "detail": "Результат проверки не передан.",
                "source_refs": [],
            },
        )
        for name in sorted(AUDIENCE_CHECK_NAMES)
    ]


def _normalized_segments(value: object) -> object:
    if not isinstance(value, dict) or not isinstance(value.get("segments"), list):
        return value
    result = dict(value)
    result["segments"] = [
        {**segment, "segment_id": f"seg_{index:03d}"} if isinstance(segment, dict) else segment
        for index, segment in enumerate(value["segments"], start=1)
    ]
    return result


class AudienceJTBDAlignment(StrictModel):
    """Comparison between one product job and one possible segment job."""

    product_jtbd_ref: str | None
    product_jtbd_text: str | None
    segment_jtbd_index: int | None
    segment_jtbd_text: str | None
    segment_jtbd_ref: str | None = None
    relation: Literal["aligned", "partial", "not_aligned", "unknown"]
    rationale: str
    source_refs: list[str] = Field(default_factory=list)


class AudienceInterpretation(StrictModel):
    context: AudienceClaim
    jtbd: list[AudienceClaim] = Field(default_factory=list)
    jtbd_alignment: list[AudienceJTBDAlignment] = Field(default_factory=list)
    pains: list[AudienceClaim]
    needs: list[AudienceClaim]
    readiness: list[AudienceClaim]
    motivations: list[AudienceClaim]
    triggers: list[AudienceClaim]
    barriers: list[AudienceClaim]
    doubts: list[AudienceClaim]

    @model_validator(mode="before")
    @classmethod
    def normalize_jtbd(cls, value: object) -> object:
        if not isinstance(value, dict):
            return value
        result = dict(value)
        if isinstance(result.get("jtbd"), dict):
            result["jtbd"] = [result["jtbd"]]
        result.setdefault("jtbd_alignment", [])
        return result


class AudienceSegment(StrictModel):
    segment_id: str = Field(pattern=r"^seg_[0-9]{3,}$")
    name: str
    origin: Literal["observed", "proposed"]
    definition: AudienceDefinition
    size: AudienceSize
    characteristics: list[AudienceClaim]
    interpretation: AudienceInterpretation
    checks: list[AudienceCheck] = Field(min_length=8, max_length=8)
    risks: list[str]

    @model_validator(mode="before")
    @classmethod
    def normalize_checks(cls, value: object) -> object:
        if not isinstance(value, dict):
            return value
        result = dict(value)
        result["checks"] = _normalized_checks(result.get("checks"))
        return result

    @model_validator(mode="after")
    def require_exactly_one_of_each_check(self) -> AudienceSegment:
        if {check.name for check in self.checks} != AUDIENCE_CHECK_NAMES:
            raise ValueError("checks must contain each required check exactly once")
        if self.origin == "observed" and not self.characteristics:
            raise ValueError("observed segments require at least one characteristic")
        return self


class AudienceOverlaps(StrictModel):
    status: Literal["measured", "unknown", "not_applicable"]
    detail: str
    source_refs: list[str]


class AudienceResult(StrictModel):
    schema_version: Literal["0.1"]
    request_id: str | None
    product_id: str | None
    status: ExtractionStatus
    mode: Literal["data_based", "hypothesis_only", "none"]
    data_profile: AudienceDataProfile
    features_used: list[AudienceFeature]
    features_excluded: list[ExcludedAudienceFeature]
    segments: list[AudienceSegment]
    overlaps: AudienceOverlaps
    missing_data: list[str]
    questions: list[str] = Field(max_length=3)
    limitations: list[str]
    review_status: Literal["pending"]
    configuration: dict[str, object] = Field(default_factory=dict)

    @model_validator(mode="before")
    @classmethod
    def normalize_segments(cls, value: object) -> object:
        return _normalized_segments(value)


class AudienceReviewCheck(StrictModel):
    name: str
    status: Literal["pass", "fail", "unknown"]
    detail: str
    source_refs: list[str]


class ProductManagerConfirmation(StrictModel):
    status: Literal["pending"]
    detail: str


class AudienceReview(StrictModel):
    critic_checks: list[AudienceReviewCheck]
    validator_checks: list[AudienceReviewCheck]
    product_manager_confirmation: ProductManagerConfirmation


class AudienceFilter(StrictModel):
    field: str
    value: str


class AudienceQueryCondition(StrictModel):
    name: str
    filters: list[AudienceFilter] = Field(min_length=1)


class AudienceQuery(StrictModel):
    name: str
    window_start: str
    window_end: str
    conditions: list[AudienceQueryCondition] = Field(min_length=1, max_length=5)


class AudienceQueryDecision(StrictModel):
    decision: Literal["use_result", "refine"]
    reason: str
    next_query: AudienceQuery | None


class AudienceNarrativeClaim(StrictModel):
    text: str | None
    kind: Literal["fact", "inference", "unknown"]
    rationale: str | None


class AudienceNarrative(StrictModel):
    name: str
    context: AudienceNarrativeClaim
    jtbd: list[AudienceNarrativeClaim] = Field(min_length=1, max_length=5)
    jtbd_alignment: list[AudienceJTBDAlignment] = Field(default_factory=list)
    pains: list[AudienceNarrativeClaim]
    needs: list[AudienceNarrativeClaim]
    readiness: list[AudienceNarrativeClaim]
    motivations: list[AudienceNarrativeClaim]
    triggers: list[AudienceNarrativeClaim]
    barriers: list[AudienceNarrativeClaim]
    doubts: list[AudienceNarrativeClaim]
    risks: list[str]
    questions: list[str] = Field(max_length=3)
    limitations: list[str]

    @model_validator(mode="before")
    @classmethod
    def normalize_jtbd(cls, value: object) -> object:
        if not isinstance(value, dict):
            return value
        result = dict(value)
        jtbd = result.get("jtbd")
        if isinstance(jtbd, dict):
            result["jtbd"] = [jtbd]
        elif not jtbd:
            result["jtbd"] = [
                {
                    "text": None,
                    "kind": "unknown",
                    "rationale": "Данные запуска не дают достаточного основания сформулировать JTBD сегмента.",
                }
            ]
        result.setdefault("jtbd_alignment", [])
        return result


class AudienceAgentCheck(StrictModel):
    name: str
    status: Literal["pass", "fail", "unknown"]
    detail: str
    rationale: str | None


class AudienceAgentReview(StrictModel):
    overall_status: Literal["pass", "fail", "unknown"]
    checks: list[AudienceAgentCheck] = Field(min_length=1)
    blocking_issues: list[str]


class AudienceEvidenceRequest(StrictModel):
    kind: Literal["condition_breakdown", "monthly_breakdown", "value_distribution", "condition_overlap"]
    condition_name: str | None
    field: str | None


class AudienceReviewDecision(StrictModel):
    decision: Literal["inspect", "final"]
    reason: str
    request: AudienceEvidenceRequest | None
    review: AudienceAgentReview | None


class AudiencePrivacyReview(StrictModel):
    status: Literal["pass", "fail", "unknown"]
    detail: str
    detected_categories: list[str]


class Source(StrictModel):
    source: str
    content: str


class Draft(StrictModel):
    source: str
    result: Passport


class Validation(StrictModel):
    valid: bool
    errors: list[str]
