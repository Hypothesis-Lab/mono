#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
env_file="${ENV_FILE:-$repo_root/.env}"

if [[ ! -r "$env_file" ]]; then
    printf 'Cannot read environment file: %s\n' "$env_file" >&2
    exit 1
fi

token=""
while IFS= read -r line || [[ -n "$line" ]]; do
    case "$line" in
        YANDEX_DIRECT_TOKEN=*)
            token=${line#YANDEX_DIRECT_TOKEN=}
            break
            ;;
        "export YANDEX_DIRECT_TOKEN="*)
            token=${line#"export YANDEX_DIRECT_TOKEN="}
            break
            ;;
    esac
done < "$env_file"

token=${token%$'\r'}
case "$token" in
    \"*\") token=${token:1:-1} ;;
    \'*\') token=${token:1:-1} ;;
esac

if [[ -z "$token" ]]; then
    printf 'YANDEX_DIRECT_TOKEN is not set in %s\n' "$env_file" >&2
    exit 1
fi

for dictionary in Interests AudienceInterests; do
    printf '%s\n' "$dictionary"
    curl --fail --silent --show-error \
        --header "Authorization: Bearer $token" \
        --header 'Accept-Language: ru' \
        --header 'Content-Type: application/json; charset=utf-8' \
        --data "{\"method\":\"get\",\"params\":{\"DictionaryNames\":[\"$dictionary\"]}}" \
        'https://api.direct.yandex.com/json/v5/dictionaries'
    printf '\n'
done
