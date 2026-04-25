#!/usr/bin/env bash
# Tail logs from every Falcon deployment, multiplexed into one stream so
# you can watch the whole cluster behave without juggling terminals.
#
# Usage:
#   deployment/scripts/logs.sh                # follow all services
#   deployment/scripts/logs.sh api signal     # filter by name substring
#
# Features:
#   - Prefixes every line with the service name so grep'ing for a specific
#     event across services is one pipe away.
#   - Streams all pods in a deployment (`-l app=<name>` + `--max-log-requests`
#     so kubectl doesn't cap at the default 5 pods if a deploy scales up).
#   - Survives pod restarts: kubectl logs -f auto-reconnects to the new pod
#     because we select by label, not by pod name.
#
# Requires kubectl configured against the falcon namespace.

set -euo pipefail

# NAMESPACE (default: falcon) comes from the shared helper.
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

# Services managed via the usual {name: <service>} deployment label. Keep
# in sync with deployment/scripts/rollout.sh — if a new service lands in
# k8s, add it here too.
ALL_SERVICES=(
  falcon-api
  falcon-realtime
  falcon-admin
  falcon-signal
  falcon-normalizer
  falcon-match-engine
  falcon-dispatch
  falcon-storage
  falcon-landing
  falcon-scout
)

# Optional args: filter by substring. `deployment/scripts/logs.sh api signal`
# matches falcon-api + falcon-signal.
if [[ $# -gt 0 ]]; then
  SERVICES=()
  for needle in "$@"; do
    for svc in "${ALL_SERVICES[@]}"; do
      if [[ "$svc" == *"$needle"* ]]; then
        SERVICES+=("$svc")
      fi
    done
  done
  if [[ ${#SERVICES[@]} -eq 0 ]]; then
    echo "no service matched: $*" >&2
    echo "available: ${ALL_SERVICES[*]}" >&2
    exit 1
  fi
else
  SERVICES=("${ALL_SERVICES[@]}")
fi

echo "tailing ${#SERVICES[@]} service(s) in namespace $NAMESPACE: ${SERVICES[*]}"
echo "--- ctrl-c to stop ---"

# Start one kubectl logs per service, prefix each line with the service
# name, and fan in to stdout. `trap` kills every child on exit so ctrl-c
# doesn't leave orphaned kubectl processes.
pids=()
trap 'kill ${pids[@]} 2>/dev/null || true' EXIT INT TERM

for svc in "${SERVICES[@]}"; do
  (
    # --tail=50 seeds a bit of recent history so you see the last
    # events rather than waiting for fresh logs. --max-log-requests
    # bumps the per-pod cap so scaled deployments stream in full.
    # || true swallows transient failures (pod just restarted, etc.);
    # kubectl will happily reconnect on the next iteration of the loop.
    while true; do
      # --prefix=true prepends `[pod/<pod-name>/<container>]` to every
      # line so when a deployment has >1 replica (or a pod restarts
      # mid-tail) you can tell which one emitted a given event. Pod
      # names carry the replica hash, so "falcon-realtime-7b4c…-abcde"
      # is distinguishable from "falcon-realtime-7b4c…-xyz12".
      # --max-log-requests bumps the per-pod cap so scaled deployments
      # stream in full. `|| true` swallows transient failures (pod
      # just restarted); the outer while reconnects after a short sleep.
      kubectl logs -f \
        -n "$NAMESPACE" \
        -l "app=$svc" \
        --tail=50 \
        --max-log-requests=10 \
        --prefix=true 2>&1 || true
      sleep 2
    done
  ) &
  pids+=($!)
done

wait
