#!/usr/bin/env bash
# Create a preview environment for the payments FRONTEND MODULE.
#
# What happens:
#   1. Builds a "preview" version of payments-module with enhanced UI
#   2. Loads the image into k3d
#   3. Creates an Environment CR — Diverge deploys preview pod
#   4. Module registry detects preview pod and returns preview URL
#   5. Shell dynamically loads preview module instead of baseline
#
# Usage: ./preview-frontend.sh [preview-id]

set -euo pipefail

CLUSTER_NAME="diverge-demo"
DEMO_NS="demo-bank"
PREVIEW_ID="${1:-99}"

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
SERVICES_DIR="${DEMO_DIR}/.services"

echo ""
echo -e "${BOLD}${CYAN}🎨 Creating FRONTEND preview environment: ${YELLOW}preview-${PREVIEW_ID}${NC}"
echo -e "   This previews the ${BOLD}payments-module${NC} (frontend), not the API (backend)"
echo ""

# ─── 1. Build preview image ──────────────────────────────
step "Step 1/4 · Building preview frontend module"

SVC_DIR="${SERVICES_DIR}/demo-payments-module"
[[ -d "$SVC_DIR" ]] || err "Service not found. Run ./scripts/setup.sh first."

IMAGE="divergedev/demo-payments-module:preview-${PREVIEW_ID}"
log "Building ${IMAGE}..."
docker build -t "$IMAGE" "$SVC_DIR" --build-arg "APP_VERSION=preview-${PREVIEW_ID}" --quiet
ok "Frontend module image built"

# ─── 2. Load into k3d ────────────────────────────────────
step "Step 2/4 · Loading image into cluster"
log "Importing image to k3d..."
k3d image import "$IMAGE" -c "$CLUSTER_NAME"
ok "Image available in cluster"

# ─── 3. Create Environment CR ────────────────────────────
step "Step 3/4 · Creating Environment CR for frontend module"
log "Applying Environment custom resource..."
log "The Diverge controller will now:"
log "  → Deploy a preview pod for payments-module (frontend)"
log "  → Create an HTTPRoute with header matching"
log "  → Module registry will discover it automatically"

SHA=$(echo -n "demo-sha-frontend-${PREVIEW_ID}" | shasum -a 256 | cut -c1-12)

kubectl apply -n "$DEMO_NS" -f - <<EOF
apiVersion: diverge.io/v1alpha1
kind: Environment
metadata:
  name: preview-${PREVIEW_ID}
  namespace: ${DEMO_NS}
spec:
  source:
    provider: github
    project: divergedev/demo-payments-module
    mr: ${PREVIEW_ID}
    branch: feat/enhanced-payments-ui
    commitSHA: ${SHA}
  deploy:
    namespace: same
    changedServices:
      - payments-module
  routing:
    mode: header
    provider: gateway
    headerKey: x-preview-id
    headerValue: "${PREVIEW_ID}"
  serviceConfig:
    serviceName: payments-module
    namespace: ${DEMO_NS}
    port: 8080
    image: "${IMAGE}"
    parentRef: demo-gateway
    headerKey: x-preview-id
    pathPrefix: /modules/payments-module
    env:
      - name: APP_VERSION
        value: "preview-${PREVIEW_ID}"
  lifecycle:
    ttl: 1h
    cleanupOnMerge: true
EOF
ok "Environment CR created"

# ─── 4. Wait for controller reconciliation ───────────────
step "Step 4/4 · Waiting for controller to reconcile"
log "Waiting for preview pod to appear (polling up to 120s)..."

for _ in $(seq 1 30); do
    if kubectl get pod -l "diverge.io/environment=preview-${PREVIEW_ID}" -n "$DEMO_NS" 2>/dev/null | grep -q "preview-${PREVIEW_ID}"; then
        break
    fi
    sleep 4
done

log "Waiting for preview pod to become Ready (timeout: 60s)..."
if ! kubectl wait --for=condition=Ready pod \
    -l "diverge.io/environment=preview-${PREVIEW_ID},diverge.io/role=preview" \
    -n "$DEMO_NS" --timeout=60s 2>&1; then
    if ! kubectl wait --for=condition=Ready pod \
        -l "app=preview-${PREVIEW_ID}-payments-module" \
        -n "$DEMO_NS" --timeout=30s 2>&1; then
        echo ""
        log "Pod status:"
        kubectl get pods -n "$DEMO_NS" -l "diverge.io/environment=preview-${PREVIEW_ID}" 2>/dev/null || true
        err "Preview pod did not become Ready. Check: kubectl describe pod -n $DEMO_NS -l diverge.io/environment=preview-${PREVIEW_ID}"
    fi
fi
ok "Preview frontend module pod is Ready"

# ─── Summary ─────────────────────────────────────────────
echo ""
echo -e "${BOLD}${GREEN}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${GREEN}║                                                      ║${NC}"
echo -e "${BOLD}${GREEN}║    🎨  Frontend Preview Ready!                       ║${NC}"
echo -e "${BOLD}${GREEN}║    Preview ID: ${YELLOW}${PREVIEW_ID}${GREEN}                                    ║${NC}"
echo -e "${BOLD}${GREEN}║                                                      ║${NC}"
echo -e "${BOLD}${GREEN}╚══════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  ${BOLD}How it works:${NC}"
echo -e "    The shell (web-app) loads frontend modules dynamically."
echo -e "    When you enter preview ID ${YELLOW}${PREVIEW_ID}${NC}, the shell asks the"
echo -e "    ${CYAN}module registry${NC} for module URLs. The registry discovers"
echo -e "    the preview pod and returns the preview module URL."
echo ""
echo -e "  ${BOLD}Test it:${NC}"
echo ""
echo -e "  ${CYAN}1. Open the dashboard:${NC}"
echo -e "     ${BOLD}http://localhost:8080/${NC}"
echo ""
echo -e "  ${CYAN}2. Enter preview ID:${NC} ${YELLOW}${PREVIEW_ID}${NC}"
echo -e "     → Payments module loads from preview pod"
echo -e "     → Fee column appears (new feature)"
echo -e "     → Topology shows payments-module as ${GREEN}preview${NC}"
echo ""
echo -e "  ${CYAN}3. Clear preview ID:${NC}"
echo -e "     → Payments module loads from baseline pod"
echo -e "     → Fee column disappears"
echo -e "     → Same shell, same gateway, different module"
echo ""
echo -e "  ${BOLD}API verification:${NC}"
echo ""
echo -e "  ${CYAN}Module registry (baseline):${NC}"
echo -e "    ${BOLD}curl -s http://localhost:8080/api/module-registry | jq .${NC}"
echo ""
echo -e "  ${CYAN}Module registry (preview):${NC}"
echo -e "    ${BOLD}curl -s -H 'x-preview-id: ${PREVIEW_ID}' http://localhost:8080/api/module-registry | jq .${NC}"
echo ""
echo -e "  ${BOLD}Inspect:${NC}"
echo -e "    kubectl get environment -n ${DEMO_NS}"
echo -e "    kubectl get pods -n ${DEMO_NS}"
echo ""
