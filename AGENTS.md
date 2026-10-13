# Temporary rules override
I'm currently exploring by writing code.
- do not write new tests
- run non-integration tests
- you can still run linter and fix linter issues

# Project map
docs_dump/ad_platforms/yandex.md

# Deployment (only on user request)
- commit to main and push
- ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i ~/.ssh/phone_to_pi_clanker -p 22 clanker@192.168.0.13       ▐
- in ~/projects/startup/code/mono switch to main pull
- restart the docker compose (affected services) (compose.yaml)

# Local Compose tips

`compose.yaml` runs this workspace locally. The stack is currently running with Podman Compose.

Podman Compose 1.3.0 creates the stack in a shared pod and its `up` command may try to run containers that already exist, report name conflicts, and then start the old containers. Do not use `podman compose up -d --build <service>` to refresh a rebuilt image. Recreate the full stack instead:

```sh
podman compose down
podman compose up -d --build
```

Do not use service-scoped `--force-recreate`: containers in the shared pod can depend on services omitted from the command, causing removal and name conflicts to fail. The compose volumes are preserved by `down` unless `--volumes` is explicitly supplied.
