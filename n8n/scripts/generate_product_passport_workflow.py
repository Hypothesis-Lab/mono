#!/usr/bin/env python3
"""Generate the importable n8n version of the Tempo passport graph."""

from __future__ import annotations

import json
import re
import uuid
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
PROMPTS_FILE = ROOT / "llm/src/prompts.ts"
OUTPUT_FILE = ROOT / "n8n/workflows/tempo-product-passport.json"

PROMPT_IDS = ("questions", "parser", "brand", "insights", "critic", "validator", "output")


def prompt_templates() -> dict[str, str]:
    source = PROMPTS_FILE.read_text()
    templates: dict[str, str] = {}
    for index, prompt_id in enumerate(PROMPT_IDS):
        marker = f"  {prompt_id}: `"
        start = source.index(marker) + len(marker)
        end_marker = "`,\n  " if index < len(PROMPT_IDS) - 1 else "`,\n}"
        end = source.index(end_marker, start)
        templates[prompt_id] = source[start:end]
    return templates


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


def expression_prompt(template: str, replacements: dict[str, str]) -> str:
    result = template
    for key, expression in replacements.items():
        result = result.replace(f"{{{{{key}}}}}", f"{{{{ {expression} }}}}")
    return result


def main() -> None:
    prompts = prompt_templates()
    context_ref = "$('Собрать документы').first().json.productContext"
    source_files_ref = "$('Собрать документы').first().json.sourceFiles"
    focus_ref = "$('Собрать документы').first().json.focus"

    read_documents = {
        "parameters": {"fileSelector": "/product/*.md", "options": {}},
        "type": "n8n-nodes-base.readWriteFile",
        "typeVersion": 1.1,
        "position": [-760, -220],
        "id": node_id(),
        "name": "Документы продукта",
    }
    focus = {
        "parameters": {
            "assignments": {
                "assignments": [
                    {
                        "id": node_id(),
                        "name": "focus",
                        "value": "Проведи полный анализ продукта Tempo и подготовь основу для коммуникаций.",
                        "type": "string",
                    }
                ]
            },
            "options": {},
        },
        "type": "n8n-nodes-base.set",
        "typeVersion": 3.4,
        "position": [-760, 40],
        "id": node_id(),
        "name": "Фокус анализа",
    }
    assemble_documents = {
        "parameters": {
            "jsCode": """const documents = [];
for (let index = 0; index < items.length; index += 1) {
  if (!items[index].binary?.data) continue;
  const buffer = await this.helpers.getBinaryDataBuffer(index, 'data');
  const binary = items[index].binary?.data ?? {};
  const name = binary.fileName ?? items[index].json.fileName ?? `document-${index + 1}.md`;
  documents.push({ name, content: buffer.toString('utf8') });
}

return [{
  json: {
    focus: items.find((item) => item.json?.focus)?.json.focus ?? 'Проведи полный анализ продукта Tempo и подготовь основу для коммуникаций.',
    sourceFiles: documents.map((document) => document.name).join(', '),
    productContext: documents.map((document) => `--- ${document.name} ---\\n${document.content}`).join('\\n\\n'),
  },
}];"""
        },
        "type": "n8n-nodes-base.code",
        "typeVersion": 2,
        "position": [-500, -220],
        "id": node_id(),
        "name": "Собрать документы",
    }
    merge_context = {
        "parameters": {"mode": "append", "numberInputs": 2},
        "type": "n8n-nodes-base.merge",
        "typeVersion": 3.2,
        "position": [-620, -100],
        "id": node_id(),
        "name": "Документы + фокус",
    }

    questions_prompt = expression_prompt(
        prompts["questions"],
        {"FOCUS": "$json.focus", "PRODUCT_CONTEXT": "$json.productContext", "SOURCE_FILES": "$json.sourceFiles"},
    )
    questions = basic_chain("Вопросы к продукту", questions_prompt, [-220, -220])

    extractor_positions = {"parser": [180, -480], "brand": [180, -220], "insights": [180, 40]}
    extractor_names = {"parser": "Извлечение фактов продукта", "brand": "Извлечение бренда и ограничений", "insights": "Извлечение продуктовых инсайтов"}
    extractors = {}
    for prompt_id in ("parser", "brand", "insights"):
        extractors[prompt_id] = basic_chain(
            extractor_names[prompt_id],
            expression_prompt(
                prompts[prompt_id],
                {
                    "FOCUS": focus_ref,
                    "QUESTIONS": "$json.text",
                    "PRODUCT_CONTEXT": context_ref,
                    "SOURCE_FILES": source_files_ref,
                },
            ),
            extractor_positions[prompt_id],
        )

    merge_extractors = {
        "parameters": {"mode": "append", "numberInputs": 3},
        "type": "n8n-nodes-base.merge",
        "typeVersion": 3.2,
        "position": [500, -220],
        "id": node_id(),
        "name": "Синхронизировать извлечения",
    }
    assemble_extractors = {
        "parameters": {
            "jsCode": """const read = (name) => $(name).first().json.text ?? '';
return [{
  json: {
    extractors: JSON.stringify({
      facts: read('Извлечение фактов продукта'),
      brand: read('Извлечение бренда и ограничений'),
      insights: read('Извлечение продуктовых инсайтов'),
    }, null, 2),
  },
}];"""
        },
        "type": "n8n-nodes-base.code",
        "typeVersion": 2,
        "position": [740, -220],
        "id": node_id(),
        "name": "Собрать извлечения",
    }

    critic = basic_chain(
        "Критики",
        expression_prompt(
            prompts["critic"],
            {
                "FOCUS": focus_ref,
                "QUESTIONS": "$('Вопросы к продукту').first().json.text",
                "EXTRACTORS": "$json.extractors",
                "PRODUCT_CONTEXT": context_ref,
                "SOURCE_FILES": source_files_ref,
            },
        ),
        [1000, -220],
    )
    validator = basic_chain(
        "Валидаторы",
        expression_prompt(
            prompts["validator"],
            {
                "FOCUS": focus_ref,
                "EXTRACTORS": "$('Собрать извлечения').first().json.extractors",
                "CRITIC": "$json.text",
                "PRODUCT_CONTEXT": context_ref,
                "SOURCE_FILES": source_files_ref,
            },
        ),
        [1260, -220],
    )
    output = basic_chain(
        "Паспорт продукта",
        expression_prompt(
            prompts["output"],
            {
                "FOCUS": focus_ref,
                "QUESTIONS": "$('Вопросы к продукту').first().json.text",
                "EXTRACTORS": "$('Собрать извлечения').first().json.extractors",
                "CRITIC": "$('Критики').first().json.text",
                "VALIDATOR": "$json.text",
                "PRODUCT_CONTEXT": context_ref,
                "SOURCE_FILES": source_files_ref,
            },
        ),
        [1520, -220],
    )

    model = {
        "parameters": {
            "model": {"__rl": True, "mode": "list", "value": "gpt-4o-mini"},
            "options": {},
        },
        "type": "@n8n/n8n-nodes-langchain.lmChatOpenAi",
        "typeVersion": 1.3,
        "position": [700, 260],
        "id": node_id(),
        "name": "OpenAI-compatible model",
        "notes": "Select or create the model credential in this node. The prompt stages are intentionally Basic LLM Chains, not autonomous agents.",
        "notesInFlow": True,
    }

    nodes = [
        {"parameters": {}, "type": "n8n-nodes-base.manualTrigger", "typeVersion": 1, "position": [-1000, -220], "id": node_id(), "name": "Запустить паспорт"},
        read_documents,
        focus,
        merge_context,
        assemble_documents,
        questions,
        extractors["parser"],
        extractors["brand"],
        extractors["insights"],
        merge_extractors,
        assemble_extractors,
        critic,
        validator,
        output,
        model,
    ]

    connections = {
        "Запустить паспорт": {"main": [[
            {"node": "Документы продукта", "type": "main", "index": 0},
            {"node": "Фокус анализа", "type": "main", "index": 0},
        ]]},
        "Документы продукта": {"main": [[{"node": "Документы + фокус", "type": "main", "index": 0}]]},
        "Фокус анализа": {"main": [[{"node": "Документы + фокус", "type": "main", "index": 1}]]},
        "Документы + фокус": {"main": [[{"node": "Собрать документы", "type": "main", "index": 0}]]},
        "Собрать документы": {"main": [[{"node": "Вопросы к продукту", "type": "main", "index": 0}]]},
        "Вопросы к продукту": {"main": [[
            {"node": "Извлечение фактов продукта", "type": "main", "index": 0},
            {"node": "Извлечение бренда и ограничений", "type": "main", "index": 0},
            {"node": "Извлечение продуктовых инсайтов", "type": "main", "index": 0},
        ]]},
        "Извлечение фактов продукта": {"main": [[{"node": "Синхронизировать извлечения", "type": "main", "index": 0}]]},
        "Извлечение бренда и ограничений": {"main": [[{"node": "Синхронизировать извлечения", "type": "main", "index": 1}]]},
        "Извлечение продуктовых инсайтов": {"main": [[{"node": "Синхронизировать извлечения", "type": "main", "index": 2}]]},
        "Синхронизировать извлечения": {"main": [[{"node": "Собрать извлечения", "type": "main", "index": 0}]]},
        "Собрать извлечения": {"main": [[{"node": "Критики", "type": "main", "index": 0}]]},
        "Критики": {"main": [[{"node": "Валидаторы", "type": "main", "index": 0}]]},
        "Валидаторы": {"main": [[{"node": "Паспорт продукта", "type": "main", "index": 0}]]},
    }
    for chain_name in ("Вопросы к продукту", "Извлечение фактов продукта", "Извлечение бренда и ограничений", "Извлечение продуктовых инсайтов", "Критики", "Валидаторы", "Паспорт продукта"):
        connections.setdefault("OpenAI-compatible model", {"ai_languageModel": [[]]})
        connections["OpenAI-compatible model"]["ai_languageModel"][0].append({"node": chain_name, "type": "ai_languageModel", "index": 0})

    workflow = {
        "id": node_id(),
        "name": "Tempo product passport",
        "nodes": nodes,
        "connections": connections,
        "pinData": {},
        "active": False,
        "settings": {"executionOrder": "v1"},
        "versionId": "tempo-product-passport-v1",
        "meta": {"templateCredsSetupCompleted": False},
        "tags": [],
    }
    OUTPUT_FILE.write_text(json.dumps(workflow, ensure_ascii=False, indent=2) + "\n")
    print(f"wrote {OUTPUT_FILE}")


if __name__ == "__main__":
    main()
