"""Launch DevUI and discover this project's workflow."""

from agent_framework.devui import serve

from .config import PROJECT_ROOT


def main() -> None:
    """Start the local development UI."""
    # Keep DevUI bound to loopback; auth is unnecessary for local development.
    serve(entities_dir=str(PROJECT_ROOT / "agents"), port=8080, auto_open=True, auth_enabled=False)
