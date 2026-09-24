# Generator End-State Plan

## Goal

Replace the custom ready-node scheduler with a LangGraph workflow and update the FastAPI API and local worker together. LangGraph owns pipeline state, completed node outputs, retries, and resume position. The application owns job queueing, per-job PostgreSQL execution locks, and final-result delivery. This is a direct local rewrite: no dual runner or compatibility period for existing job progress.

Use LangGraph for orchestration and LangChain's `ChatOpenAI` integration for model calls. Implement the audience pipeline as a new `app/pipelines/audiences/` package. Remove `app/workflow.py` and the custom OpenAI-compatible `LLMClient`; do not retain a `Workflow` class or an adapter around `Workflow.execute()`.

`ChatOpenAI` receives the configured model, API key, base URL, timeout, and reasoning effort. Use native JSON-schema structured output for each operation, retaining the current dynamically constrained schemas. Set `max_retries=2` (three total provider requests) and do not add a LangGraph retry policy to model-call nodes: the OpenAI SDK handles transient provider errors and supported `Retry-After` headers.

## Pipeline

The audience pipeline has the following stages and dependencies:

```text
For every audience:
  eligibility --------------------------+
  pre-interview -> post-interview -> insights -> score --+
                                                       |
All eligibility and score nodes ------------------------> ranking
                                                           |
For each selected audience only:                         |
  hypothesis 1 -> hypothesis 2 -> hypothesis 3 ---------+
                                                           |
All selected audiences' final hypotheses -> diversity -> result -> END
```

- Independent audience nodes run in parallel when ready.
- A post-interview waits for its own pre-interview, then insights and scoring wait for their own predecessor.
- Ranking waits for every audience's eligibility and score.
- Ranking determines which audiences generate hypotheses.
- Hypotheses are sequential within an audience so each can avoid duplicating the previous hypothesis.
- Diversity waits for every selected audience's final hypothesis.

## LangGraph State

Each generation uses `thread_id = str(job.id)`.

The graph state (checkpointed when a checkpointer is supplied) contains:

- Immutable input bundle.
- Completed operation outputs, keyed by operation ID such as `insights:audience-a` or `hypothesis:audience-a:1`.
- Model repair context for invalid outputs: operation ID, correction attempt, raw response, and compact validation feedback.
- Final result after the result node completes.

The job table is the sole source for the API's `queued`, `running`, `failed`, and `completed` status. A node exception leaves the failed operation pending in LangGraph's checkpoint; it does not checkpoint a second graph-level `failed` status. The worker logs the failing operation and error and updates the job status while holding the job lock.

The checkpointer writes a checkpoint after successful graph steps. Completed nodes are never rerun on normal resume. If a process stops during a node, that node can run again because it did not reach a checkpoint. Current nodes only call the LLM and produce data, so rerunning them has no external business side effect.

## Persistence

Use LangGraph's Postgres checkpointer against the existing PostgreSQL service. This is required for durable recovery and safe concurrent workers; it is not a production-only concern.

- Install `langgraph-checkpoint-postgres` and `langchain-openai` as normal generator dependencies.
- Keep the existing startup migration path: acquire its global PostgreSQL migration advisory lock, run Alembic for our own tables, then call `PostgresSaver.setup()` before starting the dispatcher or accepting jobs. Use a separate autocommit psycopg connection with `dict_row` for the checkpointer setup, while retaining the migration lock until it finishes. Do not wrap its internal migrations in Alembic's transaction.
- `PostgresSaver.setup()` tracks its own schema versions in `checkpoint_migrations`; it can be called at each startup under the migration lock, but only applies missing changes. This creates checkpoint tables on first use and picks up future checkpointer-library schema migrations without a new Alembic revision solely to call `setup()`. Do not call it for individual jobs or workers. Use the same database schema/search path for setup and normal checkpoint connections.
- Use a dedicated psycopg connection and `PostgresSaver` instance for each graph invocation. Do not use a shared database connection pool or silently reconnect the saver connection during that invocation: the same connection must hold its advisory lock for its entire lifetime.
- Do not use `InMemorySaver` or SQLite for the integrated service worker; its resumable, concurrently executed jobs use PostgreSQL.

## Standalone Evaluation And Observability

Build the audience graph from injected dependencies: a model instance, an optional checkpointer, and an observation sink. The production worker supplies `ChatOpenAI`, its dedicated locked `PostgresSaver`, and a Prometheus sink. Give the model factory its own model configuration (model, base URL, API key, timeout, reasoning effort, retries), independent of the service's required database settings, so the CLI can use it without `DATABASE_URL`. The graph definition and business operations do not open a job row, acquire a job lock, or read process-wide settings. The worker owns those concerns; a standalone caller can invoke the same graph without them.

Add `app/pipelines/audiences/evaluate.py` as a CLI: read and validate one specified input file, configure the chosen model and endpoint, create a fresh graph/run, and write its final result and a structured measurement report. It does not create a `generator.job`, start a dispatcher, or run the job-table migrations. Generate a fresh thread ID for each checkpointed evaluation by default; allow an explicit thread ID plus resume mode for an interrupted SQLite/Postgres run, and never run the same evaluation thread concurrently. Support these explicitly selected checkpointer modes:

| Mode | Evaluation behavior |
| --- | --- |
| None (`compile()` without a checkpointer) | Run once with no saved checkpoints; a failed process requires a fresh run. |
| `InMemorySaver` | Keep checkpoints for this CLI process only; useful for inspecting state without writing to disk. |
| `SqliteSaver` | Optional local checkpoint file for single-process evaluation and resume; install its package as an evaluation dependency. Do not share it with the threaded service worker. |
| `PostgresSaver` | Use an explicitly supplied evaluation database/DSN or schema and call its `setup()` there before the run. Never use the service's job table or its production checkpoint namespace implicitly. |

Use the same audience graph, input contract, prompts, validation, and output contract in both callers. Make the model injectable so evaluation can select a different `ChatOpenAI` model/base URL without modifying graph code. A different model provider also needs to satisfy the pipeline's native JSON-schema/reasoning requirements; swap its adapter deliberately rather than assuming every chat model supports the same options.

Add *new, consumed* measurement hooks, independent of the deleted `app/pipeline/telemetry.py`:

- Time each model invocation in the shared LLM request helper and record the operation ID, model, success/error, elapsed time, and `AIMessage.usage_metadata` input/output tokens when the provider returns them. Keep access to the raw `AIMessage` before parsing or structured-output transformation discards its usage. An SDK retry is included in the invocation's elapsed time; SDK-internal attempts and their token usage are not individually reported by this hook.
- Time each graph node (including validation and non-model work) and the overall run outside the LLM helper. Keep evaluation's per-operation/per-call records in a run-local, thread-safe collector so parallel nodes do not mix measurements; output a JSON report and totals alongside the result. Report missing token usage as unknown rather than zero.
- In the service, use the same observation points with a Prometheus metrics sink: request/call counts, token counters when available, and duration histograms for model calls, graph nodes, and whole jobs, plus failure counts. Expose `/metrics` for Prometheus to scrape; use bounded labels such as pipeline kind, stage, model, and outcome, not job/product/audience IDs. The CLI sink keeps detailed operation IDs; production metrics stay aggregate.

## Graph Updates And Deployments

LangGraph checkpoints state and pending node names, **not** the Python implementation that created them. Resuming a job uses the graph code installed in the instance that claims it. `PostgresSaver.setup()` only migrates the checkpointer's database tables; it does not migrate our graph topology, state semantics, or prompts.

- For ordinary compatible updates, deploy one current audience graph. Keep pending node names resolvable, accept old checkpoint fields, and make new state fields optional or supply defaults. Completed operations remain checkpointed; pending operations run with the new code. Check existing unfinished threads with `graph.get_state()` when changing node names or state shape. Business behavior can change on resume even when the checkpoint is technically compatible, so assess changes to prompts, selection rules, and output schemas as well.
- Do not add permanent v1/v2 graph implementations or a job `graph_version` column to the initial rewrite. If a future change cannot safely resume old checkpoints, stop admitting new jobs and let queued/running jobs finish under the old implementation. For failed jobs with old checkpoints, either retry them under the old implementation or explicitly retire/recreate them on the new one; do not quietly resume them with incompatible code. Then deploy the incompatible graph. This avoids silently mixing incompatible stages within a job.
- If a future release **must** run old and new graph versions simultaneously, introduce explicit version routing at that point: stamp the job when it is created, keep both implementations available until old jobs are resolved, and have dispatchers claim only versions their instance can execute. A version value on the job without corresponding code and claim filtering is insufficient. The advisory lock prevents simultaneous execution of one job, not execution by the wrong graph version during a rolling deployment.

## Job Table And Worker

Keep `generator.job`, but reduce its role to worker coordination and API delivery:

- Job ID, product ID, input, creation time, final result, and completion time remain; add a required `kind` identifying the pipeline (`audiences` for all jobs in this rewrite) and a small `retry_generation` counter for explicit manual retries. Require `kind` in `POST /jobs` and validate supported values; it is not a graph-code version.
- Remove `claim_token` and `claim_until`; the advisory lock is the sole ownership mechanism for a graph invocation and its job-table update.
- Remove the job table's per-node `progress` column in the rewrite; store new runs' node state only in LangGraph checkpoints. There is no migration of old local job progress.
- The job table and API share one status set: `queued`, `running`, `failed`, and `completed`.
- `running` is set when a worker begins a graph invocation. After a crash, it may remain `running` briefly, but scanners consider both `queued` and `running` jobs. An unlocked `running` job can be claimed and resumed; no lease timeout or reaper is needed.
- An exhausted provider or corrective failure becomes `failed` until manually retried; there is no additional automatic full-job retry loop.
- Include `kind` in job API responses and completed-result delivery so callers can interpret a result with the correct pipeline contract. Scope the existing one-unpersisted-job-per-product rule to `(product_id, kind)`, so different pipeline kinds can process the same product independently. Index claim candidates by kind, status, and creation order.

The worker flow is:

Each generator instance initially has one **audiences** dispatcher and a pool of at most `GENERATOR_WORKER_CONCURRENCY` graph-running workers (default: 2 in the rewrite). It scans only jobs with `kind = 'audiences'`. The dispatcher scans immediately on startup. When it has a free slot but finds no claimable job, it waits five seconds **after that scan finishes** before scanning again. When all slots are busy, it does not poll; a completed worker wakes the dispatcher immediately to fill the freed slot. The backend's existing `POST /jobs` only creates a queued job; it does not need to ping workers. Keep polling as the sole wake-up mechanism initially, avoiding a separate notification channel.

When another pipeline is actually introduced, add its own kind-filtered dispatcher and separately configured worker pool in the same service. Each runs its own graph without sharing a job slot or scan loop with audiences. Reuse the small dispatcher implementation if worthwhile, but do not create a speculative pipeline registry or second coordinator now. Separate pool limits are per kind **per service instance**; total model traffic across kinds/instances may need a separate shared limit later.

Implement this with one dispatcher thread and the standard-library `ThreadPoolExecutor(max_workers=GENERATOR_WORKER_CONCURRENCY)`. The dispatcher alone scans and submits work; it tracks active `Future`s, so a free slot is `max_workers - active_jobs`. Use a `threading.Condition` to wait for worker completion or shutdown: wait without a polling timeout when all slots are occupied, or for five seconds after an unsuccessful scan when capacity remains. A completion notification wakes the dispatcher to collect the finished future and scan immediately. Workers run the synchronous graph invocation and model/database calls, each using its own dedicated locked psycopg connection. Do not convert the worker to asyncio or add a queue/scheduler library for this small concurrency limit.

1. The dispatcher scans unfinished audience jobs (`kind = 'audiences'` and status `queued` or `running`). It opens a dedicated Postgres connection and calls `pg_try_advisory_lock(job_key)` for each candidate. If another instance holds the lock, skip that job and try another; do not change its row.
2. While holding the lock, re-read the row. If it is already `completed` or `failed`, release the lock and move on; otherwise set it to `running` and hand the locked connection to a free worker. That worker constructs the job's `PostgresSaver` on the same connection; the dispatcher must not use it again.
3. Invoke the graph with the job input for a new thread, or invoke the existing `thread_id` with no input to resume it. Keep the advisory lock and saver connection open until the invocation has stopped.
4. Save the final result and set the job to `completed` on the same locked connection, then release the advisory lock by closing that connection.
5. On an exhausted node failure, let the exception stop the graph with the failed operation still pending. Record the failure and set the job to `failed` on the locked connection as the worker's last database write, then close it. The manual retry endpoint can subsequently set the job back to `queued`.

If a worker crashes, its PostgreSQL session closes and the advisory lock is released. Another worker sees the unfinished `running` job, obtains the lock, and resumes its LangGraph checkpoint. If the database connection is lost during a model call, the worker must not reconnect the saver or write the final job result; after the call returns, the saver write fails on that connection and another worker can repeat the uncheckpointed node. If the database is unreachable, no worker can acquire the lock or checkpoints until it recovers. Configure model request timeouts so a live but stuck worker does not hold the lock indefinitely.

PostgreSQL's session-level advisory lock protects checkpoint writes **and** worker status/result updates for the same job across worker instances, because they all use the locked connection. The lock remains held for as long as the session is alive, regardless of how long the graph runs. Derive a stable advisory-lock key from the job UUID; a hash collision would merely serialize unrelated jobs. The manual retry endpoint does not execute the graph or write checkpoints: it uses a single conditional row update (`WHERE status = 'failed'`) to change the job to `queued` and increment `retry_generation` atomically, without acquiring the advisory lock. Concurrent retry requests cannot both succeed. If a dispatcher sees `queued` while the failed worker is still closing its connection, the dispatcher skips the held lock and tries again later.

Scan candidates with `queued` jobs ahead of `running` jobs and advance past locked candidates rather than repeatedly checking only the first few. The short poll is inexpensive but is not a time bound on crash recovery: PostgreSQL must first recognize that the crashed worker's session ended and release its advisory lock.

| Event | What happens |
| --- | --- |
| New job created | If there is capacity and PostgreSQL is responsive, the dispatcher finds the queued job on its next scan: no more than five seconds of polling wait, plus scan time. If all slots are occupied, it scans when a worker finishes. |
| Job completed | Mark `completed` under the lock; future scans ignore it. Existing completed-result delivery remains unchanged. |
| Model/validation attempts exhausted | The graph stops with the failing operation pending. Mark the job `failed` under the lock and close the connection. Scans exclude it. |
| Manual retry of a failed job | Atomically increment `retry_generation` and change `failed` to `queued` only if the row is still `failed`. The next dispatcher scan claims it and resumes the pending operation from the checkpoint without repeating successful operations. |
| Worker process crashes | The row may still say `running`, but its lock is eventually released. A dispatcher with free capacity acquires it and assigns it to a worker to resume from the checkpoint. |
| Saver connection is lost mid-call | The old call cannot save a checkpoint or result. After the lock is released, another worker can resume; the incomplete node may run again. |
| Model call hangs while connection stays live | The lock remains held; polling cannot recover it. Model request timeouts bound the call. |
| Database is unavailable | Scans fail and retry after the poll delay; work resumes when the database returns. |

## Retry Handling

There are three distinct recovery paths:

- `ChatOpenAI(max_retries=2)` lets the OpenAI SDK retry connection failures, timeouts, 408/409/429 responses, and provider 5xx responses within one model call; the SDK handles supported `Retry-After` headers. No additional LangGraph retry policy on model-call nodes.
- A corrective graph loop handles model output that is syntactically, structurally, or semantically invalid.
- The checkpointer preserves completed work and correction context across worker crashes; exhausted failures are recorded as failed until manual retry.

`retry_generation` is a persistent marker for an **explicit manual retry**, not a model-call attempt counter. It starts at 0 on the job row and increments only when the retry endpoint atomically changes `failed` to `queued`. Each model operation stores the last retry generation it consumed in graph state. On resume, if the job's value is higher, the pending failed operation first checkpoints a state-only step that records the new generation and resets **only its** three-generation correction budget, retaining the last invalid response and feedback. Only then may it make another model call. Both a pending generation node (after provider failure) and a pending validation-exhaustion guard must handle this case, so a provider failure cannot leave an unconsumed retry marker that grants an unintended reset later. A crash before this checkpoint can repeat the reset step; a crash after it cannot reset the budget again. Ordinary recovery of an unfinished `running` job does not increment the marker or reset the budget. Provider transport retries have no checkpointed attempt counter; each fresh model call gets its configured SDK retry budget.

Do not retry every exception identically.

- Keep the SDK's transient retry budget at three total requests per model call. Do not stack LangGraph `RetryPolicy` on top of it, since that multiplies requests and can ignore server-directed backoff.
- Do not configure a graph-wide `RetryPolicy` or add one to the current nodes: model generation already has SDK transport retries; validation needs a checkpointed feedback loop rather than an identical node re-execution; ranking, diversity, and other pure calculations will not improve by retrying; job claiming and checkpointer writes are outside node retry and a lost lock connection must abort the invocation. LangGraph's default retry predicate is broader than the OpenAI SDK's HTTP-status classification, so adding a policy to model nodes would also need explicit error filtering. Reconsider a **node-specific** `RetryPolicy` only if a future node performs distinct transient external I/O without another retry owner.
- Do not send schema, Pydantic, or business-validation errors through transport retry.
- Split each model operation into generation and validation steps. The generation step produces raw model output; the validation step parses JSON and applies Pydantic plus business validation.
- On validation failure, the validation step returns a LangGraph `Command` that checkpoints the raw response, compact validation feedback, and correction attempt, then routes back to that operation's generation step.
- The next generation prompt includes the original instructions/context, prior raw response, and validation feedback, asking for a complete corrected object. This is the pipeline equivalent of an agent seeing a tool error, but it uses a normal corrective prompt rather than a `ToolMessage` because this pipeline does not make tool calls.
- Limit corrections to three generations per operation in total (original plus two corrections), counted in checkpointed graph state so an app restart does not reset the budget.
- When either budget is exhausted, let the failing node stop the graph instead of routing the run to `END`. The manual retry endpoint conditionally changes a `failed` job to `queued` and increments its `retry_generation` in one row update; it does not acquire the advisory lock or edit LangGraph checkpoints. Resume with `invoke(None, {"configurable": {"thread_id": job_id, "retry_generation": job.retry_generation}})`. Successful dependencies and outputs remain checkpointed and are not regenerated.
- A provider failure supplies no new validation feedback; manual retry reruns the pending generation request, including any correction context already checkpointed from earlier invalid outputs. Invalid input or configuration must be fixed rather than repeatedly requeued against the same unchanged input.
- The pending failed operation reads the retry generation from config. On a new value, the validation-exhaustion guard or the generation node (if it previously failed at the provider) returns a state-only `Command` update to record the new generation and reset that operation's correction budget, then routes to generation with previous raw response and validation feedback intact. The generation node must route back to itself for the model call on the next checkpointed step, not call the model in the same step as the reset. With no new retry generation, a provider-failed generation node simply reruns with its saved context. Do not call `graph.update_state()` on the failed checkpoint to reset counters: that can reschedule successful parallel nodes and repeat LLM calls.

LangGraph checkpoints successful work and pending tasks, but an exception alone does not set the job's application status. After provider retries are exhausted, let the error propagate to the graph worker. The worker records the failing operation and error for diagnostics and marks the job failed while holding its lock. Do not use a graph error handler that routes the run to `END` for a recoverable failure, or automatically requeue an exhausted model call; either would complicate manual resume or create an unbounded retry layer.

## Error Classification

Use typed exceptions from `ChatOpenAI` and the OpenAI SDK. Do not recreate the current `LLMClient` behavior that reduces all provider HTTP failures to `RuntimeError`, because failure handling needs the original error category.

| Error category | Examples | Handling |
| --- | --- | --- |
| Invalid input or configuration | Invalid input bundle, missing product facts, invalid environment configuration | Reject before graph invocation; do not retry. |
| Transient provider failure | Connection failure, timeout, 408/409/429, provider 5xx | OpenAI SDK retries through `ChatOpenAI(max_retries=2)`; failed after exhaustion until manual retry. |
| Node timeout | Graph node takes too long | Set model request timeout; if a separate graph node timeout is needed, handle it explicitly without stacking retries. |
| Provider request/configuration failure | Invalid API key, invalid model, bad request, unsupported provider feature | Fail immediately; retrying cannot correct it. |
| Invalid model structure | Non-JSON output, missing field, invalid field type | Corrective graph loop with raw response and validation feedback. |
| Invalid model business output | Invalid interview question IDs, non-literal evidence, unknown facts, forbidden claims, duplicate hypothesis | Corrective graph loop with the violated rule in validation feedback. |
| Graph/state invariant | Missing dependency output, stale score, unknown stage, inconsistent result | Terminal failure; treat as a code or checkpoint problem. |
| Job/database connection failure | Database connection failure, lost advisory-lock session | Stop that graph invocation; do not reconnect its saver. Another worker can resume when the database and lock are available. |
| Unexpected code failure | Programming error or incompatible persisted state | Terminal failure with logging. |

## Concurrency

There are two independent limits:

- `GENERATOR_WORKER_CONCURRENCY=2`: simultaneous generation jobs claimed by the worker pool, configurable from the environment per generator instance.
- `GENERATOR_GRAPH_MAX_CONCURRENCY=2`: pass as LangGraph `max_concurrency` for each graph invocation, limiting simultaneously ready operations inside one run.

Potential simultaneous LLM requests are approximately:

```text
worker concurrency x graph max concurrency
```

Start with these configurable defaults: at most four simultaneous LLM calls across the generator (some graph nodes do not call the model, so the actual number can be lower). Increase only after observing provider rate limits, LLM latency, checkpoint contention, and memory use.

Require the per-job PostgreSQL advisory lock before graph execution so only one worker instance with a live saver connection can write a thread's checkpoints or its job-table result. If the old connection dies during an in-flight LLM call, a replacement may issue another LLM call concurrently, but the old call's result cannot be checkpointed or published through its dead connection.

## Audience Pipeline Module Shape

Target `app/` layout (add further packages only when there is a real need):

```text
app/
  main.py                         # API lifespan and dispatcher startup
  routes.py                       # HTTP endpoints
  config.py                       # environment settings
  migrations.py                   # serialized startup database setup
  db.py                           # connection helpers; no shared checkpointer pool
  api_models.py                   # job request/response/status contracts
  observability.py                # run-local collector and Prometheus metrics sink
  repositories/
    jobs.py                       # job queries and status transitions
  pipelines/
    audiences/
      models.py                   # audience input, domain, model-output, result contracts
      state.py                    # checkpoint state and parallel-update reducers
      graph.py                    # StateGraph nodes, edges, routing, and retry gates
      catalog.py                  # personas, questions, selection constants
      input.py                    # input readiness
      llm.py                      # ChatOpenAI setup and shared request helpers
      research.py                 # eligibility, interviews, insights, scoring, ranking
      hypotheses.py               # hypothesis generation and diversity
      result.py                   # final result assembly and verification
      evaluate.py                 # standalone single-input evaluation CLI
```

The new `app/pipelines/audiences/` package owns audience-generation behavior. These are responsibilities and example function names, not an instruction to transplant the old class method by method:

| Module | Responsibility and representative interfaces |
| --- | --- |
| `models.py` | Audience-specific Pydantic input and output contracts: product, experiment, campaign, persona, interview answers, insights, scores, hypotheses, and the final audience result. Define only fields and validators required by the desired pipeline and backend contract; do not preserve old progress/signature fields just because they exist today. |
| `state.py` | `AudiencePipelineState` and `RepairContext` types: audience input, completed outputs keyed by operation/audience, per-operation invalid-output feedback, correction count, consumed manual-retry generation, and final result. Define reducers for outputs from concurrently running audience nodes. This is the graph's checkpoint state, not an HTTP response model. |
| `graph.py` | `build_graph(model, checkpointer=None, observer=None)`: declare `StateGraph(AudiencePipelineState)` and register per-audience eligibility, interview, insights, and score nodes; wait for all audiences to rank; route only selected audiences through three sequential hypotheses; then diversity and result. Define reusable generation/validation node wiring, node timing, and the manual-retry gate; compile with the caller's optional checkpointer. The graph does not claim jobs or decide worker concurrency. |
| `llm.py` | `ModelConfig`, `create_model(model_config)`, and shared model-request helpers: configure `ChatOpenAI`, render prompts, apply the operation's strict JSON schema, retain the raw `AIMessage` for token usage, record model-call timing, and return the raw response for the separate validation node. The old call collector is not reused. |
| `catalog.py` | Audience-specific persona definitions, interview questions, and selection constants. |
| `input.py` | `validate_input(bundle)`: reject incomplete or inconsistent product/campaign input before graph execution. |
| `research.py` | Domain functions for eligibility, pre/post interview prompts and answer validation, insights prompts and evidence validation, scoring and score validation, and audience ranking. Functions accept explicit product/audience/prior-output data and return typed results or validation feedback. |
| `hypotheses.py` | Hypothesis prompts using the selected audience's research and earlier hypotheses; hypothesis validation and materialization; diversity checks across selected audiences. |
| `result.py` | Assemble and verify the final API result from completed audience data. |
| `evaluate.py` | CLI entry point for a specified input, model configuration, checkpointer choice, and local result/measurement report. Calls `build_graph` directly; no job API or dispatcher. |

Keep job persistence, advisory locking, dispatch, and API routes outside this package. Put job-status/request/response API models outside the audience package (for example `app/api_models.py`); API request types can refer to the audience input contract. Keep the graph's `TypedDict` checkpoint state separate from Pydantic models used for input, model-output validation, and the public result. Design functions and types for LangGraph state rather than for `Workflow`, `PipelineNode`, or `job.progress`.

## Code Changes

1. Implement the integrated audience graph in `app/pipelines/audiences/graph.py` with injectable model, checkpointer, and observation sink, and explicit nodes and edges, including distinct generation and validation steps for model-backed operations. Remove the experimental prototype once the integrated graph replaces it.
2. Add `app/pipelines/audiences/llm.py` for `ChatOpenAI` configuration and shared prompt/schema/response handling, using the existing OpenAI-compatible endpoint, native JSON-schema output, configured reasoning effort, and `max_retries=2`. Delete `app/llm_client.py`.
3. Implement the audience operations in `research.py`, `hypotheses.py`, and `result.py` with explicit inputs and outputs. Use the current domain constraints and result schema as behavior requirements; design prompts and validation for the new graph rather than porting the legacy class structure. Delete `app/workflow.py` and the old `Workflow` class.
4. Add a graph-backed job runner with per-job PostgreSQL advisory locking on each checkpointer connection.
5. Put job queries and status transitions in `app/repositories/jobs.py` and keep `app/db.py` limited to connection helpers. Update the repository and migrations for unified `queued`, `running`, `failed`, and `completed` status; remove lease/token columns and let workers resume unlocked unfinished jobs. Checkpointer/lock connections must be dedicated, not drawn from a shared pool. Avoid adding a pool for short-lived repository queries until it is useful.
6. Define audience-domain models in `app/pipelines/audiences/models.py` and move its static catalog there; define job API models separately. Delete the mixed `app/models.py` and `app/catalog.py` after updating imports. Update generator, backend, and frontend API contracts together to expose `kind` and the new statuses directly; route completed results by kind, and update retry endpoints to use job status plus LangGraph checkpoint state rather than `job.progress`.
7. Switch `main.py` to construct the graph worker.
8. Delete the entire legacy `app/pipeline/` (singular) package, including `telemetry.py`; do not transplant its unused call collector. Remove its imports from `JobRepository` and `main.py`; the old `Workflow` and prototype that imported it are also removed. Neither LangGraph nor the new worker uses its runner, blueprint, graph, storage, or telemetry modules. Drop the `progress` column as part of the same rewrite.
9. Add the standalone audience evaluation CLI and new observability sinks described above. Keep both callers on the same graph and model-invocation path; only storage and metric destinations differ.

## Verification

- Run `task format` then `task lint`.
- Exercise a successful job through the public API.
- Force one retryable LLM failure and confirm the node retries without rerunning completed dependencies.
- Stop the generator during a long node, restart it, and confirm the same job resumes from its last LangGraph checkpoint.
- Run at least two jobs concurrently and verify separate thread IDs, advisory locks, outputs, and checkpoints.
- Run two generator instances; verify only one acquires a given job's lock, the other skips it, and a killed lock holder's unfinished job resumes elsewhere without repeating completed nodes.
- Drop the saver/lock connection during an in-flight model call and confirm the old invocation cannot checkpoint or publish its result after a replacement acquires the lock.
- Run the same evaluation input with two model configurations without a job row or service database, and compare result, per-node durations, per-call durations, and provider-reported token usage. Verify an optional SQLite evaluation checkpoint can resume and a separately configured Postgres checkpointer uses only its specified database/schema.
- Check that production `/metrics` reports aggregate model/node/job timings and token usage when supplied by the provider, without high-cardinality job or audience labels.
