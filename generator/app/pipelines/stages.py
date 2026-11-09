from __future__ import annotations

from app.language_model_client.protocols import LanguageModelClient


class PipelineStages:
    def __init__(self, model_client: LanguageModelClient) -> None:
        self.model_client = model_client
