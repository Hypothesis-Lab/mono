# Temporary rules override
I'm currently exploring by writing code.
- do not write new tests
- run non-integration tests
- you can still run linter and fix linter issues

# Local Compose

`compose.yaml` runs this workspace locally. The stack is currently running with Podman Compose.

After making changes, restart the affected service(s) and inspect their logs to confirm they started successfully.

Podman Compose 1.3.0 creates the stack in a shared pod and its `up` command may try to run containers that already exist, report name conflicts, and then start the old containers. Do not use `podman compose up -d --build <service>` to refresh a rebuilt image. Recreate the full stack instead:

```sh
podman compose down
podman compose up -d --build
```

Do not use service-scoped `--force-recreate`: containers in the shared pod can depend on services omitted from the command, causing removal and name conflicts to fail. The compose volumes are preserved by `down` unless `--volumes` is explicitly supplied.

Don't read .env, it contains secrets.

# Project map
docs_dump/ad_platforms/yandex.md
