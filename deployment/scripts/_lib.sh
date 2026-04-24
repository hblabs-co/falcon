#!/usr/bin/env bash
# Shared bootstrap for every script under deployment/scripts/. Source
# it at the top of each script (not exec, not invoke):
#
#   source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"
#
# Gives you:
#   SCRIPT_DIR     — dir containing THIS lib (deployment/scripts/)
#   DEPLOYMENT_DIR — deployment/ root
#   APPS_DIR       — deployment/apps/      (app deployments)
#   CONFIGS_DIR    — deployment/configs/   (configmap + secret)
#   INFRA_DIR      — deployment/infra/     (mongo, nats, qdrant, minio, ingress)
#   NAMESPACE      — k8s namespace, defaults to "falcon" (env override)
#
# Paths are absolute so the caller works from any cwd (repo root,
# /tmp, CI runner, cron). The `_` prefix signals "helper, don't run
# directly" — it has no side effects, just sets variables.
#
# Callers should set `set -euo pipefail` themselves — left out here
# so sourcing doesn't surprise older scripts with stricter flags.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOYMENT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
APPS_DIR="$DEPLOYMENT_DIR/apps"
CONFIGS_DIR="$DEPLOYMENT_DIR/configs"
INFRA_DIR="$DEPLOYMENT_DIR/infra"

NAMESPACE="${NAMESPACE:-falcon}"
