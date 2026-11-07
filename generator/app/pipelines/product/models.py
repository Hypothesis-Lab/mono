from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, ConfigDict, Field


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid")


class Source(StrictModel):
    source: str = Field(min_length=1)
    content: str


class ProductInput(StrictModel):
    target_product: str = Field(min_length=1)
    as_of_date: str | None
    sources: list[Source] = Field(min_length=1)
    unavailable_sources: list[str] = Field(default_factory=list)


class Evidence(StrictModel):
    source: str
    locator: str | None
    quote: str


class Record(StrictModel):
    id: str
    category: Literal[
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
    subject: str
    statement: str
    statement_type: Literal["description", "claim", "reported_result", "plan", "target", "rule"]
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
    extraction_status: Literal["complete", "partial", "blocked"]
    records: list[Record]
    conflicts: list[Conflict]
    gaps: list[Gap]
    warnings: list[str]
