"""Populate the temporary n8n audience visual mirror with Python prompt copies."""

from __future__ import annotations

import inspect
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
EVAL = ROOT / "eval"
WORKFLOW = ROOT / "n8n/workflows/tempo-audience-python-visual.json"
sys.path.insert(0, str(EVAL))

from tempo_eval import audience_workflow  # noqa: E402


def prompt_copy(title: str, function_name: str) -> str:
    function = getattr(audience_workflow, function_name)
    return (
        f"# {title}\n\n"
        "TEMPORARY VISUAL COPY — Python remains authoritative and executes this prompt.\n\n"
        f"Python source: eval/tempo_eval/audience_workflow.py::{function_name}\n\n"
        "The following is the duplicated Python prompt builder:\n\n"
        f"```python\n{inspect.getsource(function)}\n```"
    )


def main() -> None:
    workflow = json.loads(WORKFLOW.read_text(encoding="utf-8"))
    notes = {
        "Prompt — query proposal": prompt_copy("audience_query", "_query_prompt"),
        "Prompt — result review and refinement": prompt_copy(
            "audience_query_result_review", "_query_result_prompt"
        ),
        "Prompt map — interpretation and reviews": "\n\n".join(
            [
                prompt_copy("audience_interpretation", "_interpretation_prompt"),
                prompt_copy("audience_critic", "_critic_prompt"),
                prompt_copy("audience_validator", "_validator_prompt"),
                prompt_copy("audience_privacy_review", "_privacy_prompt"),
            ]
        ),
    }
    for node in workflow["nodes"]:
        if node.get("name") in notes:
            node["parameters"]["content"] = notes[node["name"]]
            node["parameters"]["height"] = 520 if node["name"] != "Prompt map — interpretation and reviews" else 900
    WORKFLOW.write_text(json.dumps(workflow, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"Wrote {WORKFLOW}")


if __name__ == "__main__":
    main()
