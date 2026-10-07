#!/bin/sh
set -eu

while [ ! -f /setup/complete ]; do
  sleep 2
done

exec /etc/temporal/entrypoint.sh
