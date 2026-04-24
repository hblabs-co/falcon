#!/usr/bin/env bash
# Zero-downtime rolling update for every Falcon deployment.
#
# Usage (from anywhere — paths resolve relative to this script):
#   deployment/scripts/rollout.sh                # roll everything
#   deployment/scripts/rollout.sh api landing    # only these (substring match)
#
# Layout it expects (post-split):
#   deployment/
#   ├── apps/      20-api.yaml, 21-realtime.yaml, … (each Falcon service)
#   ├── configs/   10-configmap.yaml, 11-secret.yaml
#   ├── infra/     50-mongo.yaml, … (mongo, nats, qdrant, minio, ingress)
#   └── scripts/   this file + siblings
#
# Difference vs `redeploy.sh`:
#   - redeploy.sh uses `kubectl replace --force` → deletes + recreates
#     the deployment. Guaranteed downtime. Required when you change an
#     immutable field (container name, selector, etc).
#   - rollout.sh uses `kubectl apply` + rolling update → k8s spins up
#     the new pod, waits for the readiness probe, shifts traffic, and
#     only THEN terminates the old pod. Zero downtime when the
#     readiness probe is configured correctly.
#
# Requirements for actual zero-downtime:
#   1. readinessProbe declared on each HTTP deployment (api, landing,
#      realtime). Inspect with `kubectl describe deploy/<name>`.
#   2. A new image tag (or imagePullPolicy=Always). If the tag hasn't
#      changed, k8s won't notice the "update" and this script falls
#      back to `kubectl rollout restart` to force one.
#   3. Graceful SIGTERM handling in the Go binary — `system.Wait()` in
#      common/system/context.go already covers this via
#      signal.NotifyContext(…, SIGTERM).

set -euo pipefail

# Paths + NAMESPACE resolved by the shared helper.
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

# Service → manifest file pairs. Parallel arrays instead of an
# associative array because macOS ships bash 3.2 which doesn't support
# `declare -A` — parsing `[falcon-api]=…` there triggers arithmetic
# expansion and `set -u` fails with "falcon: unbound variable".
ORDER=(
  falcon-api
  falcon-realtime
  falcon-signal
  falcon-normalizer
  falcon-match-engine
  falcon-dispatch
  falcon-storage
  falcon-scout
  falcon-landing
)
MANIFEST_FOR=(
  "$APPS_DIR/20-api.yaml"
  "$APPS_DIR/21-realtime.yaml"
  "$APPS_DIR/30-signal.yaml"
  "$APPS_DIR/31-normalizer.yaml"
  "$APPS_DIR/32-match-engine.yaml"
  "$APPS_DIR/33-dispatch.yaml"
  "$APPS_DIR/34-storage.yaml"
  "$APPS_DIR/35-scout.yaml"
  "$APPS_DIR/60-landing.yaml"
)

# manifest_of echoes the manifest file for the given service name, or
# exits non-zero when the name isn't in ORDER. Keeps the mapping
# centralized without associative arrays.
manifest_of() {
  local name="$1"
  local i
  for i in "${!ORDER[@]}"; do
    if [[ "${ORDER[$i]}" == "$name" ]]; then
      echo "${MANIFEST_FOR[$i]}"
      return 0
    fi
  done
  return 1
}

# Optional filter args — substring match against deployment names.
if [[ $# -gt 0 ]]; then
  SERVICES=()
  for needle in "$@"; do
    for svc in "${ORDER[@]}"; do
      if [[ "$svc" == *"$needle"* ]]; then
        SERVICES+=("$svc")
      fi
    done
  done
  if [[ ${#SERVICES[@]} -eq 0 ]]; then
    echo "no service matched: $*" >&2
    echo "available: ${ORDER[*]}" >&2
    exit 1
  fi
else
  SERVICES=("${ORDER[@]}")
fi

echo "rolling ${#SERVICES[@]} service(s) in namespace $NAMESPACE: ${SERVICES[*]}"
echo "--- configmap + secret apply first (picked up on next pod restart) ---"

kubectl apply -n "$NAMESPACE" -f "$CONFIGS_DIR/10-configmap.yaml"
kubectl apply -n "$NAMESPACE" -f "$CONFIGS_DIR/11-secret.yaml"

for svc in "${SERVICES[@]}"; do
  manifest="$(manifest_of "$svc")"
  echo
  echo "▶ $svc ($manifest)"

  # apply reconciles the deployment spec with the cluster. If the image
  # tag changed (most rollouts), k8s auto-triggers a rolling update. If
  # nothing material changed in the yaml, the apply is a no-op — we
  # then use `rollout restart` to force fresh pods, picking up new
  # configmap/secret values.
  kubectl apply -n "$NAMESPACE" -f "$manifest"
  kubectl rollout restart -n "$NAMESPACE" "deployment/$svc"

  # rollout status blocks until every pod in the new revision is
  # ready. Default timeout is 10m; 3m is plenty for our services and
  # fails fast if the readiness probe never flips healthy (stuck
  # pod == instant feedback instead of silent broken deploy).
  kubectl rollout status -n "$NAMESPACE" "deployment/$svc" --timeout=3m
done

echo
echo "✔ rollout complete — ${#SERVICES[@]} service(s) on the new revision"
