from __future__ import annotations

from typing import Annotated, Literal

from pydantic import BaseModel, ConfigDict, Field, model_validator

Text = Annotated[str, Field(min_length=1, max_length=1600)]


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)


class ProductFact(StrictModel):
    fact_id: Text
    text: Text


class ProductPrice(StrictModel):
    status: Literal["unknown", "known"] = "unknown"
    description: Text = "Стоимость не указана; не считать продукт бесплатным."
    source_fact_ids: list[str] = Field(default_factory=list)

    @model_validator(mode="after")
    def known_price_has_sources(self) -> ProductPrice:
        if self.status == "known" and not self.source_fact_ids:
            raise ValueError("known price requires source_fact_ids")
        return self


class EligibilityRule(StrictModel):
    rule_id: str
    field: str
    operator: Literal["eq", "in", "gte", "lte", "between", "truthy"]
    value: str | int | bool | list[str] | list[int]
    description: str


class Product(StrictModel):
    brand: Text
    name: Text
    description: Text
    category: Text
    custom_category: str | None = None
    country: Text
    town: str | None = None
    source_url: str | None = None
    facts: list[ProductFact] = Field(min_length=3)
    key_fact_ids: list[str] = Field(min_length=1)
    price: ProductPrice = Field(default_factory=ProductPrice)
    keywords: list[str] = Field(default_factory=list)
    facts_checked_at: str | None = None
    eligibility_rules: list[EligibilityRule] = Field(default_factory=list)
    forbidden_claims: list[str] = Field(default_factory=list)
    known_constraints: list[str] = Field(default_factory=list)
    unknowns: list[str] = Field(default_factory=list)

    @model_validator(mode="after")
    def references_existing_facts(self) -> Product:
        fact_ids = [fact.fact_id for fact in self.facts]
        if len(fact_ids) != len(set(fact_ids)):
            raise ValueError("product contains duplicate fact_id")
        for fact_id in self.key_fact_ids + self.price.source_fact_ids:
            if fact_id not in fact_ids:
                raise ValueError(f"unknown fact_id {fact_id}")
        if self.category == "other" and self.custom_category is None:
            raise ValueError("custom_category is required when category is other")
        if self.category != "other" and self.custom_category is not None:
            raise ValueError("custom_category is only allowed when category is other")
        return self


class ExperimentConfig(StrictModel):
    business_goal: Text
    goal_origin: Literal["demo_assumption", "user_provided"] = "user_provided"
    target_action: Text
    primary_metric: Text
    metric_direction: Literal["increase", "decrease"] = "increase"
    metric_numerator: Text
    metric_denominator: Text
    success_criterion: str | None = None


class CampaignConfig(StrictModel):
    allowed_channels: list[str] = Field(default_factory=list)
    contactable_persona_ids: list[str] = Field(default_factory=list)
    note: str = "Channels are not used for research or ranking."


class InputBundle(StrictModel):
    product: Product
    experiment: ExperimentConfig
    campaign: CampaignConfig = Field(default_factory=CampaignConfig)


class CreateJobRequest(StrictModel):
    product_id: str = Field(min_length=1)
    input: InputBundle


class CreateJobResponse(StrictModel):
    id: str
    finished: bool


class JobResponse(StrictModel):
    id: str
    product_id: str
    input: InputBundle
    progress: dict[str, object]
    result: dict[str, object] | None
    finished: bool
    created_at: str
    finished_at: str | None


class CompletedJobResponse(StrictModel):
    id: str
    product_id: str
    result: dict[str, object]


class WorkerStatusResponse(StrictModel):
    current_job_id: str | None
    queued_jobs: int
    blocked_jobs: int


class ProductGenerationStatusResponse(StrictModel):
    status: Literal["created", "working", "failed"]


class SyntheticPersona(StrictModel):
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
    country: str
    town: str | None = None
    cifra_relationship: str
    has_cifra_debit_or_salary_card: bool
    financial_behavior: str
    credit_attitude: str


class InterviewQuestion(StrictModel):
    question_id: str
    dimension: str
    text: str


class InterviewAnswer(StrictModel):
    question_id: str
    answer: Text


class AnswerBatch(StrictModel):
    answers: list[InterviewAnswer] = Field(min_length=5, max_length=5)


class EvidenceQuote(StrictModel):
    question_id: str
    quote: Text


class InsightDraft(StrictModel):
    text: Text
    present: bool
    evidence: list[EvidenceQuote] = Field(min_length=1, max_length=2)


class InsightBatch(StrictModel):
    pains: list[InsightDraft] = Field(min_length=1, max_length=3)
    needs: list[InsightDraft] = Field(min_length=1, max_length=3)
    desires: list[InsightDraft] = Field(default_factory=list, max_length=3)
    objections: list[InsightDraft] = Field(default_factory=list, max_length=3)
    triggers: list[InsightDraft] = Field(default_factory=list, max_length=3)
    use_cases: list[InsightDraft] = Field(default_factory=list, max_length=3)


class ResearchRecord(StrictModel):
    persona_id: str
    input_signature: str
    answers: list[InterviewAnswer] = Field(min_length=10, max_length=10)
    insights: InsightBatch
    synthetic: Literal[True] = True


class MetricVote(StrictModel):
    value: int = Field(ge=0, le=4, strict=True)
    reason: Text
    evidence_ids: list[str] = Field(min_length=1, max_length=5)
    fact_ids: list[str] = Field(default_factory=list, max_length=3)


class ScoreDraft(StrictModel):
    need_strength: MetricVote
    product_value_fit: MetricVote
    adoption_readiness: MetricVote
    digital_access_fit: MetricVote
    hypothesis_testability: MetricVote
    main_barrier: Text
    unknowns: list[str] = Field(default_factory=list, max_length=5)


class ScoreRecord(StrictModel):
    persona_id: str
    input_signature: str
    research_signature: str
    scores: ScoreDraft


class HypothesisDraft(StrictModel):
    pain_id: str
    need_id: str
    supporting_insight_ids: list[str] = Field(default_factory=list, max_length=2)
    selected_fact_ids: list[str] = Field(min_length=1, max_length=3)
    headline: Text
    offer_value: Text
    change_to_test: Text
    mechanism: Text
    why_for_persona: Text
    channel: str | None
    risks: list[str] = Field(min_length=1, max_length=3)
