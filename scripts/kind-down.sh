#!/bin/sh
# Delete local Kind cluster.

set -e

CLUSTER=spring-notes

command -v kind >/dev/null 2>&1 || { echo "kind not found"; exit 1; }

if kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  kind delete cluster --name "$CLUSTER"
else
  echo "cluster '$CLUSTER' does not exist"
fi
