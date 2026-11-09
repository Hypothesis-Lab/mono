from __future__ import annotations

from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid")


class HypothesisInput(StrictModel):
    product_passport: dict[str, Any]
    audience_result: dict[str, Any]
    candidate_count: int = Field(default=5, ge=1, le=12)
    validation_target: int = Field(default=3, ge=1)


class Claim(StrictModel):
    text: str | None
    kind: Literal["fact", "inference", "unknown"]
    source_refs: list[str]
    rationale: str


class Assumption(StrictModel):
    text: str
    basis_refs: list[str]
    rationale: str
    verification: str


class SegmentRef(StrictModel):
    segment_id: str
    version: str | None


class Change(StrictModel):
    object: str
    proposal: str
    control: Claim
    variable: str
    implementation_dependency: str | None


class ExpectedEffect(StrictModel):
    direction: Literal["increase", "decrease", "maintain"]
    amount: float | None
    unit: str | None
    amount_kind: Literal["provided_target", "scenario_estimate"] | None
    source_refs: list[str]
    assumptions: list[str]


class MetricValue(StrictModel):
    value: float | None
    unit: str | None
    source_refs: list[str]


class Metric(StrictModel):
    name: str
    definition: str
    formula: str | None
    unit_of_analysis: str | None
    event: str | None
    population: str | None
    window: str | None
    direction: Literal["increase", "decrease", "maintain"]
    availability: Literal["available", "requires_instrumentation", "unknown"]
    baseline: MetricValue
    threshold: MetricValue
    purpose: str
    source_refs: list[str]


class TestCandidate(StrictModel):
    question: str
    control: Claim
    treatment: str
    variable: str
    channel: str | None
    observations: list[str]
    dependencies: list[str]
    supported_conclusion: str
    unsupported_conclusions: list[str]


class HistoricalAnalog(StrictModel):
    experiment_id: str
    outcome: str
    similarities: list[str]
    differences: list[str]
    implication: str
    source_refs: list[str]


class ExternalFactor(StrictModel):
    factor: Claim
    date: str | None
    affected_metric: str
    implication: str
    verification: str


class Handoff(StrictModel):
    offer_task: str
    experiment_design_questions: list[str]


class Draft(StrictModel):
    candidate_id: str = Field(pattern=r"^hyp_[0-9]{3,}$")
    mode: Literal["grounded", "exploratory"]
    title: str
    statement: str
    segment_refs: list[SegmentRef]
    context: Claim
    problem_jtbd: Claim
    product_value: Claim
    business_link: Claim
    change: Change
    mechanism: Assumption
    expected_effect: ExpectedEffect
    primary_metric: Metric
    guardrail_metrics: list[Metric]
    assumptions: list[Assumption]
    alternative_explanations: list[Assumption]
    falsifying_observation: str
    learning_value: str
    test_candidate: TestCandidate
    historical_analogs: list[HistoricalAnalog]
    external_factors: list[ExternalFactor]
    risks: list[Claim]
    handoff: Handoff
    source_refs: list[str]


class DraftBatch(StrictModel):
    candidates: list[Draft] = Field(min_length=1, max_length=12)
    missing_data: list[str]
    limitations: list[str]


class Factor(StrictModel):
    name: Literal["evidence", "business_value", "feasibility", "novelty", "measurability", "risk"]
    rating: Literal["high", "medium", "low", "unknown"]
    score: float | None
    rationale: str
    source_refs: list[str]


class Priority(StrictModel):
    rank: int = Field(ge=1)
    method: str
    provisional: bool
    total_score: float | None
    factors: list[Factor] = Field(min_length=6, max_length=6)
    rationale: str


class RankingItem(StrictModel):
    candidate_id: str
    priority: Priority


class Ranking(StrictModel):
    ordered: list[RankingItem] = Field(min_length=1, max_length=12)
    method: str
    provisional: bool
    rationale: str
    missing_data: list[str]


class Check(StrictModel):
    name: str
    criterion: str
    status: Literal["pass", "fail", "unknown"]
    detail: str
    source_refs: list[str]


class Validation(StrictModel):
    candidate_id: str
    overall_status: Literal["pass", "fail", "unknown"]
    checks: list[Check] = Field(min_length=1)
    blocking_issues: list[str]
    limitations: list[str]


class InputRef(StrictModel):
    artifact_type: Literal["product_result", "audience_result"]
    artifact_id: str
    version: str | None
    source_refs: list[str]


class ContextSummary(StrictModel):
    product: Claim
    business_goal: Claim
    audience_ids: list[str]
    constraints: list[Claim]


class ExcludedCandidate(StrictModel):
    title: str
    reason: str
    source_refs: list[str]


class Card(Draft):
    hypothesis_id: str | None
    hypothesis_version: str | None
    checks: list[Check]
    priority: Priority
    selection_status: Literal["selected", "backup", "rejected_validation"]
    validation_status: Literal["pass", "fail", "unknown", "not_attempted"]
    validation_issues: list[str]


class PipelineResult(StrictModel):
    schema_version: Literal["0.1"]
    request_id: str
    workspace_id: str | None
    product_id: str
    status: Literal["complete", "partial", "blocked"]
    input_refs: list[InputRef]
    context_summary: ContextSummary
    hypotheses: list[Card]
    excluded_candidates: list[ExcludedCandidate]
    conflicts: list[str]
    missing_data: list[str]
    questions: list[str] = Field(max_length=3)
    limitations: list[str]
    review_status: Literal["pending"]
    human_decision: Literal["pending"]
    validation_attempts: list[Validation]
    selected_candidate_ids: list[str]
