from __future__ import annotations

from dataclasses import asdict, dataclass


@dataclass(frozen=True)
class AudiencePipelineConfig:
    """Persistent policy for audience quality and activation checks."""

    minimum_segment_size: int = 100
    max_query_iterations: int = 10
    max_review_iterations: int = 10
    allowed_channels: tuple[str, ...] = ("ad_platform",)
    product_manager_confirmation: str = "external_after_pipeline"

    def __post_init__(self) -> None:
        if self.minimum_segment_size < 0:
            raise ValueError("minimum_segment_size must be non-negative")
        if self.max_query_iterations < 1:
            raise ValueError("max_query_iterations must be at least 1")
        if self.max_review_iterations < 1:
            raise ValueError("max_review_iterations must be at least 1")
        if not self.allowed_channels:
            raise ValueError("at least one activation channel must be configured")

    def as_dict(self) -> dict[str, object]:
        return asdict(self)


DEFAULT_AUDIENCE_CONFIG = AudiencePipelineConfig()
