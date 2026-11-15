# Local services

Postgres, Temporal, SeaweedFS, and the Temporal UI run in `compose.yaml`. The
backend, generator, and frontend run as per-user systemd services so they can be
restarted without rebuilding or recreating the infrastructure containers. The
frontend serves the production build with Vite preview; run `npm run build`
after frontend changes before restarting it.

Install the tracked units:

```sh
mkdir -p ~/.config/systemd/user
ln -sfn /home/clanker/projects/startup/code/systemd/startup-generator.service ~/.config/systemd/user/startup-generator.service
ln -sfn /home/clanker/projects/startup/code/systemd/startup-backend.service ~/.config/systemd/user/startup-backend.service
ln -sfn /home/clanker/projects/startup/code/systemd/startup-frontend.service ~/.config/systemd/user/startup-frontend.service
systemctl --user daemon-reload
systemctl --user enable --now startup-generator.service startup-backend.service startup-frontend.service
```

Start the infrastructure separately with:

```sh
podman compose up -d
```

The native services listen on ports 5175 (frontend), 8000 (backend), and 8001
(generator). Temporal UI remains available on port 8080.

The units load credentials from the existing `.env` without copying them into
the unit files. Service logs are available with `journalctl --user`:

```sh
journalctl --user -u startup-backend.service -n 30 --no-pager
journalctl --user -u startup-generator.service -n 30 --no-pager
journalctl --user -u startup-frontend.service -n 30 --no-pager
```

## Local A/B development stack

When the existing local Postgres and SeaweedFS already occupy ports 5433 and
8333, reuse them and start only Temporal with Podman:

```sh
systemctl --user start podman.socket
podman compose -f compose.yaml -f compose.ab.yaml up -d temporal-admin-tools temporal temporal-create-namespace temporal-ui
```

The override connects Temporal to the host Postgres at port 5433. Temporal creates
its own `temporal` and `temporal_visibility` databases. Create a separate
`hypothesis_ab` application database to preserve the existing `app` database:

```sh
docker exec backend-postgres-1 createdb -U postgres hypothesis_ab
```

Run that database command only once. Install generator dependencies with
`uv sync --frozen --no-dev` in `generator/`, and run `npm ci` followed by
`npm run build` in `frontend/`. Configure private runtime settings using the
local Codex proxy's configuration (from the repository root):

```sh
generator/.venv/bin/python systemd/configure-local-ab.py --proxy-config "$HOME/projects/codex-proxy/data/local.yaml"
```

This stores the proxy key and a generated OAuth-token encryption key in
`~/.local/share/hypothesislab-ab/runtime.conf` with mode 0600. Re-running preserves
the encryption key. Keep this file for as long as the `hypothesis_ab` database
contains encrypted connections; it must not be used to decrypt another database.
The configured proxy URL is `http://127.0.0.1:18080/v1`; sign in through its
dashboard before requesting generation. This setup does not inspect `.env`.

Wait for Temporal and its `default` namespace to be ready, then run:

```sh
bash systemd/run-local-ab.sh
```

This starts transient user services `startup-ab-backend`, `startup-ab-generator`,
and `startup-ab-frontend` on ports 8000, 8001, and 5175. The script resolves paths
relative to the checkout. systemd loads the existing `.env` and private runtime
settings; the script does not print or source them. Local connection settings
override environment-file values.
The generator still requires a reachable configured Codex proxy for generation.
The development JWT signing value is local-only, not a production credential.
The backend write timeout is 30 minutes to permit synchronous A requests; each
CLI invocation has its own configurable timeout.

The application database is initially empty: an account, hypothesis, product,
audiences, and a Yandex connection must be prepared before a real advertising
run. A standalone Yandex API token is not an application login token.

Use `systemctl --user restart startup-ab-backend.service` after backend edits,
and `journalctl --user -u startup-ab-backend.service` for logs. Stop the three
application services with `systemctl --user stop startup-ab-backend.service
startup-ab-generator.service startup-ab-frontend.service`. These transient units
are not enabled at login; run the script again after they have been removed.

See [the A/B CLI](../backend/cmd/hypothesis_ab/README.md) for launch and status
commands. Starting the stack does not submit an advertising preparation command.

For an identical Tempo pair with three synthetic audiences and nine draft-ad
variants per arm, after backend migrations have completed, run from `backend/`:

```sh
DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5433/hypothesis_ab?sslmode=disable' \
  go run ./cmd/seed_tempo --clone-source SOURCE_HYPOTHESIS_ID --ab-pair \
  --pilot-audience --draft-landing-url https://example.org/tempo-pilot
```

This creates fresh copies without replacing source data. The reserved URL and
deterministic calendar PNGs are engineering fixtures, not a real landing page or
production marketing assets. The opt-in Unified draft plan has 43 logical
provider objects plus one non-serving campaign guard. Both variants use the same
snapshots. Native image deduplication may reuse assets across the two arms.
Campaigns remain non-serving drafts or suspended; ads must remain DRAFT/OFF.
No moderation or activation command is issued. See
[fixture details](../backend/cmd/seed_tempo/README.md) for limitations.
