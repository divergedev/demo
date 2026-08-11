#!/usr/bin/env bash
# Clean up a preview environment by deleting the Environment CR.
# The Diverge controller handles tearing down the preview pod and HTTPRoute.
#
# Usage: ./cleanup.sh [preview-id]

set -euo pipefail

DEMO_NS="demo-bank"
PREVIEW_ID="${1:-42}"
TIMEOUT=30

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "  ${BLUE}▸${NC} $*"; }
ok()   { echo -e "  ${GREEN}✅${NC} $*"; }
err()  { echo -e "  ${RED}❌${NC} $*"; exit 1; }

echo ""
echo -e "${BOLD}${CYAN}🧹 Cleaning up preview: ${YELLOW}preview-${PREVIEW_ID}${NC}"
echo ""

# Delete the Environment CR — controller handles the rest
log "Deleting Environment CR..."
if ! kubectl delete environment "preview-${PREVIEW_ID}" -n "$DEMO_NS" 2>/dev/null; then
    err "Failed to delete Environment CR preview-${PREVIEW_ID} (does it exist?)"
fi
ok "Environment CR deleted"

# Poll until controller finishes teardown
log "Waiting for controller to clean up resources (timeout: ${TIMEOUT}s)..."
ELAPSED=0
while [[ $ELAPSED -lt $TIMEOUT ]]; do
    ENV_GONE=true
    PODS_GONE=true
    ROUTES_GONE=true

    kubectl get environment "preview-${PREVIEW_ID}" -n "$DEMO_NS" --no-headers 2>/dev/null && ENV_GONE=false
    [[ $(kubectl get pods -n "$DEMO_NS" -l "diverge.io/environment=preview-${PREVIEW_ID}" --no-headers 2>/dev/null | wc -l | tr -d ' ') == "0" ]] || PODS_GONE=false
    [[ $(kubectl get httproute -n "$DEMO_NS" -l "diverge.io/environment=preview-${PREVIEW_ID}" --no-headers 2>/dev/null | wc -l | tr -d ' ') == "0" ]] || ROUTES_GONE=false

    if $ENV_GONE && $PODS_GONE && $ROUTES_GONE; then
        break
    fi
    sleep 2
    ELAPSED=$((ELAPSED + 2))
done

# Verify cleanup
if ! $ENV_GONE; then
    err "Environment CR preview-${PREVIEW_ID} still exists after ${TIMEOUT}s"
fi
ok "Environment CR removed"

if ! $PODS_GONE; then
    err "Preview pod still exists after ${TIMEOUT}s"
fi
ok "Preview pod removed"

if ! $ROUTES_GONE; then
    err "HTTPRoute still exists after ${TIMEOUT}s"
fi
ok "HTTPRoute removed"

echo ""
echo -e "  ${GREEN}Preview ${YELLOW}${PREVIEW_ID}${GREEN} cleaned up.${NC}"
echo -e "  Baseline services are unaffected."
echo ""
