#!/bin/sh
# Start local Kind cluster.

set -e

CLUSTER=spring-notes

command -v kind >/dev/null 2>&1 || { echo "kind not found"; exit 1; }

if kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  echo "cluster '$CLUSTER' already exists"
else
  kind create cluster --name "$CLUSTER"
fi

kubectl config use-context "kind-$CLUSTER"
kubectl cluster-info
