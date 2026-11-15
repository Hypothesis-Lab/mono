# Local Yandex Direct mock

This is a stateful, loopback-only simulator of the subset used by the A/B
preparation chain. It is not a complete or authoritative copy of Yandex Direct.
No real requests are forwarded. Mock resources, counters and rules persist in
`state.json`; an append-only `requests.jsonl` records method, occurrence,
fault and whether the simulator applied an effect. Credentials and request
bodies are not logged. Keep the state directory private.

Start it in one terminal from `backend/`:

```sh
python tools/yandex_mock/server.py --state-dir /tmp/opencode/yandex-mock-run \
  --rules tools/yandex_mock/scenario.example.json
```

The API listens only on `127.0.0.1:18081`, at `/json/v5` and `/json/v501`.
`--port` changes the port. Set the following environment variable **on the
backend service before starting it**, not on the CLI:

```text
YANDEX_DIRECT_MOCK_ENDPOINT=http://127.0.0.1:18081/json/v5
```

The backend accepts only a numeric loopback HTTP address with a port and that
path. In mock mode the provider client sends a fixed dummy token; it does not
decrypt or send the user's Yandex token. The application's normal account,
hypothesis and connection prerequisites still apply. Verify that the backend
was started with the mock setting before issuing a launch: without it the
backend targets the real API. Do not share one database/Temporal task queue
between a real worker and a mock worker: either worker could process the job.
The backend accepts `TEMPORAL_NAMESPACE` for isolation; register a dedicated
namespace first and point every mock backend/worker at it. Unset retains the
SDK's normal `default` namespace. Use a separate application database as well.

`GET /admin/state` shows simulator state. To replace rules and reset occurrence
counters without deleting objects:

```sh
curl -sS -X POST http://127.0.0.1:18081/admin/rules \
  -H 'Content-Type: application/json' \
  --data '{"rules":[{"service":"ads","method":"add","occurrence":2,"action":"disconnect_after"}]}'
```

An occurrence counts **every received** matching service/method call, including
retries. If multiple rules match, the first is applied. Supported actions:

| Action | Extra fields | Effect |
| --- | --- | --- |
| `http_error` | `status` 400–599, optional `error_code`, `message` | Respond without applying operation |
| `yandex_error` | `error_code`, optional `message` | HTTP 200 with top-level provider error, no effect |
| `item_error` | `error_code`, optional `message` | HTTP 200 item error, no effect; add/delete/suspend only |
| `disconnect_before` | none | Disconnect before applying operation |
| `disconnect_after` | none | Apply operation, then disconnect without a response |
| `delay_before` | `delay_ms` | Delay before applying operation, even if client disconnects |
| `delay_after` | `delay_ms` | Apply operation, then delay the response |

For a new independent run use a new state directory. The mock supports the
current campaign, group, image, responsive-ad, keyword, demographic modifier,
retargeting-list and audience-target operations. It enforces parent dependencies,
image deduplication and selected delete constraints. It does not model every
validation, quota, visibility delay, billing rule or Yandex-side guarantee;
especially campaign deletion is conservative and requires an empty campaign.
Only `ResponsiveAd` is implemented for ads. Batch partial success and simulated
provider crashes are not implemented. Rules apply per request, not per item.
Timers live in the mock process: restarting it cancels delayed requests. A
timeout or a single empty `get` is not proof that a delayed action cannot
complete. Use the state and request journal to check actual outcomes.
