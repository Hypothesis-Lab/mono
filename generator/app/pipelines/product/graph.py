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
from app.pipelines.product.models import Evaluation, Passport, ProductInput, Source


class ProductState(TypedDict):
    input: dict[str, Any]
    source: dict[str, Any]
    drafts: Annotated[list[dict[str, Any]], operator.add]
    passport: dict[str, Any]
    errors: list[str]
    result: dict[str, Any]
    evaluation: dict[str, Any]


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
        prompt = """Ты — этап консолидации паспорта продукта после независимой экстракции по источникам.

Используй правила и формат результата из основного промпта ниже. Объедини только поддержанные записи из черновиков.
Не добавляй утверждения, которых нет в черновиках. Сохрани все доказательства и существенные условия.

Правила консолидации:
1. Объединяй только дубли об одном объекте и при одинаковых условиях, добавляя все evidence.
2. Если утверждения несовместимы после проверки продукта, тарифа, периода, аудитории и условий, сохрани обе записи
   и добавь conflict.
3. Перенумеруй итоговые records последовательно от r1 и отсортируй их по порядку категорий из схемы.
4. Если хотя бы один источник не обработан, поставь extraction_status partial. Если пригодного содержимого нет,
   поставь blocked.
5. Не превращай gaps, warnings или неизвестные значения в факты.
6. Верни только валидный JSON итогового паспорта. Не используй Markdown fences.

Целевой продукт: {target_product}
Дата актуальности: {as_of_date}

Черновики экстракции по источникам:
{drafts}

Итоговая структура содержит product_name, extraction_status, records, conflicts, gaps и warnings.
Допустимые category: identity, purpose, jtbd, audience, features, benefits, pricing, terms, limitations, economics,
funnel,
offers_channels, communication_rules.
Допустимые statement_type: description, claim, reported_result, plan, target, rule.
Допустимые evidence_status: supported, conflicted.
""".format(
            target_product=product.target_product,
            as_of_date=product.as_of_date or "null",
            drafts=json.dumps(drafts, ensure_ascii=False, indent=2),
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
        prompt = """Исправь JSON паспорта продукта по результатам механической проверки.

Не добавляй новых фактов. Используй только исходный JSON и уже имеющиеся evidence. Сохрани смысл, условия,
конфликты и неизвестность. Верни только валидный JSON по схеме ниже, без Markdown fences. Исправь только структурные
ошибки, перечисленные в validation.errors.

Исходный JSON:
{passport}

Ошибки проверки:
{errors}

Верни объект с полями product_name, extraction_status, records, conflicts, gaps и warnings. Для каждой записи сохрани
id, category, subject, statement, statement_type, conditions, time_scope, evidence_status и evidence. Не добавляй
факты и не удаляй evidence.
""".format(
            passport=json.dumps(state["passport"], ensure_ascii=False, indent=2),
            errors=json.dumps(state["errors"], ensure_ascii=False),
        )
        passport = complete_with_repair(stages.model_client, ModelCallContext(), Passport, prompt, None)
        return {"passport": passport.model_dump(mode="json")}

    def finalize(state: ProductState) -> dict[str, Any]:
        errors = validate(state)["errors"]
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

        return {
            "result": {
                "passport": passport.model_dump(mode="json"),
                "validation": {"valid": not errors, "errors": errors},
            }
        }

    def evaluate(state: ProductState) -> dict[str, Any]:
        passport = Passport.model_validate(state["result"]["passport"])
        materials = [
            Source.model_validate(source).model_dump(mode="json")
            for source in ProductInput.model_validate(state["input"]).sources
        ]
        evaluation_prompt = Path(__file__).with_name("evaluation_prompt.md").read_text(encoding="utf-8")
        prompt = evaluation_prompt
        for key, value in {
            "PASSPORT_JSON": json.dumps(passport.model_dump(mode="json"), ensure_ascii=False, indent=2),
            "VALIDATION_JSON": json.dumps(state["result"]["validation"], ensure_ascii=False),
            "SOURCE_MATERIALS_JSON": json.dumps(materials, ensure_ascii=False, indent=2),
        }.items():
            prompt = prompt.replace("{{" + key + "}}", value)

        evaluation = complete_with_repair(stages.model_client, ModelCallContext(), Evaluation, prompt, None)
        return {"result": {**state["result"], "evaluation": evaluation.model_dump(mode="json")}}

    builder.add_node("extract", extract)
    builder.add_node("consolidate", consolidate)
    builder.add_node("validate", validate)
    builder.add_node("repair", repair)
    builder.add_node("finalize", finalize)
    builder.add_node("evaluate", evaluate)
    builder.add_conditional_edges(START, send_sources, ["extract"])
    builder.add_edge("extract", "consolidate")
    builder.add_edge("consolidate", "validate")
    builder.add_conditional_edges("validate", route_validation, ["repair", "finalize"])
    builder.add_edge("repair", "finalize")
    builder.add_edge("finalize", "evaluate")
    builder.add_edge("evaluate", END)
    return builder.compile(checkpointer=checkpointer)
