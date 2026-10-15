import json
from pathlib import Path

from promptflow.core import tool


@tool
def get_audiences() -> str:
    """Load the audience profiles used by the ranking step."""
    catalog_path = Path(__file__).with_name("audiences.json")
    return json.dumps(json.loads(catalog_path.read_text(encoding="utf-8")), ensure_ascii=False)
