import json

from promptflow.core import tool


@tool
def validate_ranking(ranking: str, audience_catalog: str) -> str:
    """Validate that the model ranked three unique known profiles."""
    start = ranking.find("{")
    if start < 0:
        raise ValueError("The model did not return a JSON object")

    try:
        parsed, _ = json.JSONDecoder().raw_decode(ranking[start:])
        audiences = json.loads(audience_catalog)
    except json.JSONDecodeError as error:
        raise ValueError(f"Could not parse ranking JSON: {error}") from error

    ranked = parsed.get("top_audiences")
    if not isinstance(ranked, list) or len(ranked) != 3:
        raise ValueError("The model must return exactly three audiences")

    allowed = {audience["id"]: audience for audience in audiences}
    seen = set()
    result = []
    for item in ranked:
        audience_id = item.get("audience_id")
        if audience_id not in allowed or audience_id in seen:
            raise ValueError("The model returned an unknown or duplicate audience")
        seen.add(audience_id)
        result.append(
            {
                "audience_id": audience_id,
                "label": allowed[audience_id]["label"],
                "reason": (item.get("reason") or "Обоснование не указано.").strip(),
            }
        )

    return json.dumps({"top_audiences": result}, ensure_ascii=False, indent=2)
