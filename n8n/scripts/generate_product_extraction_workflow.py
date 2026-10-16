#!/usr/bin/env python3
"""Generate the separate n8n workflow for the product extraction prompt."""

from __future__ import annotations

import json
import uuid
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
PROMPT_FILE = ROOT / "n8n/prompts/tempo-product-extraction.md"
EVALUATION_PROMPT_FILE = ROOT / "n8n/prompts/tempo-product-extraction-evaluation.md"
OUTPUT_FILE = ROOT / "n8n/workflows/tempo-product-extraction.json"


def node_id() -> str:
    return str(uuid.uuid4())


def basic_chain(name: str, prompt: str, position: list[int]) -> dict:
    return {
        "parameters": {
            "promptType": "define",
            "text": f"={prompt}",
            "options": {},
            "batching": {},
        },
        "type": "@n8n/n8n-nodes-langchain.chainLlm",
        "typeVersion": 1.7,
        "position": position,
        "id": node_id(),
        "name": name,
    }


def replace_placeholders(template: str, replacements: dict[str, str]) -> str:
    result = template
    for placeholder, expression in replacements.items():
        result = result.replace(f"{{{{{placeholder}}}}}", f"{{{{ {expression} }}}}")
    return result


def code_node(name: str, js_code: str, position: list[int]) -> dict:
    return {
        "parameters": {"jsCode": js_code},
        "type": "n8n-nodes-base.code",
        "typeVersion": 2,
        "position": position,
        "id": node_id(),
        "name": name,
    }


def passport_schema() -> dict:
    nullable_string = {"anyOf": [{"type": "string"}, {"type": "null"}]}
    evidence = {
        "type": "object",
        "properties": {
            "source": {"type": "string"},
            "locator": nullable_string,
            "quote": {"type": "string"},
        },
        "required": ["source", "locator", "quote"],
        "additionalProperties": False,
    }
    record = {
        "type": "object",
        "properties": {
            "id": {"type": "string"},
            "category": {"type": "string", "enum": ["identity", "purpose", "audience", "features", "benefits", "pricing", "terms", "limitations", "economics", "funnel", "offers_channels", "communication_rules"]},
            "subject": {"type": "string"},
            "statement": {"type": "string"},
            "statement_type": {"type": "string", "enum": ["description", "claim", "reported_result", "plan", "target", "rule"]},
            "conditions": {"type": "array", "items": {"type": "string"}},
            "time_scope": nullable_string,
            "evidence_status": {"type": "string", "enum": ["supported", "conflicted"]},
            "evidence": {"type": "array", "items": evidence, "minItems": 1},
        },
        "required": ["id", "category", "subject", "statement", "statement_type", "conditions", "time_scope", "evidence_status", "evidence"],
        "additionalProperties": False,
    }
    conflict = {
        "type": "object",
        "properties": {
            "record_ids": {"type": "array", "items": {"type": "string"}, "minItems": 2},
            "description": {"type": "string"},
            "needed_to_resolve": {"type": "string"},
        },
        "required": ["record_ids", "description", "needed_to_resolve"],
        "additionalProperties": False,
    }
    gap = {
        "type": "object",
        "properties": {"category": {"type": "string"}, "missing": {"type": "string"}},
        "required": ["category", "missing"],
        "additionalProperties": False,
    }
    return {
        "type": "object",
        "properties": {
            "product_name": nullable_string,
            "extraction_status": {"type": "string", "enum": ["complete", "partial", "blocked"]},
            "records": {"type": "array", "items": record},
            "conflicts": {"type": "array", "items": conflict},
            "gaps": {"type": "array", "items": gap},
            "warnings": {"type": "array", "items": {"type": "string"}},
        },
        "required": ["product_name", "extraction_status", "records", "conflicts", "gaps", "warnings"],
        "additionalProperties": False,
    }


def structured_parser(name: str, position: list[int]) -> dict:
    return {
        "parameters": {
            "schemaType": "manual",
            "inputSchema": json.dumps(passport_schema(), ensure_ascii=False, indent=2),
            "autoFix": True,
            "customizeRetryPrompt": False,
        },
        "type": "@n8n/n8n-nodes-langchain.outputParserStructured",
        "typeVersion": 1.3,
        "position": position,
        "id": node_id(),
        "name": name,
        "notes": "Native n8n JSON Schema validation. Auto-fix performs a model retry before the downstream validator.",
        "notesInFlow": True,
    }


def evaluation_schema() -> dict:
    criterion = {
        "type": "object",
        "properties": {
            "id": {"type": "string"},
            "name": {"type": "string"},
            "weight": {"type": "number"},
            "score": {"anyOf": [{"type": "number", "minimum": 0, "maximum": 5}, {"type": "null"}]},
            "status": {"type": "string", "enum": ["assessed", "not_tested"]},
            "weighted_score": {"anyOf": [{"type": "number", "minimum": 0, "maximum": 100}, {"type": "null"}]},
            "findings": {"type": "array", "items": {"type": "string"}},
            "recommendations": {"type": "array", "items": {"type": "string"}},
        },
        "required": ["id", "name", "weight", "score", "status", "weighted_score", "findings", "recommendations"],
        "additionalProperties": False,
    }
    mandatory_check = {
        "type": "object",
        "properties": {
            "id": {"type": "string"},
            "name": {"type": "string"},
            "status": {"type": "string", "enum": ["pass", "fail", "not_tested"]},
            "finding": {"type": "string"},
        },
        "required": ["id", "name", "status", "finding"],
        "additionalProperties": False,
    }
    scenario = {
        "type": "object",
        "properties": {
            "number": {"type": "integer", "minimum": 1, "maximum": 12},
            "status": {"type": "string", "enum": ["pass", "fail", "partial", "not_tested"]},
            "finding": {"type": "string"},
        },
        "required": ["number", "status", "finding"],
        "additionalProperties": False,
    }
    return {
        "type": "object",
        "properties": {
            "evaluation_status": {"type": "string", "enum": ["complete", "partial", "blocked"]},
            "mode": {"type": "string", "enum": ["single_run_source_audit", "benchmark"]},
            "overall_score": {"anyOf": [{"type": "number", "minimum": 0, "maximum": 100}, {"type": "null"}]},
            "acceptance": {"type": "string", "enum": ["accepted", "rejected", "not_tested"]},
            "score_basis": {"type": "string"},
            "criteria": {"type": "array", "items": criterion, "minItems": 9},
            "mandatory_checks": {"type": "array", "items": mandatory_check, "minItems": 6},
            "scenarios": {"type": "array", "items": scenario, "minItems": 12},
            "summary": {"type": "string"},
            "limitations": {"type": "array", "items": {"type": "string"}},
        },
        "required": ["evaluation_status", "mode", "overall_score", "acceptance", "score_basis", "criteria", "mandatory_checks", "scenarios", "summary", "limitations"],
        "additionalProperties": False,
    }


def evaluation_parser(name: str, position: list[int]) -> dict:
    return {
        "parameters": {
            "schemaType": "manual",
            "inputSchema": json.dumps(evaluation_schema(), ensure_ascii=False, indent=2),
            "autoFix": True,
            "customizeRetryPrompt": False,
        },
        "type": "@n8n/n8n-nodes-langchain.outputParserStructured",
        "typeVersion": 1.3,
        "position": position,
        "id": node_id(),
        "name": name,
        "notes": "Structured evaluation output. A single-run audit cannot establish acceptance without the fixed benchmark and reference.",
        "notesInFlow": True,
    }


def main() -> None:
    prompt = PROMPT_FILE.read_text()
    evaluation_prompt = EVALUATION_PROMPT_FILE.read_text()

    trigger = {
        "parameters": {},
        "type": "n8n-nodes-base.manualTrigger",
        "typeVersion": 1,
        "position": [-1320, -220],
        "id": node_id(),
        "name": "Запустить экстракцию",
    }
    read_sources = {
        "parameters": {"fileSelector": "/product/*.md", "options": {}},
        "type": "n8n-nodes-base.readWriteFile",
        "typeVersion": 1.1,
        "position": [-1080, -320],
        "id": node_id(),
        "name": "Материалы продукта",
        "notes": "Reads the mounted Tempo product documents. Replace this node with an API, database, or webhook input when needed.",
        "notesInFlow": True,
    }
    run_parameters = {
        "parameters": {
            "assignments": {
                "assignments": [
                    {"id": node_id(), "name": "target_product", "value": "Tempo", "type": "string"},
                    {"id": node_id(), "name": "as_of_date", "value": "", "type": "string"},
                ]
            },
            "options": {},
        },
        "type": "n8n-nodes-base.set",
        "typeVersion": 3.4,
        "position": [-1080, -100],
        "id": node_id(),
        "name": "Параметры запуска",
        "notes": "Edit the target product and optional effective date before executing.",
        "notesInFlow": True,
    }
    merge_input = {
        "parameters": {"mode": "append", "numberInputs": 2},
        "type": "n8n-nodes-base.merge",
        "typeVersion": 3.2,
        "position": [-840, -220],
        "id": node_id(),
        "name": "Материалы + параметры",
    }
    prepare_sources = code_node(
        "Подготовить источники",
        """const settings = items.find((item) => item.json?.target_product)?.json ?? {};
const sources = [];

for (let index = 0; index < items.length; index += 1) {
  const item = items[index];
  if (!item.binary?.data) continue;

  const buffer = await this.helpers.getBinaryDataBuffer(index, 'data');
  const binary = item.binary.data;
  const source = binary.fileName ?? item.json.fileName ?? `document-${index + 1}.md`;
  sources.push({
    target_product: settings.target_product ?? 'Tempo',
    as_of_date: settings.as_of_date || null,
    source,
    content: buffer.toString('utf8'),
  });
}

if (sources.length === 0) {
  return [{
    json: {
      target_product: settings.target_product ?? 'Tempo',
      as_of_date: settings.as_of_date || null,
      source: null,
      content: '',
      preparation_warning: 'No readable source documents were provided.',
    },
  }];
}

return sources.map((source) => ({ json: source }));""",
        [-600, -220],
    )

    extraction_prompt = replace_placeholders(
        prompt,
        {
            "TARGET_PRODUCT": "$json.target_product",
            "AS_OF_DATE": "$json.as_of_date",
            "SOURCE": "$json.source",
            "CONTENT": "$json.content",
        },
    )
    extract = basic_chain("Извлечь факты по источнику", extraction_prompt, [-320, -220])
    extract["notes"] = "Runs once per prepared source. Keep the prompt editable here."
    extract["notesInFlow"] = True
    extract_parser = structured_parser("Структура черновика", [-320, 20])

    collect_extractions = code_node(
        "Собрать черновики",
        """function parseJson(value) {
  if (typeof value !== 'string') return value;
  const cleaned = value.trim().replace(/^```(?:json)?\\s*/i, '').replace(/\\s*```$/, '');
  try { return JSON.parse(cleaned); } catch (error) {
    return { extraction_status: 'partial', records: [], conflicts: [], gaps: [], warnings: [`Invalid JSON from source extractor: ${error.message}`], raw: value };
  }
}

const drafts = $input.all().map((item, index) => ({
  source: $('Подготовить источники').all()[index]?.json?.source ?? null,
  result: parseJson(item.json.text ?? item.json.output ?? ''),
}));

return [{
  json: {
    source_extractions: JSON.stringify(drafts, null, 2),
    extraction_errors: JSON.stringify(drafts.filter((draft) => draft.result?.raw).map((draft) => draft.source)),
  },
}];""",
        [-40, -220],
    )

    consolidation_prompt = """Ты — этап консолидации паспорта продукта после независимой экстракции по источникам.

Используй правила и формат результата из основного промпта ниже. Объедини только поддержанные записи из черновиков. Не добавляй утверждения, которых нет в черновиках. Сохрани все доказательства и существенные условия.

Правила консолидации:
1. Объединяй только дубли об одном объекте и при одинаковых условиях, добавляя все evidence.
2. Если утверждения несовместимы после проверки продукта, тарифа, периода, аудитории и условий, сохрани обе записи и добавь conflict.
3. Перенумеруй итоговые records последовательно от r1 и отсортируй их по порядку категорий из схемы.
4. Если хотя бы один источник не обработан, поставь extraction_status partial. Если пригодного содержимого нет, поставь blocked.
5. Не превращай gaps, warnings или неизвестные значения в факты.
6. Верни только валидный JSON итогового паспорта. Не используй Markdown fences.

Целевой продукт: {{TARGET_PRODUCT}}
Дата актуальности: {{AS_OF_DATE}}

Черновики экстракции по источникам:
{{SOURCE_EXTRACTIONS}}

Ошибки подготовки/экстракции:
{{EXTRACTION_ERRORS}}

Итоговая структура:
{
  "product_name": null,
  "extraction_status": "complete|partial|blocked",
  "records": [],
  "conflicts": [],
  "gaps": [],
  "warnings": []
}

Допустимые category: identity, purpose, audience, features, benefits, pricing, terms, limitations, economics, funnel, offers_channels, communication_rules.
Допустимые statement_type: description, claim, reported_result, plan, target, rule.
Допустимые evidence_status: supported, conflicted.
"""
    consolidation_prompt = replace_placeholders(
        consolidation_prompt,
        {
            "TARGET_PRODUCT": "$('Параметры запуска').first().json.target_product",
            "AS_OF_DATE": "$('Параметры запуска').first().json.as_of_date || null",
            "SOURCE_EXTRACTIONS": "$json.source_extractions",
            "EXTRACTION_ERRORS": "$json.extraction_errors",
        },
    )
    consolidate = basic_chain("Свести паспорт", consolidation_prompt, [240, -220])
    consolidate["notes"] = "Merges source-level passports, deduplicates records, and preserves conflicts."
    consolidate["notesInFlow"] = True
    consolidate_parser = structured_parser("Структура итогового паспорта", [240, 20])

    validate = code_node(
        "Проверить JSON паспорта",
        """function parseJson(value) {
  if (typeof value !== 'string') return value;
  const cleaned = value.trim().replace(/^```(?:json)?\\s*/i, '').replace(/\\s*```$/, '');
  try { return JSON.parse(cleaned); } catch (error) { return null; }
}

function validatePassport(passport) {
  const errors = [];
  const categories = ['identity', 'purpose', 'audience', 'features', 'benefits', 'pricing', 'terms', 'limitations', 'economics', 'funnel', 'offers_channels', 'communication_rules'];
  const statementTypes = ['description', 'claim', 'reported_result', 'plan', 'target', 'rule'];
  const statuses = ['supported', 'conflicted'];
  const extractionStatuses = ['complete', 'partial', 'blocked'];

  if (!passport || typeof passport !== 'object') return ['Output is not a JSON object.'];
  for (const key of ['product_name', 'extraction_status', 'records', 'conflicts', 'gaps', 'warnings']) {
    if (!(key in passport)) errors.push(`Missing top-level field: ${key}`);
  }
  if (!extractionStatuses.includes(passport.extraction_status)) errors.push('Invalid extraction_status.');
  if (!Array.isArray(passport.records)) errors.push('records must be an array.');

  const records = Array.isArray(passport.records) ? passport.records : [];
  records.forEach((record, index) => {
    if (record.id !== `r${index + 1}`) errors.push(`Record ${index + 1} must have id r${index + 1}.`);
    if (!categories.includes(record.category)) errors.push(`Invalid category on ${record.id}.`);
    if (!statementTypes.includes(record.statement_type)) errors.push(`Invalid statement_type on ${record.id}.`);
    if (!record.statement || typeof record.statement !== 'string') errors.push(`Missing statement on ${record.id}.`);
    if (!Array.isArray(record.conditions)) errors.push(`conditions must be an array on ${record.id}.`);
    if (!statuses.includes(record.evidence_status)) errors.push(`Invalid evidence_status on ${record.id}.`);
    if (!Array.isArray(record.evidence) || record.evidence.length === 0) errors.push(`Missing evidence on ${record.id}.`);
    for (const evidence of record.evidence ?? []) {
      if (!evidence.source || !evidence.quote) errors.push(`Evidence requires source and quote on ${record.id}.`);
    }
  });
  for (const conflict of passport.conflicts ?? []) {
    if (!Array.isArray(conflict.record_ids) || conflict.record_ids.length < 2) errors.push('Each conflict needs at least two record_ids.');
  }
  return errors;
}

const raw = $json.text ?? $json.output ?? '';
const passport = parseJson(raw);
const errors = validatePassport(passport);
return [{ json: {
  passport,
  passport_json: JSON.stringify(passport, null, 2),
  validation: { valid: errors.length === 0, errors },
  validation_errors: JSON.stringify(errors),
  raw,
} }];""",
        [520, -220],
    )

    needs_repair = {
        "parameters": {
            "conditions": {
                "options": {"caseSensitive": True, "leftValue": "", "typeValidation": "strict", "version": 2},
                "conditions": [
                    {
                        "id": node_id(),
                        "leftValue": "={{ $json.validation.valid }}",
                        "rightValue": True,
                        "operator": {"type": "boolean", "operation": "true", "singleValue": True},
                    }
                ],
                "combinator": "and",
            },
            "options": {},
        },
        "type": "n8n-nodes-base.if",
        "typeVersion": 2.2,
        "position": [760, -220],
        "id": node_id(),
        "name": "Паспорт валиден?",
    }

    repair_prompt = """Исправь JSON паспорта продукта по результатам механической проверки.

Не добавляй новых фактов. Используй только исходный JSON и уже имеющиеся evidence. Сохрани смысл, условия, конфликты и неизвестность. Верни только валидный JSON по схеме ниже, без Markdown fences. Исправь только структурные ошибки, перечисленные в validation.errors.

Исходный JSON:
{{PASSPORT}}

Ошибки проверки:
{{ERRORS}}

Схема и правила:
Верни объект с полями product_name, extraction_status, records, conflicts, gaps и warnings. Для каждой записи сохрани id, category, subject, statement, statement_type, conditions, time_scope, evidence_status и evidence. Не добавляй факты и не удаляй evidence.
"""
    repair_prompt = replace_placeholders(
        repair_prompt,
        {
            "PASSPORT": "$json.passport_json",
            "ERRORS": "$json.validation_errors",
        },
    )
    repair = basic_chain("Исправить паспорт", repair_prompt, [1000, 20])
    repair["notes"] = "Runs only when deterministic validation finds structural errors."
    repair["notesInFlow"] = True
    repair_parser = structured_parser("Структура исправленного паспорта", [1000, 260])

    final = code_node(
        "Финальный паспорт",
        """function parseJson(value) {
  if (typeof value !== 'string') return value;
  const cleaned = value.trim().replace(/^```(?:json)?\\s*/i, '').replace(/\\s*```$/, '');
  try { return JSON.parse(cleaned); } catch (error) { return null; }
}

const incoming = $input.all()[0]?.json ?? {};
const passport = incoming.passport ?? parseJson(incoming.text ?? incoming.output ?? '');
return [{
  json: {
    passport,
    validation: incoming.validation ?? { valid: false, errors: ['Repair output requires review.'] },
  },
}];""",
        [1240, -220],
    )

    prepare_evaluation = code_node(
        "Подготовить оценку",
        """const final = $input.all()[0]?.json ?? {};
const sourceItems = $('Подготовить источники').all();
const sourceMaterials = sourceItems.map((item) => ({
  source: item.json?.source ?? null,
  content: item.json?.content ?? '',
}));

return [{
  json: {
    passport_json: JSON.stringify(final.passport ?? null, null, 2),
    validation_json: JSON.stringify(final.validation ?? null, null, 2),
    source_materials_json: JSON.stringify(sourceMaterials, null, 2),
  },
}];""",
        [1480, -220],
    )

    evaluation_prompt = replace_placeholders(
        evaluation_prompt,
        {
            "PASSPORT_JSON": "$json.passport_json",
            "VALIDATION_JSON": "$json.validation_json",
            "SOURCE_MATERIALS_JSON": "$json.source_materials_json",
        },
    )
    evaluate = basic_chain("Оценить паспорт по критериям", evaluation_prompt, [1720, -220])
    evaluate["notes"] = "Runs after the final passport and audits it against the supplied evaluation standard and source materials."
    evaluate["notesInFlow"] = True
    evaluate_parser = evaluation_parser("Структура оценки паспорта", [1720, 20])

    evaluated_final = code_node(
        "Паспорт и оценка",
        """function parseJson(value) {
  if (typeof value !== 'string') return value;
  const cleaned = value.trim().replace(/^```(?:json)?\\s*/i, '').replace(/\\s*```$/, '');
  try { return JSON.parse(cleaned); } catch (error) { return null; }
}

const incoming = $input.all()[0]?.json ?? {};
const evaluation = incoming.evaluation ?? parseJson(incoming.text ?? incoming.output ?? '');
const final = $('Финальный паспорт').first().json ?? {};
return [{
  json: {
    passport: final.passport ?? null,
    validation: final.validation ?? null,
    evaluation,
  },
}];""",
        [1960, -220],
    )

    model = {
        "parameters": {"model": {"__rl": True, "mode": "list", "value": "gpt-4o-mini"}, "options": {}},
        "type": "@n8n/n8n-nodes-langchain.lmChatOpenAi",
        "typeVersion": 1.3,
        "position": [260, 300],
        "id": node_id(),
        "name": "OpenAI-compatible model (extraction)",
        "notes": "Attach the desired credential here. This is a separate model node from the existing passport workflow.",
        "notesInFlow": True,
    }

    nodes = [
        trigger,
        read_sources,
        run_parameters,
        merge_input,
        prepare_sources,
        extract,
        extract_parser,
        collect_extractions,
        consolidate,
        consolidate_parser,
        validate,
        needs_repair,
        repair,
        repair_parser,
        final,
        prepare_evaluation,
        evaluate,
        evaluate_parser,
        evaluated_final,
        model,
    ]

    connections = {
        "Запустить экстракцию": {"main": [[
            {"node": "Материалы продукта", "type": "main", "index": 0},
            {"node": "Параметры запуска", "type": "main", "index": 0},
        ]]},
        "Материалы продукта": {"main": [[{"node": "Материалы + параметры", "type": "main", "index": 0}]]},
        "Параметры запуска": {"main": [[{"node": "Материалы + параметры", "type": "main", "index": 1}]]},
        "Материалы + параметры": {"main": [[{"node": "Подготовить источники", "type": "main", "index": 0}]]},
        "Подготовить источники": {"main": [[{"node": "Извлечь факты по источнику", "type": "main", "index": 0}]]},
        "Извлечь факты по источнику": {"main": [[{"node": "Собрать черновики", "type": "main", "index": 0}]]},
        "Собрать черновики": {"main": [[{"node": "Свести паспорт", "type": "main", "index": 0}]]},
        "Свести паспорт": {"main": [[{"node": "Проверить JSON паспорта", "type": "main", "index": 0}]]},
        "Проверить JSON паспорта": {"main": [[{"node": "Паспорт валиден?", "type": "main", "index": 0}]]},
        "Паспорт валиден?": {"main": [
            [{"node": "Финальный паспорт", "type": "main", "index": 0}],
            [{"node": "Исправить паспорт", "type": "main", "index": 0}],
        ]},
        "Исправить паспорт": {"main": [[{"node": "Финальный паспорт", "type": "main", "index": 0}]]},
        "Финальный паспорт": {"main": [[{"node": "Подготовить оценку", "type": "main", "index": 0}]]},
        "Подготовить оценку": {"main": [[{"node": "Оценить паспорт по критериям", "type": "main", "index": 0}]]},
        "Оценить паспорт по критериям": {"main": [[{"node": "Паспорт и оценка", "type": "main", "index": 0}]]},
        "Структура черновика": {"ai_outputParser": [[{"node": "Извлечь факты по источнику", "type": "ai_outputParser", "index": 0}]]},
        "Структура итогового паспорта": {"ai_outputParser": [[{"node": "Свести паспорт", "type": "ai_outputParser", "index": 0}]]},
        "Структура исправленного паспорта": {"ai_outputParser": [[{"node": "Исправить паспорт", "type": "ai_outputParser", "index": 0}]]},
        "Структура оценки паспорта": {"ai_outputParser": [[{"node": "Оценить паспорт по критериям", "type": "ai_outputParser", "index": 0}]]},
    }

    for chain_name in (
        "Извлечь факты по источнику",
        "Свести паспорт",
        "Исправить паспорт",
        "Структура черновика",
        "Структура итогового паспорта",
        "Структура исправленного паспорта",
        "Оценить паспорт по критериям",
        "Структура оценки паспорта",
    ):
        connections.setdefault("OpenAI-compatible model (extraction)", {"ai_languageModel": [[]]})
        connections["OpenAI-compatible model (extraction)"]["ai_languageModel"][0].append({"node": chain_name, "type": "ai_languageModel", "index": 0})

    workflow = {
        "id": node_id(),
        "name": "Tempo product extraction — structured output + evaluation",
        "nodes": nodes,
        "connections": connections,
        "pinData": {},
        "active": False,
        "settings": {"executionOrder": "v1"},
        "versionId": "tempo-product-extraction-v1",
        "meta": {"templateCredsSetupCompleted": False},
        "tags": [],
    }
    OUTPUT_FILE.write_text(json.dumps(workflow, ensure_ascii=False, indent=2) + "\n")
    print(f"wrote {OUTPUT_FILE}")


if __name__ == "__main__":
    main()
