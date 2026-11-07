from __future__ import annotations

from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid")


class AudienceSelectionInput(StrictModel):
    product_passport: dict[str, Any]
    segment_set_id: str = Field(min_length=1)
    segments: list[dict[str, Any]] = Field(min_length=1)
    limitations: list[str]


class Selection(StrictModel):
    segment_id: str | None
    rationale: str
    supporting_signals: list[str]
    unknowns: list[str]


class SelectionReview(StrictModel):
    status: Literal["pass", "fail", "unknown"]
    rationale: str
    limitations: list[str]
