# LangGraph pipeline prototype

This prototype distills the workflow into audience and pipeline operations without using the refactored `PipelineGraph` or `PipelineRunner`:

1. For each audience, check eligibility and run the pre-product interview.
2. Per audience, run the post-product interview, extract insights, and score product fit.
3. Rank all audiences and select the top three.
4. For each selected audience, draft three sequential hypotheses, then check diversity and assemble the result.

Eligibility and pre-interviews can run independently. Post-interview follows its audience's pre-interview; insights and scoring follow that audience's interview chain. Ranking waits for all audience eligibility and scoring operations. Only selected audiences produce hypotheses. Every operation is a named LangGraph node, so completed node outputs are checkpointed individually and node failures use LangGraph's retry policy. The graph state is separate from job scheduling and does not change the production worker.

## Run locally

From `generator/`, install the optional prototype dependencies:

```sh
uv sync --extra prototype
```

Use SQLite for durable local checkpoints and a unique `thread_id` per generation job:

```python
from app.config import Settings
from app.llm_client import LLMClient
from app.workflow import Workflow
from prototypes.langgraph.pipeline import build_graph, sqlite_checkpointer

settings = Settings.from_environment()
llm = LLMClient(
    base_url=settings.openai_base_url,
    api_key=settings.openai_api_key,
    model=settings.openai_model,
    reasoning_effort=settings.openai_reasoning_effort,
)

with sqlite_checkpointer("generator-prototype.sqlite") as checkpointer:
    checkpointer.setup()
    graph = build_graph(Workflow(llm), checkpointer=checkpointer)
    state = graph.invoke(
        {"input": bundle.model_dump(mode="json"), "progress": {"nodes": {}}},
        {"configurable": {"thread_id": job_id}},
    )
```

SQLite is for local evaluation. Before using this in the service, the checkpoint store would need to move to a production backend and be coordinated with the existing job lifecycle.
