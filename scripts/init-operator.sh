#!/bin/sh
# Create operator/ via kubebuilder (SpringApp CRD + controller skeleton).
# Prerequisite: ./scripts/kind-up.sh
#
# Layout:
#   app/       — Spring Boot microservice (already in repo)
#   operator/  — created by this script

set -e

ROOT=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
OPERATOR_DIR="$ROOT/operator"
OPERATOR_MODULE="${OPERATOR_MODULE:-github.com/Rurutia1027/Spring-App-K8S-Operator/operator}"

# Default to a reachable Go proxy when proxy.golang.org is slow/unavailable.
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

command -v go >/dev/null 2>&1 || { echo "go not found"; exit 1; }
command -v kubebuilder >/dev/null 2>&1 || { echo "kubebuilder not found"; exit 1; }
command -v make >/dev/null 2>&1 || { echo "make not found"; exit 1; }

if [ -f "$OPERATOR_DIR/go.mod" ] && [ -d "$OPERATOR_DIR/api" ]; then
  echo "operator/ already exists — regenerating manifests"
  cd "$OPERATOR_DIR"
  make manifests generate
  exit 0
fi

mkdir -p "$OPERATOR_DIR"
cd "$OPERATOR_DIR"

if [ ! -f go.mod ]; then
  echo "kubebuilder init (downloading Go modules, may take 1–3 min)..."
  kubebuilder init \
    --domain example.com \
    --repo "$OPERATOR_MODULE"
else
  echo "operator/ partially initialized — skipping init"
fi

echo "kubebuilder create api (SpringApp)..."
kubebuilder create api \
  --group apps \
  --version v1alpha1 \
  --kind SpringApp \
  --resource \
  --controller \
  --force

echo "generating CRD and DeepCopy..."
make manifests generate

echo "operator/ framework ready"
