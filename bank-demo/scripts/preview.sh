#!/usr/bin/env bash
# Create a preview environment for payments-api using the Diverge controller.
#
# What happens:
#   1. Builds a "changed" version of payments-api
#   2. Loads the image into k3d
#   3. Creates an Environment CR — the Diverge controller handles the rest
#   4. Controller deploys preview pod + creates HTTPRoute automatically
#
# Usage: ./preview.sh [preview-id]

set -euo pipefail

CLUSTER_NAME="diverge-demo"
DEMO_NS="demo-bank"
PREVIEW_ID="${1:-42}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

step() { echo -e "\n${CYAN}━━━ $* ━━━${NC}"; }
log()  { echo -e "  ${BLUE}▸${NC} $*"; }
ok()   { echo -e "  ${GREEN}✅${NC} $*"; }
err()  { echo -e "  ${RED}❌${NC} $*"; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEMO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SERVICES_DIR="${DEMO_DIR}/services"

echo ""
echo -e "${BOLD}${CYAN}🚀 Creating preview environment: ${YELLOW}preview-${PREVIEW_ID}${NC}"
echo ""

# ─── 1. Build preview image ──────────────────────────────
step "Step 1/4 · Building preview image"

SVC_DIR="${SERVICES_DIR}/payments-api"
[[ -d "$SVC_DIR" ]] || err "Service not found. Run ./scripts/setup.sh first."

TMPDIR=$(mktemp -d)
cp -r "${SVC_DIR}/"* "$TMPDIR/" 2>/dev/null
# Simulate code changes: update version string
sed -i.bak "s/version = \"baseline\"/version = \"preview-${PREVIEW_ID}\"/" "$TMPDIR/main.go" 2>/dev/null || \
  sed -i '' "s/version = \"baseline\"/version = \"preview-${PREVIEW_ID}\"/" "$TMPDIR/main.go"

IMAGE="divergedev/demo-payments-api:preview-${PREVIEW_ID}"
log "Building ${IMAGE}..."
docker build -t "$IMAGE" "$TMPDIR" --quiet
ok "Image built"

# ─── 2. Load into k3d ────────────────────────────────────
step "Step 2/4 · Loading image into cluster"
log "Importing image to k3d..."
k3d image import "$IMAGE" -c "$CLUSTER_NAME"
rm -rf "$TMPDIR"
ok "Image available in cluster"

# Build migration image
MIGRATION_DIR="${DEMO_DIR}/migrations"
MIGRATION_IMAGE="divergedev/demo-migrations:latest"
log "Building migration image..."
docker build -t "$MIGRATION_IMAGE" "$MIGRATION_DIR" --quiet
k3d image import "$MIGRATION_IMAGE" -c "$CLUSTER_NAME"
ok "Migration image loaded"

# ─── 3. Create Environment CR ────────────────────────────
step "Step 3/4 · Creating Environment CR"
log "Applying Environment custom resource..."
log "The Diverge controller will now:"
log "  → Deploy a preview pod for payments-api"
log "  → Create an HTTPRoute with header matching"
log "  → Wire traffic for x-preview-id: ${PREVIEW_ID}"

SHA=$(echo -n "demo-sha-${PREVIEW_ID}" | sha256sum | cut -c1-12)

kubectl apply -n "$DEMO_NS" -f - <<EOF
apiVersion: diverge.io/v1alpha1
kind: Environment
metadata:
  name: preview-${PREVIEW_ID}
  namespace: ${DEMO_NS}
spec:
  source:
    provider: github
    project: divergedev/demo-payments-api
    mr: ${PREVIEW_ID}
    branch: feat/preview-${PREVIEW_ID}
    commitSHA: ${SHA}
  database:
    mode: schema
    migrationJob:
      image: divergedev/demo-migrations:latest
      args:
        - migrate
        - apply
        - --url
        - \$(DATABASE_URL)
        - --dir
        - file:///migrations
      timeoutSeconds: 120
  deploy:
    namespace: same
    changedServices:
      - payments-api
  routing:
    mode: header
    provider: gateway
    headerKey: x-preview-id
    headerValue: "${PREVIEW_ID}"
  serviceConfig:
    serviceName: payments-api
    namespace: ${DEMO_NS}
    port: 8080
    image: "${IMAGE}"
    parentRef: demo-gateway
    headerKey: x-preview-id
    pathPrefix: /api/payments
    env:
      - name: APP_VERSION
        value: "preview-${PREVIEW_ID}"
      - name: ACCOUNTS_API_URL
        value: "http://accounts-api:8080"
      - name: DATABASE_URL
        value: "postgres://postgres:diverge-demo-pass@postgres.demo-bank.svc.cluster.local:5432/diverge_preview?sslmode=disable&search_path=diverge_env_preview_${PREVIEW_ID}_$(echo -n "demo-bank/preview-${PREVIEW_ID}" | shasum -a 256 | cut -c1-8)"
  lifecycle:
    ttl: 1h
    cleanupOnMerge: true
EOF
ok "Environment CR created"

# ─── 4. Wait for controller reconciliation ───────────────
step "Step 4/4 · Waiting for controller to reconcile"
log "Waiting for preview pod to become Ready (timeout: 120s)..."

# Wait for the pod to be created by the controller (leader lease + reconcile takes time)
for i in $(seq 1 30); do
    if kubectl get pod -l "diverge.io/environment=preview-${PREVIEW_ID}" -n "$DEMO_NS" 2>/dev/null | grep -q "preview-${PREVIEW_ID}"; then
        break
    fi
    sleep 4
done

if ! kubectl wait --for=condition=Ready pod \
    -l "diverge.io/environment=preview-${PREVIEW_ID},diverge.io/role=preview" \
    -n "$DEMO_NS" --timeout=120s 2>/dev/null; then
    # Fallback: try matching by deployment name directly
    if ! kubectl wait --for=condition=Ready pod \
        -l "app=preview-${PREVIEW_ID}-payments-api" \
        -n "$DEMO_NS" --timeout=30s 2>/dev/null; then
        echo ""
        log "Pod status:"
        kubectl get pods -n "$DEMO_NS" -l "diverge.io/environment=preview-${PREVIEW_ID}" 2>/dev/null || true
        err "Preview pod did not become Ready within 120s. Check: kubectl describe pod -n $DEMO_NS -l diverge.io/environment=preview-${PREVIEW_ID}"
    fi
fi
ok "Preview pod is Ready"

# ─── Summary ─────────────────────────────────────────────
echo ""
echo -e "${BOLD}${GREEN}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${GREEN}║                                                      ║${NC}"
echo -e "${BOLD}${GREEN}║    ✅  Preview Environment Ready!                     ║${NC}"
echo -e "${BOLD}${GREEN}║    Preview ID: ${YELLOW}${PREVIEW_ID}${GREEN}                                    ║${NC}"
echo -e "${BOLD}${GREEN}║                                                      ║${NC}"
echo -e "${BOLD}${GREEN}╚══════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  ${BOLD}Test it:${NC}"
echo ""
echo -e "  ${CYAN}Baseline${NC} (no header → payments-api v1):"
echo -e "    ${BOLD}curl -s http://localhost:8080/api/payments | jq .version${NC}"
echo -e "    → ${GREEN}\"baseline\"${NC}"
echo ""
echo -e "  ${CYAN}Preview${NC} (with header → payments-api v2):"
echo -e "    ${BOLD}curl -s -H 'x-preview-id: ${PREVIEW_ID}' http://localhost:8080/api/payments | jq .version${NC}"
echo -e "    → ${GREEN}\"preview-${PREVIEW_ID}\"${NC}"
echo ""
echo -e "  ${CYAN}Other services${NC} (still baseline, header passes through):"
echo -e "    ${BOLD}curl -s -H 'x-preview-id: ${PREVIEW_ID}' http://localhost:8080/api/accounts | jq .version${NC}"
echo -e "    → ${GREEN}\"baseline\"${NC}"
echo ""
echo -e "  ${CYAN}Database Isolation${NC} (preview has 'fee' column):"
echo -e "    ${BOLD}curl -s -H 'x-preview-id: ${PREVIEW_ID}' http://localhost:8080/api/payments/transactions | jq '.transactions[0].fee'${NC}"
echo -e "    → ${GREEN}\"2.25\"${NC}  (new column only in preview schema)"
echo ""
echo -e "  ${CYAN}Baseline Unchanged${NC} (no 'fee' column):"
echo -e "    ${BOLD}curl -s http://localhost:8080/api/payments/transactions | jq '.transactions[0].fee'${NC}"
echo -e "    → ${GREEN}null${NC}  (column doesn't exist in baseline)"
echo ""
echo -e "  ${BOLD}Inspect:${NC}"
echo -e "    kubectl get environment -n ${DEMO_NS}"
echo -e "    kubectl get pods -n ${DEMO_NS}"
echo -e "    kubectl get httproute -n ${DEMO_NS}"
echo ""
echo -e "  ${BOLD}Cleanup:${NC}"
echo -e "    ${BOLD}./scripts/cleanup.sh ${PREVIEW_ID}${NC}"
echo ""
