# Product questions flow

This is the Prompt Flow port of the graph in `../llm/`. It takes a product
description, answers three product questions in parallel, then ranks three
matching audience profiles from `audiences.json`.

The prompt nodes are editable in `prompts/`. LLM calls use the local OpenCode
CLI and the same `opencode/cat-jam-free` model as the original graph. Audience
selection is checked for exactly three unique IDs from the catalog.

## Run locally

From this directory, with `opencode` available on `PATH`:

```sh
uv run pf flow test --flow . --inputs data.jsonl
```

To edit or inspect the graph, open this folder in VS Code, open
`flow.dag.yaml`, then use the Prompt Flow extension's **Visual editor** action.
The browser server (`uv run pf flow serve --source .`) provides a local flow
test page; it is not the graph editor.
