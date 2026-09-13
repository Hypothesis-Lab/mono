from __future__ import annotations

from typing import Literal, Self

from pydantic import BaseModel, Field, model_validator


class ProductFact(BaseModel):
    fact_id: str = Field(min_length=1)
    text: str = Field(min_length=1)


class EligibilityRule(BaseModel):
    rule_id: str = Field(min_length=1)
    field: str = Field(min_length=1)
    operator: Literal["eq", "in", "gte", "lte", "between", "truthy"]
    value: str | int | bool | list[str] | list[int]
    description: str = Field(min_length=1)


class Product(BaseModel):
    brand: str = Field(min_length=1)
    name: str = Field(min_length=1)
    description: str = Field(min_length=1)
    category: str = Field(min_length=1)
    custom_category: str | None = Field(default=None, min_length=1, max_length=64)
    country: str = Field(min_length=1)
    town: str | None = None
    experiment_goal: str = Field(min_length=1)
    target_action: str = Field(min_length=1)
    source_url: str | None = None
    facts: list[ProductFact] = Field(min_length=3)
    eligibility_rules: list[EligibilityRule] = Field(default_factory=list)
    allowed_channels: list[str] = Field(min_length=1)
    forbidden_claims: list[str] = Field(default_factory=list)
    known_constraints: list[str] = Field(default_factory=list)

    @model_validator(mode="after")
    def validate_custom_category(self) -> Self:
        if self.category == "other" and self.custom_category is None:
            raise ValueError("custom_category is required when category is other")
        if self.category != "other" and self.custom_category is not None:
            raise ValueError("custom_category is only allowed when category is other")

        return self


class CreateJobRequest(BaseModel):
    product_id: str = Field(min_length=1)
    input: Product


class CreateJobResponse(BaseModel):
    id: str
    finished: bool


class JobResponse(BaseModel):
    id: str
    product_id: str
    input: Product
    progress: dict[str, object]
    result: dict[str, object] | None
    finished: bool
    created_at: str
    finished_at: str | None


class CompletedJobResponse(BaseModel):
    id: str
    product_id: str
    result: dict[str, object]


class WorkerStatusResponse(BaseModel):
    current_job_id: str | None
    queued_jobs: int
    blocked_jobs: int


class ProductGenerationStatusResponse(BaseModel):
    status: Literal["created", "working", "failed"]


class SyntheticPersona(BaseModel):
    persona_id: str
    label: str
    gender: Literal["женщина", "мужчина"]
    age: int = Field(ge=18, le=80)
    monthly_income_rub: int = Field(gt=0)
    occupation: str
    employment_type: str
    citizenship: str
    permanent_registration_country: str
    device: str
    operating_system: str
    digital_behavior: str
    interests: list[str] = Field(min_length=2)
    country: str = Field(min_length=1)
    town: str | None = None
    cifra_relationship: str
    has_cifra_debit_or_salary_card: bool
    financial_behavior: str
    credit_attitude: str


class InterviewAnswer(BaseModel):
    question_id: str
    answer: str = Field(min_length=1)


class PersonaResearch(BaseModel):
    persona_id: str
    interview_answers: list[InterviewAnswer] = Field(min_length=10, max_length=10)
    problems: list[str] = Field(min_length=2, max_length=5)
    pains: list[str] = Field(min_length=2, max_length=5)
    needs: list[str] = Field(min_length=2, max_length=5)
    desires: list[str] = Field(min_length=2, max_length=5)
    objections: list[str] = Field(min_length=1, max_length=4)
    decision_triggers: list[str] = Field(min_length=1, max_length=4)
    decision_factors: list[str] = Field(min_length=2, max_length=5)
    relevant_use_cases: list[str] = Field(min_length=1, max_length=4)
    best_message_angle: str = Field(min_length=1)
    fit_summary: str = Field(min_length=1)


class PersonaMetricScores(BaseModel):
    persona_id: str
    need_strength: int = Field(ge=0, le=100)
    product_value_fit: int = Field(ge=0, le=100)
    adoption_readiness: int = Field(ge=0, le=100)
    digital_access_fit: int = Field(ge=0, le=100)
    hypothesis_testability: int = Field(ge=0, le=100)
    strongest_signal: str = Field(min_length=1)
    main_barrier: str = Field(min_length=1)
    evidence: list[str] = Field(min_length=2, max_length=5)
    explanation: str = Field(min_length=1)


class HypothesisDraft(BaseModel):
    hypothesis_id: str
    persona_id: str
    insight_type: Literal["pain", "need", "desire", "objection", "trigger", "use_case"]
    insight_basis: list[str] = Field(min_length=1, max_length=3)
    value_angle: Literal[
        "safety_net",
        "control",
        "one_card",
        "flexibility",
        "convenience",
        "clarity",
        "planned_expense",
    ]
    hypothesis_statement: str = Field(min_length=1)
    headline: str = Field(min_length=1)
    message_frame: str = Field(min_length=1)
    cta: str = Field(min_length=1)
    channel: str = Field(min_length=1)
    selected_fact_ids: list[str] = Field(min_length=1, max_length=3)
    experiment_design: str = Field(min_length=1)
    primary_metric: str = Field(min_length=1)
    why_for_persona: str = Field(min_length=1)
    risks: list[str] = Field(min_length=1, max_length=3)


class HypothesisBatch(BaseModel):
    hypotheses: list[HypothesisDraft] = Field(min_length=3, max_length=3)
