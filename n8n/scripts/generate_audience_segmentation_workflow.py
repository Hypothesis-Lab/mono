#!/usr/bin/env python3
"""Generate the importable, aggregate-only Tempo audience n8n workflow."""

from __future__ import annotations

import json
import uuid
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
PROMPT = ROOT / "n8n/prompts/tempo-audience.md"
STAGE_PROMPT = ROOT / "n8n/prompts/tempo-audience-stage.md"
OUTPUT = ROOT / "n8n/workflows/tempo-audience-segmentation.json"


def node_id() -> str:
    return str(uuid.uuid4())


def code(name: str, js_code: str, position: list[int]) -> dict:
    return {
        "parameters": {"jsCode": js_code},
        "type": "n8n-nodes-base.code",
        "typeVersion": 2,
        "position": position,
        "id": node_id(),
        "name": name,
    }


def chain(name: str, text: str, position: list[int]) -> dict:
    return {
        "parameters": {"promptType": "define", "text": f"={text}", "options": {}, "batching": {}},
        "type": "@n8n/n8n-nodes-langchain.chainLlm",
        "typeVersion": 1.7,
        "position": position,
        "id": node_id(),
        "name": name,
    }


def parser(name: str, position: list[int]) -> dict:
    return {
        "parameters": {
            "schemaType": "manual",
            "inputSchema": json.dumps({"type": "object", "additionalProperties": True}),
            "autoFix": True,
            "customizeRetryPrompt": False,
        },
        "type": "@n8n/n8n-nodes-langchain.outputParserStructured",
        "typeVersion": 1.3,
        "position": position,
        "id": node_id(),
        "name": name,
        "notes": "Parses JSON before the deterministic validation layer. Keep the stage prompt and contract editable.",
        "notesInFlow": True,
    }


def main() -> None:
    base = PROMPT.read_text(encoding="utf-8")
    stage = STAGE_PROMPT.read_text(encoding="utf-8")
    stage_prefix = f"{base}\n\n{stage}"

    manual = {
        "parameters": {},
        "type": "n8n-nodes-base.manualTrigger",
        "typeVersion": 1,
        "position": [-1040, -200],
        "id": node_id(),
        "name": "Запустить аудитории",
    }
    read_fixture = {
        "parameters": {"fileSelector": "/audience/context.json", "options": {}},
        "type": "n8n-nodes-base.readWriteFile",
        "typeVersion": 1.1,
        "position": [-800, -300],
        "id": node_id(),
        "name": "Агрегированный контекст аудитории",
        "notes": "Only the mounted aggregate fixture is read. Do not replace it with raw client exports.",
        "notesInFlow": True,
    }
    passport = {
        "parameters": {
            "assignments": {
                "assignments": [
                    {
                        "id": node_id(),
                        "name": "product_passport_json",
                        "value": "",
                        "type": "string",
                    }
                ]
            },
            "options": {},
        },
        "type": "n8n-nodes-base.set",
        "typeVersion": 3.4,
        "position": [-800, -80],
        "id": node_id(),
        "name": "Паспорт продукта",
        "notes": "Paste or connect a real product-passport JSON. Empty or invented fallback passports are rejected.",
        "notesInFlow": True,
    }
    merge = {
        "parameters": {"mode": "append", "numberInputs": 2},
        "type": "n8n-nodes-base.merge",
        "typeVersion": 3.2,
        "position": [-600, -200],
        "id": node_id(),
        "name": "Контекст + паспорт",
    }
    prepare = code(
        "Подготовить безопасный вход",
        """const fixtureIndex = items.findIndex((item) => item.binary?.data);
const passportItem = items.find((item) => typeof item.json?.product_passport_json === 'string');
if (fixtureIndex < 0) throw new Error('Aggregate audience fixture was not loaded.');
if (!passportItem?.json.product_passport_json?.trim()) {
  throw new Error('A real product_passport_json handoff is required.');
}
const context = JSON.parse((await this.helpers.getBinaryDataBuffer(fixtureIndex, 'data')).toString('utf8'));
const passport = JSON.parse(passportItem.json.product_passport_json);
const forbidden = /(^|_)(client_?id|email|phone|contact|first_?name|last_?name|address)($|_)/i;
const scan = (value, path = '') => {
  if (Array.isArray(value)) value.forEach((item, index) => scan(item, `${path}[${index}]`));
  else if (value && typeof value === 'object') {
    for (const [key, item] of Object.entries(value)) {
      if (forbidden.test(key)) throw new Error(`Raw-record/contact field is not allowed: ${path}.${key}`);
      scan(item, `${path}.${key}`);
    }
  }
};
scan(context);
scan(passport);
context.product_passport = passport;
return [{ json: { context } }];""",
        [-360, -200],
    )
    profile = chain(
        "Профилировать аудиторию",
        stage_prefix
        + "\n\nЭТАП: profile. Сформируй компактный профиль данных, допустимых признаков, наблюдений, неизвестностей и вопросов. Не создавай сегменты.\nКОНТЕКСТ:\n{{ JSON.stringify($json.context) }}",
        [-100, -200],
    )
    segment = chain(
        "Сегментировать аудиторию",
        stage_prefix
        + "\n\nЭТАП: segment. Используй только доверенные calculation_results из контекста для observed сегментов. Не пересчитывай значения.\nКОНТЕКСТ:\n{{ JSON.stringify($('Подготовить безопасный вход').first().json.context) }}\nПРОФИЛЬ:\n{{ JSON.stringify($json.output ?? $json.text ?? $json) }}",
        [180, -200],
    )
    interpret = chain(
        "Интерпретировать сегменты",
        stage_prefix
        + "\n\nЭТАП: interpret. Сформируй полный AudienceResult. Unknown остаётся unknown; review_status всегда pending.\nПРОФИЛЬ:\n{{ JSON.stringify($('Профилировать аудиторию').first().json.output ?? $('Профилировать аудиторию').first().json.text ?? $('Профилировать аудиторию').first().json) }}\nПЛАН:\n{{ JSON.stringify($json.output ?? $json.text ?? $json) }}",
        [460, -200],
    )
    normalize = code(
        "Нормализовать и проверить",
        """const unwrap = (value) => typeof value === 'string' ? JSON.parse(value) : value;
const audience = unwrap($json.output ?? $json.text ?? $json);
const context = $('Подготовить безопасный вход').first().json.context;
const calculations = context.calculation_results ?? {};
const source = context.audience_sources?.length === 1 ? context.audience_sources[0] : null;
const find = (suffix) => Object.entries(calculations).find(([key]) => key.endsWith(suffix));
const quality = find('#quality'); const privacy = find('#privacy'); const time = find('#time');
const stability = find('#stability'); const activation = find('#activation'); const relevance = find('#relevance');
const status = (entry) => ['pass', 'fail', 'unknown'].includes(entry?.[1]?.status) ? entry[1].status : 'unknown';
const ref = (entry) => entry ? [entry[0]] : [];
const errors = [];
if (!audience || typeof audience !== 'object') errors.push('Audience result must be a JSON object.');
if (audience?.mode === 'data_based') {
  for (const segment of audience.segments ?? []) {
    const match = Object.entries(calculations).find(([, value]) => value?.rule_ref === segment.definition?.rule_or_model_ref && value?.membership_ref === segment.definition?.membership_ref);
    if (!match) { errors.push(`Observed segment lacks trusted artifacts: ${segment.segment_id ?? 'unknown'}`); continue; }
    const [calculationRef, calculation] = match;
    segment.origin = 'observed';
    segment.definition = { inclusion_rules: calculation.inclusion_rules ?? [], exclusion_rules: calculation.exclusion_rules ?? [], missing_value_policy: calculation.missing_value_policy ?? null, method: calculation.method ?? null, rule_or_model_ref: calculation.rule_ref, membership_ref: calculation.membership_ref };
    segment.size = { count: calculation.count, base_count: calculation.base_count, share: calculation.share, population: calculation.population, source_refs: [calculationRef] };
    segment.checks = [
      ['data_quality', quality?.[1]?.missing_feature_values === 0 && quality?.[1]?.duplicate_clients_after_deduplication === 0 && quality?.[1]?.join_expansion === false ? 'pass' : 'unknown', ref(quality)],
      ['relevance', status(relevance), ref(relevance)], ['reproducibility', 'pass', [calculationRef]],
      ['min_size', calculation.count >= (context.config?.min_segment_size ?? Infinity) && calculation.count >= (context.config?.small_group_suppression_threshold ?? Infinity) ? 'pass' : 'unknown', [calculationRef]],
      ['stability', status(stability), ref(stability)], ['privacy_bias', privacy?.[1]?.bias_check === 'pass' ? 'pass' : 'unknown', ref(privacy)],
      ['leakage', time?.[1]?.all_event_times_before_cutoff && time?.[1]?.all_available_at_before_cutoff && time?.[1]?.current_experiment_fields_used === false ? 'pass' : 'unknown', ref(time)],
      ['activation', status(activation), ref(activation)],
    ].map(([name, checkStatus, source_refs]) => ({ name, status: checkStatus, detail: 'Status is derived from trusted calculation artifacts.', source_refs }));
  }
}
if (source) {
  audience.data_profile = { unit: source.unit ?? null, cutoff_at: source.cutoff_at ?? null, observation_window: source.window ?? { start: null, end: null }, snapshot_refs: source.snapshot_ref ? [source.snapshot_ref] : [], coverage: { text: quality?.[1]?.coverage_share == null ? null : `Aggregate coverage: ${quality[1].coverage_share}.`, kind: quality?.[1]?.coverage_share == null ? 'unknown' : 'fact', source_refs: quality?.[1]?.coverage_share == null ? [] : ref(quality), rationale: quality?.[1]?.coverage_share == null ? 'Coverage was not supplied.' : null }, quality_issues: [] };
}
audience.review_status = 'pending';
return [{ json: { audience, validation: { valid: errors.length === 0, errors }, repair_attempted: $json.repair_attempted === true } }];""",
        [740, -200],
    )
    valid = {
        "parameters": {"conditions": {"options": {"caseSensitive": True, "leftValue": "", "typeValidation": "strict"}, "conditions": [{"id": node_id(), "leftValue": "={{ $json.validation.valid || $json.repair_attempted === true }}", "rightValue": True, "operator": {"type": "boolean", "operation": "true", "singleValue": True}}], "combinator": "and"}, "options": {}},
        "type": "n8n-nodes-base.if",
        "typeVersion": 2.2,
        "position": [980, -200],
        "id": node_id(),
        "name": "Валиден?",
    }
    repair = chain(
        "Исправить только контракт",
        stage_prefix
        + "\n\nЭТАП: repair. Исправь только ошибки проверки. Не добавляй факты, источники, расчёты или подтверждения.\nРЕЗУЛЬТАТ:\n{{ JSON.stringify($json.audience) }}\nОШИБКИ:\n{{ JSON.stringify($json.validation.errors) }}",
        [1220, -80],
    )
    repair_normalize = code(
        "Повторно проверить ремонт",
        """const repaired = $json.output ?? $json.text ?? $json;
return [{ json: { audience: typeof repaired === 'string' ? JSON.parse(repaired) : repaired, repair_attempted: true } }];""",
        [1500, -80],
    )
    review = code(
        "Детерминированный review",
        """const segmentChecks = ($json.audience?.segments ?? []).flatMap((segment) => segment.checks ?? []);
const unknown = segmentChecks.filter((check) => check.status === 'unknown').map((check) => check.name);
return [{ json: { ...$json, review: { critic_checks: [], validator_checks: unknown.map((name) => ({ name, status: 'unknown', detail: 'Trusted calculation artifact is missing or unresolved.', source_refs: [] })), product_manager_confirmation: { status: 'pending', detail: 'Product-manager confirmation is an external event.' } } } }];""",
        [1500, -260],
    )
    model = {
        "parameters": {"model": {"__rl": True, "mode": "list", "value": "gpt-4o-mini"}, "options": {}},
        "type": "@n8n/n8n-nodes-langchain.lmChatOpenAi",
        "typeVersion": 1.3,
        "position": [460, 140],
        "id": node_id(),
        "name": "OpenAI-compatible model (audience)",
        "notes": "Attach a credential before execution. This workflow never reads raw client records.",
        "notesInFlow": True,
    }
    profile_parser, segment_parser, interpret_parser, repair_parser = (
        parser("Структура профиля", [-100, 20]),
        parser("Структура плана", [180, 20]),
        parser("Структура результата", [460, 20]),
        parser("Структура ремонта", [1220, 140]),
    )
    nodes = [manual, read_fixture, passport, merge, prepare, profile, segment, interpret, normalize, valid, repair, repair_normalize, review, model, profile_parser, segment_parser, interpret_parser, repair_parser]
    connections = {
        manual["name"]: {"main": [[{"node": read_fixture["name"], "type": "main", "index": 0}, {"node": passport["name"], "type": "main", "index": 0}]]},
        read_fixture["name"]: {"main": [[{"node": merge["name"], "type": "main", "index": 0}]]},
        passport["name"]: {"main": [[{"node": merge["name"], "type": "main", "index": 1}]]},
        merge["name"]: {"main": [[{"node": prepare["name"], "type": "main", "index": 0}]]},
        prepare["name"]: {"main": [[{"node": profile["name"], "type": "main", "index": 0}]]},
        profile["name"]: {"main": [[{"node": segment["name"], "type": "main", "index": 0}]]},
        segment["name"]: {"main": [[{"node": interpret["name"], "type": "main", "index": 0}]]},
        interpret["name"]: {"main": [[{"node": normalize["name"], "type": "main", "index": 0}]]},
        normalize["name"]: {"main": [[{"node": valid["name"], "type": "main", "index": 0}]]},
        valid["name"]: {"main": [[{"node": review["name"], "type": "main", "index": 0}], [{"node": repair["name"], "type": "main", "index": 0}]]},
        repair["name"]: {"main": [[{"node": repair_normalize["name"], "type": "main", "index": 0}]]},
        repair_normalize["name"]: {"main": [[{"node": normalize["name"], "type": "main", "index": 0}]]},
    }
    chains = (profile, segment, interpret, repair)
    for prompt_node, output_parser in zip(chains, (profile_parser, segment_parser, interpret_parser, repair_parser)):
        connections[output_parser["name"]] = {
            "ai_outputParser": [[{"node": prompt_node["name"], "type": "ai_outputParser", "index": 0}]]
        }
    connections[model["name"]] = {
        "ai_languageModel": [[{"node": prompt_node["name"], "type": "ai_languageModel", "index": 0} for prompt_node in chains]]
    }
    workflow = {"name": "Tempo audience segmentation — aggregate only", "nodes": nodes, "connections": connections, "pinData": {}, "active": False, "settings": {"executionOrder": "v1"}, "versionId": node_id(), "meta": {"templateCredsSetupCompleted": False}, "tags": []}
    OUTPUT.write_text(json.dumps(workflow, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
