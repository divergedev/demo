#!/usr/bin/env bash
# Clean up a preview environment by deleting the Environment CR.
# The Diverge controller handles tearing down the preview pod and HTTPRoute.
#
# Usage: ./cleanup.sh [preview-id]

set -euo pipefail

DEMO_NS="demo-bank"
PREVIEW_ID="${1:-42}"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "  ${BLUE}▸${NC} $*"; }
ok()   { echo -e "  ${GREEN}✅${NC} $*"; }

echo ""
echo -e "${BOLD}${CYAN}🧹 Cleaning up preview: ${YELLOW}preview-${PREVIEW_ID}${NC}"
echo ""

# Delete the Environment CR — controller handles the rest
log "Deleting Environment CR..."
kubectl delete environment "preview-${PREVIEW_ID}" -n "$DEMO_NS" 2>/dev/null || true
ok "Environment CR deleted"

log "Waiting for controller to clean up resources..."
sleep 3

# Verify cleanup
REMAINING=$(kubectl get pods -n "$DEMO_NS" -l "diverge.io/environment=preview-${PREVIEW_ID}" --no-headers 2>/dev/null | wc -l | tr -d ' ')
if [[ "$REMAINING" == "0" ]]; then
    ok "Preview pod removed"
else
    log "Preview pod still terminating (this is normal)"
fi

ROUTES=$(kubectl get httproute -n "$DEMO_NS" -l "diverge.io/environment=preview-${PREVIEW_ID}" --no-headers 2>/dev/null | wc -l | tr -d ' ')
if [[ "$ROUTES" == "0" ]]; then
    ok "HTTPRoute removed"
else
    log "HTTPRoute still cleaning up"
fi

echo ""
echo -e "  ${GREEN}Preview ${YELLOW}${PREVIEW_ID}${GREEN} cleaned up.${NC}"
echo -e "  Baseline services are unaffected."
echo ""
