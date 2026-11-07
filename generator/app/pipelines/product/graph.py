from __future__ import annotations

import json
import operator
from pathlib import Path
from typing import Annotated, Any, Literal, TypedDict

from langgraph.graph import END, START, StateGraph
from langgraph.types import Send

from app.language_model_client.structured_generation import complete_with_repair
from app.pipelines.audiences.stages import AudienceStages
from app.pipelines.audiences.stages.execution import ModelCallContext
from app.pipelines.product.models import Passport, ProductInput, Source


class ProductState(TypedDict):
    input: dict[str, Any]
    source: dict[str, Any]
    drafts: Annotated[list[dict[str, Any]], operator.add]
    passport: dict[str, Any]
    errors: list[str]
    result: dict[str, Any]


def build_graph(stages: AudienceStages, *, checkpointer: Any = None, observer: Any = None):
    builder = StateGraph(ProductState)
    extraction_prompt = Path(__file__).with_name("prompt.md").read_text(encoding="utf-8")

    def send_sources(state: ProductState) -> list[Send]:
        product = ProductInput.model_validate(state["input"])
        return [Send("extract", {"input": state["input"], "source": source.model_dump()}) for source in product.sources]

    def extract(state: ProductState) -> dict[str, Any]:
        product = ProductInput.model_validate(state["input"])
        source = Source.model_validate(state["source"])
        prompt = extraction_prompt
        for key, value in {
            "TARGET_PRODUCT": product.target_product,
            "AS_OF_DATE": product.as_of_date or "null",
            "SOURCE": source.source,
            "CONTENT": source.content,
        }.items():
            prompt = prompt.replace("{{" + key + "}}", value)

        passport = complete_with_repair(stages.model_client, ModelCallContext(), Passport, prompt, None)
        return {"drafts": [{"source": source.source, "result": passport.model_dump(mode="json")}]}

    def consolidate(state: ProductState) -> dict[str, Any]:
        product = ProductInput.model_validate(state["input"])
        drafts = sorted(state["drafts"], key=lambda item: item["source"])
        prompt = (
            "Объедини черновики паспорта продукта. Не добавляй фактов, сохрани существенные условия и все "
            "доказательства. Объединяй только дубли при одинаковых условиях; противоречия сохраняй в conflicts. "
            "Перенумеруй записи последовательно. Если часть источников не обработана, статус partial; если "
            "пригодного содержимого нет, blocked. Верни JSON по схеме.\n"
            f"Целевой продукт: {product.target_product}\n"
            f"Дата актуальности: {product.as_of_date or 'null'}\n"
            f"Черновики: {json.dumps(drafts, ensure_ascii=False)}"
        )
        passport = complete_with_repair(stages.model_client, ModelCallContext(), Passport, prompt, None)
        return {"passport": passport.model_dump(mode="json")}

    def validate(state: ProductState) -> dict[str, Any]:
        passport = Passport.model_validate(state["passport"])
        errors = []
        for index, record in enumerate(passport.records, 1):
            if record.id != f"r{index}":
                errors.append(f"Record {index} must have id r{index}.")

            if not record.statement:
                errors.append(f"Missing statement on {record.id}.")

            if any(not item.source or not item.quote for item in record.evidence):
                errors.append(f"Evidence requires source and quote on {record.id}.")

        record_ids = {record.id for record in passport.records}
        for conflict in passport.conflicts:
            missing = set(conflict.record_ids) - record_ids
            if missing:
                errors.append(f"Conflict references unknown records: {', '.join(sorted(missing))}.")

        return {"errors": errors}

    def route_validation(state: ProductState) -> Literal["repair", "finalize"]:
        return "repair" if state["errors"] else "finalize"

    def repair(state: ProductState) -> dict[str, Any]:
        prompt = (
            "Исправь только структурные ошибки паспорта продукта. Не добавляй фактов и не удаляй доказательства.\n"
            f"Паспорт: {json.dumps(state['passport'], ensure_ascii=False)}\n"
            f"Ошибки: {json.dumps(state['errors'], ensure_ascii=False)}"
        )
        passport = complete_with_repair(stages.model_client, ModelCallContext(), Passport, prompt, None)
        return {"passport": passport.model_dump(mode="json")}

    def finalize(state: ProductState) -> dict[str, Any]:
        errors = validate(state)["errors"]
        if errors:
            raise ValueError(f"product passport validation failed: {'; '.join(errors)}")

        product = ProductInput.model_validate(state["input"])
        passport = Passport.model_validate(state["passport"])
        if product.unavailable_sources:
            passport = passport.model_copy(
                update={
                    "extraction_status": "blocked" if passport.extraction_status == "blocked" else "partial",
                    "warnings": [
                        *passport.warnings,
                        "Текст недоступен для источников: " + ", ".join(product.unavailable_sources),
                    ],
                }
            )

        return {"result": {"passport": passport.model_dump(mode="json"), "validation": {"valid": True, "errors": []}}}

    builder.add_node("extract", extract)
    builder.add_node("consolidate", consolidate)
    builder.add_node("validate", validate)
    builder.add_node("repair", repair)
    builder.add_node("finalize", finalize)
    builder.add_conditional_edges(START, send_sources, ["extract"])
    builder.add_edge("extract", "consolidate")
    builder.add_edge("consolidate", "validate")
    builder.add_conditional_edges("validate", route_validation, ["repair", "finalize"])
    builder.add_edge("repair", "finalize")
    builder.add_edge("finalize", END)
    return builder.compile(checkpointer=checkpointer)
