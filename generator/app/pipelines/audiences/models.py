from __future__ import annotations

from typing import Annotated, Literal

from pydantic import BaseModel, ConfigDict, Field

Text = Annotated[str, Field(min_length=1, max_length=1600)]


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)


class ProductPrice(StrictModel):
    status: Literal["unknown", "known"] = "unknown"
    description: Text = "Цена не указана."


class Product(StrictModel):
    brand: Text
    name: Text
    description: Text
    category: Text
    custom_category: str | None = None
    country: Text
    town: str | None = None
    source_url: str | None = None
    price: ProductPrice = Field(default_factory=ProductPrice)
    keywords: list[str] = Field(default_factory=list)
    forbidden_claims: list[str] = Field(default_factory=list)
    known_constraints: list[str] = Field(default_factory=list)
    unknowns: list[str] = Field(default_factory=list)


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


class InputBundle(StrictModel):
    product: Product
    experiment: ExperimentConfig
    campaign: CampaignConfig = Field(default_factory=CampaignConfig)


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
    headline: Text
    offer_value: Text
    change_to_test: Text
    mechanism: Text
    why_for_persona: Text
    channel: str | None
    risks: list[str] = Field(min_length=1, max_length=3)
