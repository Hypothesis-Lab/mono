"""Create private runtime settings for the isolated A/B application database."""

import argparse
import base64
import os
from pathlib import Path
import secrets

import yaml


parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--proxy-config", type=Path, required=True)
args = parser.parse_args()
proxy = yaml.safe_load(args.proxy_config.read_text())
proxy_key = proxy.get("server", {}).get("proxy_api_key")
if not isinstance(proxy_key, str) or not proxy_key:
    raise SystemExit("Proxy configuration has no server.proxy_api_key")
if any(char in proxy_key for char in "\r\n'\\"):
    raise SystemExit("Proxy key contains unsupported environment-file characters")

directory = Path(os.environ.get("XDG_DATA_HOME", Path.home() / ".local/share")) / "hypothesislab-ab"
directory.mkdir(mode=0o700, parents=True, exist_ok=True)
runtime = directory / "runtime.conf"
existing = {}
if runtime.exists():
    existing = dict(line.split("=", 1) for line in runtime.read_text().splitlines() if "=" in line)
encryption_key = existing.get("YANDEX_DIRECT_TOKEN_ENCRYPTION_KEY")
if not encryption_key:
    encryption_key = base64.b64encode(secrets.token_bytes(32)).decode()

settings = (
    f"YANDEX_DIRECT_TOKEN_ENCRYPTION_KEY={encryption_key}\n"
    "CODEX_PROXY_BASE_URL=http://127.0.0.1:18080/v1\n"
    f"CODEX_PROXY_KEY='{proxy_key}'\n"
)
fd = os.open(runtime, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
with os.fdopen(fd, "w") as output:
    output.write(settings)
runtime.chmod(0o600)
print(f"Runtime settings saved to {runtime}; secret values not printed.")
