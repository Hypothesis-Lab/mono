from __future__ import annotations

import re

from app.pipelines.audiences.models import InputBundle


def validate_input(bundle: InputBundle) -> None:
    questions, _ = readiness(bundle)
    if questions:
        raise ValueError("input is not ready: " + "; ".join(questions))


def readiness(bundle: InputBundle) -> tuple[list[str], list[str]]:
    product, experiment = bundle.product, bundle.experiment
    questions = []
    if re.search(r"выбрать.*(?:три|3).*персон|сформировать.*гипотез", experiment.business_goal.lower()):
        questions.append("replace the technical generation task with a business goal")

    warnings = list(product.unknowns)
    if product.price.status == "unknown":
        warnings.append("price is unknown: do not claim that the product is free or has no fee")

    if not bundle.campaign.allowed_channels:
        warnings.append("no campaign channels are configured; hypotheses will use channel=null")

    if not experiment.success_criterion:
        warnings.append("success criterion is not set; define it after baseline and power analysis")

    return questions, warnings
