from __future__ import annotations

import json
import operator
from typing import Annotated, Any, Literal, TypedDict

from langgraph.graph import END, START, StateGraph
from langgraph.types import RetryPolicy, Send

from .client import ModelClient
from .models import Draft, Evaluation, Passport, Source, Validation
from .prompts import load_prompt, render


class WorkflowState(TypedDict, total=False):
    pipeline: str
    model: str
    target_product: str
    as_of_date: str | None
    sources: list[Source]
    drafts: Annotated[list[Draft], operator.add]
    passport: Passport
    validation: Validation
    evaluation: Evaluation
    repair_attempted: bool


class Workflow:
    def __init__(self, client: ModelClient, *, step_retries: int = 3) -> None:
        self.client = client
        self.step_retries = step_retries

    def compile(self, checkpointer: Any):
        retry_policy = RetryPolicy(max_attempts=1)
        builder = StateGraph(WorkflowState)
        builder.add_node("extract_source", self.extract_source, retry_policy=retry_policy)
        builder.add_node("consolidate", self.consolidate, retry_policy=retry_policy)
        builder.add_node("validate", self.validate)
        builder.add_node("repair", self.repair, retry_policy=retry_policy)
        builder.add_node("finalize", self.finalize)
        builder.add_node("evaluate", self.evaluate, retry_policy=retry_policy)
        builder.add_conditional_edges(START, self.fan_out_sources, ["extract_source"])
        builder.add_edge("extract_source", "consolidate")
        builder.add_edge("consolidate", "validate")
        builder.add_conditional_edges("validate", self.route_validation, ["repair", "finalize"])
        builder.add_edge("repair", "finalize")
        builder.add_edge("finalize", "evaluate")
        builder.add_edge("evaluate", END)
        return builder.compile(checkpointer=checkpointer)

    def fan_out_sources(self, state: WorkflowState) -> list[Send]:
        return [Send("extract_source", {"source": source, **state}) for source in state["sources"]]

    async def extract_source(self, state: dict[str, Any]) -> dict[str, list[Draft]]:
        source = Source.model_validate(state["source"])
        prompt = render(
            load_prompt("tempo-product-extraction.md"),
            {
                "TARGET_PRODUCT": state["target_product"],
                "AS_OF_DATE": state.get("as_of_date") or "null",
                "SOURCE": source.source,
                "CONTENT": source.content,
            },
        )
        result = await self.client.complete_structured("extract_source", prompt, Passport)
        return {"drafts": [Draft(source=source.source, result=result)]}

    async def consolidate(self, state: WorkflowState) -> dict[str, Passport]:
        drafts = sorted(
            (Draft.model_validate(draft) for draft in state.get("drafts", [])),
            key=lambda draft: draft.source,
        )
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
            target_product=state["target_product"],
            as_of_date=state.get("as_of_date") or "null",
            drafts=json.dumps([draft.model_dump(mode="json") for draft in drafts], ensure_ascii=False, indent=2),
        )
        return {"passport": await self.client.complete_structured("consolidate", prompt, Passport)}

    def validate(self, state: WorkflowState) -> dict[str, Validation]:
        passport = _passport_from_state(state.get("passport"))
        errors = validate_passport(passport)
        return {"validation": Validation(valid=not errors, errors=errors)}

    def route_validation(self, state: WorkflowState) -> Literal["repair", "finalize"]:
        validation = Validation.model_validate(state["validation"])
        if validation.valid and not state.get("repair_attempted", False):
            return "finalize"
        if state.get("repair_attempted", False):
            return "finalize"
        return "repair"

    async def repair(self, state: WorkflowState) -> dict[str, object]:
        passport = _passport_from_state(state["passport"])
        validation = Validation.model_validate(state["validation"])
        errors = validation.errors
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
            passport=json.dumps(passport.model_dump(mode="json"), ensure_ascii=False, indent=2),
            errors=json.dumps(errors, ensure_ascii=False),
        )
        repaired = await self.client.complete_structured("repair", prompt, Passport)
        return {"passport": repaired, "repair_attempted": True}

    def finalize(self, state: WorkflowState) -> dict[str, Validation]:
        errors = validate_passport(_passport_from_state(state.get("passport")))
        return {"validation": Validation(valid=not errors, errors=errors)}

    async def evaluate(self, state: WorkflowState) -> dict[str, Evaluation]:
        passport = _passport_from_state(state.get("passport"))
        validation = Validation.model_validate(state["validation"])
        materials = [Source.model_validate(source).model_dump(mode="json") for source in state["sources"]]
        prompt = render(
            load_prompt("tempo-product-extraction-evaluation.md"),
            {
                "PASSPORT_JSON": json.dumps(passport.model_dump(mode="json"), ensure_ascii=False, indent=2),
                "VALIDATION_JSON": json.dumps(validation.model_dump(mode="json"), ensure_ascii=False),
                "SOURCE_MATERIALS_JSON": json.dumps(materials, ensure_ascii=False, indent=2),
            },
        )
        return {"evaluation": await self.client.complete_structured("evaluate", prompt, Evaluation)}


def validate_passport(passport: Passport | None) -> list[str]:
    if passport is None:
        return ["Output is not a JSON object."]

    errors: list[str] = []
    for index, record in enumerate(passport.records, start=1):
        if record.id != f"r{index}":
            errors.append(f"Record {index} must have id r{index}.")
        if not record.statement:
            errors.append(f"Missing statement on {record.id}.")
        if not record.evidence:
            errors.append(f"Missing evidence on {record.id}.")
        for evidence in record.evidence:
            if not evidence.source or not evidence.quote:
                errors.append(f"Evidence requires source and quote on {record.id}.")
    record_ids = {record.id for record in passport.records}
    for conflict in passport.conflicts:
        if len(conflict.record_ids) < 2:
            errors.append("Each conflict needs at least two record_ids.")
        missing = set(conflict.record_ids) - record_ids
        if missing:
            errors.append(f"Conflict references unknown records: {', '.join(sorted(missing))}.")
    return errors


def _passport_from_state(value: Passport | dict[str, Any] | None) -> Passport | None:
    if value is None:
        return None
    return Passport.model_validate(value)
