#!/bin/sh
set -eu

address="${TEMPORAL_ADDRESS:-temporal:7233}"
namespace="${DEFAULT_NAMESPACE:-default}"

until temporal operator cluster health --address "$address"; do
  sleep 5
done

if ! temporal operator namespace describe --namespace "$namespace" --address "$address" >/dev/null 2>&1; then
  temporal operator namespace create --namespace "$namespace" --address "$address"
fi
