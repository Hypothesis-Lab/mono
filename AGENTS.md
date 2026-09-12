# Temporary rules override
I'm currently exploring by writing code.
- do not write new tests
- run non-integration tests
- you can still run linter and fix linter issues

# Local Compose

`compose.yaml` runs this workspace locally. The stack is currently running with Podman Compose.

After making changes, restart the affected service(s) and inspect their logs to confirm they started successfully.

Don't read .env, it contains secrets.

# Project map
docs_dump/ad_platforms/yandex.md