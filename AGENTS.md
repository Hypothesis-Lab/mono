# Temporary rules override
I'm currently exploring by writing code.
- do not write new tests
- run non-integration tests
- you can still run linter and fix linter issues

# Project map
docs_dump/ad_platforms/yandex.md

# Deployment (outdated — do not push to Pi)
- The Pi/SSH deployment instructions below are obsolete.
- For local deployment, commit/push only when explicitly requested, then restart
  the affected local per-user systemd service with `systemctl --user restart ...`.

# Local service architecture

`compose.yaml` runs only infrastructure locally with Podman Compose: Postgres,
Temporal, SeaweedFS, and Temporal UI. The backend, generator, and frontend run
as per-user systemd services defined in `systemd/`.

Install or refresh the application services with:

```bash
mkdir -p ~/.config/systemd/user
ln -sfn /home/clanker/projects/startup/code/systemd/startup-generator.service ~/.config/systemd/user/startup-generator.service
ln -sfn /home/clanker/projects/startup/code/systemd/startup-backend.service ~/.config/systemd/user/startup-backend.service
ln -sfn /home/clanker/projects/startup/code/systemd/startup-frontend.service ~/.config/systemd/user/startup-frontend.service
systemctl --user daemon-reload
systemctl --user enable --now startup-generator.service startup-backend.service startup-frontend.service
```

Start the infrastructure with `podman compose up -d`. Restart application code
with `systemctl --user restart startup-backend.service`,
`startup-generator.service`, or `startup-frontend.service`; do not rebuild or
recreate containers for application-only changes.

Podman Compose 1.3.0 still creates infrastructure in a shared pod. If the
infrastructure stack must be recreated, use `podman compose down` followed by
`podman compose up -d`. Do not use service-scoped `--force-recreate`; containers
in the shared pod can depend on services omitted from the command. Compose
volumes are preserved by `down` unless `--volumes` is explicitly supplied.

See `systemd/README.md` for ports, logs, and service setup details.

## Command execution

- Run long-running builds, tests, servers, and service commands in the background
  when possible so the user can continue the conversation.
