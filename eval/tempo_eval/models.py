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


class Source(StrictModel):
    source: str
    content: str


class Draft(StrictModel):
    source: str
    result: Passport


class Validation(StrictModel):
    valid: bool
    errors: list[str]
