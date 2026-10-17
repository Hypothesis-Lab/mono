from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, ConfigDict, Field

Category = Literal[
    "identity",
    "purpose",
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
    run_comparison: list[str]
    summary: str
    limitations: list[str]


class AudienceClaim(StrictModel):
    text: str | None
    kind: Literal["fact", "inference", "unknown"]
    source_refs: list[str]
    rationale: str | None


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


class AudienceCheck(StrictModel):
    name: AudienceCheckName
    status: Literal["pass", "fail", "unknown"]
    detail: str
    source_refs: list[str]


class AudienceInterpretation(StrictModel):
    context: AudienceClaim
    jtbd: AudienceClaim
    pains: list[AudienceClaim]
    needs: list[AudienceClaim]
    motivations: list[AudienceClaim]
    triggers: list[AudienceClaim]
    barriers: list[AudienceClaim]
    doubts: list[AudienceClaim]


class AudienceSegment(StrictModel):
    segment_id: str
    name: str
    origin: Literal["observed", "proposed"]
    definition: AudienceDefinition
    size: AudienceSize
    characteristics: list[AudienceClaim]
    interpretation: AudienceInterpretation
    checks: list[AudienceCheck]
    risks: list[str]


class AudienceOverlaps(StrictModel):
    status: Literal["measured", "unknown", "not_applicable"]
    detail: str
    source_refs: list[str]


class AudienceObservation(StrictModel):
    text: str
    source_refs: list[str]


class AudienceProfile(StrictModel):
    request_id: str | None
    product_id: str | None
    data_profile: AudienceDataProfile
    features_used: list[AudienceFeature]
    features_excluded: list[ExcludedAudienceFeature]
    observations: list[AudienceObservation]
    missing_data: list[str]
    questions: list[str] = Field(max_length=3)
    limitations: list[str]


class AudienceSegmentPlan(StrictModel):
    segment_id: str
    name: str
    origin: Literal["observed", "proposed"]
    definition: AudienceDefinition
    size: AudienceSize
    characteristics: list[AudienceClaim]
    checks: list[AudienceCheck]
    risks: list[str]


class AudiencePlan(StrictModel):
    status: ExtractionStatus
    mode: Literal["data_based", "hypothesis_only", "none"]
    segments: list[AudienceSegmentPlan]
    overlaps: AudienceOverlaps
    missing_data: list[str]
    questions: list[str] = Field(max_length=3)
    limitations: list[str]


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


class Source(StrictModel):
    source: str
    content: str


class Draft(StrictModel):
    source: str
    result: Passport


class Validation(StrictModel):
    valid: bool
    errors: list[str]
