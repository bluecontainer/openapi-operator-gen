#!/bin/bash
# Rundeck ResourceModelSource script for Kubernetes workload discovery
# Invokes kubectl-rundeck-nodes to discover StatefulSets, Deployments, and Helm releases

set -euo pipefail

# Configuration from Rundeck plugin (passed via RD_CONFIG_* env vars)
K8S_TOKEN="${RD_CONFIG_K8S_TOKEN:-}"
K8S_URL="${RD_CONFIG_K8S_URL:-}"
NAMESPACE="${RD_CONFIG_NAMESPACE:-}"
LABEL_SELECTOR="${RD_CONFIG_LABEL_SELECTOR:-}"
EXECUTION_MODE="${RD_CONFIG_EXECUTION_MODE:-native}"
DOCKER_IMAGE="${RD_CONFIG_DOCKER_IMAGE:-bluecontainer/kubectl-rundeck-nodes:latest}"
DOCKER_NETWORK="${RD_CONFIG_DOCKER_NETWORK:-host}"
CLUSTER_NAME="${RD_CONFIG_CLUSTER_NAME:-}"
CLUSTER_TOKEN_SUFFIX="${RD_CONFIG_CLUSTER_TOKEN_SUFFIX:-}"
DEFAULT_TOKEN_SUFFIX="${RD_CONFIG_DEFAULT_TOKEN_SUFFIX:-rundeck/k8s-token}"

if [ -z "$K8S_TOKEN" ]; then
  echo "Error: K8S_TOKEN not provided" >&2
  exit 1
fi

if [ -z "$K8S_URL" ]; then
  echo "Error: K8S_URL not provided" >&2
  exit 1
fi

# Build flags
FLAGS=""
if [ -n "$NAMESPACE" ]; then
  FLAGS="$FLAGS -n $NAMESPACE"
else
  FLAGS="$FLAGS -A"
fi
[ -n "$LABEL_SELECTOR" ] && FLAGS="$FLAGS -l $LABEL_SELECTOR"
[ -n "$CLUSTER_NAME" ] && FLAGS="$FLAGS --cluster-name=$CLUSTER_NAME"
[ -n "$K8S_URL" ] && FLAGS="$FLAGS --cluster-url=$K8S_URL"
[ -n "$CLUSTER_TOKEN_SUFFIX" ] && FLAGS="$FLAGS --cluster-token-suffix=$CLUSTER_TOKEN_SUFFIX"
[ -n "$DEFAULT_TOKEN_SUFFIX" ] && FLAGS="$FLAGS --default-token-suffix=$DEFAULT_TOKEN_SUFFIX"

# Find the kubectl-rundeck-nodes binary
# Priority: 1) bundled in plugin, 2) system PATH
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -x "$SCRIPT_DIR/kubectl-rundeck-nodes" ]; then
  KUBECTL_RUNDECK_NODES="$SCRIPT_DIR/kubectl-rundeck-nodes"
elif command -v kubectl-rundeck-nodes &>/dev/null; then
  KUBECTL_RUNDECK_NODES="kubectl-rundeck-nodes"
else
  echo "Error: kubectl-rundeck-nodes not found (not bundled or in PATH)" >&2
  exit 1
fi

case "$EXECUTION_MODE" in
  native)
    # Native execution: kubectl-rundeck-nodes runs directly on Rundeck host
    "$KUBECTL_RUNDECK_NODES" --server="$K8S_URL" --token="$K8S_TOKEN" \
      --insecure-skip-tls-verify $FLAGS
    ;;

  docker)
    # Docker execution: kubectl-rundeck-nodes runs in Docker container
    docker run --rm --network "$DOCKER_NETWORK" "$DOCKER_IMAGE" \
      --server="$K8S_URL" --token="$K8S_TOKEN" \
      --insecure-skip-tls-verify $FLAGS
    ;;

  *)
    echo "Error: Unknown execution mode: $EXECUTION_MODE" >&2
    exit 1
    ;;
esac
