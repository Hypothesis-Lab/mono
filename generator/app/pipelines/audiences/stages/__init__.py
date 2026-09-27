"""Audience pipeline stage implementations."""

from app.pipelines.audiences.stages.result import ResultStage


class AudienceStages(ResultStage):
    """Shared model and stage implementations used by graph nodes."""


__all__ = ["AudienceStages"]
