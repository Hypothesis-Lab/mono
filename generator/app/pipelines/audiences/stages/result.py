from __future__ import annotations

from datetime import UTC, datetime
from typing import Any

from app.pipelines.audiences.catalog import (
    INTERVIEW_QUESTIONS,
    METRIC_WEIGHTS,
    MIN_FIT_LEVEL,
    MIN_NEED_LEVEL,
    MIN_TOTAL_SCORE,
    PERSONAS,
)
from app.pipelines.audiences.models import HypothesisDraft, InputBundle
from app.pipelines.audiences.stages.hypotheses import HypothesisAngle, HypothesisStage
from app.pipelines.audiences.stages.input import readiness
from app.pipelines.audiences.stages.support import PIPELINE_VERSION


class ResultStage(HypothesisStage):
    def run_result(self, bundle, nodes):
        result = self._result(bundle, {"nodes": nodes})
        self._verify_result(result, bundle, {"nodes": nodes})
        return result

    def _result(self, bundle: InputBundle, progress: dict[str, Any]) -> dict[str, Any]:
        nodes = self._nodes(progress)
        ranking = self._output(nodes, "ranking")
        research = [self._research(bundle.product, nodes, persona) for persona in PERSONAS]
        scores = [self._output(nodes, f"score:{persona.persona_id}") for persona in PERSONAS]
        return {
            "run_id": datetime.now(UTC).strftime("run_%Y%m%d_%H%M%S_utc"),
            "pipeline_version": PIPELINE_VERSION,
            "input_signature": self._fingerprint(bundle),
            "input": bundle.model_dump(mode="json"),
            "personas": [persona.model_dump(mode="json") for persona in PERSONAS],
            "interview_questions": [question.model_dump(mode="json") for question in INTERVIEW_QUESTIONS],
            "persona_research": [item.model_dump(mode="json") for item in research],
            "persona_scores": scores,
            "eligibility": {
                persona.persona_id: self._output(nodes, f"eligibility:{persona.persona_id}") for persona in PERSONAS
            },
            "ranking": ranking["ranking"],
            "top_3": ranking["top_3"],
            "selection_policy": {
                "weights": METRIC_WEIGHTS,
                "min_score": MIN_TOTAL_SCORE,
                "min_need": MIN_NEED_LEVEL,
                "min_fit": MIN_FIT_LEVEL,
                "tie_break": "score -> fit -> need -> readiness -> persona_id",
                "calibration": "expert_MVP_not_empirical",
            },
            "hypotheses_by_persona": {
                row["persona_id"]: [
                    self._output(nodes, f"hypothesis:{row['persona_id']}:{index}")
                    for index in range(len(HypothesisAngle))
                ]
                for row in ranking["top_3"]
            },
            "warnings": readiness(bundle)[1] + ranking["warnings"] + self._output(nodes, "diversity")["warnings"],
            "synthetic_data_warning": (
                "Answers, insights, and scores are model simulation, not real research. "
                "Scores are not conversion or approval probabilities."
            ),
            "generation_policy": {
                "provider": "codex_proxy",
                "model": self.model_client.model,
                "reasoning_effort": self.model_client.reasoning_effort,
                "reproducibility": (
                    "Completed node outputs are persisted; model outputs are otherwise not deterministic."
                ),
            },
        }

    def _verify_result(self, result: dict[str, Any], bundle: InputBundle, progress: dict[str, Any]) -> None:
        if result["pipeline_version"] != PIPELINE_VERSION or result["input_signature"] != self._fingerprint(bundle):
            raise ValueError("result does not match current input")

        nodes = self._nodes(progress)
        expected = self._ranking(bundle.product, nodes)
        if result["ranking"] != expected["ranking"] or result["top_3"] != expected["top_3"]:
            raise ValueError("ranking changed before persistence")

        for row in result["top_3"]:
            persona = self.personas[row["persona_id"]]
            research = self._research(bundle.product, nodes, persona)
            previous = []
            hypotheses = result["hypotheses_by_persona"].get(persona.persona_id, [])
            if len(hypotheses) != len(HypothesisAngle):
                raise ValueError("selected persona is missing hypotheses")

            for index, hypothesis in enumerate(hypotheses):
                draft = HypothesisDraft.model_validate({key: hypothesis[key] for key in HypothesisDraft.model_fields})
                self._validate_hypothesis(draft, bundle, persona, research, previous)
                if hypothesis != self._materialize_hypothesis(bundle, persona, research, row, draft, index):
                    raise ValueError("hypothesis was modified after validation")

                previous.append(hypothesis)
