import json
import subprocess

from promptflow.core import tool


MODEL = "opencode/cat-jam-free"


@tool
def call_opencode(prompt: str) -> str:
    """Run a prompt using the same local OpenCode model as the LLM Lab graph."""
    try:
        result = subprocess.run(
            ["opencode", "run", "--standalone", "--model", MODEL, "--format", "json", prompt],
            check=False,
            capture_output=True,
            text=True,
            timeout=300,
        )
    except FileNotFoundError as error:
        raise RuntimeError("OpenCode CLI was not found on PATH") from error
    except subprocess.TimeoutExpired as error:
        raise RuntimeError("OpenCode request timed out after 300 seconds") from error

    if result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or f"OpenCode exited with status {result.returncode}")

    text_parts = []
    for line in result.stdout.splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        part = event.get("part", {})
        if event.get("type") == "text" and part.get("type") == "text" and part.get("text"):
            text_parts.append(part["text"])

    response = "".join(text_parts).strip()
    if not response:
        raise RuntimeError("OpenCode returned no text")
    return response
