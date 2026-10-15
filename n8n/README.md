# Tempo n8n workspace

This directory is a local n8n workspace for rebuilding the Tempo product-passport workflow as a visible, editable automation graph. The local runtime is intended to use rootless Podman.

## What n8n provides

n8n is both:

- a browser-based workflow editor at `http://localhost:5678`;
- a workflow runtime that executes nodes and records executions;
- an AI workflow layer with chat triggers, AI agents, language-model nodes, tools, and structured output;
- persistent storage for workflows, credentials, and execution data.

The first scaffold uses the built-in SQLite storage in the n8n Docker volume. PostgreSQL can be added later if this becomes a shared or production service.

## Start the browser UI

From this directory, prepare the environment:

```sh
cp .env.example .env
openssl rand -hex 32
# Put the generated value into N8N_ENCRYPTION_KEY in .env
```

Start the rootless Podman API service in one terminal (skip this if it is already running):

```sh
export XDG_RUNTIME_DIR="/run/user/$(id -u)"
mkdir -p "$XDG_RUNTIME_DIR/podman"
podman system service --time=0 "unix://$XDG_RUNTIME_DIR/podman/podman.sock"
```

In another terminal, start n8n detached:

```sh
export XDG_RUNTIME_DIR="/run/user/$(id -u)"
export DOCKER_HOST="unix://$XDG_RUNTIME_DIR/podman/podman.sock"
podman compose up -d
```

Open [http://localhost:5678](http://localhost:5678), create the local owner account, and use the n8n editor in the browser.

Stop it with:

```sh
podman compose down
```

The named `n8n_data` volume is intentionally preserved by `down`, so workflows and credentials survive restarts. Do not use `podman compose down -v` unless you intend to delete the local n8n data.

## Try the AI starter workflow

1. In n8n, open the workflow menu and choose **Import from File**.
2. Select `workflows/tempo-ai-starter.json`.
3. Create an OpenAI credential in n8n and attach it to **OpenAI-compatible model**.
4. Execute the workflow from the editor and use the Chat Trigger test interface.

The model node is named “OpenAI-compatible model” because n8n's OpenAI credential can be pointed at an OpenAI-compatible endpoint. The current OpenCode CLI integration is not automatically available inside n8n: it can be connected later through an HTTP wrapper or a carefully configured self-hosted command node. For the first browser test, configure a supported OpenAI-compatible provider in n8n.

## Import the product-passport graph

The full graph is in `workflows/tempo-product-passport.json`. Import it after the starter workflow, or import it instead if you want the complete pipeline immediately.

It includes:

- a visible **Product documents** node that reads `../agent-framework/product/*.md` through the `/product` container mount;
- an editable **Analysis focus** node;
- the questions stage;
- three parallel extraction stages;
- an extractor merge, critic, validator, and final passport stage;
- a shared OpenAI-compatible language-model node that must be connected to each LLM chain in the UI.

The seven stage prompts are embedded as editable node text. They use n8n expressions to inject the focus, loaded documents, and prior stage results. The workflow intentionally uses Basic LLM Chain nodes rather than autonomous agents: these stages are deterministic prompt calls, matching the current React implementation.

## Planned Tempo graph

The starter workflow is deliberately small so the UI can be verified before importing the full passport pipeline. The target graph is:

```text
Documents + focus
      ↓
Questions
      ↓
Facts ───────┐
Brand ───────┼→ Critics → Validators → Product passport
Insights ────┘
```

The full version should use:

- file or database nodes for the nine shared product documents;
- editable prompt records in a database or n8n Data Store;
- parallel extraction branches;
- AI Agent or LLM Chain nodes with structured JSON output;
- explicit `UNKNOWN` handling and source validation;
- a final passport persisted as JSON and displayed through Chat Trigger or a companion React UI.

## Is n8n the right place?

Yes for orchestration, experiments, integrations, execution history, and a visual graph. It is less suitable as the only layer if we want the current custom product-passport screen with rich document browsing, per-node prompt editors, custom zoom/status UI, and a polished final report.

The likely long-term shape is:

```text
React UI (custom passport view)
          ↓ webhook/API
n8n (workflow orchestration and AI stages)
          ↓
PostgreSQL or shared storage
```

For an internal tool, n8n's editor and Chat Trigger may be enough. For a product-facing tool, keep the React UI and use n8n as the backend workflow engine.

## Research basis

This scaffold follows the official n8n guidance. The Compose image uses n8n's official GitHub Container Registry mirror because the Docker-compatible registry can be rate-limited in shared environments:

- [Docker Compose installation](https://docs.n8n.io/deploy/host-n8n/install-options/install-using-docker-compose)
- [Docker installation](https://docs.n8n.io/deploy/host-n8n/install-options/install-with-docker)
- [LangChain and AI nodes](https://docs.n8n.io/build/integrate-ai/langchain-in-n8n)
- [Chat Trigger](https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-langchain.chattrigger)
- [Structured Output Parser](https://docs.n8n.io/integrations/builtin/cluster-nodes/sub-nodes/n8n-nodes-langchain.outputparserstructured)
