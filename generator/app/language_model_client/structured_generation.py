from __future__ import annotations

import json
from typing import Any

from pydantic import BaseModel, ValidationError

from app.pipelines.audiences.protocols import LanguageModelClient
from app.pipelines.audiences.stages.execution import ModelCallContext


def complete_structured[ModelT: BaseModel](
    model_client: LanguageModelClient,
    context: ModelCallContext,
    output_model: type[ModelT],
    prompt: str,
    correction_feedback: str | None,
    response_schema: dict[str, Any] | None = None,
) -> ModelT:
    correction = (
        ""
        if not correction_feedback
        else "\n\nReturn the whole object again and fix this validation error: " + correction_feedback[:1000]
    )
    schema = _strict_schema(response_schema or output_model.model_json_schema())
    response = model_client.complete(
        "Return one valid JSON object without Markdown or explanation.", prompt + correction, schema
    )
    context.raw_response = response
    try:
        return output_model.model_validate(_parse_json(response))
    except ValidationError as error:
        raise ValueError(_compact_error(error)) from error


def _strict_schema(schema: dict[str, Any]) -> dict[str, Any]:
    values: list[object] = [schema]
    while values:
        value = values.pop()
        if isinstance(value, dict):
            if "properties" in value:
                value["required"] = list(value["properties"])
                value["additionalProperties"] = False

            values.extend(value.values())
        elif isinstance(value, list):
            values.extend(value)

    return schema


def _parse_json(content: str) -> dict[str, Any]:
    cleaned = content.strip().removeprefix("```json").removeprefix("```").removesuffix("```").strip()
    try:
        value = json.loads(cleaned)
    except json.JSONDecodeError as error:
        raise ValueError(f"model output was not valid JSON: {error.msg}") from error

    if not isinstance(value, dict):
        raise ValueError("model output JSON must be an object")

    return value


def _compact_error(error: ValidationError) -> str:
    return "; ".join(
        f"{'.'.join(map(str, item['loc']))}: {item['msg']}"
        for item in error.errors(include_url=False, include_input=False)
    )[:1400]
