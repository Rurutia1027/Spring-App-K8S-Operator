#!/bin/sh
# Kind smoke test: operator deploy + app image on cluster + SpringApp CR.

set -e

ROOT=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
OPERATOR_DIR="$ROOT/operator"
CLUSTER="${KIND_CLUSTER:-spring-app}"
IMG="${IMG:-spring-app-operator:ci}"
APP_IMG="${APP_IMG:-notes-service:ci}"

cd "$OPERATOR_DIR"

echo "==> build images"
make docker-build IMG="$IMG"
make docker-build-app APP_IMG="$APP_IMG"

echo "==> load images into kind"
kind load docker-image "$IMG" --name "$CLUSTER"
kind load docker-image "$APP_IMG" --name "$CLUSTER"

echo "==> install CRD and deploy operator"
make install
make deploy IMG="$IMG"
kubectl wait deployment/operator-controller-manager -n operator-system --for=condition=Available --timeout=180s

echo "==> apply demo namespace, postgres, SpringApp CR"
make demo-up
kubectl wait pod -l app=postgres -n demo --for=condition=Ready --timeout=180s

echo "==> deploy app smoke workload (validates image on kind)"
kubectl apply -f deploy/ci/app-smoke.yaml
kubectl wait deployment/notes-service-smoke -n demo --for=condition=Available --timeout=180s

echo "==> verify app health and CRUD"
kubectl run curl-smoke -n demo --rm -i --restart=Never --image=curlimages/curl:8.5.0 -- \
  sh -ec '
    set -e
    curl -sf http://notes-service-smoke:8080/actuator/health
    curl -sf -X POST http://notes-service-smoke:8080/api/notes \
      -H "Content-Type: application/json" \
      -d "{\"title\":\"ci\",\"content\":\"ok\"}"
    curl -sf http://notes-service-smoke:8080/api/notes | grep ci
  '

echo "==> verify SpringApp CR accepted"
kubectl get springapp notes-service -n demo

echo "kind smoke ok"
