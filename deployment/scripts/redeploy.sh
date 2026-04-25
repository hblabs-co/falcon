#!/usr/bin/env bash
# Hard redeploy — deletes and recreates every app deployment.
#
# Causes downtime (pods are gone for ~10s during the replace). Use when:
#   - You changed an immutable field (container name, selector labels).
#   - You want a clean slate for debugging a stuck controller.
#
# For the normal iteration loop use `rollout.sh` instead (zero downtime).
#
# Usage (from anywhere):
#   deployment/scripts/redeploy.sh

set -euo pipefail

# Paths + NAMESPACE resolved by the shared helper.
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

kubectl apply -f "$CONFIGS_DIR/10-configmap.yaml"
kubectl apply -f "$CONFIGS_DIR/11-secret.yaml"

# falcon-config bootstrap Job — HARD GATE. We apply it, block on
# completion, and only then touch the Deployments below. If the
# bootstrap fails (timeout, backoffLimit hit, mongo unreachable),
# `set -e` aborts the script before any service is replaced — better
# to keep the cluster on the previous revision than half-roll a
# broken schema migration.
#
# Jobs are largely immutable (kubectl apply on an existing Job
# rejects most spec changes), so we delete first. --ignore-not-found
# is a safe no-op on a fresh cluster.
kubectl delete job/falcon-config -n "$NAMESPACE" --ignore-not-found
kubectl apply -f "$APPS_DIR/10-config.yaml"
echo "--- waiting for falcon-config bootstrap to finish ---"
kubectl wait --for=condition=complete --timeout=180s \
  -n "$NAMESPACE" job/falcon-config

kubectl replace --force -f "$APPS_DIR/20-api.yaml"
kubectl replace --force -f "$APPS_DIR/21-realtime.yaml"
kubectl replace --force -f "$APPS_DIR/22-admin.yaml"
kubectl replace --force -f "$APPS_DIR/30-signal.yaml"
kubectl replace --force -f "$APPS_DIR/31-normalizer.yaml"
kubectl replace --force -f "$APPS_DIR/32-match-engine.yaml"
kubectl replace --force -f "$APPS_DIR/33-dispatch.yaml"
kubectl replace --force -f "$APPS_DIR/34-storage.yaml"
kubectl replace --force -f "$APPS_DIR/60-landing.yaml"

# scout is commented out intentionally — replacing it while it's
# mid-scrape drops in-flight scrape state (not persisted anywhere).
# Uncomment for a clean reset when you've already verified the
# scraper's log tail is quiet.
# kubectl replace --force -f "$APPS_DIR/35-scout.yaml"
