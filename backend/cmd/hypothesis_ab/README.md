# hypothesis_ab

Small client for the hypothesis launch API. It does not call Yandex directly.
The bearer token authenticates to this backend; it is **not** a Yandex token.

Initialize the local backend and isolated `hypothesis_ab` database through the
[local setup](../../../systemd/README.md#local-ab-development-stack) before use;
this keeps older application data untouched. The database persists in the
existing local Postgres instance. A fresh database has no OAuth/Yandex connection.
Prerequisites are an existing user, hypothesis, product, audiences, and a
configured Yandex connection for that user. This CLI neither creates nor seeds
them. Run the following commands from `backend/`.

```sh
export HYPOTHESIS_AB_TOKEN='backend API token'
go run ./cmd/hypothesis_ab launch --hypothesis-id ID --mode a --wait
go run ./cmd/hypothesis_ab launch --hypothesis-id ID --mode b
go run ./cmd/hypothesis_ab status --hypothesis-id ID
```

Alternatively provide `--token-file PATH`. Avoid `--token`, since command-line
arguments may be visible to other processes. Launch mode `a` is synchronous;
`--request-timeout` controls its potentially long request. `--wait` polls with
`--poll-interval` up to `--timeout`. If a synchronous launch times out, its
outcome is unknown: check status and do not resubmit. `reconciliation_required`
is unresolved, not success. Mode `b` returns the queued acknowledgement; the
eventual full B-mode contract is not guaranteed yet. Existing automatic
reconciliation/outbox cycles may continue after a B launch; neither a failed
status nor connection cancellation should be interpreted as safe cleanup or
proof that background work stopped. Cancelling an A connection does not imply
cancellation of B's queued/background operation.

Mode A creates the same resource ledger but no Temporal outbox entry. Its
`workflow_id` identifies the application operation; it does not refer to an
actual Temporal execution. On an execution error, A attempts rollback inside the
request; it does not schedule background recovery if the request or process
ends. B runs its rollback activity through Temporal. Preparation creates
resources; it does not activate advertising.

Rollback-enabled versions now report `rolling_back` while automatic cleanup is
running; `--wait` continues polling this state. A cleanly compensated failure
returns `failed` (nonzero CLI exit), not preparation success. An uncertain effect,
unknown ownership, failed deletion or unresolved shared-asset reference leaves
`failed_with_residuals` in new executions after the automatic budget is exhausted.
The CLI exits nonzero and does not resubmit: a **new explicit command** creates a
separate attempt. Old launch/resource history remains, and numeric resource
names/IDs are scoped to the attempt. Images can be reused by hash and are not
owned by the new attempt in that case. Active creation and rollback still block
another preparation. Legacy `reconciliation_required` records remain blocking;
they are not silently reclassified or resumed by this change.

After A's request is canceled, it uses a bounded independent context only to save
the stopped outcome, not to create a background worker or perform detached
cleanup. Failure to save that outcome (for example an unavailable database) is
still an error, not a successful recovery. An already-sent external request can
finish later; its uncertain effect stays associated with the old attempt.

Phase checks prevent stale workers/outbox dispatch from starting new steps for a
stopped attempt. They do not revoke an already-sent provider request. These paths
need scenario-by-scenario validation and do not guarantee that arbitrary late
provider effects cannot interfere with shared assets. Old Temporal histories
keep their prior versioned behavior. Product/hypothesis deletion returns HTTP409
while unresolved preparation history exists; the database also protects that
history against cascaded deletion.

Flags: `--mode a|b` (launch only; default `b`), `--base-url` (default
`http://127.0.0.1:8000`), required `--hypothesis-id`, `--token-file`,
`--wait`, `--timeout` (default `10m`), `--poll-interval` (default `2s`), and
`--request-timeout` (default `10m`). `status` prints the workflow identifier
and state. `stop` invokes the existing stop endpoint.
