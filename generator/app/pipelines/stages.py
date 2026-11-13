from __future__ import annotations

from app.language_model_client.protocols import LanguageModelClient
from app.research import ResearchRetriever


class PipelineStages:
    def __init__(self, model_client: LanguageModelClient, research_retriever: ResearchRetriever | None = None) -> None:
        self.model_client = model_client
        self.research_retriever = research_retriever or ResearchRetriever()
