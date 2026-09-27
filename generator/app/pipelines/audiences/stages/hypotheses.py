from __future__ import annotations

import re
from enum import StrEnum
from typing import Any

from app.language_model_client.structured_generation import complete_structured
from app.pipelines.audiences.models import (
    CampaignConfig,
    HypothesisDraft,
    InputBundle,
    InsightBatch,
    Product,
    ResearchRecord,
    SyntheticPersona,
)
from app.pipelines.audiences.stages.errors import CorrectionRequired
from app.pipelines.audiences.stages.execution import ModelCallContext
from app.pipelines.audiences.stages.research import ResearchStage


class HypothesisAngle(StrEnum):
    PAIN_RELIEF = "pain_relief"
    NEED_SOLUTION = "need_solution"
    DECISION_SUPPORT = "decision_support"


HYPOTHESIS_ROLES = {
    HypothesisAngle.PAIN_RELIEF: "Test a message about relieving the selected pain.",
    HypothesisAngle.NEED_SOLUTION: "Test a different message about the selected need.",
    HypothesisAngle.DECISION_SUPPORT: "Test decision support without replacing the pain or need.",
}


class HypothesisStage(ResearchStage):
    def run_hypotheses(self, bundle, nodes, operation, previous_error):
        execution = ModelCallContext()
        try:
            return self._run_hypotheses(execution, bundle, nodes, operation, previous_error)
        except ValueError as error:
            if execution.raw_response is None:
                raise

            raise CorrectionRequired(str(error), execution.raw_response) from error

    def _run_hypotheses(self, execution: ModelCallContext, bundle, nodes, operation, previous_error):
        if operation.stage.value == "hypothesis" and operation.audience_id and operation.angle is not None:
            return self._hypothesis(
                execution, bundle, nodes, self.personas[operation.audience_id], operation.angle, previous_error
            )

        if operation.stage.value == "diversity":
            return {"warnings": self._diversity_warnings(nodes)}

        raise ValueError(f"unsupported hypothesis stage {operation.id}")

    def _hypothesis(
        self,
        execution: ModelCallContext,
        bundle: InputBundle,
        nodes: dict[str, Any],
        persona: SyntheticPersona,
        slot_index: int,
        previous_error: str | None,
    ) -> dict[str, Any]:
        research = self._research(bundle.product, nodes, persona)
        ranking = self._output(nodes, "ranking")
        row = next(item for item in ranking["top_3"] if item["persona_id"] == persona.persona_id)
        previous = [self._output(nodes, f"hypothesis:{persona.persona_id}:{index}") for index in range(slot_index)]
        channels = self._available_channels(bundle.product, persona, bundle.campaign)
        catalog = self._evidence_catalog(persona, research)
        response_schema = HypothesisDraft.model_json_schema()
        properties = response_schema["properties"]
        properties["pain_id"]["enum"] = [
            key for key, value in catalog.items() if value["kind"] == "pains" and value["present"]
        ]
        properties["need_id"]["enum"] = [
            key for key, value in catalog.items() if value["kind"] == "needs" and value["present"]
        ]
        properties["supporting_insight_ids"]["items"]["enum"] = [
            key for key, value in catalog.items() if value["kind"] in {"desires", "objections", "triggers", "use_cases"}
        ]
        properties["selected_fact_ids"]["items"]["enum"] = [fact.fact_id for fact in bundle.product.facts]
        prompt = self._prompt(
            """
Create one testable marketing hypothesis. Select one present pain and one present need from the evidence
catalog, then choose confirmed product facts that may help. Do not replace the pain with a bank rule,
invent audiences, rates, prices, approval, features, or numerical results. In headline and offer_value,
do not use numbers: confirmed conditions are added separately. A channel may be null when none is
available. Keep fields short, write in Russian, and return only JSON.
""",
            role=HYPOTHESIS_ROLES[list(HypothesisAngle)[slot_index]],
            persona=persona,
            insights={key: value for key, value in catalog.items() if value["kind"] in InsightBatch.model_fields},
            product=self._product_for_research(bundle.product),
            experiment=bundle.experiment,
            allowed_channels=channels,
            previous=[{"headline": item["headline"], "change_to_test": item["change_to_test"]} for item in previous],
        )
        draft = complete_structured(
            self.model_client, execution, HypothesisDraft, prompt, previous_error, response_schema
        )
        self._validate_hypothesis(draft, bundle, persona, research, previous)
        return self._materialize_hypothesis(bundle, persona, research, row, draft, slot_index)

    def _available_channels(self, product: Product, persona: SyntheticPersona, campaign: CampaignConfig) -> list[str]:
        channels = []
        for channel in campaign.allowed_channels:
            if channel == "in_app_banner" and not persona.has_cifra_debit_or_salary_card:
                continue

            if channel in {"push", "email", "sms"} and persona.persona_id not in campaign.contactable_persona_ids:
                continue

            channels.append(channel)

        return sorted(set(channels))

    def _validate_hypothesis(
        self,
        draft: HypothesisDraft,
        bundle: InputBundle,
        persona: SyntheticPersona,
        research: ResearchRecord,
        previous: list[dict[str, Any]],
    ) -> None:
        catalog = self._evidence_catalog(persona, research)
        for insight_id, kind in ((draft.pain_id, "pains"), (draft.need_id, "needs")):
            if insight_id not in catalog or catalog[insight_id]["kind"] != kind or not catalog[insight_id]["present"]:
                raise ValueError(f"{kind} must be a present insight for this persona")

        for insight_id in draft.supporting_insight_ids:
            if insight_id not in catalog or catalog[insight_id]["kind"] not in {
                "desires",
                "objections",
                "triggers",
                "use_cases",
            }:
                raise ValueError("supporting insight is invalid")

        fact_ids = {fact.fact_id for fact in bundle.product.facts}
        if not set(draft.selected_fact_ids).issubset(fact_ids) or len(draft.selected_fact_ids) != len(
            set(draft.selected_fact_ids)
        ):
            raise ValueError("hypothesis references unknown or duplicate facts")

        if draft.channel is not None and draft.channel not in self._available_channels(
            bundle.product, persona, bundle.campaign
        ):
            raise ValueError("hypothesis uses an unavailable channel")

        free_text = " ".join(
            (draft.headline, draft.offer_value, draft.change_to_test, draft.mechanism, draft.why_for_persona)
        )
        if re.search(r"\d", draft.headline + draft.offer_value):
            raise ValueError("headline and offer_value cannot contain numbers")

        for claim in bundle.product.forbidden_claims + ["гарантированно", "бесплатные деньги"]:
            if self._norm(claim) in self._norm(free_text):
                raise ValueError(f"hypothesis contains forbidden claim {claim}")

        if bundle.product.price.status == "unknown" and re.search(
            r"бесплатн|без комисси|без переплат", self._norm(draft.headline + " " + draft.offer_value)
        ):
            raise ValueError("unknown price cannot be presented as free")

        for item in previous:
            if (
                self._norm(item["headline"]) == self._norm(draft.headline)
                or self._word_similarity(item["change_to_test"], draft.change_to_test) >= 0.85
            ):
                raise ValueError("hypothesis duplicates a prior experiment")

    def _materialize_hypothesis(
        self,
        bundle: InputBundle,
        persona: SyntheticPersona,
        research: ResearchRecord,
        row: dict[str, Any],
        draft: HypothesisDraft,
        slot_index: int,
    ) -> dict[str, Any]:
        catalog = self._evidence_catalog(persona, research)
        facts = {fact.fact_id: fact.text for fact in bundle.product.facts}
        result = draft.model_dump(mode="json")
        result.update(
            {
                "hypothesis_id": f"{persona.persona_id}_hyp_{slot_index + 1:02d}",
                "persona_id": persona.persona_id,
                "persona_label": persona.label,
                "persona_rank": row["selection_order"],
                "persona_score": row["final_score"],
                "input_signature": research.input_signature,
                "research_signature": self._fingerprint(research),
                "angle": list(HypothesisAngle)[slot_index].value,
                "insight_type": "pain_and_need",
                "insight_basis": [
                    {"insight_id": insight_id, **catalog[insight_id]} for insight_id in (draft.pain_id, draft.need_id)
                ],
                "supporting_insights": [
                    {"insight_id": insight_id, **catalog[insight_id]} for insight_id in draft.supporting_insight_ids
                ],
                "proof_points": [{"fact_id": fact_id, "text": facts[fact_id]} for fact_id in draft.selected_fact_ids],
                "primary_metric": bundle.experiment.primary_metric,
                "metric_definition": {
                    "numerator": bundle.experiment.metric_numerator,
                    "denominator": bundle.experiment.metric_denominator,
                },
                "hypothesis_statement": (
                    f"If {draft.change_to_test}, then the {bundle.experiment.primary_metric} should "
                    f"{'increase' if bundle.experiment.metric_direction == 'increase' else 'decrease'} "
                    f"because {draft.mechanism}"
                ),
                "experiment_design": (
                    "A/B test in the selected segment; randomize by customer and confirm product "
                    "availability and communication consent before launch."
                ),
                "success_criterion": bundle.experiment.success_criterion
                or "Not set: establish baseline, MDE, and sample size before launch.",
                "status": "synthetic_draft_requires_human_review",
                "review_checklist": [
                    "Fact addresses the selected pain and need",
                    "Free text contains no new promise",
                    "Tariffs and legal conditions are current",
                ],
                "offer": {
                    "headline": draft.headline,
                    "value_proposition": draft.offer_value,
                    "source_facts": [
                        {"fact_id": fact_id, "text": facts[fact_id]} for fact_id in draft.selected_fact_ids
                    ],
                    "conditions": bundle.product.known_constraints,
                    "cta": "Learn the conditions",
                    "not_ready_for_publication": True,
                },
            }
        )
        return result

    def _diversity_warnings(self, nodes: dict[str, Any]) -> list[str]:
        top = self._output(nodes, "ranking")["top_3"]
        hypotheses = [
            self._output(nodes, f"hypothesis:{row['persona_id']}:{index}")
            for row in top
            for index in range(len(HypothesisAngle))
        ]
        warnings = []
        for index, left in enumerate(hypotheses):
            for right in hypotheses[index + 1 :]:
                if (
                    left["persona_id"] != right["persona_id"]
                    and self._word_similarity(left["change_to_test"], right["change_to_test"]) >= 0.85
                ):
                    warnings.append(f"{left['hypothesis_id']} and {right['hypothesis_id']} may duplicate meaning")

        return warnings
