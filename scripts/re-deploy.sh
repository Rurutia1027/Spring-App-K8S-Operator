#!/bin/sh
# Rebuild images, load into Kind, redeploy operator + demo.
# Prerequisite: ./scripts/kind-up.sh (cluster name: spring-notes)

set -e

ROOT=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
OPERATOR_DIR="$ROOT/operator"
CLUSTER="${KIND_CLUSTER:-spring-notes}"
IMG="${IMG:-spring-notes-operator:dev}"
APP_IMG="${APP_IMG:-notes-service:dev}"

export KIND_CLUSTER="$CLUSTER"

command -v kind >/dev/null 2>&1 || { echo "kind not found"; exit 1; }

if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  echo "Kind cluster '$CLUSTER' not found. Run: ./scripts/kind-up.sh"
  exit 1
fi

kubectl config use-context "kind-$CLUSTER"

cd "$OPERATOR_DIR"

echo "==> build and load images (cluster: $CLUSTER)"
make docker-build IMG="$IMG"
make docker-build-app APP_IMG="$APP_IMG"
make kind-load IMG="$IMG" APP_IMG="$APP_IMG" KIND_CLUSTER="$CLUSTER"

echo "==> deploy operator"
make install
make deploy IMG="$IMG"

echo "==> deploy demo"
make demo-up

echo "re-deploy ok (cluster=$CLUSTER, operator=$IMG, app=$APP_IMG)"
echo ""
echo "Smoke test (run in another terminal after SpringApp is Ready):"
echo "  kubectl get springapp notes-service -n demo"
echo "  kubectl port-forward -n demo svc/notes-service 8080:8080"
echo ""
echo "  curl -s http://localhost:8080/actuator/health"
echo "  curl -s http://localhost:8080/api/notes"
echo "  curl -s -X POST http://localhost:8080/api/notes \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"title\":\"hello\",\"content\":\"from operator\"}'"