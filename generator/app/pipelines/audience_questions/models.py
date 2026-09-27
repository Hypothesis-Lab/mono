from __future__ import annotations

from pydantic import Field

from app.pipelines.audiences.models import (
    InterviewQuestion,
    StrictModel,
    Text,
)


class AudienceAnswer(StrictModel):
    audience_id: str
    audience_label: Text
    question_id: str
    answer: Text


class QuestionsResult(StrictModel):
    questions: list[InterviewQuestion] = Field(min_length=5, max_length=5)
    answers: list[AudienceAnswer] = Field(min_length=10, max_length=10)
